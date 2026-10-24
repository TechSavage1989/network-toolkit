package main

import (
	"encoding/json"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"networktoolkit/internal/model"
	"networktoolkit/internal/terminal"
)

func (a *application) serveTerminalView(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodError(w)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/terminal-view/")
	if len(id) != 32 || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	token, ok := a.views.token(id)
	if !ok {
		jsonError(w, http.StatusGone, "This detached terminal has returned to its workspace.")
		return
	}
	info, ok := a.views.info(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	b, err := web.ReadFile("web/index.html")
	if err != nil {
		jsonError(w, 500, "Interface unavailable.")
		return
	}
	bootstrap, _ := json.Marshal(a.terminalViewBootstrap(info))
	html := strings.ReplaceAll(string(b), "__SESSION_TOKEN__", token)
	html = strings.ReplaceAll(html, "__WINDOW_MODE__", "detached")
	html = strings.ReplaceAll(html, "__TERMINAL_VIEW__", string(bootstrap))
	html = embeddedAssetPaths(html)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, html)
}

func (a *application) terminalViewBootstrap(info terminalViewInfo) map[string]any {
	return map[string]any{"id": info.ID, "sessionId": info.SessionID, "workspace": info.Workspace, "sessionIds": info.SessionIDs, "ownerInstanceId": a.windowInstanceID, "homeWindowMode": a.windowMode}
}

