//go:build linux

package platform

import (
	"context"
	"net/netip"
	"testing"
	"time"
)

func TestLinuxNeighborCacheRead(t *testing.T) {
	// This reads the local kernel cache only. It does not contact the entries,
	// resolve neighbors, or require any device to be present in the table.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rows, err := Neighbors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) > maxNeighborEntries {
		t.Fatalf("unbounded table size: %d", len(rows))
	}
	for _, row := range rows {
		address, err := netip.ParseAddr(row.Address)
		if err != nil || row.InterfaceIndex <= 0 || row.Interface == "" || row.State == "" {
			t.Fatalf("invalid kernel neighbor entry: %+v", row)
		}
		if address.Is4() && row.Family != "IPv4" || address.Is6() && row.Family != "IPv6" {
			t.Fatalf("wrong kernel neighbor address family: %+v", row)
		}
	}
}
