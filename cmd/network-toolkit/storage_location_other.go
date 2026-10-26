//go:build !windows && !linux

package main

import (
	"errors"
	"os"
)

func lockStorageFile(*os.File, bool) error {
	return errors.New("Safe app-data maintenance currently supports Windows and Linux development checks.")
}
func unlockStorageFile(*os.File) error         { return nil }
func storageIsLink(info os.FileInfo) bool      { return info.Mode()&os.ModeSymlink != 0 }
func validateStoragePlatformPath(string) error { return nil }
