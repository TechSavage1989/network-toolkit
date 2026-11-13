package editorbuffer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

// The Find window of the Text Editor (Find, Replace and Mark, owner request
// v1.21.55) searches every document with this code: a disk-backed document
// through a Snapshot of its buffer, text held by a page through TextSnapshot.
// Positions are UTF-16 units and lines count from 0, as everywhere in the
// editor.

// Search is one search of the Find window.
type Search struct {
	Query     string `json:"query"`
	Regex     bool   `json:"regex"`
	MatchCase bool   `json:"matchCase"`
	WholeWord bool   `json:"wholeWord"`
	// DotAll: "." also matches a line break (". matches newline", regular
	// expressions only).
	DotAll bool `json:"dotAll"`
}

// TextSnapshot is a read-only snapshot of text that a page holds, so the
// same search code runs over it. Nothing is written to disk.
func TextSnapshot(text string) (*Snapshot, error) {
	if !utf8.ValidString(text) {
		return nil, errors.New("The text is not valid Unicode.")
	}
	data := []byte(text)
	var tree *node
	for off := 0; off < len(data); {
		n := min(len(data)-off, ChunkBytes)
		for off+n < len(data) && !utf8.RuneStart(data[off+n]) {
			n--
		}
		tree = join(tree, branch(nil, piece{int64(off), measureText(data[off : off+n])}, nil))
		off += n
	}
	b := &Buffer{mem: data, root: tree, saved: tree, revision: 1}
	b.lifetime.RLock()
	return &Snapshot{b: b, root: tree, Revision: 1}, nil
}

// scope is the searched part of a document, in bytes: a match counts when it
// lies entirely in [lo, hi). A match is found as in the whole text, so ^, $,
// word edges and anchors see the text around the range; a match that runs
// past hi, or that starts at hi, is outside. The whole document (whole) also
// counts a match of empty text at its very end.
type scope struct {
	lo, hi int64
	whole  bool
}

func (s *Snapshot) scope(from, to int64) (scope, error) {
	lo, hi, err := s.span(from, to)
	return scope{lo, hi, to < 0}, err
}

// searchWindow is how far past the part that matters the text is read for
// the context of a match before the search looks further (within).
const searchWindow = 64 << 10

// within is the first match at or after at that lies in sc and starts
// before want (want <= sc.hi). The text after the range is read only as far
// as the match needs: in steps from searchWindow, and again with more text
// whenever the text past the window could have changed the match found.
func (m *matchScan) within(at int64, sc scope, want int64) (int64, int64, bool, error) {
	total := size(m.cursor.snapshot.root).bytes
	for window := int64(searchWindow); ; window *= 8 {
		m.limit = -1
		if end := min(want, sc.hi) + window; end < total {
			m.limit = end
		}
		m.cursor.cut = false
		a, z, found, err := m.next(at)
		m.limit = -1
		if err != nil {
			return 0, 0, false, err
		}
		if found && m.cursor.cut {
			// The match was found in text that ends at the window; the rest
			// of the document could make it another match.
			continue
		}
		// Without a match in the window there is none in range either: every
		// match that ends in the range is a match of the window's text.
		if !found || z > sc.hi || a == sc.hi && !sc.whole || a >= want && want < sc.hi {
			return 0, 0, false, nil
		}
		return a, z, true, nil
	}
}

// span is a range in UTF-16 units as bytes; to < 0 is the end of the
// document.
func (s *Snapshot) span(from, to int64) (int64, int64, error) {
	units := size(s.root).units
	if to < 0 {
		to = units
	}
	if from < 0 || from > to || to > units {
		return 0, 0, ErrBoundary
	}
	lo, err := s.bytePosition(s.root, from)
	if err != nil {
		return 0, 0, err
	}
	hi, err := s.bytePosition(s.root, to)
	return lo, hi, err
}

// scanIn prepares a search of the bytes [lo, hi) and reports how far it has
// read as a fraction of that range.
func (s *Snapshot) scanIn(ctx context.Context, q Search, lo, hi int64, progress func(float64)) (*matchScan, error) {
	scan, err := s.searchScan(ctx, q)
	if err != nil {
		return nil, err
	}
	if progress != nil && hi > lo {
		scan.cursor.onRead = func(at int64) { progress(min(1, max(0, float64(at-lo)/float64(hi-lo)))) }
	}
	return scan, nil
}

