package platform

import (
	"encoding/binary"
	"net"
	"testing"
	"unsafe"
)

func TestWindowsICMPFixtures(t *testing.T) {
	ipv4 := make([]byte, 128)
	copy(ipv4[:4], net.IPv4(192, 0, 2, 10).To4())
	binary.LittleEndian.PutUint32(ipv4[8:12], 17)
	ipv4[int(unsafe.Offsetof(icmpEchoReply{}.Options))] = 57
	reply := parseICMP4Reply(ipv4)
	if !reply.Success || reply.Address != "192.0.2.10" || reply.RTTMs != 17 || reply.TTL != 57 {
		t.Fatalf("invalid IPv4 reply: %+v", reply)
	}
	binary.LittleEndian.PutUint32(ipv4[4:8], 11013)
	reply = parseICMP4Reply(ipv4)
	if reply.Success || reply.Status != "ttl_expired" || reply.Address != "192.0.2.10" {
		t.Fatalf("TTL response lost: %+v", reply)
	}
	ipv6 := make([]byte, 128)
	copy(ipv6[6:22], net.ParseIP("2001:db8::1234").To16())
	binary.LittleEndian.PutUint32(ipv6[32:36], 23)
	reply = parseICMP6Reply(ipv6)
	if !reply.Success || reply.Address != "2001:db8::1234" || reply.RTTMs != 23 || reply.TTL != 0 {
		t.Fatalf("invalid IPv6 reply: %+v", reply)
	}
	copy(ipv6[6:22], net.ParseIP("fe80::1").To16())
	binary.LittleEndian.PutUint32(ipv6[22:26], 7)
	binary.LittleEndian.PutUint32(ipv6[28:32], 11013)
	reply = parseICMP6Reply(ipv6)
	if reply.Address != "fe80::1%7" || reply.Status != "ttl_expired" || reply.Success {
		t.Fatalf("invalid scoped reply: %+v", reply)
	}
}
