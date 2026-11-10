//go:build !windows || !toolkit_terminal_handoff

package main

func runTerminalHandoffIfRequested([]string) bool { return false }
