//go:build windows

package main

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

// A folder-specific named kernel mutex serializes different launcher processes
// without a stale on-disk lock after a crash. Windows mutex ownership is bound
// to the OS thread, so retain that thread until releasing ownership.
func lockUpdateNamingDirectory(dir string) (func(), error) {
	name, err := syscall.UTF16PtrFromString(`Local\NetworkToolkitNaming-` + updateHash([]byte(strings.ToLower(dir))))
	if err != nil {
		return nil, err
	}
	runtime.LockOSThread()
	kernel := syscall.NewLazyDLL("kernel32.dll")
	handle, _, callErr := kernel.NewProc("CreateMutexW").Call(0, 0, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("Cannot lock versioned filename management: %w", callErr)
	}
	h := syscall.Handle(handle)
	result, err := syscall.WaitForSingleObject(h, 5000)
	if err != nil || (result != 0 && result != 0x80) { // acquired or abandoned
		_ = syscall.CloseHandle(h)
		runtime.UnlockOSThread()
		return nil, errors.New("Another process is updating the versioned filename; try again after it finishes")
	}
	return func() {
		_, _, _ = kernel.NewProc("ReleaseMutex").Call(handle)
		_ = syscall.CloseHandle(h)
		runtime.UnlockOSThread()
	}, nil
}

func moveUpdateNamingNoReplace(source, target string) error {
	s, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	t, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	// MOVEFILE_WRITE_THROUGH, deliberately WITHOUT MOVEFILE_REPLACE_EXISTING.
	result, _, err := syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW").Call(uintptr(unsafe.Pointer(s)), uintptr(unsafe.Pointer(t)), 8)
	if result == 0 {
		return err
	}
	return nil
}
