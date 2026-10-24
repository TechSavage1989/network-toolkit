package diag

import (
	"context"
	"fmt"
	"math"
	"time"

	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

const MaxRetainedSamples = 10000

// Lifetime statistics remain complete even when old individual rows roll off.
type sampleSeries struct {
	rows                  []platform.PingReply
	next                  int
	sent, received, pairs int
	sum, minimum, maximum float64
	jitter, previous      float64
	hasPrevious           bool
}

func (s *sampleSeries) add(reply platform.PingReply) {
	s.sent++
	if reply.Success {
		s.received++
		s.sum += reply.RTTMs
		if s.received == 1 || reply.RTTMs < s.minimum {
			s.minimum = reply.RTTMs
		}
		if s.received == 1 || reply.RTTMs > s.maximum {
			s.maximum = reply.RTTMs
		}
		if s.hasPrevious {
			s.jitter += math.Abs(reply.RTTMs - s.previous)
			s.pairs++
		}
		s.previous, s.hasPrevious = reply.RTTMs, true
	} else {
		s.hasPrevious = false
	}
	if len(s.rows) < MaxRetainedSamples {
		s.rows = append(s.rows, reply)
	} else {
		s.rows[s.next] = reply
		s.next = (s.next + 1) % MaxRetainedSamples
	}
}

func (s *sampleSeries) statistics() Statistics {
	stats := Statistics{Sent: s.sent, Received: s.received, Lost: s.sent - s.received}
	if s.sent > 0 {
		stats.LossPercent = 100 * float64(stats.Lost) / float64(s.sent)
	}
	if s.received > 0 {
		minimum, maximum, average := s.minimum, s.maximum, s.sum/float64(s.received)
		stats.MinMs, stats.MaxMs, stats.AvgMs = &minimum, &maximum, &average
	}
	if s.pairs > 0 {
		jitter := s.jitter / float64(s.pairs)
		stats.JitterMs = &jitter
	}
	return stats
}

func (s *sampleSeries) samples() []platform.PingReply {
	rows := make([]platform.PingReply, 0, len(s.rows))
	rows = append(rows, s.rows[s.next:]...)
	return append(rows, s.rows[:s.next]...)
}

func (s *sampleSeries) collect(ctx context.Context, count int, interval time.Duration, mode string, port int, emit model.Emit, probe func(context.Context) (platform.PingReply, error)) error {
	for sequence := 1; count == 0 || sequence <= count; sequence++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		started := time.Now()
		reply, err := probe(ctx)
		reply.Sequence, reply.Time = sequence, time.Now().UTC().Format(time.RFC3339Nano)
		if err != nil {
			reply.Success, reply.Status, reply.Error = false, "error", err.Error()
		}
		s.add(reply)
		level, message := "warning", fmt.Sprintf("#%d %s", sequence, reply.Status)
		if reply.Success {
			level, message = "success", fmt.Sprintf("#%d %s: %.2f ms", sequence, reply.Address, reply.RTTMs)
		}
		if mode == "tcp" {
			message = fmt.Sprintf("#%d TCP %d: %s (%.2f ms)", sequence, port, reply.Status, reply.RTTMs)
		}
		model.Log(emit, level, message, map[string]any{"type": "sample", "value": reply, "statistics": s.statistics(), "continuous": count == 0})
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if count != 0 {
				return err
			}
		}
		if count == 0 || sequence < count {
			if err := pause(ctx, interval-time.Since(started)); err != nil {
				return err
			}
		}
	}
	return ctx.Err()
}
