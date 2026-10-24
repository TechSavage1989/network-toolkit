//go:build windows

package platform

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

var getIPNetTable2 = ipHelper.NewProc("GetIpNetTable2")
var freeMibTable = ipHelper.NewProc("FreeMibTable")

type mibIPNetRow2 struct {
	Address               [28]byte
	InterfaceIndex        uint32
	InterfaceLUID         uint64
	PhysicalAddress       [32]byte
	PhysicalAddressLength uint32
	State                 uint32
	Flags                 uint8
	_                     [3]byte
	ReachabilityTime      uint32
}

type mibIPNetTable2 struct {
	NumEntries uint32
	Table      [1]mibIPNetRow2
}

func readNeighbors(ctx context.Context) ([]Neighbor, error) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		return nil, errors.New("native neighbor inspection requires the 64-bit Windows build")
	}
	if err := getIPNetTable2.Find(); err != nil {
		return nil, fmt.Errorf("native neighbor table unavailable: %w", err)
	}
	if err := freeMibTable.Find(); err != nil {
		return nil, fmt.Errorf("native neighbor table cleanup unavailable: %w", err)
	}
	adapters, adapterErr := readWindowsAdapters()
	adapterContext := captureAdapterContext(adapters)
	names := adapterContext.interfaceNames()
	if adapterErr != nil || len(names) == 0 {
		// Adapter labels are optional; keep the cache usable if extra native
		// metadata could not be read, without attributing a description by guess.
		names = neighborInterfaceNames()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var table unsafe.Pointer
	// AF_UNSPEC reads both IPv4 ARP and IPv6 neighbor discovery cache entries.
	// This is an enumeration API; ResolveIpNetEntry2 is intentionally not used.
	status, _, _ := getIPNetTable2.Call(0, uintptr(unsafe.Pointer(&table)))
	if table != nil {
		defer freeMibTable.Call(uintptr(table))
	}
	if status == 1168 { // ERROR_NOT_FOUND: successful enumeration, empty table.
		return []Neighbor{}, nil
	}
	if status != 0 {
		return nil, fmt.Errorf("GetIpNetTable2: %w", syscall.Errno(status))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if table == nil {
		return nil, errors.New("native neighbor API returned no table")
	}
	count := *(*uint32)(table)
	if count > maxNeighborEntries {
		return nil, fmt.Errorf("neighbor table exceeds the %d-entry limit", maxNeighborEntries)
	}
	length := windowsNeighborTableOffset + int(count)*windowsNeighborRowSize
	rows, err := parseWindowsNeighborTable(unsafe.Slice((*byte)(table), length), names)
	adapterContext.annotateNeighbors(rows)
	runtime.KeepAlive(table)
	return rows, err
}
