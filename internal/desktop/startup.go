package desktop

import "time"

type startupDecision uint8

const (
	startupWait startupDecision = iota
	startupProbe
	startupShow
	startupFail
)

// The splash remains visible until navigation and asynchronous UI setup finish.
// A stalled script callback must not prevent the deadline or cancellation.
type startupGate struct {
	deadline time.Time
	loaded   bool
	pending  bool
	finished bool
}

func (s *startupGate) tick(now time.Time) startupDecision {
	if s.finished {
		return startupWait
	}
	if !now.Before(s.deadline) {
		s.finished = true
		return startupFail
	}
	if s.loaded && !s.pending {
		s.pending = true
		return startupProbe
	}
	return startupWait
}

func (s *startupGate) completed(state int) startupDecision {
	if s.finished {
		return startupWait
	}
	s.pending = false
	if state == 1 || state == -1 {
		s.finished = true
		if state == 1 {
			return startupShow
		}
		return startupFail
	}
	return startupWait
}

func splashBounds(work Rect, width, height int32) Rect {
	width = min(max(width, 1), work.Right-work.Left)
	height = min(max(height, 1), work.Bottom-work.Top)
	x := work.Left + (work.Right-work.Left-width)/2
	y := work.Top + (work.Bottom-work.Top-height)/2
	return Rect{x, y, x + width, y + height}
}
