package platform

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"unsafe"
)

// Windows SDK ICMP wire and native reply structures are parsed here so their
// fixed fixtures can also be validated on the build host. SDK IPv6 packing:
// https://github.com/microsoft/win32metadata/blob/main/generation/WinSDK/RecompiledIdlHeaders/um/IPExport.h
// Independent native interop definitions:
// https://github.com/dotnet/runtime/blob/main/src/libraries/Common/src/Interop/Windows/IpHlpApi/Interop.ICMP.cs
type icmpOptions struct {
	TTL, TOS, Flags, OptionsSize uint8
	OptionsData                  uintptr
}
type icmpEchoReply struct {
	Address, Status, RTT uint32
	DataSize, Reserved   uint16
	Data                 uintptr
	Options              icmpOptions
}

func parseICMP4Reply(data []byte) PingReply {
	optionsOffset := int(unsafe.Offsetof(icmpEchoReply{}.Options))
	if len(data) < optionsOffset+4 {
		return PingReply{Status: "error", Error: "Windows returned an incomplete ICMP reply"}
	}
	code := binary.LittleEndian.Uint32(data[4:8])
	status, message := icmpStatus(code)
	reply := PingReply{Address: net.IP(data[:4]).String(), Success: code == 0,
		RTTMs: float64(binary.LittleEndian.Uint32(data[8:12])), TTL: int(data[optionsOffset]), Status: status}
	if code != 0 {
		reply.Error = message
	}
	if reply.Address == "0.0.0.0" {
		reply.Address = ""
	}
	return reply
}

func parseICMP6Reply(data []byte) PingReply {
	if len(data) < 36 {
		return PingReply{Status: "error", Error: "Windows returned an incomplete ICMPv6 reply"}
	}
	code := binary.LittleEndian.Uint32(data[28:32])
	status, message := icmpStatus(code)
	// IP Helper's reply address is packed: port (2), flow info (4), IPv6
	// address (16), scope (4); the outer reply pads to status at offset 28.
	ip := net.IP(data[6:22])
	address := ip.String()
	scope := binary.LittleEndian.Uint32(data[22:26])
	if ip.IsLinkLocalUnicast() && scope != 0 {
		address += "%" + strconv.FormatUint(uint64(scope), 10)
	}
	if ip.IsUnspecified() {
		address = ""
	}
	reply := PingReply{Address: address, Success: code == 0, RTTMs: float64(binary.LittleEndian.Uint32(data[32:36])), Status: status}
	if code != 0 {
		reply.Error = message
	}
	return reply
}

func icmpStatus(code uint32) (string, string) {
	switch code {
	case 0:
		return "reply", "Echo reply received"
	case 11010:
		return "timeout", "No ICMP response before the timeout"
	case 11013:
		return "ttl_expired", "TTL or hop limit expired in transit"
	case 11014:
		return "ttl_expired", "TTL or hop limit expired during reassembly"
	case 11002:
		return "unreachable", "Destination network unreachable or no route"
	case 11003:
		return "unreachable", "Destination host or address unreachable"
	case 11004:
		return "unreachable", "Destination protocol unreachable or administratively prohibited"
	case 11005:
		return "unreachable", "Destination port unreachable"
	case 11009:
		return "packet_too_big", "Packet too large for the path"
	case 11040:
		return "unreachable", "Destination unreachable"
	case 11041:
		return "ttl_expired", "Time exceeded"
	case 11045:
		return "error", "IPv6 destination scope mismatch; check the interface scope ID"
	case 11050:
		return "error", "General ICMP failure; check interface and routing state"
	default:
		return "error", fmt.Sprintf("Windows ICMP status %d", code)
	}
}
