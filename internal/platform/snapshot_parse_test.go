package platform

import "testing"

func TestWindowsRoutesAndARPParsing(t *testing.T) {
	routes := parseWindowsRoutes("IPv4 Route Table\nNetwork Destination Netmask Gateway Interface Metric\n 0.0.0.0 0.0.0.0 192.0.2.1 192.0.2.10 25\n 192.0.2.0 255.255.255.0 On-link 192.0.2.10 281\n 192.0.3.0 255.255.255.0 Sur le lien 192.0.3.10 50\nPersistent Routes:\n192.0.4.0 255.255.255.0 192.0.2.1 1\nIPv6 Route Table\n 12 25 ::/0 fe80::1\n 1 331 ::1/128 On-link\n")
	if len(routes) != 5 {
		t.Fatalf("expected five active routes, got %v", routes)
	}
	if routes[0]["gateway"] != "192.0.2.1" || routes[0]["onLink"] != false {
		t.Fatalf("invalid gateway route %v", routes[0])
	}
	if routes[2]["onLink"] != true || routes[2]["interface"] != "192.0.3.10" {
		t.Fatalf("localized on-link route not parsed: %v", routes[2])
	}
	if routes[3]["family"] != "IPv6" || routes[3]["interface"] != 12 || routes[3]["gateway"] != "fe80::1" {
		t.Fatalf("invalid IPv6 route %v", routes[3])
	}
	neighbors := parseWindowsARP("Interface: 192.0.2.10 --- 0xc\n Internet Address Physical Address Type\n 192.0.2.1 00-11-22-33-44-55 dynamic\nInterface: 192.0.3.10 --- 0xe\n 192.0.3.1 00-aa-bb-cc-dd-ee dynamisch\n")
	if len(neighbors) != 2 || neighbors[0]["mac"] != "00:11:22:33:44:55" || neighbors[1]["interface"] != "192.0.3.10" {
		t.Fatalf("invalid ARP rows: %v", neighbors)
	}
}
