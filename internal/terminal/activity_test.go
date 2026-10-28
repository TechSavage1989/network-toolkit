package terminal

import (
	"context"
	"errors"
	"testing"
)

func TestActivityConnectedCountFollowsTransportLifetime(t *testing.T) {
	first, second := newFakeTerminal(false), newFakeTerminal(false)
	next := make(chan transport, 2)
	next <- first
	next <- second
	m := testManager(t, func(context.Context, ConnectRequest, transportCallbacks) (transport, error) { return <-next, nil })
	if got := m.ConnectedCount(); got != 0 {
		t.Fatalf("empty manager count = %d", got)
	}
	a, err := m.Connect(terminalRequest())
	if err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, a.ID, func(s SessionInfo) bool { return s.State == "connected" })
	b, err := m.Connect(terminalRequest())
	if err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, b.ID, func(s SessionInfo) bool { return s.State == "connected" })
	for i := 0; i < 100; i++ {
		if got := m.ConnectedCount(); got != 2 {
			t.Fatalf("idle connected sessions = %d", got)
		}
	}
	// Unexpected peer close must be observed without the activity reader draining
	// transport.Done or issuing a new network probe.
	_ = first.Close()
	awaitSession(t, m, a.ID, func(s SessionInfo) bool { return s.State == "closed" })
	if got := m.ConnectedCount(); got != 1 {
		t.Fatalf("one remaining session = %d", got)
	}
	if err := m.Disconnect(b.ID); err != nil {
		t.Fatal(err)
	}
	if got := m.ConnectedCount(); got != 0 {
		t.Fatalf("disconnected last session = %d", got)
	}
	_ = m.Close()
	if got := m.ConnectedCount(); got != 0 {
		t.Fatalf("closed manager = %d", got)
	}
}

func TestActivityOpeningFailedAndCanceledSessionsAreNotConnected(t *testing.T) {
	release := make(chan struct{})
	m := testManager(t, func(ctx context.Context, _ ConnectRequest, _ transportCallbacks) (transport, error) {
		select {
		case <-release:
			return nil, errors.New("synthetic authentication failure")
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	info, err := m.Connect(terminalRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got := m.ConnectedCount(); got != 0 {
		t.Fatalf("opening session = %d", got)
	}
	close(release)
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "error" })
	if got := m.ConnectedCount(); got != 0 {
		t.Fatalf("failed authentication = %d", got)
	}
}
