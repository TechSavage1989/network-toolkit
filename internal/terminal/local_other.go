//go:build !windows

package terminal

import (
	"context"
	"errors"
)

func LocalShellAvailable() bool { return false }
func openLocalShell(context.Context, ConnectRequest, transportCallbacks) (transport, error) {
	return nil, errors.New("local shell tabs come to the Linux edition in a later beta")
}
