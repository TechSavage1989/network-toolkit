package main

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"networktoolkit/internal/terminal"
)

const maxTerminalViews = 8
const maxTerminalSnapshot = 8 << 20
const maxTerminalEmulatorState = 256 << 10
const terminalViewBodyLimit = 16 << 20

type terminalScreenSnapshot struct {
	OutputBlocks *terminalOutputBlocks `json:"outputBlocks,omitempty"`
	Fresh        bool                  `json:"fresh,omitempty"`
	Data         string                `json:"data"`
	Emulator     json.RawMessage       `json:"emulator"`
	Cols         int                   `json:"cols"`
	Rows         int                   `json:"rows"`
	ViewportY    int                   `json:"viewportY"`
	After        uint64                `json:"after"`
	Config       *terminalWindowPreset `json:"config"`
}

func (s *terminalScreenSnapshot) validate(latest uint64) error {
	if len(s.Data) > maxTerminalSnapshot || !utf8.ValidString(s.Data) {
		return errors.New("The terminal screen exceeds the 8 MiB transfer limit or is not valid UTF-8.")
	}
	if len(s.Emulator) > maxTerminalEmulatorState || !json.Valid(s.Emulator) || !strings.HasPrefix(strings.TrimSpace(string(s.Emulator)), "{") {
		return errors.New("The terminal emulator state is invalid or too large.")
	}
	if s.Fresh {
		var state map[string]json.RawMessage
		if s.Data != "" || json.Unmarshal(s.Emulator, &state) != nil || len(state) != 0 {
			return errors.New("A fresh terminal screen must be empty before its output is replayed.")
		}
	}
	if s.Cols < 2 || s.Cols > 500 || s.Rows < 2 || s.Rows > 300 || s.ViewportY < 0 || s.ViewportY > 100000 {
		return errors.New("The terminal screen dimensions are invalid.")
	}
	if s.After > latest {
		return errors.New("The terminal screen refers to a future output cursor.")
	}
	if s.Config == nil {
		return errors.New("The terminal connection settings are missing.")
	}
	if err := s.OutputBlocks.validate(); err != nil {
		return err
	}
	return s.Config.normalize()
}
func (s terminalScreenSnapshot) clone() terminalScreenSnapshot {
	s.OutputBlocks = s.OutputBlocks.clone()
	s.Emulator = append(json.RawMessage(nil), s.Emulator...)
	if s.Config != nil {
		c := *s.Config
		s.Config = &c
	}
	return s
}

type terminalViewInfo struct {
	Workspace  bool     `json:"workspace,omitempty"`
	SessionIDs []string `json:"sessionIds"`
	ID         string   `json:"id"`
	SessionID  string   `json:"sessionId"`
	State      string   `json:"state"`
	Revision   uint64   `json:"revision"`
	Note       string   `json:"note,omitempty"`
}
type terminalViewOwner struct {
	ViewID   string `json:"viewId"`
	Revision uint64 `json:"revision"`
	Ready    bool   `json:"ready"`
}
type terminalViewState struct {
	Available   bool                         `json:"available"`
	Views       []terminalViewInfo           `json:"views"`
	Owners      map[string]terminalViewOwner `json:"owners"`
	HomePresent bool                         `json:"homePresent"`
}
type terminalView struct {
	info            terminalViewInfo
	token           string
	snapshot        terminalScreenSnapshot
	workspace       *terminalWorkspaceSnapshot
	display         json.RawMessage
	created         time.Time
	lastPulse       time.Time
	returnRequested time.Time
}
type retiredTerminalView struct {
	token string
	until time.Time
}
type terminalViewRegistry struct {
	mu          sync.Mutex
	app         *application
	views       map[string]*terminalView
	owners      map[string]terminalViewOwner
	retired     map[string]retiredTerminalView
	changed     chan struct{}
	homePresent bool
	homeClosing bool
	children    sync.WaitGroup
}

type terminalViewContextKey struct{}

func terminalRequestView(r *http.Request) string {
	value, _ := r.Context().Value(terminalViewContextKey{}).(string)
	return value
}

