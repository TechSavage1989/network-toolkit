package capture

import (
	"testing"
	"time"
)

func TestPacketRecordedSeverityFiltersAndTimelineAgree(t *testing.T) {
	stamp := time.Date(2026, 10, 26, 14, 30, 0, 123456789, time.UTC)
	cases := []struct {
		packet Packet
		level  string
	}{
		{Packet{Protocol: "TCP", Transport: "TCP"}, "normal"},
		{Packet{Protocol: "ICMP", Info: "Destination unreachable"}, "warn"},
		{Packet{Protocol: "ICMPv6", Info: "Administratively prohibited"}, "warn"},
		{Packet{Protocol: "TCP", Transport: "TCP", Info: "Out-of-order"}, "note"},
		{Packet{Protocol: "TCP", Transport: "TCP", TCPFlags: 4}, "note"},
		{Packet{Protocol: "DNS", Info: "NXDOMAIN"}, "note"},
		{Packet{Protocol: "DNS", Info: "SERVFAIL"}, "warn"},
		{Packet{Expert: []string{"incomplete IPv4 header"}}, "error"},
		{Packet{Expert: []string{"Windows drop reason 0x1"}}, "warn"},
		{Packet{Expert: []string{"Truncated payload"}}, "note"},
		{Packet{Highlight: "warn", Expert: []string{"Informational decoder detail"}}, "warn"},
	}
	packets := make([]Packet, len(cases))
	for i, tc := range cases {
		p := tc.packet
		p.Number = uint64(i + 1)
		p.Time = stamp
		p.Length = 100
		packets[i] = p
		if got := PacketSeverity(p); got != tc.level {
			t.Fatalf("case %d severity=%s want %s", i, got, tc.level)
		}
		for _, level := range []string{"normal", "error", "warn", "note"} {
			predicate, err := Compile(`frame.severity == "` + level + `" && frame.time_recorded && frame.len > 0`)
			if err != nil || predicate(p) != (level == tc.level) {
				t.Fatalf("case %d filter %s: %v", i, level, err)
			}
		}
	}
	stats := Stats(packets)
	if len(stats.Timeline) != 1 || stats.Timeline[0].Severities["normal"] != 1 || stats.Timeline[0].Severities["error"] != 1 || stats.Timeline[0].Severities["warn"] != 5 || stats.Timeline[0].Severities["note"] != 4 {
		t.Fatalf("recorded clue counts differ from filter semantics: %+v", stats.Timeline)
	}
	if !stats.Started.Equal(stamp) || stats.Bytes != uint64(len(packets)*100) {
		t.Fatal("severity analysis changed source timestamps or bytes")
	}
	timed, err := Compile("frame.time_recorded")
	if err != nil || timed(Packet{}) {
		t.Fatal("missing time became recorded")
	}
	for _, expression := range []string{`frame.severity == "success"`, `frame.severity contains "warn"`} {
		if _, err := Compile(expression); err == nil {
			t.Fatalf("unsupported severity filter accepted: %s", expression)
		}
	}
}

func TestTroubleshootingCounterFiltersUseAssessedRecordedContributors(t *testing.T) {
	query := troubleTestPacket(1, 0, troubleTestDNS("example.test", 1, 12, false, 0))
	reply := troubleTestPacket(2, 1, troubleTestDNS("example.test", 1, 12, true, 0))
	unsupported := query
	unsupported.Record.Data = append([]byte(nil), query.Record.Data...)
	unsupported.Record.Data[44] |= 0x08 // unsupported DNS opcode
	icmp := troubleTestPacket(4, 2, testFrame(1, []byte{3, 1, 0, 0, 0, 0, 0, 0}))
	echo := troubleTestPacket(5, 3, testFrame(1, []byte{8, 0, 0, 0, 0, 0, 0, 0}))
	packets := []Packet{query, reply, unsupported, icmp, echo}
	for _, tc := range []struct {
		expression string
		number     uint64
	}{{"dns.request", 1}, {"dns.response", 2}, {"icmp.error", 4}} {
		match, err := Compile(tc.expression)
		if err != nil {
			t.Fatal(err)
		}
		matches := 0
		for _, packet := range packets {
			if match(packet) {
				matches++
				if packet.Number != tc.number {
					t.Fatalf("%s selected unassessed packet %d", tc.expression, packet.Number)
				}
			}
		}
		if matches != 1 {
			t.Fatalf("%s matched %d packets", tc.expression, matches)
		}
	}
	report := Troubleshoot(packets, TroubleshootContext{})
	if report.Metrics.DNSQueries != 1 || report.Metrics.DNSResponses != 1 || report.Metrics.ICMPErrors != 1 {
		t.Fatalf("counter predicates differ from report semantics: %+v", report.Metrics)
	}
}