// FindNext is Find Next of the Find window: the first match at or after at
// in the range [from, to) (to < 0: the end of the document), or with
// backwards the last match that starts before at. With wrap a search that
// reaches the end of the range goes on from its other end.
func (s *Snapshot) FindNext(ctx context.Context, q Search, at, from, to int64, backwards, wrap bool) (Match, error) {
	result := Match{Revision: s.Revision}
	sc, err := s.scope(from, to)
	if err != nil {
		return result, err
	}
	lo, hi := sc.lo, sc.hi
	scan, err := s.scanIn(ctx, q, lo, hi, nil)
	if err != nil {
		return result, err
	}
	units := size(s.root).units
	if to < 0 {
		to = units
	}
	position, err := s.bytePosition(s.root, min(max(at, from), to))
	if err != nil {
		return result, err
	}
	var a, z int64
	var found bool
	if !backwards {
		a, z, found, err = scan.forward(position, sc)
		if err == nil && !found && wrap && position > lo {
			a, z, found, err = scan.within(lo, sc, sc.hi)
			found = found && a < position
			result.Wrapped = found
		}
	} else {
		// Matches that start before the position; a match of empty text at
		// the position itself is the one already found.
		a, z, found, err = scan.last(sc, position)
		if err == nil && !found && wrap && position < hi {
			a, z, found, err = scan.last(sc, hi+1)
			found = found && a >= position
			result.Wrapped = found
		}
	}
	if err != nil || !found {
		result.Wrapped = false
		return result, err
	}
	result.Found = true
	if result.From, err = s.unitPosition(s.root, a); err != nil {
		return result, err
	}
	if result.To, err = s.unitPosition(s.root, z); err != nil {
		return result, err
	}
	result.Line, err = s.b.lineAt(s.root, result.From)
	return result, err
}

// forward is the first match in sc at or after at, passing over a match of
// empty text at at itself: Find Next from a match of empty text (^$ on an
// empty line) moves on.
func (m *matchScan) forward(at int64, sc scope) (int64, int64, bool, error) {
	a, z, found, err := m.within(at, sc, sc.hi)
	if err == nil && found && a == at && z == at {
		var next int64
		if next, err = m.after(a, z); err != nil || next > sc.hi {
			return 0, 0, false, err
		}
		return m.within(next, sc, sc.hi)
	}
	return a, z, found, err
}

// last is the last match in sc that starts before before.
func (m *matchScan) last(sc scope, before int64) (int64, int64, bool, error) {
	var a, z int64
	found := false
	err := m.each(sc, func(x, y int64) error {
		if x >= before {
			return errStop
		}
		a, z, found = x, y, true
		return nil
	})
	if err == errStop {
		err = nil
	}
	return a, z, found, err
}

var errStop = errors.New("stop")

// each passes every match in sc to fn, in order.
func (m *matchScan) each(sc scope, fn func(a, z int64) error) error {
	for at := sc.lo; at <= sc.hi; {
		a, z, found, err := m.within(at, sc, sc.hi)
		if err != nil || !found {
			return err
		}
		if err = fn(a, z); err != nil {
			return err
		}
		if at, err = m.after(a, z); err != nil {
			return err
		}
	}
	return nil
}

// Count is the number of matches in [from, to).
func (s *Snapshot) Count(ctx context.Context, q Search, from, to int64, progress func(float64)) (int64, error) {
	sc, err := s.scope(from, to)
	if err != nil {
		return 0, err
	}
	scan, err := s.scanIn(ctx, q, sc.lo, sc.hi, progress)
	if err != nil {
		return 0, err
	}
	var count int64
	err = scan.each(sc, func(int64, int64) error { count++; return nil })
	return count, err
}

// Hit is one match: [From, To) in UTF-16 units, on Line (the line of From).
type Hit struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
	Line int64 `json:"line"`
}

