package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// These fixtures hold the request body at an actual read boundary. A check
// performed only before decoding therefore cannot pass accidentally because
// the entire request arrived before the primary window started closing.
type delayedTerminalBody struct {
	reader  io.Reader
	entered chan struct{}
	release <-chan struct{}
	once    sync.Once
}

func (body *delayedTerminalBody) Read(p []byte) (int, error) {
	body.once.Do(func() {
		close(body.entered)
		<-body.release
	})
	return body.reader.Read(p)
}

func (body *delayedTerminalBody) Close() error { return nil }

func TestTerminalConnectBodyCannotOutlivePrimaryOwnership(t *testing.T) {
	for _, closeHome := range []bool{false, true} {
		t.Run(fmt.Sprintf("homeAbsent=%v", closeHome), func(t *testing.T) {
			a := testApplication(t)
			a.launchTerminalView = func(context.Context, terminalViewLaunch) (<-chan error, error) { return nil, nil }
			kept := viewTestConnected(t, a)
			view, err := a.views.create(kept.ID, viewTestSnapshot(a, kept))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := a.views.ready(view.ID); err != nil {
				t.Fatal(err)
			}

			release := make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			defer unblock()
			body := &delayedTerminalBody{
				reader:  strings.NewReader(`{"type":"telnet","host":"127.0.0.1","port":1}`),
				entered: make(chan struct{}), release: release,
			}
			request := httptest.NewRequest(http.MethodPost, "http://"+a.host+"/api/terminals", body)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Session-Token", a.token)
			response := httptest.NewRecorder()
			done := make(chan struct{})
			go func() { defer close(done); a.ServeHTTP(response, request) }()
			select {
			case <-body.entered:
			case <-time.After(3 * time.Second):
				t.Fatal("the connection request did not reach body decoding")
			}
			a.beginPrimaryClose()
			if closeHome && !a.finishPrimaryClose() {
				t.Fatal("the live detached view did not retain the workspace")
			}
			if a.ctx.Err() != nil {
				t.Fatal("fixture requires the detached transport to keep the owner alive")
			}
			unblock()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("delayed connection request did not finish")
			}
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("late connection status %d: %s", response.Code, response.Body.String())
			}
			sessions := a.terminals.List()
			if len(sessions) != 1 || sessions[0].ID != kept.ID || sessions[0].State != "connected" {
				t.Fatalf("closing home created or disrupted a connection: %+v", sessions)
			}
		})
	}
}

type terminalReadSignalBody struct {
	io.ReadCloser
	entered chan struct{}
	once    sync.Once
}

func (body *terminalReadSignalBody) Read(p []byte) (int, error) {
	body.once.Do(func() { close(body.entered) })
	return body.ReadCloser.Read(p)
}

func TestShutdownDrainsPendingSettingsBodyBeforeDataCleanup(t *testing.T) {
	a := testApplication(t)
	if response := requestApp(a, http.MethodPut, "/api/state", `{"theme":"dark"}`, true); response.Code != http.StatusOK {
		t.Fatalf("initial state: %d %s", response.Code, response.Body.String())
	}
	entered, handled := make(chan struct{}), make(chan struct{})
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = &terminalReadSignalBody{ReadCloser: r.Body, entered: entered}
		defer close(handled)
		a.ServeHTTP(w, r)
	}))
	server.Config.BaseContext = func(net.Listener) context.Context { return a.ctx }
	server.Config.ReadTimeout = 5 * time.Second
	server.Start()
	defer server.Close()
	a.host = strings.TrimPrefix(server.URL, "http://")
	connection, err := net.DialTimeout("tcp4", a.host, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if _, err := fmt.Fprintf(connection, "PUT /api/state HTTP/1.1\r\nHost: %s\r\nContent-Type: application/json\r\nX-Session-Token: %s\r\nContent-Length: 128\r\n\r\n{", a.host, a.token); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("the partial settings request did not enter the handler")
	}
	a.stop()

	// A short grace represents the same shutdown sequence as main: cancellation
	// and graceful shutdown do not themselves unblock an unfinished body read.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	err = server.Config.Shutdown(ctx)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unfinished request did not require forced close: %v", err)
	}
	drained := make(chan struct{})
	go func() {
		a.httpGate.Lock()
		defer a.httpGate.Unlock()
		close(drained)
	}()
	select {
	case <-drained:
		t.Fatal("data cleanup passed the gate while a state handler was reading")
	case <-time.After(25 * time.Millisecond):
	}
	if err := server.Config.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-handled:
	case <-time.After(3 * time.Second):
		t.Fatal("forced connection close did not release the settings handler")
	}
	select {
	case <-drained:
	case <-time.After(3 * time.Second):
		t.Fatal("the completed settings handler retained the cleanup gate")
	}
	if err := os.Remove(a.dataPath); err != nil {
		t.Fatal(err)
	}
	response := requestApp(a, http.MethodPut, "/api/state", `{"theme":"light"}`, true)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("state mutation after shutdown: %d %s", response.Code, response.Body.String())
	}
	if _, err := os.Stat(a.dataPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a late request recreated deleted app data: %v", err)
	}
}
