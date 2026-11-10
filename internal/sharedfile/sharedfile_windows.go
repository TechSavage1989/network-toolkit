//go:build windows

package sharedfile

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	fileShareDelete = 0x00000004
	// As os.Open does for a read-only open: without it a folder cannot be
	// opened at all and answers "access denied".
	fileFlagBackupSemantics  = 0x02000000
	fileFlagOpenReparsePoint = 0x00200000
	accessDelete             = 0x00010000
	accessSynchronize        = 0x00100000
	moveFileReplace          = 0x1
	moveFileWriteThrough     = 0x8
	// SetFileInformationByHandle(FileRenameInfoEx) and its flags.
	fileRenameInfoEx       = 22
	renameReplaceIfExists  = 0x1
	renamePosixSemantics   = 0x2
	errorSharingViolation  = syscall.Errno(32)
	errorLockViolation     = syscall.Errno(33)
	errorDeletePending     = syscall.Errno(303)
	errorUserMappedSection = syscall.Errno(1224)
)

var (
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	moveFileEx                 = kernel32.NewProc("MoveFileExW")
	setFileInformationByHandle = kernel32.NewProc("SetFileInformationByHandle")
)

// open is os.Open with FILE_SHARE_DELETE. Go's os.Open shares reading and
// writing only, so a file it has open cannot be replaced or removed by
// another process until it is closed.
func open(path string) (*os.File, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|fileShareDelete, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL|fileFlagBackupSemantics, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(handle), path), nil
}

// replace puts source in the place of target. MoveFileEx does that when
// nobody has target open. While a reader has it open, MoveFileEx answers
// "access denied" even if the reader allowed deletion; a rename with POSIX
// semantics (Windows 10 1607 and later, NTFS) replaces the name at once and
// leaves the reader its old file. If that is not available either, the
// caller waits a moment and tries again.
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
	if result != 0 {
		return nil
	}
	failure := &os.LinkError{Op: "rename", Old: source, New: target, Err: callErr}
	if Transient(failure) && replaceOpenTarget(from, target) == nil {
		return nil
	}
	return failure
}

// replaceOpenTarget renames the file over target with POSIX semantics. Any
// error means "not done": the caller keeps the error it already has.
func replaceOpenTarget(from *uint16, target string) error {
	absolute, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	name, err := syscall.UTF16FromString(absolute)
	if err != nil {
		return err
	}
	handle, err := syscall.CreateFile(from, accessDelete|accessSynchronize, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|fileShareDelete, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL|fileFlagBackupSemantics|fileFlagOpenReparsePoint, 0)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(handle)
	// FILE_RENAME_INFO: Flags, RootDirectory (a handle, aligned to the pointer
	// size), FileNameLength in bytes, then the name.
	pointer := int(unsafe.Sizeof(uintptr(0)))
	lengthAt, nameAt := 2*pointer, 2*pointer+4
	nameBytes := (len(name) - 1) * 2
	buffer := make([]uint64, (nameAt+nameBytes+2+7)/8) // 8-byte aligned storage
	raw := unsafe.Slice((*byte)(unsafe.Pointer(&buffer[0])), len(buffer)*8)
	binary.LittleEndian.PutUint32(raw[0:], renameReplaceIfExists|renamePosixSemantics)
	binary.LittleEndian.PutUint32(raw[lengthAt:], uint32(nameBytes))
	for i, unit := range name[:len(name)-1] {
		binary.LittleEndian.PutUint16(raw[nameAt+2*i:], unit)
	}
	result, _, callErr := setFileInformationByHandle.Call(uintptr(handle), fileRenameInfoEx, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(raw)))
	if result == 0 {
		return callErr
	}
	return nil
}

// HeldByAnother reports whether err says that another program has the file
// open in a way that excludes this one (a sharing or lock violation). Unlike
// "access denied" this is never a matter of permissions.
func HeldByAnother(err error) bool {
	var code syscall.Errno
	return errors.As(err, &code) && (code == errorSharingViolation || code == errorLockViolation)
}

// Transient reports whether err only says that another program has the file
// at this moment: a sharing or lock violation, a file that is being removed,
// or "access denied", which is what replacing or opening a file answers
// while someone else has it open or it is on its way out.
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
