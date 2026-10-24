package platform

import (
	"net"
	"strconv"
	"strings"
)

// Numeric columns remain stable across display languages. Headers and localized
// state labels are preserved only in the accompanying full command output.
func parseWindowsRoutes(output string) []map[string]any {
	rows := []map[string]any{}
	for _, line := range strings.Split(output, "\n") {
		f := strings.Fields(line)
		if len(f) >= 5 && net.ParseIP(f[0]) != nil && net.ParseIP(f[0]).To4() != nil && net.ParseIP(f[1]) != nil && net.ParseIP(f[len(f)-2]) != nil {
			metric, err := strconv.Atoi(f[len(f)-1])
			if err != nil {
				continue
			}
			gateway := strings.Join(f[2:len(f)-2], " ")
			rows = append(rows, map[string]any{"family": "IPv4", "destination": f[0], "mask": f[1], "gateway": gateway, "interface": f[len(f)-2], "metric": metric, "onLink": net.ParseIP(gateway) == nil})
			continue
		}
		if len(f) >= 4 && strings.Contains(f[2], ":") {
			index, indexErr := strconv.Atoi(f[0])
			metric, metricErr := strconv.Atoi(f[1])
			_, prefix, prefixErr := net.ParseCIDR(f[2])
			if indexErr != nil || metricErr != nil || prefixErr != nil || prefix.IP.To4() != nil {
				continue
			}
			gateway := strings.Join(f[3:], " ")
			gatewayIP := gateway
			if i := strings.LastIndex(gatewayIP, "%"); i > 0 {
				gatewayIP = gatewayIP[:i]
			}
			rows = append(rows, map[string]any{"family": "IPv6", "destination": f[2], "gateway": gateway, "interface": index, "metric": metric, "onLink": net.ParseIP(gatewayIP) == nil})
		}
	}
	return rows
}

func parseWindowsARP(output string) []map[string]any {
	rows := []map[string]any{}
	iface := ""
	for _, line := range strings.Split(output, "\n") {
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		if net.ParseIP(f[0]) == nil {
			// ARP interface headers contain the interface IP and a hex index.
			if strings.Contains(strings.ToLower(line), "0x") {
				for _, value := range f {
					if ip := net.ParseIP(value); ip != nil && ip.To4() != nil {
						iface = value
						break
					}
				}
			}
			continue
		}
		ip := net.ParseIP(f[0])
		if ip.To4() == nil {
			continue
		}
		mac, err := net.ParseMAC(f[1])
		if err != nil {
			continue
		}
		rows = append(rows, map[string]any{"address": ip.String(), "mac": mac.String(), "interface": iface, "state": strings.Join(f[2:], " ")})
	}
	return rows
}