// Matches lists the matches that start in [from, to) and lie in the marked
// range [scopeFrom, scopeTo) (scopeTo < 0: the whole document): the marks
// of the part of a document that is shown. The text past to is read only as
// far as a match needs (within), so a page of a huge file never searches
// the rest of it. At most max matches are listed; more says there were
// others.
func (s *Snapshot) Matches(ctx context.Context, q Search, from, to, scopeFrom, scopeTo int64, most int) ([]Hit, bool, error) {
	units := size(s.root).units
	if scopeTo > units {
		scopeTo = units
	}
	scopeFrom = min(max(scopeFrom, 0), units)
	if scopeTo >= 0 && scopeFrom > scopeTo {
		scopeFrom = scopeTo
	}
	sc, err := s.scope(scopeFrom, scopeTo)
	if err != nil {
		return nil, false, err
	}
	to = min(max(to, 0), units)
	from = min(max(from, 0), to)
	lo, end, err := s.span(from, to)
	if err != nil {
		return nil, false, err
	}
	lo, end = max(lo, sc.lo), min(end, sc.hi)
	if end < lo {
		return nil, false, nil
	}
	sc.lo = lo
	scan, err := s.scanIn(ctx, q, lo, end, nil)
	if err != nil {
		return nil, false, err
	}
	track := s.tracker(lo)
	var hits []Hit
	more := false
	stop := errors.New("stop")
	err = scan.each(sc, func(a, z int64) error {
		if a >= end && !(a == end && end == sc.hi && sc.whole) {
			return stop
		}
		if len(hits) >= most {
			more = true
			return stop
		}
		if err := track.advance(a); err != nil {
			return err
		}
		hit := Hit{From: track.units, Line: track.lines}
		if err := track.advance(z); err != nil {
			return err
		}
		hit.To = track.units
		hits = append(hits, hit)
		return nil
	})
	if err == stop {
		err = nil
	}
	return hits, more, err
}

// Mark counts the matches in [from, to) and lists the lines they start on
// (Mark All with "Bookmark line"), at most maxLines of them; more says that
// other lines had matches too.
func (s *Snapshot) Mark(ctx context.Context, q Search, from, to int64, maxLines int, progress func(float64)) (int64, []int64, bool, error) {
	sc, err := s.scope(from, to)
	if err != nil {
		return 0, nil, false, err
	}
	lo := sc.lo
	scan, err := s.scanIn(ctx, q, sc.lo, sc.hi, progress)
	if err != nil {
		return 0, nil, false, err
	}
	var count int64
	var lines []int64
	more := false
	var track *posTracker
	if maxLines > 0 {
		track = s.tracker(lo)
	}
	err = scan.each(sc, func(a, z int64) error {
		count++
		if track == nil || more {
			return nil
		}
		if err := track.advance(a); err != nil {
			return err
		}
		if n := len(lines); n == 0 || lines[n-1] != track.lines {
			if n >= maxLines {
				more = true
				return nil
			}
			lines = append(lines, track.lines)
		}
		return nil
	})
	return count, lines, more, err
}

// MarkSearch is one search of Mark All: its pattern and the range it
// marked (nil: the whole document).
type MarkSearch struct {
	Search Search    `json:"search"`
	Within *[2]int64 `json:"within"`
}

// MatchedText is the text of every match of one search in [from, to), with
// sep between them (Copy Marked Text of one mark).
func (s *Snapshot) MatchedText(ctx context.Context, q Search, from, to int64, sep string, max int64) (string, int64, error) {
	within := &[2]int64{from, to}
	if to < 0 {
		within = nil
	}
	return s.MarkedText(ctx, []MarkSearch{{Search: q, Within: within}}, sep, max)
}

// MarkedText is the marked text of a document (Copy Marked Text): the
// matches of every mark, in document order, with sep between them. Text
// that several marks (or overlapping matches) cover is copied once, as one
// stretch. Matches of empty text copy nothing. It is refused when it would
// be longer than most bytes, and it returns the number of stretches.
func (s *Snapshot) MarkedText(ctx context.Context, marks []MarkSearch, sep string, most int64) (string, int64, error) {
	type source struct {
		scan *matchScan
		sc   scope
		a, z int64
		ok   bool
	}
	var sources []*source
	advance := func(src *source, at int64) error {
		var err error
		src.ok = false
		if at <= src.sc.hi {
			src.a, src.z, src.ok, err = src.scan.within(at, src.sc, src.sc.hi)
		}
		return err
	}
	for _, mark := range marks {
		from, to := int64(0), int64(-1)
		if mark.Within != nil {
			from, to = mark.Within[0], mark.Within[1]
		}
		sc, err := s.scope(from, to)
		if err != nil {
			return "", 0, err
		}
		scan, err := s.scanIn(ctx, mark.Search, sc.lo, sc.hi, nil)
		if err != nil {
			return "", 0, err
		}
		src := &source{scan: scan, sc: sc}
		if err = advance(src, sc.lo); err != nil {
			return "", 0, err
		}
		sources = append(sources, src)
	}
	var out []byte
	var count int64
	read := s.reader(ctx)
	start, end := int64(-1), int64(-1)
	flush := func() error {
		if start < 0 || end <= start {
			return nil
		}
		if count > 0 {
			out = append(out, sep...)
		}
		count++
		if int64(len(out))+end-start > most {
			return ErrTooMuchText
		}
		var err error
		out, err = read.appendBytes(out, start, end)
		return err
	}
	for {
		var next *source
		for _, src := range sources {
			if src.ok && (next == nil || src.a < next.a) {
				next = src
			}
		}
		if next == nil {
			break
		}
		a, z := next.a, next.z
		if z > a {
			if start >= 0 && a <= end {
				end = max(end, z)
			} else {
				if err := flush(); err != nil {
					return "", 0, err
				}
				start, end = a, z
			}
		}
		at, err := next.scan.after(a, z)
		if err == nil {
			err = advance(next, at)
		}
		if err != nil {
			return "", 0, err
		}
	}
	if err := flush(); err != nil {
		return "", 0, err
	}
	return string(out), count, nil
}

