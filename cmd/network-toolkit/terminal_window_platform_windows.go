//go:build windows

package main

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

var (
	terminalLockKernel   = syscall.NewLazyDLL("kernel32.dll") // Windows KnownDLL.
	terminalLockFileEx   = terminalLockKernel.NewProc("LockFileEx")
	terminalUnlockFileEx = terminalLockKernel.NewProc("UnlockFileEx")
)

func acquireTerminalWindowLock(path string) (release func(), err error) {
	file, err := openTerminalWindowLockFile(path)
	if err != nil {
		return nil, err
	}
	// A synchronous handle plus FAIL_IMMEDIATELY never leaves an asynchronous
	// request outstanding. Windows permits locking this byte beyond empty EOF.
	var overlapped syscall.Overlapped
	ok, _, callErr := terminalLockFileEx.Call(file.Fd(), 1|2, 0, 1, 0, uintptr(unsafe.Pointer(&overlapped))) // FAIL_IMMEDIATELY | EXCLUSIVE_LOCK.
	runtime.KeepAlive(file)
	if ok == 0 {
		_ = file.Close()
		if errors.Is(callErr, syscall.Errno(33)) { // ERROR_LOCK_VIOLATION.
			return nil, ErrTerminalWindowInUse
		}
		if callErr == syscall.Errno(0) {
			callErr = errors.New("LockFileEx failed")
		}
		return nil, fmt.Errorf("lock the Terminal instance: %w", callErr)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			var unlock syscall.Overlapped
			terminalUnlockFileEx.Call(file.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&unlock)))
			_ = file.Close() // Closing also releases the lock if explicit unlock failed.
		})
	}, nil
}

func configureTerminalChild(cmd *exec.Cmd) {
	// The GUI build needs no console. CREATE_NO_WINDOW also keeps a development
	// console build independent; it is ignored for a Windows GUI executable.
	// Do not use HideWindow: STARTF_USESHOWWINDOW/SW_HIDE can hide its native UI.
	// Console process groups and handle isolation do not override external jobs.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		NoInheritHandles: true,
		CreationFlags:    0x08000000 | syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
