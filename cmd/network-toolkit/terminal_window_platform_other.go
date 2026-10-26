//go:build !windows && !linux

package main

import (
	"errors"
	"os/exec"
)

func acquireTerminalWindowLock(string) (func(), error) {
	return nil, errors.New("independent Terminal windows are supported on Windows; Linux supports local development checks")
}

func configureTerminalChild(*exec.Cmd) {}
