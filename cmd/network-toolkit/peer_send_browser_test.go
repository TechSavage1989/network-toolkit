package main

import (
	"bytes"
	"context"
	"encoding/json"
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

// The shipped pages of BOTH standalone apps in Chromium, each against its own
// real engine (every production handler, real loopback HTTP between the two
// engines, real protected instance records under one temporary data root) and
// a real Telnet session to a synthetic loopback device. Text sent from the
// Text Editor page must reach the device through the Terminal's paste path,
// and terminal text must reach the Text Editor's documents. Only the update
// routes are synthetic, so the test never contacts another computer.
func TestStandalonePeerSendBrowser(t *testing.T) {
	if os.Getenv("NTK_PEER_SEND_BROWSER") != "1" {
		t.Skip("requires the isolated Playwright browser")
	}
	root := t.TempDir()
	for _, mode := range []string{"terminal", standaloneEditorMode} {
		dir := peerWorkspaceDir(root, mode)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		page := "terminal"
		if mode == standaloneEditorMode {
			page = "editor"
		}
		settings := `{"activePage":"` + page + `","theme":"light","appUpdatePreferences":{"enabled":false},"terminalInteraction":{"pasteReview":true,"copyOnSelect":false,"rightClick":"menu"},"editorPreferences":{"restoreSession":false}}`
		if err := os.WriteFile(filepath.Join(dir, "Network_Toolkit.settings.json"), []byte(settings), 0600); err != nil {
			t.Fatal(err)
		}
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
	terminalApp, _ := peerTestAppWith(t, root, "terminal", quietUpdates)
	editorApp, _ := peerTestAppWith(t, root, standaloneEditorMode, quietUpdates)
	peerRegister(t, terminalApp, root)
	peerRegister(t, editorApp, root)

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
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
	code, _, raw := peerPage(t, terminalApp, http.MethodPost, "/api/terminals", `{"type":"telnet","host":"127.0.0.1","port":`+strconv.Itoa(port)+`,"cols":100,"rows":28}`)
	if code != http.StatusCreated {
		t.Fatalf("connect the synthetic device: %d %s", code, raw)
	}
	var connected struct {
		Session terminal.SessionInfo `json:"session"`
	}
	if err = json.Unmarshal([]byte(raw), &connected); err != nil || connected.Session.ID == "" {
		t.Fatalf("connect reply: %s", raw)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "node", "scripts/standalone-peer-live.browser.cjs")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(),
		"NTK_PEER_TERMINAL_URL=http://"+terminalApp.host, "NTK_PEER_TERMINAL_TOKEN="+terminalApp.token,
		"NTK_PEER_EDITOR_URL=http://"+editorApp.host, "NTK_PEER_EDITOR_TOKEN="+editorApp.token,
		"NTK_PEER_SESSION="+connected.Session.ID)
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
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
	for _, secret := range []string{terminalApp.token, terminalApp.windowActivationToken, editorApp.token, editorApp.windowActivationToken} {
		if bytes.Contains(output, []byte(secret)) || strings.Contains(got, secret) {
			t.Fatal("a token appeared in the browser run or reached the device")
		}
	}
}
