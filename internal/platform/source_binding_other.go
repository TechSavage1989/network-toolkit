//go:build !windows && !linux

package platform

import (
	"errors"
	"syscall"
)

func setSourceInterface(_ syscall.RawConn, _ SourceBinding) error {
	return errors.New("explicit source-interface sockets are supported on Windows and Linux only")
}
