package capture

import (
	"reflect"
	"testing"
	"time"
)

func TestNavigatorFullExtentFiltersGapsPrecisionAndBoundedCounts(t *testing.T) {
	start := time.Date(2026, 10, 27, 0, 0, 0, 123456789, time.UTC)
	packets := make([]Packet, 10001)
	for i := range packets {
		packets[i] = Packet{Number: uint64(i + 1), Time: start.Add(time.Duration(i) * time.Second), Length: 100}
	}
	packets[10000].Time = start.Add(24 * time.Hour)
	packets[20].Time = time.Time{}
	copyBefore := append([]Packet(nil), packets...)
	n := TimelineNavigator(packets, packets[9000:])
	if len(n.Bins) != 240 || n.Timed != 10000 || n.Untimed != 1 || !n.Start.Equal(start) || !n.End.Equal(start.Add(24*time.Hour)) {
		t.Fatalf("bad full context: %+v", n)
	}
	count, empty := 0, 0
	for _, bin := range n.Bins {
		count += bin.Packets
		if bin.Packets == 0 {
			empty++
		}
		if bin.EndTime.Before(*bin.Time) {
			t.Fatal("inverted interval")
		}
	}
	if count != 1001 || empty < 100 {
		t.Fatalf("lost contributors or gaps: %d %d", count, empty)
	}
	if !reflect.DeepEqual(packets, copyBefore) {
		t.Fatal("navigator changed packet data")
	}
	// Long recordings must not overflow integer interpolation of bin boundaries.
	packets[10000].Time = start.AddDate(10, 0, 0)
	n = TimelineNavigator(packets, nil)
	if !n.Bins[239].EndTime.Equal(packets[10000].Time) {
		t.Fatal("long span overflow")
	}
}
func TestNavigatorEmptyUnknownAndSingleTimestamp(t *testing.T) {
	if n := TimelineNavigator([]Packet{{}}, nil); len(n.Bins) != 0 || n.Start != nil || n.Untimed != 1 {
		t.Fatal(n)
	}
	stamp := time.Now().UTC()
	packets := []Packet{{Time: stamp, Length: 0}, {Time: stamp, Length: 100}}
	n := TimelineNavigator(packets, packets)
	if len(n.Bins) != 1 || n.Bins[0].Packets != 2 || n.Bins[0].Bytes != 100 || n.Bins[0].IntervalSeconds != 0 {
		t.Fatal(n)
	}
}
