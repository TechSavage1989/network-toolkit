package main

import (
	"bytes"
	"encoding/json"
	"net/url"
	"networktoolkit/internal/capture"
	"testing"
	"time"
)

func TestPacketGraphRangeIntervalContributorsAndExactNumberLocation(t *testing.T) {
	a := testApplication(t)
	stamp := time.Date(2026, 10, 26, 12, 0, 0, 123456789, time.UTC)
	records := make([]capture.Record, 4)
	for i := range records {
		records[i] = captureFixtureRecord(443)
		records[i].Time = stamp.Add(time.Duration(i) * time.Nanosecond)
		records[i].Data[29] = byte(4 - i)
	}
	var file bytes.Buffer
	if err := capture.WritePCAPNG(&file, records); err != nil {
		t.Fatal(err)
	}
	w := captureRequest(a, "POST", "/api/packet-captures/import?name=history.pcapng", file.Bytes(), a.token)
	var info packetCaptureInfo
	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil || w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	base := "/api/packet-captures/" + info.ID
	query := url.Values{"graph_mode": {"condensed"}, "graph_start": {stamp.Add(time.Nanosecond).Format(time.RFC3339Nano)}, "graph_end": {stamp.Add(2 * time.Nanosecond).Format(time.RFC3339Nano)}}
	w = captureRequest(a, "GET", base+"/statistics?"+query.Encode(), nil, a.token)
	var stats capture.Statistics
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil || w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if stats.TimedPackets != 4 || len(stats.Timeline) != 2 || stats.Timeline[0].PacketNumber != 2 || stats.Timeline[1].PacketNumber != 3 {
		t.Fatalf("display range changed source statistics or timestamps: %+v", stats)
	}
	query = url.Values{"interval_start": {stamp.Add(time.Nanosecond).Format(time.RFC3339Nano)}, "interval_end": {stamp.Add(2 * time.Nanosecond).Format(time.RFC3339Nano)}, "interval_inclusive": {"1"}, "limit": {"1"}, "offset": {"1"}, "sort": {"number"}, "direction": {"asc"}}
	w = captureRequest(a, "GET", base+"/interval?"+query.Encode(), nil, a.token)
	var page struct {
		Total       int
		Packets     []capture.Packet
		LocateIndex int
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if page.Total != 2 || len(page.Packets) != 1 || page.Packets[0].Number != 3 || !page.Packets[0].Time.Equal(records[2].Time) {
		t.Fatalf("exact interval contributors: %+v", page)
	}
	w = captureRequest(a, "GET", base+"/packets?overview=0&sort=source&direction=asc&locate_number=2&limit=1", nil, a.token)
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if page.LocateIndex != 2 || page.Packets[0].Number != 2 {
		t.Fatalf("exact result identity under source sort: %+v", page)
	}
	for _, bad := range []string{"graph_mode=unknown", "graph_mode=condensed&graph_duration_ms=-1", "graph_mode=condensed&graph_recent_ms=0", "graph_start=bad", "graph_start=2026-10-26T12:00:02Z&graph_end=2026-10-26T12:00:01Z"} {
		w = captureRequest(a, "GET", base+"/statistics?"+bad, nil, a.token)
		if w.Code != 400 {
			t.Fatalf("invalid graph query accepted %s: %d", bad, w.Code)
		}
	}
}
