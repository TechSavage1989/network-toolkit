//go:build !windows

package terminal

import (
	"context"
	"errors"
)

func LocalShellAvailable() bool { return false }
func openLocalShell(context.Context, ConnectRequest, transportCallbacks) (transport, error) {
	return nil, errors.New("local Command Prompt and PowerShell require Windows 10 version 1809 or later")
}
