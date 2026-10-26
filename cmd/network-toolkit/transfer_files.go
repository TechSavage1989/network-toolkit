package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type transferFileRecord struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Directory bool   `json:"directory"`
	Size      int64  `json:"size"`
	Modified  string `json:"modified,omitempty"`
}

func transferList(root, sub string) (map[string]any, error) {
	full, e := transferPath(root, sub, true)
	if e != nil {
		return nil, e
	}
	items, e := os.ReadDir(full)
	if e != nil {
		return nil, e
	}
	if len(items) > 10000 {
		return nil, errors.New("more than 10,000 entries; choose a smaller directory")
	}
	result := make([]transferFileRecord, 0, len(items))
	for _, item := range items {
		if item.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, e := item.Info()
		if e != nil || !info.Mode().IsRegular() && !info.IsDir() {
			continue
		}
		relative, e := filepath.Rel(root, filepath.Join(full, item.Name()))
		if e != nil {
			continue
		}
		entry := transferFileRecord{Name: item.Name(), Path: filepath.ToSlash(relative), Directory: info.IsDir(), Size: info.Size(), Modified: info.ModTime().UTC().Format(time.RFC3339)}
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Directory != result[j].Directory {
			return result[i].Directory
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	rel, _ := filepath.Rel(root, full)
	if rel == "." {
		rel = ""
	}
	parent := filepath.Dir(rel)
	if parent == "." {
		parent = ""
	}
	return map[string]any{"root": root, "path": filepath.ToSlash(rel), "parent": filepath.ToSlash(parent), "entries": result}, nil
}
func transferFileAction(root, action, sub, target, local string, overwrite, confirm bool) (string, error) {
	path, e := transferPath(root, sub, false)
	if e != nil && action != "mkdir" {
		return "", e
	}
	switch action {
	case "mkdir":
		path, e = transferPath(root, sub, false)
		if e != nil {
			return "", e
		}
		e = os.Mkdir(path, 0700)
	case "copy":
		if target == "" {
			return "", errors.New("enter a destination path")
		}
		dst, err := transferPath(root, target, false)
		if err != nil {
			return "", err
		}
		if path == dst {
			return "", errors.New("source and destination must differ")
		}
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		if info.IsDir() {
			e = copyTransferDirectory(path, dst, overwrite)
		} else if info.Mode().IsRegular() {
			e = copyTransferFile(path, dst, overwrite)
		} else {
			return "", errors.New("unsupported source file")
		}
	case "rename", "move":
		if target == "" {
			return "", errors.New("enter a destination path")
		}
		var dst string
		dst, e = transferPath(root, target, false)
		if e != nil {
			return "", e
		}
		if !overwrite {
			if _, err := os.Stat(dst); err == nil {
				return "", errors.New("destination exists; enable overwrite to replace it")
			}
		}
		if _, err := os.Stat(dst); err == nil && overwrite {
			return "", errors.New("rename cannot replace an existing file; use a unique destination name")
		}
		e = os.Rename(path, dst)
	case "delete":
		if !confirm {
			return "", errors.New("confirm deletion first")
		}
		e = os.Remove(path) // directories only when empty
	case "upload":
		if strings.TrimSpace(local) == "" {
			return "", errors.New("choose a local source file")
		}
		var info os.FileInfo
		info, e = os.Lstat(local)
		if e != nil {
			return "", e
		}
		if info.IsDir() {
			e = copyTransferDirectory(local, path, overwrite)
			break
		}
		if !info.Mode().IsRegular() {
			return "", errors.New("source must be a file or directory")
		}
		if !overwrite {
			if _, err := os.Stat(path); err == nil {
				return "", errors.New("destination already exists")
			}
		}
		e = copyTransferFile(local, path, overwrite)
	case "download":
		if strings.TrimSpace(local) == "" {
			return "", errors.New("choose a local destination filename")
		}
		var info os.FileInfo
		info, e = os.Lstat(path)
		if e != nil {
			return "", e
		}
		if info.IsDir() {
			e = copyTransferDirectory(path, local, overwrite)
			break
		}
		if !info.Mode().IsRegular() {
			return "", errors.New("choose a file or directory")
		}
		e = copyTransferFile(path, local, overwrite)
	default:
		return "", errors.New("unsupported file management operation")
	}
	if e != nil {
		return "", e
	}
	return fmt.Sprintf("%s completed", strings.ToUpper(action[:1])+action[1:]), nil
}

// copyTransferDirectory copies folder contents without following links, junctions
// or special files. Folder transfers do not delete anything at the destination.
// The item limit bounds accidental recursive bulk operations; overwrites must
// be explicitly requested. This operation is local file management, not a
// TFTP recursive wire operation (TFTP cannot list remote directories).
func copyTransferDirectory(src, dst string, overwrite bool) error {
	from, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	to, err := filepath.Abs(dst)
	if err != nil {
		return err
	}
	if relative, e := filepath.Rel(from, to); e == nil && (relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) && !filepath.IsAbs(relative)) {
		return errors.New("cannot copy a folder into itself")
	}
	items := 0
	var visit func(string, string, int) error
	visit = func(source, dest string, depth int) error {
		if depth > 64 || items > 10000 {
			return errors.New("folder transfer exceeds the 10,000 entry or 64 level limit")
		}
		info, err := os.Lstat(source)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("folder copy does not follow symlinks or special files")
		}
		if existing, err := os.Lstat(dest); err == nil {
			if !existing.IsDir() || existing.Mode()&os.ModeSymlink != 0 {
				return errors.New("destination is not a regular directory")
			}
			if !overwrite {
				return errors.New("destination directory exists; confirm merge/overwrite to continue")
			}
		} else if os.IsNotExist(err) {
			if err := os.Mkdir(dest, 0700); err != nil {
				return err
			}
		} else {
			return err
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			items++
			if items > 10000 {
				return errors.New("folder transfer exceeds 10,000 entries")
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return errors.New("folder contains a symbolic link; refusing to follow it")
			}
			childSrc, childDst := filepath.Join(source, entry.Name()), filepath.Join(dest, entry.Name())
			info, err := os.Lstat(childSrc)
			if err != nil {
				return err
			}
			if info.IsDir() {
				if err := visit(childSrc, childDst, depth+1); err != nil {
					return err
				}
			} else if info.Mode().IsRegular() {
				if info.Size() > 4<<30 {
					return errors.New("file exceeds 4 GiB supported limit")
				}
				if err := copyTransferFile(childSrc, childDst, overwrite); err != nil {
					return err
				}
			} else {
				return errors.New("folder contains an unsupported special file")
			}
		}
		return nil
	}
	return visit(from, to, 0)
}

