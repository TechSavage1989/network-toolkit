//go:build !windows

package main

import (
	"errors"
	"networktoolkit/internal/terminal"
)

func configureLocalShellElevation(*terminal.Manager) {}
func runTerminalShellHelper(string, uint32) error {
	return errors.New("administrator local shells require Windows")
}
