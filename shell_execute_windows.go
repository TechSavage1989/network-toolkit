//go:build windows

package main

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

type externalShellExecuteInfo struct {
	Size          uint32
	Mask          uint32
	Window        uintptr
	Verb          *uint16
	File          *uint16
	Parameters    *uint16
	Directory     *uint16
	Show          int32
	Instance      uintptr
	IDList        uintptr
	Class         *uint16
	ClassKey      uintptr
	HotKey        uint32
	IconOrMonitor uintptr
	Process       syscall.Handle
}

// The connection tools use this helper for a validated HTTP(S) destination or
// the system Remote Desktop client. It never accepts a terminal executable,
// command shell, PowerShell command, or command template from the interface.
func shellExecuteWindowsOpen(ctx context.Context, filename string, arguments []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole := syscall.NewLazyDLL("ole32.dll")
	result, _, _ := ole.NewProc("CoInitializeEx").Call(0, 2|4) // STA | DISABLE_OLE1DDE.
	if int32(result) < 0 {
		return errors.New("Windows could not initialize the application launcher")
	}
	defer ole.NewProc("CoUninitialize").Call()
	verb, _ := syscall.UTF16PtrFromString("open")
	file, err := syscall.UTF16PtrFromString(filename)
	if err != nil {
		return err
	}
	var parameters *uint16
	if len(arguments) > 0 {
		args := make([]string, len(arguments))
		for i, arg := range arguments {
			args[i] = syscall.EscapeArg(arg)
		}
		parameters, err = syscall.UTF16PtrFromString(strings.Join(args, " "))
		if err != nil {
			return err
		}
	}
	var directory *uint16
	if home, err := os.UserHomeDir(); err == nil {
		directory, _ = syscall.UTF16PtrFromString(home)
	}
	info := externalShellExecuteInfo{Mask: 0x40 | 0x100 | 0x400, Verb: verb, File: file, Parameters: parameters, Directory: directory, Show: 1}
	info.Size = uint32(unsafe.Sizeof(info))
	// The validated system Remote Desktop client runs directly. HTTP(S) URLs
	// use the user's associated browser and keep lpParameters nil.
	proc := syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteExW")
	ok, _, callErr := proc.Call(uintptr(unsafe.Pointer(&info)))
	runtime.KeepAlive(info)
	if info.Process != 0 {
		_ = syscall.CloseHandle(info.Process)
	}
	if ok == 0 {
		if callErr == syscall.Errno(0) {
			return errors.New("Windows did not start the selected application")
		}
		return callErr
	}
	return nil
}
