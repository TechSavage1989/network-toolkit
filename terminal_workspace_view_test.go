package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"networktoolkit/internal/terminal"
)

type workspaceTestResponse struct {
	View      terminalViewInfo          `json:"view"`
	Workspace terminalWorkspaceSnapshot `json:"workspace"`
	Sessions  []terminal.SessionInfo    `json:"sessions"`
	ViewState terminalViewState         `json:"viewState"`
}

func workspaceTestDecode(t *testing.T, response *httptest.ResponseRecorder, code int) workspaceTestResponse {
	t.Helper()
	if response.Code != code {
		t.Fatalf("workspace response %d, want %d: %s", response.Code, code, response.Body.String())
	}
	var decoded workspaceTestResponse
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func workspaceTestCreate(t *testing.T, a *application, snapshot terminalWorkspaceSnapshot) (terminalViewInfo, terminalViewLaunch) {
	t.Helper()
	var launch terminalViewLaunch
	a.launchTerminalView = func(_ context.Context, request terminalViewLaunch) (<-chan error, error) {
		launch = request
		return nil, nil
	}
	response := workspaceTestDecode(t, viewTestRequest(a, a.token, http.MethodPost, "/api/terminal-views/workspace", map[string]any{"workspace": snapshot}, nil), http.StatusCreated)
	if !response.View.Workspace || launch.ViewID != response.View.ID || launch.ViewToken == a.token {
		t.Fatal("workspace did not receive an independent view capability")
	}
	return response.View, launch
}

func TestTerminalWorkspaceEmptyAndSafeDraftRoundTrip(t *testing.T) {
	for _, ready := range []bool{false, true} {
		t.Run(fmt.Sprintf("ready=%v", ready), func(t *testing.T) {
			a := testApplication(t)
			initial := terminalWorkspaceSnapshot{Display: map[string]any{
				"theme": "dark", "terminalLayout": map[string]any{"kind": "grid", "columnRatio": .65, "rowRatio": .4},
				"terminalSidebarWidth": float64(372),
				"terminalConnection":   map[string]any{"type": "ssh", "host": "draft.example", "username": "operator", "password": "draft-secret", "privateKey": "key-secret", "passphrase": "phrase-secret"},
				"terminalProfiles":     []any{map[string]any{"name": "Saved", "type": "ssh", "host": "profile.example", "password": "profile-secret"}},
				"targets":              []any{map[string]any{"host": "unrelated.example"}},
			}}
			info, launch := workspaceTestCreate(t, a, initial)
			current := workspaceTestDecode(t, viewTestRequest(a, launch.ViewToken, "GET", "/api/terminal-views/current", nil, nil), 200)
			if len(a.terminals.List()) != 0 || len(current.Sessions) != 0 || len(current.Workspace.Sessions) != 0 || len(current.Workspace.Panes) != 4 {
				t.Fatal("opening an empty workspace created a connection or lost its empty layout")
			}
			page := requestApp(a, "GET", "/terminal-view/"+info.ID, "", false)
			if page.Code != 200 || !strings.Contains(page.Body.String(), `"workspace":true`) || strings.Contains(page.Body.String(), a.token) {
				t.Fatal("linked workspace bootstrap did not preserve its scoped identity")
			}
			for _, asset := range []string{"subnet-planner.js", "report-export.js"} {
				if !strings.Contains(page.Body.String(), "src=\"/assets/"+asset+"\"") {
					t.Fatalf("Detached interface cannot resolve embedded asset %s", asset)
				}
			}
			for _, forbidden := range []string{"/api/jobs", "/api/report", "/api/storage", "/api/local-tools/ip", "/api/local-tools/hosts", "/api/local-tools/wifi", "/api/local-tools/wifi/reveal", "/api/terminal-window", "/api/external-terminal", "/api/terminals/known-hosts", "/api/terminal-views"} {
				if response := viewTestRequest(a, launch.ViewToken, "GET", forbidden, nil, nil); response.Code != 403 {
					t.Fatalf("workspace capability reached %s: %d", forbidden, response.Code)
				}
			}
			for _, forbidden := range []string{"/api/local-tools/ip", "/api/local-tools/hosts"} {
				if response := viewTestRequest(a, launch.ViewToken, "POST", forbidden, map[string]any{}, nil); response.Code != 403 {
					t.Fatalf("workspace capability reached local mutation %s: %d", forbidden, response.Code)
				}
			}
			if response := viewTestRequest(a, launch.ViewToken, "POST", "/api/terminals", map[string]any{"type": "telnet", "host": "127.0.0.1", "port": 1}, nil); response.Code != 503 {
				t.Fatal("opening workspace accepted a connection before ownership was ready")
			}
			state := viewTestRequest(a, launch.ViewToken, "GET", "/api/state", nil, nil)
			for _, forbidden := range []string{"draft-secret", "key-secret", "phrase-secret", "profile-secret", "unrelated.example"} {
				if strings.Contains(state.Body.String(), forbidden) {
					t.Fatalf("transferred draft retained %q", forbidden)
				}
			}
			if !strings.Contains(state.Body.String(), "draft.example") || !strings.Contains(state.Body.String(), "profile.example") {
				t.Fatal("safe draft or profile was lost")
			}
			if current.Workspace.Display["terminalSidebarWidth"] != float64(372) {
				t.Fatal("workspace transfer lost the sidebar width")
			}
			if ready {
				workspaceTestDecode(t, viewTestRequest(a, launch.ViewToken, "POST", "/api/terminal-views/current/ready", map[string]any{}, nil), 200)
				updated := map[string]any{"theme": "light", "terminalFontSize": 16, "terminalSidebarWidth": 416, "terminalConnection": map[string]any{"type": "serial", "serialPort": "COM9", "baudRate": 115200, "password": "late-secret"}, "terminalLayout": map[string]any{"kind": "columns", "columnRatio": .7}}
				if response := viewTestRequest(a, launch.ViewToken, "PUT", "/api/state", updated, nil); response.Code != 200 {
					t.Fatal(response.Body.String())
				}
				current = workspaceTestDecode(t, viewTestRequest(a, launch.ViewToken, "GET", "/api/terminal-views/current", nil, nil), 200)
				workspaceTestDecode(t, viewTestRequest(a, launch.ViewToken, "POST", "/api/terminal-views/current/reattach", map[string]any{"workspace": current.Workspace}, nil), 200)
			} else {
				workspaceTestDecode(t, viewTestRequest(a, a.token, "POST", "/api/terminal-views/"+info.ID+"/reattach", map[string]any{}, nil), 200)
			}
			returned := workspaceTestDecode(t, viewTestRequest(a, a.token, "GET", "/api/terminal-views/"+info.ID+"/snapshot", nil, nil), 200)
			if returned.View.State != "returned" || len(returned.Sessions) != 0 || len(a.terminals.List()) != 0 {
				t.Fatal("empty workspace could not return without creating a session")
			}
			if ready && returned.Workspace.Display["theme"] != "light" {
				t.Fatal("returning workspace lost its edited preferences")
			}
			wantWidth := float64(372)
			if ready {
				wantWidth = 416
			}
			if returned.Workspace.Display["terminalSidebarWidth"] != wantWidth {
				t.Fatal("returning workspace lost its sidebar width")
			}
			encoded, _ := json.Marshal(returned)
			if strings.Contains(string(encoded), "late-secret") || string(a.state) != "{}" {
				t.Fatal("transient workspace preferences leaked secrets or overwrote home preferences")
			}
			workspaceTestDecode(t, viewTestRequest(a, a.token, "POST", "/api/terminal-views/"+info.ID+"/accept", map[string]uint64{"revision": returned.View.Revision}, nil), 200)
			if response := viewTestRequest(a, launch.ViewToken, "POST", "/api/terminal-views/current/pulse", map[string]any{}, nil); response.Code != 410 {
				t.Fatal("accepted empty workspace retained its capability")
			}
		})
	}
}

func TestTerminalWorkspaceFourScreensPreserveSessionsAndScope(t *testing.T) {
	a := testApplication(t)
	sessions := make([]terminal.SessionInfo, 4)
	workspace := terminalWorkspaceSnapshot{Sessions: map[string]terminalScreenSnapshot{}, PaneFocus: 2, Maximized: 2, Display: map[string]any{"terminalLayout": map[string]any{"kind": "grid", "columnRatio": .65, "rowRatio": .35}}}
	for i := range sessions {
		sessions[i] = viewTestConnected(t, a)
		workspace.Sessions[sessions[i].ID] = viewTestSnapshot(a, sessions[i])
		workspace.Panes = append(workspace.Panes, sessions[i].ID)
	}
	workspace.Active = sessions[2].ID
	other := viewTestConnected(t, a)
	if err := a.terminals.Disconnect(sessions[3].ID); err != nil {
		t.Fatal(err)
	}
	// Group and session revisions deliberately differ, as after earlier moves.
	a.views.mu.Lock()
	a.views.owners[sessions[0].ID] = terminalViewOwner{Revision: 4, Ready: true}
	a.views.mu.Unlock()
	info, launch := workspaceTestCreate(t, a, workspace)
	ready := workspaceTestDecode(t, viewTestRequest(a, launch.ViewToken, "POST", "/api/terminal-views/current/ready", map[string]any{}, nil), 200)
	if len(ready.ViewState.Owners) != 4 || ready.ViewState.Owners[sessions[0].ID].Revision != 5 || ready.View.Revision != 1 {
		t.Fatal("workspace ownership did not preserve independent session revisions")
	}
	for _, path := range []string{"/api/terminals", "/api/terminals/events?wait=0", "/api/terminal-views/current"} {
		response := viewTestRequest(a, launch.ViewToken, "GET", path, nil, nil)
		if response.Code != 200 || strings.Contains(response.Body.String(), other.ID) {
			t.Fatalf("workspace session scope failed: %d %s", response.Code, response.Body.String())
		}
	}
	input := map[string]string{"data": base64.StdEncoding.EncodeToString([]byte("show version\r"))}
	for i := 0; i < 3; i++ {
		revision := ready.ViewState.Owners[sessions[i].ID].Revision
		path := "/api/terminals/" + sessions[i].ID + "/input"
		if response := viewTestRequest(a, a.token, "POST", path, input, &revision); response.Code != 409 {
			t.Fatal("home typed into a detached workspace connection")
		}
		if response := viewTestRequest(a, launch.ViewToken, "POST", path, input, &revision); response.Code != 200 {
			t.Fatalf("workspace could not type using its session revision: %s", response.Body.String())
		}
	}
	one := uint64(1)
	if response := viewTestRequest(a, launch.ViewToken, "POST", "/api/terminals/"+other.ID+"/disconnect", map[string]any{}, &one); response.Code != 403 {
		t.Fatal("workspace reached a home-owned session")
	}
	missing := workspace.clone()
	delete(missing.Sessions, sessions[3].ID)
	missing.Panes[3] = ""
	if response := viewTestRequest(a, launch.ViewToken, "POST", "/api/terminal-views/current/reattach", map[string]any{"workspace": missing}, nil); response.Code != 400 {
		t.Fatal("a partial checkpoint silently dropped a retained session")
	}
	workspaceTestDecode(t, viewTestRequest(a, a.token, "POST", "/api/terminal-views/"+info.ID+"/reattach", map[string]any{}, nil), 200)
	for id, screen := range workspace.Sessions {
		screen.Data += "\r\nFinal workspace screen"
		screen.After = a.terminalLatest()
		workspace.Sessions[id] = screen
	}
	workspaceTestDecode(t, viewTestRequest(a, launch.ViewToken, "POST", "/api/terminal-views/current/reattach", map[string]any{"workspace": workspace}, nil), 200)
	returned := workspaceTestDecode(t, viewTestRequest(a, a.token, "GET", "/api/terminal-views/"+info.ID+"/snapshot", nil, nil), 200)
	if !reflect.DeepEqual(returned.Workspace.Panes, workspace.Panes) || returned.Workspace.PaneFocus != 2 || returned.Workspace.Maximized != 2 || returned.Workspace.Active != workspace.Active {
		t.Fatal("workspace pane arrangement was not preserved")
	}
	for _, session := range sessions {
		owner := returned.ViewState.Owners[session.ID]
		if owner.ViewID != "" || owner.Ready || !strings.Contains(returned.Workspace.Sessions[session.ID].Data, "Final workspace screen") {
			t.Fatal("home was writable before all returned screens were restored")
		}
	}
	workspaceTestDecode(t, viewTestRequest(a, a.token, "POST", "/api/terminal-views/"+info.ID+"/accept", map[string]uint64{"revision": returned.View.Revision}, nil), 200)
	for i, session := range sessions {
		current, found := a.terminalSession(session.ID)
		wantState := "connected"
		if i == 3 {
			wantState = "closed"
		}
		if !found || current.State != wantState || current.CreatedAt != session.CreatedAt {
			t.Fatal("workspace transfer changed an existing connection")
		}
	}
}

func workspaceTestTelnetEndpoint(t *testing.T) terminal.ConnectRequest {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		_ = connection.SetDeadline(time.Now().Add(10 * time.Second))
		_, _ = io.WriteString(connection, "Early workspace output\r\nrouter> ")
		_, _ = io.Copy(io.Discard, connection)
	}()
	return terminal.ConnectRequest{Type: "telnet", Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port}
}

