//go:build windows

package main

import (
	"context"
	"errors"

	"networktoolkit/internal/terminal"
)

// Administrator local shells (owner decision after v1.21.80): a shell opened
// with Run as administrator is a direct child of this process, the same as
// any other local shell, so it keeps every Terminal feature. That needs this
// process itself to run as administrator; otherwise the page offers Restart
// as administrator (admin_restart.go). There is no hidden elevated helper and
// no pipe relay: a hidden elevated copy that runs cmd.exe for another process
// looks like a remote shell to security software, which removed the program.
var errAdminShellNeedsRestart = errors.New("Run as administrator needs Network Toolkit itself to run as administrator. Choose Restart as administrator, or start Network Toolkit with Run as administrator.")

var adminShellProcessElevated = localElevated

func configureLocalShellElevation(m *terminal.Manager) {
	if m != nil {
		m.SetElevatedLocalShellOpener(openAdministratorShell)
	}
}

func openAdministratorShell(ctx context.Context, o terminal.LocalShellOptions, output func([]byte)) (terminal.LocalShellTransport, error) {
	if !adminShellProcessElevated() {
		return nil, errAdminShellNeedsRestart
	}
	return terminal.OpenLocalShellProcess(ctx, o, output)
}

func processElevated() bool { return localElevated() }
