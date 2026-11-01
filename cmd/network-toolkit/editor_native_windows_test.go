//go:build windows

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"networktoolkit/internal/desktop"
)

// The owner is the real application backend and installed native WebView2. Each
// destination is a separately built production executable running its actual
// --editor-view CLI, stdin bootstrap, scoped document and heartbeat lifetime.
// Only automation/reporting and bounded failure injection are test-only HTTP
// decorators. There is no production bridge, debug port, browser mock or tag.
func TestNativeEditorTransferInstalledWebView2(t *testing.T) {
	if os.Getenv("NETWORK_TOOLKIT_NATIVE_EDITOR_TEST") != "1" {
		t.Skip("explicit hosted Windows editor two-window acceptance only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	private, err := os.MkdirTemp("", "network-toolkit-editor108-native-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		deadline := time.Now().Add(12 * time.Second)
		for {
			if err := os.RemoveAll(private); err == nil {
				return
			} else if time.Now().After(deadline) {
				t.Errorf("synthetic native editor folder remained locked: %v", err)
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	})
	executable := filepath.Join(private, "Network_Toolkit.native-editor108.exe")
	build := exec.CommandContext(ctx, "go", "build", "-trimpath", "-ldflags=-H=windowsgui", "-o", executable, "./cmd/network-toolkit")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build actual production editor child: %v\n%s", err, output)
	}
	driver, err := os.ReadFile(filepath.Join(root, "scripts", "editor-native-driver.js"))
	if err != nil {
		t.Fatal(err)
	}
	trace, err := os.ReadFile(filepath.Join(root, "scripts", "editor-native-trace128.js"))
	if err != nil {
		t.Fatal(err)
	}
	driver = append(append(trace, []byte("\n")...), driver...)
	data := filepath.Join(private, "owner-data")
	if err := os.MkdirAll(data, 0700); err != nil {
		t.Fatal(err)
	}
	a := newApplication(ctx, filepath.Join(data, "Network_Toolkit.settings.json"))
	defer a.stop()
	defer func() {
		if a.terminals != nil {
			_ = a.terminals.Close()
		}
		if a.transfers != nil {
			a.transfers.Close()
		}
	}()
	a.uiHost = "webview2"
	a.state = json.RawMessage(`{"activePage":"editor","appUpdatePreferences":{"enabled":false}}`)
	output := os.Getenv("TOOLKIT_NATIVE_TEST_OUTPUT")
	if output == "" {
		output = filepath.Join(private, "evidence")
	}
	if err := os.MkdirAll(output, 0700); err != nil {
		t.Fatal(err)
	}
	h := &nativeEditorFixture{ctx: ctx, app: a, root: root, executable: executable, output: output, driver: driver, queues: map[string]chan nativeEditorCommand{}, replies: map[uint64]chan nativeEditorReply{}, tokens: map[string]string{}, children: map[string]*nativeEditorChild{}}
	a.launchEditorView = h.launch
	server := httptest.NewServer(http.HandlerFunc(h.serve))
	defer server.Close()
	a.host = strings.TrimPrefix(server.URL, "http://")
	defer h.cleanup()
	ready := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		select {
		case <-ready:
		case <-ctx.Done():
			result <- errors.New("native owner did not reach visible startup readiness")
			return
		}
		var err error
		if os.Getenv("NETWORK_TOOLKIT_NATIVE_PERFORMANCE114") == "1" {
			err = h.exercisePerformance114()
		} else {
			err = h.exercise()
		}
		if err == nil && os.Getenv("NETWORK_TOOLKIT_NATIVE_PERFORMANCE114") != "1" {
			err = h.exerciseDrag()
		}
		if err == nil && os.Getenv("NETWORK_TOOLKIT_NATIVE_PERFORMANCE114") != "1" {
			err = h.exerciseClose113()
		}
		if err != nil && ctx.Err() == nil {
			_ = h.capture("", "failure-owner")
			h.mu.Lock()
			live := map[string]*nativeEditorChild{}
			for id, child := range h.children {
				live[id] = child
			}
			h.mu.Unlock()
			for id, child := range live {
				select {
				case <-child.done:
				default:
					_ = h.capture(id, "failure-child-"+id[:8])
				}
			}
		}
		h.collectDiagnostics128()
		h.writeReport(err)
		result <- err
		cancel()
	}()
	err = desktop.Run(ctx, desktop.Config{Title: "Network Toolkit synthetic editor acceptance", URL: server.URL + "/", SessionToken: a.token, DataDir: filepath.Join(private, "owner-webview"), RequireCloseApproval: true, Stop: cancel, OnReady: func() { close(ready) }})
	if err != nil {
		cancel()
		t.Errorf("actual owner WebView2: %v", err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("native editor acceptance driver did not complete")
	}
	t.Logf("Actual native owner + production child CLI transfer/merge, cancellation and failed-launch acceptance: %s", output)
}

type nativeEditorCommand struct {
	ID          uint64   `json:"id"`
	Action      string   `json:"action"`
	Mode        string   `json:"mode,omitempty"`
	Destination string   `json:"destination,omitempty"`
	Texts       []string `json:"texts,omitempty"`
	Enabled     bool     `json:"enabled"`
	Size        int      `json:"size,omitempty"`
}
type nativeEditorDocument struct {
	ID, UID, Path, Source, Hash, Content, Original, Saved, EOL, Encoding, Language string
	SavedEOL, SavedEncoding                                                        string
	Dirty, BOM, SavedBOM, Untitled, Frozen, Staged, Saving, SaveFailed             bool
	Undo, Redo                                                                     []string
	View, CompareView                                                              *editorSelection
	Binding                                                                        editorBinding
	EditRevision                                                                   uint64
}
type nativeEditorSnapshot struct {
	Documents []nativeEditorDocument
	Active    string
	Window    struct {
		Enabled, Running, DragPreparing, Closing bool
		Self, Error                              string
		Owned                                    map[string]uint64
		Outgoing, Incoming                       int
	}
	Textarea struct {
		Value      string
		Disabled   bool
		Start, End int
		Direction  string
		Top, Left  float64
	}
	Diagnostic json.RawMessage
	Geometry   nativeEditorGeometry
	Drag       struct {
		Armed, Started, Ready bool
		UID                   string
	}
	CloseDialog, AskSaveBeforeClose bool
	Message                         string
	Startup                         int
	NativeCapability                bool
	Location                        string
	Errors                          []string
}
type nativeEditorRect struct{ Left, Top, Width, Height float64 }
type nativeEditorTabRect struct {
	UID string
	nativeEditorRect
}
type nativeEditorGeometry struct {
	Width, Height             float64
	Main, Editor, DetachPoint nativeEditorRect
	Tabs                      []nativeEditorTabRect
}
type nativeEditorReply struct {
	Kind, Error string
	ID          uint64
	State       nativeEditorSnapshot
}
type nativeEditorChild struct {
	command *exec.Cmd
	done    chan struct{}
	err     error
}
type nativeEditorFixture struct {
	ctx                       context.Context
	app                       *application
	root, executable, output  string
	driver                    []byte
	mu                        sync.Mutex
	queues                    map[string]chan nativeEditorCommand
	replies                   map[uint64]chan nativeEditorReply
	tokens                    map[string]string
	children                  map[string]*nativeEditorChild
	gate                      chan struct{}
	blocked, phases, failures []string
	captures                  []json.RawMessage
	sequence                  atomic.Uint64
	failLaunch                atomic.Bool
	failedLaunches            atomic.Int32
	pointerDrags              atomic.Int32
	dragComplete              bool
	closePreferenceComplete   bool
	gestures                  []json.RawMessage
	brokerTrace128            []map[string]any
	finalTrace128             []map[string]any
}

func (h *nativeEditorFixture) queue(view string) chan nativeEditorCommand {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.queues[view] == nil {
		h.queues[view] = make(chan nativeEditorCommand, 1)
	}
	return h.queues[view]
}

func (h *nativeEditorFixture) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/__native_editor_fixture" {
		h.automation(w, r)
		return
	}
	if action := nativeEditorBrokerAction128(r.URL.Path); action != "" {
		status := &nativeEditorStatus128{ResponseWriter: w, status: 200}
		w = status
		defer func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if len(h.brokerTrace128) < 256 {
				h.brokerTrace128 = append(h.brokerTrace128, map[string]any{"at": time.Now().UnixMilli(), "action": action, "method": r.Method, "status": status.status})
			}
		}()
	}
	// No public-address lookup, update check, scan, terminal connection, native
	// helper, credential read/write or real file operation is needed by this test.
	if strings.HasPrefix(r.URL.Path, "/api/public-ip") || strings.HasPrefix(r.URL.Path, "/api/updates/check") {
		jsonError(w, http.StatusServiceUnavailable, "External lookup disabled in isolated native fixture.")
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/file-editor/") ||
		strings.HasPrefix(r.URL.Path, "/api/editor-views/documents/") && strings.HasSuffix(r.URL.Path, "/file") ||
		r.Method != http.MethodGet && r.Method != http.MethodHead && r.URL.Path != "/api/state" && !strings.HasPrefix(r.URL.Path, "/api/editor-views/") {
		h.mu.Lock()
		h.blocked = append(h.blocked, r.Method+" "+r.URL.Path)
		h.mu.Unlock()
		jsonError(w, http.StatusForbidden, "Operation outside synthetic native editor acceptance.")
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/editor-views/transfers/") && strings.HasSuffix(r.URL.Path, "/accept") {
		h.mu.Lock()
		gate := h.gate
		h.mu.Unlock()
		if gate != nil {
			select {
			case <-gate:
			case <-r.Context().Done():
				return
			case <-h.ctx.Done():
				return
			}
		}
	}
	if r.Method == http.MethodGet && (r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/editor-view/")) {
		recorded := httptest.NewRecorder()
		h.app.ServeHTTP(recorded, r)
		for name, values := range recorded.Header() {
			w.Header()[name] = values
		}
		w.Header().Del("Content-Length")
		w.WriteHeader(recorded.Code)
		body := recorded.Body.Bytes()
		if recorded.Code == http.StatusOK {
			body = bytes.Replace(body, []byte("</body>"), append(append([]byte("<script>"), h.driver...), []byte("</script></body>")...), 1)
		}
		_, _ = w.Write(body)
		return
	}
	h.app.ServeHTTP(w, r)
}

