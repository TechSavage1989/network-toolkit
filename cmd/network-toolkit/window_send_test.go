package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// windowSendFixture is the main engine with a detached Terminal window that
// holds one session and a detached Text Editor window that holds one
// document. The main window keeps the other session and its own document.
type windowSendFixture struct {
	app                      *application
	terminalToken, editorTok string
	terminalKey, editorKey   string
	mainSession, viewSession string
}

func newWindowSendFixture(t *testing.T) windowSendFixture {
	t.Helper()
	a := testApplication(t)
	var launched terminalViewLaunch
	a.launchTerminalView = func(_ context.Context, launch terminalViewLaunch) (<-chan error, error) {
		launched = launch
		return nil, nil
	}
	a.launchEditorView = func(context.Context, editorViewLaunch) (<-chan error, error) { return nil, nil }
	mainSession := viewTestConnected(t, a)
	viewSession := viewTestConnected(t, a)
	if r := viewTestRequest(a, a.token, "POST", "/api/terminal-views", map[string]any{"sessionId": viewSession.ID, "snapshot": viewTestSnapshot(a, viewSession)}, nil); r.Code != 201 {
		t.Fatalf("detach the terminal: %d %s", r.Code, r.Body.String())
	}
	if r := viewTestRequest(a, launched.ViewToken, "POST", "/api/terminal-views/current/ready", map[string]any{}, nil); r.Code != 200 {
		t.Fatalf("detached terminal ready: %d %s", r.Code, r.Body.String())
	}
	m := a.editorViews
	req, _, editorID := editorFixtureOffer(t, m)
	if _, err := m.accept(editorID, req.ID); err != nil {
		t.Fatal(err)
	}
	editorToken := m.views[editorID].token
	if r := editorHTTP(a, editorToken, "/api/editor-views/current/pulse", map[string]any{}); r.Code != 200 {
		t.Fatalf("editor window ready: %d %s", r.Code, r.Body.String())
	}
	return windowSendFixture{app: a, terminalToken: launched.ViewToken, editorTok: editorToken, terminalKey: windowSendTerminal + launched.ViewID, editorKey: windowSendEditor + editorID, mainSession: mainSession.ID, viewSession: viewSession.ID}
}

func windowSendCall(t *testing.T, a *application, token, method, path string, body any) (int, map[string]any) {
	t.Helper()
	r := viewTestRequest(a, token, method, path, body, nil)
	var out map[string]any
	if err := json.Unmarshal(r.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s %s: %d %s", method, path, r.Code, r.Body.String())
	}
	return r.Code, out
}

// poll and report as each window's page does.
func (f windowSendFixture) pages(t *testing.T) {
	t.Helper()
	a := f.app
	for _, page := range []struct {
		token string
		body  map[string]any
	}{
		{a.token, map[string]any{"targets": []map[string]any{{"id": f.mainSession, "name": "lab-switch-01", "type": "telnet", "connected": true}, {"id": f.viewSession, "name": "not shown here", "type": "telnet", "connected": true}}, "documents": []map[string]any{{"id": editorFixtureUID2, "name": `C:\Users\operator\notes.txt`, "unsaved": true}}}},
		{f.terminalToken, map[string]any{"targets": []map[string]any{{"id": f.viewSession, "name": "lab-router-02", "type": "telnet", "connected": true}}, "documents": []map[string]any{}}},
		{f.editorTok, map[string]any{"targets": []map[string]any{}, "documents": []map[string]any{{"id": editorFixtureUID, "name": "Untitled.txt", "unsaved": true}}}},
	} {
		if code, out := windowSendCall(t, a, page.token, "GET", "/api/window-send/inbox", nil); code != 200 || out["report"] != true {
			t.Fatalf("first inbox poll: %d %v", code, out)
		}
		if code, out := windowSendCall(t, a, page.token, "POST", "/api/window-send/report", page.body); code != 200 {
			t.Fatalf("report: %d %v", code, out)
		}
	}
}

func windowSendLists(t *testing.T, out map[string]any) map[string][]string {
	t.Helper()
	lists := map[string][]string{}
	windows, _ := out["windows"].([]any)
	for _, raw := range windows {
		w := raw.(map[string]any)
		ids := []string{}
		for _, target := range w["targets"].([]any) {
			ids = append(ids, target.(map[string]any)["id"].(string)+"="+target.(map[string]any)["name"].(string))
		}
		lists[w["window"].(string)+"|"+w["name"].(string)] = ids
	}
	return lists
}

