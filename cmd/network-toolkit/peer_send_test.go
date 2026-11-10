package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

const peerTestRequestID = "0123456789abcdef0123456789abcdef"

// peerTestApp is one app (standalone Terminal, standalone Text Editor or the
// main window) under a shared data root, served by a real loopback listener
// and registered with its instance record, as main() does.
func peerTestApp(t *testing.T, root, mode string) (*application, *httptest.Server) {
	t.Helper()
	return peerTestAppWith(t, root, mode, func(a *application) http.Handler { return a })
}

func peerTestAppWith(t *testing.T, root, mode string, handler func(*application) http.Handler) (*application, *httptest.Server) {
	t.Helper()
	dir := peerWorkspaceDir(root, mode)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	a := newApplication(context.Background(), filepath.Join(dir, "Network_Toolkit.settings.json"))
	a.windowMode = mode
	if mode == standaloneEditorMode || mode == "main" {
		a.editorLaunchAvailable, a.editorLaunchReusable = true, true
	}
	server := httptest.NewServer(handler(a))
	a.host = strings.TrimPrefix(server.URL, "http://")
	a.configurePeerSend(root)
	t.Cleanup(func() {
		server.Close()
		a.stop()
		if a.terminals != nil {
			_ = a.terminals.Close()
		}
	})
	return a, server
}

func peerRegister(t *testing.T, a *application, root string) func() {
	t.Helper()
	unregister, err := a.registerWorkspaceWindow(peerWorkspaceDir(root, a.windowMode))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unregister)
	return unregister
}

// peerPage is a request from the app's own page: its session token over HTTP.
func peerPage(t *testing.T, a *application, method, path, body string) (int, map[string]any, string) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, "http://"+a.host+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Session-Token", a.token)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	return response.StatusCode, decoded, string(raw)
}

// peerPageTick is one tick of the app's page over HTTP: report, then poll.
func peerPageTick(t *testing.T, a *application, report string) (int, map[string]any, string) {
	t.Helper()
	if code, _, raw := peerPage(t, a, http.MethodPost, "/api/peer-send/report", report); code != http.StatusOK {
		t.Fatalf("page report refused: %d %s", code, raw)
	}
	return peerPage(t, a, http.MethodGet, "/api/peer-send/inbox", "")
}

// peerInbound is a request as the other app's engine would send it, straight
// into ServeHTTP so headers and the remote address can be chosen.
func peerInbound(a *application, method, path, body, token string, change func(*http.Request)) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://"+a.host+path, strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:50123"
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		r.Header.Set("X-Window-Activation-Token", token)
	}
	if change != nil {
		change(r)
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}

