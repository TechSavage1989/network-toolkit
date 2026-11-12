package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// Owner decisions of v1.21.50, resolving four limits of popped-out windows:
// the receiving window comes to the front, undo history moves with handed-over
// tabs, detached windows of a standalone app send and receive peer text through
// their owner engine, and Edit in a detached workspace reopens a closed main
// window.

// windowRaiseRecorder replaces the foreground permission with a recorder and
// runs the second raise at once.
func windowRaiseRecorder(t *testing.T) func() []int {
	t.Helper()
	var mu sync.Mutex
	var calls []int
	allow, later := windowAllowForeground, windowRaiseLater
	windowAllowForeground = func(pid int) {
		mu.Lock()
		calls = append(calls, pid)
		mu.Unlock()
	}
	windowRaiseLater = func(f func()) { f() }
	t.Cleanup(func() { windowAllowForeground, windowRaiseLater = allow, later })
	return func() []int {
		mu.Lock()
		defer mu.Unlock()
		return append([]int(nil), calls...)
	}
}

// windowPending is what a child window has still to collect.
func windowPending(a *application, key string) (raise bool, allow int) {
	a.windowSend.mu.Lock()
	defer a.windowSend.mu.Unlock()
	if act := a.windowSend.activations[key]; act != nil {
		return act.raise, act.allow
	}
	return false, 0
}

func primaryRaised(a *application) bool {
	select {
	case <-a.windowActivate:
		return true
	default:
		return false
	}
}

func TestWindowSendRaisesOnlyTheWindowThatTookTheText(t *testing.T) {
	allowed := windowRaiseRecorder(t)
	f := newWindowSendFixture(t)
	a := f.app
	a.windowSend.mu.Lock()
	a.windowSend.pids = map[string]int{f.terminalKey: 4101, f.editorKey: 4202}
	a.windowSend.mu.Unlock()
	f.pages(t)

	// A refusal raises nothing and allows nothing.
	for _, body := range []map[string]any{
		{"kind": "terminal", "window": f.terminalKey, "session": f.mainSession, "text": "x"},
		{"kind": "editor", "window": f.terminalKey, "target": "new", "text": "x"},
		{"kind": "terminal", "window": windowSendTerminal + strings.Repeat("0", 32), "session": f.viewSession, "text": "x"},
	} {
		if code, out := windowSendCall(t, a, f.editorTok, "POST", "/api/window-send/send", body); code == http.StatusOK {
			t.Fatalf("a refused send was queued: %v", out)
		}
	}
	if calls := allowed(); len(calls) != 0 || primaryRaised(a) {
		t.Fatalf("a refusal raised a window: %v", calls)
	}
	for _, key := range []string{f.terminalKey, f.editorKey} {
		if raise, allow := windowPending(a, key); raise || allow != 0 {
			t.Fatalf("a refusal left an activation for %s: %v %d", key, raise, allow)
		}
	}

	// The main window sends to the Terminal window: the engine allows that
	// window's process only, and the window is told to come forward.
	if code, out := windowSendCall(t, a, a.token, "POST", "/api/window-send/send", map[string]any{"kind": "terminal", "window": f.terminalKey, "session": f.viewSession, "text": "show clock"}); code != 200 {
		t.Fatalf("send: %d %v", code, out)
	}
	if calls := allowed(); len(calls) != 1 || calls[0] != 4101 {
		t.Fatalf("foreground allowed for %v, want only the Terminal window's process", calls)
	}
	if raise, allow := windowPending(a, f.terminalKey); !raise || allow != 0 {
		t.Fatalf("the Terminal window was not told to come forward: %v %d", raise, allow)
	}
	if raise, _ := windowPending(a, f.editorKey); raise || primaryRaised(a) {
		t.Fatal("another window was raised")
	}
	// The Terminal window's own process collects it with its view token.
	if code, out := windowSendCall(t, a, f.terminalToken, "GET", "/api/window-send/activation", nil); code != 200 || out["raise"] != true || out["allow"] != float64(0) {
		t.Fatalf("activation: %d %v", code, out)
	}
	if raise, _ := windowPending(a, f.terminalKey); raise {
		t.Fatal("an activation was collected twice")
	}
	// The window takes the text and acknowledges; the next send can go.
	_, inbox := windowSendCall(t, a, f.terminalToken, "GET", "/api/window-send/inbox", nil)
	id := inbox["request"].(map[string]any)["requestId"].(string)
	windowSendCall(t, a, f.terminalToken, "POST", "/api/window-send/ack", map[string]any{"requestId": id, "ok": true, "outcome": "sent"})

	// The editor window sends to the Terminal window: the sending window (the
	// foreground process) is told to allow the Terminal window's process.
	if code, out := windowSendCall(t, a, f.editorTok, "POST", "/api/window-send/send", map[string]any{"kind": "terminal", "window": f.terminalKey, "session": f.viewSession, "text": "show version"}); code != 200 {
		t.Fatalf("send: %d %v", code, out)
	}
	if code, out := windowSendCall(t, a, f.editorTok, "GET", "/api/window-send/activation", nil); code != 200 || out["raise"] != false || out["allow"] != float64(4101) {
		t.Fatalf("the sending window's permission: %d %v", code, out)
	}
	if calls := allowed(); len(calls) != 2 || calls[1] != 4101 {
		t.Fatalf("foreground allowed for %v", calls)
	}

	// The Terminal window sends to the main window: the main window is raised
	// (twice: after the sender's permission too), the sender allows the
	// engine's own process, and the engine allows nobody else.
	if code, out := windowSendCall(t, a, f.terminalToken, "POST", "/api/window-send/send", map[string]any{"kind": "editor", "window": "main", "target": "new", "text": "from the Terminal window"}); code != 200 {
		t.Fatalf("send: %d %v", code, out)
	}
	if !primaryRaised(a) {
		t.Fatal("the main window was not raised")
	}
	if _, allow := windowPending(a, f.terminalKey); allow != os.Getpid() {
		t.Fatalf("the sending Terminal window allows %d, want the engine %d", allow, os.Getpid())
	}
	if calls := allowed(); len(calls) != 2 {
		t.Fatalf("the engine allowed another process for its own window: %v", calls)
	}

	// The main window has no activation route; a stale raise is dropped.
	if r := viewTestRequest(a, a.token, "GET", "/api/window-send/activation", nil, nil); r.Code != http.StatusNotFound {
		t.Fatalf("the main window's activation route: %d", r.Code)
	}
	a.windowSend.mu.Lock()
	a.windowSend.signalLocked(f.editorKey, true, 0, time.Now().Add(-2*windowRaiseFresh))
	a.windowSend.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if raise, _ := a.windowSendActivationWait(ctx, f.editorKey); raise {
		t.Fatal("a raise nobody collected in time was used later")
	}
	if r := viewTestRequest(a, f.terminalToken, "POST", "/api/window-send/activation", map[string]any{}, nil); r.Code != http.StatusMethodNotAllowed {
		t.Fatalf("activation by POST: %d", r.Code)
	}
}

