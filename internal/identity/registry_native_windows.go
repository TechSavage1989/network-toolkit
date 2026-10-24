//go:build windows

package identity

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

var registryWinLoad sync.Once
var registryWinLoadError error
var registryWinOpen, registryWinConnect, registryWinOpenRequest, registryWinSetOption, registryWinSetTimeouts, registryWinSend, registryWinReceive, registryWinQuery, registryWinRead, registryWinClose, registryWinSetCallback *syscall.LazyProc

var registryWinRequestSequence atomic.Uint64
var registryWinRequests sync.Map
var registryWinStatusCallback = syscall.NewCallback(func(_ uintptr, token uintptr, status uintptr, information *uint32, length uintptr) uintptr {
	if status == 0x00010000 && information != nil && length >= 4 {
		if state, ok := registryWinRequests.Load(token); ok {
			state.(*atomic.Uint32).Or(*information)
		}
	}
	return 0
})

func registryLoadWindowsHTTP() error {
	registryWinLoad.Do(func() {
		// Resolve the system directory with the OS API, never the working
		// directory or environment search path. No companion DLL is shipped.
		getDirectory := syscall.NewLazyDLL("kernel32.dll").NewProc("GetSystemDirectoryW")
		buffer := make([]uint16, 32768)
		length, _, err := getDirectory.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
		runtime.KeepAlive(buffer)
		if length == 0 || length >= uintptr(len(buffer)) {
			registryWinLoadError = registryWindowsError("GetSystemDirectoryW", err)
			return
		}
		dll := syscall.NewLazyDLL(filepath.Join(syscall.UTF16ToString(buffer[:length]), "winhttp.dll"))
		bindings := []struct {
			name        string
			destination **syscall.LazyProc
		}{
			{"WinHttpOpen", &registryWinOpen}, {"WinHttpConnect", &registryWinConnect},
			{"WinHttpOpenRequest", &registryWinOpenRequest}, {"WinHttpSetOption", &registryWinSetOption},
			{"WinHttpSetTimeouts", &registryWinSetTimeouts}, {"WinHttpSendRequest", &registryWinSend},
			{"WinHttpReceiveResponse", &registryWinReceive}, {"WinHttpQueryHeaders", &registryWinQuery},
			{"WinHttpReadData", &registryWinRead}, {"WinHttpCloseHandle", &registryWinClose},
			{"WinHttpSetStatusCallback", &registryWinSetCallback},
		}
		for _, binding := range bindings {
			procedure := dll.NewProc(binding.name)
			if err := procedure.Find(); err != nil {
				registryWinLoadError = err
				return
			}
			*binding.destination = procedure
		}
	})
	return registryWinLoadError
}

func registryPlatformFallback() func(context.Context, registrySource) ([]byte, error) {
	return func(ctx context.Context, source registrySource) ([]byte, error) {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.url, nil)
		if err != nil {
			return nil, err
		}
		proxyURL, err := http.ProxyFromEnvironment(request)
		if err != nil {
			return nil, err
		}
		proxy, err := registryNativeProxyConfig(proxyURL, os.Getenv("HTTPS_PROXY") != "" || os.Getenv("https_proxy") != "")
		if err != nil {
			return nil, err
		}
		return downloadRegistryNative(ctx, source, proxy, registryOpenWindows)
	}
}

type registryWindowsSession struct{ handle uintptr }
type registryWindowsRequest struct {
	handle, connection, token uintptr
	secureFlags               atomic.Uint32
}

