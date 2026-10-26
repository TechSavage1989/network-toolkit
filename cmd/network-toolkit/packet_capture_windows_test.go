//go:build windows && amd64

package main

import (
	"encoding/binary"
	"runtime"
	"testing"
	"time"
	"unsafe"
)

// These paired array bounds verify the documented x64 C layout at compile
// time, including when Windows execution is unavailable to the build host.
var _ [32 - int(unsafe.Sizeof(packetStreamConfiguration{}))]byte
var _ [int(unsafe.Sizeof(packetStreamConfiguration{})) - 32]byte
var _ [32 - int(unsafe.Sizeof(packetStreamDescriptor{}))]byte
var _ [int(unsafe.Sizeof(packetStreamDescriptor{})) - 32]byte
var _ [48 - int(unsafe.Sizeof(packetStreamMetadata{}))]byte
var _ [int(unsafe.Sizeof(packetStreamMetadata{})) - 48]byte
var _ [40 - int(unsafe.Offsetof(packetStreamMetadata{}.Timestamp))]byte
var _ [int(unsafe.Offsetof(packetStreamMetadata{}.Timestamp)) - 40]byte
var _ [24 - int(unsafe.Offsetof(packetStreamMetadata{}.DropReason))]byte
var _ [int(unsafe.Offsetof(packetStreamMetadata{}.DropReason)) - 24]byte
var _ [16 - int(unsafe.Offsetof(packetStreamConfiguration{}.Data))]byte
var _ [int(unsafe.Offsetof(packetStreamConfiguration{}.Data)) - 16]byte

func TestPacketNativeDescriptorBoundsCopyTimestampAndDirection(t *testing.T) {
	raw := make([]byte, 48+54)
	meta := (*packetStreamMetadata)(unsafe.Pointer(&raw[0]))
	meta.Timestamp = 116444736000000000 + 1780000000*10000000 + 1234567
	meta.PacketType = 1
	meta.Direction = 4
	meta.Component = 7
	copy(raw[48:], captureFixtureRecord(80).Data)
	d := packetStreamDescriptor{Data: uintptr(unsafe.Pointer(&raw[0])), Size: uint32(len(raw)), PacketOffset: 48, PacketLength: 54, MissedReads: 2, MissedWrites: 3}
	r, lost, e := packetNativeRecord(&d)
	if e != nil || lost != 5 || r.Direction != "out" || r.LinkType != 1 || r.Time != time.Unix(1780000000, 123456700).UTC() {
		t.Fatal(r, lost, e)
	}
	raw[48] = 255
	if r.Data[0] == 255 {
		t.Fatal("callback did not copy transient OS bytes")
	}
	d.PacketLength = 9001
	if _, _, e = packetNativeRecord(&d); e == nil {
		t.Fatal("invalid descriptor accepted")
	}
	runtime.KeepAlive(raw)
}
func TestPacketNativeSourceNameBounds(t *testing.T) {
	b := make([]byte, 20)
	binary.LittleEndian.PutUint16(b[4:], 'N')
	binary.LittleEndian.PutUint16(b[6:], 'I')
	binary.LittleEndian.PutUint16(b[8:], 'C')
	base := uintptr(unsafe.Pointer(&b[0]))
	name, e := packetSourceName(b, base)
	if e != nil || name != "NIC" {
		t.Fatal(name, e)
	}
	if _, e = packetSourceName(b, base+1000); e == nil {
		t.Fatal("out of bounds adapter pointer accepted")
	}
	runtime.KeepAlive(b)
}
