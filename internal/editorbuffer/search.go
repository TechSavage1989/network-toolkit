package editorbuffer

import (
	"bufio"
	"context"
	"errors"
	"io"
	"regexp"
)

// snapshotReader traverses immutable pieces with one chunk of working memory.
type snapshotReader struct {
	ctx   context.Context
	b     *Buffer
	stack []*node
	data  []byte
	skip  int64
	err   error
}

func (r *snapshotReader) push(n *node) {
	for n != nil {
		r.stack = append(r.stack, n)
		n = n.left
	}
}

func (r *snapshotReader) seek(n *node, skip int64) {
	for n != nil {
		left := size(n.left).bytes
		if skip < left {
			r.stack = append(r.stack, n)
			n = n.left
		} else if skip < left+n.part.bytes {
			r.stack = append(r.stack, n)
			r.skip = skip - left
			return
		} else {
			skip -= left + n.part.bytes
			n = n.right
		}
	}
}
func (r *snapshotReader) Read(p []byte) (int, error) {
	for len(r.data) == 0 {
		if e := r.ctx.Err(); e != nil {
			r.err = e
			return 0, e
		}
		if len(r.stack) == 0 {
			return 0, io.EOF
		}
		n := r.stack[len(r.stack)-1]
		r.stack = r.stack[:len(r.stack)-1]
		r.push(n.right)
		if r.skip >= n.part.bytes {
			r.skip -= n.part.bytes
			continue
		}
		data, e := r.b.bytes(n.part)
		if e != nil {
			r.err = e
			return 0, e
		}
		r.data = data[r.skip:]
		r.skip = 0
	}
	count := copy(p, r.data)
	r.data = r.data[count:]
	return count, nil
}
func (s *Snapshot) bytePosition(n *node, units int64) (int64, error) {
	if units < 0 || units > size(n).units {
		return 0, ErrBoundary
	}
	if n == nil {
		return 0, nil
	}
	l := size(n.left)
	if units < l.units {
		return s.bytePosition(n.left, units)
	}
	if units >= l.units+n.part.units {
		at, e := s.bytePosition(n.right, units-l.units-n.part.units)
		return l.bytes + n.part.bytes + at, e
	}
	data, e := s.b.bytes(n.part)
	if e != nil {
		return 0, e
	}
	at, e := byteAt(data, units-l.units)
	return l.bytes + int64(at), e
}
func (s *Snapshot) unitPosition(n *node, offset int64) (int64, error) {
	if offset < 0 || offset > size(n).bytes {
		return 0, ErrBoundary
	}
	if n == nil {
		return 0, nil
	}
	l := size(n.left)
	if offset < l.bytes {
		return s.unitPosition(n.left, offset)
	}
	if offset >= l.bytes+n.part.bytes {
		at, e := s.unitPosition(n.right, offset-l.bytes-n.part.bytes)
		return l.units + n.part.units + at, e
	}
	data, e := s.b.bytes(n.part)
	if e != nil {
		return 0, e
	}
	return l.units + measureText(data[:offset-l.bytes]).units, nil
}

type Match struct {
	Revision uint64 `json:"revision"`
	Found    bool   `json:"found"`
	From     int64  `json:"from"`
	To       int64  `json:"to"`
	Line     int64  `json:"line"`
}

func (s *Snapshot) Find(ctx context.Context, query string, from int64, regex, matchCase, wholeWord bool) (Match, error) {
	result := Match{Revision: s.Revision}
	query, e := searchPattern(query, regex, matchCase, wholeWord)
	if e != nil {
		return result, e
	}
	start, e := s.bytePosition(s.root, from)
	if e != nil {
		return result, e
	}
	group := 0
	if from > 0 {
		// Include the preceding rune to preserve anchors and word boundaries.
		start, e = s.bytePosition(s.root, from-1)
		if e == ErrBoundary && from >= 2 {
			start, e = s.bytePosition(s.root, from-2)
		}
		if e != nil {
			return result, e
		}
		query = resumeSearchPattern(query)
		group = 2
	}
	pattern, e := regexp.Compile(query)
	if e != nil {
		return result, e
	}
	reader := &snapshotReader{ctx: ctx, b: s.b}
	reader.seek(s.root, start)
	match := pattern.FindReaderSubmatchIndex(bufio.NewReaderSize(reader, ChunkBytes))
	if reader.err != nil {
		return result, reader.err
	}
	if e = ctx.Err(); e != nil {
		return result, e
	}
	if match == nil {
		return result, nil
	}
	result.Found = true
	result.From, e = s.unitPosition(s.root, start+int64(match[group]))
	if e != nil {
		return result, e
	}
	result.To, e = s.unitPosition(s.root, start+int64(match[group+1]))
	if e != nil {
		return result, e
	}
	result.Line, e = s.b.lineAt(s.root, result.From)
	return result, e
}

func resumeSearchPattern(pattern string) string {
	return `(?s:\A..*?)(` + pattern + `)`
}

func searchPattern(query string, regex, matchCase, wholeWord bool) (string, error) {
	if query == "" || len(query) > 4096 {
		return "", errors.New("enter a search pattern of up to 4096 bytes")
	}
	if !regex {
		query = regexp.QuoteMeta(query)
	}
	if wholeWord {
		query = `\b(?:` + query + `)\b`
	}
	if !matchCase {
		query = "(?i)" + query
	}
	return query, nil
}
