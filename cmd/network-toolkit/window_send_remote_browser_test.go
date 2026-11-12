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
	"sync/atomic"
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
	var sending atomic.Int32 // window-send requests in flight
	a := f.app
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/updates/") {
			jsonResponse(w, http.StatusOK, map[string]any{})
			return
		}
		if r.URL.Path == "/api/window-send/send" {
			sending.Add(1)
			defer sending.Add(-1)
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
	// Owner decision (v1.21.50): with the main window closed, Edit reopens it.
	// The script closes the main page and says so through a private file; this
	// test then does what the native host's close does (the engine runs on for
	// the workspace), waits for the engine to ask for the main window again
	// (activatePrimary wakes runPrimaryLifecycle) and lets the script open the
	// main page again, as the reopened native window would.
	closedFile, reopenFile := filepath.Join(filepath.Dir(launchFile), "main-closed"), filepath.Join(filepath.Dir(launchFile), "main-reopen")
	var reopened sync.WaitGroup
	reopened.Add(1)
	go func() {
		defer reopened.Done()
		for {
			if _, err := os.Stat(closedFile); err == nil {
				break
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
		a.views.mu.Lock()
		a.views.homePresent = false
		a.views.mu.Unlock()
		a.editorViews.setHomePresent(false)
		a.windowSendHomeClosed() // as runPrimaryLifecycle does when the native window closes
		for drained := false; !drained; {
			select {
			case <-a.primaryWake:
			default:
				drained = true
			}
		}
		_ = os.Remove(closedFile) // the script waits for this
		// Only Edit's request counts: closing the page can ask for the main
		// window too (its last requests as it unloads).
		for asked := false; !asked; {
			select {
			case <-a.primaryWake:
				asked = sending.Load() > 0
			case <-ctx.Done():
				return
			}
		}
		a.views.mu.Lock()
		a.views.homePresent = true
		a.views.mu.Unlock()
		a.editorViews.setHomePresent(true)
		_ = os.WriteFile(reopenFile, []byte("reopen"), 0600)
	}()
	command := exec.CommandContext(ctx, "node", "scripts/window-send-remote-live.browser.cjs")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(), "NTK_WINDOW_URL="+server.URL, "NTK_WINDOW_MAIN_TOKEN="+a.token, "NTK_WINDOW_TERMINAL_LAUNCH="+launchFile, "NTK_WINDOW_SESSION="+session,
		"NTK_WINDOW_MAIN_CLOSED="+closedFile, "NTK_WINDOW_MAIN_REOPEN="+reopenFile)
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	reopened.Wait()
	if _, err := os.Stat(reopenFile); err != nil {
		t.Fatal("Edit never asked for the closed main window")
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
