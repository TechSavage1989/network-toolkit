package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestEditorSessionBrowserToolbarButtons runs scripts/editor-toolbar-icons.browser.cjs:
// the Text Editor toolbar's icon and text buttons (owner request after
// v1.21.57) in the shipped page as the main Toolkit window and its detached
// Text Editor window, both of ONE real engine (every production handler, real
// loopback HTTP). The toolbar has no Files button and the sidebar rail reads
// Docs, Connect, Files; the buttons are icons by default and text after More →
// Toolbar buttons → Text, with the same names and tooltips; the choice is kept
// in the data folder's shared settings across a reload and reaches a detached
// editor window through the engine's checked move batch (validateEditorBatch).
// Each native child's heartbeat is played by this test (the pages are Chromium
// tabs, not WebView2 windows); no native Windows claim.
func TestEditorSessionBrowserToolbarButtons(t *testing.T) {
	if os.Getenv("NTK_EDITOR_SESSION_BROWSER") != "1" {
		t.Skip("requires the isolated Playwright browser")
	}
	root := t.TempDir()
	settings := `{"activePage":"editor","theme":"light","welcomeDismissed":true,"firstLaunchWelcome":{"dismissed":true},"appUpdatePreferences":{"enabled":false},"editorPreferences":{"restoreSession":false}}`
	if err := os.WriteFile(filepath.Join(root, appSettingsFilename), []byte(settings), 0600); err != nil {
		t.Fatal(err)
	}
	quietUpdates := func(a *application) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/updates/") {
				jsonResponse(w, http.StatusOK, map[string]any{})
				return
			}
			a.ServeHTTP(w, r)
		})
	}
	a, _ := peerTestAppWith(t, root, "main", quietUpdates)
	// The main page detaches its editor with its own code; each launch that
	// would start a native editor window is handed to the script through a
	// private file instead (it opens the view page in Chromium).
	launchFile := filepath.Join(t.TempDir(), "editor-views.json")
	var launchMu sync.Mutex
	var launches []map[string]string
	a.launchEditorView = func(_ context.Context, launch editorViewLaunch) (<-chan error, error) {
		launchMu.Lock()
		defer launchMu.Unlock()
		launches = append(launches, map[string]string{"view": launch.ViewID, "token": launch.ViewToken})
		raw, _ := json.Marshal(launches)
		return nil, os.WriteFile(launchFile, raw, 0600)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	// The native children's heartbeat (editor_view_child.go), which keeps
	// their windows open in the engine, as promptly as a native child pulses.
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			launchMu.Lock()
			tokens := make([]string, 0, len(launches))
			for _, launch := range launches {
				tokens = append(tokens, launch["token"])
			}
			launchMu.Unlock()
			for _, token := range tokens {
				request, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+a.host+"/api/editor-views/current/pulse", strings.NewReader("{}"))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Session-Token", token)
				if response, err := http.DefaultClient.Do(request); err == nil {
					response.Body.Close()
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	command := exec.CommandContext(ctx, "node", "scripts/editor-toolbar-icons.browser.cjs")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(), "NTK_TOOLBAR_URL=http://"+a.host, "NTK_TOOLBAR_MAIN_TOKEN="+a.token, "NTK_TOOLBAR_EDITOR_LAUNCHES="+launchFile)
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	// The choice is a shared editor preference, kept in the data folder's
	// settings file (sharedDataKeys: editorPreferences); the script leaves Text.
	prefs, _ := sharedDisk(t, root)["editorPreferences"].(map[string]any)
	if prefs["toolbarLabels"] != "text" {
		t.Fatalf("the shared settings do not keep the Toolbar buttons choice: editorPreferences=%v", prefs)
	}
	launchMu.Lock()
	defer launchMu.Unlock()
	if len(launches) < 2 {
		t.Fatalf("the main page opened %d editor windows, want 2", len(launches))
	}
	secrets := []string{a.token}
	for _, launch := range launches {
		secrets = append(secrets, launch["token"])
	}
	for _, secret := range secrets {
		if bytes.Contains(output, []byte(secret)) {
			t.Fatal("a token appeared in the browser run")
		}
	}
}
