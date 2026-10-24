package desktop

import (
	"testing"
	"time"
)

func TestStartupWaitsForNavigationAndInterface(t *testing.T) {
	now := time.Now()
	s := startupGate{deadline: now.Add(time.Second)}
	if got := s.tick(now); got != startupWait {
		t.Fatal("probe started before navigation completed")
	}
	s.loaded = true
	if s.tick(now) != startupProbe || s.tick(now) != startupWait {
		t.Fatal("startup did not serialize asynchronous probes")
	}
	if s.completed(0) != startupWait || s.tick(now) != startupProbe {
		t.Fatal("unfinished interface did not keep the splash visible")
	}
	if s.completed(1) != startupShow || s.tick(now) != startupWait || s.completed(1) != startupWait {
		t.Fatal("main window must be shown exactly once")
	}
}

func TestStartupTimeoutCancellationAndFailureRejectLateResults(t *testing.T) {
	now := time.Now()
	for _, navigation := range []bool{false, true} {
		s := startupGate{deadline: now, loaded: navigation, pending: true}
		if s.tick(now) != startupFail || s.completed(1) != startupWait {
			t.Fatal("stalled startup or probe bypassed deadline")
		}
	}
	s := startupGate{deadline: now.Add(time.Second), loaded: true, pending: true}
	if s.completed(-1) != startupFail || s.completed(1) != startupWait {
		t.Fatal("failed interface was later revealed")
	}
	s = startupGate{finished: true, loaded: true, pending: true}
	if s.completed(1) != startupWait || s.tick(now) != startupWait {
		t.Fatal("cancelled startup was reopened by a late callback")
	}
}

func TestSplashCentersWithinSelectedMonitorWorkArea(t *testing.T) {
	for _, work := range []Rect{{0, 0, 1920, 1040}, {-1920, -200, 0, 880}, {320, 80, 600, 240}} {
		r := splashBounds(work, 660, 336)
		if r.Left < work.Left || r.Top < work.Top || r.Right > work.Right || r.Bottom > work.Bottom {
			t.Fatalf("splash is outside monitor: %+v in %+v", r, work)
		}
		if (r.Left-work.Left)-(work.Right-r.Right) > 1 || (r.Top-work.Top)-(work.Bottom-r.Bottom) > 1 {
			t.Fatal("splash is not centered")
		}
	}
}
