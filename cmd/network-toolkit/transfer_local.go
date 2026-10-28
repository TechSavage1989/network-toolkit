package main

// Only the authenticated local interface can enumerate candidate files.
// Selecting a file does not upload or download it. The user must start an
// explicit SFTP/TFTP transfer before any file content is accessed.
import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"
)

type localTransferEntry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Directory bool   `json:"directory"`
	Size      int64  `json:"size,omitempty"`
	Modified  string `json:"modified,omitempty"`
}

func (a *application) handleTransferLocal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodError(w)
		return
	}
	// Local file previews are read-only and restricted to 128 KiB, like
	// previews of files inside the configured server root.
	if filename := strings.TrimSpace(r.URL.Query().Get("preview")); filename != "" {
		if len(filename) > 4096 || !filepath.IsAbs(filename) || strings.ContainsRune(filename, 0) {
			jsonError(w, 400, "Choose a valid absolute file path for preview.")
			return
		}
		file, err := os.Open(filename)
		if err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 128*1024 {
			jsonError(w, 400, "Preview requires a regular file no larger than 128 KiB.")
			return
		}
		content, err := io.ReadAll(io.LimitReader(file, 128*1024+1))
		if err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		if !utf8.Valid(content) || strings.ContainsRune(string(content), 0) {
			jsonResponse(w, 200, map[string]any{"binary": true, "bytes": len(content)})
			return
		}
		jsonResponse(w, 200, map[string]any{"binary": false, "bytes": len(content), "content": string(content)})
		return
	}
	requested := strings.TrimSpace(r.URL.Query().Get("path"))
	if len(requested) > 4096 || strings.ContainsRune(requested, 0) {
		jsonError(w, 400, "Invalid local file path.")
		return
	}
	if requested == "" {
		requested, _ = os.UserHomeDir()
	}
	if requested == "" {
		requested = "."
	}
	full, err := filepath.Abs(requested)
	if err != nil {
		jsonError(w, 400, "Cannot resolve the local file path.")
		return
	}
	if info, statErr := os.Stat(full); statErr != nil {
		jsonError(w, 400, "Cannot open that local folder: "+statErr.Error())
		return
	} else if !info.IsDir() {
		full = filepath.Dir(full)
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		jsonError(w, 400, "Cannot open that local folder: "+err.Error())
		return
	}
	if len(entries) > 2500 {
		jsonError(w, 400, "Folder has more than 2,500 entries; enter a more specific path.")
		return
	}
	result := make([]localTransferEntry, 0, len(entries))
	for _, item := range entries {
		if item.Type()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			continue
		}
		entry := localTransferEntry{Name: item.Name(), Path: filepath.Join(full, item.Name()), Directory: item.IsDir()}
		if info, e := item.Info(); e == nil {
			if info.Mode().IsRegular() {
				entry.Size = info.Size()
			}
			entry.Modified = info.ModTime().UTC().Format(time.RFC3339)
		}
		result = append(result, entry)
	}
	roots := []string{}
	if runtime.GOOS == "windows" {
		for letter := 'A'; letter <= 'Z'; letter++ {
			drive := string(letter) + ":\\"
			if info, e := os.Stat(drive); e == nil && info.IsDir() {
				roots = append(roots, drive)
			}
		}
	} else {
		roots = append(roots, "/")
	}
	jsonResponse(w, 200, map[string]any{"path": full, "parent": filepath.Dir(full), "separator": string(os.PathSeparator), "roots": roots, "entries": result})
}
