//go:build windows

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"networktoolkit/internal/desktop"
	"networktoolkit/internal/terminal"
)

// Full production main and detached Terminal WORKSPACE pages, authentic scoped
// child bootstrap, retained loopback SSH shell/SFTP and real Explorer/OLE input.
// The single-session child intentionally has no Files sidebar and is not faked.
// Existing Editor/tool acceptance remains TestNativeSFTPDirect121InstalledWebView2.
func TestNativeTerminalSFTP123InstalledWebView2(t *testing.T) {
	if os.Getenv("NETWORK_TOOLKIT_NATIVE_TERMINAL_SFTP123_TEST") != "1" {
		t.Skip("explicit isolated hosted Windows Terminal SFTP acceptance only")
	}
	phase := os.Getenv("NETWORK_TOOLKIT_NATIVE_TERMINAL_SFTP123_PHASE")
	if phase != "baseline122" && phase != "candidate123" {
		t.Fatal("explicit baseline122 or candidate123 phase required")
	}
	baseline := phase == "baseline122"
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	private, err := os.MkdirTemp("", "network-toolkit-terminal-sftp123-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(private)
	source := filepath.Join(private, "source")
	for _, p := range []string{source, filepath.Join(source, "tree", "empty"), filepath.Join(source, "tree", "nested")} {
		if err = os.MkdirAll(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	contents := map[string][]byte{"one.txt": []byte("SYNTHETIC TERMINAL123 FILE\n"), "tree/nested/two.txt": []byte("SYNTHETIC TERMINAL123 TREE\n"), "slow.bin": bytes.Repeat([]byte("SYNTHETIC TERMINAL123 SLOW\n"), 400000)}
	for name, data := range contents {
		if err = os.WriteFile(filepath.Join(source, filepath.FromSlash(name)), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var slow atomic.Bool
	var marker atomic.Value
	marker.Store("")
	var accepted atomic.Int32
	port, fingerprint, closeSSH := terminalSFTP123Server(t, ctx, source, &slow, &marker, &accepted)
	defer closeSSH()
	a := newApplication(ctx, filepath.Join(private, "owner-data", "Network_Toolkit.settings.json"))
	defer a.stop()
	defer a.terminals.Close()
	defer a.closeBrowserConnections()
	a.uiHost = "webview2"
	a.state = json.RawMessage(`{"activePage":"terminal","theme":"dark","appUpdatePreferences":{"enabled":false},"terminalSidebarCollapsed":false,"terminalSidebarWidth":300,"terminalSidebarSide":"left"}`)
	session, err := a.connectTerminalForRequest(httptest.NewRequest("POST", "http://fixture.invalid/api/terminals", nil), terminal.ConnectRequest{Type: "ssh", Host: "127.0.0.1", Port: port, Username: "synthetic123", Password: "SYNTHETIC_NOT_A_SECRET", AuthMethod: "password", ExpectedHostFingerprint: fingerprint, Cols: 80, Rows: 24, TimeoutMs: 5000})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		s, exists := a.terminalSession(session.ID)
		if exists && s.State == "connected" {
			break
		}
		if !exists || s.State == "error" || time.Now().After(deadline) {
			t.Fatalf("real SSH terminal preflight: %+v", s)
		}
		if s.Prompt != nil {
			t.Fatal("backend fingerprint pin unexpectedly requested user trust")
		}
		time.Sleep(20 * time.Millisecond)
	}
	executable := filepath.Join(private, "Network_Toolkit.terminal-sftp123.exe")
	build := exec.CommandContext(ctx, "go", "build", "-mod=vendor", "-trimpath", "-ldflags=-H=windowsgui", "-o", executable, "./cmd/network-toolkit")
	build.Dir = root
	if out, e := build.CombinedOutput(); e != nil {
		t.Fatalf("production Terminal child build: %v %s", e, out)
	}
	driver, err := os.ReadFile(filepath.Join(root, "scripts", "terminal-sftp123-native-driver.js"))
	if err != nil {
		t.Fatal(err)
	}
	output := os.Getenv("TOOLKIT_NATIVE_TEST_OUTPUT")
	if output == "" {
		output = filepath.Join(private, "evidence")
	}
	if err = os.MkdirAll(output, 0700); err != nil {
		t.Fatal(err)
	}
	h := &nativeTerminalFixture{nativeEditorFixture: &nativeEditorFixture{ctx: ctx, app: a, root: root, executable: executable, output: output, driver: driver, tokens: map[string]string{}, children: map[string]*nativeEditorChild{}}, commands: map[string]chan nativeEditorCommand{}, results: map[uint64]chan nativeTerminalReply{}, session: session.ID}
	a.launchTerminalView = h.launch
	defer h.cleanup()
	var finished atomic.Value
	finished.Store("")
	var activeCase atomic.Value
	activeCase.Store("")
	var mu sync.Mutex
	var actions, transitions []map[string]any
	var nativeTrace []desktop.NativeFileDragObservation
	var materialized atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/remote-browser" && r.Method == "POST" || r.URL.Path == "/api/remote-browser/drag-ticket" {
			body, e := io.ReadAll(io.LimitReader(r.Body, 64<<10))
			var request remoteBrowserRequest
			if e != nil || json.Unmarshal(body, &request) != nil || request.SessionID != session.ID || request.ConnectionID != "" || browserRequestHasLogin(request) || request.LocalPath != "" {
				http.Error(w, "outside retained synthetic terminal", 403)
				return
			}
			switch request.Action {
			case "probe", "list", "stat", "preview", "read-text":
			default:
				http.Error(w, "outside synthetic read-only SFTP", 403)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			a.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/explorer-transfer" || r.URL.Path == "/api/explorer-transfer/native" {
			var request explorerTransferRequest
			if r.Method == "POST" {
				body, e := io.ReadAll(io.LimitReader(r.Body, 64<<10))
				if e != nil || json.Unmarshal(body, &request) != nil {
					http.Error(w, "invalid synthetic native request", 400)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
				if request.Action == "prepare" {
					http.Error(w, "fixture forbids prerequisite Prepare/export", 403)
					return
				}
				if request.Action == "direct" && (request.Binding.Kind != "terminal" || request.Binding.SessionID != session.ID) {
					http.Error(w, "outside own Terminal binding", 403)
					return
				}
				if request.Action == "materialize" {
					materialized.Add(1)
				}
				mu.Lock()
				actions = append(actions, map[string]any{"case": activeCase.Load(), "action": request.Action, "tickMS": sftp122TickMS()})
				mu.Unlock()
			}
			a.ServeHTTP(w, r)
			if request.Action != "" {
				event := map[string]any{"case": activeCase.Load(), "action": request.Action, "tickMS": sftp122TickMS()}
				a.explorerTransfers.mu.Lock()
				if j := a.explorerTransfers.jobs[request.ID]; j != nil {
					event["state"] = j.state
					event["note"] = j.note
					event["owner"] = j.owner
					event["stagePresent"] = j.stage != ""
				}
				a.explorerTransfers.mu.Unlock()
				mu.Lock()
				transitions = append(transitions, event)
				mu.Unlock()
				if request.Action == "finish" {
					if p := finished.Load().(string); p != "" {
						_ = os.WriteFile(p, []byte("finished"), 0600)
					}
				}
			}
			return
		}
		h.serve(w, r)
	}))
	defer server.Close()
	a.host = strings.TrimPrefix(server.URL, "http://")
	report := map[string]any{"passed": false, "phase": phase, "fullProductionTerminalPage": true, "productionDetachedWorkspaceBootstrap": true, "actualExplorer": true, "realRetainedSSHAndSFTP": true, "syntheticTemporaryFilesOnly": true, "ordinaryGestures": "unchanged121 Normal Win32 timings; negative control alone real 1500ms hover", "baselineScope": "one main Terminal file; exact owned Save dialog cancelled without save"}
	for _, what := range []string{"HEAD", "HEAD^{tree}"} {
		cmd := exec.CommandContext(ctx, "git", "rev-parse", what)
		cmd.Dir = root
		out, e := cmd.Output()
		if e != nil {
			t.Fatal(e)
		}
		report["source_"+what] = strings.TrimSpace(string(out))
	}
	if baseline && report["source_HEAD"] != "60b02cae25c1bfaf14d0c15a038706d193c99a37" {
		t.Fatal("negative control must overlay test-only files on exact original122 source 60b02cae")
	}
	if baseline {
		// The exact original production tree must remain intact; only these
		// new fixture files may be overlaid (untracked or staged).
		check := exec.CommandContext(ctx, "git", "diff", "--exit-code", "HEAD", "--", ".", ":(exclude)cmd/network-toolkit/terminal_sftp123_windows_test.go", ":(exclude)scripts/native-terminal-sftp123.ps1", ":(exclude)scripts/terminal-sftp123-native-driver.js", ":(exclude)scripts/terminal-sftp123-native-fixture.test.cjs")
		check.Dir = root
		if out, e := check.CombinedOutput(); e != nil {
			t.Fatalf("original122 tracked production tree was modified: %v %s", e, out)
		}
		report["baselineProductionTreeUnmodified"] = true
	}
	ready := make(chan struct{})
	done := make(chan error, 1)
	var cases []map[string]any
	activeView := ""
	go func() {
		select {
		case <-ready:
		case <-ctx.Done():
			done <- ctx.Err()
			return
		}
		run := func() error {
			for _, childMode := range []bool{false, true} {
				if baseline && childMode {
					break
				}
				view, label := "", "main"
				if childMode {
					if _, e := h.command("", "detach-workspace"); e != nil {
						return e
					}
					end := time.Now().Add(20 * time.Second)
					for time.Now().Before(end) && view == "" {
						for _, v := range a.views.state().Views {
							if v.Workspace && v.State != "returned" {
								view = v.ID
								break
							}
						}
						if view == "" {
							time.Sleep(30 * time.Millisecond)
						}
					}
					if view == "" {
						return errors.New("real detached Terminal workspace did not launch")
					}
					if _, e := h.wait(view, func(s nativeTerminalSnapshot) bool {
						return s.Session == session.ID && s.Editable && s.Owner.ViewID == view && s.Owner.Ready
					}); e != nil {
						return e
					}
					label = "detached-workspace"
				}
				activeView = view
				pid, exe, e := h.identity(view)
				if e != nil {
					return e
				}
				arrange := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-File", filepath.Join(root, "scripts", "native-terminal-sftp123.ps1"), "-Mode", "Arrange", "-OwnerProcessID", strconv.Itoa(pid), "-ExecutablePath", exe)
				if out, e := arrange.CombinedOutput(); e != nil {
					return fmt.Errorf("fixture arrange: %w %s", e, out)
				}
				initial, e := h.command(view, "setup-sftp")
				if e != nil {
					return e
				}
				if !initial.Editable || initial.Session != session.ID {
					return errors.New("Terminal fixture not own connected session")
				}
				if e = h.capture(view, "terminal-sftp123-"+label+"-ready"); e != nil {
					return e
				}
				type gestureCase struct{ name, path, expected, mode string }
				tests := []gestureCase{{"file", "/one.txt", "one.txt", "Copy"}, {"folder", "/tree", "tree/nested/two.txt", "Copy"}, {"cancel-before", "/one.txt", "", "CancelBefore"}, {"cancel-during", "/slow.bin", "", "CancelDuring"}}
				if baseline {
					tests = []gestureCase{{"original122-save-dialog", "/one.txt", "", "BaselineSaveDialog"}}
				}
				for _, test := range tests {
					name := label + "-" + test.name
					activeCase.Store(name)
					dest := filepath.Join(private, "dest-"+name)
					if e = os.MkdirAll(dest, 0700); e != nil {
						return e
					}
					finish := filepath.Join(private, name+".finished")
					started := filepath.Join(private, name+".materializing")
					finished.Store(finish)
					marker.Store(started)
					slow.Store(test.mode == "CancelDuring")
					before := materialized.Load()
					s, e := h.command(view, "row:"+test.path)
					if e != nil {
						return e
					}
					point, e := sftpDirect121RowPoint(nativeEditorSnapshot{Geometry: s.Geometry}, test.path)
					if e != nil {
						return e
					}
					args := []string{"-NoProfile", "-NonInteractive", "-File", filepath.Join(root, "scripts", "native-terminal-sftp123.ps1"), "-OwnerProcessID", strconv.Itoa(pid), "-ExecutablePath", exe, "-DestinationDirectory", dest, "-FinishedFile", finish, "-MaterializingFile", started, "-Mode", test.mode, "-FromX", fmt.Sprint(point.Left), "-FromY", fmt.Sprint(point.Top), "-FromWidth", fmt.Sprint(s.Geometry.Width), "-FromHeight", fmt.Sprint(s.Geometry.Height)}
					if test.expected != "" {
						args = append(args, "-ExpectedPath", filepath.Join(dest, filepath.FromSlash(test.expected)))
					}
					out, inputErr := exec.CommandContext(ctx, "powershell.exe", args...).CombinedOutput()
					slow.Store(false)
					row := map[string]any{"name": name, "passed": false, "sourcePID": pid, "sourceLocation": s.Location, "sourceGeometry": s.Geometry, "sourceHitTested": true, "materializeRequests": materialized.Load() - before}
					if b, e := os.ReadFile(finish + ".input.json"); e == nil {
						var v any
						if json.Unmarshal(bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf}), &v) == nil {
							row["inputTimeline"] = v
						}
					}
					if inputErr != nil {
						row["error"] = fmt.Sprintf("%v %s", inputErr, out)
						cases = append(cases, row)
						return fmt.Errorf("actual %s: %w %s", name, inputErr, out)
					}
					if !json.Valid(bytes.TrimSpace(out)) {
						return fmt.Errorf("invalid real input evidence: %s", out)
					}
					row["input"] = json.RawMessage(bytes.TrimSpace(out))
					var input struct {
						ActualMouseDrag, ActualExplorer, Finished, ExpectedFailureReproduced, SaveSafelyCancelled bool
						SaveDialogCount, FixtureHostWindowCount                                                   int
						SaveDialogs                                                                               []json.RawMessage
					}
					if e = json.Unmarshal(bytes.TrimSpace(out), &input); e != nil || !input.ActualMouseDrag || !input.ActualExplorer {
						return errors.New("real input/Explorer proof missing")
					}
					if baseline {
						if !input.ExpectedFailureReproduced || !input.SaveSafelyCancelled || len(input.SaveDialogs) != 1 || input.Finished {
							return errors.New("original122 Save dialog negative control not established")
						}
					} else {
						if !input.Finished || input.SaveDialogCount != 0 || input.FixtureHostWindowCount != 1 {
							return errors.New("ordinary copy had a Save/export dialog or extra Toolkit window")
						}
						expectedOwner := ""
						if view != "" {
							expectedOwner = "terminal:" + view
						}
						mu.Lock()
						nativeFinished := false
						for _, event := range transitions {
							if event["case"] == name && event["action"] == "finish" && event["owner"] == expectedOwner {
								nativeFinished = true
								row["nativeTerminalState"] = event["state"]
							}
						}
						mu.Unlock()
						if !nativeFinished {
							return errors.New("authenticated native finish did not belong to exact Terminal window")
						}
					}
					if test.mode == "Copy" {
						for rel, want := range contents {
							if test.name == "file" && rel != "one.txt" || test.name == "folder" && !strings.HasPrefix(rel, "tree/") {
								continue
							}
							got, e := os.ReadFile(filepath.Join(dest, filepath.FromSlash(rel)))
							if e != nil || !bytes.Equal(got, want) {
								return fmt.Errorf("Explorer bytes differ %s/%s: %v", name, rel, e)
							}
						}
						if test.name == "folder" {
							info, e := os.Stat(filepath.Join(dest, "tree", "empty"))
							if e != nil || !info.IsDir() {
								return errors.New("remote empty directory lost")
							}
						}
					} else {
						entries, e := os.ReadDir(dest)
						if e != nil || len(entries) != 0 {
							return errors.New("cancelled destination not empty")
						}
					}
					if (test.mode == "CancelBefore" || baseline) && materialized.Load() != before {
						return errors.New("cancel-before/baseline unexpectedly materialized native source")
					}
					if test.mode == "CancelDuring" {
						if _, e = os.Stat(started); e != nil {
							return errors.New("during cancellation missed actual SFTP bulk response")
						}
						mu.Lock()
						cancelled := false
						for _, v := range transitions {
							if v["case"] == name && v["action"] == "finish" && v["state"] == "cancelled" {
								cancelled = true
							}
						}
						mu.Unlock()
						if !cancelled {
							return errors.New("during-copy did not finish cancelled")
						}
					}
					a.explorerTransfers.mu.Lock()
					pending := false
					for _, j := range a.explorerTransfers.jobs {
						if j.state == "dragging" || j.stage != "" || len(j.paths) > 0 {
							pending = true
						}
					}
					a.explorerTransfers.mu.Unlock()
					if pending {
						return errors.New("finished native drag retained active staging/paths")
					}
					retained, e := h.command(view, "verify-retained")
					if e != nil {
						return e
					}
					row["retainedTerminal"] = retained
					row["passed"] = true
					cases = append(cases, row)
					if accepted.Load() != 1 {
						return errors.New("native copy reconnected retained SSH transport")
					}
				}
				if e = h.capture(view, "terminal-sftp123-"+label+"-completed"); e != nil {
					return e
				}
			}
			for rel, want := range contents {
				got, e := os.ReadFile(filepath.Join(source, filepath.FromSlash(rel)))
				if e != nil || !bytes.Equal(got, want) {
					return errors.New("original synthetic SFTP source mutated")
				}
			}
			if !baseline && len(cases) != 8 {
				return errors.New("Terminal acceptance case set incomplete")
			}
			return nil
		}
		e := run()
		if e != nil && ctx.Err() == nil {
			_ = h.capture(activeView, "terminal-sftp123-failure")
		}
		report["passed"] = e == nil
		report["cases"] = cases
		report["captures"] = h.captures
		report["sshTCPAccepts"] = accepted.Load()
		report["sourceUnchanged"] = e == nil
		mu.Lock()
		report["actions"] = actions
		report["transitions"] = transitions
		report["mainNativeTrace"] = nativeTrace
		mu.Unlock()
		if e != nil {
			report["error"] = e.Error()
		}
		raw, _ := json.MarshalIndent(report, "", "  ")
		_ = os.WriteFile(filepath.Join(output, "native-terminal-sftp123-"+phase+"-results.json"), raw, 0600)
		done <- e
		cancel()
	}()
	err = desktop.Run(ctx, desktop.Config{Title: "Network Toolkit synthetic Terminal SFTP123 acceptance", URL: server.URL + "/", SessionToken: a.token, DataDir: filepath.Join(private, "owner-webview"), Stop: cancel, OnReady: func() { close(ready) }, NativeFileDragObserver: func(event desktop.NativeFileDragObservation) {
		mu.Lock()
		if len(nativeTrace) < 512 {
			nativeTrace = append(nativeTrace, event)
		}
		mu.Unlock()
	}})
	if err != nil {
		t.Errorf("native Terminal: %v", err)
		cancel()
	}
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Terminal SFTP driver unfinished")
	}
}

