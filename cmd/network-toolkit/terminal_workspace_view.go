package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"networktoolkit/internal/terminal"
)

const maxTerminalWorkspaceSnapshot = 32 << 20
const terminalWorkspaceBodyLimit = 64 << 20
const maxTerminalWorkspaceSessions = 64

// Workspace snapshots contain screens and nonsecret presentation settings only.
// Transports, authentication and SSH trust remain owned by the original manager.
type terminalWorkspaceSnapshot struct {
	Sessions  map[string]terminalScreenSnapshot `json:"sessions"`
	Panes     []string                          `json:"panes"`
	Active    string                            `json:"active"`
	PaneFocus int                               `json:"paneFocus"`
	Maximized int                               `json:"maximized"`
	Display   map[string]any                    `json:"display,omitempty"`
}

func terminalWorkspaceDisplay(input map[string]any) map[string]any {
	clean := map[string]any{"activePage": "terminal"}
	for _, key := range []string{"terminalTabOrder", "colorPickerPreferences", "theme", "terminalFontSize", "terminalHighlight", "terminalInteraction", "terminalActiveActionsPosition", "terminalAppearance", "terminalLayout", "terminalConnection", "terminalProfiles", "terminalRecent", "terminalSidebarCollapsed", "terminalSidebarTab", "terminalSidebarSide", "terminalSidebarWidth", "terminalComposerCollapsed", "terminalMacroConfirmation", "terminalConfirmClose", "terminalRecentCollapsed", "terminalSavedCollapsed", "terminalSessionConfirmDelete"} {
		if value, ok := input[key]; ok {
			clean[key] = value
		}
	}
	if value, exists := clean["theme"]; exists {
		theme, ok := value.(string)
		if !ok || (theme != "dark" && theme != "light" && theme != "system") {
			delete(clean, "theme")
		}
	}
	if value, exists := clean["terminalTabOrder"]; exists {
		var values []string
		switch items := value.(type) {
		case []any:
			for _, item := range items {
				if id, ok := item.(string); ok {
					values = append(values, id)
				}
			}
		case []string:
			values = items
		}
		ids := []string{}
		seen := map[string]bool{}
		for _, id := range values {
			if id != "" && len(id) <= 100 && strings.IndexFunc(id, func(r rune) bool { return r < 32 || r == 127 }) < 0 && !seen[id] {
				ids = append(ids, id)
				seen[id] = true
				if len(ids) >= 128 {
					break
				}
			}
		}
		clean["terminalTabOrder"] = ids
	}
	sanitizeTerminalSettings(clean)
	sanitizeColorPickerPreferences(clean)
	return clean
}

func (s *terminalWorkspaceSnapshot) validate(latest uint64) error {
	if len(s.Sessions) > maxTerminalWorkspaceSessions {
		return errors.New("A detached workspace can retain at most 64 terminal tabs.")
	}
	if s.Sessions == nil {
		s.Sessions = map[string]terminalScreenSnapshot{}
	}
	var size int
	for id, snapshot := range s.Sessions {
		if id == "" || len(id) > 128 {
			return errors.New("The terminal workspace contains an invalid session identifier.")
		}
		if err := snapshot.validate(latest); err != nil {
			return err
		}
		size += len(snapshot.Data) + len(snapshot.Emulator)
		if snapshot.OutputBlocks != nil {
			for _, block := range snapshot.OutputBlocks.Blocks {
				if block.Archive != nil {
					size += len(block.Archive.Screen)
				}
			}
		}
		if size > maxTerminalWorkspaceSnapshot {
			return errors.New("The combined terminal screens exceed the 32 MiB workspace transfer limit.")
		}
		s.Sessions[id] = snapshot
	}
	if len(s.Panes) > 4 || s.PaneFocus < 0 || s.PaneFocus > 3 || s.Maximized < -1 || s.Maximized > 3 {
		return errors.New("The terminal workspace pane layout is invalid.")
	}
	for len(s.Panes) < 4 {
		s.Panes = append(s.Panes, "")
	}
	seen := map[string]bool{}
	for _, id := range s.Panes {
		if id == "" {
			continue
		}
		if _, ok := s.Sessions[id]; !ok || seen[id] {
			return errors.New("Each terminal pane must reference one distinct retained session.")
		}
		seen[id] = true
	}
	if s.Active != "" {
		if _, ok := s.Sessions[s.Active]; !ok {
			return errors.New("The active terminal is not part of this workspace.")
		}
	}
	if len(s.Sessions) == 0 {
		s.Active, s.Maximized = "", -1
	}
	if s.Display != nil {
		s.Display = terminalWorkspaceDisplay(s.Display)
		data, err := json.Marshal(s.Display)
		if err != nil || len(data) > 1<<20 {
			return errors.New("The terminal workspace display settings exceed their transfer limit.")
		}
	}
	return nil
}

