//go:build !windows

package platform

import (
	"context"
	"fmt"
	"net"
)

func readSourceInterfaces(ctx context.Context) ([]sourceInterface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	rows := make([]sourceInterface, 0, len(interfaces))
	for _, nic := range interfaces {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		addresses, err := nic.Addrs()
		if err != nil {
			return nil, fmt.Errorf("read addresses for %q: %w", nic.Name, err)
		}
		row := sourceInterface{index: nic.Index, ipv6Index: nic.Index, name: nic.Name, up: nic.Flags&net.FlagUp != 0}
		for _, address := range addresses {
			row.addresses = append(row.addresses, address.String())
		}
		// /etc/resolv.conf does not identify a DNS server's owning adapter.
		// Leave adapter DNS unknown instead of silently using the system route.
		rows = append(rows, row)
	}
	return rows, nil
}
