package main

import (
	"errors"
	"fmt"
	"os"
)

// ErrTerminalWindowInUse means another process currently owns this Terminal
// data directory. The caller should contact that instance instead of starting
// a second writer for the same settings, host keys and browser profile.
var ErrTerminalWindowInUse = errors.New("the Terminal window is already running")

// The parent directory must already exist. The file intentionally remains in
// place after release: ownership is an OS lock, never the presence of a file.
// Removing it would let another process lock a different file at the same path.
func openTerminalWindowLockFile(path string) (*os.File, error) {
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return nil, errors.New("the Terminal instance lock must be a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect the Terminal instance lock: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open the Terminal instance lock: %w", err)
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = file.Close()
		if err != nil {
			return nil, fmt.Errorf("inspect the Terminal instance lock: %w", err)
		}
		return nil, errors.New("the Terminal instance lock must be a regular file")
	}
	return file, nil
}
