package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"networktoolkit/internal/editorbuffer"
	"networktoolkit/internal/texttools"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
)

// This state stays with the existing document grant across window moves. Only
// the owning lease may reach it; checkpoints contain no source text or history.
type editorDiskDocument struct {
	editMu         sync.Mutex
	receipts       []editorDiskReceipt
	mu             sync.Mutex
	imported       *editorbuffer.Imported
	progress       int64
	err            string
	loading        bool
	cancel         context.CancelFunc
	closed         bool
	ctx            context.Context
	saving         bool
	reloading      bool
	upload         *editorDiskUpload
	uploadSize     int64
	reloadUpload   bool
	reloadRevision uint64
	reloadError    string
	uploadCancel   context.CancelFunc
	mode           string
	bulk           *editorDiskEdit
	exported       *editorDiskExport
	verify         *editorDiskVerify
	ftp            *editorFTPEndpoint
	restored       *editorSessionRestoreState
	// Replace Alls and text tools that still read this document's buffer,
	// each with the way to stop it.
	replacing map[*replaceAllContext]struct{}
	// tool is the last text tool started on this document (editor_text_tools.go).
	tool *editorToolJob
	// derived counts what a derived document's tool wrote (settings of a
	// readable SonicWall export), for the page's message.
	derived int64
}

// replaceAllContext is the request's context for one Replace All, with a way
// for the document to stop it. The search only asks Err.
type replaceAllContext struct {
	context.Context
	stopped atomic.Bool
}

func (c *replaceAllContext) Err() error {
	if c.stopped.Load() {
		return context.Canceled
	}
	return c.Context.Err()
}

// stopReplaceAll stops every Replace All and text tool that is still reading
// this document. Whoever replaces or closes the buffer calls it first: the
// work holds the old buffer open, and its result must not be applied.
func (d *editorDiskDocument) stopReplaceAll() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for search := range d.replacing {
		search.stopped.Store(true)
	}
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
	if d.bulk != nil {
		return editorbuffer.Metadata{}, errors.New("finish or cancel the pending bulk edit")
	}
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
		d.rememberMutationLocked(editorDiskReceipt{req.RequestID, digest, metadata})
	}
	return metadata, err
}

func (d *editorDiskDocument) rememberMutationLocked(receipt editorDiskReceipt) {
	d.receipts = append(d.receipts, receipt)
	if len(d.receipts) > 64 {
		copy(d.receipts, d.receipts[len(d.receipts)-64:])
		d.receipts = d.receipts[:64]
	}
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
	g.buffer.editMu.Lock()
	defer g.buffer.editMu.Unlock()
	if g.buffer.bulk != nil {
		return false
	}
	return !g.buffer.closed && !g.buffer.loading && g.buffer.imported != nil && s.Path == g.snapshot.Path && s.EditRevision == g.buffer.imported.Buffer.Metadata().Revision
}

