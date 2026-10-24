//go:build windows

package identity

import (
	"syscall"
	"unsafe"
)

var moveDatabaseFile = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

func replaceDatabaseFile(source, destination string) error {
	a, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	b, err := syscall.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	ok, _, err := moveDatabaseFile.Call(uintptr(unsafe.Pointer(a)), uintptr(unsafe.Pointer(b)), 1|8)
	if ok == 0 {
		return err
	}
	return nil
}
