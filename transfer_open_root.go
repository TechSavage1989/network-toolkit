package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"runtime"
)

func openExplorerRoot(root string) error {
	if runtime.GOOS != "windows" {
		return errors.New("Open in Explorer is available in the Windows application")
	}
	info, e := os.Stat(root)
	if e != nil {
		return e
	}
	if !info.IsDir() {
		return errors.New("server root is not a directory")
	}
	return exec.Command("explorer.exe", root).Start()
}
func (a *application) handleTransferOpenRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	var req struct {
		Protocol string `json:"protocol"`
	}
	if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); e != nil || !transferProtocol(req.Protocol) {
		jsonError(w, 400, "Choose a transfer server")
		return
	}
	dir := a.transfers.root(req.Protocol)
	if e := openExplorerRoot(dir); e != nil {
		jsonError(w, 400, e.Error())
		return
	}
	jsonResponse(w, 200, map[string]any{"ok": true})
}
