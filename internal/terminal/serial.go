package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

type serialDevice interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
	SetSignals(dtr, rts *bool) error
	SetBreak(bool) error
}

type serialTransport struct {
	device          serialDevice
	ctx             context.Context
	cancel          context.CancelFunc
	cb              transportCallbacks
	done            chan error
	closeOnce       sync.Once
	controlMu       sync.Mutex
	closed          bool
	breakActive     bool
	breakGeneration uint64
	breakTimer      *time.Timer
	errMu           sync.Mutex
	exitErr         error
}

// Opening a faulty device driver must not block the session manager. The
// bounded opening pool also prevents repeated cancellations from creating an
// unlimited number of threads in an unresponsive driver's CreateFile call.
var serialOpeningSlots = make(chan struct{}, 4)

func openSerial(ctx context.Context, req ConnectRequest, cb transportCallbacks) (transport, error) {
	req, err := normalizeSerialRequest(req)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	if timeout <= 0 || timeout > 30*time.Second {
		timeout = 10 * time.Second
	}
	opening, stopOpening := context.WithTimeout(ctx, timeout)
	defer stopOpening()
	select {
	case serialOpeningSlots <- struct{}{}:
	case <-opening.Done():
		return nil, opening.Err()
	}
	type result struct {
		device serialDevice
		err    error
	}
	ready := make(chan result)
	go func() {
		defer func() { <-serialOpeningSlots }()
		device, err := openSerialDevice(opening, req)
		select {
		case ready <- result{device, err}:
		case <-opening.Done():
			if device != nil {
				_ = device.Close()
			}
		}
	}()
	select {
	case opened := <-ready:
		if opened.err != nil {
			return nil, opened.err
		}
		if err := opening.Err(); err != nil {
			_ = opened.device.Close()
			return nil, err
		}
		return startSerial(ctx, opened.device, cb), nil
	case <-opening.Done():
		return nil, opening.Err()
	}
}

func startSerial(ctx context.Context, device serialDevice, cb transportCallbacks) *serialTransport {
	lifetime, cancel := context.WithCancel(ctx)
	s := &serialTransport{device: device, ctx: lifetime, cancel: cancel, cb: cb, done: make(chan error, 1)}
	go s.readLoop()
	go func() { <-lifetime.Done(); _ = s.Close() }()
	return s
}

func (s *serialTransport) Done() <-chan error { return s.done }

func (s *serialTransport) rememberError(err error) {
	if err == nil {
		return
	}
	s.errMu.Lock()
	if s.exitErr == nil {
		s.exitErr = err
	}
	s.errMu.Unlock()
}

func (s *serialTransport) readLoop() {
	defer func() {
		_ = s.Close()
		s.errMu.Lock()
		err := s.exitErr
		s.errMu.Unlock()
		s.done <- err
		close(s.done)
	}()
	buffer := make([]byte, 16*1024)
	for {
		n, err := s.device.Read(buffer)
		if n > 0 && s.cb.Output != nil {
			s.cb.Output(buffer[:n])
		}
		if err != nil {
			if s.ctx.Err() == nil && !errors.Is(err, io.EOF) {
				s.rememberError(fmt.Errorf("read serial port: %w", err))
			}
			return
		}
		if s.ctx.Err() != nil {
			return
		}
	}
}

func (s *serialTransport) Write(data []byte) (int, error) {
	if len(data) > maxInputChunk {
		return 0, errors.New("terminal input chunk is too large")
	}
	if err := s.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := s.device.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil && s.ctx.Err() == nil {
		s.rememberError(fmt.Errorf("write serial port: %w", err))
		_ = s.Close()
	}
	return n, err
}

func (s *serialTransport) Resize(cols, rows int) error {
	if cols < 1 || rows < 1 || cols > 65535 || rows > 65535 {
		return errors.New("invalid terminal dimensions")
	}
	// A serial byte stream has no standard remote window-size negotiation.
	return s.ctx.Err()
}

func (s *serialTransport) SetSignals(dtr, rts *bool) error {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	if s.closed {
		return errors.New("serial session is closed")
	}
	return s.device.SetSignals(dtr, rts)
}

func (s *serialTransport) SetBreak(enabled bool) error {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	if s.closed {
		return errors.New("serial session is closed")
	}
	if err := s.device.SetBreak(enabled); err != nil {
		return err
	}
	s.breakActive = enabled
	s.breakGeneration++
	generation := s.breakGeneration
	if s.breakTimer != nil {
		s.breakTimer.Stop()
		s.breakTimer = nil
	}
	if enabled {
		// The UI normally clears a short break sooner. This failsafe also clears
		// it when a request is lost or the renderer is closed unexpectedly.
		s.breakTimer = time.AfterFunc(time.Second, func() {
			s.controlMu.Lock()
			if s.closed || s.breakGeneration != generation {
				s.controlMu.Unlock()
				return
			}
			err := s.device.SetBreak(false)
			if err == nil {
				s.breakActive = false
			}
			s.breakTimer = nil
			s.controlMu.Unlock()
			if err != nil {
				s.rememberError(fmt.Errorf("clear serial break: %w", err))
				_ = s.Close()
			}
		})
	}
	return nil
}

func (s *serialTransport) Close() error {
	var closeErr error
	s.closeOnce.Do(func() {
		s.cancel()
		s.controlMu.Lock()
		s.closed = true
		s.breakGeneration++
		if s.breakTimer != nil {
			s.breakTimer.Stop()
			s.breakTimer = nil
		}
		if s.breakActive {
			_ = s.device.SetBreak(false)
			s.breakActive = false
		}
		s.controlMu.Unlock()
		closeErr = s.device.Close()
	})
	return closeErr
}
