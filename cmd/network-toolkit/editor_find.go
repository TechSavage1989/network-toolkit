package main

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"networktoolkit/internal/editorbuffer"
)

// The Find window of the Text Editor (Find, Replace and Mark, owner request
// v1.21.55). Every search runs here, with internal/editorbuffer's one
// search code:
//   - a disk-backed document is searched on its buffer route: "find" (Find
//     Next, with a range, wrap-around and direction), "find-matches" (the
//     marks of the part that is shown), "find-start" (Count, Find All, Mark
//     All, Copy Marked Text and Replace All, in the background with progress
//     and Cancel; a reload or close of the document stops them);
//   - text held by a page is sent to POST /api/editor-views/find with the
//     same actions ("next", "matches", "start", and "replace" and
//     "replace-text", whose result the page applies as one edit);
//   - the jobs of both are read through that route by their id ("status",
//     "page" for the rows of a Find All, "cancel", "release"), only by the
//     window that started them;
//   - the search history ("history", "remember") lives in this engine's
//     memory only, shared by its editor windows: search text can hold
//     passwords and is never written to disk.
// A file that is not text is searched, counted, listed and marked; nothing
// replaces anything in it (the buffer route and editorbuffer refuse it).

const (
	// editorFindJobs is how many jobs the engine keeps; the oldest goes
	// first (a running one is cancelled).
	editorFindJobs = 96
	// editorFindHistory is the length of each search history list.
	editorFindHistory = 20
	// editorBookmarkLimit is the most lines one Mark All bookmarks.
	editorBookmarkLimit = 100000
	// editorFindPageRows is the most rows of a Find All read at once.
	editorFindPageRows = 500
	// editorFindMatches is the most marks of a shown page of a disk-backed
	// document; editorFindPageMatches of a page-held document, which the
	// page marks whole.
	editorFindMatches     = 20000
	editorFindPageMatches = 250000
)

type editorFind struct {
	mu      sync.Mutex
	history map[string][]string
	jobs    map[string]*editorFindJob
	order   []string
}

// editorFindJob is a Count, Find All, Mark All, Copy Marked Text or Replace
// All running in the engine.
type editorFindJob struct {
	id, owner, kind string
	revision        uint64
	cancel          context.CancelFunc
	progress        atomic.Uint64
	results         *editorbuffer.Results
	mu              sync.Mutex
	done            bool
	err             string
	answer          map[string]any
}

func (j *editorFindJob) setProgress(p float64) { j.progress.Store(math.Float64bits(p)) }

func (j *editorFindJob) finish(answer map[string]any, err error) {
	j.mu.Lock()
	j.done, j.answer = true, answer
	if err != nil {
		j.err = findMessage(err)
	}
	j.mu.Unlock()
}

func findMessage(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "The search was cancelled."
	case errors.Is(err, editorbuffer.ErrRevision):
		return "The document changed while the search ran. Nothing was changed; search again."
	}
	text := err.Error()
	if strings.HasPrefix(text, "error parsing regexp: ") {
		return "Invalid regular expression: " + strings.TrimPrefix(text, "error parsing regexp: ")
	}
	if len(text) > 0 {
		text = strings.ToUpper(text[:1]) + text[1:]
	}
	return text
}

func (j *editorFindJob) state() map[string]any {
	j.mu.Lock()
	defer j.mu.Unlock()
	state := map[string]any{"kind": j.kind, "revision": j.revision}
	switch {
	case !j.done:
		state["state"] = "running"
		state["progress"] = math.Float64frombits(j.progress.Load())
	case j.err != "":
		state["state"] = "failed"
		state["error"] = j.err
	default:
		state["state"] = "done"
		for k, v := range j.answer {
			state[k] = v
		}
	}
	if j.results != nil {
		state["rows"], state["hits"] = j.results.Size()
	}
	return state
}

func (f *editorFind) add(job *editorFindJob) {
	f.mu.Lock()
	if f.jobs == nil {
		f.jobs = map[string]*editorFindJob{}
	}
	var gone []*editorFindJob
	for len(f.order) >= editorFindJobs {
		gone = append(gone, f.jobs[f.order[0]])
		delete(f.jobs, f.order[0])
		f.order = f.order[1:]
	}
	f.jobs[job.id] = job
	f.order = append(f.order, job.id)
	f.mu.Unlock()
	for _, old := range gone {
		old.stop()
	}
}

