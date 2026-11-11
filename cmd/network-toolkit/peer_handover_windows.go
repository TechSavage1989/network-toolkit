//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

const peerHandoverDriveRemote = 4 // DRIVE_REMOTE

var peerHandoverGetDriveType = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDriveTypeW")

// peerHandoverRemoteDrive reports a path on a mapped network drive: such a
// file is not on this computer, so its tab is not handed over. The caller has
// already required a drive letter.
func peerHandoverRemoteDrive(path string) bool {
	if len(path) < 3 {
		return true
	}
	root, err := syscall.UTF16PtrFromString(path[:2] + `\`)
	if err != nil {
		return true
	}
	kind, _, _ := peerHandoverGetDriveType.Call(uintptr(unsafe.Pointer(root)))
	return kind == peerHandoverDriveRemote
}
