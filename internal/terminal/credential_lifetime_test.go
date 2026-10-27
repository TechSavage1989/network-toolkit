package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func assertNoRetainedAuthentication(t *testing.T, request ConnectRequest) {
	t.Helper()
	if request.Password != "" || request.PrivateKey != "" || request.Passphrase != "" {
		t.Fatal("retained connection settings contain plaintext authentication")
	}
}

func TestManualAuthenticationRequiresReentryWithoutClosingTransport(t *testing.T) {
	for _, method := range []string{"password", "key"} {
		t.Run(method, func(t *testing.T) {
			var opens atomic.Int32
			first := newFakeTerminal(false)
			m := testManager(t, func(context.Context, ConnectRequest, transportCallbacks) (transport, error) {
				opens.Add(1)
				return first, nil
			})
			req := terminalRequest()
			if method == "key" {
				req.AuthMethod, req.Password = "key", ""
				req.PrivateKey, req.Passphrase = "private-key-fixture", "passphrase-fixture"
			}
			info, err := m.Connect(req)
			if err != nil {
				t.Fatal(err)
			}
			awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
			m.mu.Lock()
			retained := m.sessions[info.ID].request
			m.mu.Unlock()
			assertNoRetainedAuthentication(t, retained)
			if _, err := m.Reconnect(context.Background(), info.ID); !errors.Is(err, ErrAuthenticationRequired) {
				t.Fatalf("manual reconnect = %v, want ErrAuthenticationRequired", err)
			}
			if opens.Load() != 1 {
				t.Fatal("manual reconnect reused retained authentication")
			}
			select {
			case <-first.closed:
				t.Fatal("missing reconnect authentication closed an established session")
			default:
			}
		})
	}
}

func TestSavedReconnectResolvesCurrentCredentialAndFailsClosed(t *testing.T) {
	for _, failure := range []string{"missing-resolver", "deleted", "locked", "wrong-host", "wrong-port", "wrong-user", "wrong-id", "wrong-type", "empty-password", "missing-private-key"} {
		t.Run(failure, func(t *testing.T) {
			var opens atomic.Int32
			first := newFakeTerminal(false)
			m := testManager(t, func(context.Context, ConnectRequest, transportCallbacks) (transport, error) {
				opens.Add(1)
				return first, nil
			})
			req := terminalRequest()
			req.CredentialID = "credential-bound-fixture"
			info, err := m.Connect(req)
			if err != nil {
				t.Fatal(err)
			}
			awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
			if failure != "missing-resolver" {
				m.SetCredentialResolver(func(_ context.Context, safe ConnectRequest) (ConnectRequest, error) {
					assertNoRetainedAuthentication(t, safe)
					safe.Password = "new-secret-fixture"
					switch failure {
					case "deleted", "locked":
						return ConnectRequest{}, ErrCredentialUnavailable
					case "wrong-host":
						safe.Host = "different.example"
					case "wrong-port":
						safe.Port++
					case "wrong-user":
						safe.Username = "different-user"
					case "wrong-id":
						safe.CredentialID = "different-credential"
					case "wrong-type":
						safe.Type = "telnet"
					case "empty-password":
						safe.Password = ""
					case "missing-private-key":
						safe.AuthMethod, safe.Password = "key", ""
					}
					return safe, nil
				})
			}
			if _, err := m.Reconnect(context.Background(), info.ID); err == nil {
				t.Fatal("invalid or unavailable saved authentication was accepted")
			}
			if opens.Load() != 1 {
				t.Fatal("failed credential lookup reached a new transport")
			}
			select {
			case <-first.closed:
				t.Fatal("failed credential lookup closed the established transport")
			default:
			}
			m.mu.Lock()
			retained, reconnecting := m.sessions[info.ID].request, m.sessions[info.ID].reconnecting
			m.mu.Unlock()
			assertNoRetainedAuthentication(t, retained)
			if reconnecting {
				t.Fatal("failed lookup left a reconnect reservation behind")
			}
		})
	}
}

