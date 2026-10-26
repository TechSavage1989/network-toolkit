package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"networktoolkit/internal/terminal"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// A short-lived, single-use, unguessable export ticket allows Chromium's
// DownloadURL drag handler to retrieve a file when it is dropped in Explorer.
// Browser authentication headers cannot be attached to the OS drag request.
// The ticket is in process memory only; passwords and keys are never in URLs.
type remoteDragTicket struct {
	connection remoteBrowserRequest
	filename   string
	expires    time.Time
}

func validDragFilename(s string) bool {
	return len(s) > 0 && len(s) <= 255 && s != "." && s != ".." && !strings.ContainsAny(s, "/\\\x00\r\n")
}
func (a *application) createRemoteDragTicket(r remoteBrowserRequest, name string) string {
	id := randomHex(24)
	a.dragMu.Lock()
	if a.dragTickets == nil {
		a.dragTickets = map[string]remoteDragTicket{}
	}
	now := time.Now()
	for old, item := range a.dragTickets {
		if now.After(item.expires) {
			delete(a.dragTickets, old)
		}
	}
	// Bound ticket memory during rapid file hovering.
	if len(a.dragTickets) >= 256 {
		oldestID := ""
		var oldest time.Time
		for key, item := range a.dragTickets {
			if oldestID == "" || item.expires.Before(oldest) {
				oldestID, oldest = key, item.expires
			}
		}
		delete(a.dragTickets, oldestID)
	}
	a.dragTickets[id] = remoteDragTicket{connection: r, filename: name, expires: now.Add(90 * time.Second)}
	a.dragMu.Unlock()
	// Expiration is enforced even when the user does not request another ticket.
	time.AfterFunc(91*time.Second, func() { a.dragMu.Lock(); delete(a.dragTickets, id); a.dragMu.Unlock() })
	return id
}
func (a *application) consumeRemoteDragTicket(id string) (remoteDragTicket, bool) {
	a.dragMu.Lock()
	ticket, ok := a.dragTickets[id]
	delete(a.dragTickets, id)
	a.dragMu.Unlock()
	if !ok || time.Now().After(ticket.expires) {
		return remoteDragTicket{}, false
	}
	return ticket, true
}
func (a *application) handleRemoteDragTicket(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodError(w)
		return
	}
	var r remoteBrowserRequest
	if err := decodeBody(w, request, 2<<20, &r); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	if r.Action != "stat" || len(r.Path) == 0 || len(r.Path) > 4096 || strings.HasSuffix(r.Path, "/") || r.Path == "." || r.Path == ".." {
		jsonError(w, 400, "Choose one remote file to drag")
		return
	}
	name := filepath.Base(strings.ReplaceAll(r.Path, "\\", "/"))
	if !validDragFilename(name) || strings.ContainsAny(name, "<>:\"|?*") || strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		jsonError(w, 400, "File name cannot be dragged into Windows Explorer. Use Download instead")
		return
	}
	if r.Protocol != "ssh" && r.Protocol != "sftp" && r.Protocol != "ftp" && r.Protocol != "local" {
		jsonError(w, 400, "Unsupported remote protocol")
		return
	}
	if len(r.Password) > 4096 || len(r.PrivateKey) > 1<<20 || len(r.KeyPath) > 4096 || len(r.Host) > 253 || len(r.SessionID) > 128 || len(r.ConnectionID) > 128 {
		jsonError(w, 400, "Invalid connection details")
		return
	}
	if r.Protocol == "local" {
		if r.ConnectionID != "" || r.SessionID != "" || r.Host != "" || r.Username != "" || r.Password != "" || r.PrivateKey != "" || r.KeyPath != "" || r.Passphrase != "" || r.Fingerprint != "" || r.Port != 0 {
			jsonError(w, 400, "Local drags cannot include remote connection details")
			return
		}
		if !filepath.IsAbs(r.Path) {
			jsonError(w, 400, "An absolute local file path is required")
			return
		}
		info, e := os.Lstat(r.Path)
		if e != nil || !info.Mode().IsRegular() || info.Size() > 1<<30 {
			jsonError(w, 400, "Choose an existing regular file smaller than 1 GiB")
			return
		}
		id := a.createRemoteDragTicket(r, name)
		jsonResponse(w, 200, map[string]any{"ticket": id, "filename": name, "expiresInSeconds": 90})
		return
	}
	if r.ConnectionID != "" {
		if r.Protocol == "ftp" || r.SessionID != "" || browserRequestHasLogin(r) {
			jsonError(w, 400, "SFTP connection mode cannot include login details")
			return
		}
		if !a.browserHomeAvailable(request) {
			jsonError(w, 403, "SFTP connection is not available to this window")
			return
		}
		if _, err := a.browserConnection(r.ConnectionID); err != nil {
			jsonError(w, 409, err.Error())
			return
		}
		r.Protocol = "sftp"
	} else if r.SessionID != "" {
		if r.Protocol == "ftp" || r.Host != "" || r.Username != "" || r.Password != "" || r.PrivateKey != "" || r.Fingerprint != "" {
			jsonError(w, 400, "Terminal mode cannot include credentials")
			return
		}
		if a.terminals == nil || a.views == nil {
			jsonError(w, 503, "Terminal service unavailable")
			return
		}
		if err := a.withTerminalOwner(request, r.SessionID, "browse", func() error { _, _, err := a.terminals.SFTPClient(r.SessionID); return err }); err != nil {
			jsonError(w, 409, "SSH session is not available: "+err.Error())
			return
		}
		r.Protocol = "sftp"
	} else {
		if r.Host == "" || r.Username == "" {
			jsonError(w, 400, "Server login is required")
			return
		}
		if r.Protocol != "ftp" && r.Fingerprint == "" {
			jsonError(w, 400, "SSH server fingerprint must be verified")
			return
		}
		if r.Protocol == "ftp" {
			if r.Security != "explicit" && r.Security != "implicit" && r.Security != "plain" {
				jsonError(w, 400, "Invalid FTP security mode")
				return
			}
		} else {
			r.Protocol = "sftp"
		}
	}
	// Assert the remote path exists and is a regular file before offering it as
	// an OS-level drag source. The full transfer waits until the actual drop.
	ctx, cancel := context.WithTimeout(request.Context(), 20*time.Second)
	defer cancel()
	result, err := a.runRemoteBrowserOperation(ctx, r)
	if err != nil {
		jsonError(w, 400, "Cannot prepare drag: "+err.Error())
		return
	}
	if data, ok := result.Data.(map[string]any); ok {
		if kind, _ := data["kind"].(string); strings.EqualFold(kind, "directory") {
			jsonError(w, 400, "Drag an individual file; download folders from the file manager")
			return
		}
		// Native diagnostic results contain int64 sizes, not JSON float64.
		switch size := data["sizeBytes"].(type) {
		case int64:
			if size > 1<<30 || size < 0 {
				jsonError(w, 400, "Drag-out file exceeds the 1 GiB limit")
				return
			}
		case uint64:
			if size > 1<<30 {
				jsonError(w, 400, "Drag-out file exceeds the 1 GiB limit")
				return
			}
		case float64:
			if size > 1<<30 || size < 0 {
				jsonError(w, 400, "Drag-out file exceeds the 1 GiB limit")
				return
			}
		}
	}
	r.Action = "download"
	r.Overwrite = false
	r.LocalPath = ""
	id := a.createRemoteDragTicket(r, name)
	jsonResponse(w, 200, map[string]any{"ticket": id, "filename": name, "expiresInSeconds": 90})
}
func (a *application) handleRemoteDragDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodError(w)
		return
	}
	// This endpoint is purposefully inaccessible without a 192-bit ticket,
	// since an Explorer/Chromium OS drag cannot send X-Session-Token headers.
	// Keep the ticket single-use and short-lived.
	id := strings.TrimPrefix(r.URL.Path, "/drag-download/")
	if len(id) != 48 || strings.ContainsAny(id, "/\\") {
		http.NotFound(w, r)
		return
	}
	ticket, ok := a.consumeRemoteDragTicket(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, "Cross-origin download is not allowed", 403)
		return
	}
	req := ticket.connection
	localPath := req.Path
	if req.Protocol != "local" {
		dir, err := os.MkdirTemp("", "nt-drag-export-")
		if err != nil {
			http.Error(w, "Cannot prepare export", 500)
			return
		}
		defer os.RemoveAll(dir)
		req.Action = "download"
		req.LocalPath = filepath.Join(dir, "export.bin")
		req.Overwrite = false
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
		defer cancel()
		_, err = a.runRemoteBrowserOperation(ctx, req)
		if err != nil {
			if errors.Is(err, terminal.ErrNotConnected) {
				http.Error(w, "SSH connection is no longer available", 409)
			} else {
				http.Error(w, "File transfer failed: "+err.Error(), 502)
			}
			return
		}
		localPath = req.LocalPath
	}
	file, err := os.Open(localPath)
	if err != nil {
		http.Error(w, "Exported file was not found", 500)
		return
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil || !stat.Mode().IsRegular() || stat.Size() > 1<<30 {
		http.Error(w, "Drag export exceeds the size limit", 413)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(ticket.filename))
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))
	_, _ = io.Copy(w, file)
}
