package platform

import "time"

func snapshotTime() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func stampSnapshotRows(rows []map[string]any, stamp string) []map[string]any {
	for _, row := range rows {
		row["time"] = stamp
		row["timeKind"] = "snapshot"
	}
	return rows
}

func stampSystemSnapshot(data map[string]any, stamp string) {
	if data == nil {
		return
	}
	data["time"], data["captured"], data["timeKind"] = stamp, stamp, "snapshot"
	if interfaces, ok := data["interfaces"].([]map[string]any); ok {
		stampSnapshotRows(interfaces, stamp)
	}
}