func (h *nativeEditorFixture) automation(w http.ResponseWriter, r *http.Request) {
	// This endpoint exists only in this _test.go server. It cannot be reached
	// from a production executable's HTTP server or an unbootstrapped HTTP page.
	if r.Method != http.MethodPost || r.Host != h.app.host || r.Header.Get("Origin") != "http://"+h.app.host || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, "fixture origin/method rejected", http.StatusForbidden)
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
		http.Error(w, "native fixture capability required", http.StatusUnauthorized)
		return
	}
	var reply nativeEditorReply
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&reply); err != nil {
		http.Error(w, "invalid fixture payload", http.StatusBadRequest)
		return
	}
	switch reply.Kind {
	case "next":
		select {
		case command := <-h.queue(view):
			jsonResponse(w, 200, command)
		case <-time.After(250 * time.Millisecond):
			jsonResponse(w, 200, map[string]any{})
		case <-h.ctx.Done():
			jsonResponse(w, 200, map[string]any{})
		}
	case "result":
		h.mu.Lock()
		channel := h.replies[reply.ID]
		h.mu.Unlock()
		if channel != nil {
			select {
			case channel <- reply:
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
		http.Error(w, "unknown fixed fixture action", http.StatusBadRequest)
	}
}

func (h *nativeEditorFixture) launch(ctx context.Context, launch editorViewLaunch) (<-chan error, error) {
	raw, err := encodeEditorViewLaunch(launch)
	if err != nil {
		return nil, err
	}
	executable := h.executable
	if h.failLaunch.Load() {
		executable = filepath.Join(filepath.Dir(executable), "does-not-exist", "Network_Toolkit.exe")
	}
	command := exec.Command(executable, editorViewChildArguments(false)...)
	// Same production private pipe/start/wait path. The only launch substitution
	// is choosing the built GUI EXE instead of the Go test runner executable.
	done, err := startEditorViewChild(ctx, command, raw)
	if err != nil {
		if h.failLaunch.Load() {
			h.failedLaunches.Add(1)
		}
		return nil, err
	}
	child := &nativeEditorChild{command: command, done: make(chan struct{})}
	h.mu.Lock()
	h.tokens[launch.ViewToken] = launch.ViewID
	h.children[launch.ViewID] = child
	h.mu.Unlock()
	observed := make(chan error, 1)
	go func() {
		child.err = <-done
		close(child.done)
		observed <- child.err
		close(observed)
	}()
	return observed, nil
}

func (h *nativeEditorFixture) command(view string, command nativeEditorCommand) (nativeEditorSnapshot, error) {
	command.ID = h.sequence.Add(1)
	channel := make(chan nativeEditorReply, 1)
	h.mu.Lock()
	h.replies[command.ID] = channel
	h.mu.Unlock()
	defer func() { h.mu.Lock(); delete(h.replies, command.ID); h.mu.Unlock() }()
	select {
	case h.queue(view) <- command:
	case <-h.ctx.Done():
		return nativeEditorSnapshot{}, h.ctx.Err()
	}
	select {
	case reply := <-channel:
		if reply.Error != "" {
			return reply.State, errors.New(reply.Error)
		}
		expectedPath := "/"
		if view != "" {
			expectedPath = "/editor-view/" + view
		}
		if reply.State.Startup != 1 || !reply.State.NativeCapability || reply.State.Window.Self != view || reply.State.Location != expectedPath || len(reply.State.Errors) != 0 {
			return reply.State, fmt.Errorf("native document startup/capability/JS failure: %+v", reply.State)
		}
		return reply.State, nil
	case <-time.After(35 * time.Second):
		h.mu.Lock()
		failures := append([]string{}, h.failures...)
		h.mu.Unlock()
		return nativeEditorSnapshot{}, fmt.Errorf("native %q command %s timed out; startup errors: %v", view, command.Action, failures)
	case <-h.ctx.Done():
		return nativeEditorSnapshot{}, h.ctx.Err()
	}
}

func (h *nativeEditorFixture) waitState(view, description string, predicate func(nativeEditorSnapshot) bool) (nativeEditorSnapshot, error) {
	deadline := time.Now().Add(18 * time.Second)
	var state nativeEditorSnapshot
	for time.Now().Before(deadline) {
		var err error
		state, err = h.command(view, nativeEditorCommand{Action: "snapshot"})
		if err != nil {
			return state, fmt.Errorf("%s: %w", description, err)
		}
		if predicate(state) {
			return state, nil
		}
		time.Sleep(80 * time.Millisecond)
	}
	return state, fmt.Errorf("%s not reached: %+v", description, state)
}

func nativeEditorDoc(state nativeEditorSnapshot, uid string) (nativeEditorDocument, bool) {
	for _, d := range state.Documents {
		if d.UID == uid {
			return d, true
		}
	}
	return nativeEditorDocument{}, false
}

func nativeEditorEquivalent(before, after nativeEditorDocument) error {
	before.Frozen, before.Staged, after.Frozen, after.Staged = false, false, false, false
	if !reflect.DeepEqual(before, after) {
		// Report the actual differing fields (including selection values, not
		// pointer addresses) so hosted failures identify the preserved-state gap.
		a, b := reflect.ValueOf(before), reflect.ValueOf(after)
		differences := map[string]any{}
		for i := 0; i < a.NumField(); i++ {
			if !reflect.DeepEqual(a.Field(i).Interface(), b.Field(i).Interface()) {
				differences[a.Type().Field(i).Name] = map[string]any{"before": a.Field(i).Interface(), "after": b.Field(i).Interface()}
			}
		}
		data, _ := json.Marshal(differences)
		return fmt.Errorf("transferred document differs (text/dirty/history/selection/metadata): %s", data)
	}
	return nil
}

func (h *nativeEditorFixture) holdAcceptance() func() {
	gate := make(chan struct{})
	h.mu.Lock()
	h.gate = gate
	h.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			h.mu.Lock()
			if h.gate == gate {
				h.gate = nil
			}
			close(gate)
			h.mu.Unlock()
		})
	}
}