func newTerminalViewRegistry(a *application) *terminalViewRegistry {
	registry := &terminalViewRegistry{app: a, views: make(map[string]*terminalView), owners: make(map[string]terminalViewOwner), retired: make(map[string]retiredTerminalView), changed: make(chan struct{}, 1), homePresent: true}
	go registry.monitor()
	return registry
}
func (v *terminalViewRegistry) notifyLocked() {
	select {
	case v.changed <- struct{}{}:
	default:
	}
}
func (v *terminalViewRegistry) ownerLocked(id string) terminalViewOwner {
	if owner, ok := v.owners[id]; ok {
		return owner
	}
	return terminalViewOwner{Ready: true}
}
func (v *terminalViewRegistry) state() terminalViewState {
	v.mu.Lock()
	defer v.mu.Unlock()
	state := terminalViewState{Available: v.app.launchTerminalView != nil, Views: []terminalViewInfo{}, Owners: make(map[string]terminalViewOwner), HomePresent: v.homePresent}
	for _, view := range v.views {
		state.Views = append(state.Views, copyTerminalViewInfo(view.info))
	}
	sort.Slice(state.Views, func(i, j int) bool { return state.Views[i].ID < state.Views[j].ID })
	for id, owner := range v.owners {
		state.Owners[id] = owner
	}
	return state
}
func (v *terminalViewRegistry) count() int { v.mu.Lock(); defer v.mu.Unlock(); return len(v.views) }
func (v *terminalViewRegistry) hasReturned() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, view := range v.views {
		if view.info.State == "returned" {
			return true
		}
	}
	return false
}
func (v *terminalViewRegistry) authenticate(token string) (string, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	for id, view := range v.views {
		if subtle.ConstantTimeCompare([]byte(token), []byte(view.token)) == 1 {
			return id, true
		}
	}
	// Successful home acceptance can precede the native child's next pulse.
	// Retired capabilities resolve only to a scoped 410, never another route.
	for id, view := range v.retired {
		if time.Now().Before(view.until) && subtle.ConstantTimeCompare([]byte(token), []byte(view.token)) == 1 {
			return id, true
		}
	}
	return "", false
}
func (v *terminalViewRegistry) get(id string) (terminalViewInfo, terminalScreenSnapshot, json.RawMessage, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	view := v.views[id]
	if view == nil {
		return terminalViewInfo{}, terminalScreenSnapshot{}, nil, false
	}
	return copyTerminalViewInfo(view.info), view.snapshot.clone(), append(json.RawMessage(nil), view.display...), true
}
func (v *terminalViewRegistry) info(id string) (terminalViewInfo, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	view := v.views[id]
	if view == nil {
		return terminalViewInfo{}, false
	}
	return copyTerminalViewInfo(view.info), true
}
func (v *terminalViewRegistry) token(id string) (string, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	view := v.views[id]
	if view == nil || view.info.State == "returned" {
		return "", false
	}
	return view.token, true
}
func (a *application) terminalSession(id string) (terminal.SessionInfo, bool) {
	if a.terminals == nil {
		return terminal.SessionInfo{}, false
	}
	for _, s := range a.terminals.List() {
		if s.ID == id {
			return s, true
		}
	}
	return terminal.SessionInfo{}, false
}
func (a *application) terminalLatest() uint64 {
	if a.terminals == nil {
		return 0
	}
	_, latest := a.terminals.EventBounds()
	return latest
}
func (a *application) terminalViewDisplay() json.RawMessage {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	var original map[string]any
	_ = json.Unmarshal(a.state, &original)
	return terminalSingleViewDisplay(original)
}

func terminalSingleViewDisplay(original map[string]any) json.RawMessage {
	display := map[string]any{"activePage": "terminal"}
	for _, key := range []string{"colorPickerPreferences", "theme", "terminalFontSize", "terminalHighlight", "terminalInteraction", "terminalActiveActionsPosition", "terminalAppearance", "terminalSidebarSide", "terminalConfirmClose"} {
		if value, ok := original[key]; ok {
			display[key] = value
		}
	}
	sanitizeTerminalSettings(display)
	sanitizeColorPickerPreferences(display)
	data, _ := json.Marshal(display)
	return data
}

func (v *terminalViewRegistry) create(sessionID string, snapshot terminalScreenSnapshot) (terminalViewInfo, error) {
	return v.createFromWorkspace("", sessionID, snapshot)
}

