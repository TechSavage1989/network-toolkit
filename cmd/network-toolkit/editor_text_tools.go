package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sync"
	"sync/atomic"

	"networktoolkit/internal/editorbuffer"
	"networktoolkit/internal/texttools"
)

// Text tools of the Text Editor (Tools menu, owner request v1.21.53). The
// tools themselves are internal/texttools, one implementation for every
// document:
//   - page-held text goes to POST /api/editor-views/text-tools, which runs
//     the tool over the text it was sent and answers with the result; the
//     page applies it with its ordinary edit (one undo step);
//   - a disk-backed document runs the tool in the engine over a snapshot of
//     its buffer ("tool-start" on the document's buffer route), in the
//     background, with progress ("tool-status") and cancellation
//     ("tool-cancel"); the result replaces the range as one edit
//     (Snapshot.Rewrite, committed like Replace All);
//   - the readable copy of a SonicWall export is a new untitled document
//     whose buffer the engine fills from the source document's snapshot
//     (open mode "derive"): no text passes through the page, and the export
//     itself is never changed.
// A file that is not text refuses every tool (the buffer route refuses the
// actions, and Rewrite refuses a binary buffer).

// editorToolJob is a text tool running over a disk-backed document.
type editorToolJob struct {
	id       string
	cancel   context.CancelFunc
	progress atomic.Uint64 // float64 bits, 0 to 1
	mu       sync.Mutex
	done     bool
	result   map[string]any
	err      string
}

func (j *editorToolJob) setProgress(p float64) { j.progress.Store(math.Float64bits(p)) }

func (j *editorToolJob) finish(result map[string]any, err string) {
	j.mu.Lock()
	j.done, j.result, j.err = true, result, err
	j.mu.Unlock()
}

func (j *editorToolJob) state() map[string]any {
	j.mu.Lock()
	defer j.mu.Unlock()
	switch {
	case !j.done:
		return map[string]any{"state": "running", "progress": math.Float64frombits(j.progress.Load())}
	case j.err != "":
		return map[string]any{"state": "failed", "error": j.err}
	}
	state := map[string]any{"state": "done"}
	for k, v := range j.result {
		state[k] = v
	}
	return state
}

func newJobID() string {
	var b [12]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// toolMessage turns a tool's error into the message the page shows, naming
// the line and column of a problem in the document.
func toolMessage(err error, position func(int64) (int64, int64, bool), from int64) string {
	var at *texttools.PositionError
	if errors.Is(err, context.Canceled) {
		return "The text tool was cancelled. Nothing was changed."
	}
	if errors.Is(err, editorbuffer.ErrRevision) {
		return "The document changed while the tool ran. Nothing was changed; run the tool again."
	}
	if errors.As(err, &at) && position != nil {
		if line, column, ok := position(from + at.Offset); ok {
			return fmt.Sprintf("%s (line %d, column %d) Nothing was changed.", at.Message, line+1, column+1)
		}
	}
	return err.Error() + " Nothing was changed."
}

// registerStop lets a reload or close of d stop work that holds its buffer
// open (the same list Replace All uses).
func (d *editorDiskDocument) registerStop(ctx context.Context) (*replaceAllContext, func()) {
	stop := &replaceAllContext{Context: ctx}
	d.mu.Lock()
	if d.replacing == nil {
		d.replacing = map[*replaceAllContext]struct{}{}
	}
	d.replacing[stop] = struct{}{}
	d.mu.Unlock()
	return stop, func() {
		d.mu.Lock()
		delete(d.replacing, stop)
		d.mu.Unlock()
	}
}

// startTool starts a text tool over the document's selection, or the whole
// document when nothing is selected. Tools that work on lines widen the
// selection to whole lines first.
func (d *editorDiskDocument) startTool(v *editorbuffer.Imported, req editorDiskRequest) (any, error) {
	tool, ok := texttools.Lookup(req.Tool)
	if !ok {
		return nil, texttools.ErrUnknownTool
	}
	if req.Tool == "sonicwall-readable" {
		return nil, errors.New("The readable copy of a SonicWall export opens in a new tab.")
	}
	if v.Binary {
		return nil, editorbuffer.ErrBinary
	}
	d.mu.Lock()
	if d.tool != nil && !d.tool.isDone() {
		d.mu.Unlock()
		return nil, errors.New("Another text tool is still running on this document.")
	}
	ctx, cancel := context.WithCancel(d.ctx)
	job := &editorToolJob{id: newJobID(), cancel: cancel}
	d.tool = job
	d.mu.Unlock()
	// Registered before the start is answered: a reload or close that comes
	// right after it stops the tool.
	stop, unregister := d.registerStop(ctx)
	go func() {
		defer unregister()
		d.runTool(stop, job, tool, v.Buffer, req)
	}()
	return map[string]any{"job": job.id}, nil
}

func (j *editorToolJob) isDone() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.done
}

