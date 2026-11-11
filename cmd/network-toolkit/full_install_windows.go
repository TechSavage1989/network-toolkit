//go:build windows

package main

import "context"

// Setup and the installed program start like a double-click: ShellExecuteEx
// "open" with no arguments, so Setup runs interactively (never silent).
func fullInstallStartSetup(ctx context.Context, path string) error {
	return shellExecuteWindowsOpen(ctx, path, nil)
}
