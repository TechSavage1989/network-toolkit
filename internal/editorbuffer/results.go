package editorbuffer

import (
	"encoding/binary"
	"errors"
	"os"
	"sort"
	"sync"
)

// Results holds the rows of one Find All, in order, so that a page can read
// them a page at a time while the search goes on (owner request v1.21.55:
// the Search results panel never holds every row of a large result). Rows
// are kept compactly in blocks; once the blocks in memory pass a budget,
// further blocks go to a temporary file in the scratch folder, so a search
// with millions of matches needs little memory. Close removes the file.
type Results struct {
	mu       sync.Mutex
	dir      string
	blocks   []resultBlock
	open     []byte
	openRows int
	rows     int64
	hits     int64
	memory   int64
	file     *os.File
	end      int64
	closed   bool
}

type resultBlock struct {
	first int64
	count int
	data  []byte // nil when the block is in the file
	at    int64
	size  int
}

const resultBlockRows = 256

// resultMemory is how many bytes of rows one Results keeps in memory.
var resultMemory int64 = 8 << 20

// ErrResultsClosed answers a read of results that were cleared.
var ErrResultsClosed = errors.New("These search results were cleared.")

// NewResults keeps rows in memory and, past its budget, in a temporary file
// in dir (the system's temporary folder when dir is empty).
func NewResults(dir string) *Results { return &Results{dir: dir} }

// Add appends a row.
func (r *Results) Add(row Row) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrResultsClosed
	}
	r.open = appendRow(r.open, row)
	r.openRows++
	r.rows++
	r.hits += int64(len(row.Hits))
	if r.openRows >= resultBlockRows {
		return r.sealLocked()
	}
	return nil
}

func (r *Results) sealLocked() error {
	if r.openRows == 0 {
		return nil
	}
	block := resultBlock{first: r.rows - int64(r.openRows), count: r.openRows, size: len(r.open)}
	if r.memory+int64(len(r.open)) <= resultMemory {
		block.data = append([]byte(nil), r.open...)
		r.memory += int64(len(r.open))
	} else {
		if r.file == nil {
			f, err := os.CreateTemp(r.dir, "nt-editor-results-*")
			if err != nil {
				return err
			}
			r.file = f
		}
		if _, err := r.file.WriteAt(r.open, r.end); err != nil {
			return err
		}
		block.at = r.end
		r.end += int64(len(r.open))
	}
	r.blocks = append(r.blocks, block)
	r.open, r.openRows = r.open[:0], 0
	return nil
}

// Size is the number of rows and of matches added so far.
func (r *Results) Size() (int64, int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rows, r.hits
}

// Page reads up to limit rows from row start.
func (r *Results) Page(start int64, limit int) ([]Row, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, ErrResultsClosed
	}
	if start < 0 || limit < 0 {
		return nil, ErrBoundary
	}
	var out []Row
	i := sort.Search(len(r.blocks), func(i int) bool { return r.blocks[i].first+int64(r.blocks[i].count) > start })
	for ; i < len(r.blocks) && len(out) < limit; i++ {
		b := r.blocks[i]
		data := b.data
		if data == nil {
			data = make([]byte, b.size)
			if _, err := r.file.ReadAt(data, b.at); err != nil {
				return nil, err
			}
		}
		rows, err := decodeRows(data, b.count)
		if err != nil {
			return nil, err
		}
		from := max(0, start-b.first)
		for _, row := range rows[from:] {
			if len(out) == limit {
				break
			}
			out = append(out, row)
		}
	}
	if len(out) < limit && r.openRows > 0 {
		first := r.rows - int64(r.openRows)
		rows, err := decodeRows(r.open, r.openRows)
		if err != nil {
			return nil, err
		}
		for k, row := range rows {
			if first+int64(k) >= start && len(out) < limit {
				out = append(out, row)
			}
		}
	}
	return out, nil
}

// Close forgets the rows and removes the temporary file.
func (r *Results) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.closed = true
	r.blocks, r.open = nil, nil
	if r.file != nil {
		name := r.file.Name()
		r.file.Close()
		os.Remove(name)
		r.file = nil
	}
}

func appendRow(p []byte, row Row) []byte {
	p = binary.AppendUvarint(p, uint64(row.Line))
	p = binary.AppendUvarint(p, uint64(row.Start))
	p = binary.AppendUvarint(p, uint64(len(row.Text)))
	p = append(p, row.Text...)
	var flags byte
	if row.CutStart {
		flags |= 1
	}
	if row.CutEnd {
		flags |= 2
	}
	p = append(p, flags)
	p = binary.AppendUvarint(p, uint64(len(row.Hits)))
	base := row.Start
	for _, hit := range row.Hits {
		p = binary.AppendVarint(p, hit[0]-base)
		p = binary.AppendUvarint(p, uint64(hit[1]-hit[0]))
		base = hit[0]
	}
	return p
}

var errResultData = errors.New("damaged search results")

func decodeRows(p []byte, count int) ([]Row, error) {
	rows := make([]Row, 0, count)
	next := func() (uint64, error) {
		v, n := binary.Uvarint(p)
		if n <= 0 {
			return 0, errResultData
		}
		p = p[n:]
		return v, nil
	}
	for len(rows) < count {
		var row Row
		line, err := next()
		if err != nil {
			return nil, err
		}
		start, err := next()
		if err != nil {
			return nil, err
		}
		size, err := next()
		if err != nil || uint64(len(p)) < size+1 {
			return nil, errResultData
		}
		row.Line, row.Start, row.Text = int64(line), int64(start), string(p[:size])
		p = p[size:]
		row.CutStart, row.CutEnd = p[0]&1 != 0, p[0]&2 != 0
		p = p[1:]
		hits, err := next()
		if err != nil || hits > uint64(len(p)) {
			return nil, errResultData
		}
		row.Hits = make([][2]int64, 0, hits)
		base := row.Start
		for k := uint64(0); k < hits; k++ {
			delta, n := binary.Varint(p)
			if n <= 0 {
				return nil, errResultData
			}
			p = p[n:]
			width, err := next()
			if err != nil {
				return nil, err
			}
			from := base + delta
			row.Hits = append(row.Hits, [2]int64{from, from + int64(width)})
			base = from
		}
		rows = append(rows, row)
	}
	return rows, nil
}