// runTool runs a tool started by startTool; ctx is already registered to be
// stopped by a reload or close of the document.
func (d *editorDiskDocument) runTool(ctx context.Context, job *editorToolJob, tool *texttools.Tool, b *editorbuffer.Buffer, req editorDiskRequest) {
	defer job.cancel()
	position := func(at int64) (int64, int64, bool) {
		w, err := b.WindowNear(at, 1)
		return w.Line, w.Column, err == nil
	}
	from, to := req.From, req.To
	fail := func(err error) { job.finish(nil, toolMessage(err, position, from)) }
	s, err := b.Snapshot()
	if err != nil {
		fail(err)
		return
	}
	if s.Revision != req.Revision {
		s.Close()
		fail(editorbuffer.ErrRevision)
		return
	}
	scope := "selection"
	if from == to {
		from, to, scope = 0, s.Units(), "document"
	} else if tool.Lines {
		if from, to, err = s.LineRange(from, to); err != nil {
			s.Close()
			fail(err)
			return
		}
	}
	var count int64
	staged, err := s.Rewrite(ctx, from, to, tool.Rewrite, func(src *editorbuffer.RangeSource, out editorbuffer.Edits) (bool, error) {
		result, err := texttools.Run(ctx, tool.ID, req.Options, src, out, texttools.Env{Dir: b.ScratchDir(), Progress: job.setProgress})
		count = result.Count
		return result.Changed, err
	})
	// The snapshot holds the buffer open: it is released before this job
	// waits for the document's edit lock (see replaceDiskMatches).
	s.Close()
	if err != nil {
		fail(err)
		return
	}
	defer staged.Abort()
	d.editMu.Lock()
	if d.bulk != nil {
		d.editMu.Unlock()
		fail(errors.New("finish or cancel the pending bulk edit"))
		return
	}
	if err = ctx.Err(); err != nil {
		d.editMu.Unlock()
		fail(err)
		return
	}
	meta, err := staged.Commit()
	d.editMu.Unlock()
	if err != nil {
		fail(err)
		return
	}
	job.finish(map[string]any{"metadata": meta, "changed": staged.Count > 0, "count": count, "from": from, "to": from + staged.Units, "scope": scope}, "")
}

// toolStatus answers the page's poll of a running tool.
func (d *editorDiskDocument) toolStatus(req editorDiskRequest, cancel bool) (any, error) {
	d.mu.Lock()
	job := d.tool
	d.mu.Unlock()
	if job == nil || job.id != req.Job {
		return nil, errors.New("This text tool is no longer running.")
	}
	if cancel {
		job.cancel()
		return map[string]any{"ok": true}, nil
	}
	return job.state(), nil
}

// pageToolOutputAllowance is part of the bound on the result of a tool over
// page-held text (handleTextTools).
const pageToolOutputAllowance = 32 << 20

// detectPrefix is how much of a document's start is read to recognise it.
const detectPrefix = 4096

// detectTool tells the page whether the start of a document looks like a
// SonicWall settings export; only its first few kilobytes are read.
func detectTool(text string) map[string]any {
	return map[string]any{"sonicwall": texttools.LooksLikeSonicWall([]byte(text))}
}

// replaceText answers a single Replace of a selected match: with a regular
// expression, the Replace text expanded with that match's groups.
func replaceText(ctx context.Context, b *editorbuffer.Buffer, req editorDiskRequest) (any, error) {
	s, err := b.Snapshot()
	if err != nil {
		return nil, err
	}
	defer s.Close()
	if s.Revision != req.Revision {
		return nil, editorbuffer.ErrRevision
	}
	text, found, err := s.ReplaceText(ctx, req.Query, req.Replace, req.From, req.To, req.Regex, req.MatchCase, req.WholeWord)
	if err != nil {
		return nil, err
	}
	return map[string]any{"found": found, "text": text}, nil
}

