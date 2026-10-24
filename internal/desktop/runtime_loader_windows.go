//go:build windows

package desktop

// Runtime discovery is adapted from wailsapp/go-webview2 v1.0.23, commit
// e08e9397f04ad0b1050d7e5bdc8e27faddc87987. That implementation is a Go port
// of OpenWebView2Loader. Copyright (c) 2020 John Chadwick and (c) 2022 Wails
// Project Developers. See LOADER_LICENSE.txt for the full ISC license.

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

const installedWebViewKey = `Software\Microsoft\EdgeUpdate\ClientState\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`

var runtimeLoadLibraryEx = syscall.NewLazyDLL("kernel32.dll").NewProc("LoadLibraryExW")

type installedWebViewRuntime struct {
	DLL     *syscall.DLL
	Create  *syscall.Proc
	Path    string
	Version string
}

// loadInstalledWebViewRuntime uses only the registered Evergreen Runtime.
// Nothing is extracted, mapped from an embedded image, searched on PATH, or
// installed. The returned native module must remain loaded after Create is
// invoked because it may own asynchronous callbacks and environment objects.
func loadInstalledWebViewRuntime() (*installedWebViewRuntime, error) {
	if runtime.GOARCH != "amd64" {
		return nil, errors.New("this Network Toolkit build requires Windows x64")
	}
	var problems []string
	for _, location := range []struct {
		name string
		key  syscall.Handle
	}{
		{"machine", syscall.HKEY_LOCAL_MACHINE},
		{"user", syscall.HKEY_CURRENT_USER},
	} {
		directory, err := readInstalledRuntimeDirectory(location.key)
		if err != nil {
			if err != syscall.ERROR_FILE_NOT_FOUND && err != syscall.ERROR_PATH_NOT_FOUND {
				problems = append(problems, fmt.Sprintf("%s registration: %v", location.name, err))
			}
			continue
		}
		if directory == "" {
			continue
		}
		filename, version, err := installedRuntimePath(directory)
		if err == nil {
			err = validateRuntimeDLL(filename)
		}
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s runtime: %v", location.name, err))
			continue
		}
		path, err := syscall.UTF16PtrFromString(filename)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s runtime path: %v", location.name, err))
			continue
		}
		// Dependencies are resolved only in the installed DLL's own directory
		// and System32. Never search the working directory or the app folder.
		module, _, loadErr := runtimeLoadLibraryEx.Call(uintptr(unsafe.Pointer(path)), 0, 0x0100|0x0800)
		runtime.KeepAlive(path)
		if module == 0 {
			problems = append(problems, fmt.Sprintf("load %s runtime: %v", location.name, nativeError("Windows DLL loading", loadErr)))
			continue
		}
		dll := &syscall.DLL{Name: filename, Handle: syscall.Handle(module)}
		create, err := dll.FindProc("CreateWebViewEnvironmentWithOptionsInternal")
		if err != nil {
			dll.Release() // No environment creation or asynchronous call has begun.
			problems = append(problems, fmt.Sprintf("%s runtime: the WebView2 environment entry point is unavailable", location.name))
			continue
		}
		return &installedWebViewRuntime{DLL: dll, Create: create, Path: filename, Version: version}, nil
	}
	message := "A usable installed Microsoft Edge WebView2 Runtime was not found. This application requires the x64 WebView2 Runtime already installed on Windows."
	if len(problems) != 0 {
		message += "\n\n" + strings.Join(problems, "\n")
	}
	return nil, errors.New(message)
}

func readInstalledRuntimeDirectory(root syscall.Handle) (string, error) {
	keyName, _ := syscall.UTF16PtrFromString(installedWebViewKey)
	var key syscall.Handle
	if err := syscall.RegOpenKeyEx(root, keyName, 0, syscall.KEY_QUERY_VALUE|syscall.KEY_WOW64_32KEY, &key); err != nil {
		return "", err
	}
	defer syscall.RegCloseKey(key)
	valueName, _ := syscall.UTF16PtrFromString("EBWebView")
	// A concurrently updating Evergreen registration may change between the
	// size query and read. Retry that case twice, keeping allocation bounded.
	for attempt := 0; attempt < 3; attempt++ {
		var kind, size uint32
		if err := syscall.RegQueryValueEx(key, valueName, nil, &kind, nil, &size); err != nil {
			return "", err
		}
		if kind != syscall.REG_SZ && kind != syscall.REG_EXPAND_SZ {
			return "", errors.New("the WebView2 installation path has an invalid registry type")
		}
		if size == 0 {
			return "", nil
		}
		if size%2 != 0 || size > 65536 {
			return "", errors.New("the WebView2 installation path has an invalid length")
		}
		buffer := make([]uint16, size/2+1)
		if err := syscall.RegQueryValueEx(key, valueName, nil, &kind, (*byte)(unsafe.Pointer(&buffer[0])), &size); err != nil {
			if err == syscall.ERROR_MORE_DATA {
				continue
			}
			return "", err
		}
		if (kind != syscall.REG_SZ && kind != syscall.REG_EXPAND_SZ) || size%2 != 0 || int(size/2) >= len(buffer) {
			return "", errors.New("the WebView2 installation path changed to an invalid value")
		}
		// Upstream reads a literal installation path even for REG_EXPAND_SZ.
		// Do not turn environment variables into additional runtime overrides.
		return syscall.UTF16ToString(buffer[:size/2]), nil
	}
	return "", errors.New("the WebView2 installation is changing; reopen the application after the runtime update completes")
}
