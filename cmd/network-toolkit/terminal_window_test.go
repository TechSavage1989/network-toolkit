package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTerminalWindowRequestGuardsAndNoAutomaticConnection(t *testing.T) {
	a := testApplication(t)
	a.windowMode = "terminal"
	for _, path := range []string{"/api/terminal-window", "/api/terminal-window/requests"} {
		if r := requestApp(a, "POST", path, `{}`, false); r.Code != 401 {
			t.Fatalf("Unauthenticated window route accepted: %d", r.Code)
		}
	}
	for _, body := range []string{`null`, `[]`, `{} {}`, `{"config":{"type":"shell"}}`, `{"config":{"password":"must-not-cross-process"}}`, `{"config":{"privateKey":"secret"}}`, `{"config":{"passphrase":"secret"}}`, `{"config":{"type":"ssh","port":65536}}`, `{"config":{"host":"router\ncommand"}}`, `{"config":{"type":"serial","baudRate":-1}}`, `{"config":{"host":"` + strings.Repeat("a", 9000) + `"}}`} {
		r := requestApp(a, "POST", "/api/terminal-window", body, true)
		if r.Code != http.StatusBadRequest {
			t.Fatalf("Invalid window request accepted (%d): %.100s", r.Code, body)
		}
	}
	for _, query := range []string{"-1", "x", "18446744073709551616"} {
		if r := requestApp(a, "GET", "/api/terminal-window/requests?after="+query, "", true); r.Code != 400 {
			t.Fatalf("Invalid request cursor accepted: %d", r.Code)
		}
	}
	r := requestApp(a, "POST", "/api/terminal-window", `{"config":{"type":"telnet","host":"192.0.2.10"}}`, true)
	if r.Code != 200 {
		t.Fatalf("Window request failed: %d %s", r.Code, r.Body.String())
	}
	batch := a.windowInbox.batch(0)
	if len(batch.Requests) != 1 || batch.Requests[0].Config.Port != 23 || batch.Requests[0].Config.Host != "192.0.2.10" {
		t.Fatalf("Wrong window preset: %+v", batch)
	}
	if len(a.terminals.List()) != 0 || len(a.jobs) != 0 {
		t.Fatal("Preparing a window opened a connection or diagnostic")
	}
	select {
	case <-a.windowActivate:
	default:
		t.Fatal("Native activation was not signaled")
	}
	if r := requestApp(a, "POST", "/api/terminal-window", `{}`, true); r.Code != 200 || len(a.windowInbox.batch(0).Requests) != 1 {
		t.Fatal("Focus-only request changed the connection inbox")
	}
	if r := requestApp(a, "GET", "/api/terminal-window", ``, true); r.Code != 405 {
		t.Fatal("Window launch must be an explicit POST")
	}
	a.stop()
	if r := requestApp(a, "POST", "/api/terminal-window", `{}`, true); r.Code != 503 {
		t.Fatal("Closing window accepted a launch request")
	}
}

func TestTerminalWindowLauncherDelegationAndUnavailableMode(t *testing.T) {
	a := testApplication(t)
	if r := requestApp(a, "POST", "/api/terminal-window", `{}`, true); r.Code != 501 {
		t.Fatal("Unsupported host claimed a native window")
	}
	if r := requestApp(a, "GET", "/api/terminal-window/requests", "", true); r.Code != 409 {
		t.Fatal("Main window must not expose an independent window inbox")
	}
	called := 0
	a.openTerminalWindow = func(ctx context.Context, preset *terminalWindowPreset) (bool, error) {
		called++
		if _, ok := ctx.Deadline(); !ok || preset == nil || preset.Host != "2001:db8::1" || preset.Port != 22 {
			t.Fatal("Window launcher received the wrong bounded request")
		}
		return true, nil
	}
	r := requestApp(a, "POST", "/api/terminal-window", `{"config":{"type":"ssh","host":"2001:db8::1"}}`, true)
	if r.Code != 200 || called != 1 || !strings.Contains(r.Body.String(), `"reused":true`) {
		t.Fatalf("Launch delegation failed: %s", r.Body.String())
	}
	a.openTerminalWindow = func(context.Context, *terminalWindowPreset) (bool, error) {
		return false, errors.New("fixture launch failure")
	}
	if r := requestApp(a, "POST", "/api/terminal-window", `{}`, true); r.Code != 503 || !strings.Contains(r.Body.String(), "fixture launch failure") {
		t.Fatal("Failed process launch claimed success")
	}
}

