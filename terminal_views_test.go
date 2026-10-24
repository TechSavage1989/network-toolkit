package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"networktoolkit/internal/terminal"
)

func viewTestRequest(a *application, token, method, path string, body any, revision *uint64) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, "http://"+a.host+path, strings.NewReader(string(raw)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Session-Token", token)
	if revision != nil {
		r.Header.Set("X-Terminal-Revision", strconv.FormatUint(*revision, 10))
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}

func viewTestConnected(t *testing.T, a *application) terminal.SessionInfo {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
		_, _ = io.WriteString(conn, "Live checkpoint fixture\r\nrouter> ")
		_, _ = io.Copy(io.Discard, conn)
	}()
	session, err := a.terminals.Connect(terminal.ConnectRequest{Type: "telnet", Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port})
	if err != nil {
		t.Fatal(err)
	}
	viewTestWait(t, func() bool {
		current, ok := a.terminalSession(session.ID)
		if ok && current.State == "connected" {
			session = current
			return true
		}
		return false
	})
	return session
}

func viewTestWait(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for a local terminal view transition")
}

func viewTestSnapshot(a *application, session terminal.SessionInfo) terminalScreenSnapshot {
	return terminalScreenSnapshot{Data: "\x1b[0mLive checkpoint fixture\r\nrouter> ", Emulator: json.RawMessage(`{"version":1}`), Cols: 80, Rows: 24, After: a.terminalLatest(), Config: &terminalWindowPreset{Type: session.Type, Host: session.Host, Port: session.Port}}
}

