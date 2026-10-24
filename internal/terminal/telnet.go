package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type telnetTransport struct {
	conn      net.Conn
	ctx       context.Context
	cancel    context.CancelFunc
	cb        transportCallbacks
	mu        sync.Mutex // Protocol state, wire writes, and displayed bytes share one ordering.
	protocol  *telnetProtocol
	closeOnce sync.Once
	done      chan error
	errMu     sync.Mutex
	exitErr   error
}

func openTelnet(ctx context.Context, req ConnectRequest, cb transportCallbacks) (transport, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	host := strings.TrimSpace(req.Host)
	if host == "" {
		return nil, errors.New("a Telnet host is required")
	}
	port := req.Port
	if port == 0 {
		port = 23
	}
	if port < 1 || port > 65535 {
		return nil, errors.New("Telnet port must be between 1 and 65535")
	}
	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if timeout > 60*time.Second {
		timeout = 60 * time.Second
	}
	conn, err := (&net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("connect Telnet: %w", err)
	}
	return startTelnet(ctx, conn, req, cb)
}

// startTelnet also permits tests to use a loopback connection or net.Pipe.
func startTelnet(ctx context.Context, conn net.Conn, req ConnectRequest, cb transportCallbacks) (*telnetTransport, error) {
	lifetime, cancel := context.WithCancel(ctx)
	t := &telnetTransport{conn: conn, ctx: lifetime, cancel: cancel, cb: cb, protocol: newTelnetProtocol(req.Cols, req.Rows, req.Terminal), done: make(chan error, 1)}
	t.mu.Lock()
	initial := t.protocol.initial()
	go t.readLoop()
	go func() { <-lifetime.Done(); _ = t.Close() }()
	_, err := t.writeWireLocked(initial)
	t.mu.Unlock()
	if err != nil {
		t.stop(err)
		return nil, fmt.Errorf("start Telnet negotiation: %w", err)
	}
	return t, nil
}

func (t *telnetTransport) Done() <-chan error { return t.done }

func (t *telnetTransport) stop(err error) {
	if err != nil {
		t.errMu.Lock()
		if t.exitErr == nil {
			t.exitErr = err
		}
		t.errMu.Unlock()
	}
	_ = t.Close()
}

func (t *telnetTransport) Close() error {
	var err error
	t.closeOnce.Do(func() { t.cancel(); err = t.conn.Close() })
	return err
}

func (t *telnetTransport) readLoop() {
	var terminalErr error
	defer func() {
		t.errMu.Lock()
		if t.exitErr != nil {
			terminalErr = t.exitErr
		}
		t.errMu.Unlock()
		_ = t.Close()
		t.done <- terminalErr
		close(t.done)
	}()
	buffer := make([]byte, 16*1024)
	for {
		n, err := t.conn.Read(buffer)
		if n > 0 {
			t.mu.Lock()
			output, reply, parseErr := t.protocol.feed(buffer[:n])
			_, writeErr := t.writeWireLocked(reply)
			if len(output) > 0 && t.cb.Output != nil {
				t.cb.Output(output)
			}
			t.mu.Unlock()
			if parseErr != nil {
				terminalErr = parseErr
				return
			}
			if writeErr != nil {
				if t.ctx.Err() == nil {
					terminalErr = fmt.Errorf("Telnet negotiation: %w", writeErr)
				}
				return
			}
		}
		if err != nil {
			if t.ctx.Err() == nil && !errors.Is(err, io.EOF) {
				terminalErr = fmt.Errorf("read Telnet: %w", err)
			}
			return
		}
	}
}

func (t *telnetTransport) writeWireLocked(wire []byte) (int, error) {
	if len(wire) == 0 {
		return 0, nil
	}
	if err := t.ctx.Err(); err != nil {
		return 0, err
	}
	if err := t.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return 0, err
	}
	defer t.conn.SetWriteDeadline(time.Time{})
	written := 0
	for written < len(wire) {
		n, err := t.conn.Write(wire[written:])
		written += n
		if err != nil {
			return written, err
		}
		if n == 0 {
			return written, io.ErrShortWrite
		}
	}
	return written, nil
}

func (t *telnetTransport) Write(data []byte) (int, error) {
	if len(data) > maxInputChunk {
		return 0, errors.New("terminal input chunk is too large")
	}
	t.mu.Lock()
	wire, ends := telnetEncode(data, t.protocol.local[tnBinary] == qYes)
	localEcho := t.protocol.remote[tnEcho] != qYes
	written, err := t.writeWireLocked(wire)
	consumed := 0
	for _, end := range ends {
		if end > written {
			break
		}
		consumed++
	}
	if localEcho && consumed > 0 && t.cb.Output != nil {
		// Output only appends to the manager's event ring and never reenters
		// the transport. Keeping it ordered here prevents a quick response
		// from appearing before the local echo of its triggering command.
		t.cb.Output(telnetLocalEcho(data[:consumed]))
	}
	t.mu.Unlock()
	if err != nil && t.ctx.Err() == nil {
		t.stop(fmt.Errorf("write Telnet: %w", err))
	}
	return consumed, err
}

func (t *telnetTransport) Resize(cols, rows int) error {
	t.mu.Lock()
	wire, err := t.protocol.resize(cols, rows)
	if err != nil {
		t.mu.Unlock()
		return err
	}
	_, err = t.writeWireLocked(wire)
	t.mu.Unlock()
	if err != nil && t.ctx.Err() == nil {
		t.stop(fmt.Errorf("resize Telnet: %w", err))
	}
	return err
}
