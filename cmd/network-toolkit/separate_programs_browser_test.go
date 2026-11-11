//go:build linux

package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"networktoolkit/internal/terminal"
)

// The shipped pages against the real separate programs, built from this
// checkout as build.sh builds them: the separate Terminal program holds a real
// Telnet session to a synthetic loopback device, the Text Editor is opened
// through Network_Toolkit.exe --editor-standalone (forwarded to its separate
// program), and peer send between the two programs works. Then the Text
// Editor is closed and one Send to editor in the Terminal starts the separate
// Text Editor program itself (launchPeerApp), whose page receives the text.
// Chromium stands in for the app windows; no native Windows claim.
func TestSeparateProgramsBrowser(t *testing.T) {
	if os.Getenv("NTK_SEPARATE_PROGRAMS_BROWSER") != "1" {
		t.Skip("requires the isolated Playwright browser")
	}
	programs := t.TempDir()
	buildSeparatePrograms(t, programs)
	main, editor, terminalProgram := filepath.Join(programs, programName("")), filepath.Join(programs, programName(standaloneEditorMode)), filepath.Join(programs, programName("terminal"))
	home := t.TempDir()
	env := append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, ".config"), "XDG_DATA_HOME="+filepath.Join(home, ".local", "share"))
	root := filepath.Join(home, ".config", "NetworkToolkit")
	terminalDir, editorDir := peerWorkspaceDir(root, "terminal"), peerWorkspaceDir(root, standaloneEditorMode)
	for dir, page := range map[string]string{terminalDir: "terminal", editorDir: "editor"} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		settings := `{"activePage":"` + page + `","theme":"light","welcomeDismissed":true,"firstLaunchWelcome":{"dismissed":true},"appUpdatePreferences":{"enabled":false},"terminalInteraction":{"pasteReview":true,"copyOnSelect":false,"rightClick":"menu"},"editorPreferences":{"restoreSession":false}}`
		if err := os.WriteFile(filepath.Join(dir, "Network_Toolkit.settings.json"), []byte(settings), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// The synthetic device of peer_send_browser_test.go.
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
	kill := func(pid int) {
		_ = syscall.Kill(pid, syscall.SIGINT)
	}

	runQuiet(t, env, terminalProgram, "--no-browser")
	terminalRecord, exe := waitForRecord(t, terminalDir, "")
	defer kill(terminalRecord.PID)
	if exe != terminalProgram {
		t.Fatalf("the Terminal runs in %s", exe)
	}
	created := pageCall(t, terminalRecord.Host, http.MethodPost, "/api/terminals", `{"type":"telnet","host":"127.0.0.1","port":`+strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)+`,"cols":100,"rows":28}`)
	var connected struct {
		Session terminal.SessionInfo `json:"session"`
	}
	if err := json.Unmarshal(created, &connected); err != nil || connected.Session.ID == "" {
		t.Fatalf("connect the synthetic device: %s", created)
	}
	runQuiet(t, env, main, "--editor-standalone", "--no-browser").wait(t, 20*time.Second)
	editorRecord, exe := waitForRecord(t, editorDir, "")
	defer func() {
		if record, err := readPeerInstance(editorDir); err == nil {
			kill(record.PID)
		}
	}()
	if exe != editor {
		t.Fatalf("Network_Toolkit.exe --editor-standalone runs in %s, want the separate program", exe)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "node", "scripts/separate-programs.browser.cjs")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(),
		"NTK_SEPARATE_TERMINAL_URL=http://"+terminalRecord.Host, "NTK_SEPARATE_EDITOR_URL=http://"+editorRecord.Host,
		"NTK_SEPARATE_SESSION="+connected.Session.ID, "NTK_SEPARATE_EDITOR_RECORD="+filepath.Join(editorDir, terminalInstanceFilename))
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	// The selected line reached the device once, with no Enter added.
	if got := device.input(); got != "show version" {
		t.Fatalf("the device received %q", got)
	}
	// The Text Editor that the Terminal started is the separate program.
	started, exe := waitForRecord(t, editorDir, editorRecord.InstanceID)
	if exe != editor {
		t.Fatalf("peer send started %s, want the separate Text Editor program", exe)
	}
	if strings.Contains(string(output), started.Token) || strings.Contains(string(output), terminalRecord.Token) {
		t.Fatal("an activation token appeared in the browser run")
	}
}
