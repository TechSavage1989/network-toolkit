package main

import (
	"errors"
	"io"
	"net/http"
	"networktoolkit/internal/terminal"
	"os"
	"path/filepath"
)

// Directories only, for an explicitly opened local-shell folder chooser. This
// grants no file read/write, preview, transfer or administrator capability.
func (a *application) handleTerminalLocalDirectories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodError(w)
		return
	}
	if !a.terminalCanConnect(r) {
		jsonError(w, http.StatusForbidden, "This window cannot create local shells.")
		return
	}
	for key := range r.URL.Query() {
		if key != "path" {
			jsonError(w, 400, "Only a folder path may be requested.")
			return
		}
	}
	directory := r.URL.Query().Get("path")
	var err error
	if directory == "" {
		directory, err = os.UserHomeDir()
	}
	if err == nil {
		directory, err = terminal.ValidateLocalWorkingDirectory(directory)
	}
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	folder, err := os.Open(directory)
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	defer folder.Close()
	entries, err := folder.ReadDir(2501)
	if err != nil && !errors.Is(err, io.EOF) {
		jsonError(w, 400, err.Error())
		return
	}
	if len(entries) > 2500 {
		jsonError(w, 400, "Folder has more than 2,500 entries; enter a more specific path.")
		return
	}
	type item struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	items := make([]item, 0)
	for _, entry := range entries {
		if entry.IsDir() && entry.Type()&(os.ModeSymlink|os.ModeIrregular) == 0 {
			items = append(items, item{entry.Name(), filepath.Join(directory, entry.Name())})
		}
	}
	jsonResponse(w, 200, map[string]any{"path": directory, "parent": filepath.Dir(directory), "drives": localTransferDrives(), "entries": items})
}
