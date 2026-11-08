package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"networktoolkit/internal/editorbuffer"
	"os"
	"path/filepath"
	"sync"
)

// This state stays with the existing document grant across window moves. Only
// the owning lease may reach it; checkpoints contain no source text or history.
type editorDiskDocument struct {
	editMu    sync.Mutex
	receipts  []editorDiskReceipt
	mu        sync.Mutex
	imported  *editorbuffer.Imported
	progress  int64
	err       string
	loading   bool
	cancel    context.CancelFunc
	closed    bool
	ctx       context.Context
	saving    bool
	reloading bool
}

type editorDiskReceipt struct {
	id       string
	digest   [32]byte
	metadata editorbuffer.Metadata
}

// A renderer may lose an edit acknowledgement after the server applied it.
// Replaying that exact request must acknowledge the original edit, not apply
// it twice or force the renderer to throw away newer, unacknowledged typing.
func (d *editorDiskDocument) mutate(b *editorbuffer.Buffer, req editorDiskRequest) (editorbuffer.Metadata, error) {
	d.editMu.Lock()
	defer d.editMu.Unlock()
	if len(req.RequestID) > 128 {
		return editorbuffer.Metadata{}, errors.New("invalid edit request identity")
	}
	encoded, _ := json.Marshal(req)
	digest := sha256.Sum256(encoded)
	if req.RequestID != "" {
		for _, receipt := range d.receipts {
			if receipt.id == req.RequestID {
				if receipt.digest != digest {
					return editorbuffer.Metadata{}, errors.New("edit request identity was reused with different contents")
				}
				return receipt.metadata, nil
			}
		}
	}
	var metadata editorbuffer.Metadata
	var err error
	if req.Action == "edit" {
		metadata, err = b.Edit(req.Revision, req.From, req.To, req.Text)
	} else {
		metadata, err = b.History(req.Revision, req.Action == "redo")
	}
	if err == nil && req.RequestID != "" {
		d.receipts = append(d.receipts, editorDiskReceipt{req.RequestID, digest, metadata})
		if len(d.receipts) > 64 {
			copy(d.receipts, d.receipts[len(d.receipts)-64:])
			d.receipts = d.receipts[:64]
		}
	}
	return metadata, err
}

func (g *editorDocumentGrant) diskSnapshotMatches(s editorDocumentSnapshot) bool {
	if (g.buffer != nil) != s.Disk {
		return false
	}
	if g.buffer == nil {
		return true
	}
	g.buffer.mu.Lock()
	defer g.buffer.mu.Unlock()
	return !g.buffer.closed && !g.buffer.loading && g.buffer.imported != nil && s.Path == g.snapshot.Path && s.EditRevision == g.buffer.imported.Buffer.Metadata().Revision
}

