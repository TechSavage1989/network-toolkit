package diag

import (
	"math"
	"strings"
	"time"

	"networktoolkit/internal/platform"
)

// SampleHistory summarizes a discarded, chronological prefix. It never owns
// raw rows. Counts and value ranges remain meaningful after retention rollover.
// Status labels are bounded as well as the sample storage.
type SampleHistory struct {
	Count         int            `json:"count"`
	Received      int            `json:"received"`
	Lost          int            `json:"lost"`
	Measured      int            `json:"measured"`
	MinMs         *float64       `json:"minMs"`
	MaxMs         *float64       `json:"maxMs"`
	SumMs         float64        `json:"sumMs"`
	ZeroCount     int            `json:"zeroCount"`
	FirstTime     string         `json:"firstTime"`
	LastTime      string         `json:"lastTime"`
	FirstSequence int            `json:"firstSequence"`
	LastSequence  int            `json:"lastSequence"`
	StatusCounts  map[string]int `json:"statusCounts"`
	UntimedCount  int            `json:"untimedCount"`
	JitterSumMs   float64        `json:"jitterSumMs"`
	JitterPairs   int            `json:"jitterPairs"`
	JitterMinMs   *float64       `json:"jitterMinMs"`
	JitterMaxMs   *float64       `json:"jitterMaxMs"`
	LastSuccess   bool           `json:"lastSuccess"`
	LastRttMs     *float64       `json:"lastRttMs"`
}

func historyExtrema(minimum, maximum **float64, value float64) {
	if *minimum == nil || value < **minimum {
		v := value
		*minimum = &v
	}
	if *maximum == nil || value > **maximum {
		v := value
		*maximum = &v
	}
}

// Add must be called once, before an original sample leaves its raw window.
// Failures do not become zero-valued measurements or bridge jitter pairs.
func (h *SampleHistory) Add(reply platform.PingReply) {
	if h.Count == 0 {
		h.FirstSequence = reply.Sequence
	}
	consecutive := h.Count > 0 && (reply.Sequence == 0 || h.LastSequence == 0 || reply.Sequence == h.LastSequence+1)
	h.Count++
	if reply.Success {
		h.Received++
	} else {
		h.Lost++
	}
	stamp, err := time.Parse(time.RFC3339Nano, reply.Time)
	if err != nil {
		h.UntimedCount++
	} else {
		first, _ := time.Parse(time.RFC3339Nano, h.FirstTime)
		last, _ := time.Parse(time.RFC3339Nano, h.LastTime)
		if h.FirstTime == "" || stamp.Before(first) {
			h.FirstTime = reply.Time
		}
		if h.LastTime == "" || stamp.After(last) {
			h.LastTime = reply.Time
		}
	}
	status := strings.ToLower(strings.TrimSpace(reply.Status))
	if status == "" {
		if reply.Success {
			status = "reply"
		} else {
			status = "unsuccessful"
		}
	}
	if len(status) > 64 {
		status = "other"
	}
	if h.StatusCounts == nil {
		h.StatusCounts = make(map[string]int)
	}
	if _, known := h.StatusCounts[status]; !known && len(h.StatusCounts) >= 16 {
		status = "other"
	}
	h.StatusCounts[status]++
	measured := reply.Success && reply.RTTMs >= 0 && !math.IsNaN(reply.RTTMs) && !math.IsInf(reply.RTTMs, 0)
	if measured {
		h.Measured++
		h.SumMs += reply.RTTMs
		historyExtrema(&h.MinMs, &h.MaxMs, reply.RTTMs)
		if reply.RTTMs == 0 {
			h.ZeroCount++
		}
		if consecutive && h.LastSuccess && h.LastRttMs != nil {
			jitter := math.Abs(reply.RTTMs - *h.LastRttMs)
			h.JitterPairs++
			h.JitterSumMs += jitter
			historyExtrema(&h.JitterMinMs, &h.JitterMaxMs, jitter)
		}
		value := reply.RTTMs
		h.LastRttMs = &value
	} else {
		h.LastRttMs = nil
	}
	h.LastSuccess, h.LastSequence = measured, reply.Sequence
}

// Snapshot separates mutable status counters from a completed result payload.
func (h *SampleHistory) Snapshot() *SampleHistory {
	if h == nil || h.Count == 0 {
		return nil
	}
	copy := *h
	copy.StatusCounts = make(map[string]int, len(h.StatusCounts))
	for status, count := range h.StatusCounts {
		copy.StatusCounts[status] = count
	}
	return &copy
}
