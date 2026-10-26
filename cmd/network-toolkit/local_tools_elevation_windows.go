//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const localPipePrefix = `\\.\pipe\NetworkToolkit.LocalConfig.`

type localPipe struct {
	handle syscall.Handle
	ctx    context.Context
}

func localEvent() (syscall.Handle, error) {
	h, _, err := localKernel.NewProc("CreateEventW").Call(0, 1, 0, 0)
	if h == 0 {
		return 0, err
	}
	return syscall.Handle(h), nil
}

func localWaitOverlapped(ctx context.Context, handle syscall.Handle, ov *syscall.Overlapped) (count uint32, resultErr error) {
	defer func() {
		if resultErr != nil {
			_ = syscall.CancelIoEx(handle, ov)
			// Every error path drains the pending request, including an unusual
			// event-wait failure, before its Go-owned storage is released.
			var transferred uint32
			localKernel.NewProc("GetOverlappedResult").Call(uintptr(handle), uintptr(unsafe.Pointer(ov)), uintptr(unsafe.Pointer(&transferred)), 1)
		}
	}()
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		status, err := syscall.WaitForSingleObject(ov.HEvent, 25)
		if err != nil {
			return 0, err
		}
		if status == syscall.WAIT_TIMEOUT {
			continue
		}
		if status != syscall.WAIT_OBJECT_0 {
			return 0, errors.New("The local configuration pipe wait failed.")
		}
		var transferred uint32
		ok, _, e := localKernel.NewProc("GetOverlappedResult").Call(uintptr(handle), uintptr(unsafe.Pointer(ov)), uintptr(unsafe.Pointer(&transferred)), 0)
		if ok == 0 {
			return 0, e
		}
		return transferred, nil
	}
}

func (p *localPipe) transfer(data []byte, write bool) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if err := p.ctx.Err(); err != nil {
		return 0, err
	}
	event, err := localEvent()
	if err != nil {
		return 0, err
	}
	defer syscall.CloseHandle(event)
	ov := syscall.Overlapped{HEvent: event}
	var done uint32
	if write {
		err = syscall.WriteFile(p.handle, data, &done, &ov)
	} else {
		err = syscall.ReadFile(p.handle, data, &done, &ov)
	}
	if errors.Is(err, syscall.ERROR_IO_PENDING) {
		done, err = localWaitOverlapped(p.ctx, p.handle, &ov)
	}
	runtime.KeepAlive(data)
	runtime.KeepAlive(&ov)
	if !write && (errors.Is(err, syscall.ERROR_BROKEN_PIPE) || errors.Is(err, syscall.Errno(232))) {
		return int(done), io.EOF
	}
	if err == nil && !write && done == 0 {
		return 0, io.EOF
	}
	return int(done), err
}
func (p *localPipe) Read(data []byte) (int, error)  { return p.transfer(data, false) }
func (p *localPipe) Write(data []byte) (int, error) { return p.transfer(data, true) }

func localCreatePipe(id string) (syscall.Handle, error) {
	if !validLocalHelperID(id) {
		return syscall.InvalidHandle, errors.New("Invalid local configuration pipe identifier.")
	}
	token, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return syscall.InvalidHandle, err
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return syscall.InvalidHandle, err
	}
	sid, err := user.User.Sid.String()
	if err != nil {
		return syscall.InvalidHandle, err
	}
	// Remote clients are rejected below. Only this user, administrators and
	// SYSTEM can open the local pipe, and both ends additionally verify PIDs.
	sddl, _ := syscall.UTF16PtrFromString("D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GA;;;" + sid + ")")
	var descriptor uintptr
	advapi := syscall.NewLazyDLL(filepath.Join(localSystemDirectory, "advapi32.dll"))
	ok, _, err := advapi.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW").Call(uintptr(unsafe.Pointer(sddl)), 1, uintptr(unsafe.Pointer(&descriptor)), 0)
	if ok == 0 {
		return syscall.InvalidHandle, err
	}
	defer localKernel.NewProc("LocalFree").Call(descriptor)
	sa := syscall.SecurityAttributes{Length: uint32(unsafe.Sizeof(syscall.SecurityAttributes{})), SecurityDescriptor: descriptor}
	name, _ := syscall.UTF16PtrFromString(localPipePrefix + id)
	// Duplex, overlapped, first instance only; byte mode; reject remote clients.
	h, _, err := localKernel.NewProc("CreateNamedPipeW").Call(uintptr(unsafe.Pointer(name)), 3|0x40000000|0x00080000, 8, 1, 65536, 65536, 0, uintptr(unsafe.Pointer(&sa)))
	runtime.KeepAlive(sddl)
	runtime.KeepAlive(name)
	runtime.KeepAlive(sa)
	if h == uintptr(syscall.InvalidHandle) {
		return syscall.InvalidHandle, err
	}
	return syscall.Handle(h), nil
}

