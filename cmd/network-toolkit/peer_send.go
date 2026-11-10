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
// It is a separate capability from window launch (terminal_window.go,
// editor_launch.go). Launch requests still carry only connection settings or a
// file to open. Peer send carries text only:
//   - The sender's page asks its own engine (session token, /api/peer-send/).
//     The engine reads the other app's instance record, which is protected for
//     this Windows user (workspace_instance.go), and calls that app on loopback
//     with its activation token. The page never sees the other app's token.
//   - The receiving engine accepts /api/peer/ only with its activation token,
//     only from loopback, never from a page, and only queues the request. It
//     holds at most one request, for at most a minute.
//   - The receiving page polls its engine's inbox once a second (a GET) and
//     reports what can receive text only when asked and when that changes. It
//     collects the request and runs the same code as a send inside one window: the Terminal's paste path
//     (paste review, paste warnings, bracketed paste, 1 MiB limit) or the
//     editor's ordinary undoable insert.
//
// Nothing here accepts or returns credentials, tokens, host keys, file paths,
// document text or terminal output other than the text the user chose to send.
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

	peerQueued  = "queued"
	peerTaken   = "taken"
	peerDone    = "done"
	peerFailed  = "failed"
	peerExpired = "expired"
)

var (
	peerNow          = time.Now
	peerLaunchWait   = 15 * time.Second
	peerIDPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	peerRequestID    = regexp.MustCompile(`^[a-f0-9]{32}$`)
	peerTypePattern  = regexp.MustCompile(`^[a-z0-9-]{0,24}$`)
	errPeerNotOpen   = errors.New("The other app is not open.")
	errPeerUnchecked = errors.New("The other app did not confirm the request.")
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
}

type peerRequest struct {
	id      string
	created time.Time
	state   string
	outcome string
	message string
	payload *peerPayload // nil once the page has it or the request ended
	timer   *time.Timer
}

type peerSendState struct {
	mu       sync.Mutex
	root     string       // data root shared by the standalone apps; empty elsewhere
	targets  []peerTarget // what this app's page last reported; the page keeps it current
	reported bool
	polledAt time.Time // the page's latest inbox poll: proof that it is running
	request  *peerRequest
	launchMu sync.Mutex
	launch   func(context.Context) error // tests start the other app in-process
}

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
	if mode == "terminal" {
		return appName + " Terminal"
	}
	return standaloneEditorTitle
}

func peerKind(mode string) string {
	if mode == "terminal" {
		return "terminal"
	}
	return "editor"
}

func peerWorkspaceDir(root, mode string) string {
	if mode == "terminal" {
		return terminalWorkspace(root)
	}
	return filepath.Join(root, standaloneEditorDirectory)
}

// configurePeerSend enables peer send in the standalone Text Editor and the
// standalone Terminal. The main window and its child windows never get it.
func (a *application) configurePeerSend(root string) {
	if root == "" || peerOf(a.windowMode) == "" {
		return
	}
	a.peer.mu.Lock()
	a.peer.root = root
	a.peer.mu.Unlock()
}

