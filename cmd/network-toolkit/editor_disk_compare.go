package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"net/http"
	"networktoolkit/internal/editorbuffer"
	"os"
	"sync"
	"time"
)

const diskCompareRecordSize = 32
const diskComparePageSize = 128
const diskCompareMapWindow = 4096

type diskComparisonSource struct {
	UID      string  `json:"uid"`
	Lease    uint64  `json:"lease"`
	Revision uint64  `json:"revision"`
	Text     *string `json:"text"`
}

type diskComparisonRequest struct {
	ID     string               `json:"id"`
	Action string               `json:"action"`
	Left   diskComparisonSource `json:"left"`
	Right  diskComparisonSource `json:"right"`
	Start  int64                `json:"start"`
	Limit  int                  `json:"limit"`
	Side   string               `json:"side"`
	Line   int64                `json:"line"`
	End    int64                `json:"end"`
}

type diskComparisonSnapshot struct {
	uid      string
	lease    uint64
	revision uint64
	buffer   *editorbuffer.Buffer
}

type diskComparisonStatus struct {
	editorbuffer.CompareProgress
	LeftSize   int64  `json:"leftSize"`
	RightSize  int64  `json:"rightSize"`
	LeftTotal  int64  `json:"leftTotal"`
	RightTotal int64  `json:"rightTotal"`
	Count      int64  `json:"count"`
	Done       bool   `json:"done"`
	Error      string `json:"error,omitempty"`
}

type diskComparisonJob struct {
	id       string
	sources  [2]diskComparisonSnapshot
	lastSeen time.Time // protected by the editor registry mutex
	cancel   context.CancelFunc
	mu       sync.Mutex
	file     *os.File
	status   diskComparisonStatus
	closed   bool
}

func (j *diskComparisonJob) discard() {
	j.cancel()
	j.mu.Lock()
	defer j.mu.Unlock()
	if !j.closed {
		j.closed = true
		j.file.Close()
		os.Remove(j.file.Name())
	}
}

func (j *diskComparisonJob) validLocked(m *editorViewRegistry, caller string) bool {
	if !m.callerLocked(caller) {
		return false
	}
	for _, source := range j.sources {
		if source.uid == "" {
			continue
		}
		grant := m.documents[source.uid]
		if grant == nil || grant.owner != caller || grant.lease != source.lease || grant.pending != "" || grant.buffer == nil {
			return false
		}
		grant.buffer.mu.Lock()
		imported := grant.buffer.imported
		valid := imported != nil && imported.Buffer == source.buffer && imported.Buffer.Metadata().Revision == source.revision
		grant.buffer.mu.Unlock()
		if !valid {
			return false
		}
	}
	return true
}

func (m *editorViewRegistry) expireComparisonsLocked(now time.Time) {
	for caller, job := range m.diskComparisons {
		if now.Sub(job.lastSeen) > 2*time.Minute || !job.validLocked(m, caller) {
			delete(m.diskComparisons, caller)
			job.discard()
		}
	}
}

func (m *editorViewRegistry) cancelComparisonsUsing(uid string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for caller, job := range m.diskComparisons {
		if job.sources[0].uid == uid || job.sources[1].uid == uid {
			delete(m.diskComparisons, caller)
			job.discard()
		}
	}
}

func (m *editorViewRegistry) comparisonInputLocked(caller string, source diskComparisonSource, ctxCancel context.CancelFunc) (editorbuffer.ComparisonInput, diskComparisonSnapshot, func(), error) {
	var identity diskComparisonSnapshot
	closeInput := func() {}
	if source.UID == "" && source.Text != nil && source.Lease == 0 && source.Revision == 0 {
		return editorbuffer.InlineComparisonInput(*source.Text), identity, closeInput, nil
	}
	grant := m.documents[source.UID]
	if source.Text != nil || grant == nil || grant.owner != caller || grant.lease != source.Lease || grant.pending != "" || grant.buffer == nil {
		return editorbuffer.ComparisonInput{}, identity, closeInput, errEditorLease
	}
	disk := grant.buffer
	disk.mu.Lock()
	defer disk.mu.Unlock()
	if disk.closed || disk.loading || disk.reloading || disk.imported == nil {
		return editorbuffer.ComparisonInput{}, identity, closeInput, errors.New("Wait for the comparison document to finish opening.")
	}
	buffer := disk.imported.Buffer
	snapshot, err := buffer.Snapshot()
	if err != nil {
		return editorbuffer.ComparisonInput{}, identity, closeInput, err
	}
	if snapshot.Revision != source.Revision {
		snapshot.Close()
		return editorbuffer.ComparisonInput{}, identity, closeInput, editorbuffer.ErrRevision
	}
	stop := context.AfterFunc(disk.ctx, ctxCancel)
	identity = diskComparisonSnapshot{uid: source.UID, lease: source.Lease, revision: source.Revision, buffer: buffer}
	return snapshot.ComparisonInput(), identity, func() {
		stop()
		snapshot.Close()
	}, nil
}

