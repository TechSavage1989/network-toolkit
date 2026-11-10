package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSharedDataBrowser runs the shipped page of three apps (main window,
// standalone Terminal, standalone Text Editor) against three real engines that
// share one temporary data folder. It checks what a user would see: a
// connection saved in one app appears in the others, and a save from a page
// that has not refreshed removes nothing. No connection is opened.
func TestSharedDataBrowser(t *testing.T) {
	if os.Getenv("NTK_SHARED_DATA_BROWSER") != "1" {
		t.Skip("requires the isolated Playwright browser")
	}
	root := t.TempDir()
	quiet := `{"appUpdatePreferences":{"enabled":false},"firstLaunchWelcomeSeen":true,"activePage":"terminal","terminalSidebarCollapsed":false,"terminalSidebarTab":"saved"}`
	for _, workspace := range []string{"", terminalWorkspaceName, standaloneEditorDirectory} {
		dir := filepath.Join(root, workspace)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, appSettingsFilename), []byte(quiet), 0600); err != nil {
			t.Fatal(err)
		}
	}
	env := os.Environ()
	for _, app := range []struct{ name, workspace, mode string }{{"MAIN", "", "main"}, {"TERMINAL", terminalWorkspaceName, "terminal"}, {"EDITOR", standaloneEditorDirectory, standaloneEditorMode}} {
		a := sharedTestApp(t, root, app.workspace, app.mode)
		server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/updates/") {
				// Never contact the update server from a test.
				jsonResponse(w, 200, map[string]any{})
				return
			}
			a.ServeHTTP(w, r)
		}))
		// The app learns its address before the server answers anyone: another
		// test's client can reach a reused port at once (the race detector saw
		// that write next to a request).
		a.host = server.Listener.Addr().String()
		server.Start()
		t.Cleanup(server.Close)
		env = append(env, "NTK_SHARED_"+app.name+"_URL="+server.URL, "NTK_SHARED_"+app.name+"_TOKEN="+a.token)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "node", "scripts/shared-data.browser.cjs")
	command.Dir = filepath.Join("..", "..")
	reportDir := os.Getenv("TOOLKIT_TEST_OUTPUT")
	if reportDir == "" {
		reportDir = t.TempDir()
		env = append(env, "TOOLKIT_TEST_OUTPUT="+reportDir)
	}
	command.Env = env
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	// Every key the three pages keep in their settings is classified as shared
	// or per-window in the engine table.
	var report struct {
		Keys     map[string][]string `json:"keys"`
		Profiles []string            `json:"profiles"`
	}
	raw, err := os.ReadFile(filepath.Join(reportDir, "shared-data-report.json"))
	if err != nil || json.Unmarshal(raw, &report) != nil || len(report.Keys["main"]) < 60 {
		t.Fatalf("the browser run did not report the pages' settings keys: %v", err)
	}
	classified := map[string]bool{}
	for _, key := range sharedDataKeys {
		classified[key.Name] = true
	}
	for _, name := range perWindowStateKeys {
		classified[name] = true
	}
	for app, names := range report.Keys {
		for _, name := range names {
			if !classified[name] {
				t.Errorf("%s keeps %q, which is neither in sharedDataKeys nor in perWindowStateKeys (shared_data.go)", app, name)
			}
		}
	}
	disk := sharedDisk(t, root)
	prefs, _ := disk["editorPreferences"].(map[string]any)
	appearance, _ := disk["terminalAppearance"].(map[string]any)
	if disk["theme"] != "dark" || disk["terminalFontSize"] != float64(17) || prefs["wordWrap"] != true || prefs["fontSize"] != float64(16) || appearance["cursor"] != "bar" {
		t.Fatalf("settings on disk after the browser run: theme=%v font=%v editor=%v appearance=%v", disk["theme"], disk["terminalFontSize"], prefs, appearance)
	}
	// What the three pages agreed on is what is on disk, in the main settings
	// file only.
	// The run ends with a full list of saved sessions: the three from the
	// first steps and every one added in bulk since.
	got := sharedDiskIDs(t, root, "terminalProfiles")
	limit, _ := sharedKeyNamed("terminalProfiles")
	if len(got) != limit.MaxSaved || !sameIDs(got, report.Profiles...) {
		t.Fatalf("saved connections on disk after the browser run: %d on disk, %d in the page", len(got), len(report.Profiles))
	}
	have := map[string]bool{}
	for _, id := range got {
		have[id] = true
	}
	for _, id := range []string{"from-terminal", "from-main", "while-stale", "bulk-000", "bulk-120", "bulk-146"} {
		if !have[id] {
			t.Fatalf("%s is missing from the data folder after the browser run", id)
		}
	}
	for _, workspace := range []string{terminalWorkspaceName, standaloneEditorDirectory} {
		raw, err := os.ReadFile(filepath.Join(root, workspace, appSettingsFilename))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "from-terminal") || strings.Contains(string(raw), "bulk-") || strings.Contains(string(raw), "terminalProfiles") || strings.Contains(string(raw), `"theme"`) || strings.Contains(string(raw), "terminalAppearance") {
			t.Fatalf("%s holds shared data in its own settings file: %s", workspace, raw)
		}
		if !strings.Contains(string(raw), "activePage") {
			t.Fatalf("%s lost its own window state: %s", workspace, raw)
		}
	}
}
