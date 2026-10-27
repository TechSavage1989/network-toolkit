package capture

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCondensedGraphPreservesAllPacketsCountsSeverityAndExactTimes(t *testing.T) {
	start := time.Date(2026, 10, 26, 12, 0, 0, 123456789, time.UTC)
	packets := make([]Packet, 7201)
	for i := range packets {
		packets[i] = Packet{Number: uint64(i + 1), Time: start.Add(time.Duration(i) * time.Second), Record: Record{Time: start.Add(time.Duration(i) * time.Second)}, Length: 100, Protocol: "TCP"}
		if i%19 == 0 {
			packets[i].Expert = []string{"capture note"}
		}
	}
	before, _ := json.Marshal(packets)
	s := Stats(packets)
	ApplyGraph(&s, packets, GraphOptions{Condensed: true, Recent: time.Minute})
	if len(s.Timeline) >= 250 {
		t.Fatalf("older history was not condensed: %d", len(s.Timeline))
	}
	total, bytes, notes, singles := 0, uint64(0), 0, 0
	for _, b := range s.Timeline {
		total += b.Packets
		bytes += b.Bytes
		notes += b.Severities["note"]
		if b.PacketNumber != 0 {
			singles++
		}
		if !b.EndInclusive || b.Time == nil || b.EndTime == nil || b.Time.Nanosecond() != 123456789 {
			t.Fatal("graph lost recorded precision or inclusive boundaries")
		}
	}
	if total != len(packets) || bytes != uint64(len(packets)*100) || notes != s.Expert || singles < 60 {
		t.Fatalf("lost contributions: %d %d %d %d", total, bytes, notes, singles)
	}
	after, _ := json.Marshal(packets)
	if string(before) != string(after) {
		t.Fatal("graph changed source packets")
	}
}
func TestGraphTimeRangeFiltersOnlyTimelineAndKeepsFullStatistics(t *testing.T) {
	start := time.Date(2026, 10, 26, 12, 0, 0, 999999999, time.UTC)
	packets := []Packet{{Number: 1, Time: start, Record: Record{Time: start}, Length: 100, Protocol: "TCP"}, {Number: 2, Time: start.Add(time.Nanosecond), Record: Record{Time: start.Add(time.Nanosecond)}, Length: 200, Protocol: "UDP"}, {Number: 3, Length: 50, Protocol: "TCP"}}
	end := start.Add(time.Nanosecond)
	s := Stats(packets)
	ApplyGraph(&s, packets, GraphOptions{Condensed: true, Start: &end, End: &end, Recent: time.Minute})
	if len(s.Timeline) != 1 || s.Timeline[0].PacketNumber != 2 || s.Timeline[0].Packets != 1 {
		t.Fatalf("exact interval: %+v", s.Timeline)
	}
	if s.Bytes != 350 || s.UntimedPackets != 1 || s.Protocols["TCP"] != 2 {
		t.Fatal("graph range changed whole-capture summary")
	}
	later := start.Add(time.Hour)
	ApplyGraph(&s, packets, GraphOptions{Start: &later, End: &later})
	if len(s.Timeline) != 0 || s.GraphStarted == nil || !s.GraphStarted.Equal(later) {
		t.Fatal("empty range invented measurements or lost its range")
	}
}
func TestHighVolumeCondensedGraphRemainsBoundedWithoutLosingSameTimestampPackets(t *testing.T) {
	stamp := time.Date(2026, 10, 26, 12, 0, 0, 1, time.UTC)
	packets := make([]Packet, 10000)
	for i := range packets {
		packets[i] = Packet{Number: uint64(i + 1), Time: stamp, Record: Record{Time: stamp}, Length: 64, Protocol: "TCP"}
	}
	s := Stats(packets)
	ApplyGraph(&s, packets, GraphOptions{Condensed: true, Recent: time.Minute})
	if len(s.Timeline) != 1 || s.Timeline[0].Packets != len(packets) || s.Timeline[0].PacketNumber != 0 || s.GraphNote == "" {
		t.Fatal("high volume must be an honest grouped interval")
	}
}

func TestGraphZoomAndPanUseExactVisibleContributorsAndKeepFullStatistics(t *testing.T) {
	start := time.Date(2026, 10, 27, 0, 0, 0, 123456789, time.UTC)
	packets := make([]Packet, 21)
	for i := range packets {
		stamp := start.Add(time.Duration(i) * time.Second)
		packets[i] = Packet{Number: uint64(i + 1), Time: stamp, Record: Record{Time: stamp}, Length: 100, Protocol: "TCP"}
	}
	before, _ := json.Marshal(packets)
	for _, condensed := range []bool{false, true} {
		s := Stats(packets)
		ApplyGraph(&s, packets, GraphOptions{Condensed: condensed, Duration: 10 * time.Second, Zoom: 2, Recent: time.Second})
		wantStart, wantEnd := start.Add(15*time.Second), start.Add(20*time.Second)
		if !s.GraphStarted.Equal(wantStart) || !s.GraphFinished.Equal(wantEnd) {
			t.Fatalf("zoomed bounds: %v %v", s.GraphStarted, s.GraphFinished)
		}
		total := 0
		for _, b := range s.Timeline {
			total += b.Packets
			if b.Time.Before(wantStart) || b.EndTime.After(wantEnd) {
				t.Fatal("outside contributors stretched the visible graph")
			}
		}
		if total != 6 || s.Protocols["TCP"] != 21 || s.Bytes != 2100 {
			t.Fatalf("zoom affected original totals: %d %+v", total, s)
		}
		viewEnd := start.Add(17 * time.Second)
		ApplyGraph(&s, packets, GraphOptions{Condensed: condensed, Duration: 10 * time.Second, Zoom: 2, ViewEnd: &viewEnd})
		if !s.GraphStarted.Equal(start.Add(12*time.Second)) || !s.GraphFinished.Equal(viewEnd) {
			t.Fatal("panning changed span or lost end precision")
		}
	}
	after, _ := json.Marshal(packets)
	if string(before) != string(after) {
		t.Fatal("graph navigation changed original packets")
	}
}

func TestGraphZoomKeepsAdjacentNanosecondContributorsAndEmptyRanges(t *testing.T) {
	start := time.Date(2026, 10, 27, 0, 0, 0, 123456780, time.UTC)
	end := start.Add(10 * time.Nanosecond)
	packets := make([]Packet, 11)
	for i := range packets {
		stamp := start.Add(time.Duration(i))
		packets[i] = Packet{Number: uint64(i + 1), Time: stamp, Record: Record{Time: stamp}, Length: 64}
	}
	s := Stats(packets)
	ApplyGraph(&s, packets, GraphOptions{Start: &start, End: &end, Zoom: 2, Condensed: true})
	if !s.GraphStarted.Equal(start.Add(5*time.Nanosecond)) || !s.GraphFinished.Equal(end) {
		t.Fatal("zoom rounded native precision")
	}
	count := 0
	for _, b := range s.Timeline {
		count += b.Packets
	}
	if count != 6 {
		t.Fatalf("nanosecond contributors: %d", count)
	}
	empty := end.Add(time.Hour)
	ApplyGraph(&s, packets, GraphOptions{Start: &start, End: &end, Zoom: 2, ViewEnd: &empty})
	if len(s.Timeline) != 0 || !s.GraphFinished.Equal(empty) {
		t.Fatal("empty zoom range invented points")
	}
}