// peerReport is one tick of the app's page: it reports what can receive text
// and collects the waiting request, if any.
func peerReport(t *testing.T, a *application, targets string) *peerPayload {
	t.Helper()
	if r := requestApp(a, http.MethodPost, "/api/peer-send/report", `{"targets":`+targets+`}`, true); r.Code != http.StatusOK {
		t.Fatalf("page report refused: %d %s", r.Code, r.Body)
	}
	r := requestApp(a, http.MethodGet, "/api/peer-send/inbox", "", true)
	if r.Code != http.StatusOK {
		t.Fatalf("page poll refused: %d %s", r.Code, r.Body)
	}
	var reply struct {
		Request *peerPayload `json:"request"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &reply); err != nil {
		t.Fatal(err)
	}
	return reply.Request
}

func peerTestClock(t *testing.T) func(time.Duration) {
	t.Helper()
	now := time.Now()
	previous := peerNow
	peerNow = func() time.Time { return now }
	t.Cleanup(func() { peerNow = previous })
	return func(d time.Duration) { now = now.Add(d) }
}

func TestPeerRoutesAcceptOnlyTheActivationTokenFromLoopback(t *testing.T) {
	a := testApplication(t)
	a.windowMode = "terminal"
	a.configurePeerSend(t.TempDir())
	token := a.windowActivationToken
	const list = "/api/peer/terminal/sessions"
	if r := peerInbound(a, http.MethodGet, list, "", token, nil); r.Code != http.StatusOK {
		t.Fatalf("authenticated peer list refused: %d %s", r.Code, r.Body)
	}
	for name, r := range map[string]*httptest.ResponseRecorder{
		"no token":                peerInbound(a, http.MethodGet, list, "", "", nil),
		"wrong token":             peerInbound(a, http.MethodGet, list, "", strings.Repeat("0", 64), nil),
		"session token":           peerInbound(a, http.MethodGet, list, "", "", func(r *http.Request) { r.Header.Set("X-Session-Token", a.token) }),
		"api token as activation": peerInbound(a, http.MethodGet, list, "", a.token, nil),
		"send without token":      peerInbound(a, http.MethodPost, "/api/peer/terminal/send", `{"session":"s1","text":"x","requestId":"`+peerTestRequestID+`"}`, "", nil),
	} {
		if r.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status %d %s", name, r.Code, r.Body)
		}
		if strings.Contains(r.Body.String(), a.windowInstanceID) {
			t.Fatalf("%s: an unauthenticated reply identified the instance", name)
		}
	}
	for name, change := range map[string]func(*http.Request){
		"other computer": func(r *http.Request) { r.RemoteAddr = "192.0.2.7:50123" },
		"no address":     func(r *http.Request) { r.RemoteAddr = "" },
		"page origin":    func(r *http.Request) { r.Header.Set("Origin", "http://"+a.host) },
		"cross origin":   func(r *http.Request) { r.Header.Set("Origin", "https://attacker.invalid") },
		"fetch metadata": func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "same-origin") },
		"referer":        func(r *http.Request) { r.Header.Set("Referer", "http://"+a.host+"/") },
	} {
		if r := peerInbound(a, http.MethodGet, list, "", token, change); r.Code != http.StatusForbidden {
			t.Fatalf("%s: status %d %s", name, r.Code, r.Body)
		}
	}
	if r := peerInbound(a, http.MethodGet, list, "", token, func(r *http.Request) { r.Host = "attacker.invalid" }); r.Code != http.StatusForbidden {
		t.Fatalf("foreign Host accepted: %d", r.Code)
	}
	// The Terminal has no editor routes, and the token opens nothing else.
	for _, path := range []string{"/api/peer/editor/documents", "/api/peer/editor/insert", "/api/peer/terminal", "/api/peer/terminal/other", "/api/peer/"} {
		if r := peerInbound(a, http.MethodGet, path, "", token, nil); r.Code != http.StatusNotFound {
			t.Fatalf("%s: status %d", path, r.Code)
		}
	}
	for _, path := range []string{"/api/state", "/api/terminals", "/api/peer-send/targets", "/api/peer-send/inbox", "/api/credentials", "/api/terminals/known-hosts"} {
		if r := peerInbound(a, http.MethodGet, path, "", token, nil); r.Code != http.StatusUnauthorized {
			t.Fatalf("the activation token reached %s: %d", path, r.Code)
		}
	}
	if r := peerInbound(a, http.MethodPost, list, "", token, nil); r.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST to the list: %d", r.Code)
	}
	if r := peerInbound(a, http.MethodGet, "/api/peer/terminal/send", "", token, nil); r.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET to send: %d", r.Code)
	}
	a.shutdownRequested.Store(true)
	if r := peerInbound(a, http.MethodGet, list, "", token, nil); r.Code != http.StatusServiceUnavailable {
		t.Fatalf("a closing app accepted a peer request: %d", r.Code)
	}
}

// Which window has which peer routes. The owner's request made the main window
// a receiver, so it now serves /api/peer/ for both kinds (its guards are in
// TestPeerMainWindowReceivesWithTheSameGuards); it still has no sending
// routes, and child windows have neither.
func TestPeerRoutesByKindOfWindow(t *testing.T) {
	main := testApplication(t)
	main.configurePeerSend(t.TempDir())
	if main.peerRoot() == "" || main.peerSenderRoot() != "" {
		t.Fatal("the main window must receive peer text and never send it")
	}
	for _, path := range []string{"/api/peer/terminal/sessions", "/api/peer/editor/documents"} {
		if r := peerInbound(main, http.MethodGet, path, "", main.windowActivationToken, nil); r.Code != http.StatusOK {
			t.Fatalf("main window did not serve %s: %d", path, r.Code)
		}
	}
	if r := requestApp(main, http.MethodGet, "/api/peer-send/targets", "", true); r.Code != http.StatusNotFound {
		t.Fatalf("main window page route: %d", r.Code)
	}
	if r := requestApp(main, http.MethodGet, "/api/peer-send/targets", "", false); r.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated page route: %d", r.Code)
	}
	// Only a main window that registered its instance record tells its page to
	// run the inbox (it cannot be found otherwise).
	if main.peerReceiveAvailable() {
		t.Fatal("an unregistered main window announced peer receive")
	}
	main.editorLaunchReusable = true
	if !main.peerReceiveAvailable() {
		t.Fatal("a registered main window did not announce peer receive")
	}
	// Without a data root, and in every other window mode, there is nothing.
	bare := testApplication(t)
	child := testApplication(t)
	child.windowMode = "detached"
	child.configurePeerSend(t.TempDir())
	editorChild := testApplication(t)
	editorChild.windowMode = "editor"
	editorChild.editorLaunchReusable = true
	editorChild.configurePeerSend(t.TempDir())
	for name, a := range map[string]*application{"main without a data root": bare, "detached terminal": child, "detached editor": editorChild} {
		if a.peerRoot() != "" || a.peerSenderRoot() != "" || a.peerReceiveAvailable() {
			t.Fatalf("%s was given peer send", name)
		}
		for _, path := range []string{"/api/peer/terminal/sessions", "/api/peer/editor/documents"} {
			if r := peerInbound(a, http.MethodGet, path, "", a.windowActivationToken, nil); r.Code != http.StatusNotFound {
				t.Fatalf("%s served %s: %d", name, path, r.Code)
			}
		}
		for _, route := range []string{"targets", "inbox"} {
			if r := requestApp(a, http.MethodGet, "/api/peer-send/"+route, "", true); r.Code != http.StatusNotFound {
				t.Fatalf("%s page route %s: %d", name, route, r.Code)
			}
		}
	}
	// Real child windows of the MAIN engine: the token of a detached terminal
	// view and of a detached editor view reaches no peer page route and is not
	// an activation token. (The main engine's own page does reach its inbox.)
	if r := requestApp(main, http.MethodGet, "/api/peer-send/inbox", "", true); r.Code != http.StatusOK {
		t.Fatalf("the main page's own inbox: %d %s", r.Code, r.Body)
	}
	session := viewTestConnected(t, main)
	var terminalLaunch terminalViewLaunch
	main.launchTerminalView = func(_ context.Context, value terminalViewLaunch) (<-chan error, error) {
		terminalLaunch = value
		return nil, nil
	}
	view, err := main.views.create(session.ID, viewTestSnapshot(main, session))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = main.views.ready(view.ID); err != nil {
		t.Fatal(err)
	}
	main.launchEditorView = func(context.Context, editorViewLaunch) (<-chan error, error) { return nil, nil }
	_, _, editorView := editorFixtureOffer(t, main.editorViews)
	main.editorViews.mu.Lock()
	editorToken := main.editorViews.views[editorView].token
	main.editorViews.mu.Unlock()
	for name, token := range map[string]string{"detached terminal view": terminalLaunch.ViewToken, "detached editor view": editorToken} {
		if len(token) != 64 {
			t.Fatalf("%s: no view token", name)
		}
		if _, found := main.views.authenticate(token); !found {
			if _, found = main.editorViews.authenticate(token); !found {
				t.Fatalf("%s: the token is not a live view token", name)
			}
		}
		for route, method := range map[string]string{"inbox": http.MethodGet, "targets": http.MethodGet, "result?requestId=" + peerTestRequestID: http.MethodGet, "report": http.MethodPost, "ack": http.MethodPost, "send": http.MethodPost, "open": http.MethodPost} {
			var body any
			if method == http.MethodPost {
				body = map[string]any{"targets": []any{}}
			}
			if r := viewTestRequest(main, token, method, "/api/peer-send/"+route, body, nil); r.Code < 400 || strings.Contains(r.Body.String(), `"report"`) || strings.Contains(r.Body.String(), `"ok":true`) {
				t.Fatalf("%s reached /api/peer-send/%s: %d %s", name, route, r.Code, r.Body)
			}
		}
		for _, path := range []string{"/api/peer/terminal/sessions", "/api/peer/editor/documents"} {
			if r := peerInbound(main, http.MethodGet, path, "", token, nil); r.Code != http.StatusUnauthorized {
				t.Fatalf("%s as an activation token on %s: %d", name, path, r.Code)
			}
		}
	}
	main.peer.mu.Lock()
	reported := main.peer.reported
	main.peer.mu.Unlock()
	if reported {
		t.Fatal("a child window's report was stored")
	}
	editor := testApplication(t)
	editor.windowMode = standaloneEditorMode
	editor.configurePeerSend(t.TempDir())
	if r := peerInbound(editor, http.MethodGet, "/api/peer/editor/documents", "", editor.windowActivationToken, nil); r.Code != http.StatusOK {
		t.Fatalf("editor documents: %d %s", r.Code, r.Body)
	}
	if r := peerInbound(editor, http.MethodGet, "/api/peer/terminal/sessions", "", editor.windowActivationToken, nil); r.Code != http.StatusNotFound {
		t.Fatalf("the editor served terminal routes: %d", r.Code)
	}
	// A page in a child window (terminal or editor view) is a different token
	// and never reaches the page routes.
	if r := peerInbound(editor, http.MethodGet, "/api/peer-send/targets", "", "", func(r *http.Request) { r.Header.Set("X-Session-Token", strings.Repeat("a", 64)) }); r.Code != http.StatusUnauthorized {
		t.Fatalf("unknown session token: %d", r.Code)
	}
}

func TestPeerTerminalSendValidationLimitsAndSinglePendingRequest(t *testing.T) {
	advance := peerTestClock(t)
	a := testApplication(t)
	a.windowMode = "terminal"
	a.configurePeerSend(t.TempDir())
	token := a.windowActivationToken
	const send = "/api/peer/terminal/send"
	body := func(session, text, id string) string {
		b, _ := json.Marshal(map[string]string{"session": session, "text": text, "requestId": id})
		return string(b)
	}
	// Before the page has reported its sessions nothing can be queued.
	if r := peerInbound(a, http.MethodPost, send, body("s1", "show version", peerTestRequestID), token, nil); r.Code != http.StatusServiceUnavailable {
		t.Fatalf("send before the page was ready: %d %s", r.Code, r.Body)
	}
	if r := requestApp(a, http.MethodGet, "/api/peer-send/inbox", "", true); !strings.Contains(r.Body.String(), `"report":true`) {
		t.Fatalf("the engine did not ask its page for the sessions: %s", r.Body)
	}
	// A page that only polls has not said what can receive text yet.
	if r := peerInbound(a, http.MethodPost, send, body("s1", "show version", peerTestRequestID), token, nil); r.Code != http.StatusServiceUnavailable {
		t.Fatalf("send before the page reported: %d %s", r.Code, r.Body)
	}
	if got := peerReport(t, a, `[{"id":"s1","name":"lab-switch-01","type":"ssh","connected":true,"active":true},{"id":"s2","name":"closed","type":"telnet","connected":false}]`); got != nil {
		t.Fatalf("unexpected request: %+v", got)
	}
	if r := requestApp(a, http.MethodGet, "/api/peer-send/inbox", "", true); !strings.Contains(r.Body.String(), `"report":false`) {
		t.Fatalf("the engine asked again for an unchanged summary: %s", r.Body)
	}
	list := peerInbound(a, http.MethodGet, "/api/peer/terminal/sessions", "", token, nil)
	var listed struct {
		OK         bool                 `json:"ok"`
		Ready      bool                 `json:"ready"`
		WindowMode string               `json:"windowMode"`
		InstanceID string               `json:"instanceId"`
		Sessions   []peerSessionSummary `json:"sessions"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil || !listed.OK || !listed.Ready || listed.WindowMode != "terminal" || listed.InstanceID != a.windowInstanceID {
		t.Fatalf("list reply: %s (%v)", list.Body, err)
	}
	if want := []peerSessionSummary{{ID: "s1", Name: "lab-switch-01", Type: "ssh", Connected: true, Active: true}}; !reflect.DeepEqual(listed.Sessions, want) {
		t.Fatalf("only sessions that accept input are listed: %+v", listed.Sessions)
	}
	var keys map[string]json.RawMessage
	_ = json.Unmarshal(list.Body.Bytes(), &keys)
	for key := range keys {
		if !map[string]bool{"ok": true, "ready": true, "windowMode": true, "instanceId": true, "sessions": true}[key] {
			t.Fatalf("unexpected field in the session list: %s", key)
		}
	}
	for _, secret := range []string{a.token, a.windowActivationToken, a.dataPath, filepath.Dir(a.dataPath)} {
		if strings.Contains(list.Body.String(), secret) {
			t.Fatal("the session list exposed a token or a path")
		}
	}
	for name, bad := range map[string]string{
		"unknown field":     `{"session":"s1","text":"x","requestId":"` + peerTestRequestID + `","password":"secret"}`,
		"credential field":  `{"session":"s1","text":"x","requestId":"` + peerTestRequestID + `","credentialId":"c1"}`,
		"editor field":      `{"session":"s1","text":"x","requestId":"` + peerTestRequestID + `","target":"new"}`,
		"no session":        body("", "x", peerTestRequestID),
		"path as session":   body("../s1", "x", peerTestRequestID),
		"no text":           body("s1", "", peerTestRequestID),
		"short request id":  body("s1", "x", "abc"),
		"upper request id":  body("s1", "x", strings.ToUpper(peerTestRequestID)),
		"two values":        body("s1", "x", peerTestRequestID) + `{}`,
		"array":             `[]`,
		"null":              `null`,
		"over 1 MiB":        body("s1", strings.Repeat("a", peerTerminalTextLimit+1), peerTestRequestID),
		"body over limit":   `{"session":"s1","requestId":"` + peerTestRequestID + `","text":"` + strings.Repeat("\\u0000", peerTerminalTextLimit+2000) + `"}`,
		"number as session": `{"session":7,"text":"x","requestId":"` + peerTestRequestID + `"}`,
	} {
		if r := peerInbound(a, http.MethodPost, send, bad, token, nil); r.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d %s", name, r.Code, r.Body)
		}
	}
	if r := peerInbound(a, http.MethodPost, send, body("s1", "x", peerTestRequestID), token, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }); r.Code != http.StatusBadRequest {
		t.Fatalf("non-JSON content type: %d", r.Code)
	}
	for _, session := range []string{"s2", "s9"} {
		if r := peerInbound(a, http.MethodPost, send, body(session, "x", peerTestRequestID), token, nil); r.Code != http.StatusConflict {
			t.Fatalf("send to a session that cannot accept input (%s): %d %s", session, r.Code, r.Body)
		}
	}
	// Exactly 1 MiB is accepted and held as the only pending request.
	text := strings.Repeat("a", peerTerminalTextLimit-1) + "\n"
	if r := peerInbound(a, http.MethodPost, send, body("s1", text, peerTestRequestID), token, nil); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `"state":"queued"`) {
		t.Fatalf("1 MiB send: %d %s", r.Code, r.Body)
	}
	select {
	case <-a.windowActivate:
	default:
		t.Fatal("the receiving window was not raised")
	}
	second := strings.Replace(peerTestRequestID, "0", "f", 1)
	if r := peerInbound(a, http.MethodPost, send, body("s1", "second", second), token, nil); r.Code != http.StatusConflict {
		t.Fatalf("a second pending request was accepted: %d %s", r.Code, r.Body)
	}
	result := func(id string) (int, string) {
		r := peerInbound(a, http.MethodGet, "/api/peer/terminal/result?requestId="+id, "", token, nil)
		var reply struct{ State, Outcome, Message string }
		_ = json.Unmarshal(r.Body.Bytes(), &reply)
		if strings.Contains(r.Body.String(), "aaaa") {
			t.Fatal("a result echoed the text")
		}
		return r.Code, reply.State + "/" + reply.Outcome
	}
	if code, state := result(peerTestRequestID); code != http.StatusOK || state != "queued/" {
		t.Fatalf("queued result: %d %s", code, state)
	}
	if code, _ := result(second); code != http.StatusNotFound {
		t.Fatalf("unknown request: %d", code)
	}
	if code, _ := result("nonsense"); code != http.StatusNotFound {
		t.Fatalf("invalid request id: %d", code)
	}
	// The page takes the request once; the engine keeps no copy of the text.
	taken := peerReport(t, a, `[{"id":"s1","name":"lab-switch-01","type":"ssh","connected":true}]`)
	if taken == nil || taken.RequestID != peerTestRequestID || taken.Session != "s1" || taken.Text != text {
		t.Fatalf("the page did not receive the request: %+v", taken)
	}
	a.peer.mu.Lock()
	held := a.peer.request.payload
	a.peer.mu.Unlock()
	if held != nil {
		t.Fatal("the engine kept the text after the page took it")
	}
	if again := peerReport(t, a, `[{"id":"s1","name":"lab-switch-01","type":"ssh","connected":true}]`); again != nil {
		t.Fatal("the request was delivered twice")
	}
	if _, state := result(peerTestRequestID); state != "taken/" {
		t.Fatalf("taken result: %s", state)
	}
	if r := peerInbound(a, http.MethodPost, send, body("s1", "second", second), token, nil); r.Code != http.StatusConflict {
		t.Fatalf("a request was accepted while one was in progress: %d", r.Code)
	}
	for name, bad := range map[string]string{
		"editor outcome":   `{"requestId":"` + peerTestRequestID + `","ok":true,"outcome":"inserted"}`,
		"no outcome":       `{"requestId":"` + peerTestRequestID + `","ok":true}`,
		"failed + outcome": `{"requestId":"` + peerTestRequestID + `","ok":false,"outcome":"sent"}`,
		"unknown field":    `{"requestId":"` + peerTestRequestID + `","ok":true,"outcome":"sent","text":"x"}`,
		"bad id":           `{"requestId":"1","ok":true,"outcome":"sent"}`,
	} {
		if r := requestApp(a, http.MethodPost, "/api/peer-send/ack", bad, true); r.Code != http.StatusBadRequest {
			t.Fatalf("ack %s: %d %s", name, r.Code, r.Body)
		}
	}
	if r := requestApp(a, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+second+`","ok":true,"outcome":"sent"}`, true); r.Code != http.StatusConflict {
		t.Fatalf("ack for another request: %d", r.Code)
	}
	if r := requestApp(a, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+peerTestRequestID+`","ok":true,"outcome":"review"}`, true); r.Code != http.StatusOK {
		t.Fatalf("ack: %d %s", r.Code, r.Body)
	}
	if _, state := result(peerTestRequestID); state != "done/review" {
		t.Fatalf("acknowledged result: %s", state)
	}
	if r := requestApp(a, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+peerTestRequestID+`","ok":true,"outcome":"sent"}`, true); r.Code != http.StatusConflict {
		t.Fatalf("a finished request was acknowledged again: %d", r.Code)
	}
	// A finished request makes room for the next; a failure carries plain text.
	if r := peerInbound(a, http.MethodPost, send, body("s1", "second", second), token, nil); r.Code != http.StatusOK {
		t.Fatalf("next request: %d %s", r.Code, r.Body)
	}
	if peerReport(t, a, `[{"id":"s1","name":"lab-switch-01","type":"ssh","connected":true}]`) == nil {
		t.Fatal("next request was not delivered")
	}
	if r := requestApp(a, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+second+`","ok":false,"error":"That terminal session\n is no longer connected."}`, true); r.Code != http.StatusOK {
		t.Fatalf("failure ack: %d %s", r.Code, r.Body)
	}
	failed := peerInbound(a, http.MethodGet, "/api/peer/terminal/result?requestId="+second, "", token, nil)
	if !strings.Contains(failed.Body.String(), `"state":"failed"`) || !strings.Contains(failed.Body.String(), `"message":"That terminal session is no longer connected."`) {
		t.Fatalf("failure result: %s", failed.Body)
	}
	// A request the page never collects expires after a minute and is dropped.
	third := strings.Replace(peerTestRequestID, "0", "e", 1)
	if r := peerInbound(a, http.MethodPost, send, body("s1", "never collected", third), token, nil); r.Code != http.StatusOK {
		t.Fatalf("third request: %d %s", r.Code, r.Body)
	}
	advance(peerRequestLifetime - time.Second)
	if _, state := result(third); state != "queued/" {
		t.Fatalf("expired early: %s", state)
	}
	advance(2 * time.Second)
	if _, state := result(third); state != "expired/" {
		t.Fatalf("not expired: %s", state)
	}
	if got := peerReport(t, a, `[{"id":"s1","name":"lab-switch-01","type":"ssh","connected":true}]`); got != nil {
		t.Fatalf("an expired request was delivered: %+v", got)
	}
	a.peer.mu.Lock()
	held = a.peer.request.payload
	a.peer.mu.Unlock()
	if held != nil {
		t.Fatal("expired text stayed in memory")
	}
	// A page that stopped reporting is not ready, so nothing new is queued.
	advance(peerTargetsFresh + time.Second)
	if r := peerInbound(a, http.MethodPost, send, body("s1", "x", peerTestRequestID), token, nil); r.Code != http.StatusServiceUnavailable {
		t.Fatalf("send to a silent page: %d %s", r.Code, r.Body)
	}
	if r := peerInbound(a, http.MethodGet, "/api/peer/terminal/sessions", "", token, nil); !strings.Contains(r.Body.String(), `"ready":false`) || !strings.Contains(r.Body.String(), `"sessions":[]`) {
		t.Fatalf("a silent page still listed sessions: %s", r.Body)
	}
}

