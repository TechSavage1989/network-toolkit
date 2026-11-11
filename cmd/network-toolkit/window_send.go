package main

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/ssh"

	"networktoolkit/internal/diag"
	"networktoolkit/internal/model"
)

// Window send: "Send to terminal" and "Send to Text Editor" between the
// windows of ONE engine (owner request: "all the single toolkit app features
// between them must continue to work"). The main Toolkit window and its
// detached Terminal and Text Editor windows share this engine; a detached
// window runs as the separate program (edition.go) but its page, sessions and
// documents stay here. A send from one of these windows can reach a session
// or a document shown in another of them.
//
// This is not peer send (peer_send.go): nothing crosses to another engine,
// no instance record or activation token is used, and the routes live under
// each window's own session or view token. Child windows still have no peer
// routes.
//
// The protocol is peer send's, inside the engine:
//   - Every window's page polls /api/window-send/inbox once a second and
//     reports what it can receive (connected sessions it shows and owns; open
//     documents) when asked and when that changes.
//   - A sender asks /api/window-send/targets for the other windows' lists and
//     sends {kind, window, session|target, ...} to /api/window-send/send.
//   - The engine checks that the window still lists the target and, for a
//     terminal session, that the window still owns it (the ownership that
//     withTerminalOwner checks for input); for a document held by the editor
//     registry, that it is not moving. It queues the request for that window
//     only (one pending request per window, one minute at most).
//   - The receiving page runs the same code as a send inside one window
//     (peer-send.js receiveTerminal/receiveEditor: pasteText with paste
//     review, warnings and bracketed paste; newDocument/insertText/pickInsert)
//     and never replaces an open dialog (blocked()). It acknowledges; the
//     sender reads the result.
//
// Only text travels, never credentials, host keys, file paths or an Enter.
//
// One more kind, "remote" (owner decision): Edit in the SFTP browser of a
// detached Terminal workspace opens that remote file in the main window's
// Text Editor, on the same SSH connection. The workspace asks; the engine
// checks that the workspace owns the session and records a grant for that
// one path on that exact SSH transport; the main page opens the file with the
// main window's own SFTP editor code (/api/remote-browser read-text and
// write-text with the same hash, encoding and staging checks). The grant
// admits only reading and saving that path from the main window
// (remoteBrowserGrant, makeGrant), on the pinned transport (windowSendRun),
// while that tab is open and the session stays in the asking workspace; Save
// As elsewhere and moving the tab out of the main window are refused. Nothing
// else of the session becomes the main window's.
const (
	windowSendPrefix   = "/api/window-send/"
	windowSendMain     = "main"
	windowSendTerminal = "terminal:"
	windowSendEditor   = "editor:"
	windowSendKept     = 16
	windowSendGrants   = 32
	windowSendPathMax  = 4096
	windowSendForget   = 2 * time.Minute
)

var windowSendNow = time.Now

type windowSendWindow struct {
	targets  map[string][]peerTarget // by kind: what the page last reported
	reported bool
	polledAt time.Time
	request  *windowSendRequest // the one request waiting for this window
}

type windowSendRequest struct {
	id, kind, from, to string
	created            time.Time
	state              string // peerQueued, peerTaken, peerDone, peerFailed, peerExpired
	outcome, message   string
	payload            *windowSendPayload // nil once the page has it or the request ended
}

// windowSendPayload is what the receiving page gets: peer send's request, plus
// the remote path for kind "remote".
type windowSendPayload struct {
	peerPayload
	Path string `json:"path,omitempty"`
}

// windowSendGrant lets the main window edit one remote file of a session that a
// detached Terminal workspace owns, on the SSH transport it was granted on.
type windowSendGrant struct {
	session, path string
	view          string // the Terminal workspace that asked; the session must stay its own
	request       string // the request that opened it; refused or expired drops the grant
	document      string // the main window's tab, once confirmed; closing it drops the grant
	confirmed     bool
	client        *ssh.Client
	lifetime      context.Context
	dropped       atomic.Bool
}

