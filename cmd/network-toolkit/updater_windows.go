//go:build windows

package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
	"unsafe"
)

func updateInstallSupported() bool { return true }
func configureUpdateHelperProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
func waitUpdateParent(pid int, timeout time.Duration, ready string) error {
	h, e := syscall.OpenProcess(0x00100000, false, uint32(pid))
	if e != nil {
		return errors.New("Could not open the running application process; no files were replaced.")
	}
	defer syscall.CloseHandle(h)
	if err := updateWriteExclusive(ready, []byte("ready"), 0600); err != nil {
		return err
	}
	result, e := syscall.WaitForSingleObject(h, uint32(timeout/time.Millisecond))
	if e != nil {
		return e
	}
	if result != 0 {
		return errors.New("The application did not close in time. Close its other windows and retry.")
	}
	return nil
}

// ReplaceFileW changes the executable and retains the old file in one filesystem
// operation. All three paths are on the same volume; no delete-then-move gap.
// https://learn.microsoft.com/windows/win32/api/winbase/nf-winbase-replacefilew
func replaceUpdateExecutable(target, candidate, backup string) error {
	t, e := syscall.UTF16PtrFromString(target)
	if e != nil {
		return e
	}
	c, e := syscall.UTF16PtrFromString(candidate)
	if e != nil {
		return e
	}
	b, e := syscall.UTF16PtrFromString(backup)
	if e != nil {
		return e
	}
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("ReplaceFileW")
	var last error
	for i := 0; i < 30; i++ {
		result, _, err := proc.Call(uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), uintptr(unsafe.Pointer(b)), 0, 0, 0)
		if result != 0 {
			return nil
		}
		last = err
		if err != syscall.Errno(32) && err != syscall.Errno(33) {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return last
}

func openUpdateReleases(ctx context.Context) error {
	return shellExecuteWindowsOpen(ctx, updateReleasesURL, nil)
}
func restoreMissingUpdateTarget(backup, target string) error {
	if _, e := os.Lstat(target); !os.IsNotExist(e) {
		return errors.New("Target unexpectedly exists")
	}
	return os.Rename(backup, target)
}

// The GUI must not inherit SW_HIDE from the deliberately hidden helper.
func configureUpdateAppProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000 | syscall.CREATE_NEW_PROCESS_GROUP}
}