func (m *editorViewRegistry) startDiskComparison(r *http.Request, caller string, req diskComparisonRequest) (*diskComparisonJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.callerLocked(caller) || r.Context().Err() != nil {
		return nil, errEditorLease
	}
	if existing := m.diskComparisons[caller]; existing != nil && existing.id == req.ID {
		if !existing.validLocked(m, caller) {
			return nil, errEditorLease
		}
		existing.lastSeen = m.now()
		return existing, nil
	}
	parent := m.app.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	left, leftIdentity, closeLeft, err := m.comparisonInputLocked(caller, req.Left, cancel)
	if err != nil {
		cancel()
		return nil, err
	}
	right, rightIdentity, closeRight, err := m.comparisonInputLocked(caller, req.Right, cancel)
	if err != nil {
		cancel()
		closeLeft()
		return nil, err
	}
	file, err := os.CreateTemp("", "nt-editor-compare-*")
	if err != nil {
		cancel()
		closeLeft()
		closeRight()
		return nil, errors.New("Cannot create a temporary comparison index.")
	}
	job := &diskComparisonJob{id: req.ID, sources: [2]diskComparisonSnapshot{leftIdentity, rightIdentity}, lastSeen: m.now(), cancel: cancel, file: file}
	if m.diskComparisons == nil {
		m.diskComparisons = make(map[string]*diskComparisonJob)
	}
	if previous := m.diskComparisons[caller]; previous != nil {
		previous.discard()
	}
	m.diskComparisons[caller] = job
	go func() {
		defer closeLeft()
		defer closeRight()
		job.scan(ctx, left, right)
	}()
	return job, nil
}

func (j *diskComparisonJob) scan(ctx context.Context, left, right editorbuffer.ComparisonInput) {
	status := diskComparisonStatus{
		LeftTotal: left.Lines(), RightTotal: right.Lines(), LeftSize: left.Bytes(), RightSize: right.Bytes(),
	}
	j.mu.Lock()
	j.status = status
	j.mu.Unlock()
	writer := bufio.NewWriterSize(j.file, 64<<10)
	var writeErr error
	publish := func(progress editorbuffer.CompareProgress) {
		if writeErr == nil {
			writeErr = writer.Flush()
		}
		status.CompareProgress = progress
		j.mu.Lock()
		if !j.closed && writeErr == nil {
			j.status = status
		}
		j.mu.Unlock()
	}
	err := editorbuffer.Compare(ctx, left, right, func(diff editorbuffer.Difference) error {
		if writeErr != nil {
			return writeErr
		}
		var record [diskCompareRecordSize]byte
		for i, value := range [...]int64{diff.Left, diff.Right, diff.LeftCount, diff.RightCount} {
			binary.LittleEndian.PutUint64(record[i*8:], uint64(value))
		}
		if _, err := writer.Write(record[:]); err != nil {
			return err
		}
		status.Count++
		return nil
	}, publish)
	j.mu.Lock()
	defer j.mu.Unlock()
	if !j.closed {
		j.status.Done = true
		if err != nil || writeErr != nil {
			j.status.Error = "Comparison could not finish. Choose Compare to try again."
		}
	}
}

func (j *diskComparisonJob) read(index int64) (editorbuffer.Difference, error) {
	var record [diskCompareRecordSize]byte
	if _, err := j.file.ReadAt(record[:], index*diskCompareRecordSize); err != nil {
		return editorbuffer.Difference{}, err
	}
	return editorbuffer.Difference{
		Left: int64(binary.LittleEndian.Uint64(record[0:8])), Right: int64(binary.LittleEndian.Uint64(record[8:16])),
		LeftCount: int64(binary.LittleEndian.Uint64(record[16:24])), RightCount: int64(binary.LittleEndian.Uint64(record[24:32])),
	}, nil
}

func (j *diskComparisonJob) page(start int64, limit int) ([]editorbuffer.Difference, error) {
	out := make([]editorbuffer.Difference, 0, limit)
	for n := start; n < j.status.Count && len(out) < limit; n++ {
		difference, err := j.read(n)
		if err != nil {
			return nil, err
		}
		out = append(out, difference)
	}
	return out, nil
}

func comparisonSide(diff editorbuffer.Difference, right bool) (start, count, peer, peerCount int64) {
	if right {
		return diff.Right, diff.RightCount, diff.Left, diff.LeftCount
	}
	return diff.Left, diff.LeftCount, diff.Right, diff.RightCount
}

