package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
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

// syntheticTelnetDevice is a loopback device for the Terminal: it shows a
// prompt, echoes what it receives and answers each line. It records every byte
// that reaches it, so the test can prove exactly what was sent to the session.
type syntheticTelnetDevice struct {
	mu       sync.Mutex
	received []byte
}

// input returns what was typed or pasted into the session: the received bytes
// without Telnet option negotiation and the NUL that follows a bare CR.
func (d *syntheticTelnetDevice) input() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var text []byte
	for i := 0; i < len(d.received); i++ {
		switch b := d.received[i]; {
		case b == 0xff && i+2 < len(d.received):
			i += 2
		case b != 0:
			text = append(text, b)
		}
	}
	return string(text)
}

func (d *syntheticTelnetDevice) serve(conn net.Conn) {
	defer conn.Close()
	const prompt = "lab-switch-01# "
	// IAC WILL ECHO: like a network device, this one echoes what it receives,
	// so the Terminal does not echo locally as well.
	_, _ = conn.Write([]byte{0xff, 0xfb, 0x01})
	_, _ = io.WriteString(conn, "Synthetic Telnet device\r\n"+prompt)
	buf := make([]byte, 4096)
	var line []byte
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			d.mu.Lock()
			d.received = append(d.received, buf[:n]...)
			d.mu.Unlock()
			for _, b := range buf[:n] {
				switch {
				case b == '\r' || b == '\n':
					reply := "\r\n"
					if command := strings.TrimSpace(string(line)); command == "show clock" {
						reply += "10:00:00.000 UTC Thu Nov 5 2026\r\n"
					} else if command != "" {
						reply += "synthetic reply to " + command + "\r\n"
					}
					line = line[:0]
					_, _ = io.WriteString(conn, reply+prompt)
				case b >= 0x20 && b < 0x7f:
					line = append(line, b)
					_, _ = conn.Write([]byte{b})
				}
			}
		}
		if err != nil {
			return
		}
	}
}

