//go:build windows

package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"networktoolkit/internal/capture"
)

// Windows native pipe + launcher-spy acceptance. The real runas call is never
// used and no Packet Monitor API, capture, user adapter or settings is touched.
func TestPacketCapturePrivilegeBoundaryNavigationAndExplicitCancel(t *testing.T) {
	read, launch := packetCaptureReadAdapters, packetCaptureLaunch
	t.Cleanup(func() { packetCaptureReadAdapters, packetCaptureLaunch = read, launch })
	var reads, launches atomic.Int32
	var releaseLaunch <-chan struct{}
	packetCaptureReadAdapters = func(ctx context.Context) ([]packetSystemAdapter, error) {
		reads.Add(1)
		return []packetSystemAdapter{{ID: "{00000000-0000-0000-0000-000000000001}", Name: "Synthetic NIC"}}, ctx.Err()
	}
	packetCaptureLaunch = func(id string, parent uint32) (syscall.Handle, error) {
		launches.Add(1)
		if releaseLaunch != nil {
			<-releaseLaunch
		}
		if !validLocalHelperID(id) || parent == 0 {
			return 0, errors.New("invalid synthetic launch identity")
		}
		return 0, syscall.Errno(1223)
	}
	manager := newPacketCaptureManager(context.Background())
	defer manager.close()
	app := &application{packetCaptures: manager}
	for i := 0; i < 3; i++ {
		for _, route := range []struct{ method, path string }{{"GET", "/capabilities"}, {"POST", "/interfaces"}, {"GET", ""}} {
			r := httptest.NewRequest(route.method, "/api/packet-captures"+route.path, strings.NewReader("{}"))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			app.handlePacketCapture(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("%s %s: %d %s", route.method, route.path, w.Code, w.Body.String())
			}
		}
	}
	if launches.Load() != 0 || reads.Load() != 3 {
		t.Fatal("ordinary navigation/refresh elevated", reads.Load(), launches.Load())
	}
	// The old elevated enumeration request and malformed selections fail before
	// pipe creation/launcher invocation, even when called directly.
	for _, request := range []packetCaptureRequest{{Version: 1, Operation: "interfaces"}, {Version: 2, Operation: "capture"}, {Version: 1, Operation: "capture", Options: capture.Options{Interface: "file:1 --capture"}}} {
		if err := packetCaptureRoundTrip(context.Background(), request, nil, nil); err == nil {
			t.Fatal("invalid privileged operation accepted")
		}
	}
	if launches.Load() != 0 {
		t.Fatal("invalid request elevated")
	}
	for attempt := int32(1); attempt <= 2; attempt++ {
		gate := make(chan struct{})
		releaseLaunch = gate
		info, err := manager.start(capture.Options{Interface: "adapter:{00000000-0000-0000-0000-000000000001}"})
		if err != nil {
			t.Fatal(err)
		}
		if _, duplicateErr := manager.start(capture.Options{}); duplicateErr == nil {
			close(gate)
			t.Fatal("duplicate pending Start was accepted")
		}
		close(gate)
		manager.mu.RLock()
		done := manager.sessions[info.ID].done
		manager.mu.RUnlock()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("synthetic UAC cancellation did not settle")
		}
		rows := manager.list()
		if launches.Load() != attempt || rows[0].State != "failed" || !strings.Contains(rows[0].Error, "cancelled. No capture was started") {
			t.Fatal(launches.Load(), rows)
		}
		for i := 0; i < 5; i++ {
			_ = manager.list() // polling and retained result review do not retry
		}
		if launches.Load() != attempt {
			t.Fatal("automatic elevation retry after cancellation")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := packetCaptureRun(ctx, capture.Options{}, nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	close(stop)
	if err := packetCaptureRun(context.Background(), capture.Options{}, stop, nil); err != nil || launches.Load() != 2 {
		t.Fatal("pre-cancelled capture elevated", err, launches.Load())
	}
}
