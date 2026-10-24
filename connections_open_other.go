//go:build !windows

package main

import "context"

func launchDeviceConnection(ctx context.Context, _ connectionOpenTarget) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return errConnectionLaunchUnavailable
}
