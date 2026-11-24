//go:build !windows

package main

import "networktoolkit/internal/terminal"

func configureLocalShellElevation(*terminal.Manager) {}

func processElevated() bool { return false }
