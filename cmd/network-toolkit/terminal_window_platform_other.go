//go:build !windows && !linux

package main

import (
	"errors"
	"os/exec"
)

func acquireTerminalWindowLock(string) (func(), error) {
	return nil, errors.New("separate Terminal windows come to the Linux edition in a later beta")
}

func configureTerminalChild(*exec.Cmd) {}
