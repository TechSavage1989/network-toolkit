package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"networktoolkit/internal/terminal"
)

// The shipped page in Chromium as the main Toolkit window, one detached
// Terminal window and one detached Text Editor window of ONE real engine
// (every production handler, real loopback HTTP), with real Telnet sessions
// to synthetic loopback devices. Owner request: popped-out windows keep the
// single Toolkit window's features between them. Text sent from the editor
// window must reach a session of the Terminal window and of the main window
// through that window's paste path, and terminal text must reach the editor
// window's document and a new document in the main window. Each native
// child's heartbeat is played by this test (the pages are Chromium tabs, not
// WebView2 windows); no native Windows claim.
func TestWindowSendBrowser(t *testing.T) {
	if os.Getenv("NTK_WINDOW_SEND_BROWSER") != "1" {
		t.Skip("requires the isolated Playwright browser")
	}
	root := t.TempDir()
	quietUpdates := func(a *application) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/updates/") {
				jsonResponse(w, http.StatusOK, map[string]any{})
				return
			}
			a.ServeHTTP(w, r)
		})
	}
	dir := peerWorkspaceDir(root, "main")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	settings := `{"activePage":"terminal","theme":"light","welcomeDismissed":true,"firstLaunchWelcome":{"dismissed":true},"appUpdatePreferences":{"enabled":false},"terminalInteraction":{"pasteReview":true,"copyOnSelect":false,"rightClick":"menu"},"editorPreferences":{"restoreSession":false}}`
	if err := os.WriteFile(filepath.Join(dir, "Network_Toolkit.settings.json"), []byte(settings), 0600); err != nil {
		t.Fatal(err)
	}
	a, _ := peerTestAppWith(t, root, "main", quietUpdates)
	// The main page detaches the session with its own code; the launch that
	// would start the separate Terminal program is handed to the script
	// through a private file instead (it opens the view page in Chromium).
	launchFile := filepath.Join(t.TempDir(), "terminal-view.json")
	var launchMu sync.Mutex
	var terminalLaunch terminalViewLaunch
	a.launchTerminalView = func(_ context.Context, launch terminalViewLaunch) (<-chan error, error) {
		launchMu.Lock()
		terminalLaunch = launch
		launchMu.Unlock()
		raw, _ := json.Marshal(map[string]string{"view": launch.ViewID, "token": launch.ViewToken})
		return nil, os.WriteFile(launchFile, raw, 0600)
	}
	editorFile := filepath.Join(filepath.Dir(launchFile), "editor-view.json")
	var editorToken string
	a.launchEditorView = func(_ context.Context, launch editorViewLaunch) (<-chan error, error) {
		launchMu.Lock()
		editorToken = launch.ViewToken
		launchMu.Unlock()
		raw, _ := json.Marshal(map[string]string{"view": launch.ViewID, "token": launch.ViewToken})
		return nil, os.WriteFile(editorFile, raw, 0600)
	}

	connect := func() (*syntheticTelnetDevice, string) {
		t.Helper()
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { listener.Close() })
		device := &syntheticTelnetDevice{}
		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				go device.serve(conn)
			}
		}()
		port := listener.Addr().(*net.TCPAddr).Port
		code, _, raw := peerPage(t, a, http.MethodPost, "/api/terminals", `{"type":"telnet","host":"127.0.0.1","port":`+strconv.Itoa(port)+`,"cols":100,"rows":28}`)
		if code != http.StatusCreated {
			t.Fatalf("connect the synthetic device: %d %s", code, raw)
		}
		var connected struct {
			Session terminal.SessionInfo `json:"session"`
		}
		if err = json.Unmarshal([]byte(raw), &connected); err != nil || connected.Session.ID == "" {
			t.Fatalf("connect reply: %s", raw)
		}
		return device, connected.Session.ID
	}
	mainDevice, mainSession := connect()
	viewDevice, viewSession := connect()

	// The native children's heartbeat (terminal_view_child.go,
	// editor_view_child.go), which keeps their windows open in the engine.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			launchMu.Lock()
			viewToken, editorViewToken := terminalLaunch.ViewToken, editorToken
			launchMu.Unlock()
			for _, beat := range []struct{ token, path string }{{viewToken, "/api/terminal-views/current/pulse"}, {editorViewToken, "/api/editor-views/current/pulse"}} {
				if beat.token == "" {
					continue
				}
				request, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+a.host+beat.path, strings.NewReader("{}"))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Session-Token", beat.token)
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

	command := exec.CommandContext(ctx, "node", "scripts/window-send-live.browser.cjs")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(),
		"NTK_WINDOW_URL=http://"+a.host, "NTK_WINDOW_MAIN_TOKEN="+a.token,
		"NTK_WINDOW_TERMINAL_LAUNCH="+launchFile,
		"NTK_WINDOW_EDITOR_LAUNCH="+editorFile,
		"NTK_WINDOW_MAIN_SESSION="+mainSession, "NTK_WINDOW_VIEW_SESSION="+viewSession)
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	// The detached Terminal window's device: the selected line with no Enter,
	// then nothing while a dialog was open there.
	if got, want := viewDevice.input(), "show version"; got != want {
		t.Fatalf("the Terminal window's device received %q, want %q", got, want)
	}
	// The main window's device: the reviewed text once, after it was confirmed there.
	if got, want := mainDevice.input(), "show clock\r"; got != want {
		t.Fatalf("the main window's device received %q, want %q", got, want)
	}
	launchMu.Lock()
	defer launchMu.Unlock()
	if terminalLaunch.ViewToken == "" || editorToken == "" {
		t.Fatal("the main page did not open the Terminal and Text Editor windows")
	}
	// Owner decision (v1.21.50): each window that took text was told to come
	// to the front (its native host would collect this; Chromium has none):
	// the Terminal window, and the main window through its own activation.
	a.windowSend.mu.Lock()
	act := a.windowSend.activations[windowSendTerminal+terminalLaunch.ViewID]
	raised := act != nil && !act.raiseAt.IsZero()
	a.windowSend.mu.Unlock()
	if !raised {
		t.Fatal("the Terminal window that took text was never told to come to the front")
	}
	if len(a.windowActivate) == 0 {
		t.Fatal("the main window that took text was never raised")
	}
	for _, secret := range []string{a.token, terminalLaunch.ViewToken, editorToken} {
		if bytes.Contains(output, []byte(secret)) {
			t.Fatal("a token appeared in the browser run")
		}
	}
}
