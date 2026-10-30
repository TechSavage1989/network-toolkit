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
	"syscall"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"networktoolkit/internal/desktop"
)

// The only sources/destinations are temporary synthetic files. Full production
// index/scripts, the real backend, a retained loopback SSH transport and genuine
// Windows mouse/OLE input run together; no DOM drag events or debug port.
func TestNativeSFTPDirect121InstalledWebView2(t *testing.T) {
	if os.Getenv("NETWORK_TOOLKIT_NATIVE_SFTP121_TEST") != "1" {
		t.Skip("explicit hosted synthetic full-app SFTP/Explorer acceptance only")
	}
	diagnostic122 := os.Getenv("NETWORK_TOOLKIT_NATIVE_DRAG122_DIAGNOSTIC") == "1"
	phase122 := os.Getenv("NETWORK_TOOLKIT_NATIVE_DRAG122_PHASE")
	if diagnostic122 && phase122 != "baseline121" && phase122 != "candidate122" {
		t.Fatal("explicit baseline121 or candidate122 phase required")
	}
	timeout := 6 * time.Minute
	if diagnostic122 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	private, err := os.MkdirTemp("", "network-toolkit-sftp121-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(private)
	traceDir122 := filepath.Join(private, "child-trace122")
	if diagnostic122 {
		if err = os.Mkdir(traceDir122, 0700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("NETWORK_TOOLKIT_NATIVE_SFTP122_TEST", "1")
		t.Setenv("NETWORK_TOOLKIT_NATIVE_DRAG_TRACE122", traceDir122)
	}
	source := filepath.Join(private, "source")
	local := filepath.Join(private, "local")
	for _, p := range []string{source, local, filepath.Join(source, "tree", "empty"), filepath.Join(source, "tree", "nested")} {
		if err = os.MkdirAll(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	contents := map[string][]byte{"one.txt": []byte("SYNTHETIC DIRECT121 FILE\n"), "tree/nested/two.txt": []byte("SYNTHETIC DIRECT121 TREE\n"), "slow.bin": bytes.Repeat([]byte("SYNTHETIC DIRECT121 SLOW\n"), 400000)}
	for name, data := range contents {
		if err = os.WriteFile(filepath.Join(source, filepath.FromSlash(name)), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	a := newApplication(ctx, filepath.Join(private, "owner-data", "Network_Toolkit.settings.json"))
	defer a.stop()
	defer a.closeBrowserConnections()
	a.uiHost = "webview2"
	a.state = json.RawMessage(`{"activePage":"editor","appUpdatePreferences":{"enabled":false},"editorFilesPreferences":{"width":300,"panel":"files","side":"remote","collapsed":false}}`)
	var slow atomic.Bool
	var marker atomic.Value
	marker.Store("")
	port, fingerprint, closeSSH := sftpDirect121Server(t, ctx, source, &slow, &marker)
	defer closeSSH()
	connection, err := a.openBrowserConnection(httptest.NewRequest("POST", "/api/remote-browser", nil), remoteBrowserRequest{Protocol: "ssh", Host: "127.0.0.1", Port: port, Username: "synthetic121", Password: "SYNTHETIC_NOT_A_SECRET", AuthMode: "password", Fingerprint: fingerprint})
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := a.runRemoteBrowserOperation(ctx, remoteBrowserRequest{Protocol: "ssh", ConnectionID: connection, Action: "read-text", Path: "/one.txt"})
	if err != nil {
		t.Fatalf("real retained SFTP preflight read: %v", err)
	}
	data, _ := preflight.Data.(map[string]any)
	bytesRead, _ := data["contentBytes"].([]byte)
	if !bytes.Equal(bytesRead, contents["one.txt"]) {
		t.Fatal("real retained SFTP preflight bytes differ")
	}
	executable := filepath.Join(private, "Network_Toolkit.sftp121.exe")
	buildArgs := []string{"build", "-trimpath", "-ldflags=-H=windowsgui", "-o", executable}
	if diagnostic122 {
		buildArgs = append(buildArgs, "-tags", "toolkit_native_drag_trace122")
	}
	buildArgs = append(buildArgs, "./cmd/network-toolkit")
	build := exec.CommandContext(ctx, "go", buildArgs...)
	build.Dir = root
	if out, e := build.CombinedOutput(); e != nil {
		t.Fatalf("production child build: %v %s", e, out)
	}
	driver, err := os.ReadFile(filepath.Join(root, "scripts", "sftp-direct121-native-driver.js"))
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
	h := &nativeEditorFixture{ctx: ctx, app: a, root: root, executable: executable, output: output, driver: driver, queues: map[string]chan nativeEditorCommand{}, replies: map[uint64]chan nativeEditorReply{}, tokens: map[string]string{}, children: map[string]*nativeEditorChild{}}
	a.launchEditorView = h.launch
	defer h.cleanup()
	var finishFile atomic.Value
	finishFile.Store("")
	var mu sync.Mutex
	var actions []string
	var activeCase atomic.Value
	activeCase.Store("")
	var transitions []map[string]any
	var nativeTrace []desktop.NativeFileDragObservation
	var materialized atomic.Int32
	within := func(p string) bool {
		rel, e := filepath.Rel(local, p)
		return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/transfer/local" {
			q := r.URL.Query()
			p := q.Get("path")
			if p == "" {
				p = local
				q.Set("path", p)
				r.URL.RawQuery = q.Encode()
			}
			if r.Method != "GET" || !within(p) {
				http.Error(w, "outside synthetic local fixture", 403)
				return
			}
			a.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/remote-browser" && r.Method == "POST" {
			body, e := io.ReadAll(io.LimitReader(r.Body, 64<<10))
			var req remoteBrowserRequest
			if e != nil || json.Unmarshal(body, &req) != nil || req.ConnectionID != connection || browserRequestHasLogin(req) {
				http.Error(w, "outside retained loopback connection", 403)
				return
			}
			switch req.Action {
			case "list", "stat", "probe", "read-text", "download":
			default:
				http.Error(w, "outside synthetic read-only operation", 403)
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
					http.Error(w, "bad synthetic request", 400)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
				mu.Lock()
				actions = append(actions, request.Action)
				transitions = append(transitions, map[string]any{"tickMS": sftp122TickMS(), "case": activeCase.Load(), "phase": "request", "action": request.Action, "outcome": request.Outcome, "gesture": request.Gesture})
				mu.Unlock()
				if request.Action == "prepare" {
					http.Error(w, "fixture forbids prerequisite Prepare copy", 403)
					return
				}
				if request.Action == "materialize" {
					materialized.Add(1)
				}
			}
			a.ServeHTTP(w, r)
			if request.Action != "" {
				detail := map[string]any{"tickMS": sftp122TickMS(), "case": activeCase.Load(), "phase": "response", "action": request.Action, "outcome": request.Outcome}
				a.explorerTransfers.mu.Lock()
				if j := a.explorerTransfers.jobs[request.ID]; j != nil {
					detail["state"] = j.state
					detail["note"] = j.note
					detail["items"] = j.entries
					detail["bytes"] = j.bytes
					detail["renderStarted"] = j.renderStarted
					detail["rendered"] = j.rendered
					detail["cancelRequested"] = j.cancelRequested
					detail["stagePresent"] = j.stage != ""
				}
				a.explorerTransfers.mu.Unlock()
				mu.Lock()
				transitions = append(transitions, detail)
				mu.Unlock()
			}
			if request.Action == "finish" {
				if p := finishFile.Load().(string); p != "" {
					_ = os.WriteFile(p, []byte("finished"), 0600)
				}
			}
			return
		}
		h.serve(w, r)
	}))
	defer server.Close()
	a.host = strings.TrimPrefix(server.URL, "http://")
	report := map[string]any{"passed": false, "fullProductionPage": true, "actualRetainedLoopbackSFTP": true, "preflightRetainedRead": true, "actualWindowsExplorer": true, "syntheticTemporaryFilesOnly": true, "nativeInput": "Win32 pointer down/move/up; no DOM drag event substitution"}
	report["diagnostic122"] = diagnostic122
	report["phase122"] = phase122
	if diagnostic122 {
		for _, what := range []string{"HEAD", "HEAD^{tree}"} {
			cmd := exec.CommandContext(ctx, "git", "rev-parse", what)
			cmd.Dir = root
			if out, e := cmd.Output(); e == nil {
				report["source_"+what] = strings.TrimSpace(string(out))
			} else {
				t.Fatal(e)
			}
		}
		report["childInstrumentation"] = "toolkit_native_drag_trace122 build tag; typed path-free observer only; production admission decisions unchanged"
	}
	activeView := ""
	var caseFailures []string
	var cases []map[string]any
	var attempts []map[string]any
	ready := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		select {
		case <-ready:
		case <-ctx.Done():
			result <- ctx.Err()
			return
		}
		run := func() error {
			if e := h.nativeInput("Arrange", "Tab", "", "", nativeEditorSnapshot{}, nativeEditorSnapshot{}, nativeEditorRect{}, nativeEditorRect{}); e != nil {
				return e
			}
			for _, childMode := range []bool{false, true} {
				view, label := "", "main"
				if childMode {
					req := editorTransferRequest{ID: randomHex(16), Destination: "new", Batch: editorFixtureBatch()}
					out, e := a.editorViews.offer("", req)
					if e != nil {
						return e
					}
					view = out["destination"].(string)
					label = "child"
					if _, e = h.waitState(view, "real child acknowledged", func(s nativeEditorSnapshot) bool { return s.Window.Owned[editorFixtureUID] >= 2 && !s.Window.Running }); e != nil {
						return e
					}
					if e = a.editorViews.complete("", req.ID); e != nil {
						return e
					}
					token := a.editorViews.views[view].token
					if r := editorHTTP(a, token, "/api/editor-views/current/explorer", map[string]any{}); r.Code != 200 {
						return fmt.Errorf("synthetic child access request: %d %s", r.Code, r.Body.String())
					}
					nonce := a.editorViews.views[view].explorer.RequestID
					if r := editorHTTP(a, a.token, "/api/editor-views/explorer/"+view, map[string]any{"requestId": nonce, "allow": true}); r.Code != 200 {
						return fmt.Errorf("explicit fixture approval: %d %s", r.Code, r.Body.String())
					}
					choices := editorGET(a, token, "/api/remote-browser")
					var approved struct {
						Connections []struct {
							ID string `json:"id"`
						} `json:"connections"`
					}
					if choices.Code != 200 || json.Unmarshal(choices.Body.Bytes(), &approved) != nil {
						return fmt.Errorf("authenticated child approved-transport metadata: %d %s", choices.Code, choices.Body.String())
					}
					found := false
					for _, item := range approved.Connections {
						if item.ID == connection {
							found = true
						}
					}
					if !found {
						return errors.New("authenticated child grant omitted retained synthetic SFTP connection")
					}
					report["childApprovedConnectionVisible"] = true
					if e = h.nativeInput("Arrange", "Tab", view, view, nativeEditorSnapshot{}, nativeEditorSnapshot{}, nativeEditorRect{}, nativeEditorRect{}); e != nil {
						return e
					}
				}
				activeView = view
				pid, exe, e := h.identity(view)
				if e != nil {
					return e
				}
				arrange := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-File", filepath.Join(root, "scripts", "native-sftp-direct121.ps1"), "-Mode", "Arrange", "-OwnerProcessID", strconv.Itoa(pid), "-ExecutablePath", exe)
				if out, e := arrange.CombinedOutput(); e != nil {
					return fmt.Errorf("visible fixture window arrangement: %w %s", e, out)
				}
				if _, e := h.command(view, nativeEditorCommand{Action: "setup-sftp", Destination: connection}); e != nil {
					return e
				}
				if e := h.capture(view, "sftp-direct121-"+label+"-ready"); e != nil {
					return e
				}
				type gestureCase struct{ name, kind, path, expected, mode, speed string }
				tests := []gestureCase{{"file", "file", "/one.txt", "one.txt", "Copy", "Normal"}, {"folder", "folder", "/tree", "tree/nested/two.txt", "Copy", "Normal"}, {"cancel-before", "cancel-before", "/one.txt", "", "CancelBefore", "Normal"}, {"cancel-during", "cancel-during", "/slow.bin", "", "CancelDuring", "Normal"}}
				if diagnostic122 {
					tests = nil
					for repeat := 1; repeat <= 2; repeat++ {
						for _, speed := range []string{"Fast", "Normal", "Slow"} {
							tests = append(tests, gestureCase{fmt.Sprintf("file-%s-%d", strings.ToLower(speed), repeat), "file", "/one.txt", "one.txt", "Copy", speed})
						}
					}
					tests = append(tests, gestureCase{"folder", "folder", "/tree", "tree/nested/two.txt", "Copy", "Normal"}, gestureCase{"cancel-before", "cancel-before", "/one.txt", "", "CancelBefore", "Normal"}, gestureCase{"cancel-during", "cancel-during", "/slow.bin", "", "CancelDuring", "Normal"})
				}
				capturedFailure := false
				for _, test := range tests {
					name := label + "-" + test.name
					activeCase.Store(name)
					caseStart := sftp122TickMS()
					runCase := func() error {
						dest := filepath.Join(private, "dest-"+name)
						if e := os.MkdirAll(dest, 0700); e != nil {
							return e
						}
						finished := filepath.Join(private, name+".finished")
						started := filepath.Join(private, name+".materializing")
						finishFile.Store(finished)
						marker.Store(started)
						slow.Store(test.mode == "CancelDuring")
						before := materialized.Load()
						mu.Lock()
						transitionStart := len(transitions)
						mu.Unlock()
						state, e := h.command(view, nativeEditorCommand{Action: "row", Destination: test.path})
						if e != nil {
							return e
						}
						point, e := sftpDirect121RowPoint(state, test.path)
						if e != nil {
							return e
						}
						pid, exe, e := h.identity(view)
						if e != nil {
							return e
						}
						args := []string{"-NoProfile", "-NonInteractive", "-File", filepath.Join(root, "scripts", "native-sftp-direct121.ps1"), "-OwnerProcessID", strconv.Itoa(pid), "-ExecutablePath", exe, "-DestinationDirectory", dest, "-FinishedFile", finished, "-MaterializingFile", started, "-Mode", test.mode, "-Speed", test.speed, "-FromX", fmt.Sprint(point.Left), "-FromY", fmt.Sprint(point.Top), "-FromWidth", fmt.Sprint(state.Geometry.Width), "-FromHeight", fmt.Sprint(state.Geometry.Height)}
						if test.expected != "" {
							args = append(args, "-ExpectedPath", filepath.Join(dest, filepath.FromSlash(test.expected)))
						}
						cmd := exec.CommandContext(ctx, "powershell.exe", args...)
						out, e := cmd.CombinedOutput()
						attempt := map[string]any{"name": name, "sourcePID": pid, "caseStartTickMS": caseStart, "commandEndTickMS": sftp122TickMS(), "speed": test.speed, "sourceGeometry": state.Geometry, "sourceHitTested": true}
						if b, readErr := os.ReadFile(finished + ".input.json"); readErr == nil {
							var v any
							if json.Unmarshal(bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf}), &v) == nil {
								attempt["destinationInput"] = v
							}
						}
						attempts = append(attempts, attempt)
						slow.Store(false)
						if e != nil {
							return fmt.Errorf("actual %s gesture: %w %s", name, e, out)
						}
						if !json.Valid(bytes.TrimSpace(out)) {
							return fmt.Errorf("native evidence not JSON: %s", out)
						}
						terminalState, terminalNote := "", ""
						mu.Lock()
						for _, transition := range transitions[transitionStart:] {
							if transition["phase"] == "response" && transition["action"] == "finish" {
								terminalState, _ = transition["state"].(string)
								terminalNote, _ = transition["note"].(string)
							}
						}
						mu.Unlock()
						if test.mode == "CancelDuring" && (terminalState != "cancelled" || !strings.Contains(terminalNote, "cancelled")) {
							return fmt.Errorf("explicit materialization cancellation ended with state %q: %s", terminalState, terminalNote)
						}
						if test.mode == "Copy" {
							for rel, want := range contents {
								if test.kind == "file" && rel != "one.txt" || test.kind == "folder" && !strings.HasPrefix(rel, "tree/") {
									continue
								}
								got, e := os.ReadFile(filepath.Join(dest, filepath.FromSlash(rel)))
								if e != nil || !bytes.Equal(got, want) {
									return fmt.Errorf("Explorer destination bytes differ for %s: %v", name, e)
								}
							}
							if test.kind == "folder" {
								if info, e := os.Stat(filepath.Join(dest, "tree", "empty")); e != nil || !info.IsDir() {
									return errors.New("empty remote directory lost")
								}
							}
						} else {
							entries, e := os.ReadDir(dest)
							if e != nil || len(entries) != 0 {
								return errors.New("cancelled destination is not empty")
							}
						}
						if test.mode == "CancelBefore" && materialized.Load() != before {
							return errors.New("Escape before drop read remote source bytes")
						}
						if test.mode == "CancelDuring" {
							if _, e = os.Stat(started); e != nil {
								return errors.New("during-copy cancellation never observed real bulk SFTP response")
							}
						}
						a.explorerTransfers.mu.Lock()
						pending := false
						for _, j := range a.explorerTransfers.jobs {
							if j.owner == view && (j.state == "dragging" || j.stage != "" || len(j.paths) > 0) {
								pending = true
							}
						}
						a.explorerTransfers.mu.Unlock()
						if pending {
							return errors.New("finished native copy retained active stage or paths")
						}
						if _, e = h.command(view, nativeEditorCommand{Action: "verify-dirty"}); e != nil {
							return e
						}
						cases = append(cases, map[string]any{"name": name, "speed": test.speed, "passed": true, "materializeRequests": materialized.Load() - before, "terminalState": terminalState, "terminalNote": terminalNote, "sourceHitTested": true, "sourceGeometry": state.Geometry, "input": json.RawMessage(bytes.TrimSpace(out))})
						return nil
					}
					if err := runCase(); err != nil {
						cases = append(cases, map[string]any{"name": name, "speed": test.speed, "passed": false, "error": err.Error()})
						if !diagnostic122 || test.mode != "Copy" {
							return err
						}
						// Continue observations only after the genuine native gesture has ended;
						// never cancel/reclaim a live transfer to make the matrix proceed.
						a.explorerTransfers.mu.Lock()
						pending := false
						for _, j := range a.explorerTransfers.jobs {
							if j.owner == view && (j.state == "preparing" || j.state == "ready" || j.state == "claimed" || j.state == "dragging" || j.stage != "" || len(j.paths) > 0) {
								pending = true
							}
						}
						a.explorerTransfers.mu.Unlock()
						if pending {
							return fmt.Errorf("diagnostic copy failed with live transfer; stopping without reclamation: %w", err)
						}
						caseFailures = append(caseFailures, name+": "+err.Error())
						if !capturedFailure {
							_ = h.capture(view, "sftp-direct121-"+label+"-first-failure")
							capturedFailure = true
						}
						if _, e := h.command(view, nativeEditorCommand{Action: "verify-dirty"}); e != nil {
							return e
						}
					}
				}
				activeCase.Store(label + "-row-to-editor")
				state, e := h.command(view, nativeEditorCommand{Action: "row", Destination: "/one.txt"})
				if e != nil {
					return e
				}
				point, e := sftpDirect121RowPoint(state, "/one.txt")
				if e != nil {
					return e
				}
				target := nativeEditorRect{Left: state.Geometry.Editor.Left + state.Geometry.Editor.Width*.65, Top: state.Geometry.Editor.Top + state.Geometry.Editor.Height*.45}
				if e = h.nativeInput("Drag", "Tab", view, view, state, state, point, target); e != nil {
					return e
				}
				if _, e = h.command(view, nativeEditorCommand{Action: "wait-open"}); e != nil {
					return e
				}
				cases = append(cases, map[string]any{"name": label + "-row-to-editor", "passed": true, "dirtyAndUndoRetained": true})
				if e = h.capture(view, "sftp-direct121-"+label+"-opened"); e != nil {
					return e
				}
			}
			for rel, want := range contents {
				got, e := os.ReadFile(filepath.Join(source, filepath.FromSlash(rel)))
				if e != nil || !bytes.Equal(got, want) {
					return errors.New("original SFTP source changed")
				}
			}
			if len(caseFailures) > 0 {
				return fmt.Errorf("%d diagnostic copy gestures failed; see bounded cases", len(caseFailures))
			}
			return nil
		}
		e := run()
		if e != nil {
			_ = h.capture(activeView, "sftp-direct121-failure")
		}
		report["passed"] = e == nil
		report["cases"] = cases
		report["attempts"] = attempts
		report["caseFailures"] = caseFailures
		if diagnostic122 {
			childTraces := map[string][]json.RawMessage{}
			childEventCount := 0
			childAdmissionEvents := 0
			var childDropped uint32
			files, _ := filepath.Glob(filepath.Join(traceDir122, "native-drag122-*.jsonl"))
			for _, file := range files {
				b, e := os.ReadFile(file)
				if e != nil || len(b) > 4<<20 {
					continue
				}
				var events []json.RawMessage
				for _, line := range bytes.Split(b, []byte("\n")) {
					if len(line) > 0 && json.Valid(line) && len(events) < 4160 {
						var typed desktop.NativeFileDragObservation
						if json.Unmarshal(line, &typed) != nil || typed.Kind == "" || typed.TickMS == 0 {
							continue
						}
						if typed.Dropped > childDropped {
							childDropped = typed.Dropped
						}
						if typed.Kind == "claim-received" || typed.Kind == "dispatch-enter" || typed.Kind == "ole-enter" {
							childAdmissionEvents++
						}
						events = append(events, append(json.RawMessage{}, line...))
					}
				}
				childTraces[filepath.Base(file)] = events
				childEventCount += len(events)
			}
			report["childNativeTrace"] = childTraces
			report["childTraceEventCount"] = childEventCount
			report["childTraceAdmissionEvents"] = childAdmissionEvents
			report["childTraceDropped"] = childDropped
			if (childEventCount == 0 || childAdmissionEvents == 0 || childDropped > 0) && report["childApprovedConnectionVisible"] == true {
				report["childTraceMissing"] = true
				if e == nil {
					e = errors.New("diagnostic child observer trace missing or dropped events")
					report["passed"] = false
				}
			}
		}
		report["captures"] = h.captures
		mu.Lock()
		report["actions"] = append([]string{}, actions...)
		report["transitions"] = append([]map[string]any{}, transitions...)
		report["nativeTrace"] = append([]desktop.NativeFileDragObservation{}, nativeTrace...)
		mu.Unlock()
		if e != nil {
			report["error"] = e.Error()
		}
		raw, _ := json.MarshalIndent(report, "", "  ")
		_ = os.WriteFile(filepath.Join(output, "native-sftp-direct121-results.json"), raw, 0600)
		result <- e
		cancel()
	}()
	err = desktop.Run(ctx, desktop.Config{Title: "Network Toolkit synthetic SFTP121 acceptance", URL: server.URL + "/", SessionToken: a.token, DataDir: filepath.Join(private, "webview"), Stop: cancel, NativeFileDragObserver: func(event desktop.NativeFileDragObservation) {
		mu.Lock()
		if len(nativeTrace) < 4096 {
			nativeTrace = append(nativeTrace, event)
		}
		mu.Unlock()
	}, OnReady: func() { close(ready) }})
	if err != nil {
		t.Errorf("native full app: %v", err)
		cancel()
	}
	select {
	case e := <-result:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("native SFTP fixture driver unfinished")
	}
}

