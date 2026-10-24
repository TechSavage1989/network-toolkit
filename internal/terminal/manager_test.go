package terminal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeTerminal struct {
	done    chan error
	closed  chan struct{}
	writes  chan []byte
	resizes chan [2]int
	block   bool
	once    sync.Once
}

func newFakeTerminal(block bool) *fakeTerminal {
	return &fakeTerminal{done: make(chan error, 1), closed: make(chan struct{}), writes: make(chan []byte, 64), resizes: make(chan [2]int, 64), block: block}
}
func (f *fakeTerminal) Write(data []byte) (int, error) {
	copy := append([]byte(nil), data...)
	select {
	case f.writes <- copy:
	case <-f.closed:
		return 0, io.EOF
	}
	if f.block {
		<-f.closed
		return 0, io.EOF
	}
	return len(data), nil
}
func (f *fakeTerminal) Resize(cols, rows int) error {
	select {
	case f.resizes <- [2]int{cols, rows}:
		return nil
	case <-f.closed:
		return io.EOF
	}
}
func (f *fakeTerminal) Close() error {
	f.once.Do(func() { close(f.closed); f.done <- nil; close(f.done) })
	return nil
}
func (f *fakeTerminal) Done() <-chan error { return f.done }

func testManager(t *testing.T, opener transportOpener) *Manager {
	t.Helper()
	m := newManager(context.Background(), nil, map[string]transportOpener{"ssh": opener, "telnet": opener})
	t.Cleanup(func() { _ = m.Close() })
	return m
}

func terminalRequest() ConnectRequest {
	return ConnectRequest{Type: "ssh", Host: "127.0.0.1", Username: "fixture-user", Password: "fixture-secret", AuthMethod: "password", Cols: 80, Rows: 24}
}

func awaitSession(t *testing.T, m *Manager, id string, predicate func(SessionInfo) bool) SessionInfo {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var after uint64
	for {
		batch, err := m.Events(ctx, after, 250*time.Millisecond)
		if err != nil {
			t.Fatalf("waiting for terminal state: %v; sessions %+v", err, m.List())
		}
		after = batch.Next
		for _, info := range batch.Sessions {
			if info.ID == id && predicate(info) {
				return info
			}
		}
	}
}

