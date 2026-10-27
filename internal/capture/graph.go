package capture

import (
	"math"
	"sort"
	"time"
)

// GraphOptions only changes the displayed timeline. Statistics and source
// packets keep their original retained-filter scope and recorded precision.
type GraphOptions struct {
	Condensed        bool
	Start, End       *time.Time
	Duration, Recent time.Duration
}

func ApplyGraph(s *Statistics, packets []Packet, options GraphOptions) {
	timed := make([]Packet, 0, len(packets))
	for _, packet := range packets {
		if !packet.Time.IsZero() {
			timed = append(timed, packet)
		}
	}
	sort.SliceStable(timed, func(i, j int) bool {
		if timed[i].Time.Equal(timed[j].Time) {
			return timed[i].Number < timed[j].Number
		}
		return timed[i].Time.Before(timed[j].Time)
	})
	s.Timeline = []Bucket{}
	if len(timed) == 0 {
		return
	}
	start, end := timed[0].Time, timed[len(timed)-1].Time
	if options.End != nil {
		end = *options.End
	}
	if options.Duration > 0 {
		start = end.Add(-options.Duration)
	}
	if options.Start != nil {
		start = *options.Start
	}
	from, to := start.UTC(), end.UTC()
	s.GraphStarted, s.GraphFinished = &from, &to
	window := timed[:0]
	for _, p := range timed {
		if !p.Time.Before(start) && !p.Time.After(end) {
			window = append(window, p)
		}
	}
	if len(window) == 0 {
		return
	}
	recent := options.Recent
	if recent <= 0 {
		recent = time.Minute
	}
	recentCount := 0
	for _, p := range window {
		if end.Sub(p.Time) < recent {
			recentCount++
		}
	}
	// Very high-volume recent traffic cannot have one SVG node per frame.
	// Group the entire recent interval honestly rather than silently dropping
	// frames or double-counting a timestamp shared by many packets.
	groupRecent := recentCount > 2000
	if groupRecent && options.Condensed {
		s.GraphNote = "High-volume recent traffic is grouped into intervals. Every retained matching frame remains available in the interval details and packet list."
	}
	bins := map[string]int{}
	uniformStep := max(time.Second, time.Duration(math.Ceil(float64(window[len(window)-1].Time.Sub(window[0].Time))/60)))
	for _, p := range window {
		age := max(time.Duration(0), end.Sub(p.Time))
		key := ""
		if options.Condensed {
			if age < recent && !groupRecent {
				key = "packet:" + uintString(p.Number)
			} else {
				tier := 0
				if age >= recent {
					tier = max(0, int(math.Floor(math.Log2(float64(age)/float64(recent)))))
				}
				step := max(time.Nanosecond, time.Duration(float64(recent)*math.Pow(2, float64(tier))/12))
				prefix := "older:"
				if age < recent {
					step = max(time.Nanosecond, recent/1000)
					prefix = "recent:"
				}
				key = prefix + uintString(uint64(tier)) + ":" + uintString(uint64(age/step))
			}
		} else {
			key = "uniform:" + uintString(uint64(min(59, p.Time.Sub(window[0].Time)/uniformStep)))
		}
		index, ok := bins[key]
		if !ok {
			index = len(s.Timeline)
			bins[key] = index
			t := p.Time.UTC()
			s.Timeline = append(s.Timeline, Bucket{Time: &t, EndTime: &t, Severities: map[string]int{}, EndInclusive: true})
		}
		b := &s.Timeline[index]
		b.Packets++
		b.Bytes += uint64(p.Length)
		b.Severities[PacketSeverity(p)]++
		t := p.Time.UTC()
		if t.Before(*b.Time) {
			b.Time = &t
		}
		if t.After(*b.EndTime) {
			b.EndTime = &t
		}
		b.IntervalSeconds = b.EndTime.Sub(*b.Time).Seconds()
		b.Seconds = b.Time.Sub(start).Seconds()
		if b.Packets == 1 {
			b.PacketNumber = p.Number
		} else {
			b.PacketNumber = 0
		}
	}
	sort.SliceStable(s.Timeline, func(i, j int) bool { return s.Timeline[i].Time.Before(*s.Timeline[j].Time) })
}

func uintString(n uint64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte(n%10) + '0'
		n /= 10
	}
	return string(b[i:])
}
