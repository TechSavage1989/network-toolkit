//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"networktoolkit/internal/terminal"
	"os"
	"sync"
	"syscall"
	"time"
)

var terminalShellLaunch = func(id string, parent uint32) (syscall.Handle, error) {
	return localLaunchElevatedRole(id, parent, "terminal-shell")
}

func configureLocalShellElevation(m *terminal.Manager) {
	if m != nil {
		m.SetElevatedLocalShellOpener(openElevatedTerminalShell)
	}
}

type elevatedTerminalShell struct {
	ctx     context.Context
	cancel  context.CancelFunc
	stream  *localPipe
	process syscall.Handle
	state   terminal.LocalShellState
	mu      sync.Mutex
	done    chan error
}

func (t *elevatedTerminalShell) LocalShellState() terminal.LocalShellState { return t.state }
func (t *elevatedTerminalShell) Done() <-chan error                        { return t.done }
func (t *elevatedTerminalShell) Close() error                              { t.cancel(); return nil }
func (t *elevatedTerminalShell) send(f terminalShellFrame) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.ctx.Err(); err != nil {
		return err
	}
	return writeTerminalShellFrame(t.stream, f)
}
func (t *elevatedTerminalShell) Write(data []byte) (int, error) {
	total := 0
	for len(data) > 0 {
		n := len(data)
		if n > 16<<10 {
			n = 16 << 10
		}
		if err := t.send(terminalShellFrame{Version: 1, Kind: "input", Data: data[:n]}); err != nil {
			return total, err
		}
		total += n
		data = data[n:]
	}
	return total, nil
}
func (t *elevatedTerminalShell) Resize(cols, rows int) error {
	f := terminalShellFrame{Version: 1, Kind: "resize", Cols: cols, Rows: rows}
	if err := validateTerminalShellControl(f); err != nil {
		return err
	}
	return t.send(f)
}
func openElevatedTerminalShell(parent context.Context, o terminal.LocalShellOptions, output func([]byte)) (_ terminal.LocalShellTransport, err error) {
	if err = parent.Err(); err != nil {
		return nil, err
	}
	if !terminal.LocalShellAvailable() {
		return nil, errors.New("local administrator shells require ConPTY support")
	}
	if o.WorkingDirectory == "" {
		o.WorkingDirectory, err = os.UserHomeDir()
		if err != nil {
			return nil, err
		}
	}
	o.WorkingDirectory, err = terminal.ValidateLocalWorkingDirectory(o.WorkingDirectory)
	if err != nil {
		return nil, err
	}
	request := terminalShellFrame{Version: 1, Kind: "start", Options: &o}
	if err = validateTerminalShellStart(request); err != nil {
		return nil, err
	}
	id := randomHex(16)
	pipe, err := localCreatePipe(id)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	var process syscall.Handle
	success := false
	defer func() {
		if !success {
			cancel()
			syscall.CloseHandle(pipe)
			if process != 0 {
				syscall.CloseHandle(process)
			}
		}
	}()
	type launched struct {
		h   syscall.Handle
		err error
	}
	result := make(chan launched)
	go func() {
		h, e := terminalShellLaunch(id, uint32(os.Getpid()))
		select {
		case result <- launched{h, e}:
		case <-ctx.Done():
			if h != 0 {
				syscall.CloseHandle(h)
			}
		}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case launch := <-result:
		process, err = launch.h, launch.err
	}
	if err != nil {
		if errors.Is(err, syscall.Errno(1223)) {
			return nil, errors.New("Windows administrator approval was cancelled; no shell was started")
		}
		return nil, err
	}
	startup, startupCancel := context.WithTimeout(ctx, 20*time.Second)
	defer startupCancel()
	if err = localConnectPipe(startup, pipe); err != nil {
		return nil, err
	}
	pid, _, _ := localKernel.NewProc("GetProcessId").Call(uintptr(process))
	peer, e := localPipePID(pipe, false)
	if e != nil || !terminalShellPeerMatches(uint32(pid), peer) {
		return nil, errors.New("unexpected administrator terminal helper")
	}
	if err = localVerifyProcessImage(process); err != nil {
		return nil, err
	}
	stream := &localPipe{handle: pipe, ctx: startup}
	if err = writeTerminalShellFrame(stream, request); err != nil {
		return nil, err
	}
	ready, err := readTerminalShellFrame(stream)
	if err != nil {
		return nil, err
	}
	if ready.Version != 1 {
		return nil, errors.New("invalid administrator terminal response")
	}
	if ready.Kind == "error" && len(ready.Error) <= 4096 {
		return nil, errors.New(ready.Error)
	}
	actual, err := validateTerminalShellReady(ready, o.WorkingDirectory)
	if err != nil {
		return nil, err
	}
	t := &elevatedTerminalShell{ctx: ctx, cancel: cancel, stream: &localPipe{handle: pipe, ctx: ctx}, process: process, state: actual, done: make(chan error, 1)}
	success = true
	go func() {
		var result error
		defer func() {
			cancel()
			t.mu.Lock()
			syscall.CloseHandle(pipe)
			t.mu.Unlock()
			_, _ = syscall.WaitForSingleObject(process, 5000)
			syscall.CloseHandle(process)
			t.done <- result
			close(t.done)
		}()
		for {
			message, e := readTerminalShellFrame(t.stream)
			if e != nil {
				if ctx.Err() == nil {
					result = e
				}
				return
			}
			if message.Version != 1 || message.Options != nil || message.State != nil || message.Cols != 0 || message.Rows != 0 {
				result = errors.New("invalid administrator terminal output")
				return
			}
			switch message.Kind {
			case "output":
				if len(message.Data) == 0 || len(message.Data) > 16<<10 || message.Error != "" {
					result = errors.New("invalid administrator terminal output size")
					return
				}
				if output != nil {
					output(message.Data)
				}
				clear(message.Data)
			case "done":
				if len(message.Data) != 0 || len(message.Error) > 4096 {
					result = errors.New("invalid administrator terminal exit")
				} else if message.Error != "" {
					result = errors.New(message.Error)
				}
				return
			default:
				result = errors.New("unexpected administrator terminal message")
				return
			}
		}
	}()
	return t, nil
}