func TestPeerEditorInsertValidationAndDocumentSummaries(t *testing.T) {
	advance := peerTestClock(t)
	a := testApplication(t)
	a.windowMode = standaloneEditorMode
	a.configurePeerSend(t.TempDir())
	token := a.windowActivationToken
	const insert = "/api/peer/editor/insert"
	body := func(fields map[string]any) string {
		if _, ok := fields["requestId"]; !ok {
			fields["requestId"] = peerTestRequestID
		}
		b, _ := json.Marshal(fields)
		return string(b)
	}
	const uid = "0b6f5d3e-8a55-4b0e-9f3c-2d1f0a6c7e11"
	// The page's summary is validated strictly and reduced to file names.
	for name, bad := range map[string]string{
		"path field":       `[{"id":"` + uid + `","name":"a.txt","path":"C:\\Users\\operator\\a.txt"}]`,
		"content field":    `[{"id":"` + uid + `","name":"a.txt","content":"secret"}]`,
		"bad id":           `[{"id":"local:C:\\a.txt","name":"a.txt"}]`,
		"duplicate id":     `[{"id":"` + uid + `","name":"a"},{"id":"` + uid + `","name":"b"}]`,
		"control in name":  `[{"id":"` + uid + `","name":"a\u0007.txt"}]`,
		"long name":        `[{"id":"` + uid + `","name":"` + strings.Repeat("n", peerNameRunes+1) + `"}]`,
		"terminal fields":  `[{"id":"` + uid + `","name":"a","type":"ssh","connected":true}]`,
		"not a list":       `{"id":"x"}`,
		"too many targets": "[" + strings.TrimSuffix(strings.Repeat(`{"id":"d","name":"n"},`, peerDocumentLimit+1), ",") + "]",
	} {
		if r := requestApp(a, http.MethodPost, "/api/peer-send/report", `{"targets":`+bad+`}`, true); r.Code != http.StatusBadRequest {
			t.Fatalf("summary %s: %d %s", name, r.Code, r.Body)
		}
	}
	if r := requestApp(a, http.MethodPost, "/api/peer-send/report", `{"targets":[],"token":"x"}`, true); r.Code != http.StatusBadRequest {
		t.Fatalf("unknown field in the page report: %d", r.Code)
	}
	if r := requestApp(a, http.MethodPost, "/api/peer-send/inbox", `{}`, true); r.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST inbox: %d", r.Code)
	}
	if r := requestApp(a, http.MethodGet, "/api/peer-send/report", "", true); r.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET report: %d", r.Code)
	}
	// The engine asks for the summary until the page has given it once.
	if r := requestApp(a, http.MethodGet, "/api/peer-send/inbox", "", true); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `"report":true`) || !strings.Contains(r.Body.String(), `"request":null`) {
		t.Fatalf("first poll: %d %s", r.Code, r.Body)
	}
	report := `[{"id":"` + uid + `","name":"C:\\Users\\operator\\Documents\\switch notes.txt","unsaved":true,"active":true},{"id":"busy-1","name":"/home/operator/loading.cfg","busy":true}]`
	if got := peerReport(t, a, report); got != nil {
		t.Fatalf("unexpected request: %+v", got)
	}
	list := peerInbound(a, http.MethodGet, "/api/peer/editor/documents", "", token, nil)
	var listed struct {
		Ready     bool                  `json:"ready"`
		Documents []peerDocumentSummary `json:"documents"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil || !listed.Ready {
		t.Fatalf("documents reply: %s (%v)", list.Body, err)
	}
	want := []peerDocumentSummary{{UID: uid, Name: "switch notes.txt", Unsaved: true, Active: true}, {UID: "busy-1", Name: "loading.cfg", Busy: true}}
	if !reflect.DeepEqual(listed.Documents, want) {
		t.Fatalf("document summaries: %+v", listed.Documents)
	}
	for _, private := range []string{"operator", "Users", "Documents", "home", a.token, a.windowActivationToken, filepath.Dir(a.dataPath)} {
		if strings.Contains(list.Body.String(), private) {
			t.Fatalf("the document list exposed %q", private)
		}
	}
	for name, bad := range map[string]string{
		"no target":          body(map[string]any{"text": "x"}),
		"path as target":     body(map[string]any{"target": "C:/a.txt", "text": "x"}),
		"unknown position":   body(map[string]any{"target": uid, "where": "middle", "text": "x"}),
		"line without where": body(map[string]any{"target": uid, "where": "end", "line": 3, "text": "x"}),
		"line zero":          body(map[string]any{"target": uid, "where": "line", "line": 0, "text": "x"}),
		"new not at start":   body(map[string]any{"target": "new", "where": "end", "text": "x"}),
		"terminal field":     body(map[string]any{"target": "new", "session": "s1", "text": "x"}),
		"file path field":    body(map[string]any{"target": "new", "path": "C:/a.txt", "text": "x"}),
		"control in label":   body(map[string]any{"target": "new", "label": "a\nb", "text": "x"}),
		"no text":            body(map[string]any{"target": "new"}),
		"over 8 MiB":         body(map[string]any{"target": "new", "text": strings.Repeat("a", peerEditorTextLimit+1)}),
		"object as where":    `{"target":"` + uid + `","where":{"line":2},"text":"x","requestId":"` + peerTestRequestID + `"}`,
	} {
		if r := peerInbound(a, http.MethodPost, insert, bad, token, nil); r.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d %s", name, r.Code, r.Body)
		}
	}
	if r := peerInbound(a, http.MethodPost, insert, body(map[string]any{"target": "closed-doc", "where": "end", "text": "x"}), token, nil); r.Code != http.StatusConflict {
		t.Fatalf("insert into a closed document: %d %s", r.Code, r.Body)
	}
	if r := peerInbound(a, http.MethodPost, insert, body(map[string]any{"target": "busy-1", "where": "end", "text": "x"}), token, nil); r.Code != http.StatusConflict {
		t.Fatalf("insert into a busy document: %d %s", r.Code, r.Body)
	}
	text := strings.Repeat("é", peerEditorTextLimit/2)
	if r := peerInbound(a, http.MethodPost, insert, body(map[string]any{"target": uid, "where": "line", "line": 12, "text": text, "label": "Last command output from lab-switch-01"}), token, nil); r.Code != http.StatusOK {
		t.Fatalf("8 MiB insert: %d %s", r.Code, r.Body)
	}
	taken := peerReport(t, a, report)
	if taken == nil || taken.Target != uid || taken.Where != "line" || taken.Line != 12 || taken.Text != text || taken.Label != "Last command output from lab-switch-01" {
		t.Fatal("the editor page did not receive the insert")
	}
	if r := requestApp(a, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+peerTestRequestID+`","ok":true,"outcome":"sent"}`, true); r.Code != http.StatusBadRequest {
		t.Fatalf("terminal outcome accepted by the editor: %d", r.Code)
	}
	if r := requestApp(a, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+peerTestRequestID+`","ok":true,"outcome":"inserted"}`, true); r.Code != http.StatusOK {
		t.Fatalf("ack: %d %s", r.Code, r.Body)
	}
	// A new document can be queued while the editor's page is still starting;
	// the text waits at most a minute. A request the page took but never
	// confirmed ends the same way.
	advance(peerTargetsFresh + time.Second)
	second := strings.Replace(peerTestRequestID, "0", "f", 1)
	if r := peerInbound(a, http.MethodPost, insert, body(map[string]any{"target": uid, "where": "end", "text": "x", "requestId": second}), token, nil); r.Code != http.StatusServiceUnavailable {
		t.Fatalf("insert into a document of a silent page: %d %s", r.Code, r.Body)
	}
	if r := peerInbound(a, http.MethodPost, insert, body(map[string]any{"target": "new", "text": "starting up", "requestId": second}), token, nil); r.Code != http.StatusOK {
		t.Fatalf("new document while starting: %d %s", r.Code, r.Body)
	}
	if got := peerReport(t, a, `[]`); got == nil || got.Target != "new" || got.Where != "start" || got.Text != "starting up" {
		t.Fatalf("queued new document: %+v", got)
	}
	advance(peerRequestLifetime + time.Second)
	expired := peerInbound(a, http.MethodGet, "/api/peer/editor/result?requestId="+second, "", token, nil)
	if !strings.Contains(expired.Body.String(), `"state":"expired"`) || !strings.Contains(expired.Body.String(), "did not confirm") {
		t.Fatalf("unconfirmed request: %s", expired.Body)
	}
	if r := requestApp(a, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+second+`","ok":true,"outcome":"inserted"}`, true); r.Code != http.StatusConflict {
		t.Fatalf("late ack accepted: %d", r.Code)
	}
}

func TestPeerSendBetweenTwoStandaloneAppsOfOneDataRoot(t *testing.T) {
	root := t.TempDir()
	terminal, _ := peerTestApp(t, root, "terminal")
	editor, _ := peerTestApp(t, root, standaloneEditorMode)
	// Neither app is registered yet: each reports the other as not open.
	for _, a := range []*application{terminal, editor} {
		code, reply, raw := peerPage(t, a, http.MethodGet, "/api/peer-send/targets", "")
		if code != http.StatusOK || reply["running"] != false || len(reply["targets"].([]any)) != 0 {
			t.Fatalf("targets without the other app: %d %s", code, raw)
		}
	}
	if code, _, raw := peerPage(t, editor, http.MethodPost, "/api/peer-send/send", `{"session":"s1","text":"show version"}`); code != http.StatusConflict || !strings.Contains(raw, "Nothing was sent") || !strings.Contains(raw, "Choose Send to terminal again.") {
		t.Fatalf("send without the Terminal: %d %s", code, raw)
	}
	peerRegister(t, terminal, root)
	unregisterEditor := peerRegister(t, editor, root)
	secrets := []string{terminal.token, terminal.windowActivationToken, editor.token, editor.windowActivationToken, root}
	clean := func(name, raw string) {
		t.Helper()
		for _, secret := range secrets {
			if strings.Contains(raw, secret) {
				t.Fatalf("%s exposed a token or the data folder: %s", name, raw)
			}
		}
	}

	// ---- Text Editor -> Terminal ----
	code, reply, raw := peerPage(t, editor, http.MethodGet, "/api/peer-send/targets", "")
	if code != http.StatusOK || reply["running"] != true || reply["ready"] != false || reply["app"] != "terminal" || reply["name"] != "Network Toolkit Terminal" {
		t.Fatalf("Terminal running but its page not ready: %d %s", code, raw)
	}
	clean("targets", raw)
	if code, _, raw = peerPageTick(t, terminal, `{"targets":[{"id":"s1","name":"lab-switch-01","type":"ssh","connected":true,"active":true},{"id":"s2","name":"COM3","type":"serial","connected":true}]}`); code != http.StatusOK || !strings.Contains(raw, `"request":null`) {
		t.Fatalf("Terminal page report: %d %s", code, raw)
	}
	code, reply, raw = peerPage(t, editor, http.MethodGet, "/api/peer-send/targets", "")
	targets, _ := reply["targets"].([]any)
	if code != http.StatusOK || reply["ready"] != true || len(targets) != 2 || targets[0].(map[string]any)["name"] != "lab-switch-01" || targets[1].(map[string]any)["type"] != "serial" {
		t.Fatalf("Terminal sessions as the editor sees them: %d %s", code, raw)
	}
	clean("targets", raw)
	for name, bad := range map[string]string{
		"request id from the page": `{"session":"s1","text":"x","requestId":"` + peerTestRequestID + `"}`,
		"unknown field":            `{"session":"s1","text":"x","token":"y"}`,
		"editor fields":            `{"target":"new","text":"x"}`,
		"too much text":            `{"session":"s1","text":"` + strings.Repeat("a", peerTerminalTextLimit+1) + `"}`,
	} {
		if code, _, raw = peerPage(t, editor, http.MethodPost, "/api/peer-send/send", bad); code != http.StatusBadRequest {
			t.Fatalf("editor page send %s: %d %s", name, code, raw)
		}
	}
	if code, _, raw = peerPage(t, editor, http.MethodPost, "/api/peer-send/send", `{"session":"s7","text":"x"}`); code != http.StatusConflict || !strings.Contains(raw, "no longer connected") {
		t.Fatalf("send to a missing session: %d %s", code, raw)
	}
	const commands = "show version\nshow ip interface brief\n"
	sendBody, _ := json.Marshal(map[string]string{"session": "s1", "text": commands})
	code, reply, raw = peerPage(t, editor, http.MethodPost, "/api/peer-send/send", string(sendBody))
	id, _ := reply["requestId"].(string)
	if code != http.StatusOK || !peerRequestID.MatchString(id) || reply["state"] != "queued" {
		t.Fatalf("editor send: %d %s", code, raw)
	}
	clean("send", raw)
	select {
	case <-terminal.windowActivate:
	case <-time.After(2 * time.Second):
		t.Fatal("the Terminal window was not raised")
	}
	if len(terminal.terminals.List()) != 0 {
		t.Fatal("a peer send opened a connection")
	}
	if code, _, raw = peerPage(t, editor, http.MethodPost, "/api/peer-send/send", string(sendBody)); code != http.StatusConflict || !strings.Contains(raw, "still waiting in Network Toolkit Terminal") {
		t.Fatalf("second send while one is pending: %d %s", code, raw)
	}
	if code, reply, raw = peerPage(t, editor, http.MethodGet, "/api/peer-send/result?requestId="+id, ""); code != http.StatusOK || reply["state"] != "queued" {
		t.Fatalf("queued result: %d %s", code, raw)
	}
	code, reply, raw = peerPageTick(t, terminal, `{"targets":[{"id":"s1","name":"lab-switch-01","type":"ssh","connected":true}]}`)
	request, _ := reply["request"].(map[string]any)
	if code != http.StatusOK || request["requestId"] != id || request["session"] != "s1" || request["text"] != commands || reply["from"] != "Network Toolkit Text Editor" {
		t.Fatalf("Terminal page inbox: %d %s", code, raw)
	}
	clean("inbox", raw)
	if code, reply, _ = peerPage(t, editor, http.MethodGet, "/api/peer-send/result?requestId="+id, ""); reply["state"] != "taken" {
		t.Fatalf("taken result: %d %v", code, reply)
	}
	if code, _, raw = peerPage(t, terminal, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+id+`","ok":true,"outcome":"review"}`); code != http.StatusOK {
		t.Fatalf("Terminal ack: %d %s", code, raw)
	}
	if code, reply, raw = peerPage(t, editor, http.MethodGet, "/api/peer-send/result?requestId="+id, ""); reply["state"] != "done" || reply["outcome"] != "review" {
		t.Fatalf("final result: %d %s", code, raw)
	}
	clean("result", raw)
	if code, _, raw = peerPage(t, editor, http.MethodGet, "/api/peer-send/result?requestId=xyz", ""); code != http.StatusBadRequest {
		t.Fatalf("invalid result id: %d %s", code, raw)
	}

	// ---- Terminal -> Text Editor ----
	const uid = "0b6f5d3e-8a55-4b0e-9f3c-2d1f0a6c7e11"
	documents := `{"targets":[{"id":"` + uid + `","name":"notes.txt","unsaved":true,"active":true}]}`
	if code, _, raw = peerPageTick(t, editor, documents); code != http.StatusOK {
		t.Fatalf("editor page report: %d %s", code, raw)
	}
	code, reply, raw = peerPage(t, terminal, http.MethodGet, "/api/peer-send/targets", "")
	targets, _ = reply["targets"].([]any)
	if code != http.StatusOK || reply["app"] != "editor" || reply["name"] != "Network Toolkit Text Editor" || reply["ready"] != true || len(targets) != 1 || targets[0].(map[string]any)["id"] != uid || targets[0].(map[string]any)["unsaved"] != true {
		t.Fatalf("editor documents as the Terminal sees them: %d %s", code, raw)
	}
	clean("targets", raw)
	const output = "10:00:00.000 UTC Thu Nov 5 2026\n"
	insertBody, _ := json.Marshal(map[string]any{"target": uid, "where": "line", "line": 2, "text": output, "label": "Last command output from lab-switch-01"})
	code, reply, raw = peerPage(t, terminal, http.MethodPost, "/api/peer-send/send", string(insertBody))
	id, _ = reply["requestId"].(string)
	if code != http.StatusOK || !peerRequestID.MatchString(id) {
		t.Fatalf("terminal send: %d %s", code, raw)
	}
	select {
	case <-editor.windowActivate:
	case <-time.After(2 * time.Second):
		t.Fatal("the Text Editor window was not raised")
	}
	code, reply, raw = peerPageTick(t, editor, documents)
	request, _ = reply["request"].(map[string]any)
	if code != http.StatusOK || request["requestId"] != id || request["target"] != uid || request["where"] != "line" || request["line"] != float64(2) || request["text"] != output || request["label"] != "Last command output from lab-switch-01" || reply["from"] != "Network Toolkit Terminal" {
		t.Fatalf("editor page inbox: %d %s", code, raw)
	}
	if code, _, raw = peerPage(t, editor, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+id+`","ok":false,"error":"Close the document comparison first."}`); code != http.StatusOK {
		t.Fatalf("editor ack: %d %s", code, raw)
	}
	if code, reply, raw = peerPage(t, terminal, http.MethodGet, "/api/peer-send/result?requestId="+id, ""); reply["state"] != "failed" || reply["message"] != "Close the document comparison first." {
		t.Fatalf("failure reported to the Terminal: %d %s", code, raw)
	}
	if code, _, raw = peerPage(t, terminal, http.MethodPost, "/api/peer-send/send", `{"session":"s1","text":"x"}`); code != http.StatusBadRequest {
		t.Fatalf("terminal page sent a terminal request: %d %s", code, raw)
	}

	// ---- The Text Editor closes; the Terminal reopens it and delivers ----
	unregisterEditor()
	if code, reply, raw = peerPage(t, terminal, http.MethodGet, "/api/peer-send/targets", ""); reply["running"] != false {
		t.Fatalf("closed editor still reported: %d %s", code, raw)
	}
	if code, reply, raw = peerPage(t, terminal, http.MethodGet, "/api/peer-send/result?requestId="+id, ""); reply["state"] != "lost" {
		t.Fatalf("result after the editor closed: %d %s", code, raw)
	}
	launches := 0
	terminal.peer.mu.Lock()
	terminal.peer.launch = func(context.Context) error {
		launches++
		go func() {
			time.Sleep(200 * time.Millisecond)
			if _, err := editor.registerWorkspaceWindow(peerWorkspaceDir(root, standaloneEditorMode)); err != nil {
				t.Error(err)
			}
		}()
		return nil
	}
	terminal.peer.mu.Unlock()
	if code, reply, raw = peerPage(t, terminal, http.MethodPost, "/api/peer-send/open", ""); code != http.StatusOK || reply["running"] != true || launches != 1 {
		t.Fatalf("open the Text Editor: %d %s (launches %d)", code, raw, launches)
	}
	newBody, _ := json.Marshal(map[string]any{"target": "new", "text": output, "label": "Selected terminal text from lab-switch-01"})
	code, reply, raw = peerPage(t, terminal, http.MethodPost, "/api/peer-send/send", string(newBody))
	id, _ = reply["requestId"].(string)
	if code != http.StatusOK || id == "" {
		t.Fatalf("send to a new document: %d %s", code, raw)
	}
	_, reply, raw = peerPageTick(t, editor, `{"targets":[]}`)
	request, _ = reply["request"].(map[string]any)
	if request["requestId"] != id || request["target"] != "new" || request["where"] != "start" || request["text"] != output {
		t.Fatalf("new document request: %s", raw)
	}
	if code, _, raw = peerPage(t, editor, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+id+`","ok":true,"outcome":"inserted"}`); code != http.StatusOK {
		t.Fatalf("editor ack: %d %s", code, raw)
	}
	if _, reply, raw = peerPage(t, terminal, http.MethodGet, "/api/peer-send/result?requestId="+id, ""); reply["state"] != "done" || reply["outcome"] != "inserted" {
		t.Fatalf("new document result: %s", raw)
	}
	// A launch that fails ends with a plain explanation.
	editor.peer.mu.Lock()
	editor.peer.launch = func(context.Context) error { return errors.New("synthetic launch failure") }
	editor.peer.mu.Unlock()
	if code, _, raw = peerPage(t, editor, http.MethodPost, "/api/peer-send/open", ""); code != http.StatusServiceUnavailable || !strings.Contains(raw, "Network Toolkit Terminal could not be opened") {
		t.Fatalf("failed launch: %d %s", code, raw)
	}
}

