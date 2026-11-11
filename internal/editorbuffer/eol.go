package editorbuffer

import (
	"bytes"
	"io"
	"sort"
	"unicode/utf8"
)

// A document whose lines do not all end the same way remembers how each line
// ended, so that a save writes a line the user did not retype the way it was
// read. The editing text holds one '\n' for every line break; the record is
// kept beside it, by position in the scratch file. Pieces refer to the scratch
// file by offset and text there is never rewritten, so a '\n' at a recorded
// offset is always the same line break, through any edit, undo or redo.
//
// The record is a list of segments. The first covers the imported text. An
// edit that replaces lines by as many lines (a case change, an indent, a
// sort) adds a segment for its new text, in which each line break is written
// like the one it replaced. Text outside every segment was typed or pasted:
// its line breaks get base, the most common ending of the file as it was read.
// Each recorded line costs two bits.

const (
	eolLF byte = iota
	eolCRLF
	eolCR
)

var eolNames = [...]string{"LF", "CRLF", "CR"}

// eolKinds collects line endings in order, two bits each. While they are all
// the same it only counts them.
type eolKinds struct {
	mixed bool
	first byte
	count int64
	kinds []byte
}

func (k *eolKinds) set(index int64, kind byte) {
	for int64(len(k.kinds)) <= index>>2 {
		k.kinds = append(k.kinds, 0)
	}
	k.kinds[index>>2] |= kind << ((index & 3) * 2)
}

func (k *eolKinds) add(kind byte) {
	if !k.mixed {
		if k.count == 0 || kind == k.first {
			k.first = kind
			k.count++
			return
		}
		// The first different ending: write down all the earlier ones.
		k.spread()
	}
	k.set(k.count, kind)
	k.count++
}

func (k *eolKinds) spread() {
	if k.mixed {
		return
	}
	k.mixed = true
	whole := k.count >> 2
	k.kinds = bytes.Repeat([]byte{k.first * 0x55}, int(whole))
	for i := whole << 2; i < k.count; i++ {
		k.set(i, k.first)
	}
}

// eolSegment records the line endings of the scratch range [start, end).
// starts and ranks are index points: ranks[i] line breaks of the segment lie
// before the scratch offset starts[i].
type eolSegment struct {
	start, end int64
	count      int64
	kinds      []byte
	starts     []int64
	ranks      []int64
}

func (g *eolSegment) kind(index int64) byte {
	return g.kinds[index>>2] >> ((index & 3) * 2) & 3
}

func newSegment(start, end int64, kinds *eolKinds, starts, ranks []int64) *eolSegment {
	kinds.spread()
	return &eolSegment{start: start, end: end, count: kinds.count, kinds: kinds.kinds, starts: starts, ranks: ranks}
}

// eolMap is immutable: an edit that adds a segment installs a new map, and a
// Snapshot keeps the map of its revision.
type eolMap struct {
	base     byte
	segments []*eolSegment // ordered by start; they never overlap
}

func (m *eolMap) with(segment *eolSegment) *eolMap {
	at := sort.Search(len(m.segments), func(i int) bool { return m.segments[i].start > segment.start })
	next := make([]*eolSegment, 0, len(m.segments)+1)
	next = append(append(append(next, m.segments[:at]...), segment), m.segments[at:]...)
	return &eolMap{base: m.base, segments: next}
}

func (m *eolMap) segment(offset int64) *eolSegment {
	i := sort.Search(len(m.segments), func(i int) bool { return m.segments[i].start > offset }) - 1
	if i < 0 || offset >= m.segments[i].end {
		return nil
	}
	return m.segments[i]
}

// eolCursor finds, for one pass over a document, which recorded line break a
// piece starts at. It keeps the stretch of scratch text it last counted, so a
// document cut into many pieces by edits reads each stretch once instead of
// once for every piece.
type eolCursor struct {
	m       *eolMap
	file    io.ReaderAt
	segment *eolSegment
	chunk   int
	data    []byte
	at      int   // bytes of data counted so far
	lines   int64 // '\n' in data[:at]
}