func TestTerminalWindowRejectsActivationBeforeShutdownGraceEnds(t *testing.T) {
	a := testApplication(t)
	a.windowMode = "terminal"
	if r := requestApp(a, "GET", "/api/shutdown", "", true); r.Code != http.StatusMethodNotAllowed || a.shutdownRequested.Load() {
		t.Fatal("A non-POST shutdown request changed closing state")
	}
	if r := requestApp(a, "POST", "/api/shutdown", `{}`, true); r.Code != http.StatusOK || !a.shutdownRequested.Load() {
		t.Fatal("Shutdown was acknowledged before recording closing state")
	}
	if r := requestApp(a, "POST", "/api/terminal-window", `{"config":{"type":"ssh","host":"192.0.2.9"}}`, true); r.Code != http.StatusServiceUnavailable {
		t.Fatalf("Shutdown grace accepted an activation: %d %s", r.Code, r.Body.String())
	}
	if len(a.windowInbox.batch(0).Requests) != 0 {
		t.Fatal("A closing host queued a connection request")
	}
	select {
	case <-a.windowActivate:
		t.Fatal("A closing host signaled native activation")
	default:
	}
}

func TestTerminalWindowInboxBoundedHistoryAndSnapshots(t *testing.T) {
	var inbox terminalWindowInbox
	for i := 1; i <= 20; i++ {
		p := &terminalWindowPreset{Type: "ssh", Host: fmt.Sprint("host-", i)}
		inbox.add(p)
		p.Host = "mutated"
	}
	batch := inbox.batch(0)
	if len(batch.Requests) != 16 || batch.Latest != 20 || batch.Oldest != 5 || !batch.Lost || batch.Requests[0].Config.Host != "host-5" {
		t.Fatalf("Incorrect bounded inbox: %+v", batch)
	}
	batch.Requests[0].Config.Host = "mutated again"
	if inbox.batch(0).Requests[0].Config.Host != "host-5" {
		t.Fatal("Inbox snapshot aliases retained state")
	}
	if batch = inbox.batch(18); len(batch.Requests) != 2 || batch.Lost || batch.Reset {
		t.Fatal("Request cursor did not return the remaining requests")
	}
	if batch = inbox.batch(100); !batch.Reset || len(batch.Requests) != 16 {
		t.Fatal("Future request cursor did not reset")
	}
}

