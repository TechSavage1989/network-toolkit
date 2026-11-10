package main

// Notepad-style Text Editor session restore. Each primary workspace (the main
// window and the standalone Text Editor) keeps one session folder beside its
// settings: a small metadata index plus one kept copy per document with
// unsaved text. The index never holds document text, bindings, connection
// identities or credentials. Kept copies have no size ceiling: disk-backed
// documents are streamed by this process from a stable buffer snapshot, so
// their text never passes through the renderer.
import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"networktoolkit/internal/editorbuffer"
)

const (
	editorSessionEndpoint   = "/api/editor-session"
	editorSessionDirectory  = "Network_Toolkit.editor-session"
	editorSessionIndexName  = "session.json"
	editorSessionIndexLimit = 1 << 20
	editorSessionTabLimit   = 64
	// A kept copy written by this process is protected from cleanup until an
	// index refers to it or this much time has passed.
	editorSessionGrace = 10 * time.Minute
	// Kept copies of memory documents are read back as text only when the
	// renderer has no disk-backed editing; the decoded limit matches it.
	editorSessionTextLimit = 3*editorDocumentLimit + 3
)

var (
	editorSessionBackupName = regexp.MustCompile(`^backup-([0-9a-f]{32})\.txt$`)
	editorSessionTempName   = regexp.MustCompile(`^\.backup-[0-9a-f]{32}\.tmp$`)
	editorSessionAsideName  = regexp.MustCompile(`^session\.unreadable-[0-9]+\.json$`)
	editorSessionLanguage   = regexp.MustCompile(`^[A-Za-z0-9._+-]{0,64}$`)
	// Every kept copy starts with a UTF-8 BOM so a document that itself begins
	// with U+FEFF keeps that character when the copy is imported again.
	editorSessionBOM         = []byte{0xef, 0xbb, 0xbf}
	errEditorSessionInvalid  = errors.New("The saved editor session is invalid.")
	errEditorSessionNoBackup = errors.New("The kept copy of this document is no longer available.")
)

