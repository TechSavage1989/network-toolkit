package texttools

import (
	"bufio"
	"container/heap"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"slices"
)

// runBudget is how many bytes of records a sorter holds in memory before it
// writes them, sorted, to a run file. Tests make it small.
var runBudget = 32 << 20

// mergeWidth is how many run files one merge reads at once.
var mergeWidth = 64

// sorter is an external merge sort of byte records. Records are added in
// order; each records group that fills the budget is sorted (stably) and
// written to a run file in the scratch folder; the runs are then merged, and
// records that compare equal come out in the order they were added. With a
// nil comparison the sorter only keeps runs in order (Reverse line order).
//
// Memory holds one run plus, while merging, one record of each run file.
type sorter struct {
	j      *job
	cmp    func(a, b []byte) int
	run    [][]byte
	bytes  int
	runs   []*runFile
	count  int64
	closed bool
}

type runFile struct {
	f     *os.File
	count int64
}

func newSorter(j *job, cmp func(a, b []byte) int) *sorter { return &sorter{j: j, cmp: cmp} }

func (s *sorter) add(record []byte) error {
	s.run = append(s.run, record)
	s.bytes += len(record) + 32
	s.count++
	if s.bytes >= runBudget {
		return s.spill()
	}
	return nil
}

func (s *sorter) sortRun() {
	if s.cmp != nil {
		slices.SortStableFunc(s.run, s.cmp)
	}
}

// spill writes the records held in memory to a new run file.
func (s *sorter) spill() error {
	if len(s.run) == 0 {
		return nil
	}
	if err := s.j.ctx.Err(); err != nil {
		return err
	}
	s.sortRun()
	f, err := os.CreateTemp(s.j.env.Dir, "nt-text-tool-*")
	if err != nil {
		return err
	}
	run := &runFile{f: f, count: int64(len(s.run))}
	s.runs = append(s.runs, run)
	w := bufio.NewWriterSize(f, 256<<10)
	var size [binary.MaxVarintLen64]byte
	for _, record := range s.run {
		n := binary.PutUvarint(size[:], uint64(len(record)))
		if _, err = w.Write(size[:n]); err == nil {
			_, err = w.Write(record)
		}
		if err != nil {
			return err
		}
	}
	if err = w.Flush(); err != nil {
		return err
	}
	s.run, s.bytes = s.run[:0], 0
	return nil
}

func (s *sorter) close() {
	if s.closed {
		return
	}
	s.closed = true
	for _, run := range s.runs {
		run.f.Close()
		os.Remove(run.f.Name())
	}
	s.runs = nil
}

// runReader reads the records of one run file from its start.
type runReader struct {
	r      *bufio.Reader
	record []byte
	index  int
}

func openRun(run *runFile, index int) (*runReader, error) {
	if _, err := run.f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return &runReader{r: bufio.NewReaderSize(run.f, 64<<10), index: index}, nil
}

