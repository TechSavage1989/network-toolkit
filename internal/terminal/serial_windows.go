//go:build windows

package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

var (
	serialKernel32            = syscall.NewLazyDLL("kernel32.dll") // Windows KnownDLL.
	serialGetCommState        = serialKernel32.NewProc("GetCommState")
	serialSetCommState        = serialKernel32.NewProc("SetCommState")
	serialSetupComm           = serialKernel32.NewProc("SetupComm")
	serialSetCommTimeouts     = serialKernel32.NewProc("SetCommTimeouts")
	serialCreateEvent         = serialKernel32.NewProc("CreateEventW")
	serialGetOverlappedResult = serialKernel32.NewProc("GetOverlappedResult")
	serialEscapeCommFunction  = serialKernel32.NewProc("EscapeCommFunction")
	serialSetCommBreak        = serialKernel32.NewProc("SetCommBreak")
	serialClearCommBreak      = serialKernel32.NewProc("ClearCommBreak")
)

type serialCommTimeouts struct {
	ReadInterval    uint32
	ReadMultiplier  uint32
	ReadConstant    uint32
	WriteMultiplier uint32
	WriteConstant   uint32
}

type windowsSerialDevice struct {
	handle     syscall.Handle
	flow       string
	writeChunk int
	mu         sync.Mutex
	closed     bool
	operations sync.WaitGroup
	writeMu    sync.Mutex
	closeOnce  sync.Once
}

func serialWindowsError(action string, err error) error {
	if err == nil || err == syscall.Errno(0) {
		return errors.New(action + " failed")
	}
	return fmt.Errorf("%s: %w", action, err)
}

