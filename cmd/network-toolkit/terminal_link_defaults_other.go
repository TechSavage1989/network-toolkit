//go:build !windows

package main

import (
	"context"
	"errors"
)

// The Linux edition has no standalone Terminal menu entry, so it does not
// register as a handler for ssh, telnet or serial links.
func terminalLinkRegistrationState() terminalLinkRegistration {
	return terminalLinkRegistration{}
}

func openTerminalLinkSettings(context.Context) error {
	return errors.New("Default apps are available in Windows.")
}
