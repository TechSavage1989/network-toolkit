package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// Peer send passes text between the standalone Text Editor and the standalone
// Terminal of ONE data root, so each app's existing "Send to terminal" and "To
// editor" commands work across the two processes.
//
// Receivers (owner request): a standalone app sends to the first of these that
// answers, without asking the user to open anything:
//  1. the other standalone app, if it is running;
//  2. the main Toolkit window of the same data root, if its page is running:
//     the text goes to its Terminal page or its Text Editor, and that window
//     comes to the front. A main window that answers but whose page has not
//     reported yet is probed again for up to peerMainGrace first;
//  3. otherwise the other standalone app is started (launchPeerApp: the fixed
//     launch arguments, never text), and the text waits in THIS engine's
//     memory until the new app's record appears and its page is ready, for at
//     most peerLaunchWait. A send that is waiting when this app closes is
//     dropped; it is never written anywhere.
//
// Detached windows (owner decision, v1.21.50). A window popped out of a
// standalone app sends exactly as that app's own window: its page asks its
// OWNER engine with its view token (handleChildPeerSend: only the sending
// routes, and only a window of the app's own tool), and the owner engine sends
// with every rule below. No peer route is ever served to a child window or
// reached by it directly. On the receiving side, an engine lists and accepts
// the sessions and documents of its detached windows too (what they report
// through window send), and text for one of them is queued for that window
// (peerEnqueue, window_send.go) and raises it.
//
// The main window only receives text. Its own Send to commands stay inside its
// window, and its child windows send only through window send. Its one sending use is the
// document hand-over of peer_handover.go: when the user pops its Text Editor
// out while the standalone Text Editor runs, local and untitled tabs are given
// to that editor ("documents", the only request that carries file paths).
//
// It is a separate capability from window launch (terminal_window.go,
// editor_launch.go). Launch requests still carry only connection settings or a
// file to open. Peer send carries text only:
//   - The sender's page asks its own engine (session token, /api/peer-send/).
//     The engine reads the other app's instance record, which is protected for
//     this Windows user (workspace_instance.go), and calls that app on loopback
//     with its activation token. The page never sees the other app's token.
//   - The receiving engine (a standalone app or the main window) accepts
//     /api/peer/ only with its activation token, only from loopback, never
//     from a page, and only queues the request. It holds at most one request,
//     for at most a minute.
//   - A running app whose record is plain or unreadable, or that has no
//     /api/peer/ route (an older version), is never a receiver: it is skipped
//     and the user is told.
//   - The receiving page polls its engine's inbox once a second (a GET) and
//     reports what can receive text only when asked and when that changes. It
//     collects the request and runs the same code as a send inside one window: the Terminal's paste path
//     (paste review, paste warnings, bracketed paste, 1 MiB limit) or the
//     editor's ordinary undoable insert.
//
// Nothing here accepts or returns credentials, tokens, host keys, file paths,
// document text or terminal output other than the text the user chose to send.
// The one exception is the main window's document hand-over (kind "documents",
// peer_handover.go): local file paths and the tabs' unsaved text and undo
// history.
const (
	peerInboundPrefix = "/api/peer/"
	peerPagePrefix    = "/api/peer-send/"

	peerTerminalTextLimit = 1 << 20 // the Terminal's own paste limit
	peerEditorTextLimit   = 8 << 20
	peerSessionLimit      = 64
	peerDocumentLimit     = 200
	peerNameRunes         = 200
	peerMessageRunes      = 300
	peerSummaryBodyLimit  = 512 << 10
	peerRequestLifetime   = time.Minute
	peerTargetsFresh      = 5 * time.Second

	peerStarting = "starting" // sender only: the other app is being started
	peerQueued   = "queued"
	peerTaken    = "taken"
	peerDone     = "done"
	peerFailed   = "failed"
	peerExpired  = "expired"
)

var (
	peerNow = time.Now
	// peerLaunchWait bounds starting the other app: launch, instance record,
	// page ready. It stays under the HTTP server's 30 second write timeout.
	peerLaunchWait = 25 * time.Second
	// peerMainGrace is how long a main window whose protected record is valid
	// and whose engine answers, but whose page has not reported within
	// peerTargetsFresh, is probed again before it is passed over. Its page
	// reports within a second or two of starting; without the wait a send just
	// after the window opened would start the other standalone app instead.
	peerMainGrace    = 3 * time.Second
	peerIDPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	peerRequestID    = regexp.MustCompile(`^[a-f0-9]{32}$`)
	peerTypePattern  = regexp.MustCompile(`^[a-z0-9-]{0,24}$`)
	errPeerNotOpen   = errors.New("The other app is not open.")
	errPeerUnchecked = errors.New("The other app did not confirm the request.")
	// errPeerOlder: something is registered, but its record is plain or cannot
	// be read, or its engine has no peer routes. It is not a valid receiver.
	errPeerOlder = errors.New("The other app is open from an older version.")
)

// peerTarget is the nonsecret summary a page reports about what can receive
// text: connected terminal sessions, or open editor documents. Names are what
// the tab shows; a document name is the file name without its folder.
type peerTarget struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type,omitempty"`
	Connected bool   `json:"connected,omitempty"`
	Unsaved   bool   `json:"unsaved,omitempty"`
	Busy      bool   `json:"busy,omitempty"`
	Active    bool   `json:"active,omitempty"`
}

type peerSessionSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Connected bool   `json:"connected"`
	Active    bool   `json:"active,omitempty"`
}

type peerDocumentSummary struct {
	UID     string `json:"uid"`
	Name    string `json:"name"`
	Unsaved bool   `json:"unsaved,omitempty"`
	Busy    bool   `json:"busy,omitempty"`
	Active  bool   `json:"active,omitempty"`
}

// peerPayload is one queued request. Session is set for the Terminal; Target,
// Where, Line and Label for the Text Editor.
type peerPayload struct {
	RequestID string `json:"requestId"`
	Session   string `json:"session,omitempty"`
	Target    string `json:"target,omitempty"`
	Where     string `json:"where,omitempty"`
	Line      int    `json:"line,omitempty"`
	Label     string `json:"label,omitempty"`
	Text      string `json:"text"`
	// Documents is set only for a hand-over from the main window
	// (peer_handover.go); a terminal send or an editor insert never carries it.
	Documents []peerHandoverItem `json:"documents,omitempty"`
}

type peerRequest struct {
	id      string
	kind    string // "terminal" or "editor": what the text is for
	created time.Time
	state   string
	outcome string
	message string
	payload *peerPayload // nil once the page has it or the request ended
	timer   *time.Timer
	ids     []string             // a hand-over: the tabs it carries, kept after the page has it
	items   []peerHandoverResult // a hand-over: what the page did with each tab
	// window: text for a session or document of a detached window of this
	// engine. The request lives in window send (from windowSendPeer); its
	// state, result and expiry are read there.
	window string
}

type peerSendState struct {
	mu       sync.Mutex
	root     string                  // data root shared by the apps; empty where peer send is off
	targets  map[string][]peerTarget // by kind: what this app's page last reported; the page keeps it current
	reported bool
	polledAt time.Time // the page's latest inbox poll: proof that it is running
	request  *peerRequest
	launchMu sync.Mutex
	launch   func(context.Context) error // tests start the other app in-process
	// Sending side (standalone apps only), by sending window: windowSendMain
	// for the app's own window, a window-send key for a detached window.
	listed   map[string]peerListed // the receiver whose targets each window's page was last shown
	outbound []*peerOutbound       // recent sends: where each went, or that it is waiting for a start
	// Main window only: the one document hand-over that is waiting (peer_handover.go).
	handover *peerHandoverOutbound
}