func openSerialDevice(ctx context.Context, req ConnectRequest) (serialDevice, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	name, err := syscall.UTF16PtrFromString("\\\\.\\" + req.SerialPort)
	if err != nil {
		return nil, err
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s (the port must not be in use by another application): %w", req.SerialPort, err)
	}
	owned := true
	defer func() {
		if owned {
			_ = syscall.CloseHandle(handle)
		}
	}()
	current := serialDCB{Length: 28}
	if ok, _, err := serialGetCommState.Call(uintptr(handle), uintptr(unsafe.Pointer(&current))); ok == 0 {
		return nil, serialWindowsError("read serial configuration", err)
	}
	settings := makeSerialDCB(req)
	if ok, _, err := serialSetCommState.Call(uintptr(handle), uintptr(unsafe.Pointer(&settings))); ok == 0 {
		return nil, serialWindowsError("apply serial configuration (the device may not support these settings)", err)
	}
	// SetupComm recommends queue sizes; a driver may manage its own queues.
	serialSetupComm.Call(uintptr(handle), 64*1024, 64*1024)
	timeouts := serialCommTimeouts{ReadInterval: ^uint32(0), ReadMultiplier: ^uint32(0), ReadConstant: 250,
		WriteMultiplier: uint32((12000 + req.BaudRate - 1) / req.BaudRate), WriteConstant: 2000}
	if ok, _, err := serialSetCommTimeouts.Call(uintptr(handle), uintptr(unsafe.Pointer(&timeouts))); ok == 0 {
		return nil, serialWindowsError("set serial timeouts", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	chunk := req.BaudRate / 12
	if chunk < 8 {
		chunk = 8
	}
	if chunk > 1024 {
		chunk = 1024
	}
	device := &windowsSerialDevice{handle: handle, flow: req.FlowControl, writeChunk: chunk}
	owned = false
	return device, nil
}

// io owns a separate manual-reset event, OVERLAPPED and buffer reference for
// each outstanding operation. Close requests CancelIoEx while holding mu,
// preventing a new ReadFile/WriteFile from starting after cancellation. It
// waits for the actual completions before releasing the port or any buffers.
func (d *windowsSerialDevice) io(data []byte, write bool) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	event, _, err := serialCreateEvent.Call(0, 1, 0, 0)
	if event == 0 {
		return 0, serialWindowsError("create serial I/O event", err)
	}
	defer syscall.CloseHandle(syscall.Handle(event))
	overlapped := &syscall.Overlapped{HEvent: syscall.Handle(event)}
	var transferred uint32
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	d.operations.Add(1)
	if write {
		err = syscall.WriteFile(d.handle, data, &transferred, overlapped)
	} else {
		err = syscall.ReadFile(d.handle, data, &transferred, overlapped)
	}
	d.mu.Unlock()
	defer d.operations.Done()
	if err == nil || errors.Is(err, syscall.ERROR_IO_PENDING) {
		// Even immediate completion uses the final result, never an intermediate
		// byte count from the initiating asynchronous call.
		ok, _, resultErr := serialGetOverlappedResult.Call(uintptr(d.handle), uintptr(unsafe.Pointer(overlapped)), uintptr(unsafe.Pointer(&transferred)), 1)
		if ok == 0 {
			err = serialWindowsError("complete serial I/O", resultErr)
		} else {
			err = nil
		}
	}
	runtime.KeepAlive(overlapped)
	runtime.KeepAlive(data)
	if transferred > uint32(len(data)) {
		return 0, errors.New("serial driver returned an invalid byte count")
	}
	return int(transferred), err
}

func (d *windowsSerialDevice) Read(data []byte) (int, error) { return d.io(data, false) }

func (d *windowsSerialDevice) Write(data []byte) (int, error) {
	d.writeMu.Lock()
	defer d.writeMu.Unlock()
	written := 0
	for written < len(data) {
		end := written + d.writeChunk
		if end > len(data) {
			end = len(data)
		}
		n, err := d.io(data[written:end], true)
		written += n
		if err != nil {
			return written, err
		}
		if n == 0 {
			return written, errors.New("serial write timed out; check flow control and the device connection")
		}
	}
	return written, nil
}

func (d *windowsSerialDevice) SetSignals(dtr, rts *bool) error {
	if rts != nil && d.flow == "rtscts" {
		return errors.New("RTS is managed by hardware flow control")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return io.ErrClosedPipe
	}
	for _, setting := range []struct {
		value   *bool
		on, off uintptr
	}{{dtr, 5, 6}, {rts, 3, 4}} { // SET/CLR DTR, SET/CLR RTS.
		if setting.value == nil {
			continue
		}
		function := setting.off
		if *setting.value {
			function = setting.on
		}
		if ok, _, err := serialEscapeCommFunction.Call(uintptr(d.handle), function); ok == 0 {
			return serialWindowsError("change serial modem line", err)
		}
	}
	return nil
}

func (d *windowsSerialDevice) SetBreak(enabled bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return io.ErrClosedPipe
	}
	function := serialClearCommBreak
	if enabled {
		function = serialSetCommBreak
	}
	if ok, _, err := function.Call(uintptr(d.handle)); ok == 0 {
		return serialWindowsError("change serial break", err)
	}
	return nil
}

func (d *windowsSerialDevice) Close() error {
	var closeErr error
	d.closeOnce.Do(func() {
		d.mu.Lock()
		d.closed = true
		serialClearCommBreak.Call(uintptr(d.handle))
		_ = syscall.CancelIoEx(d.handle, nil)
		d.mu.Unlock()
		// CancelIoEx is a cancellation request, not a completion notification.
		// Keep all OVERLAPPED storage alive until GetOverlappedResult returns.
		d.operations.Wait()
		closeErr = syscall.CloseHandle(d.handle)
	})
	return closeErr
}

var _ [28 - unsafe.Sizeof(serialDCB{})]byte
var _ [unsafe.Sizeof(serialDCB{}) - 28]byte
var _ [20 - unsafe.Sizeof(serialCommTimeouts{})]byte
var _ [unsafe.Sizeof(serialCommTimeouts{}) - 20]byte