func copyTransferFile(src, dst string, overwrite bool) error {
	input, e := os.Open(src)
	if e != nil {
		return e
	}
	defer input.Close()
	info, e := os.Lstat(src)
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() {
		return errors.New("source is not a regular file")
	}
	if info.Size() > 4<<30 {
		return errors.New("file exceeds 4 GiB supported limit")
	}
	if !overwrite {
		if _, err := os.Stat(dst); err == nil {
			return errors.New("destination already exists")
		}
	}
	tmp, e := os.CreateTemp(filepath.Dir(dst), ".ntk-file-*.part")
	if e != nil {
		return e
	}
	defer os.Remove(tmp.Name())
	_, e = io.Copy(tmp, input)
	if e != nil {
		_ = tmp.Close()
		return e
	}
	if e = tmp.Sync(); e != nil {
		_ = tmp.Close()
		return e
	}
	if e = tmp.Close(); e != nil {
		return e
	}
	return atomicTransferPublish(tmp.Name(), dst, overwrite)
}
func (a *application) handleTransferFiles(w http.ResponseWriter, r *http.Request) {
	if a.transfers == nil {
		jsonError(w, 503, "File manager not available.")
		return
	}
	var proto string
	if r.Method == http.MethodGet {
		proto = r.URL.Query().Get("protocol")
	} else if r.Method == http.MethodPost {
	} else {
		methodError(w)
		return
	}
	if r.Method == http.MethodGet {
		if !transferProtocol(proto) {
			jsonError(w, 400, "Choose SFTP, TFTP or FTP.")
			return
		}
		root := a.transfers.root(proto)
		if strings.TrimSpace(r.URL.Query().Get("preview")) != "" {
			sub := r.URL.Query().Get("preview")
			full, e := transferPath(root, sub, false)
			if e != nil {
				jsonError(w, 400, e.Error())
				return
			}
			file, e := os.Open(full)
			if e != nil {
				jsonError(w, 400, e.Error())
				return
			}
			defer file.Close()
			info, e := file.Stat()
			if e != nil || !info.Mode().IsRegular() {
				jsonError(w, 400, "Preview requires a regular file.")
				return
			}
			if info.Size() > 128*1024 {
				jsonError(w, 400, "Preview is limited to files up to 128 KiB.")
				return
			}
			data, e := io.ReadAll(io.LimitReader(file, 128*1024+1))
			if e != nil {
				jsonError(w, 400, e.Error())
				return
			}
			if strings.ContainsRune(string(data), 0) {
				jsonResponse(w, 200, map[string]any{"binary": true, "bytes": len(data)})
				return
			}
			jsonResponse(w, 200, map[string]any{"binary": false, "content": string(data), "bytes": len(data)})
			return
		}
		result, e := transferList(root, r.URL.Query().Get("path"))
		if e != nil {
			jsonError(w, 400, e.Error())
			return
		}
		jsonResponse(w, 200, result)
		return
	}
	var req struct {
		Protocol  string `json:"protocol"`
		Action    string `json:"action"`
		Path      string `json:"path"`
		NewPath   string `json:"newPath"`
		LocalPath string `json:"localPath"`
		Overwrite bool   `json:"overwrite"`
		Confirm   bool   `json:"confirm"`
	}
	if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&req); e != nil {
		jsonError(w, 400, "Invalid file-management request.")
		return
	}
	if !transferProtocol(req.Protocol) {
		jsonError(w, 400, "Choose SFTP or TFTP.")
		return
	}
	root := a.transfers.root(req.Protocol)
	msg, e := transferFileAction(root, req.Action, req.Path, req.NewPath, req.LocalPath, req.Overwrite, req.Confirm)
	if e != nil {
		jsonError(w, 400, e.Error())
		return
	}
	jsonResponse(w, 200, map[string]any{"ok": true, "message": msg})
}