func TestTerminalWorkspaceEmptyWindowSurvivesHomeCloseAndCreatesOwnedSession(t *testing.T) {
	a := testApplication(t)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		a.runPrimaryLifecycle("http://"+a.host+"/", true, t.TempDir(), "Fixture")
	}()
	viewTestWait(t, func() bool { a.primaryMu.Lock(); defer a.primaryMu.Unlock(); return a.primaryCancel != nil })
	info, launch := workspaceTestCreate(t, a, terminalWorkspaceSnapshot{})
	a.beginPrimaryClose()
	a.closePrimary()
	viewTestWait(t, func() bool { return !a.views.state().HomePresent })
	if a.ctx.Err() != nil || a.shutdownRequested.Load() {
		t.Fatal("empty detached workspace did not retain the owner engine")
	}
	workspaceTestDecode(t, viewTestRequest(a, launch.ViewToken, "POST", "/api/terminal-views/current/ready", map[string]any{}, nil), 200)
	request := workspaceTestTelnetEndpoint(t)
	request.Password, request.PrivateKey, request.Passphrase = "password-fixture", "key-fixture", "passphrase-fixture"
	response := viewTestRequest(a, launch.ViewToken, "POST", "/api/terminals", request, nil)
	if response.Code != 201 {
		t.Fatalf("detached connection after home close: %d %s", response.Code, response.Body.String())
	}
	var created struct {
		Session   terminal.SessionInfo `json:"session"`
		ViewState terminalViewState    `json:"viewState"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	owner := created.ViewState.Owners[created.Session.ID]
	if owner.ViewID != info.ID || owner.Revision != 1 || !owner.Ready {
		t.Fatal("new session did not atomically receive the detached workspace owner")
	}
	viewTestWait(t, func() bool {
		current, ok := a.terminalSession(created.Session.ID)
		return ok && current.State == "connected" && current.BytesReceived > 0
	})
	current := workspaceTestDecode(t, viewTestRequest(a, launch.ViewToken, "GET", "/api/terminal-views/current", nil, nil), 200)
	screen, exists := current.Workspace.Sessions[created.Session.ID]
	if !exists || !screen.Fresh || screen.Data != "" || string(screen.Emulator) != "{}" {
		t.Fatal("new connection lacked a replayable initial display checkpoint")
	}
	batch, err := a.terminals.Events(context.Background(), screen.After, 0)
	if err != nil {
		t.Fatal(err)
	}
	foundOutput := false
	for _, event := range batch.Events {
		if event.SessionID == created.Session.ID && strings.Contains(string(event.Data), "Early workspace output") {
			foundOutput = true
		}
	}
	if !foundOutput {
		t.Fatal("initial checkpoint cursor skipped early connection output")
	}
	encoded, _ := json.Marshal(current)
	for _, secret := range []string{request.Password, request.PrivateKey, request.Passphrase} {
		if strings.Contains(string(encoded), secret) {
			t.Fatal("a connection secret reached the transferred screen")
		}
	}
	if response := viewTestRequest(a, a.token, "POST", "/api/terminals", request, nil); response.Code != 503 {
		t.Fatal("closed home created a connection")
	}
	// Native close uses the last checkpoint even before the renderer saves one.
	workspaceTestDecode(t, viewTestRequest(a, launch.ViewToken, "POST", "/api/terminal-views/current/release", map[string]any{}, nil), 200)
	viewTestWait(t, func() bool { return a.views.state().HomePresent })
	returned := workspaceTestDecode(t, viewTestRequest(a, a.token, "GET", "/api/terminal-views/"+info.ID+"/snapshot", nil, nil), 200)
	if len(returned.Sessions) != 1 || !returned.Workspace.Sessions[created.Session.ID].Fresh {
		t.Fatal("native close lost the newly created session")
	}
	workspaceTestDecode(t, viewTestRequest(a, a.token, "POST", "/api/terminal-views/"+info.ID+"/accept", map[string]uint64{"revision": returned.View.Revision}, nil), 200)
	still, _ := a.terminalSession(created.Session.ID)
	if still.State != "connected" || still.CreatedAt != created.Session.CreatedAt {
		t.Fatal("reattachment reconnected the child-created transport")
	}
	a.beginPrimaryClose()
	a.closePrimary()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("final home close did not stop the now empty window registry")
	}
}

func TestTerminalWorkspaceDelayedConnectionCannotOutliveReturn(t *testing.T) {
	a := testApplication(t)
	info, launch := workspaceTestCreate(t, a, terminalWorkspaceSnapshot{})
	workspaceTestDecode(t, viewTestRequest(a, launch.ViewToken, "POST", "/api/terminal-views/current/ready", map[string]any{}, nil), 200)
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	body := &delayedTerminalBody{reader: strings.NewReader(`{"type":"telnet","host":"127.0.0.1","port":1}`), entered: make(chan struct{}), release: release}
	request := httptest.NewRequest("POST", "http://"+a.host+"/api/terminals", body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Session-Token", launch.ViewToken)
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { defer close(done); a.ServeHTTP(response, request) }()
	select {
	case <-body.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("connection did not reach the delayed decode boundary")
	}
	a.views.returnLast(info.ID, "fixture close")
	unblock()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("delayed connection did not finish")
	}
	if response.Code != 503 || len(a.terminals.List()) != 0 {
		t.Fatal("a delayed detached connection escaped its returned workspace")
	}
}

func TestTerminalWorkspaceClosedHistoryEvictionAndRemoval(t *testing.T) {
	a := testApplication(t)
	closed := viewTestConnected(t, a)
	if err := a.terminals.Disconnect(closed.ID); err != nil {
		t.Fatal(err)
	}
	workspace := terminalWorkspaceSnapshot{Sessions: map[string]terminalScreenSnapshot{closed.ID: viewTestSnapshot(a, closed)}, Panes: []string{closed.ID}, Active: closed.ID}
	info, launch := workspaceTestCreate(t, a, workspace)
	workspaceTestDecode(t, viewTestRequest(a, launch.ViewToken, "POST", "/api/terminal-views/current/ready", map[string]any{}, nil), 200)
	request := workspaceTestTelnetEndpoint(t)
	for i := 0; i < 64; i++ {
		var session terminal.SessionInfo
		viewTestWait(t, func() bool {
			var err error
			session, err = a.connectHomeTerminal(request)
			if err != nil && !errors.Is(err, terminal.ErrLimit) {
				t.Fatal(err)
			}
			return err == nil
		})
		if err := a.terminals.Disconnect(session.ID); err != nil {
			t.Fatal(err)
		}
	}
	_, snapshot, ok := a.views.workspaceGet(info.ID)
	if !ok || len(snapshot.Sessions) != 0 || snapshot.Active != "" || snapshot.Panes[0] != "" {
		t.Fatal("manager history eviction left an unresolvable workspace screen")
	}
	if _, exists := a.views.state().Owners[closed.ID]; exists {
		t.Fatal("history eviction retained a capability for a removed session")
	}
	// A new closed member can also be dismissed directly by its owning group.
	r := httptest.NewRequest("POST", "http://"+a.host+"/api/terminals", nil)
	r = r.WithContext(context.WithValue(r.Context(), terminalViewContextKey{}, info.ID))
	var member terminal.SessionInfo
	viewTestWait(t, func() bool {
		var err error
		member, err = a.connectTerminalForRequest(r, request)
		return err == nil
	})
	if err := a.terminals.Disconnect(member.ID); err != nil {
		t.Fatal(err)
	}
	revision := a.views.state().Owners[member.ID].Revision
	response := viewTestRequest(a, launch.ViewToken, "DELETE", "/api/terminals/"+member.ID, nil, &revision)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	current := workspaceTestDecode(t, viewTestRequest(a, launch.ViewToken, "GET", "/api/terminal-views/current", nil, nil), 200)
	if len(current.Sessions) != 0 || len(current.Workspace.Sessions) != 0 {
		t.Fatal("dismissed group member remained in the return checkpoint")
	}
	workspaceTestDecode(t, viewTestRequest(a, launch.ViewToken, "POST", "/api/terminal-views/current/reattach", map[string]any{"workspace": current.Workspace}, nil), 200)
	returned, _ := a.views.info(info.ID)
	if err := a.views.accept(info.ID, returned.Revision); err != nil {
		t.Fatal(err)
	}
}

func TestTerminalWorkspaceSnapshotValidationAndFailureRollback(t *testing.T) {
	base := terminalScreenSnapshot{Data: "screen", Emulator: json.RawMessage(`{"version":1}`), Cols: 80, Rows: 24, Config: &terminalWindowPreset{Type: "telnet", Host: "127.0.0.1"}}
	bounded := terminalWorkspaceSnapshot{Sessions: map[string]terminalScreenSnapshot{}}
	for i := 0; i < 64; i++ {
		bounded.Sessions[fmt.Sprint(i)] = base.clone()
	}
	if err := bounded.validate(0); err != nil {
		t.Fatalf("64 retained tabs were rejected: %v", err)
	}
	tooMany := bounded.clone()
	tooMany.Sessions["extra"] = base
	if err := tooMany.validate(0); err == nil {
		t.Fatal("oversized retained tab list was accepted")
	}
	a := testApplication(t)
	launches := 0
	a.launchTerminalView = func(context.Context, terminalViewLaunch) (<-chan error, error) {
		launches++
		return nil, errors.New("fixture launch failure")
	}
	for _, mutate := range []func(*terminalWorkspaceSnapshot){
		func(s *terminalWorkspaceSnapshot) { s.Panes = []string{"missing"} },
		func(s *terminalWorkspaceSnapshot) { s.PaneFocus = 4 },
		func(s *terminalWorkspaceSnapshot) {
			s.Sessions = map[string]terminalScreenSnapshot{}
			large := base.clone()
			large.Data = strings.Repeat("x", 7<<20)
			for i := 0; i < 5; i++ {
				s.Sessions[fmt.Sprint(i)] = large
			}
		},
	} {
		snapshot := terminalWorkspaceSnapshot{}
		mutate(&snapshot)
		if _, err := a.views.createWorkspace(snapshot); err == nil || a.views.count() != 0 || launches != 0 {
			t.Fatal("invalid workspace changed ownership or launched a window")
		}
	}
	for _, body := range []string{`null`, `{}`, `{"workspace":null}`, `{"workspace":{},"password":"secret"}`} {
		if response := requestApp(a, "POST", "/api/terminal-views/workspace", body, true); response.Code != 400 {
			t.Fatal("invalid workspace envelope was accepted")
		}
	}
	info, err := a.views.createWorkspace(terminalWorkspaceSnapshot{})
	if err == nil || launches != 1 {
		t.Fatal("fixture did not reach the launch rollback")
	}
	returned, snapshot, ok := a.views.workspaceGet(info.ID)
	if !ok || returned.State != "returned" || returned.Note == "" || len(snapshot.Sessions) != 0 {
		t.Fatal("failed empty launch was not returned safely")
	}
	if err := a.views.accept(info.ID, returned.Revision); err != nil || a.views.count() != 0 {
		t.Fatal("failed launch could not be accepted by home")
	}
}

func TestTerminalWorkspaceReturnPreservesPendingSSHTrust(t *testing.T) {
	a := testApplication(t)
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	var authentications atomic.Int32
	config := &ssh.ServerConfig{PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) { authentications.Add(1); return nil, nil }}
	config.AddHostKey(signer)
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
		server, channels, requests, err := ssh.NewServerConn(conn, config)
		if err != nil {
			return
		}
		defer server.Close()
		go ssh.DiscardRequests(requests)
		for incoming := range channels {
			channel, requests, err := incoming.Accept()
			if err != nil {
				continue
			}
			go func() {
				defer channel.Close()
				for request := range requests {
					switch request.Type {
					case "pty-req", "shell":
						_ = request.Reply(true, nil)
					default:
						_ = request.Reply(false, nil)
					}
				}
			}()
		}
	}()
	session, err := a.terminals.Connect(terminal.ConnectRequest{Type: "ssh", Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, Username: "fixture", Password: "ephemeral-fixture", TimeoutMs: 1000})
	if err != nil {
		t.Fatal(err)
	}
	var pending terminal.SessionInfo
	viewTestWait(t, func() bool { pending, _ = a.terminalSession(session.ID); return pending.Prompt != nil })
	if pending.Prompt.Kind != "host-key" || authentications.Load() != 0 {
		t.Fatal("fixture did not wait for trust before authentication")
	}
	promptID := pending.Prompt.ID
	workspace := terminalWorkspaceSnapshot{Sessions: map[string]terminalScreenSnapshot{session.ID: viewTestSnapshot(a, pending)}, Panes: []string{session.ID}, Active: session.ID}
	info, launch := workspaceTestCreate(t, a, workspace)
	workspaceTestDecode(t, viewTestRequest(a, launch.ViewToken, "POST", "/api/terminal-views/current/ready", map[string]any{}, nil), 200)
	workspaceTestDecode(t, viewTestRequest(a, launch.ViewToken, "POST", "/api/terminal-views/current/reattach", map[string]any{"workspace": workspace}, nil), 200)
	returned := workspaceTestDecode(t, viewTestRequest(a, a.token, "GET", "/api/terminal-views/"+info.ID+"/snapshot", nil, nil), 200)
	if len(returned.Sessions) != 1 || returned.Sessions[0].Prompt == nil || returned.Sessions[0].Prompt.ID != promptID || authentications.Load() != 0 {
		t.Fatal("moving windows canceled, answered or replaced the pending SSH prompt")
	}
	workspaceTestDecode(t, viewTestRequest(a, a.token, "POST", "/api/terminal-views/"+info.ID+"/accept", map[string]uint64{"revision": returned.View.Revision}, nil), 200)
	revision := returned.ViewState.Owners[session.ID].Revision
	response := viewTestRequest(a, a.token, "POST", "/api/terminals/"+session.ID+"/respond", terminal.PromptReply{PromptID: promptID, Action: "once"}, &revision)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	viewTestWait(t, func() bool { current, _ := a.terminalSession(session.ID); return current.State == "connected" })
	current, _ := a.terminalSession(session.ID)
	if authentications.Load() != 1 || current.CreatedAt != session.CreatedAt {
		t.Fatal("reattachment repeated SSH authentication or recreated its transport")
	}
	_ = a.terminals.Disconnect(session.ID)
	select {
	case <-serverDone:
	case <-time.After(3 * time.Second):
		t.Fatal("SSH fixture did not close")
	}
}
