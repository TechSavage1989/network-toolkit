package main

import (
	"context"
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
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/updates/") {
				// Never contact the update server from a test.
				jsonResponse(w, 200, map[string]any{})
				return
			}
			a.ServeHTTP(w, r)
		}))
		t.Cleanup(server.Close)
		a.host = strings.TrimPrefix(server.URL, "http://")
		env = append(env, "NTK_SHARED_"+app.name+"_URL="+server.URL, "NTK_SHARED_"+app.name+"_TOKEN="+a.token)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "node", "scripts/shared-data.browser.cjs")
	command.Dir = filepath.Join("..", "..")
	command.Env = env
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	// What the three pages agreed on is what is on disk, in the main settings
	// file only.
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "from-terminal", "from-main", "while-stale") {
		t.Fatalf("saved connections on disk after the browser run: %v", got)
	}
	for _, workspace := range []string{terminalWorkspaceName, standaloneEditorDirectory} {
		raw, err := os.ReadFile(filepath.Join(root, workspace, appSettingsFilename))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "from-terminal") || strings.Contains(string(raw), "terminalProfiles") {
			t.Fatalf("%s holds saved connections in its own settings file: %s", workspace, raw)
		}
	}
}
