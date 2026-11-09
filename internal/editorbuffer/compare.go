package editorbuffer

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"hash"
	"io"
	"strings"
)

// ComparisonInput references either a stable disk snapshot or an existing
// inline document. The caller retains the snapshot until Compare returns.
type ComparisonInput struct {
	snapshot *Snapshot
	text     string
}

func (s *Snapshot) ComparisonInput() ComparisonInput    { return ComparisonInput{snapshot: s} }
func InlineComparisonInput(text string) ComparisonInput { return ComparisonInput{text: text} }

func (in ComparisonInput) Lines() int64 {
	if in.snapshot != nil {
		return size(in.snapshot.root).lines + 1
	}
	return int64(strings.Count(in.text, "\n")) + 1
}

func (in ComparisonInput) Bytes() int64 {
	if in.snapshot != nil {
		return size(in.snapshot.root).bytes
	}
	return int64(len(in.text))
}

func (in ComparisonInput) reader(ctx context.Context, reverse bool) io.Reader {
	if in.snapshot == nil {
		if reverse {
			return &reverseTextReader{text: in.text}
		}
		return strings.NewReader(in.text)
	}
	r := &comparePieceReader{ctx: ctx, b: in.snapshot.b, reverse: reverse}
	r.push(in.snapshot.root)
	return r
}

type reverseTextReader struct{ text string }

func (r *reverseTextReader) Read(p []byte) (int, error) {
	n := min(len(p), len(r.text))
	if n == 0 {
		return 0, io.EOF
	}
	for i := range n {
		p[i] = r.text[len(r.text)-1-i]
	}
	r.text = r.text[:len(r.text)-n]
	return n, nil
}

// Both input streams reuse a piece buffer. Retained memory and allocation
// traffic stay bounded even while scanning multi-gigabyte snapshots.
type comparePieceReader struct {
	ctx     context.Context
	b       *Buffer
	stack   []*node
	chunk   []byte
	scratch []byte
	reverse bool
}

func (r *comparePieceReader) push(n *node) {
	for n != nil {
		r.stack = append(r.stack, n)
		if r.reverse {
			n = n.right
		} else {
			n = n.left
		}
	}
}

func (r *comparePieceReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if len(r.chunk) == 0 {
		if len(r.stack) == 0 {
			return 0, io.EOF
		}
		n := r.stack[len(r.stack)-1]
		r.stack = r.stack[:len(r.stack)-1]
		if r.reverse {
			r.push(n.left)
		} else {
			r.push(n.right)
		}
		if int64(cap(r.scratch)) < n.part.bytes {
			r.scratch = make([]byte, max(ChunkBytes, n.part.bytes))
		}
		r.chunk = r.scratch[:n.part.bytes]
		if _, err := r.b.file.ReadAt(r.chunk, n.part.offset); err != nil {
			return 0, err
		}
	}
	n := min(len(p), len(r.chunk))
	if !r.reverse {
		copy(p, r.chunk[:n])
		r.chunk = r.chunk[n:]
		return n, nil
	}
	for i := range n {
		p[i] = r.chunk[len(r.chunk)-1-i]
	}
	r.chunk = r.chunk[:len(r.chunk)-n]
	return n, nil
}

// Difference describes a contiguous changed region in zero-based lines.
// A zero count represents insertion/deletion at that side's line boundary.
type Difference struct {
	Left       int64 `json:"left"`
	Right      int64 `json:"right"`
	LeftCount  int64 `json:"leftCount"`
	RightCount int64 `json:"rightCount"`
}

type CompareProgress struct {
	LeftLines  int64 `json:"leftLines"`
	RightLines int64 `json:"rightLines"`
	LeftBytes  int64 `json:"leftBytes"`
	RightBytes int64 `json:"rightBytes"`
}

type CompareOptions struct {
	// Spaces can be "changed" to ignore indentation and runs of spaces and
	// tabs, or "all" to ignore spaces and tabs anywhere in a line.
	Spaces string
}