// peerReceiver is one app that may receive: the other standalone app or the
// main window. dir holds its instance record.
type peerReceiver struct {
	mode string
	dir  string
}

func (r peerReceiver) name() string { return peerAppName(r.mode) }

func (r peerReceiver) role() string {
	if r.mode == "main" {
		return "main"
	}
	return "standalone"
}

type peerListed struct {
	mode       string
	instanceID string
}

// peerOutbound is one send as the sending engine knows it. While the other app
// is being started the text is held by the goroutine that delivers it, never
// here and never on disk.
type peerOutbound struct {
	id         string
	from       string // the sending window; only it reads the result
	receiver   peerReceiver
	instanceID string // the instance the text was delivered to
	state      string // peerStarting, peerQueued (delivered: ask the receiver), peerFailed
	message    string
}

const peerOutboundKept = 8

func peerOf(mode string) string {
	switch mode {
	case "terminal":
		return standaloneEditorMode
	case standaloneEditorMode:
		return "terminal"
	}
	return ""
}

func peerAppName(mode string) string {
	switch mode {
	case "terminal":
		return appName + " Terminal"
	case "main":
		return appName
	}
	return standaloneEditorTitle
}

// peerReceives lists what this app's page can receive: a standalone app its
// own kind, the main window both.
func peerReceives(mode string) []string {
	switch mode {
	case "terminal":
		return []string{"terminal"}
	case standaloneEditorMode:
		return []string{"editor"}
	case "main":
		return []string{"terminal", "editor"}
	}
	return nil
}

func peerReceivesKind(mode, kind string) bool {
	for _, k := range peerReceives(mode) {
		if k == kind {
			return true
		}
	}
	return false
}

// peerSenderName names the standalone app that sends text of this kind.
func peerSenderName(kind string) string {
	if kind == peerHandoverKind {
		return appName // documents come from the main window
	}
	if kind == "terminal" {
		return standaloneEditorTitle
	}
	return appName + " Terminal"
}

func peerKind(mode string) string {
	if mode == "terminal" {
		return "terminal"
	}
	return "editor"
}

func peerWorkspaceDir(root, mode string) string {
	switch mode {
	case "terminal":
		return terminalWorkspace(root)
	case "main":
		return root // the main window registers in the data root itself
	}
	return filepath.Join(root, standaloneEditorDirectory)
}

// configurePeerSend enables peer send: sending and receiving in the standalone
// Text Editor and the standalone Terminal, receiving only in the main window.
// Child windows of the main window never get it (they use view tokens and
// never reach these routes).
func (a *application) configurePeerSend(root string) {
	if root == "" || len(peerReceives(a.windowMode)) == 0 {
		return
	}
	a.peer.mu.Lock()
	a.peer.root = root
	a.peer.mu.Unlock()
}

// peerRoot is the data root when this app can receive peer text.
func (a *application) peerRoot() string {
	a.peer.mu.Lock()
	defer a.peer.mu.Unlock()
	if len(peerReceives(a.windowMode)) == 0 {
		return ""
	}
	return a.peer.root
}

// peerSenderRoot is the data root when this app can send: standalone apps only.
func (a *application) peerSenderRoot() string {
	if peerOf(a.windowMode) == "" {
		return ""
	}
	return a.peerRoot()
}

// peerReceiveAvailable tells the main window's page whether to run the peer
// inbox: only a main window that registered its instance record can be found.
func (a *application) peerReceiveAvailable() bool {
	return a.windowMode == "main" && a.editorLaunchReusable && a.peerRoot() != ""
}

func peerPlainText(value string, limit int) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == utf8.RuneError {
			return ' '
		}
		return r
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	if utf8.RuneCountInString(value) > limit {
		value = string([]rune(value)[:limit])
	}
	return value
}

