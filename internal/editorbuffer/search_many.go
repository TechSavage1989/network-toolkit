package editorbuffer

import (
	"bytes"
	"context"
	"io"
	"regexp"
	"unicode/utf8"
)

// The regular expression engine can read ahead before reporting a match.
// Cache one immutable piece so the next match can resume at the previous
// match's end without reading the same piece from disk for every occurrence.
type searchCursor struct {
	snapshot *Snapshot
	ctx      context.Context
	chunk    []byte
	base     int64
	at       int64
	// limit is where reading stops, as if the document ended there (a
	// window past a searched range, find.go within); -1 is the end of the
	// document. cut records that reading reached it.
	limit int64
	cut   bool
	err   error
	// onRead, when set, is told the position of each piece the cursor reads
	// (the progress of a long search).
	onRead func(at int64)
}

// current is the text from the cursor to the end of its piece, or to the
// limit when that comes first.
func (c *searchCursor) current() ([]byte, error) {
	if c.limit >= 0 && c.at >= c.limit {
		c.cut = true
		return nil, io.EOF
	}
	data, err := c.piece()
	if err == nil && c.limit >= 0 && int64(len(data)) > c.limit-c.at {
		data = data[:c.limit-c.at]
	}
	return data, err
}

func (c *searchCursor) piece() ([]byte, error) {
	if c.at >= c.base && c.at-c.base < int64(len(c.chunk)) {
		return c.chunk[c.at-c.base:], nil
	}
	if err := c.ctx.Err(); err != nil {
		return nil, err
	}
	n, base := c.snapshot.root, int64(0)
	for n != nil {
		start := base + size(n.left).bytes
		end := start + n.part.bytes
		switch {
		case c.at < start:
			n = n.left
		case c.at >= end:
			base, n = end, n.right
		default:
			data, err := c.snapshot.b.bytes(n.part)
			if err != nil {
				return nil, err
			}
			if c.onRead != nil {
				c.onRead(start)
			}
			c.chunk, c.base = data, start
			return data[c.at-start:], nil
		}
	}
	return nil, io.EOF
}

func (c *searchCursor) ReadRune() (rune, int, error) {
	data, err := c.current()
	if err != nil {
		if err != io.EOF {
			c.err = err
		}
		return 0, 0, err
	}
	r, n := utf8.DecodeRune(data)
	c.at += int64(n)
	return r, n, nil
}

type matchScan struct {
	pattern, resumed *regexp.Regexp
	cursor           searchCursor
	previous         searchCursor
	literal          []byte
	// groups holds the byte range of each group of the last match (-1 for a
	// group that took no part), group 0 being the whole match.
	groups []int64
	// limit is the byte position where the searched text ends (-1: the end
	// of the document). The text before a match is still read, so anchors
	// and word boundaries see the document.
	limit int64
}

func (s *Snapshot) matchScan(ctx context.Context, query string, regex, matchCase, wholeWord bool) (*matchScan, error) {
	return s.searchScan(ctx, Search{Query: query, Regex: regex, MatchCase: matchCase, WholeWord: wholeWord})
}

func (s *Snapshot) searchScan(ctx context.Context, q Search) (*matchScan, error) {
	pattern, err := q.pattern()
	if err != nil {
		return nil, err
	}
	plain, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	resumed, err := regexp.Compile(resumeSearchPattern(pattern))
	if err != nil {
		return nil, err
	}
	result := &matchScan{pattern: plain, resumed: resumed, cursor: searchCursor{snapshot: s, ctx: ctx, limit: -1}, previous: searchCursor{snapshot: s, ctx: ctx, limit: -1}, limit: -1}
	if !q.Regex && q.MatchCase && !q.WholeWord {
		result.literal = []byte(q.Query)
	}
	return result, nil
}