func (d *editorDiskDocument) close() {
	d.mu.Lock()
	d.closed = true
	d.cancel()
	b := d.imported
	d.imported = nil
	d.mu.Unlock()
	if b != nil {
		b.Buffer.Close()
	}
}
func (m *editorViewRegistry) openDiskDocument(w http.ResponseWriter, r *http.Request, caller string) {
	var req struct {
		Document editorDocumentSnapshot `json:"document"`
	}
	if e := decodeEditorBody(w, r, 16384, &req); e != nil {
		jsonError(w, 400, e.Error())
		return
	}
	s := req.Document
	if s.Disk || s.Content != "" || s.Original != "" || s.Saved != "" || s.Dirty || len(s.Undo) > 0 || len(s.Redo) > 0 {
		jsonError(w, 400, "Open disk-backed files from an empty document identity.")
		return
	}
	if e := validateEditorBatch(editorBatch{Documents: []editorDocumentSnapshot{s}, Active: s.UID}); e != nil {
		jsonError(w, 400, e.Error())
		return
	}
	grant, e := m.makeGrant(caller, s)
	if e != nil {
		jsonError(w, 403, e.Error())
		return
	}
	if s.Binding.Kind == "local" || s.Binding.Kind == "localDraft" {
		p, e := absoluteExplorerPath(s.Path)
		if e != nil {
			jsonError(w, 400, e.Error())
			return
		}
		s.Path = p
		grant.snapshot.Path = p
	}
	parent := m.app.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	disk := &editorDiskDocument{loading: true, ctx: ctx, cancel: cancel}
	grant.buffer = disk
	grant.snapshot.Disk = true
	m.mu.Lock()
	owned := 0
	for _, d := range m.documents {
		if d.owner == caller {
			owned++
		}
	}
	if !m.callerLocked(caller) || m.documents[s.UID] != nil || len(m.documents) >= editorDocumentCountLimit || owned >= 12 || m.explorerConflictLocked(s.Binding, s.Path) {
		m.mu.Unlock()
		cancel()
		jsonError(w, 409, "This document cannot be opened in the current workspace.")
		return
	}
	m.documents[s.UID] = grant
	m.registerLoadedLocked(s.UID)
	m.mu.Unlock()
	go func() {
		result, err := m.importDiskDocument(ctx, grant, disk)
		disk.mu.Lock()
		disk.loading = false
		if disk.closed {
			disk.mu.Unlock()
			if result != nil {
				result.Buffer.Close()
			}
			return
		}
		if err != nil {
			disk.err = err.Error()
		} else {
			disk.imported = result
		}
		disk.mu.Unlock()
		m.mu.Lock()
		if m.documents[s.UID] == grant && err == nil {
			grant.snapshot.Hash = result.Hash
			grant.snapshot.Encoding = result.Format.Encoding
			grant.snapshot.SavedEncoding = result.Format.Encoding
			grant.snapshot.BOM = result.Format.BOM
			grant.snapshot.SavedBOM = result.Format.BOM
			grant.snapshot.EOL = result.Format.EOL
			grant.snapshot.SavedEOL = result.Format.EOL
			grant.snapshot.EditRevision = result.Buffer.Metadata().Revision
		}
		m.mu.Unlock()
	}()
	jsonResponse(w, 202, map[string]any{"lease": grant.lease, "loading": true, "disk": true})
}
func (m *editorViewRegistry) importDiskDocument(ctx context.Context, g *editorDocumentGrant, d *editorDiskDocument) (*editorbuffer.Imported, error) {
	path := g.snapshot.Path
	cleanup := func() {}
	if g.snapshot.Binding.Kind != "local" && g.snapshot.Binding.Kind != "localDraft" {
		if g.valid == nil || !g.valid() {
			return nil, errEditorLease
		}
		dir, e := os.MkdirTemp("", "nt-editor-download-*")
		if e != nil {
			return nil, e
		}
		cleanup = func() { os.RemoveAll(dir) }
		defer cleanup()
		path = filepath.Join(dir, "source")
		if _, e = g.run(ctx, remoteBrowserRequest{Protocol: "sftp", Action: "download", Path: g.snapshot.Path, LocalPath: path}); e != nil {
			return nil, e
		}
	}
	info, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("text editor requires a regular file")
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	opened, e := f.Stat()
	if e != nil || !os.SameFile(info, opened) {
		return nil, errors.New("file changed while opening")
	}
	result, e := editorbuffer.Import(ctx, f, "", func(bytes int64) { d.mu.Lock(); d.progress = bytes; d.mu.Unlock() })
	if e != nil {
		return nil, e
	}
	after, e := f.Stat()
	if e != nil || after.Size() != opened.Size() || !after.ModTime().Equal(opened.ModTime()) {
		result.Buffer.Close()
		return nil, errors.New("file changed while loading; reopen it")
	}
	return result, nil
}

type editorDiskRequest struct {
	RequestID string              `json:"requestId"`
	Query     string              `json:"query"`
	Regex     bool                `json:"regex"`
	MatchCase bool                `json:"matchCase"`
	WholeWord bool                `json:"wholeWord"`
	Lease     uint64              `json:"lease"`
	Action    string              `json:"action"`
	Revision  uint64              `json:"revision"`
	Start     int64               `json:"start"`
	Line      *int64              `json:"line,omitempty"`
	Limit     int                 `json:"limit"`
	Align     bool                `json:"align"`
	From      int64               `json:"from"`
	To        int64               `json:"to"`
	Text      string              `json:"text"`
	Path      string              `json:"path"`
	Overwrite bool                `json:"overwrite"`
	Format    editorbuffer.Format `json:"format"`
}