func (a *application) handleTerminalViews(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/terminal-views")
	caller := terminalRequestView(r)
	if caller != "" {
		a.handleCurrentTerminalView(w, r, caller, path)
		return
	}
	if path == "/workspace" {
		if r.Method != http.MethodPost {
			methodError(w)
			return
		}
		var request *struct {
			Workspace *terminalWorkspaceSnapshot `json:"workspace"`
		}
		if err := decodeTerminalBody(w, r, terminalWorkspaceBodyLimit, &request); err != nil || request == nil || request.Workspace == nil {
			jsonError(w, 400, "Send a valid terminal workspace, including its screens and pane layout.")
			return
		}
		if a.launchTerminalView == nil {
			jsonError(w, 501, "Workspace detachment is available in the native Windows app.")
			return
		}
		info, err := a.views.createWorkspace(*request.Workspace)
		if err != nil {
			terminalHTTPError(w, err)
			return
		}
		response := map[string]any{"view": info, "viewState": a.terminalViewStateForRequest(r)}
		if a.headless {
			response["url"] = "http://" + a.host + "/terminal-view/" + info.ID
		}
		jsonResponse(w, http.StatusCreated, response)
		return
	}
	if path == "" {
		switch r.Method {
		case http.MethodGet:
			jsonResponse(w, 200, a.views.state())
		case http.MethodPost:
			var request *struct {
				SessionID string                 `json:"sessionId"`
				Snapshot  terminalScreenSnapshot `json:"snapshot"`
			}
			if err := decodeTerminalBody(w, r, terminalViewBodyLimit, &request); err != nil || request == nil {
				jsonError(w, 400, "Send a terminal session and a valid screen snapshot.")
				return
			}
			if a.launchTerminalView == nil {
				jsonError(w, 501, "Live detachment is available in the native Windows app.")
				return
			}
			info, err := a.views.create(request.SessionID, request.Snapshot)
			if err != nil {
				terminalHTTPError(w, err)
				return
			}
			response := map[string]any{"view": info}
			if a.headless {
				response["url"] = "http://" + a.host + "/terminal-view/" + info.ID
			}
			jsonResponse(w, http.StatusCreated, response)
		default:
			methodError(w)
		}
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) != 2 || len(parts[0]) != 32 {
		http.NotFound(w, r)
		return
	}
	id, action := parts[0], parts[1]
	switch action {
	case "snapshot":
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		info, snapshot, _, ok := a.views.get(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		if info.State != "returned" {
			jsonError(w, 409, "The detached window is still preparing its screen.")
			return
		}
		if info.Workspace {
			info, workspace, found := a.views.workspaceGet(id)
			if !found {
				http.NotFound(w, r)
				return
			}
			jsonResponse(w, 200, map[string]any{"view": info, "workspace": workspace, "sessions": a.workspaceSessions(info.SessionIDs), "viewState": a.terminalViewStateForRequest(r)})
			return
		}
		session, ok := a.terminalSession(info.SessionID)
		if !ok {
			terminalHTTPError(w, terminal.ErrNotFound)
			return
		}
		jsonResponse(w, 200, map[string]any{"view": info, "snapshot": snapshot, "session": session, "viewState": a.terminalViewStateForRequest(r)})
	case "reattach":
		if r.Method != http.MethodPost {
			methodError(w)
			return
		}
		var request struct{}
		if err := decodeTerminalBody(w, r, 1024, &request); err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		info, err := a.views.requestReturn(id)
		if err != nil {
			terminalHTTPError(w, err)
			return
		}
		jsonResponse(w, 200, map[string]any{"ok": true, "view": info})
	case "accept":
		if r.Method != http.MethodPost {
			methodError(w)
			return
		}
		var request struct {
			Revision uint64 `json:"revision"`
		}
		if err := decodeTerminalBody(w, r, 1024, &request); err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		if err := a.views.accept(id, request.Revision); err != nil {
			terminalHTTPError(w, err)
			return
		}
		jsonResponse(w, 200, map[string]any{"ok": true, "viewState": a.terminalViewStateForRequest(r)})
	default:
		http.NotFound(w, r)
	}
}

func (a *application) handleCurrentTerminalView(w http.ResponseWriter, r *http.Request, id, path string) {
	if path != "/current" && !strings.HasPrefix(path, "/current/") {
		jsonError(w, 403, "A detached window can access only its own terminal view.")
		return
	}
	info, ok := a.views.info(id)
	if !ok {
		jsonError(w, http.StatusGone, "This terminal view is no longer active.")
		return
	}
	action := strings.TrimPrefix(path, "/current")
	if action == "" {
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		response := map[string]any{"view": info, "ownerInstanceId": a.windowInstanceID, "homeWindowMode": a.windowMode, "viewState": a.terminalViewStateForRequest(r)}
		if r.URL.Query().Get("metadata") != "1" {
			if info.Workspace {
				current, workspace, found := a.views.workspaceGet(id)
				if !found {
					jsonError(w, http.StatusGone, "This terminal view is no longer active.")
					return
				}
				response["view"], response["workspace"] = current, workspace
				response["sessions"] = a.workspaceSessions(current.SessionIDs)
			} else {
				_, snapshot, _, _ := a.views.get(id)
				response["snapshot"] = snapshot
				if session, found := a.terminalSession(info.SessionID); found {
					response["session"] = session
				}
			}
		}
		jsonResponse(w, 200, response)
		return
	}
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	switch action {
	case "/detach-session":
		if !info.Workspace {
			jsonError(w, http.StatusForbidden, "Only a workspace can detach one of its own sessions.")
			return
		}
		var request *struct {
			SessionID string                 `json:"sessionId"`
			Snapshot  terminalScreenSnapshot `json:"snapshot"`
		}
		if err := decodeTerminalBody(w, r, terminalViewBodyLimit, &request); err != nil || request == nil {
			jsonError(w, http.StatusBadRequest, "Send the session and its screen snapshot.")
			return
		}
		updated, err := a.views.createFromWorkspace(id, request.SessionID, request.Snapshot)
		if err != nil {
			terminalHTTPError(w, err)
			return
		}
		jsonResponse(w, http.StatusCreated, map[string]any{"view": updated, "viewState": a.terminalViewStateForRequest(r)})
	case "/checkpoint", "/reattach":
		if info.Workspace {
			var request *struct {
				Workspace *terminalWorkspaceSnapshot `json:"workspace"`
			}
			if err := decodeTerminalBody(w, r, terminalWorkspaceBodyLimit, &request); err != nil || request == nil || request.Workspace == nil {
				jsonError(w, 400, "Send a valid terminal workspace checkpoint.")
				return
			}
			updated, err := a.views.checkpointWorkspace(id, *request.Workspace, action == "/reattach")
			if err != nil {
				terminalHTTPError(w, err)
				return
			}
			jsonResponse(w, 200, map[string]any{"ok": true, "view": updated, "viewState": a.terminalViewStateForRequest(r)})
			return
		}
		var request *struct {
			Snapshot terminalScreenSnapshot `json:"snapshot"`
		}
		if err := decodeTerminalBody(w, r, terminalViewBodyLimit, &request); err != nil || request == nil {
			jsonError(w, 400, "Send a valid terminal screen checkpoint.")
			return
		}
		updated, err := a.views.checkpoint(id, request.Snapshot, action == "/reattach")
		if err != nil {
			terminalHTTPError(w, err)
			return
		}
		jsonResponse(w, 200, map[string]any{"ok": true, "view": updated})
	case "/ready", "/pulse", "/release":
		var request struct{}
		if err := decodeTerminalBody(w, r, 1024, &request); err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		switch action {
		case "/ready":
			updated, err := a.views.ready(id)
			if err != nil {
				terminalHTTPError(w, err)
				return
			}
			jsonResponse(w, 200, map[string]any{"view": updated, "viewState": a.terminalViewStateForRequest(r)})
		case "/pulse":
			updated, found := a.views.pulse(id)
			if !found {
				jsonError(w, http.StatusGone, "This terminal view has closed.")
				return
			}
			jsonResponse(w, 200, map[string]any{"ownerInstanceId": a.windowInstanceID, "viewId": id, "state": updated.State})
		case "/release":
			a.views.returnLast(id, "The detached window closed. Its last display checkpoint was restored without reconnecting the session.")
			jsonResponse(w, 200, map[string]bool{"ok": true})
		}
	default:
		http.NotFound(w, r)
	}
}

// A view token cannot enumerate other workspaces or mutate application settings.
// A linked workspace can create connections only within its own capability.
func (a *application) serveDetachedAPI(w http.ResponseWriter, r *http.Request, id string) {
	info, ok := a.views.info(id)
	if !ok {
		jsonError(w, http.StatusGone, "This terminal view has closed.")
		return
	}
	switch {
	case r.URL.Path == "/api/status":
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		jsonResponse(w, 200, map[string]any{"name": appName, "version": model.Version, "os": runtime.GOOS, "arch": runtime.GOARCH, "uiHost": a.uiHost, "windowMode": "detached", "terminalWindowAvailable": false, "terminalDetachAvailable": info.Workspace && a.launchTerminalView != nil, "started": a.started.UTC().Format(time.RFC3339), "portable": true, "storageWritable": false, "storageError": "", "capabilities": []string{}, "system": map[string]any{"interfaces": []any{}}, "terminalView": a.terminalViewBootstrap(info)})
	case r.URL.Path == "/api/state":
		a.handleDetachedState(w, r, id)
	case strings.HasPrefix(r.URL.Path, "/api/terminal-automation/"):
		a.handleTerminalAutomation(w, r)
	case info.Workspace && (r.URL.Path == "/api/credentials" || strings.HasPrefix(r.URL.Path, "/api/credentials/")):
		a.handleCredentials(w, r)
	case r.URL.Path == "/api/terminal-views/current" || strings.HasPrefix(r.URL.Path, "/api/terminal-views/current/"):
		a.handleTerminalViews(w, r)
	case r.URL.Path == "/api/terminals":
		if r.Method != http.MethodGet && !(info.Workspace && r.Method == http.MethodPost) {
			jsonError(w, 403, "A detached view cannot create another connection.")
			return
		}
		a.handleTerminals(w, r)
	case r.URL.Path == "/api/terminals/events":
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		a.handleTerminals(w, r)
	case info.Workspace && r.URL.Path == "/api/terminals/serial-ports" && r.Method == http.MethodGet:
		a.handleTerminals(w, r)
	case terminalViewPermitsSessionPath(info, r.URL.Path):
		if info.State != "detached" && info.State != "return_requested" {
			jsonError(w, 409, "This terminal is moving between windows.")
			return
		}
		a.handleTerminals(w, r)
	case r.URL.Path == "/api/shutdown":
		if r.Method != http.MethodPost {
			methodError(w)
			return
		}
		a.views.returnLast(id, "The detached window returned to its workspace using its last display checkpoint.")
		jsonResponse(w, 200, map[string]bool{"ok": true})
	default:
		jsonError(w, 403, "This detached window can access only the terminal sessions assigned to it.")
	}
}

func terminalViewPermitsSessionPath(info terminalViewInfo, path string) bool {
	if !strings.HasPrefix(path, "/api/terminals/") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(path, "/api/terminals/"), "/")
	if len(parts) < 1 || len(parts) > 2 || parts[0] == "" {
		return false
	}
	if !info.Workspace {
		return len(parts) == 2 && parts[0] == info.SessionID
	}
	for _, id := range info.SessionIDs {
		if parts[0] == id {
			return true
		}
	}
	return false
}

