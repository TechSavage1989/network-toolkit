package diag

import (
	"encoding/binary"
	"testing"
)

func TestDNSWireTTLIsPreserved(t *testing.T) {
	id := uint16(0x4321)
	response, err := dnsQuestion("example.test", 1, id)
	if err != nil {
		t.Fatal(err)
	}
	response = append([]byte(nil), response...)
	binary.BigEndian.PutUint16(response[2:], 0x8180) // recursive, valid DNS response
	binary.BigEndian.PutUint16(response[6:], 1)
	// Pointer to queried owner, A, IN, TTL 3600, four address bytes.
	response = append(response, 0xC0, 0x0C, 0, 1, 0, 1, 0, 0, 0x0E, 0x10, 0, 4, 207, 38, 77, 175)
	rows, code, err := parseDNS(response, id, "example.test", 1)
	if err != nil {
		t.Fatal(err)
	}
	if code != "NOERROR" || len(rows) != 1 || rows[0].TTL == nil || *rows[0].TTL != 3600 || rows[0].Value != "207.38.77.175" {
		t.Fatalf("lost DNS wire TTL or record: code=%s records=%+v", code, rows)
	}
}

func TestDNSCheckerStyleRecordMetadata(t *testing.T) {
	tests := []struct{ kind, value, key, want string }{
		{"MX", "10 d367636a.ess.barracudanetworks.com.", "preference", "10"},
		{"MX", "20 d367636b.ess.barracudanetworks.com.", "mailExchanger", "d367636b.ess.barracudanetworks.com."},
		{"SOA", "ns-1386.awsdns-45.org. dnsops.integrityschool.com. 1 7200 900 1209600 86400", "serial", "1"},
		{"SOA", "ns-1386.awsdns-45.org. dnsops.integrityschool.com. 1 7200 900 1209600 86400", "expireSeconds", "1209600"},
		{"TXT", "v=spf1 include:spf.protection.outlook.com ~all", "purpose", "SPF"},
	}
	for _, tt := range tests {
		t.Run(tt.kind+tt.key, func(t *testing.T) {
			got := dnsRecordDetails(DNSRecord{Type: tt.kind, Value: tt.value})[tt.key]
			if got != tt.want {
				t.Errorf("%s=%v; want %s", tt.key, got, tt.want)
			}
		})
	}
}
