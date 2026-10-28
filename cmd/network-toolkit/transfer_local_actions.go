package main

// Local explorer file modifications are explicit, authenticated commands.
// Unlike TFTP/SFTP network clients they do not require a listening server.
import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func absoluteExplorerPath(p string) (string, error) {
	if len(p) == 0 || len(p) > 4096 || strings.ContainsRune(p, 0) || !filepath.IsAbs(p) {
		return "", errors.New("choose an absolute local path")
	}
	clean := filepath.Clean(p)
	if clean == filepath.Dir(clean) {
		return "", errors.New("cannot modify a drive or filesystem root")
	}
	return clean, nil
}

func localExplorerAction(action, source, target string, overwrite, confirm bool) error {
	path, err := absoluteExplorerPath(source)
	if err != nil {
		return err
	}
	switch action {
	case "mkdir":
		if !validExplorerName(filepath.Base(path)) {
			return errors.New("choose a valid folder name without reserved characters or trailing dots/spaces")
		}
		return os.Mkdir(path, 0700)
	case "rename", "copy":
		dest, err := absoluteExplorerPath(target)
		if err != nil {
			return err
		}
		if path == dest {
			return errors.New("source and destination must differ")
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("cannot operate on symbolic links")
		}
		if action == "rename" {
			if _, err := os.Lstat(dest); err == nil {
				return errors.New("destination exists; choose another name")
			} else if !os.IsNotExist(err) {
				return err
			}
			return os.Rename(path, dest)
		}
		if info.IsDir() {
			return copyTransferDirectory(path, dest, overwrite)
		}
		if !info.Mode().IsRegular() {
			return errors.New("cannot copy special files")
		}
		return copyTransferFile(path, dest, overwrite)
	case "delete":
		if !confirm {
			return errors.New("confirm permanent deletion first")
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("cannot delete symbolic links from this explorer")
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return errors.New("cannot delete special files")
		}
		// os.Remove does not recursively remove folders. Empty only.
		return os.Remove(path)
	default:
		return errors.New("unsupported local file action")
	}
}
func (a *application) handleTransferLocalFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	var req struct {
		Action    string `json:"action"`
		Path      string `json:"path"`
		NewPath   string `json:"newPath"`
		Overwrite bool   `json:"overwrite"`
		Confirm   bool   `json:"confirm"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&req); err != nil {
		jsonError(w, 400, "Invalid local file operation.")
		return
	}
	targets := []editorExplorerTarget{}
	if req.Action == "rename" || req.Action == "delete" || req.Action == "mkdir" {
		targets = append(targets, editorExplorerTarget{Binding: editorBinding{Kind: "local"}, Path: req.Path})
	}
	if req.Action == "rename" || req.Action == "copy" {
		targets = append(targets, editorExplorerTarget{Binding: editorBinding{Kind: "local"}, Path: req.NewPath})
	}
	release, err := a.reserveExplorerMutation(targets...)
	if err != nil {
		jsonError(w, 409, err.Error())
		return
	}
	defer release()
	if err := localExplorerAction(req.Action, req.Path, req.NewPath, req.Overwrite, req.Confirm); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	jsonResponse(w, 200, map[string]any{"ok": true, "message": "Local file operation completed"})
}
