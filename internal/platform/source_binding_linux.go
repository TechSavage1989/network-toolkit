//go:build linux

package platform

import "syscall"

func setSourceInterface(raw syscall.RawConn, binding SourceBinding) error {
	var optionErr error
	err := raw.Control(func(fd uintptr) {
		optionErr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, binding.InterfaceName)
	})
	if err != nil {
		return err
	}
	// Kernel/privilege failures are returned; never retry with an unbound socket.
	return optionErr
}
