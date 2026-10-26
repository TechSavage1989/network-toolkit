//go:build windows

package main

import (
	"testing"
	"unsafe"
)

// Cross-compilation verifies these fixed Windows WLAN ABI layouts. This test
// file deliberately calls no Windows WLAN API and never reads live keys.
var (
	_ [16 - int(unsafe.Sizeof(localWLANGUID{}))]byte
	_ [int(unsafe.Sizeof(localWLANGUID{})) - 16]byte
	_ [532 - int(unsafe.Sizeof(localWLANInterfaceInfo{}))]byte
	_ [int(unsafe.Sizeof(localWLANInterfaceInfo{})) - 532]byte
	_ [8 - int(unsafe.Offsetof(localWLANInterfaceList{}.Items))]byte
	_ [int(unsafe.Offsetof(localWLANInterfaceList{}.Items)) - 8]byte
	_ [516 - int(unsafe.Sizeof(localWLANProfileInfo{}))]byte
	_ [int(unsafe.Sizeof(localWLANProfileInfo{})) - 516]byte
	_ [8 - int(unsafe.Offsetof(localWLANProfileList{}.Items))]byte
	_ [int(unsafe.Offsetof(localWLANProfileList{}.Items)) - 8]byte
	_ [68 - int(unsafe.Sizeof(localWLANAssociation{}))]byte
	_ [int(unsafe.Sizeof(localWLANAssociation{})) - 68]byte
	_ [56 - int(unsafe.Offsetof(localWLANAssociation{}.SignalQuality))]byte
	_ [int(unsafe.Offsetof(localWLANAssociation{}.SignalQuality)) - 56]byte
	_ [604 - int(unsafe.Sizeof(localWLANConnection{}))]byte
	_ [int(unsafe.Sizeof(localWLANConnection{})) - 604]byte
	_ [520 - int(unsafe.Offsetof(localWLANConnection{}.Association))]byte
	_ [int(unsafe.Offsetof(localWLANConnection{}.Association)) - 520]byte
	_ [588 - int(unsafe.Offsetof(localWLANConnection{}.Security))]byte
	_ [int(unsafe.Offsetof(localWLANConnection{}.Security)) - 588]byte
)

func TestLocalWiFiNativeGUIDRoundTrip(t *testing.T) {
	id := "{12345678-90AB-CDEF-1122-334455667788}"
	guid, err := localWLANGUIDFromString(id)
	if err != nil || guid.String() != id || guid.Data1 != 0x12345678 || guid.Data2 != 0x90AB || guid.Data3 != 0xCDEF || guid.Data4[0] != 0x11 || guid.Data4[7] != 0x88 {
		t.Fatal("Windows GUID byte order changed")
	}
}