// A linked workspace may move one of its own sessions into a separate view.
// Ownership changes under the same lock as input and workspace checkpoints.
func (v *terminalViewRegistry) createFromWorkspace(sourceID, sessionID string, snapshot terminalScreenSnapshot) (terminalViewInfo, error) {
	a := v.app
	if a.launchTerminalView == nil {
		return terminalViewInfo{}, errors.New("Detaching a live terminal requires the native Windows app window.")
	}
	session, ok := a.terminalSession(sessionID)
	if !ok {
		return terminalViewInfo{}, terminal.ErrNotFound
	}
	if session.State != "connected" {
		return terminalViewInfo{}, errors.New("Connect the terminal and finish authentication before detaching it.")
	}
	if snapshot.Config == nil || snapshot.Config.Type != session.Type {
		return terminalViewInfo{}, errors.New("The screen settings do not match this terminal session.")
	}
	if err := snapshot.validate(a.terminalLatest()); err != nil {
		return terminalViewInfo{}, err
	}
	display := a.terminalViewDisplay()
	v.mu.Lock()
	if a.ctx.Err() != nil || a.shutdownRequested.Load() || sourceID == "" && (!v.homePresent || v.homeClosing) {
		v.mu.Unlock()
		return terminalViewInfo{}, errors.New("The source window is closing.")
	}
	if len(v.views) >= maxTerminalViews {
		v.mu.Unlock()
		return terminalViewInfo{}, errors.New("Eight terminal windows are already moving or detached.")
	}
	var source *terminalView
	var sourceLayout terminalWorkspaceSnapshot
	if sourceID != "" {
		source = v.views[sourceID]
		if source == nil || !source.info.Workspace || source.info.State != "detached" || !source.includesSession(sessionID) {
			v.mu.Unlock()
			return terminalViewInfo{}, terminal.ErrBusy
		}
		previous := source.workspace.Sessions[sessionID]
		if snapshot.After < previous.After || snapshot.Config.Type != previous.Config.Type {
			v.mu.Unlock()
			return terminalViewInfo{}, errors.New("The session screen is older than its workspace checkpoint.")
		}
		sourceLayout = terminalWorkspaceSnapshot{Panes: append([]string(nil), source.workspace.Panes...), Active: source.workspace.Active}
		var original map[string]any
		_ = json.Unmarshal(source.display, &original)
		display = terminalSingleViewDisplay(original)
	}
	for _, view := range v.views {
		if view != source && view.includesSession(sessionID) {
			v.mu.Unlock()
			return terminalViewInfo{}, errors.New("This terminal already has a detached window or a transfer in progress.")
		}
	}
	owner := v.ownerLocked(sessionID)
	if owner.ViewID != sourceID || !owner.Ready {
		v.mu.Unlock()
		return terminalViewInfo{}, errors.New("The terminal is still moving between windows.")
	}
	// The first lookup preceded display-state locking. A home close/remove
	// may have happened while we waited; reserve against the same owner lock
	// used by those mutations so an opening view always has a real session.
	session, ok = a.terminalSession(sessionID)
	if !ok {
		v.mu.Unlock()
		return terminalViewInfo{}, terminal.ErrNotFound
	}
	if session.State != "connected" || snapshot.Config.Type != session.Type {
		v.mu.Unlock()
		return terminalViewInfo{}, terminal.ErrNotConnected
	}
	now := time.Now()
	id := randomHex(16)
	view := &terminalView{info: terminalViewInfo{ID: id, SessionID: sessionID, State: "opening", Revision: owner.Revision}, token: randomHex(32), snapshot: snapshot.clone(), display: display, created: now, lastPulse: now}
	if source != nil {
		source.removeWorkspaceSession(sessionID)
		source.info.Revision++
		owner.ViewID, owner.Ready = id, false
		owner.Revision++
		v.owners[sessionID] = owner
		view.info.Revision = owner.Revision
	}
	v.views[id] = view
	v.notifyLocked()
	info := copyTerminalViewInfo(view.info)
	launch := terminalViewLaunch{Version: 1, OwnerHost: a.host, OwnerInstanceID: a.windowInstanceID, ViewID: id, ViewToken: view.token, DataDir: filepath.Join(filepath.Dir(a.dataPath), "Network_Toolkit.Views", id), Title: appName + " Terminal - " + session.Label, NoBrowser: a.headless}
	v.children.Add(1)
	v.mu.Unlock()
	done, err := a.launchTerminalView(a.ctx, launch)
	if err != nil {
		v.children.Done()
		if sourceID == "" || !v.restoreWorkspaceDetachment(sourceID, id, sessionID, snapshot, sourceLayout) {
			v.returnLast(id, "The detached window could not be opened. The existing session remains connected.")
		}
		return info, fmt.Errorf("Open detached terminal: %w", err)
	}
	if done != nil {
		go func() {
			defer v.children.Done()
			<-done
			v.returnLast(id, "The detached window closed before its final screen was saved. Restored its last display checkpoint; the session was not reconnected.")
		}()
	} else {
		v.children.Done()
	}
	return info, nil
}

