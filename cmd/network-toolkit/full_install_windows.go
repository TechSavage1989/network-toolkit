//go:build windows

package main

import "context"

// The installed program starts like a double-click: ShellExecuteEx "open"
// with no arguments. (Setup itself is started from a held, verified handle:
// fullInstallStartSetup in private_files_windows.go.)
func fullInstallOpenProgram(ctx context.Context, path string) error {
	return shellExecuteWindowsOpen(ctx, path, nil)
}