// The shipped pages of the standalone apps and of the main window in Chromium,
// each against its own real engine (every production handler, real loopback
// HTTP between the engines, real protected instance records under temporary
// data roots) and real Telnet sessions to synthetic loopback devices. Text
// sent from a Text Editor page must reach a device through a Terminal's paste
// path, and terminal text must reach a Text Editor's documents. Only the
// update routes are synthetic, so the test never contacts another computer.
//
// Three data roots, one per situation of the receiver order:
//
//	both:  the two standalone apps run and send to each other.
//	main:  only the main Toolkit window runs; a standalone Text Editor sends to
//	       its Terminal page and a standalone Terminal to its Text Editor.
//	start: nothing runs. A send "starts" the other standalone app (the launch
//	       hook registers an engine whose page is already loaded, where the
//	       product starts a process) and the text that was sent is delivered.
func TestStandalonePeerSendBrowser(t *testing.T) {
	if os.Getenv("NTK_PEER_SEND_BROWSER") != "1" {
		t.Skip("requires the isolated Playwright browser")
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
	// engine starts one app of a data root with its settings; it is not
	// registered (not "running" for the other apps) until the test says so.
	engine := func(root, mode string) *application {
		t.Helper()
		dir := peerWorkspaceDir(root, mode)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		page := "terminal"
		if mode == standaloneEditorMode {
			page = "editor"
		}
		settings := `{"activePage":"` + page + `","theme":"light","welcomeDismissed":true,"firstLaunchWelcome":{"dismissed":true},"appUpdatePreferences":{"enabled":false},"terminalInteraction":{"pasteReview":true,"copyOnSelect":false,"rightClick":"menu"},"editorPreferences":{"restoreSession":false}}`
		if err := os.WriteFile(filepath.Join(dir, "Network_Toolkit.settings.json"), []byte(settings), 0600); err != nil {
			t.Fatal(err)
		}
		a, _ := peerTestAppWith(t, root, mode, quietUpdates)
		return a
	}
	// connect gives an app a real Telnet session to its own synthetic device.
	connect := func(a *application) (*syntheticTelnetDevice, string) {
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
	type liveApp struct {
		URL     string `json:"url"`
		Token   string `json:"token"`
		Session string `json:"session,omitempty"`
	}
	live := func(a *application, session string) liveApp {
		return liveApp{URL: "http://" + a.host, Token: a.token, Session: session}
	}
	var apps []*application
	track := func(a *application) *application { apps = append(apps, a); return a }

	// both: the two standalone apps run.
	both := t.TempDir()
	terminalApp, editorApp := track(engine(both, "terminal")), track(engine(both, standaloneEditorMode))
	peerRegister(t, terminalApp, both)
	peerRegister(t, editorApp, both)
	device, session := connect(terminalApp)

	// main: only the main window runs; the two standalone apps of this data
	// root are open as senders but are not registered, so they are not receivers.
	onlyMain := t.TempDir()
	mainApp := track(engine(onlyMain, "main"))
	mainEditor, mainTerminal := track(engine(onlyMain, standaloneEditorMode)), track(engine(onlyMain, "terminal"))
	peerRegister(t, mainApp, onlyMain)
	if !mainApp.peerReceiveAvailable() {
		t.Fatal("the registered main window does not announce peer receive")
	}
	mainDevice, mainSession := connect(mainApp)
	mainTerminalDevice, mainTerminalSession := connect(mainTerminal)

	// start: nothing is registered. Each app's launch hook stands in for
	// starting the other app's process: it registers that engine a moment later.
	start := t.TempDir()
	startTerminal, startEditor := track(engine(start, "terminal")), track(engine(start, standaloneEditorMode))
	startDevice, startSession := connect(startTerminal)
	var launchMu sync.Mutex
	launches := map[string]int{}
	hook := func(sender, started *application) {
		sender.peer.mu.Lock()
		defer sender.peer.mu.Unlock()
		sender.peer.launch = func(context.Context) error {
			launchMu.Lock()
			launches[started.windowMode]++
			launchMu.Unlock()
			go func() {
				time.Sleep(1500 * time.Millisecond)
				unregister, err := started.registerWorkspaceWindow(peerWorkspaceDir(start, started.windowMode))
				if err != nil {
					t.Error(err)
					return
				}
				t.Cleanup(unregister)
			}()
			return nil
		}
	}
	hook(startTerminal, startEditor)
	hook(startEditor, startTerminal)
	// main: the main window runs, so nothing may be started. A start is
	// recorded and refused (instead of starting this test binary as an app).
	for _, sender := range []*application{mainEditor, mainTerminal} {
		sender := sender
		sender.peer.mu.Lock()
		sender.peer.launch = func(context.Context) error {
			launchMu.Lock()
			launches["main-only "+sender.windowMode]++
			launchMu.Unlock()
			return errors.New("the main window runs, so no app is started in this phase")
		}
		sender.peer.mu.Unlock()
	}

	more, err := json.Marshal(map[string]map[string]liveApp{
		"main":  {"main": live(mainApp, mainSession), "editor": live(mainEditor, ""), "terminal": live(mainTerminal, mainTerminalSession)},
		"start": {"terminal": live(startTerminal, startSession), "editor": live(startEditor, "")},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "node", "scripts/standalone-peer-live.browser.cjs")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(),
		"NTK_PEER_TERMINAL_URL=http://"+terminalApp.host, "NTK_PEER_TERMINAL_TOKEN="+terminalApp.token,
		"NTK_PEER_EDITOR_URL=http://"+editorApp.host, "NTK_PEER_EDITOR_TOKEN="+editorApp.token,
		"NTK_PEER_SESSION="+session, "NTK_PEER_MORE="+string(more),
		"NTK_PEER_LAUNCH_WAIT_MS="+strconv.FormatInt(peerLaunchWait.Milliseconds(), 10))
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		launchMu.Lock()
		t.Logf("apps started: %v", launches)
		launchMu.Unlock()
		t.Fatal(err)
	}
	// Everything the device received, in order: the selected line exactly as
	// sent (peer send added no Enter), the Enter the script pressed in the
	// Terminal itself, then the reviewed text once, after it was confirmed
	// there. The send refused during the review never reached the device.
	got := device.input()
	if want := "show version" + "\r" + "show clock\r\r"; got != want {
		t.Fatalf("the device received %q, want %q", got, want)
	}
	// The main window's session: the selected line with no Enter added, then
	// the reviewed line once, after it was confirmed in the main window.
	if got, want := mainDevice.input(), "show version"+"show clock\r"; got != want {
		t.Fatalf("the main window's device received %q, want %q", got, want)
	}
	// A Terminal that only sent its output received nothing.
	if got := mainTerminalDevice.input(); got != "" {
		t.Fatalf("the sending Terminal's device received %q", got)
	}
	// The started Terminal's session: the one line, once, with no Enter.
	if got, want := startDevice.input(), "show version"; got != want {
		t.Fatalf("the started Terminal's device received %q, want %q", got, want)
	}
	launchMu.Lock()
	defer launchMu.Unlock()
	if launches[standaloneEditorMode] != 1 || launches["terminal"] != 1 || len(launches) != 2 {
		t.Fatalf("each app must be started exactly once: %v", launches)
	}
	for _, a := range apps {
		for _, secret := range []string{a.token, a.windowActivationToken} {
			if bytes.Contains(output, []byte(secret)) {
				t.Fatal("a token appeared in the browser run")
			}
			for _, received := range []string{got, mainDevice.input(), startDevice.input()} {
				if strings.Contains(received, secret) {
					t.Fatal("a token reached a device")
				}
			}
		}
	}
}
