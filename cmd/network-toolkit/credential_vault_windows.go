//go:build windows

package main

import (
	"errors"
	"syscall"
	"unsafe"
)

type dataBlob struct {
	Size uint32
	Data *byte
}

var (
	crypt32DLL         = syscall.NewLazyDLL("crypt32.dll")
	kernel32CredDLL    = syscall.NewLazyDLL("kernel32.dll")
	cryptProtectData   = crypt32DLL.NewProc("CryptProtectData")
	cryptUnprotectData = crypt32DLL.NewProc("CryptUnprotectData")
	localFreeCred      = kernel32CredDLL.NewProc("LocalFree")
)

func credentialProtectionAvailable() bool { return true }

const cryptProtectUIForbidden = 0x1

func blobFromBytes(value []byte) dataBlob {
	if len(value) == 0 {
		return dataBlob{}
	}
	return dataBlob{Size: uint32(len(value)), Data: &value[0]}
}

func bytesFromBlob(blob dataBlob) []byte {
	if blob.Data == nil || blob.Size == 0 {
		return []byte{}
	}
	return append([]byte(nil), unsafe.Slice(blob.Data, int(blob.Size))...)
}

func protectCredentialBlob(value []byte) ([]byte, error) {
	in := blobFromBytes(value)
	var out dataBlob
	result, _, callErr := cryptProtectData.Call(
		uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, cryptProtectUIForbidden,
		uintptr(unsafe.Pointer(&out)),
	)
	if result == 0 {
		if callErr == syscall.Errno(0) {
			callErr = errors.New("CryptProtectData failed")
		}
		return nil, callErr
	}
	defer localFreeCred.Call(uintptr(unsafe.Pointer(out.Data)))
	return bytesFromBlob(out), nil
}

func unprotectCredentialBlob(value []byte) ([]byte, error) {
	in := blobFromBytes(value)
	var out dataBlob
	result, _, callErr := cryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, cryptProtectUIForbidden,
		uintptr(unsafe.Pointer(&out)),
	)
	if result == 0 {
		if callErr == syscall.Errno(0) {
			callErr = errors.New("CryptUnprotectData failed")
		}
		return nil, callErr
	}
	defer localFreeCred.Call(uintptr(unsafe.Pointer(out.Data)))
	if out.Data != nil && out.Size > 0 {
		defer clear(unsafe.Slice(out.Data, int(out.Size)))
	}
	return bytesFromBlob(out), nil
}
