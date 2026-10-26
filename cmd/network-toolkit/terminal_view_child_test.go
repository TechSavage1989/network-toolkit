package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func detachedViewFixture(t *testing.T) terminalViewLaunch {
	t.Helper()
	id := strings.Repeat("ab", 16)
	return terminalViewLaunch{
		Version: 1, OwnerHost: "127.0.0.1:32123", OwnerInstanceID: strings.Repeat("cd", 16),
		ViewID: id, ViewToken: strings.Repeat("ef", 32),
		DataDir: filepath.Join(t.TempDir(), terminalViewDirectoryName, id),
		Title:   "Network Toolkit Terminal — fixture",
	}
}

func TestDetachedTerminalBootstrapValidation(t *testing.T) {
	valid := detachedViewFixture(t)
	for name, change := range map[string]func(*terminalViewLaunch){
		"version":           func(v *terminalViewLaunch) { v.Version = 2 },
		"remote owner":      func(v *terminalViewLaunch) { v.OwnerHost = "192.0.2.1:1234" },
		"hostname owner":    func(v *terminalViewLaunch) { v.OwnerHost = "localhost:1234" },
		"IPv6 owner":        func(v *terminalViewLaunch) { v.OwnerHost = "[::1]:1234" },
		"padded port":       func(v *terminalViewLaunch) { v.OwnerHost = "127.0.0.1:01234" },
		"zero port":         func(v *terminalViewLaunch) { v.OwnerHost = "127.0.0.1:0" },
		"large port":        func(v *terminalViewLaunch) { v.OwnerHost = "127.0.0.1:65536" },
		"owner URL":         func(v *terminalViewLaunch) { v.OwnerHost = "http://127.0.0.1:1234" },
		"short instance":    func(v *terminalViewLaunch) { v.OwnerInstanceID = "ab" },
		"nonhex view":       func(v *terminalViewLaunch) { v.ViewID = strings.Repeat("z", 32) },
		"uppercase token":   func(v *terminalViewLaunch) { v.ViewToken = strings.Repeat("AB", 32) },
		"relative folder":   func(v *terminalViewLaunch) { v.DataDir = filepath.Join(terminalViewDirectoryName, v.ViewID) },
		"wrong view folder": func(v *terminalViewLaunch) { v.DataDir = filepath.Join(filepath.Dir(v.DataDir), "different") },
		"wrong parent": func(v *terminalViewLaunch) {
			v.DataDir = filepath.Join(filepath.Dir(filepath.Dir(v.DataDir)), "shared", v.ViewID)
		},
		"unclean folder": func(v *terminalViewLaunch) {
			v.DataDir = filepath.Dir(v.DataDir) + string(filepath.Separator) + "." + string(filepath.Separator) + v.ViewID
		},
		"control in folder": func(v *terminalViewLaunch) {
			v.DataDir = filepath.Join(filepath.Dir(filepath.Dir(v.DataDir)), "bad\nfolder", terminalViewDirectoryName, v.ViewID)
		},
		"long title":    func(v *terminalViewLaunch) { v.Title = strings.Repeat("x", 257) },
		"control title": func(v *terminalViewLaunch) { v.Title = "Terminal\x00fixture" },
	} {
		t.Run(name, func(t *testing.T) {
			launch := valid
			change(&launch)
			if err := launch.validate(); err == nil {
				t.Fatal("invalid view bootstrap was accepted")
			}
		})
	}
	valid.Title = "  "
	if err := valid.validate(); err != nil || valid.Title != "Network Toolkit Terminal" {
		t.Fatalf("default window title: %q, %v", valid.Title, err)
	}
}

func TestDetachedTerminalBootstrapIsBoundedAndRejectsExtraFields(t *testing.T) {
	launch := detachedViewFixture(t)
	raw, err := encodeTerminalViewLaunch(launch)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeTerminalViewLaunch(bytes.NewReader(raw))
	if err != nil || !reflect.DeepEqual(decoded, launch) {
		t.Fatalf("bootstrap roundtrip: %+v, %v", decoded, err)
	}
	for name, input := range map[string]string{
		"empty": "", "null": "null", "extra object": string(raw) + "{}",
		"oversize":    strings.Repeat(" ", terminalViewBootstrapLimit+1),
		"credentials": strings.TrimSpace(string(raw[:len(raw)-2])) + ",\"password\":\"not-allowed\"}",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeTerminalViewLaunch(strings.NewReader(input)); err == nil {
				t.Fatal("invalid bootstrap was accepted")
			}
		})
	}
	if _, err := decodeTerminalViewLaunch(nil); err == nil {
		t.Fatal("missing bootstrap was accepted")
	}
	for _, headless := range []bool{false, true} {
		want := []string{"--terminal-view"}
		if headless {
			want = append(want, "--no-browser")
		}
		if got := terminalViewChildArguments(headless); !reflect.DeepEqual(got, want) {
			t.Fatalf("child argv may contain bootstrap data: %q", got)
		}
	}
}

