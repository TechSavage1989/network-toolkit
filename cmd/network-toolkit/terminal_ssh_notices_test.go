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
		map[string]any{"id": "existing", "name": "Existing SFTP", "host": "notice.example", "port": 22, "username": "operator", "authMethod": "password"},
		map[string]any{"id": "disabled", "name": "Disabled SFTP", "host": "notice.example", "port": 22, "username": "operator", "authMethod": "password", "sshAutoContinueNotices": false},
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
				enabled := profile["id"] == "enabled" || profile["id"] == "existing"
				if (profile["sshAutoContinueNotices"] == true) != enabled {
					t.Fatalf("notice opt-in changed for profile %v", profile["id"])
				}
				if (profile["id"] == "disabled" || profile["id"] == "string" || profile["id"] == "object") && profile["sshAutoContinueNotices"] != false {
					t.Fatal("disabled or malformed notice preference was omitted and could adopt the default later")
				}
				if profile["id"] == "enabled" && (profile["host"] != "notice.example" || profile["username"] != "operator" || profile["credentialId"] != "saved-reference") {
					t.Fatal("notice preference changed the saved connection identity")
				}
			}
		}
		if len(state.SFTPRecent) != len(sftpRecent) {
			t.Fatal("SFTP recent history changed the saved list")
		}
		for _, profile := range state.SFTPRecent {
			want := profile["id"] == "enabled" || profile["id"] == "existing"
			if profile["sshAutoContinueNotices"] != want {
				t.Fatal("SFTP recent history lost the default or an explicit/malformed opt-out")
			}
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
	for _, bad := range []string{`"true"`, `1`, `{}`, `[]`, `null`} {
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
	for _, test := range []struct {
		body string
		want bool
	}{
		{`{"type":"ssh"}`, true},
		{`{"type":" SSH "}`, true},
		{`{}`, true},
		{`{"type":"ssh","sshAutoContinueNotices":false}`, false},
		{`{"type":"ssh","SSHAutoContinueNotices":false}`, false},
		{`{"type":"ssh","SSHAutoContinueNotices":true,"sshAutoContinueNotices":false}`, false},
		{`{"type":"telnet","sshAutoContinueNotices":true}`, false},
		{`{"type":"serial","sshAutoContinueNotices":true}`, false},
		{`{"type":"cmd","sshAutoContinueNotices":true}`, false},
		{`{"type":"powershell","sshAutoContinueNotices":true}`, false},
	} {
		decoded, err = decodeTerminalWindowPreset(base64.RawURLEncoding.EncodeToString([]byte(test.body)))
		if err != nil || decoded.SSHAutoContinueNotices != test.want {
			t.Fatalf("window notice default changed: %s %v", test.body, err)
		}
		args, err = terminalChildArguments(t.TempDir(), decoded, false)
		if err != nil {
			t.Fatal(err)
		}
		restored, err := decodeTerminalWindowPreset(args[4])
		if err != nil || restored.SSHAutoContinueNotices != test.want {
			t.Fatal("child argument round trip changed the notice setting")
		}
	}
	for _, body := range []string{`{"type":"ssh","unknown":true}`, `{"type":"ssh","password":"secret-canary"}`, `{"type":"ssh","sshAutoContinueNotices":null}`} {
		if _, err = decodeTerminalWindowPreset(base64.RawURLEncoding.EncodeToString([]byte(body))); err == nil {
			t.Fatal("window notice decoding bypassed unknown field or boolean validation")
		}
		if response := requestApp(a, http.MethodPost, "/api/terminal-window", `{"config":`+body+`}`, true); response.Code != http.StatusBadRequest {
			t.Fatal("nested window API decoding bypassed unknown field or boolean validation")
		}
	}
}

func TestSSHNoticeWorkspaceDraftRoundTrip(t *testing.T) {
	for _, mode := range []string{"missing", "enabled", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			a := testApplication(t)
			config := map[string]any{"type": "ssh", "host": "notice.example", "password": "notice-secret-canary"}
			if mode != "missing" {
				config["sshAutoContinueNotices"] = mode == "enabled"
			}
			want := mode != "disabled"
			initial := terminalWorkspaceSnapshot{Display: map[string]any{"terminalConnection": config}}
			info, launch := workspaceTestCreate(t, a, initial)
			workspaceTestDecode(t, viewTestRequest(a, launch.ViewToken, http.MethodPost, "/api/terminal-views/current/ready", map[string]any{}, nil), http.StatusOK)
			current := workspaceTestDecode(t, viewTestRequest(a, launch.ViewToken, http.MethodGet, "/api/terminal-views/current", nil, nil), http.StatusOK)
			workspaceTestDecode(t, viewTestRequest(a, launch.ViewToken, http.MethodPost, "/api/terminal-views/current/reattach", map[string]any{"workspace": current.Workspace}, nil), http.StatusOK)
			returned := workspaceTestDecode(t, viewTestRequest(a, a.token, http.MethodGet, "/api/terminal-views/"+info.ID+"/snapshot", nil, nil), http.StatusOK)
			for _, snapshot := range []terminalWorkspaceSnapshot{current.Workspace, returned.Workspace} {
				config, ok := snapshot.Display["terminalConnection"].(map[string]any)
				if !ok || config["sshAutoContinueNotices"] != want || config["host"] != "notice.example" {
					t.Fatal("detached workspace lost the notice preference or changed its connection")
				}
				if _, exists := config["password"]; exists {
					t.Fatal("detached workspace retained a password with the notice preference")
				}
			}
			if len(a.terminals.List()) != 0 || string(a.state) != "{}" {
				t.Fatal("workspace draft transfer connected or overwrote home preferences")
			}
		})
	}
}