func (s terminalWorkspaceSnapshot) clone() terminalWorkspaceSnapshot {
	copy := s
	copy.Sessions = make(map[string]terminalScreenSnapshot, len(s.Sessions))
	for id, snapshot := range s.Sessions {
		copy.Sessions[id] = snapshot.clone()
	}
	copy.Panes = append([]string(nil), s.Panes...)
	if s.Display != nil {
		data, _ := json.Marshal(s.Display)
		copy.Display = nil
		_ = json.Unmarshal(data, &copy.Display)
	}
	return copy
}

func copyTerminalViewInfo(info terminalViewInfo) terminalViewInfo {
	if info.Workspace {
		info.SessionIDs = append([]string{}, info.SessionIDs...)
	} else {
		info.SessionIDs = append([]string(nil), info.SessionIDs...)
	}
	return info
}

func (view *terminalView) sessionIDs() []string {
	if !view.info.Workspace {
		return []string{view.info.SessionID}
	}
	return view.info.SessionIDs
}

func (view *terminalView) includesSession(id string) bool {
	if !view.info.Workspace {
		return id == view.info.SessionID
	}
	_, ok := view.workspace.Sessions[id]
	return ok
}

func (view *terminalView) updateWorkspaceIDs() {
	view.info.SessionIDs = make([]string, 0, len(view.workspace.Sessions))
	for id := range view.workspace.Sessions {
		view.info.SessionIDs = append(view.info.SessionIDs, id)
	}
	sort.Strings(view.info.SessionIDs)
}

func (view *terminalView) removeWorkspaceSession(id string) {
	delete(view.workspace.Sessions, id)
	for i, pane := range view.workspace.Panes {
		if pane == id {
			view.workspace.Panes[i] = ""
		}
	}
	if view.workspace.Active == id {
		view.workspace.Active = ""
	}
	view.updateWorkspaceIDs()
}

// Called with the ownership lock immediately after a manager registration.
// The manager may evict one closed tab at its global retained-history limit.
func (v *terminalViewRegistry) pruneWorkspaceHistoryLocked() {
	retained := make(map[string]bool)
	for _, session := range v.app.terminals.List() {
		retained[session.ID] = true
	}
	for _, view := range v.views {
		if !view.info.Workspace {
			continue
		}
		changed := false
		for id := range view.workspace.Sessions {
			if !retained[id] {
				view.removeWorkspaceSession(id)
				delete(v.owners, id)
				changed = true
			}
		}
		if changed {
			view.info.Revision++
			v.notifyLocked()
		}
	}
}

func (v *terminalViewRegistry) workspaceGet(id string) (terminalViewInfo, terminalWorkspaceSnapshot, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	view := v.views[id]
	if view == nil || !view.info.Workspace {
		return terminalViewInfo{}, terminalWorkspaceSnapshot{}, false
	}
	snapshot := view.workspace.clone()
	snapshot.Display = nil
	_ = json.Unmarshal(view.display, &snapshot.Display)
	return copyTerminalViewInfo(view.info), snapshot, true
}