func registryWindowsError(operation string, err error) error {
	if err == nil || err == syscall.Errno(0) {
		err = syscall.EINVAL
	}
	if code, ok := err.(syscall.Errno); ok {
		return fmt.Errorf("%s (Windows error %d): %w", operation, uint32(code), err)
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func registryOpenWindows(agent string, proxy registryNativeProxy) (registryNativeSession, error) {
	if err := registryLoadWindowsHTTP(); err != nil {
		return nil, err
	}
	agentUTF, err := syscall.UTF16PtrFromString(agent)
	if err != nil {
		return nil, err
	}
	var proxyUTF *uint16
	if proxy.server != "" {
		proxyUTF, err = syscall.UTF16PtrFromString(proxy.server)
		if err != nil {
			return nil, err
		}
	}
	handle, _, callErr := registryWinOpen.Call(uintptr(unsafe.Pointer(agentUTF)), uintptr(proxy.mode), uintptr(unsafe.Pointer(proxyUTF)), 0, 0)
	runtime.KeepAlive(agentUTF)
	runtime.KeepAlive(proxyUTF)
	if handle == 0 {
		return nil, registryWindowsError("WinHttpOpen", callErr)
	}
	return &registryWindowsSession{handle}, nil
}

func registryWindowsOption(handle uintptr, option, value uint32) error {
	ok, _, err := registryWinSetOption.Call(handle, uintptr(option), uintptr(unsafe.Pointer(&value)), 4)
	runtime.KeepAlive(&value)
	if ok == 0 {
		return registryWindowsError("WinHttpSetOption", err)
	}
	return nil
}

func (session *registryWindowsSession) SetOption(option, value uint32) error {
	return registryWindowsOption(session.handle, option, value)
}
func (session *registryWindowsSession) Close() {
	if session.handle != 0 {
		registryWinClose.Call(session.handle)
		session.handle = 0
	}
}

func (session *registryWindowsSession) OpenRequest(host, path string) (registryNativeRequest, error) {
	hostUTF, err := syscall.UTF16PtrFromString(host)
	if err != nil {
		return nil, err
	}
	connection, _, callErr := registryWinConnect.Call(session.handle, uintptr(unsafe.Pointer(hostUTF)), 443, 0)
	runtime.KeepAlive(hostUTF)
	if connection == 0 {
		return nil, registryWindowsError("WinHttpConnect", callErr)
	}
	getUTF, _ := syscall.UTF16PtrFromString("GET")
	pathUTF, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		registryWinClose.Call(connection)
		return nil, err
	}
	// WINHTTP_FLAG_SECURE. Redirects are disabled before any request is sent.
	handle, _, callErr := registryWinOpenRequest.Call(connection, uintptr(unsafe.Pointer(getUTF)), uintptr(unsafe.Pointer(pathUTF)), 0, 0, 0, 0x00800000)
	runtime.KeepAlive(getUTF)
	runtime.KeepAlive(pathUTF)
	if handle == 0 {
		registryWinClose.Call(connection)
		return nil, registryWindowsError("WinHttpOpenRequest", callErr)
	}
	request := &registryWindowsRequest{handle: handle, connection: connection, token: uintptr(registryWinRequestSequence.Add(1))}
	// Store only an opaque, unique integer in WinHTTP. No Go pointer survives
	// in native context memory, and a late callback cannot attach to a reused
	// handle. The shared callback and copied DWORD are safe across OS threads.
	registryWinRequests.Store(request.token, &request.secureFlags)
	previous, _, callbackErr := registryWinSetCallback.Call(handle, registryWinStatusCallback, 0x00010000, 0)
	if previous == ^uintptr(0) {
		request.Close()
		return nil, registryWindowsError("WinHttpSetStatusCallback", callbackErr)
	}
	return request, nil
}

func (request *registryWindowsRequest) failure(operation string, err error) error {
	if err == syscall.Errno(12175) {
		return &registryNativeSecureError{operation: operation, flags: request.secureFlags.Load(), cause: err}
	}
	return registryWindowsError(operation, err)
}

func (request *registryWindowsRequest) SetOption(option, value uint32) error {
	return registryWindowsOption(request.handle, option, value)
}
func (request *registryWindowsRequest) SetTimeouts(millis int) error {
	ok, _, err := registryWinSetTimeouts.Call(request.handle, uintptr(millis), uintptr(millis), uintptr(millis), uintptr(millis))
	if ok == 0 {
		return registryWindowsError("WinHttpSetTimeouts", err)
	}
	return nil
}
func (request *registryWindowsRequest) Send(headers string) error {
	value, err := syscall.UTF16FromString(headers)
	if err != nil {
		return err
	}
	ok, _, callErr := registryWinSend.Call(request.handle, uintptr(unsafe.Pointer(&value[0])), uintptr(len(value)-1), 0, 0, 0, request.token)
	runtime.KeepAlive(value)
	if ok == 0 {
		return request.failure("WinHttpSendRequest", callErr)
	}
	return nil
}
func (request *registryWindowsRequest) Receive() error {
	ok, _, err := registryWinReceive.Call(request.handle, 0)
	if ok == 0 {
		return request.failure("WinHttpReceiveResponse", err)
	}
	return nil
}
func (request *registryWindowsRequest) Status() (int, error) {
	var status uint32
	size := uint32(4)
	ok, _, err := registryWinQuery.Call(request.handle, 19|0x20000000, 0, uintptr(unsafe.Pointer(&status)), uintptr(unsafe.Pointer(&size)), 0)
	runtime.KeepAlive(&status)
	runtime.KeepAlive(&size)
	if ok == 0 {
		return 0, registryWindowsError("WinHttpQueryHeaders status", err)
	}
	return int(status), nil
}
func (request *registryWindowsRequest) Location() (string, error) {
	var size uint32
	_, _, err := registryWinQuery.Call(request.handle, 33, 0, 0, uintptr(unsafe.Pointer(&size)), 0)
	if err != syscall.ERROR_INSUFFICIENT_BUFFER {
		return "", registryWindowsError("WinHttpQueryHeaders redirect", err)
	}
	if size == 0 || size > 64<<10 {
		return "", errors.New("invalid Windows HTTPS redirect header size")
	}
	value := make([]uint16, (size+1)/2)
	ok, _, callErr := registryWinQuery.Call(request.handle, 33, 0, uintptr(unsafe.Pointer(&value[0])), uintptr(unsafe.Pointer(&size)), 0)
	runtime.KeepAlive(value)
	runtime.KeepAlive(&size)
	if ok == 0 {
		return "", registryWindowsError("WinHttpQueryHeaders redirect", callErr)
	}
	return syscall.UTF16ToString(value), nil
}
func (request *registryWindowsRequest) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	var read uint32
	ok, _, err := registryWinRead.Call(request.handle, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), uintptr(unsafe.Pointer(&read)))
	runtime.KeepAlive(buffer)
	runtime.KeepAlive(&read)
	if ok == 0 {
		return 0, registryWindowsError("WinHttpReadData", err)
	}
	if read > uint32(len(buffer)) {
		return 0, errors.New("invalid Windows HTTPS read length")
	}
	if read == 0 {
		return 0, io.EOF
	}
	return int(read), nil
}
func (request *registryWindowsRequest) Close() {
	if request.handle != 0 {
		registryWinClose.Call(request.handle)
		request.handle = 0
	}
	if request.connection != 0 {
		registryWinClose.Call(request.connection)
		request.connection = 0
	}
	registryWinRequests.Delete(request.token)
}
