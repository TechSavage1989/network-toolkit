//go:build windows

package platform

import (
	"syscall"
	"testing"
)

func TestSourceBindingWindowsSocketInterfaceByteOrder(t *testing.T) {
	for _, test := range []struct {
		binding      SourceBinding
		level, value int
	}{
		{SourceBinding{InterfaceIndex: 7, Family: "ipv4"}, syscall.IPPROTO_IP, 0x07000000},
		{SourceBinding{InterfaceIndex: 0x01020304, Family: "ipv4"}, syscall.IPPROTO_IP, 0x04030201},
		{SourceBinding{InterfaceIndex: 7, IPv6InterfaceIndex: 17, Family: "ipv6"}, syscall.IPPROTO_IPV6, 17},
	} {
		level, option, value := windowsSourceSocketOption(test.binding)
		if level != test.level || option != 31 || value != test.value {
			t.Fatalf("Windows source option: %d/%d/%#x, want %d/31/%#x", level, option, value, test.level, test.value)
		}
	}
}
