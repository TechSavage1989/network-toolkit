package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"networktoolkit/internal/terminal"
)

type terminalSFTP123Fixture struct {
	*persistentBrowserFixture
	session  terminal.SessionInfo
	launch   terminalViewLaunch
	revision uint64
	exited   chan error
}

func newTerminalSFTP123ShellFixture(t *testing.T) *persistentBrowserFixture {
	t.Helper()
	f := newShellSFTPBrowserFixture(t)
	// Match production startup before any client/channel can use this root.
	// Windows Temp can contain an 8.3 alias; confinement compares resolved paths.
	root, err := transferRoot(f.root)
	if err != nil {
		t.Fatal(err)
	}
	f.root = root
	return f
}

func newTerminalSFTP123Fixture(t *testing.T, detached bool) *terminalSFTP123Fixture {
	t.Helper()
	f := &terminalSFTP123Fixture{persistentBrowserFixture: newTerminalSFTP123ShellFixture(t)}
	id := f.connect(t, nil)
	code, result := f.operations(t, id, "open-terminal", nil)
	if code != 200 {
		t.Fatal(code, result)
	}
	sessionID := result["session"].(map[string]any)["id"].(string)
	viewTestWait(t, func() bool { f.session, _ = f.app.terminalSession(sessionID); return f.session.State == "connected" })
	if detached {
		f.exited = make(chan error, 1)
		f.app.launchTerminalView = func(_ context.Context, l terminalViewLaunch) (<-chan error, error) {
			f.launch = l
			return f.exited, nil
		}
		t.Cleanup(func() {
			select {
			case f.exited <- nil:
			default:
			}
		})
		if _, err := f.app.views.createWorkspace(terminalWorkspaceSnapshot{Sessions: map[string]terminalScreenSnapshot{f.session.ID: viewTestSnapshot(f.app, f.session)}, Panes: []string{f.session.ID}, Active: f.session.ID}); err != nil {
			t.Fatal(err)
		}
		if _, err := f.app.views.ready(f.launch.ViewID); err != nil {
			t.Fatal(err)
		}
	}
	f.revision = f.app.views.state().Owners[f.session.ID].Revision
	t.Cleanup(func() {
		f.app.finishExplorerOwner("")
		f.app.finishExplorerOwner(terminalExplorerOwnerPrefix + f.launch.ViewID)
	})
	return f
}
func (f *terminalSFTP123Fixture) token() string {
	if f.launch.ViewToken != "" {
		return f.launch.ViewToken
	}
	return f.app.token
}
func (f *terminalSFTP123Fixture) request(method, route string, body any) *httptest.ResponseRecorder {
	return viewTestRequest(f.app, f.token(), method, route, body, &f.revision)
}
func (f *terminalSFTP123Fixture) binding() editorBinding {
	return editorBinding{Kind: "terminal", SessionID: f.session.ID, Revision: f.revision}
}
func (f *terminalSFTP123Fixture) direct(t *testing.T, path string) string {
	t.Helper()
	w := f.request("POST", "/api/explorer-transfer", explorerTransferRequest{Action: "direct", Binding: f.binding(), Paths: []string{path}})
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var out struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &out)
	return out.ID
}
func (f *terminalSFTP123Fixture) action(id, action string) *httptest.ResponseRecorder {
	return f.request("POST", "/api/explorer-transfer", map[string]any{"id": id, "action": action, "gesture": 1, "outcome": "copied"})
}
func (f *terminalSFTP123Fixture) start(t *testing.T, id string) {
	t.Helper()
	if w := f.action(id, "arm"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := f.request("GET", "/api/explorer-transfer/native", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"direct":true`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := f.action(id, "start"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestTerminalSFTP123ScopedReadsAndStrictRequests(t *testing.T) {
	f := newTerminalSFTP123Fixture(t, true)
	for _, action := range []string{"probe", "list", "stat", "preview", "read-text"} {
		p := "startup.cfg"
		if action == "list" {
			p = "."
		}
		w := f.request("POST", "/api/remote-browser", map[string]any{"protocol": "ssh", "sessionId": f.session.ID, "action": action, "path": p})
		if w.Code != 200 {
			t.Fatal(action, w.Code, w.Body.String())
		}
		if action == "probe" && !strings.Contains(w.Body.String(), `"readOnly":true`) {
			t.Fatal("missing scoped capability", w.Body.String())
		}
		if action == "preview" && !strings.Contains(w.Body.String(), "hostname test-switch") {
			t.Fatal("preview missing bytes")
		}
		for _, secret := range []string{f.app.token, f.launch.ViewToken, "persistent-test-password", "privateKey", "passphrase"} {
			if strings.Contains(w.Body.String(), secret) {
				t.Fatal("secret in response")
			}
		}
	}
	for _, action := range []string{"download", "upload", "write-text", "mkdir", "remove", "rmdir", "rename", "chmod", "copy", "connect", "disconnect", "open-terminal", "fingerprint"} {
		w := f.request("POST", "/api/remote-browser", map[string]any{"protocol": "ssh", "sessionId": f.session.ID, "action": action})
		if w.Code != 403 {
			t.Fatal("action admitted", action, w.Code, w.Body.String())
		}
	}
	for _, field := range []string{"connectionId", "host", "password", "username", "port", "profileId", "credentialId", "privateKey", "keyPath", "passphrase", "localPath", "newPath", "content", "overwrite", "safeExport"} {
		w := f.request("POST", "/api/remote-browser", map[string]any{"protocol": "ssh", "sessionId": f.session.ID, "action": "list", field: "SYNTHETIC_SECRET"})
		if w.Code != 400 || strings.Contains(w.Body.String(), "SYNTHETIC_SECRET") {
			t.Fatal("unknown/sensitive field admitted or echoed", field, w.Code)
		}
	}
	for _, method := range []string{"GET", "PUT", "DELETE", "PATCH", "HEAD"} {
		if w := f.request(method, "/api/remote-browser", nil); w.Code != 405 {
			t.Fatal(method, w.Code)
		}
	}
	if w := f.request("POST", "/api/remote-browser", map[string]any{"protocol": "ssh", "sessionId": "foreign", "action": "list"}); w.Code != 409 {
		t.Fatal("foreign session", w.Code)
	}
	stale := f.revision - 1
	if w := viewTestRequest(f.app, f.token(), "POST", "/api/remote-browser", map[string]any{"protocol": "ssh", "sessionId": f.session.ID, "action": "list"}, &stale); w.Code != 409 {
		t.Fatal("stale revision", w.Code)
	}
	if w := viewTestRequest(f.app, f.app.token, "POST", "/api/remote-browser", map[string]any{"protocol": "ssh", "sessionId": f.session.ID, "action": "list"}, &f.revision); w.Code != 409 {
		t.Fatal("main borrowed child", w.Code)
	}
	for _, route := range []string{"/api/remote-browser/drag-ticket", "/api/remote-browser/drop-upload", "/api/explorer-transfer/other", "/api/local-files", "/api/transfer/local-drop"} {
		if w := f.request("POST", route, map[string]any{}); w.Code != 403 {
			t.Fatal("broadened route", route, w.Code)
		}
	}
	for _, variant := range []string{"no-token", "origin", "cross-site", "method"} {
		req := httptest.NewRequest("POST", "http://"+f.app.host+"/api/explorer-transfer", strings.NewReader(`{"action":"direct","binding":{"kind":"terminal"},"paths":["/file"]}`))
		req.Header.Set("X-Session-Token", f.token())
		expected := 403
		switch variant {
		case "no-token":
			req.Header.Del("X-Session-Token")
			expected = 401
		case "origin":
			req.Header.Set("Origin", "https://foreign.invalid")
		case "cross-site":
			req.Header.Set("Sec-Fetch-Site", "cross-site")
		case "method":
			req.Method = "DELETE"
			expected = 405
		}
		w := httptest.NewRecorder()
		f.app.ServeHTTP(w, req)
		if w.Code != expected {
			t.Fatal(variant, w.Code, w.Body.String())
		}
	}
	if f.connections.Load() != 1 || f.logins.Load() != 1 {
		t.Fatal("read opened another connection/login")
	}
	if err := f.app.terminals.Write(f.session.ID, []byte("retained shell\r")); err != nil {
		t.Fatal(err)
	}
}

func TestTerminalSFTP123DirectCopiesAndNamespace(t *testing.T) {
	for _, detached := range []bool{false, true} {
		t.Run(strconv.FormatBool(detached), func(t *testing.T) {
			f := newTerminalSFTP123Fixture(t, detached)
			if err := os.MkdirAll(filepath.Join(f.root, "folder", "empty"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(f.root, "folder", "child.txt"), []byte("SYNTHETIC123\r\n"), 0600); err != nil {
				t.Fatal(err)
			}
			for _, source := range []string{"/startup.cfg", "/folder"} {
				id := f.direct(t, source)
				f.app.explorerTransfers.mu.Lock()
				j := f.app.explorerTransfers.jobs[id]
				stage, reads := j.stage, j.entries
				f.app.explorerTransfers.mu.Unlock()
				if stage != "" || reads != 0 {
					t.Fatal("registration read source")
				}
				if detached {
					if j.owner != terminalExplorerOwnerPrefix+f.launch.ViewID {
						t.Fatal("missing terminal namespace")
					}
					if w := viewTestRequest(f.app, f.app.token, "GET", "/api/explorer-transfer?id="+id, nil, nil); w.Code != 404 {
						t.Fatal("main accessed child job", w.Code)
					}
					f.app.editorViews.mu.Lock()
					f.app.editorViews.views[f.launch.ViewID] = &editorWindow{info: editorViewInfo{ID: f.launch.ViewID, State: "ready"}}
					f.app.editorViews.mu.Unlock()
					if w := exportRequest(f.app, f.launch.ViewID, "GET", "/api/explorer-transfer?id="+id, nil); w.Code != 404 {
						t.Fatal("colliding editor ID accessed terminal job", w.Code)
					}
				}
				f.start(t, id)
				if w := f.action(id, "materialize"); w.Code != 200 {
					t.Fatal(w.Code, w.Body.String())
				}
				var output []string
				viewTestWait(t, func() bool {
					w := f.request("GET", "/api/explorer-transfer/native?id="+id, nil)
					if w.Code == 202 {
						return false
					}
					if w.Code != 200 {
						t.Fatal(w.Code, w.Body.String())
					}
					var result struct{ Paths []string }
					json.Unmarshal(w.Body.Bytes(), &result)
					output = result.Paths
					return len(output) == 1
				})
				file, want := output[0], "hostname test-switch\r\n"
				if source == "/folder" {
					file = filepath.Join(file, "child.txt")
					want = "SYNTHETIC123\r\n"
					if info, err := os.Stat(filepath.Join(output[0], "empty")); err != nil || !info.IsDir() {
						t.Fatal("empty directory missing", err)
					}
				}
				got, err := os.ReadFile(file)
				if err != nil || string(got) != want {
					t.Fatal("wire copy differs", err, string(got))
				}
				if w := f.action(id, "finish"); w.Code != 200 {
					t.Fatal(w.Code, w.Body.String())
				}
				if _, err := os.Stat(output[0]); !os.IsNotExist(err) {
					t.Fatal("finished staging retained")
				}
			}
			if f.connections.Load() != 1 || f.logins.Load() != 1 {
				t.Fatal("copy reauthenticated")
			}
		})
	}
}

func TestTerminalSFTP123RevocationCancellationAndExit(t *testing.T) {
	f := newTerminalSFTP123Fixture(t, true)
	for _, binding := range []editorBinding{{Kind: "local"}, {Kind: "sshConnection", ConnectionID: "guessed"}, {Kind: "terminal", SessionID: "foreign", Revision: f.revision}, {Kind: "terminal", SessionID: f.session.ID, Revision: f.revision - 1}, {Kind: "terminal", SessionID: f.session.ID, Revision: f.revision, ConnectionID: "mixed"}} {
		w := f.request("POST", "/api/explorer-transfer", explorerTransferRequest{Action: "direct", Binding: binding, Paths: []string{"/startup.cfg"}})
		if w.Code != 403 {
			t.Fatal("forged binding", binding, w.Code, w.Body.String())
		}
	}
	id := f.direct(t, "/startup.cfg")
	if w := f.action(id, "cancel"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := f.action(id, "arm"); w.Code != 404 {
		t.Fatal("cancel-before registration retained", w.Code)
	}
	id = f.direct(t, "/startup.cfg")
	f.start(t, id)
	if w := f.action(id, "cancel"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := f.action(id, "materialize"); w.Code != 409 {
		t.Fatal("cancelled drag materialized")
	}
	if w := f.action(id, "finish"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	grant, err := f.app.terminalSFTPGrant(f.launch.ViewID, f.binding())
	if err != nil {
		t.Fatal(err)
	}
	// Main closing must not revoke a still-owned detached session.
	f.app.views.mu.Lock()
	f.app.views.homePresent = false
	f.app.views.homeClosing = true
	f.app.views.mu.Unlock()
	if !grant.valid() {
		t.Fatal("main closure revoked independent owned session")
	}
	id = f.direct(t, "/startup.cfg")
	f.start(t, id)
	// Simulate a completed private stage under an active native reader. Ownership
	// revocation cancels the job, but must never delete this reader's bytes.
	stage := t.TempDir()
	path := filepath.Join(stage, "in-use")
	os.WriteFile(path, []byte("reader"), 0600)
	f.app.explorerTransfers.mu.Lock()
	j := f.app.explorerTransfers.jobs[id]
	j.stage = stage
	j.paths = []string{path}
	f.app.explorerTransfers.mu.Unlock()
	f.app.views.mu.Lock()
	owner := f.app.views.owners[f.session.ID]
	owner.Revision++
	f.app.views.owners[f.session.ID] = owner
	f.app.views.mu.Unlock()
	if grant.valid() {
		t.Fatal("stale lease survived ownership revision")
	}
	if _, err := grant.run(context.Background(), remoteBrowserRequest{Protocol: "sftp", Action: "list", Path: "."}); err == nil {
		t.Fatal("revoked read reached transport")
	}
	viewTestWait(t, func() bool {
		f.app.explorerTransfers.mu.Lock()
		defer f.app.explorerTransfers.mu.Unlock()
		return j.cancelRequested
	})
	if _, err := os.Stat(path); err != nil {
		t.Fatal("active native reader deleted on revocation", err)
	}
	if w := f.action(id, "materialize"); w.Code != 409 {
		t.Fatal("revoked materialization admitted")
	}
	f.exited <- nil
	viewTestWait(t, func() bool {
		f.app.explorerTransfers.mu.Lock()
		defer f.app.explorerTransfers.mu.Unlock()
		return f.app.explorerTransfers.jobs[id] == nil
	})
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("confirmed child exit retained staging")
	}
}

func TestTerminalSFTP123TransportLifetimeInvalidatesGrant(t *testing.T) {
	f := newTerminalSFTP123Fixture(t, true)
	grant, err := f.app.terminalSFTPGrant(f.launch.ViewID, f.binding())
	if err != nil {
		t.Fatal(err)
	}
	if err = f.app.terminals.Disconnect(f.session.ID); err != nil {
		t.Fatal(err)
	}
	if grant.valid() {
		t.Fatal("disconnected transport retained grant")
	}
	if _, err = grant.run(context.Background(), remoteBrowserRequest{Protocol: "sftp", Action: "list", Path: "."}); err == nil {
		t.Fatal("disconnected transport read")
	}
}

func TestTerminalSFTP123SingleViewHasNoNewFileCapability(t *testing.T) {
	f := newTerminalSFTP123Fixture(t, false)
	var launch terminalViewLaunch
	f.app.launchTerminalView = func(_ context.Context, l terminalViewLaunch) (<-chan error, error) { launch = l; return nil, nil }
	if _, err := f.app.views.create(f.session.ID, viewTestSnapshot(f.app, f.session)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.views.ready(launch.ViewID); err != nil {
		t.Fatal(err)
	}
	revision := f.app.views.state().Owners[f.session.ID].Revision
	for _, route := range []string{"/api/remote-browser", "/api/explorer-transfer", "/api/explorer-transfer/native"} {
		w := viewTestRequest(f.app, launch.ViewToken, "POST", route, map[string]any{"protocol": "ssh", "sessionId": f.session.ID, "action": "probe"}, &revision)
		if w.Code != 403 {
			t.Fatal("single view gained Files route", route, w.Code)
		}
	}
	if _, err := f.app.terminalSFTPGrant(launch.ViewID, editorBinding{Kind: "terminal", SessionID: f.session.ID, Revision: revision}); err == nil {
		t.Fatal("single-view acquired read/export grant")
	}
}

func TestTerminalSFTP123SameIDReplacementCannotRetargetGrant(t *testing.T) {
	f := newTerminalSFTP123ShellFixture(t)
	req := terminal.ConnectRequest{Type: "ssh", Host: "127.0.0.1", Port: f.credentials["port"].(int), Username: "tester", AuthMethod: "password", Password: "persistent-test-password", CredentialID: "synthetic-fixture", ExpectedHostFingerprint: f.credentials["fingerprint"].(string)}
	session, err := f.app.terminals.Connect(req)
	if err != nil {
		t.Fatal(err)
	}
	viewTestWait(t, func() bool { current, _ := f.app.terminalSession(session.ID); return current.State == "connected" })
	binding := editorBinding{Kind: "terminal", SessionID: session.ID}
	old, err := f.app.terminalSFTPGrant("", binding)
	if err != nil {
		t.Fatal(err)
	}
	f.app.terminals.SetCredentialResolver(func(_ context.Context, r terminal.ConnectRequest) (terminal.ConnectRequest, error) {
		r.Password = "persistent-test-password"
		return r, nil
	})
	// Deliberately retain the same owner/revision: the transport pin must reject
	// replacement even without relying on the HTTP reconnect revision increment.
	if _, err = f.app.terminals.Reconnect(context.Background(), session.ID); err != nil {
		t.Fatal(err)
	}
	viewTestWait(t, func() bool { current, _ := f.app.terminalSession(session.ID); return current.State == "connected" })
	if old.valid() {
		t.Fatal("old grant retargeted a replacement SSH client")
	}
	if _, err = old.run(context.Background(), remoteBrowserRequest{Protocol: "sftp", Action: "list", Path: "."}); err == nil {
		t.Fatal("old grant read replacement transport")
	}
	fresh, err := f.app.terminalSFTPGrant("", binding)
	if err != nil || !fresh.valid() {
		t.Fatal("fresh transport unavailable", err)
	}
	if _, err = fresh.run(context.Background(), remoteBrowserRequest{Protocol: "sftp", Action: "list", Path: "."}); err != nil {
		t.Fatal(err)
	}
}
