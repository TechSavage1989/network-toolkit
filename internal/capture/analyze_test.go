package capture

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTCPFollowOrderingOverlapGapsAndStatistics(t *testing.T) {
	var packets []Packet
	for _, v := range []struct {
		seq  uint32
		text string
	}{{103, "DEF"}, {100, "ABC"}, {100, "ABC"}, {105, "FGH"}, {111, "K"}} {
		packets = append(packets, Decode(testTCP(v.seq, 0x18, v.text)))
	}
	packets = append(packets, Decode(testTCP(112, 4, "")))
	id := packets[0].Stream
	s := Follow(packets, id)
	if len(s.Directions) != 1 || s.Directions[0].Text != "ABCDEFGH\n[Missing 3 captured bytes]\nK" || s.Directions[0].Gaps != 1 {
		t.Fatalf("bad stream: %+v", s)
	}
	stats := Stats(packets)
	if len(stats.Conversations) != 1 || stats.Conversations[0].Retransmissions != 1 || stats.Conversations[0].Resets != 1 || len(stats.Endpoints) != 2 {
		t.Fatal(stats)
	}
	if !strings.Contains(s.Note, "not decrypted") {
		t.Fatal("missing encrypted payload context")
	}
}

func TestStatisticsTimelineUsesObservedTimesAndAccountsForUntimedPackets(t *testing.T) {
	start := time.Date(2026, 10, 26, 9, 30, 0, 123456789, time.FixedZone("fixture", -5*3600))
	packet := func(stamp time.Time, length int) Packet {
		return Packet{Time: stamp, Length: uint32(length), Source: "192.0.2.1", Destination: "198.51.100.2", SourcePort: 1234, DestinationPort: 443, Protocol: "TCP", Transport: "TCP", Stream: "fixture"}
	}
	stats := Stats([]Packet{packet(start.Add(2200*time.Millisecond), 300), packet(time.Time{}, 999), packet(start, 100), packet(start.Add(time.Second), 200)})
	if stats.Started == nil || stats.Finished == nil || !stats.Started.Equal(start) || !stats.Finished.Equal(start.Add(2200*time.Millisecond)) {
		t.Fatalf("timestamps must describe the observed packets, including unordered input: %+v", stats)
	}
	if stats.Duration != 2.2 || stats.TimedPackets != 3 || stats.UntimedPackets != 1 || stats.TimedBytes != 600 || stats.Bytes != 1599 || len(stats.Timeline) != 3 {
		t.Fatalf("unexpected timeline scope: %+v", stats)
	}
	for i, bucket := range stats.Timeline {
		if bucket.Packets != 1 || bucket.Time == nil || bucket.EndTime == nil || !bucket.Time.Equal(start.Add(time.Duration(i)*time.Second)) || bucket.EndTime.After(*stats.Finished) {
			t.Fatalf("bucket must use its actual observed interval: %+v", bucket)
		}
	}
	if stats.Timeline[2].IntervalSeconds != .2 || !stats.Timeline[2].EndTime.Equal(*stats.Finished) {
		t.Fatalf("final partial interval was extended or lost: %+v", stats.Timeline[2])
	}
	for _, endpoint := range stats.Endpoints {
		if endpoint.Start == nil || endpoint.End == nil || !endpoint.Start.Equal(start) || !endpoint.End.Equal(*stats.Finished) {
			t.Fatalf("endpoint has incorrect observed times: %+v", endpoint)
		}
	}
	if len(stats.Conversations) != 1 || !stats.Conversations[0].Start.Equal(start) || stats.Conversations[0].Duration != 2.2 || stats.Conversations[0].End == nil || !stats.Conversations[0].End.Equal(*stats.Finished) {
		t.Fatalf("conversation times depend on packet order: %+v", stats.Conversations)
	}
	encoded, err := json.Marshal(stats)
	if err != nil || !strings.Contains(string(encoded), "2026-10-26T14:30:00.123456789Z") {
		t.Fatalf("UTC nanosecond timestamp was not preserved: %s, %v", encoded, err)
	}
}

func TestStatisticsDoesNotFabricateTimesOrIntervals(t *testing.T) {
	unknown := Stats([]Packet{{Length: 120, Protocol: "UDP"}})
	if unknown.Started != nil || unknown.Finished != nil || len(unknown.Timeline) != 0 || unknown.TimedPackets != 0 || unknown.UntimedPackets != 1 || unknown.Duration != 0 {
		t.Fatalf("untimed packets must not acquire a fabricated timeline: %+v", unknown)
	}
	stamp := time.Unix(0, 999).UTC()
	same := Stats([]Packet{{Time: stamp, Length: 120}, {Time: stamp, Length: 80}})
	if len(same.Timeline) != 1 || same.Duration != 0 || same.Timeline[0].IntervalSeconds != 0 || same.Timeline[0].Packets != 2 || same.Started == nil || !same.Started.Equal(stamp) {
		t.Fatalf("a single instant must retain its count without inventing a rate interval: %+v", same)
	}
	long := Stats([]Packet{{Time: stamp}, {Time: stamp.Add(120 * time.Second)}})
	if len(long.Timeline) != 60 || long.Timeline[59].Packets != 1 || long.Timeline[59].IntervalSeconds != 2 || !long.Timeline[59].EndTime.Equal(*long.Finished) {
		t.Fatalf("bounded timeline dropped its final observation: %+v", long)
	}
}
func TestTCPFollowSequenceWrap(t *testing.T) {
	a := Decode(testTCP(0xfffffffd, 0x18, "ABC"))
	b := Decode(testTCP(0, 0x18, "DEF"))
	s := Follow([]Packet{b, a}, a.Stream)
	if len(s.Directions) != 1 || s.Directions[0].Text != "ABCDEF" {
		t.Fatal(s)
	}
}
