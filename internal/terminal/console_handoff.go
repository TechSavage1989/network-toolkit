package terminal

import (
	"context"
	"io"
)

// ConsoleHandoff is a console already running under Windows. Its native
// owner supplies an authenticated stream. Close must unblock Read and Write.
type ConsoleHandoff interface {
	io.ReadWriteCloser
	Resize(cols, rows int) error
}

type consoleTransport struct {
	ConsoleHandoff
	done chan error
}

func (t *consoleTransport) Done() <-chan error { return t.done }

// AdoptConsole admits an already-started console without starting or retaining
// a shell command. Ownership of stream transfers only when registration succeeds.
func (m *Manager) AdoptConsole(stream ConsoleHandoff) (SessionInfo, error) {
	if stream == nil {
		return SessionInfo{}, ErrNotConnected
	}
	return m.connectWithOpener(ConnectRequest{Type: "console"}, func(_ context.Context, _ ConnectRequest, cb transportCallbacks) (transport, error) {
		t := &consoleTransport{ConsoleHandoff: stream, done: make(chan error, 1)}
		go func() {
			defer close(t.done)
			buf := make([]byte, 16<<10)
			for {
				n, err := stream.Read(buf)
				if n > 0 {
					cb.Output(buf[:n])
				}
				if err != nil {
					t.done <- err
					return
				}
			}
		}()
		return t, nil
	})
}
