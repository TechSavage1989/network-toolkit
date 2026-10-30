package main

// Explorer export stages copies before an OS drag. Jobs are caller-bound and
// contain no reusable credentials. The native host receives only private staged
// local paths, never a remote pathname; OLE is copy-only.
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"networktoolkit/internal/model"
)

const explorerExportMaxBytes int64 = 2 << 30
const explorerExportMaxEntries = 1500

type explorerTransferManager struct {
	mu      sync.Mutex
	jobs    map[string]*explorerTransferJob
	imports map[string]*explorerImport
}
type explorerTransferJob struct {
	id, owner, state, note, stage   string
	names, paths                    []string
	entries                         int
	bytes, planned                  int64
	expires, armed                  time.Time
	gesture, revoked                uint64
	cancel                          context.CancelFunc
	valid                           func() bool
	cancelRequested                 bool
	direct, renderStarted, rendered bool
	render                          func()
}
type explorerTransferRequest struct {
	Action  string        `json:"action"`
	Outcome string        `json:"outcome,omitempty"`
	Gesture uint64        `json:"gesture,omitempty"`
	ID      string        `json:"id"`
	Binding editorBinding `json:"binding"`
	Paths   []string      `json:"paths"`
}

func (a *application) explorerCallerValid(owner string) bool {
	if a.editorViews == nil {
		return owner == "" && !a.shutdownRequested.Load()
	}
	a.editorViews.mu.Lock()
	defer a.editorViews.mu.Unlock()
	if owner == "" {
		return a.editorViews.callerLocked("")
	}
	v := a.editorViews.views[owner]
	return a.editorViews.callerLocked(owner) && v != nil
}
func exportJobStatus(j *explorerTransferJob) map[string]any {
	return map[string]any{"id": j.id, "state": j.state, "names": j.names, "entries": j.entries, "bytes": j.bytes, "message": j.note, "cancelRequested": j.cancelRequested}
}
func (a *application) handleExplorerTransfer(w http.ResponseWriter, r *http.Request) {
	owner := editorRequestView(r)
	if !a.explorerCallerValid(owner) {
		jsonError(w, 403, "File explorer access is no longer available.")
		return
	}
	m := &a.explorerTransfers
	if r.URL.Path == "/api/explorer-transfer/native" {
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if id := r.URL.Query().Get("id"); id != "" {
			j := m.jobs[id]
			if j == nil || j.owner != owner || !j.direct || j.state != "dragging" || !j.valid() || j.cancelRequested {
				jsonError(w, 409, "The copy is no longer available.")
				return
			}
			if !j.rendered {
				jsonResponse(w, 202, exportJobStatus(j))
				return
			}
			jsonResponse(w, 200, map[string]any{"id": j.id, "paths": j.paths, "gesture": j.gesture})
			return
		}
		for _, j := range m.jobs {
			expireExplorerClaim(j)
			if j.owner == owner && (j.state == "ready" || j.state == "claimed") && !j.armed.IsZero() && time.Now().Before(j.armed) && time.Now().Before(j.expires) && j.valid() {
				j.state = "claimed"
				jsonResponse(w, 200, map[string]any{"id": j.id, "paths": j.paths, "gesture": j.gesture, "direct": j.direct})
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method == http.MethodGet {
		m.mu.Lock()
		defer m.mu.Unlock()
		j := m.jobs[r.URL.Query().Get("id")]
		if j == nil || j.owner != owner {
			jsonError(w, 404, "Copy preparation is no longer available.")
			return
		}
		expireExplorerClaim(j)
		jsonResponse(w, 200, exportJobStatus(j))
		return
	}
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	var req explorerTransferRequest
	if e := decodeEditorBody(w, r, 64<<10, &req); e != nil {
		jsonError(w, 400, "Invalid file copy request.")
		return
	}
	if req.Action == "prepare" || req.Action == "direct" {
		a.prepareExplorerTransfer(w, r, req)
		return
	}
	if !editorHexID.MatchString(req.ID) || len(req.Paths) != 0 || req.Binding.Kind != "" {
		jsonError(w, 400, "Invalid copy operation.")
		return
	}
	m.mu.Lock()
	j := m.jobs[req.ID]
	if j == nil || j.owner != owner {
		m.mu.Unlock()
		jsonError(w, 404, "Copy preparation is no longer available.")
		return
	}
	expireExplorerClaim(j)
	switch req.Action {
	case "materialize":
		if !j.direct || j.state != "dragging" || !j.valid() || j.cancelRequested {
			m.mu.Unlock()
			jsonError(w, 409, "The copy is no longer available.")
			return
		}
		if !j.renderStarted {
			j.renderStarted = true
			j.note = "Copying to Windows Explorer… Press Escape to cancel."
			go j.render()
		}
	case "start":
		if req.Gesture == j.gesture && j.state == "dragging" {
			break
		}
		if req.Gesture != j.gesture || req.Gesture <= j.revoked || j.state != "claimed" || !time.Now().Before(j.armed) || !j.valid() {
			m.mu.Unlock()
			jsonError(w, 409, "The drag gesture expired or was cancelled.")
			return
		}
		j.state = "dragging"
		j.armed = time.Time{}
	case "arm":
		if req.Gesture == 0 || req.Gesture > 1000000 || req.Gesture <= j.revoked || req.Gesture <= j.gesture {
			m.mu.Unlock()
			jsonError(w, 409, "This drag gesture ended. Start a new drag.")
			return
		}
		if j.state != "ready" || !j.valid() || time.Now().After(j.expires) {
			m.mu.Unlock()
			jsonError(w, 409, "Prepare this selection again before dragging.")
			return
		}
		j.gesture = req.Gesture
		j.armed = time.Now().Add(2 * time.Second)
	case "disarm":
		if req.Gesture == 0 || req.Gesture > 1000000 {
			m.mu.Unlock()
			jsonError(w, 400, "Invalid gesture.")
			return
		}
		if req.Gesture > j.revoked {
			j.revoked = req.Gesture
		}
		if req.Gesture >= j.gesture && j.state != "dragging" {
			j.armed = time.Time{}
			if j.state == "claimed" {
				j.state = "ready"
			}
		}
	case "cancel":
		if j.state == "dragging" && j.direct {
			j.cancelRequested = true
			j.cancel()
			j.note = "Cancelling the Windows copy…"
			break
		}
		if j.state == "dragging" {
			m.mu.Unlock()
			jsonError(w, 409, "Release the drag or press Escape in Windows to cancel.")
			return
		}
		j.cancel()
		j.state = "cancelled"
		j.note = "Cancelled. Original files are unchanged."
		if j.stage != "" {
			os.RemoveAll(j.stage)
			j.stage = ""
		}
		j.paths = nil
	case "finish":
		if j.state == "finished" || j.state == "cancelled" || j.state == "failed" {
			break
		}
		if j.state != "dragging" {
			m.mu.Unlock()
			jsonError(w, 409, "No native drag is active.")
			return
		}
		j.cancel()
		j.state = "finished"
		switch req.Outcome {
		case "copied":
			j.note = "Windows reported the copy completed. Original files are unchanged."
		case "cancelled":
			j.note = "Windows copy cancelled. Original files are unchanged."
		default:
			j.state = "failed"
			j.note = "Windows did not complete the copy. Check the destination before retrying; original files are unchanged."
		}
		if j.stage != "" {
			os.RemoveAll(j.stage)
			j.stage = ""
		}
		j.paths = nil
	default:
		m.mu.Unlock()
		jsonError(w, 400, "Unknown copy operation.")
		return
	}
	response := exportJobStatus(j)
	m.mu.Unlock()
	jsonResponse(w, 200, response)
}
func (a *application) prepareExplorerTransfer(w http.ResponseWriter, r *http.Request, req explorerTransferRequest) {
	if req.ID != "" || len(req.Paths) < 1 || len(req.Paths) > 100 {
		jsonError(w, 400, "Select 1–100 files or folders.")
		return
	}
	owner := editorRequestView(r)
	local := req.Binding.Kind == "local"
	if local && (req.Binding.SessionID != "" || req.Binding.ConnectionID != "" || req.Binding.Revision != 0) {
		jsonError(w, 400, "Local copy cannot include a remote binding.")
		return
	}
	if !local && req.Binding.Kind != "terminal" && req.Binding.Kind != "sshConnection" {
		jsonError(w, 400, "Use local files or an already connected SFTP session.")
		return
	}
	var grant *editorDocumentGrant
	var err error
	if !local {
		if a.editorViews == nil {
			jsonError(w, 503, "File service unavailable.")
			return
		}
		grant, err = a.editorViews.makeGrant(owner, editorDocumentSnapshot{Binding: req.Binding})
		if err != nil {
			jsonError(w, 403, err.Error())
			return
		}
	}
	names := make([]string, 0, len(req.Paths))
	seen := map[string]bool{}
	for _, p := range req.Paths {
		if len(p) == 0 || len(p) > 4096 || strings.ContainsAny(p, "\x00\r\n") || local && !filepath.IsAbs(p) {
			jsonError(w, 400, "Select valid file paths.")
			return
		}
		name := path.Base(strings.ReplaceAll(p, "\\", "/"))
		if validateExplorerDropName(name) != nil || seen[strings.ToLower(name)] {
			jsonError(w, 400, "Selection contains duplicate or unsupported names. Copy them separately.")
			return
		}
		seen[strings.ToLower(name)] = true
		names = append(names, name)
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 15*time.Minute)
	j := &explorerTransferJob{id: randomHex(16), owner: owner, state: "preparing", names: names, note: "Preparing a private copy for Windows Explorer…", cancel: cancel, expires: time.Now().Add(15 * time.Minute)}
	var approved *editorExplorerAccess
	if owner != "" && !local {
		a.editorViews.mu.Lock()
		if v := a.editorViews.views[owner]; v != nil {
			approved = v.explorer
		}
		a.editorViews.mu.Unlock()
	}
	localValid := a.localExplorerGuard(owner)
	j.valid = func() bool {
		if local && !localValid() {
			return false
		}
		if !a.explorerCallerValid(owner) {
			return false
		}
		if owner != "" && !local {
			a.editorViews.mu.Lock()
			v := a.editorViews.views[owner]
			current := approved != nil && approved.Allowed && v != nil && v.explorer == approved
			a.editorViews.mu.Unlock()
			if !current {
				return false
			}
		}
		return grant == nil || grant.valid()
	}
	m := &a.explorerTransfers
	m.mu.Lock()
	if m.jobs == nil {
		m.jobs = map[string]*explorerTransferJob{}
	}
	active := 0
	for id, old := range m.jobs {
		if time.Now().After(old.expires) && old.state != "dragging" {
			old.cancel()
			if old.stage != "" {
				os.RemoveAll(old.stage)
			}
			delete(m.jobs, id)
			continue
		}
		if old.owner == owner && (old.state == "preparing" || old.state == "ready" || old.state == "claimed" || old.state == "dragging") {
			active++
		}
	}
	if active > 0 || len(m.jobs) >= 24 {
		m.mu.Unlock()
		cancel()
		jsonError(w, 409, "Finish or cancel the existing prepared copy first.")
		return
	}
	m.jobs[j.id] = j
	response := exportJobStatus(j)
	m.mu.Unlock()
	if req.Action == "direct" {
		// This registration retains only the existing pinned grant and selected
		// source names. No source bytes are read until the native drop receiver
		// asks for the copy after the physical button is released.
		m.mu.Lock()
		j.direct = true
		j.state = "ready"
		j.note = "Drag to Windows Explorer to copy."
		j.render = func() { a.stageExplorerExport(ctx, j, req.Binding, req.Paths, grant) }
		response = exportJobStatus(j)
		m.mu.Unlock()
	} else {
		go a.stageExplorerExport(ctx, j, req.Binding, req.Paths, grant)
	}
	go a.watchExplorerCopy(ctx, j)
	time.AfterFunc(15*time.Minute, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.jobs[j.id] != j || j.state == "dragging" {
			return
		}
		j.cancel()
		if j.stage != "" {
			os.RemoveAll(j.stage)
		}
		delete(m.jobs, j.id)
	})
	jsonResponse(w, 202, response)
}
func (a *application) stageExplorerExport(ctx context.Context, j *explorerTransferJob, b editorBinding, sources []string, g *editorDocumentGrant) {
	stage, err := os.MkdirTemp("", "ntk-explorer-copy-")
	var paths []string
	if err == nil {
		for i, source := range sources {
			dest := filepath.Join(stage, j.names[i])
			if b.Kind == "local" {
				err = a.stageExplorerLocal(ctx, j, source, dest, 0)
			} else {
				err = a.stageExplorerRemote(ctx, j, g, source, dest, 0)
			}
			if err != nil {
				break
			}
			paths = append(paths, dest)
		}
	}
	m := &a.explorerTransfers
	m.mu.Lock()
	defer m.mu.Unlock()
	if err == nil {
		err = ctx.Err()
	}
	if err == nil && !j.valid() {
		err = errors.New("File explorer access or connection changed.")
	}
	if err != nil {
		if stage != "" {
			os.RemoveAll(stage)
		}
		if j.direct && j.cancelRequested {
			j.state = "cancelled"
			j.note = "Windows copy cancelled. Original files are unchanged."
		} else if j.state != "cancelled" {
			j.state = "failed"
			j.note = err.Error()
		}
		return
	}
	j.stage = stage
	j.paths = paths
	if j.direct {
		j.rendered = true
		j.note = "Copying to Windows Explorer…"
		return
	}
	j.state = "ready"
	j.note = "Ready. Drag the prepared copy to Windows Explorer. Windows asks before replacing existing items."
}
func (a *application) exportCheck(ctx context.Context, j *explorerTransferJob, depth int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !j.valid() {
		return errors.New("File explorer access or connection changed.")
	}
	m := &a.explorerTransfers
	m.mu.Lock()
	defer m.mu.Unlock()
	j.entries++
	if depth > 32 || j.entries > explorerExportMaxEntries {
		return errors.New("Copy exceeds 1,500 items or 32 folder levels.")
	}
	return nil
}
func (a *application) exportAddBytes(j *explorerTransferJob, n int64) error {
	m := &a.explorerTransfers
	m.mu.Lock()
	defer m.mu.Unlock()
	if n < 0 || j.planned > explorerExportMaxBytes-n {
		return errors.New("Prepared copy exceeds 2 GiB.")
	}
	j.planned += n
	return nil
}
func (a *application) stageExplorerLocal(ctx context.Context, j *explorerTransferJob, source, dest string, depth int) error {
	root, err := os.OpenRoot(filepath.Dir(source))
	if err != nil {
		return err
	}
	defer root.Close()
	return a.stageExplorerLocalAt(ctx, j, root, filepath.Base(source), dest, depth)
}
func (a *application) stageExplorerLocalAt(ctx context.Context, j *explorerTransferJob, root *os.Root, source, dest string, depth int) error {
	if err := a.exportCheck(ctx, j, depth); err != nil {
		return err
	}
	info, err := root.Lstat(source)
	if err != nil {
		return err
	}
	if validateExplorerDropName(filepath.Base(source)) != nil || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return errors.New("Links, shortcuts and special files are not copied.")
	}
	if info.IsDir() {
		if err = os.Mkdir(dest, 0700); err != nil {
			return err
		}
		f, err := root.Open(source)
		if err != nil {
			return err
		}
		opened, e := f.Stat()
		if e != nil || !os.SameFile(info, opened) {
			f.Close()
			return errors.New("Source folder changed.")
		}
		rows, err := f.ReadDir(explorerExportMaxEntries + 1)
		f.Close()
		if err != nil && err != io.EOF {
			return err
		}
		if len(rows) > explorerExportMaxEntries {
			return errors.New("Folder contains too many entries.")
		}
		for _, row := range rows {
			if err := a.stageExplorerLocalAt(ctx, j, root, filepath.Join(source, row.Name()), filepath.Join(dest, row.Name()), depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return errors.New("Special files are not copied.")
	}
	if err = a.exportAddBytes(j, info.Size()); err != nil {
		return err
	}
	in, err := root.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	now, e := in.Stat()
	if e != nil || !os.SameFile(info, now) {
		return errors.New("Source file changed.")
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	n, err := io.Copy(out, io.LimitReader(&explorerContextReader{ctx: ctx, reader: in}, info.Size()+1))
	closeErr := out.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if n != info.Size() {
		return errors.New("Source file changed size while copying.")
	}
	a.explorerTransfers.mu.Lock()
	j.bytes += n
	a.explorerTransfers.mu.Unlock()
	return nil
}
func remoteExportData(result model.Result) map[string]any {
	data, _ := result.Data.(map[string]any)
	return data
}
func (a *application) stageExplorerRemote(ctx context.Context, j *explorerTransferJob, g *editorDocumentGrant, source, dest string, depth int) error {
	if err := a.exportCheck(ctx, j, depth); err != nil {
		return err
	}
	expectedSize := int64(0)
	run := func(action, p, local string) (model.Result, error) {
		if !j.valid() {
			return model.Result{}, errors.New("SFTP access changed.")
		}
		finish := a.activity.begin(ctx, "sftp")
		defer finish()
		return g.run(ctx, remoteBrowserRequest{Protocol: "sftp", Action: action, Path: p, LocalPath: local, SafeExport: true, ExportSize: expectedSize})
	}
	stat, err := run("stat", source, "")
	if err != nil {
		return err
	}
	data := remoteExportData(stat)
	kind, _ := data["kind"].(string)
	if kind == "directory" {
		if err = os.Mkdir(dest, 0700); err != nil {
			return err
		}
		listing, err := run("list", source, "")
		if err != nil {
			return err
		}
		raw, _ := json.Marshal(remoteExportData(listing)["entries"])
		var entries []struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
		}
		if err = json.Unmarshal(raw, &entries); err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.Name == "." || entry.Name == ".." {
				continue
			}
			if validateExplorerDropName(entry.Name) != nil || entry.Kind != "File" && entry.Kind != "Directory" {
				return errors.New("Remote folder contains a link, special file or unsupported name.")
			}
			if err = a.stageExplorerRemote(ctx, j, g, path.Join(source, entry.Name), filepath.Join(dest, entry.Name), depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if kind != "file" {
		return errors.New("Remote links and special files are not copied.")
	}
	raw, _ := json.Marshal(data["sizeBytes"])
	var size int64
	if err = json.Unmarshal(raw, &size); err != nil {
		return fmt.Errorf("Unknown remote file size: %w", err)
	}
	if err = a.exportAddBytes(j, size); err != nil {
		return err
	}
	expectedSize = size
	_, err = run("download", source, dest)
	if err == nil {
		a.explorerTransfers.mu.Lock()
		j.bytes += size
		a.explorerTransfers.mu.Unlock()
	}
	return err
}

// Revocation requests cancellation but never removes bytes under an active OLE
// reader. Completion or a confirmed native owner exit owns the final cleanup.
func (a *application) watchExplorerCopy(ctx context.Context, j *explorerTransferJob) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for range tick.C {
		valid := j.valid()
		m := &a.explorerTransfers
		m.mu.Lock()
		if m.jobs[j.id] != j || j.state == "finished" || j.state == "failed" || j.state == "cancelled" {
			m.mu.Unlock()
			return
		}
		if (!valid || ctx.Err() != nil) && j.state == "dragging" {
			j.cancelRequested = true
			j.note = "Cancellation pending. Press Escape or release the mouse to end the Windows drag. Original files are unchanged."
			m.mu.Unlock()
			continue
		}
		if !valid || ctx.Err() != nil {
			j.cancel()
			j.state = "cancelled"
			j.note = "Copy preparation expired or access was withdrawn. Original files are unchanged."
			if j.stage != "" {
				os.RemoveAll(j.stage)
				j.stage = ""
			}
			j.paths = nil
			m.mu.Unlock()
			return
		}
		m.mu.Unlock()
	}
}

func expireExplorerClaim(j *explorerTransferJob) {
	if j.state == "claimed" && !time.Now().Before(j.armed) {
		j.state = "ready"
		j.armed = time.Time{}
	}
}

// Called only after the native main host returned, or its exact child process
// exited. A missed heartbeat, timeout or withdrawn approval is not this proof.
func (a *application) finishExplorerOwner(owner string) {
	m := &a.explorerTransfers
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, j := range m.jobs {
		if j.owner != owner {
			continue
		}
		j.cancel()
		if j.stage != "" {
			os.RemoveAll(j.stage)
		}
		delete(m.jobs, id)
	}
}

// Whole-app shutdown must not reclaim another still-live editor child's active
// OLE source. An interrupted backend can leave that private stage for cleanup.
func (a *application) finishInactiveExplorerCopies() {
	m := &a.explorerTransfers
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, j := range m.jobs {
		if j.state == "dragging" {
			continue
		}
		j.cancel()
		if j.stage != "" {
			os.RemoveAll(j.stage)
		}
		delete(m.jobs, id)
	}
}