// ErrTooMuchText refuses to copy more marked text than the clipboard is
// given at once.
var ErrTooMuchText = errors.New("The marked text is too large to copy at once. Mark fewer matches or select a smaller range.")

// Row is one row of a Find All result: a line with one or more matches, as
// much of the line as is shown, and the matches on it. A long line is cut
// down around its matches (CutStart, CutEnd), and its matches may take
// several rows.
type Row struct {
	Line int64 `json:"line"`
	// Start is the position of Text in the document (UTF-16 units).
	Start    int64  `json:"start"`
	Text     string `json:"text"`
	CutStart bool   `json:"cutStart,omitempty"`
	CutEnd   bool   `json:"cutEnd,omitempty"`
	// Hits are the matches of the row, [from, to) in the document. A match
	// may go on past the end of Text (one that spans lines).
	Hits [][2]int64 `json:"hits"`
}

const (
	// rowBefore is how much of a line is shown before the row's first match,
	// rowText the longest text of a row, rowSpan the distance from the row's
	// first match within which later matches on its line share the row, and
	// rowHits the most matches of one row (all UTF-16 units).
	rowBefore = 40
	rowText   = 240
	rowSpan   = 160
	rowHits   = 64
)

// FindAll passes each row of the matches in [from, to) to add, in order,
// and returns the number of matches.
func (s *Snapshot) FindAll(ctx context.Context, q Search, from, to int64, add func(Row) error, progress func(float64)) (int64, error) {
	sc, err := s.scope(from, to)
	if err != nil {
		return 0, err
	}
	lo, hi := sc.lo, sc.hi
	scan, err := s.scanIn(ctx, q, lo, hi, progress)
	if err != nil {
		return 0, err
	}
	track, read := s.tracker(lo), s.reader(ctx)
	type pending struct {
		line, startByte, startUnits, fromByte, fromUnits int64
		hits                                             [][2]int64
	}
	var row *pending
	var hits int64
	emit := func() error {
		if row == nil {
			return nil
		}
		r, err := s.row(read, row.line, row.startByte, row.fromByte, row.fromUnits)
		if err != nil {
			return err
		}
		r.Hits = row.hits
		row = nil
		return add(r)
	}
	err = scan.each(sc, func(a, z int64) error {
		if err := track.advance(a); err != nil {
			return err
		}
		fromUnits, line := track.units, track.lines
		if row != nil && (line != row.line || fromUnits-row.fromUnits > rowSpan || len(row.hits) >= rowHits) {
			if err := emit(); err != nil {
				return err
			}
		}
		if row == nil {
			lineByte, lineUnits, err := track.lineStart()
			if err != nil {
				return err
			}
			row = &pending{line: line, startByte: lineByte, startUnits: lineUnits, fromByte: a, fromUnits: fromUnits}
		}
		if err := track.advance(z); err != nil {
			return err
		}
		row.hits = append(row.hits, [2]int64{fromUnits, track.units})
		hits++
		return nil
	})
	if err == nil {
		err = emit()
	}
	return hits, err
}

