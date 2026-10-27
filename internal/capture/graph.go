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
	ViewEnd          *time.Time
	Zoom             float64
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
	zoom := options.Zoom
	if zoom <= 0 || math.IsNaN(zoom) || math.IsInf(zoom, 0) {
		zoom = 1
	}
	if zoom != 1 || options.ViewEnd != nil {
		span := max(float64(time.Nanosecond), math.Round(float64(end.Sub(start))/zoom))
		span = min(span, float64(1<<63-1)-1024)
		if options.ViewEnd != nil {
			end = *options.ViewEnd
		}
		start = end.Add(-time.Duration(span))
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
	gaps := make([]time.Duration, 0, 256)
	for i := max(1, len(window)-256); i < len(window); i++ {
		if gap := window[i].Time.Sub(window[i-1].Time); gap > 0 {
			gaps = append(gaps, gap)
		}
	}
	sort.Slice(gaps, func(i, j int) bool { return gaps[i] < gaps[j] })
	cadence := float64(recent) / 60
	if len(gaps) > 0 {
		cadence = float64(gaps[len(gaps)/2])
	}
	oldest := max(1, float64(end.Sub(window[0].Time))/float64(recent))
	density := min(120, max(1, float64(recent)/cadence), 1400/max(1, 2*(math.Sqrt(oldest)-1)))
	bins := map[string]int{}
	uniformStep := max(time.Second, time.Duration(math.Ceil(float64(window[len(window)-1].Time.Sub(window[0].Time))/60)))
	for _, p := range window {
		age := max(time.Duration(0), end.Sub(p.Time))
		key := ""
		if options.Condensed {
			if age < recent && !groupRecent {
				key = "packet:" + uintString(p.Number)
			} else {
				if age < recent {
					step := max(time.Nanosecond, recent/1000)
					key = "recent:" + uintString(uint64(age/step))
				} else {
					// Growing square-root-age intervals stay denser toward older
					// time after nonlinear projection, without resolution tiers.
					bin := uint64(math.Floor(2 * density * (math.Sqrt(float64(age)/float64(recent)) - 1)))
					key = "older:" + uintString(bin)
				}
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
