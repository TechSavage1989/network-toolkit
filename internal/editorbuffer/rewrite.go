package editorbuffer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"unicode/utf8"
)

// Text tools (internal/texttools) run over a range of a snapshot and report
// what they do as edits in range order: bytes they keep, bytes they drop and
// new text they insert. Rewrite turns those edits into a Replacement that
// replaces the range as one undo step, the way ReplaceAll does for its
// matches:
//   - a long stretch the tool kept keeps its pieces, so it still is the text
//     that was read; a short one is copied beside the new text with the
//     ending of each of its line breaks recorded, which bounds the number of
//     pieces when a tool changes something on every line;
//   - a line break in inserted text is new: it gets the file's most common
//     ending;
//   - a tool that rewrites its whole range (positional) and gives as many
//     lines as it read gives each line break the ending of the one in the
//     same place, as Buffer.Edit does for a sort.
//
// The new text goes to the scratch file as it comes, in chunks; memory holds
// one chunk. Nothing changes until Commit; Abort gives the space back.

// Edits receives a text tool's edits; texttools.Edits has the same methods.
type Edits interface {
	Keep(n int64) error
	Drop(n int64) error
	Insert(p []byte) error
}

// RangeSource reads a range of a snapshot, from its start, as often as asked.
type RangeSource struct {
	ctx        context.Context
	s          *Snapshot
	start, end int64 // bytes
}

func (r *RangeSource) Size() int64 { return r.end - r.start }

func (r *RangeSource) Open() (io.Reader, error) {
	reader := &snapshotReader{ctx: r.ctx, b: r.s.b}
	reader.seek(r.s.root, r.start)
	return io.LimitReader(reader, r.end-r.start), nil
}

// Source returns a reader of the text of [from, to) (UTF-16 units).
func (s *Snapshot) Source(ctx context.Context, from, to int64) (*RangeSource, error) {
	if from < 0 || to < from || to > size(s.root).units {
		return nil, ErrBoundary
	}
	start, err := s.bytePosition(s.root, from)
	if err != nil {
		return nil, err
	}
	end, err := s.bytePosition(s.root, to)
	if err != nil {
		return nil, err
	}
	return &RangeSource{ctx: ctx, s: s, start: start, end: end}, nil
}

// Units is the length of the snapshot's text.
func (s *Snapshot) Units() int64 { return size(s.root).units }

// LineRange widens [from, to) to the whole lines it touches: from the start
// of the first line to the end of the last, with the line break that ends it
// when there is one. A range that ends just after a line break ends on the
// line before it. With its line break, an empty last line is a line of the
// range (selecting "a", "" and "" of "a\n\n\nb" is "a\n\n\n", three lines);
// without it, the range would end where that empty line starts and a line
// tool would not see it.
func (s *Snapshot) LineRange(from, to int64) (int64, int64, error) {
	total := size(s.root)
	if from < 0 || to < from || to > total.units {
		return 0, 0, ErrBoundary
	}
	first, err := s.b.lineAt(s.root, from)
	if err != nil {
		return 0, 0, err
	}
	start, err := s.b.lineStart(s.root, first)
	if err != nil {
		return 0, 0, err
	}
	limit := to
	if to > from {
		var last []byte
		if err = s.b.readRange(s.root, to-1, to, &last); err == nil && len(last) == 1 && last[0] == '\n' {
			limit = to - 1
		}
	}
	line, err := s.b.lineAt(s.root, limit)
	if err != nil {
		return 0, 0, err
	}
	end := total.units
	if line < total.lines {
		if end, err = s.b.lineStart(s.root, line+1); err != nil {
			return 0, 0, err
		}
	}
	return start, max(end, start), nil
}

// ScratchDir is the folder of the document's scratch file, where a text
// tool spills sorted runs.
func (b *Buffer) ScratchDir() string { return filepath.Dir(b.file.Name()) }

// appender stores new text in a buffer's scratch file in chunks that end on
// a character boundary, and keeps the index of its line breaks.
type appender struct {
	b          *Buffer
	run        []byte
	tree       *node
	start, end int64
	contiguous bool
	starts     []int64
	ranks      []int64
	lines      int64
	// stored is called for each stored chunk with its scratch range.
	stored func(start, end int64, chunk []byte)
}

func newAppender(b *Buffer) *appender { return &appender{b: b, start: -1, contiguous: true} }

var errToolText = errors.New("a text tool produced text the editor cannot hold")

func (a *appender) add(p []byte) error {
	a.run = append(a.run, p...)
	if len(a.run) >= ChunkBytes {
		return a.flush(false)
	}
	return nil
}