func TestWindowSendListsAndRoutesAcrossTheEnginesWindows(t *testing.T) {
	f := newWindowSendFixture(t)
	a := f.app
	for _, token := range []string{a.token, f.terminalToken} {
		if code, out := windowSendCall(t, a, token, "GET", "/api/status", nil); code != 200 || out["windowSend"] != true {
			t.Fatalf("status does not announce window send: %v", out)
		}
	}
	if r := editorGET(a, f.editorTok, "/api/status"); r.Code != 200 || !strings.Contains(r.Body.String(), `"windowSend":true`) {
		t.Fatalf("editor window status: %s", r.Body.String())
	}
	f.pages(t)

	// The detached editor window lists the sessions of the main window and of
	// the Terminal window; the main window's report of a session it does not
	// own was dropped by the engine.
	code, out := windowSendCall(t, a, f.editorTok, "GET", "/api/window-send/targets?kind=terminal", nil)
	if code != 200 {
		t.Fatal(code, out)
	}
	lists := windowSendLists(t, out)
	want := map[string][]string{"main|Main window": {f.mainSession + "=lab-switch-01"}, f.terminalKey + "|Terminal window": {f.viewSession + "=lab-router-02"}}
	if len(lists) != 2 || strings.Join(lists["main|Main window"], ",") != strings.Join(want["main|Main window"], ",") || strings.Join(lists[f.terminalKey+"|Terminal window"], ",") != strings.Join(want[f.terminalKey+"|Terminal window"], ",") {
		t.Fatalf("terminal targets %v, want %v", lists, want)
	}
	// The detached Terminal window lists the documents of the main window
	// (file name only) and of the editor window; a new document goes to the
	// main window.
	code, out = windowSendCall(t, a, f.terminalToken, "GET", "/api/window-send/targets?kind=editor", nil)
	lists = windowSendLists(t, out)
	if code != 200 || strings.Join(lists["main|Main window"], ",") != editorFixtureUID2+"=notes.txt" || strings.Join(lists[f.editorKey+"|Text Editor window"], ",") != editorFixtureUID+"=Untitled.txt" || out["newDocument"] != "main" {
		t.Fatalf("editor targets %v %v", lists, out)
	}
	// The main window sees the other windows only.
	code, out = windowSendCall(t, a, a.token, "GET", "/api/window-send/targets?kind=terminal", nil)
	if lists = windowSendLists(t, out); code != 200 || len(lists) != 1 || out["others"] != float64(2) {
		t.Fatalf("main window targets %v %v", lists, out)
	}

	// Editor window -> Terminal window: queued for that window only.
	code, out = windowSendCall(t, a, f.editorTok, "POST", "/api/window-send/send", map[string]any{"kind": "terminal", "window": f.terminalKey, "session": f.viewSession, "text": "show version"})
	if code != 200 || out["name"] != "Terminal window" {
		t.Fatalf("send: %d %v", code, out)
	}
	requestID := out["requestId"].(string)
	if code, out = windowSendCall(t, a, a.token, "POST", "/api/window-send/send", map[string]any{"kind": "terminal", "window": f.terminalKey, "session": f.viewSession, "text": "second"}); code != http.StatusConflict || !strings.Contains(out["error"].(string), "still waiting") {
		t.Fatalf("a second request for the same window: %d %v", code, out)
	}
	if _, out = windowSendCall(t, a, a.token, "GET", "/api/window-send/inbox", nil); out["request"] != nil {
		t.Fatal("the request reached the main window")
	}
	_, out = windowSendCall(t, a, f.terminalToken, "GET", "/api/window-send/inbox", nil)
	request, _ := out["request"].(map[string]any)
	if out["kind"] != "terminal" || request["session"] != f.viewSession || request["text"] != "show version" || request["requestId"] != requestID {
		t.Fatalf("the Terminal window's inbox: %v", out)
	}
	// Only the sender reads the result, only the receiver acknowledges.
	if _, out = windowSendCall(t, a, a.token, "GET", "/api/window-send/result?requestId="+requestID, nil); out["state"] != "lost" {
		t.Fatalf("another window read the result: %v", out)
	}
	if _, out = windowSendCall(t, a, a.token, "POST", "/api/window-send/ack", map[string]any{"requestId": requestID, "ok": true, "outcome": "sent"}); out["found"] != false {
		t.Fatalf("another window acknowledged: %v", out)
	}
	if code, _ = windowSendCall(t, a, f.terminalToken, "POST", "/api/window-send/ack", map[string]any{"requestId": requestID, "ok": true, "outcome": "inserted"}); code != http.StatusBadRequest {
		t.Fatal("an editor outcome was accepted for terminal text")
	}
	if _, out = windowSendCall(t, a, f.terminalToken, "POST", "/api/window-send/ack", map[string]any{"requestId": requestID, "ok": true, "outcome": "review"}); out["found"] != true {
		t.Fatalf("ack: %v", out)
	}
	if _, out = windowSendCall(t, a, f.editorTok, "GET", "/api/window-send/result?requestId="+requestID, nil); out["state"] != "done" || out["outcome"] != "review" {
		t.Fatalf("result: %v", out)
	}

	// Terminal window -> a new document: the main window's editor.
	code, out = windowSendCall(t, a, f.terminalToken, "POST", "/api/window-send/send", map[string]any{"kind": "editor", "window": "main", "target": "new", "text": "output", "label": "Last command output from lab-router-02"})
	if code != 200 {
		t.Fatal(code, out)
	}
	_, out = windowSendCall(t, a, a.token, "GET", "/api/window-send/inbox", nil)
	if request, _ = out["request"].(map[string]any); out["kind"] != "editor" || request["target"] != "new" || request["where"] != "start" {
		t.Fatalf("main inbox: %v", out)
	}
	// Main window -> a document in the editor window, at a line.
	code, out = windowSendCall(t, a, a.token, "POST", "/api/window-send/send", map[string]any{"kind": "editor", "window": f.editorKey, "target": editorFixtureUID, "where": "line", "line": 3, "text": "x"})
	if code != 200 {
		t.Fatal(code, out)
	}
	_, out = windowSendCall(t, a, f.editorTok, "GET", "/api/window-send/inbox", nil)
	if request, _ = out["request"].(map[string]any); request["target"] != editorFixtureUID || request["line"] != float64(3) {
		t.Fatalf("editor inbox: %v", out)
	}
}

