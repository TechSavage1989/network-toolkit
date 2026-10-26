package diag

import (
	"time"

	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

func measurementTime() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// Preserve timestamps supplied by a capture/probe implementation. Older probe
// implementations report no clock time, so completion at this boundary is the
// earliest reliable observation available to the diagnostic.
func stampReply(reply *platform.PingReply) {
	if reply.Time == "" {
		reply.Time = measurementTime()
	}
}

func laterMeasurementTime(first, second string) string {
	a, errA := time.Parse(time.RFC3339Nano, first)
	b, errB := time.Parse(time.RFC3339Nano, second)
	if errB == nil && (errA != nil || b.After(a)) {
		return second
	}
	return first
}

// Result time describes finalization, never a synthetic timestamp for older
// individual rows. Copy maps because some diagnostics also publish them as
// immutable activity snapshots before they return their final result.
func recordResultTime(result model.Result) model.Result {
	if result.Time == "" {
		result.Time = measurementTime()
	}
	if data, ok := result.Data.(map[string]any); ok {
		copy := make(map[string]any, len(data)+1)
		for key, value := range data {
			copy[key] = value
		}
		if stamp, _ := copy["time"].(string); stamp == "" {
			copy["time"] = result.Time
		}
		result.Data = copy
	}
	return result
}
