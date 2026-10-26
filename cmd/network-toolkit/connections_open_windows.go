//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

func launchDeviceConnection(ctx context.Context, target connectionOpenTarget) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var mstsc string
	if target.Protocol == "rdp" {
		var err error
		mstsc, err = connectionSystemMSTSC()
		if err != nil {
			return err
		}
	}
	command, err := buildConnectionOpenCommand(target, mstsc)
	if err != nil {
		return err
	}
	// HTTP(S) uses the registered default browser with no lpParameters. RDP
	// uses one fixed /v: argument on the system client, without a command shell.
	return shellExecuteWindowsOpen(ctx, command.File, command.Arguments)
}

func connectionSystemMSTSC() (string, error) {
	// Resolve the genuine system directory rather than PATH, the app directory,
	// an association, or a user-modifiable SystemRoot environment variable.
	buffer := make([]uint16, 32768)
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetSystemDirectoryW")
	n, _, callErr := proc.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if n == 0 {
		if callErr == syscall.Errno(0) {
			return "", errors.New("Windows could not locate its system directory.")
		}
		return "", fmt.Errorf("Locate the Windows system directory: %w", callErr)
	}
	if n >= uintptr(len(buffer)) {
		return "", errors.New("The Windows system directory path is too long.")
	}
	directory := syscall.UTF16ToString(buffer[:n])
	if !filepath.IsAbs(directory) {
		return "", errors.New("Windows returned an invalid system directory.")
	}
	path := filepath.Join(directory, "mstsc.exe")
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("Windows Remote Desktop Connection (mstsc.exe) was not found in the system directory.")
	}
	return path, nil
}