// editorDeriveRequest names the document a derived document is made from.
type editorDeriveRequest struct {
	UID      string `json:"uid"`
	Lease    uint64 `json:"lease"`
	Tool     string `json:"tool"`
	Revision uint64 `json:"revision"`
}

// deriveDiskDocument fills a new document with a tool's result over the
// whole of another document. The source is read through a snapshot and
// never changed; closing or reloading it stops the work: ctx is registered
// with the source (registerStop) before the open is answered.
func (m *editorViewRegistry) deriveDiskDocument(ctx context.Context, d *editorDiskDocument, source *editorDiskDocument, req editorDeriveRequest) (*editorbuffer.Imported, error) {
	tool, ok := texttools.Lookup(req.Tool)
	if !ok || req.Tool != "sonicwall-readable" {
		return nil, texttools.ErrUnknownTool
	}
	source.mu.Lock()
	v := source.imported
	source.mu.Unlock()
	if v == nil {
		return nil, errors.New("The export is still opening. Try again when it has opened.")
	}
	if v.Binary {
		return nil, editorbuffer.ErrBinary
	}
	s, err := v.Buffer.Snapshot()
	if err != nil {
		return nil, err
	}
	if s.Revision != req.Revision {
		s.Close()
		return nil, editorbuffer.ErrRevision
	}
	src, err := s.Source(ctx, 0, s.Units())
	if err != nil {
		s.Close()
		return nil, err
	}
	var count int64
	result, err := editorbuffer.NewText(ctx, "", func(out editorbuffer.Edits) error {
		r, err := texttools.Run(ctx, tool.ID, texttools.Options{}, src, out, texttools.Env{Dir: v.Buffer.ScratchDir(), Progress: func(p float64) {
			d.mu.Lock()
			d.progress = int64(p * float64(src.Size()))
			d.mu.Unlock()
		}})
		count = r.Count
		return err
	})
	// The snapshot holds the source open: it is released before the message
	// reads the source again (WindowNear claims the same lock, and a close
	// of the source may be waiting in between).
	s.Close()
	if err != nil {
		position := func(at int64) (int64, int64, bool) {
			w, e := v.Buffer.WindowNear(at, 1)
			return w.Line, w.Column, e == nil
		}
		return nil, errors.New(toolMessage(err, position, 0))
	}
	d.mu.Lock()
	d.derived = count
	d.mu.Unlock()
	return result, nil
}

// handleTextTools runs a text tool over page-held text.
func (m *editorViewRegistry) handleTextTools(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action  string            `json:"action"`
		Tool    string            `json:"tool"`
		Text    string            `json:"text"`
		Options texttools.Options `json:"options"`
	}
	if err := decodeEditorBody(w, r, editorTransferLimit+16384, &req); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	if req.Action == "detect" {
		text := req.Text
		if len(text) > detectPrefix*4 {
			text = text[:detectPrefix*4]
		}
		jsonResponse(w, 200, detectTool(text))
		return
	}
	if req.Action != "run" {
		jsonError(w, 400, "Unsupported text tool request.")
		return
	}
	out := &texttools.Output{In: []byte(req.Text)}
	// The result is held here and sent to the page, so it is bounded: four
	// times the text sent plus the page's own document size (32 MiB,
	// EDITOR_DOCUMENT_BYTES in file-editor.js). Ordinary tools stay far below;
	// a crafted input that would blow up stops instead of filling memory.
	result, err := texttools.Run(r.Context(), req.Tool, req.Options, texttools.BytesSource(out.In), out, texttools.Env{MaxOutput: 4*int64(len(out.In)) + pageToolOutputAllowance})
	if err == nil && result.Changed {
		err = out.Done()
	}
	if err != nil {
		answer := map[string]any{"ok": false, "error": err.Error()}
		var at *texttools.PositionError
		if errors.As(err, &at) {
			answer["offset"] = at.Offset
		}
		jsonResponse(w, 200, answer)
		return
	}
	text := req.Text
	if result.Changed {
		text = string(out.Out)
	}
	jsonResponse(w, 200, map[string]any{"ok": true, "changed": result.Changed, "count": result.Count, "text": text})
}