// locate returns the segment that records the line breaks at a scratch
// offset and the index, in that segment, of the first one at or after the
// offset. A nil segment means text added by hand: every break there is base.
func (c *eolCursor) locate(offset int64) (*eolSegment, int64, error) {
	g := c.m.segment(offset)
	if g == nil {
		return nil, 0, nil
	}
	i := sort.Search(len(g.starts), func(i int) bool { return g.starts[i] > offset }) - 1
	if i < 0 {
		return g, 0, nil
	}
	if c.data == nil || g != c.segment || i != c.chunk {
		end := g.end
		if i+1 < len(g.starts) {
			end = g.starts[i+1]
		}
		data := make([]byte, end-g.starts[i])
		if _, e := c.file.ReadAt(data, g.starts[i]); e != nil {
			return nil, 0, e
		}
		c.segment, c.chunk, c.data, c.at, c.lines = g, i, data, 0, 0
	}
	to := int(offset - g.starts[i])
	if to < c.at {
		c.at, c.lines = 0, 0
	}
	c.lines += int64(bytes.Count(c.data[c.at:to], []byte{'\n'}))
	c.at = to
	return g, g.ranks[i] + c.lines, nil
}

// kindsOf appends the ending of every line break of a tree, in order.
func (m *eolMap) kindsOf(file io.ReaderAt, root *node, out *eolKinds) error {
	cursor := &eolCursor{m: m, file: file}
	var visit func(*node) error
	visit = func(n *node) error {
		if n == nil || n.total.lines == 0 {
			return nil
		}
		if e := visit(n.left); e != nil {
			return e
		}
		if n.part.lines > 0 {
			g, line, e := cursor.locate(n.part.offset)
			if e != nil {
				return e
			}
			for i := int64(0); i < n.part.lines; i++ {
				if g == nil {
					out.add(m.base)
				} else {
					out.add(g.kind(line + i))
				}
			}
		}
		return visit(n.right)
	}
	return visit(root)
}

// inherit returns the map after new text at the scratch range [start, end)
// replaced the tree old with as many line breaks as old had: each new break
// is recorded with the ending of the one it stands for. With a different
// number of breaks nothing can be matched up; the map is returned unchanged
// and the new breaks are written with base.
func (m *eolMap) inherit(file io.ReaderAt, old, added *node, start, end int64, starts, ranks []int64) (*eolMap, error) {
	if m == nil || size(old).lines == 0 || size(old).lines != size(added).lines {
		return m, nil
	}
	kinds := &eolKinds{}
	if e := m.kindsOf(file, old, kinds); e != nil {
		return m, e
	}
	return m.with(newSegment(start, end, kinds, starts, ranks)), nil
}

// textIndex is the index of line breaks for text stored at a scratch offset.
func textIndex(start int64, text string) (starts, ranks []int64) {
	lines := int64(0)
	for at := 0; at < len(text) || at == 0; at += ChunkBytes {
		starts = append(starts, start+int64(at))
		ranks = append(ranks, lines)
		lines += int64(bytes.Count([]byte(text[at:min(len(text), at+ChunkBytes)]), []byte{'\n'}))
	}
	return starts, ranks
}

// commonEnds returns how many bytes two texts share at their start and, in
// what remains, at their end, both at character boundaries.
func commonEnds(old []byte, text string) (prefix, suffix int) {
	limit := min(len(old), len(text))
	for prefix < limit && old[prefix] == text[prefix] {
		prefix++
	}
	for prefix > 0 && prefix < len(text) && !utf8.RuneStart(text[prefix]) {
		prefix--
	}
	for suffix < limit-prefix && old[len(old)-1-suffix] == text[len(text)-1-suffix] {
		suffix++
	}
	for suffix > 0 && !utf8.RuneStart(text[len(text)-suffix]) {
		suffix--
	}
	return prefix, suffix
}

// NewLineEOL is the line ending of text added to a document whose imported
// line endings differ, or "" for any other document.
func (b *Buffer) NewLineEOL() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.eol == nil {
		return ""
	}
	return eolNames[b.eol.base]
}
