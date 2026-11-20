//go:build windows

package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestWindowsBrowserWindow runs the real app the way it opens on Windows
// Server without the WebView2 Runtime (owner report on v1.21.72), with a
// stand-in for Edge: the window is a dedicated app window opened through a
// one-time loopback link, only that window gets the token, pop-out windows are
// off, and closing the window stops the app. Hosted runners have WebView2, so
// NETWORK_TOOLKIT_WINDOW=browser chooses the browser window here.
func TestWindowsBrowserWindow(t *testing.T) {
	if os.Getenv("NETWORK_TOOLKIT_BROWSER_WINDOW_TEST") != "1" {
		t.Skip("set NETWORK_TOOLKIT_BROWSER_WINDOW_TEST=1 to run the app with a stand-in browser")
	}
	work := t.TempDir()
	app, standin := filepath.Join(work, "Network_Toolkit.exe"), filepath.Join(work, "browser", "msedge.exe")
	for _, build := range [][2]string{{app, "."}, {standin, "./testdata/browser-window-standin"}} {
		out, err := exec.Command("go", "build", "-mod=vendor", "-o", build[0], build[1]).CombinedOutput()
		if err != nil {
			t.Fatalf("build %s: %v\n%s", build[1], err, out)
		}
	}
	cmd := exec.Command(app, "--data-dir", filepath.Join(work, "data"))
	cmd.Env = append(os.Environ(), "NETWORK_TOOLKIT_WINDOW=browser", "NETWORK_TOOLKIT_BROWSER="+standin)
	// The app's output goes to a file, never a pipe: a pipe kept open by a
	// process the app started would hold cmd.Wait, and with it the cleanup,
	// until the test binary's own deadline hid the real failure (CI on v1.21.73).
	logFile, err := os.Create(filepath.Join(work, "app.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// Closed when the app has ended, so the body and the cleanup can both
	// wait for it (a value sent once was taken by the body, and the cleanup
	// then waited for a second one that never came: CI on v1.21.73).
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	appLog := func() string {
		data, _ := os.ReadFile(filepath.Join(work, "app.log"))
		report, _ := os.ReadFile(filepath.Join(work, "browser", "report.json"))
		return "app output:\n" + string(data) + "\nstand-in report:\n" + string(report)
	}
	t.Cleanup(func() {
		_ = os.WriteFile(filepath.Join(work, "browser", "close"), nil, 0o600)
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			select {
			case <-exited:
			case <-time.After(10 * time.Second):
				t.Errorf("the app did not end after it was killed")
			}
		}
	})
	var r struct {
		Args     []string       `json:"args"`
		Launch   string         `json:"launch"`
		Status   int            `json:"status"`
		Path     string         `json:"path"`
		Token    bool           `json:"token"`
		Engine   map[string]any `json:"engine"`
		Problems []string       `json:"problems"`
	}
	deadline := time.Now().Add(60 * time.Second)
	for {
		if data, err := os.ReadFile(filepath.Join(work, "browser", "report.json")); err == nil && json.Unmarshal(data, &r) == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the app never opened its browser window\n%s", appLog())
		}
		time.Sleep(200 * time.Millisecond)
	}
	joined := strings.Join(r.Args, " ")
	for _, want := range []string{"--app=http://127.0.0.1:", "/launch?ticket=", "--user-data-dir=" + filepath.Join(work, "data", storageBrowserDirectory, "BrowserWindow"), "--disable-background-mode", "--no-first-run"} {
		if !strings.Contains(joined, want) {
			t.Errorf("window arguments lack %q: %v", want, r.Args)
		}
	}
	if strings.Contains(joined, "no-sandbox") || strings.Contains(joined, "disable-web-security") {
		t.Errorf("browser protections must stay on: %v", r.Args)
	}
	if len(r.Problems) != 0 || r.Status != http.StatusOK || r.Path != "/" || !r.Token {
		t.Fatalf("the launched window must reach the page and its token: %+v\n%s", r, appLog())
	}
	if r.Engine["os"] != "windows" || r.Engine["uiHost"] != "browser-window" || r.Engine["terminalDetachAvailable"] != false || r.Engine["editorWindowAvailable"] != false {
		t.Errorf("status in a browser window: %v\n%s", r.Engine, appLog())
	}
	base := r.Launch[:strings.Index(r.Launch, "/launch")]
	for _, address := range []string{base + "/", r.Launch} {
		response, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Get(address)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != http.StatusForbidden || tokenInPage(body) {
			t.Errorf("%s without the window's cookie (or a used link) must be refused without a token: %d", address, response.StatusCode)
		}
	}
	// Closing the window: the stand-in exits and stops polling; the app stops.
	if err := os.WriteFile(filepath.Join(work, "browser", "close"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
	case <-time.After(45 * time.Second):
		t.Fatalf("the app kept running after its window closed\n%s", appLog())
	}
}

func tokenInPage(body []byte) bool {
	return strings.Contains(string(body), "__networkToolkitNativeSession:'")
}