func TestDetachedTerminalCanceledLaunchDoesNotStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bootstrap := []byte("private fixture bootstrap")
	cmd := exec.Command("must-not-be-executed")
	done, err := startTerminalViewChild(ctx, cmd, bootstrap)
	if !errors.Is(err, context.Canceled) || done != nil || cmd.Process != nil {
		t.Fatalf("canceled launch unexpectedly started: %v, %v", done, err)
	}
	if !bytes.Equal(bootstrap, make([]byte, len(bootstrap))) {
		t.Fatal("the unused bootstrap was retained")
	}
}

func TestDetachedTerminalControlBindsIdentityAndNeverRedirects(t *testing.T) {
	launch := detachedViewFixture(t)
	var redirected atomic.Int32
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected.Add(1) }))
	defer redirectTarget.Close()
	type response struct {
		status         int
		body, location string
	}
	var next atomic.Value
	next.Store(response{status: http.StatusOK})
	owner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/terminal-views/current/pulse" || r.Header.Get("X-Session-Token") != launch.ViewToken {
			t.Errorf("invalid child control request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "{}" {
			t.Errorf("unexpected control body %q", body)
		}
		value := next.Load().(response)
		if value.location != "" {
			w.Header().Set("Location", value.location)
		}
		w.WriteHeader(value.status)
		_, _ = io.WriteString(w, value.body)
	}))
	defer owner.Close()
	launch.OwnerHost = strings.TrimPrefix(owner.URL, "http://")
	control := newTerminalViewControl(launch)
	defer control.client.CloseIdleConnections()
	if control.client.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("view control requests could use a configured proxy")
	}
	for _, state := range []string{"opening", "detached", "return_requested", "returned"} {
		raw, _ := json.Marshal(terminalViewPulse{OwnerInstanceID: launch.OwnerInstanceID, ViewID: launch.ViewID, State: state})
		next.Store(response{status: http.StatusOK, body: string(raw)})
		got, err := control.pulse(context.Background())
		if err != nil || got.State != state {
			t.Fatalf("state %s: %+v %v", state, got, err)
		}
	}
	for name, value := range map[string]response{
		"wrong owner":    {status: 200, body: fmt.Sprintf(`{"ownerInstanceId":"other","viewId":%q,"state":"detached"}`, launch.ViewID)},
		"wrong view":     {status: 200, body: fmt.Sprintf(`{"ownerInstanceId":%q,"viewId":"other","state":"detached"}`, launch.OwnerInstanceID)},
		"unknown state":  {status: 200, body: fmt.Sprintf(`{"ownerInstanceId":%q,"viewId":%q,"state":"unknown"}`, launch.OwnerInstanceID, launch.ViewID)},
		"extra response": {status: 200, body: `{} {}`},
		"oversize":       {status: 200, body: strings.Repeat(" ", terminalViewResponseLimit+1)},
		"redirect":       {status: 302, location: redirectTarget.URL},
		"rejected":       {status: 403},
	} {
		t.Run(name, func(t *testing.T) {
			next.Store(value)
			_, err := control.pulse(context.Background())
			var protocol *terminalViewProtocolError
			if !errors.As(err, &protocol) {
				t.Fatalf("invalid owner response was not rejected: %v", err)
			}
		})
	}
	if redirected.Load() != 0 {
		t.Fatal("the view token was sent to a redirected destination")
	}
	next.Store(response{status: http.StatusGone})
	if got, err := control.pulse(context.Background()); err != nil || got.State != "returned" {
		t.Fatalf("410 must finish the local view: %+v %v", got, err)
	}
	next.Store(response{status: http.StatusServiceUnavailable})
	if _, err := control.pulse(context.Background()); err == nil {
		t.Fatal("owner outage was accepted as success")
	} else {
		var protocol *terminalViewProtocolError
		if errors.As(err, &protocol) {
			t.Fatal("a temporary owner outage was classified as a protocol violation")
		}
	}
}

func TestDetachedTerminalDirectoryRejectsSharedLink(t *testing.T) {
	launch := detachedViewFixture(t)
	if err := ensureTerminalViewDirectory(launch.DataDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(launch.DataDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), launch.DataDir); err != nil {
		t.Skipf("directory symlinks are unavailable: %v", err)
	}
	if err := ensureTerminalViewDirectory(launch.DataDir); err == nil {
		t.Fatal("view profile followed a shared directory link")
	}
}

