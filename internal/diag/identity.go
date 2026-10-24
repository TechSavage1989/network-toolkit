package diag

import (
	"context"
	"net"
	"net/netip"
	"strings"
	"time"

	"networktoolkit/internal/platform"
)

// HostIdentity describes an exact-IP cache observation. It is separate from
// device-reported fields so refreshing a MAC assignment cannot erase a model.
type HostIdentity struct {
	MAC          string `json:"mac,omitempty"`
	Vendor       string `json:"vendor,omitempty"`
	VendorSource string `json:"vendorSource,omitempty"`
	MACKind      string `json:"macKind,omitempty"`
	IdentityNote string `json:"identityNote,omitempty"`
}

type neighborReader func(context.Context) ([]platform.Neighbor, error)

func readNeighborIdentities(ctx context.Context, read neighborReader) (map[string]HostIdentity, error) {
	rows, err := read(ctx)
	if binding, selected := platform.SourceBindingFromContext(ctx); selected {
		filtered := make([]platform.Neighbor, 0, len(rows))
		for _, row := range rows {
			address, parseErr := netip.ParseAddr(row.Address)
			if parseErr != nil || (binding.Family == "ipv4") != address.Unmap().Is4() {
				continue
			}
			address = address.Unmap()
			index := binding.InterfaceIndex
			if address.Is6() && binding.IPv6InterfaceIndex != 0 {
				index = binding.IPv6InterfaceIndex
			}
			// Unknown interface association cannot prove that an entry belongs
			// to the selected path, so it is omitted instead of guessed by IP.
			if row.InterfaceIndex == index {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	}
	return neighborIdentities(rows), err
}

func refreshNeighborIdentities(ctx context.Context, read neighborReader) (map[string]HostIdentity, error) {
	// This is one bounded, read-only OS cache snapshot, not a new network probe.
	// It can enrich already collected partial results after user cancellation.
	refreshCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	return readNeighborIdentities(refreshCtx, read)
}

func neighborIdentities(rows []platform.Neighbor) map[string]HostIdentity {
	identities := make(map[string]HostIdentity)
	conflicts := make(map[string]bool)
	for _, row := range rows {
		address, err := netip.ParseAddr(row.Address)
		if err != nil {
			continue
		}
		mac, err := net.ParseMAC(row.MAC)
		if err != nil || len(mac) != 6 || mac[0]&1 != 0 {
			continue
		}
		var nonzero byte
		for _, b := range mac {
			nonzero |= b
		}
		if nonzero == 0 {
			continue
		}
		key := address.Unmap().String()
		if conflicts[key] {
			continue
		}
		observed := HostIdentity{MAC: mac.String(), Vendor: row.Vendor, VendorSource: row.VendorSource, MACKind: row.MACKind}
		if previous, exists := identities[key]; exists {
			if previous.MAC != observed.MAC {
				conflicts[key] = true
				identities[key] = HostIdentity{IdentityNote: "Conflicting neighbor MAC entries exist for this IP; MAC and vendor are omitted. Review the ARP / neighbors table for their interfaces."}
			} else if previous.Vendor == "" && observed.Vendor != "" {
				identities[key] = observed
			}
			continue
		}
		identities[key] = observed
	}
	return identities
}

func joinIdentityNote(first, second string) string {
	if first == "" {
		return second
	}
	if second == "" {
		return first
	}
	return strings.TrimSpace(first) + "; " + second
}
