package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Owner decision, in the shipped page with one real engine: the main window
// pops its Terminal workspace out (its own code), the workspace's SFTP browser
// offers Edit, and the file opens in the main window's Text Editor on the same
// SSH connection, where it is edited and saved with the main window's checks.
// The SSH server is a synthetic loopback fixture with SFTP; Chromium tabs stand
// in for the native windows. No native Windows claim.
func TestWindowSendRemoteFileBrowser(t *testing.T) {
	if os.Getenv("NTK_WINDOW_SEND_BROWSER") != "1" {
		t.Skip("requires the isolated Playwright browser")
	}
	f := newShellSFTPBrowserFixture(t)
	a := f.app
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/updates/") {
			jsonResponse(w, http.StatusOK, map[string]any{})
			return
		}
		a.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	a.host = strings.TrimPrefix(server.URL, "http://")
	if r := requestApp(a, "PUT", "/api/state", `{"activePage":"terminal","theme":"light","welcomeDismissed":true,"firstLaunchWelcome":{"dismissed":true},"appUpdatePreferences":{"enabled":false},"terminalSidebarTab":"files","terminalSidebarCollapsed":false,"editorPreferences":{"restoreSession":false}}`, true); r.Code != 200 {
		t.Fatalf("settings: %d %s", r.Code, r.Body.String())
	}
	id := f.connect(t, nil)
	code, result := f.operations(t, id, "open-terminal", nil)
	if code != 200 {
		t.Fatal(code, result)
	}
	session := result["session"].(map[string]any)["id"].(string)
	viewTestWait(t, func() bool { info, _ := a.terminalSession(session); return info.State == "connected" })

	launchFile := filepath.Join(t.TempDir(), "terminal-view.json")
	var mu sync.Mutex
	var viewToken string
	a.launchTerminalView = func(_ context.Context, launch terminalViewLaunch) (<-chan error, error) {
		mu.Lock()
		viewToken = launch.ViewToken
		mu.Unlock()
		raw, _ := json.Marshal(map[string]string{"view": launch.ViewID, "token": launch.ViewToken})
		return nil, os.WriteFile(launchFile, raw, 0600)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			mu.Lock()
			token := viewToken
			mu.Unlock()
			if token != "" {
				request, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/api/terminal-views/current/pulse", strings.NewReader("{}"))
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
	command := exec.CommandContext(ctx, "node", "scripts/window-send-remote-live.browser.cjs")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(), "NTK_WINDOW_URL="+server.URL, "NTK_WINDOW_MAIN_TOKEN="+a.token, "NTK_WINDOW_TERMINAL_LAUNCH="+launchFile, "NTK_WINDOW_SESSION="+session)
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(filepath.Join(f.root, "startup.cfg")); err != nil || string(raw) != "hostname test-switch\r\nbanner edited-in-main\r\n" {
		t.Fatalf("the remote file after the save: %q %v", raw, err)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, secret := range []string{a.token, viewToken, "persistent-test-password"} {
		if secret != "" && bytes.Contains(output, []byte(secret)) {
			t.Fatal("a secret appeared in the browser run")
		}
	}
}