// row reads the text of a row whose first match starts at fromByte on a
// line that starts at lineByte.
func (s *Snapshot) row(read *searchCursor, line, lineByte, fromByte, fromUnits int64) (Row, error) {
	r := Row{Line: line}
	before, err := read.readBytes(max(lineByte, fromByte-4*rowBefore), fromByte)
	if err != nil {
		return r, err
	}
	// The last rowBefore units of the line before the match.
	cut, units := len(before), int64(0)
	for cut > 0 {
		c, n := utf8.DecodeLastRune(before[:cut])
		w := int64(1)
		if c > 0xffff {
			w = 2
		}
		if units+w > rowBefore {
			break
		}
		units += w
		cut -= n
	}
	start := fromByte - int64(len(before)-cut)
	r.CutStart = start > lineByte
	r.Start = fromUnits - units
	total := size(s.root).bytes
	text, err := read.readBytes(start, min(total, start+4*rowText+4))
	if err != nil {
		return r, err
	}
	end, shown := 0, int64(0)
	r.CutEnd = true
	for end < len(text) {
		c, n := utf8.DecodeRune(text[end:])
		if c == '\n' || c == utf8.RuneError && n == 1 && !utf8.FullRune(text[end:]) {
			r.CutEnd = c != '\n' && start+int64(end) < total
			break
		}
		w := int64(1)
		if c > 0xffff {
			w = 2
		}
		if shown+w > rowText {
			break
		}
		shown += w
		end += n
	}
	if end == len(text) && start+int64(end) >= total {
		r.CutEnd = false
	}
	r.Text = string(text[:end])
	return r, nil
}

// reader reads small stretches of a snapshot one after the other, keeping
// the piece it last read: the rows of a Find All and the stretches of Copy
// Marked Text are read without fetching a piece again for each of them.
func (s *Snapshot) reader(ctx context.Context) *searchCursor {
	return &searchCursor{snapshot: s, ctx: ctx, limit: -1}
}

// appendBytes adds the document's bytes [from, to) to out.
func (c *searchCursor) appendBytes(out []byte, from, to int64) ([]byte, error) {
	for at := from; at < to; {
		c.at = at
		part, err := c.current()
		if err == io.EOF {
			break
		}
		if err != nil {
			return out, err
		}
		n := min(int64(len(part)), to-at)
		out = append(out, part[:n]...)
		at += n
	}
	return out, nil
}

// readBytes reads the document's bytes [from, to), starting at the first
// character that begins in it.
func (c *searchCursor) readBytes(from, to int64) ([]byte, error) {
	if to <= from {
		return nil, nil
	}
	out, err := c.appendBytes(nil, from, to)
	for len(out) > 0 && !utf8.RuneStart(out[0]) {
		out = out[1:]
	}
	return out, err
}

// posTracker follows a scan through the document and knows the UTF-16
// position and line of the byte it has reached, and where its line starts.
// Whole pieces are passed by their measure; only the piece a position falls
// in is read.
type posTracker struct {
	s     *Snapshot
	stack []*node
	has   bool
	leaf  piece
	// The document's measure up to the start of the leaf.
	leafByte, leafUnits, leafLines int64
	data                           []byte
	at, units, lines               int64
	// The start of the current line, when known; otherwise it is after the
	// last line break of the pending piece that was passed whole.
	lineKnown          bool
	lineByte, lineUnit int64
	pend               piece
	pendByte, pendUnit int64
	err                error
}

func (s *Snapshot) tracker(at int64) *posTracker {
	t := &posTracker{s: s}
	t.err = t.seek(at)
	return t
}

func (t *posTracker) seek(target int64) error {
	n := t.s.root
	var base measure
	for n != nil {
		left := size(n.left)
		start := base.plus(left)
		end := start.bytes + n.part.bytes
		switch {
		case target < start.bytes:
			t.stack = append(t.stack, n)
			n = n.left
		case target < end || target == end && n.right == nil:
			t.has, t.leaf = true, n.part
			t.leafByte, t.leafUnits, t.leafLines = start.bytes, start.units, start.lines
			t.at, t.units, t.lines = start.bytes, start.units, start.lines
			for r := n.right; r != nil; r = r.left {
				t.stack = append(t.stack, r)
			}
			n = nil
		default:
			base = start.plus(n.part.measure)
			n = n.right
		}
	}
	// Where the line the tracker starts on starts is found from the tree once.
	var err error
	if t.lineUnit, err = t.s.b.lineStart(t.s.root, t.lines); err != nil {
		return err
	}
	if t.lineByte, err = t.s.bytePosition(t.s.root, t.lineUnit); err != nil {
		return err
	}
	t.lineKnown = true
	return t.advance(target)
}

func (t *posTracker) load() error {
	if t.data == nil {
		data, err := t.s.b.bytes(t.leaf)
		if err != nil {
			return err
		}
		t.data = data
	}
	return nil
}

