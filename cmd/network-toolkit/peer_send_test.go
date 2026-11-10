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
	"testing"
	"time"
)

const peerTestRequestID = "0123456789abcdef0123456789abcdef"

// peerTestApp is one standalone app (Terminal or Text Editor) under a shared
// data root, served by a real loopback listener and registered with its
// instance record, as main() does.
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
	if mode == standaloneEditorMode {
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

func peerReport(t *testing.T, a *application, targets string) *peerPayload {
	t.Helper()
	r := requestApp(a, http.MethodPost, "/api/peer-send/inbox", `{"targets":`+targets+`}`, true)
	if r.Code != http.StatusOK {
		t.Fatalf("page report refused: %d %s", r.Code, r.Body)
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

func TestPeerRoutesExistOnlyInTheStandaloneApps(t *testing.T) {
	main := testApplication(t)
	main.configurePeerSend(t.TempDir())
	if main.peerRoot() != "" {
		t.Fatal("the main window was given peer send")
	}
	for _, path := range []string{"/api/peer/terminal/sessions", "/api/peer/editor/documents"} {
		if r := peerInbound(main, http.MethodGet, path, "", main.windowActivationToken, nil); r.Code != http.StatusNotFound {
			t.Fatalf("main window served %s: %d", path, r.Code)
		}
	}
	if r := requestApp(main, http.MethodGet, "/api/peer-send/targets", "", true); r.Code != http.StatusNotFound {
		t.Fatalf("main window page route: %d", r.Code)
	}
	if r := requestApp(main, http.MethodGet, "/api/peer-send/targets", "", false); r.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated page route: %d", r.Code)
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
	if got := peerReport(t, a, `[{"id":"s1","name":"lab-switch-01","type":"ssh","connected":true,"active":true},{"id":"s2","name":"closed","type":"telnet","connected":false}]`); got != nil {
		t.Fatalf("unexpected request: %+v", got)
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
		if r := requestApp(a, http.MethodPost, "/api/peer-send/inbox", `{"targets":`+bad+`}`, true); r.Code != http.StatusBadRequest {
			t.Fatalf("summary %s: %d %s", name, r.Code, r.Body)
		}
	}
	if r := requestApp(a, http.MethodPost, "/api/peer-send/inbox", `{"targets":[],"token":"x"}`, true); r.Code != http.StatusBadRequest {
		t.Fatalf("unknown field in the page report: %d", r.Code)
	}
	if r := requestApp(a, http.MethodGet, "/api/peer-send/inbox", "", true); r.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET inbox: %d", r.Code)
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
	if code, _, raw := peerPage(t, editor, http.MethodPost, "/api/peer-send/send", `{"session":"s1","text":"show version"}`); code != http.StatusServiceUnavailable || !strings.Contains(raw, "Network Toolkit Terminal is not open") {
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
	if code, _, raw = peerPage(t, terminal, http.MethodPost, "/api/peer-send/inbox", `{"targets":[{"id":"s1","name":"lab-switch-01","type":"ssh","connected":true,"active":true},{"id":"s2","name":"COM3","type":"serial","connected":true}]}`); code != http.StatusOK || !strings.Contains(raw, `"request":null`) {
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
	code, reply, raw = peerPage(t, terminal, http.MethodPost, "/api/peer-send/inbox", `{"targets":[{"id":"s1","name":"lab-switch-01","type":"ssh","connected":true}]}`)
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
	if code, _, raw = peerPage(t, editor, http.MethodPost, "/api/peer-send/inbox", documents); code != http.StatusOK {
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
	code, reply, raw = peerPage(t, editor, http.MethodPost, "/api/peer-send/inbox", documents)
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
	_, reply, raw = peerPage(t, editor, http.MethodPost, "/api/peer-send/inbox", `{"targets":[]}`)
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
	if code, _, raw := peerPage(t, editor, http.MethodPost, "/api/peer-send/send", `{"session":"s1","text":"enable secret synthetic"}`); code != http.StatusServiceUnavailable {
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