func (a *application) peerRoot() string {
	a.peer.mu.Lock()
	defer a.peer.mu.Unlock()
	if peerOf(a.windowMode) == "" {
		return ""
	}
	return a.peer.root
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
	if r == nil || (r.state != peerQueued && r.state != peerTaken) || now.Sub(r.created) < peerRequestLifetime {
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
func (s *peerSendState) freshTargetsLocked(now time.Time) ([]peerTarget, bool) {
	if !s.reported || s.polledAt.IsZero() || now.Sub(s.polledAt) > peerTargetsFresh || now.Before(s.polledAt) {
		return nil, false
	}
	return s.targets, true
}

// peerEnqueue queues one validated request for this app's page. It returns an
// HTTP status with a plain-language reason when the request cannot be taken.
func (a *application) peerEnqueue(payload peerPayload) (int, error) {
	mode := a.windowMode
	now := peerNow()
	a.peer.mu.Lock()
	defer a.peer.mu.Unlock()
	a.peer.expireLocked(now)
	if current := a.peer.request; current != nil && (current.state == peerQueued || current.state == peerTaken) {
		return http.StatusConflict, errors.New("Another send is still waiting in " + peerAppName(mode) + ". Finish it there, then send again.")
	}
	targets, fresh := a.peer.freshTargetsLocked(now)
	find := func(id string) *peerTarget {
		for i := range targets {
			if targets[i].ID == id {
				return &targets[i]
			}
		}
		return nil
	}
	if mode == "terminal" {
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
	request := &peerRequest{id: payload.RequestID, created: now, state: peerQueued, payload: &payload}
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
func (a *application) peerPoll() (payload *peerPayload, report bool) {
	now := peerNow()
	a.peer.mu.Lock()
	defer a.peer.mu.Unlock()
	a.peer.polledAt = now
	a.peer.expireLocked(now)
	report = !a.peer.reported
	r := a.peer.request
	if r == nil || r.state != peerQueued || r.payload == nil {
		return nil, report
	}
	payload = r.payload
	r.state, r.payload = peerTaken, nil
	return payload, report
}

// peerReport stores the page's validated summary. The page sends it when the
// engine asks for it and again whenever it changes.
func (a *application) peerReport(targets []peerTarget) {
	a.peer.mu.Lock()
	defer a.peer.mu.Unlock()
	a.peer.targets, a.peer.reported = targets, true
}

func (a *application) peerAcknowledge(id string, ok bool, outcome, message string) bool {
	a.peer.mu.Lock()
	defer a.peer.mu.Unlock()
	a.peer.expireLocked(peerNow())
	r := a.peer.request
	if r == nil || r.id != id || r.state != peerTaken {
		return false
	}
	if ok {
		r.state, r.outcome, r.message = peerDone, outcome, ""
	} else {
		r.state, r.outcome, r.message = peerFailed, "", message
	}
	if r.timer != nil {
		r.timer.Stop()
	}
	return true
}

func (a *application) peerResult(id string) (state, outcome, message string, found bool) {
	a.peer.mu.Lock()
	defer a.peer.mu.Unlock()
	a.peer.expireLocked(peerNow())
	r := a.peer.request
	if r == nil || r.id != id {
		return "", "", "", false
	}
	return r.state, r.outcome, r.message, true
}

func peerOutcomeAllowed(mode, outcome string) bool {
	if mode == "terminal" {
		return outcome == "sent" || outcome == "review"
	}
	return outcome == "inserted" || outcome == "pick"
}

// ---- Inbound routes: the other standalone app, activation token only ----

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
	kind := peerKind(mode)
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
		a.peer.mu.Lock()
		targets, ready := a.peer.freshTargetsLocked(peerNow())
		targets = append([]peerTarget(nil), targets...)
		a.peer.mu.Unlock()
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
		if err := decodePeerBody(w, r, peerBodyLimit(mode), &payload); err != nil {
			a.peerRefuse(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := validatePeerPayload(mode, payload); err != nil {
			a.peerRefuse(w, http.StatusBadRequest, err.Error())
			return
		}
		if status, err := a.peerEnqueue(*payload); err != nil {
			a.peerRefuse(w, status, err.Error())
			return
		}
		// Raise this window so a paste review or the position bar is seen.
		a.activatePrimary()
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

// ---- Sending side: this app's engine calling the other app ----

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
}

// readPeerInstance reads the other app's instance record for peer send. The
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

// peerCall sends one request to the other standalone app of this data root.
// A reply counts only when it echoes the window mode and the instance
// identifier from the protected record, as window activation requires.
func (a *application) peerCall(ctx context.Context, method, path string, body any, timeout time.Duration) (peerResponse, int, error) {
	var reply peerResponse
	root, mode := a.peerRoot(), peerOf(a.windowMode)
	if root == "" {
		return reply, 0, errPeerNotOpen
	}
	record, err := readPeerInstance(peerWorkspaceDir(root, mode))
	if err != nil {
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
	if json.NewDecoder(io.LimitReader(response.Body, peerSummaryBodyLimit)).Decode(&reply) != nil || reply.WindowMode != mode || reply.InstanceID != record.InstanceID {
		// Another program may now own a stale record's port.
		return peerResponse{}, response.StatusCode, errPeerUnchecked
	}
	if response.StatusCode != http.StatusOK || !reply.OK {
		message := peerPlainText(reply.Error, peerMessageRunes)
		if message == "" {
			message = peerAppName(mode) + " refused the request."
		}
		return reply, response.StatusCode, errors.New(message)
	}
	return reply, response.StatusCode, nil
}

func (a *application) peerTargetsPath() string {
	if peerOf(a.windowMode) == "terminal" {
		return peerInboundPrefix + "terminal/sessions"
	}
	return peerInboundPrefix + "editor/documents"
}

// peerList asks the other app what can receive text. It also proves, before
// any text is sent, that the record's endpoint is that app.
func (a *application) peerList(ctx context.Context) ([]peerTarget, bool, error) {
	reply, _, err := a.peerCall(ctx, http.MethodGet, a.peerTargetsPath(), nil, 2*time.Second)
	if err != nil {
		return nil, false, err
	}
	mode := peerOf(a.windowMode)
	targets := make([]peerTarget, 0, len(reply.Sessions)+len(reply.Documents))
	if mode == "terminal" {
		for _, s := range reply.Sessions {
			targets = append(targets, peerTarget{ID: s.ID, Name: s.Name, Type: s.Type, Connected: s.Connected, Active: s.Active})
		}
	} else {
		for _, d := range reply.Documents {
			targets = append(targets, peerTarget{ID: d.UID, Name: d.Name, Unsaved: d.Unsaved, Busy: d.Busy, Active: d.Active})
		}
	}
	// The other app validated its own list; validate again before showing it.
	targets, err = normalizePeerTargets(mode, targets)
	if err != nil {
		return nil, false, errPeerUnchecked
	}
	return targets, reply.Ready, nil
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

// launchPeerApp opens the other standalone app the way its Start menu shortcut
// does: a running app is raised, otherwise this executable is started with the
// fixed --terminal or --editor-standalone arguments. No text, credentials,
// shell, parent-owned pipes or parent cancellation are passed to it.
func launchPeerApp(ctx context.Context, root, mode string, headless bool) error {
	dir := peerWorkspaceDir(root, mode)
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
	exe, err := updateCurrentExecutable()
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

// peerOpen raises or starts the other app and waits, for a bounded time, until
// its engine answers through its protected instance record.
func (a *application) peerOpen(ctx context.Context) (ready bool, err error) {
	root, mode := a.peerRoot(), peerOf(a.windowMode)
	if root == "" {
		return false, errPeerNotOpen
	}
	a.peer.launchMu.Lock()
	defer a.peer.launchMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, peerLaunchWait)
	defer cancel()
	a.peer.mu.Lock()
	launch := a.peer.launch
	a.peer.mu.Unlock()
	if launch == nil {
		launch = func(ctx context.Context) error { return launchPeerApp(ctx, root, mode, a.headless) }
	}
	if err = launch(ctx); err != nil {
		return false, fmt.Errorf("%s could not be opened: %w", peerAppName(mode), err)
	}
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, ready, err = a.peerList(ctx); err == nil {
			return ready, nil
		}
		select {
		case <-ctx.Done():
			return false, errors.New(peerAppName(mode) + " did not become ready. If it is open from an older version, close it and open it again.")
		case <-ticker.C:
		}
	}
}

// ---- Page routes: this app's own page, session token ----

func (a *application) handlePeerSend(w http.ResponseWriter, r *http.Request) {
	own, mode := a.windowMode, peerOf(a.windowMode)
	if a.peerRoot() == "" {
		jsonError(w, http.StatusNotFound, "Sending between apps is available in the standalone Text Editor and Terminal.")
		return
	}
	name := peerAppName(mode)
	route := strings.TrimPrefix(r.URL.Path, peerPagePrefix)
	method := map[string]string{"targets": http.MethodGet, "result": http.MethodGet, "open": http.MethodPost, "send": http.MethodPost, "inbox": http.MethodGet, "report": http.MethodPost, "ack": http.MethodPost}[route]
	if method == "" {
		http.NotFound(w, r)
		return
	}
	if r.Method != method {
		methodError(w)
		return
	}
	switch route {
	case "targets":
		body := map[string]any{"app": peerKind(mode), "name": name, "running": false, "ready": false, "targets": []peerTarget{}}
		if targets, ready, err := a.peerList(r.Context()); err == nil {
			body["running"], body["ready"], body["targets"] = true, ready, targets
		}
		jsonResponse(w, http.StatusOK, body)
	case "open":
		ready, err := a.peerOpen(r.Context())
		if err != nil {
			jsonError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"app": peerKind(mode), "name": name, "running": true, "ready": ready})
	case "send":
		var payload *peerPayload
		if err := decodePeerBody(w, r, peerBodyLimit(mode), &payload); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		if payload == nil || payload.RequestID != "" {
			jsonError(w, http.StatusBadRequest, "The request is invalid or too large.")
			return
		}
		payload.RequestID = randomHex(16)
		if err := validatePeerPayload(mode, payload); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		// Prove the endpoint is the other app before any text leaves this one.
		if _, _, err := a.peerList(r.Context()); err != nil {
			jsonError(w, http.StatusServiceUnavailable, name+" is not open. Nothing was sent.")
			return
		}
		path := peerInboundPrefix + "terminal/send"
		if mode != "terminal" {
			path = peerInboundPrefix + "editor/insert"
		}
		reply, status, err := a.peerCall(r.Context(), http.MethodPost, path, payload, 20*time.Second)
		if err != nil || reply.RequestID != payload.RequestID {
			message := name + " did not accept the text. Nothing was sent."
			code := http.StatusServiceUnavailable
			if err != nil && !errors.Is(err, errPeerNotOpen) && !errors.Is(err, errPeerUnchecked) {
				message = err.Error()
				if status >= 400 && status < 500 {
					code = http.StatusConflict
				}
			}
			jsonError(w, code, message)
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"requestId": payload.RequestID, "state": peerQueued, "name": name})
	case "result":
		id := r.URL.Query().Get("requestId")
		if !peerRequestID.MatchString(id) {
			jsonError(w, http.StatusBadRequest, "The request identifier is invalid.")
			return
		}
		reply, status, err := a.peerCall(r.Context(), http.MethodGet, peerInboundPrefix+peerKind(mode)+"/result?requestId="+id, nil, 2*time.Second)
		if err != nil {
			message := name + " closed before it confirmed the text."
			if status == http.StatusNotFound {
				message = name + " no longer knows this request. Check that window."
			}
			jsonResponse(w, http.StatusOK, map[string]any{"requestId": id, "state": "lost", "message": message})
			return
		}
		state, outcome := reply.State, reply.Outcome
		if state != peerQueued && state != peerTaken && state != peerDone && state != peerFailed && state != peerExpired || reply.RequestID != id {
			state = "lost"
		}
		if state != peerDone || !peerOutcomeAllowed(mode, outcome) {
			outcome = ""
		}
		jsonResponse(w, http.StatusOK, map[string]any{"requestId": id, "state": state, "outcome": outcome, "message": peerPlainText(reply.Message, peerMessageRunes)})
	case "inbox":
		request, report := a.peerPoll()
		jsonResponse(w, http.StatusOK, map[string]any{"request": request, "report": report, "from": name})
	case "report":
		var report *struct {
			Targets []peerTarget `json:"targets"`
		}
		if err := decodePeerBody(w, r, peerSummaryBodyLimit, &report); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		if report == nil {
			jsonError(w, http.StatusBadRequest, "Send one JSON object.")
			return
		}
		targets, err := normalizePeerTargets(own, report.Targets)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		a.peerReport(targets)
		jsonResponse(w, http.StatusOK, map[string]any{"ok": true})
	case "ack":
		var ack *struct {
			RequestID string `json:"requestId"`
			OK        bool   `json:"ok"`
			Outcome   string `json:"outcome"`
			Error     string `json:"error"`
		}
		if err := decodePeerBody(w, r, 8192, &ack); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		if ack == nil || !peerRequestID.MatchString(ack.RequestID) || ack.OK && !peerOutcomeAllowed(own, ack.Outcome) || !ack.OK && ack.Outcome != "" {
			jsonError(w, http.StatusBadRequest, "The acknowledgement is invalid.")
			return
		}
		message := peerPlainText(ack.Error, peerMessageRunes)
		if !ack.OK && message == "" {
			message = "The text was not accepted."
		}
		if !a.peerAcknowledge(ack.RequestID, ack.OK, ack.Outcome, message) {
			jsonError(w, http.StatusConflict, "That request is no longer waiting.")
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"ok": true})
	}
}