func (j *editorFindJob) stop() {
	j.cancel()
	if j.results != nil {
		j.results.Close()
	}
}

func (f *editorFind) job(owner, id string) (*editorFindJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	job := f.jobs[id]
	if job == nil || job.owner != owner {
		return nil, errors.New("These search results are no longer available. Search again.")
	}
	return job, nil
}

func (f *editorFind) release(owner, id string) {
	f.mu.Lock()
	job := f.jobs[id]
	if job == nil || job.owner != owner {
		f.mu.Unlock()
		return
	}
	delete(f.jobs, id)
	for k, other := range f.order {
		if other == id {
			f.order = append(f.order[:k], f.order[k+1:]...)
			break
		}
	}
	f.mu.Unlock()
	job.stop()
}

// dropOwner stops and forgets every job of a window that went away; with
// "*" every job (the engine stops).
func (f *editorFind) dropOwner(owner string) {
	f.mu.Lock()
	var gone []*editorFindJob
	kept := f.order[:0]
	for _, id := range f.order {
		if job := f.jobs[id]; owner == "*" || job.owner == owner {
			gone = append(gone, job)
			delete(f.jobs, id)
		} else {
			kept = append(kept, id)
		}
	}
	f.order = kept
	f.mu.Unlock()
	for _, job := range gone {
		job.stop()
	}
}

func (f *editorFind) remember(list, value string) error {
	if list != "find" && list != "replace" {
		return errors.New("Unknown search history.")
	}
	if value == "" || len(value) > 4096 || !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
		return errors.New("Invalid search history entry.")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.history == nil {
		f.history = map[string][]string{}
	}
	entries := []string{value}
	for _, old := range f.history[list] {
		if old != value && len(entries) < editorFindHistory {
			entries = append(entries, old)
		}
	}
	f.history[list] = entries
	return nil
}

func (f *editorFind) recall() map[string][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return map[string][]string{"find": append([]string{}, f.history["find"]...), "replace": append([]string{}, f.history["replace"]...)}
}

// within is the searched range of a request: [from, to) with to = -1 for
// the end of the document; nil is the whole document.
func findRange(within *[2]int64) (int64, int64) {
	if within == nil {
		return 0, -1
	}
	return within[0], within[1]
}

// runFind runs a read-only job over a snapshot, which it closes.
func runFind(ctx context.Context, job *editorFindJob, s *editorbuffer.Snapshot, q editorbuffer.Search, within *[2]int64, separator string, maxLines int) {
	defer job.cancel()
	defer s.Close()
	from, to := findRange(within)
	var answer map[string]any
	var err error
	switch job.kind {
	case "count":
		var count int64
		count, err = s.Count(ctx, q, from, to, job.setProgress)
		answer = map[string]any{"count": count}
	case "all":
		var hits int64
		hits, err = s.FindAll(ctx, q, from, to, job.results.Add, job.setProgress)
		answer = map[string]any{"count": hits}
	case "mark":
		count, lines, more, e := s.Mark(ctx, q, from, to, maxLines, job.setProgress)
		answer, err = map[string]any{"count": count, "lines": lines, "more": more}, e
	case "copy":
		text, count, e := s.MatchedText(ctx, q, from, to, separator, pageToolOutputAllowance)
		answer, err = map[string]any{"count": count, "text": text}, e
	}
	job.finish(answer, err)
}

func findKind(kind string) bool {
	return kind == "count" || kind == "all" || kind == "mark" || kind == "copy"
}

func findSeparator(separator string) string {
	if separator == "\r\n" || separator == "\r" {
		return separator
	}
	return "\n"
}

func (m *editorViewRegistry) newFindJob(caller, kind string, revision uint64, parent context.Context) (*editorFindJob, context.Context) {
	ctx, cancel := context.WithCancel(parent)
	job := &editorFindJob{id: newJobID(), owner: caller, kind: kind, revision: revision, cancel: cancel}
	if kind == "all" {
		job.results = editorbuffer.NewResults("")
	}
	m.find.add(job)
	return job, ctx
}

