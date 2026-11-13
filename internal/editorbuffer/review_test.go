package editorbuffer

import (
	"context"
	"runtime"
	"strings"
	"testing"
)

// Fixes from the review of the text tools (v1.21.53). Each test failed
// before its fix.

// A single Replace refuses a Replace text that would expand beyond one edit
// before it builds it: many $0 against a long match would otherwise
// allocate the whole expansion first.
func TestReplaceTextRefusesALongExpansionBeforeBuildingIt(t *testing.T) {
	match := strings.Repeat("x", 512<<10)
	v := importBytes(t, []byte(match))
	s, _ := v.Buffer.Snapshot()
	defer s.Close()
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	_, _, err := s.ReplaceText(context.Background(), "x+", strings.Repeat("$0", 256), 0, int64(len(match)), true, true, false)
	runtime.ReadMemStats(&after)
	if err == nil || !strings.Contains(err.Error(), "too long") {
		t.Fatal(err)
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 32<<20 {
		t.Fatalf("refusing a 128 MiB expansion allocated %d MiB", allocated>>20)
	}
	// One that fits is still expanded.
	text, found, err := s.ReplaceText(context.Background(), "x+", "<$0>", 0, int64(len(match)), true, true, false)
	if err != nil || !found || text != "<"+match+">" {
		t.Fatal(err, found, len(text))
	}
}
