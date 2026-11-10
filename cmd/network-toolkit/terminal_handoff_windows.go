//go:build windows && toolkit_terminal_handoff

package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"networktoolkit/internal/desktop"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const terminalHandoffTitle = "Network Toolkit Terminal - Windows Console"

type terminalHandoffHandles struct {
	read, write, signal, reference, ready syscall.Handle
}

func parseTerminalHandoffHandles(args []string) (terminalHandoffHandles, error) {
	if len(args) != 5 {
		return terminalHandoffHandles{}, errors.New("expected five inherited console handles")
	}
	handles := make([]syscall.Handle, len(args))
	for i, arg := range args {
		value, err := strconv.ParseUint(arg, 10, strconv.IntSize)
		h := syscall.Handle(value)
		if err != nil || h == 0 || h == syscall.InvalidHandle || arg != strconv.FormatUint(value, 10) {
			return terminalHandoffHandles{}, errors.New("invalid inherited console handle")
		}
		for _, previous := range handles[:i] {
			if h == previous {
				return terminalHandoffHandles{}, errors.New("duplicate inherited console handle")
			}
		}
		handles[i] = h
	}
	return terminalHandoffHandles{handles[0], handles[1], handles[2], handles[3], handles[4]}, nil
}

func (h terminalHandoffHandles) validate() error {
	for i, handle := range []syscall.Handle{h.read, h.write, h.signal, h.reference, h.ready} {
		var flags uint32
		ok, _, err := localKernel.NewProc("GetHandleInformation").Call(uintptr(handle), uintptr(unsafe.Pointer(&flags)))
		if ok == 0 {
			return fmt.Errorf("check inherited console handle: %w", err)
		}
		if flags&syscall.HANDLE_FLAG_INHERIT == 0 {
			return errors.New("console handle was not inherited")
		}
		if i < 3 {
			kind, err := syscall.GetFileType(handle)
			if err != nil || kind != syscall.FILE_TYPE_PIPE {
				return errors.New("console stream handle is not a pipe")
			}
		}
	}
	for _, handle := range []syscall.Handle{h.read, h.write, h.signal, h.reference, h.ready} {
		if err := syscall.SetHandleInformation(handle, syscall.HANDLE_FLAG_INHERIT, 0); err != nil {
			return fmt.Errorf("make console handle private: %w", err)
		}
	}
	return nil
}

type terminalHandoffStream struct {
	ctx               context.Context
	cancel            context.CancelFunc
	read, write       localPipe
	signal, reference syscall.Handle
	mu                sync.RWMutex
	referenceMu       sync.Mutex
}

func newTerminalHandoffStream(parent context.Context, h terminalHandoffHandles) *terminalHandoffStream {
	ctx, cancel := context.WithCancel(parent)
	return &terminalHandoffStream{ctx: ctx, cancel: cancel, read: localPipe{h.read, ctx}, write: localPipe{h.write, ctx}, signal: h.signal, reference: h.reference}
}

func (t *terminalHandoffStream) Read(data []byte) (int, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.read.Read(data)
}

func (t *terminalHandoffStream) Write(data []byte) (int, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.write.Write(data)
}

func (t *terminalHandoffStream) Resize(cols, rows int) error {
	if cols < 2 || cols > 500 || rows < 2 || rows > 300 {
		return errors.New("invalid Windows console dimensions")
	}
	packet := terminalHandoffResizePacket(cols, rows)
	t.mu.RLock()
	defer t.mu.RUnlock()
	ctx, cancel := context.WithTimeout(t.ctx, 2*time.Second)
	defer cancel()
	return writeTerminalHandoffSignal(ctx, t.signal, packet[:])
}

func terminalHandoffResizePacket(cols, rows int) [6]byte {
	// Microsoft ConPTY signal channel: PTY_SIGNAL_RESIZE_WINDOW, then COORD.
	var packet [6]byte
	binary.LittleEndian.PutUint16(packet[:], 8)
	binary.LittleEndian.PutUint16(packet[2:], uint16(cols))
	binary.LittleEndian.PutUint16(packet[4:], uint16(rows))
	return packet
}

