package editorbuffer

import (
	"context"
	"errors"
	"io"
	"unicode/utf8"
)

// Replacement is a prepared Replace All: the document of one snapshot with
// every match replaced. Commit makes it the current text as one undo step.
type Replacement struct {
	b        *Buffer
	revision uint64
	root     *node
	eol      *eolMap
	Count    int64
	// Units is the new length of the replaced range (Rewrite).
	Units int64
	// The new text lies in the scratch file at [start, end). While nothing
	// else was stored in between or after it, Abort gives the space back.
	start, end int64
	contiguous bool
	done       bool
}

// A stretch of the document's own text between two matches that is shorter
// than this is copied next to the replacements; a longer one keeps its pieces.
const replaceInlineBytes = 4096

type leafWalk struct{ stack []*node }

func (w *leafWalk) push(n *node) {
	for ; n != nil; n = n.left {
		w.stack = append(w.stack, n)
	}
}

func (w *leafWalk) next() (piece, bool) {
	if len(w.stack) == 0 {
		return piece{}, false
	}
	n := w.stack[len(w.stack)-1]
	w.stack = w.stack[:len(w.stack)-1]
	w.push(n.right)
	return n.part, true
}

// ReplaceAll replaces each match on its own instead of replacing the whole
// document with the result. In a document whose lines keep their own endings
// (eol.go) that is what keeps a line without a match the way it was read:
//   - a long stretch between two matches keeps its pieces, so it still is the
//     text that was read;
//   - a short stretch is copied together with the replacements around it, and
//     every line break in the copy is recorded with the ending it had. That
//     bounds the number of pieces when a word on every line is replaced.
//
// A line break inside a replacement is new text. The work is one pass over
// the document; nothing changes until Commit. With a regular expression the
// replacement is a template ($1, ${name}, \n ...: template.go); the text of
// a group is copied from the document a chunk at a time.
func (s *Snapshot) ReplaceAll(ctx context.Context, query, replacement string, regex, matchCase, wholeWord bool) (*Replacement, error) {
	b := s.b
	if b.binary {
		return nil, ErrBinary
	}
	if err := validateEditFragment(replacement); err != nil {
		return nil, err
	}
	scan, err := s.matchScan(ctx, query, regex, matchCase, wholeWord)
	if err != nil {
		return nil, err
	}
	var parts []templatePart
	if regex {
		if parts, err = parseTemplate(replacement, scan.pattern.SubexpNames()); err != nil {
			return nil, err
		}
	}
	result := &Replacement{b: b, revision: s.Revision, eol: s.eol, start: -1, contiguous: true}
	// Whatever stops the work, a cancelled search or a failed read, the text
	// stored so far is given back.
	finished := false
	defer func() {
		if !finished {
			result.Abort()
		}
	}()
	var endings *eolCursor
	if s.eol != nil {
		endings = &eolCursor{m: s.eol, file: b.file}
	}
	// run is new text waiting to be stored: replacements and the short
	// stretches between them. runKinds holds the ending of each of its line
	// breaks. Runs stored one after the other share one record.
	var run, runKinds []byte
	var segmentKinds *eolKinds
	var segmentStarts, segmentRanks []int64
	segmentStart, segmentEnd := int64(-1), int64(-1)
	closeSegment := func() {
		if segmentKinds != nil && segmentKinds.count > 0 {
			result.eol = result.eol.with(newSegment(segmentStart, segmentEnd, segmentKinds, segmentStarts, segmentRanks))
		}
		segmentKinds, segmentStarts, segmentRanks = nil, nil, nil
	}
	flush := func() error {
		if len(run) == 0 {
			return nil
		}
		b.mu.Lock()
		if b.closed {
			b.mu.Unlock()
			return ErrClosed
		}
		start := b.end
		if result.start < 0 {
			result.start = start
		} else if start != result.end {
			result.contiguous = false
		}
		tree, err := b.append(run)
		end := b.end
		result.end = end
		b.mu.Unlock()
		if err != nil {
			return err
		}
		result.root = join(result.root, tree)
		if endings != nil {
			if segmentKinds == nil || segmentEnd != start {
				closeSegment()
				segmentKinds, segmentStart = &eolKinds{}, start
			}
			segmentStarts, segmentRanks = append(segmentStarts, start), append(segmentRanks, segmentKinds.count)
			for _, kind := range runKinds {
				segmentKinds.add(kind)
			}
			segmentEnd = end
		}
		run, runKinds = run[:0], runKinds[:0]
		return nil
	}
	// addNew adds new text (a replacement, or a group's text copied into
	// it): its line breaks are new and get the file's most common ending.
	addNew := func(p []byte) error {
		run = append(run, p...)
		if endings != nil {
			for _, c := range p {
				if c == '\n' {
					runKinds = append(runKinds, s.eol.base)
				}
			}
		}
		if len(run) >= ChunkBytes {
			return flush()
		}
		return nil
	}
	walk := &leafWalk{}
	walk.push(s.root)
	var leaf piece
	var data []byte
	leafStart, pos, started := int64(0), int64(0), false
	// keep carries the document's own text from pos up to a byte position.
	keep := func(target int64) error {
		inline := target-pos < replaceInlineBytes
		if !inline {
			if err := flush(); err != nil {
				return err
			}
		}
		for pos < target {
			for !started || leafStart+leaf.bytes <= pos {
				if started {
					leafStart += leaf.bytes
				}
				next, more := walk.next()
				if !more {
					return errors.New("search produced a replacement outside the document")
				}
				leaf, data, started = next, nil, true
			}
			from, to := pos-leafStart, min(leaf.bytes, target-leafStart)
			whole := from == 0 && to == leaf.bytes
			if (inline || !whole) && data == nil {
				if data, err = b.bytes(leaf); err != nil {
					return err
				}
			}
			switch {
			case inline:
				run = append(run, data[from:to]...)
				if endings != nil && leaf.lines > 0 {
					recorded, line, err := endings.locate(leaf.offset + from)
					if err != nil {
						return err
					}
					for _, c := range data[from:to] {
						if c != '\n' {
							continue
						}
						kind := s.eol.base
						if recorded != nil {
							kind = recorded.kind(line)
							line++
						}
						runKinds = append(runKinds, kind)
					}
				}
			case whole:
				result.root = join(result.root, branch(nil, leaf, nil))
			default:
				result.root = join(result.root, branch(nil, piece{leaf.offset + from, measureText(data[from:to])}, nil))
			}
			pos = leafStart + to
		}
		return nil
	}
	total := size(s.root).bytes
	for at := int64(0); at <= total; {
		from, to, found, err := scan.next(at)
		if err != nil {
			return nil, err
		}
		if !found {
			break
		}
		if from < pos {
			return nil, errors.New("search produced an invalid overlapping replacement")
		}
		if err = keep(from); err != nil {
			return nil, err
		}
		if parts == nil {
			err = addNew([]byte(replacement))
		} else {
			groups := append([]int64(nil), scan.groups...)
			for _, part := range parts {
				if err != nil {
					break
				}
				if part.group < 0 {
					err = addNew([]byte(part.text))
				} else if 2*part.group+1 < len(groups) && groups[2*part.group] >= 0 {
					err = s.copyBytes(ctx, groups[2*part.group], groups[2*part.group+1], addNew)
				}
			}
		}
		if err != nil {
			return nil, err
		}
		pos = to
		result.Count++
		if at, err = scan.after(from, to); err != nil {
			return nil, err
		}
	}
	if result.Count == 0 {
		finished = true
		return result, nil
	}
	if err = keep(total); err != nil {
		return nil, err
	}
	if err = flush(); err != nil {
		return nil, err
	}
	closeSegment()
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	finished = true
	return result, nil
}

