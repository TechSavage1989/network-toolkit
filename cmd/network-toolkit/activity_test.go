package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"networktoolkit/internal/capture"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestActivitySnapshotAllowlistAndNoHistoricalOrSecretData(t *testing.T) {
	a := &application{ctx: context.Background(), jobs: map[string]*Job{}}
	for _, tool := range []string{"discover", "monitor", "ping", "ftp", "wifi", "secret-unknown-tool"} {
		a.jobs[tool] = &Job{ID: "secret-id", Tool: tool, Target: "secret-host", Status: "running", Options: map[string]any{"password": "secret-password"}}
	}
	for _, status := range []string{"queued", "completed", "failed", "cancelled"} {
		a.jobs[status] = &Job{Tool: "tls", Status: status}
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	a.jobs["unwinding"] = &Job{Tool: "tls", Status: "running", activityContext: canceled}
	a.browserConnecting = 4 // Attempts, not authenticated retained transports.
	a.localTools = newLocalToolsManagerWithBackend(context.Background(), localToolsBackend{
		readHosts: func(context.Context) (localHostsSnapshot, error) {
			t.Fatal("snapshot read hosts")
			return localHostsSnapshot{}, nil
		},
		readIP: func(context.Context) (localIPSnapshot, error) {
			t.Fatal("snapshot read adapters")
			return localIPSnapshot{}, nil
		},
		apply: func(context.Context, localToolsRequest) localApplyResult {
			t.Fatal("snapshot launched helper")
			return localApplyResult{}
		},
	})
	defer a.localTools.cancel()
	a.localTools.operations["running"] = localToolsOperation{Kind: "hosts", Status: "running"}
	a.localTools.operations["old"] = localToolsOperation{Kind: "ip", Status: "completed"}
	want := map[string]activityCounts{
		"discover": {Operations: 1}, "monitor": {Operations: 1}, "ping": {Operations: 1},
		"sftp": {Operations: 1}, "local": {Operations: 1}, "hosts": {Operations: 1},
	}
	for i := 0; i < 10; i++ {
		got := a.activitySnapshot()
		if !reflect.DeepEqual(got.Tools, want) {
			t.Fatalf("snapshot = %+v, want %+v", got.Tools, want)
		}
		encoded, err := json.Marshal(got)
		if err != nil || strings.Contains(string(encoded), "secret") {
			t.Fatalf("non-count metadata escaped: %s (%v)", encoded, err)
		}
	}
	a.shutdownRequested.Store(true)
	if len(a.activitySnapshot().Tools) != 0 {
		t.Fatal("shutdown retained activity")
	}
	if len((&application{}).activitySnapshot().Tools) != 0 {
		t.Fatal("new application inherited activity")
	}
}

func TestActivityOperationsConcurrentCancellationAndCleanup(t *testing.T) {
	a := &application{}
	ctx, cancel := context.WithCancel(context.Background())
	end := a.activity.begin(ctx, "ftp")
	if got := a.activitySnapshot().Tools["sftp"].Operations; got != 1 {
		t.Fatalf("running FTP operation = %d", got)
	}
	cancel()
	if len(a.activitySnapshot().Tools) != 0 {
		t.Fatal("canceled operation remained active while unwinding")
	}
	end()
	end() // Cleanup is idempotent, including cancellation/error paths.
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 30; n++ {
				finish := a.activity.begin(context.Background(), "ssh")
				_ = a.activitySnapshot()
				finish()
			}
		}()
	}
	wg.Wait()
	a.activity.begin(context.Background(), "secret-unknown")()
	if len(a.activitySnapshot().Tools) != 0 || len(a.activity.entries) != 0 {
		t.Fatal("finished operations remained active")
	}
}

