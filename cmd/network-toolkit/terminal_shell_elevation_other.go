//go:build !windows

package main

import (
	"errors"
	"networktoolkit/internal/terminal"
)

func configureLocalShellElevation(*terminal.Manager) {}
func runTerminalShellHelper(string, uint32) error {
	return errors.New("administrator shell tabs come to the Linux edition in a later beta")
}
