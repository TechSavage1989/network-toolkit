package editorbuffer

import "strings"

// The Document Map of a document that is not all in the page draws the whole
// document from evenly spaced samples: at most MaxMapRows rows of at most
// MaxMapColumns UTF-16 units each, whatever the document's size.
const MaxMapRows = 4096
const MaxMapColumns = 512

// MapSample is the text of one map row: from Start (Column units into line
// Line) to the end of that line, at most the requested columns.
type MapSample struct {
	Start  int64  `json:"start"`
	Line   int64  `json:"line"`
	Column int64  `json:"column"`
	Text   string `json:"text"`
}
type MapSamples struct {
	Metadata
	Rows []MapSample `json:"rows"`
}

// MapSamples reads `rows` samples spread evenly over the document. By line
// (no word wrap) a sample is the start of a line. By offset (word wrap) it is
// the wrapped row of `columns` units that holds the position: rows start at
// the line's start and every `columns` units after it.
func (b *Buffer) MapSamples(rows, columns int, byOffset bool) (MapSamples, error) {
	b.lifetime.RLock()
	defer b.lifetime.RUnlock()
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return MapSamples{}, ErrClosed
	}
	if rows < 1 || rows > MaxMapRows || columns < 1 || columns > MaxMapColumns {
		return MapSamples{}, ErrBoundary
	}
	total := size(b.root)
	out := MapSamples{Metadata: b.metadataLocked(), Rows: make([]MapSample, 0, rows)}
	data := make([]byte, 0, columns*3)
	for k := 0; k < rows; k++ {
		var line, start, column int64
		var err error
		if byOffset {
			at := int64(k) * total.units / int64(rows)
			if line, err = b.lineAt(b.root, at); err == ErrBoundary && at > 0 {
				at--
				line, err = b.lineAt(b.root, at)
			}
			if err != nil {
				return MapSamples{}, err
			}
			if start, err = b.lineStart(b.root, line); err != nil {
				return MapSamples{}, err
			}
			column = (at - start) / int64(columns) * int64(columns)
			start += column
		} else {
			line = int64(k) * (total.lines + 1) / int64(rows)
			if start, err = b.lineStart(b.root, line); err != nil {
				return MapSamples{}, err
			}
		}
		// A row may begin or end inside a surrogate pair: read it without
		// that half.
		end := min(total.units, start+int64(columns))
		err = ErrBoundary
		for _, r := range [][2]int64{{start, end}, {start, end - 1}, {start + 1, end}, {start + 1, end - 1}} {
			if r[0] > r[1] {
				continue
			}
			data = data[:0]
			if err = b.readRange(b.root, r[0], r[1], &data); err != ErrBoundary {
				break
			}
		}
		if err != nil {
			return MapSamples{}, err
		}
		text := string(data)
		if i := strings.IndexByte(text, '\n'); i >= 0 {
			text = text[:i]
		}
		out.Rows = append(out.Rows, MapSample{Start: start, Line: line, Column: column, Text: strings.TrimSuffix(text, "\r")})
	}
	return out, nil
}