func writeTerminalHandoffSignal(ctx context.Context, handle syscall.Handle, packet []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// The signal handle belongs to ConPTY and is synchronous. Pin the write
	// to one thread so a stopped console cannot keep shutdown waiting forever.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	thread, _, _ := localKernel.NewProc("GetCurrentThread").Call()
	process, err := syscall.GetCurrentProcess()
	if err != nil {
		return err
	}
	var ownedThread syscall.Handle
	if err := syscall.DuplicateHandle(process, syscall.Handle(thread), process, &ownedThread, 0, false, syscall.DUPLICATE_SAME_ACCESS); err != nil {
		return err
	}
	defer syscall.CloseHandle(ownedThread)
	finished := make(chan struct{})
	cancelled := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(cancelled)
		// Cancellation may arrive just before WriteFile starts. Retry only
		// until the synchronous write has entered the kernel or completed.
		for {
			select {
			case <-finished:
				return
			default:
			}
			ok, _, err := localKernel.NewProc("CancelSynchronousIo").Call(uintptr(ownedThread))
			if ok != 0 || !errors.Is(err, syscall.ERROR_NOT_FOUND) {
				return
			}
			time.Sleep(time.Millisecond)
		}
	})
	var written uint32
	err = syscall.WriteFile(handle, packet, &written, nil)
	close(finished)
	if !stop() {
		<-cancelled
	}
	if err == nil && written != uint32(len(packet)) {
		err = io.ErrShortWrite
	}
	return err
}

func (t *terminalHandoffStream) releaseReference() {
	t.referenceMu.Lock()
	defer t.referenceMu.Unlock()
	if t.reference != 0 {
		_ = syscall.CloseHandle(t.reference)
		t.reference = 0
	}
}

func (t *terminalHandoffStream) Close() error {
	t.cancel()
	t.releaseReference()
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, handle := range []*syscall.Handle{&t.read.handle, &t.write.handle, &t.signal} {
		if *handle != 0 {
			_ = syscall.CloseHandle(*handle)
			*handle = 0
		}
	}
	return nil
}

func runTerminalHandoffIfRequested(args []string) bool {
	if len(args) == 0 || args[0] != "--terminal-handoff" {
		return false
	}
	if err := runTerminalHandoff(args[1:]); err != nil {
		// The broker sees startup failure and lets Windows handle the console.
		_, _ = fmt.Fprintln(os.Stderr, "Windows console handoff:", err)
		os.Exit(1)
	}
	return true
}

func runTerminalHandoff(args []string) error {
	h, err := parseTerminalHandoffHandles(args)
	if err != nil {
		return err
	}
	if err := h.validate(); err != nil {
		return err
	}
	defer func() {
		if h.ready != 0 {
			_ = syscall.CloseHandle(h.ready)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := newTerminalHandoffStream(ctx, h)
	defer stream.Close()

	// A COM-launched window does not join a pre-existing workspace or load
	// its profiles, credentials, trust files or settings.
	dir, err := os.MkdirTemp("", "network-toolkit-console-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	a := newApplication(ctx, filepath.Join(dir, "Network_Toolkit.settings.json"))
	a.windowMode, a.uiHost = "terminal", "webview2"
	defer func() {
		a.stop()
		a.closeBrowserConnections()
		_ = a.terminals.Close()
		a.packetCaptures.close()
		a.transfers.Close()
	}()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()
	a.host = listener.Addr().String()
	if _, err := a.terminals.AdoptConsole(stream); err != nil {
		return err
	}
	server := &http.Server{Handler: a, BaseContext: func(net.Listener) context.Context { return a.ctx }, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	defer server.Close()
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.stop()
		}
	}()
	var readyErr error
	err = desktop.Run(a.ctx, desktop.Config{
		Title: terminalHandoffTitle, MinWidth: 640, MinHeight: 400,
		URL: "http://" + a.host + "/", SessionToken: a.token, DataDir: dir, Stop: a.stop,
		OnClosing: a.beginPrimaryClose,
		OnReady: func() {
			ok, _, err := localKernel.NewProc("SetEvent").Call(uintptr(h.ready))
			if ok == 0 {
				readyErr = err
				a.stop()
				return
			}
			_ = syscall.CloseHandle(h.ready)
			h.ready = 0
			stream.releaseReference()
		},
	})
	return errors.Join(err, readyErr)
}
