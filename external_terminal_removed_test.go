package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalTerminalRoutesRemoved(t *testing.T) {
	a := testApplication(t)
	for _, path := range []string{"/api/external-terminal", "/api/external-terminal/open", "/api/external-terminal/anything"} {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			if response := requestApp(a, method, path, `{}`, false); response.Code != http.StatusUnauthorized {
				t.Fatalf("removed route bypassed session authentication: %s %s returned %d", method, path, response.Code)
			}
			if response := requestApp(a, method, path, `{}`, true); response.Code != http.StatusNotFound {
				t.Fatalf("removed route is still available: %s %s returned %d", method, path, response.Code)
			}
		}
	}
	// These requests used to detect and launch third-party terminal programs.
	// Production dispatch must reject even well-formed old envelopes, including
	// explicit paths, before any program lookup or native launcher is reachable.
	for _, app := range []string{"putty", "windows-terminal", "openssh"} {
		body := `{"app":"` + app + `","executable":"C:\\Fixture\\terminal.exe","config":{"type":"ssh","host":"192.0.2.10","port":22,"username":"operator"}}`
		if response := requestApp(a, http.MethodPost, "/api/external-terminal/open", body, true); response.Code != http.StatusNotFound {
			t.Fatalf("stale %s launch request was accepted: %d %s", app, response.Code, response.Body.String())
		}
	}
}

const legacyTerminalSettings = `{"theme":"dark","terminalLaunch":{"target":"external","app":"putty","executable":"C:\\Fixture\\putty.exe","password":"forbidden-secret","args":["unsafe"]},"terminalConnection":{"type":"ssh","host":"192.0.2.9","username":"operator","password":"connection-secret"},"terminalProfiles":[{"id":"ssh-profile","name":"Lab","type":"ssh","host":"192.0.2.1","username":"operator","sshCompatibility":"legacy","terminalLaunch":{"target":"external","app":"openssh","executable":"C:\\Fixture\\ssh.exe","password":"forbidden-secret"}},{"id":"serial-profile","name":"Console","type":"serial","serialPort":"COM12","baudRate":115200,"dataBits":8,"stopBits":"1","parity":"none","flowControl":"none","terminalLaunch":{"target":"external","app":"putty"}}],"terminalLayout":{"kind":"grid","columnRatio":9,"rowRatio":-1,"sessionIds":["live-secret"]}}`

func assertBuiltInTerminalSettings(t *testing.T, raw []byte) {
	t.Helper()
	for _, forbidden := range []string{"terminalLaunch", "putty", "openssh", "windows-terminal", "executable", "secret", "unsafe"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("removed launch preferences or sensitive data remain: %s", forbidden)
		}
	}
	var state map[string]any
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	profiles, ok := state["terminalProfiles"].([]any)
	if !ok || len(profiles) != 2 {
		t.Fatal("safe saved connections were lost during migration")
	}
	ssh := profiles[0].(map[string]any)
	serial := profiles[1].(map[string]any)
	if ssh["name"] != "Lab" || ssh["host"] != "192.0.2.1" || ssh["sshCompatibility"] != "legacy" || serial["serialPort"] != "COM12" || serial["baudRate"] != float64(115200) {
		t.Fatal("built-in connection settings changed during migration")
	}
	if layout := state["terminalLayout"].(map[string]any); layout["kind"] != "grid" || layout["columnRatio"] != 0.8 || layout["rowRatio"] != 0.2 {
		t.Fatal("safe split settings were not preserved and bounded")
	}
}

func TestRemovedExternalTerminalPreferencesOnReadWriteAndWorkspaceSeed(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "Network_Toolkit.settings.json")
	if err := os.WriteFile(settings, []byte(legacyTerminalSettings), 0600); err != nil {
		t.Fatal(err)
	}
	a := newApplication(context.Background(), settings)
	a.host = "127.0.0.1:4567"
	t.Cleanup(a.stop)
	if response := requestApp(a, http.MethodGet, "/api/state", "", true); response.Code != http.StatusOK {
		t.Fatalf("read old settings: %d", response.Code)
	} else {
		assertBuiltInTerminalSettings(t, response.Body.Bytes())
	}
	if response := requestApp(a, http.MethodPut, "/api/state", legacyTerminalSettings, true); response.Code != http.StatusOK {
		t.Fatalf("save old settings: %d %s", response.Code, response.Body.String())
	}
	stored, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	assertBuiltInTerminalSettings(t, stored)
	// Seed from an old main-workspace file so the independent native terminal
	// window receives the same safe transport preferences without app launchers.
	if err = os.WriteFile(settings, []byte(legacyTerminalSettings), 0600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "terminal-fixture")
	if err = os.Mkdir(dest, 0700); err != nil {
		t.Fatal(err)
	}
	if err = seedTerminalWorkspace(dir, dest); err != nil {
		t.Fatal(err)
	}
	seed, err := os.ReadFile(filepath.Join(dest, "Network_Toolkit.settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	assertBuiltInTerminalSettings(t, seed)
}