// A new Terminal workspace starts with the main window's arrangement of the
// terminal page and nothing else: connections, settings, SSH host trust and
// macros are shared by every app of the data folder, not copied.
func TestTerminalWorkspaceSeedsOnlyWindowStateAndDoesNotOverwrite(t *testing.T) {
	root := t.TempDir()
	dest := terminalWorkspace(root)
	if err := os.MkdirAll(dest, 0700); err != nil {
		t.Fatal(err)
	}
	parent := `{"theme":"light","terminalFontSize":16,"terminalHighlight":false,"terminalSidebarWidth":372,"terminalSidebarCollapsed":true,"terminalPromptNames":[{"type":"ssh","host":"192.0.2.1","port":22,"username":"operator","name":"EDGE-SW01","password":"private-prompt-marker"}],"activePage":"discover","reportNotes":"private-report-marker","terminalConnection":{"type":"ssh","host":"192.0.2.1","password":"private-password-marker"},"terminalProfiles":[{"id":"one","name":"Lab","host":"192.0.2.1","privateKey":"private-key-marker","passphrase":"private-passphrase-marker"}]}`
	if err := os.WriteFile(filepath.Join(root, "Network_Toolkit.settings.json"), []byte(parent), 0600); err != nil {
		t.Fatal(err)
	}
	trust := []byte(`{"version":1,"hosts":[]}`)
	if err := os.WriteFile(filepath.Join(root, "Network_Toolkit.ssh_known_hosts.json"), trust, 0600); err != nil {
		t.Fatal(err)
	}
	if err := seedTerminalWorkspace(root, dest); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(dest, "Network_Toolkit.settings.json")
	b, err := os.ReadFile(settingsPath)
	if err != nil || strings.Contains(string(b), "private-") || !strings.Contains(string(b), `"activePage": "terminal"`) || !strings.Contains(string(b), `"terminalSidebarWidth": 372`) || !strings.Contains(string(b), `"terminalSidebarCollapsed": true`) || !strings.Contains(string(b), `"host": "192.0.2.1"`) {
		t.Fatalf("Unsafe or incomplete seeded window state: %s %v", b, err)
	}
	var seeded map[string]any
	if err := json.Unmarshal(b, &seeded); err != nil {
		t.Fatal(err)
	}
	for _, key := range sharedDataKeys {
		if _, copied := seeded[key.Name]; copied {
			t.Fatalf("shared data (%s) was copied into the Terminal workspace: %s", key.Name, b)
		}
	}
	for _, name := range []string{"Network_Toolkit.ssh_known_hosts.json", terminalMacroLibraryFilename, credentialVaultV2Filename} {
		if _, err := os.Stat(filepath.Join(dest, name)); !os.IsNotExist(err) {
			t.Fatalf("%s was created in the Terminal workspace: %v", name, err)
		}
	}
	// The window still starts with the shared connections and settings.
	loaded := string(standaloneTerminalState(t, root))
	if strings.Contains(loaded, "private-") || !strings.Contains(loaded, "Lab") || !strings.Contains(loaded, `"name": "EDGE-SW01"`) || !strings.Contains(loaded, `"theme": "light"`) || !strings.Contains(loaded, `"terminalFontSize": 16`) || !strings.Contains(loaded, `"terminalSidebarWidth": 372`) {
		t.Fatalf("the Terminal window does not start with the shared data: %s", loaded)
	}
	child := []byte(`{"activePage":"terminal","terminalSidebarWidth":300}`)
	if err := os.WriteFile(settingsPath, child, 0600); err != nil {
		t.Fatal(err)
	}
	if err := seedTerminalWorkspace(root, dest); err != nil {
		t.Fatal(err)
	}
	if b, _ = os.ReadFile(settingsPath); string(b) != string(child) {
		t.Fatal("Later launch overwrote the window's own state")
	}
	if b, _ = os.ReadFile(filepath.Join(root, "Network_Toolkit.settings.json")); string(b) != parent {
		t.Fatal("Seeding modified main-window preferences")
	}
	if b, _ = os.ReadFile(filepath.Join(root, "Network_Toolkit.ssh_known_hosts.json")); string(b) != string(trust) {
		t.Fatal("Seeding modified the data folder's SSH host trust")
	}
}

// SSH host trust is one file per data folder. A trust file an earlier version
// left in the Terminal workspace is never created, replaced or removed by a
// launch.
func TestTerminalWorkspaceTrustFileOfEarlierVersionIsLeftAlone(t *testing.T) {
	root := t.TempDir()
	dest := terminalWorkspace(root)
	if err := os.MkdirAll(dest, 0700); err != nil {
		t.Fatal(err)
	}
	if err := seedTerminalWorkspace(root, dest); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dest, "Network_Toolkit.ssh_known_hosts.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("a trust file was created in the Terminal workspace: %v", err)
	}
	previous := []byte("{\"version\":1,\"hosts\":[]}\n")
	if err := os.WriteFile(path, previous, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Network_Toolkit.ssh_known_hosts.json"), []byte("{\n  \"version\": 1,\n  \"hosts\": []\n}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := seedTerminalWorkspace(root, dest); err != nil {
		t.Fatal(err)
	}
	if current, err := os.ReadFile(path); err != nil || string(current) != string(previous) {
		t.Fatal("A later launch changed the trust file an earlier version left in the workspace")
	}
}

