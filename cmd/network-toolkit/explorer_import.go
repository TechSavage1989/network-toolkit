package main

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// A recursive import retains the originally selected root handle. Every newly
// created directory is identity-pinned; subsequent files never re-root at a
// pathname that another process could replace with a junction.
type explorerImport struct {
	mu          sync.Mutex
	owner, path string
	root        *os.Root
	info        os.FileInfo
	dirs        map[string]os.FileInfo
	valid       func() bool
	expires     time.Time
	closed      bool
	bytes       int64
	files       int
}

func importRelative(value string) (string, error) {
	if value == "" || value == "." {
		return ".", nil
	}
	if strings.Contains(value, "\\") || strings.HasPrefix(value, "/") {
		return "", errors.New("Invalid relative folder")
	}
	parts := strings.Split(value, "/")
	if len(parts) > 32 {
		return "", errors.New("Folder exceeds32levels")
	}
	for _, part := range parts {
		if validateExplorerDropName(part) != nil {
			return "", errors.New("Invalid relative folder")
		}
	}
	return filepath.Join(parts...), nil
}
func (j *explorerImport) verify(relative string) error {
	if j.closed || !j.valid() || time.Now().After(j.expires) {
		return errors.New("Copy destination approval expired")
	}
	current, err := os.Lstat(j.path)
	if err != nil || current.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 || !os.SameFile(j.info, current) {
		return errors.New("Selected destination root changed")
	}
	currentPath := "."
	if relative == "." {
		return nil
	}
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		currentPath = filepath.Join(currentPath, part)
		expected := j.dirs[currentPath]
		info, err := j.root.Lstat(currentPath)
		if expected == nil || err != nil || !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 || !os.SameFile(expected, info) {
			return errors.New("Created destination folder changed; copy stopped")
		}
	}
	return nil
}
func (a *application) importSession(owner, id string) (*explorerImport, error) {
	a.explorerTransfers.mu.Lock()
	j := a.explorerTransfers.imports[id]
	a.explorerTransfers.mu.Unlock()
	if j == nil || j.owner != owner {
		return nil, errors.New("Copy destination expired")
	}
	return j, nil
}
func (a *application) handleExplorerImportAction(w http.ResponseWriter, r *http.Request) {
	var req struct{ Action, ID, Path string }
	if err := decodeEditorBody(w, r, 16<<10, &req); err != nil {
		jsonError(w, 400, "Invalid folder copy request")
		return
	}
	owner := editorRequestView(r)
	if !a.explorerCallerValid(owner) {
		jsonError(w, 403, "File explorer approval unavailable")
		return
	}
	if req.Action == "begin" {
		if req.ID != "" || !filepath.IsAbs(req.Path) || len(req.Path) > 4096 {
			jsonError(w, 400, "Choose an absolute destination folder")
			return
		}
		info, err := os.Lstat(req.Path)
		if err != nil || !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			jsonError(w, 400, "Choose a regular destination folder")
			return
		}
		root, err := os.OpenRoot(req.Path)
		if err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		opened, err := root.Stat(".")
		if err != nil || !os.SameFile(info, opened) {
			root.Close()
			jsonError(w, 409, "Destination changed")
			return
		}
		valid := a.localExplorerGuard(owner)
		if !valid() {
			root.Close()
			jsonError(w, 403, "Local file explorer is no longer available")
			return
		}
		j := &explorerImport{owner: owner, path: req.Path, root: root, info: info, dirs: map[string]os.FileInfo{".": info}, valid: valid, expires: time.Now().Add(15 * time.Minute)}
		m := &a.explorerTransfers
		m.mu.Lock()
		if m.imports == nil {
			m.imports = map[string]*explorerImport{}
		}
		if len(m.imports) >= 8 {
			m.mu.Unlock()
			root.Close()
			jsonError(w, 409, "Finish another folder copy first")
			return
		}
		id := randomHex(16)
		m.imports[id] = j
		m.mu.Unlock()
		time.AfterFunc(15*time.Minute, func() {
			j.mu.Lock()
			j.closed = true
			j.root.Close()
			j.mu.Unlock()
			m.mu.Lock()
			delete(m.imports, id)
			m.mu.Unlock()
		})
		jsonResponse(w, 200, map[string]any{"id": id})
		return
	}
	j, err := a.importSession(owner, req.ID)
	if err != nil {
		jsonError(w, 404, err.Error())
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if req.Action == "end" {
		j.closed = true
		j.root.Close()
		a.explorerTransfers.mu.Lock()
		delete(a.explorerTransfers.imports, req.ID)
		a.explorerTransfers.mu.Unlock()
		jsonResponse(w, 200, map[string]bool{"ok": true})
		return
	}
	relative, err := importRelative(req.Path)
	if err != nil || relative == "." || req.Action != "mkdir" {
		jsonError(w, 400, "Invalid new folder")
		return
	}
	if len(j.dirs)+j.files >= 1500 {
		jsonError(w, 400, "Copy exceeds1,500entries")
		return
	}
	if err = j.verify(filepath.Dir(relative)); err != nil {
		jsonError(w, 409, err.Error())
		return
	}
	release, err := a.reserveExplorerMutation(editorExplorerTarget{Binding: editorBinding{Kind: "local"}, Path: filepath.Join(j.path, relative)})
	if err != nil {
		jsonError(w, 409, err.Error())
		return
	}
	defer release()
	if err = j.root.Mkdir(relative, 0700); err != nil {
		jsonError(w, 409, "New folder could not be created; rename or skip existing folders")
		return
	}
	info, err := j.root.Lstat(relative)
	if err != nil || !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		jsonError(w, 409, "New destination folder changed")
		return
	}
	j.dirs[relative] = info
	if err = j.verify(relative); err != nil {
		jsonError(w, 409, err.Error())
		return
	}
	jsonResponse(w, 200, map[string]bool{"ok": true})
}