func sftpDirect121RowPoint(s nativeEditorSnapshot, path string) (nativeEditorRect, error) {
	for _, r := range s.Geometry.Tabs {
		if r.UID == path && r.Width > 0 && r.Height > 0 {
			return nativeEditorRect{Left: r.Left, Top: r.Top}, nil
		}
	}
	return nativeEditorRect{}, fmt.Errorf("visible real file row absent: %s", path)
}

type sftpDirect121Channel struct {
	ssh.Channel
	slow   *atomic.Bool
	marker *atomic.Value
	ctx    context.Context
}

func (c *sftpDirect121Channel) Write(p []byte) (int, error) {
	if c.slow.Load() && len(p) > 1024 {
		if name := c.marker.Load().(string); name != "" {
			_ = os.WriteFile(name, []byte("actual SFTP bulk response"), 0600)
		}
		select {
		case <-time.After(35 * time.Millisecond):
		case <-c.ctx.Done():
			return 0, c.ctx.Err()
		}
	}
	return c.Channel.Write(p)
}
func sftpDirect121Server(t *testing.T, ctx context.Context, root string, slow *atomic.Bool, marker *atomic.Value) (int, string, func()) {
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
		if m.User() != "synthetic121" || string(password) != "SYNTHETIC_NOT_A_SECRET" {
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

var sftp122TickProc = syscall.NewLazyDLL("kernel32.dll").NewProc("GetTickCount64")

func sftp122TickMS() uint64 { tick, _, _ := sftp122TickProc.Call(); return uint64(tick) }