func TestActivityCaptureOnlyReadyRunningNeverStartsInventoryOrHelper(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := newPacketCaptureManager(ctx)
	m.run = func(context.Context, capture.Options, <-chan struct{}, func(packetCaptureMessage) error) error {
		t.Fatal("activity launched capture helper")
		return nil
	}
	a := &application{packetCaptures: m}
	for _, state := range []string{"starting", "running", "stopping", "completed", "failed"} {
		m.sessions[state] = &packetCaptureSession{info: packetCaptureInfo{State: state, Engine: "synthetic"}}
	}
	m.sessions["import"] = &packetCaptureSession{info: packetCaptureInfo{State: "running", Engine: "import"}}
	if got := a.activitySnapshot().Tools["capture"].Captures; got != 1 {
		t.Fatalf("live capture count = %d", got)
	}
	for _, state := range []string{"stopping", "completed", "failed"} {
		m.sessions["running"].info.State = state
		if len(a.activitySnapshot().Tools) != 0 {
			t.Fatalf("capture %s retained activity", state)
		}
	}
	m.sessions["running"].info.State = "running"
	cancel()
	if len(a.activitySnapshot().Tools) != 0 {
		t.Fatal("canceled capture manager retained activity")
	}
}

func activityServerFixture(t *testing.T, proto string) (*transferHostManager, transferServerConfig) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	m := newTransferHostManager(ctx, t.TempDir())
	t.Cleanup(func() { cancel(); m.Close() })
	var port int
	if proto == "tftp" {
		l, err := net.ListenPacket("udp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port = l.LocalAddr().(*net.UDPAddr).Port
		_ = l.Close()
	} else {
		l, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port = l.Addr().(*net.TCPAddr).Port
		_ = l.Close()
	}
	cfg := transferServerConfig{Protocol: proto, Root: filepath.Join(m.base, "synthetic-share"), Bind: "127.0.0.1", Port: port, Access: "read", Username: "synthetic-user", Password: "synthetic-password", Security: "plain"}
	return m, cfg
}

func TestActivityIdleListenersUnexpectedExitAndRestart(t *testing.T) {
	for _, proto := range []string{"ftp", "sftp", "tftp"} {
		t.Run(proto, func(t *testing.T) {
			m, cfg := activityServerFixture(t, proto)
			a := &application{transfers: m}
			if len(a.activitySnapshot().Tools) != 0 {
				t.Fatal("off server counted")
			}
			status, err := m.start(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if status["running"] != true || a.activitySnapshot().Tools[activityPage(proto)].Servers != 1 {
				t.Fatal("idle listener was not counted")
			}
			m.mu.Lock()
			old := m.servers[proto]
			m.mu.Unlock()
			_ = old.listener.Close() // Simulate unexpected accept/read-loop exit.
			viewTestWait(t, func() bool {
				m.mu.Lock()
				defer m.mu.Unlock()
				return m.servers[proto] == nil
			})
			if old.ctx.Err() == nil || old.snapshot()["running"] != false || len(a.activitySnapshot().Tools) != 0 {
				t.Fatal("dead listener retained a live state/context")
			}
			if _, err := m.start(cfg); err != nil {
				t.Fatalf("restart: %v", err)
			}
			m.forgetStopped(proto, old) // A delayed old watcher cannot remove replacement.
			if a.activitySnapshot().Tools[activityPage(proto)].Servers != 1 {
				t.Fatal("old cleanup removed replacement")
			}
			if err := m.stop(proto); err != nil {
				t.Fatal(err)
			}
			if len(a.activitySnapshot().Tools) != 0 {
				t.Fatal("explicit stop retained activity")
			}
		})
	}
}

func TestActivityFailedListenerBindNeverCounts(t *testing.T) {
	m, cfg := activityServerFixture(t, "ftp")
	l, err := net.Listen("tcp4", net.JoinHostPort(cfg.Bind, strconv.Itoa(cfg.Port)))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if _, err := m.start(cfg); err == nil {
		t.Fatal("accepted occupied port")
	}
	if len((&application{transfers: m}).activitySnapshot().Tools) != 0 {
		t.Fatal("failed bind counted")
	}
}

func TestActivityHTTPGuardAndDetachedSessionsCountWithoutScopeExpansion(t *testing.T) {
	a := testApplication(t)
	if r := requestApp(a, "GET", "/api/activity", "", false); r.Code != http.StatusUnauthorized {
		t.Fatal(r.Code)
	}
	if r := requestApp(a, "POST", "/api/activity", "{}", true); r.Code != http.StatusMethodNotAllowed {
		t.Fatal(r.Code)
	}
	if r := requestApp(a, "GET", "/api/activity", "", true); r.Code != 200 || strings.TrimSpace(r.Body.String()) != `{"tools":{}}` {
		t.Fatal(r.Code, r.Body.String())
	}
	for _, mutate := range []func(*http.Request){
		func(r *http.Request) { r.Host = "attacker.invalid" },
		func(r *http.Request) { r.Header.Set("Origin", "https://attacker.invalid") },
	} {
		r := httptest.NewRequest("GET", "http://"+a.host+"/api/activity", nil)
		r.Header.Set("X-Session-Token", a.token)
		mutate(r)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatal(w.Code)
		}
	}
	first, second := viewTestConnected(t, a), viewTestConnected(t, a)
	// Window ownership is not a second transport, nor a reason to exclude it.
	a.views.mu.Lock()
	a.views.owners[first.ID] = terminalViewOwner{ViewID: "synthetic-detached", Revision: 2, Ready: true}
	a.views.mu.Unlock()
	if got := a.activitySnapshot().Tools["terminal"].Connections; got != 2 {
		t.Fatal(got)
	}
	if err := a.terminals.Disconnect(second.ID); err != nil {
		t.Fatal(err)
	}
	if got := a.activitySnapshot().Tools["terminal"].Connections; got != 1 {
		t.Fatal(got)
	}
	if err := a.terminals.Disconnect(first.ID); err != nil {
		t.Fatal(err)
	}
	if len(a.activitySnapshot().Tools) != 0 {
		t.Fatal("last disconnect retained activity")
	}
	_, launch := workspaceTestCreate(t, a, terminalWorkspaceSnapshot{})
	if r := viewTestRequest(a, launch.ViewToken, "GET", "/api/activity", nil, nil); r.Code != http.StatusForbidden {
		t.Fatalf("detached capability broadened: %d", r.Code)
	}
}

