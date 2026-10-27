package capture

import "time"

// Navigator is an immutable, bounded display summary. Its full extent comes
// from all retained packets; active filters only affect counts and statuses.
// Empty bins represent genuinely empty intervals, not measured RTT values.
type Navigator struct {
	Revision uint64     `json:"revision,omitempty"`
	Start    *time.Time `json:"start,omitempty"`
	End      *time.Time `json:"end,omitempty"`
	Bins     []Bucket   `json:"bins"`
	Timed    int        `json:"timed"`
	Untimed  int        `json:"untimed"`
}

func TimelineNavigator(all, matching []Packet) Navigator {
	n := Navigator{Bins: []Bucket{}}
	var first, last time.Time
	for _, p := range all {
		if p.Time.IsZero() {
			n.Untimed++
			continue
		}
		n.Timed++
		if first.IsZero() || p.Time.Before(first) {
			first = p.Time
		}
		if last.IsZero() || p.Time.After(last) {
			last = p.Time
		}
	}
	if first.IsZero() {
		return n
	}
	a, b := first.UTC(), last.UTC()
	n.Start, n.End = &a, &b
	count := 240
	if first.Equal(last) {
		count = 1
	}
	span := last.Sub(first)
	n.Bins = make([]Bucket, count)
	for i := range n.Bins {
		start, end := first.Add(time.Duration(int64(span)/int64(count)*int64(i)+int64(span)%int64(count)*int64(i)/int64(count))).UTC(), first.Add(time.Duration(int64(span)/int64(count)*int64(i+1)+int64(span)%int64(count)*int64(i+1)/int64(count))).UTC()
		n.Bins[i] = Bucket{Time: &start, EndTime: &end, EndInclusive: i == count-1, IntervalSeconds: end.Sub(start).Seconds(), Severities: map[string]int{}}
	}
	for _, p := range matching {
		if p.Time.IsZero() {
			continue
		}
		index := 0
		if span > 0 {
			index = min(count-1, max(0, int(float64(p.Time.Sub(first))/float64(span)*float64(count))))
		}
		bin := &n.Bins[index]
		bin.Packets++
		bin.Bytes += uint64(p.Length)
		bin.Severities[PacketSeverity(p)]++
	}
	return n
}
