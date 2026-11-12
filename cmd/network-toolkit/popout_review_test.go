package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Review of v1.21.50's pop-out changes.

// Owner decision (kept and made explicit): a main Toolkit engine whose main
// window is closed while a detached Terminal window stays open is still a
// receiver for the sessions that window shows. The text goes to that window's
// paste path, and the main window is not reopened for it.
func TestPeerSendReachesADetachedWindowOfAClosedMainWindow(t *testing.T) {
	windowRaiseRecorder(t)
	root := t.TempDir()
	main, _ := peerTestApp(t, root, "main")
	peerRegister(t, main, root)
	editor, _ := peerTestApp(t, root, standaloneEditorMode)
	var launched terminalViewLaunch
	main.launchTerminalView = func(_ context.Context, l terminalViewLaunch) (<-chan error, error) { launched = l; return nil, nil }
	session := viewTestConnected(t, main)
	if r := viewTestRequest(main, main.token, "POST", "/api/terminal-views", map[string]any{"sessionId": session.ID, "snapshot": viewTestSnapshot(main, session)}, nil); r.Code != 201 {
		t.Fatalf("detach: %d %s", r.Code, r.Body.String())
	}
	if r := viewTestRequest(main, launched.ViewToken, "POST", "/api/terminal-views/current/ready", map[string]any{}, nil); r.Code != 200 {
		t.Fatalf("ready: %d", r.Code)
	}
	token, key := launched.ViewToken, windowSendTerminal+launched.ViewID
	windowSendCall(t, main, token, "GET", "/api/window-send/inbox", nil)
	windowSendCall(t, main, token, "POST", "/api/window-send/report", map[string]any{"targets": []map[string]any{{"id": session.ID, "name": "lab-router-02", "type": "telnet", "connected": true}}, "documents": []any{}})
	// The main window closes; the engine runs on for the Terminal window.
	main.views.mu.Lock()
	main.views.homePresent = false
	main.views.mu.Unlock()
	main.editorViews.setHomePresent(false)
	main.windowSendHomeClosed()

	code, listed, raw := peerPage(t, editor, http.MethodGet, "/api/peer-send/targets", "")
	if code != 200 || listed["receiver"] != "main" || listed["ready"] != true || !strings.Contains(raw, session.ID) {
		t.Fatalf("the main engine's detached session is not offered: %d %s", code, raw)
	}
	code, sent, raw := peerPage(t, editor, http.MethodPost, "/api/peer-send/send", `{"session":"`+session.ID+`","text":"show ip route"}`)
	if code != 200 {
		t.Fatalf("send: %d %s", code, raw)
	}
	_, inbox := windowSendCall(t, main, token, "GET", "/api/window-send/inbox", nil)
	request, _ := inbox["request"].(map[string]any)
	if inbox["kind"] != "terminal" || request["session"] != session.ID || request["text"] != "show ip route" {
		t.Fatalf("the Terminal window's request: %v", inbox)
	}
	if raise, _ := windowPending(main, key); !raise {
		t.Fatal("the Terminal window that took the text was not raised")
	}
	select {
	case <-main.primaryWake:
		t.Fatal("text for a detached window reopened the closed main window")
	default:
	}
	windowSendCall(t, main, token, "POST", "/api/window-send/ack", map[string]any{"requestId": sent["requestId"], "ok": true, "outcome": "sent"})
	peerAwaitResult(t, editor, sent["requestId"].(string), peerDone)
}