func TestActivityRetainedSFTPCountsIdleAndUnexpectedClose(t *testing.T) {
	m, cfg := activityServerFixture(t, "sftp")
	status, err := m.start(cfg)
	if err != nil {
		t.Fatal(err)
	}
	a := &application{ctx: context.Background()}
	t.Cleanup(a.closeBrowserConnections)
	r := remoteBrowserRequest{Protocol: "ssh", Host: cfg.Bind, Port: cfg.Port, Username: cfg.Username, Password: cfg.Password, AuthMode: "password", Fingerprint: status["fingerprint"].(string)}
	id, err := a.openBrowserConnection(httptest.NewRequest("POST", "/api/remote-browser", nil), r)
	if err != nil {
		t.Fatal(err)
	}
	if got := a.activitySnapshot().Tools["sftp"].Connections; got != 1 {
		t.Fatalf("idle SFTP count = %d", got)
	}
	bad := r
	bad.Password = "wrong-synthetic-password"
	if _, err := a.openBrowserConnection(httptest.NewRequest("POST", "/api/remote-browser", nil), bad); err == nil {
		t.Fatal("accepted bad authentication")
	}
	if got := a.activitySnapshot().Tools["sftp"].Connections; got != 1 {
		t.Fatalf("failed authentication changed count = %d", got)
	}
	if err := m.stop("sftp"); err != nil {
		t.Fatal(err)
	}
	viewTestWait(t, func() bool { return len(a.activitySnapshot().Tools) == 0 })
	if _, err := a.browserConnection(id); err == nil {
		t.Fatal("closed peer still available")
	}
}

