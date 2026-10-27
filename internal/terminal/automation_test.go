package terminal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func automationTestTarget(t *testing.T, m *Manager, id string) AutomationTarget {
	t.Helper()
	for _, target := range m.AutomationTargets() {
		if target.ID == id {
			return target
		}
	}
	t.Fatalf("connected session %s is absent from automation targets", id)
	return AutomationTarget{}
}

func automationTestRead(t *testing.T, sub *AutomationSubscription) AutomationOutput {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	output, err := sub.Read(ctx)
	if err != nil {
		t.Fatalf("reading automation output: %v", err)
	}
	return output
}

func automationTestEOF(t *testing.T, sub *AutomationSubscription) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if output, err := sub.Read(ctx); !errors.Is(err, io.EOF) {
		t.Fatalf("read after draining ended subscription = %+v, %v; want EOF", output, err)
	}
}

func TestAutomationReconnectChangesGenerationAndRejectsStaleWrites(t *testing.T) {
	connections := make(chan *fakeTerminal, 2)
	m := testManager(t, func(context.Context, ConnectRequest, transportCallbacks) (transport, error) {
		f := newFakeTerminal(false)
		connections <- f
		return f, nil
	})
	req := terminalRequest()
	req.ProfileID, req.LineEnding = "fixture-profile", "lf"
	req.AuthMethod, req.Password = "keyboard", ""
	info, err := m.Connect(req)
	if err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	first := <-connections
	original := automationTestTarget(t, m, info.ID)
	if original.Generation == 0 || original.ProfileID != req.ProfileID || original.LineEnding != req.LineEnding || original.Host != req.Host || original.Username != req.Username {
		t.Fatalf("automation target lost connection metadata: %+v", original)
	}
	if err := m.AutomationWrite(original.ID, original.Generation, []byte("original\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-first.writes:
		if string(got) != "original\n" {
			t.Fatalf("original transport received %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("automation input did not reach its selected transport")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := m.Reconnect(ctx, info.ID); err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	replacement := <-connections
	next := automationTestTarget(t, m, info.ID)
	if next.Generation <= original.Generation {
		t.Fatalf("reconnect generation = %d; want newer than %d", next.Generation, original.Generation)
	}
	metadata := next
	metadata.Generation = original.Generation
	if metadata != original {
		t.Fatalf("reconnect changed automation target metadata: before %+v, after %+v", original, next)
	}
	if sessions := m.List(); len(sessions) != 1 || sessions[0].ProfileID != req.ProfileID || sessions[0].LineEnding != req.LineEnding {
		t.Fatalf("reconnect lost public session metadata: %+v", sessions)
	}
	if err := m.AutomationWrite(original.ID, original.Generation, []byte("stale\n")); !errors.Is(err, ErrTransportChanged) {
		t.Errorf("stale automation write returned %v; want ErrTransportChanged", err)
	}
	if sub, err := m.AutomationSubscribe(original.ID, original.Generation); !errors.Is(err, ErrTransportChanged) {
		if sub != nil {
			sub.Close()
		}
		t.Errorf("stale automation subscription returned %v; want ErrTransportChanged", err)
	}
	if err := m.AutomationWrite(next.ID, next.Generation, []byte("fresh\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-replacement.writes:
		if string(got) != "fresh\n" {
			t.Fatalf("replacement transport received %q; stale input must never reach it", got)
		}
	case <-time.After(time.Second):
		t.Fatal("fresh automation input did not reach the replacement transport")
	}
	select {
	case <-first.closed:
	default:
		t.Fatal("reconnect left the original transport open")
	}
}

func TestAutomationSubscriptionRetainsAllFreshOutputBeyondUIRing(t *testing.T) {
	outputs := make(chan func([]byte), 1)
	m := testManager(t, func(_ context.Context, _ ConnectRequest, cb transportCallbacks) (transport, error) {
		outputs <- cb.Output
		return newFakeTerminal(false), nil
	})
	info, err := m.Connect(terminalRequest())
	if err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	output := <-outputs
	output([]byte("historical prompt must not be replayed\r\n"))
	target := automationTestTarget(t, m, info.ID)
	sub, err := m.AutomationSubscribe(target.ID, target.Generation)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := sub.Read(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("new subscription replayed historical output: %+v, %v", got, err)
	}
	m.mu.Lock()
	m.eventBudget = 64 * 1024
	m.mu.Unlock()
	payload := make([]byte, 4*64*1024+73)
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	want := append([]byte(nil), payload...)
	output(payload)
	clear(payload)
	batch, err := m.Events(context.Background(), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !batch.Lost || batch.LostEvents == 0 {
		t.Fatal("fixture did not exceed the UI event retention budget")
	}
	var got []byte
	var previous uint64
	for len(got) < len(want) {
		chunk := automationTestRead(t, sub)
		if chunk.Seq <= previous {
			t.Fatalf("automation output sequence did not increase: %d after %d", chunk.Seq, previous)
		}
		previous = chunk.Seq
		got = append(got, chunk.Data...)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("automation output changed or was discarded: got %d bytes, want %d", len(got), len(want))
	}
	sub.Close()
	automationTestEOF(t, sub)
}

func TestAutomationWriteCursorDistinguishesQueuedOldOutput(t *testing.T) {
	outputs := make(chan func([]byte), 1)
	f := newFakeTerminal(false)
	m := testManager(t, func(_ context.Context, _ ConnectRequest, cb transportCallbacks) (transport, error) {
		outputs <- cb.Output
		return f, nil
	})
	info, err := m.Connect(terminalRequest())
	if err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	output := <-outputs
	target := automationTestTarget(t, m, info.ID)
	sub, err := m.AutomationSubscribe(target.ID, target.Generation)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	output([]byte("old prompt> "))
	cursor, err := m.AutomationWriteCursor(target.ID, target.Generation, []byte("show status\r"))
	if err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-f.writes:
		if string(got) != "show status\r" {
			t.Fatalf("transport received %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("cursor write did not reach transport")
	}
	output([]byte("command complete\r\nnew prompt> "))
	old := automationTestRead(t, sub)
	fresh := automationTestRead(t, sub)
	if string(old.Data) != "old prompt> " || old.Seq > cursor {
		t.Fatalf("pending pre-write output was marked fresh: output %+v, cursor %d", old, cursor)
	}
	if string(fresh.Data) != "command complete\r\nnew prompt> " || fresh.Seq <= cursor {
		t.Fatalf("post-write output was not marked fresh: output %+v, cursor %d", fresh, cursor)
	}
}

func TestAutomationWriteDeliversLargeStepsInBoundedChunks(t *testing.T) {
	for _, test := range []struct {
		name string
		data []byte
	}{
		{name: "64 KiB command plus CRLF", data: append(bytes.Repeat([]byte{'c'}, maxInputChunk), '\r', '\n')},
		{name: "maximum queue size", data: bytes.Repeat([]byte{'m'}, maxQueuedInput)},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newFakeTerminal(false)
			m := testManager(t, func(context.Context, ConnectRequest, transportCallbacks) (transport, error) {
				return f, nil
			})
			info, err := m.Connect(terminalRequest())
			if err != nil {
				t.Fatal(err)
			}
			awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
			target := automationTestTarget(t, m, info.ID)
			want := append([]byte(nil), test.data...)
			if _, err := m.AutomationWriteCursor(target.ID, target.Generation, test.data); err != nil {
				t.Fatalf("supported large automation step was rejected: %v", err)
			}
			clear(test.data)
			var got []byte
			for len(got) < len(want) {
				select {
				case chunk := <-f.writes:
					if len(chunk) == 0 || len(chunk) > maxInputChunk {
						t.Fatalf("transport write size = %d; want 1..%d", len(chunk), maxInputChunk)
					}
					got = append(got, chunk...)
				case <-time.After(time.Second):
					t.Fatalf("large automation step delivered %d of %d bytes", len(got), len(want))
				}
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("large automation step changed: delivered %d bytes, want %d", len(got), len(want))
			}
		})
	}
}

func TestAutomationWriteRejectsOversizeWithoutEnqueue(t *testing.T) {
	f := newFakeTerminal(true)
	m := testManager(t, func(context.Context, ConnectRequest, transportCallbacks) (transport, error) {
		return f, nil
	})
	info, err := m.Connect(terminalRequest())
	if err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	target := automationTestTarget(t, m, info.ID)
	if _, err := m.AutomationWriteCursor(target.ID, target.Generation, make([]byte, maxQueuedInput+1)); err == nil {
		t.Fatal("accepted an automation step larger than the queue budget")
	}
	m.mu.Lock()
	s := m.sessions[info.ID]
	queuedBytes, queuedChunks := s.queuedInput, len(s.input)
	m.mu.Unlock()
	if queuedBytes != 0 || queuedChunks != 0 {
		t.Fatalf("oversized step partially queued: %d bytes in %d chunks", queuedBytes, queuedChunks)
	}
	select {
	case data := <-f.writes:
		t.Fatalf("oversized step reached transport: %d bytes", len(data))
	default:
	}
}

func TestAutomationWriteRejectsInsufficientCapacityAtomically(t *testing.T) {
	for _, capacity := range []string{"byte budget", "chunk slots"} {
		t.Run(capacity, func(t *testing.T) {
			f := newFakeTerminal(true)
			m := testManager(t, func(context.Context, ConnectRequest, transportCallbacks) (transport, error) {
				return f, nil
			})
			info, err := m.Connect(terminalRequest())
			if err != nil {
				t.Fatal(err)
			}
			awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
			target := automationTestTarget(t, m, info.ID)
			if err := m.AutomationWrite(target.ID, target.Generation, []byte{'b'}); err != nil {
				t.Fatal(err)
			}
			select {
			case <-f.writes:
			case <-time.After(time.Second):
				t.Fatal("fixture writer did not block")
			}
			m.mu.Lock()
			s := m.sessions[info.ID]
			inputCapacity := cap(s.input)
			m.mu.Unlock()
			if capacity == "byte budget" {
				// Leave exactly one full chunk's byte budget for a two-chunk step.
				if err := m.AutomationWrite(target.ID, target.Generation, make([]byte, maxQueuedInput-maxInputChunk-1)); err != nil {
					t.Fatal(err)
				}
			} else {
				// Small queued writes leave one slot but ample byte budget.
				for i := 0; i < inputCapacity-1; i++ {
					if err := m.AutomationWrite(target.ID, target.Generation, []byte{'q'}); err != nil {
						t.Fatal(err)
					}
				}
			}
			m.mu.Lock()
			beforeBytes, beforeChunks := s.queuedInput, len(s.input)
			m.mu.Unlock()
			result := make(chan error, 1)
			go func() {
				result <- m.AutomationWrite(target.ID, target.Generation, make([]byte, maxInputChunk+2))
			}()
			select {
			case err := <-result:
				if !errors.Is(err, ErrQueueFull) {
					t.Fatalf("insufficient %s returned %v; want ErrQueueFull", capacity, err)
				}
			case <-time.After(time.Second):
				// Release a missing-preflight regression so cleanup cannot deadlock.
				for {
					select {
					case <-s.input:
					default:
						goto released
					}
				}
			released:
				select {
				case <-result:
				case <-time.After(time.Second):
					t.Fatal("automation write remained blocked after releasing queue slots")
				}
				t.Fatal("automation write blocked instead of rejecting insufficient capacity")
			}
			m.mu.Lock()
			afterBytes, afterChunks := s.queuedInput, len(s.input)
			m.mu.Unlock()
			if afterBytes != beforeBytes || afterChunks != beforeChunks {
				t.Fatalf("rejected step partially queued: before %d bytes/%d chunks, after %d bytes/%d chunks", beforeBytes, beforeChunks, afterBytes, afterChunks)
			}
		})
	}
}

func TestAutomationSubscriptionDrainsBeforeEOF(t *testing.T) {
	for _, end := range []string{"subscription close", "disconnect", "remote EOF", "manager close"} {
		t.Run(end, func(t *testing.T) {
			outputs := make(chan func([]byte), 1)
			f := newFakeTerminal(false)
			m := testManager(t, func(_ context.Context, _ ConnectRequest, cb transportCallbacks) (transport, error) {
				outputs <- cb.Output
				return f, nil
			})
			info, err := m.Connect(terminalRequest())
			if err != nil {
				t.Fatal(err)
			}
			awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
			output := <-outputs
			target := automationTestTarget(t, m, info.ID)
			sub, err := m.AutomationSubscribe(target.ID, target.Generation)
			if err != nil {
				t.Fatal(err)
			}
			defer sub.Close()
			output([]byte("first\r\n"))
			output([]byte("second\r\n"))
			switch end {
			case "subscription close":
				sub.Close()
			case "disconnect":
				if err := m.Disconnect(info.ID); err != nil {
					t.Fatal(err)
				}
			case "remote EOF":
				f.done <- io.EOF
				awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "closed" })
			case "manager close":
				if err := m.Close(); err != nil {
					t.Fatal(err)
				}
			}
			output([]byte("after end must not be delivered\r\n"))
			for _, want := range []string{"first\r\n", "second\r\n"} {
				if got := automationTestRead(t, sub); string(got.Data) != want {
					t.Fatalf("draining ended subscription received %q; want %q", got.Data, want)
				}
			}
			automationTestEOF(t, sub)
			sub.Close()
			automationTestEOF(t, sub)
			if end != "subscription close" && len(m.AutomationTargets()) != 0 {
				t.Fatal("ended transport remains available as an automation target")
			}
		})
	}
}

func TestAutomationMetadataValidation(t *testing.T) {
	var opened atomic.Int32
	m := testManager(t, func(context.Context, ConnectRequest, transportCallbacks) (transport, error) {
		opened.Add(1)
		return newFakeTerminal(false), nil
	})
	for _, test := range []struct {
		name       string
		profileID  string
		lineEnding string
	}{
		{name: "profile too long", profileID: strings.Repeat("p", 81)},
		{name: "profile newline", profileID: "profile\nother"},
		{name: "profile null", profileID: "profile\x00"},
		{name: "profile control", profileID: "profile\u0085"},
		{name: "literal carriage return", lineEnding: "\r"},
		{name: "uppercase line ending", lineEnding: "CRLF"},
		{name: "unsupported line ending", lineEnding: "auto"},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := terminalRequest()
			req.ProfileID, req.LineEnding = test.profileID, test.lineEnding
			if _, err := m.Connect(req); err == nil {
				t.Fatal("accepted invalid automation metadata")
			}
		})
	}
	if opened.Load() != 0 || len(m.List()) != 0 {
		t.Fatal("invalid automation metadata initiated a terminal connection")
	}
	for _, lineEnding := range []string{"cr", "lf", "crlf"} {
		req := terminalRequest()
		req.ProfileID, req.LineEnding = strings.Repeat("p", 80), lineEnding
		got, err := normalizeRequest(req)
		if err != nil {
			t.Fatalf("rejected supported metadata: %v", err)
		}
		if got.ProfileID != req.ProfileID || got.LineEnding != lineEnding {
			t.Fatalf("normalization changed supported metadata: %+v", got)
		}
	}
}

func TestSetLineEndingUpdatesLiveSerialAndReconnect(t *testing.T) {
	requests := make(chan ConnectRequest, 2)
	opener := func(_ context.Context, req ConnectRequest, _ transportCallbacks) (transport, error) {
		requests <- req
		return newFakeTerminal(false), nil
	}
	m := testManager(t, opener)
	m.mu.Lock()
	m.openers["serial"] = opener
	m.mu.Unlock()
	port := "/dev/ttyUSB0"
	if runtime.GOOS == "windows" {
		port = "COM3"
	}
	info, err := m.Connect(ConnectRequest{Type: "serial", SerialPort: port, LineEnding: "crlf"})
	if err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	<-requests
	original := automationTestTarget(t, m, info.ID)
	for _, invalid := range []string{"", "CRLF", "\r\n", "auto"} {
		if err = m.SetLineEnding(info.ID, invalid); err == nil {
			t.Fatalf("accepted line ending %q", invalid)
		}
	}
	for _, ending := range []string{"cr", "crlf", "lf"} {
		if err = m.SetLineEnding(info.ID, ending); err != nil {
			t.Fatal(err)
		}
		target := automationTestTarget(t, m, info.ID)
		if target.LineEnding != ending || target.Generation != original.Generation {
			t.Fatalf("live metadata was not updated in place: %+v", target)
		}
		sessions := m.List()
		if len(sessions) != 1 || sessions[0].LineEnding != ending {
			t.Fatalf("public metadata not updated: %+v", sessions)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err = m.Reconnect(ctx, info.ID); err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	if req := <-requests; req.LineEnding != "lf" {
		t.Fatalf("reconnect used old line ending: %+v", req)
	}
	if target := automationTestTarget(t, m, info.ID); target.LineEnding != "lf" {
		t.Fatalf("reconnect metadata lost live preference: %+v", target)
	}
	if err = m.Disconnect(info.ID); err != nil {
		t.Fatal(err)
	}
	if err = m.SetLineEnding(info.ID, "cr"); err != nil {
		t.Fatalf("disconnected preference update failed: %v", err)
	}
	if _, err = m.Reconnect(ctx, info.ID); err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	if req := <-requests; req.LineEnding != "cr" {
		t.Fatalf("reconnect lost disconnected preference: %+v", req)
	}
	if err = m.SetLineEnding("missing", "cr"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing update=%v", err)
	}
}

func TestSetLineEndingRejectsNonSerialSession(t *testing.T) {
	m := testManager(t, func(context.Context, ConnectRequest, transportCallbacks) (transport, error) {
		return newFakeTerminal(false), nil
	})
	info, err := m.Connect(terminalRequest())
	if err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	if err = m.SetLineEnding(info.ID, "lf"); err == nil {
		t.Fatal("changed SSH line ending")
	}
	if target := automationTestTarget(t, m, info.ID); target.LineEnding != "cr" {
		t.Fatalf("rejected update changed metadata: %+v", target)
	}
}