const compareLookahead = 128

type lineSignature [sha256.Size]byte

type lineRole uint8

const (
	ordinaryLine lineRole = iota
	editHeader
	blockBoundary
	uuidField
)

type lineRecord struct {
	signature lineSignature
	role      lineRole
	uuid      [36]byte
}

func (r *lineRecord) readRole(text []byte) {
	text = bytes.Trim(text, " 	")
	switch {
	case bytes.HasPrefix(text, []byte("edit ")):
		r.role = editHeader
	case bytes.Equal(text, []byte("next")), bytes.Equal(text, []byte("end")):
		r.role = blockBoundary
	case len(text) == 45 && bytes.HasPrefix(text, []byte("set uuid ")):
		uuid := text[9:]
		for i, c := range uuid {
			if i == 8 || i == 13 || i == 18 || i == 23 {
				if c != '-' {
					return
				}
			} else if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
				return
			}
		}
		r.role = uuidField
		copy(r.uuid[:], uuid)
	}
}

type lineSignatures struct {
	ctx         context.Context
	reader      *bufio.Reader
	longLine    hash.Hash
	records     [compareLookahead]lineRecord
	reverse     bool
	first       int
	count       int
	line        int64
	end         int64
	bytesRead   int64
	notifyChunk func()
	options     CompareOptions
}

func newLineSignatures(ctx context.Context, in ComparisonInput, reverse bool, lines int64, options CompareOptions) *lineSignatures {
	return &lineSignatures{
		ctx: ctx, reader: bufio.NewReaderSize(in.reader(ctx, reverse), ChunkBytes), end: lines, options: options, reverse: reverse,
	}
}

func (s *lineSignatures) next() (lineRecord, error) {
	var record lineRecord
	seen, pendingSpace, chunked := false, false, false
	for {
		if err := s.ctx.Err(); err != nil {
			return record, err
		}
		part, err := s.reader.ReadSlice('\n')
		s.bytesRead += int64(len(part))
		if err == nil {
			part = part[:len(part)-1]
		}
		if !s.reverse && !chunked && (err == nil || err == io.EOF) {
			record.readRole(part)
		}
		if s.options.Spaces != "" {
			// Normalize in place in either read direction. Retain space state
			// across chunks so a single long line stays bounded.
			n := 0
			for _, b := range part {
				if b == ' ' || b == '\t' {
					if s.options.Spaces == "changed" {
						pendingSpace = seen
					}
					continue
				}
				if pendingSpace {
					if n == 0 {
						// Emit a carried separator without growing this chunk.
						if s.longLine == nil {
							s.longLine = sha256.New()
						}
						s.longLine.Write([]byte{' '})
					} else {
						part[n] = ' '
						n++
					}
					pendingSpace = false
				}
				part[n] = b
				n++
				seen = true
			}
			part = part[:n]
		}
		switch err {
		case nil:
		case io.EOF:
			// The line count includes the empty line after a final newline.
		case bufio.ErrBufferFull:
			chunked = true
			if s.notifyChunk != nil {
				s.notifyChunk()
			}
			if s.longLine == nil {
				s.longLine = sha256.New()
			}
			s.longLine.Write(part)
			continue
		default:
			return record, err
		}
		if s.longLine == nil {
			record.signature = sha256.Sum256(part)
			return record, nil
		}
		s.longLine.Write(part)
		s.longLine.Sum(record.signature[:0])
		s.longLine.Reset()
		return record, nil
	}
}

func (s *lineSignatures) fill(n int) error {
	for s.count < n && s.line+int64(s.count) < s.end {
		record, err := s.next()
		if err != nil {
			return err
		}
		s.records[(s.first+s.count)%len(s.records)] = record
		s.count++
	}
	return nil
}

func (s *lineSignatures) at(i int) lineSignature {
	return s.record(i).signature
}

func (s *lineSignatures) record(i int) lineRecord {
	return s.records[(s.first+i)%len(s.records)]
}