var (
	errWindowSendSaveElsewhere = errors.New("This file was opened from a popped-out terminal window, so it can be saved only to the same file. Save a copy locally, or open the file from the main window's own connection to save it elsewhere.")
	errWindowSendGone          = errors.New("This file was opened through the popped-out terminal, which no longer holds the connection; reopen it from here to save.")
	errWindowSendStays         = errors.New("This file was opened from a popped-out terminal window and stays in the main window. Save a copy locally to move it.")
)

type windowSendState struct {
	mu       sync.Mutex
	windows  map[string]*windowSendWindow
	requests []*windowSendRequest // recent requests, newest last
	grants   []*windowSendGrant   // remote files opened from detached workspaces, newest last
}

// windowSendAvailable: only the main Toolkit engine has windows of both
// tools. A standalone app's detached windows show its own tool only.
func (a *application) windowSendAvailable() bool { return a.windowMode == "main" }

// windowSendReceives reports whether a window takes text of a kind.
func windowSendReceives(key, kind string) bool {
	switch {
	case key == windowSendMain:
		return kind == "terminal" || kind == "editor" || kind == "remote"
	case strings.HasPrefix(key, windowSendTerminal):
		return kind == "terminal"
	case strings.HasPrefix(key, windowSendEditor):
		return kind == "editor"
	}
	return false
}

// windowSendAlive reports whether the window still exists in this engine:
// the main page is present and not closing; a detached Terminal window holds
// its sessions; a detached editor window is open.
func (a *application) windowSendAlive(key string) bool {
	switch {
	case key == windowSendMain:
		a.views.mu.Lock()
		ok := a.views.homePresent && !a.views.homeClosing
		a.views.mu.Unlock()
		if ok && a.editorViews != nil {
			a.editorViews.mu.Lock()
			ok = a.editorViews.homePresent && !a.editorViews.homeClosing
			a.editorViews.mu.Unlock()
		}
		return ok
	case strings.HasPrefix(key, windowSendTerminal):
		info, ok := a.views.info(strings.TrimPrefix(key, windowSendTerminal))
		return ok && info.State == "detached"
	case strings.HasPrefix(key, windowSendEditor):
		if a.editorViews == nil {
			return false
		}
		info, ok := a.editorViews.info(strings.TrimPrefix(key, windowSendEditor))
		return ok && info.State == "ready"
	}
	return false
}

// windowSendNames names each live window the way the user sees it. Windows of
// one kind are numbered in the order of their identifiers when there are
// several.
func windowSendNames(keys []string) map[string]string {
	sort.Strings(keys)
	count := map[string]int{}
	kind := func(key string) string {
		switch {
		case strings.HasPrefix(key, windowSendTerminal):
			return "Terminal window"
		case strings.HasPrefix(key, windowSendEditor):
			return "Text Editor window"
		}
		return "Main window"
	}
	for _, key := range keys {
		count[kind(key)]++
	}
	seen := map[string]int{}
	names := make(map[string]string, len(keys))
	for _, key := range keys {
		name := kind(key)
		if count[name] > 1 {
			seen[name]++
			name += " " + strconv.Itoa(seen[name])
		}
		names[key] = name
	}
	return names
}

func (s *windowSendState) windowLocked(key string) *windowSendWindow {
	if s.windows == nil {
		s.windows = map[string]*windowSendWindow{}
	}
	w := s.windows[key]
	if w == nil {
		w = &windowSendWindow{}
		s.windows[key] = w
	}
	return w
}

func (s *windowSendState) expireLocked(now time.Time) {
	for _, r := range s.requests {
		if (r.state != peerQueued && r.state != peerTaken) || now.Sub(r.created) < peerRequestLifetime {
			continue
		}
		if r.state == peerQueued {
			r.message = "The window did not pick the text up in time. Nothing was sent."
		} else {
			r.message = "The window did not confirm what happened to the text. Check that window."
		}
		r.state, r.payload = peerExpired, nil
	}
	for _, g := range append([]*windowSendGrant(nil), s.grants...) {
		if g.confirmed {
			continue
		}
		known := false
		for _, r := range s.requests {
			if r.id == g.request {
				known = r.state == peerQueued || r.state == peerTaken || r.state == peerDone
			}
		}
		if !known {
			s.dropGrantLocked(g)
		}
	}
	for key, w := range s.windows {
		if w.request != nil && w.request.state != peerQueued && w.request.state != peerTaken {
			w.request = nil
		}
		if w.request == nil && now.Sub(w.polledAt) > windowSendForget {
			delete(s.windows, key)
		}
	}
}