func (h *nativeEditorFixture) transfer(source, destination, mode string, expected []nativeEditorDocument, active string) (string, nativeEditorSnapshot, error) {
	return h.transferAction(source, destination, expected, active, func() error {
		_, err := h.command(source, nativeEditorCommand{Action: "move", Mode: mode, Destination: destination})
		return err
	})
}
func (h *nativeEditorFixture) transferAction(source, destination string, expected []nativeEditorDocument, active string, action func() error, checks ...nativeEditorCloseTransferCheck) (string, nativeEditorSnapshot, error) {
	release := h.holdAcceptance()
	defer release()
	if err := action(); err != nil {
		return "", nativeEditorSnapshot{}, err
	}
	actualDestination := destination
	if destination == "main" {
		actualDestination = ""
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		h.app.editorViews.mu.Lock()
		grant := h.app.editorViews.documents[expected[0].UID]
		pending := ""
		if grant != nil {
			pending = grant.pending
		}
		if pending != "" {
			actualDestination = h.app.editorViews.transfers[pending].destination
		}
		h.app.editorViews.mu.Unlock()
		if pending != "" {
			break
		}
		if time.Now().After(deadline) {
			return "", nativeEditorSnapshot{}, errors.New("real backend never recorded pending transfer")
		}
		time.Sleep(50 * time.Millisecond)
	}
	staged, err := h.waitState(actualDestination, "destination stages every document read-only before ACK", func(state nativeEditorSnapshot) bool {
		for _, d := range expected {
			got, found := nativeEditorDoc(state, d.UID)
			if !found || !got.Frozen || !got.Staged {
				return false
			}
		}
		return true
	})
	if err != nil {
		return actualDestination, staged, err
	}
	protected, err := h.command(source, nativeEditorCommand{Action: "snapshot"})
	if err != nil {
		return actualDestination, staged, err
	}
	for _, d := range expected {
		got, found := nativeEditorDoc(protected, d.UID)
		if !found || !got.Frozen || got.Staged {
			return actualDestination, staged, errors.New("source lost/freed/editable document before destination ACK")
		}
		if err := nativeEditorEquivalent(d, got); err != nil {
			return actualDestination, staged, err
		}
		h.app.editorViews.mu.Lock()
		grant := h.app.editorViews.documents[d.UID]
		valid := grant != nil && grant.owner == source && grant.pending != ""
		h.app.editorViews.mu.Unlock()
		if !valid {
			return actualDestination, staged, errors.New("backend moved ownership before ACK")
		}
	}
	if protected.Active == active && !protected.Textarea.Disabled {
		return actualDestination, staged, errors.New("source active textarea remained editable during move")
	}
	if len(checks) > 0 && checks[0].BeforeACK != nil {
		if err := checks[0].BeforeACK(actualDestination); err != nil {
			return actualDestination, staged, err
		}
	}
	release()
	accepted, err := h.waitState(actualDestination, "destination becomes sole editable owner after ACK", func(state nativeEditorSnapshot) bool {
		for _, d := range expected {
			got, found := nativeEditorDoc(state, d.UID)
			if !found || got.Frozen || got.Staged || state.Window.Owned[d.UID] < 2 {
				return false
			}
		}
		return state.Active == active && !state.Textarea.Disabled
	})
	if err != nil {
		return actualDestination, accepted, err
	}
	for _, d := range expected {
		got, _ := nativeEditorDoc(accepted, d.UID)
		if err := nativeEditorEquivalent(d, got); err != nil {
			return actualDestination, accepted, err
		}
		h.app.editorViews.mu.Lock()
		grant := h.app.editorViews.documents[d.UID]
		valid := grant != nil && grant.owner == actualDestination && grant.pending == ""
		h.app.editorViews.mu.Unlock()
		if !valid {
			return actualDestination, accepted, errors.New("ACK did not grant exclusive backend ownership")
		}
	}
	if len(checks) > 0 && checks[0].SourceExits {
		return actualDestination, accepted, h.waitClosed113(source)
	}
	if _, err = h.waitState(source, "source releases moved documents only after ACK", func(state nativeEditorSnapshot) bool {
		for _, d := range expected {
			if _, found := nativeEditorDoc(state, d.UID); found {
				return false
			}
			if state.Window.Owned[d.UID] != 0 {
				return false
			}
		}
		return state.Window.Outgoing == 0
	}); err != nil {
		return actualDestination, accepted, err
	}
	return actualDestination, accepted, nil
}