func TestActivityRealRemoteDispatchCancellationClearsWhileTransportUnwinds(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			accepted <- connection
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := &application{}
	finished := make(chan error, 1)
	go func() {
		_, err := a.runRemoteBrowserOperation(ctx, remoteBrowserRequest{Protocol: "ftp", Action: "list", Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, Username: "synthetic-user", Password: "synthetic-password", Security: "plain"})
		finished <- err
	}()
	select {
	case connection := <-accepted:
		defer connection.Close() // No real FTP endpoint; keep the synthetic greeting pending.
	case err := <-finished:
		t.Fatalf("dispatch failed before synthetic listener: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("synthetic dispatch did not start")
	}
	if got := a.activitySnapshot().Tools["sftp"]; got.Operations != 1 || got.Connections != 0 {
		t.Fatalf("ephemeral FTP transport misreported: %+v", got)
	}
	cancel()
	if len(a.activitySnapshot().Tools) != 0 {
		t.Fatal("canceled dispatch retained green state")
	}
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("pending greeting succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled synthetic transport did not unwind")
	}
	if len(a.activity.entries) != 0 {
		t.Fatal("dispatch error retained operation record")
	}
}

func TestActivityCaptureRealManagerLifecycleWithSyntheticRunner(t *testing.T) {
	for _, ending := range []string{"complete", "fail", "cancel", "denied"} {
		t.Run(ending, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			manager := newPacketCaptureManager(ctx)
			defer manager.close()
			begin := make(chan struct{})
			ready := make(chan struct{})
			finish := make(chan struct{})
			manager.run = func(ctx context.Context, _ capture.Options, stop <-chan struct{}, emit func(packetCaptureMessage) error) error {
				select {
				case <-begin:
				case <-ctx.Done():
					return ctx.Err()
				}
				if ending == "denied" {
					return errors.New("synthetic elevation declined before capture started")
				}
				if err := emit(packetCaptureMessage{Type: "ready", Engine: "synthetic"}); err != nil {
					return err
				}
				close(ready)
				select {
				case <-finish:
					if ending == "fail" {
						return errors.New("synthetic helper exited")
					}
					return nil
				case <-stop:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			app := &application{packetCaptures: manager}
			info, err := manager.start(capture.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if len(app.activitySnapshot().Tools) != 0 {
				t.Fatal("starting/approval-pending capture counted")
			}
			manager.mu.RLock()
			done := manager.sessions[info.ID].done
			manager.mu.RUnlock()
			close(begin)
			if ending != "denied" {
				<-ready
				if app.activitySnapshot().Tools["capture"].Captures != 1 {
					t.Fatal("ready running capture not counted")
				}
				if ending == "cancel" {
					if err := manager.stopCapture(info.ID); err != nil {
						t.Fatal(err)
					}
					if len(app.activitySnapshot().Tools) != 0 {
						t.Fatal("stopping capture still counted")
					}
				} else {
					close(finish)
				}
			}
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("synthetic capture did not finish")
			}
			if len(app.activitySnapshot().Tools) != 0 {
				t.Fatalf("capture %s retained activity", ending)
			}
		})
	}
}

func TestActivityServerCancellationAndReadsAreRaceSafe(t *testing.T) {
	m, cfg := activityServerFixture(t, "ftp")
	app := &application{transfers: m}
	if _, err := m.start(cfg); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = app.activitySnapshot()
				_ = m.status("ftp")
			}
		}()
	}
	if err := m.stop("ftp"); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if len(app.activitySnapshot().Tools) != 0 {
		t.Fatal("stopped server retained activity")
	}
}

type activityAlreadyClosedListener struct{}

func (activityAlreadyClosedListener) Close() error { return net.ErrClosed }

func TestActivityExplicitStopAfterContextClosedSocketIsSuccessful(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := newTransferHostManager(ctx, t.TempDir())
	serverCtx, serverCancel := context.WithCancel(ctx)
	s := &transferHost{proto: "ftp", ctx: serverCtx, cancel: serverCancel, done: make(chan struct{}), listener: activityAlreadyClosedListener{}}
	m.servers["ftp"] = s
	if err := m.stop("ftp"); err != nil {
		t.Fatalf("already stopped socket returned failure: %v", err)
	}
	if len((&application{transfers: m}).activitySnapshot().Tools) != 0 {
		t.Fatal("already closed listener retained activity")
	}
}
