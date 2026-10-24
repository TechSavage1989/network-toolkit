//go:build !windows

package main

import "os/exec"

func configureTerminalViewChild(cmd *exec.Cmd) {
	configureTerminalChild(cmd)
}