func (h *nativeEditorFixture) capture(view, phase string) error {
	pid, executable := os.Getpid(), ""
	var err error
	if view == "" {
		executable, err = os.Executable()
	} else {
		h.mu.Lock()
		child := h.children[view]
		h.mu.Unlock()
		if child == nil {
			return errors.New("capture has no actual child process")
		}
		pid, executable = child.command.Process.Pid, h.executable
	}
	if err != nil {
		return err
	}
	command := exec.CommandContext(h.ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-File", filepath.Join(h.root, "scripts", "native-editor-window.ps1"), "-OwnerProcessID", strconv.Itoa(pid), "-ExecutablePath", executable, "-OutputFile", filepath.Join(h.output, "native-editor108-"+phase+".png"))
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exact native HWND capture %s: %w: %s", phase, err, output)
	}
	if !json.Valid(bytes.TrimSpace(output)) {
		return fmt.Errorf("native HWND evidence was not JSON: %s", output)
	}
	h.mu.Lock()
	h.captures = append(h.captures, json.RawMessage(bytes.TrimSpace(output)))
	h.mu.Unlock()
	return nil
}

func (h *nativeEditorFixture) closeEmpty(view string) error {
	if _, err := h.command(view, nativeEditorCommand{Action: "release-empty"}); err != nil {
		return err
	}
	h.mu.Lock()
	child := h.children[view]
	h.mu.Unlock()
	select {
	case <-child.done:
		if child.err != nil {
			return fmt.Errorf("actual child exited unsuccessfully: %w", child.err)
		}
	case <-time.After(12 * time.Second):
		return errors.New("production editor child did not exit after confirmed empty release/heartbeat")
	case <-h.ctx.Done():
		return h.ctx.Err()
	}
	return nil
}