func (a *application) handleDetachedState(w http.ResponseWriter, r *http.Request, id string) {
	info, snapshot, display, ok := a.views.get(id)
	if !ok {
		jsonError(w, 410, "This terminal view has closed.")
		return
	}
	switch r.Method {
	case http.MethodGet:
		state := map[string]any{}
		_ = json.Unmarshal(display, &state)
		state["activePage"] = "terminal"
		if !info.Workspace {
			state["terminalConnection"] = snapshot.Config
		}
		jsonResponse(w, 200, state)
	case http.MethodPut:
		if info.State == "returned" {
			jsonError(w, 409, "The terminal has returned to its workspace.")
			return
		}
		var original map[string]any
		if err := decodeBody(w, r, 2<<20, &original); err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		state := terminalWorkspaceDisplay(original)
		if !info.Workspace {
			// Single-session views retain their narrow presentation-only scope.
			for _, key := range []string{"terminalConnection", "terminalProfiles", "terminalRecent", "terminalRecentCollapsed", "terminalSavedCollapsed", "terminalSessionConfirmDelete", "terminalLayout"} {
				delete(state, key)
			}
		}
		data, _ := json.Marshal(state)
		a.views.mu.Lock()
		view := a.views.views[id]
		if view == nil || view.info.State == "returned" {
			a.views.mu.Unlock()
			jsonError(w, 409, "The terminal has returned to its workspace.")
			return
		}
		view.display = data
		a.views.mu.Unlock()
		jsonResponse(w, 200, map[string]bool{"ok": true})
	default:
		methodError(w)
	}
}

