package main

// Remote browser commands are deliberately separate from diagnostic history.
// Saved authentication is resolved in the backend from an approved credential
// reference. Manual input is transient; loopback HTTP is not HTTPS.
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"networktoolkit/internal/diag"
	"networktoolkit/internal/model"
	"networktoolkit/internal/terminal"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type remoteBrowserRequest struct {
	Protocol         string `json:"protocol"`
	SessionID        string `json:"sessionId,omitempty"`
	ConnectionID     string `json:"connectionId,omitempty"`
	ProfileID        string `json:"profileId,omitempty"`
	CredentialID     string `json:"credentialId,omitempty"`
	TimeoutMs        int    `json:"timeoutMs,omitempty"`
	Action           string `json:"action"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	KeyPath          string `json:"keyPath"`
	PrivateKey       string `json:"privateKey,omitempty"`
	Passphrase       string `json:"passphrase"`
	AuthMode         string `json:"authMode"`
	Fingerprint      string `json:"fingerprint"`
	SSHCompatibility string `json:"sshCompatibility,omitempty"`
	Security         string `json:"security"`
	Path             string `json:"path"`
	NewPath          string `json:"newPath"`
	LocalPath        string `json:"localPath"`
	Confirm          bool   `json:"confirm"`
	Overwrite        bool   `json:"overwrite"`
	StagedUpload     bool   `json:"stagedUpload,omitempty"`
	Content          string `json:"content,omitempty"`
	ExpectedHash     string `json:"expectedHash,omitempty"`
	BOM              bool   `json:"bom,omitempty"`
	Encoding         string `json:"encoding,omitempty"`
	Permissions      string `json:"permissions,omitempty"`
}

func (r remoteBrowserRequest) opts() map[string]any {
	return map[string]any{
		"port": r.Port, "username": r.Username, "password": r.Password, "keyPath": r.KeyPath, "privateKey": r.PrivateKey, "passphrase": r.Passphrase, "authMode": r.AuthMode, "fingerprint": r.Fingerprint, "sshCompatibility": r.SSHCompatibility, "security": r.Security, "operation": r.Action, "remotePath": r.Path, "newPath": r.NewPath, "localPath": r.LocalPath, "confirmDelete": r.Confirm, "overwrite": r.Overwrite, "stagedUpload": r.StagedUpload, "permissions": r.Permissions, "timeoutMs": 15000,
	}
}
func (r remoteBrowserRequest) request() model.Request {
	tool := "sftp"
	if r.Protocol == "ftp" {
		tool = "ftp"
	}
	return model.Request{Tool: tool, Target: r.Host, Options: r.opts()}
}

// The session picker exposes only connected SSH terminals currently owned by
// the home window. Detached window sessions cannot be borrowed by this UI.
func (a *application) remoteBrowserSessions() []map[string]any {
	sessions := []map[string]any{}
	if a.terminals == nil || a.views == nil {
		return sessions
	}
	a.views.mu.Lock()
	defer a.views.mu.Unlock()
	if !a.views.homePresent || a.views.homeClosing {
		return sessions
	}
	for _, info := range a.terminals.List() {
		if info.Type != "ssh" || info.State != "connected" {
			continue
		}
		owner := a.views.ownerLocked(info.ID)
		if owner.ViewID != "" || !owner.Ready {
			continue
		}
		sessions = append(sessions, map[string]any{
			"id": info.ID, "label": info.Label, "host": info.Host,
			"port": info.Port, "username": info.Username, "revision": owner.Revision,
		})
	}
	return sessions
}

func (a *application) runRemoteBrowserOperation(ctx context.Context, r remoteBrowserRequest) (model.Result, error) {
	var err error
	r, err = a.resolveRemoteCredential(ctx, r)
	if err != nil {
		return model.Result{}, err
	}
	finishActivity := a.activity.begin(ctx, r.Protocol)
	defer finishActivity()
	if r.ConnectionID != "" {
		entry, err := a.browserConnection(r.ConnectionID)
		if err != nil {
			return model.Result{}, err
		}
		return entry.client.Browse(ctx, r.request())
	}
	if r.SessionID != "" {
		return a.terminals.BrowseSFTP(ctx, r.SessionID, r.request())
	}
	return diag.Run(ctx, r.request(), nil)
}

func (a *application) handleRemoteBrowser(w http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		jsonResponse(w, http.StatusOK, map[string]any{"sessions": a.remoteBrowserSessions(), "connections": a.browserConnectionInfos()})
		return
	}
	if request.Method != http.MethodPost {
		methodError(w)
		return
	}
	var r remoteBrowserRequest
	if e := json.NewDecoder(http.MaxBytesReader(w, request.Body, 12<<20)).Decode(&r); e != nil {
		jsonError(w, 400, "Invalid remote file request")
		return
	}
	if r.SSHCompatibility != "" && r.SSHCompatibility != "modern" && r.SSHCompatibility != "legacy" {
		jsonError(w, 400, "Unsupported SSH compatibility policy")
		return
	}
	if r.Protocol != "ssh" && r.Protocol != "sftp" && r.Protocol != "ftp" {
		jsonError(w, 400, "Select SSH/SFTP or FTP")
		return
	}
	if (r.SessionID == "" && r.ConnectionID == "" && r.Host == "") || len(r.Host) > 253 || len(r.Password) > 4096 || len(r.PrivateKey) > (1<<20) || len(r.KeyPath) > 4096 || len(r.Path) > 4096 || len(r.NewPath) > 4096 || len(r.LocalPath) > 4096 || len(r.SessionID) > 128 || len(r.ConnectionID) > 128 || len(r.ProfileID) > 80 {
		jsonError(w, 400, "Invalid remote connection details")
		return
	}
	if r.Action == "remove" || r.Action == "rmdir" {
		if !r.Confirm {
			jsonError(w, 400, "Confirm permanent remote deletion")
			return
		}
	}
	switch r.Action {
	case "connect", "disconnect", "open-terminal", "fingerprint", "probe", "list", "stat", "upload", "download", "mkdir", "remove", "rmdir", "rename", "preview", "read-text", "write-text", "copy", "chmod":
	default:
		jsonError(w, 400, "Unsupported remote file operation")
		return
	}
	if r.Action == "probe" && r.SessionID == "" && r.ConnectionID == "" {
		jsonError(w, http.StatusBadRequest, "SFTP capability checks require an authenticated SSH terminal")
		return
	}
	if r.ConnectionID != "" {
		if r.Protocol == "ftp" || r.SessionID != "" || browserRequestHasLogin(r) || r.Action == "fingerprint" || r.Action == "connect" {
			jsonError(w, 400, "SFTP connection mode accepts only its connection ID and file operation")
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
	}
	if r.SessionID != "" {
		// Terminal-owned SSH connections have already authenticated and
		// verified the server key. Reject mixed or forged credential fields.
		if r.Protocol == "ftp" || r.CredentialID != "" || r.Action == "fingerprint" || r.Host != "" || r.Username != "" || r.Password != "" || r.KeyPath != "" || r.PrivateKey != "" || r.Passphrase != "" || r.Fingerprint != "" {
			jsonError(w, 400, "Terminal session mode requires only a session ID and file operation")
			return
		}
		if a.terminals == nil || a.views == nil {
			jsonError(w, 503, "No terminal service is available")
			return
		}
		if err := a.withTerminalOwner(request, r.SessionID, "browse", func() error {
			_, _, err := a.terminals.SFTPClient(r.SessionID)
			return err
		}); err != nil {
			jsonError(w, http.StatusConflict, "SSH terminal is not available to this window: "+err.Error())
			return
		}
	}
	if r.Protocol != "ftp" {
		r.Protocol = "sftp"
		if r.AuthMode == "" {
			r.AuthMode = "password"
		}
		if r.Port == 0 {
			r.Port = 22
		}
		if r.Security == "" {
			r.Security = "ssh"
		}
		if r.SessionID == "" && r.ConnectionID == "" && r.CredentialID == "" && strings.TrimSpace(r.Fingerprint) == "" && r.Action != "fingerprint" {
			jsonError(w, 400, "Verify and enter the SSH server fingerprint before browsing files")
			return
		}
	}
	if r.Protocol == "ftp" {
		if r.Port == 0 {
			if r.Security == "implicit" {
				r.Port = 990
			} else {
				r.Port = 21
			}
		}
		if r.Security == "" {
			r.Security = "explicit"
		}
	}

	newConnectionID := ""
	defer func() {
		if newConnectionID != "" {
			a.closeBrowserConnection(newConnectionID)
		}
	}()
	if r.Action == "connect" {
		if r.Protocol == "ftp" || r.SessionID != "" {
			jsonError(w, 400, "Persistent file connections require standalone SFTP settings")
			return
		}
		id, err := a.openBrowserConnection(request, r)
		if err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		newConnectionID = id
		r = remoteBrowserRequest{Protocol: "sftp", ConnectionID: id, Action: "list", Path: r.Path}
		// The initial listing is handled below on the retained authenticated transport.
	}
	if r.Action == "disconnect" {
		if r.ConnectionID == "" {
			jsonError(w, 400, "Choose an owned SFTP connection to disconnect")
			return
		}
		a.closeBrowserConnection(r.ConnectionID)
		jsonResponse(w, 200, map[string]any{"ok": true})
		return
	}
	if r.Action == "open-terminal" {
		if r.ConnectionID == "" {
			jsonError(w, 400, "Choose an owned SFTP connection")
			return
		}
		session, err := a.promoteBrowserConnection(request, r.ConnectionID)
		if err != nil {
			jsonError(w, 409, err.Error())
			return
		}
		jsonResponse(w, 200, map[string]any{"session": session, "viewState": a.terminalViewStateForRequest(request)})
		return
	}
	if r.Action == "copy" {
		a.handleRemoteCopy(w, request, r)
		return
	}
	if r.Action == "chmod" && r.Protocol == "ftp" {
		jsonError(w, 400, "Permissions changes require SFTP")
		return
	}
	if r.Action == "read-text" || r.Action == "write-text" {
		if len(r.ExpectedHash) != 0 && len(r.ExpectedHash) != 64 {
			jsonError(w, 400, "Invalid file revision")
			return
		}
		a.handleRemoteTextEditor(w, request, r)
		return
	}
	if r.Action == "preview" {
		// File previews are bounded and never persist remote content in history.
		probe := r
		probe.Action = "stat"
		ctx, cancel := context.WithTimeout(request.Context(), 40*time.Second)
		defer cancel()
		result, e := a.runRemoteBrowserOperation(ctx, probe)
		if e != nil {
			jsonError(w, 400, e.Error())
			return
		}
		v, _ := result.Data.(map[string]any)
		if raw, ok := v["sizeBytes"]; ok {
			var n float64
			switch size := raw.(type) {
			case int64:
				n = float64(size)
			case uint64:
				n = float64(size)
			case float64:
				n = size
			}
			if n > 131072 {
				jsonError(w, 400, "Preview is limited to 128 KiB; download the file instead")
				return
			}
		}
		tmp, e := os.MkdirTemp("", "network-toolkit-preview-")
		if e != nil {
			jsonError(w, 500, "Cannot prepare preview")
			return
		}
		defer os.RemoveAll(tmp)
		probe.Action = "download"
		probe.LocalPath = filepath.Join(tmp, "preview.txt")
		probe.Overwrite = false
		_, e = a.runRemoteBrowserOperation(ctx, probe)
		if e != nil {
			jsonError(w, 400, e.Error())
			return
		}
		f, e := os.Open(probe.LocalPath)
		if e != nil {
			jsonError(w, 500, "Cannot read preview")
			return
		}
		defer f.Close()
		b, e := io.ReadAll(io.LimitReader(f, 131073))
		if e != nil {
			jsonError(w, 500, "Cannot read preview")
			return
		}
		if len(b) > 131072 {
			jsonError(w, 400, "File exceeds preview size limit")
			return
		}
		binary := false
		for _, v := range b {
			if v == 0 {
				binary = true
				break
			}
		}
		if binary {
			jsonResponse(w, 200, map[string]any{"binary": true})
			return
		}
		jsonResponse(w, 200, map[string]any{"binary": false, "content": string(b)})
		return
	}
	timeout := 10 * time.Minute
	if r.Action == "probe" {
		timeout = 8 * time.Second
	}
	ctx, cancel := context.WithTimeout(request.Context(), timeout)
	defer cancel()
	result, e := a.runRemoteBrowserOperation(ctx, r)
	if e != nil {
		code := 400
		if errors.Is(e, terminal.ErrNotConnected) || errors.Is(e, terminal.ErrNotFound) {
			code = 409
		}
		jsonError(w, code, e.Error())
		return
	}
	data, _ := result.Data.(map[string]any)
	if data == nil {
		data = map[string]any{}
	}
	response := map[string]any{"ok": true, "summary": result.Summary}
	if r.ConnectionID != "" {
		response["connectionId"] = r.ConnectionID
	}
	switch r.Action {
	case "probe":
		response["supported"] = data["supported"] == true
	case "fingerprint":
		if r.Protocol == "ftp" {
			response["fingerprint"] = data["certificateFingerprint"]
		} else {
			response["fingerprint"] = data["hostKeyFingerprint"]
			response["keyType"] = data["hostKeyType"]
		}
	case "list":
		type fileRecord struct {
			Name      string `json:"name"`
			Path      string `json:"path"`
			Directory bool   `json:"directory"`
			Size      any    `json:"size"`
			Modified  any    `json:"modified,omitempty"`
			Kind      string `json:"kind,omitempty"`
		}
		entries := []fileRecord{}
		if raw, ok := data["entries"]; ok {
			b, _ := json.Marshal(raw)
			var values []struct {
				Name     string `json:"name"`
				Path     string `json:"path"`
				Kind     string `json:"kind"`
				Size     any    `json:"size"`
				Modified any    `json:"modified"`
			}
			if e = json.Unmarshal(b, &values); e == nil {
				for _, v := range values {
					if v.Kind == "Link" {
						continue
					}
					entries = append(entries, fileRecord{Name: v.Name, Path: v.Path, Directory: strings.EqualFold(v.Kind, "directory"), Size: v.Size, Modified: v.Modified, Kind: v.Kind})
				}
			}
		}
		here := r.Path
		if here == "" {
			here = "."
		}
		parent := path.Dir(strings.TrimSuffix(here, "/"))
		if parent == "." && strings.HasPrefix(here, "/") {
			parent = "/"
		}
		response["entries"] = entries
		response["path"] = here
		response["parent"] = parent
	case "stat":
		response["details"] = data
	default:
		response["details"] = data
	}
	newConnectionID = ""
	jsonResponse(w, 200, response)
}

var _ = fmt.Sprintf
var _ = errors.New
