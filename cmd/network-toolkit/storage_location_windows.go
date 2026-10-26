//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

var storageKernel = syscall.NewLazyDLL("kernel32.dll") // Windows KnownDLL.
var storageLockFileEx = storageKernel.NewProc("LockFileEx")
var storageUnlockFileEx = storageKernel.NewProc("UnlockFileEx")

func lockStorageFile(file *os.File, exclusive bool) error {
	flags := uintptr(1) // LOCKFILE_FAIL_IMMEDIATELY; synchronous file handle.
	if exclusive {
		flags |= 2
	}
	var overlapped syscall.Overlapped
	ok, _, err := storageLockFileEx.Call(file.Fd(), flags, 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	runtime.KeepAlive(file)
	if ok != 0 {
		return nil
	}
	if errors.Is(err, syscall.Errno(33)) {
		return ErrAppStorageBusy
	}
	if err == syscall.Errno(0) {
		return errors.New("The app-data file lock failed.")
	}
	return err
}

func unlockStorageFile(file *os.File) error {
	var overlapped syscall.Overlapped
	ok, _, err := storageUnlockFileEx.Call(file.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	runtime.KeepAlive(file)
	if ok != 0 {
		return nil
	}
	if err == syscall.Errno(0) {
		return errors.New("The app-data file unlock failed.")
	}
	return err
}

func storageIsLink(info os.FileInfo) bool {
	if info.Mode()&os.ModeSymlink != 0 {
		return true
	}
	attributes, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return ok && attributes.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

func validateStoragePlatformPath(path string) error {
	if strings.HasPrefix(path, `\\.\`) || strings.HasPrefix(path, `\\?\`) || strings.Contains(path[len(filepath.VolumeName(path)):], ":") {
		return errors.New("Choose a normal folder path, not a Windows device or alternate data stream.")
	}
	for _, component := range strings.Split(path[len(filepath.VolumeName(path)):], string(filepath.Separator)) {
		if component != "" && strings.TrimRight(component, " .") != component {
			return errors.New("An app-data folder name cannot end in a dot or space.")
		}
	}
	return nil
}