// The engine knows a detached window's process only while it runs.
func TestWindowProcessIsForgottenWhenItEnds(t *testing.T) {
	a := testApplication(t)
	done := make(chan error, 1)
	out, err := a.trackWindowProcess(windowSendTerminal+"v1", func(started func(int)) (<-chan error, error) {
		started(7310)
		return done, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	a.windowSend.mu.Lock()
	pid := a.windowSend.pids[windowSendTerminal+"v1"]
	a.windowSend.mu.Unlock()
	if pid != 7310 {
		t.Fatalf("recorded %d", pid)
	}
	done <- errors.New("exited")
	close(done)
	if got := <-out; got == nil || got.Error() != "exited" {
		t.Fatalf("the launch's own result was not passed on: %v", got)
	}
	a.windowSend.mu.Lock()
	_, kept := a.windowSend.pids[windowSendTerminal+"v1"]
	a.windowSend.mu.Unlock()
	if kept {
		t.Fatal("a process that ended is still named")
	}
	if _, err := a.trackWindowProcess(windowSendEditor+"v2", func(started func(int)) (<-chan error, error) {
		started(7311)
		return nil, errors.New("not started")
	}); err == nil {
		t.Fatal("a failed launch succeeded")
	}
	a.windowSend.mu.Lock()
	_, kept = a.windowSend.pids[windowSendEditor+"v2"]
	a.windowSend.mu.Unlock()
	if kept {
		t.Fatal("a failed launch left its process named")
	}
}

// Item 4: Edit in a detached workspace's SFTP browser while the main window is
// closed reopens it and opens the file there, with every grant rule.
func TestWindowSendRemoteEditReopensTheMainWindow(t *testing.T) {
	windowRaiseRecorder(t)
	f := newTerminalSFTP123Fixture(t, true)
	a := f.app
	session, view := f.session.ID, f.launch.ViewToken
	windowSendCall(t, a, view, "GET", "/api/window-send/inbox", nil)
	windowSendCall(t, a, view, "POST", "/api/window-send/report", map[string]any{"targets": []any{}, "documents": []any{}})
	closeMain := func() {
		a.views.mu.Lock()
		a.views.homePresent = false
		a.views.mu.Unlock()
		a.editorViews.setHomePresent(false)
		a.windowSendHomeClosed()
		for {
			select {
			case <-a.primaryWake:
			case <-a.windowActivate:
			default:
				return
			}
		}
	}
	// The main page ran a moment ago; once its window closed that no longer counts.
	windowSendCall(t, a, a.token, "GET", "/api/window-send/inbox", nil)
	windowSendCall(t, a, a.token, "POST", "/api/window-send/report", map[string]any{"targets": []any{}, "documents": []any{}})
	closeMain()
	edit := func() (int, map[string]any) {
		r := viewTestRequest(a, view, "POST", "/api/window-send/send", map[string]any{"kind": "remote", "window": "main", "session": session, "path": "startup.cfg"}, nil)
		var out map[string]any
		_ = json.Unmarshal(r.Body.Bytes(), &out)
		return r.Code, out
	}
	mainRead := func() int {
		return viewTestRequest(a, a.token, "POST", "/api/remote-browser", map[string]any{"protocol": "ssh", "sessionId": session, "action": "read-text", "path": "startup.cfg"}, nil).Code
	}

	// A refused request never reopens the main window.
	if r := viewTestRequest(a, view, "POST", "/api/window-send/send", map[string]any{"kind": "remote", "window": "main", "session": "other-session", "path": "startup.cfg"}, nil); r.Code != http.StatusConflict {
		t.Fatalf("a session the workspace does not own: %d", r.Code)
	}
	select {
	case <-a.primaryWake:
		t.Fatal("a refused Edit reopened the main window")
	default:
	}

	// The main window does not come back in time: nothing is opened or granted.
	before := windowSendReopenWait
	windowSendReopenWait = 300 * time.Millisecond
	code, out := edit()
	windowSendReopenWait = before
	if code != http.StatusConflict || !strings.Contains(out["error"].(string), "did not open in time") {
		t.Fatalf("a main window that never opened: %d %v", code, out)
	}
	select {
	case <-a.primaryWake:
	default:
		t.Fatal("Edit did not ask for the main window")
	}
	if mainRead() != http.StatusConflict {
		t.Fatal("a grant was left for a file that never opened")
	}

	// The main window reopens (runPrimaryLifecycle) and its page starts: the
	// request waits for it and is then queued with its grant.
	closeMain()
	type reply struct {
		code int
		out  map[string]any
	}
	answered := make(chan reply, 1)
	go func() { c, o := edit(); answered <- reply{c, o} }()
	select {
	case <-a.primaryWake:
	case <-time.After(5 * time.Second):
		t.Fatal("Edit did not reopen the closed main window")
	}
	select {
	case r := <-answered:
		t.Fatalf("Edit answered before the main window opened: %v", r)
	case <-time.After(300 * time.Millisecond):
	}
	a.views.mu.Lock()
	a.views.homePresent = true
	a.views.mu.Unlock()
	a.editorViews.setHomePresent(true)
	windowSendCall(t, a, a.token, "GET", "/api/window-send/inbox", nil)
	windowSendCall(t, a, a.token, "POST", "/api/window-send/report", map[string]any{"targets": []any{}, "documents": []any{}})
	var r reply
	select {
	case r = <-answered:
	case <-time.After(5 * time.Second):
		t.Fatal("Edit never finished after the main window opened")
	}
	if r.code != 200 {
		t.Fatalf("Edit after reopening: %d %v", r.code, r.out)
	}
	_, inbox := windowSendCall(t, a, a.token, "GET", "/api/window-send/inbox", nil)
	request, _ := inbox["request"].(map[string]any)
	if inbox["kind"] != "remote" || request["path"] != "startup.cfg" || request["session"] != session {
		t.Fatalf("the reopened main window's request: %v", inbox)
	}
	if mainRead() != 200 {
		t.Fatal("the granted file cannot be read in the reopened main window")
	}
	// Exactly that path, as before.
	if code := viewTestRequest(a, a.token, "POST", "/api/remote-browser", map[string]any{"protocol": "ssh", "sessionId": session, "action": "read-text", "path": "other.cfg"}, nil).Code; code != http.StatusConflict {
		t.Fatalf("another file through the grant: %d", code)
	}
	// Only the main Toolkit engine opens remote files: a standalone Terminal's
	// workspace is refused before anything is opened.
	a.windowMode = "terminal"
	if code, out := edit(); code != http.StatusBadRequest {
		t.Fatalf("a standalone Terminal's workspace asked for Edit: %d %v", code, out)
	}
	a.windowMode = "main"
}

// Item 3: a detached window of a standalone app sends through its owner engine
// with every peer-send rule, and text for a session or a document of a
// detached window of the receiving app goes to that window.
func TestPeerSendFromAndToDetachedWindowsOfStandaloneApps(t *testing.T) {
	allowed := windowRaiseRecorder(t)
	root := t.TempDir()
	editor, _ := peerTestApp(t, root, standaloneEditorMode)
	peerRegister(t, editor, root)
	term, _ := peerTestApp(t, root, "terminal")
	peerRegister(t, term, root)

	// A Text Editor window popped out of the standalone Text Editor.
	editor.launchEditorView = func(context.Context, editorViewLaunch) (<-chan error, error) { return nil, nil }
	req, _, editorID := editorFixtureOffer(t, editor.editorViews)
	if _, err := editor.editorViews.accept(editorID, req.ID); err != nil {
		t.Fatal(err)
	}
	editorToken := editor.editorViews.views[editorID].token
	editorKey := windowSendEditor + editorID
	if r := editorHTTP(editor, editorToken, "/api/editor-views/current/pulse", map[string]any{}); r.Code != 200 {
		t.Fatalf("editor window ready: %d", r.Code)
	}
	// A Terminal window popped out of the standalone Terminal, holding a session.
	var launched terminalViewLaunch
	term.launchTerminalView = func(_ context.Context, l terminalViewLaunch) (<-chan error, error) { launched = l; return nil, nil }
	session := viewTestConnected(t, term)
	if r := viewTestRequest(term, term.token, "POST", "/api/terminal-views", map[string]any{"sessionId": session.ID, "snapshot": viewTestSnapshot(term, session)}, nil); r.Code != 201 {
		t.Fatalf("detach: %d %s", r.Code, r.Body.String())
	}
	if r := viewTestRequest(term, launched.ViewToken, "POST", "/api/terminal-views/current/ready", map[string]any{}, nil); r.Code != 200 {
		t.Fatalf("terminal window ready: %d", r.Code)
	}
	termToken, termKey := launched.ViewToken, windowSendTerminal+launched.ViewID
	term.windowSend.mu.Lock()
	term.windowSend.pids = map[string]int{termKey: 6201}
	term.windowSend.mu.Unlock()

	// Each detached window's page runs window send (to receive), and its
	// status says it sends as its app.
	for _, page := range []struct {
		a     *application
		token string
		body  map[string]any
		kind  string
	}{
		{editor, editorToken, map[string]any{"targets": []any{}, "documents": []map[string]any{{"id": editorFixtureUID, "name": "Untitled.txt", "unsaved": true}}}, "editor"},
		{term, termToken, map[string]any{"targets": []map[string]any{{"id": session.ID, "name": "lab-router-02", "type": "telnet", "connected": true}}, "documents": []any{}}, "terminal"},
	} {
		if code, out := windowSendCall(t, page.a, page.token, "GET", "/api/window-send/inbox", nil); code != 200 || out["report"] != true {
			t.Fatalf("a standalone app's detached window cannot receive: %d %v", code, out)
		}
		windowSendCall(t, page.a, page.token, "POST", "/api/window-send/report", page.body)
		if _, status := windowSendCall(t, page.a, page.token, "GET", "/api/status", nil); status["peerSend"] != page.kind || status["windowSend"] != true {
			t.Fatalf("status: %v", status)
		}
	}
	// The standalone Terminal's own page shows another session.
	peerPage(t, term, http.MethodPost, "/api/peer-send/report", `{"targets":[{"id":"s1","name":"lab-switch-01","type":"ssh","connected":true}]}`)
	peerPage(t, term, http.MethodGet, "/api/peer-send/inbox", "")

	// Only the sending routes, only through the owner.
	for _, route := range []struct{ method, path string }{{"GET", "/api/peer-send/inbox"}, {"POST", "/api/peer-send/report"}, {"POST", "/api/peer-send/ack"}, {"GET", "/api/peer/terminal/sessions"}} {
		if r := viewTestRequest(editor, editorToken, route.method, route.path, map[string]any{}, nil); r.Code == http.StatusOK {
			t.Fatalf("a detached window reached %s %s", route.method, route.path)
		}
	}

	// The Text Editor window lists the Terminal app's sessions: its own
	// window's and its detached window's.
	code, listed := windowSendCall(t, editor, editorToken, "GET", "/api/peer-send/targets", nil)
	ids := []string{}
	for _, target := range listed["targets"].([]any) {
		ids = append(ids, target.(map[string]any)["id"].(string))
	}
	if code != 200 || listed["receiver"] != "standalone" || strings.Join(ids, ",") != "s1,"+session.ID {
		t.Fatalf("targets: %d %v", code, listed)
	}
	// What one window was shown is its own: the app's own window never listed.
	if code, _, raw := peerPage(t, editor, http.MethodPost, "/api/peer-send/send", `{"session":"`+session.ID+`","text":"x"}`); code != http.StatusConflict || !strings.Contains(raw, "out of date") {
		t.Fatalf("the app's own window used the detached window's list: %d %s", code, raw)
	}
	code, sent := windowSendCall(t, editor, editorToken, "POST", "/api/peer-send/send", map[string]any{"session": session.ID, "text": "show version"})
	if code != 200 || sent["state"] != peerQueued {
		t.Fatalf("send from the detached window: %d %v", code, sent)
	}
	id := sent["requestId"].(string)
	// The Terminal app's detached window holds it; its own window does not.
	if _, inbox, _ := peerPage(t, term, http.MethodGet, "/api/peer-send/inbox", ""); inbox["request"] != nil {
		t.Fatalf("the Terminal app's own window got text for its detached window: %v", inbox)
	}
	_, inbox := windowSendCall(t, term, termToken, "GET", "/api/window-send/inbox", nil)
	request, _ := inbox["request"].(map[string]any)
	if inbox["kind"] != "terminal" || request["session"] != session.ID || request["text"] != "show version" || request["requestId"] != id {
		t.Fatalf("the detached Terminal window's request: %v", inbox)
	}
	// That window comes forward, not the app's own window; the sending window
	// allows the receiving engine's process (its protected record names it).
	if raise, _ := windowPending(term, termKey); !raise || primaryRaised(term) {
		t.Fatal("the window holding the session was not raised (or the app's own window was)")
	}
	if calls := allowed(); len(calls) != 1 || calls[0] != 6201 {
		t.Fatalf("foreground allowed for %v", calls)
	}
	if _, allow := windowPending(editor, editorKey); allow != os.Getpid() {
		t.Fatalf("the sending window allows %d, want the receiving app's process", allow)
	}
	// One request at a time across the receiving app.
	if code, _, raw := peerPage(t, editor, http.MethodGet, "/api/peer-send/targets", ""); code != 200 {
		t.Fatal(raw)
	}
	if code, _, raw := peerPage(t, editor, http.MethodPost, "/api/peer-send/send", `{"session":"s1","text":"x"}`); code != http.StatusConflict || !strings.Contains(raw, "still waiting") {
		t.Fatalf("a second send while one waits: %d %s", code, raw)
	}
	// Only the sending window reads the result.
	if _, out, _ := peerPage(t, editor, http.MethodGet, "/api/peer-send/result?requestId="+id, ""); out["state"] != "lost" {
		t.Fatalf("another window read the result: %v", out)
	}
	windowSendCall(t, term, termToken, "POST", "/api/window-send/ack", map[string]any{"requestId": id, "ok": true, "outcome": "review"})
	if _, out := windowSendCall(t, editor, editorToken, "GET", "/api/peer-send/result?requestId="+id, nil); out["state"] != peerDone || out["outcome"] != "review" {
		t.Fatalf("result: %v", out)
	}

	// The other way: the Terminal window sends to a new document. The Text
	// Editor app's own page never ran, so its detached window takes it.
	if code, out := windowSendCall(t, term, termToken, "GET", "/api/peer-send/targets", nil); code != 200 || out["ready"] != true || out["receiver"] != "standalone" {
		t.Fatalf("the Text Editor app's targets: %d %v", code, out)
	}
	code, sent = windowSendCall(t, term, termToken, "POST", "/api/peer-send/send", map[string]any{"target": "new", "text": "output from a window"})
	if code != 200 {
		t.Fatalf("send to a new document: %d %v", code, sent)
	}
	_, inbox = windowSendCall(t, editor, editorToken, "GET", "/api/window-send/inbox", nil)
	if request, _ := inbox["request"].(map[string]any); inbox["kind"] != "editor" || request["target"] != "new" || request["text"] != "output from a window" {
		t.Fatalf("the detached Text Editor window's request: %v", inbox)
	}

	// A session that left the detached window is refused, and nothing is raised.
	windowSendCall(t, term, termToken, "POST", "/api/window-send/report", map[string]any{"targets": []any{}, "documents": []any{}})
	windowSendCall(t, editor, editorToken, "GET", "/api/peer-send/targets", nil)
	before := len(allowed())
	if code, out := windowSendCall(t, editor, editorToken, "POST", "/api/peer-send/send", map[string]any{"session": session.ID, "text": "x"}); code == 200 {
		t.Fatalf("text for a session no window holds was taken: %v", out)
	}
	if len(allowed()) != before {
		t.Fatal("a refusal raised a window")
	}

	// A window of the other tool never sends as the app, and children of the
	// main Toolkit window never reach peer send.
	term.launchEditorView = func(context.Context, editorViewLaunch) (<-chan error, error) { return nil, nil }
	req, _, otherID := editorFixtureOffer(t, term.editorViews)
	if _, err := term.editorViews.accept(otherID, req.ID); err != nil {
		t.Fatal(err)
	}
	otherToken := term.editorViews.views[otherID].token
	editorHTTP(term, otherToken, "/api/editor-views/current/pulse", map[string]any{})
	if r := viewTestRequest(term, otherToken, "GET", "/api/peer-send/targets", nil, nil); r.Code != http.StatusForbidden {
		t.Fatalf("an editor window of the standalone Terminal reached peer send: %d", r.Code)
	}
}

// Item 2: undo history travels with a handed-over tab within the 8 MiB of the
// request, reaches the receiving page exactly, and a tab that moved without
// the history it carried is reported to the main window.
func TestPeerHandoverCarriesUndoHistory(t *testing.T) {
	path := peerHandoverAbsolute("notes.txt")
	hash := strings.Repeat("a", 64)
	item := peerHandoverItem{ID: "doc-1", Kind: "file", Path: path, Hash: hash, Dirty: true, Text: "edited twice\n", Format: peerHandoverLF, SavedFormat: peerHandoverLF, Undo: []string{"original\n", "edited once\n"}, Redo: []string{"edited thrice\n"}}
	if err := validatePeerHandover([]peerHandoverItem{item, {ID: "doc-2", Kind: "file", Path: path, Hash: hash, Undo: []string{"before the save\n"}}}); err != nil {
		t.Fatalf("history refused: %v", err)
	}
	long := make([]string, peerHandoverHistoryEntries+1)
	for name, bad := range map[string]peerHandoverItem{
		"more entries than the editor keeps": {ID: "doc-1", Kind: "untitled", Name: "a.txt", Format: peerHandoverLF, Undo: long},
		"invalid UTF-8":                      {ID: "doc-1", Kind: "untitled", Name: "a.txt", Format: peerHandoverLF, Redo: []string{"\xff"}},
		"over 8 MiB with the text":           {ID: "doc-1", Kind: "untitled", Name: "a.txt", Dirty: true, Text: strings.Repeat("t", peerHandoverTextLimit/2), Format: peerHandoverLF, Undo: []string{strings.Repeat("u", peerHandoverTextLimit/2+1)}},
	} {
		if err := validatePeerHandover([]peerHandoverItem{bad}); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
	// A confirmation may say that a tab that opened lost its history; a tab
	// that did not open cannot.
	if !peerHandoverResultsValid([]string{"a"}, []peerHandoverResult{{ID: "a", OK: true, HistoryLost: true}}) || peerHandoverResultsValid([]string{"a"}, []peerHandoverResult{{ID: "a", Error: "refused", HistoryLost: true}}) {
		t.Fatal("history-lost confirmation rules")
	}

	// Round trip between two engines.
	_, main, editor, collected := peerHandoverPair(t)
	peerHandoverAwaitReceiver(t, main)
	files := t.TempDir()
	file, digest := peerHandoverFile(t, files, "running-config.txt", "hostname lab-switch-01\n")
	items := []peerHandoverItem{
		{ID: "doc-file", Kind: "file", Path: file, Hash: digest, Dirty: true, Text: "hostname lab-switch-03\n", Format: peerHandoverLF, SavedFormat: peerHandoverLF, Undo: []string{"hostname lab-switch-01\n", "hostname lab-switch-02\n"}, Redo: []string{"hostname lab-switch-04\n"}},
		{ID: "doc-draft", Kind: "untitled", Name: "Untitled.txt", Dirty: true, Text: "b", Format: peerHandoverLF, Undo: []string{"", "a"}},
	}
	code, started, raw := peerPage(t, main, http.MethodPost, "/api/peer-handover/start", peerHandoverJSON(t, map[string]any{"items": items}))
	if code != http.StatusOK {
		t.Fatalf("start: %d %s", code, raw)
	}
	id := started["requestId"].(string)
	request := peerAwaitRequest(t, collected, "hand-over with history")
	documents := request["documents"].([]any)
	first := documents[0].(map[string]any)
	if strings.Join(toStrings(first["undo"]), "|") != "hostname lab-switch-01\n|hostname lab-switch-02\n" || strings.Join(toStrings(first["redo"]), "|") != "hostname lab-switch-04\n" {
		t.Fatalf("the history on the receiving page: %v", first)
	}
	if strings.Join(toStrings(documents[1].(map[string]any)["undo"]), "|") != "|a" {
		t.Fatalf("an empty earlier text must survive: %v", documents[1])
	}
	if code, raw := peerHandoverAck(t, editor, `{"requestId":"`+id+`","ok":true,"outcome":"opened","items":[{"id":"doc-file","ok":true},{"id":"doc-draft","ok":true,"historyLost":true}]}`); code != http.StatusOK {
		t.Fatalf("confirmation: %d %s", code, raw)
	}
	result := peerHandoverAwait(t, main, id, func(s string) bool { return s == peerDone })
	lost := map[string]bool{}
	for _, entry := range result["items"].([]any) {
		e := entry.(map[string]any)
		lost[e["id"].(string)] = e["historyLost"] == true
	}
	if lost["doc-file"] || !lost["doc-draft"] {
		t.Fatalf("history-lost relay: %v", result)
	}
}

func toStrings(value any) []string {
	list, _ := value.([]any)
	out := make([]string, 0, len(list))
	for _, v := range list {
		s, _ := v.(string)
		out = append(out, s)
	}
	return out
}

// Window send runs in every engine with windows; remote Edit and peer send
// keep their own boundaries.
func TestWindowSendInEveryEngineAndPeerSendOnlyForStandaloneWindows(t *testing.T) {
	for _, mode := range []string{"terminal", standaloneEditorMode} {
		a := testApplication(t)
		a.windowMode = mode
		if r := requestApp(a, "GET", "/api/window-send/inbox", "", true); r.Code != http.StatusOK {
			t.Fatalf("%s: window send: %d", mode, r.Code)
		}
	}
	a := testApplication(t)
	a.windowMode = "other"
	if r := requestApp(a, "GET", "/api/window-send/inbox", "", true); r.Code != http.StatusNotFound {
		t.Fatalf("an engine without windows served window send: %d", r.Code)
	}
	if r := requestApp(a, "GET", "/api/window-send/inbox", "", false); r.Code != http.StatusUnauthorized {
		t.Fatalf("window send without a token: %d", r.Code)
	}
	// The main Toolkit window's children never reach peer send.
	f := newWindowSendFixture(t)
	for _, token := range []string{f.terminalToken, f.editorTok} {
		if r := viewTestRequest(f.app, token, "GET", "/api/peer-send/targets", nil, nil); r.Code != http.StatusForbidden {
			t.Fatalf("a child window of the main window reached peer send: %d %s", r.Code, r.Body.String())
		}
		if _, status := windowSendCall(t, f.app, token, "GET", "/api/status", nil); status["peerSend"] != "" {
			t.Fatalf("a main window's child says it sends between apps: %v", status)
		}
	}
	if _, status := windowSendCall(t, f.app, f.terminalToken, "GET", "/api/status", nil); status["windowSendRemote"] != true {
		t.Fatalf("a main window's workspace cannot offer Edit: %v", status)
	}
}