func TestTerminalLiveViewOwnershipAndScope(t *testing.T) {
	a := testApplication(t)
	var launched terminalViewLaunch
	a.launchTerminalView = func(_ context.Context, launch terminalViewLaunch) (<-chan error, error) {
		launched = launch
		return nil, nil
	}
	session := viewTestConnected(t, a)
	other := viewTestConnected(t, a)
	snapshot := viewTestSnapshot(a, session)
	r := viewTestRequest(a, a.token, "POST", "/api/terminal-views", map[string]any{"sessionId": session.ID, "snapshot": snapshot}, nil)
	if r.Code != 201 {
		t.Fatalf("Create view: %d %s", r.Code, r.Body.String())
	}
	if launched.ViewID == "" || launched.ViewToken == a.token || launched.OwnerInstanceID != a.windowInstanceID {
		t.Fatal("invalid isolated launch capability")
	}
	page := requestApp(a, "GET", "/terminal-view/"+launched.ViewID, "", false)
	if page.Code != 200 || !strings.Contains(page.Body.String(), launched.ViewToken) || strings.Contains(page.Body.String(), a.token) {
		t.Fatal("detached page did not isolate its token")
	}
	if !strings.Contains(page.Body.String(), `src="/assets/Network_Toolkit_Icon.svg"`) || strings.Contains(page.Body.String(), `"../assets/`) {
		t.Fatal("detached page did not resolve the embedded app icon")
	}
	for _, path := range []string{"/api/jobs", "/api/report", "/api/vendors/status", "/api/storage", "/api/local-tools/ip", "/api/local-tools/hosts", "/api/local-tools/wifi", "/api/local-tools/wifi/reveal", "/api/external-terminal", "/api/terminal-window", "/api/terminals/known-hosts", "/api/terminals/serial-ports", "/api/terminal-views"} {
		if result := viewTestRequest(a, launched.ViewToken, "GET", path, nil, nil); result.Code != 403 {
			t.Fatalf("Scoped token reached %s: %d", path, result.Code)
		}
	}
	for _, path := range []string{"/api/local-tools/ip", "/api/local-tools/hosts"} {
		if result := viewTestRequest(a, launched.ViewToken, "POST", path, map[string]any{}, nil); result.Code != 403 {
			t.Fatalf("Scoped token reached local mutation %s: %d", path, result.Code)
		}
	}
	if result := viewTestRequest(a, launched.ViewToken, "POST", "/api/terminals", map[string]string{"type": "telnet"}, nil); result.Code != 403 {
		t.Fatal("view created another transport")
	}
	for _, path := range []string{"/api/terminals", "/api/terminals/events?wait=0"} {
		result := viewTestRequest(a, launched.ViewToken, "GET", path, nil, nil)
		if result.Code != 200 || strings.Contains(result.Body.String(), other.ID) || !strings.Contains(result.Body.String(), session.ID) {
			t.Fatalf("view list leaked or lost session: %s", result.Body.String())
		}
	}
	zero, one, two := uint64(0), uint64(1), uint64(2)
	input := map[string]string{"data": base64.StdEncoding.EncodeToString([]byte("input\r"))}
	path := "/api/terminals/" + session.ID + "/input"
	if result := viewTestRequest(a, a.token, "POST", path, input, &zero); result.Code != 200 {
		t.Fatal("opening reservation rejected in-flight owner input")
	}
	if result := viewTestRequest(a, launched.ViewToken, "POST", "/api/terminal-views/current/ready", map[string]any{}, nil); result.Code != 200 {
		t.Fatal("view could not commit its ownership")
	}
	for _, token := range []string{a.token, launched.ViewToken} {
		if result := viewTestRequest(a, token, "POST", path, input, &zero); result.Code != 409 {
			t.Fatalf("stale input accepted: %d", result.Code)
		}
	}
	if result := viewTestRequest(a, a.token, "POST", path, input, &one); result.Code != 409 {
		t.Fatal("wrong window typed using the current revision")
	}
	if result := viewTestRequest(a, launched.ViewToken, "POST", path, input, &one); result.Code != 200 {
		t.Fatalf("owning view input: %d %s", result.Code, result.Body.String())
	}
	if result := viewTestRequest(a, launched.ViewToken, "POST", "/api/terminals/"+other.ID+"/disconnect", map[string]any{}, &one); result.Code != 403 {
		t.Fatal("view reached a different connection")
	}
	if result := viewTestRequest(a, launched.ViewToken, "POST", "/api/terminals/"+session.ID+"/resize", map[string]int{"cols": 110, "rows": 31}, &one); result.Code != 200 {
		t.Fatal("owning view could not resize")
	}
	if result := viewTestRequest(a, a.token, "DELETE", "/api/terminals/"+session.ID, nil, &one); result.Code != 409 {
		t.Fatal("home removed a detached connection")
	}
	stateBefore := string(a.state)
	if result := viewTestRequest(a, launched.ViewToken, "PUT", "/api/state", map[string]any{"theme": "light", "terminalSidebarTab": "macros", "terminalSidebarWidth": 372, "terminalSidebarCollapsed": true, "terminalComposerCollapsed": true, "terminalMacroConfirmation": "never", "terminalProfiles": []any{map[string]string{"password": "forbidden-secret"}}}, nil); result.Code != 200 {
		t.Fatal("transient view preferences failed")
	}
	preferences := viewTestRequest(a, launched.ViewToken, "GET", "/api/state", nil, nil)
	var presented map[string]any
	if preferences.Code != 200 || json.Unmarshal(preferences.Body.Bytes(), &presented) != nil || presented["terminalSidebarTab"] != "macros" || presented["terminalSidebarWidth"] != float64(372) || presented["terminalSidebarCollapsed"] != true || presented["terminalComposerCollapsed"] != true || presented["terminalMacroConfirmation"] != "never" {
		t.Fatalf("detached macro presentation preferences were lost: %s", preferences.Body.String())
	}
	if _, exists := presented["terminalProfiles"]; exists || bytes.Contains(preferences.Body.Bytes(), []byte("forbidden-secret")) {
		t.Fatal("single-session presentation retained saved connection data")
	}
	if string(a.state) != stateBefore {
		t.Fatal("detached view changed workspace preferences")
	}
	snapshot.Data += "final checkpoint"
	snapshot.After = a.terminalLatest()
	if result := viewTestRequest(a, launched.ViewToken, "POST", "/api/terminal-views/current/reattach", map[string]any{"snapshot": snapshot}, nil); result.Code != 200 {
		t.Fatalf("return: %d %s", result.Code, result.Body.String())
	}
	if result := viewTestRequest(a, a.token, "POST", path, input, &two); result.Code != 409 {
		t.Fatal("home typed before its display was restored")
	}
	if result := viewTestRequest(a, launched.ViewToken, "POST", path, input, &one); result.Code != 409 {
		t.Fatal("old view typed after returning")
	}
	result := viewTestRequest(a, a.token, "GET", "/api/terminal-views/"+launched.ViewID+"/snapshot", nil, nil)
	if result.Code != 200 || !strings.Contains(result.Body.String(), "final checkpoint") {
		t.Fatal("final checkpoint was not available for home restore")
	}
	if result := viewTestRequest(a, a.token, "POST", "/api/terminal-views/"+launched.ViewID+"/accept", map[string]uint64{"revision": two}, nil); result.Code != 200 {
		t.Fatal("home could not accept restored display")
	}
	if result := viewTestRequest(a, launched.ViewToken, "POST", "/api/terminal-views/current/pulse", map[string]any{}, nil); result.Code != http.StatusGone {
		t.Fatalf("accepted native view did not close silently: %d", result.Code)
	}
	if result := viewTestRequest(a, launched.ViewToken, "GET", "/api/state", nil, nil); result.Code != http.StatusGone {
		t.Fatal("retired capability reached an app route")
	}
	if result := viewTestRequest(a, a.token, "POST", path, input, &two); result.Code != 200 {
		t.Fatalf("home input after restore: %d %s", result.Code, result.Body.String())
	}
	current, found := a.terminalSession(session.ID)
	if !found || current.State != "connected" || current.CreatedAt != session.CreatedAt || a.views.count() != 0 {
		t.Fatal("moving the display recreated or disconnected the original transport")
	}
	if data, err := os.ReadFile(a.dataPath); err == nil && strings.Contains(string(data), "checkpoint") {
		t.Fatal("live screen was persisted")
	}
}

