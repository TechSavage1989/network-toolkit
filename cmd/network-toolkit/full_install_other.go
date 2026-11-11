//go:build !windows

package main

import (
	"context"
	"errors"
	"os"
)

func fullInstallStartSetup(context.Context, string) error {
	return errors.New("Setup runs on Windows.")
}

func fullInstallOpenProgram(context.Context, string) error {
	return errors.New("The installed program runs on Windows.")
}

// Outside Windows (where only the tests run this) a private folder is mode
// 0700, and the held file is an ordinary read-only open.
func fullInstallMakePrivateDir(path string) error {
	err := os.Mkdir(path, 0o700)
	if errors.Is(err, os.ErrExist) {
		return os.ErrExist
	}
	return err
}

func fullInstallCheckPrivateDir(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
		return errors.New(path + " is not a private plain folder")
	}
	return nil
}

func fullInstallOpenHeld(path string) (*os.File, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("The downloaded Setup is not a plain file.")
	}
	return os.Open(path)
}
