package platform

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sort"

	"networktoolkit/internal/identity"
)

// Neighbor is an observed entry in the operating system's local neighbor cache.
// It is not a liveness assertion. The same address can occur on more than one
// interface, so consumers must retain interface association when MACs conflict.
type Neighbor struct {
	Address              string `json:"address"`
	MAC                  string `json:"mac"`
	Interface            string `json:"interface"`
	InterfaceIndex       int    `json:"interfaceIndex"`
	InterfaceDescription string `json:"interfaceDescription,omitempty"`
	State                string `json:"state"`
	Family               string `json:"family"`
	Vendor               string `json:"vendor,omitempty"`
	VendorSource         string `json:"vendorSource,omitempty"`
	MACKind              string `json:"macKind,omitempty"`
	IsRouter             bool   `json:"isRouter,omitempty"`
}

const maxNeighborEntries = 65536

const neighborCacheNote = "Entries come from this computer's IPv4 and IPv6 neighbor caches and may be stale. A MAC vendor identifies the IEEE address assignment holder, which may be the network adapter maker. It does not identify the device model. Locally administered addresses do not establish a manufacturer."

// Neighbors reads the local cache without sending ARP, neighbor discovery, or
// remote network probes. Vendor lookups use the currently loaded local database.
// A non-nil error may accompany a partial table; callers should preserve it.
func Neighbors(ctx context.Context) ([]Neighbor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rows, err := readNeighbors(ctx)
	if rows == nil {
		rows = []Neighbor{}
	}
	for i := range rows {
		if contextErr := ctx.Err(); contextErr != nil {
			return rows[:i], contextErr
		}
		info := identity.LookupMAC(rows[i].MAC)
		rows[i].Vendor = info.Vendor
		rows[i].VendorSource = info.VendorSource
		rows[i].MACKind = info.MACKind
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Address != b.Address {
			ap, ae := netip.ParseAddr(a.Address)
			bp, be := netip.ParseAddr(b.Address)
			if ae == nil && be == nil {
				return ap.Less(bp)
			}
			return a.Address < b.Address
		}
		if a.InterfaceIndex != b.InterfaceIndex {
			return a.InterfaceIndex < b.InterfaceIndex
		}
		return a.MAC < b.MAC
	})
	return rows, err
}

func neighborInterfaceNames() map[int]string {
	names := map[int]string{}
	interfaces, _ := net.Interfaces()
	for _, nic := range interfaces {
		names[nic.Index] = nic.Name
	}
	return names
}

func neighborInterfaceName(index int, names map[int]string) string {
	if name := names[index]; name != "" {
		return name
	}
	return fmt.Sprintf("Interface %d", index)
}

func neighborMAC(raw []byte) string {
	if len(raw) == 0 || len(raw) > 32 {
		return ""
	}
	for _, b := range raw {
		if b != 0 {
			return net.HardwareAddr(raw).String()
		}
	}
	// Unresolved table entries commonly carry an all-zero address. Keep the
	// absence explicit rather than attributing a made-up vendor to that value.
	return ""
}