func TestWindowSendRefusesWhatTheWindowDoesNotHold(t *testing.T) {
	f := newWindowSendFixture(t)
	a := f.app
	f.pages(t)
	for _, tc := range []struct {
		name, token string
		body        map[string]any
		status      int
		says        string
	}{
		{"a session another window owns", f.editorTok, map[string]any{"kind": "terminal", "window": "main", "session": f.viewSession, "text": "x"}, http.StatusConflict, "moved to another window"},
		{"a session the window does not list", f.editorTok, map[string]any{"kind": "terminal", "window": f.terminalKey, "session": f.mainSession, "text": "x"}, http.StatusConflict, "moved to another window"},
		{"its own window", f.terminalToken, map[string]any{"kind": "terminal", "window": f.terminalKey, "session": f.viewSession, "text": "x"}, http.StatusBadRequest, "another window"},
		{"terminal text to an editor window", a.token, map[string]any{"kind": "terminal", "window": f.editorKey, "session": f.mainSession, "text": "x"}, http.StatusBadRequest, "cannot take"},
		{"a document to a Terminal window", a.token, map[string]any{"kind": "editor", "window": f.terminalKey, "target": "new", "text": "x"}, http.StatusBadRequest, "cannot take"},
		{"a document the window does not hold", a.token, map[string]any{"kind": "editor", "window": f.editorKey, "target": editorFixtureUID2, "where": "end", "text": "x"}, http.StatusConflict, "no longer open"},
		{"a document held by another window", f.terminalToken, map[string]any{"kind": "editor", "window": "main", "target": editorFixtureUID, "where": "end", "text": "x"}, http.StatusConflict, "moved to another window"},
		{"an unknown window", a.token, map[string]any{"kind": "terminal", "window": windowSendTerminal + strings.Repeat("0", 32), "session": f.viewSession, "text": "x"}, http.StatusConflict, ""},
		{"no text", f.editorTok, map[string]any{"kind": "terminal", "window": f.terminalKey, "session": f.viewSession, "text": ""}, http.StatusBadRequest, "no text"},
		{"an Enter with credentials", f.editorTok, map[string]any{"kind": "terminal", "window": f.terminalKey, "session": f.viewSession, "text": "x", "password": "secret"}, http.StatusBadRequest, ""},
		{"terminal text over 1 MiB", f.editorTok, map[string]any{"kind": "terminal", "window": f.terminalKey, "session": f.viewSession, "text": strings.Repeat("a", 1<<20+1)}, http.StatusBadRequest, "1 MiB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out := windowSendCall(t, a, tc.token, "POST", "/api/window-send/send", tc.body)
			message, _ := out["error"].(string)
			if code != tc.status || !strings.Contains(message, tc.says) {
				t.Fatalf("%d %v, want %d %q", code, out, tc.status, tc.says)
			}
		})
	}
	// A window that stopped polling is no longer a receiver.
	before := windowSendNow
	t.Cleanup(func() { windowSendNow = before })
	f.app.windowSend.mu.Lock()
	f.app.windowSend.windows[f.terminalKey].polledAt = f.app.windowSend.windows[f.terminalKey].polledAt.Add(-peerTargetsFresh * 2)
	f.app.windowSend.mu.Unlock()
	if code, out := windowSendCall(t, a, f.editorTok, "POST", "/api/window-send/send", map[string]any{"kind": "terminal", "window": f.terminalKey, "session": f.viewSession, "text": "x"}); code != http.StatusConflict {
		t.Fatalf("a window that stopped polling took text: %d %v", code, out)
	}
	code, out := windowSendCall(t, a, f.editorTok, "GET", "/api/window-send/targets?kind=terminal", nil)
	if lists := windowSendLists(t, out); code != 200 || len(lists) != 1 {
		t.Fatalf("a window that stopped polling is still listed: %v", lists)
	}
}

// Window send now runs in the standalone apps' engines too (v1.21.50):
// TestWindowSendInEveryEngineAndPeerSendOnlyForStandaloneWindows.