func (d *editorDiskDocument) close() {
	d.stopReplaceAll()
	d.mu.Lock()
	d.closed = true
	d.cancel()
	b := d.imported
	d.imported = nil
	upload := d.upload
	d.upload = nil
	d.mu.Unlock()
	d.editMu.Lock()
	d.clearBulkLocked()
	d.editMu.Unlock()
	if upload != nil {
		upload.close()
	}
	if b != nil {
		b.Buffer.Close()
	}
}
func (m *editorViewRegistry) openDiskDocument(w http.ResponseWriter, r *http.Request, caller string) {
	var req struct {
		Document editorDocumentSnapshot `json:"document"`
		Mode     string                 `json:"mode"`
		Remote   *remoteBrowserRequest  `json:"remote"`
		Restore  *editorSessionRestore  `json:"restore,omitempty"`
		Derive   *editorDeriveRequest   `json:"derive,omitempty"`
	}
	if e := decodeEditorBody(w, r, 64<<10, &req); e != nil {
		jsonError(w, 400, e.Error())
		return
	}
	s := req.Document
	// A session restore imports a kept copy of unsaved text; only the main
	// workspace (or the standalone Text Editor) reopens its own session.
	var backup *os.File
	var expected editorSessionBackup
	defer func() {
		if backup != nil {
			backup.Close()
		}
	}()
	if (req.Mode == "restore") != (req.Restore != nil) {
		jsonError(w, 400, "A kept copy is restored only in restore mode.")
		return
	}
	if req.Mode == "restore" {
		if caller != "" {
			jsonError(w, 403, "Only the main editor workspace reopens its last session.")
			return
		}
		if e := req.Restore.validate(s.Binding.Kind); e != nil {
			jsonError(w, 400, e.Error())
			return
		}
		file, entry, e := m.app.openEditorSessionBackup(req.Restore.Backup)
		if e != nil {
			jsonError(w, 404, e.Error())
			return
		}
		backup, expected = file, entry
	}
	if s.Disk || s.Content != "" || s.Original != "" || s.Saved != "" || s.Dirty || len(s.Undo) > 0 || len(s.Redo) > 0 {
		jsonError(w, 400, "Open disk-backed files from an empty document identity.")
		return
	}
	validation := s
	if s.Binding.Kind == "ftp" {
		// FTP is admitted only through this authenticated file opening path.
		validation.Disk = true
	}
	if e := validateEditorBatch(editorBatch{Documents: []editorDocumentSnapshot{validation}, Active: s.UID}); e != nil {
		jsonError(w, 400, e.Error())
		return
	}
	var grant *editorDocumentGrant
	var ftp *editorFTPEndpoint
	if s.Binding.Kind == "ftp" {
		if caller != "" {
			jsonError(w, 403, "FTP file editing requires the main workspace.")
			return
		}
		_, endpoint, err := diskFTPConnection(req.Remote)
		if err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		ftp = &endpoint
		grant = &editorDocumentGrant{snapshot: s, owner: caller, lease: 1, bytes: editorSnapshotBytes(s)}
	} else {
		var err error
		grant, err = m.makeGrant(caller, s)
		if err != nil {
			jsonError(w, 403, err.Error())
			return
		}
	}
	if (req.Mode == "derive") != (req.Derive != nil) || req.Mode == "derive" && s.Binding.Kind != "localDraft" {
		jsonError(w, 400, "A derived document is a new untitled document made from an open one.")
		return
	}
	if req.Mode != "" && req.Mode != "draft" && req.Mode != "upload" && req.Mode != "restore" && req.Mode != "derive" {
		jsonError(w, 400, "Unsupported file opening mode.")
		return
	}
	if req.Mode == "upload" && (s.Binding.Kind != "localDraft" || filepath.Base(s.Path) != s.Path) {
		jsonError(w, 400, "Choose a file name for the browser import.")
		return
	}
	if s.Binding.Kind == "local" || s.Binding.Kind == "localDraft" && req.Mode == "" {
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
	disk := &editorDiskDocument{loading: true, ctx: ctx, cancel: cancel, mode: req.Mode, ftp: ftp}
	if req.Mode == "upload" {
		f, err := os.CreateTemp("", "nt-editor-import-*")
		if err != nil {
			cancel()
			jsonError(w, 500, "Could not prepare browser file import: "+err.Error())
			return
		}
		disk.upload = &editorDiskUpload{file: f}
	}
	grant.buffer = disk
	grant.snapshot.Disk = true
	m.mu.Lock()
	owned := 0
	for _, d := range m.documents {
		if d.owner == caller {
			owned++
		}
	}
	if !m.callerLocked(caller) || r.Context().Err() != nil || m.documents[s.UID] != nil || len(m.documents) >= editorDocumentCountLimit || owned >= 12 || m.explorerConflictLocked(s.Binding, s.Path) {
		m.mu.Unlock()
		disk.close()
		jsonError(w, 409, "This document cannot be opened in the current workspace.")
		return
	}
	var source *editorDiskDocument
	if req.Mode == "derive" {
		// The source must be a loaded document of the same window, under
		// the lease the page holds.
		if from := m.documents[req.Derive.UID]; from != nil && from.owner == caller && from.lease == req.Derive.Lease && from.pending == "" {
			source = from.buffer
		}
		if source == nil {
			m.mu.Unlock()
			disk.close()
			jsonError(w, 409, "The document to make a copy of is not open in this window.")
			return
		}
	}
	m.documents[s.UID] = grant
	m.registerLoadedLocked(s.UID)
	lease := grant.lease
	m.mu.Unlock()
	if req.Mode == "draft" {
		result, err := editorbuffer.Import(ctx, strings.NewReader(""), "", nil)
		if result != nil {
			result.Hash = ""
			result.Format = editorbuffer.Format{Encoding: s.Encoding, BOM: s.BOM, EOL: s.EOL}
		}
		m.finishDiskOpen(grant, disk, result, err)
	} else if req.Mode == "derive" {
		derive := *req.Derive
		// Registered with the source before this open is answered: closing
		// or reloading the source right after it stops the copy.
		stop, unregister := source.registerStop(ctx)
		go func() {
			defer unregister()
			result, err := m.deriveDiskDocument(stop, disk, source, derive)
			m.finishDiskOpen(grant, disk, result, err)
		}()
	} else if req.Mode == "restore" {
		file, restore := backup, *req.Restore
		backup = nil
		go func() {
			result, err := m.importSessionBackup(ctx, grant, disk, file, expected, restore)
			m.finishDiskOpen(grant, disk, result, err)
		}()
	} else if req.Mode != "upload" {
		go func() {
			result, err := m.importDiskDocument(ctx, grant, disk, req.Remote)
			m.finishDiskOpen(grant, disk, result, err)
		}()
	}
	jsonResponse(w, 202, map[string]any{"lease": lease, "loading": true, "disk": true})
}

func (m *editorViewRegistry) finishDiskOpen(g *editorDocumentGrant, d *editorDiskDocument, result *editorbuffer.Imported, err error) {
	m.mu.Lock()
	d.mu.Lock()
	defer m.mu.Unlock()
	defer d.mu.Unlock()
	if d.closed {
		if result != nil {
			result.Buffer.Close()
		}
		return
	}
	d.loading = false
	if err != nil {
		d.err = err.Error()
		return
	}
	d.imported = result
	g.snapshot.Hash = result.Hash
	g.snapshot.Encoding, g.snapshot.SavedEncoding = result.Format.Encoding, result.Format.Encoding
	g.snapshot.BOM, g.snapshot.SavedBOM = result.Format.BOM, result.Format.BOM
	g.snapshot.EOL, g.snapshot.SavedEOL = result.Format.EOL, result.Format.EOL
	g.snapshot.EditRevision = result.Buffer.Metadata().Revision
}

func (m *editorViewRegistry) importDiskDocument(ctx context.Context, g *editorDocumentGrant, d *editorDiskDocument, details *remoteBrowserRequest) (*editorbuffer.Imported, error) {
	m.mu.Lock()
	path, binding := g.snapshot.Path, g.snapshot.Binding.Kind
	m.mu.Unlock()
	if binding != "local" && binding != "localDraft" {
		dir, e := os.MkdirTemp("", "nt-editor-download-*")
		if e != nil {
			return nil, e
		}
		defer os.RemoveAll(dir)
		source := path
		path = filepath.Join(dir, "source")
		if _, e = m.diskRemoteOperation(ctx, g, d, details, remoteBrowserRequest{Protocol: "sftp", Action: "download", Path: source, LocalPath: path}); e != nil {
			return nil, e
		}
	} else if !filepath.IsAbs(path) {
		return nil, errors.New("this document has no saved local path; open the source file again to reload it")
	}
	info, e := os.Lstat(path)
	if e != nil {
		return nil, editorOpenError(e)
	}
	if info.IsDir() {
		return nil, errors.New("This is a folder, not a file.")
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("text editor requires a regular file")
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, editorOpenError(e)
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

// editorOpenError says in plain words why a file could not be opened. Nothing
// of the file has been read when it is returned, so no document is shown.
func editorOpenError(e error) error {
	var code syscall.Errno
	switch {
	case errors.Is(e, fs.ErrNotExist):
		return errors.New("The file does not exist.")
	case runtime.GOOS == "windows" && errors.As(e, &code) && (code == 32 || code == 33):
		// ERROR_SHARING_VIOLATION, ERROR_LOCK_VIOLATION
		return errors.New("The file is in use by another program and could not be read.")
	case errors.Is(e, fs.ErrPermission):
		return errors.New("The file could not be read: access was denied.")
	}
	return e
}

type editorDiskRequest struct {
	RequestID string                `json:"requestId"`
	Query     string                `json:"query"`
	Regex     bool                  `json:"regex"`
	MatchCase bool                  `json:"matchCase"`
	WholeWord bool                  `json:"wholeWord"`
	Lease     uint64                `json:"lease"`
	Action    string                `json:"action"`
	Revision  uint64                `json:"revision"`
	Start     int64                 `json:"start"`
	Line      *int64                `json:"line,omitempty"`
	Limit     int                   `json:"limit"`
	Align     bool                  `json:"align"`
	From      int64                 `json:"from"`
	To        int64                 `json:"to"`
	Text      string                `json:"text"`
	Path      string                `json:"path"`
	Overwrite bool                  `json:"overwrite"`
	Format    editorbuffer.Format   `json:"format"`
	Offset    int64                 `json:"offset"`
	Size      int64                 `json:"size"`
	Data      []byte                `json:"data"`
	Backwards bool                  `json:"backwards"`
	Replace   string                `json:"replacement"`
	Receipt   string                `json:"receipt"`
	Range     *editorDiskRange      `json:"range"`
	Remote    *remoteBrowserRequest `json:"remote"`
	// Text tools (editor_text_tools.go).
	Tool    string            `json:"tool"`
	Options texttools.Options `json:"options"`
	Job     string            `json:"job"`
	// The Find window (editor_find.go): its search, the searched range
	// ("In selection"; nil is the whole document), the position Find Next
	// starts at, wrap-around, the kind of a background search and the
	// separator of Copy Marked Text.
	Search    *editorbuffer.Search `json:"search,omitempty"`
	Within    *[2]int64            `json:"within,omitempty"`
	At        int64                `json:"at"`
	Wrap      bool                 `json:"wrap"`
	Kind      string               `json:"kind"`
	Separator string               `json:"separator"`
	// Marks are the searches of Copy Marked Text (editor_find.go).
	Marks []editorbuffer.MarkSearch `json:"marks,omitempty"`
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
	path := g.snapshot.Path
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
	if req.Action == "import-chunk" || req.Action == "import-finish" || req.Action == "reload-start" {
		m.diskUpload(w, g, disk, req)
		return
	}
	disk.mu.Lock()
	if disk.closed {
		disk.mu.Unlock()
		jsonError(w, 410, "The document is closed.")
		return
	}
	loading, progress, failure, v, reloadError, restored, derived := disk.loading, disk.progress, disk.err, disk.imported, disk.reloadError, disk.restored, disk.derived
	if req.Action == "cancel" && disk.reloadUpload {
		if disk.upload != nil {
			disk.upload.close()
			disk.upload = nil
			disk.reloadUpload = false
			disk.reloadError = "Browser file reload cancelled."
		} else if disk.uploadCancel != nil {
			disk.uploadCancel()
		}
		disk.mu.Unlock()
		jsonResponse(w, 200, map[string]any{"ok": true})
		return
	}
	if req.Action == "cancel" && loading {
		disk.cancel()
		if disk.upload != nil {
			disk.upload.close()
			disk.upload = nil
			disk.loading = false
			disk.err = "Browser file import cancelled."
		}
	}
	var format editorbuffer.Format
	var hash string
	if v != nil {
		format, hash = v.Format, v.Hash
	}
	disk.mu.Unlock()
	if req.Action == "cancel" {
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
	if v.Binary {
		// A file that is not text is shown, searched and reloaded. Nothing
		// changes it and nothing writes it, whatever the page asks for.
		// (Reading it again from a browser file handle, "reload-start",
		// "import-chunk", "import-finish" and "cancel", is answered above.)
		switch req.Action {
		case "status", "window", "find", "find-matches", "find-start", "reload":
			// find-start refuses Replace All (editor_find.go).
		default:
			jsonError(w, 409, editorbuffer.ErrBinary.Error())
			return
		}
	}
	var value any
	var err error
	switch req.Action {
	case "status":
		status := map[string]any{"loading": false, "metadata": v.Buffer.Metadata(), "format": format, "hash": hash, "path": path, "reloadError": reloadError}
		if ending := v.Buffer.NewLineEOL(); ending != "" {
			status["newLineEol"] = ending
		}
		if restored != nil {
			status["restore"] = restored
		}
		if derived > 0 {
			status["derived"] = derived
		}
		value = status
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
	case "edit-begin", "edit-chunk", "edit-commit", "edit-abort":
		value, err = disk.bulkEdit(v.Buffer, req)
	case "save":
		value, err = m.saveDiskDocument(ctx, g, disk, req)
	case "export":
		disk.export(ctx, w, v.Buffer, req)
		return
	case "mark-saved":
		value, err = disk.markExportSaved(ctx, v, req.Receipt)
	case "verify-start", "verify-chunk", "verify-finish":
		value, err = disk.verifyBrowserSource(req)
	case "find", "find-matches", "find-start":
		value, err = m.findDisk(ctx, caller, disk, v, req)
	case "replace-all":
		value, err = disk.replaceDiskMatches(ctx, v.Buffer, req)
	case "replace-text":
		value, err = replaceText(ctx, v.Buffer, req)
	case "tool-start":
		value, err = disk.startTool(v, req)
	case "tool-status", "tool-cancel":
		value, err = disk.toolStatus(req, req.Action == "tool-cancel")
	case "tool-detect":
		var head editorbuffer.Window
		if head, err = v.Buffer.Window(0, int(min(detectPrefix, max(1, v.Buffer.Metadata().Units)))); err == nil {
			value = detectTool(head.Text)
		}
	case "reload":
		var replacement *editorbuffer.Imported
		if g.snapshot.Untitled && g.snapshot.Binding.Kind == "localDraft" && !filepath.IsAbs(g.snapshot.Path) {
			replacement, err = editorbuffer.Import(ctx, strings.NewReader(""), "", nil)
		} else {
			replacement, err = m.importDiskDocument(ctx, g, disk, req.Remote)
		}
		if err == nil {
			disk.mu.Lock()
			old := disk.imported
			disk.imported = replacement
			disk.exported = nil
			disk.mu.Unlock()
			v = replacement
			disk.editMu.Lock()
			disk.clearBulkLocked()
			disk.receipts = nil
			disk.editMu.Unlock()
			m.cancelComparisonsUsing(uid)
			disk.stopReplaceAll()
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
			value = map[string]any{"metadata": v.Buffer.Metadata(), "format": v.Format, "hash": v.Hash, "newLineEol": v.Buffer.NewLineEOL()}
		}
	default:
		jsonError(w, 400, "Unsupported disk-backed editor action.")
		return
	}
	if err != nil {
		jsonError(w, 409, err.Error())
		return
	}
	m.mu.Lock()
	meta := v.Buffer.Metadata()
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
	if v.Binary {
		// Before any temporary file or remote transfer: nothing is written.
		return nil, editorbuffer.ErrBinary
	}
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
		destination = req.Path
	}
	destination, e := absoluteExplorerPath(destination)
	if e != nil {
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
	if exists {
		e = replaceSettingsFile(temp, destination)
	} else {
		e = moveUpdateNamingNoReplace(temp, destination)
	}
	if e != nil {
		return nil, e
	}
	newHash := hex.EncodeToString(hash.Sum(nil))
	meta := s.MarkSaved()
	d.mu.Lock()
	v.Hash = newHash
	v.Format = req.Format
	d.exported = nil
	d.verify = nil
	if d.mode == "upload" {
		d.mode = ""
	}
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

// remoteSaveDestination is where a remote disk-backed save writes: the opened
// path, or a Save As path. A file a detached Terminal workspace opened in the
// main window is saved only to itself, and only while the workspace still
// holds the connection.
func remoteSaveDestination(g *editorDocumentGrant, path, requested string) (string, error) {
	destination := path
	if requested != "" {
		destination = requested
	}
	if g != nil && g.fixedPath != "" {
		if destination != g.fixedPath {
			return "", errWindowSendSaveElsewhere
		}
		if g.valid == nil || !g.valid() {
			return "", errWindowSendGone
		}
	}
	if !validEditorText(destination, 4096) || destination == "" {
		return "", errors.New("invalid remote destination")
	}
	return destination, nil
}

func (m *editorViewRegistry) saveRemoteDiskDocument(ctx context.Context, g *editorDocumentGrant, d *editorDiskDocument, v *editorbuffer.Imported, path string, req editorDiskRequest) (any, error) {
	if d.ftp == nil && g.fixedPath == "" && (g.valid == nil || !g.valid()) {
		return nil, errEditorLease
	}
	destination, e := remoteSaveDestination(g, path, req.Path)
	if e != nil {
		return nil, e
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
		if _, e = m.diskRemoteOperation(ctx, g, d, req.Remote, remoteBrowserRequest{Protocol: "sftp", Action: "download", Path: destination, LocalPath: old}); e != nil {
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
	if _, e = m.diskRemoteOperation(ctx, g, d, req.Remote, remoteBrowserRequest{Protocol: "sftp", Action: "upload", Path: destination, LocalPath: temp, Overwrite: expected != "" || req.Overwrite, StagedUpload: true}); e != nil {
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
