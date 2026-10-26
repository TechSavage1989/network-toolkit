//go:build linux

package main

import (
	"errors"
	"os"
	"syscall"
)

func lockStorageFile(file *os.File, exclusive bool) error {
	mode := syscall.LOCK_SH | syscall.LOCK_NB
	if exclusive {
		mode = syscall.LOCK_EX | syscall.LOCK_NB
	}
	err := syscall.Flock(int(file.Fd()), mode)
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return ErrAppStorageBusy
	}
	return err
}

func unlockStorageFile(file *os.File) error    { return syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }
func storageIsLink(info os.FileInfo) bool      { return info.Mode()&os.ModeSymlink != 0 }
func validateStoragePlatformPath(string) error { return nil }