// editorFindRequest is a request of the Find window to
// /api/editor-views/find.
type editorFindRequest struct {
	Action      string              `json:"action"`
	Text        string              `json:"text"`
	Search      editorbuffer.Search `json:"search"`
	Replacement string              `json:"replacement"`
	Within      *[2]int64           `json:"within"`
	At          int64               `json:"at"`
	From        int64               `json:"from"`
	To          int64               `json:"to"`
	Backwards   bool                `json:"backwards"`
	Wrap        bool                `json:"wrap"`
	Kind        string              `json:"kind"`
	Job         string              `json:"job"`
	Start       int64               `json:"start"`
	Limit       int                 `json:"limit"`
	Separator   string              `json:"separator"`
	List        string              `json:"list"`
	Value       string              `json:"value"`
}

func (m *editorViewRegistry) handleFind(w http.ResponseWriter, r *http.Request, caller string) {
	var req editorFindRequest
	if err := decodeEditorBody(w, r, editorTransferLimit+16384, &req); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	var value any
	var err error
	switch req.Action {
	case "history":
		value = m.find.recall()
	case "remember":
		if err = m.find.remember(req.List, req.Value); err == nil {
			value = map[string]any{"ok": true}
		}
	case "status", "page", "cancel", "release":
		value, err = m.findJobAction(caller, req.Action, req.Job, req.Start, req.Limit)
	case "next", "matches", "replace", "replace-text", "start":
		value, err = m.findInText(r.Context(), caller, req)
	default:
		jsonError(w, 400, "Unsupported Find request.")
		return
	}
	if err != nil {
		jsonError(w, 409, findMessage(err))
		return
	}
	jsonResponse(w, 200, value)
}

func (m *editorViewRegistry) findJobAction(caller, action, id string, start int64, limit int) (any, error) {
	if action == "release" {
		m.find.release(caller, id)
		return map[string]any{"ok": true}, nil
	}
	job, err := m.find.job(caller, id)
	if err != nil {
		return nil, err
	}
	switch action {
	case "cancel":
		job.cancel()
		return map[string]any{"ok": true}, nil
	case "page":
		if job.results == nil {
			return nil, errors.New("These results have no rows.")
		}
		rows, err := job.results.Page(start, min(max(limit, 1), editorFindPageRows))
		if err != nil {
			return nil, err
		}
		state := job.state()
		state["start"], state["items"] = start, rows
		return state, nil
	}
	return job.state(), nil
}

// findInText searches text that a page holds.
func (m *editorViewRegistry) findInText(ctx context.Context, caller string, req editorFindRequest) (any, error) {
	s, err := editorbuffer.TextSnapshot(req.Text)
	if err != nil {
		return nil, err
	}
	from, to := findRange(req.Within)
	switch req.Action {
	case "next":
		defer s.Close()
		return s.FindNext(ctx, req.Search, req.At, from, to, req.Backwards, req.Wrap)
	case "matches":
		defer s.Close()
		limit := editorFindPageMatches
		if req.Limit > 0 && req.Limit < limit {
			limit = req.Limit
		}
		hits, more, err := s.Matches(ctx, req.Search, req.From, req.To, to, limit)
		return map[string]any{"hits": hits, "more": more}, err
	case "replace":
		s.Close()
		text, count, err := editorbuffer.ReplaceInText(ctx, req.Text, req.Search, req.Replacement, from, to, 4*int64(len(req.Text))+pageToolOutputAllowance)
		if err != nil {
			return nil, err
		}
		return map[string]any{"text": text, "count": count, "changed": count > 0 && text != req.Text}, nil
	case "replace-text":
		defer s.Close()
		text, found, err := s.ReplaceTextFor(ctx, req.Search, req.Replacement, req.From, req.To)
		return map[string]any{"found": found, "text": text}, err
	}
	if !findKind(req.Kind) {
		s.Close()
		return nil, errors.New("Unsupported Find request.")
	}
	parent := context.Background()
	if m.app != nil && m.app.ctx != nil {
		parent = m.app.ctx
	}
	job, jobCtx := m.newFindJob(caller, req.Kind, 0, parent)
	go runFind(jobCtx, job, s, req.Search, req.Within, findSeparator(req.Separator), editorBookmarkLimit)
	return map[string]any{"job": job.id}, nil
}