func (r *runReader) next() (bool, error) {
	n, err := binary.ReadUvarint(r.r)
	if err == io.EOF {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	record := make([]byte, n)
	if _, err = io.ReadFull(r.r, record); err != nil {
		return false, err
	}
	r.record = record
	return true, nil
}

type runHeap struct {
	items []*runReader
	cmp   func(a, b []byte) int
}

func (h *runHeap) Len() int { return len(h.items) }
func (h *runHeap) Less(a, b int) bool {
	if c := h.cmp(h.items[a].record, h.items[b].record); c != 0 {
		return c < 0
	}
	return h.items[a].index < h.items[b].index
}
func (h *runHeap) Swap(a, b int) { h.items[a], h.items[b] = h.items[b], h.items[a] }
func (h *runHeap) Push(x any)    { h.items = append(h.items, x.(*runReader)) }
func (h *runHeap) Pop() any {
	x := h.items[len(h.items)-1]
	h.items = h.items[:len(h.items)-1]
	return x
}

// merge merges run files into fn, in order.
func (s *sorter) merge(runs []*runFile, fn func([]byte) error) error {
	h := &runHeap{cmp: s.cmp}
	for i, run := range runs {
		r, err := openRun(run, i)
		if err != nil {
			return err
		}
		ok, err := r.next()
		if err != nil {
			return err
		}
		if ok {
			h.items = append(h.items, r)
		}
	}
	heap.Init(h)
	var done int64
	for h.Len() > 0 {
		r := h.items[0]
		if err := fn(r.record); err != nil {
			return err
		}
		if done++; done%4096 == 0 {
			if err := s.j.ctx.Err(); err != nil {
				return err
			}
		}
		ok, err := r.next()
		if err != nil {
			return err
		}
		if ok {
			heap.Fix(h, 0)
		} else {
			heap.Pop(h)
		}
	}
	return nil
}

// narrow merges groups of run files until one merge can read them all.
func (s *sorter) narrow() error {
	for len(s.runs) > mergeWidth {
		var next []*runFile
		for start := 0; start < len(s.runs); start += mergeWidth {
			group := s.runs[start:min(len(s.runs), start+mergeWidth)]
			if len(group) == 1 {
				next = append(next, group[0])
				continue
			}
			f, err := os.CreateTemp(s.j.env.Dir, "nt-text-tool-*")
			if err != nil {
				return err
			}
			merged := &runFile{f: f}
			next = append(next, merged)
			w := bufio.NewWriterSize(f, 256<<10)
			var size [binary.MaxVarintLen64]byte
			err = s.merge(group, func(record []byte) error {
				merged.count++
				n := binary.PutUvarint(size[:], uint64(len(record)))
				if _, err := w.Write(size[:n]); err != nil {
					return err
				}
				_, err := w.Write(record)
				return err
			})
			if err == nil {
				err = w.Flush()
			}
			for _, run := range group {
				run.f.Close()
				os.Remove(run.f.Name())
			}
			if err != nil {
				s.runs = append(next, s.runs[start+len(group):]...)
				return err
			}
		}
		s.runs = next
	}
	return nil
}

// each calls fn for every record in sorted order.
func (s *sorter) each(fn func([]byte) error) error {
	if s.cmp == nil {
		return errors.New("this sorter keeps order only")
	}
	if len(s.runs) == 0 {
		s.sortRun()
		for i, record := range s.run {
			if err := fn(record); err != nil {
				return err
			}
			if i%4096 == 4095 {
				if err := s.j.ctx.Err(); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := s.spill(); err != nil {
		return err
	}
	if err := s.narrow(); err != nil {
		return err
	}
	return s.merge(s.runs, fn)
}

// stream returns the records in sorted order one at a time, for a reader
// that consumes them while it reads something else.
func (s *sorter) stream() (func() ([]byte, bool, error), func(), error) {
	if len(s.runs) == 0 {
		s.sortRun()
		i := 0
		return func() ([]byte, bool, error) {
			if i >= len(s.run) {
				return nil, false, nil
			}
			i++
			return s.run[i-1], true, nil
		}, func() {}, nil
	}
	if err := s.spill(); err != nil {
		return nil, nil, err
	}
	if err := s.narrow(); err != nil {
		return nil, nil, err
	}
	h := &runHeap{cmp: s.cmp}
	for i, run := range s.runs {
		r, err := openRun(run, i)
		if err != nil {
			return nil, nil, err
		}
		ok, err := r.next()
		if err != nil {
			return nil, nil, err
		}
		if ok {
			h.items = append(h.items, r)
		}
	}
	heap.Init(h)
	return func() ([]byte, bool, error) {
		if h.Len() == 0 {
			return nil, false, nil
		}
		r := h.items[0]
		record := r.record
		ok, err := r.next()
		if err != nil {
			return nil, false, err
		}
		if ok {
			heap.Fix(h, 0)
		} else {
			heap.Pop(h)
		}
		return record, true, nil
	}, func() {}, nil
}

// reversed calls fn for every record, last added first: each run in memory
// is written backwards, and the run files are read last first.
func (s *sorter) reversed(fn func([]byte) error) error {
	slices.Reverse(s.run)
	if len(s.runs) == 0 {
		for _, record := range s.run {
			if err := fn(record); err != nil {
				return err
			}
		}
		return nil
	}
	for _, record := range s.run {
		if err := fn(record); err != nil {
			return err
		}
	}
	for i := len(s.runs) - 1; i >= 0; i-- {
		// A spilled run was written in the order its records came; it is
		// read into memory one run at a time and given out backwards.
		r, err := openRun(s.runs[i], i)
		if err != nil {
			return err
		}
		records := make([][]byte, 0, s.runs[i].count)
		for {
			ok, err := r.next()
			if err != nil {
				return err
			}
			if !ok {
				break
			}
			records = append(records, r.record)
		}
		for k := len(records) - 1; k >= 0; k-- {
			if err := fn(records[k]); err != nil {
				return err
			}
		}
		if err := s.j.ctx.Err(); err != nil {
			return err
		}
	}
	return nil
}
