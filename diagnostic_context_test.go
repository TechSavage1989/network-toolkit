package main

import (
	"context"
	"testing"

	"networktoolkit/internal/model"
)

func TestSamplingJobsRunUntilCountOrExplicitStopWithoutFixedDeadline(t *testing.T) {
	for _, tool := range []string{"ping", "monitor", "trace", "discover"} {
		ctx, cancel := diagnosticJobContext(context.Background(), model.Request{Tool: tool})
		_, deadline := ctx.Deadline()
		if deadline == (tool == "ping" || tool == "monitor") {
			t.Fatalf("incorrect diagnostic lifetime for %s", tool)
		}
		cancel()
		if ctx.Err() == nil {
			t.Fatal("Stop did not cancel the diagnostic")
		}
	}
}