func TestDetachedTerminalViewChildUsesStdinAndIndependentLifetime(t *testing.T) {
	launch := detachedViewFixture(t)
	launch.NoBrowser = true
	var state atomic.Value
	state.Store("opening")
	pulses := make(chan string, 16)
	released := make(chan struct{}, 1)
	owner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("X-Session-Token") != launch.ViewToken {
			t.Errorf("child did not authenticate its control request")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/api/terminal-views/current/pulse":
			current := state.Load().(string)
			select {
			case pulses <- current:
			default:
			}
			_ = json.NewEncoder(w).Encode(terminalViewPulse{OwnerInstanceID: launch.OwnerInstanceID, ViewID: launch.ViewID, State: current})
		case "/api/terminal-views/current/release":
			select {
			case released <- struct{}{}:
			default:
			}
			_, _ = io.WriteString(w, "{}")
		default:
			t.Errorf("view client attempted an unrelated endpoint: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer owner.Close()
	launch.OwnerHost = strings.TrimPrefix(owner.URL, "http://")
	bootstrap, err := encodeTerminalViewLaunch(launch)
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestDetachedTerminalViewHelperProcess$")
	cmd.Env = append(os.Environ(), "NETWORK_TOOLKIT_DETACHED_VIEW_HELPER=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done, err := startTerminalViewChild(ctx, cmd, bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	finished := false
	t.Cleanup(func() {
		if !finished {
			_ = cmd.Process.Kill()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("fixture child did not exit after cleanup")
			}
		}
	})
	waitState := func(want string) {
		t.Helper()
		deadline := time.NewTimer(8 * time.Second)
		defer deadline.Stop()
		for {
			select {
			case current := <-pulses:
				if current == want {
					return
				}
			case err := <-done:
				finished = true
				t.Fatalf("child exited before %s: %v; %s", want, err, stderr.String())
			case <-deadline.C:
				t.Fatalf("child did not reach state %s", want)
			}
		}
	}
	waitState("opening")
	cancel() // Successful launch must not tie the view lifetime to its request.
	state.Store("return_requested")
	waitState("return_requested")
	select {
	case err := <-done:
		finished = true
		t.Fatalf("return_requested closed before the UI checkpoint: %v; %s", err, stderr.String())
	case <-time.After(100 * time.Millisecond):
	}
	state.Store("returned")
	select {
	case err := <-done:
		finished = true
		if err != nil {
			t.Fatalf("view child failed: %v; %s", err, stderr.String())
		}
	case <-time.After(8 * time.Second):
		t.Fatal("returned view did not exit")
	}
	select {
	case <-released:
	default:
		t.Fatal("child did not release its scoped view on exit")
	}
	if !bytes.Equal(bootstrap, make([]byte, len(bootstrap))) {
		t.Fatal("completed child bootstrap was retained")
	}
	entries, err := os.ReadDir(launch.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("headless view created owner settings, trust, or connection state")
	}
}

func TestDetachedTerminalViewHelperProcess(t *testing.T) {
	if os.Getenv("NETWORK_TOOLKIT_DETACHED_VIEW_HELPER") != "1" {
		return
	}
	if err := runDetachedTerminalView(os.Stdin, true); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(0)
}

func TestDetachedTerminalViewChildOwnerShutdownExitsCleanly(t *testing.T) {
	launch := detachedViewFixture(t)
	ready := make(chan struct{}, 1)
	owner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Session-Token") != launch.ViewToken {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_ = json.NewEncoder(w).Encode(terminalViewPulse{OwnerInstanceID: launch.OwnerInstanceID, ViewID: launch.ViewID, State: "detached"})
		select {
		case ready <- struct{}{}:
		default:
		}
	}))
	defer owner.Close()
	launch.OwnerHost = strings.TrimPrefix(owner.URL, "http://")
	bootstrap, err := encodeTerminalViewLaunch(launch)
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestDetachedTerminalViewHelperProcess$")
	cmd.Env = append(os.Environ(), "NETWORK_TOOLKIT_DETACHED_VIEW_HELPER=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	done, err := startTerminalViewChild(context.Background(), cmd, bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	finished := false
	t.Cleanup(func() {
		if !finished {
			_ = cmd.Process.Kill()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("owner-loss fixture did not close")
			}
		}
	})
	select {
	case <-ready:
	case err := <-done:
		finished = true
		t.Fatalf("view startup failed: %v; %s", err, stderr.String())
	case <-time.After(5 * time.Second):
		t.Fatal("view did not reach its owner")
	}
	owner.Close()
	select {
	case err := <-done:
		finished = true
		if err != nil {
			t.Fatalf("normal owner shutdown became a fatal view error: %v; %s", err, stderr.String())
		}
	case <-time.After(12 * time.Second):
		t.Fatal("view remained open after its owner stopped")
	}
}
