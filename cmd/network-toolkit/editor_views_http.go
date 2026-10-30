package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"networktoolkit/internal/model"
	"runtime"
	"strings"
)

func decodeEditorBody(w http.ResponseWriter, r *http.Request, limit int64, out any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return errors.New("Invalid or oversized editor request.")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("Send one editor request only.")
	}
	return nil
}
func (a *application) serveEditorAPI(w http.ResponseWriter, r *http.Request) {
	if a.editorViews == nil {
		jsonError(w, 503, "Editor windows are unavailable.")
		return
	}
	if a.editorViews.returned(editorRequestView(r)) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/editor-views/current/pulse" {
			jsonError(w, http.StatusGone, "Editor documents returned.")
		} else {
			jsonError(w, http.StatusForbidden, "This editor window has returned its documents.")
		}
		return
	}
	if a.serveEditorExplorer(w, r) {
		return
	}
	switch {
	case r.URL.Path == "/api/status" && r.Method == http.MethodGet:
		jsonResponse(w, 200, map[string]any{"editorWindowAvailable": a.launchEditorView != nil, "version": model.Version, "os": runtime.GOOS, "arch": runtime.GOARCH})
	case r.URL.Path == "/api/editor-views" || strings.HasPrefix(r.URL.Path, "/api/editor-views/"):
		a.handleEditorViews(w, r)
	case r.URL.Path == "/api/file-editor/local":
		// New local opens are explicit reads. Every child write must go through
		// its document lease, including Untitled/Save As; no stale-owner bypass.
		if r.Method != http.MethodPost {
			methodError(w)
			return
		}
		var request struct {
			Action string `json:"action"`
			Path   string `json:"path"`
		}
		if err := decodeEditorBody(w, r, 8192, &request); err != nil || request.Action != "read-text" {
			jsonError(w, 403, "Editor writes require the current document lease.")
			return
		}
		payload, _ := json.Marshal(request)
		next := r.Clone(r.Context())
		next.Body = io.NopCloser(bytes.NewReader(payload))
		next.ContentLength = int64(len(payload))
		a.handleLocalTextEditor(w, next)
	case r.URL.Path == "/api/remote-browser" && r.Method == http.MethodGet:
		jsonResponse(w, 200, map[string]any{"sessions": []any{}, "connections": []any{}})
	default:
		jsonError(w, http.StatusForbidden, "This capability is limited to its editor documents.")
	}
}
func (a *application) handleEditorViews(w http.ResponseWriter, r *http.Request) {
	m := a.editorViews
	if m == nil {
		jsonError(w, 503, "Editor windows are unavailable.")
		return
	}
	caller := editorRequestView(r)
	path := strings.TrimPrefix(r.URL.Path, "/api/editor-views")
	if (r.Method == http.MethodGet && (path == "" || path == "/current")) || (r.Method == http.MethodPost && (path == "/transfers" || path == "/current/state" || (strings.HasPrefix(path, "/documents/") && strings.HasSuffix(path, "/file")))) {
		var release func()
		var admitted bool
		r, release, admitted = admitEditorMemory(w, r)
		if !admitted {
			return
		}
		defer release()
	}
	if m.handleSavedConnections(w, r, caller, path) {
		return
	}
	if m.handleExplorerAccess(w, r, caller, path) {
		return
	}
	if r.Method == http.MethodGet && (path == "" || path == "/current") {
		jsonResponse(w, 200, m.state(caller))
		return
	}
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	if path == "/drags" || strings.HasPrefix(path, "/drags/") {
		m.handleDrag(w, r, caller, path)
		return
	}
	if path == "/current/prepare-close" || path == "/current/cancel-close" {
		if caller != "" {
			jsonError(w, 403, "Only the main workspace can prepare its close.")
			return
		}
		if path == "/current/prepare-close" {
			if err := m.beginHomeClose(); err != nil {
				jsonError(w, 409, err.Error())
				return
			}
		} else {
			if err := m.cancelHomeClose(); err != nil {
				jsonError(w, 409, err.Error())
				return
			}
		}
		jsonResponse(w, 200, map[string]any{"ok": true})
		return
	}
	if path == "/current/pulse" {
		if caller == "" {
			jsonError(w, 403, "Choose an editor window.")
			return
		}
		m.mu.Lock()
		v := m.views[caller]
		if v == nil || v.info.State == "closed" {
			m.mu.Unlock()
			jsonError(w, 410, "Editor window unavailable.")
			return
		}
		if v.info.State == "returned" {
			m.mu.Unlock()
			jsonError(w, 410, "Editor documents returned.")
			return
		}
		v.lastPulse = m.now()
		v.info.State = "ready"
		info := v.info
		m.mu.Unlock()
		jsonResponse(w, 200, map[string]any{"viewId": caller, "state": info.State, "ownerInstanceId": a.windowInstanceID})
		return
	}
	if path == "/current/release" {
		if err := m.release(caller); err != nil {
			jsonError(w, 409, err.Error())
			return
		}
		jsonResponse(w, 200, map[string]any{"ok": true})
		return
	}
	if path == "/transfers" {
		var request editorTransferRequest
		if err := decodeEditorBody(w, r, editorTransferLimit+16384, &request); err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		info, err := m.offer(caller, request)
		if err != nil {
			jsonError(w, 409, err.Error())
			return
		}
		jsonResponse(w, 200, info)
		return
	}
	if path == "/current/activity" {
		m.handleLoaded(w, r, caller)
		return
	}
	if path == "/current/state" {
		var request struct {
			Batch  editorBatch       `json:"batch"`
			Leases map[string]uint64 `json:"leases"`
		}
		if err := decodeEditorBody(w, r, editorTransferLimit+16384, &request); err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		leases, err := m.checkpoint(caller, request.Batch, request.Leases)
		if err != nil {
			jsonError(w, 409, err.Error())
			return
		}
		jsonResponse(w, 200, map[string]any{"owned": leases})
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) == 3 && parts[0] == "transfers" && editorHexID.MatchString(parts[1]) && parts[2] == "complete" {
		if err := m.complete(caller, parts[1]); err != nil {
			status := http.StatusConflict
			if errors.Is(err, errEditorTransferMissing) {
				status = http.StatusNotFound
			}
			jsonError(w, status, err.Error())
			return
		}
		jsonResponse(w, 200, map[string]any{"ok": true})
		return
	}
	if len(parts) == 3 && parts[0] == "transfers" && editorHexID.MatchString(parts[1]) && (parts[2] == "accept" || parts[2] == "cancel") {
		var result map[string]any
		var err error
		if parts[2] == "accept" {
			result, err = m.accept(caller, parts[1])
		} else {
			result, err = m.cancel(caller, parts[1])
		}
		if err != nil {
			jsonError(w, 409, err.Error())
			return
		}
		jsonResponse(w, 200, result)
		return
	}
	if len(parts) == 3 && parts[0] == "documents" && editorUID.MatchString(parts[1]) {
		if parts[2] == "file" {
			m.file(w, r, caller, parts[1])
			return
		}
		if parts[2] == "release" {
			var request struct {
				Lease uint64 `json:"lease"`
			}
			if err := decodeEditorBody(w, r, 1024, &request); err != nil {
				jsonError(w, 400, err.Error())
				return
			}
			if err := m.releaseDocument(caller, parts[1], request.Lease); err != nil {
				jsonError(w, 409, err.Error())
				return
			}
			jsonResponse(w, 200, map[string]any{"ok": true})
			return
		}
	}
	http.NotFound(w, r)
}

