//go:build windows

package platform

import (
	"context"
	"encoding/binary"
	"syscall"
)

func readSourceInterfaces(ctx context.Context) ([]sourceInterface, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rows, err := readWindowsAdapters()
	if err != nil {
		return nil, err
	}
	return sourceInterfacesFromRows(rows), ctx.Err()
}

func setSourceInterface(raw syscall.RawConn, binding SourceBinding) error {
	level, option, value := windowsSourceSocketOption(binding)
	var optionErr error
	err := raw.Control(func(fd uintptr) {
		optionErr = syscall.SetsockoptInt(syscall.Handle(fd), level, option, value)
	})
	if err != nil {
		return err
	}
	return optionErr
}

// Windows IP_UNICAST_IF expects network-order bytes; IPV6_UNICAST_IF expects
// host order. Both options are 31 in the Windows SDK, at different IP levels.
// https://learn.microsoft.com/windows/win32/winsock/ipproto-ip-socket-options
// https://learn.microsoft.com/windows/win32/winsock/ipproto-ipv6-socket-options
func windowsSourceSocketOption(binding SourceBinding) (level, option, value int) {
	if binding.Family == "ipv6" {
		return syscall.IPPROTO_IPV6, 31, binding.socketIndex()
	}
	var bytes [4]byte
	binary.BigEndian.PutUint32(bytes[:], uint32(binding.socketIndex()))
	return syscall.IPPROTO_IP, 31, int(binary.LittleEndian.Uint32(bytes[:]))
}
