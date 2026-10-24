//go:build windows && amd64

package main

import (
	"testing"
	"unsafe"
)

// Compile-time ABI checks run during cross-compilation as well. They exercise
// no Windows API and cannot change this computer's network configuration.
var (
	_ [280 - int(unsafe.Sizeof(localNativeAdapter{}))]byte
	_ [int(unsafe.Sizeof(localNativeAdapter{})) - 280]byte
	_ [272 - int(unsafe.Offsetof(localNativeAdapter{}.TunnelType))]byte
	_ [int(unsafe.Offsetof(localNativeAdapter{}.TunnelType)) - 272]byte
	_ [208 - int(unsafe.Offsetof(localNativeAdapter{}.Gateway))]byte
	_ [int(unsafe.Offsetof(localNativeAdapter{}.Gateway)) - 208]byte
	_ [216 - int(unsafe.Offsetof(localNativeAdapter{}.IPv4Metric))]byte
	_ [int(unsafe.Offsetof(localNativeAdapter{}.IPv4Metric)) - 216]byte
	_ [56 - int(unsafe.Offsetof(localNativeUnicast{}.PrefixLength))]byte
	_ [int(unsafe.Offsetof(localNativeUnicast{}.PrefixLength)) - 56]byte
	_ [104 - int(unsafe.Sizeof(localRouteRow{}))]byte
	_ [int(unsafe.Sizeof(localRouteRow{})) - 104]byte
	_ [84 - int(unsafe.Offsetof(localRouteRow{}.Metric))]byte
	_ [int(unsafe.Offsetof(localRouteRow{}.Metric)) - 84]byte
	_ [8 - int(unsafe.Offsetof(localRouteTable{}.Rows))]byte
	_ [int(unsafe.Offsetof(localRouteTable{}.Rows)) - 8]byte
)

func TestLocalWindowsNativeLayouts(t *testing.T) {
	if unsafe.Sizeof(localSockaddrInet{}) != 28 || unsafe.Sizeof(localIPPrefix{}) != 32 || unsafe.Offsetof(localRouteRow{}.NextHop) != 44 {
		t.Fatal("Windows route/address structure layout is incompatible")
	}
}