// flush stores what is waiting. Before the end a character cut in two waits
// for the rest of it.
func (a *appender) flush(final bool) error {
	n := len(a.run)
	if !final {
		for k := 1; k <= utf8.UTFMax && k <= n; k++ {
			if utf8.RuneStart(a.run[n-k]) {
				if !utf8.FullRune(a.run[n-k:]) {
					n -= k
				}
				break
			}
		}
	}
	if n == 0 {
		return nil
	}
	chunk := a.run[:n]
	if !utf8.Valid(chunk) || bytes.IndexByte(chunk, 0) >= 0 || bytes.IndexByte(chunk, '\r') >= 0 {
		return errToolText
	}
	b := a.b
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return ErrClosed
	}
	start := b.end
	if a.start < 0 {
		a.start = start
	} else if start != a.end {
		a.contiguous = false
	}
	tree, err := b.append(chunk)
	end := b.end
	a.end = end
	b.mu.Unlock()
	if err != nil {
		return err
	}
	a.starts, a.ranks = append(a.starts, start), append(a.ranks, a.lines)
	a.lines += int64(bytes.Count(chunk, []byte{'\n'}))
	a.tree = join(a.tree, tree)
	if a.stored != nil {
		a.stored(start, end, chunk)
	}
	a.run = append(a.run[:0], a.run[n:]...)
	return nil
}

// abort gives the stored text back while nothing else was stored after it.
func (a *appender) abort() {
	b := a.b
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.closed && a.contiguous && a.start >= 0 && b.end == a.end && a.end > a.start && b.file.Truncate(a.start) == nil {
		b.end = a.start
	}
}

// rewriteSink builds the new text of a range from a tool's edits.
type rewriteSink struct {
	s          *Snapshot
	out        *appender
	endings    *eolCursor
	positional bool
	walk       leafWalk
	leaf       piece
	data       []byte
	leafStart  int64
	pos, limit int64
	started    bool
	mid        *node // kept pieces and stored text, in order
	kinds      []byte
	segment    struct {
		kinds         *eolKinds
		start, end    int64
		starts, ranks []int64
		segments      []*eolSegment
	}
}

// advance makes leaf the piece that holds pos.
func (w *rewriteSink) advance() error {
	for !w.started || w.leafStart+w.leaf.bytes <= w.pos {
		if w.started {
			w.leafStart += w.leaf.bytes
		}
		next, more := w.walk.next()
		if !more {
			return errors.New("a text tool went past the end of its text")
		}
		w.leaf, w.data, w.started = next, nil, true
	}
	return nil
}

func (w *rewriteSink) Keep(n int64) error {
	if n < 0 || w.pos+n > w.limit {
		return errors.New("a text tool kept text it was not given")
	}
	target := w.pos + n
	inline := n < replaceInlineBytes
	if !inline {
		if err := w.out.flush(true); err != nil {
			return err
		}
		w.mid = join(w.mid, w.out.tree)
		w.out.tree = nil
	}
	for w.pos < target {
		if err := w.advance(); err != nil {
			return err
		}
		from, to := w.pos-w.leafStart, min(w.leaf.bytes, target-w.leafStart)
		whole := from == 0 && to == w.leaf.bytes
		if (inline || !whole) && w.data == nil {
			var err error
			if w.data, err = w.s.b.bytes(w.leaf); err != nil {
				return err
			}
		}
		switch {
		case inline:
			part := w.data[from:to]
			if w.endings != nil && w.leaf.lines > 0 && bytes.IndexByte(part, '\n') >= 0 {
				recorded, line, err := w.endings.locate(w.leaf.offset + from)
				if err != nil {
					return err
				}
				for _, c := range part {
					if c != '\n' {
						continue
					}
					kind := w.s.eol.base
					if recorded != nil {
						kind = recorded.kind(line)
						line++
					}
					w.kinds = append(w.kinds, kind)
				}
			}
			if err := w.out.add(part); err != nil {
				return err
			}
		case whole:
			w.mid = join(w.mid, branch(nil, w.leaf, nil))
		default:
			w.mid = join(w.mid, branch(nil, piece{w.leaf.offset + from, measureText(w.data[from:to])}, nil))
		}
		w.pos = w.leafStart + to
	}
	return nil
}

func (w *rewriteSink) Drop(n int64) error {
	if n < 0 || w.pos+n > w.limit {
		return errors.New("a text tool dropped text it was not given")
	}
	w.pos += n
	return nil
}

func (w *rewriteSink) Insert(p []byte) error {
	if w.endings != nil && !w.positional {
		for _, c := range p {
			if c == '\n' {
				w.kinds = append(w.kinds, w.s.eol.base)
			}
		}
	}
	return w.out.add(p)
}

