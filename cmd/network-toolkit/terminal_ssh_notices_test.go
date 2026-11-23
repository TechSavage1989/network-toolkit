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

func TestSSHNoticeProfilePersistenceAndNormalization(t *testing.T) {
	a := testApplication(t)
	a.dataPath = filepath.Join(filepath.Dir(a.dataPath), "Network_Toolkit.settings.json")
	profiles := []any{
		map[string]any{"id": "enabled", "name": "Enabled", "type": "ssh", "host": "notice.example", "username": "operator", "credentialId": "saved-reference", "sshAutoContinueNotices": true, "password": "notice-secret-canary"},
		map[string]any{"id": "disabled", "name": "Disabled", "type": "ssh", "sshAutoContinueNotices": false},
		map[string]any{"id": "existing", "name": "Existing", "type": "ssh"},
		map[string]any{"id": "string", "name": "Invalid string", "type": "ssh", "sshAutoContinueNotices": "true"},
		map[string]any{"id": "object", "name": "Invalid object", "type": "ssh", "sshAutoContinueNotices": map[string]any{"password": "notice-secret-canary"}},
		map[string]any{"id": "telnet", "name": "Telnet", "type": "telnet", "sshAutoContinueNotices": true},
		map[string]any{"id": "serial", "name": "Serial", "type": "serial", "sshAutoContinueNotices": true},
		map[string]any{"id": "shell", "name": "Shell", "type": "powershell", "sshAutoContinueNotices": true},
	}
	sftpRecent := []any{
		map[string]any{"id": "enabled", "name": "Enabled SFTP", "host": "notice.example", "port": 22, "username": "operator", "authMethod": "password", "sshAutoContinueNotices": true},
		map[string]any{"id": "invalid", "name": "Invalid SFTP", "host": "notice.example", "port": 22, "username": "operator", "authMethod": "password", "sshAutoContinueNotices": "true"},
	}
	body, err := json.Marshal(map[string]any{"terminalConnection": profiles[0], "terminalProfiles": profiles, "terminalRecent": profiles, "sftpRecent": sftpRecent})
	if err != nil {
		t.Fatal(err)
	}
	if response := requestApp(a, http.MethodPut, "/api/state", string(body), true); response.Code != http.StatusOK {
		t.Fatalf("save notice preference: %d %s", response.Code, response.Body.String())
	}
	check := func(raw []byte) {
		t.Helper()
		if strings.Contains(string(raw), "notice-secret-canary") {
			t.Fatal("a saved notice preference retained credential material")
		}
		var state struct {
			Connection map[string]any   `json:"terminalConnection"`
			Profiles   []map[string]any `json:"terminalProfiles"`
			Recent     []map[string]any `json:"terminalRecent"`
			SFTPRecent []map[string]any `json:"sftpRecent"`
		}
		if err := json.Unmarshal(raw, &state); err != nil {
			t.Fatal(err)
		}
		if state.Connection["sshAutoContinueNotices"] != true {
			t.Fatal("connection draft lost its explicit notice preference")
		}
		for _, saved := range [][]map[string]any{state.Profiles, state.Recent} {
			if len(saved) != len(profiles) {
				t.Fatal("saving the notice preference changed the saved/recent list")
			}
			for _, profile := range saved {
				enabled := profile["id"] == "enabled"
				if (profile["sshAutoContinueNotices"] == true) != enabled {
					t.Fatalf("notice opt-in changed for profile %v", profile["id"])
				}
				if profile["id"] == "disabled" && profile["sshAutoContinueNotices"] != false {
					t.Fatal("explicitly disabled notice preference was lost")
				}
				if enabled && (profile["host"] != "notice.example" || profile["username"] != "operator" || profile["credentialId"] != "saved-reference") {
					t.Fatal("notice preference changed the saved connection identity")
				}
			}
		}
		if len(state.SFTPRecent) != 2 || state.SFTPRecent[0]["sshAutoContinueNotices"] != true || state.SFTPRecent[1]["sshAutoContinueNotices"] == true {
			t.Fatal("SFTP recent history did not preserve only a boolean notice opt-in")
		}
	}
	raw, err := os.ReadFile(a.dataPath)
	if err != nil {
		t.Fatal(err)
	}
	check(raw)
	check(standaloneTerminalState(t, filepath.Dir(a.dataPath)))
	if len(a.terminals.List()) != 0 {
		t.Fatal("saving the notice preference opened a connection")
	}
}

