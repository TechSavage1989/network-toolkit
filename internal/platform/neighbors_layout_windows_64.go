//go:build windows && (amd64 || arm64)

package platform

import "unsafe"

// Microsoft SDK MIB_IPNET_ROW2 contains an eight-byte aligned NET_LUID and a
// 32-byte physical address. These checks also run during cross compilation.
var (
	_ [int(unsafe.Offsetof(mibIPNetRow2{}.InterfaceIndex)) - 28]byte
	_ [28 - int(unsafe.Offsetof(mibIPNetRow2{}.InterfaceIndex))]byte
	_ [int(unsafe.Offsetof(mibIPNetRow2{}.InterfaceLUID)) - 32]byte
	_ [32 - int(unsafe.Offsetof(mibIPNetRow2{}.InterfaceLUID))]byte
	_ [int(unsafe.Offsetof(mibIPNetRow2{}.PhysicalAddress)) - 40]byte
	_ [40 - int(unsafe.Offsetof(mibIPNetRow2{}.PhysicalAddress))]byte
	_ [int(unsafe.Offsetof(mibIPNetRow2{}.PhysicalAddressLength)) - 72]byte
	_ [72 - int(unsafe.Offsetof(mibIPNetRow2{}.PhysicalAddressLength))]byte
	_ [int(unsafe.Offsetof(mibIPNetRow2{}.State)) - 76]byte
	_ [76 - int(unsafe.Offsetof(mibIPNetRow2{}.State))]byte
	_ [int(unsafe.Offsetof(mibIPNetRow2{}.Flags)) - 80]byte
	_ [80 - int(unsafe.Offsetof(mibIPNetRow2{}.Flags))]byte
	_ [int(unsafe.Offsetof(mibIPNetRow2{}.ReachabilityTime)) - 84]byte
	_ [84 - int(unsafe.Offsetof(mibIPNetRow2{}.ReachabilityTime))]byte
	_ [int(unsafe.Sizeof(mibIPNetRow2{})) - windowsNeighborRowSize]byte
	_ [windowsNeighborRowSize - int(unsafe.Sizeof(mibIPNetRow2{}))]byte
	_ [int(unsafe.Offsetof(mibIPNetTable2{}.Table)) - windowsNeighborTableOffset]byte
	_ [windowsNeighborTableOffset - int(unsafe.Offsetof(mibIPNetTable2{}.Table))]byte
)
