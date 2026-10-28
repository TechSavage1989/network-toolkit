package main

// A detached editor keeps its document-only capability until the main window
// explicitly approves file management. Approval captures existing transports;
// it never delegates passwords, credential storage or terminal control.
import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type editorExplorerAccess struct {
	RequestID   string
	Pending     bool
	Allowed     bool
	Remotes     map[string]*editorDocumentGrant
	Sessions    []map[string]any
	Connections []map[string]any
}

func explorerBindingKey(b editorBinding) string {
	return b.Kind + ":" + b.SessionID + ":" + b.ConnectionID
}

func (m *editorViewRegistry) explorerRequestsLocked(caller string) []map[string]any {
	rows := []map[string]any{}
	if caller != "" {
		return rows
	}
	for id, v := range m.views {
		if v.explorer != nil && (v.explorer.Pending || v.explorer.Allowed) && v.info.State != "closed" && v.info.State != "returned" {
			rows = append(rows, map[string]any{"viewId": id, "name": v.info.Name, "requestId": v.explorer.RequestID, "pending": v.explorer.Pending, "allowed": v.explorer.Allowed})
		}
	}
	return rows
}

func (m *editorViewRegistry) handleExplorerAccess(w http.ResponseWriter, r *http.Request, caller, path string) bool {
	if path != "/current/explorer" && !strings.HasPrefix(path, "/explorer/") {
		return false
	}
	if path == "/current/explorer" {
		m.mu.Lock()
		defer m.mu.Unlock()
		v := m.views[caller]
		if caller == "" || v == nil || !m.callerLocked(caller) {
			jsonError(w, 403, "Choose an active editor window.")
			return true
		}
		if r.Method == http.MethodPost {
			if v.explorer == nil || !v.explorer.Pending {
				v.explorer = &editorExplorerAccess{RequestID: randomHex(16), Pending: true}
			}
		} else if r.Method != http.MethodGet {
			methodError(w)
			return true
		}
		jsonResponse(w, 200, map[string]any{"allowed": v.explorer != nil && v.explorer.Allowed, "pending": v.explorer != nil && v.explorer.Pending})
		return true
	}
	if caller != "" {
		jsonError(w, 403, "Only the main window can approve file explorer access.")
		return true
	}
	if r.Method != http.MethodPost {
		methodError(w)
		return true
	}
	var req struct {
		RequestID string `json:"requestId"`
		Allow     bool   `json:"allow"`
	}
	if err := decodeEditorBody(w, r, 4096, &req); err != nil {
		jsonError(w, 400, err.Error())
		return true
	}
	id := strings.TrimPrefix(path, "/explorer/")
	m.mu.Lock()
	v := m.views[id]
	if !m.callerLocked("") || v == nil || v.explorer == nil || v.explorer.RequestID != req.RequestID || v.info.State == "closed" || v.info.State == "returned" {
		m.mu.Unlock()
		jsonError(w, 409, "The file explorer request is no longer current.")
		return true
	}
	m.mu.Unlock()
	access := &editorExplorerAccess{RequestID: req.RequestID, Allowed: req.Allow, Remotes: map[string]*editorDocumentGrant{}, Sessions: []map[string]any{}, Connections: []map[string]any{}}
	if req.Allow {
		for _, info := range m.app.remoteBrowserSessions() {
			b := editorBinding{Kind: "terminal", SessionID: info["id"].(string), Revision: info["revision"].(uint64)}
			grant, err := m.makeGrant("", editorDocumentSnapshot{Binding: b})
			if err == nil {
				access.Remotes[explorerBindingKey(b)] = grant
				access.Sessions = append(access.Sessions, info)
			}
		}
		for _, info := range m.app.browserConnectionInfos() {
			b := editorBinding{Kind: "sshConnection", ConnectionID: info["id"].(string)}
			grant, err := m.makeGrant("", editorDocumentSnapshot{Binding: b})
			if err == nil {
				access.Remotes[explorerBindingKey(b)] = grant
				access.Connections = append(access.Connections, info)
			}
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.callerLocked("") || m.views[id] != v || v.explorer.RequestID != req.RequestID || v.info.State == "closed" || v.info.State == "returned" {
		jsonError(w, 409, "The file explorer request changed before approval.")
		return true
	}
	v.explorer = access
	jsonResponse(w, 200, map[string]any{"ok": true})
	return true
}

func (a *application) serveApprovedEditorExplorer(w http.ResponseWriter, r *http.Request) bool {
	m := a.editorViews
	m.mu.Lock()
	v := m.views[editorRequestView(r)]
	var access *editorExplorerAccess
	if v != nil && m.callerLocked(v.info.ID) && v.explorer != nil && v.explorer.Allowed {
		access = v.explorer
	}
	m.mu.Unlock()
	if access == nil {
		return false
	}
	switch r.URL.Path {
	case "/api/remote-browser/drop-upload":
		a.handleRemoteDropUpload(w, r)
		return true
	case "/api/transfer/local":
		a.handleTransferLocal(w, r)
		return true
	case "/api/transfer/local-files":
		a.handleTransferLocalFiles(w, r)
		return true
	case "/api/remote-browser":
		if r.Method == http.MethodGet {
			sessions, connections := []map[string]any{}, []map[string]any{}
			for _, info := range access.Sessions {
				if g := access.Remotes[explorerBindingKey(editorBinding{Kind: "terminal", SessionID: info["id"].(string)})]; g != nil && g.valid() {
					sessions = append(sessions, info)
				}
			}
			for _, info := range access.Connections {
				if g := access.Remotes[explorerBindingKey(editorBinding{Kind: "sshConnection", ConnectionID: info["id"].(string)})]; g != nil && g.valid() {
					connections = append(connections, info)
				}
			}
			jsonResponse(w, 200, map[string]any{"sessions": sessions, "connections": connections})
			return true
		}
		if r.Method != http.MethodPost {
			methodError(w)
			return true
		}
		var req remoteBrowserRequest
		if err := decodeEditorBody(w, r, 12<<20, &req); err != nil {
			jsonError(w, 400, err.Error())
			return true
		}
		b := editorBinding{Kind: "sshConnection", ConnectionID: req.ConnectionID}
		if req.SessionID != "" {
			b = editorBinding{Kind: "terminal", SessionID: req.SessionID}
		}
		grant := access.Remotes[explorerBindingKey(b)]
		if browserRequestHasLogin(req) || (req.Protocol != "ssh" && req.Protocol != "sftp") || (req.SessionID != "" && req.ConnectionID != "") || grant == nil || !grant.valid() {
			jsonError(w, 403, "Choose an SFTP connection approved by the main window. Request access again after reconnecting.")
			return true
		}
		switch req.Action {
		case "list", "stat", "probe", "preview", "read-text", "mkdir", "remove", "rmdir", "rename", "copy", "chmod", "upload", "download":
		default:
			jsonError(w, 403, "Use document Save for text changes; connection control stays in the main window.")
			return true
		}
		payload, _ := json.Marshal(req)
		next := r.Clone(r.Context())
		next.Body = io.NopCloser(bytes.NewReader(payload))
		next.ContentLength = int64(len(payload))
		if b.Kind == "terminal" {
			next.Header = r.Header.Clone()
			next.Header.Set("X-Terminal-Revision", strconv.FormatUint(grant.snapshot.Binding.Revision, 10))
		}
		a.handleRemoteBrowser(w, next)
		return true
	}
	return false
}

type editorExplorerTarget struct {
	Binding editorBinding
	Path    string
}

func canonicalExplorerPath(b editorBinding, p string) string {
	if p == "" {
		return ""
	}
	if b.Kind == "local" || b.Kind == "localDraft" {
		p = filepath.Clean(p)
		if absolute, err := filepath.Abs(p); err == nil {
			p = absolute
		}
		if resolved, err := filepath.EvalSymlinks(p); err == nil {
			p = resolved
		} else {
			// A new Save As destination may not exist yet. Resolve its existing
			// parent so a junction/symlink does not hide a reserved destination.
			parent, suffix := filepath.Dir(p), filepath.Base(p)
			for parent != filepath.Dir(parent) {
				if resolved, err := filepath.EvalSymlinks(parent); err == nil {
					p = filepath.Join(resolved, suffix)
					break
				}
				suffix = filepath.Join(filepath.Base(parent), suffix)
				parent = filepath.Dir(parent)
			}
		}
		p = filepath.ToSlash(p)
		if runtime.GOOS == "windows" {
			p = strings.ToLower(p)
		}
	} else {
		// SFTP paths always use slash separators, independently of this host.
		p = path.Clean(p)
	}
	return p
}
func sameExplorerBinding(a, b editorBinding) bool {
	if (a.Kind == "local" || a.Kind == "localDraft") && (b.Kind == "local" || b.Kind == "localDraft") {
		return true
	}
	return explorerBindingKey(a) == explorerBindingKey(b)
}
func explorerTargetContains(target editorExplorerTarget, b editorBinding, p string) bool {
	if !sameExplorerBinding(target.Binding, b) {
		return false
	}
	base, current := canonicalExplorerPath(b, target.Path), canonicalExplorerPath(b, p)
	if base == "" || current == "" {
		return false
	}
	if base == "." {
		return !path.IsAbs(current) && current != ".." && !strings.HasPrefix(current, "../")
	}
	return current == base || strings.HasPrefix(current, strings.TrimRight(base, "/")+"/")
}
func (m *editorViewRegistry) explorerConflictLocked(b editorBinding, p string) bool {
	for _, targets := range m.explorerOperations {
		for _, target := range targets {
			if explorerTargetContains(target, b, p) {
				return true
			}
		}
	}
	return false
}
func (a *application) reserveExplorerMutation(targets ...editorExplorerTarget) (func(), error) {
	if a.editorViews == nil || len(targets) == 0 {
		return func() {}, nil
	}
	m := a.editorViews
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, target := range targets {
		for _, d := range m.documents {
			if explorerTargetContains(target, d.snapshot.Binding, d.snapshot.Path) || (d.filePath != "" && explorerTargetContains(target, d.snapshot.Binding, d.filePath)) {
				return nil, errors.New("Close the open editor document before changing its file or containing folder from the explorer. Unsaved work remains open.")
			}
		}
	}
	if m.explorerOperations == nil {
		m.explorerOperations = map[string][]editorExplorerTarget{}
	}
	id := randomHex(16)
	m.explorerOperations[id] = targets
	return func() { m.mu.Lock(); delete(m.explorerOperations, id); m.mu.Unlock() }, nil
}
