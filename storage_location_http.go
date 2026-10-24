package main

import (
	"errors"
	"net/http"
	"path/filepath"
	"time"
)

// Only the authenticated home API may reach this handler. A detached view's
// scoped capability has no access to storage selection or deletion.
func (a *application) handleStorage(w http.ResponseWriter, r *http.Request) {
	if a.storage == nil {
		jsonError(w, http.StatusServiceUnavailable, "App-data location controls are unavailable in this instance.")
		return
	}
	switch r.URL.Path {
	case "/api/storage":
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		status := a.storage.status()
		status.WorkspaceDir = filepath.Dir(a.dataPath)
		jsonResponse(w, http.StatusOK, status)
	case "/api/storage/location":
		if r.Method != http.MethodPut {
			methodError(w)
			return
		}
		var request struct {
			Path         *string `json:"path"`
			CopyExisting bool    `json:"copyExisting"`
		}
		if err := decodeBody(w, r, storageMetadataLimit, &request); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		if request.Path == nil {
			jsonError(w, http.StatusBadRequest, "Supply a folder path, or an empty path to choose the default folder.")
			return
		}
		status, copied, err := a.storage.changeLocation(*request.Path, request.CopyExisting)
		if err != nil {
			code := http.StatusBadRequest
			if errors.Is(err, ErrAppStorageBusy) || status.Explicit || status.DeletePending {
				code = http.StatusConflict
			}
			jsonError(w, code, err.Error())
			return
		}
		status.WorkspaceDir = filepath.Dir(a.dataPath)
		jsonResponse(w, http.StatusOK, map[string]any{"ok": true, "storage": status, "copiedFiles": copied})
	case "/api/storage/delete":
		if r.Method != http.MethodPost {
			methodError(w)
			return
		}
		var request struct {
			Confirmation string `json:"confirmation"`
		}
		if err := decodeBody(w, r, storageMetadataLimit, &request); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		if request.Confirmation != "DELETE APP DATA" {
			jsonError(w, http.StatusBadRequest, "Confirm deletion of this app-data folder with DELETE APP DATA.")
			return
		}
		if err := a.storage.scheduleDelete(); err != nil {
			code := http.StatusBadRequest
			if errors.Is(err, ErrAppStorageBusy) {
				code = http.StatusConflict
			}
			jsonError(w, code, err.Error())
			return
		}
		a.shutdownRequested.Store(true)
		jsonResponse(w, http.StatusOK, map[string]any{"ok": true, "closing": true})
		go func() { time.Sleep(200 * time.Millisecond); a.stop() }()
	default:
		http.NotFound(w, r)
	}
}
