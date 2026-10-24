package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"networktoolkit/internal/terminal"
)

func reconnectTestSession(t *testing.T, a *application) terminal.SessionInfo {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
				_, _ = io.WriteString(conn, "Reconnect fixture\r\nrouter> ")
				_, _ = io.Copy(io.Discard, conn)
			}()
		}
	}()
	response := requestApp(a, http.MethodPost, "/api/terminals", `{"type":"telnet","host":"127.0.0.1","port":`+strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)+`}`, true)
	if response.Code != http.StatusCreated {
		t.Fatalf("fixture connect: %d %s", response.Code, response.Body.String())
	}
	var body struct {
		Session terminal.SessionInfo `json:"session"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	viewTestWait(t, func() bool {
		info, ok := a.terminalSession(body.Session.ID)
		if ok && info.State == "connected" {
			body.Session = info
			return true
		}
		return false
	})
	return body.Session
}

func TestTerminalReconnectAPIKeepsIDAndRejectsOldTransportInput(t *testing.T) {
	a := testApplication(t)
	session := reconnectTestSession(t, a)
	path := "/api/terminals/" + session.ID + "/reconnect"
	zero, one := uint64(0), uint64(1)
	if response := requestApp(a, http.MethodPost, path, `{}`, false); response.Code != http.StatusUnauthorized {
		t.Fatal("unauthenticated reconnect reached a session")
	}
	if response := requestApp(a, http.MethodGet, path, "", true); response.Code != http.StatusMethodNotAllowed {
		t.Fatal("GET initiated a reconnect")
	}
	if response := requestApp(a, http.MethodPost, path, `{"host":"other.example"}`, true); response.Code != http.StatusBadRequest {
		t.Fatal("reconnect accepted replacement connection settings")
	}
	response := viewTestRequest(a, a.token, http.MethodPost, path, map[string]any{}, &zero)
	if response.Code != http.StatusOK {
		t.Fatalf("reconnect: %d %s", response.Code, response.Body.String())
	}
	var body struct {
		Session   terminal.SessionInfo `json:"session"`
		ViewState terminalViewState    `json:"viewState"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Session.ID != session.ID || body.Session.CreatedAt != session.CreatedAt || len(a.terminals.List()) != 1 {
		t.Fatal("reconnect replaced the session instead of its transport")
	}
	viewTestWait(t, func() bool { info, _ := a.terminalSession(session.ID); return info.State == "connected" })
	input := map[string]string{"data": base64.StdEncoding.EncodeToString([]byte{3, 23, 26})}
	if response := viewTestRequest(a, a.token, "POST", "/api/terminals/"+session.ID+"/input", input, &zero); response.Code != http.StatusConflict {
		t.Fatal("input queued for the old transport reached the replacement")
	}
	if response := viewTestRequest(a, a.token, "POST", "/api/terminals/"+session.ID+"/input", input, &one); response.Code != http.StatusOK {
		t.Fatalf("control keys on reconnected transport: %d %s", response.Code, response.Body.String())
	}
	if response := viewTestRequest(a, a.token, "POST", path, map[string]any{}, &zero); response.Code != http.StatusConflict {
		t.Fatal("a delayed duplicate reconnect restarted the replacement again")
	}
}

func TestDetachedTerminalReconnectKeepsSameWindowAndSession(t *testing.T) {
	a := testApplication(t)
	session := reconnectTestSession(t, a)
	other := reconnectTestSession(t, a)
	var launch terminalViewLaunch
	a.launchTerminalView = func(_ context.Context, request terminalViewLaunch) (<-chan error, error) {
		launch = request
		return nil, nil
	}
	response := viewTestRequest(a, a.token, "POST", "/api/terminal-views", map[string]any{"sessionId": session.ID, "snapshot": viewTestSnapshot(a, session)}, nil)
	if response.Code != http.StatusCreated {
		t.Fatalf("detach: %d %s", response.Code, response.Body.String())
	}
	response = viewTestRequest(a, launch.ViewToken, "POST", "/api/terminal-views/current/ready", map[string]any{}, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("ready: %d %s", response.Code, response.Body.String())
	}
	one := uint64(1)
	if response := viewTestRequest(a, a.token, "POST", "/api/terminals/"+session.ID+"/reconnect", map[string]any{}, &one); response.Code != http.StatusConflict {
		t.Fatal("the home window reconnected a session owned by its detached window")
	}
	if response := viewTestRequest(a, launch.ViewToken, "POST", "/api/terminals/"+other.ID+"/reconnect", map[string]any{}, &one); response.Code != http.StatusForbidden {
		t.Fatal("the detached window reconnected another pane's session")
	}
	response = viewTestRequest(a, launch.ViewToken, "POST", "/api/terminals/"+session.ID+"/reconnect", map[string]any{}, &one)
	if response.Code != http.StatusOK {
		t.Fatalf("reconnect in single detached window: %d %s", response.Code, response.Body.String())
	}
	info, ok := a.views.info(launch.ViewID)
	if !ok || info.SessionID != session.ID || info.State != "detached" || len(a.terminals.List()) != 2 {
		t.Fatal("reconnecting changed the owning window or created another session")
	}
	a.views.mu.Lock()
	owner := a.views.ownerLocked(session.ID)
	a.views.mu.Unlock()
	if owner.ViewID != launch.ViewID || !owner.Ready || owner.Revision != 2 {
		t.Fatal("reconnect did not retain window ownership with a new input revision")
	}
}