type editorSessionBackup struct {
	ID     string `json:"id"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// A tab record. "local" tabs reopen their file; dirty local tabs also name the
// kept copy, the file digest when it was loaded or last saved, and both the
// current and the file's original format. "untitled" and "copy" tabs (unsaved
// text of a remote or browser-handle document) always name a kept copy.
type editorSessionTab struct {
	Kind        string               `json:"kind"`
	Path        string               `json:"path,omitempty"`
	Name        string               `json:"name,omitempty"`
	Origin      string               `json:"origin,omitempty"`
	Language    string               `json:"language,omitempty"`
	Dirty       bool                 `json:"dirty"`
	Hash        string               `json:"hash,omitempty"`
	Format      *editorbuffer.Format `json:"format,omitempty"`
	SavedFormat *editorbuffer.Format `json:"savedFormat,omitempty"`
	Backup      *editorSessionBackup `json:"backup,omitempty"`
	// Exists is reported by GET for local tabs; it is never stored.
	Exists *bool `json:"exists,omitempty"`
}

type editorSessionIndex struct {
	Version int                `json:"version"`
	Active  int                `json:"active"`
	Tabs    []editorSessionTab `json:"tabs"`
}

type editorSessionRestore struct {
	Backup string              `json:"backup"`
	Hash   string              `json:"hash"`
	Format editorbuffer.Format `json:"format"`
}

type editorSessionRestoreState struct {
	Changed bool `json:"changed,omitempty"`
	Missing bool `json:"missing,omitempty"`
}

type editorSessionStore struct {
	mu sync.Mutex
	// Kept copies written by this process that no saved index refers to yet.
	pending map[string]time.Time
	// Temporary files that are being written now.
	writing map[string]bool
	// Copies this process wrote or saw in a readable index.
	known map[string]bool
	// An unreadable index was set aside: only known copies may be removed.
	unreadable bool
}

func (s *editorSessionStore) initLocked() {
	if s.pending == nil {
		s.pending, s.writing, s.known = map[string]time.Time{}, map[string]bool{}, map[string]bool{}
	}
}

func emptyEditorSession() editorSessionIndex {
	return editorSessionIndex{Version: 1, Active: -1, Tabs: []editorSessionTab{}}
}

func validEditorSessionFormat(f *editorbuffer.Format) bool {
	return f != nil && validEditorDiskFormat(*f)
}

func validEditorDiskFormat(f editorbuffer.Format) bool {
	return (f.Encoding == "UTF-8" || f.Encoding == "UTF-16LE" || f.Encoding == "UTF-16BE") && (f.EOL == "LF" || f.EOL == "CRLF" || f.EOL == "CR")
}

func validEditorSessionLabel(s string, limit int) bool {
	if s == "" || !validEditorText(s, limit) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func validateEditorSession(index editorSessionIndex) error {
	if index.Version != 1 {
		return errors.New("This editor session version is not supported.")
	}
	if len(index.Tabs) > editorSessionTabLimit {
		return fmt.Errorf("An editor session can hold at most %d tabs.", editorSessionTabLimit)
	}
	if index.Active < -1 || index.Active >= len(index.Tabs) {
		return errors.New("The active editor session tab is invalid.")
	}
	backups, paths := map[string]bool{}, map[string]bool{}
	for i, tab := range index.Tabs {
		fail := func(reason string) error { return fmt.Errorf("Editor session tab %d: %s", i+1, reason) }
		if tab.Exists != nil {
			return fail("file status is reported, never stored.")
		}
		if !editorSessionLanguage.MatchString(tab.Language) {
			return fail("invalid language.")
		}
		switch tab.Kind {
		case "local":
			if tab.Name != "" || tab.Origin != "" || !validEditorSessionLabel(tab.Path, 4096) || !filepath.IsAbs(tab.Path) || paths[tab.Path] {
				return fail("a local tab needs one absolute file path.")
			}
			paths[tab.Path] = true
			if !tab.Dirty {
				if tab.Hash != "" || tab.Backup != nil || tab.Format != nil || tab.SavedFormat != nil {
					return fail("a saved local tab keeps no copy.")
				}
				continue
			}
			if tab.Hash != "" && !editorHash.MatchString(tab.Hash) {
				return fail("invalid file revision.")
			}
			if !validEditorSessionFormat(tab.Format) || !validEditorSessionFormat(tab.SavedFormat) {
				return fail("unsupported encoding or line ending.")
			}
		case "untitled", "copy":
			if tab.Path != "" || !validEditorSessionLabel(tab.Name, 255) || !tab.Dirty || tab.Hash != "" || tab.SavedFormat != nil || !validEditorSessionFormat(tab.Format) {
				return fail("an unsaved document needs a name, its format and a kept copy.")
			}
			if (tab.Kind == "copy") != (tab.Origin != "") || tab.Origin != "" && !validEditorSessionLabel(tab.Origin, 1024) {
				return fail("only a copy names where it came from.")
			}
		default:
			return fail("unknown tab kind.")
		}
		b := tab.Backup
		if b == nil || !editorHexID.MatchString(b.ID) || b.Size < int64(len(editorSessionBOM)) || !editorHash.MatchString(b.SHA256) || backups[b.ID] {
			return fail("invalid kept copy.")
		}
		backups[b.ID] = true
	}
	return nil
}

func editorSessionBackupFile(id string) string { return "backup-" + id + ".txt" }

// editorSessionRoot opens this workspace's session folder through the anchored
// app-data API, which refuses links and reparse points.
func (a *application) editorSessionRoot(create bool) (*storageRoot, error) {
	workspace, err := openStorageRoot(filepath.Dir(a.dataPath), create)
	if err != nil {
		return nil, err
	}
	defer workspace.Close()
	if create {
		if err := workspace.Mkdir(editorSessionDirectory, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	return openStorageChild(workspace, editorSessionDirectory)
}

func readEditorSessionIndex(root *storageRoot) (editorSessionIndex, error) {
	raw, err := readStorageFile(root, editorSessionIndexName, editorSessionIndexLimit)
	if errors.Is(err, os.ErrNotExist) {
		return emptyEditorSession(), nil
	}
	if err != nil {
		return emptyEditorSession(), err
	}
	var index editorSessionIndex
	if err := decodeStorageJSON(raw, &index); err != nil {
		return emptyEditorSession(), fmt.Errorf("%w %v", errEditorSessionInvalid, err)
	}
	if index.Tabs == nil {
		index.Tabs = []editorSessionTab{}
	}
	if err := validateEditorSession(index); err != nil {
		return emptyEditorSession(), fmt.Errorf("%w %v", errEditorSessionInvalid, err)
	}
	return index, nil
}

// readEditorSessionLocked returns the saved index. An unreadable index is set
// aside (never deleted) and reported as an empty session plus a warning, so a
// later save cannot remove kept copies it no longer knows about.
func (a *application) readEditorSessionLocked(root *storageRoot) (editorSessionIndex, string, error) {
	s := &a.editorSession
	s.initLocked()
	index, err := readEditorSessionIndex(root)
	if errors.Is(err, errEditorSessionInvalid) {
		aside := fmt.Sprintf("session.unreadable-%d.json", time.Now().UnixNano())
		if renameErr := root.Rename(editorSessionIndexName, aside); renameErr != nil {
			return index, "", errors.Join(err, renameErr)
		}
		s.unreadable = true
		return index, "The last Text Editor session could not be read. It was kept as " + aside + " in the app data folder, and no unsaved copies were removed.", nil
	}
	if err != nil {
		return index, "", err
	}
	for _, tab := range index.Tabs {
		if tab.Backup != nil {
			s.known[tab.Backup.ID] = true
		}
	}
	return index, "", nil
}

func (a *application) editorSessionDeletePending() bool {
	return a.storage != nil && a.storage.status().DeletePending
}

// writeEditorSessionIndex saves a validated index and removes kept copies that
// it no longer refers to. Callers hold editorSession.mu.
func (a *application) writeEditorSessionLocked(root *storageRoot, index editorSessionIndex) error {
	s := &a.editorSession
	s.initLocked()
	if _, err := root.Lstat(editorSessionIndexName); err == nil {
		if _, _, err := a.readEditorSessionLocked(root); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	referenced := map[string]bool{}
	for i, tab := range index.Tabs {
		if tab.Backup == nil {
			continue
		}
		info, err := root.Lstat(editorSessionBackupFile(tab.Backup.ID))
		if err != nil || !info.Mode().IsRegular() || storageIsLink(info) || info.Size() != tab.Backup.Size {
			return fmt.Errorf("The kept copy for editor tab %d is missing or changed.", i+1)
		}
		referenced[tab.Backup.ID] = true
	}
	raw, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	if err := writeStorageAtomicLimit(root, editorSessionIndexName, raw, editorSessionIndexLimit); err != nil {
		return err
	}
	for id := range referenced {
		delete(s.pending, id)
		s.known[id] = true
	}
	// The index is saved; a copy that cannot be removed now (for example while
	// another program reads it) is retried by the next save.
	if err := a.cleanupEditorSessionLocked(root, referenced); err != nil {
		log.Print("Text Editor session cleanup: ", err)
	}
	return nil
}

func (a *application) cleanupEditorSessionLocked(root *storageRoot, referenced map[string]bool) error {
	s := &a.editorSession
	entries, err := readStorageDirectory(root)
	if err != nil {
		return err
	}
	now := time.Now()
	var failures []error
	for _, entry := range entries {
		name := entry.Name()
		remove := false
		if match := editorSessionBackupName.FindStringSubmatch(name); match != nil {
			id := match[1]
			written, pending := s.pending[id]
			remove = !referenced[id] && (!pending || now.Sub(written) > editorSessionGrace) && (!s.unreadable || s.known[id])
		} else if editorSessionTempName.MatchString(name) {
			remove = !s.writing[name]
		}
		if !remove {
			continue
		}
		info, err := root.Lstat(name)
		if err != nil || !info.Mode().IsRegular() || storageIsLink(info) {
			continue
		}
		if err := root.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			failures = append(failures, err)
			continue
		}
		if match := editorSessionBackupName.FindStringSubmatch(name); match != nil {
			delete(s.pending, match[1])
		}
	}
	return errors.Join(failures...)
}

// deleteEditorSessionLocked removes the index, every kept copy and any index
// that was set aside, then the folder when it is empty.
func (a *application) deleteEditorSessionLocked() error {
	s := &a.editorSession
	s.initLocked()
	root, err := a.editorSessionRoot(false)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	entries, err := readStorageDirectory(root)
	if err != nil {
		return err
	}
	var failures []error
	for _, entry := range entries {
		name := entry.Name()
		if s.writing[name] || name != editorSessionIndexName && !editorSessionBackupName.MatchString(name) && !editorSessionTempName.MatchString(name) && !editorSessionAsideName.MatchString(name) {
			continue
		}
		info, err := root.Lstat(name)
		if err != nil || !info.Mode().IsRegular() || storageIsLink(info) {
			continue
		}
		if err := root.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			failures = append(failures, err)
		}
	}
	clear(s.pending)
	s.unreadable = false
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	workspace, err := openStorageRoot(filepath.Dir(a.dataPath), false)
	if err != nil {
		return err
	}
	defer workspace.Close()
	return removeStorageEmptyDirectory(workspace, editorSessionDirectory)
}

type editorSessionCounter struct{ n int64 }

func (c *editorSessionCounter) Write(p []byte) (int, error) { c.n += int64(len(p)); return len(p), nil }

// writeEditorSessionBackup streams one kept copy to a temporary file in the
// session folder, flushes it and gives it its final name. No size ceiling
// applies other than available disk space.
func (a *application) writeEditorSessionBackup(ctx context.Context, write func(io.Writer) error) (editorSessionBackup, error) {
	s := &a.editorSession
	root, err := a.editorSessionRoot(true)
	if err != nil {
		return editorSessionBackup{}, err
	}
	defer root.Close()
	id := randomHex(16)
	temp := ".backup-" + id + ".tmp"
	s.mu.Lock()
	s.initLocked()
	s.pending[id], s.writing[temp], s.known[id] = time.Now(), true, true
	s.mu.Unlock()
	success := false
	defer func() {
		s.mu.Lock()
		delete(s.writing, temp)
		if !success {
			delete(s.pending, id)
		}
		s.mu.Unlock()
	}()
	file, err := root.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return editorSessionBackup{}, err
	}
	defer root.Remove(temp)
	hash, counter := sha256.New(), &editorSessionCounter{}
	// The buffered writer reads each buffer piece directly into one reusable
	// 1 MiB block, so writing a copy allocates no memory per byte written.
	out := bufio.NewWriterSize(io.MultiWriter(file, hash, counter), 1<<20)
	_, err = out.Write(editorSessionBOM)
	if err == nil {
		err = write(out)
	}
	if err == nil {
		err = out.Flush()
	}
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = file.Sync()
	}
	err = errors.Join(err, file.Close())
	if err == nil {
		err = root.Rename(temp, editorSessionBackupFile(id))
	}
	if err != nil {
		return editorSessionBackup{}, err
	}
	success = true
	return editorSessionBackup{ID: id, Size: counter.n, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

// openEditorSessionBackup opens a kept copy that the saved index refers to.
func (a *application) openEditorSessionBackup(id string) (*os.File, editorSessionBackup, error) {
	if !editorHexID.MatchString(id) {
		return nil, editorSessionBackup{}, errEditorSessionNoBackup
	}
	s := &a.editorSession
	s.mu.Lock()
	defer s.mu.Unlock()
	s.initLocked()
	root, err := a.editorSessionRoot(false)
	if err != nil {
		return nil, editorSessionBackup{}, errEditorSessionNoBackup
	}
	index, err := readEditorSessionIndex(root)
	if err != nil {
		return nil, editorSessionBackup{}, err
	}
	for _, tab := range index.Tabs {
		if tab.Backup == nil || tab.Backup.ID != id {
			continue
		}
		file, err := openStorageRegular(root, editorSessionBackupFile(id), os.O_RDONLY, 0)
		if err != nil {
			return nil, editorSessionBackup{}, errEditorSessionNoBackup
		}
		info, err := file.Stat()
		if err != nil || info.Size() != tab.Backup.Size {
			file.Close()
			return nil, editorSessionBackup{}, errors.New("The kept copy of this document is damaged and was not restored.")
		}
		return file, *tab.Backup, nil
	}
	return nil, editorSessionBackup{}, errEditorSessionNoBackup
}

// readEditorSessionText reads a small kept copy back as text for renderers
// without disk-backed editing.
func (a *application) readEditorSessionText(id string) (string, error) {
	file, expected, err := a.openEditorSessionBackup(id)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if expected.Size > editorSessionTextLimit {
		return "", errors.New("This kept copy is too large to restore without disk-backed editing.")
	}
	raw, err := io.ReadAll(io.LimitReader(file, expected.Size+1))
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	if int64(len(raw)) != expected.Size || hex.EncodeToString(digest[:]) != expected.SHA256 || !utf8.Valid(raw) || !strings.HasPrefix(string(raw), string(editorSessionBOM)) {
		return "", errors.New("The kept copy of this document is damaged and was not restored.")
	}
	return string(raw[len(editorSessionBOM):]), nil
}

// editorSessionFileStatus reports which local files exist. Paths that do not
// answer in time (for example an unreachable network share) are omitted.
func editorSessionFileStatus(paths []string, timeout time.Duration) map[string]bool {
	type answer struct {
		path   string
		exists bool
	}
	answers := make(chan answer, len(paths))
	for _, path := range paths {
		go func(path string) {
			info, err := os.Lstat(path)
			answers <- answer{path, err == nil && info.Mode().IsRegular() || err != nil && !errors.Is(err, os.ErrNotExist)}
		}(path)
	}
	result := map[string]bool{}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for range paths {
		select {
		case value := <-answers:
			result[value.path] = value.exists
		case <-deadline.C:
			return result
		}
	}
	return result
}

func (a *application) handleEditorSession(w http.ResponseWriter, r *http.Request) {
	if a.windowMode != "main" && a.windowMode != standaloneEditorMode {
		jsonError(w, http.StatusForbidden, "Only the main window and the Text Editor keep an editor session.")
		return
	}
	s := &a.editorSession
	switch {
	case r.URL.Path == editorSessionEndpoint && r.Method == http.MethodGet:
		s.mu.Lock()
		root, err := a.editorSessionRoot(false)
		index, warning := emptyEditorSession(), ""
		if err == nil {
			index, warning, err = a.readEditorSessionLocked(root)
		} else if errors.Is(err, os.ErrNotExist) {
			err = nil
		}
		s.mu.Unlock()
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "The last Text Editor session could not be read: "+err.Error())
			return
		}
		var paths []string
		for _, tab := range index.Tabs {
			if tab.Kind == "local" {
				paths = append(paths, tab.Path)
			}
		}
		status := editorSessionFileStatus(paths, 2*time.Second)
		for i := range index.Tabs {
			if exists, ok := status[index.Tabs[i].Path]; ok && index.Tabs[i].Kind == "local" {
				index.Tabs[i].Exists = &exists
			}
		}
		jsonResponse(w, http.StatusOK, map[string]any{"version": index.Version, "active": index.Active, "tabs": index.Tabs, "warning": warning})
	case r.URL.Path == editorSessionEndpoint && r.Method == http.MethodPut:
		var index editorSessionIndex
		if err := decodeEditorBody(w, r, editorSessionIndexLimit, &index); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		if index.Tabs == nil {
			index.Tabs = []editorSessionTab{}
		}
		if err := validateEditorSession(index); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		if a.editorSessionDeletePending() {
			jsonError(w, http.StatusConflict, "App data is being deleted, so open files are not kept for the next launch.")
			return
		}
		s.mu.Lock()
		root, err := a.editorSessionRoot(true)
		if err == nil {
			err = a.writeEditorSessionLocked(root, index)
		}
		s.mu.Unlock()
		if err != nil {
			jsonError(w, http.StatusConflict, "Open files could not be kept for the next launch: "+err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"ok": true, "tabs": len(index.Tabs)})
	case r.URL.Path == editorSessionEndpoint && r.Method == http.MethodDelete:
		s.mu.Lock()
		err := a.deleteEditorSessionLocked()
		s.mu.Unlock()
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "The kept editor session could not be removed: "+err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"ok": true})
	case r.URL.Path == editorSessionEndpoint+"/backup" && r.Method == http.MethodPost:
		a.postEditorSessionBackup(w, r)
	case r.URL.Path == editorSessionEndpoint+"/backup" && r.Method == http.MethodGet:
		text, err := a.readEditorSessionText(r.URL.Query().Get("id"))
		if err != nil {
			code := http.StatusConflict
			if errors.Is(err, errEditorSessionNoBackup) {
				code = http.StatusNotFound
			}
			jsonError(w, code, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"text": text})
	case r.URL.Path == editorSessionEndpoint || r.URL.Path == editorSessionEndpoint+"/backup":
		methodError(w)
	default:
		http.NotFound(w, r)
	}
}

// postEditorSessionBackup keeps the unsaved text of one document. A memory
// document sends its (small) text; a disk-backed document names its buffer and
// lease, and this process streams a stable snapshot of it.
func (a *application) postEditorSessionBackup(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength > 1<<20 {
		var release func()
		var admitted bool
		r, release, admitted = admitEditorMemory(w, r)
		if !admitted {
			return
		}
		defer release()
	}
	var req struct {
		Source string `json:"source"`
		Text   string `json:"text"`
		UID    string `json:"uid"`
		Lease  uint64 `json:"lease"`
	}
	if err := decodeEditorBody(w, r, int64(editorDocumentLimit)*6+4096, &req); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if a.editorSessionDeletePending() {
		jsonError(w, http.StatusConflict, "App data is being deleted, so open files are not kept for the next launch.")
		return
	}
	started := time.Now()
	var backup editorSessionBackup
	var revision uint64
	var err error
	switch req.Source {
	case "text":
		if req.UID != "" || req.Lease != 0 || !validEditorText(req.Text, 3*editorDocumentLimit) {
			jsonError(w, http.StatusBadRequest, "Invalid unsaved text.")
			return
		}
		backup, err = a.writeEditorSessionBackup(r.Context(), func(out io.Writer) error {
			_, err := io.WriteString(out, req.Text)
			return err
		})
	case "buffer":
		if req.Text != "" || !editorUID.MatchString(req.UID) || req.Lease == 0 || a.editorViews == nil {
			jsonError(w, http.StatusBadRequest, "Invalid disk-backed document.")
			return
		}
		var snapshot *editorbuffer.Snapshot
		var parent context.Context
		snapshot, parent, err = a.editorViews.sessionSnapshot(req.UID, req.Lease)
		if err != nil {
			jsonError(w, http.StatusConflict, err.Error())
			return
		}
		revision = snapshot.Revision
		ctx, cancel := context.WithCancel(r.Context())
		stop := context.AfterFunc(parent, cancel)
		backup, err = a.writeEditorSessionBackup(ctx, func(out io.Writer) error { return snapshot.WriteTo(ctx, out) })
		stop()
		cancel()
		snapshot.Close()
	default:
		jsonError(w, http.StatusBadRequest, "Choose memory text or a disk-backed document.")
		return
	}
	if err != nil {
		jsonError(w, http.StatusInsufficientStorage, "The unsaved text could not be kept: "+err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"id": backup.ID, "size": backup.Size, "sha256": backup.SHA256, "revision": revision, "elapsedMs": time.Since(started).Milliseconds()})
}

// sessionSnapshot returns a stable snapshot of a disk-backed document owned by
// the main workspace. Child editor windows never keep a session.
func (m *editorViewRegistry) sessionSnapshot(uid string, lease uint64) (*editorbuffer.Snapshot, context.Context, error) {
	m.mu.Lock()
	g := m.documents[uid]
	if g == nil || g.owner != "" || g.lease != lease || g.pending != "" || g.buffer == nil {
		m.mu.Unlock()
		return nil, nil, errEditorLease
	}
	disk := g.buffer
	m.mu.Unlock()
	disk.mu.Lock()
	if disk.closed || disk.loading || disk.imported == nil {
		disk.mu.Unlock()
		return nil, nil, errors.New("The document is still opening.")
	}
	v, parent := disk.imported, disk.ctx
	disk.mu.Unlock()
	snapshot, err := v.Buffer.Snapshot()
	if err != nil {
		return nil, nil, err
	}
	return snapshot, parent, nil
}

func (restore editorSessionRestore) validate(kind string) error {
	if !editorHexID.MatchString(restore.Backup) || restore.Hash != "" && !editorHash.MatchString(restore.Hash) || !validEditorDiskFormat(restore.Format) {
		return errors.New("Invalid kept copy.")
	}
	if kind != "local" && kind != "localDraft" || kind == "localDraft" && restore.Hash != "" {
		return errors.New("Only local files and unsaved documents are restored from a kept copy.")
	}
	return nil
}

// importSessionBackup loads a kept copy into a new disk buffer. The buffer
// stays unsaved; a local file keeps the digest from the last session as its
// save baseline, so saving refuses to replace a file that changed meanwhile.
func (m *editorViewRegistry) importSessionBackup(ctx context.Context, g *editorDocumentGrant, d *editorDiskDocument, file *os.File, expected editorSessionBackup, restore editorSessionRestore) (*editorbuffer.Imported, error) {
	defer file.Close()
	result, err := editorbuffer.Import(ctx, file, "", func(bytes int64) { d.mu.Lock(); d.progress = bytes; d.mu.Unlock() })
	if err != nil {
		return nil, fmt.Errorf("The kept copy could not be read: %w", err)
	}
	if result.Hash != expected.SHA256 || result.Bytes != expected.Size {
		result.Buffer.Close()
		return nil, errors.New("The kept copy of this document is damaged and was not restored.")
	}
	result.Hash, result.Format = restore.Hash, restore.Format
	result.Buffer.MarkUnsaved()
	state := &editorSessionRestoreState{}
	m.mu.Lock()
	path, kind := g.snapshot.Path, g.snapshot.Binding.Kind
	m.mu.Unlock()
	if kind == "local" {
		current, _, err := hashDiskFile(ctx, path)
		switch {
		case err == nil:
			state.Changed = current != restore.Hash
		case errors.Is(err, os.ErrNotExist):
			state.Missing = true
		case ctx.Err() != nil:
			result.Buffer.Close()
			return nil, ctx.Err()
		}
	}
	d.mu.Lock()
	d.restored = state
	d.mu.Unlock()
	return result, nil
}
