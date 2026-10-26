package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestSavedSessionFoldersPersistWithoutSecrets(t *testing.T) {
	a := testApplication(t)
	body := `{"terminalProfiles":[{"id":"ssh","name":"Core","type":"ssh","host":"core.test","port":22,"folder":" MDF ","credentialId":"vault-reference","password":"secret-marker"}],"ftpProfiles":[{"id":"ftp","name":"Files","host":"files.test","username":"operator","port":990,"folder":"Branch","security":"implicit","password":"secret-marker"}]}`
	if r := requestApp(a, "PUT", "/api/state", body, true); r.Code != http.StatusOK {
		t.Fatalf("save: %d %s", r.Code, r.Body.String())
	}
	disk, err := os.ReadFile(a.dataPath)
	if err != nil || strings.Contains(string(disk), "secret-marker") {
		t.Fatalf("settings persistence: %v", err)
	}
	r := requestApp(a, "GET", "/api/state", "", true)
	var saved struct {
		Terminal []map[string]any `json:"terminalProfiles"`
		FTP      []map[string]any `json:"ftpProfiles"`
	}
	if r.Code != http.StatusOK || json.Unmarshal(r.Body.Bytes(), &saved) != nil || len(saved.Terminal) != 1 || len(saved.FTP) != 1 {
		t.Fatalf("reload: %d %s", r.Code, r.Body.String())
	}
	if saved.Terminal[0]["folder"] != "MDF" || saved.Terminal[0]["credentialId"] != "vault-reference" || saved.FTP[0]["folder"] != "Branch" {
		t.Fatalf("folder metadata lost: %#v", saved)
	}
	saved.Terminal, saved.FTP = nil, nil
	if err := json.Unmarshal(disk, &saved); err != nil || len(saved.Terminal) != 1 || len(saved.FTP) != 1 || saved.Terminal[0]["folder"] != "MDF" || saved.FTP[0]["folder"] != "Branch" {
		t.Fatalf("folders were not written to disk: %v", err)
	}
}

func TestSavedSessionFolderBoundsAndWorkspaceDisplay(t *testing.T) {
	if got := safeSessionFolder("  Site\n\x00 A\x7f  "); got != "Site A" {
		t.Fatalf("folder control characters retained: %q", got)
	}
	if got := safeSessionFolder(strings.Repeat("😀", 101)); len([]rune(got)) != 100 {
		t.Fatal("folder labels are not bounded by Unicode characters")
	}
	if got := safeSessionFolder(map[string]any{"password": "secret-marker"}); got != "" {
		t.Fatal("structured data became a folder")
	}
	profile := map[string]any{"id": "ssh", "name": "Core", "type": "ssh", "host": "core.test", "folder": "MDF", "password": "secret-marker"}
	display := terminalWorkspaceDisplay(map[string]any{"terminalProfiles": []any{profile}})
	rows := display["terminalProfiles"].([]map[string]any)
	if len(rows) != 1 || rows[0]["folder"] != "MDF" || rows[0]["password"] != nil {
		t.Fatalf("linked workspace folder metadata: %#v", rows)
	}
	if _, exists := safeTerminalProfile(profile, false)["folder"]; exists {
		t.Fatal("folder label entered the connection configuration")
	}
}