func (m *editorViewRegistry) checkpoint(caller string, batch editorBatch, leases map[string]uint64) (map[string]uint64, error) {
	if err := validateEditorBatch(batch); err != nil {
		return nil, err
	}
	if err := validateEditorLeases(batch, leases); err != nil {
		return nil, err
	}
	grants := map[string]*editorDocumentGrant{}
	m.mu.Lock()
	for _, d := range batch.Documents {
		if m.documents[d.UID] == nil {
			grants[d.UID] = nil
		}
	}
	m.mu.Unlock()
	for _, d := range batch.Documents {
		if _, ok := grants[d.UID]; ok {
			grant, err := m.makeGrant(caller, d)
			if err != nil {
				return nil, err
			}
			grants[d.UID] = grant
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.callerLocked(caller) {
		return nil, errEditorLease
	}
	total, count, ownedCount := 0, len(m.documents), 0
	for _, d := range m.documents {
		total += d.bytes
		if d.owner == caller {
			ownedCount++
		}
	}
	for _, snapshot := range batch.Documents {
		if m.explorerConflictLocked(snapshot.Binding, snapshot.Path) {
			return nil, errors.New("A file explorer operation is changing this path.")
		}
		d := m.documents[snapshot.UID]
		if d == nil {
			if grants[snapshot.UID] == nil || leases[snapshot.UID] != 0 {
				return nil, errEditorLease
			}
			count++
			ownedCount++
		} else {
			if d.owner != caller || d.lease != leases[snapshot.UID] || d.pending != "" || d.busy != 0 || !editorSameBinding(d.snapshot.Binding, snapshot.Binding) {
				return nil, errEditorLease
			}
			total -= d.bytes
		}
		total += editorSnapshotBytes(snapshot)
	}
	if count > editorDocumentCountLimit || ownedCount > 12 || total > editorTransferLimit {
		return nil, errors.New("The workspace editor recovery budget is full.")
	}
	batch = editorCloneBatch(batch)
	out := map[string]uint64{}
	for _, snapshot := range batch.Documents {
		if m.explorerConflictLocked(snapshot.Binding, snapshot.Path) {
			return nil, errors.New("A file explorer operation is changing this path.")
		}
		d := m.documents[snapshot.UID]
		if d == nil {
			d = grants[snapshot.UID]
			m.registerLoadedLocked(snapshot.UID)
			m.documents[snapshot.UID] = d
		}
		d.snapshot = snapshot
		d.bytes = editorSnapshotBytes(snapshot)
		out[snapshot.UID] = d.lease
		delete(m.recovery, snapshot.UID)
	}
	for uid, d := range m.documents {
		if d.owner == caller {
			out[uid] = d.lease
		}
	}
	return out, nil
}
func (m *editorViewRegistry) releaseDocument(caller, uid string, lease uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d := m.documents[uid]
	if d == nil || d.owner != caller || d.lease != lease || d.pending != "" || d.busy != 0 {
		return errEditorLease
	}
	delete(m.documents, uid)
	delete(m.recovery, uid)
	return nil
}

type editorFileRequest struct {
	Lease        uint64 `json:"lease"`
	Action       string `json:"action"`
	Path         string `json:"path"`
	Content      string `json:"content"`
	ExpectedHash string `json:"expectedHash"`
	BOM          bool   `json:"bom"`
	Encoding     string `json:"encoding"`
}

type editorResponseStatus struct {
	http.ResponseWriter
	status int
}

func (w *editorResponseStatus) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *editorResponseStatus) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (m *editorViewRegistry) file(w http.ResponseWriter, r *http.Request, caller, uid string) {
	var request editorFileRequest
	if err := decodeEditorBody(w, r, textEditorRequestLimit, &request); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	if (request.Action != "read-text" && request.Action != "write-text") || !validEditorText(request.Path, 4096) || request.Path == "" || request.ExpectedHash != "" && !editorHash.MatchString(request.ExpectedHash) {
		jsonError(w, 400, "Unsupported editor file operation.")
		return
	}
	m.mu.Lock()
	d := m.documents[uid]
	if !m.callerLocked(caller) || d == nil || d.owner != caller || d.lease != request.Lease || d.pending != "" || d.busy != 0 || m.explorerConflictLocked(d.snapshot.Binding, request.Path) {
		m.mu.Unlock()
		jsonError(w, 409, errEditorLease.Error())
		return
	}
	if request.Path != d.snapshot.Path && (request.Action != "write-text" || request.ExpectedHash != "") {
		m.mu.Unlock()
		jsonError(w, 409, "Reload the owned file or explicitly save a new copy.")
		return
	}
	d.busy++
	if request.Action == "write-text" {
		d.filePath = request.Path
	}
	binding := d.snapshot.Binding
	m.mu.Unlock()
	response := &editorResponseStatus{ResponseWriter: w}
	defer func() {
		m.mu.Lock()
		// Preserve content/hash from one checkpoint together; only advance the
		// successfully written Save As path before the next read is admitted.
		if request.Action == "write-text" && response.status >= 200 && response.status < 300 {
			d.snapshot.Path = request.Path
		}
		d.filePath = ""
		d.busy--
		recover := d.orphan && d.busy == 0
		if recover {
			d.owner = ""
			d.lease++
			d.orphan = false
			m.recovery[uid] = true
		}
		m.mu.Unlock()
		if recover {
			m.app.activatePrimary()
		}
	}()
	if d.valid != nil && !d.valid() {
		jsonError(w, 409, "The original SSH transport or ownership changed. Save a local copy or reopen the file.")
		return
	}
	if binding.Kind == "local" || binding.Kind == "localDraft" {
		payload, _ := json.Marshal(map[string]any{"action": request.Action, "path": request.Path, "content": request.Content, "expectedHash": request.ExpectedHash, "bom": request.BOM, "encoding": request.Encoding})
		next := r.Clone(r.Context())
		next.Body = io.NopCloser(bytes.NewReader(payload))
		next.ContentLength = int64(len(payload))
		m.app.handleLocalTextEditor(response, next)
		return
	}
	finishActivity := m.app.activity.begin(r.Context(), "sftp")
	defer finishActivity()
	// This callback never resolves IDs again. Even a concurrent reconnect can
	// only cancel the captured client; it cannot retarget the file operation.
	req := remoteBrowserRequest{Protocol: "sftp", Action: request.Action, Path: request.Path, Content: request.Content, ExpectedHash: request.ExpectedHash, BOM: request.BOM, Encoding: request.Encoding}
	m.app.handleRemoteTextEditorWithOperation(response, r, req, func(ctx context.Context, req remoteBrowserRequest) (model.Result, error) {
		if d.valid == nil || !d.valid() {
			return model.Result{}, errors.New("The original SSH transport or ownership changed.")
		}
		return d.run(ctx, req)
	})
}