func TestPeerOpenWaitsForTheOtherAppForABoundedTime(t *testing.T) {
	previous := peerLaunchWait
	peerLaunchWait = 400 * time.Millisecond
	defer func() { peerLaunchWait = previous }()
	terminal, _ := peerTestApp(t, t.TempDir(), "terminal")
	terminal.peer.mu.Lock()
	terminal.peer.launch = func(context.Context) error { return nil } // started, but never answers
	terminal.peer.mu.Unlock()
	started := time.Now()
	code, _, raw := peerPage(t, terminal, http.MethodPost, "/api/peer-send/open", "")
	if code != http.StatusServiceUnavailable || !strings.Contains(raw, "Network Toolkit Text Editor did not become ready") {
		t.Fatalf("open without an answering editor: %d %s", code, raw)
	}
	if waited := time.Since(started); waited < 300*time.Millisecond || waited > 5*time.Second {
		t.Fatalf("the wait was not bounded as configured: %s", waited)
	}
	if code, _, raw = peerPage(t, terminal, http.MethodGet, "/api/peer-send/open", ""); code != http.StatusMethodNotAllowed {
		t.Fatalf("GET open: %d %s", code, raw)
	}
}

func TestPeerSendIgnoresAnEndpointThatIsNotTheOtherApp(t *testing.T) {
	root := t.TempDir()
	editor, _ := peerTestApp(t, root, standaloneEditorMode)
	received := make(chan string, 4)
	// Another program now listens on the port a stale record names.
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- r.Method + " " + r.URL.Path + " " + string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true,"windowMode":"terminal","instanceId":"00000000000000000000000000000000","ready":true,"sessions":[{"id":"s1","name":"trap","type":"ssh","connected":true}],"requestId":"x","state":"queued"}`)
	}))
	defer other.Close()
	dir := peerWorkspaceDir(root, "terminal")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	record, _ := json.Marshal(terminalWindowInstance{Version: 2, Host: strings.TrimPrefix(other.URL, "http://"), Token: randomHex(32), InstanceID: randomHex(16), PID: os.Getpid()})
	stored, err := workspaceInstanceGuard.seal(append(record, '\n'))
	if err != nil {
		t.Fatal(err)
	}
	if err = writeAtomic(filepath.Join(dir, terminalInstanceFilename), stored); err != nil {
		t.Fatal(err)
	}
	if code, reply, raw := peerPage(t, editor, http.MethodGet, "/api/peer-send/targets", ""); code != http.StatusOK || reply["running"] != false || strings.Contains(raw, "trap") {
		t.Fatalf("an unconfirmed endpoint was trusted: %d %s", code, raw)
	}
	if code, _, raw := peerPage(t, editor, http.MethodPost, "/api/peer-send/send", `{"session":"s1","text":"enable secret synthetic"}`); code != http.StatusConflict {
		t.Fatalf("send to an unconfirmed endpoint: %d %s", code, raw)
	}
	close(received)
	for request := range received {
		if strings.Contains(request, "synthetic") || !strings.HasPrefix(request, "GET /api/peer/terminal/sessions") {
			t.Fatalf("text or an unexpected request reached another program: %s", request)
		}
	}
	// A record that is readable by other users is not used either.
	if runtime.GOOS != "windows" {
		if err = os.Chmod(filepath.Join(dir, terminalInstanceFilename), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err = readPeerInstance(dir); err == nil {
			t.Fatal("a record other users can read was trusted for peer send")
		}
	}
}

func TestPeerLaunchUsesFixedArgumentsOnly(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Portable Tools")
	for _, test := range []struct {
		mode     string
		headless bool
		want     []string
	}{
		{"terminal", false, []string{"--terminal", "--data-dir", root}},
		{"terminal", true, []string{"--terminal", "--data-dir", root, "--no-browser"}},
		{standaloneEditorMode, false, []string{"--editor-standalone", "--data-dir", root}},
		{standaloneEditorMode, true, []string{"--editor-standalone", "--data-dir", root, "--no-browser"}},
	} {
		got, err := peerLaunchArguments(root, test.mode, test.headless)
		if err != nil || !reflect.DeepEqual(got, test.want) {
			t.Fatalf("%s: %v (%v)", test.mode, got, err)
		}
	}
	if peerOf("main") != "" || peerOf("detached") != "" || peerOf("editor") != "" || peerOf("terminal") != standaloneEditorMode || peerOf(standaloneEditorMode) != "terminal" {
		t.Fatal("peer send is offered to the wrong windows")
	}
	if got := peerWorkspaceDir(root, "terminal"); got != filepath.Join(root, terminalWorkspaceName) {
		t.Fatal(got)
	}
	if got := peerWorkspaceDir(root, standaloneEditorMode); got != filepath.Join(root, standaloneEditorDirectory) {
		t.Fatal(got)
	}
}

func TestPeerPlainTextAndTargetNormalization(t *testing.T) {
	if got := peerPlainText("  first\r\n\tsecond\x00\x1b[31m  third ", 300); got != "first second [31m third" {
		t.Fatalf("%q", got)
	}
	if got := peerPlainText(strings.Repeat("é", 400), 300); len([]rune(got)) != 300 {
		t.Fatal(len([]rune(got)))
	}
	sessions, err := normalizePeerTargets("terminal", []peerTarget{{ID: "a1", Name: "  ", Type: "cmd", Connected: true}, {ID: "a2", Name: "x", Type: "powershell"}})
	if err != nil || len(sessions) != 1 || sessions[0].Name != "Terminal" || sessions[0].Type != "cmd" {
		t.Fatalf("%+v %v", sessions, err)
	}
	for _, bad := range [][]peerTarget{
		{{ID: "", Name: "x", Connected: true}},
		{{ID: "a b", Name: "x", Connected: true}},
		{{ID: "a1", Name: "x", Type: "SSH", Connected: true}},
		{{ID: "a1", Name: "x", Connected: true, Busy: true}},
		{{ID: strings.Repeat("a", 65), Name: "x", Connected: true}},
	} {
		if _, err := normalizePeerTargets("terminal", bad); err == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
	many := make([]peerTarget, peerSessionLimit+1)
	for i := range many {
		many[i] = peerTarget{ID: "s" + string(rune('a'+i%26)) + string(rune('a'+i/26)), Name: "x", Connected: true}
	}
	if _, err := normalizePeerTargets("terminal", many); err == nil {
		t.Fatal("unbounded session list accepted")
	}
	var encoded bytes.Buffer
	_ = json.NewEncoder(&encoded).Encode(peerPayload{RequestID: peerTestRequestID, Target: "new", Where: "start", Text: "x"})
	if strings.Contains(encoded.String(), "session") || strings.Contains(encoded.String(), "line") {
		t.Fatalf("empty fields are sent: %s", encoded.String())
	}
}

// ---- Receiver order: the other standalone app, the main window, auto-start ----

// peerLivePage plays an app's page until the test ends: it reports what can
// receive text, polls the inbox every 50 ms and passes on each request it
// collects. A page that stays silent is simply not started.
func peerLivePage(t *testing.T, a *application, report string) <-chan map[string]any {
	t.Helper()
	collected := make(chan map[string]any, 8)
	done := make(chan struct{})
	finished := make(chan struct{})
	call := func(method, path, body string) map[string]any {
		var reader io.Reader
		if body != "" {
			reader = strings.NewReader(body)
		}
		req, err := http.NewRequest(method, "http://"+a.host+path, reader)
		if err != nil {
			return nil
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("X-Session-Token", a.token)
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil
		}
		defer response.Body.Close()
		var decoded map[string]any
		_ = json.NewDecoder(response.Body).Decode(&decoded)
		return decoded
	}
	go func() {
		defer close(finished)
		for {
			call(http.MethodPost, "/api/peer-send/report", report)
			if reply := call(http.MethodGet, "/api/peer-send/inbox", ""); reply != nil {
				if request, ok := reply["request"].(map[string]any); ok {
					request["kind"], request["from"] = reply["kind"], reply["from"]
					collected <- request
				}
			}
			select {
			case <-done:
				return
			case <-time.After(50 * time.Millisecond):
			}
		}
	}()
	t.Cleanup(func() { close(done); <-finished })
	return collected
}

func peerAwaitRequest(t *testing.T, collected <-chan map[string]any, what string) map[string]any {
	t.Helper()
	select {
	case request := <-collected:
		return request
	case <-time.After(5 * time.Second):
		t.Fatalf("%s: the page never received the text", what)
		return nil
	}
}

// peerAwaitResult polls the sender's page route until the state is reached.
func peerAwaitResult(t *testing.T, a *application, id, state string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		code, reply, raw := peerPage(t, a, http.MethodGet, "/api/peer-send/result?requestId="+id, "")
		if code == http.StatusOK && reply["state"] == state {
			return reply
		}
		if time.Now().After(deadline) {
			t.Fatalf("result never became %q: %d %s", state, code, raw)
		}
		time.Sleep(40 * time.Millisecond)
	}
}

func peerRaised(t *testing.T, a *application, what string) {
	t.Helper()
	select {
	case <-a.windowActivate:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s was not brought to the front", what)
	}
}

const (
	peerMainSessions  = `{"targets":[{"id":"m1","name":"lab-core-01","type":"ssh","connected":true,"active":true}],"documents":[{"id":"doc-main-1","name":"main notes.txt","active":true}]}`
	peerOwnSessions   = `{"targets":[{"id":"s1","name":"lab-switch-01","type":"ssh","connected":true,"active":true}]}`
	peerOwnDocuments  = `{"targets":[{"id":"doc-own-1","name":"notes.txt","active":true}]}`
	peerNoTargetsBody = `{"targets":[]}`
)

func TestPeerSendPrefersTheOtherStandaloneAppThenTheMainWindow(t *testing.T) {
	root := t.TempDir()
	editor, _ := peerTestApp(t, root, standaloneEditorMode)
	terminal, _ := peerTestApp(t, root, "terminal")
	main, _ := peerTestApp(t, root, "main")
	unregisterEditor := peerRegister(t, editor, root)
	unregisterTerminal := peerRegister(t, terminal, root)
	peerRegister(t, main, root)
	mainPage := peerLivePage(t, main, peerMainSessions)
	terminalPage := peerLivePage(t, terminal, peerOwnSessions)
	secrets := []string{editor.token, editor.windowActivationToken, terminal.token, terminal.windowActivationToken, main.token, main.windowActivationToken, root}
	clean := func(name, raw string) {
		t.Helper()
		for _, secret := range secrets {
			if strings.Contains(raw, secret) {
				t.Fatalf("%s exposed a token or the data folder: %s", name, raw)
			}
		}
	}
	listed := func(a *application, receiver, name, target string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			code, reply, raw := peerPage(t, a, http.MethodGet, "/api/peer-send/targets", "")
			targets, _ := reply["targets"].([]any)
			if code == http.StatusOK && reply["receiver"] == receiver && reply["name"] == name && reply["running"] == true && reply["ready"] == true && len(targets) == 1 && targets[0].(map[string]any)["id"] == target {
				clean("targets", raw)
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("wanted %s (%s) with %s: %d %s", name, receiver, target, code, raw)
			}
			time.Sleep(40 * time.Millisecond)
		}
	}

	// 1. Both are running: the standalone Terminal is the receiver.
	listed(editor, "standalone", "Network Toolkit Terminal", "s1")
	code, reply, raw := peerPage(t, editor, http.MethodPost, "/api/peer-send/send", `{"session":"s1","text":"show version"}`)
	id, _ := reply["requestId"].(string)
	if code != http.StatusOK || reply["receiver"] != "standalone" || reply["name"] != "Network Toolkit Terminal" || reply["state"] != "queued" {
		t.Fatalf("send with both running: %d %s", code, raw)
	}
	clean("send", raw)
	if request := peerAwaitRequest(t, terminalPage, "standalone Terminal"); request["requestId"] != id || request["session"] != "s1" || request["text"] != "show version" || request["kind"] != "terminal" {
		t.Fatalf("standalone Terminal request: %v", request)
	}
	select {
	case request := <-mainPage:
		t.Fatalf("the main window also received the text: %v", request)
	case <-time.After(300 * time.Millisecond):
	}
	if code, _, raw = peerPage(t, terminal, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+id+`","ok":true,"outcome":"sent"}`); code != http.StatusOK {
		t.Fatalf("ack: %d %s", code, raw)
	}
	if got := peerAwaitResult(t, editor, id, "done"); got["outcome"] != "sent" || got["name"] != "Network Toolkit Terminal" {
		t.Fatalf("result: %v", got)
	}
	// A session of the main window is not reachable while the page was shown the
	// standalone Terminal's sessions: the text never goes somewhere unlisted.
	if code, _, raw = peerPage(t, editor, http.MethodPost, "/api/peer-send/send", `{"session":"m1","text":"x"}`); code != http.StatusConflict || !strings.Contains(raw, "no longer connected") {
		t.Fatalf("send to a session of another receiver: %d %s", code, raw)
	}

	// 2. The standalone Terminal closes: the main window's Terminal page receives.
	unregisterTerminal()
	// The page still shows the Terminal app's sessions: a send for them is
	// refused instead of going to the main window.
	if code, _, raw = peerPage(t, editor, http.MethodPost, "/api/peer-send/send", `{"session":"s1","text":"x"}`); code != http.StatusServiceUnavailable || !strings.Contains(raw, "Network Toolkit Terminal is not open any more. Nothing was sent.") {
		t.Fatalf("send to a receiver that closed: %d %s", code, raw)
	}
	listed(editor, "main", "Network Toolkit", "m1")
	const commands = "show version\nshow ip interface brief\n"
	body, _ := json.Marshal(map[string]string{"session": "m1", "text": commands})
	code, reply, raw = peerPage(t, editor, http.MethodPost, "/api/peer-send/send", string(body))
	id, _ = reply["requestId"].(string)
	if code != http.StatusOK || reply["receiver"] != "main" || reply["name"] != "Network Toolkit" {
		t.Fatalf("send to the main window: %d %s", code, raw)
	}
	clean("send", raw)
	peerRaised(t, main, "the main window")
	request := peerAwaitRequest(t, mainPage, "main window terminal")
	if request["requestId"] != id || request["session"] != "m1" || request["text"] != commands || request["kind"] != "terminal" || request["from"] != "Network Toolkit Text Editor" {
		t.Fatalf("main window terminal request: %v", request)
	}
	if len(main.terminals.List()) != 0 {
		t.Fatal("a peer send opened a connection in the main window")
	}
	// The main window's page answers by the kind of the request.
	if code, _, raw = peerPage(t, main, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+id+`","ok":true,"outcome":"inserted"}`); code != http.StatusBadRequest {
		t.Fatalf("editor outcome for terminal text: %d %s", code, raw)
	}
	if code, _, raw = peerPage(t, main, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+id+`","ok":true,"outcome":"review"}`); code != http.StatusOK {
		t.Fatalf("main ack: %d %s", code, raw)
	}
	if got := peerAwaitResult(t, editor, id, "done"); got["outcome"] != "review" || got["name"] != "Network Toolkit" {
		t.Fatalf("result from the main window: %v", got)
	}

	// The standalone Terminal sends to the main window's Text Editor when the
	// standalone Text Editor is not the one running.
	// standalone Text Editor is not running (close it for this part).
	sender := terminal
	unregisterEditor()
	deadline := time.Now().Add(5 * time.Second)
	for {
		code, reply, raw = peerPage(t, sender, http.MethodGet, "/api/peer-send/targets", "")
		documents, _ := reply["targets"].([]any)
		if code == http.StatusOK && reply["receiver"] == "main" && reply["name"] == "Network Toolkit" && reply["app"] == "editor" && len(documents) == 1 && documents[0].(map[string]any)["id"] == "doc-main-1" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("main window documents: %d %s", code, raw)
		}
		time.Sleep(40 * time.Millisecond)
	}
	const output = "10:00:00.000 UTC Thu Nov 5 2026\n"
	insert, _ := json.Marshal(map[string]any{"target": "doc-main-1", "where": "pick", "text": output, "label": "Last command output from lab-switch-01"})
	code, reply, raw = peerPage(t, sender, http.MethodPost, "/api/peer-send/send", string(insert))
	id, _ = reply["requestId"].(string)
	if code != http.StatusOK || reply["receiver"] != "main" {
		t.Fatalf("terminal send to the main window: %d %s", code, raw)
	}
	peerRaised(t, main, "the main window")
	request = peerAwaitRequest(t, mainPage, "main window editor")
	if request["requestId"] != id || request["target"] != "doc-main-1" || request["where"] != "pick" || request["text"] != output || request["kind"] != "editor" || request["from"] != "Network Toolkit Terminal" {
		t.Fatalf("main window editor request: %v", request)
	}
	if code, _, raw = peerPage(t, main, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+id+`","ok":true,"outcome":"sent"}`); code != http.StatusBadRequest {
		t.Fatalf("terminal outcome for editor text: %d %s", code, raw)
	}
	if code, _, raw = peerPage(t, main, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+id+`","ok":true,"outcome":"pick"}`); code != http.StatusOK {
		t.Fatalf("main ack: %d %s", code, raw)
	}
	if got := peerAwaitResult(t, sender, id, "done"); got["outcome"] != "pick" || got["name"] != "Network Toolkit" {
		t.Fatalf("result from the main window: %v", got)
	}
	// A new document goes to the main window too, with no launch.
	launches := 0
	sender.peer.mu.Lock()
	sender.peer.launch = func(context.Context) error { launches++; return nil }
	sender.peer.mu.Unlock()
	code, reply, raw = peerPage(t, sender, http.MethodPost, "/api/peer-send/send", `{"target":"new","text":"to main"}`)
	if code != http.StatusOK || reply["receiver"] != "main" || reply["state"] != "queued" || launches != 0 {
		t.Fatalf("new document with the main window running: %d %s (launches %d)", code, raw, launches)
	}
	if request = peerAwaitRequest(t, mainPage, "main window new document"); request["target"] != "new" || request["text"] != "to main" || request["kind"] != "editor" {
		t.Fatalf("main window new document: %v", request)
	}
}

