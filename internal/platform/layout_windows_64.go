//go:build windows && (amd64 || arm64)

package platform

import "unsafe"

// A negative array bound makes any ABI offset regression a Windows x64/ARM64
// compilation error, even on a build host that cannot execute Windows tests.
var (
	_ [int(unsafe.Offsetof(icmpEchoReply{}.Options)) - 24]byte
	_ [24 - int(unsafe.Offsetof(icmpEchoReply{}.Options))]byte
	_ [int(unsafe.Sizeof(icmpEchoReply{})) - 40]byte
	_ [40 - int(unsafe.Sizeof(icmpEchoReply{}))]byte
	_ [int(unsafe.Sizeof(sockaddr6{})) - 28]byte
	_ [28 - int(unsafe.Sizeof(sockaddr6{}))]byte
	_ [int(unsafe.Offsetof(adapterAddress{}.TransmitLinkSpeed)) - 184]byte
	_ [184 - int(unsafe.Offsetof(adapterAddress{}.TransmitLinkSpeed))]byte
	_ [int(unsafe.Offsetof(adapterAddress{}.Gateway)) - 208]byte
	_ [208 - int(unsafe.Offsetof(adapterAddress{}.Gateway))]byte
)
