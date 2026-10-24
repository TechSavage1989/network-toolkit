package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestReconnectRetainsIdentitySettingsAndCredentials(t *testing.T) {
	for _, kind := range []string{"password", "key", "keyboard", "telnet", "serial"} {
		t.Run(kind, func(t *testing.T) {
			requests := make(chan ConnectRequest, 4)
			transports := make(chan *fakeTerminal, 4)
			opener := func(_ context.Context, req ConnectRequest, cb transportCallbacks) (transport, error) {
				requests <- cloneConnectRequest(req)
				f := newFakeTerminal(false)
				transports <- f
				cb.Output([]byte("retained console output\r\n"))
				return f, nil
			}
			m := testManager(t, opener)
			m.openers["serial"] = opener
			req := terminalRequest()
			req.SSHCompatibility, req.TimeoutMs, req.Port = "legacy", 17000, 2222
			switch kind {
			case "password":
			case "key":
				req.AuthMethod, req.Password = "key", ""
				req.PrivateKey, req.Passphrase = "private-key-reconnect-canary", "passphrase-reconnect-canary"
			case "keyboard":
				req.AuthMethod = "keyboard"
			case "telnet":
				req.Type = "telnet"
			case "serial":
				req.Type, req.SerialPort, req.BaudRate = "serial", "COM7", 115200
				if runtime.GOOS != "windows" {
					req.SerialPort = "/dev/ttyUSB7"
				}
				req.DataBits, req.StopBits, req.Parity, req.FlowControl = 7, "2", "even", "none"
			}
			info, err := m.Connect(req)
			if err != nil {
				t.Fatal(err)
			}
			awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
			original, first := <-requests, <-transports
			if err := m.Resize(info.ID, 132, 38); err != nil {
				t.Fatal(err)
			}
			original.Cols, original.Rows = 132, 38
			for attempt := 0; attempt < 2; attempt++ {
				if attempt == 1 {
					if err := m.Disconnect(info.ID); err != nil {
						t.Fatal(err)
					}
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				next, err := m.Reconnect(ctx, info.ID)
				cancel()
				if err != nil {
					t.Fatal(err)
				}
				if next.ID != info.ID || next.CreatedAt != info.CreatedAt || len(m.List()) != 1 {
					t.Fatal("reconnect replaced the session or created another tab")
				}
				awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
				if got := <-requests; !reflect.DeepEqual(got, original) {
					t.Fatalf("%s reconnect did not preserve the exact connection request", kind)
				}
				<-transports
			}
			select {
			case <-first.closed:
			default:
				t.Fatal("old transport remained open after reconnect")
			}
			batch, err := m.Events(context.Background(), 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(batch)
			for _, secret := range []string{"fixture-secret", "private-key-reconnect-canary", "passphrase-reconnect-canary"} {
				if strings.Contains(string(raw), secret) {
					t.Fatal("reconnect credentials escaped into public session or event data")
				}
			}
			if !strings.Contains(string(raw), "Reconnecting using previous connection settings") {
				t.Fatal("reconnect status was not published")
			}
			m.mu.Lock()
			retained := m.sessions[info.ID]
			m.mu.Unlock()
			if err := m.Disconnect(info.ID); err != nil {
				t.Fatal(err)
			}
			if err := m.Remove(info.ID); err != nil {
				t.Fatal(err)
			}
			m.mu.Lock()
			cleared := reflect.DeepEqual(retained.request, ConnectRequest{})
			m.mu.Unlock()
			if !cleared {
				t.Fatal("removing the tab retained its reconnect credentials")
			}
		})
	}
}

func TestReconnectCancellationAndDuplicateRequests(t *testing.T) {
	opening := make(chan struct{})
	canceled := make(chan struct{})
	release := make(chan struct{})
	m := testManager(t, func(ctx context.Context, _ ConnectRequest, _ transportCallbacks) (transport, error) {
		close(opening)
		<-ctx.Done()
		close(canceled)
		<-release
		return nil, ctx.Err()
	})
	defer close(release)
	info, err := m.Connect(terminalRequest())
	if err != nil {
		t.Fatal(err)
	}
	<-opening
	done := make(chan error, 1)
	go func() { _, err := m.Reconnect(context.Background(), info.ID); done <- err }()
	<-canceled
	if _, err := m.Reconnect(context.Background(), info.ID); !errors.Is(err, ErrBusy) {
		t.Fatalf("duplicate reconnect returned %v", err)
	}
	if err := m.Remove(info.ID); !errors.Is(err, ErrBusy) {
		t.Fatalf("removed a session while its reconnect was reserved: %v", err)
	}
	if err := m.Disconnect(info.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("disconnect did not cancel the pending reconnect: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("pending reconnect did not cancel promptly")
	}
}

func TestControlKeysDoNotRemoveOrCloseLocalSession(t *testing.T) {
	f := newFakeTerminal(false)
	m := testManager(t, func(context.Context, ConnectRequest, transportCallbacks) (transport, error) { return f, nil })
	info, err := m.Connect(terminalRequest())
	if err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	for _, code := range []byte{3, 4, 23, 26, 12, 18} {
		if err := m.Write(info.ID, []byte{code}); err != nil {
			t.Fatal(err)
		}
		select {
		case got := <-f.writes:
			if len(got) != 1 || got[0] != code {
				t.Fatal("control input was modified")
			}
		case <-time.After(time.Second):
			t.Fatal("control input did not reach the transport")
		}
		if sessions := m.List(); len(sessions) != 1 || sessions[0].State != "connected" {
			t.Fatal("a control key closed or removed the local session")
		}
	}
	// A device may itself end a shell. The tab/session record must still exist.
	f.done <- io.EOF
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "closed" })
	if sessions := m.List(); len(sessions) != 1 || sessions[0].ID != info.ID {
		t.Fatal("remote shell exit removed the retained session")
	}
}

func TestManagerCloseReleasesReconnectCredentials(t *testing.T) {
	m := testManager(t, func(context.Context, ConnectRequest, transportCallbacks) (transport, error) {
		return newFakeTerminal(false), nil
	})
	info, err := m.Connect(terminalRequest())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !reflect.DeepEqual(m.sessions[info.ID].request, ConnectRequest{}) {
		t.Fatal("closing the app retained reconnect authentication")
	}
}
