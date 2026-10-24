package platform

import (
	"net/netip"
	"strings"
)

// Keep native adapter metadata translation independent of OS calls so source
// identity, DNS, address-state, and separate-family-index fixtures run anywhere.
func sourceInterfacesFromRows(rows []map[string]any) []sourceInterface {
	interfaces := make([]sourceInterface, 0, len(rows))
	for _, row := range rows {
		nic := sourceInterface{addressStates: map[string]string{}}
		nic.index, _ = row["index"].(int)
		nic.ipv6Index, _ = row["ipv6Index"].(int)
		nic.name, _ = row["name"].(string)
		nic.description, _ = row["description"].(string)
		nic.id, _ = row["id"].(string)
		nic.up, _ = row["up"].(bool)
		nic.addresses, _ = row["addresses"].([]string)
		nic.dns, _ = row["dns"].([]string)
		if details, ok := row["addressDetails"].([]map[string]any); ok {
			for _, detail := range details {
				address, _ := detail["address"].(string)
				if ip, err := netip.ParseAddr(strings.SplitN(address, "/", 2)[0]); err == nil {
					nic.addressStates[ip.Unmap().WithZone("").String()], _ = detail["state"].(string)
				}
			}
		}
		interfaces = append(interfaces, nic)
	}
	return interfaces
}