func localConnectPipe(ctx context.Context, handle syscall.Handle) error {
	event, err := localEvent()
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(event)
	ov := syscall.Overlapped{HEvent: event}
	ok, _, e := localKernel.NewProc("ConnectNamedPipe").Call(uintptr(handle), uintptr(unsafe.Pointer(&ov)))
	if ok != 0 || errors.Is(e, syscall.Errno(535)) {
		return nil
	} // ERROR_PIPE_CONNECTED.
	if !errors.Is(e, syscall.ERROR_IO_PENDING) {
		return e
	}
	_, err = localWaitOverlapped(ctx, handle, &ov)
	runtime.KeepAlive(&ov)
	return err
}

func localPipePID(handle syscall.Handle, server bool) (uint32, error) {
	var pid uint32
	proc := "GetNamedPipeClientProcessId"
	if server {
		proc = "GetNamedPipeServerProcessId"
	}
	ok, _, err := localKernel.NewProc(proc).Call(uintptr(handle), uintptr(unsafe.Pointer(&pid)))
	if ok == 0 || pid == 0 {
		return 0, fmt.Errorf("Verify the local configuration pipe peer: %w", err)
	}
	return pid, nil
}

func localVerifyProcessImage(handle syscall.Handle) error {
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	ok, _, err := localKernel.NewProc("QueryFullProcessImageNameW").Call(uintptr(handle), 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)))
	if ok == 0 {
		return fmt.Errorf("Verify the local configuration helper executable: %w", err)
	}
	own, err := os.Executable()
	if err != nil {
		return err
	}
	other := syscall.UTF16ToString(buffer[:size])
	if !strings.EqualFold(filepath.Clean(own), filepath.Clean(other)) {
		return errors.New("The local configuration pipe peer is not this application.")
	}
	first, err := os.Stat(own)
	if err != nil {
		return err
	}
	second, err := os.Stat(other)
	if err != nil {
		return err
	}
	if !os.SameFile(first, second) {
		return errors.New("The local configuration helper executable changed.")
	}
	return nil
}

func localLaunchElevated(id string, parent uint32) (syscall.Handle, error) {
	return localLaunchElevatedMode(id, parent, false)
}