func TestManagerValidationAndLiveSessionLimit(t *testing.T) {
	var started atomic.Int32
	m := testManager(t, func(ctx context.Context, _ ConnectRequest, _ transportCallbacks) (transport, error) {
		started.Add(1)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	for _, bad := range []ConnectRequest{{Type: "other"}, {Type: "ssh", Host: "http://host", Username: "u"}, {Type: "ssh", Host: "host", Username: "u", Port: 65536}, {Type: "ssh", Host: "host", Username: "u", Cols: 501}, {Type: "ssh", Host: "host", Username: "u", TimeoutMs: 999}, {Type: "ssh", Host: "host", Username: "u", AuthMethod: "key"}} {
		if _, err := m.Connect(bad); err == nil {
			t.Errorf("accepted invalid request %+v", bad)
		}
	}
	if started.Load() != 0 {
		t.Fatal("validation initiated transport work")
	}
	for i := 0; i < maxLiveSessions; i++ {
		if _, err := m.Connect(terminalRequest()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.Connect(terminalRequest()); !errors.Is(err, ErrLimit) {
		t.Fatalf("ninth live session returned %v", err)
	}
	done := make(chan struct{})
	go func() { _ = m.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation did not release opening sessions")
	}
	if _, err := m.Connect(terminalRequest()); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
}

func TestManagerBoundedInputResizeAndDisconnect(t *testing.T) {
	f := newFakeTerminal(true)
	m := testManager(t, func(context.Context, ConnectRequest, transportCallbacks) (transport, error) { return f, nil })
	info, err := m.Connect(terminalRequest())
	if err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	chunk := bytes.Repeat([]byte{'a'}, maxInputChunk)
	for i := 0; i < 4; i++ {
		if err := m.Write(info.ID, chunk); err != nil {
			t.Fatal(err)
		}
	}
	chunk[0] = 'z'
	if err := m.Write(info.ID, []byte("overflow")); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("queue overflow returned %v", err)
	}
	select {
	case written := <-f.writes:
		if written[0] != 'a' {
			t.Fatal("caller input mutation changed queued bytes")
		}
	case <-time.After(time.Second):
		t.Fatal("writer did not start")
	}
	if err := m.Resize(info.ID, 120, 40); err != nil {
		t.Fatal(err)
	}
	if err := m.Remove(info.ID); !errors.Is(err, ErrBusy) {
		t.Fatal("removed a live terminal without disconnect")
	}
	if err := m.Disconnect(info.ID); err != nil {
		t.Fatal(err)
	}
	if err := m.Write(info.ID, []byte("after close")); !errors.Is(err, ErrNotConnected) {
		t.Fatal(err)
	}
	if err := m.Remove(info.ID); err != nil {
		t.Fatal(err)
	}
	if len(m.List()) != 0 {
		t.Fatal("removed session remains listed")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestManagerResizeUsesColumnsAndRows(t *testing.T) {
	f := newFakeTerminal(false)
	m := testManager(t, func(context.Context, ConnectRequest, transportCallbacks) (transport, error) { return f, nil })
	info, err := m.Connect(terminalRequest())
	if err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	if err := m.Resize(info.ID, 132, 43); err != nil {
		t.Fatal(err)
	}
	select {
	case dimensions := <-f.resizes:
		if dimensions != [2]int{132, 43} {
			t.Fatal(dimensions)
		}
	case <-time.After(time.Second):
		t.Fatal("resize not dispatched")
	}
}

func TestManagerHistoryEvictionPublishesRemoval(t *testing.T) {
	m := testManager(t, func(ctx context.Context, _ ConnectRequest, _ transportCallbacks) (transport, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	m.mu.Lock()
	for i := 0; i < maxSessionHistory; i++ {
		id := fmt.Sprintf("finished-%03d", i)
		m.sessions[id] = &liveSession{finished: true, info: SessionInfo{
			ID: id, Type: "ssh", State: "closed", CreatedAt: time.Unix(int64(i), 0).UTC().Format(time.RFC3339Nano),
		}}
	}
	m.mu.Unlock()
	connected, err := m.Connect(terminalRequest())
	if err != nil {
		t.Fatal(err)
	}
	batch, err := m.Events(context.Background(), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Sessions) != maxSessionHistory {
		t.Fatalf("retained %d sessions, want %d", len(batch.Sessions), maxSessionHistory)
	}
	for _, info := range batch.Sessions {
		if info.ID == "finished-000" {
			t.Fatal("oldest finished session was not evicted")
		}
	}
	if len(batch.Events) < 2 || batch.Events[0].SessionID != "finished-000" || batch.Events[0].Type != "status" || batch.Events[0].Session == nil || batch.Events[0].Session.State != "removed" {
		t.Fatalf("history eviction did not notify an existing UI tab: %+v", batch.Events)
	}
	if batch.Events[1].SessionID != connected.ID || batch.Events[1].Session == nil || batch.Events[1].Session.State != "connecting" {
		t.Fatalf("removal did not precede the replacement session: %+v", batch.Events)
	}
}

func TestManagerSharedFeedBoundsAndExplicitLoss(t *testing.T) {
	outputs := make(chan func([]byte), 8)
	m := testManager(t, func(_ context.Context, _ ConnectRequest, cb transportCallbacks) (transport, error) {
		outputs <- cb.Output
		return newFakeTerminal(false), nil
	})
	ids := map[string]bool{}
	for i := 0; i < 8; i++ {
		info, err := m.Connect(terminalRequest())
		if err != nil {
			t.Fatal(err)
		}
		ids[info.ID] = true
		awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	}
	var callbacks []func([]byte)
	for i := 0; i < 8; i++ {
		callbacks = append(callbacks, <-outputs)
	}
	for i, output := range callbacks {
		output([]byte{0xff, 0x00, byte(i), 0x1b, '['})
	}
	batch, err := m.Events(context.Background(), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, event := range batch.Events {
		if event.Type == "output" {
			seen[event.SessionID] = true
		}
	}
	if len(seen) != 8 || len(batch.Sessions) != 8 {
		t.Fatalf("one shared feed missed sessions: %d/%d", len(seen), len(batch.Sessions))
	}
	for id := range seen {
		if !ids[id] {
			t.Fatal("output belongs to another session")
		}
	}
	m.mu.Lock()
	m.eventBudget = 64 * 1024
	m.mu.Unlock()
	data := bytes.Repeat([]byte{0x82}, 8192)
	for i := 0; i < 20; i++ {
		callbacks[0](data)
	}
	data[0] = 0
	batch, err = m.Events(context.Background(), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !batch.Lost || batch.LostEvents == 0 || batch.Oldest <= 1 || len(batch.Events) == 0 {
		t.Fatalf("history eviction not reported: %+v", batch)
	}
	for _, event := range batch.Events {
		if event.Type == "output" && event.Data[0] != 0x82 {
			t.Fatal("output buffer was not copied")
		}
	}
	batch.Events[0].Data[0] = 1
	again, _ := m.Events(context.Background(), batch.Oldest-1, 0)
	if again.Events[0].Data[0] != 0x82 {
		t.Fatal("consumer mutated retained output")
	}
	m.mu.Lock()
	if m.ringBytes > m.eventBudget {
		t.Error("ring exceeded memory budget")
	}
	m.mu.Unlock()
	reset, _ := m.Events(context.Background(), batch.Latest+100, 0)
	if !reset.Reset || reset.Next > reset.Latest {
		t.Fatalf("old application cursor was not reset: %+v", reset)
	}
}

func TestManagerPromptsNeverPublishReplies(t *testing.T) {
	m := testManager(t, func(ctx context.Context, _ ConnectRequest, cb transportCallbacks) (transport, error) {
		reply, err := cb.Prompt(ctx, Prompt{Kind: "keyboard-interactive", Title: "Authentication", Message: "Fixture challenge", Fields: []PromptField{{Label: "OTP", Echo: false}}, Actions: []string{"cancel", "submit"}})
		if err != nil {
			return nil, err
		}
		if len(reply.Answers) != 1 || reply.Answers[0] != "private-one-time-answer" {
			return nil, errors.New("wrong fixture answer")
		}
		return newFakeTerminal(false), nil
	})
	info, err := m.Connect(terminalRequest())
	if err != nil {
		t.Fatal(err)
	}
	pending := awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.Prompt != nil })
	if err := m.Reply(info.ID, PromptReply{PromptID: "stale", Action: "submit", Answers: []string{"private-one-time-answer"}}); err == nil {
		t.Fatal("accepted stale prompt")
	}
	if err := m.Reply(info.ID, PromptReply{PromptID: pending.Prompt.ID, Action: "submit", Answers: []string{"private-one-time-answer"}}); err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	batch, _ := m.Events(context.Background(), 0, 0)
	raw, _ := json.Marshal(batch)
	for _, secret := range []string{"fixture-secret", "private-one-time-answer"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("secret %s appeared in terminal protocol", secret)
		}
	}
}

func TestManagerDisconnectUnblocksPendingPrompt(t *testing.T) {
	returned := make(chan error, 1)
	m := testManager(t, func(ctx context.Context, _ ConnectRequest, cb transportCallbacks) (transport, error) {
		_, err := cb.Prompt(ctx, Prompt{Kind: "key-passphrase", Title: "Key", Fields: []PromptField{{Label: "Passphrase"}}, Actions: []string{"cancel", "submit"}})
		returned <- err
		return nil, err
	})
	info, err := m.Connect(terminalRequest())
	if err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.Prompt != nil })
	if err := m.Disconnect(info.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-returned:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("pending prompt blocked disconnect")
	}
}

func TestManagerLongPollCancellation(t *testing.T) {
	m := testManager(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := m.Events(ctx, 0, 20*time.Second); done <- err }()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("long poll did not cancel")
	}
	if _, err := m.Events(context.Background(), 0, 26*time.Second); err == nil {
		t.Fatal("accepted unbounded long poll")
	}
}

func TestTerminalRequestDropsIrrelevantSecrets(t *testing.T) {
	req := terminalRequest()
	req.Type = "telnet"
	req.PrivateKey = "private-key"
	req.Passphrase = "passphrase"
	got, err := normalizeRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if got.Password != "" || got.PrivateKey != "" || got.Passphrase != "" || got.Username != "" {
		t.Fatal("Telnet retained unused SSH credentials")
	}
	req = terminalRequest()
	req.PrivateKey = strings.Repeat("x", 256*1024+1)
	if _, err := normalizeRequest(req); err == nil {
		t.Fatal("accepted oversized private key")
	}
}