func (s *lineSignatures) consume(n int64) {
	s.line += n
	s.count -= int(n)
	s.first = (s.first + int(n)) % len(s.records)
}

func (s *lineSignatures) keepSuffix(n int64) {
	s.end -= n
	s.count = min(s.count, int(s.end-s.line))
}

// blockUUID looks only at the first few children of the identified edit block.
func blockUUID(lines *lineSignatures, start int) ([36]byte, bool) {
	var value [36]byte
	found := false
	for i := start + 1; i < min(start+9, lines.count); i++ {
		line := lines.record(i)
		if line.role == editHeader || line.role == blockBoundary {
			break
		}
		if line.role == uuidField {
			if found {
				return value, false
			}
			value, found = line.uuid, true
		}
	}
	return value, found
}

// uniqueHeadAnchor recognizes a block after a local insertion or deletion.
// A unique header with matching context or policy UUID is stronger evidence
// than a closer run of repeated lines inside an unrelated block.
func uniqueHeadAnchor(source, other *lineSignatures) (int64, bool) {
	const run = 3
	if source.count < 2 || other.count < 2 {
		return 0, false
	}
	head := source.at(0)
	match := -1
	for i := 1; i < other.count; i++ {
		if other.at(i) == head {
			if match >= 0 {
				return 0, false
			}
			match = i
		}
	}
	if match < 0 {
		return 0, false
	}
	exactRun := source.count >= run && match+run <= other.count &&
		source.at(1) == other.at(match+1) && source.at(2) == other.at(match+2)
	if !exactRun {
		if source.record(0).role != editHeader || other.record(match).role != editHeader {
			return 0, false
		}
		leftUUID, leftOK := blockUUID(source, 0)
		rightUUID, rightOK := blockUUID(other, match)
		if !leftOK || !rightOK || leftUUID != rightUUID {
			return 0, false
		}
	}
	for i := 1; i < source.count; i++ {
		if source.at(i) == head {
			return 0, false
		}
	}
	return int64(match), true
}

// nearestAnchor preserves a clear inserted or deleted block, then finds a
// local resynchronization with fewest skipped lines.
func nearestAnchor(left, right *lineSignatures, positions map[lineSignature]int) (int64, int64, bool) {
	l, deleted := uniqueHeadAnchor(right, left)
	r, inserted := uniqueHeadAnchor(left, right)
	if inserted != deleted {
		return l, r, true
	}
	// Keep a one-line replacement paired when neighboring repeated lines
	// otherwise make a spurious insertion plus trailing deletion look cheaper.
	if left.count > 1 && right.count > 1 && left.end-left.line == right.end-right.line && left.at(1) == right.at(1) {
		return 1, 1, true
	}
	clear(positions)
	for j := 0; j < right.count; j++ {
		signature := right.at(j)
		if _, ok := positions[signature]; !ok {
			positions[signature] = j
		}
	}
	bestLeft, bestRight, bestCost, bestRun := 0, 0, left.count+right.count, 0
	for i := 0; i < left.count && i <= bestCost; i++ {
		j, ok := positions[left.at(i)]
		if !ok || i+j > bestCost {
			continue
		}
		run := 1
		for run < 3 && i+run < left.count && j+run < right.count && left.at(i+run) == right.at(j+run) {
			run++
		}
		if i+j < bestCost || run > bestRun {
			bestLeft, bestRight, bestCost, bestRun = i, j, i+j, run
		}
	}
	return int64(bestLeft), int64(bestRight), bestRun > 0
}

func commonSuffix(ctx context.Context, left, right ComparisonInput, maxLines int64, progress func(int64, int64), options CompareOptions) (int64, int64, int64, error) {
	a := newLineSignatures(ctx, left, true, maxLines, options)
	b := newLineSignatures(ctx, right, true, maxLines, options)
	report := func() { progress(a.bytesRead, b.bytesRead) }
	a.notifyChunk, b.notifyChunk = report, report
	for a.line < maxLines {
		l, err := a.next()
		if err != nil {
			return 0, 0, 0, err
		}
		r, err := b.next()
		if err != nil {
			return 0, 0, 0, err
		}
		if l.signature != r.signature {
			break
		}
		a.line++
		if a.line%16384 == 0 {
			report()
		}
	}
	return a.line, a.bytesRead, b.bytesRead, nil
}

