package diag

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	"networktoolkit/internal/platform"
)

func TestHistoricalSummarySurvivesMillionSamplesWithBoundedRawStorage(t *testing.T) {
	const total = 1000000
	const omitted = total - MaxRetainedSamples
	series := &sampleSeries{}
	base := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	var sum, changes, previous float64
	var measured, lost, zero, pairs int
	var priorOK bool
	for i := 1; i <= total; i++ {
		ok := i%5 != 0
		value := float64(i % 101)
		status := "reply"
		if !ok { status = "timeout" }
		series.add(platform.PingReply{Sequence: i, Time: base.Add(time.Duration(i)*time.Second).Format(time.RFC3339Nano), Success: ok, RTTMs: value, Status: status})
		if i <= omitted {
			if ok {
				measured++
				sum += value
				if value == 0 { zero++ }
				if priorOK { pairs++; changes += math.Abs(value-previous) }
				previous = value
			} else { lost++ }
			priorOK = ok
		}
	}
	h := series.history.Snapshot()
	if h == nil || h.Count != omitted || h.Received != measured || h.Measured != measured || h.Lost != lost || h.SumMs != sum || h.ZeroCount != zero || h.JitterPairs != pairs || h.JitterSumMs != changes {
		t.Fatalf("discarded history lost exact contributors: %+v", h)
	}
	if h.MinMs == nil || *h.MinMs != 0 || h.MaxMs == nil || *h.MaxMs != 100 || h.FirstSequence != 1 || h.LastSequence != omitted || h.FirstTime != base.Add(time.Second).Format(time.RFC3339Nano) || h.LastTime != base.Add(time.Duration(omitted)*time.Second).Format(time.RFC3339Nano) {
		t.Fatalf("history range or extrema changed: %+v", h)
	}
	rows := series.samples()
	if len(rows) != MaxRetainedSamples || rows[0].Sequence != omitted+1 || rows[len(rows)-1].Sequence != total || h.Count+len(rows) != total { t.Fatal("raw storage grew or history overlapped") }
	encoded, err := json.Marshal(h)
	if err != nil || len(encoded) > 4096 || len(h.StatusCounts) != 2 { t.Fatalf("summary is not bounded: %d bytes %v", len(encoded), err) }
	var restored SampleHistory
	if json.Unmarshal(encoded, &restored) != nil || restored.Count != h.Count || restored.SumMs != sum || restored.LastSequence != omitted { t.Fatal("saved summary loses history") }
	if (&sampleSeries{}).history.Snapshot() != nil { t.Fatal("new run inherited prior history") }
}

func TestHistoricalSummaryKeepsFailuresDistinctAndBreaksInvalidJitter(t *testing.T) {
	var h SampleHistory
	for _, row := range []platform.PingReply{
		{Sequence: 1, Success: false, RTTMs: 0, Status: "timeout"},
		{Sequence: 2, Success: true, RTTMs: 0, Status: "reply"},
		{Sequence: 3, Success: true, RTTMs: 99, Status: "reply"},
		{Sequence: 5, Success: true, RTTMs: 2, Status: "reply"},
		{Sequence: 6, Success: false, RTTMs: 99999, Status: "error"},
		{Sequence: 7, Success: true, RTTMs: math.NaN(), Status: "reply"},
		{Sequence: 8, Success: true, RTTMs: 3, Status: "reply"},
	} { h.Add(row) }
	if h.Count != 7 || h.Received != 5 || h.Lost != 2 || h.Measured != 4 || h.ZeroCount != 1 || h.SumMs != 104 || *h.MinMs != 0 || *h.MaxMs != 99 || h.JitterPairs != 1 || h.JitterSumMs != 99 || h.UntimedCount != 7 { t.Fatalf("invalid or failed sample became an RTT: %+v", h) }
	if _, err := json.Marshal(&h); err != nil { t.Fatal(err) }
}

func TestHistoricalSummaryBoundsStatusKindsAndFreezesSnapshots(t *testing.T) {
	var h SampleHistory
	for i := 1; i <= 1000; i++ { h.Add(platform.PingReply{Sequence: i, Status: fmt.Sprintf("status-%d", i)}) }
	if len(h.StatusCounts) > 17 || h.StatusCounts["other"] != 984 { t.Fatalf("unbounded status labels: %+v", h.StatusCounts) }
	snapshot := h.Snapshot()
	h.Add(platform.PingReply{Sequence: 1001, Status: "status-1"})
	if snapshot.Count != 1000 || snapshot.StatusCounts["status-1"] != 1 || h.StatusCounts["status-1"] != 2 { t.Fatal("later samples mutated an exported summary") }
}