func (a *application) workspaceSessions(ids []string) []terminal.SessionInfo {
	wanted := make(map[string]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	sessions := []terminal.SessionInfo{}
	if a.terminals != nil {
		for _, session := range a.terminals.List() {
			if wanted[session.ID] {
				sessions = append(sessions, session)
			}
		}
	}
	return sessions
}

func (v *terminalViewRegistry) createWorkspace(snapshot terminalWorkspaceSnapshot) (terminalViewInfo, error) {
	a := v.app
	if a.launchTerminalView == nil {
		return terminalViewInfo{}, errors.New("Detaching a terminal workspace requires the native Windows app window.")
	}
	if err := snapshot.validate(a.terminalLatest()); err != nil {
		return terminalViewInfo{}, err
	}
	var display []byte
	if snapshot.Display != nil {
		display, _ = json.Marshal(snapshot.Display)
	} else {
		a.stateMu.Lock()
		var state map[string]any
		_ = json.Unmarshal(a.state, &state)
		a.stateMu.Unlock()
		display, _ = json.Marshal(terminalWorkspaceDisplay(state))
	}
	v.mu.Lock()
	if a.ctx.Err() != nil || a.shutdownRequested.Load() || !v.homePresent || v.homeClosing {
		v.mu.Unlock()
		return terminalViewInfo{}, errors.New("The source window is closing.")
	}
	if len(v.views) >= maxTerminalViews {
		v.mu.Unlock()
		return terminalViewInfo{}, errors.New("Eight terminal windows are already moving or detached.")
	}
	for _, view := range v.views {
		if view.info.Workspace {
			v.mu.Unlock()
			return terminalViewInfo{}, errors.New("The terminal workspace is already detached or returning. Reattach it before moving it again.")
		}
	}
	for id, screen := range snapshot.Sessions {
		session, found := a.terminalSession(id)
		if !found {
			v.mu.Unlock()
			return terminalViewInfo{}, terminal.ErrNotFound
		}
		if screen.Config.Type != session.Type {
			v.mu.Unlock()
			return terminalViewInfo{}, errors.New("A transferred screen belongs to another terminal transport.")
		}
		owner := v.ownerLocked(id)
		if owner.ViewID != "" || !owner.Ready {
			v.mu.Unlock()
			return terminalViewInfo{}, terminal.ErrBusy
		}
		for _, view := range v.views {
			if view.includesSession(id) {
				v.mu.Unlock()
				return terminalViewInfo{}, terminal.ErrBusy
			}
		}
	}
	now, id := time.Now(), randomHex(16)
	copy := snapshot.clone()
	view := &terminalView{info: terminalViewInfo{ID: id, State: "opening", Workspace: true}, workspace: &copy, token: randomHex(32), display: display, created: now, lastPulse: now}
	view.updateWorkspaceIDs()
	v.views[id] = view
	v.notifyLocked()
	info := copyTerminalViewInfo(view.info)
	launch := terminalViewLaunch{Version: 1, OwnerHost: a.host, OwnerInstanceID: a.windowInstanceID, ViewID: id, ViewToken: view.token, DataDir: filepath.Join(filepath.Dir(a.dataPath), terminalViewDirectoryName, id), Title: appName + " Terminal workspace", NoBrowser: a.headless}
	v.children.Add(1)
	v.mu.Unlock()
	done, err := a.launchTerminalView(a.ctx, launch)
	if err != nil {
		v.children.Done()
		v.returnLast(id, "The detached workspace could not be opened. Its connections remain in the original workspace.")
		return info, fmt.Errorf("Open detached terminal workspace: %w", err)
	}
	if done != nil {
		go func() {
			defer v.children.Done()
			<-done
			v.returnLast(id, "The detached workspace closed before its final screen was saved. Restored its last checkpoints without reconnecting the sessions.")
		}()
	} else {
		v.children.Done()
	}
	return info, nil
}

func (v *terminalViewRegistry) checkpointWorkspace(id string, snapshot terminalWorkspaceSnapshot, returning bool) (terminalViewInfo, error) {
	if err := snapshot.validate(v.app.terminalLatest()); err != nil {
		return terminalViewInfo{}, err
	}
	v.mu.Lock()
	view := v.views[id]
	if view == nil || !view.info.Workspace {
		v.mu.Unlock()
		return terminalViewInfo{}, terminal.ErrNotFound
	}
	if view.info.State == "returned" {
		info := copyTerminalViewInfo(view.info)
		v.mu.Unlock()
		return info, nil
	}
	if view.info.State != "detached" && view.info.State != "return_requested" {
		v.mu.Unlock()
		return terminalViewInfo{}, terminal.ErrBusy
	}
	for sessionID, screen := range snapshot.Sessions {
		old, found := view.workspace.Sessions[sessionID]
		if !found || screen.After < old.After || screen.Config.Type != old.Config.Type {
			v.mu.Unlock()
			return terminalViewInfo{}, errors.New("The workspace checkpoint is older than its saved screens or references another connection.")
		}
	}
	// Missing known screens must never silently drop a transport during a return.
	// Closed sessions evicted by the manager may be discarded from the display.
	for sessionID := range view.workspace.Sessions {
		if _, found := snapshot.Sessions[sessionID]; !found {
			if _, exists := v.app.terminalSession(sessionID); exists {
				v.mu.Unlock()
				return terminalViewInfo{}, errors.New("The workspace checkpoint is missing a retained terminal. Refresh its session list before returning.")
			}
		}
	}
	copy := snapshot.clone()
	view.workspace = &copy
	view.updateWorkspaceIDs()
	if snapshot.Display != nil {
		view.display, _ = json.Marshal(snapshot.Display)
	}
	if returning {
		v.returnLocked(view, "")
	}
	info := copyTerminalViewInfo(view.info)
	v.mu.Unlock()
	if returning {
		v.app.activatePrimary()
	}
	return info, nil
}

func (a *application) terminalCanConnect(r *http.Request) bool {
	id := terminalRequestView(r)
	if id == "" {
		return a.homeCanConnect()
	}
	a.views.mu.Lock()
	defer a.views.mu.Unlock()
	view := a.views.views[id]
	return view != nil && view.info.Workspace && view.info.State == "detached" && !a.shutdownRequested.Load() && a.ctx.Err() == nil
}

func (a *application) connectTerminalForRequest(r *http.Request, request terminal.ConnectRequest) (terminal.SessionInfo, error) {
	request.DisplayName = a.terminalProfileLabel(request.ProfileID)
	var err error
	request, err = a.resolveTerminalCredential(r.Context(), request)
	if err != nil {
		return terminal.SessionInfo{}, err
	}
	id := terminalRequestView(r)
	if id == "" {
		return a.connectHomeTerminal(request)
	}
	a.views.mu.Lock()
	defer a.views.mu.Unlock()
	view := a.views.views[id]
	if view == nil || !view.info.Workspace || view.info.State != "detached" || a.shutdownRequested.Load() || a.ctx.Err() != nil {
		return terminal.SessionInfo{}, terminal.ErrClosed
	}
	// Capture the cursor before Connect can produce output. If this window
	// closes before its first checkpoint, home can still replay all new output.
	after := a.terminalLatest()
	session, err := a.terminals.Connect(request)
	if err != nil {
		return terminal.SessionInfo{}, err
	}
	a.views.pruneWorkspaceHistoryLocked()
	config := &terminalWindowPreset{Type: session.Type, Host: session.Host, Port: session.Port, Username: session.Username, SSHCompatibility: session.SSHCompatibility, TimeoutMs: request.TimeoutMs}
	if session.Type == "ssh" {
		config.AuthMethod = request.AuthMethod
		if config.AuthMethod == "" && request.PrivateKey != "" {
			config.AuthMethod = "key"
		}
	}
	if session.Type == "serial" {
		config.SerialPort, config.BaudRate, config.DataBits = session.SerialPort, request.BaudRate, request.DataBits
		config.StopBits, config.Parity, config.FlowControl = request.StopBits, request.Parity, request.FlowControl
	}
	_ = config.normalize()
	view.workspace.Sessions[session.ID] = terminalScreenSnapshot{Fresh: true, Data: "", Emulator: json.RawMessage(`{}`), Cols: session.Cols, Rows: session.Rows, After: after, Config: config}
	view.workspace.Active = session.ID
	for i, pane := range view.workspace.Panes {
		if pane == "" {
			view.workspace.Panes[i] = session.ID
			break
		}
	}
	view.updateWorkspaceIDs()
	view.info.Revision++
	a.views.owners[session.ID] = terminalViewOwner{ViewID: id, Revision: 1, Ready: true}
	a.views.notifyLocked()
	return session, nil
}