func TestSSHNoticeIndependentWindowAndChildArguments(t *testing.T) {
	a := testApplication(t)
	a.windowMode = "terminal"
	for _, bad := range []string{`"true"`, `1`, `{}`, `[]`} {
		body := `{"config":{"type":"ssh","host":"notice.example","sshAutoContinueNotices":` + bad + `}}`
		if response := requestApp(a, http.MethodPost, "/api/terminal-window", body, true); response.Code != http.StatusBadRequest {
			t.Fatalf("invalid notice preset accepted: %d", response.Code)
		}
	}
	if response := requestApp(a, http.MethodPost, "/api/terminal-window", `{"config":{"type":"ssh","host":"notice.example","sshAutoContinueNotices":true}}`, true); response.Code != http.StatusOK {
		t.Fatalf("window notice configuration failed: %d %s", response.Code, response.Body.String())
	}
	batch := a.windowInbox.batch(0)
	if len(batch.Requests) != 1 || !batch.Requests[0].Config.SSHAutoContinueNotices || len(a.terminals.List()) != 0 {
		t.Fatal("window launch lost the notice opt-in or opened a connection")
	}
	preset := batch.Requests[0].Config
	args, err := terminalChildArguments(t.TempDir(), &preset, false)
	if err != nil || len(args) != 5 {
		t.Fatalf("create separate-window arguments: %v %v", args, err)
	}
	decoded, err := decodeTerminalWindowPreset(args[4])
	if err != nil || !decoded.SSHAutoContinueNotices || decoded.Host != "notice.example" {
		t.Fatal("separate-process configuration lost the notice opt-in or changed its destination")
	}
	for _, body := range []string{`{"type":"ssh"}`, `{"type":"ssh","sshAutoContinueNotices":false}`, `{"type":"telnet","sshAutoContinueNotices":true}`, `{"type":"serial","sshAutoContinueNotices":true}`, `{"type":"cmd","sshAutoContinueNotices":true}`, `{"type":"powershell","sshAutoContinueNotices":true}`} {
		decoded, err = decodeTerminalWindowPreset(base64.RawURLEncoding.EncodeToString([]byte(body)))
		if err != nil || decoded.SSHAutoContinueNotices {
			t.Fatalf("default, disabled, or non-SSH window enabled automatic notices: %s %v", body, err)
		}
	}
}

func TestSSHNoticeWorkspaceDraftRoundTrip(t *testing.T) {
	a := testApplication(t)
	initial := terminalWorkspaceSnapshot{Display: map[string]any{"terminalConnection": map[string]any{"type": "ssh", "host": "notice.example", "sshAutoContinueNotices": true, "password": "notice-secret-canary"}}}
	info, launch := workspaceTestCreate(t, a, initial)
	workspaceTestDecode(t, viewTestRequest(a, launch.ViewToken, http.MethodPost, "/api/terminal-views/current/ready", map[string]any{}, nil), http.StatusOK)
	current := workspaceTestDecode(t, viewTestRequest(a, launch.ViewToken, http.MethodGet, "/api/terminal-views/current", nil, nil), http.StatusOK)
	workspaceTestDecode(t, viewTestRequest(a, launch.ViewToken, http.MethodPost, "/api/terminal-views/current/reattach", map[string]any{"workspace": current.Workspace}, nil), http.StatusOK)
	returned := workspaceTestDecode(t, viewTestRequest(a, a.token, http.MethodGet, "/api/terminal-views/"+info.ID+"/snapshot", nil, nil), http.StatusOK)
	for _, snapshot := range []terminalWorkspaceSnapshot{current.Workspace, returned.Workspace} {
		config, ok := snapshot.Display["terminalConnection"].(map[string]any)
		if !ok || config["sshAutoContinueNotices"] != true || config["host"] != "notice.example" {
			t.Fatal("detached workspace lost the notice preference or changed its connection")
		}
		if _, exists := config["password"]; exists {
			t.Fatal("detached workspace retained a password with the notice preference")
		}
	}
	if len(a.terminals.List()) != 0 || string(a.state) != "{}" {
		t.Fatal("workspace draft transfer connected or overwrote home preferences")
	}
}

func TestSSHNoticeAPIValidationAndExistingConnectionBoundary(t *testing.T) {
	a := testApplication(t)
	for _, bad := range []string{`"true"`, `1`, `{}`, `[]`} {
		for path, prefix := range map[string]string{
			"/api/terminals":      `{"type":"ssh","host":"127.0.0.1","username":"operator",`,
			"/api/remote-browser": `{"protocol":"sftp","action":"connect","host":"127.0.0.1","username":"operator",`,
		} {
			response := requestApp(a, http.MethodPost, path, prefix+`"sshAutoContinueNotices":`+bad+`}`, true)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid notice type reached %s: %d", path, response.Code)
			}
		}
	}
	for _, id := range []string{"connectionId", "sessionId"} {
		body := `{"protocol":"sftp","action":"list","` + id + `":"existing","sshAutoContinueNotices":true}`
		if response := requestApp(a, http.MethodPost, "/api/remote-browser", body, true); response.Code != http.StatusBadRequest {
			t.Fatalf("existing %s accepted a fresh authentication preference: %d", id, response.Code)
		}
	}
	if len(a.terminals.List()) != 0 || len(a.browserConnectionInfos()) != 0 {
		t.Fatal("invalid notice configuration created a connection")
	}
}