// Abort gives up a Replace All that was not committed and returns the space
// of its new text to the scratch file. It is safe to call more than once and
// after Commit, where it does nothing. Every caller of ReplaceAll calls it
// (usually deferred) unless Commit succeeded.
func (r *Replacement) Abort() {
	b := r.b
	b.mu.Lock()
	defer b.mu.Unlock()
	if r.done {
		return
	}
	r.done = true
	r.root, r.eol = nil, nil
	// No live root points into text that was never committed. Only the tail
	// of the file is cut: never a later edit or another operation's text.
	if !b.closed && r.contiguous && r.start >= 0 && b.end == r.end && r.end > r.start && b.file.Truncate(r.start) == nil {
		b.end = r.start
	}
}

// Commit applies the replacement if the document is still at the revision it
// was prepared from.
//
// It does not hold the buffer open (the lifetime lock) the way an edit does:
// it reads nothing from the scratch file, and the closed flag is checked
// under the editing mutex. A caller may still hold the Snapshot the
// replacement was prepared from, and a second claim on the lifetime lock
// would wait forever behind a Close that is itself waiting for that Snapshot.
func (r *Replacement) Commit() (Metadata, error) {
	b := r.b
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return Metadata{}, ErrClosed
	}
	if b.binary {
		return b.metadataLocked(), ErrBinary
	}
	if r.done {
		return b.metadataLocked(), errors.New("this Replace All was already applied or given up")
	}
	if r.revision != b.revision {
		return b.metadataLocked(), ErrRevision
	}
	r.done = true
	if r.Count > 0 {
		b.undo = append(b.undo, b.root)
		b.redo = nil
		b.root = r.root
		b.eol = r.eol
		b.revision++
	}
	return b.metadataLocked(), nil
}

