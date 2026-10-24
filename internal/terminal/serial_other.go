//go:build !windows && !linux

package terminal

import (
	"context"
	"errors"
)

func openSerialDevice(context.Context, ConnectRequest) (serialDevice, error) {
	return nil, errors.New("native serial sessions are supported on Windows and Linux")
}

func ListSerialPorts(ctx context.Context) ([]SerialPort, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []SerialPort{}, errors.New("serial enumeration is supported on Windows and Linux")
}
