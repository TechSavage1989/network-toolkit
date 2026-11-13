package editorbuffer

import (
	"bytes"
	"context"
	"errors"
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
	scan.limit = hi
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
	lo, hi, err := s.span(from, to)
	if err != nil {
		return result, err
	}
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
		a, z, found, err = scan.next(position)
		if err == nil && !found && wrap && position > lo {
			a, z, found, err = scan.next(lo)
			found = found && a < position
			result.Wrapped = found
		}
	} else {
		a, z, found, err = scan.last(lo, position)
		if err == nil && !found && wrap && position < hi {
			a, z, found, err = scan.last(position, hi)
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

// last is the last match that starts in [from, before).
func (m *matchScan) last(from, before int64) (int64, int64, bool, error) {
	var a, z int64
	found := false
	for at := from; at < before; {
		x, y, ok, err := m.next(at)
		if err != nil {
			return 0, 0, false, err
		}
		if !ok || x >= before {
			break
		}
		a, z, found = x, y, true
		if at, err = m.after(x, y); err != nil {
			return 0, 0, false, err
		}
	}
	return a, z, found, nil
}

// each passes every match in [lo, hi) to fn, in order.
func (m *matchScan) each(lo, hi int64, fn func(a, z int64) error) error {
	for at := lo; at <= hi; {
		a, z, found, err := m.next(at)
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
	lo, hi, err := s.span(from, to)
	if err != nil {
		return 0, err
	}
	scan, err := s.scanIn(ctx, q, lo, hi, progress)
	if err != nil {
		return 0, err
	}
	var count int64
	err = scan.each(lo, hi, func(int64, int64) error { count++; return nil })
	return count, err
}

// Hit is one match: [From, To) in UTF-16 units, on Line (the line of From).
type Hit struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
	Line int64 `json:"line"`
}

// Matches lists the matches that start in [from, to): the marks of the
// part of a document that is shown. A match is read to at most 64 KiB past
// to (and never past limit, the end of the marked range; limit < 0 is the
// end of the document), so a page of a huge file never searches the rest of
// it. At most max matches are listed; more says there were others.
func (s *Snapshot) Matches(ctx context.Context, q Search, from, to, limit int64, max int) ([]Hit, bool, error) {
	units := size(s.root).units
	if limit < 0 || limit > units {
		limit = units
	}
	if to > limit {
		to = limit
	}
	if from > to {
		from = to
	}
	lo, end, err := s.span(from, to)
	if err != nil {
		return nil, false, err
	}
	_, hi, err := s.span(0, limit)
	if err != nil {
		return nil, false, err
	}
	hi = min(hi, end+64<<10)
	scan, err := s.scanIn(ctx, q, lo, hi, nil)
	if err != nil {
		return nil, false, err
	}
	track := s.tracker(lo)
	var hits []Hit
	more := false
	stop := errors.New("stop")
	err = scan.each(lo, hi, func(a, z int64) error {
		if a >= end {
			return stop
		}
		if len(hits) >= max {
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
	lo, hi, err := s.span(from, to)
	if err != nil {
		return 0, nil, false, err
	}
	scan, err := s.scanIn(ctx, q, lo, hi, progress)
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
	err = scan.each(lo, hi, func(a, z int64) error {
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

// MatchedText is the text of every match in [from, to), one match after the
// other with sep between them (Copy Marked Text). It is refused when it
// would be longer than max bytes.
func (s *Snapshot) MatchedText(ctx context.Context, q Search, from, to int64, sep string, max int64) (string, int64, error) {
	lo, hi, err := s.span(from, to)
	if err != nil {
		return "", 0, err
	}
	scan, err := s.scanIn(ctx, q, lo, hi, nil)
	if err != nil {
		return "", 0, err
	}
	var out []byte
	var count int64
	err = scan.each(lo, hi, func(a, z int64) error {
		if count > 0 {
			out = append(out, sep...)
		}
		count++
		if int64(len(out))+z-a > max {
			return ErrTooMuchText
		}
		return s.copyBytes(ctx, a, z, func(p []byte) error { out = append(out, p...); return nil })
	})
	return string(out), count, err
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
	lo, hi, err := s.span(from, to)
	if err != nil {
		return 0, err
	}
	scan, err := s.scanIn(ctx, q, lo, hi, progress)
	if err != nil {
		return 0, err
	}
	track := s.tracker(lo)
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
		r, err := s.row(ctx, row.line, row.startByte, row.fromByte, row.fromUnits)
		if err != nil {
			return err
		}
		r.Hits = row.hits
		row = nil
		return add(r)
	}
	err = scan.each(lo, hi, func(a, z int64) error {
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
func (s *Snapshot) row(ctx context.Context, line, lineByte, fromByte, fromUnits int64) (Row, error) {
	r := Row{Line: line}
	before, err := s.readBytes(ctx, max(lineByte, fromByte-4*rowBefore), fromByte)
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
	text, err := s.readBytes(ctx, start, min(total, start+4*rowText+4))
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

// readBytes reads the document's bytes [from, to), starting at the first
// character that begins in it.
func (s *Snapshot) readBytes(ctx context.Context, from, to int64) ([]byte, error) {
	if to <= from {
		return nil, nil
	}
	var out []byte
	err := s.copyBytes(ctx, from, to, func(p []byte) error { out = append(out, p...); return nil })
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
	lo, hi, err := s.span(from, to)
	if err != nil {
		return "", 0, err
	}
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
	err = scan.each(lo, hi, func(a, z int64) error {
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