func terminalSFTP123Server(t *testing.T, ctx context.Context, root string, slow *atomic.Bool, marker *atomic.Value, accepted *atomic.Int32) (int, string, func()) {
	t.Helper()
	// Match production server startup. Windows Temp may use an 8.3 alias;
	// the confinement guard correctly compares resolved paths to this root.
	canonical, err := transferRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	root = canonical
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{PasswordCallback: func(m ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if m.User() != "synthetic123" || string(password) != "SYNTHETIC_NOT_A_SECRET" {
			return nil, io.EOF
		}
		return nil, nil
	}}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	opened := map[net.Conn]bool{}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			tcp, e := listener.Accept()
			if e != nil {
				return
			}
			accepted.Add(1)
			mu.Lock()
			opened[tcp] = true
			mu.Unlock()
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { tcp.Close(); mu.Lock(); delete(opened, tcp); mu.Unlock() }()
				server, channels, requests, e := ssh.NewServerConn(tcp, config)
				if e != nil {
					return
				}
				defer server.Close()
				go func() {
					for req := range requests {
						if req.WantReply {
							_ = req.Reply(true, nil)
						}
					}
				}()
				for request := range channels {
					if request.ChannelType() != "session" {
						_ = request.Reject(ssh.UnknownChannelType, "session only")
						continue
					}
					channel, reqs, e := request.Accept()
					if e != nil {
						continue
					}
					wg.Add(1)
					go func() {
						defer wg.Done()
						defer channel.Close()
						for req := range reqs {
							switch req.Type {
							case "pty-req", "window-change":
								if req.WantReply {
									_ = req.Reply(true, nil)
								}
								continue
							case "shell":
								if req.WantReply {
									_ = req.Reply(true, nil)
								}
								_, _ = io.WriteString(channel, "SYNTHETIC TERMINAL123 READY\r\nfixture> ")
								// Keep the shell alive without executing input or changing output.
								go func() { _, _ = io.Copy(io.Discard, channel) }()
								continue
							}
							var sub struct{ Name string }
							if req.Type != "subsystem" || ssh.Unmarshal(req.Payload, &sub) != nil || sub.Name != "sftp" {
								_ = req.Reply(false, nil)
								continue
							}
							_ = req.Reply(true, nil)
							wrapped := &sftpDirect121Channel{Channel: channel, slow: slow, marker: marker, ctx: ctx}
							host := &transferHost{root: root, access: "readwrite", overwrite: false}
							(&sftpSession{channel: wrapped, server: host, root: root, access: "readwrite", handles: map[string]*sftpHandleState{}}).serve()
							return
						}
					}()
				}
			}()
		}
	}()
	cleanup := func() {
		listener.Close()
		mu.Lock()
		for c := range opened {
			c.Close()
		}
		mu.Unlock()
		done := make(chan struct{})
		go func() { wg.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("synthetic SFTP workers did not stop")
		}
	}
	return listener.Addr().(*net.TCPAddr).Port, ssh.FingerprintSHA256(signer.PublicKey()), cleanup
}