func TestPeerSendSkipsAMainWindowWithoutARunningPage(t *testing.T) {
	root := t.TempDir()
	terminal, _ := peerTestApp(t, root, "terminal")
	main, _ := peerTestApp(t, root, "main")
	peerRegister(t, main, root)
	// The main engine answers, but no page polls it (its window is closed or
	// still loading): it cannot show a review or the position bar.
	code, reply, raw := peerPage(t, terminal, http.MethodGet, "/api/peer-send/targets", "")
	if code != http.StatusOK || reply["running"] != false || reply["receiver"] != "" || reply["name"] != "Network Toolkit Text Editor" {
		t.Fatalf("a main window without a page was chosen: %d %s", code, raw)
	}
	if _, has := reply["note"]; has {
		t.Fatalf("a current main window was reported as skipped: %s", raw)
	}
}

func TestPeerSendStartsTheOtherAppAndDeliversTheTextOnce(t *testing.T) {
	root := t.TempDir()
	terminal, _ := peerTestApp(t, root, "terminal")
	editor, _ := peerTestApp(t, root, standaloneEditorMode)
	editorPage := peerLivePage(t, editor, peerNoTargetsBody) // its page runs as soon as it is "started"
	launches := 0
	terminal.peer.mu.Lock()
	terminal.peer.launch = func(context.Context) error {
		launches++
		go func() {
			time.Sleep(700 * time.Millisecond)
			if _, err := editor.registerWorkspaceWindow(peerWorkspaceDir(root, standaloneEditorMode)); err != nil {
				t.Error(err)
			}
		}()
		return nil
	}
	terminal.peer.mu.Unlock()
	// Nothing is running: the page is told so, with no question to answer.
	if code, reply, raw := peerPage(t, terminal, http.MethodGet, "/api/peer-send/targets", ""); code != http.StatusOK || reply["running"] != false || reply["name"] != "Network Toolkit Text Editor" {
		t.Fatalf("targets with nothing running: %d %s", code, raw)
	}
	const output = "10:00:00.000 UTC Thu Nov 5 2026\n"
	body, _ := json.Marshal(map[string]any{"target": "new", "text": output, "label": "Last command output from lab-switch-01"})
	code, reply, raw := peerPage(t, terminal, http.MethodPost, "/api/peer-send/send", string(body))
	id, _ := reply["requestId"].(string)
	if code != http.StatusOK || reply["state"] != "starting" || reply["receiver"] != "standalone" || reply["name"] != "Network Toolkit Text Editor" || !peerRequestID.MatchString(id) {
		t.Fatalf("send with nothing running: %d %s", code, raw)
	}
	if strings.Contains(raw, "10:00:00") {
		t.Fatal("the reply echoed the text")
	}
	// While the app starts the text is in this engine's memory only.
	if code, reply, raw = peerPage(t, terminal, http.MethodGet, "/api/peer-send/result?requestId="+id, ""); code != http.StatusOK || reply["state"] != "starting" || strings.Contains(raw, "10:00:00") {
		t.Fatalf("result while starting: %d %s", code, raw)
	}
	if code, _, raw = peerPage(t, terminal, http.MethodPost, "/api/peer-send/send", string(body)); code != http.StatusConflict || !strings.Contains(raw, "still waiting for Network Toolkit Text Editor to open") {
		t.Fatalf("a second send while the app starts: %d %s", code, raw)
	}
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		_ = filepath.WalkDir(filepath.Join(root, entry.Name()), func(path string, d os.DirEntry, err error) error {
			if err == nil && d.Type().IsRegular() {
				if b, readErr := os.ReadFile(path); readErr == nil && bytes.Contains(b, []byte("10:00:00.000")) {
					t.Errorf("the waiting text was written to %s", path)
				}
			}
			return nil
		})
	}
	// The record appears, the page is ready, and the text sent ONCE arrives.
	request := peerAwaitRequest(t, editorPage, "started Text Editor")
	if request["requestId"] != id || request["target"] != "new" || request["where"] != "start" || request["text"] != output || request["label"] != "Last command output from lab-switch-01" {
		t.Fatalf("delivered request: %v", request)
	}
	if launches != 1 {
		t.Fatalf("launches: %d", launches)
	}
	peerRaised(t, editor, "the started Text Editor")
	if code, _, raw = peerPage(t, editor, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+id+`","ok":true,"outcome":"inserted"}`); code != http.StatusOK {
		t.Fatalf("ack: %d %s", code, raw)
	}
	if got := peerAwaitResult(t, terminal, id, "done"); got["outcome"] != "inserted" || got["name"] != "Network Toolkit Text Editor" {
		t.Fatalf("result: %v", got)
	}
	select {
	case again := <-editorPage:
		t.Fatalf("the text was delivered twice: %v", again)
	case <-time.After(300 * time.Millisecond):
	}
	// Now that it runs, the next send is direct and starts nothing.
	code, reply, raw = peerPage(t, terminal, http.MethodPost, "/api/peer-send/send", `{"target":"new","text":"second"}`)
	if code != http.StatusOK || reply["state"] != "queued" || launches != 1 {
		t.Fatalf("send to the running app: %d %s (launches %d)", code, raw, launches)
	}
	// A terminal session cannot be chosen before the Terminal runs, so the Text
	// Editor never queues terminal text for a start: it is refused.
	alone, _ := peerTestApp(t, t.TempDir(), standaloneEditorMode)
	started := false
	alone.peer.mu.Lock()
	alone.peer.launch = func(context.Context) error { started = true; return nil }
	alone.peer.mu.Unlock()
	if code, _, raw = peerPage(t, alone, http.MethodPost, "/api/peer-send/send", `{"session":"s1","text":"reload"}`); code != http.StatusConflict || !strings.Contains(raw, "Nothing was sent") || !strings.Contains(raw, "Choose Send to terminal again.") || started {
		t.Fatalf("terminal text with nothing running: %d %s (started %v)", code, raw, started)
	}
}

