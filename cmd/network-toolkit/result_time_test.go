package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLocalSnapshotTimesPreserveAcquisitionAndConfigurationIdentity(t *testing.T) {
	acquired := time.Date(2026, 11, 1, 5, 59, 59, 123456789, time.UTC)
	adapter := localTestAdapter()
	revision := localAdapterRevision(adapter)
	snapshot := localIPSnapshot{Available: true, Adapters: []localIPAdapter{adapter}}
	stampLocalIPSnapshot(&snapshot, acquired)
	stampLocalIPSnapshot(&snapshot, acquired.Add(time.Minute))
	if snapshot.Time != acquired.Format(time.RFC3339Nano) || snapshot.Adapters[0].Time != snapshot.Time || snapshot.TimeKind != "snapshot" {
		t.Fatalf("acquisition was replaced by later display time: %+v", snapshot)
	}
	if localAdapterRevision(snapshot.Adapters[0]) != revision {
		t.Fatal("observation time changed the optimistic configuration guard")
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var restored localIPSnapshot
	if err := json.Unmarshal(raw, &restored); err != nil || restored.Time != snapshot.Time || restored.Adapters[0].Time != snapshot.Time {
		t.Fatalf("snapshot timestamp was lost through JSON: %s, %v", raw, err)
	}
	wifi := localWiFiSnapshot{Available: true, Interfaces: []localWiFiInterface{{ID: "fixture"}}}
	stampLocalWiFiSnapshot(&wifi, acquired)
	stampLocalWiFiSnapshot(&wifi, acquired.Add(time.Hour))
	if wifi.Time != snapshot.Time || wifi.Interfaces[0].Time != wifi.Time || wifi.TimeKind != "snapshot" {
		t.Fatalf("Wi-Fi snapshot timestamp changed: %+v", wifi)
	}
}

func TestUnavailableLocalSnapshotsDoNotInventObservationTimes(t *testing.T) {
	ip := localIPSnapshot{}
	wifi := localWiFiSnapshot{}
	stampLocalIPSnapshot(&ip, time.Now())
	stampLocalWiFiSnapshot(&wifi, time.Now())
	if ip.Time != "" || wifi.Time != "" || ip.TimeKind != "" || wifi.TimeKind != "" {
		t.Fatal("unavailable data received an observation timestamp")
	}
}
