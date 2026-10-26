package main

import (
	"strings"
	"testing"
	"time"

	"networktoolkit/internal/capture"
)

func TestPacketOverviewSignalsAndBins(t *testing.T) {
	packets := []capture.Packet{
		{Number: 1, Protocol: "TCP", Transport: "TCP", Info: "SYN"},
		{Number: 2, Protocol: "TCP", Transport: "TCP", TCPFlags: 4, Info: "RST"},
		{Number: 3, Protocol: "DNS", Info: "Standard query response SERVFAIL"},
		{Number: 4, Protocol: "UDP", Expert: []string{"Packet truncated by capture length"}},
		{Number: 5, Protocol: "ARP"},
	}
	overview := packetOverviewFor(packets)
	if len(overview.Bins) != len(packets) || overview.Interesting != 3 || overview.Counts["warn"] != 1 || overview.Counts["note"] != 2 {
		t.Fatalf("unexpected overview: %+v", overview)
	}
	if len(overview.Signals) != 3 || overview.Signals[0].ID != "dns-failure" || overview.Signals[0].Index != 2 || overview.Signals[0].Number != 3 {
		t.Fatalf("unexpected signal ordering/position: %+v", overview.Signals)
	}
	if overview.Bins[2].Level != "warn" || overview.Bins[2].Protocol != "DNS" {
		t.Fatalf("map bins did not preserve packet order: %+v", overview.Bins)
	}
	if packetOverviewFor(nil).Interesting != 0 || len(packetOverviewFor(nil).Signals) != 0 {
		t.Fatal("empty overview should not invent events")
	}
}
func TestFrameMetadataLayerPreservesByteBounds(t *testing.T) {
	tm := time.Date(2026, 10, 23, 12, 30, 40, 123456789, time.UTC)
	prev := capture.Packet{Time: tm.Add(-25 * time.Millisecond)}
	p := capture.Packet{Number: 7, Time: tm, Relative: 1.25, Length: 234, Captured: 220, Interface: "Ethernet", Record: capture.Record{LinkType: 1}, Layers: []capture.Layer{{Name: "Ethernet"}, {Name: "Internet Protocol v4"}}}
	l := frameCaptureLayer(p, &prev)
	if l.Offset != 0 || l.Length != 220 || !strings.Contains(l.Name, "234 bytes on wire") || len(l.Fields) < 8 {
		t.Fatalf("unexpected frame metadata: %+v", l)
	}
	got := map[string]string{}
	for _, f := range l.Fields {
		got[f.Name] = f.Value
		if f.Offset != 0 || f.Length != 0 {
			t.Fatalf("metadata is not a byte range: %+v", f)
		}
	}
	for _, name := range []string{"Arrival time (local)", "Arrival time (UTC)", "Epoch time", "Time since first captured frame", "Time since previous captured frame", "Frame number", "Frame length", "Captured length", "Interface", "Encapsulation", "Protocols in frame"} {
		if got[name] == "" {
			t.Errorf("missing %s", name)
		}
	}
	if got["Time since previous captured frame"] != "0.025000000 seconds" {
		t.Fatal(got["Time since previous captured frame"])
	}
	if got["Protocols in frame"] != "Ethernet / Internet Protocol v4" {
		t.Fatal(got["Protocols in frame"])
	}
}

func TestPacketOverviewMarksPreserveEveryPacketAndProtocolSeverity(t *testing.T) {
	packets := []capture.Packet{
		{Number: 10, Protocol: "TCP", Transport: "TCP", Info: "SYN"},
		{Number: 12, Protocol: "DNS", Info: "Standard query response SERVFAIL"},
		{Number: 14, Protocol: "UDP", Expert: []string{"Packet truncated by capture length"}},
		{Number: 17, Protocol: "ARP"},
		{Number: 22, Protocol: "WireGuard", Expert: []string{"invalid payload"}},
	}
	value := packetOverviewFor(packets)
	if len(value.PacketMarks) != len(packets) {
		t.Fatalf("packet-level map lost frames: %d vs %d", len(value.PacketMarks), len(packets))
	}
	want := string([]byte{
		packetMapCode("TCP", ""),
		packetMapCode("DNS", "warn"),
		packetMapCode("UDP", "note"),
		packetMapCode("ARP", ""),
		packetMapCode("WireGuard", "error"),
	})
	if value.PacketMarks != want {
		t.Fatalf("wrong packet-level map %q expected %q", value.PacketMarks, want)
	}
	big := make([]capture.Packet, 50_000)
	for i := range big {
		big[i].Protocol = "TCP"
		if i%123 == 0 {
			big[i].Protocol = "DNS"
			big[i].Info = "SERVFAIL"
		}
	}
	marks := packetOverviewFor(big).PacketMarks
	if len(marks) != len(big) {
		t.Fatalf("large packet-level map has %d of %d packets", len(marks), len(big))
	}
	if marks[123] != packetMapCode("DNS", "warn") {
		t.Fatal("a warning at position 123 was not retained")
	}
	if packetOverviewFor(nil).PacketMarks != "" {
		t.Fatal("empty list should not have markers")
	}
}
