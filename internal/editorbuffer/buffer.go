// Package editorbuffer stores editable Unicode text on disk. Its persistent
// piece tree keeps editing and undo proportional to the changed region rather
// than the size of the document. Offsets exposed to renderers are UTF-16 units.
package editorbuffer

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"unicode/utf8"
)

const ChunkBytes = 64 * 1024
const MaxWindowUnits = 128 * 1024
const MaxEditBytes = 1024 * 1024

var ErrRevision = errors.New("the document changed; refresh this view before editing")
var ErrClosed = errors.New("the document buffer is closed")

// ErrBinary answers every attempt to change or write a file that is not text.
var ErrBinary = errors.New("This is not a text file. It is shown read-only and cannot be changed or saved in the Text Editor.")
var ErrBoundary = errors.New("text position splits a Unicode character or is outside the document")

type measure struct{ bytes, units, lines int64 }

func (a measure) plus(b measure) measure {
	return measure{a.bytes + b.bytes, a.units + b.units, a.lines + b.lines}
}

type piece struct {
	offset int64
	measure
}
type node struct {
	left, right *node
	part        piece
	height      int
	total       measure
}

func size(n *node) measure {
	if n == nil {
		return measure{}
	}
	return n.total
}
func height(n *node) int {
	if n == nil {
		return 0
	}
	return n.height
}
func rawNode(left *node, p piece, right *node) *node {
	return &node{left: left, right: right, part: p, height: 1 + max(height(left), height(right)), total: size(left).plus(p.measure).plus(size(right))}
}
func balance(left *node, p piece, right *node) *node {
	if height(left) > height(right)+1 {
		if height(left.left) < height(left.right) {
			c := left.right
			left = rawNode(rawNode(left.left, left.part, c.left), c.part, c.right)
		}
		return rawNode(left.left, left.part, rawNode(left.right, p, right))
	}
	if height(right) > height(left)+1 {
		if height(right.right) < height(right.left) {
			c := right.left
			right = rawNode(c.left, c.part, rawNode(c.right, right.part, right.right))
		}
		return rawNode(rawNode(left, p, right.left), right.part, right.right)
	}
	return rawNode(left, p, right)
}
func branch(left *node, p piece, right *node) *node {
	if height(left) > height(right)+1 {
		return balance(left.left, left.part, branch(left.right, p, right))
	}
	if height(right) > height(left)+1 {
		return balance(branch(left, p, right.left), right.part, right.right)
	}
	return rawNode(left, p, right)
}
func takeFirst(n *node) (piece, *node) {
	if n.left == nil {
		return n.part, n.right
	}
	p, rest := takeFirst(n.left)
	return p, balance(rest, n.part, n.right)
}
func join(a, b *node) *node {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	p, rest := takeFirst(b)
	return branch(a, p, rest)
}
func measureText(p []byte) measure {
	m := measure{bytes: int64(len(p))}
	for len(p) > 0 {
		r, n := utf8.DecodeRune(p)
		p = p[n:]
		m.units++
		if r > 0xffff {
			m.units++
		}
		if r == '\n' {
			m.lines++
		}
	}
	return m
}

type Metadata struct {
	Revision uint64 `json:"revision"`
	Units    int64  `json:"units"`
	Bytes    int64  `json:"bytes"`
	Lines    int64  `json:"lines"`
	Dirty    bool   `json:"dirty"`
	CanUndo  bool   `json:"canUndo"`
	CanRedo  bool   `json:"canRedo"`
}
type Window struct {
	Metadata
	Start     int64  `json:"start"`
	Text      string `json:"text"`
	Line      int64  `json:"line"`
	Column    int64  `json:"column"`
	LineUnits int64  `json:"lineUnits"`
}
type Buffer struct {
	// lifetime lets a save read an immutable tree while edits append new pieces.
	// Only Close takes it exclusively; a long save never holds the editing mutex.
	lifetime    sync.RWMutex
	mu          sync.RWMutex
	file        *os.File
	root, saved *node
	undo, redo  []*node
	revision    uint64
	end         int64
	closed      bool
	// eol is set at import for a document whose line endings differ (eol.go).
	eol *eolMap
	// binary marks the display-only rendering of a file that is not text.
	binary bool
}