func (h *nativeEditorFixture) exercise() error {
	text := "SYNTHETIC NATIVE EDITOR108 ONLY\n" + strings.Repeat("Synthetic retained document line, no devices or credentials.\n", 100)
	initial, err := h.command("", nativeEditorCommand{Action: "seed", Texts: []string{text + "FIRST", text + "SECOND"}})
	if err != nil {
		return err
	}
	if len(initial.Documents) != 2 || !initial.Documents[0].Dirty || len(initial.Documents[1].Undo) != 2 || initial.Textarea.Start != 11 || initial.Textarea.End != 29 || initial.Textarea.Direction != "backward" {
		return fmt.Errorf("synthetic dirty/undo/selection fixture not initialized: %+v", initial)
	}
	selected, _ := nativeEditorDoc(initial, initial.Active)
	child, _, err := h.transfer("", "new", "tab", []nativeEditorDocument{selected}, selected.UID)
	if err != nil {
		return fmt.Errorf("single-tab native detach: %w", err)
	}
	h.phases = append(h.phases, "single-tab: source frozen + destination staged before ACK; exclusive lease and source release after ACK")
	for _, shot := range []struct{ view, label string }{{"", "single-source"}, {child, "single-child"}} {
		if err := h.capture(shot.view, shot.label); err != nil {
			return err
		}
	}
	edited, err := h.command(child, nativeEditorCommand{Action: "edit-history"})
	if err != nil {
		return err
	}
	selected, _ = nativeEditorDoc(edited, selected.UID)
	_, merged, err := h.transfer(child, "main", "all", []nativeEditorDocument{selected}, selected.UID)
	if err != nil {
		return fmt.Errorf("single-tab native merge back: %w", err)
	}
	if len(merged.Documents) != 2 {
		return errors.New("merge lost the untouched main document")
	}
	if err := nativeEditorEquivalent(initial.Documents[0], merged.Documents[0]); err != nil {
		return err
	}
	if err := h.closeEmpty(child); err != nil {
		return err
	}
	h.phases = append(h.phases, "single-tab merge: text/dirty/undo/redo/selection retained; actual child CLI exits after empty release")
	child, detached, err := h.transfer("", "new", "all", merged.Documents, merged.Active)
	if err != nil {
		return fmt.Errorf("full-editor native detach: %w", err)
	}
	if len(detached.Documents) != 2 {
		return errors.New("full detach did not receive both documents")
	}
	if err := h.capture(child, "full-child"); err != nil {
		return err
	}
	_, merged, err = h.transfer(child, "main", "all", detached.Documents, detached.Active)
	if err != nil {
		return fmt.Errorf("full-editor native merge back: %w", err)
	}
	h.phases = append(h.phases, "full-editor detach and merge: both immutable UIDs/history/text/dirty/view retained across actual native processes")
	if err := h.capture("", "full-merged"); err != nil {
		return err
	}
	// Keep the now-empty actual child open, offer one tab, then cancel while
	// its real accept HTTP request is gated. No ownership response is mocked.
	release := h.holdAcceptance()
	defer release()
	if _, err := h.command("", nativeEditorCommand{Action: "move", Mode: "tab", Destination: child}); err != nil {
		return err
	}
	if _, err := h.waitState(child, "cancel scenario staged read-only", func(state nativeEditorSnapshot) bool {
		return len(state.Documents) == 1 && state.Documents[0].Staged && state.Documents[0].Frozen
	}); err != nil {
		return err
	}
	cancelled, err := h.command("", nativeEditorCommand{Action: "cancel"})
	if err != nil {
		return err
	}
	if len(cancelled.Documents) != len(merged.Documents) || cancelled.Textarea.Disabled {
		return errors.New("cancel did not restore editable source ownership")
	}
	for _, before := range merged.Documents {
		after, found := nativeEditorDoc(cancelled, before.UID)
		if !found {
			return errors.New("cancel lost a source document")
		}
		if err := nativeEditorEquivalent(before, after); err != nil {
			return err
		}
	}
	release()
	if _, err := h.waitState(child, "cancelled unaccepted destination removed", func(state nativeEditorSnapshot) bool { return len(state.Documents) == 0 && state.Window.Incoming == 0 }); err != nil {
		return err
	}
	if err := h.closeEmpty(child); err != nil {
		return err
	}
	h.phases = append(h.phases, "cancel before ACK: source ownership/history retained; staged destination removed; empty actual child exited")
	h.failLaunch.Store(true)
	if _, err := h.command("", nativeEditorCommand{Action: "move", Mode: "tab", Destination: "new"}); err != nil {
		return err
	}
	failed, err := h.waitState("", "real exec.Start failure restores source", func(state nativeEditorSnapshot) bool {
		return state.Window.Outgoing == 0 && !state.Window.Running && len(state.Documents) == 2 && !state.Textarea.Disabled && h.failedLaunches.Load() == 1
	})
	if err != nil {
		return err
	}
	for _, before := range cancelled.Documents {
		after, found := nativeEditorDoc(failed, before.UID)
		if !found {
			return errors.New("failed native launch lost document")
		}
		if err := nativeEditorEquivalent(before, after); err != nil {
			return err
		}
	}
	if err := h.capture("", "failed-launch-retained"); err != nil {
		return err
	}
	h.phases = append(h.phases, "nonexistent private EXE: real exec.Start failure, no destination process and source fully editable without state loss")
	if raw, err := os.ReadFile(h.app.dataPath); err == nil && bytes.Contains(raw, []byte("SYNTHETIC NATIVE EDITOR108 ONLY")) {
		return errors.New("unsaved document text leaked into settings persistence")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.blocked) != 0 || len(h.failures) != 0 || len(h.children) != 2 {
		return fmt.Errorf("unexpected native fixture operations/errors/child count: %v %v %d", h.blocked, h.failures, len(h.children))
	}
	return nil
}

