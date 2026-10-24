package platform

import (
	"context"
	"encoding/binary"
	"net/netip"
	"testing"
)

func TestWindowsNeighborFixtureLayout(t *testing.T) {
	// SDK layout: table count at 0, first 88-byte row at 8, SOCKADDR_INET
	// at row 0, interface index at 28, physical address at 40, length at 72.
	raw := make([]byte, 8+3*88)
	binary.LittleEndian.PutUint32(raw[:4], 3)
	first := raw[8:96]
	binary.LittleEndian.PutUint16(first[:2], 2)
	copy(first[4:8], []byte{192, 0, 2, 1})
	binary.LittleEndian.PutUint32(first[28:32], 12)
	copy(first[40:46], []byte{0, 0x11, 0x22, 0x33, 0x44, 0x55})
	binary.LittleEndian.PutUint32(first[72:76], 6)
	binary.LittleEndian.PutUint32(first[76:80], 5)
	first[80] = 1
	second := raw[96:184]
	binary.LittleEndian.PutUint16(second[:2], 23)
	ipv6 := netip.MustParseAddr("fe80::1234").As16()
	copy(second[8:24], ipv6[:])
	binary.LittleEndian.PutUint32(second[24:28], 14)
	binary.LittleEndian.PutUint32(second[28:32], 14)
	copy(second[40:46], []byte{0x02, 0xaa, 0xbb, 0xcc, 0xdd, 0xee})
	binary.LittleEndian.PutUint32(second[72:76], 6)
	binary.LittleEndian.PutUint32(second[76:80], 4)
	third := raw[184:]
	binary.LittleEndian.PutUint16(third[:2], 2)
	copy(third[4:8], []byte{192, 0, 2, 99})
	binary.LittleEndian.PutUint32(third[28:32], 12)
	binary.LittleEndian.PutUint32(third[72:76], 6)
	binary.LittleEndian.PutUint32(third[76:80], 1)
	rows, err := parseWindowsNeighborTable(raw, map[int]string{12: "Ethernet", 14: "Wi-Fi"})
	if err != nil || len(rows) != 3 {
		t.Fatalf("parse native fixture: %v, %v", rows, err)
	}
	if row := rows[0]; row.Address != "192.0.2.1" || row.MAC != "00:11:22:33:44:55" || row.Interface != "Ethernet" || row.InterfaceIndex != 12 || row.State != "reachable" || row.Family != "IPv4" || !row.IsRouter {
		t.Fatalf("incorrect IPv4 row: %+v", row)
	}
	if row := rows[1]; row.Address != "fe80::1234%14" || row.MAC != "02:aa:bb:cc:dd:ee" || row.Interface != "Wi-Fi" || row.State != "stale" || row.Family != "IPv6" {
		t.Fatalf("incorrect IPv6 row: %+v", row)
	}
	if rows[2].MAC != "" || rows[2].State != "incomplete" {
		t.Fatalf("unresolved MAC was invented: %+v", rows[2])
	}
	if _, err := parseWindowsNeighborTable(raw[:len(raw)-1], nil); err == nil {
		t.Fatal("accepted truncated native table")
	}
	binary.LittleEndian.PutUint32(first[72:76], 33)
	if _, err := parseWindowsNeighborTable(raw, nil); err == nil {
		t.Fatal("accepted out-of-bounds native MAC length")
	}
	binary.LittleEndian.PutUint32(raw[:4], maxNeighborEntries+1)
	if _, err := parseWindowsNeighborTable(raw, nil); err == nil {
		t.Fatal("accepted unbounded native table count")
	}
}

func linuxNeighborFixture(address string, mac []byte, index int, state uint16) []byte {
	ip := netip.MustParseAddr(address)
	row := make([]byte, 12)
	row[0] = 2
	if ip.Is6() {
		row[0] = 10
	}
	binary.NativeEndian.PutUint32(row[4:8], uint32(index))
	binary.NativeEndian.PutUint16(row[8:10], state)
	appendAttr := func(kind uint16, value []byte) {
		length := 4 + len(value)
		attr := make([]byte, (length+3)&^3)
		binary.NativeEndian.PutUint16(attr[:2], uint16(length))
		binary.NativeEndian.PutUint16(attr[2:4], kind)
		copy(attr[4:], value)
		row = append(row, attr...)
	}
	appendAttr(1, ip.AsSlice())
	if mac != nil {
		appendAttr(2, mac)
	}
	return row
}

func TestLinuxNeighborFixtures(t *testing.T) {
	mac := []byte{0, 0x11, 0x22, 0x33, 0x44, 0x55}
	for _, tc := range []struct {
		ip, wantAddress, family string
	}{{"192.0.2.1", "192.0.2.1", "IPv4"}, {"fe80::1", "fe80::1%7", "IPv6"}, {"2001:db8::1", "2001:db8::1", "IPv6"}} {
		row := linuxNeighborFixture(tc.ip, mac, 7, 4)
		row[10] = 0x80
		got, include, err := parseLinuxNeighborMessage(row, map[int]string{7: "eth7"})
		if err != nil || !include || got.Address != tc.wantAddress || got.Family != tc.family || got.Interface != "eth7" || got.InterfaceIndex != 7 || got.MAC != "00:11:22:33:44:55" || got.State != "stale" || !got.IsRouter {
			t.Fatalf("invalid %s fixture: %+v %t %v", tc.ip, got, include, err)
		}
	}
	row := linuxNeighborFixture("192.0.2.2", nil, 3, 1)
	got, include, err := parseLinuxNeighborMessage(row, nil)
	if err != nil || !include || got.MAC != "" || got.State != "incomplete" || got.Interface != "Interface 3" {
		t.Fatalf("invalid unresolved entry: %+v %t %v", got, include, err)
	}
	row[0] = 7 // AF_BRIDGE must not become an IP/device association.
	if _, include, err := parseLinuxNeighborMessage(row, nil); err != nil || include {
		t.Fatalf("bridge forwarding entry accepted: %t %v", include, err)
	}
	row[0] = 2
	binary.NativeEndian.PutUint16(row[12:14], 0xffff)
	if _, _, err := parseLinuxNeighborMessage(row, nil); err == nil {
		t.Fatal("accepted malformed neighbor attribute")
	}
}

func TestNeighborsCancelledBeforeRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Neighbors(ctx); err != context.Canceled {
		t.Fatalf("canceled neighbor read: %v", err)
	}
}