func (v *terminalViewRegistry) restoreWorkspaceDetachment(sourceID, childID, sessionID string, snapshot terminalScreenSnapshot, layout terminalWorkspaceSnapshot) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	source, child := v.views[sourceID], v.views[childID]
	if source == nil || !source.info.Workspace || source.info.State != "detached" && source.info.State != "return_requested" || child == nil || child.info.State != "opening" {
		return false
	}
	delete(v.views, childID)
	v.retired[childID] = retiredTerminalView{token: child.token, until: time.Now().Add(time.Minute)}
	source.workspace.Sessions[sessionID] = snapshot.clone()
	for index, pane := range layout.Panes {
		if pane == sessionID && source.workspace.Panes[index] == "" {
			source.workspace.Panes[index] = sessionID
		}
	}
	if layout.Active == sessionID && source.workspace.Active == "" {
		source.workspace.Active = sessionID
	}
	source.updateWorkspaceIDs()
	source.info.Revision++
	owner := v.ownerLocked(sessionID)
	owner.ViewID, owner.Ready = sourceID, true
	owner.Revision++
	v.owners[sessionID] = owner
	v.notifyLocked()
	return true
}
func (v *terminalViewRegistry) ready(id string) (terminalViewInfo, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	view := v.views[id]
	if view == nil {
		return terminalViewInfo{}, terminal.ErrNotFound
	}
	if view.info.State == "detached" {
		return copyTerminalViewInfo(view.info), nil
	}
	if view.info.State != "opening" {
		return copyTerminalViewInfo(view.info), errors.New("This terminal window is already returning to its workspace.")
	}
	for _, sessionID := range view.sessionIDs() {
		owner := v.ownerLocked(sessionID)
		owner.Revision++
		owner.ViewID = id
		owner.Ready = true
		v.owners[sessionID] = owner
		if !view.info.Workspace {
			view.info.Revision = owner.Revision
		}
	}
	if view.info.Workspace {
		view.info.Revision++
	}
	view.info.State = "detached"
	view.lastPulse = time.Now()
	v.notifyLocked()
	return copyTerminalViewInfo(view.info), nil
}
func (v *terminalViewRegistry) checkpoint(id string, snapshot terminalScreenSnapshot, returning bool) (terminalViewInfo, error) {
	if err := snapshot.validate(v.app.terminalLatest()); err != nil {
		return terminalViewInfo{}, err
	}
	v.mu.Lock()
	view := v.views[id]
	if view == nil || view.info.Workspace {
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
		return terminalViewInfo{}, errors.New("The detached terminal is not ready.")
	}
	if snapshot.After < view.snapshot.After || snapshot.Config.Type != view.snapshot.Config.Type {
		v.mu.Unlock()
		return terminalViewInfo{}, errors.New("The terminal screen checkpoint is older than the saved display or belongs to another transport.")
	}
	view.snapshot = snapshot.clone()
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
func (v *terminalViewRegistry) returnLocked(view *terminalView, note string) {
	if view.info.State == "returned" {
		return
	}
	for _, sessionID := range view.sessionIDs() {
		owner := v.ownerLocked(sessionID)
		owner.ViewID = ""
		owner.Revision++
		owner.Ready = false
		v.owners[sessionID] = owner
		if !view.info.Workspace {
			view.info.Revision = owner.Revision
		}
	}
	if view.info.Workspace {
		view.info.Revision++
	}
	view.info.State = "returned"
	view.info.Note = note
	v.notifyLocked()
}
func (v *terminalViewRegistry) returnLast(id, note string) {
	v.mu.Lock()
	view := v.views[id]
	changed := view != nil && view.info.State != "returned"
	if changed {
		v.returnLocked(view, note)
	}
	v.mu.Unlock()
	if changed {
		v.app.activatePrimary()
	}
}
func (v *terminalViewRegistry) requestReturn(id string) (terminalViewInfo, error) {
	v.mu.Lock()
	view := v.views[id]
	if view == nil {
		v.mu.Unlock()
		return terminalViewInfo{}, terminal.ErrNotFound
	}
	if view.info.State == "opening" {
		v.returnLocked(view, "")
	} else if view.info.State == "detached" {
		view.info.State = "return_requested"
		view.returnRequested = time.Now()
		v.notifyLocked()
	}
	info := copyTerminalViewInfo(view.info)
	v.mu.Unlock()
	if info.State == "returned" {
		v.app.activatePrimary()
	}
	return info, nil
}
func (v *terminalViewRegistry) accept(id string, revision uint64) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	view := v.views[id]
	if view == nil {
		return terminal.ErrNotFound
	}
	if view.info.State != "returned" || revision != view.info.Revision {
		return errors.New("The terminal ownership changed. Refresh its display before resuming.")
	}
	for _, sessionID := range view.sessionIDs() {
		owner := v.ownerLocked(sessionID)
		if owner.ViewID != "" || owner.Ready || (!view.info.Workspace && owner.Revision != revision) {
			return errors.New("The terminal ownership changed. Refresh its display before resuming.")
		}
	}
	for _, sessionID := range view.sessionIDs() {
		owner := v.ownerLocked(sessionID)
		owner.Ready = true
		v.owners[sessionID] = owner
	}
	if len(v.retired) >= 32 {
		oldestID := ""
		var oldest time.Time
		for candidate, item := range v.retired {
			if oldestID == "" || item.until.Before(oldest) {
				oldestID, oldest = candidate, item.until
			}
		}
		delete(v.retired, oldestID)
	}
	v.retired[id] = retiredTerminalView{token: view.token, until: time.Now().Add(time.Minute)}
	delete(v.views, id)
	v.notifyLocked()
	return nil
}
func (v *terminalViewRegistry) pulse(id string) (terminalViewInfo, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	view := v.views[id]
	if view == nil {
		return terminalViewInfo{}, false
	}
	view.lastPulse = time.Now()
	return copyTerminalViewInfo(view.info), true
}
func (v *terminalViewRegistry) monitor() {
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	for {
		select {
		case <-v.app.ctx.Done():
			return
		case now := <-timer.C:
			activate := false
			v.mu.Lock()
			for id, view := range v.retired {
				if !now.Before(view.until) {
					delete(v.retired, id)
				}
			}
			for _, view := range v.views {
				if view.info.State == "returned" {
					continue
				}
				stale := view.info.State != "opening" && now.Sub(view.lastPulse) > 15*time.Second
				opening := view.info.State == "opening" && now.Sub(view.created) > 45*time.Second
				returning := view.info.State == "return_requested" && now.Sub(view.returnRequested) > 10*time.Second
				if stale || opening || returning {
					v.returnLocked(view, "The terminal window stopped responding. Restored its last display checkpoint; the transport remains connected.")
					activate = true
				}
			}
			v.mu.Unlock()
			if activate {
				v.app.activatePrimary()
			}
		}
	}
}

