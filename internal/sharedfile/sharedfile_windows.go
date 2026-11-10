//go:build windows

package sharedfile

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

const (
	fileShareDelete        = 0x00000004
	moveFileReplace        = 0x1
	moveFileWriteThrough   = 0x8
	errorSharingViolation  = syscall.Errno(32)
	errorLockViolation     = syscall.Errno(33)
	errorDeletePending     = syscall.Errno(303)
	errorUserMappedSection = syscall.Errno(1224)
)

var moveFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

// open is os.Open with FILE_SHARE_DELETE. Go's os.Open shares reading and
// writing only, so a file it has open cannot be replaced or removed by
// another process until it is closed.
func open(path string) (*os.File, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|fileShareDelete, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(handle), path), nil
}

func replace(source, target string) error {
	from, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return &os.LinkError{Op: "rename", Old: source, New: target, Err: err}
	}
	to, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return &os.LinkError{Op: "rename", Old: source, New: target, Err: err}
	}
	result, _, callErr := moveFileEx.Call(uintptr(unsafe.Pointer(from)), uintptr(unsafe.Pointer(to)), moveFileReplace|moveFileWriteThrough)
	if result == 0 {
		return &os.LinkError{Op: "rename", Old: source, New: target, Err: callErr}
	}
	return nil
}

// Transient reports whether err only says that another program has the file
// at this moment: a sharing or lock violation, a file that is being removed,
// or "access denied", which is what replacing a file that someone has open
// without delete sharing answers.
func Transient(err error) bool {
	var code syscall.Errno
	if !errors.As(err, &code) {
		return false
	}
	switch code {
	case syscall.ERROR_ACCESS_DENIED, errorSharingViolation, errorLockViolation, errorDeletePending, errorUserMappedSection:
		return true
	}
	return false
}