// copyBytes passes the document's bytes [from, to) to fn in chunks that end
// on a character boundary.
func (s *Snapshot) copyBytes(ctx context.Context, from, to int64, fn func([]byte) error) error {
	reader := &snapshotReader{ctx: ctx, b: s.b}
	reader.seek(s.root, from)
	in := io.LimitReader(reader, to-from)
	buf := make([]byte, ChunkBytes)
	var carry []byte
	for {
		n, err := in.Read(buf)
		if n > 0 {
			chunk := append(carry, buf[:n]...)
			cut := len(chunk)
			for k := 1; k <= utf8.UTFMax && k <= len(chunk); k++ {
				if utf8.RuneStart(chunk[len(chunk)-k]) {
					if !utf8.FullRune(chunk[len(chunk)-k:]) {
						cut = len(chunk) - k
					}
					break
				}
			}
			if e := fn(chunk[:cut]); e != nil {
				return e
			}
			carry = append([]byte(nil), chunk[cut:]...)
		}
		if err == io.EOF {
			if len(carry) > 0 {
				return fn(carry)
			}
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// ReplaceText is the Replace text for the match of query that is selected,
// [from, to): with a regular expression its template is expanded with that
// match's groups. found is false when the selection is not such a match.
func (s *Snapshot) ReplaceText(ctx context.Context, query, replacement string, from, to int64, regex, matchCase, wholeWord bool) (string, bool, error) {
	if s.b.binary {
		return "", false, ErrBinary
	}
	if err := validateEditFragment(replacement); err != nil {
		return "", false, err
	}
	scan, err := s.matchScan(ctx, query, regex, matchCase, wholeWord)
	if err != nil {
		return "", false, err
	}
	start, err := s.bytePosition(s.root, from)
	if err != nil {
		return "", false, err
	}
	end, err := s.bytePosition(s.root, to)
	if err != nil {
		return "", false, err
	}
	if end-start > MaxEditBytes {
		return "", false, errors.New("This match is too long to replace on its own. Use Replace all.")
	}
	a, z, found, err := scan.next(start)
	if err != nil || !found || a != start || z != end {
		return "", false, err
	}
	if !regex {
		return replacement, true, nil
	}
	parts, err := parseTemplate(replacement, scan.pattern.SubexpNames())
	if err != nil {
		return "", false, err
	}
	var groups []string
	for k := 0; k+1 < len(scan.groups); k += 2 {
		var text []byte
		if scan.groups[k] >= 0 {
			if err = s.copyBytes(ctx, scan.groups[k], scan.groups[k+1], func(p []byte) error { text = append(text, p...); return nil }); err != nil {
				return "", false, err
			}
		}
		groups = append(groups, string(text))
	}
	text := expandTemplate(parts, groups)
	if len(text) > MaxEditBytes {
		return "", false, errors.New("The replacement is too long to apply on its own. Use Replace all.")
	}
	return text, true, nil
}