// CI finding (v1.21.50): right after a session pops out, the app's own page
// still reports it until its next report. Text for it must never be queued
// for that page (which refuses it); the engine's ownership decides, and the
// session is offered again once the detached window reports it.
func TestPeerSendNeverQueuesAPoppedOutSessionForTheAppsOwnWindow(t *testing.T) {
	windowRaiseRecorder(t)
	root := t.TempDir()
	term, _ := peerTestApp(t, root, "terminal")
	peerRegister(t, term, root)
	editor, _ := peerTestApp(t, root, standaloneEditorMode)
	var launched terminalViewLaunch
	term.launchTerminalView = func(_ context.Context, l terminalViewLaunch) (<-chan error, error) { launched = l; return nil, nil }
	session := viewTestConnected(t, term)
	report := `{"targets":[{"id":"` + session.ID + `","name":"lab-router-02","type":"telnet","connected":true}]}`
	peerPage(t, term, http.MethodPost, "/api/peer-send/report", report)
	peerPage(t, term, http.MethodGet, "/api/peer-send/inbox", "")
	if r := viewTestRequest(term, term.token, "POST", "/api/terminal-views", map[string]any{"sessionId": session.ID, "snapshot": viewTestSnapshot(term, session)}, nil); r.Code != 201 {
		t.Fatalf("detach: %d %s", r.Code, r.Body.String())
	}
	if r := viewTestRequest(term, launched.ViewToken, "POST", "/api/terminal-views/current/ready", map[string]any{}, nil); r.Code != 200 {
		t.Fatalf("ready: %d", r.Code)
	}
	// The app's own page has not reported since: it still lists the session.
	peerPage(t, term, http.MethodGet, "/api/peer-send/inbox", "")
	if _, _, raw := peerPage(t, editor, http.MethodGet, "/api/peer-send/targets", ""); strings.Contains(raw, session.ID) {
		t.Fatalf("a popped-out session was offered as the app's own window's: %s", raw)
	}
	// A sender whose list is older still names it: refused, never queued there.
	editor.peer.mu.Lock()
	editor.peer.listed[windowSendMain] = peerListed{mode: "terminal", instanceID: term.windowInstanceID}
	editor.peer.mu.Unlock()
	if code, _, raw := peerPage(t, editor, http.MethodPost, "/api/peer-send/send", `{"session":"`+session.ID+`","text":"show inventory"}`); code == 200 || !strings.Contains(raw, "moved to another window") {
		t.Fatalf("text for a popped-out session: %d %s", code, raw)
	}
	if _, inbox, _ := peerPage(t, term, http.MethodGet, "/api/peer-send/inbox", ""); inbox["request"] != nil {
		t.Fatalf("the app's own window got text for a session it no longer holds: %v", inbox)
	}
	// Once the Terminal window reports it, it is offered and the text goes there.
	windowSendCall(t, term, launched.ViewToken, "GET", "/api/window-send/inbox", nil)
	windowSendCall(t, term, launched.ViewToken, "POST", "/api/window-send/report", map[string]any{"targets": []map[string]any{{"id": session.ID, "name": "lab-router-02", "type": "telnet", "connected": true}}, "documents": []any{}})
	if _, _, raw := peerPage(t, editor, http.MethodGet, "/api/peer-send/targets", ""); !strings.Contains(raw, session.ID) {
		t.Fatalf("the Terminal window's session is not offered: %s", raw)
	}
	if code, _, raw := peerPage(t, editor, http.MethodPost, "/api/peer-send/send", `{"session":"`+session.ID+`","text":"show inventory"}`); code != 200 {
		t.Fatalf("send: %d %s", code, raw)
	}
	if _, inbox := windowSendCall(t, term, launched.ViewToken, "GET", "/api/window-send/inbox", nil); inbox["request"] == nil {
		t.Fatal("the Terminal window did not get the text")
	}
}

// A detached window that sends to another app allows that app's process
// before the text is posted, and the receiving app raises its window twice.
func TestADetachedSenderAllowsTheReceiverBeforeTheTextIsPosted(t *testing.T) {
	windowRaiseRecorder(t)
	var later atomic.Int32
	windowRaiseLater = func(f func()) { later.Add(1); f() }
	root := t.TempDir()
	var postedMu sync.Mutex
	var postedAt time.Time
	term, _ := peerTestAppWith(t, root, "terminal", func(a *application) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/peer/terminal/send" {
				postedMu.Lock()
				postedAt = time.Now()
				postedMu.Unlock()
			}
			a.ServeHTTP(w, r)
		})
	})
	peerRegister(t, term, root)
	editor, _ := peerTestApp(t, root, standaloneEditorMode)
	editor.launchEditorView = func(context.Context, editorViewLaunch) (<-chan error, error) { return nil, nil }
	req, _, editorID := editorFixtureOffer(t, editor.editorViews)
	if _, err := editor.editorViews.accept(editorID, req.ID); err != nil {
		t.Fatal(err)
	}
	childToken, childKey := editor.editorViews.views[editorID].token, windowSendEditor+editorID
	editorHTTP(editor, childToken, "/api/editor-views/current/pulse", map[string]any{})
	peerPage(t, term, http.MethodPost, "/api/peer-send/report", `{"targets":[{"id":"s1","name":"lab-switch-01","type":"ssh","connected":true}]}`)
	peerPage(t, term, http.MethodGet, "/api/peer-send/inbox", "")

	// The child window's native host waits for its activation, as it does.
	type collected struct {
		at    time.Time
		allow float64
	}
	got := make(chan collected, 1)
	go func() {
		r := viewTestRequest(editor, childToken, "GET", "/api/window-send/activation", nil, nil)
		var out map[string]any
		_ = json.Unmarshal(r.Body.Bytes(), &out)
		allow, _ := out["allow"].(float64)
		got <- collected{time.Now(), allow}
	}()
	viewTestWait(t, func() bool {
		editor.windowSend.mu.Lock()
		defer editor.windowSend.mu.Unlock()
		act := editor.windowSend.activations[childKey]
		return act != nil && act.polling > 0
	})
	windowSendCall(t, editor, childToken, "GET", "/api/peer-send/targets", nil)
	if code, out := windowSendCall(t, editor, childToken, "POST", "/api/peer-send/send", map[string]any{"session": "s1", "text": "show version"}); code != 200 {
		t.Fatalf("send: %d %v", code, out)
	}
	var c collected
	select {
	case c = <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("the sending window was never asked to allow the receiver")
	}
	postedMu.Lock()
	posted := postedAt
	postedMu.Unlock()
	if int(c.allow) != os.Getpid() || posted.IsZero() || !c.at.Before(posted) {
		t.Fatalf("the permission (pid %v at %v) did not come before the text was posted (%v)", c.allow, c.at, posted)
	}
	if !primaryRaised(term) || later.Load() < 1 {
		t.Fatal("the receiving app did not raise its window, and again shortly after")
	}
}