func localLaunchElevatedMode(id string, parent uint32, packetCapture bool) (syscall.Handle, error) {
	if !validLocalHelperID(id) || parent == 0 {
		return 0, errors.New("Invalid local configuration helper invocation.")
	}
	if localSystemDirectoryError != nil {
		return 0, localSystemDirectoryError
	}
	executable, err := updateCurrentExecutable()
	if err != nil {
		return 0, err
	}
	if !filepath.IsAbs(executable) {
		return 0, errors.New("The application executable path is not absolute.")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole := syscall.NewLazyDLL(filepath.Join(localSystemDirectory, "ole32.dll"))
	status, _, _ := ole.NewProc("CoInitializeEx").Call(0, 2|4)
	if int32(status) < 0 {
		return 0, errors.New("Windows could not initialize administrator approval.")
	}
	defer ole.NewProc("CoUninitialize").Call()
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(executable)
	flags := "--local-tools-helper " + id + " --local-tools-parent " + strconv.FormatUint(uint64(parent), 10)
	if packetCapture {
		flags = "--packet-capture-helper " + id + " --packet-capture-parent " + strconv.FormatUint(uint64(parent), 10)
	}
	parameters, _ := syscall.UTF16PtrFromString(flags)
	directory, _ := syscall.UTF16PtrFromString(filepath.Dir(executable))
	info := externalShellExecuteInfo{Mask: 0x40 | 0x100 | 0x400, Verb: verb, File: file, Parameters: parameters, Directory: directory, Show: 0}
	info.Size = uint32(unsafe.Sizeof(info))
	ok, _, callErr := syscall.NewLazyDLL(filepath.Join(localSystemDirectory, "shell32.dll")).NewProc("ShellExecuteExW").Call(uintptr(unsafe.Pointer(&info)))
	runtime.KeepAlive(info)
	if ok == 0 || info.Process == 0 {
		if info.Process != 0 {
			syscall.CloseHandle(info.Process)
		}
		if callErr == syscall.Errno(0) {
			return 0, errors.New("Windows did not start the local configuration helper.")
		}
		return 0, callErr
	}
	return info.Process, nil
}

func localElevatedApply(ctx context.Context, request localToolsRequest) (result localApplyResult) {
	sent := false
	fail := func(err error) localApplyResult {
		message, code := err.Error(), localErrorCode(err)
		if errors.Is(err, syscall.Errno(1223)) {
			message = "The Windows administrator prompt was cancelled. No elevated change was made."
			code = "cancelled"
		}
		if sent {
			message += " Review the refreshed configuration before retrying."
		}
		return localApplyResult{Error: message, ErrorCode: code, Partial: sent}
	}
	defer func() {
		// Never leave the UI displaying a guessed rollback after a transport
		// failure. These are reads only, with a separate bounded refresh window.
		refreshCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if request.Kind == "hosts" && result.Result.Hosts == nil {
			if s, e := readLocalHosts(refreshCtx); e == nil {
				result.Result.Hosts = &s
			}
		}
		if request.Kind == "ip" && result.Result.IP == nil {
			if s, e := readLocalIP(refreshCtx); e == nil {
				result.Result.IP = &s
			}
		}
		if result.Result.Hosts != nil {
			result.Result.Hosts.Elevated = localElevated()
		}
		if result.Result.IP != nil {
			result.Result.IP.Elevated = localElevated()
		}
	}()
	response, dispatched, err := localHelperRoundTrip(ctx, localHelperRequest{Version: localToolsProtocolVersion, Request: request})
	sent = dispatched
	if err != nil {
		return fail(err)
	}
	return *response.Result
}

// Both fixed-purpose mutation requests and one-profile key reveals use this
// authenticated, bounded channel. Wi-Fi results never enter operation history.
func localHelperRoundTrip(ctx context.Context, envelope localHelperRequest) (localHelperResponse, bool, error) {
	sent := false
	if err := validateLocalHelperRequest(envelope); err != nil {
		return localHelperResponse{}, sent, err
	}
	if err := ctx.Err(); err != nil {
		return localHelperResponse{}, sent, err
	}
	id := randomHex(16)
	pipe, err := localCreatePipe(id)
	if err != nil {
		return localHelperResponse{}, sent, err
	}
	var process syscall.Handle
	defer func() {
		// Closing the pipe cancels the helper's operation monitor. It drains a
		// cancelled netsh command before exit; no new step can start afterward.
		syscall.CloseHandle(pipe)
		if process != 0 {
			syscall.WaitForSingleObject(process, 2000)
			syscall.CloseHandle(process)
		}
	}()
	type launchResult struct {
		process syscall.Handle
		err     error
	}
	launched := make(chan launchResult)
	go func() {
		h, e := localLaunchElevated(id, uint32(os.Getpid()))
		select {
		case launched <- launchResult{h, e}:
		case <-ctx.Done():
			if h != 0 {
				syscall.CloseHandle(h)
			}
		}
	}()
	select {
	case <-ctx.Done():
		return localHelperResponse{}, sent, ctx.Err()
	case launch := <-launched:
		process, err = launch.process, launch.err
	}
	if err != nil {
		return localHelperResponse{}, sent, err
	}
	if err = ctx.Err(); err != nil {
		return localHelperResponse{}, sent, err
	}
	if err = localConnectPipe(ctx, pipe); err != nil {
		return localHelperResponse{}, sent, err
	}
	pid, _, e := localKernel.NewProc("GetProcessId").Call(uintptr(process))
	if pid == 0 {
		return localHelperResponse{}, sent, e
	}
	peer, err := localPipePID(pipe, false)
	if err != nil {
		return localHelperResponse{}, sent, err
	}
	if peer != uint32(pid) {
		return localHelperResponse{}, sent, errors.New("An unexpected process connected to the local configuration pipe.")
	}
	if err = localVerifyProcessImage(process); err != nil {
		return localHelperResponse{}, sent, err
	}
	if err = ctx.Err(); err != nil {
		return localHelperResponse{}, sent, err
	}
	stream := &localPipe{handle: pipe, ctx: ctx}
	// Count dispatch as potentially applied even if a later pipe write fails.
	sent = true
	if err = writeLocalFrame(stream, envelope); err != nil {
		return localHelperResponse{}, sent, err
	}
	var response localHelperResponse
	if err = readLocalFrame(stream, &response); err != nil {
		return localHelperResponse{}, sent, err
	}
	if response.Version != localToolsProtocolVersion || (envelope.WiFi == nil && (response.Result == nil || response.WiFi != nil)) || (envelope.WiFi != nil && (response.WiFi == nil || response.Result != nil)) {
		if response.WiFi != nil && response.WiFi.Reply != nil {
			response.WiFi.Reply.Secret = ""
		}
		return localHelperResponse{}, sent, errors.New("The local configuration helper returned an invalid response.")
	}
	return response, sent, nil
}

func runLocalToolsHelper(id string, parent uint32) error {
	if !validLocalHelperID(id) || parent == 0 {
		return errors.New("Invalid local configuration helper arguments.")
	}
	if localSystemDirectoryError != nil {
		return localSystemDirectoryError
	}
	if !localElevated() {
		return errors.New("The local configuration helper requires administrator approval.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	name, _ := syscall.UTF16PtrFromString(localPipePrefix + id)
	pipe, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_EXISTING, 0x40000000|0x00100000, 0) // OVERLAPPED | SECURITY_SQOS_PRESENT, anonymous impersonation.
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(pipe)
	pid, err := localPipePID(pipe, true)
	if err != nil {
		return err
	}
	if pid != parent {
		return errors.New("The local configuration pipe belongs to an unexpected process.")
	}
	process, err := syscall.OpenProcess(0x1000|syscall.SYNCHRONIZE, false, parent)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(process)
	if err = localVerifyProcessImage(process); err != nil {
		return err
	}
	stream := &localPipe{handle: pipe, ctx: ctx}
	var envelope localHelperRequest
	if err = readLocalFrame(stream, &envelope); err != nil {
		return err
	}
	if envelope.Version != localToolsProtocolVersion {
		return errors.New("Unsupported local configuration helper protocol.")
	}
	if err = validateLocalHelperRequest(envelope); err != nil {
		if envelope.WiFi != nil {
			result := localWiFiFailure("validation", err.Error(), false)
			return writeLocalFrame(stream, localHelperResponse{Version: localToolsProtocolVersion, WiFi: &result})
		}
		result := localApplyResult{Error: err.Error(), ErrorCode: "validation"}
		return writeLocalFrame(stream, localHelperResponse{Version: localToolsProtocolVersion, Result: &result})
	}
	if status, _ := syscall.WaitForSingleObject(process, 0); status != syscall.WAIT_TIMEOUT {
		return errors.New("The requesting application closed before the change started.")
	}
	applyCtx, stopApply := context.WithCancel(ctx)
	defer stopApply()
	monitorCtx, stopMonitor := context.WithCancel(ctx)
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		var one [1]byte
		_, _ = (&localPipe{handle: pipe, ctx: monitorCtx}).Read(one[:])
		stopApply()
	}()
	defer func() { stopMonitor(); <-monitorDone }()
	if envelope.WiFi != nil {
		result := localPerformWiFiHelper(applyCtx, *envelope.WiFi, process)
		defer func() {
			if result.Reply != nil {
				result.Reply.Secret = ""
			}
		}()
		if err := applyCtx.Err(); err != nil {
			return err
		}
		return writeLocalFrame(&localPipe{handle: pipe, ctx: applyCtx}, localHelperResponse{Version: localToolsProtocolVersion, WiFi: &result})
	}
	result := localPerformTools(applyCtx, envelope.Request)
	// The monitor remains active during response transmission. Unexpected
	// extra input or a disconnected parent prevents subsequent mutations.
	return writeLocalFrame(stream, localHelperResponse{Version: localToolsProtocolVersion, Result: &result})
}
