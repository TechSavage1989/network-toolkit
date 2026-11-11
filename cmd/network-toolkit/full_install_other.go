//go:build !windows

package main

import (
	"context"
	"errors"
)

func fullInstallStartSetup(context.Context, string) error {
	return errors.New("Setup runs on Windows.")
}