func empty(dir string) (*Buffer, error) {
	f, e := os.CreateTemp(dir, "nt-editor-buffer-*")
	if e != nil {
		return nil, e
	}
	return &Buffer{file: f, revision: 1}, nil
}
func (b *Buffer) append(p []byte) (*node, error) {
	var tree *node
	for len(p) > 0 {
		n := min(len(p), ChunkBytes)
		for n < len(p) && !utf8.RuneStart(p[n]) {
			n--
		}
		part := p[:n]
		off := b.end
		written, e := b.file.WriteAt(part, off)
		b.end += int64(written)
		if e != nil {
			return nil, e
		}
		if written != len(part) {
			return nil, io.ErrShortWrite
		}
		tree = join(tree, branch(nil, piece{off, measureText(part)}, nil))
		p = p[n:]
	}
	return tree, nil
}
func (b *Buffer) bytes(p piece) ([]byte, error) {
	data := make([]byte, int(p.bytes))
	_, e := b.file.ReadAt(data, p.offset)
	return data, e
}
func byteAt(data []byte, units int64) (int, error) {
	if units < 0 {
		return 0, ErrBoundary
	}
	at := 0
	for units > 0 && at < len(data) {
		r, n := utf8.DecodeRune(data[at:])
		u := int64(1)
		if r > 0xffff {
			u = 2
		}
		if units < u {
			return 0, ErrBoundary
		}
		units -= u
		at += n
	}
	if units != 0 {
		return 0, ErrBoundary
	}
	return at, nil
}
func (b *Buffer) split(n *node, units int64) (*node, *node, error) {
	if units < 0 || units > size(n).units {
		return nil, nil, ErrBoundary
	}
	if n == nil {
		return nil, nil, nil
	}
	l := size(n.left).units
	if units < l {
		a, c, e := b.split(n.left, units)
		if e != nil {
			return nil, nil, e
		}
		return a, branch(c, n.part, n.right), nil
	}
	if units > l+n.part.units {
		a, c, e := b.split(n.right, units-l-n.part.units)
		if e != nil {
			return nil, nil, e
		}
		return branch(n.left, n.part, a), c, nil
	}
	if units == l {
		return n.left, branch(nil, n.part, n.right), nil
	}
	if units == l+n.part.units {
		return branch(n.left, n.part, nil), n.right, nil
	}
	data, e := b.bytes(n.part)
	if e != nil {
		return nil, nil, e
	}
	at, e := byteAt(data, units-l)
	if e != nil {
		return nil, nil, e
	}
	left := piece{n.part.offset, measureText(data[:at])}
	right := piece{n.part.offset + int64(at), measureText(data[at:])}
	return branch(n.left, left, nil), branch(nil, right, n.right), nil
}
func (b *Buffer) metadataLocked() Metadata {
	m := size(b.root)
	return Metadata{b.revision, m.units, m.bytes, m.lines + 1, b.root != b.saved, len(b.undo) > 0, len(b.redo) > 0}
}
func (b *Buffer) Metadata() Metadata { b.mu.RLock(); defer b.mu.RUnlock(); return b.metadataLocked() }
func (b *Buffer) Edit(revision uint64, from, to int64, text string) (Metadata, error) {
	b.lifetime.RLock()
	defer b.lifetime.RUnlock()
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return Metadata{}, ErrClosed
	}
	if b.binary {
		return b.metadataLocked(), ErrBinary
	}
	if revision != b.revision {
		return b.metadataLocked(), ErrRevision
	}
	if from < 0 || to < from || to > size(b.root).units {
		return b.metadataLocked(), ErrBoundary
	}
	if e := validateEditFragment(text); e != nil {
		return b.metadataLocked(), e
	}
	if b.eol != nil && from < to && text != "" && to-from <= MaxEditBytes {
		// Lines keep their own endings in this document. A command that
		// rewrites a range hands over text that mostly repeats what was
		// there (duplicate a line, change one word of a selection): what the
		// new text repeats at the start and the end of the range is not
		// replaced at all, so those lines stay the lines that were read.
		var old []byte
		if e := b.readRange(b.root, from, to, &old); e != nil {
			return b.metadataLocked(), e
		}
		if string(old) != text {
			prefix, suffix := commonEnds(old, text)
			from += measureText(old[:prefix]).units
			to -= measureText(old[len(old)-suffix:]).units
			text = text[prefix : len(text)-suffix]
		}
	}
	left, rest, e := b.split(b.root, from)
	if e != nil {
		return b.metadataLocked(), e
	}
	replaced, right, e := b.split(rest, to-from)
	if e != nil {
		return b.metadataLocked(), e
	}
	start := b.end
	insert, e := b.append([]byte(text))
	if e != nil {
		return b.metadataLocked(), e
	}
	if from == to && text == "" {
		return b.metadataLocked(), nil
	}
	if b.eol != nil && insert != nil {
		// As many lines as before (a case change, an indent, a sort): each
		// line break is written like the one it replaced.
		starts, ranks := textIndex(start, text)
		if b.eol, e = b.eol.inherit(b.file, replaced, insert, start, b.end, starts, ranks); e != nil {
			return b.metadataLocked(), e
		}
	}
	b.undo = append(b.undo, b.root)
	b.redo = nil
	b.root = join(join(left, insert), right)
	b.revision++
	return b.metadataLocked(), nil
}
func (b *Buffer) History(revision uint64, redo bool) (Metadata, error) {
	b.lifetime.RLock()
	defer b.lifetime.RUnlock()
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return Metadata{}, ErrClosed
	}
	if b.binary {
		return b.metadataLocked(), ErrBinary
	}
	if revision != b.revision {
		return b.metadataLocked(), ErrRevision
	}
	from, to := &b.undo, &b.redo
	if redo {
		from, to = to, from
	}
	if len(*from) == 0 {
		return b.metadataLocked(), nil
	}
	*to = append(*to, b.root)
	b.root = (*from)[len(*from)-1]
	*from = (*from)[:len(*from)-1]
	b.revision++
	return b.metadataLocked(), nil
}