func (m *matchScan) next(at int64) (int64, int64, bool, error) {
	if err := m.cursor.ctx.Err(); err != nil {
		return 0, 0, false, err
	}
	total := size(m.cursor.snapshot.root).bytes
	limited := m.limit >= 0 && m.limit < total
	if limited {
		total = m.limit
	}
	if at > total {
		m.cursor.cut = limited
		return 0, 0, false, nil
	}
	m.cursor.limit = m.limit
	if len(m.literal) > 0 {
		// Plain text needs no context: only a search that ran into the limit
		// without a match was cut short.
		a, z, found, err := m.nextLiteral(at, total)
		m.cursor.cut = limited && !found
		return a, z, found, err
	}
	start, group, pattern := at, 0, m.pattern
	if at > 0 {
		// Include the preceding rune to keep start anchors and word boundaries
		// faithful to the document rather than a new input at the search cursor.
		start = at - 1
		for {
			m.previous.at = start
			p, err := m.previous.current()
			if err != nil {
				return 0, 0, false, err
			}
			if utf8.RuneStart(p[0]) {
				break
			}
			start--
		}
		pattern, group = m.resumed, 2
	}
	m.cursor.at, m.cursor.err = start, nil
	match := pattern.FindReaderSubmatchIndex(&m.cursor)
	if m.cursor.err != nil {
		return 0, 0, false, m.cursor.err
	}
	if match == nil {
		return 0, 0, false, nil
	}
	m.groups = m.groups[:0]
	for k := group; k+1 < len(match); k += 2 {
		if match[k] < 0 {
			m.groups = append(m.groups, -1, -1)
		} else {
			m.groups = append(m.groups, start+int64(match[k]), start+int64(match[k+1]))
		}
	}
	return start + int64(match[group]), start + int64(match[group+1]), true, nil
}

func (m *matchScan) nextLiteral(at, total int64) (int64, int64, bool, error) {
	width := len(m.literal)
	for at < total {
		m.cursor.at = at
		part, err := m.cursor.current()
		if err != nil {
			return 0, 0, false, err
		}
		if i := bytes.Index(part, m.literal); i >= 0 {
			return at + int64(i), at + int64(i+width), true, nil
		}
		end := at + int64(len(part))
		if end < total && width > 1 {
			// Only the possible starts crossing this piece boundary are copied.
			tail := part[max(0, len(part)-width+1):]
			joined := append(make([]byte, 0, 2*width-2), tail...)
			for pos := end; pos < min(total, end+int64(width-1)); {
				m.previous.at = pos
				next, err := m.previous.current()
				if err != nil {
					return 0, 0, false, err
				}
				n := min(len(next), width-1-int(pos-end))
				joined = append(joined, next[:n]...)
				pos += int64(n)
			}
			if i := bytes.Index(joined, m.literal); i >= 0 {
				from := end - int64(len(tail)) + int64(i)
				return from, from + int64(width), true, nil
			}
		}
		at = end
	}
	return 0, 0, false, nil
}

func (m *matchScan) after(from, to int64) (int64, error) {
	if to > from {
		return to, nil
	}
	// A zero-width regular expression must advance by one complete rune.
	m.cursor.at = to
	_, n, err := m.cursor.ReadRune()
	if err == io.EOF {
		return to + 1, nil
	}
	return to + int64(n), err
}

func (s *Snapshot) FindPrevious(ctx context.Context, query string, before int64, regex, matchCase, wholeWord bool) (Match, error) {
	result := Match{Revision: s.Revision}
	limit, err := s.bytePosition(s.root, before)
	if err != nil {
		return result, err
	}
	scan, err := s.matchScan(ctx, query, regex, matchCase, wholeWord)
	if err != nil {
		return result, err
	}
	var from, to int64
	for at := int64(0); at < limit; {
		a, z, found, err := scan.next(at)
		if err != nil {
			return result, err
		}
		if !found || a >= limit {
			break
		}
		result.Found, from, to = true, a, z
		at, err = scan.after(a, z)
		if err != nil {
			return result, err
		}
	}
	if result.Found {
		result.From, err = s.unitPosition(s.root, from)
		if err == nil {
			result.To, err = s.unitPosition(s.root, to)
		}
		if err == nil {
			result.Line, err = s.b.lineAt(s.root, result.From)
		}
	}
	return result, err
}
