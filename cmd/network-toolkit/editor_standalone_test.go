package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStandaloneEditorActivationAndPreferenceIsolation(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("requires the platform workspace lock")
	}
	root := t.TempDir()
	const settings = "Network_Toolkit.settings.json"
	mainSettings := []byte(`{"theme":"light","editorPreferences":{"wordWrap":true},"terminalProfiles":[{"name":"private"}],"credentialConfigIdentity":"private"}`)
	if err := os.WriteFile(filepath.Join(root, settings), mainSettings, 0600); err != nil {
		t.Fatal(err)
	}
	mainRelease, reused, err := prepareEditorWorkspace(root, "main", nil)
	if err != nil || reused || mainRelease == nil {
		t.Fatalf("main workspace lock: %v (reused=%t)", err, reused)
	}
	defer mainRelease()
	dir, release, reused, err := prepareStandaloneEditor(root, nil)
	if err != nil || reused || release == nil || dir != filepath.Join(root, standaloneEditorDirectory) {
		t.Fatalf("standalone workspace: %s (%v, reused=%t)", dir, err, reused)
	}
	defer release()
	data, err := os.ReadFile(filepath.Join(dir, settings))
	if err != nil || !strings.Contains(string(data), `"wordWrap": true`) || strings.Contains(string(data), "private") {
		t.Fatalf("unexpected editor preferences: %s (%v)", data, err)
	}
	a := testApplication(t)
	a.windowMode = standaloneEditorMode
	a.editorLaunchAvailable, a.editorLaunchReusable = true, true
	server := httptest.NewServer(a)
	defer server.Close()
	a.host = strings.TrimPrefix(server.URL, "http://")
	unregister, err := a.registerWorkspaceWindow(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer unregister()
	if page := requestApp(a, http.MethodGet, "/", "", false); page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "WINDOW_MODE='editor-standalone'") {
		t.Fatal("standalone editor was not bootstrapped into its own window mode")
	}
	path := filepath.Join(root, "another synthetic file.txt")
	if err := os.WriteFile(path, []byte("fixture contents"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, reused, err := prepareStandaloneEditor(root, &editorLaunchRequest{Path: path}); err != nil || !reused {
		t.Fatalf("standalone Explorer activation: %v (reused=%t)", err, reused)
	}
	var inbox struct{ Requests []editorLaunchRequest }
	response := requestApp(a, http.MethodPost, editorLaunchEndpoint, "", true)
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &inbox) != nil || len(inbox.Requests) != 1 || inbox.Requests[0].Path != path {
		t.Fatalf("standalone editor did not receive the selected file: %d %s", response.Code, response.Body)
	}
	if current, err := os.ReadFile(filepath.Join(root, settings)); err != nil || string(current) != string(mainSettings) {
		t.Fatalf("the main settings changed: %v", err)
	}
}
