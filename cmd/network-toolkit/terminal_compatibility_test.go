package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSSHCompatibilityProfilePersistenceAndNormalization(t *testing.T) {
	a := testApplication(t)
	a.dataPath = filepath.Join(filepath.Dir(a.dataPath), "Network_Toolkit.settings.json")
	body := `{"terminalConnection":{"type":"ssh","host":"192.0.2.8","sshCompatibility":"legacy","password":"credential-canary"},"terminalProfiles":[{"id":"legacy","name":"Older switch","type":"ssh","sshCompatibility":"legacy","privateKey":"credential-canary"},{"id":"existing","name":"Existing profile","type":"ssh"},{"id":"unknown","name":"Unrecognized policy","type":"ssh","sshCompatibility":"accept-everything"},{"id":"nested","name":"Invalid policy type","type":"ssh","sshCompatibility":{"password":"credential-canary"}},{"id":"serial","name":"Console","type":"serial","sshCompatibility":"legacy"}]}`
	if r := requestApp(a, "PUT", "/api/state", body, true); r.Code != http.StatusOK {
		t.Fatalf("Save compatibility profiles: %d %s", r.Code, r.Body.String())
	}
	raw, err := os.ReadFile(a.dataPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "credential-canary") || strings.Contains(string(raw), "accept-everything") {
		t.Fatal("Unrecognized policy or credentials reached settings")
	}
	var state struct {
		Connection map[string]any   `json:"terminalConnection"`
		Profiles   []map[string]any `json:"terminalProfiles"`
	}
	if err = json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	if state.Connection["sshCompatibility"] != "legacy" || state.Profiles[0]["sshCompatibility"] != "legacy" {
		t.Fatal("Explicit SSH compatibility was lost from the saved connection/profile")
	}
	for _, profile := range state.Profiles[1:] {
		if profile["sshCompatibility"] != "modern" {
			t.Fatalf("Existing/invalid/non-SSH profile changed policy: %+v", profile)
		}
	}
	if len(a.terminals.List()) != 0 {
		t.Fatal("Saving a compatibility profile opened a connection")
	}
	dest := terminalWorkspace(filepath.Dir(a.dataPath))
	if err = os.MkdirAll(dest, 0700); err != nil {
		t.Fatal(err)
	}
	if err = seedTerminalWorkspace(filepath.Dir(a.dataPath), dest); err != nil {
		t.Fatal(err)
	}
	seeded, err := os.ReadFile(filepath.Join(dest, "Network_Toolkit.settings.json"))
	if err != nil || !strings.Contains(string(seeded), `"sshCompatibility": "legacy"`) || strings.Contains(string(seeded), "credential-canary") {
		t.Fatalf("Independent workspace did not preserve only safe compatibility settings: %s %v", seeded, err)
	}
}

func TestSSHCompatibilityIndependentWindowRequestAndChildArguments(t *testing.T) {
	a := testApplication(t)
	a.windowMode = "terminal"
	for _, bad := range []string{`"automatic"`, `true`, `{}`} {
		body := `{"config":{"type":"ssh","host":"192.0.2.8","sshCompatibility":` + bad + `}}`
		if r := requestApp(a, "POST", "/api/terminal-window", body, true); r.Code != http.StatusBadRequest {
			t.Fatalf("Invalid compatibility preset accepted: %d %s", r.Code, r.Body.String())
		}
	}
	if r := requestApp(a, "POST", "/api/terminal-window", `{"config":{"type":"ssh","host":"192.0.2.8","sshCompatibility":"legacy"}}`, true); r.Code != http.StatusOK {
		t.Fatalf("Legacy window configuration failed: %d %s", r.Code, r.Body.String())
	}
	batch := a.windowInbox.batch(0)
	if len(batch.Requests) != 1 || batch.Requests[0].Config.SSHCompatibility != "legacy" || len(a.terminals.List()) != 0 {
		t.Fatal("Window request lost explicit policy or connected automatically")
	}
	preset := batch.Requests[0].Config
	args, err := terminalChildArguments(t.TempDir(), &preset, false)
	if err != nil || len(args) != 5 {
		t.Fatalf("Create independent-window arguments: %v %v", args, err)
	}
	decoded, err := decodeTerminalWindowPreset(args[4])
	if err != nil || decoded.SSHCompatibility != "legacy" {
		t.Fatal("Independent-process configuration did not retain SSH compatibility")
	}
	for _, body := range []string{`{"type":"ssh"}`, `{"type":"telnet","sshCompatibility":"legacy"}`, `{"type":"serial","sshCompatibility":"legacy"}`} {
		decoded, err = decodeTerminalWindowPreset(base64.RawURLEncoding.EncodeToString([]byte(body)))
		if err != nil || decoded.SSHCompatibility != "modern" {
			t.Fatalf("Default/non-SSH compatibility was not modern: %s %v", body, err)
		}
	}
}

func TestSSHCompatibilityAPIMethodAndTypeValidation(t *testing.T) {
	a := testApplication(t)
	for _, value := range []string{`"automatic"`, `true`, `{}`} {
		r := requestApp(a, "POST", "/api/terminals", `{"type":"ssh","host":"127.0.0.1","username":"operator","sshCompatibility":`+value+`}`, true)
		if r.Code != http.StatusBadRequest {
			t.Fatalf("Invalid compatibility value reached the SSH transport: %d %s", r.Code, r.Body.String())
		}
	}
	if len(a.terminals.List()) != 0 {
		t.Fatal("Invalid SSH policy created a session")
	}
}