func (h *nativeEditorFixture) writeReport(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	report := map[string]any{"diagnostic128Broker": h.brokerTrace128, "diagnostic128Views": h.finalTrace128, "passed": err == nil, "actualTabPointerDrags": h.dragComplete, "pointerDragCount": h.pointerDrags.Load(), "tabGestureEvidence": h.gestures, "tabGestureScope": "Win32 mouse input into actual installed WebView2; DOM fixture creates synthetic text and observes state, never dispatches drag events", "actualNativeWebView2": true, "productionChildCLI": true, "realApplicationBackendAndRegistry": true, "productionAuthenticationAndOriginChecks": true, "testOnlyDOMAutomation": true, "productionBridgeChanges": false, "syntheticUnsavedDocumentsOnly": true, "scope": "Real parent native host and separately built production --editor-view children; fixture chooses child executable and decorates test HTTP document with DOM automation only; no main CLI/storage lifecycle assertion", "closePreference113": h.closePreferenceComplete, "phases": h.phases, "nativeCaptures": h.captures, "childProcesses": len(h.children), "realFailedLaunches": h.failedLaunches.Load(), "blockedUnexpectedOperations": h.blocked, "driverFailures": h.failures}
	if err != nil {
		report["error"] = err.Error()
	}
	raw, _ := json.MarshalIndent(report, "", "  ")
	_ = os.WriteFile(filepath.Join(h.output, "native-editor108-report.json"), raw, 0600)
	_ = os.WriteFile(filepath.Join(h.output, "native-editor110-drag-report.json"), raw, 0600)
	_ = os.WriteFile(filepath.Join(h.output, "native-editor113-close-report.json"), raw, 0600)
}

func (h *nativeEditorFixture) cleanup() {
	h.mu.Lock()
	children := make([]*nativeEditorChild, 0, len(h.children))
	for _, child := range h.children {
		children = append(children, child)
	}
	h.mu.Unlock()
	for _, child := range children {
		select {
		case <-child.done:
		default:
			_ = child.command.Process.Kill()
			select {
			case <-child.done:
			case <-time.After(5 * time.Second):
			}
		}
	}
}