func (m *editorViewRegistry) diskDocument(w http.ResponseWriter, r *http.Request, caller, uid string) {
	var req editorDiskRequest
	if e := decodeEditorBody(w, r, editorbuffer.MaxEditBytes*6+16384, &req); e != nil {
		jsonError(w, 400, e.Error())
		return
	}
	m.mu.Lock()
	g := m.documents[uid]
	if !m.callerLocked(caller) || g == nil || g.owner != caller || g.lease != req.Lease || g.pending != "" || g.buffer == nil {
		m.mu.Unlock()
		jsonError(w, 409, errEditorLease.Error())
		return
	}
	disk := g.buffer
	disk.mu.Lock()
	if disk.reloading || req.Action == "reload" && g.busy != 0 {
		disk.mu.Unlock()
		m.mu.Unlock()
		jsonError(w, 409, "Wait for the current document operation before reloading.")
		return
	}
	if req.Action == "reload" {
		disk.reloading = true
	}
	disk.mu.Unlock()
	g.busy++
	m.mu.Unlock()
	defer func() {
		if req.Action == "reload" {
			disk.mu.Lock()
			disk.reloading = false
			disk.mu.Unlock()
		}
		m.mu.Lock()
		g.busy--
		if g.orphan && g.busy == 0 {
			g.owner = ""
			g.lease++
			g.orphan = false
			m.recovery[uid] = true
		}
		m.mu.Unlock()
	}()
	disk.mu.Lock()
	if disk.closed {
		disk.mu.Unlock()
		jsonError(w, 410, "The document is closed.")
		return
	}
	loading, progress, failure, v := disk.loading, disk.progress, disk.err, disk.imported
	var format editorbuffer.Format
	var hash string
	if v != nil {
		format, hash = v.Format, v.Hash
	}
	disk.mu.Unlock()
	if req.Action == "cancel" {
		disk.cancel()
		jsonResponse(w, 200, map[string]any{"ok": true})
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	stop := context.AfterFunc(disk.ctx, cancel)
	defer stop()
	defer cancel()
	if failure != "" {
		jsonError(w, 400, failure)
		return
	}
	if loading || v == nil {
		jsonResponse(w, 202, map[string]any{"loading": true, "bytesRead": progress})
		return
	}
	var value any
	var err error
	switch req.Action {
	case "status":
		value = map[string]any{"loading": false, "metadata": v.Buffer.Metadata(), "format": format, "hash": hash}
	case "window":
		if req.Limit == 0 {
			req.Limit = 32768
		}
		if req.Line != nil {
			value, err = v.Buffer.WindowAtLine(*req.Line, req.Limit)
		} else if req.Align {
			value, err = v.Buffer.WindowNear(req.Start, req.Limit)
		} else {
			value, err = v.Buffer.Window(req.Start, req.Limit)
		}
	case "edit", "undo", "redo":
		value, err = disk.mutate(v.Buffer, req)
	case "save":
		value, err = m.saveDiskDocument(ctx, g, disk, req)
	case "find":
		var snapshot *editorbuffer.Snapshot
		snapshot, err = v.Buffer.Snapshot()
		if err == nil {
			value, err = snapshot.Find(ctx, req.Query, req.Start, req.Regex, req.MatchCase, req.WholeWord)
			snapshot.Close()
		}
	case "reload":
		var replacement *editorbuffer.Imported
		replacement, err = m.importDiskDocument(ctx, g, disk)
		if err == nil {
			disk.mu.Lock()
			old := disk.imported
			disk.imported = replacement
			disk.mu.Unlock()
			v = replacement
			disk.editMu.Lock()
			disk.receipts = nil
			disk.editMu.Unlock()
			old.Buffer.Close()
			m.mu.Lock()
			g.snapshot.Hash = v.Hash
			g.snapshot.Encoding = v.Format.Encoding
			g.snapshot.SavedEncoding = v.Format.Encoding
			g.snapshot.BOM = v.Format.BOM
			g.snapshot.SavedBOM = v.Format.BOM
			g.snapshot.EOL = v.Format.EOL
			g.snapshot.SavedEOL = v.Format.EOL
			m.mu.Unlock()
			value = map[string]any{"metadata": v.Buffer.Metadata(), "format": v.Format, "hash": v.Hash}
		}
	default:
		jsonError(w, 400, "Unsupported disk-backed editor action.")
		return
	}
	if err != nil {
		jsonError(w, 409, err.Error())
		return
	}
	meta := v.Buffer.Metadata()
	m.mu.Lock()
	g.snapshot.Dirty = meta.Dirty
	g.snapshot.EditRevision = meta.Revision
	m.mu.Unlock()
	jsonResponse(w, 200, value)
}
func hashDiskFile(ctx context.Context, path string) (string, os.FileMode, error) {
	info, e := os.Lstat(path)
	if e != nil {
		return "", 0, e
	}
	if !info.Mode().IsRegular() {
		return "", 0, errors.New("text editor requires a regular destination")
	}
	f, e := os.Open(path)
	if e != nil {
		return "", 0, e
	}
	defer f.Close()
	opened, e := f.Stat()
	if e != nil || !os.SameFile(info, opened) {
		return "", 0, errors.New("destination changed while reading")
	}
	hash := sha256.New()
	data := make([]byte, editorbuffer.ChunkBytes)
	for {
		if e = ctx.Err(); e != nil {
			return "", 0, e
		}
		n, readErr := f.Read(data)
		hash.Write(data[:n])
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", 0, readErr
		}
	}
	after, e := f.Stat()
	if e != nil || after.Size() != opened.Size() || !after.ModTime().Equal(opened.ModTime()) {
		return "", 0, errors.New("destination changed while reading")
	}
	return hex.EncodeToString(hash.Sum(nil)), info.Mode().Perm(), nil
}
func (m *editorViewRegistry) saveDiskDocument(ctx context.Context, g *editorDocumentGrant, d *editorDiskDocument, req editorDiskRequest) (any, error) {
	d.mu.Lock()
	if d.saving {
		d.mu.Unlock()
		return nil, errors.New("wait for the current save")
	}
	d.saving = true
	v := d.imported
	d.mu.Unlock()
	defer func() { d.mu.Lock(); d.saving = false; d.mu.Unlock() }()
	if req.Revision != v.Buffer.Metadata().Revision {
		return nil, editorbuffer.ErrRevision
	}
	m.mu.Lock()
	binding, path := g.snapshot.Binding, g.snapshot.Path
	m.mu.Unlock()
	if binding.Kind != "local" && binding.Kind != "localDraft" {
		return m.saveRemoteDiskDocument(ctx, g, d, v, path, req)
	}
	destination := path
	if req.Path != "" {
		var e error
		destination, e = absoluteExplorerPath(req.Path)
		if e != nil {
			return nil, e
		}
	}
	m.mu.Lock()
	if m.explorerConflictLocked(binding, destination) {
		m.mu.Unlock()
		return nil, errors.New("a file operation is changing this path")
	}
	g.filePath = destination
	m.mu.Unlock()
	defer func() { m.mu.Lock(); g.filePath = ""; m.mu.Unlock() }()
	expected := v.Hash
	if destination != path {
		expected = ""
	}
	old, mode, e := hashDiskFile(ctx, destination)
	exists := e == nil
	if exists {
		if expected != "" && old != expected {
			return nil, errors.New("file changed after opening; reload or save a new copy")
		}
		if expected == "" && !req.Overwrite {
			return nil, errors.New("destination exists; confirm replacement in Save As")
		}
	} else if os.IsNotExist(e) {
		if expected != "" {
			return nil, errors.New("file was removed; save a new copy")
		}
		mode = 0600
	} else {
		return nil, e
	}
	s, e := v.Buffer.Snapshot()
	if e != nil {
		return nil, e
	}
	defer s.Close()
	if s.Revision != req.Revision {
		return nil, editorbuffer.ErrRevision
	}
	f, e := os.CreateTemp(filepath.Dir(destination), ".network-toolkit-edit-*")
	if e != nil {
		return nil, e
	}
	temp := f.Name()
	defer os.Remove(temp)
	hash := sha256.New()
	if e = f.Chmod(mode); e == nil {
		e = s.Encode(ctx, io.MultiWriter(f, hash), req.Format)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return nil, e
	}
	current, _, checkErr := hashDiskFile(ctx, destination)
	if exists && (checkErr != nil || current != old) || !exists && !os.IsNotExist(checkErr) {
		return nil, errors.New("destination changed during save; existing file was not replaced")
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	if e = os.Rename(temp, destination); e != nil {
		return nil, e
	}
	newHash := hex.EncodeToString(hash.Sum(nil))
	meta := s.MarkSaved()
	d.mu.Lock()
	v.Hash = newHash
	v.Format = req.Format
	d.mu.Unlock()
	m.mu.Lock()
	g.snapshot.Path = destination
	g.snapshot.Hash = newHash
	g.snapshot.Encoding = req.Format.Encoding
	g.snapshot.BOM = req.Format.BOM
	g.snapshot.EOL = req.Format.EOL
	g.snapshot.SavedEncoding = req.Format.Encoding
	g.snapshot.SavedBOM = req.Format.BOM
	g.snapshot.SavedEOL = req.Format.EOL
	m.mu.Unlock()
	return map[string]any{"metadata": meta, "hash": newHash, "path": destination}, nil
}

func (m *editorViewRegistry) saveRemoteDiskDocument(ctx context.Context, g *editorDocumentGrant, d *editorDiskDocument, v *editorbuffer.Imported, path string, req editorDiskRequest) (any, error) {
	if g.valid == nil || !g.valid() {
		return nil, errEditorLease
	}
	destination := path
	if req.Path != "" {
		destination = req.Path
	}
	if !validEditorText(destination, 4096) || destination == "" {
		return nil, errors.New("invalid remote destination")
	}
	m.mu.Lock()
	if m.explorerConflictLocked(g.snapshot.Binding, destination) {
		m.mu.Unlock()
		return nil, errors.New("a file operation is changing this path")
	}
	g.filePath = destination
	m.mu.Unlock()
	defer func() { m.mu.Lock(); g.filePath = ""; m.mu.Unlock() }()
	expected := v.Hash
	if destination != path {
		expected = ""
	}
	dir, e := os.MkdirTemp("", "nt-editor-save-*")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(dir)
	s, e := v.Buffer.Snapshot()
	if e != nil {
		return nil, e
	}
	defer s.Close()
	if s.Revision != req.Revision {
		return nil, editorbuffer.ErrRevision
	}
	temp := filepath.Join(dir, "updated")
	f, e := os.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return nil, e
	}
	hash := sha256.New()
	e = s.Encode(ctx, io.MultiWriter(f, hash), req.Format)
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return nil, e
	}
	// Download and hash the current remote file only when replacing the file
	// that was opened. Save As uses the staged uploader's no-overwrite default.
	if expected != "" {
		old := filepath.Join(dir, "current")
		if !g.valid() {
			return nil, errEditorLease
		}
		if _, e = g.run(ctx, remoteBrowserRequest{Protocol: "sftp", Action: "download", Path: destination, LocalPath: old}); e != nil {
			return nil, e
		}
		actual, _, e := hashDiskFile(ctx, old)
		if e != nil {
			return nil, e
		}
		if actual != expected {
			return nil, errors.New("remote file changed after opening; reload or save a new copy")
		}
	}
	if !g.valid() {
		return nil, errEditorLease
	}
	if _, e = g.run(ctx, remoteBrowserRequest{Protocol: "sftp", Action: "upload", Path: destination, LocalPath: temp, Overwrite: expected != "" || req.Overwrite, StagedUpload: true}); e != nil {
		return nil, e
	}
	newHash := hex.EncodeToString(hash.Sum(nil))
	meta := s.MarkSaved()
	d.mu.Lock()
	v.Hash = newHash
	v.Format = req.Format
	d.mu.Unlock()
	m.mu.Lock()
	g.snapshot.Path = destination
	g.snapshot.Hash = newHash
	g.snapshot.Encoding = req.Format.Encoding
	g.snapshot.BOM = req.Format.BOM
	g.snapshot.EOL = req.Format.EOL
	g.snapshot.SavedEncoding = req.Format.Encoding
	g.snapshot.SavedBOM = req.Format.BOM
	g.snapshot.SavedEOL = req.Format.EOL
	m.mu.Unlock()
	return map[string]any{"metadata": meta, "hash": newHash, "path": destination}, nil
}