func runTerminalShellHelper(id string, parent uint32) error {
	if !validLocalHelperID(id) || parent == 0 || !localElevated() {
		return errors.New("invalid administrator terminal helper invocation")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	name, _ := syscall.UTF16PtrFromString(localPipePrefix + id)
	pipe, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_EXISTING, 0x40000000|0x00100000, 0)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(pipe)
	peer, err := localPipePID(pipe, true)
	if err != nil || !terminalShellPeerMatches(parent, peer) {
		return errors.New("unexpected administrator terminal parent")
	}
	process, err := syscall.OpenProcess(0x1000|syscall.SYNCHRONIZE, false, parent)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(process)
	if err = localVerifyProcessImage(process); err != nil {
		return err
	}
	if status, _ := syscall.WaitForSingleObject(process, 0); status != syscall.WAIT_TIMEOUT {
		return errors.New("requesting application closed")
	}
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		for {
			status, e := syscall.WaitForSingleObject(process, 100)
			if e != nil || status != syscall.WAIT_TIMEOUT {
				cancel()
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()
	defer func() { cancel(); <-monitorDone }()
	startup, endStartup := context.WithTimeout(ctx, 20*time.Second)
	defer endStartup()
	request, err := readTerminalShellFrame(&localPipe{handle: pipe, ctx: startup})
	if err != nil {
		return err
	}
	if err = validateTerminalShellStart(request); err != nil {
		return err
	}
	endStartup()
	stream := &localPipe{handle: pipe, ctx: ctx}
	var writeMu sync.Mutex
	emit := func(f terminalShellFrame) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return writeTerminalShellFrame(stream, f)
	}
	ready := make(chan struct{})
	conn, err := terminal.OpenLocalShellProcess(ctx, *request.Options, func(data []byte) {
		select {
		case <-ready:
			if e := emit(terminalShellFrame{Version: 1, Kind: "output", Data: data}); e != nil {
				cancel()
			}
		case <-ctx.Done():
		}
	})
	if err != nil {
		_ = emit(terminalShellFrame{Version: 1, Kind: "error", Error: boundedTerminalShellError(err)})
		return nil
	}
	defer conn.Close()
	actual := conn.LocalShellState()
	if !actual.Elevated {
		_ = emit(terminalShellFrame{Version: 1, Kind: "error", Error: "the new local shell is not elevated"})
		return nil
	}
	if err = emit(terminalShellFrame{Version: 1, Kind: "ready", State: &actual}); err != nil {
		return err
	}
	close(ready)
	readDone := make(chan error, 1)
	readerFinished := make(chan struct{})
	go func() {
		defer close(readerFinished)
		for {
			f, e := readTerminalShellFrame(stream)
			if e != nil {
				readDone <- e
				return
			}
			if e = validateTerminalShellControl(f); e != nil {
				readDone <- e
				return
			}
			switch f.Kind {
			case "input":
				_, e = conn.Write(f.Data)
				clear(f.Data)
			case "resize":
				e = conn.Resize(f.Cols, f.Rows)
			case "close":
				readDone <- nil
				return
			}
			if e != nil {
				readDone <- e
				return
			}
		}
	}()
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case <-readDone:
		err = nil
	case e := <-conn.Done():
		_ = emit(terminalShellFrame{Version: 1, Kind: "done", Error: boundedTerminalShellError(e)})
		err = nil
	}
	cancel()
	conn.Close()
	<-readerFinished
	return err
}
func boundedTerminalShellError(err error) string {
	if err == nil || errors.Is(err, io.EOF) {
		return ""
	}
	value := fmt.Sprint(err)
	if len(value) > 4096 {
		return "local shell operation failed"
	}
	return value
}
