//go:build !windows

package platform

import "testing"

func TestLinuxRouteDecode(t *testing.T) {
	text := "Iface Destination Gateway Flags RefCnt Use Metric Mask MTU Window IRTT\neth0 00000000 010011AC 0003 0 0 100 00000000 0 0 0\neth0 000011AC 00000000 0001 0 0 100 0000FFFF 0 0 0\n"
	rows := parseLinuxRoutes(text)
	if len(rows) != 2 || rows[0]["gateway"] != "172.17.0.1" || rows[1]["destination"] != "172.17.0.0" || rows[1]["mask"] != "255.255.0.0" {
		t.Fatalf("invalid decoded routes: %v", rows)
	}
}