// Compare scans stable inputs with bounded memory. It finds equal lines and
// local anchors, retaining a common suffix across large inserted/deleted
// regions. Distant reordered regions may be grouped into one change. The
// result is a useful line comparison without a quadratic full-file matrix.
func Compare(ctx context.Context, left, right ComparisonInput, emit func(Difference) error, progress func(CompareProgress)) error {
	return CompareWithOptions(ctx, left, right, emit, progress, CompareOptions{})
}

func CompareWithOptions(ctx context.Context, left, right ComparisonInput, emit func(Difference) error, progress func(CompareProgress), options CompareOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	a := newLineSignatures(ctx, left, false, left.Lines(), options)
	b := newLineSignatures(ctx, right, false, right.Lines(), options)
	var current Difference
	var hasChange, searchedSuffix bool
	var suffix int64
	positions := make(map[lineSignature]int, compareLookahead)
	flush := func() error {
		if !hasChange {
			return nil
		}
		hasChange = false
		return emit(current)
	}
	change := func(l, r int64) {
		if !hasChange {
			current = Difference{Left: a.line, Right: b.line}
			hasChange = true
		}
		current.LeftCount += l
		current.RightCount += r
	}
	var lastReportLines, lastReportBytes, leftReverse, rightReverse int64
	report := func(lr, rr int64) {
		if progress == nil {
			return
		}
		lines := a.line + b.line
		bytes := a.bytesRead + lr + b.bytesRead + rr
		if lines-lastReportLines < 16384 && bytes-lastReportBytes < 4<<20 {
			return
		}
		lastReportLines, lastReportBytes = lines, bytes
		progress(CompareProgress{
			LeftLines: a.line, RightLines: b.line,
			LeftBytes: min(left.Bytes(), a.bytesRead+lr), RightBytes: min(right.Bytes(), b.bytesRead+rr),
		})
	}
	notify := func() { report(leftReverse, rightReverse) }
	a.notifyChunk, b.notifyChunk = notify, notify
	for a.line < a.end && b.line < b.end {
		notify()
		if err := a.fill(1); err != nil {
			return err
		}
		if err := b.fill(1); err != nil {
			return err
		}
		if a.at(0) == b.at(0) {
			if err := flush(); err != nil {
				return err
			}
			a.consume(1)
			b.consume(1)
			continue
		}
		if err := a.fill(compareLookahead); err != nil {
			return err
		}
		if err := b.fill(compareLookahead); err != nil {
			return err
		}
		l, r, found := nearestAnchor(a, b, positions)
		if !found && !searchedSuffix {
			searchedSuffix = true
			var err error
			suffix, leftReverse, rightReverse, err = commonSuffix(ctx, left, right, min(a.end-a.line, b.end-b.line), report, options)
			if err != nil {
				return err
			}
			a.keepSuffix(suffix)
			b.keepSuffix(suffix)
			if a.line == a.end || b.line == b.end {
				break
			}
			l, r, found = nearestAnchor(a, b, positions)
		}
		if !found {
			// Compare the next blocks without retaining unmatched rows.
			l, r = int64(min(a.count, b.count)), int64(min(a.count, b.count))
		}
		change(l, r)
		a.consume(l)
		b.consume(r)
	}
	if a.line < a.end || b.line < b.end {
		change(a.end-a.line, b.end-b.line)
	}
	if err := flush(); err != nil {
		return err
	}
	if progress != nil {
		progress(CompareProgress{
			LeftLines: a.end + suffix, RightLines: b.end + suffix,
			LeftBytes: left.Bytes(), RightBytes: right.Bytes(),
		})
	}
	return ctx.Err()
}