func TestTerminalViewLaunchFailureAndCrashReturnCheckpoint(t *testing.T) {
	for _, failure := range []bool{true, false} {
		t.Run(strconv.FormatBool(failure), func(t *testing.T) {
			a := testApplication(t)
			session := viewTestConnected(t, a)
			done := make(chan error, 1)
			a.launchTerminalView = func(context.Context, terminalViewLaunch) (<-chan error, error) {
				if failure {
					return nil, errors.New("fixture launch failure")
				}
				return done, nil
			}
			info, err := a.views.create(session.ID, viewTestSnapshot(a, session))
			if failure && err == nil || !failure && err != nil {
				t.Fatalf("unexpected launch result: %v", err)
			}
			if !failure {
				if _, err = a.views.ready(info.ID); err != nil {
					t.Fatal(err)
				}
				done <- errors.New("fixture window crash")
				close(done)
			}
			viewTestWait(t, func() bool {
				current, ok := a.views.info(info.ID)
				return ok && current.State == "returned" && current.Note != ""
			})
			current, _, _, _ := a.views.get(info.ID)
			if err = a.views.accept(info.ID, current.Revision); err != nil {
				t.Fatal(err)
			}
			still, _ := a.terminalSession(session.ID)
			if still.State != "connected" || still.CreatedAt != session.CreatedAt {
				t.Fatal("view failure replaced the transport")
			}
		})
	}
}

func TestTerminalViewKeepsOwnerAliveAndReopensHome(t *testing.T) {
	a := testApplication(t)
	a.launchTerminalView = func(context.Context, terminalViewLaunch) (<-chan error, error) { return nil, nil }
	session := viewTestConnected(t, a)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		a.runPrimaryLifecycle("http://"+a.host+"/", true, t.TempDir(), "Fixture")
	}()
	viewTestWait(t, func() bool { a.primaryMu.Lock(); defer a.primaryMu.Unlock(); return a.primaryCancel != nil })
	info, err := a.views.create(session.ID, viewTestSnapshot(a, session))
	if err != nil {
		t.Fatal(err)
	}
	a.beginPrimaryClose()
	a.closePrimary()
	viewTestWait(t, func() bool { return !a.views.state().HomePresent })
	if a.ctx.Err() != nil {
		t.Fatal("opening view did not reserve the owner's lifetime")
	}
	if _, err = a.views.ready(info.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = a.views.checkpoint(info.ID, viewTestSnapshot(a, session), true); err != nil {
		t.Fatal(err)
	}
	viewTestWait(t, func() bool { return a.views.state().HomePresent })
	returned, ok := a.views.info(info.ID)
	if !ok {
		t.Fatal("pending home snapshot was lost")
	}
	if err = a.views.accept(info.ID, returned.Revision); err != nil {
		t.Fatal(err)
	}
	current, _ := a.terminalSession(session.ID)
	if current.State != "connected" || current.CreatedAt != session.CreatedAt {
		t.Fatal("closing home interrupted its detached transport")
	}
	a.beginPrimaryClose()
	a.closePrimary()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("owner did not close after its final window")
	}
	if a.ctx.Err() == nil {
		t.Fatal("final window did not stop the workspace")
	}
}

func TestTerminalSnapshotBoundsAndTransferSchema(t *testing.T) {
	a := testApplication(t)
	session := viewTestConnected(t, a)
	good := viewTestSnapshot(a, session)
	for _, mutate := range []func(*terminalScreenSnapshot){
		func(s *terminalScreenSnapshot) { s.Data = strings.Repeat("x", maxTerminalSnapshot+1) },
		func(s *terminalScreenSnapshot) { s.Emulator = json.RawMessage(`null`) },
		func(s *terminalScreenSnapshot) { s.Emulator = json.RawMessage(`[]`) },
		func(s *terminalScreenSnapshot) { s.Cols = 501 },
		func(s *terminalScreenSnapshot) { s.Rows = 1 },
		func(s *terminalScreenSnapshot) { s.After = a.terminalLatest() + 1 },
		func(s *terminalScreenSnapshot) { s.Config = nil },
	} {
		snapshot := good.clone()
		mutate(&snapshot)
		if snapshot.validate(a.terminalLatest()) == nil {
			t.Fatal("invalid screen snapshot accepted")
		}
	}
	a.launchTerminalView = func(context.Context, terminalViewLaunch) (<-chan error, error) { return nil, nil }
	body := `{"sessionId":"` + session.ID + `","snapshot":{"data":"","emulator":{},"cols":80,"rows":24,"after":0,"config":{"type":"telnet","password":"never-send"}}}`
	if result := requestApp(a, "POST", "/api/terminal-views", body, true); result.Code != http.StatusBadRequest {
		t.Fatal("snapshot accepted a credential field")
	}
}
