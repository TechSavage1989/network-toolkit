package main

import "time"

// These timestamps describe acquisition of a snapshot, not the creation time
// of a socket, address, interface, or saved configuration.
func stampLocalIPSnapshot(snapshot *localIPSnapshot, completed time.Time) {
	if snapshot == nil || !snapshot.Available {
		return
	}
	if snapshot.Time == "" {
		snapshot.Time = completed.UTC().Format(time.RFC3339Nano)
	}
	snapshot.TimeKind = "snapshot"
	for i := range snapshot.Adapters {
		if snapshot.Adapters[i].Time == "" {
			snapshot.Adapters[i].Time = snapshot.Time
		}
	}
}

func stampLocalWiFiSnapshot(snapshot *localWiFiSnapshot, completed time.Time) {
	if snapshot == nil || !snapshot.Available {
		return
	}
	if snapshot.Time == "" {
		snapshot.Time = completed.UTC().Format(time.RFC3339Nano)
	}
	snapshot.TimeKind = "snapshot"
	for i := range snapshot.Interfaces {
		if snapshot.Interfaces[i].Time == "" {
			snapshot.Interfaces[i].Time = snapshot.Time
		}
	}
}