// pass measures the leaf's bytes from t.at to end (within the leaf).
func (t *posTracker) pass(end int64) error {
	if end <= t.at {
		return nil
	}
	if t.at == t.leafByte && end == t.leafByte+t.leaf.bytes && t.data == nil {
		// A whole piece that was not read: its measure is known.
		if t.leaf.lines > 0 {
			t.lineKnown = false
			t.pend, t.pendByte, t.pendUnit = t.leaf, t.leafByte, t.leafUnits
		}
		t.at, t.units, t.lines = end, t.units+t.leaf.units, t.lines+t.leaf.lines
		return nil
	}
	if err := t.load(); err != nil {
		return err
	}
	seg := t.data[t.at-t.leafByte : end-t.leafByte]
	if i := bytes.LastIndexByte(seg, '\n'); i >= 0 {
		head := measureText(seg[:i+1])
		t.lineKnown, t.lineByte, t.lineUnit = true, t.at+int64(i)+1, t.units+head.units
		rest := measureText(seg[i+1:])
		t.units += head.units + rest.units
		t.lines += head.lines + rest.lines
	} else {
		m := measureText(seg)
		t.units += m.units
		t.lines += m.lines
	}
	t.at = end
	return nil
}

func (t *posTracker) advance(target int64) error {
	if t.err != nil {
		return t.err
	}
	for t.has && target > t.leafByte+t.leaf.bytes {
		if err := t.pass(t.leafByte + t.leaf.bytes); err != nil {
			return err
		}
		if len(t.stack) == 0 {
			t.has = false
			break
		}
		n := t.stack[len(t.stack)-1]
		t.stack = t.stack[:len(t.stack)-1]
		for r := n.right; r != nil; r = r.left {
			t.stack = append(t.stack, r)
		}
		t.leaf, t.data = n.part, nil
		t.leafByte, t.leafUnits, t.leafLines = t.at, t.units, t.lines
	}
	if target < t.at || !t.has && target > t.at {
		return ErrBoundary
	}
	if !t.has {
		return nil
	}
	return t.pass(target)
}

// lineStart is where the line of the tracker's position starts, in bytes
// and in UTF-16 units.
func (t *posTracker) lineStart() (int64, int64, error) {
	if !t.lineKnown {
		data, err := t.s.b.bytes(t.pend)
		if err != nil {
			return 0, 0, err
		}
		i := bytes.LastIndexByte(data, '\n')
		t.lineKnown, t.lineByte, t.lineUnit = true, t.pendByte+int64(i)+1, t.pendUnit+measureText(data[:i+1]).units
	}
	return t.lineByte, t.lineUnit, nil
}

// ReplaceInText is Replace All over text that a page holds: the text with
// every match in [from, to) (to < 0: the end) replaced, and the number of
// matches. With a regular expression the replacement is a template, as in
// a disk-backed document (template.go). The result is bounded: max bytes.
func ReplaceInText(ctx context.Context, text string, q Search, replacement string, from, to, max int64) (string, int64, error) {
	if !utf8.ValidString(replacement) || strings.ContainsRune(replacement, 0) {
		return "", 0, errors.New("The replacement is not valid text.")
	}
	s, err := TextSnapshot(text)
	if err != nil {
		return "", 0, err
	}
	defer s.Close()
	sc, err := s.scope(from, to)
	if err != nil {
		return "", 0, err
	}
	lo, hi := sc.lo, sc.hi
	scan, err := s.scanIn(ctx, q, lo, hi, nil)
	if err != nil {
		return "", 0, err
	}
	var parts []templatePart
	if q.Regex {
		if parts, err = parseTemplate(replacement, scan.pattern.SubexpNames()); err != nil {
			return "", 0, err
		}
	}
	data := s.b.mem
	var out strings.Builder
	var pos, count int64
	err = scan.each(sc, func(a, z int64) error {
		out.Write(data[pos:a])
		if parts == nil {
			out.WriteString(replacement)
		} else {
			groups := make([]string, 0, len(scan.groups)/2)
			for k := 0; k+1 < len(scan.groups); k += 2 {
				if scan.groups[k] >= 0 {
					groups = append(groups, string(data[scan.groups[k]:scan.groups[k+1]]))
				} else {
					groups = append(groups, "")
				}
			}
			out.WriteString(expandTemplate(parts, groups))
		}
		pos = z
		count++
		if int64(out.Len()) > max {
			return errors.New("The result of Replace All would be too large for this document.")
		}
		return nil
	})
	if err != nil {
		return "", 0, err
	}
	if count == 0 {
		return text, 0, nil
	}
	out.Write(data[pos:])
	if int64(out.Len()) > max {
		return "", 0, errors.New("The result of Replace All would be too large for this document.")
	}
	return out.String(), count, nil
}