// normalizePeerTargets validates the page's summary strictly. It keeps display
// names only; a document name loses any folder the page might have included.
func normalizePeerTargets(mode string, list []peerTarget) ([]peerTarget, error) {
	limit := peerSessionLimit
	if mode != "terminal" {
		limit = peerDocumentLimit
	}
	if len(list) > limit {
		return nil, errors.New("Too many send targets were reported.")
	}
	seen := make(map[string]bool, len(list))
	clean := make([]peerTarget, 0, len(list))
	for _, item := range list {
		if !peerIDPattern.MatchString(item.ID) || seen[item.ID] {
			return nil, errors.New("A send target has an invalid identifier.")
		}
		seen[item.ID] = true
		if strings.ContainsFunc(item.Name, unicode.IsControl) || !utf8.ValidString(item.Name) || utf8.RuneCountInString(item.Name) > peerNameRunes {
			return nil, errors.New("A send target has an invalid name.")
		}
		target := peerTarget{ID: item.ID, Name: strings.TrimSpace(item.Name), Active: item.Active}
		if mode == "terminal" {
			if !peerTypePattern.MatchString(item.Type) || item.Unsaved || item.Busy {
				return nil, errors.New("A terminal session summary is invalid.")
			}
			if !item.Connected {
				continue // only sessions that can accept input are offered
			}
			target.Type, target.Connected = item.Type, true
			if target.Name == "" {
				target.Name = "Terminal"
			}
		} else {
			if item.Type != "" || item.Connected {
				return nil, errors.New("A document summary is invalid.")
			}
			if at := strings.LastIndexAny(target.Name, `/\`); at >= 0 {
				target.Name = target.Name[at+1:]
			}
			if target.Name == "" {
				target.Name = "Untitled"
			}
			target.Unsaved, target.Busy = item.Unsaved, item.Busy
		}
		clean = append(clean, target)
	}
	return clean, nil
}

// validatePeerPayload checks one request for the receiving app (mode).
func validatePeerPayload(mode string, p *peerPayload) error {
	if p == nil {
		return errors.New("Send one JSON object.")
	}
	if !peerRequestID.MatchString(p.RequestID) {
		return errors.New("The request identifier is invalid.")
	}
	if p.Text == "" {
		return errors.New("There is no text to send.")
	}
	if !utf8.ValidString(p.Text) {
		return errors.New("The text is not valid UTF-8.")
	}
	if p.Documents != nil {
		return errors.New("Text carries no documents.")
	}
	if mode == "terminal" {
		if p.Target != "" || p.Where != "" || p.Line != 0 || p.Label != "" {
			return errors.New("A terminal send carries a session and text only.")
		}
		if !peerIDPattern.MatchString(p.Session) {
			return errors.New("Choose a terminal session.")
		}
		if len(p.Text) > peerTerminalTextLimit {
			return errors.New("Send no more than 1 MiB of text to a terminal at a time.")
		}
		return nil
	}
	if p.Session != "" {
		return errors.New("An editor insert carries a document, a position and text only.")
	}
	if len(p.Text) > peerEditorTextLimit {
		return errors.New("Send no more than 8 MiB of text to the Text Editor at a time.")
	}
	if p.Target != "new" && !peerIDPattern.MatchString(p.Target) {
		return errors.New("Choose a new document or an open document.")
	}
	if strings.ContainsFunc(p.Label, unicode.IsControl) || utf8.RuneCountInString(p.Label) > peerNameRunes {
		return errors.New("The text label is invalid.")
	}
	switch p.Where {
	case "":
		p.Where = "start"
		if p.Target != "new" {
			p.Where = "pick"
		}
	case "start", "end", "cursor", "pick", "line":
	default:
		return errors.New("Choose where the text goes: start, end, cursor, pick or line.")
	}
	if p.Target == "new" && p.Where != "start" {
		return errors.New("A new document receives the text at its start.")
	}
	if p.Where == "line" {
		if p.Line < 1 || p.Line > 100_000_000 {
			return errors.New("Enter a line number of 1 or more.")
		}
	} else if p.Line != 0 {
		return errors.New("A line number needs the line position.")
	}
	return nil
}

func peerBodyLimit(mode string) int64 {
	// JSON can spend six bytes on one control character.
	if mode == "terminal" {
		return 6*peerTerminalTextLimit + 4096
	}
	return 6*peerEditorTextLimit + 4096
}

// ---- Receiving side: queue, page poll, acknowledgement ----

func (s *peerSendState) expireLocked(now time.Time) {
	r := s.request
	if r == nil || r.window != "" || (r.state != peerQueued && r.state != peerTaken) || now.Sub(r.created) < peerRequestLifetime {
		return
	}
	if r.state == peerQueued {
		r.message = "The window did not pick the text up in time. Nothing was sent."
	} else {
		r.message = "The window did not confirm what happened to the text. Check that window."
	}
	r.state, r.payload = peerExpired, nil
}

// freshTargetsLocked returns the page's summary while the page is known to be
// running: it polls the inbox once a second and reports every change.
func (s *peerSendState) freshTargetsLocked(now time.Time, kind string) ([]peerTarget, bool) {
	if !s.reported || s.polledAt.IsZero() || now.Sub(s.polledAt) > peerTargetsFresh || now.Before(s.polledAt) {
		return nil, false
	}
	return s.targets[kind], true
}

// peerRequestStateLocked is a request's state; one queued for a detached
// window is read from window send (a.peer.mu held, then window send's lock).
func (a *application) peerRequestStateLocked(r *peerRequest) string {
	if r == nil {
		return ""
	}
	if r.window != "" {
		state, _, _, _, found := a.windowSendResult(windowSendPeer, r.id)
		if !found {
			return peerExpired
		}
		return state
	}
	return r.state
}

// peerEnqueue queues one validated request for this app's page, or, for a
// session or a document that one of this engine's detached windows holds, for
// that window (window send, from windowSendPeer). window names that window;
// it is empty when the primary page takes the request. It returns an HTTP
// status with a plain-language reason when the request cannot be taken. The
// caller raises the window that took it.
func (a *application) peerEnqueue(kind string, payload peerPayload) (status int, window string, err error) {
	mode := a.windowMode
	now := peerNow()
	a.peer.mu.Lock()
	defer a.peer.mu.Unlock()
	a.peer.expireLocked(now)
	if current := a.peer.request; current != nil {
		if state := a.peerRequestStateLocked(current); state == peerQueued || state == peerTaken {
			return http.StatusConflict, "", errors.New("Another send is still waiting in " + peerAppName(mode) + ". Finish it there, then send again.")
		}
	}
	lookup := kind
	if kind == peerHandoverKind {
		lookup = "editor" // documents go to the editor whose page reports its documents
	}
	targets, fresh := a.peer.freshTargetsLocked(now, lookup)
	find := func(id string) *peerTarget {
		for i := range targets {
			if targets[i].ID == id {
				return &targets[i]
			}
		}
		return nil
	}
	if kind == "terminal" || kind == "editor" {
		id := payload.Session
		if kind == "editor" {
			id = payload.Target
		}
		primary := fresh && (id == "new" || find(id) != nil)
		if !primary {
			// A detached window of this engine shows that session or holds that
			// document (or, with no primary page, takes the new document).
			if holder := a.windowSendHolder(kind, id); holder != "" {
				body := windowSendBody{Kind: kind, Window: holder, Session: payload.Session, Target: payload.Target, Where: payload.Where, Line: payload.Line, Label: payload.Label, Text: payload.Text}
				if _, _, code, err := a.windowSendEnqueueText(windowSendPeer, body, payload.RequestID); err != nil {
					return code, "", err
				}
				if previous := a.peer.request; previous != nil && previous.timer != nil {
					previous.timer.Stop()
				}
				a.peer.request = &peerRequest{id: payload.RequestID, kind: kind, created: now, state: peerQueued, window: holder}
				return http.StatusOK, holder, nil
			}
		}
	}
	code, err := a.peerEnqueuePrimaryLocked(kind, payload, now, fresh, find)
	return code, "", err
}

// peerEnqueuePrimaryLocked queues a request for this app's own page.
func (a *application) peerEnqueuePrimaryLocked(kind string, payload peerPayload, now time.Time, fresh bool, find func(string) *peerTarget) (int, error) {
	mode := a.windowMode
	if kind == peerHandoverKind {
		if !fresh {
			return http.StatusServiceUnavailable, errors.New(peerAppName(mode) + " is not ready yet. Try again in a moment.")
		}
	} else if kind == "terminal" {
		if !fresh {
			return http.StatusServiceUnavailable, errors.New(peerAppName(mode) + " is not ready yet. Try again in a moment.")
		}
		if target := find(payload.Session); target == nil || !target.Connected {
			return http.StatusConflict, errors.New("That terminal session is no longer connected. Choose another session.")
		}
	} else if payload.Target != "new" {
		if !fresh {
			return http.StatusServiceUnavailable, errors.New(peerAppName(mode) + " is not ready yet. Try again in a moment.")
		}
		target := find(payload.Target)
		if target == nil {
			return http.StatusConflict, errors.New("That document is no longer open. Choose another.")
		}
		if target.Busy {
			return http.StatusConflict, errors.New("That document cannot take text right now. Choose another or try again.")
		}
	}
	if previous := a.peer.request; previous != nil && previous.timer != nil {
		previous.timer.Stop()
	}
	request := &peerRequest{id: payload.RequestID, kind: kind, created: now, state: peerQueued, payload: &payload}
	for _, item := range payload.Documents {
		request.ids = append(request.ids, item.ID)
	}
	// The text never outlives the request, even if this app's page has gone.
	request.timer = time.AfterFunc(peerRequestLifetime+time.Second, func() {
		a.peer.mu.Lock()
		defer a.peer.mu.Unlock()
		if a.peer.request == request {
			a.peer.expireLocked(peerNow())
		}
	})
	a.peer.request = request
	return http.StatusOK, nil
}

// peerPoll is the page's one-second inbox poll. It hands over the queued
// request, if any (the text leaves the engine's memory with it), and says
// whether the engine still needs the page's summary of what can receive text.
func (a *application) peerPoll() (payload *peerPayload, kind string, report bool) {
	now := peerNow()
	a.peer.mu.Lock()
	defer a.peer.mu.Unlock()
	a.peer.polledAt = now
	a.peer.expireLocked(now)
	report = !a.peer.reported
	r := a.peer.request
	if r == nil || r.state != peerQueued || r.payload == nil {
		return nil, "", report
	}
	payload = r.payload
	r.state, r.payload = peerTaken, nil
	return payload, r.kind, report
}

// peerReport stores the page's validated summary. The page sends it when the
// engine asks for it and again whenever it changes.
func (a *application) peerReport(targets map[string][]peerTarget) {
	a.peer.mu.Lock()
	defer a.peer.mu.Unlock()
	a.peer.targets, a.peer.reported = targets, true
}

// peerAcknowledge records the page's result. valid is false when the outcome
// does not belong to the kind of text the request carried. items is the
// per-tab result of a hand-over, and nil for text.
func (a *application) peerAcknowledge(id string, ok bool, outcome, message string, items []peerHandoverResult) (found, valid bool) {
	a.peer.mu.Lock()
	defer a.peer.mu.Unlock()
	a.peer.expireLocked(peerNow())
	r := a.peer.request
	if r == nil || r.id != id || r.window != "" || r.state != peerTaken {
		return false, true // a detached window acknowledges through window send
	}
	if ok && !peerOutcomeAllowed(r.kind, outcome) {
		return true, false
	}
	if items != nil && (r.kind != peerHandoverKind || !ok) || ok && r.kind == peerHandoverKind && !peerHandoverResultsValid(r.ids, items) {
		return true, false
	}
	r.items = items
	if ok {
		r.state, r.outcome, r.message = peerDone, outcome, ""
	} else {
		r.state, r.outcome, r.message = peerFailed, "", message
	}
	if r.timer != nil {
		r.timer.Stop()
	}
	return true, true
}

func (a *application) peerResult(id string) (state, outcome, message string, found bool) {
	a.peer.mu.Lock()
	defer a.peer.mu.Unlock()
	a.peer.expireLocked(peerNow())
	r := a.peer.request
	if r == nil || r.id != id {
		return "", "", "", false
	}
	if r.window != "" {
		state, outcome, message, _, found = a.windowSendResult(windowSendPeer, id)
		return state, outcome, message, found
	}
	return r.state, r.outcome, r.message, true
}

func peerOutcomeAllowed(kind, outcome string) bool {
	if kind == peerHandoverKind {
		return outcome == "opened"
	}
	if kind == "terminal" {
		return outcome == "sent" || outcome == "review"
	}
	return outcome == "inserted" || outcome == "pick"
}

// ---- Inbound routes: a standalone app of this data root, activation token only ----

func peerLoopback(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (a *application) peerReply(w http.ResponseWriter, status int, body map[string]any) {
	if body == nil {
		body = map[string]any{}
	}
	body["windowMode"], body["instanceId"] = a.windowMode, a.windowInstanceID
	if status == http.StatusOK {
		body["ok"] = true
	}
	jsonResponse(w, status, body)
}

func (a *application) peerRefuse(w http.ResponseWriter, status int, message string) {
	a.peerReply(w, status, map[string]any{"error": message})
}

func decodePeerBody(w http.ResponseWriter, r *http.Request, limit int64, out any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return errors.New("Send an application/json request.")
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return errors.New("The request is invalid or too large.")
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("Send one JSON object.")
	}
	return nil
}

// servePeerInbound handles /api/peer/. ServeHTTP has already checked the Host
// header. These routes take the activation token only (never the page's
// session token), only from loopback, and never from a browser page.
func (a *application) servePeerInbound(w http.ResponseWriter, r *http.Request) {
	if !peerLoopback(r.RemoteAddr) {
		jsonError(w, http.StatusForbidden, "Peer requests are accepted from this computer only.")
		return
	}
	for _, header := range []string{"Origin", "Referer", "Sec-Fetch-Site", "Sec-Fetch-Mode", "Sec-Fetch-Dest", "Cookie"} {
		if r.Header.Get(header) != "" {
			jsonError(w, http.StatusForbidden, "Peer requests are not accepted from a page.")
			return
		}
	}
	if a.windowActivationToken == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Window-Activation-Token")), []byte(a.windowActivationToken)) != 1 {
		jsonError(w, http.StatusUnauthorized, "The app session is unavailable. Reopen the interface from Network Toolkit.")
		return
	}
	mode := a.windowMode
	if a.peerRoot() == "" {
		http.NotFound(w, r)
		return
	}
	if a.shutdownRequested.Load() || a.ctx.Err() != nil {
		a.peerRefuse(w, http.StatusServiceUnavailable, peerAppName(mode)+" is closing.")
		return
	}
	// /api/peer/<kind>/<action>: a standalone app serves its own kind only, the
	// main window both (its Terminal page and its Text Editor).
	kind, _, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, peerInboundPrefix), "/")
	if kind == peerHandoverKind && mode == standaloneEditorMode {
		a.servePeerHandoverInbound(w, r) // documents from the main window
		return
	}
	if !peerReceivesKind(mode, kind) {
		http.NotFound(w, r)
		return
	}
	list, send := "sessions", "send"
	if kind == "editor" {
		list, send = "documents", "insert"
	}
	base := peerInboundPrefix + kind + "/"
	switch r.URL.Path {
	case base + list:
		if r.Method != http.MethodGet {
			a.peerRefuse(w, http.StatusMethodNotAllowed, "This method is not supported.")
			return
		}
		targets, ready := a.peerInboundTargets(kind)
		body := map[string]any{"ready": ready}
		if kind == "terminal" {
			sessions := make([]peerSessionSummary, 0, len(targets))
			for _, t := range targets {
				sessions = append(sessions, peerSessionSummary{ID: t.ID, Name: t.Name, Type: t.Type, Connected: t.Connected, Active: t.Active})
			}
			body["sessions"] = sessions
		} else {
			documents := make([]peerDocumentSummary, 0, len(targets))
			for _, t := range targets {
				documents = append(documents, peerDocumentSummary{UID: t.ID, Name: t.Name, Unsaved: t.Unsaved, Busy: t.Busy, Active: t.Active})
			}
			body["documents"] = documents
		}
		a.peerReply(w, http.StatusOK, body)
	case base + send:
		if r.Method != http.MethodPost {
			a.peerRefuse(w, http.StatusMethodNotAllowed, "This method is not supported.")
			return
		}
		var payload *peerPayload
		if err := decodePeerBody(w, r, peerBodyLimit(kind), &payload); err != nil {
			a.peerRefuse(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := validatePeerPayload(kind, payload); err != nil {
			a.peerRefuse(w, http.StatusBadRequest, err.Error())
			return
		}
		status, window, err := a.peerEnqueue(kind, *payload)
		if err != nil {
			a.peerRefuse(w, status, err.Error())
			return
		}
		// Raise the window that took it so a paste review or the position bar
		// is seen: this app's own window, whose page then shows its Terminal
		// page or its Text Editor (deliver()), or the detached window that
		// holds the session or the document (window_raise.go).
		if window != "" {
			a.windowSendRaise(windowSendPeer, window)
		} else {
			a.activatePrimary()
		}
		a.peerReply(w, http.StatusOK, map[string]any{"requestId": payload.RequestID, "state": peerQueued})
	case base + "result":
		if r.Method != http.MethodGet {
			a.peerRefuse(w, http.StatusMethodNotAllowed, "This method is not supported.")
			return
		}
		id := r.URL.Query().Get("requestId")
		state, outcome, message, found := "", "", "", false
		if peerRequestID.MatchString(id) {
			state, outcome, message, found = a.peerResult(id)
		}
		if !found {
			a.peerRefuse(w, http.StatusNotFound, "That request is not known to "+peerAppName(mode)+".")
			return
		}
		a.peerReply(w, http.StatusOK, map[string]any{"requestId": id, "state": state, "outcome": outcome, "message": message})
	default:
		http.NotFound(w, r)
	}
}

// ---- Sending side: this app's engine calling a receiver ----

type peerResponse struct {
	OK         bool                  `json:"ok"`
	WindowMode string                `json:"windowMode"`
	InstanceID string                `json:"instanceId"`
	Error      string                `json:"error"`
	Ready      bool                  `json:"ready"`
	Sessions   []peerSessionSummary  `json:"sessions"`
	Documents  []peerDocumentSummary `json:"documents"`
	RequestID  string                `json:"requestId"`
	State      string                `json:"state"`
	Outcome    string                `json:"outcome"`
	Message    string                `json:"message"`
	Items      []peerHandoverResult  `json:"items"`
}

// readPeerInstance reads a receiver's instance record for peer send. The
// record must be protected where the platform protects it; a plain record is
// never trusted with the user's text. Elsewhere it must be a private file.
func readPeerInstance(dir string) (terminalWindowInstance, error) {
	if runtime.GOOS != "windows" {
		info, err := os.Lstat(filepath.Join(dir, terminalInstanceFilename))
		if err != nil {
			return terminalWindowInstance{}, err
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
			return terminalWindowInstance{}, errors.New("The workspace instance record is not private to this user.")
		}
	}
	return readWorkspaceWindowInstance(dir, true)
}

// peerSendKind is what this app sends: the standalone Text Editor sends text
// for a terminal, the standalone Terminal text for the editor.
func (a *application) peerSendKind() string { return peerKind(peerOf(a.windowMode)) }

// peerCandidates lists the receivers in the order they are tried: the other
// standalone app, then the main window of the same data root.
func (a *application) peerCandidates() []peerReceiver {
	root, mode := a.peerSenderRoot(), peerOf(a.windowMode)
	if root == "" {
		return nil
	}
	return []peerReceiver{{mode: mode, dir: peerWorkspaceDir(root, mode)}, {mode: "main", dir: peerWorkspaceDir(root, "main")}}
}

func (a *application) peerStandalone() peerReceiver {
	mode := peerOf(a.windowMode)
	return peerReceiver{mode: mode, dir: peerWorkspaceDir(a.peerSenderRoot(), mode)}
}

// peerCall sends one request to a receiver of this data root. A reply counts
// only when it echoes the window mode and the instance identifier from the
// protected record, as window activation requires.
//
// errPeerNotOpen: no record, or nothing answers at its endpoint.
// errPeerOlder: a record that is plain or cannot be read, or an engine without
// peer routes. errPeerUnchecked: another program answered.
//
// expect pins the call to one instance: when it is set and the record now
// names another instance (the app closed and a different one registered since
// it was probed), nothing is sent to the new endpoint and errPeerNotOpen is
// returned. Every call that carries the user's text passes it.
func (a *application) peerCall(ctx context.Context, rc peerReceiver, expect, method, path string, body any, timeout time.Duration) (peerResponse, int, error) {
	if a.peerSenderRoot() == "" {
		return peerResponse{}, 0, errPeerNotOpen
	}
	return peerCallReceiver(ctx, rc, expect, method, path, body, timeout)
}

// peerCallReceiver is peerCall without the sender check. Only peerCall and the
// main window's document hand-over (peer_handover.go) use it.
func peerCallReceiver(ctx context.Context, rc peerReceiver, expect, method, path string, body any, timeout time.Duration) (peerResponse, int, error) {
	var reply peerResponse
	if rc.mode == "" || rc.dir == "" {
		return reply, 0, errPeerNotOpen
	}
	record, err := readPeerInstance(rc.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return reply, 0, errPeerNotOpen
		}
		return reply, 0, errPeerOlder
	}
	if expect != "" && subtle.ConstantTimeCompare([]byte(record.InstanceID), []byte(expect)) != 1 {
		return reply, 0, errPeerNotOpen
	}
	var reader io.Reader
	if body != nil {
		var encoded bytes.Buffer
		encoder := json.NewEncoder(&encoded)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(body); err != nil {
			return reply, 0, err
		}
		reader = &encoded
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, method, "http://"+record.Host+path, reader)
	if err != nil {
		return reply, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Window-Activation-Token", record.Token)
	if method == http.MethodPost {
		// The receiver raises its own window for the text it is given.
		peerAllowForeground(record.PID)
	}
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: time.Second}).DialContext, DisableKeepAlives: true, MaxResponseHeaderBytes: 16384}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("Peer requests cannot follow redirects.")
	}}
	response, err := client.Do(req)
	if err != nil {
		return reply, 0, errPeerNotOpen
	}
	defer response.Body.Close()
	if json.NewDecoder(io.LimitReader(response.Body, peerSummaryBodyLimit)).Decode(&reply) != nil || reply.WindowMode != rc.mode || reply.InstanceID != record.InstanceID {
		if response.StatusCode == http.StatusNotFound {
			// The record's engine has no peer routes: an older version.
			return peerResponse{}, response.StatusCode, errPeerOlder
		}
		// Another program may now own a stale record's port.
		return peerResponse{}, response.StatusCode, errPeerUnchecked
	}
	if response.StatusCode != http.StatusOK || !reply.OK {
		message := peerPlainText(reply.Error, peerMessageRunes)
		if message == "" {
			message = rc.name() + " refused the request."
		}
		return reply, response.StatusCode, errors.New(message)
	}
	return reply, response.StatusCode, nil
}

// peerProbe asks one receiver what can receive text. It also proves, before
// any text is sent, that the record's endpoint is that app.
func (a *application) peerProbe(ctx context.Context, rc peerReceiver) (targets []peerTarget, ready bool, instanceID string, err error) {
	kind := a.peerSendKind()
	path := peerInboundPrefix + "terminal/sessions"
	if kind != "terminal" {
		path = peerInboundPrefix + "editor/documents"
	}
	reply, _, err := a.peerCall(ctx, rc, "", http.MethodGet, path, nil, 2*time.Second)
	if err != nil {
		return nil, false, "", err
	}
	targets = make([]peerTarget, 0, len(reply.Sessions)+len(reply.Documents))
	if kind == "terminal" {
		for _, s := range reply.Sessions {
			targets = append(targets, peerTarget{ID: s.ID, Name: s.Name, Type: s.Type, Connected: s.Connected, Active: s.Active})
		}
	} else {
		for _, d := range reply.Documents {
			targets = append(targets, peerTarget{ID: d.UID, Name: d.Name, Unsaved: d.Unsaved, Busy: d.Busy, Active: d.Active})
		}
	}
	// The receiver validated its own list; validate again before showing it.
	targets, err = normalizePeerTargets(kind, targets)
	if err != nil {
		return nil, false, "", errPeerUnchecked
	}
	return targets, reply.Ready, reply.InstanceID, nil
}

// peerChoose returns the first receiver that is running, in the order of
// peerCandidates, or nil when the other standalone app has to be started.
// The main window counts only while its page is running: without a page it
// cannot show a paste review or the position bar. skipped names apps that are
// registered but cannot receive (an older version, or an unreadable record).
// A main window that answers but whose page has not reported yet gets
// peerMainGrace to report (peerProbeMain); a standalone app counts as running
// even while it starts, so it needs none.
func (a *application) peerChoose(ctx context.Context) (chosen *peerReceiver, targets []peerTarget, ready bool, instanceID string, skipped []string) {
	for _, rc := range a.peerCandidates() {
		list, isReady, id, err := a.peerProbe(ctx, rc)
		if err == nil && rc.mode == "main" && !isReady {
			list, isReady, id, err = a.peerProbeMain(ctx, rc)
		}
		if errors.Is(err, errPeerOlder) {
			skipped = append(skipped, rc.name())
			continue
		}
		if err != nil || rc.mode == "main" && !isReady {
			continue
		}
		rc := rc
		return &rc, list, isReady, id, skipped
	}
	return nil, nil, false, "", skipped
}

// peerProbeMain probes the main window again, for at most peerMainGrace and
// never past the sender's request, until its page reports. Every probe goes
// through peerProbe (protected record, echoed mode and instance), so nothing
// is trusted that a single probe would not trust. The probes share the grace
// deadline, so a main engine that stops answering adds at most peerMainGrace
// to the sender's request. It returns not ready when the page did not report
// in time, or the error that ended the wait.
func (a *application) peerProbeMain(ctx context.Context, rc peerReceiver) (targets []peerTarget, ready bool, instanceID string, err error) {
	graceCtx, cancel := context.WithTimeout(ctx, peerMainGrace)
	defer cancel()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-graceCtx.Done():
			return nil, false, "", ctx.Err()
		case <-ticker.C:
		}
		targets, ready, instanceID, err = a.peerProbe(graceCtx, rc)
		if err == nil && ready {
			return targets, ready, instanceID, nil
		}
		if graceCtx.Err() != nil {
			// The probe was cut short by the deadline: not ready, not a failure.
			return nil, false, "", ctx.Err()
		}
		if err != nil {
			return nil, false, "", err
		}
	}
}

// peerNote tells the user, in plain words, which running apps were passed over.
func peerNote(skipped []string) string {
	notes := make([]string, 0, len(skipped))
	for _, name := range skipped {
		notes = append(notes, name+" was skipped: it is open from an older version, or its window record cannot be read, so it cannot receive text. Close it and open it again to send to it.")
	}
	return strings.Join(notes, " ")
}

func peerLaunchArguments(root, mode string, headless bool) ([]string, error) {
	if mode == "terminal" {
		return terminalChildArguments(root, nil, headless)
	}
	args := []string{"--" + standaloneEditorMode, "--data-dir", root}
	if headless {
		args = append(args, "--no-browser")
	}
	return args, nil
}

// peerForegroundPID names the process that may be allowed to come to the front
// when the other app is opened: only one named by a record peer send trusts
// (readPeerInstance). A plain or unreadable record names none.
func peerForegroundPID(dir string) (int, bool) {
	record, err := readPeerInstance(dir)
	if err != nil || record.PID < 1 {
		return 0, false
	}
	return record.PID, true
}

// launchPeerApp opens the other standalone app the way its Start menu shortcut
// does: a running app is raised, otherwise its program (standaloneProgram:
// the separate program, else Network_Toolkit.exe) is started with the fixed
// --terminal or --editor-standalone arguments. No text, credentials,
// shell, parent-owned pipes or parent cancellation are passed to it.
func launchPeerApp(ctx context.Context, root, mode string, headless bool) error {
	dir := peerWorkspaceDir(root, mode)
	// Only a protected (elsewhere: private) record names a process that may
	// come to the front. A plain record of an older Terminal is still raised
	// by the activation below, but it gets no foreground right from here.
	if pid, ok := peerForegroundPID(dir); ok {
		peerAllowForeground(pid)
	}
	var raised error
	if mode == "terminal" {
		raised = activateTerminalWindow(ctx, dir, nil)
	} else {
		raised = activateWorkspaceWindow(ctx, dir, standaloneEditorMode, editorLaunchEndpoint, &editorLaunchRequest{})
	}
	if raised == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// The other app's separate program when it is beside this one, else the
	// main program with the fixed switch (edition.go).
	exe, err := standaloneProgram(mode)
	if err != nil {
		return err
	}
	args, err := peerLaunchArguments(root, mode, headless)
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, args...)
	configureTerminalChild(cmd)
	if err = cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// peerOpen raises or starts the other standalone app and waits, for at most
// peerLaunchWait, until its engine answers through its protected instance
// record and its page is running. Nothing but the fixed launch arguments goes
// to the new process. It returns the instance that answered, so text is then
// delivered to that instance only.
func (a *application) peerOpen(ctx context.Context) (instanceID string, err error) {
	root := a.peerSenderRoot()
	if root == "" {
		return "", errPeerNotOpen
	}
	rc := a.peerStandalone()
	a.peer.launchMu.Lock()
	defer a.peer.launchMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, peerLaunchWait)
	defer cancel()
	a.peer.mu.Lock()
	launch := a.peer.launch
	a.peer.mu.Unlock()
	if launch == nil {
		launch = func(ctx context.Context) error { return launchPeerApp(ctx, root, rc.mode, a.headless) }
	}
	if err := launch(ctx); err != nil {
		return "", fmt.Errorf("%s could not be opened: %w", rc.name(), err)
	}
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, ready, id, err := a.peerProbe(ctx, rc); err == nil && ready {
			return id, nil
		}
		select {
		case <-ctx.Done():
			return "", errors.New(rc.name() + " did not become ready. If it is open from an older version, close it and open it again.")
		case <-ticker.C:
		}
	}
}

// peerDeliver hands one validated request to the instance of a receiver that
// peerProbe has just confirmed (instanceID; never empty). If the record names
// another instance by now, nothing is sent. It returns the HTTP status for
// this app's page.
func (a *application) peerDeliver(ctx context.Context, rc peerReceiver, instanceID string, payload *peerPayload) (int, error) {
	if instanceID == "" {
		return http.StatusServiceUnavailable, errors.New(rc.name() + " did not accept the text. Nothing was sent.")
	}
	path := peerInboundPrefix + "terminal/send"
	if a.peerSendKind() != "terminal" {
		path = peerInboundPrefix + "editor/insert"
	}
	reply, status, err := a.peerCall(ctx, rc, instanceID, http.MethodPost, path, payload, 20*time.Second)
	if err == nil && reply.RequestID == payload.RequestID {
		return http.StatusOK, nil
	}
	message := rc.name() + " did not accept the text. Nothing was sent."
	code := http.StatusServiceUnavailable
	if err != nil && !errors.Is(err, errPeerNotOpen) && !errors.Is(err, errPeerUnchecked) && !errors.Is(err, errPeerOlder) {
		message = err.Error()
		if status >= 400 && status < 500 {
			code = http.StatusConflict
		}
	}
	return code, errors.New(message)
}

func (a *application) peerRemember(o *peerOutbound) {
	a.peer.mu.Lock()
	defer a.peer.mu.Unlock()
	a.peer.rememberLocked(o)
}

func (s *peerSendState) rememberLocked(o *peerOutbound) {
	s.outbound = append(s.outbound, o)
	if extra := len(s.outbound) - peerOutboundKept; extra > 0 {
		s.outbound = append([]*peerOutbound(nil), s.outbound[extra:]...)
	}
}

func (a *application) peerOutboundFor(from, id string) (peerOutbound, bool) {
	a.peer.mu.Lock()
	defer a.peer.mu.Unlock()
	for _, o := range a.peer.outbound {
		if o.id == id && o.from == from {
			return *o, true
		}
	}
	return peerOutbound{}, false
}

// peerStartAndDeliver is step 3 of the receiver order: nothing was running, so
// the other standalone app is started and the text, held only by this
// goroutine, is delivered once that app's page is ready. If the app does not
// come up in time, or this app closes first, the text is dropped here and the
// user's own copy (the terminal output, the document) is untouched.
func (a *application) peerStartAndDeliver(o *peerOutbound, payload peerPayload) {
	instanceID, err := a.peerOpen(a.ctx)
	if err == nil {
		_, err = a.peerDeliver(a.ctx, o.receiver, instanceID, &payload)
	}
	a.peer.mu.Lock()
	defer a.peer.mu.Unlock()
	o.instanceID = instanceID
	if err != nil {
		message := strings.TrimSuffix(err.Error(), " Nothing was sent.")
		if !strings.HasSuffix(message, ".") {
			message += "."
		}
		o.state, o.message = peerFailed, message+" Nothing was sent."
		return
	}
	o.state = peerQueued
	if windowSendChild(o.from) {
		go a.windowSendAllowFrom(o.from, peerReceiverPID(o.receiver, instanceID))
	}
}

// ---- Page routes: this app's own page, session token ----

func (a *application) handlePeerSend(w http.ResponseWriter, r *http.Request) {
	a.handlePeerSendFrom(w, r, windowSendMain)
}

// peerChildKind is the app a detached window sends as: only a window of a
// standalone app's own tool (a Terminal window of the standalone Terminal, a
// Text Editor window of the standalone Text Editor) that can send at all.
func (a *application) peerChildKind(key string) string {
	if a.peerSenderRoot() == "" {
		return ""
	}
	switch {
	case a.windowMode == "terminal" && strings.HasPrefix(key, windowSendTerminal):
		return "terminal"
	case a.windowMode == standaloneEditorMode && strings.HasPrefix(key, windowSendEditor):
		return "editor"
	}
	return ""
}

// handleChildPeerSend serves /api/peer-send/ to a detached window of this
// engine (its view token): only the sending routes, which the owner engine
// runs as for its own window. A child never receives through peer send (it
// receives through window send) and never reaches /api/peer/.
func (a *application) handleChildPeerSend(w http.ResponseWriter, r *http.Request, key string) {
	route := strings.TrimPrefix(r.URL.Path, peerPagePrefix)
	if a.peerChildKind(key) == "" || route != "targets" && route != "result" && route != "open" && route != "send" {
		jsonError(w, http.StatusForbidden, "This window cannot send between apps.")
		return
	}
	if !a.windowSendAlive(key) {
		jsonError(w, http.StatusGone, "This window is closing.")
		return
	}
	a.handlePeerSendFrom(w, r, key)
}

// peerReceiverPID names the process of the receiver instance that just took
// text, from its protected record, so a sending detached window can allow it
// to come to the front. Zero when the record names another instance.
func peerReceiverPID(rc peerReceiver, instanceID string) int {
	record, err := readPeerInstance(rc.dir)
	if err != nil || instanceID == "" || subtle.ConstantTimeCompare([]byte(record.InstanceID), []byte(instanceID)) != 1 || record.PID < 1 {
		return 0
	}
	return record.PID
}

// handlePeerSendFrom serves /api/peer-send/ for one sending window: the app's
// own window (windowSendMain, which also receives) or a detached window
// (handleChildPeerSend, sending routes only).
func (a *application) handlePeerSendFrom(w http.ResponseWriter, r *http.Request, caller string) {
	own := a.windowMode
	const unavailable = "Sending between apps is available in the standalone Text Editor and Terminal."
	if a.peerRoot() == "" {
		jsonError(w, http.StatusNotFound, unavailable)
		return
	}
	route := strings.TrimPrefix(r.URL.Path, peerPagePrefix)
	method := map[string]string{"targets": http.MethodGet, "result": http.MethodGet, "open": http.MethodPost, "send": http.MethodPost, "inbox": http.MethodGet, "report": http.MethodPost, "ack": http.MethodPost}[route]
	if method == "" {
		http.NotFound(w, r)
		return
	}
	sending := route == "targets" || route == "result" || route == "open" || route == "send"
	if sending && a.peerSenderRoot() == "" {
		// The main window only receives.
		jsonError(w, http.StatusNotFound, unavailable)
		return
	}
	if r.Method != method {
		methodError(w)
		return
	}
	kind := a.peerSendKind() // meaningful on the sending routes only
	switch route {
	case "targets":
		standalone := a.peerStandalone()
		rc, targets, ready, id, skipped := a.peerChoose(r.Context())
		body := map[string]any{"app": kind, "name": standalone.name(), "receiver": "", "running": false, "ready": false, "targets": []peerTarget{}}
		listed := peerListed{}
		if rc != nil {
			body["name"], body["receiver"], body["running"], body["ready"], body["targets"] = rc.name(), rc.role(), true, ready, targets
			listed = peerListed{mode: rc.mode, instanceID: id}
		}
		if note := peerNote(skipped); note != "" {
			body["note"] = note
		}
		a.peer.mu.Lock()
		if a.peer.listed == nil {
			a.peer.listed = map[string]peerListed{}
		}
		a.peer.listed[caller] = listed
		a.peer.mu.Unlock()
		jsonResponse(w, http.StatusOK, body)
	case "open":
		instanceID, err := a.peerOpen(r.Context())
		if err != nil {
			jsonError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		a.windowSendAllowFrom(caller, peerReceiverPID(a.peerStandalone(), instanceID))
		jsonResponse(w, http.StatusOK, map[string]any{"app": kind, "name": a.peerStandalone().name(), "receiver": "standalone", "running": true, "ready": true})
	case "send":
		var payload *peerPayload
		if err := decodePeerBody(w, r, peerBodyLimit(kind), &payload); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		if payload == nil || payload.RequestID != "" {
			jsonError(w, http.StatusBadRequest, "The request is invalid or too large.")
			return
		}
		payload.RequestID = randomHex(16)
		if err := validatePeerPayload(kind, payload); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		standalone := a.peerStandalone()
		// A session or an open document belongs to the receiver the page listed.
		// Text for a new document goes to whichever receiver is running now.
		existing := kind == "terminal" || payload.Target != "new"
		// One lock for "is another send being started?" and for recording this
		// one, so two sends can never both start the other app. A send for a new
		// document is recorded as starting before a receiver is looked for.
		o := &peerOutbound{id: payload.RequestID, from: caller, receiver: standalone, state: peerStarting}
		a.peer.mu.Lock()
		listed, starting := a.peer.listed[caller], false
		for _, other := range a.peer.outbound {
			starting = starting || other.state == peerStarting
		}
		if !starting && !existing {
			a.peer.rememberLocked(o)
		}
		a.peer.mu.Unlock()
		if starting {
			jsonError(w, http.StatusConflict, "Another send is still waiting for "+standalone.name()+" to open. Try again in a moment.")
			return
		}
		if existing {
			// The engine enforces what the page shows: a session or a document
			// is sent only to the receiver that listed it, never to whichever
			// app happens to run now.
			if listed.mode == "" || listed.instanceID == "" {
				again := "Choose Send to terminal again."
				if kind != "terminal" {
					again = "Choose Send to editor again."
				}
				jsonError(w, http.StatusConflict, "Nothing was sent: the list of where the text can go is out of date. "+again)
				return
			}
			candidate := peerReceiver{mode: listed.mode, dir: peerWorkspaceDir(a.peerSenderRoot(), listed.mode)}
			// Prove the endpoint is still that app before any text leaves this one.
			if _, _, id, err := a.peerProbe(r.Context(), candidate); err != nil || id != listed.instanceID {
				jsonError(w, http.StatusServiceUnavailable, candidate.name()+" is not open any more. Nothing was sent.")
				return
			}
			if code, err := a.peerDeliver(r.Context(), candidate, listed.instanceID, payload); err != nil {
				jsonError(w, code, err.Error())
				return
			}
			a.peerRemember(&peerOutbound{id: payload.RequestID, from: caller, receiver: candidate, instanceID: listed.instanceID, state: peerQueued})
			a.windowSendAllowFrom(caller, peerReceiverPID(candidate, listed.instanceID))
			jsonResponse(w, http.StatusOK, map[string]any{"requestId": payload.RequestID, "state": peerQueued, "name": candidate.name(), "receiver": candidate.role()})
			return
		}
		// settle ends this send's "starting" record when no app is started.
		settle := func(state, message string, rc *peerReceiver, instanceID string) {
			a.peer.mu.Lock()
			defer a.peer.mu.Unlock()
			o.state, o.message, o.instanceID = state, message, instanceID
			if rc != nil {
				o.receiver = *rc
			}
		}
		rc, _, _, instanceID, skipped := a.peerChoose(r.Context())
		note := peerNote(skipped)
		if rc == nil {
			if a.shutdownRequested.Load() || a.ctx.Err() != nil {
				settle(peerFailed, "This app is closing. Nothing was sent.", nil, "")
				jsonError(w, http.StatusServiceUnavailable, "This app is closing. Nothing was sent.")
				return
			}
			// Nothing is running: start the other app and deliver when it is ready.
			go a.peerStartAndDeliver(o, *payload)
			body := map[string]any{"requestId": o.id, "state": peerStarting, "name": standalone.name(), "receiver": "standalone"}
			if note != "" {
				body["note"] = note
			}
			jsonResponse(w, http.StatusOK, body)
			return
		}
		if code, err := a.peerDeliver(r.Context(), *rc, instanceID, payload); err != nil {
			settle(peerFailed, err.Error(), rc, instanceID)
			jsonError(w, code, err.Error())
			return
		}
		settle(peerQueued, "", rc, instanceID)
		a.windowSendAllowFrom(caller, peerReceiverPID(*rc, instanceID))
		body := map[string]any{"requestId": payload.RequestID, "state": peerQueued, "name": rc.name(), "receiver": rc.role()}
		if note != "" {
			body["note"] = note
		}
		jsonResponse(w, http.StatusOK, body)
	case "result":
		id := r.URL.Query().Get("requestId")
		if !peerRequestID.MatchString(id) {
			jsonError(w, http.StatusBadRequest, "The request identifier is invalid.")
			return
		}
		o, known := a.peerOutboundFor(caller, id)
		if !known {
			jsonResponse(w, http.StatusOK, map[string]any{"requestId": id, "state": "lost", "name": a.peerStandalone().name(), "message": "This request is no longer known. Check the other window."})
			return
		}
		name := o.receiver.name()
		switch o.state {
		case peerStarting:
			jsonResponse(w, http.StatusOK, map[string]any{"requestId": id, "state": peerStarting, "name": name})
			return
		case peerFailed:
			// local: the message is this app's own and already names the other app.
			jsonResponse(w, http.StatusOK, map[string]any{"requestId": id, "state": peerFailed, "name": name, "local": true, "message": peerPlainText(o.message, peerMessageRunes)})
			return
		}
		reply, status, err := a.peerCall(r.Context(), o.receiver, o.instanceID, http.MethodGet, peerInboundPrefix+kind+"/result?requestId="+id, nil, 2*time.Second)
		if err != nil {
			message := name + " closed before it confirmed the text."
			if status == http.StatusNotFound {
				message = name + " no longer knows this request. Check that window."
			}
			jsonResponse(w, http.StatusOK, map[string]any{"requestId": id, "state": "lost", "name": name, "message": message})
			return
		}
		state, outcome := reply.State, reply.Outcome
		if state != peerQueued && state != peerTaken && state != peerDone && state != peerFailed && state != peerExpired || reply.RequestID != id {
			state = "lost"
		}
		if state != peerDone || !peerOutcomeAllowed(kind, outcome) {
			outcome = ""
		}
		jsonResponse(w, http.StatusOK, map[string]any{"requestId": id, "state": state, "outcome": outcome, "name": name, "message": peerPlainText(reply.Message, peerMessageRunes)})
	case "inbox":
		request, requestKind, report := a.peerPoll()
		body := map[string]any{"request": request, "report": report, "from": ""}
		if peerOf(own) != "" {
			body["from"] = peerAppName(peerOf(own))
		}
		if request != nil {
			body["kind"], body["from"] = requestKind, peerSenderName(requestKind)
		}
		jsonResponse(w, http.StatusOK, body)
	case "report":
		// A standalone app reports "targets" (its sessions or its documents). The
		// main window reports its sessions as "targets" and its documents as
		// "documents".
		var report *struct {
			Targets   []peerTarget  `json:"targets"`
			Documents *[]peerTarget `json:"documents"`
		}
		if err := decodePeerBody(w, r, peerSummaryBodyLimit, &report); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		if report == nil || report.Documents != nil && own != "main" {
			jsonError(w, http.StatusBadRequest, "Send one JSON object.")
			return
		}
		kinds := peerReceives(own)
		targets, err := normalizePeerTargets(kinds[0], report.Targets)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		summary := map[string][]peerTarget{kinds[0]: targets}
		if own == "main" {
			var documents []peerTarget
			if report.Documents != nil {
				documents = *report.Documents
			}
			if summary["editor"], err = normalizePeerTargets("editor", documents); err != nil {
				jsonError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		a.peerReport(summary)
		jsonResponse(w, http.StatusOK, map[string]any{"ok": true})
	case "ack":
		var ack *struct {
			RequestID string               `json:"requestId"`
			OK        bool                 `json:"ok"`
			Outcome   string               `json:"outcome"`
			Error     string               `json:"error"`
			Items     []peerHandoverResult `json:"items"`
		}
		if err := decodePeerBody(w, r, 64<<10, &ack); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		allowed := false
		if ack != nil {
			for _, received := range peerReceives(own) {
				allowed = allowed || peerOutcomeAllowed(received, ack.Outcome)
			}
			// The standalone Text Editor also receives documents from the main window.
			allowed = allowed || own == standaloneEditorMode && peerOutcomeAllowed(peerHandoverKind, ack.Outcome)
		}
		if ack == nil || !peerRequestID.MatchString(ack.RequestID) || ack.OK && !allowed || !ack.OK && ack.Outcome != "" {
			jsonError(w, http.StatusBadRequest, "The acknowledgement is invalid.")
			return
		}
		message := peerPlainText(ack.Error, peerMessageRunes)
		if !ack.OK && message == "" {
			message = "The text was not accepted."
		}
		found, valid := a.peerAcknowledge(ack.RequestID, ack.OK, ack.Outcome, message, ack.Items)
		if !valid {
			jsonError(w, http.StatusBadRequest, "The acknowledgement is invalid.")
			return
		}
		if !found {
			jsonError(w, http.StatusConflict, "That request is no longer waiting.")
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"ok": true})
	}
}