// liveLocked lists the windows that can receive now: present in the engine,
// polling, and with a reported summary. Called with s.mu held; the alive
// check takes the registries' own locks, which never take s.mu.
func (a *application) windowSendLiveLocked(now time.Time) []string {
	var keys []string
	for key, w := range a.windowSend.windows {
		if !w.reported || w.polledAt.IsZero() || now.Sub(w.polledAt) > peerTargetsFresh || now.Before(w.polledAt) {
			continue
		}
		if a.windowSendAlive(key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

type windowSendList struct {
	Window  string       `json:"window"`
	Name    string       `json:"name"`
	Targets []peerTarget `json:"targets"`
}

// windowSendTargets lists, for the calling window, what the other live
// windows can receive of a kind, and which window takes a new document.
func (a *application) windowSendTargets(caller, kind string) (lists []windowSendList, newDocument, newName string, others int) {
	now := windowSendNow()
	a.windowSend.mu.Lock()
	defer a.windowSend.mu.Unlock()
	a.windowSend.expireLocked(now)
	live := a.windowSendLiveLocked(now)
	names := windowSendNames(append([]string(nil), live...))
	lists = []windowSendList{}
	for _, key := range live {
		if key == caller {
			continue
		}
		others++
		if !windowSendReceives(key, kind) {
			continue
		}
		targets := append([]peerTarget(nil), a.windowSend.windows[key].targets[kind]...)
		if targets == nil {
			targets = []peerTarget{}
		}
		lists = append(lists, windowSendList{Window: key, Name: names[key], Targets: targets})
		// A new document goes to the main window's editor, else to the first
		// detached editor window. A window with its own editor keeps its own.
		if kind == "editor" && caller != windowSendMain && !strings.HasPrefix(caller, windowSendEditor) && (newDocument == "" || key == windowSendMain) {
			newDocument, newName = key, names[key]
		}
	}
	return lists, newDocument, newName, others
}

// windowSendOwned checks, at the moment of sending, that the target window
// still owns the session (the ownership withTerminalOwner checks for every
// input) or, for a document held by the editor registry, that the window
// holds it and it is not moving. A message says what changed.
func (a *application) windowSendOwned(key, kind, id string) error {
	if kind == "terminal" {
		want := ""
		if strings.HasPrefix(key, windowSendTerminal) {
			want = strings.TrimPrefix(key, windowSendTerminal)
		}
		a.views.mu.Lock()
		defer a.views.mu.Unlock()
		owner := a.views.ownerLocked(id)
		if owner.ViewID != want || !owner.Ready {
			return errors.New("That terminal session moved to another window. Choose Send to terminal again.")
		}
		if want != "" {
			view := a.views.views[want]
			if view == nil || !view.includesSession(id) || view.info.State != "detached" {
				return errors.New("That terminal session moved to another window. Choose Send to terminal again.")
			}
		}
		return nil
	}
	if id == "new" || a.editorViews == nil {
		return nil
	}
	want := ""
	if strings.HasPrefix(key, windowSendEditor) {
		want = strings.TrimPrefix(key, windowSendEditor)
	}
	a.editorViews.mu.Lock()
	defer a.editorViews.mu.Unlock()
	if grant := a.editorViews.documents[id]; grant != nil && (grant.owner != want || grant.pending != "") {
		return errors.New("That document moved to another window. Choose Send to Text Editor again.")
	}
	return nil
}

type windowSendBody struct {
	Kind    string `json:"kind"`
	Window  string `json:"window"`
	Session string `json:"session,omitempty"`
	Target  string `json:"target,omitempty"`
	Where   string `json:"where,omitempty"`
	Line    int    `json:"line,omitempty"`
	Label   string `json:"label,omitempty"`
	Text    string `json:"text"`
	Path    string `json:"path,omitempty"`
}

// windowSendEnqueue queues a validated request for one other window.
func (a *application) windowSendEnqueue(caller string, body windowSendBody) (id, name string, status int, err error) {
	kind := body.Kind
	if kind == "remote" {
		return a.windowSendRemoteEnqueue(caller, body)
	}
	if body.Path != "" {
		return "", "", http.StatusBadRequest, errors.New("Text carries no file path.")
	}
	payload := peerPayload{RequestID: randomHex(16), Session: body.Session, Target: body.Target, Where: body.Where, Line: body.Line, Label: body.Label, Text: body.Text}
	if err := validatePeerPayload(kind, &payload); err != nil {
		return "", "", http.StatusBadRequest, err
	}
	if body.Window == caller {
		return "", "", http.StatusBadRequest, errors.New("Choose a session or document in another window.")
	}
	if !windowSendReceives(body.Window, kind) {
		return "", "", http.StatusBadRequest, errors.New("That window cannot take this text.")
	}
	targetID := payload.Session
	if kind == "editor" {
		targetID = payload.Target
	}
	// Checked here, before anything is queued; the receiving page's paste or
	// edit is checked again by the engine's own routes (withTerminalOwner,
	// the document lease), so a move after this point sends nothing.
	if err := a.windowSendOwned(body.Window, kind, targetID); err != nil {
		return "", "", http.StatusConflict, err
	}
	now := windowSendNow()
	a.windowSend.mu.Lock()
	defer a.windowSend.mu.Unlock()
	a.windowSend.expireLocked(now)
	live := a.windowSendLiveLocked(now)
	names := windowSendNames(append([]string(nil), live...))
	found := false
	for _, key := range live {
		found = found || key == body.Window
	}
	if !found {
		return "", "", http.StatusConflict, errors.New("That window is closed or not ready. Choose Send to again.")
	}
	name = names[body.Window]
	w := a.windowSend.windows[body.Window]
	if kind == "terminal" || payload.Target != "new" {
		var target *peerTarget
		for i := range w.targets[kind] {
			if w.targets[kind][i].ID == targetID {
				target = &w.targets[kind][i]
			}
		}
		switch {
		case target == nil && kind == "terminal":
			return "", "", http.StatusConflict, errors.New("That terminal session is no longer connected in " + name + ". Choose another session.")
		case target == nil:
			return "", "", http.StatusConflict, errors.New("That document is no longer open in " + name + ". Choose another.")
		case target.Busy:
			return "", "", http.StatusConflict, errors.New("That document cannot take text right now. Choose another or try again.")
		}
	}
	if current := w.request; current != nil && (current.state == peerQueued || current.state == peerTaken) {
		return "", "", http.StatusConflict, errors.New("Another send is still waiting in " + name + ". Finish it there, then send again.")
	}
	a.windowSendQueueLocked(w, &windowSendRequest{id: payload.RequestID, kind: kind, from: caller, to: body.Window, created: now, state: peerQueued, payload: &windowSendPayload{peerPayload: payload}})
	return payload.RequestID, name, http.StatusOK, nil
}

// windowSendQueueLocked queues a request for a window and keeps the list of
// recent requests short.
func (a *application) windowSendQueueLocked(w *windowSendWindow, request *windowSendRequest) {
	w.request = request
	a.windowSend.requests = append(a.windowSend.requests, request)
	if extra := len(a.windowSend.requests) - windowSendKept; extra > 0 {
		kept := a.windowSend.requests[:0]
		for _, r := range a.windowSend.requests {
			if extra > 0 && r.state != peerQueued && r.state != peerTaken {
				extra--
				continue
			}
			kept = append(kept, r)
		}
		a.windowSend.requests = kept
	}
	// The text never outlives the request, even if the receiving page is gone.
	time.AfterFunc(peerRequestLifetime+time.Second, func() {
		a.windowSend.mu.Lock()
		defer a.windowSend.mu.Unlock()
		a.windowSend.expireLocked(windowSendNow())
	})
}

// windowSendRemoteEnqueue: a detached Terminal workspace asks the main window
// to open one remote file of a session it owns (Edit in its SFTP browser).
func (a *application) windowSendRemoteEnqueue(caller string, body windowSendBody) (id, name string, status int, err error) {
	if !strings.HasPrefix(caller, windowSendTerminal) {
		return "", "", http.StatusBadRequest, errors.New("Remote files are opened from a Terminal window's SFTP browser.")
	}
	if body.Window != windowSendMain {
		return "", "", http.StatusBadRequest, errors.New("Remote files open in the main window's Text Editor.")
	}
	if body.Text != "" || body.Target != "" || body.Where != "" || body.Line != 0 || body.Label != "" {
		return "", "", http.StatusBadRequest, errors.New("Opening a remote file carries a session and a path only.")
	}
	path := body.Path
	if path == "" || len(path) > windowSendPathMax || !utf8.ValidString(path) || strings.ContainsFunc(path, unicode.IsControl) || !peerIDPattern.MatchString(body.Session) {
		return "", "", http.StatusBadRequest, errors.New("Choose one remote file.")
	}
	view := strings.TrimPrefix(caller, windowSendTerminal)
	grant := &windowSendGrant{session: body.Session, path: path, view: view}
	a.views.mu.Lock()
	owner := a.views.ownerLocked(body.Session)
	v := a.views.views[view]
	if owner.ViewID != view || !owner.Ready || v == nil || !v.info.Workspace || !v.includesSession(body.Session) || v.info.State != "detached" {
		a.views.mu.Unlock()
		return "", "", http.StatusConflict, errors.New("That SSH session is not in this window's workspace.")
	}
	client, lifetime, clientErr := a.terminals.SFTPClient(body.Session)
	a.views.mu.Unlock()
	if clientErr != nil || lifetime == nil || lifetime.Err() != nil {
		return "", "", http.StatusConflict, errors.New("The SSH session is not connected.")
	}
	grant.client, grant.lifetime = client, lifetime
	now := windowSendNow()
	a.windowSend.mu.Lock()
	defer a.windowSend.mu.Unlock()
	a.windowSend.expireLocked(now)
	live := a.windowSendLiveLocked(now)
	names := windowSendNames(append([]string(nil), live...))
	found := false
	for _, key := range live {
		found = found || key == windowSendMain
	}
	if !found {
		return "", "", http.StatusConflict, errors.New("Open the main Network Toolkit window to edit remote files, then choose Edit again.")
	}
	name = names[windowSendMain]
	w := a.windowSend.windows[windowSendMain]
	if current := w.request; current != nil && (current.state == peerQueued || current.state == peerTaken) {
		return "", "", http.StatusConflict, errors.New("Another request is still waiting in " + name + ". Finish it there, then try again.")
	}
	id = randomHex(16)
	grant.request = id
	for _, g := range a.windowSend.grants {
		if g.lifetime.Err() != nil || g.session == grant.session && g.path == grant.path && !g.confirmed {
			a.windowSend.dropGrantLocked(g)
		}
	}
	a.windowSend.grants = append(a.windowSend.grants, grant)
	// Known limit: at most windowSendGrants files at once; the oldest ends first.
	for len(a.windowSend.grants) > windowSendGrants {
		a.windowSend.dropGrantLocked(a.windowSend.grants[0])
	}
	a.windowSendQueueLocked(w, &windowSendRequest{id: id, kind: "remote", from: caller, to: windowSendMain, created: now, state: peerQueued, payload: &windowSendPayload{peerPayload: peerPayload{RequestID: id, Session: body.Session}, Path: path}})
	return id, name, http.StatusOK, nil
}

// dropGrantLocked ends a grant: no save path admits it any more.
func (s *windowSendState) dropGrantLocked(g *windowSendGrant) {
	g.dropped.Store(true)
	kept := s.grants[:0]
	for _, other := range s.grants {
		if other != g {
			kept = append(kept, other)
		}
	}
	s.grants = kept
}

// windowSendGrantFor returns the grant for one remote file, if any.
func (a *application) windowSendGrantFor(session, path string) *windowSendGrant {
	a.windowSend.mu.Lock()
	defer a.windowSend.mu.Unlock()
	for i := len(a.windowSend.grants) - 1; i >= 0; i-- {
		if g := a.windowSend.grants[i]; g.session == session && g.path == path {
			return g
		}
	}
	return nil
}

// windowSendGrantLiveLocked (a.views.mu held) reports whether a grant still
// stands: the session is still in a detached Terminal workspace, on the SSH
// transport the grant was given on.
func (a *application) windowSendGrantLiveLocked(g *windowSendGrant) bool {
	if g == nil || g.dropped.Load() || g.lifetime.Err() != nil {
		return false
	}
	owner := a.views.ownerLocked(g.session)
	v := a.views.views[owner.ViewID]
	if owner.ViewID != g.view || !owner.Ready || v == nil || !v.info.Workspace || !v.includesSession(g.session) || (v.info.State != "detached" && v.info.State != "return_requested") {
		return false
	}
	client, lifetime, err := a.terminals.SFTPClient(g.session)
	return err == nil && client == g.client && lifetime == g.lifetime
}

// remoteBrowserGrant decides whether the main window may run action on path
// of a session that a detached Terminal workspace owns, because that workspace
// opened the file in the main window's Text Editor: only read-text and
// write-text of exactly that path, while the grant stands. It returns the
// grant, whose pinned transport the operation must use, or a plain reason when
// a grant exists for the session but not for this (Save As elsewhere, or the
// workspace no longer holds the connection).
func (a *application) remoteBrowserGrant(r *http.Request, session, action, path string) (*windowSendGrant, error) {
	if !a.windowSendAvailable() || terminalRequestView(r) != "" || editorRequestView(r) != "" || a.views == nil || a.terminals == nil {
		return nil, nil
	}
	grant := a.windowSendGrantFor(session, path)
	if grant == nil {
		if action == "write-text" && a.windowSendSessionGranted(session) {
			return nil, errWindowSendSaveElsewhere
		}
		return nil, nil
	}
	if action != "read-text" && action != "write-text" {
		return nil, nil
	}
	a.views.mu.Lock()
	live := a.windowSendGrantLiveLocked(grant)
	a.views.mu.Unlock()
	if !live {
		return nil, errWindowSendGone
	}
	return grant, nil
}

// windowSendSessionGranted reports whether any file of the session was opened
// from a workspace.
func (a *application) windowSendSessionGranted(session string) bool {
	a.windowSend.mu.Lock()
	defer a.windowSend.mu.Unlock()
	for _, g := range a.windowSend.grants {
		if g.session == session {
			return true
		}
	}
	return false
}

// windowSendRun runs one SFTP operation for a grant on the transport it was
// checked against: only reading, checking and replacing exactly the approved
// file (read-text, stat, download, upload), never another path, a rename or a
// listing, and only while the grant stands.
func (a *application) windowSendRun(g *windowSendGrant) func(context.Context, remoteBrowserRequest) (model.Result, error) {
	return func(ctx context.Context, req remoteBrowserRequest) (model.Result, error) {
		switch req.Action {
		case "read-text", "stat", "download", "upload":
		default:
			return model.Result{}, errWindowSendSaveElsewhere
		}
		if req.Path != g.path || req.NewPath != "" || req.SessionID != "" && req.SessionID != g.session || req.ConnectionID != "" {
			return model.Result{}, errWindowSendSaveElsewhere
		}
		a.views.mu.Lock()
		live := a.windowSendGrantLiveLocked(g)
		a.views.mu.Unlock()
		if !live {
			return model.Result{}, errWindowSendGone
		}
		operation, cancel := context.WithCancel(ctx)
		defer cancel()
		stop := context.AfterFunc(g.lifetime, cancel)
		defer stop()
		req.SessionID = ""
		return diag.RunSFTPOnClient(operation, g.client, req.request(), nil)
	}
}

// windowSendPoll is a page's inbox poll: it proves the page runs, hands over
// the request waiting for it, and says whether the engine needs its summary.
func (a *application) windowSendPoll(key string) (payload *windowSendPayload, kind string, report bool, others int) {
	now := windowSendNow()
	a.windowSend.mu.Lock()
	defer a.windowSend.mu.Unlock()
	w := a.windowSend.windowLocked(key)
	w.polledAt = now
	a.windowSend.expireLocked(now)
	for _, other := range a.windowSendLiveLocked(now) {
		if other != key {
			others++
		}
	}
	report = !w.reported
	r := w.request
	if r == nil || r.state != peerQueued || r.payload == nil {
		return nil, "", report, others
	}
	payload = r.payload
	r.state, r.payload = peerTaken, nil
	return payload, r.kind, report, others
}

// windowSendKeep keeps only what the window may receive: a detached Terminal
// window lists sessions it holds, the main window sessions no detached
// window holds, and only the main window and editor windows list documents.
func (a *application) windowSendKeep(key string, sessions, documents []peerTarget) ([]peerTarget, []peerTarget) {
	if !windowSendReceives(key, "terminal") {
		sessions = nil
	}
	if !windowSendReceives(key, "editor") {
		documents = nil
	}
	if len(sessions) > 0 {
		want := strings.TrimPrefix(key, windowSendTerminal)
		if key == windowSendMain {
			want = ""
		}
		a.views.mu.Lock()
		kept := sessions[:0]
		for _, s := range sessions {
			if a.views.ownerLocked(s.ID).ViewID == want {
				kept = append(kept, s)
			}
		}
		a.views.mu.Unlock()
		sessions = kept
	}
	return sessions, documents
}

func (a *application) windowSendReport(key string, sessions, documents []peerTarget) {
	sessions, documents = a.windowSendKeep(key, sessions, documents)
	a.windowSend.mu.Lock()
	defer a.windowSend.mu.Unlock()
	w := a.windowSend.windowLocked(key)
	w.targets = map[string][]peerTarget{"terminal": sessions, "editor": documents}
	w.reported = true
	if key == windowSendMain {
		open := map[string]bool{}
		for _, d := range documents {
			open[d.ID] = true
		}
		for _, g := range append([]*windowSendGrant(nil), a.windowSend.grants...) {
			if g.confirmed && !open[g.document] {
				a.windowSend.dropGrantLocked(g)
			}
		}
	}
}

func (a *application) windowSendAcknowledge(key, id string, ok bool, outcome, message, document string) (found, valid bool) {
	a.windowSend.mu.Lock()
	defer a.windowSend.mu.Unlock()
	a.windowSend.expireLocked(windowSendNow())
	for _, r := range a.windowSend.requests {
		if r.id != id || r.to != key || r.state != peerTaken {
			continue
		}
		if ok && !(r.kind == "remote" && outcome == "opened" || r.kind != "remote" && peerOutcomeAllowed(r.kind, outcome)) {
			return true, false
		}
		if ok {
			r.state, r.outcome, r.message = peerDone, outcome, ""
		} else {
			r.state, r.outcome, r.message = peerFailed, "", message
		}
		if r.kind == "remote" {
			// The grant follows the tab the main window opened, and only that.
			for _, g := range append([]*windowSendGrant(nil), a.windowSend.grants...) {
				if g.request != id {
					continue
				}
				if !ok || !peerIDPattern.MatchString(document) {
					a.windowSend.dropGrantLocked(g)
				} else {
					g.document, g.confirmed = document, true
				}
			}
		}
		return true, true
	}
	return false, true
}

// windowSendResult is read by the sending window only.
func (a *application) windowSendResult(key, id string) (state, outcome, message, to string, found bool) {
	a.windowSend.mu.Lock()
	defer a.windowSend.mu.Unlock()
	a.windowSend.expireLocked(windowSendNow())
	for _, r := range a.windowSend.requests {
		if r.id == id && r.from == key {
			return r.state, r.outcome, r.message, r.to, true
		}
	}
	return "", "", "", "", false
}

// handleWindowSend serves /api/window-send/ for the window the request's own
// token names (main, terminal:<view>, editor:<view>).
func (a *application) handleWindowSend(w http.ResponseWriter, r *http.Request, key string) {
	if !a.windowSendAvailable() {
		http.NotFound(w, r)
		return
	}
	if !a.windowSendAlive(key) {
		jsonError(w, http.StatusGone, "This window is closing.")
		return
	}
	switch strings.TrimPrefix(r.URL.Path, windowSendPrefix) {
	case "targets":
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		kind := r.URL.Query().Get("kind")
		if kind != "terminal" && kind != "editor" {
			jsonError(w, http.StatusBadRequest, "Ask for terminal or editor targets.")
			return
		}
		lists, newDocument, newName, others := a.windowSendTargets(key, kind)
		body := map[string]any{"windows": lists, "others": others}
		if newDocument != "" {
			body["newDocument"], body["newName"] = newDocument, newName
		}
		jsonResponse(w, http.StatusOK, body)
	case "send":
		if r.Method != http.MethodPost {
			methodError(w)
			return
		}
		var body windowSendBody
		if err := decodePeerBody(w, r, peerBodyLimit("editor"), &body); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		if body.Kind != "terminal" && body.Kind != "editor" && body.Kind != "remote" {
			jsonError(w, http.StatusBadRequest, "Send text to a terminal or the Text Editor.")
			return
		}
		id, name, status, err := a.windowSendEnqueue(key, body)
		if err != nil {
			jsonError(w, status, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"requestId": id, "state": peerQueued, "name": name})
	case "result":
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		id := r.URL.Query().Get("requestId")
		if !peerRequestID.MatchString(id) {
			jsonError(w, http.StatusBadRequest, "The request identifier is invalid.")
			return
		}
		state, outcome, message, _, found := a.windowSendResult(key, id)
		if !found {
			jsonResponse(w, http.StatusOK, map[string]any{"requestId": id, "state": "lost", "message": "The send is no longer known."})
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"requestId": id, "state": state, "outcome": outcome, "message": message})
	case "inbox":
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		payload, kind, report, others := a.windowSendPoll(key)
		body := map[string]any{"report": report, "others": others, "request": nil}
		if payload != nil {
			body["request"], body["kind"] = payload, kind
		}
		jsonResponse(w, http.StatusOK, body)
	case "report":
		if r.Method != http.MethodPost {
			methodError(w)
			return
		}
		var body struct {
			Targets   []peerTarget `json:"targets"`
			Documents []peerTarget `json:"documents"`
		}
		if err := decodePeerBody(w, r, peerSummaryBodyLimit, &body); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		sessions, err := normalizePeerTargets("terminal", body.Targets)
		if err == nil {
			var documents []peerTarget
			documents, err = normalizePeerTargets(standaloneEditorMode, body.Documents)
			if err == nil {
				a.windowSendReport(key, sessions, documents)
			}
		}
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"ok": true})
	case "ack":
		if r.Method != http.MethodPost {
			methodError(w)
			return
		}
		var body struct {
			RequestID string `json:"requestId"`
			OK        bool   `json:"ok"`
			Outcome   string `json:"outcome,omitempty"`
			Error     string `json:"error,omitempty"`
			Document  string `json:"document,omitempty"` // remote: the tab the main window opened
		}
		if err := decodePeerBody(w, r, 4096, &body); err != nil || !peerRequestID.MatchString(body.RequestID) {
			jsonError(w, http.StatusBadRequest, "The acknowledgement is invalid.")
			return
		}
		found, valid := a.windowSendAcknowledge(key, body.RequestID, body.OK, body.Outcome, peerPlainText(body.Error, peerMessageRunes), body.Document)
		if !valid {
			jsonError(w, http.StatusBadRequest, "The outcome does not match the request.")
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"ok": true, "found": found})
	default:
		http.NotFound(w, r)
	}
}