func TestPeerSendStartThatTimesOutOrIsAbandonedDeliversNothing(t *testing.T) {
	previous := peerLaunchWait
	peerLaunchWait = 400 * time.Millisecond
	defer func() { peerLaunchWait = previous }()
	root := t.TempDir()
	terminal, _ := peerTestApp(t, root, "terminal")
	editor, _ := peerTestApp(t, root, standaloneEditorMode)
	editorPage := peerLivePage(t, editor, peerNoTargetsBody)
	terminal.peer.mu.Lock()
	terminal.peer.launch = func(context.Context) error { return nil } // started, but it never registers
	terminal.peer.mu.Unlock()
	code, reply, raw := peerPage(t, terminal, http.MethodPost, "/api/peer-send/send", `{"target":"new","text":"never delivered"}`)
	id, _ := reply["requestId"].(string)
	if code != http.StatusOK || reply["state"] != "starting" {
		t.Fatalf("send: %d %s", code, raw)
	}
	failed := peerAwaitResult(t, terminal, id, "failed")
	if message, _ := failed["message"].(string); failed["local"] != true || !strings.Contains(message, "Network Toolkit Text Editor did not become ready") || !strings.HasSuffix(message, "Nothing was sent.") || strings.Contains(message, "never delivered") {
		t.Fatalf("timed-out start: %v", failed)
	}
	// The app comes up afterwards: the abandoned text must not arrive late.
	peerRegister(t, editor, root)
	select {
	case request := <-editorPage:
		t.Fatalf("text arrived after the start was given up: %v", request)
	case <-time.After(500 * time.Millisecond):
	}
	// A launch that fails outright says so.
	second, _ := peerTestApp(t, t.TempDir(), "terminal")
	second.peer.mu.Lock()
	second.peer.launch = func(context.Context) error { return errors.New("synthetic launch failure") }
	second.peer.mu.Unlock()
	code, reply, raw = peerPage(t, second, http.MethodPost, "/api/peer-send/send", `{"target":"new","text":"x"}`)
	id, _ = reply["requestId"].(string)
	if code != http.StatusOK {
		t.Fatalf("send: %d %s", code, raw)
	}
	if got := peerAwaitResult(t, second, id, "failed"); !strings.Contains(got["message"].(string), "Network Toolkit Text Editor could not be opened") {
		t.Fatalf("failed launch: %v", got)
	}
	// The sender closes while its text waits: the send is dropped.
	peerLaunchWait = 30 * time.Second
	closing, _ := peerTestApp(t, t.TempDir(), "terminal")
	target, _ := peerTestApp(t, closing.peerRoot(), standaloneEditorMode)
	targetPage := peerLivePage(t, target, peerNoTargetsBody)
	closing.peer.mu.Lock()
	closing.peer.launch = func(context.Context) error { return nil }
	closing.peer.mu.Unlock()
	code, reply, raw = peerPage(t, closing, http.MethodPost, "/api/peer-send/send", `{"target":"new","text":"dropped at close"}`)
	id, _ = reply["requestId"].(string)
	if code != http.StatusOK || reply["state"] != "starting" {
		t.Fatalf("send: %d %s", code, raw)
	}
	closing.stop()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if o, ok := closing.peerOutboundFor(id); ok && o.state == peerFailed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the waiting send outlived the app")
		}
		time.Sleep(20 * time.Millisecond)
	}
	peerRegister(t, target, closing.peerRoot())
	select {
	case request := <-targetPage:
		t.Fatalf("a closed app delivered its waiting text: %v", request)
	case <-time.After(500 * time.Millisecond):
	}
}

// peerOlderEngine stands in for a running app of an older version: its record
// is valid and protected, but its engine has no /api/peer/ route.
func peerOlderEngine(t *testing.T, dir string) (requests func() []string) {
	t.Helper()
	var mu sync.Mutex
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path+" "+string(body))
		mu.Unlock()
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	record, _ := json.Marshal(terminalWindowInstance{Version: 2, Host: strings.TrimPrefix(server.URL, "http://"), Token: randomHex(32), InstanceID: randomHex(16), PID: os.Getpid()})
	stored, err := workspaceInstanceGuard.seal(append(record, '\n'))
	if err != nil {
		t.Fatal(err)
	}
	if err = writeAtomic(filepath.Join(dir, terminalInstanceFilename), stored); err != nil {
		t.Fatal(err)
	}
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), seen...)
	}
}

func TestPeerSendSkipsOlderAppsAndSaysSo(t *testing.T) {
	root := t.TempDir()
	editor, _ := peerTestApp(t, root, standaloneEditorMode)
	main, _ := peerTestApp(t, root, "main")
	peerRegister(t, main, root)
	mainPage := peerLivePage(t, main, peerMainSessions)
	// An older standalone Terminal is running: valid record, no peer routes.
	older := peerOlderEngine(t, peerWorkspaceDir(root, "terminal"))
	const skipped = "Network Toolkit Terminal was skipped: it is open from an older version"
	var reply map[string]any
	var raw string
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, reply, raw = peerPage(t, editor, http.MethodGet, "/api/peer-send/targets", "")
		if reply["receiver"] == "main" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the main window was not chosen past the older Terminal: %s", raw)
		}
		time.Sleep(40 * time.Millisecond)
	}
	if note, _ := reply["note"].(string); !strings.Contains(note, skipped) || reply["name"] != "Network Toolkit" {
		t.Fatalf("the skipped app was not named: %s", raw)
	}
	code, reply, raw := peerPage(t, editor, http.MethodPost, "/api/peer-send/send", `{"session":"m1","text":"enable secret synthetic"}`)
	if code != http.StatusOK || reply["receiver"] != "main" {
		t.Fatalf("send past the older app: %d %s", code, raw)
	}
	if request := peerAwaitRequest(t, mainPage, "main window"); request["text"] != "enable secret synthetic" {
		t.Fatalf("request: %v", request)
	}
	for _, request := range older() {
		if strings.Contains(request, "synthetic") || !strings.HasPrefix(request, "GET /api/peer/terminal/sessions") {
			t.Fatalf("text or an unexpected request reached the older app: %s", request)
		}
	}
	if len(older()) == 0 {
		t.Fatal("the older app was never probed")
	}

	// A plain record (an older Terminal wrote those) or one this user cannot
	// read is skipped the same way, without contacting its endpoint.
	plainRoot := t.TempDir()
	sender, _ := peerTestApp(t, plainRoot, standaloneEditorMode)
	dir := peerWorkspaceDir(plainRoot, "terminal")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	contacted := false
	trap := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { contacted = true }))
	defer trap.Close()
	record, _ := json.Marshal(terminalWindowInstance{Version: 2, Host: strings.TrimPrefix(trap.URL, "http://"), Token: randomHex(32), InstanceID: randomHex(16), PID: os.Getpid()})
	path := filepath.Join(dir, terminalInstanceFilename)
	if runtime.GOOS == "windows" {
		if err := writeAtomic(path, append(record, '\n')); err != nil { // plain where protection is required
			t.Fatal(err)
		}
	} else {
		if err := os.WriteFile(path, append(record, '\n'), 0644); err != nil { // readable by other users
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0644); err != nil {
			t.Fatal(err)
		}
	}
	code, reply, raw = peerPage(t, sender, http.MethodGet, "/api/peer-send/targets", "")
	if note, _ := reply["note"].(string); code != http.StatusOK || reply["running"] != false || !strings.Contains(note, skipped) {
		t.Fatalf("an untrusted record: %d %s", code, raw)
	}
	if code, _, raw = peerPage(t, sender, http.MethodPost, "/api/peer-send/send", `{"session":"s1","text":"x"}`); code != http.StatusConflict || !strings.Contains(raw, "Nothing was sent") {
		t.Fatalf("send with only an untrusted record: %d %s", code, raw)
	}
	if contacted {
		t.Fatal("the endpoint of an untrusted record was contacted")
	}
	// An older main window is skipped too; the other standalone app is started.
	olderMainRoot := t.TempDir()
	terminal, _ := peerTestApp(t, olderMainRoot, "terminal")
	olderMain := peerOlderEngine(t, olderMainRoot)
	code, reply, raw = peerPage(t, terminal, http.MethodGet, "/api/peer-send/targets", "")
	if note, _ := reply["note"].(string); code != http.StatusOK || reply["running"] != false || !strings.Contains(note, "Network Toolkit was skipped: it is open from an older version") {
		t.Fatalf("an older main window: %d %s", code, raw)
	}
	for _, request := range olderMain() {
		if !strings.HasPrefix(request, "GET /api/peer/editor/documents") {
			t.Fatalf("unexpected request to the older main window: %s", request)
		}
	}
}

// ---- The main window as a receiver ----