// Serialize the owner check and short queue operations with ownership changes.
// A delayed POST from a previous window cannot resize or type after a move.
func (a *application) withTerminalOwner(r *http.Request, id, action string, run func() error) error {
	revision := uint64(0)
	if value := r.Header.Get("X-Terminal-Revision"); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return errors.New("Invalid terminal ownership revision.")
		}
		revision = parsed
	}
	a.views.mu.Lock()
	defer a.views.mu.Unlock()
	owner := a.views.ownerLocked(id)
	requestView := terminalRequestView(r)
	if owner.Revision != revision || owner.ViewID != requestView || !owner.Ready {
		return terminal.ErrBusy
	}
	if requestView == "" && (!a.views.homePresent || a.views.homeClosing) {
		return terminal.ErrBusy
	}
	if requestView != "" {
		view := a.views.views[requestView]
		if view == nil || !view.includesSession(id) || (view.info.State != "detached" && view.info.State != "return_requested") {
			return terminal.ErrBusy
		}
	}
	if action == "remove" {
		for _, view := range a.views.views {
			if view.includesSession(id) && (!view.info.Workspace || requestView != view.info.ID || view.info.State != "detached") {
				return terminal.ErrBusy
			}
		}
	}
	err := run()
	if err == nil && action == "reconnect" {
		// Reject input, resizes and pending pastes sent for the old transport.
		// Window ownership itself remains unchanged.
		owner.Revision++
		a.views.owners[id] = owner
		a.views.notifyLocked()
	}
	if err == nil && action == "remove" {
		delete(a.views.owners, id)
		if view := a.views.views[requestView]; view != nil && view.info.Workspace {
			view.removeWorkspaceSession(id)
			view.info.Revision++
			a.views.notifyLocked()
		}
	}
	return err
}
