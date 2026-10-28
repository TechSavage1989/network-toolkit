//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"

	"networktoolkit/internal/capture"
)

// Test seams stay inside the native boundary; production uses ordinary Windows
// adapter reads and the existing fixed-role, parent-checked elevation launcher.
var packetCaptureReadAdapters = packetReadSystemAdapters
var packetCaptureLaunch = func(id string, parent uint32) (syscall.Handle, error) {
	return localLaunchElevatedMode(id, parent, true)
}

func packetCaptureInterfaces(ctx context.Context) ([]capture.Interface, error) {
	adapters, err := packetCaptureReadAdapters(ctx)
	if err != nil {
		return nil, err
	}
	return packetOrdinaryInterfaces(adapters), nil
}
func packetCaptureRun(ctx context.Context, o capture.Options, stop <-chan struct{}, emit func(packetCaptureMessage) error) error {
	return packetCaptureRoundTrip(ctx, packetCaptureRequest{Version: 1, Operation: "capture", Options: o}, stop, emit)
}
func validatePacketCaptureRequest(request *packetCaptureRequest) error {
	// Enumeration is deliberately not a privileged helper operation anymore.
	if request.Version != 1 || request.Operation != "capture" {
		return errors.New("Invalid packet capture helper request.")
	}
	if err := request.Options.Normalize(); err != nil {
		return err
	}
	if !packetValidInterfaceSelection(request.Options.Interface) {
		return errors.New("Choose a valid capture adapter, then explicitly start capture.")
	}
	return nil
}
func packetCaptureRoundTrip(ctx context.Context, request packetCaptureRequest, stop <-chan struct{}, emit func(packetCaptureMessage) error) error {
	if e := validatePacketCaptureRequest(&request); e != nil {
		return e
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-stop:
		return nil
	default:
	}
	id := randomHex(16)
	pipe, e := localCreatePipe(id)
	if e != nil {
		return e
	}
	var process syscall.Handle
	defer func() {
		_ = syscall.CloseHandle(pipe)
		if process != 0 {
			_, _ = syscall.WaitForSingleObject(process, 2000)
			_ = syscall.CloseHandle(process)
		}
	}()
	launchCtx, cancelLaunch := context.WithCancel(ctx)
	defer cancelLaunch()
	type launched struct {
		process syscall.Handle
		e       error
	}
	result := make(chan launched)
	go func() {
		h, e := packetCaptureLaunch(id, uint32(os.Getpid()))
		select {
		case result <- launched{h, e}:
		case <-launchCtx.Done():
			if h != 0 {
				_ = syscall.CloseHandle(h)
			}
		}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-stop:
		return nil
	case launch := <-result:
		process, e = launch.process, launch.e
	}
	if e != nil {
		if errors.Is(e, syscall.Errno(1223)) {
			return errors.New("The Windows administrator prompt was cancelled. No capture was started.")
		}
		return e
	}
	connectCtx, cancelConnect := context.WithTimeout(ctx, 20*time.Second)
	defer cancelConnect()
	if e = localConnectPipe(connectCtx, pipe); e != nil {
		return e
	}
	pid, _, _ := localKernel.NewProc("GetProcessId").Call(uintptr(process))
	peer, e := localPipePID(pipe, false)
	if e != nil || pid == 0 || peer != uint32(pid) {
		return errors.New("Unexpected packet capture helper process.")
	}
	if e = localVerifyProcessImage(process); e != nil {
		return e
	}
	stream := &localPipe{handle: pipe, ctx: ctx}
	if e = writeLocalFrame(stream, request); e != nil {
		return e
	}
	finished := make(chan struct{})
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		select {
		case <-finished:
			return
		case <-ctx.Done():
			return
		case <-stop:
			_ = writeLocalFrame(stream, struct {
				Version   int    `json:"version"`
				Operation string `json:"operation"`
			}{1, "stop"})
		}
	}()
	defer func() { close(finished); <-writerDone }()
	for {
		var message packetCaptureMessage
		if e = readLocalFrame(stream, &message); e != nil {
			return e
		}
		if message.Version != 1 {
			return errors.New("Invalid packet capture response version.")
		}
		if message.Type == "error" {
			return errors.New(message.Error)
		}
		if message.Type == "done" {
			return nil
		}
		if len(message.Packets) > 128 || len(message.Note) > 4096 || len(message.Interfaces) > 2049 {
			return errors.New("The packet capture response exceeds its limits.")
		}
		if e = emit(message); e != nil {
			return e
		}
	}
}
func runPacketCaptureHelper(id string, parent uint32) error {
	if !validLocalHelperID(id) || parent == 0 || !localElevated() {
		return errors.New("Invalid packet capture helper invocation.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour+3*time.Minute)
	defer cancel()
	name, _ := syscall.UTF16PtrFromString(localPipePrefix + id)
	pipe, e := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_EXISTING, 0x40000000|0x00100000, 0)
	if e != nil {
		return e
	}
	defer syscall.CloseHandle(pipe)
	peer, e := localPipePID(pipe, true)
	if e != nil || peer != parent {
		return errors.New("Unexpected packet capture parent process.")
	}
	process, e := syscall.OpenProcess(0x1000|syscall.SYNCHRONIZE, false, parent)
	if e != nil {
		return e
	}
	defer syscall.CloseHandle(process)
	if e = localVerifyProcessImage(process); e != nil {
		return e
	}
	stream := &localPipe{handle: pipe, ctx: ctx}
	var request packetCaptureRequest
	if e = readLocalFrame(stream, &request); e != nil {
		return e
	}
	var writeMu sync.Mutex
	emit := func(message packetCaptureMessage) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		message.Version = 1
		return writeLocalFrame(stream, message)
	}
	if e = validatePacketCaptureRequest(&request); e != nil {
		_ = emit(packetCaptureMessage{Type: "error", Error: e.Error()})
		return nil
	}
	if status, _ := syscall.WaitForSingleObject(process, 0); status != syscall.WAIT_TIMEOUT {
		return errors.New("The requesting application closed.")
	}
	workCtx, stop := context.WithCancel(ctx)
	defer stop()
	monitorCtx, endMonitor := context.WithCancel(ctx)
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		var control struct {
			Version   int    `json:"version"`
			Operation string `json:"operation"`
		}
		_ = readLocalFrame(&localPipe{handle: pipe, ctx: monitorCtx}, &control)
		stop()
	}()
	defer func() { endMonitor(); <-monitorDone }()
	e = packetCapturePerform(workCtx, request.Options, emit)
	runtime.KeepAlive(request)
	if e != nil {
		_ = emit(packetCaptureMessage{Type: "error", Error: fmt.Sprint(e)})
		return nil
	}
	return emit(packetCaptureMessage{Type: "done"})
}