// The 110 extension uses actual Win32 mouse down/move/up, and native maximize /
// restore. The DOM driver only observes real rectangles/state; it never creates
// DataTransfer, dispatches drag events, or calls the transfer helper for gestures.
func (h *nativeEditorFixture) identity(view string) (int, string, error) {
	if view == "" {
		executable, err := os.Executable()
		return os.Getpid(), executable, err
	}
	h.mu.Lock()
	child := h.children[view]
	h.mu.Unlock()
	if child == nil {
		return 0, "", errors.New("native gesture destination process absent")
	}
	return child.command.Process.Pid, h.executable, nil
}
func (h *nativeEditorFixture) nativeInput(action, mode, source, target string, from, to nativeEditorSnapshot, point, end nativeEditorRect) error {
	pid, executable, err := h.identity(source)
	if err != nil {
		return err
	}
	args := []string{"-NoProfile", "-NonInteractive", "-File", filepath.Join(h.root, "scripts", "native-editor-drag.ps1"), "-Action", action, "-OwnerProcessID", strconv.Itoa(pid), "-ExecutablePath", executable, "-Mode", mode}
	if source != target {
		targetPID, targetEXE, e := h.identity(target)
		if e != nil {
			return e
		}
		args = append(args, "-TargetProcessID", strconv.Itoa(targetPID), "-TargetExecutablePath", targetEXE)
	}
	if action == "Drag" {
		for _, pair := range []struct {
			name  string
			value float64
		}{{"FromX", point.Left}, {"FromY", point.Top}, {"FromWidth", from.Geometry.Width}, {"FromHeight", from.Geometry.Height}, {"ToX", end.Left}, {"ToY", end.Top}, {"ToWidth", to.Geometry.Width}, {"ToHeight", to.Geometry.Height}} {
			args = append(args, "-"+pair.name, strconv.FormatFloat(pair.value, 'f', 3, 64))
		}
	}
	command := exec.CommandContext(h.ctx, "powershell.exe", args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("actual native %s/%s: %w: %s", action, mode, err, output)
	}
	if !json.Valid(bytes.TrimSpace(output)) {
		return fmt.Errorf("native gesture evidence was not JSON: %s", output)
	}
	h.mu.Lock()
	h.gestures = append(h.gestures, json.RawMessage(bytes.TrimSpace(output)))
	h.mu.Unlock()
	if action == "Drag" {
		h.pointerDrags.Add(1)
	}
	return nil
}
func nativeEditorPoint(state nativeEditorSnapshot, uid string, after bool) (nativeEditorRect, error) {
	for _, tab := range state.Geometry.Tabs {
		if tab.UID == uid && tab.Width > 10 && tab.Height > 10 {
			x := tab.Left + math.Min(30, tab.Width/2)
			if after {
				x = tab.Left + tab.Width - 3
			}
			return nativeEditorRect{Left: x, Top: tab.Top + tab.Height/2}, nil
		}
	}
	return nativeEditorRect{}, fmt.Errorf("visible source/target tab absent for captured UID %s", uid)
}
func (h *nativeEditorFixture) idle(view string) (nativeEditorSnapshot, error) {
	return h.waitState(view, "idle registered editor before/after actual pointer gesture", func(s nativeEditorSnapshot) bool {
		return !s.Window.Running && !s.Window.DragPreparing && s.Window.Outgoing == 0 && s.Window.Incoming == 0 && !s.Drag.Armed
	})
}
func nativeEditorSameDocuments(before, after nativeEditorSnapshot) error {
	if before.Active != after.Active || len(before.Documents) != len(after.Documents) {
		return errors.New("native gesture changed active identity or document count")
	}
	for _, d := range before.Documents {
		got, ok := nativeEditorDoc(after, d.UID)
		if !ok {
			return errors.New("native gesture lost a captured document UID")
		}
		if err := nativeEditorEquivalent(d, got); err != nil {
			return err
		}
	}
	return nil
}
func (h *nativeEditorFixture) exerciseDrag() error {
	h.failLaunch.Store(false)
	empty := nativeEditorSnapshot{}
	point := nativeEditorRect{}
	if err := h.nativeInput("Arrange", "Tab", "", "", empty, empty, point, point); err != nil {
		return err
	}
	before, err := h.idle("")
	if err != nil {
		return err
	}
	if len(before.Documents) != 2 {
		return errors.New("actual tab-drag acceptance needs two retained synthetic documents")
	}
	first, last := before.Documents[0].UID, before.Documents[1].UID
	from, err := nativeEditorPoint(before, first, false)
	if err != nil {
		return err
	}
	to, err := nativeEditorPoint(before, last, true)
	if err != nil {
		return err
	}
	if err = h.nativeInput("Drag", "Tab", "", "", before, before, from, to); err != nil {
		return err
	}
	reordered, err := h.waitState("", "actual mouse reorders the captured tab UID", func(s nativeEditorSnapshot) bool {
		return len(s.Documents) == 2 && s.Documents[0].UID == last && s.Documents[1].UID == first && !s.Window.DragPreparing && !s.Drag.Armed
	})
	if err != nil {
		return err
	}
	if err = nativeEditorSameDocuments(before, reordered); err != nil {
		return err
	}
	if err = h.capture("", "drag110-reordered"); err != nil {
		return err
	}
	h.phases = append(h.phases, "110 actual mouse reorder preserves active UID, dirty text, undo/redo and selection/scroll")
	// Escape cancels captured-pointer drag; a failed Alt/OLE release never infers detach.
	for _, mode := range []string{"Escape", "OutsideCancel"} {
		before, err = h.idle("")
		if err != nil {
			return err
		}
		from, err = nativeEditorPoint(before, before.Active, false)
		if err != nil {
			return err
		}
		to, err = nativeEditorPoint(before, first, false)
		if err != nil {
			return err
		}
		if err = h.nativeInput("Drag", mode, "", "", before, before, from, to); err != nil {
			return err
		}
		retained, e := h.idle("")
		if e != nil {
			return e
		}
		if err = nativeEditorSameDocuments(before, retained); err != nil {
			return err
		}
		h.mu.Lock()
		children := len(h.children)
		h.mu.Unlock()
		if children != 2 {
			return errors.New("cancelled/unsupported actual drag launched a new window")
		}
	}
	h.phases = append(h.phases, "115 actual Escape and failed Alt/OLE outside release retain source ownership; no inferred failed-drop detach")
	before, err = h.idle("")
	if err != nil {
		return err
	}
	selected, _ := nativeEditorDoc(before, before.Active)
	from, err = nativeEditorPoint(before, selected.UID, false)
	if err != nil {
		return err
	}
	child, _, err := h.transferAction("", "new", []nativeEditorDocument{selected}, selected.UID, func() error { return h.nativeInput("Drag", "Outside", "", "", before, before, from, point) })
	if err != nil {
		return fmt.Errorf("actual captured-pointer release outside source window: %w", err)
	}
	h.phases = append(h.phases, "115 actual ordinary tab drag away and outside source HWND launches production child without a drop box; source frozen and destination staged until existing ACK")
	if err = h.capture(child, "drag110-detached"); err != nil {
		return err
	}
	// Native maximize, not a browser viewport simulation. Capture the actual HWND
	// and assert its CSS main grid starts at zero and fills the client viewport.
	if err = h.nativeInput("Arrange", "Tab", child, child, empty, empty, point, point); err != nil {
		return err
	}
	if _, err = h.idle(child); err != nil {
		return err
	}
	if err = h.nativeInput("Maximize", "Tab", child, child, empty, empty, point, point); err != nil {
		return err
	}
	maximized, err := h.waitState(child, "native maximized editor fills wide client without inherited grid column", func(s nativeEditorSnapshot) bool {
		return s.Geometry.Width >= 640 && s.Geometry.Main.Width >= s.Geometry.Width*9/10 && s.Geometry.Editor.Width > 0
	})
	if err != nil {
		return err
	}
	if maximized.Geometry.Editor.Width < maximized.Geometry.Width*8/10 {
		return errors.New("maximized native editor does not use most of the client width")
	}
	if err = h.capture(child, "drag110-maximized"); err != nil {
		return err
	}
	if err = h.nativeInput("Restore", "Tab", child, child, empty, empty, point, point); err != nil {
		return err
	}
	if _, err = h.waitState(child, "native restore retains full-width grid", func(s nativeEditorSnapshot) bool {
		return s.Geometry.Width >= 640 && s.Geometry.Main.Width >= s.Geometry.Width*9/10 && s.Geometry.Editor.Width > 0
	}); err != nil {
		return err
	}
	if err = h.capture(child, "drag110-restored"); err != nil {
		return err
	}
	h.phases = append(h.phases, "110 actual native maximize/restore: main.left=0, main.width=innerWidth, wide editor fills viewport; actual HWND PNGs")
	// Arrange both exact native processes so a real pointer can cross from child
	// to the exposed left side of the main editor, then verify the same ACK gate.
	if err = h.nativeInput("Arrange", "Cross", "", child, empty, empty, point, point); err != nil {
		return err
	}
	source, err := h.idle(child)
	if err != nil {
		return err
	}
	destination, err := h.idle("")
	if err != nil {
		return err
	}
	from, err = nativeEditorPoint(source, selected.UID, false)
	if err != nil {
		return err
	}
	to = nativeEditorRect{Left: destination.Geometry.Editor.Left + 20, Top: destination.Geometry.Editor.Top + 120}
	selected, _ = nativeEditorDoc(source, selected.UID)
	_, merged, err := h.transferAction(child, "main", []nativeEditorDocument{selected}, selected.UID, func() error { return h.nativeInput("Drag", "Cross", child, "", source, destination, from, to) })
	if err != nil {
		return fmt.Errorf("actual Alt-mouse drag between two production WebViews: %w", err)
	}
	if len(merged.Documents) != 2 {
		return errors.New("native cross-window merge lost untouched document")
	}
	if err = h.capture("", "drag110-cross-window-merged"); err != nil {
		return err
	}
	if err = h.closeEmpty(child); err != nil {
		return err
	}
	h.phases = append(h.phases, "115 actual Alt-cross-WebView mouse drag merges through opaque broker ticket and original offer/accept ACK; exact source UID/history/view retained and child releases only after ACK")
	if h.pointerDrags.Load() != 5 {
		return fmt.Errorf("expected five real pointer drag scenarios, got %d", h.pointerDrags.Load())
	}
	h.dragComplete = true
	return nil
}