// findSearch is the search of a buffer request: the Find window sends
// "search"; earlier requests name the query and its options one by one.
func (req editorDiskRequest) findSearch() editorbuffer.Search {
	if req.Search != nil {
		return *req.Search
	}
	return editorbuffer.Search{Query: req.Query, Regex: req.Regex, MatchCase: req.MatchCase, WholeWord: req.WholeWord}
}

// findDisk answers the Find window's searches of a disk-backed document.
func (m *editorViewRegistry) findDisk(ctx context.Context, caller string, d *editorDiskDocument, v *editorbuffer.Imported, req editorDiskRequest) (any, error) {
	q := req.findSearch()
	from, to := findRange(req.Within)
	switch req.Action {
	case "find":
		s, err := v.Buffer.Snapshot()
		if err != nil {
			return nil, err
		}
		defer s.Close()
		at := req.Start
		if req.Search != nil {
			at = req.At
		}
		return s.FindNext(ctx, q, at, from, to, req.Backwards, req.Wrap)
	case "find-matches":
		s, err := v.Buffer.Snapshot()
		if err != nil {
			return nil, err
		}
		defer s.Close()
		hits, more, err := s.Matches(ctx, q, req.From, req.To, to, editorFindMatches)
		return map[string]any{"hits": hits, "more": more, "revision": s.Revision}, err
	}
	// find-start
	if req.Kind == "replace" {
		if v.Binary {
			return nil, editorbuffer.ErrBinary
		}
	} else if !findKind(req.Kind) {
		return nil, errors.New("Unsupported Find request.")
	}
	s, err := v.Buffer.Snapshot()
	if err != nil {
		return nil, err
	}
	if req.Kind == "replace" && s.Revision != req.Revision {
		s.Close()
		return nil, editorbuffer.ErrRevision
	}
	job, jobCtx := m.newFindJob(caller, req.Kind, s.Revision, d.ctx)
	// Registered before the start is answered: a reload or close that comes
	// right after it stops the job, and the job releases its snapshot.
	stop, unregister := d.registerStop(jobCtx)
	go func() {
		defer unregister()
		if req.Kind == "replace" {
			d.runReplaceJob(stop, job, s, q, req.Replace, req.Within)
			return
		}
		runFind(stop, job, s, q, req.Within, findSeparator(req.Separator), editorBookmarkLimit)
	}()
	return map[string]any{"job": job.id, "revision": s.Revision}, nil
}

// runReplaceJob is Replace All of a disk-backed document in the background:
// each match replaced on its own (the lines it does not touch keep their
// endings), committed as one undo step.
func (d *editorDiskDocument) runReplaceJob(ctx context.Context, job *editorFindJob, s *editorbuffer.Snapshot, q editorbuffer.Search, replacement string, within *[2]int64) {
	defer job.cancel()
	from, to := findRange(within)
	before := s.Units()
	staged, err := s.ReplaceAllIn(ctx, q, replacement, from, to, job.setProgress)
	// The snapshot holds the buffer open: it is released before this job
	// waits for the document's edit lock (see replaceDiskMatches).
	s.Close()
	if err != nil {
		job.finish(nil, err)
		return
	}
	defer staged.Abort()
	d.editMu.Lock()
	if d.bulk != nil {
		d.editMu.Unlock()
		job.finish(nil, errors.New("finish or cancel the pending bulk edit"))
		return
	}
	if err = ctx.Err(); err != nil {
		d.editMu.Unlock()
		job.finish(nil, err)
		return
	}
	meta, err := staged.Commit()
	d.editMu.Unlock()
	if err != nil {
		job.finish(nil, err)
		return
	}
	if to < 0 {
		to = before
	}
	job.finish(map[string]any{"metadata": meta, "count": staged.Count, "from": from, "to": to + meta.Units - before}, nil)
}