func TestPeerMainWindowReceivesWithTheSameGuards(t *testing.T) {
	advance := peerTestClock(t)
	a := testApplication(t)
	a.configurePeerSend(t.TempDir())
	if a.windowMode != "main" || a.peerRoot() == "" || a.peerSenderRoot() != "" {
		t.Fatal("the main window must receive and never send")
	}
	token := a.windowActivationToken
	const sessions, documents = "/api/peer/terminal/sessions", "/api/peer/editor/documents"
	const send, insert = "/api/peer/terminal/send", "/api/peer/editor/insert"
	for _, list := range []string{sessions, documents} {
		if r := peerInbound(a, http.MethodGet, list, "", token, nil); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `"windowMode":"main"`) || !strings.Contains(r.Body.String(), `"ready":false`) {
			t.Fatalf("%s: %d %s", list, r.Code, r.Body)
		}
		for name, r := range map[string]*httptest.ResponseRecorder{
			"no token":                peerInbound(a, http.MethodGet, list, "", "", nil),
			"wrong token":             peerInbound(a, http.MethodGet, list, "", strings.Repeat("0", 64), nil),
			"session token":           peerInbound(a, http.MethodGet, list, "", "", func(r *http.Request) { r.Header.Set("X-Session-Token", a.token) }),
			"api token as activation": peerInbound(a, http.MethodGet, list, "", a.token, nil),
		} {
			if r.Code != http.StatusUnauthorized || strings.Contains(r.Body.String(), a.windowInstanceID) {
				t.Fatalf("%s %s: status %d %s", list, name, r.Code, r.Body)
			}
		}
		for name, change := range map[string]func(*http.Request){
			"other computer": func(r *http.Request) { r.RemoteAddr = "192.0.2.7:50123" },
			"no address":     func(r *http.Request) { r.RemoteAddr = "" },
			"page origin":    func(r *http.Request) { r.Header.Set("Origin", "http://"+a.host) },
			"cross origin":   func(r *http.Request) { r.Header.Set("Origin", "https://attacker.invalid") },
			"fetch site":     func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "same-origin") },
			"fetch mode":     func(r *http.Request) { r.Header.Set("Sec-Fetch-Mode", "cors") },
			"fetch dest":     func(r *http.Request) { r.Header.Set("Sec-Fetch-Dest", "empty") },
			"referer":        func(r *http.Request) { r.Header.Set("Referer", "http://"+a.host+"/") },
			"cookie":         func(r *http.Request) { r.Header.Set("Cookie", "ntk_window=1") },
		} {
			if r := peerInbound(a, http.MethodGet, list, "", token, change); r.Code != http.StatusForbidden {
				t.Fatalf("%s %s: status %d %s", list, name, r.Code, r.Body)
			}
		}
		if r := peerInbound(a, http.MethodGet, list, "", token, func(r *http.Request) { r.Host = "attacker.invalid" }); r.Code != http.StatusForbidden {
			t.Fatalf("%s: foreign Host accepted: %d", list, r.Code)
		}
		if r := peerInbound(a, http.MethodPost, list, "", token, nil); r.Code != http.StatusMethodNotAllowed {
			t.Fatalf("POST %s: %d", list, r.Code)
		}
	}
	for _, path := range []string{"/api/peer/", "/api/peer/terminal", "/api/peer/editor/other", "/api/peer/main/sessions", "/api/peer/terminal/insert", "/api/peer/editor/send"} {
		if r := peerInbound(a, http.MethodGet, path, "", token, nil); r.Code != http.StatusNotFound {
			t.Fatalf("%s: status %d", path, r.Code)
		}
	}
	// The activation token still opens nothing else in the main window.
	for _, path := range []string{"/api/state", "/api/terminals", "/api/peer-send/inbox", "/api/credentials", "/api/terminals/known-hosts", "/api/status", "/api/editor-views"} {
		if r := peerInbound(a, http.MethodGet, path, "", token, nil); r.Code != http.StatusUnauthorized {
			t.Fatalf("the activation token reached %s: %d", path, r.Code)
		}
	}
	// The main window's page receives; it has no sending routes.
	for _, route := range []string{"targets", "result?requestId=" + peerTestRequestID} {
		if r := requestApp(a, http.MethodGet, "/api/peer-send/"+route, "", true); r.Code != http.StatusNotFound {
			t.Fatalf("main window page route %s: %d", route, r.Code)
		}
	}
	for _, route := range []string{"open", "send"} {
		if r := requestApp(a, http.MethodPost, "/api/peer-send/"+route, `{"target":"new","text":"x"}`, true); r.Code != http.StatusNotFound {
			t.Fatalf("main window page route %s: %d", route, r.Code)
		}
	}
	if r := requestApp(a, http.MethodGet, "/api/peer-send/inbox", "", false); r.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated inbox: %d", r.Code)
	}
	terminalBody := func(session, text, id string) string {
		b, _ := json.Marshal(map[string]string{"session": session, "text": text, "requestId": id})
		return string(b)
	}
	editorBody := func(fields map[string]any) string {
		if _, ok := fields["requestId"]; !ok {
			fields["requestId"] = peerTestRequestID
		}
		b, _ := json.Marshal(fields)
		return string(b)
	}
	// Nothing is queued for a session or an open document before the page reported.
	if r := peerInbound(a, http.MethodPost, send, terminalBody("m1", "x", peerTestRequestID), token, nil); r.Code != http.StatusServiceUnavailable || !strings.Contains(r.Body.String(), "Network Toolkit is not ready yet") {
		t.Fatalf("send before the page reported: %d %s", r.Code, r.Body)
	}
	if r := requestApp(a, http.MethodGet, "/api/peer-send/inbox", "", true); !strings.Contains(r.Body.String(), `"report":true`) {
		t.Fatalf("the engine did not ask its page: %s", r.Body)
	}
	// The page's summary: sessions as "targets", documents as "documents", both strict.
	for name, bad := range map[string]string{
		"document with a path":   `{"targets":[],"documents":[{"id":"d1","name":"a.txt","path":"C:\\a.txt"}]}`,
		"document as a session":  `{"targets":[],"documents":[{"id":"d1","name":"a","type":"ssh","connected":true}]}`,
		"session marked unsaved": `{"targets":[{"id":"m1","name":"x","type":"ssh","connected":true,"unsaved":true}],"documents":[]}`,
		"unknown field":          `{"targets":[],"documents":[],"token":"x"}`,
		"documents not a list":   `{"targets":[],"documents":{"id":"d1"}}`,
	} {
		if r := requestApp(a, http.MethodPost, "/api/peer-send/report", bad, true); r.Code != http.StatusBadRequest {
			t.Fatalf("summary %s: %d %s", name, r.Code, r.Body)
		}
	}
	const report = `{"targets":[{"id":"m1","name":"lab-core-01","type":"ssh","connected":true,"active":true},{"id":"m2","name":"closed","type":"telnet","connected":false}],"documents":[{"id":"doc-1","name":"C:\\Users\\operator\\main notes.txt","unsaved":true,"active":true},{"id":"busy-1","name":"loading.cfg","busy":true}]}`
	tick := func() (*peerPayload, string) {
		t.Helper()
		if r := requestApp(a, http.MethodPost, "/api/peer-send/report", report, true); r.Code != http.StatusOK {
			t.Fatalf("page report refused: %d %s", r.Code, r.Body)
		}
		r := requestApp(a, http.MethodGet, "/api/peer-send/inbox", "", true)
		var reply struct {
			Request *peerPayload `json:"request"`
			Kind    string       `json:"kind"`
		}
		if err := json.Unmarshal(r.Body.Bytes(), &reply); err != nil || r.Code != http.StatusOK {
			t.Fatalf("page poll: %d %s", r.Code, r.Body)
		}
		return reply.Request, reply.Kind
	}
	if got, _ := tick(); got != nil {
		t.Fatalf("unexpected request: %+v", got)
	}
	list := peerInbound(a, http.MethodGet, sessions, "", token, nil)
	if !strings.Contains(list.Body.String(), `"sessions":[{"id":"m1","name":"lab-core-01","type":"ssh","connected":true,"active":true}]`) || strings.Contains(list.Body.String(), "main notes") {
		t.Fatalf("main window sessions: %s", list.Body)
	}
	docs := peerInbound(a, http.MethodGet, documents, "", token, nil)
	if !strings.Contains(docs.Body.String(), `"documents":[{"uid":"doc-1","name":"main notes.txt","unsaved":true,"active":true},{"uid":"busy-1","name":"loading.cfg","busy":true}]`) || strings.Contains(docs.Body.String(), "lab-core") {
		t.Fatalf("main window documents: %s", docs.Body)
	}
	for _, reply := range []string{list.Body.String(), docs.Body.String()} {
		for _, private := range []string{"operator", "Users", a.token, a.windowActivationToken, filepath.Dir(a.dataPath)} {
			if strings.Contains(reply, private) {
				t.Fatalf("a list exposed %q: %s", private, reply)
			}
		}
	}
	// Strict JSON and the size limits of each kind.
	for name, bad := range map[string]string{
		"unknown field":    `{"session":"m1","text":"x","requestId":"` + peerTestRequestID + `","password":"secret"}`,
		"credential field": `{"session":"m1","text":"x","requestId":"` + peerTestRequestID + `","credentialId":"c1"}`,
		"host key field":   `{"session":"m1","text":"x","requestId":"` + peerTestRequestID + `","hostKey":"ssh-ed25519 AAAA"}`,
		"enter field":      `{"session":"m1","text":"x","requestId":"` + peerTestRequestID + `","enter":true}`,
		"editor field":     `{"session":"m1","text":"x","requestId":"` + peerTestRequestID + `","target":"new"}`,
		"path as session":  terminalBody("../m1", "x", peerTestRequestID),
		"no text":          terminalBody("m1", "", peerTestRequestID),
		"bad request id":   terminalBody("m1", "x", "abc"),
		"two values":       terminalBody("m1", "x", peerTestRequestID) + `{}`,
		"null":             `null`,
		"over 1 MiB":       terminalBody("m1", strings.Repeat("a", peerTerminalTextLimit+1), peerTestRequestID),
	} {
		if r := peerInbound(a, http.MethodPost, send, bad, token, nil); r.Code != http.StatusBadRequest {
			t.Fatalf("terminal %s: status %d %s", name, r.Code, r.Body)
		}
	}
	for name, bad := range map[string]string{
		"file path field":  editorBody(map[string]any{"target": "new", "path": "C:/a.txt", "text": "x"}),
		"path as target":   editorBody(map[string]any{"target": "C:/a.txt", "text": "x"}),
		"terminal field":   editorBody(map[string]any{"target": "new", "session": "m1", "text": "x"}),
		"new not at start": editorBody(map[string]any{"target": "new", "where": "end", "text": "x"}),
		"over 8 MiB":       editorBody(map[string]any{"target": "new", "text": strings.Repeat("a", peerEditorTextLimit+1)}),
	} {
		if r := peerInbound(a, http.MethodPost, insert, bad, token, nil); r.Code != http.StatusBadRequest {
			t.Fatalf("editor %s: status %d %s", name, r.Code, r.Body)
		}
	}
	if r := peerInbound(a, http.MethodPost, send, terminalBody("m1", "x", peerTestRequestID), token, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }); r.Code != http.StatusBadRequest {
		t.Fatalf("non-JSON content type: %d", r.Code)
	}
	for _, session := range []string{"m2", "doc-1", "m9"} {
		if r := peerInbound(a, http.MethodPost, send, terminalBody(session, "x", peerTestRequestID), token, nil); r.Code != http.StatusConflict {
			t.Fatalf("send to %s: %d %s", session, r.Code, r.Body)
		}
	}
	for _, target := range []string{"m1", "busy-1", "closed-doc"} {
		if r := peerInbound(a, http.MethodPost, insert, editorBody(map[string]any{"target": target, "where": "end", "text": "x"}), token, nil); r.Code != http.StatusConflict {
			t.Fatalf("insert into %s: %d %s", target, r.Code, r.Body)
		}
	}
	// One pending request for the whole window, whichever page it is for.
	text := strings.Repeat("a", peerTerminalTextLimit-1) + "\n"
	if r := peerInbound(a, http.MethodPost, send, terminalBody("m1", text, peerTestRequestID), token, nil); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `"state":"queued"`) || !strings.Contains(r.Body.String(), `"windowMode":"main"`) {
		t.Fatalf("1 MiB send: %d %s", r.Code, r.Body)
	}
	select {
	case <-a.windowActivate:
	default:
		t.Fatal("the main window was not brought to the front")
	}
	second := strings.Replace(peerTestRequestID, "0", "f", 1)
	if r := peerInbound(a, http.MethodPost, insert, editorBody(map[string]any{"target": "new", "text": "second", "requestId": second}), token, nil); r.Code != http.StatusConflict || !strings.Contains(r.Body.String(), "still waiting in Network Toolkit.") {
		t.Fatalf("a second pending request was accepted: %d %s", r.Code, r.Body)
	}
	if r := peerInbound(a, http.MethodGet, "/api/peer/editor/result?requestId="+peerTestRequestID, "", token, nil); !strings.Contains(r.Body.String(), `"state":"queued"`) || strings.Contains(r.Body.String(), "aaaa") {
		t.Fatalf("result: %s", r.Body)
	}
	taken, kind := tick()
	if taken == nil || kind != "terminal" || taken.RequestID != peerTestRequestID || taken.Session != "m1" || taken.Text != text {
		t.Fatalf("the page did not receive the terminal text (kind %q)", kind)
	}
	a.peer.mu.Lock()
	held := a.peer.request.payload
	a.peer.mu.Unlock()
	if held != nil {
		t.Fatal("the engine kept the text after the page took it")
	}
	if again, _ := tick(); again != nil {
		t.Fatal("the request was delivered twice")
	}
	if r := requestApp(a, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+peerTestRequestID+`","ok":true,"outcome":"pick"}`, true); r.Code != http.StatusBadRequest {
		t.Fatalf("an editor outcome was accepted for terminal text: %d", r.Code)
	}
	if r := requestApp(a, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+peerTestRequestID+`","ok":true,"outcome":"run"}`, true); r.Code != http.StatusBadRequest {
		t.Fatalf("an unknown outcome was accepted: %d", r.Code)
	}
	if r := requestApp(a, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+peerTestRequestID+`","ok":true,"outcome":"review"}`, true); r.Code != http.StatusOK {
		t.Fatalf("ack: %d %s", r.Code, r.Body)
	}
	// Editor text: 8 MiB, with a position, and a new document.
	large := strings.Repeat("é", peerEditorTextLimit/2)
	if r := peerInbound(a, http.MethodPost, insert, editorBody(map[string]any{"target": "doc-1", "where": "line", "line": 12, "text": large, "label": "Last command output from lab-switch-01", "requestId": second}), token, nil); r.Code != http.StatusOK {
		t.Fatalf("8 MiB insert: %d %s", r.Code, r.Body)
	}
	taken, kind = tick()
	if taken == nil || kind != "editor" || taken.Target != "doc-1" || taken.Where != "line" || taken.Line != 12 || taken.Text != large {
		t.Fatalf("the page did not receive the insert (kind %q)", kind)
	}
	if r := requestApp(a, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+second+`","ok":true,"outcome":"sent"}`, true); r.Code != http.StatusBadRequest {
		t.Fatalf("a terminal outcome was accepted for editor text: %d", r.Code)
	}
	if r := requestApp(a, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+second+`","ok":true,"outcome":"inserted"}`, true); r.Code != http.StatusOK {
		t.Fatalf("ack: %d %s", r.Code, r.Body)
	}
	// At most a minute: a request the page never collects is dropped.
	third := strings.Replace(peerTestRequestID, "0", "e", 1)
	if r := peerInbound(a, http.MethodPost, insert, editorBody(map[string]any{"target": "new", "text": "never collected", "requestId": third}), token, nil); r.Code != http.StatusOK {
		t.Fatalf("third request: %d %s", r.Code, r.Body)
	}
	advance(peerRequestLifetime + time.Second)
	if r := peerInbound(a, http.MethodGet, "/api/peer/editor/result?requestId="+third, "", token, nil); !strings.Contains(r.Body.String(), `"state":"expired"`) {
		t.Fatalf("not expired: %s", r.Body)
	}
	a.peer.mu.Lock()
	held = a.peer.request.payload
	a.peer.mu.Unlock()
	if held != nil {
		t.Fatal("expired text stayed in memory")
	}
	if r := peerInbound(a, http.MethodPost, send, terminalBody("m1", "x", peerTestRequestID), token, nil); r.Code != http.StatusServiceUnavailable {
		t.Fatalf("send to a silent page: %d %s", r.Code, r.Body)
	}
	a.shutdownRequested.Store(true)
	if r := peerInbound(a, http.MethodGet, sessions, "", token, nil); r.Code != http.StatusServiceUnavailable || !strings.Contains(r.Body.String(), "Network Toolkit is closing") {
		t.Fatalf("a closing main window accepted a peer request: %d %s", r.Code, r.Body)
	}
}

// peerRecordingReceiver is a receiver that records exactly what a sending
// engine puts on the wire: every header and every body.
func peerRecordingReceiver(t *testing.T, dir, mode string) func() []*http.Request {
	t.Helper()
	instance := randomHex(16)
	var mu sync.Mutex
	var seen []*http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		copied := r.Clone(context.Background())
		copied.Body = io.NopCloser(bytes.NewReader(body))
		mu.Lock()
		seen = append(seen, copied)
		mu.Unlock()
		reply := map[string]any{"ok": true, "windowMode": mode, "instanceId": instance, "ready": true, "sessions": []peerSessionSummary{{ID: "s1", Name: "lab-switch-01", Type: "ssh", Connected: true}}, "documents": []peerDocumentSummary{{UID: "doc-1", Name: "notes.txt"}}}
		if r.Method == http.MethodPost {
			var sent struct {
				RequestID string `json:"requestId"`
			}
			_ = json.Unmarshal(body, &sent)
			reply["requestId"], reply["state"] = sent.RequestID, "queued"
		}
		jsonResponse(w, http.StatusOK, reply)
	}))
	t.Cleanup(server.Close)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	record, _ := json.Marshal(terminalWindowInstance{Version: 2, Host: strings.TrimPrefix(server.URL, "http://"), Token: randomHex(32), InstanceID: instance, PID: os.Getpid()})
	stored, err := workspaceInstanceGuard.seal(append(record, '\n'))
	if err != nil {
		t.Fatal(err)
	}
	if err = writeAtomic(filepath.Join(dir, terminalInstanceFilename), stored); err != nil {
		t.Fatal(err)
	}
	return func() []*http.Request {
		mu.Lock()
		defer mu.Unlock()
		return append([]*http.Request(nil), seen...)
	}
}