// Passive fixture-only response observer: forwards original bytes/status intact.
type nativeEditorStatus128 struct {
	http.ResponseWriter
	status int
}

func (w *nativeEditorStatus128) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
func nativeEditorBrokerAction128(path string) string {
	switch path {
	case "/api/editor-views/drags":
		return "register"
	case "/api/editor-views/drags/claim":
		return "claim"
	case "/api/editor-views/drags/cancel":
		return "cancel"
	case "/api/editor-views/drags/detach":
		return "detach"
	case "/api/editor-views/transfers":
		return "transfer"
	}
	return ""
}
func (h *nativeEditorFixture) collectDiagnostics128() {
	views := []string{""}
	h.mu.Lock()
	for id, c := range h.children {
		select {
		case <-c.done:
		default:
			views = append(views, id)
		}
	}
	h.mu.Unlock()
	for _, view := range views {
		s, err := h.command(view, nativeEditorCommand{Action: "snapshot"})
		kind := "child"
		if view == "" {
			kind = "main"
		}
		row := map[string]any{"view": kind, "snapshotAvailable": err == nil}
		if err == nil {
			row["events"] = s.Diagnostic
			row["documentCount"] = len(s.Documents)
			row["window"] = map[string]any{"enabled": s.Window.Enabled, "running": s.Window.Running, "preparing": s.Window.DragPreparing, "closing": s.Window.Closing, "outgoing": s.Window.Outgoing, "incoming": s.Window.Incoming, "hasError": s.Window.Error != ""}
			row["drag"] = map[string]any{"armed": s.Drag.Armed, "started": s.Drag.Started, "ready": s.Drag.Ready}
			row["hasMessage"] = s.Message != ""
			row["errorCount"] = len(s.Errors)
		}
		h.finalTrace128 = append(h.finalTrace128, row)
	}
}

func TestNativeEditorDiagnostic128SanitizedBrokerObserver(t *testing.T) {
	for path, want := range map[string]string{
		"/api/editor-views/drags": "register", "/api/editor-views/drags/claim": "claim", "/api/editor-views/drags/cancel": "cancel", "/api/editor-views/drags/detach": "detach", "/api/editor-views/transfers": "transfer",
		"/api/editor-views/drags/private-ticket": "", "/private/path": "",
	} {
		if got := nativeEditorBrokerAction128(path); got != want {
			t.Fatalf("action classification got %q want %q", got, want)
		}
	}
	recorder := httptest.NewRecorder()
	observer := &nativeEditorStatus128{ResponseWriter: recorder, status: 200}
	observer.WriteHeader(409)
	_, _ = observer.Write([]byte("unchanged response"))
	if observer.status != 409 || recorder.Code != 409 || recorder.Body.String() != "unchanged response" {
		t.Fatal("passive observer changed original response")
	}
}