// readRange visits only the tree branches that intersect this UTF-16 range.
func (b *Buffer) readRange(n *node, from, to int64, dst *[]byte) error {
	if n == nil || from == to {
		return nil
	}
	l := size(n.left).units
	end := l + n.part.units
	if from < l {
		if e := b.readRange(n.left, from, min(to, l), dst); e != nil {
			return e
		}
	}
	if to > l && from < end {
		data, e := b.bytes(n.part)
		if e != nil {
			return e
		}
		a, e := byteAt(data, max(0, from-l))
		if e != nil {
			return e
		}
		z, e := byteAt(data, min(n.part.units, to-l))
		if e != nil {
			return e
		}
		*dst = append(*dst, data[a:z]...)
	}
	if to > end {
		return b.readRange(n.right, max(0, from-end), to-end, dst)
	}
	return nil
}
func (b *Buffer) lineAt(n *node, units int64) (int64, error) {
	if n == nil {
		return 0, nil
	}
	l := size(n.left)
	if units < l.units {
		return b.lineAt(n.left, units)
	}
	if units >= l.units+n.part.units {
		v, e := b.lineAt(n.right, units-l.units-n.part.units)
		return l.lines + n.part.lines + v, e
	}
	data, e := b.bytes(n.part)
	if e != nil {
		return 0, e
	}
	at, e := byteAt(data, units-l.units)
	if e != nil {
		return 0, e
	}
	return l.lines + measureText(data[:at]).lines, nil
}
func (b *Buffer) lineStart(n *node, line int64) (int64, error) {
	if line == 0 {
		return 0, nil
	}
	if n == nil || line < 0 || line > size(n).lines {
		return 0, ErrBoundary
	}
	l := size(n.left)
	if line <= l.lines {
		return b.lineStart(n.left, line)
	}
	line -= l.lines
	if line <= n.part.lines {
		data, e := b.bytes(n.part)
		if e != nil {
			return 0, e
		}
		units := l.units
		for len(data) > 0 {
			r, k := utf8.DecodeRune(data)
			data = data[k:]
			units++
			if r > 0xffff {
				units++
			}
			if r == '\n' {
				line--
				if line == 0 {
					return units, nil
				}
			}
		}
		return 0, ErrBoundary
	}
	at, e := b.lineStart(n.right, line-n.part.lines)
	return l.units + n.part.units + at, e
}
func (b *Buffer) Window(start int64, limit int) (Window, error) {
	b.lifetime.RLock()
	defer b.lifetime.RUnlock()
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return Window{}, ErrClosed
	}
	return b.windowLocked(start, limit)
}
func (b *Buffer) WindowAtLine(line int64, limit int) (Window, error) {
	b.lifetime.RLock()
	defer b.lifetime.RUnlock()
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return Window{}, ErrClosed
	}
	start, e := b.lineStart(b.root, line)
	if e != nil {
		return Window{}, e
	}
	return b.windowLocked(start, limit)
}