func TestTerminalWindowRegistrationAndAuthenticatedLoopbackActivation(t *testing.T) {
	a := testApplication(t)
	a.windowMode = "terminal"
	server := httptest.NewServer(a)
	defer server.Close()
	a.host = strings.TrimPrefix(server.URL, "http://")
	dir := filepath.Dir(a.dataPath)
	unregister, err := a.registerTerminalWindow(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer unregister()
	preset := &terminalWindowPreset{Type: "ssh", Host: "192.0.2.4", Username: "operator"}
	if err = preset.normalize(); err != nil {
		t.Fatal(err)
	}
	if err = activateTerminalWindow(context.Background(), dir, preset); err != nil {
		t.Fatal(err)
	}
	if batch := a.windowInbox.batch(0); len(batch.Requests) != 1 || batch.Requests[0].Config.Username != "operator" {
		t.Fatalf("Local activation lost the preset: %+v", batch)
	}
	if len(a.terminals.List()) != 0 {
		t.Fatal("IPC activation unexpectedly connected")
	}
	record, err := readTerminalWindowInstance(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, badHost := range []string{"example.com:80", "127.0.0.2:80", "127.0.0.1:080", "127.0.0.1:65536", "127.0.0.1:80/path", "[::1]:80"} {
		bad := record
		bad.Host = badHost
		b, _ := json.Marshal(bad)
		if err = writeAtomic(filepath.Join(dir, terminalInstanceFilename), b); err != nil {
			t.Fatal(err)
		}
		if _, err = readTerminalWindowInstance(dir); err == nil {
			t.Fatalf("Untrusted instance endpoint accepted: %s", badHost)
		}
	}
	b, _ := json.Marshal(record)
	if err = writeAtomic(filepath.Join(dir, terminalInstanceFilename), b); err != nil {
		t.Fatal(err)
	}
	unregister()
	if _, err = os.Stat(filepath.Join(dir, terminalInstanceFilename)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Own instance registration was not removed")
	}
}

func TestTerminalChildArgumentsAreFixedAndContainOnlySettings(t *testing.T) {
	preset := &terminalWindowPreset{Type: "ssh", Host: "router.example", Username: "operator"}
	args, err := terminalChildArguments(`C:\Portable Tools\Network Toolkit`, preset, false)
	if err != nil || len(args) != 5 || args[0] != "--terminal" || args[1] != "--data-dir" || args[3] != "--terminal-preset" {
		t.Fatalf("Unexpected process arguments: %v %v", args, err)
	}
	got, err := decodeTerminalWindowPreset(args[4])
	if err != nil || got.Host != preset.Host || got.Username != preset.Username || got.Port != 22 {
		t.Fatal("Child settings did not round-trip")
	}
	for _, bad := range []string{`{"password":"secret"}`, `{"privateKey":"secret"}`, `{"type":"ssh"} {}`, `{"type":"serial","flowControl":"bad"}`} {
		if _, err = decodeTerminalWindowPreset(base64.RawURLEncoding.EncodeToString([]byte(bad))); err == nil {
			t.Fatalf("Invalid startup preset accepted: %s", bad)
		}
	}
}

func TestListPreferencesPersistOnlyBoundedControls(t *testing.T) {
	a := testApplication(t)
	body := `{"terminalHighlight":false,"listPreferences":{"scan":{"query":"switch","sort":"address","direction":"desc","status":"responded","respondedOnly":true,"rows":["secret-row-marker"],"password":"secret-password-marker"},"__proto__":{"query":"bad"},"bad":{"query":{"output":"secret-output-marker"},"direction":"invalid"}}}`
	if r := requestApp(a, "PUT", "/api/state", body, true); r.Code != 200 {
		t.Fatalf("List preferences did not save: %s", r.Body.String())
	}
	b, err := os.ReadFile(a.dataPath)
	if err != nil || strings.Contains(string(b), "secret-") || strings.Contains(string(b), "__proto__") || !strings.Contains(string(b), "switch") {
		t.Fatalf("Unsafe list preference storage: %s %v", b, err)
	}
	views := map[string]any{}
	for i := 0; i < 200; i++ {
		views[fmt.Sprint("view-", i)] = map[string]any{"query": "x"}
	}
	state := map[string]any{"listPreferences": views}
	sanitizeListPreferences(state)
	if len(state["listPreferences"].(map[string]any)) != 160 {
		t.Fatal("Saved list views are not bounded")
	}
}