func TestSavedReconnectUsesUpdatedSecretWithoutRetainingIt(t *testing.T) {
	requests := make(chan ConnectRequest, 3)
	m := testManager(t, func(_ context.Context, req ConnectRequest, _ transportCallbacks) (transport, error) {
		requests <- req
		return newFakeTerminal(false), nil
	})
	req := terminalRequest()
	req.CredentialID = "credential-rotating-fixture"
	info, err := m.Connect(req)
	if err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	<-requests
	var lookups int
	m.SetCredentialResolver(func(_ context.Context, safe ConnectRequest) (ConnectRequest, error) {
		lookups++
		assertNoRetainedAuthentication(t, safe)
		safe.Password = "rotated-fixture-secret"
		return safe, nil
	})
	for attempt := 0; attempt < 2; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := m.Reconnect(ctx, info.ID)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
		if opened := <-requests; opened.Password != "rotated-fixture-secret" || opened.CredentialID != req.CredentialID {
			t.Fatal("reconnect did not use the newly resolved credential")
		}
		m.mu.Lock()
		retained := m.sessions[info.ID].request
		m.mu.Unlock()
		assertNoRetainedAuthentication(t, retained)
	}
	if lookups != 2 {
		t.Fatalf("saved credential was resolved %d times; want once for each reconnect", lookups)
	}
	batch, err := m.Events(context.Background(), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(batch)
	for _, sensitive := range []string{"fixture-secret", "rotated-fixture-secret", req.CredentialID} {
		if strings.Contains(string(raw), sensitive) {
			t.Fatal("reconnect authentication or private reference escaped in public events")
		}
	}
}

func TestSavedReconnectLookupIsCancelableAndDoesNotHoldManagerLock(t *testing.T) {
	first := newFakeTerminal(false)
	m := testManager(t, func(context.Context, ConnectRequest, transportCallbacks) (transport, error) {
		return first, nil
	})
	req := terminalRequest()
	req.CredentialID = "credential-cancel-fixture"
	info, err := m.Connect(req)
	if err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	started := make(chan struct{})
	m.SetCredentialResolver(func(ctx context.Context, safe ConnectRequest) (ConnectRequest, error) {
		// A resolver may inspect the manager without deadlocking it.
		_ = m.List()
		close(started)
		<-ctx.Done()
		return ConnectRequest{}, ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := m.Reconnect(ctx, info.ID); done <- err }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("resolver was blocked by the manager lock")
	}
	if _, err := m.Reconnect(context.Background(), info.ID); !errors.Is(err, ErrBusy) {
		t.Fatalf("duplicate lookup returned %v; want ErrBusy", err)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("lookup cancellation returned %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("lookup cancellation did not finish")
	}
	select {
	case <-first.closed:
		t.Fatal("canceled lookup closed the existing transport")
	default:
	}
}

func TestCredentialReferencesDoNotReachLocalOrUnauthenticatedTransports(t *testing.T) {
	for _, kind := range []string{"cmd", "powershell", "telnet"} {
		req := terminalRequest()
		req.Type, req.CredentialID = kind, "credential-ignored-fixture"
		req.PrivateKey, req.Passphrase = "private-key-fixture", "passphrase-fixture"
		got, err := normalizeRequest(req)
		if err != nil {
			t.Fatal(err)
		}
		assertNoRetainedAuthentication(t, got)
		if got.CredentialID != "" {
			t.Fatalf("%s retained a saved credential reference", kind)
		}
	}
	for _, id := range []string{"../credential", "credential/path", "credential\\path", "credential\x00suffix", strings.Repeat("a", 121)} {
		req := terminalRequest()
		req.CredentialID = id
		if _, err := normalizeRequest(req); err == nil {
			t.Fatal("invalid credential reference was accepted")
		}
	}
}