// WindowNear permits a viewport to start at an approximate UTF-16 position.
// Mutation ranges still use strict boundaries; only a read moves left by one
// unit when its requested starting position falls inside a surrogate pair.
func (b *Buffer) WindowNear(start int64, limit int) (Window, error) {
	b.lifetime.RLock()
	defer b.lifetime.RUnlock()
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return Window{}, ErrClosed
	}
	if start < 0 || start > size(b.root).units {
		return Window{}, ErrBoundary
	}
	if _, err := b.lineAt(b.root, start); err == ErrBoundary && start > 0 {
		start--
	} else if err != nil {
		return Window{}, err
	}
	return b.windowLocked(start, limit)
}
func (b *Buffer) windowLocked(start int64, limit int) (Window, error) {
	if start < 0 || start > size(b.root).units || limit < 1 || limit > MaxWindowUnits {
		return Window{}, ErrBoundary
	}
	end := min(size(b.root).units, start+int64(limit))
	data := make([]byte, 0, limit)
	if e := b.readRange(b.root, start, end, &data); e != nil {
		if e != ErrBoundary || end == start {
			return Window{}, e
		}
		data = data[:0]
		end--
		if e = b.readRange(b.root, start, end, &data); e != nil {
			return Window{}, e
		}
	}
	line, e := b.lineAt(b.root, start)
	if e != nil {
		return Window{}, e
	}
	lineStart, e := b.lineStart(b.root, line)
	if e != nil {
		return Window{}, e
	}
	lineEnd := size(b.root).units
	if line < size(b.root).lines {
		next, err := b.lineStart(b.root, line+1)
		if err != nil {
			return Window{}, err
		}
		lineEnd = next - 1
	}
	return Window{Metadata: b.metadataLocked(), Start: start, Text: string(data), Line: line, Column: start - lineStart, LineUnits: lineEnd - lineStart}, nil
}
func (b *Buffer) Close() error {
	b.lifetime.Lock()
	defer b.lifetime.Unlock()
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	e := b.file.Close()
	remove := os.Remove(b.file.Name())
	if e != nil {
		return e
	}
	return remove
}

// Snapshot supports background search and streaming save without blocking edits.
// Its Close must be called, including when a caller cancels a long operation.
type Snapshot struct {
	b        *Buffer
	root     *node
	eol      *eolMap
	Revision uint64
	once     sync.Once
}

func (b *Buffer) Snapshot() (*Snapshot, error) {
	b.lifetime.RLock()
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		b.lifetime.RUnlock()
		return nil, ErrClosed
	}
	return &Snapshot{b: b, root: b.root, eol: b.eol, Revision: b.revision}, nil
}
func (s *Snapshot) Close() { s.once.Do(func() { s.b.lifetime.RUnlock() }) }
func (s *Snapshot) WriteTo(ctx context.Context, w io.Writer) error {
	var visit func(*node) error
	visit = func(n *node) error {
		if n == nil {
			return nil
		}
		if e := ctx.Err(); e != nil {
			return e
		}
		if e := visit(n.left); e != nil {
			return e
		}
		if _, e := io.Copy(w, io.NewSectionReader(s.b.file, n.part.offset, n.part.bytes)); e != nil {
			return e
		}
		return visit(n.right)
	}
	return visit(s.root)
}

// WriteKept writes the text for a kept copy of unsaved work: UTF-8 with one
// '\n' for each line ending, except that a document whose imported line
// endings differ is written with each line's own ending, so importing the
// copy again restores the record of them.
func (s *Snapshot) WriteKept(ctx context.Context, w io.Writer) error {
	if s.b.binary {
		// There is never unsaved work in a file that is not text, and its
		// display rendering must not reach a file.
		return ErrBinary
	}
	if s.eol == nil {
		return s.WriteTo(ctx, w)
	}
	return s.Encode(ctx, w, Format{Encoding: "UTF-8", EOL: MixedEOL})
}

// MarkUnsaved records that the current text has not been written to the
// document's file, for example after restoring a kept copy of unsaved edits.
// Only a later MarkSaved clears it; undo history is unchanged.
func (b *Buffer) MarkUnsaved() Metadata {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.binary {
		// A file that is not text never has unsaved work.
		return b.metadataLocked()
	}
	b.saved = &node{}
	return b.metadataLocked()
}

func (s *Snapshot) MarkSaved() Metadata {
	s.b.mu.Lock()
	defer s.b.mu.Unlock()
	s.b.saved = s.root
	return s.b.metadataLocked()
}
