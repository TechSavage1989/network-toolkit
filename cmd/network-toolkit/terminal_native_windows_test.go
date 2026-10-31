//go:build windows

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
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

	"networktoolkit/internal/desktop"
	"networktoolkit/internal/terminal"
)

// The only transport is an isolated loopback server emitting fixed synthetic
// text. No shell, device, credentials or real application connection is used.
// The owner and children are actual native WebView2 with production bootstrap.
func TestNativeTerminalTransferInstalledWebView2(t *testing.T) {
	if os.Getenv("NETWORK_TOOLKIT_NATIVE_TERMINAL_TEST") != "1" {
		t.Skip("explicit hosted Windows native Terminal acceptance only")
	}
	budget := 4 * time.Minute
	profile130 := os.Getenv("NETWORK_TOOLKIT_NATIVE_PROFILE130") == "1"
	if os.Getenv("NETWORK_TOOLKIT_NATIVE_RESIZE129") == "1" {
		budget = 6 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	private, err := os.MkdirTemp("", "network-toolkit-terminal114-native-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for i := 0; i < 100; i++ {
			if os.RemoveAll(private) == nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Error("synthetic Terminal fixture folder remained locked")
	}()
	executable := filepath.Join(private, "Network_Toolkit.native-terminal114.exe")
	build := exec.CommandContext(ctx, "go", "build", "-trimpath", "-ldflags=-H=windowsgui", "-o", executable, "./cmd/network-toolkit")
	build.Dir = root
	if b, e := build.CombinedOutput(); e != nil {
		t.Fatalf("build production Terminal child: %v\n%s", e, b)
	}
	driver, err := os.ReadFile(filepath.Join(root, "scripts", "terminal-native-driver.js"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Join(private, "owner-data"), 0700); err != nil {
		t.Fatal(err)
	}
	a := newApplication(ctx, filepath.Join(private, "owner-data", "Network_Toolkit.settings.json"))
	defer a.stop()
	defer a.terminals.Close()
	defer a.transfers.Close()
	a.uiHost = "webview2"
	a.state = json.RawMessage(`{"activePage":"terminal","theme":"dark","appUpdatePreferences":{"enabled":false},"terminalProfiles":[{"id":"native-profile114","name":"Native saved <west>","type":"telnet","host":"127.0.0.1"}]}`)
	output := os.Getenv("TOOLKIT_NATIVE_TEST_OUTPUT")
	if output == "" {
		output = filepath.Join(private, "evidence")
	}
	if err = os.MkdirAll(output, 0700); err != nil {
		t.Fatal(err)
	}
	h := &nativeTerminalFixture{nativeEditorFixture: &nativeEditorFixture{ctx: ctx, app: a, root: root, executable: executable, output: output, driver: driver, tokens: map[string]string{}, children: map[string]*nativeEditorChild{}}, commands: map[string]chan nativeEditorCommand{}, results: map[uint64]chan nativeTerminalReply{}, promptReady: make(chan struct{})}
	if os.Getenv("NETWORK_TOOLKIT_NATIVE_RESIZE129") == "1" {
		extra, readErr := os.ReadFile(filepath.Join(root, "scripts", "terminal-resize129-native-driver.js"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		h.driver = append(append(extra, '\n'), h.driver...)
		h.resize129 = &nativeTerminalResize129Evidence{}
	}
	if profile130 {
		extra, readErr := os.ReadFile(filepath.Join(root, "scripts", "terminal-profiles130-native-driver.js"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		h.driver = append(append(extra, '\n'), h.driver...)
		h.profile130 = true
	}
	a.launchTerminalView = h.launch
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var accepted atomic.Int32
	go func() {
		for {
			c, e := listener.Accept()
			if e != nil {
				return
			}
			accepted.Add(1)
			go func() {
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(budget))
				if h.resize129 != nil {
					h.resize129.attach(c)
					_, _ = c.Write([]byte{255, 253, 31}) // IAC DO NAWS; real Telnet size protocol.
				}
				var transcript strings.Builder
				for i := 0; i < 80; i++ {
					fmt.Fprintf(&transcript, "SYNTHETIC %03d\r\n", i)
				}
				transcript.WriteString("Synthetic prompt pending\r\n")
				_, _ = io.WriteString(c, transcript.String())
				select {
				case <-h.promptReady:
					_, _ = io.WriteString(c, "7300(config-if)# ")
				case <-ctx.Done():
					return
				}
				if h.resize129 != nil {
					h.resize129.read(c)
				} else {
					_, _ = io.Copy(io.Discard, c)
				}
			}()
		}
	}()
	// Match the actual synthetic endpoint so Saved label propagation is exercised too.
	a.state = json.RawMessage(fmt.Sprintf(`{"activePage":"terminal","theme":"dark","appUpdatePreferences":{"enabled":false},"terminalProfiles":[{"id":"native-profile114","name":"Native saved <west>","type":"telnet","host":"127.0.0.1","port":%d}]}`, listener.Addr().(*net.TCPAddr).Port))
	if profile130 {
		a.state = bytes.Replace(a.state, []byte(`"name":"Native saved <west>"`), []byte(`"name":"127.0.0.1","nameExplicit":false`), 1)
		a.state = bytes.Replace(a.state, []byte(`"terminalProfiles":`), []byte(fmt.Sprintf(`"terminalRecent":[{"id":"native-recent130","name":"127.0.0.1","nameExplicit":false,"type":"telnet","host":"127.0.0.1","port":%d}],"terminalProfiles":`, listener.Addr().(*net.TCPAddr).Port)), 1)
	}
	session, err := a.connectTerminalForRequest(httptest.NewRequest(http.MethodPost, "http://fixture.invalid/api/terminals", nil), terminal.ConnectRequest{Type: "telnet", ProfileID: "native-profile114", Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port})
	if err != nil {
		t.Fatal(err)
	}
	h.session = session.ID
	server := httptest.NewServer(http.HandlerFunc(h.serve))
	defer server.Close()
	a.host = strings.TrimPrefix(server.URL, "http://")
	defer h.cleanup()
	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		select {
		case <-ready:
		case <-ctx.Done():
			done <- ctx.Err()
			return
		}
		var e error
		if profile130 {
			e = h.exerciseProfiles130(&accepted)
		} else {
			e = h.exercise(&accepted)
		}
		if e == nil && !profile130 && h.resize129 != nil {
			e = h.exerciseResize129(&accepted)
		}
		if e == nil && !profile130 && os.Getenv("NETWORK_TOOLKIT_NATIVE_TERMINAL_GESTURES") == "1" {
			e = h.exerciseTerminalGestures(&accepted)
		}
		if e != nil && ctx.Err() == nil {
			_ = h.capture("", "terminal114-failure-owner")
		}
		h.writeTerminalReport(e, accepted.Load())
		done <- e
		cancel()
	}()
	err = desktop.Run(ctx, desktop.Config{Title: "Network Toolkit synthetic Terminal acceptance", URL: server.URL + "/", SessionToken: a.token, DataDir: filepath.Join(private, "owner-webview"), Stop: cancel, OnReady: func() { close(ready) }})
	if err != nil {
		t.Errorf("native Terminal owner: %v", err)
	}
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Terminal acceptance driver did not finish")
	}
	t.Logf("Actual three-cycle Terminal timing and ownership evidence: %s", output)
}

type nativeTerminalSnapshot struct {
	Profiles130                                                            *nativeProfiles130Snapshot       `json:"profiles130"`
	Resize129                                                              *nativeTerminalResize129Snapshot `json:"resize129"`
	TabPrimary, TabAddress, SavedProfileName, LearnedName                  string
	SavedPrimary, SavedAddress, ManualName                                 string
	NameAboveIP, SmallerIP, TabVisible, SingleSessionChild                 bool
	NameExplicit, SavedNameExplicit, SavedNameAboveIP, SavedSmallerIP      bool
	NameRevision, SavedNameRevision                                        uint64
	ViewportDOM                                                            float64
	ReorderPointsVisible                                                   bool
	GeometryFits                                                           bool
	Viewport                                                               int
	Startup                                                                int
	NativeCapability                                                       bool
	Location, View, Session, State, Transcript, Selection, Label, TabLabel string
	Editable                                                               bool
	ReadyAt, ActionAt                                                      int64
	OutputBlocks                                                           json.RawMessage
	Owner                                                                  terminalViewOwner
	Geometry                                                               nativeEditorGeometry
	Errors                                                                 []string
}
type nativeTerminalReply struct {
	Kind, Error string
	ID          uint64
	State       nativeTerminalSnapshot
}
type nativeTerminalTiming struct {
	Cycle                                      int
	Direction, Session, View, TranscriptSHA256 string
	Milliseconds                               int64
	Owner                                      terminalViewOwner
	SourceEditable                             bool
	OutputBlocks                               json.RawMessage
}
type nativeTerminalFixture struct {
	*nativeEditorFixture
	commandMu      sync.Mutex
	commands       map[string]chan nativeEditorCommand
	results        map[uint64]chan nativeTerminalReply
	session        string
	promptReady    chan struct{}
	labelsVerified bool
	profile130     bool
	resize129      *nativeTerminalResize129Evidence
	allowBulk      atomic.Bool
	timings        []nativeTerminalTiming
}

func (h *nativeTerminalFixture) terminalQueue(view string) chan nativeEditorCommand {
	h.commandMu.Lock()
	defer h.commandMu.Unlock()
	if h.commands[view] == nil {
		h.commands[view] = make(chan nativeEditorCommand, 1)
	}
	return h.commands[view]
}
func (h *nativeTerminalFixture) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/__native_terminal_fixture" {
		h.automation(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/public-ip") || strings.HasPrefix(r.URL.Path, "/api/updates/check") {
		jsonError(w, 503, "External lookup disabled in synthetic native fixture.")
		return
	}
	if !(h.profile130 && r.Method == http.MethodPost && (r.URL.Path == "/api/terminals/"+h.session+"/disconnect" || r.URL.Path == "/api/terminals/"+h.session+"/reconnect")) && r.Method != http.MethodGet && r.Method != http.MethodHead && r.URL.Path != "/api/state" && !strings.HasPrefix(r.URL.Path, "/api/terminal-views") && !(strings.HasPrefix(r.URL.Path, "/api/terminals/") && strings.HasSuffix(r.URL.Path, "/resize")) && !(r.Method == http.MethodPost && r.URL.Path == "/api/terminals/"+h.session+"/rename") && r.URL.Path != "/api/editor-views/current/activity" && !(h.allowBulk.Load() && (r.Method == http.MethodPost && r.URL.Path == "/api/terminals/"+h.session+"/disconnect" || r.Method == http.MethodDelete && r.URL.Path == "/api/terminals/"+h.session)) {
		h.mu.Lock()
		h.blocked = append(h.blocked, r.Method+" "+r.URL.Path)
		h.mu.Unlock()
		jsonError(w, 403, "Operation outside synthetic Terminal acceptance.")
		return
	}
	if r.Method == http.MethodGet && (r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/terminal-view/")) {
		recorded := httptest.NewRecorder()
		h.app.ServeHTTP(recorded, r)
		for n, v := range recorded.Header() {
			w.Header()[n] = v
		}
		w.Header().Del("Content-Length")
		w.WriteHeader(recorded.Code)
		body := recorded.Body.Bytes()
		if recorded.Code == 200 {
			body = bytes.Replace(body, []byte("</body>"), append(append([]byte("<script>"), h.driver...), []byte("</script></body>")...), 1)
		}
		_, _ = w.Write(body)
		return
	}
	h.app.ServeHTTP(w, r)
}
func (h *nativeTerminalFixture) automation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.Host != h.app.host || r.Header.Get("Origin") != "http://"+h.app.host || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, "fixture origin/method rejected", 403)
		return
	}
	token := r.Header.Get("X-Session-Token")
	view, valid := "", token == h.app.token
	if !valid {
		h.mu.Lock()
		view, valid = h.tokens[token]
		h.mu.Unlock()
	}
	if !valid {
		http.Error(w, "native fixture capability required", 401)
		return
	}
	var reply nativeTerminalReply
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&reply) != nil {
		http.Error(w, "invalid fixture payload", 400)
		return
	}
	switch reply.Kind {
	case "next":
		select {
		case c := <-h.terminalQueue(view):
			jsonResponse(w, 200, c)
		case <-time.After(100 * time.Millisecond):
			jsonResponse(w, 200, map[string]any{})
		case <-h.ctx.Done():
			jsonResponse(w, 200, map[string]any{})
		}
	case "result":
		h.commandMu.Lock()
		ch := h.results[reply.ID]
		h.commandMu.Unlock()
		if ch != nil {
			select {
			case ch <- reply:
			default:
			}
		}
		jsonResponse(w, 200, map[string]any{})
	case "fatal":
		h.mu.Lock()
		h.failures = append(h.failures, view+": "+reply.Error)
		h.mu.Unlock()
		jsonResponse(w, 200, map[string]any{})
	default:
		http.Error(w, "unknown fixed fixture action", 400)
	}
}
func (h *nativeTerminalFixture) launch(ctx context.Context, launch terminalViewLaunch) (<-chan error, error) {
	raw, e := encodeTerminalViewLaunch(launch)
	if e != nil {
		return nil, e
	}
	cmd := exec.Command(h.executable, terminalViewChildArguments(false)...)
	done, e := startTerminalViewChild(ctx, cmd, raw)
	if e != nil {
		return nil, e
	}
	child := &nativeEditorChild{command: cmd, done: make(chan struct{})}
	h.mu.Lock()
	h.tokens[launch.ViewToken] = launch.ViewID
	h.children[launch.ViewID] = child
	h.mu.Unlock()
	observed := make(chan error, 1)
	go func() { child.err = <-done; close(child.done); observed <- child.err; close(observed) }()
	return observed, nil
}
func (h *nativeTerminalFixture) command(view, action string) (nativeTerminalSnapshot, error) {
	c := nativeEditorCommand{ID: h.sequence.Add(1), Action: action, Destination: h.session}
	ch := make(chan nativeTerminalReply, 1)
	h.commandMu.Lock()
	h.results[c.ID] = ch
	h.commandMu.Unlock()
	defer func() { h.commandMu.Lock(); delete(h.results, c.ID); h.commandMu.Unlock() }()
	select {
	case h.terminalQueue(view) <- c:
	case <-h.ctx.Done():
		return nativeTerminalSnapshot{}, h.ctx.Err()
	}
	select {
	case reply := <-ch:
		if reply.Error != "" {
			return reply.State, errors.New(reply.Error)
		}
		s := reply.State
		path := "/"
		if view != "" {
			path = "/terminal-view/" + view
		}
		if s.Startup != 1 || !s.NativeCapability || s.Location != path || s.View != view || len(s.Errors) > 0 {
			return s, fmt.Errorf("native Terminal capability/startup failure: %+v", s)
		}
		return s, nil
	case <-time.After(30 * time.Second):
		h.mu.Lock()
		failures := append([]string{}, h.failures...)
		h.mu.Unlock()
		return nativeTerminalSnapshot{}, fmt.Errorf("native %s/%s timeout: %v", view, action, failures)
	case <-h.ctx.Done():
		return nativeTerminalSnapshot{}, h.ctx.Err()
	}
}
func (h *nativeTerminalFixture) wait(view string, predicate func(nativeTerminalSnapshot) bool) (nativeTerminalSnapshot, error) {
	until := time.Now().Add(25 * time.Second)
	var last nativeTerminalSnapshot
	for time.Now().Before(until) {
		s, e := h.command(view, "snapshot")
		if e != nil {
			return s, e
		}
		last = s
		if predicate(s) {
			return s, nil
		}
		time.Sleep(30 * time.Millisecond)
	}
	return last, fmt.Errorf("native Terminal transition not reached: %+v", last)
}
func (h *nativeTerminalFixture) exercise(accepted *atomic.Int32) error {
	initial, e := h.command("", "select")
	if e != nil {
		return e
	}
	initial, e = h.wait("", func(s nativeTerminalSnapshot) bool {
		return s.Editable && s.GeometryFits && strings.Contains(s.Transcript, "SYNTHETIC 079")
	})
	if e != nil {
		return e
	}
	if !initial.TabVisible || initial.Label != "Native saved <west>" || initial.SavedProfileName != "Native saved <west>" || initial.TabPrimary != "Native saved <west>" || initial.TabAddress != "127.0.0.1" || !initial.NameAboveIP || !initial.SmallerIP {
		return fmt.Errorf("Initial manual name precedence or geometry failed: %+v", initial)
	}
	close(h.promptReady)
	initial, e = h.wait("", func(s nativeTerminalSnapshot) bool {
		return s.LearnedName == "7300" && s.TabPrimary == "Native saved <west>" && s.TabAddress == "127.0.0.1" && s.TabVisible && s.NameAboveIP && s.SmallerIP && s.GeometryFits
	})
	if e != nil {
		return e
	}
	if os.Getenv("NETWORK_TOOLKIT_NATIVE_TERMINAL_SELECTION") == "1" {
		initial, e = h.command("", "selection")
		if e != nil {
			return e
		}
		if initial.Selection == "" || initial.Viewport != 10 || math.Abs(initial.ViewportDOM-10) > .01 {
			return errors.New("synthetic selected output and scroll position were not established")
		}
	}
	if initial.Label != "Native saved <west>" || initial.SavedProfileName != "Native saved <west>" {
		return fmt.Errorf("saved profile name was changed by learned display: %q/%q", initial.Label, initial.SavedProfileName)
	}
	expected := initial.Transcript
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(expected)))
	manualName := "Native saved <west>"
	storedName := manualName
	for cycle := 1; cycle <= 3; cycle++ {
		action, e := h.command("", "detach")
		if e != nil {
			return e
		}
		var view string
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			for _, v := range h.app.views.state().Views {
				if v.SessionID == h.session && v.State != "returned" {
					view = v.ID
					break
				}
			}
			if view != "" {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if view == "" {
			return errors.New("detach did not create native child")
		}
		child, e := h.wait(view, func(s nativeTerminalSnapshot) bool {
			return s.Session == h.session && s.Editable && s.GeometryFits && s.Transcript == expected
		})
		if e != nil {
			return e
		}
		source, e := h.wait("", func(s nativeTerminalSnapshot) bool { return !s.Editable })
		if e != nil {
			return e
		}
		if child.TabPrimary != manualName || child.LearnedName != "7300" || child.TabAddress != "127.0.0.1" || !child.SingleSessionChild || child.TabVisible || child.Label != manualName {
			return fmt.Errorf("single-session child hidden tab metadata changed: %+v", child)
		}
		if source.Editable {
			return errors.New("source remains editable after child accepts")
		}
		if os.Getenv("NETWORK_TOOLKIT_NATIVE_TERMINAL_SELECTION") == "1" && (child.Selection != initial.Selection || child.Viewport != initial.Viewport || math.Abs(child.ViewportDOM-float64(initial.Viewport)) > .01) {
			return fmt.Errorf("detached selection/scroll changed: selected=%q viewport=%d DOM=%g wanted=%q/%d", child.Selection, child.Viewport, child.ViewportDOM, initial.Selection, initial.Viewport)
		}
		owner := h.app.views.state().Owners[h.session]
		if owner.ViewID != view || !owner.Ready {
			return errors.New("backend did not grant child ownership")
		}
		if child.ReadyAt < action.ActionAt {
			return errors.New("invalid destination readiness timestamp")
		}
		h.timings = append(h.timings, nativeTerminalTiming{cycle, "detach", h.session, view, digest, child.ReadyAt - action.ActionAt, owner, source.Editable, child.OutputBlocks})
		if cycle == 1 {
			if e = h.capture(view, "terminal114-detached"); e != nil {
				return e
			}
		}
		// Mutate names while ownership stays in the other window, reproducing the
		// stale-profile race rather than renaming only after returning home.
		previousRevision := child.NameRevision
		explicit := true
		switch cycle {
		case 1:
			_, e = h.command(view, "rename")
			manualName, storedName = "Native renamed <east>", "Native renamed <east>"
		case 2:
			_, e = h.command("", "profile-rename")
			manualName, storedName = "Native profile C", "Native profile C"
		case 3:
			_, e = h.command(view, "clear-name")
			manualName, explicit = "7300", false
		}
		if e != nil {
			return e
		}
		wantManual := manualName
		if !explicit {
			wantManual = ""
		}
		child, e = h.wait(view, func(s nativeTerminalSnapshot) bool {
			return s.Editable && s.Transcript == expected && s.TabPrimary == manualName && s.LearnedName == "7300" && s.TabAddress == "127.0.0.1" && s.NameExplicit == explicit && s.ManualName == wantManual && s.NameRevision > previousRevision && s.SingleSessionChild && !s.TabVisible
		})
		if e != nil {
			return e
		}
		if _, e = h.command("", "show-saved"); e != nil {
			return e
		}
		source, e = h.wait("", func(s nativeTerminalSnapshot) bool {
			return !s.Editable && s.SavedProfileName == storedName && s.SavedNameExplicit == explicit && s.SavedNameRevision == child.NameRevision && s.SavedPrimary == manualName && s.SavedAddress == "127.0.0.1" && s.SavedNameAboveIP && s.SavedSmallerIP
		})
		if e != nil {
			return e
		}
		if os.Getenv("NETWORK_TOOLKIT_NATIVE_TERMINAL_SELECTION") == "1" && (child.Selection != initial.Selection || child.Viewport != initial.Viewport || math.Abs(child.ViewportDOM-float64(initial.Viewport)) > .01) {
			return errors.New("cross-window naming changed child selection or scroll")
		}
		if currentOwner := h.app.views.state().Owners[h.session]; currentOwner.ViewID != view || !currentOwner.Ready {
			return errors.New("cross-window naming changed terminal ownership")
		}

		action, e = h.command(view, "reattach")
		if e != nil {
			return e
		}
		home, e := h.wait("", func(s nativeTerminalSnapshot) bool {
			return s.Session == h.session && s.Editable && s.GeometryFits && s.Transcript == expected && s.ReadyAt >= action.ActionAt
		})
		if e != nil {
			return e
		}
		if os.Getenv("NETWORK_TOOLKIT_NATIVE_TERMINAL_SELECTION") == "1" && (home.Selection != initial.Selection || home.Viewport != initial.Viewport || math.Abs(home.ViewportDOM-float64(initial.Viewport)) > .01) {
			return fmt.Errorf("returned selection/scroll changed: selected=%q viewport=%d DOM=%g wanted=%q/%d", home.Selection, home.Viewport, home.ViewportDOM, initial.Selection, initial.Viewport)
		}
		if home.TabPrimary != manualName || home.LearnedName != "7300" || home.TabAddress != "127.0.0.1" || !home.TabVisible || !home.NameAboveIP || !home.SmallerIP || home.SavedProfileName != storedName || home.NameExplicit != explicit || home.SavedNameExplicit != explicit || home.NameRevision != child.NameRevision || home.SavedNameRevision != child.NameRevision || home.ManualName != wantManual {
			return fmt.Errorf("returned manual display, learned metadata or saved name changed: %+v", home)
		}
		owner = h.app.views.state().Owners[h.session]
		if owner.ViewID != "" || !owner.Ready {
			return errors.New("backend did not accept home ownership")
		}
		h.timings = append(h.timings, nativeTerminalTiming{cycle, "reattach", h.session, view, digest, home.ReadyAt - action.ActionAt, owner, false, home.OutputBlocks})
		if accepted.Load() != 1 {
			return fmt.Errorf("transfer reconnected transport: accepts=%d", accepted.Load())
		}
		info, ok := h.app.terminalSession(h.session)
		if !ok || info.State != "connected" {
			return errors.New("original terminal transport no longer connected")
		}
	}
	h.labelsVerified = true
	return h.capture("", "terminal114-returned")
}
func (h *nativeTerminalFixture) writeTerminalReport(err error, accepted int32) {
	h.mu.Lock()
	defer h.mu.Unlock()
	failure := ""
	if err != nil {
		failure = err.Error()
	}
	raw, _ := json.MarshalIndent(map[string]any{"passed": err == nil, "error": failure, "label": os.Getenv("TOOLKIT_NATIVE_TIMING_LABEL"), "loopbackAcceptCount": accepted, "savedLabelVerified": h.labelsVerified, "manualPrecedenceNumericLearningRenameClearRevisionAndTwoLineIPVerified": h.labelsVerified, "selectionScrollRequired": os.Getenv("NETWORK_TOOLKIT_NATIVE_TERMINAL_SELECTION") == "1", "sessionID": h.session, "timings": h.timings, "gestures": h.gestures, "phases": h.phases, "captures": h.captures, "blocked": h.blocked, "failures": h.failures, "limitations": []string{"Hosted synthetic loopback text only; no actual network device or shell", "Selection/scroll are asserted only with NETWORK_TOOLKIT_NATIVE_TERMINAL_SELECTION=1; nonempty command/pager archives remain covered separately", "Timings include actual process/WebView startup; no cross-machine performance guarantee"}}, "", "  ")
	_ = os.WriteFile(filepath.Join(h.output, "native-terminal114-report.json"), raw, 0600)
}
