//go:build !windows

package main

import (
	"errors"
	"time"
)

func adminRestartSupported() bool { return false }

func (a *application) launchAdminRestart() (string, error) {
	return "", errors.New("Restart as administrator is available on Windows only.")
}

func waitAdminParent(int, time.Duration, string) error {
	return errors.New("Restart as administrator is available on Windows only.")
}