func TestPeerSendPutsNothingButTheTextOnTheWire(t *testing.T) {
	for _, receiver := range []string{"standalone", "main"} {
		for _, own := range []string{standaloneEditorMode, "terminal"} {
			root := filepath.Join(t.TempDir(), "Data Folder")
			sender, _ := peerTestApp(t, root, own)
			mode := peerOf(own)
			if receiver == "main" {
				mode = "main"
			}
			recorded := peerRecordingReceiver(t, peerWorkspaceDir(root, mode), mode)
			if code, reply, raw := peerPage(t, sender, http.MethodGet, "/api/peer-send/targets", ""); code != http.StatusOK || reply["receiver"] != receiver {
				t.Fatalf("%s to %s: targets %d %s", own, receiver, code, raw)
			}
			body, want := `{"session":"s1","text":"show version\n"}`, map[string]any{"session": "s1", "text": "show version\n"}
			if own == "terminal" {
				body, want = `{"target":"doc-1","where":"line","line":3,"text":"output\n","label":"Last command output"}`, map[string]any{"target": "doc-1", "where": "line", "line": float64(3), "text": "output\n", "label": "Last command output"}
			}
			code, reply, raw := peerPage(t, sender, http.MethodPost, "/api/peer-send/send", body)
			if code != http.StatusOK {
				t.Fatalf("%s to %s: send %d %s", own, receiver, code, raw)
			}
			want["requestId"] = reply["requestId"]
			posts := 0
			for _, request := range recorded() {
				raw, _ := io.ReadAll(request.Body)
				for name := range request.Header {
					if !map[string]bool{"X-Window-Activation-Token": true, "Content-Type": true, "Content-Length": true, "User-Agent": true, "Accept-Encoding": true, "Connection": true}[name] {
						t.Fatalf("%s to %s: unexpected header %s", own, receiver, name)
					}
				}
				for _, private := range []string{sender.token, sender.windowActivationToken, root, "Data Folder", sender.dataPath} {
					if strings.Contains(string(raw), private) || strings.Contains(request.URL.String(), private) {
						t.Fatalf("%s to %s: a token or a path crossed: %s %s", own, receiver, request.URL, raw)
					}
					for _, values := range request.Header {
						if strings.Contains(strings.Join(values, " "), private) {
							t.Fatalf("%s to %s: a token or a path crossed in a header", own, receiver)
						}
					}
				}
				if request.Method != http.MethodPost {
					if len(raw) != 0 {
						t.Fatalf("%s to %s: a GET carried a body: %s", own, receiver, raw)
					}
					continue
				}
				posts++
				var got map[string]any
				if err := json.Unmarshal(raw, &got); err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("%s to %s: the wire carried %s, want %v", own, receiver, raw, want)
				}
			}
			if posts != 1 {
				t.Fatalf("%s to %s: %d POSTs", own, receiver, posts)
			}
		}
	}
}

// ---- Hardening from the security review ----

// The engine itself enforces "only the listed receiver": a send that names a
// session or an open document is refused unless this app's page was shown that
// receiver's list, and it stays with that receiver whatever starts later.
func TestPeerSendRefusesAnExistingTargetWithoutAListedReceiver(t *testing.T) {
	root := t.TempDir()
	editor, _ := peerTestApp(t, root, standaloneEditorMode)
	terminal, _ := peerTestApp(t, root, "terminal")
	main, _ := peerTestApp(t, root, "main")
	peerRegister(t, main, root)
	mainPage := peerLivePage(t, main, `{"targets":[{"id":"s1","name":"main-s1","type":"ssh","connected":true}],"documents":[{"id":"doc-main-1","name":"main notes.txt"}]}`)
	nothingArrives := func(what string) {
		t.Helper()
		select {
		case request := <-mainPage:
			t.Fatalf("%s: the main window received %v", what, request)
		case <-time.After(400 * time.Millisecond):
		}
	}
	// Wait until the main window would be chosen, without listing it to the senders.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if rc, _, _, _, _ := editor.peerChoose(context.Background()); rc != nil && rc.mode == "main" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the main window never became a receiver")
		}
		time.Sleep(40 * time.Millisecond)
	}
	code, _, raw := peerPage(t, editor, http.MethodPost, "/api/peer-send/send", `{"session":"s1","text":"reload"}`)
	if code != http.StatusConflict || !strings.Contains(raw, "Nothing was sent") || !strings.Contains(raw, "Choose Send to terminal again.") {
		t.Fatalf("terminal text without a listed receiver: %d %s", code, raw)
	}
	code, _, raw = peerPage(t, terminal, http.MethodPost, "/api/peer-send/send", `{"target":"doc-main-1","where":"end","text":"output"}`)
	if code != http.StatusConflict || !strings.Contains(raw, "Choose Send to editor again.") {
		t.Fatalf("editor text for a document without a listed receiver: %d %s", code, raw)
	}
	nothingArrives("unlisted sends")
	// Listed: the same send goes to the main window ...
	if code, reply, raw := peerPage(t, editor, http.MethodGet, "/api/peer-send/targets", ""); code != http.StatusOK || reply["receiver"] != "main" {
		t.Fatalf("targets: %d %s", code, raw)
	}
	// ... and stays there when the standalone Terminal starts with a session of the same id.
	standalone, _ := peerTestApp(t, root, "terminal")
	peerRegister(t, standalone, root)
	standalonePage := peerLivePage(t, standalone, peerOwnSessions)
	deadline = time.Now().Add(5 * time.Second)
	for {
		if rc, _, ready, _, _ := editor.peerChoose(context.Background()); rc != nil && rc.mode == "terminal" && ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the standalone Terminal never became the first receiver")
		}
		time.Sleep(40 * time.Millisecond)
	}
	code, reply, raw := peerPage(t, editor, http.MethodPost, "/api/peer-send/send", `{"session":"s1","text":"reload"}`)
	if code != http.StatusOK || reply["receiver"] != "main" || reply["name"] != "Network Toolkit" {
		t.Fatalf("send to the listed receiver: %d %s", code, raw)
	}
	if request := peerAwaitRequest(t, mainPage, "listed main window"); request["session"] != "s1" || request["text"] != "reload" {
		t.Fatalf("request: %v", request)
	}
	select {
	case request := <-standalonePage:
		t.Fatalf("the text went to an app the page had not listed: %v", request)
	case <-time.After(400 * time.Millisecond):
	}
	// A listing that found nobody clears the listed receiver again.
	empty, _ := peerTestApp(t, t.TempDir(), standaloneEditorMode)
	if _, reply, _ := peerPage(t, empty, http.MethodGet, "/api/peer-send/targets", ""); reply["running"] != false {
		t.Fatal("unexpected receiver")
	}
	if code, _, raw = peerPage(t, empty, http.MethodPost, "/api/peer-send/send", `{"session":"s1","text":"x"}`); code != http.StatusConflict {
		t.Fatalf("send after an empty listing: %d %s", code, raw)
	}
}

// Text is delivered only to the instance that was probed. If the record names
// another instance by the time of delivery, nothing is sent to it: the check
// happens before the request is made.
func TestPeerDeliverIsPinnedToTheProbedInstance(t *testing.T) {
	root := t.TempDir()
	editor, _ := peerTestApp(t, root, standaloneEditorMode)
	dir := peerWorkspaceDir(root, "terminal")
	rc := peerReceiver{mode: "terminal", dir: dir}
	first := peerRecordingReceiver(t, dir, "terminal")
	_, ready, probed, err := editor.peerProbe(context.Background(), rc)
	if err != nil || !ready || probed == "" {
		t.Fatalf("probe: %v %v %q", err, ready, probed)
	}
	// Between the probe and the delivery another instance registers in the
	// same place (the first app closed; something else answers now).
	second := peerRecordingReceiver(t, dir, "terminal")
	payload := &peerPayload{RequestID: peerTestRequestID, Session: "s1", Text: "enable secret synthetic"}
	code, err := editor.peerDeliver(context.Background(), rc, probed, payload)
	if err == nil || code != http.StatusServiceUnavailable || !strings.Contains(err.Error(), "Nothing was sent") {
		t.Fatalf("delivery to a changed record: %d %v", code, err)
	}
	if got := second(); len(got) != 0 {
		t.Fatalf("the new instance was contacted %d times", len(got))
	}
	for _, request := range first() {
		if request.Method != http.MethodGet {
			t.Fatalf("the text was posted: %s %s", request.Method, request.URL)
		}
	}
	// Without an instance to pin to, nothing is sent either.
	if _, err = editor.peerDeliver(context.Background(), rc, "", payload); err == nil || len(second()) != 0 {
		t.Fatalf("an unpinned delivery was attempted: %v", err)
	}
	// The same through the page route: the page was shown the first instance.
	root2 := t.TempDir()
	sender, _ := peerTestApp(t, root2, standaloneEditorMode)
	dir2 := peerWorkspaceDir(root2, "terminal")
	peerRecordingReceiver(t, dir2, "terminal")
	if code, reply, raw := peerPage(t, sender, http.MethodGet, "/api/peer-send/targets", ""); code != http.StatusOK || reply["running"] != true {
		t.Fatalf("targets: %d %s", code, raw)
	}
	replaced := peerRecordingReceiver(t, dir2, "terminal")
	if code, _, raw := peerPage(t, sender, http.MethodPost, "/api/peer-send/send", `{"session":"s1","text":"enable secret synthetic"}`); code != http.StatusServiceUnavailable || !strings.Contains(raw, "is not open any more. Nothing was sent.") {
		t.Fatalf("send after the listed instance was replaced: %d %s", code, raw)
	}
	for _, request := range replaced() {
		body, _ := io.ReadAll(request.Body)
		if request.Method != http.MethodGet || strings.Contains(string(body), "synthetic") {
			t.Fatalf("text reached the replacing instance: %s %s %s", request.Method, request.URL, body)
		}
	}
	// The pinned identity is the record's: a result is not asked of another instance.
	other := peerRecordingReceiver(t, dir, "terminal")
	if _, _, err = editor.peerCall(context.Background(), rc, probed, http.MethodGet, "/api/peer/terminal/result?requestId="+peerTestRequestID, nil, time.Second); !errors.Is(err, errPeerNotOpen) || len(other()) != 0 {
		t.Fatalf("a pinned call reached another instance: %v", err)
	}
}

// Opening the other app without a click must not give foreground rights to a
// process named by a record peer send does not trust.
func TestPeerLaunchGrantsForegroundOnlyFromATrustedRecord(t *testing.T) {
	dir := t.TempDir()
	if _, ok := peerForegroundPID(dir); ok {
		t.Fatal("a missing record named a process")
	}
	record, _ := json.Marshal(terminalWindowInstance{Version: 2, Host: "127.0.0.1:9", Token: randomHex(32), InstanceID: randomHex(16), PID: os.Getpid()})
	path := filepath.Join(dir, terminalInstanceFilename)
	// A plain record where protection is required (an older Terminal), or one
	// other users can read: still valid for window activation, never for this.
	if err := os.WriteFile(path, append(record, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	previous := workspaceInstanceGuard
	defer func() { workspaceInstanceGuard = previous }()
	workspaceInstanceGuard = workspaceInstanceProtection{required: true, protect: previous.protect, unprotect: previous.unprotect}
	if _, err := readWorkspaceWindowInstance(dir, false); err != nil {
		t.Fatalf("the plain record should still serve window activation: %v", err)
	}
	if pid, ok := peerForegroundPID(dir); ok {
		t.Fatalf("a plain record granted foreground rights to process %d", pid)
	}
	workspaceInstanceGuard = previous
	if pid, ok := peerForegroundPID(dir); ok && runtime.GOOS != "windows" {
		t.Fatalf("a record other users can read granted foreground rights to process %d", pid)
	}
	// A trusted record names its process.
	stored, err := workspaceInstanceGuard.seal(append(record, '\n'))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err = writeAtomic(path, stored); err != nil {
		t.Fatal(err)
	}
	if pid, ok := peerForegroundPID(dir); !ok || pid != os.Getpid() {
		t.Fatalf("a trusted record: %d %v", pid, ok)
	}
}

// Several sends at once with nothing running: exactly one starts the other
// app, the others are refused, and the text arrives once.
func TestPeerSendConcurrentAutoStartsStartOnceAndDeliverOnce(t *testing.T) {
	root := t.TempDir()
	terminal, _ := peerTestApp(t, root, "terminal")
	editor, _ := peerTestApp(t, root, standaloneEditorMode)
	editorPage := peerLivePage(t, editor, peerNoTargetsBody)
	var mu sync.Mutex
	launches := 0
	terminal.peer.mu.Lock()
	terminal.peer.launch = func(context.Context) error {
		mu.Lock()
		launches++
		mu.Unlock()
		go func() {
			time.Sleep(400 * time.Millisecond)
			if _, err := editor.registerWorkspaceWindow(peerWorkspaceDir(root, standaloneEditorMode)); err != nil {
				t.Error(err)
			}
		}()
		return nil
	}
	terminal.peer.mu.Unlock()
	const senders = 8
	type result struct {
		code  int
		state any
		raw   string
	}
	results := make(chan result, senders)
	begin := make(chan struct{})
	for i := 0; i < senders; i++ {
		go func() {
			<-begin
			req, _ := http.NewRequest(http.MethodPost, "http://"+terminal.host+"/api/peer-send/send", strings.NewReader(`{"target":"new","text":"same text"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Session-Token", terminal.token)
			response, err := http.DefaultClient.Do(req)
			if err != nil {
				results <- result{raw: err.Error()}
				return
			}
			defer response.Body.Close()
			raw, _ := io.ReadAll(response.Body)
			var decoded map[string]any
			_ = json.Unmarshal(raw, &decoded)
			results <- result{code: response.StatusCode, state: decoded["state"], raw: string(raw)}
		}()
	}
	close(begin)
	started, refused := 0, 0
	for i := 0; i < senders; i++ {
		switch r := <-results; {
		case r.code == http.StatusOK && r.state == "starting":
			started++
		case r.code == http.StatusConflict && strings.Contains(r.raw, "still waiting for Network Toolkit Text Editor to open"):
			refused++
		default:
			t.Fatalf("unexpected reply: %d %s", r.code, r.raw)
		}
	}
	if started != 1 || refused != senders-1 {
		t.Fatalf("started %d, refused %d", started, refused)
	}
	if request := peerAwaitRequest(t, editorPage, "started Text Editor"); request["text"] != "same text" {
		t.Fatalf("request: %v", request)
	}
	select {
	case again := <-editorPage:
		t.Fatalf("the text was delivered twice: %v", again)
	case <-time.After(700 * time.Millisecond):
	}
	mu.Lock()
	defer mu.Unlock()
	if launches != 1 {
		t.Fatalf("launches: %d", launches)
	}
}