func (a *application) terminalListForRequest(r *http.Request) []terminal.SessionInfo {
	sessions := a.terminals.List()
	id := terminalRequestView(r)
	if id == "" {
		return sessions
	}
	info, ok := a.views.info(id)
	if !ok {
		return []terminal.SessionInfo{}
	}
	wanted := terminalViewSessionSet(info)
	filtered := []terminal.SessionInfo{}
	for _, session := range sessions {
		if wanted[session.ID] {
			filtered = append(filtered, session)
		}
	}
	return filtered
}

func terminalViewSessionSet(info terminalViewInfo) map[string]bool {
	wanted := map[string]bool{}
	if info.Workspace {
		for _, id := range info.SessionIDs {
			wanted[id] = true
		}
	} else if info.SessionID != "" {
		wanted[info.SessionID] = true
	}
	return wanted
}

func (a *application) terminalBatchForRequest(r *http.Request, batch terminal.Batch) terminal.Batch {
	id := terminalRequestView(r)
	if id == "" {
		return batch
	}
	info, ok := a.views.info(id)
	wanted := map[string]bool{}
	if ok {
		wanted = terminalViewSessionSet(info)
	}
	events := make([]terminal.Event, 0, len(batch.Events))
	for _, event := range batch.Events {
		if wanted[event.SessionID] {
			events = append(events, event)
		}
	}
	sessions := make([]terminal.SessionInfo, 0, len(wanted))
	for _, session := range batch.Sessions {
		if wanted[session.ID] {
			sessions = append(sessions, session)
		}
	}
	batch.Events = events
	batch.Sessions = sessions
	return batch
}

func (a *application) terminalViewStateForRequest(r *http.Request) terminalViewState {
	state := a.views.state()
	id := terminalRequestView(r)
	if id == "" {
		return state
	}
	filtered := terminalViewState{Available: false, Views: []terminalViewInfo{}, Owners: map[string]terminalViewOwner{}, HomePresent: state.HomePresent}
	for _, view := range state.Views {
		if view.ID == id {
			filtered.Available = view.Workspace && state.Available
			filtered.Views = append(filtered.Views, view)
			for sessionID := range terminalViewSessionSet(view) {
				if owner, ok := state.Owners[sessionID]; ok {
					filtered.Owners[sessionID] = owner
				}
			}
		}
	}
	return filtered
}