func TestSSHNoticeAPIValidationAndExistingConnectionBoundary(t *testing.T) {
	a := testApplication(t)
	for _, bad := range []string{`"true"`, `1`, `{}`, `[]`, `null`} {
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
		for _, field := range []string{`"sshAutoContinueNotices":true`, `"sshAutoContinueNotices":false`, `"SSHAutoContinueNotices":false`} {
			body := `{"protocol":"sftp","action":"list","` + id + `":"existing",` + field + `}`
			if response := requestApp(a, http.MethodPost, "/api/remote-browser", body, true); response.Code != http.StatusBadRequest {
				t.Fatalf("existing %s accepted a fresh authentication preference: %d", id, response.Code)
			}
		}
	}
	for _, body := range []string{`{"type":"ssh","unknown":true}`, `{"type":"ssh","expectedHostFingerprint":"forged"}`, `{"type":"ssh","sshAutoContinueNotices":"true","SSHAutoContinueNotices":false}`} {
		if response := requestApp(a, http.MethodPost, "/api/terminals", body, true); response.Code != http.StatusBadRequest {
			t.Fatal("terminal notice decoding bypassed unknown field or boolean validation")
		}
	}
	for _, body := range []string{`{"protocol":"sftp","action":"connect","host":"127.0.0.1","sshAutoContinueNotices":null,"SSHAutoContinueNotices":true}`, `{"protocol":"sftp","action":"connect","host":"127.0.0.1","sshAutoContinueNotices":"true","SSHAutoContinueNotices":false}`} {
		if response := requestApp(a, http.MethodPost, "/api/remote-browser", body, true); response.Code != http.StatusBadRequest {
			t.Fatal("SFTP decoding accepted an invalid earlier duplicate notice preference")
		}
	}
	if len(a.terminals.List()) != 0 || len(a.browserConnectionInfos()) != 0 {
		t.Fatal("invalid notice configuration created a connection")
	}
}

func TestSSHNoticeDirectSFTPDefaultAndDisabledConnections(t *testing.T) {
	for _, test := range []struct {
		name     string
		override map[string]any
		want     bool
	}{
		{"default", nil, true},
		{"disabled", map[string]any{"sshAutoContinueNotices": false}, false},
		{"case-insensitive-disabled", map[string]any{"SSHAutoContinueNotices": false}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newPersistentBrowserFixture(t)
			id := f.connect(t, test.override)
			entry, err := f.app.browserConnection(id)
			if err != nil || entry.request.SSHAutoContinueNotices != test.want {
				t.Fatalf("fresh SFTP connection lost default/opt-out: %v", err)
			}
			for _, enabled := range []bool{false, true} {
				if status, _ := f.operations(t, id, "list", map[string]any{"sshAutoContinueNotices": enabled}); status != http.StatusBadRequest {
					t.Fatal("existing SFTP operation accepted a fresh notice preference")
				}
			}
			if status, _ := f.operations(t, id, "list", map[string]any{"path": "."}); status != http.StatusOK {
				t.Fatal("an existing SFTP operation without login fields lost its valid connection")
			}
		})
	}
}