// recordKinds is the appender's stored hook: the line breaks of a stored
// chunk get the endings collected for them, in one segment per run of
// chunks stored one after the other (as in ReplaceAll).
func (w *rewriteSink) recordKinds(start, end int64, chunk []byte) {
	g := &w.segment
	count := bytes.Count(chunk, []byte{'\n'})
	if g.kinds == nil || g.end != start {
		w.closeSegment()
		g.kinds, g.start = &eolKinds{}, start
	}
	g.starts, g.ranks = append(g.starts, start), append(g.ranks, g.kinds.count)
	for _, kind := range w.kinds[:count] {
		g.kinds.add(kind)
	}
	w.kinds = w.kinds[count:]
	g.end = end
}

func (w *rewriteSink) closeSegment() {
	g := &w.segment
	if g.kinds != nil && g.kinds.count > 0 {
		g.segments = append(g.segments, newSegment(g.start, g.end, g.kinds, g.starts, g.ranks))
	}
	g.kinds, g.starts, g.ranks = nil, nil, nil
}

// Rewrite runs a text tool over [from, to) and prepares the result as a
// Replacement of that range. run returns whether the tool changed anything;
// when it did not, nothing is stored and the Replacement has Count 0.
// positional is set for a tool that rewrites its whole range.
//
// Like ReplaceAll, Rewrite stores text under the editing mutex only, never
// claiming the lifetime lock: the caller holds the Snapshot, and a second
// claim could wait forever behind a Close that waits for that Snapshot.
func (s *Snapshot) Rewrite(ctx context.Context, from, to int64, positional bool, run func(src *RangeSource, out Edits) (bool, error)) (*Replacement, error) {
	b := s.b
	if b.binary {
		return nil, ErrBinary
	}
	src, err := s.Source(ctx, from, to)
	if err != nil {
		return nil, err
	}
	left, rest, err := b.split(s.root, from)
	if err != nil {
		return nil, err
	}
	old, right, err := b.split(rest, to-from)
	if err != nil {
		return nil, err
	}
	w := &rewriteSink{s: s, out: newAppender(b), positional: positional, limit: src.Size()}
	if s.eol != nil {
		w.endings = &eolCursor{m: s.eol, file: b.file}
		if !positional {
			w.out.stored = w.recordKinds
		}
	}
	w.walk.push(old)
	finished := false
	defer func() {
		if !finished {
			w.out.abort()
		}
	}()
	changed, err := run(src, w)
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	result := &Replacement{b: b, revision: s.Revision, eol: s.eol, start: -1, contiguous: true}
	if !changed {
		// Commit of it applies nothing (Count 0), as a Replace All without a match.
		w.out.abort()
		finished = true
		return result, nil
	}
	if w.pos != w.limit {
		return nil, errors.New("a text tool did not account for all of its text")
	}
	if err = w.out.flush(true); err != nil {
		return nil, err
	}
	w.mid = join(w.mid, w.out.tree)
	w.out.tree = nil
	w.closeSegment()
	eol := s.eol
	for _, g := range w.segment.segments {
		eol = eol.with(g)
	}
	if positional && eol != nil && w.out.contiguous && w.out.start >= 0 && size(old).lines == size(w.mid).lines {
		if eol, err = eol.inherit(b.file, old, w.mid, w.out.start, w.out.end, w.out.starts, w.out.ranks); err != nil {
			return nil, err
		}
	}
	result.root = join(join(left, w.mid), right)
	result.eol = eol
	result.Count = 1
	result.Units = size(w.mid).units
	result.start, result.end, result.contiguous = w.out.start, w.out.end, w.out.contiguous
	finished = true
	return result, nil
}

// NewText makes a document from the text a tool inserts (a readable copy in
// a new tab). The text is stored as it comes; it is unsaved work in UTF-8
// with LF line breaks. The tool's Keep and Drop refer to its own input and
// are ignored.
func NewText(ctx context.Context, dir string, fill func(out Edits) error) (*Imported, error) {
	b, err := empty(dir)
	if err != nil {
		return nil, err
	}
	out := newAppender(b)
	sink := &textSink{out: out}
	if err = fill(sink); err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = out.flush(true)
	}
	if err != nil {
		b.Close()
		return nil, err
	}
	b.root = out.tree
	b.saved = &node{}
	return &Imported{Buffer: b, Format: Format{Encoding: "UTF-8", EOL: "LF"}, Bytes: size(b.root).bytes}, nil
}

type textSink struct{ out *appender }

func (t *textSink) Keep(int64) error      { return nil }
func (t *textSink) Drop(int64) error      { return nil }
func (t *textSink) Insert(p []byte) error { return t.out.add(p) }