// A request waiting for a main window that closes ends at once and says so;
// a remote file's grant ends with it.
func TestRequestsForAClosedMainWindowEndAtOnce(t *testing.T) {
	windowRaiseRecorder(t)
	f := newTerminalSFTP123Fixture(t, true)
	a := f.app
	session, view := f.session.ID, f.launch.ViewToken
	for _, token := range []string{a.token, view} {
		windowSendCall(t, a, token, "GET", "/api/window-send/inbox", nil)
		windowSendCall(t, a, token, "POST", "/api/window-send/report", map[string]any{"targets": []any{}, "documents": []any{}})
	}
	code, out := windowSendCall(t, a, view, "POST", "/api/window-send/send", map[string]any{"kind": "remote", "window": "main", "session": session, "path": "startup.cfg"})
	if code != 200 {
		t.Fatalf("Edit: %d %v", code, out)
	}
	read := func() int {
		return viewTestRequest(a, a.token, "POST", "/api/remote-browser", map[string]any{"protocol": "ssh", "sessionId": session, "action": "read-text", "path": "startup.cfg"}, nil).Code
	}
	if read() != 200 {
		t.Fatal("the grant was not made")
	}
	a.windowSendHomeClosed()
	if _, result := windowSendCall(t, a, view, "GET", "/api/window-send/result?requestId="+out["requestId"].(string), nil); result["state"] != peerFailed || !strings.Contains(result["message"].(string), "closed before it opened the file") {
		t.Fatalf("the workspace was not told at once: %v", result)
	}
	if read() != http.StatusConflict {
		t.Fatal("the grant outlived the request")
	}
	if _, inbox := windowSendCall(t, a, a.token, "GET", "/api/window-send/inbox", nil); inbox["request"] != nil {
		t.Fatalf("a reopened main window got the ended request: %v", inbox)
	}

	// Text waiting for the main window ends the same way.
	g := newWindowSendFixture(t)
	g.pages(t)
	_, sent := windowSendCall(t, g.app, g.terminalToken, "POST", "/api/window-send/send", map[string]any{"kind": "editor", "window": "main", "target": "new", "text": "x"})
	g.app.windowSendHomeClosed()
	if _, result := windowSendCall(t, g.app, g.terminalToken, "GET", "/api/window-send/result?requestId="+sent["requestId"].(string), nil); result["state"] != peerFailed || !strings.Contains(result["message"].(string), "Nothing was sent") {
		t.Fatalf("text for a closed main window: %v", result)
	}
}

// What the engine kept for a detached window is dropped when it closes.
func TestClosedDetachedWindowsAreForgotten(t *testing.T) {
	a := testApplication(t)
	key := windowSendEditor + "v9"
	a.peer.listed = map[string]peerListed{key: {mode: "terminal", instanceID: "x"}, windowSendMain: {mode: "terminal", instanceID: "y"}}
	done := make(chan error, 1)
	out, _ := a.trackWindowProcess(key, func(started func(int)) (<-chan error, error) { started(8101); return done, nil })
	a.windowSend.mu.Lock()
	a.windowSend.signalLocked(key, true, 4, time.Now())
	a.windowSend.mu.Unlock()
	close(done)
	<-out
	a.windowSend.mu.Lock()
	_, act := a.windowSend.activations[key]
	a.windowSend.mu.Unlock()
	a.peer.mu.Lock()
	_, listed := a.peer.listed[key]
	_, own := a.peer.listed[windowSendMain]
	a.peer.mu.Unlock()
	if act || listed || !own {
		t.Fatalf("after the window closed: activation %v, its list %v, the app's own list %v", act, listed, own)
	}
}
