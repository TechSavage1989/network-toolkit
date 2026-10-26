package main

import (
	"bytes"
	"encoding/json"
	"net/url"
	"networktoolkit/internal/capture"
	"testing"
	"time"
)

func TestPacketCaptureLatestLocateAndRecordedFiltersUseFullCapture(t *testing.T) {
	a := testApplication(t)
	start := time.Date(2026, 10, 26, 14, 30, 0, 123456789, time.UTC)
	records := make([]capture.Record, 4)
	for i := range records {
		records[i] = captureFixtureRecord(443)
		records[i].Time = start.Add(time.Duration(3-i) * time.Second)
		records[i].Data[29] = []byte{2, 3, 1, 2}[i]
		if i%2 == 1 {
			records[i].Data[47] = 4
		}
	}
	var source bytes.Buffer
	if err := capture.WritePCAPNG(&source, records); err != nil {
		t.Fatal(err)
	}
	imported := captureRequest(a, "POST", "/api/packet-captures/import?name=graph-fixture.pcapng", source.Bytes(), a.token)
	var info packetCaptureInfo
	if err := json.Unmarshal(imported.Body.Bytes(), &info); err != nil || imported.Code != 201 {
		t.Fatalf("import: %v %s", err, imported.Body)
	}
	base := "/api/packet-captures/" + info.ID
	type page struct {
		Total   int              `json:"total"`
		Offset  int              `json:"offset"`
		Newest  int              `json:"newestIndex"`
		Locate  int              `json:"locateIndex"`
		Packets []capture.Packet `json:"packets"`
	}
	get := func(query string) page {
		t.Helper()
		w := captureRequest(a, "GET", base+"/packets?overview=0&"+query, nil, a.token)
		var got page
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || w.Code != 200 {
			t.Fatalf("page %s: %v %s", query, err, w.Body)
		}
		return got
	}
	for _, tc := range []struct {
		query  string
		newest int
	}{{"sort=number&direction=asc&limit=1", 3}, {"sort=number&direction=desc&limit=1", 0}, {"sort=source&direction=asc&limit=1", 2}, {"sort=source&direction=desc&limit=1", 2}} {
		got := get(tc.query)
		if got.Total != 4 || got.Newest != tc.newest {
			t.Fatalf("newest under %s: %+v", tc.query, got)
		}
	}
	latest := get("sort=source&direction=asc&limit=1&latest=1")
	if latest.Offset != 2 || len(latest.Packets) != 1 || latest.Packets[0].Number != 4 {
		t.Fatalf("Latest did not locate highest packet number under sorted position: %+v", latest)
	}
	filter := `(frame.severity == "note") && (frame.time_recorded) && (protocol == "TCP")`
	filtered := get("sort=number&direction=desc&limit=1&filter=" + url.QueryEscape(filter))
	if filtered.Total != 2 || filtered.Newest != 0 || filtered.Packets[0].Number != 4 {
		t.Fatalf("full-capture predicate: %+v", filtered)
	}
	params := url.Values{"sort": {"source"}, "direction": {"asc"}, "limit": {"1"}, "locate_start": {start.Add(2 * time.Second).Format(time.RFC3339Nano)}, "locate_end": {start.Add(3 * time.Second).Format(time.RFC3339Nano)}}
	located := get(params.Encode())
	if located.Locate != 3 || located.Packets[0].Number != 2 {
		t.Fatalf("half-open absolute interval did not locate source-sorted frame: %+v", located)
	}
	params.Set("locate_end_inclusive", "1")
	located = get(params.Encode())
	if located.Locate != 1 {
		t.Fatalf("inclusive interval: %+v", located)
	}
	bad := captureRequest(a, "GET", base+"/packets?locate_start=bad&locate_end=bad", nil, a.token)
	if bad.Code != 400 {
		t.Fatal("invalid exact interval accepted")
	}
	exported := captureRequest(a, "GET", base+"/export?format=pcapng&filter="+url.QueryEscape(filter), nil, a.token)
	var exportedRecords []capture.Record
	if err := capture.Read(bytes.NewReader(exported.Body.Bytes()), func(r capture.Record) error { exportedRecords = append(exportedRecords, r); return nil }); err != nil || exported.Code != 200 || len(exportedRecords) != 2 {
		t.Fatalf("matching export: %v %s", err, exported.Body)
	}
	for i, index := range []int{1, 3} {
		if !bytes.Equal(exportedRecords[i].Data, records[index].Data) || !exportedRecords[i].Time.Equal(records[index].Time) {
			t.Fatal("filter export altered native bytes or nanosecond timestamps")
		}
	}
	all := get("limit=500")
	if all.Total != 4 {
		t.Fatal("display filters removed recorded packets")
	}
}