// Read the exact changes intersecting a small window of source lines.
func (j *diskComparisonJob) mapLines(first, end int64, right bool) ([]int, error) {
	marks := make([]int, end-first)
	low, high := int64(0), j.status.Count
	for low < high {
		mid := low + (high-low)/2
		diff, err := j.read(mid)
		if err != nil {
			return nil, err
		}
		start, count, _, _ := comparisonSide(diff, right)
		if start+count <= first {
			low = mid + 1
		} else {
			high = mid
		}
	}
	for index := low; index < j.status.Count; index++ {
		diff, err := j.read(index)
		if err != nil {
			return nil, err
		}
		start, count, _, peerCount := comparisonSide(diff, right)
		if start >= end {
			break
		}
		for line := max(first, start); line < min(end, start+count); line++ {
			kind := 1
			if line-start < peerCount {
				kind = 3
			} else if right {
				kind = 2
			}
			marks[line-first] = kind
		}
	}
	return marks, nil
}

func (j *diskComparisonJob) locate(req diskComparisonRequest) (map[string]any, error) {
	right := req.Side == "right"
	processed, peerTotal := j.status.LeftLines, j.status.RightTotal
	if right {
		processed, peerTotal = j.status.RightLines, j.status.LeftTotal
	}
	low, high := int64(0), j.status.Count
	for low < high {
		mid := low + (high-low)/2
		diff, err := j.read(mid)
		if err != nil {
			return nil, err
		}
		start, _, _, _ := comparisonSide(diff, right)
		if start <= req.Line {
			low = mid + 1
		} else {
			high = mid
		}
	}
	index, peerLine := low, req.Line
	if low > 0 {
		previous, err := j.read(low - 1)
		if err != nil {
			return nil, err
		}
		start, count, peer, peerCount := comparisonSide(previous, right)
		relative := req.Line - start
		if relative < count {
			peerLine = peer + min(relative, max(0, peerCount-1))
			index--
		} else {
			peerLine = peer + peerCount + relative - count
		}
	}
	differences := make([]editorbuffer.Difference, 0, req.Limit)
	for n := index; n < j.status.Count && len(differences) < req.Limit; n++ {
		diff, err := j.read(n)
		if err != nil {
			return nil, err
		}
		start, _, _, _ := comparisonSide(diff, right)
		if start > req.End {
			break
		}
		differences = append(differences, diff)
	}
	return map[string]any{
		"aligned": req.Line < processed, "peerLine": min(max(0, peerLine), max(0, peerTotal-1)), "start": index, "differences": differences,
	}, nil
}

func (m *editorViewRegistry) compareDiskDocuments(w http.ResponseWriter, r *http.Request, caller string) {
	var req diskComparisonRequest
	if err := decodeEditorBody(w, r, editorTransferLimit+16384, &req); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !editorHexID.MatchString(req.ID) {
		jsonError(w, http.StatusBadRequest, "Invalid comparison ID.")
		return
	}
	if req.Action == "start" {
		job, err := m.startDiskComparison(r, caller, req)
		if err != nil {
			jsonError(w, http.StatusConflict, err.Error())
			return
		}
		job.mu.Lock()
		status := job.status
		job.mu.Unlock()
		jsonResponse(w, http.StatusAccepted, status)
		return
	}
	m.mu.Lock()
	job := m.diskComparisons[caller]
	if req.Action == "cancel" {
		if job != nil && job.id == req.ID {
			delete(m.diskComparisons, caller)
			job.discard()
		}
		m.mu.Unlock()
		jsonResponse(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if job == nil || job.id != req.ID {
		m.mu.Unlock()
		jsonError(w, http.StatusGone, "Comparison has closed. Choose Compare to start again.")
		return
	}
	if !job.validLocked(m, caller) {
		delete(m.diskComparisons, caller)
		job.discard()
		m.mu.Unlock()
		jsonError(w, http.StatusConflict, "The documents changed. Choose Compare to update the result.")
		return
	}
	job.lastSeen = m.now()
	m.mu.Unlock()
	req.Limit = max(1, min(req.Limit, diskComparePageSize))
	job.mu.Lock()
	if job.closed {
		job.mu.Unlock()
		jsonError(w, http.StatusGone, "Comparison has closed. Choose Compare to start again.")
		return
	}
	var value any
	var err error
	switch req.Action {
	case "status":
		value = job.status
	case "map":
		if req.Start < 0 || req.End <= req.Start || req.End-req.Start > diskCompareMapWindow {
			err = editorbuffer.ErrBoundary
			break
		}
		var left, right []int
		left, err = job.mapLines(req.Start, req.End, false)
		if err == nil {
			right, err = job.mapLines(req.Start, req.End, true)
		}
		value = map[string]any{"left": left, "right": right}
	case "page":
		if req.Start < 0 {
			err = editorbuffer.ErrBoundary
			break
		}
		var differences []editorbuffer.Difference
		differences, err = job.page(req.Start, req.Limit)
		value = map[string]any{"start": req.Start, "differences": differences}
	case "locate":
		if (req.Side != "left" && req.Side != "right") || req.Line < 0 || req.End < req.Line {
			err = editorbuffer.ErrBoundary
			break
		}
		value, err = job.locate(req)
	default:
		err = errors.New("Unsupported comparison action.")
	}
	job.mu.Unlock()
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, value)
}
