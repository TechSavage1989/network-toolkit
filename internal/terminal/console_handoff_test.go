package terminal

import (
	"context"
	"io"
	"testing"
	"time"
)

type consoleHandoffFixture struct {
	*io.PipeReader
	*io.PipeWriter
	resizes chan [2]int
}

func (s *consoleHandoffFixture) Resize(cols, rows int) error {
	s.resizes <- [2]int{cols, rows}
	return nil
}

func (s *consoleHandoffFixture) Close() error {
	_ = s.PipeReader.Close()
	return s.PipeWriter.Close()
}

func TestAdoptConsoleTransportsOnlyExistingConsole(t *testing.T) {
	m := newManager(context.Background(), nil, nil)
	t.Cleanup(func() { _ = m.Close() })
	if _, err := m.Connect(ConnectRequest{Type: "console"}); err == nil {
		t.Fatal("regular connection accepted a Windows handoff type")
	}
	output, emit := io.Pipe()
	received, input := io.Pipe()
	t.Cleanup(func() { _ = emit.Close(); _ = received.Close() })
	stream := &consoleHandoffFixture{output, input, make(chan [2]int, 2)}
	info, err := m.AdoptConsole(stream)
	if err != nil || info.Type != "console" || info.Label != "Windows Console" {
		t.Fatalf("adopt: %+v, %v", info, err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	if _, err := m.Reconnect(context.Background(), info.ID); err == nil {
		t.Fatal("reconnect accepted for an externally started console")
	}
	if _, err := emit.Write([]byte("fixture prompt")); err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.BytesReceived == 14 })
	if err := m.Write(info.ID, []byte("echo fixture\r")); err != nil {
		t.Fatal(err)
	}
	inputSeen := make(chan string, 1)
	go func() {
		buf := make([]byte, 13)
		n, _ := io.ReadFull(received, buf)
		inputSeen <- string(buf[:n])
	}()
	select {
	case data := <-inputSeen:
		if data != "echo fixture\r" {
			t.Fatalf("input = %q", data)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("console never received input")
	}
	if err := m.Resize(info.ID, 97, 31); err != nil {
		t.Fatal(err)
	}
	select {
	case dimensions := <-stream.resizes:
		if dimensions != [2]int{97, 31} {
			t.Fatalf("resize = %v", dimensions)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("console never received resize")
	}
	_ = emit.Close()
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "closed" })
}
