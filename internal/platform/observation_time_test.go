package platform

import (
	"testing"
	"time"
)

func TestSnapshotRowsKeepTheirSectionAcquisitionClock(t *testing.T) {
	const first = "2026-10-26T12:34:30.123456789Z"
	const second = "2026-10-26T12:34:31.987654321Z"
	routes := stampSnapshotRows([]map[string]any{{"destination": "192.0.2.0/24"}}, first)
	connections := stampSnapshotRows([]map[string]any{{"state": "ESTABLISHED"}}, second)
	if routes[0]["time"] != first || connections[0]["time"] != second || routes[0]["timeKind"] != "snapshot" {
		t.Fatalf("snapshot rows were retimed or confused with connection creation: %+v %+v", routes, connections)
	}
	before := time.Now().UTC()
	stamp, err := time.Parse(time.RFC3339Nano, snapshotTime())
	if err != nil || stamp.Before(before) || stamp.After(time.Now().UTC()) || stamp.Location() != time.UTC {
		t.Fatalf("invalid acquisition clock: %v %v", stamp, err)
	}
}
