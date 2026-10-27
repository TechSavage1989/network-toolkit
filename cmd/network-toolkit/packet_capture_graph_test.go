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

func TestPacketGraphZoomQueryValidation(t *testing.T) {
	for _, value := range []string{"0", "0.1", "17", "NaN", "Inf", "bad"} {
		if _, _, err := packetGraphOptions(url.Values{"graph_zoom": {value}}); err == nil {
			t.Fatalf("invalid zoom accepted: %s", value)
		}
	}
	end := "2026-10-27T00:00:10.123456789Z"
	for _, value := range []string{".25", "1", "1.325", "16"} {
		options, active, err := packetGraphOptions(url.Values{"graph_zoom": {value}, "graph_view_end": {end}})
		if err != nil || !active || options.ViewEnd == nil || options.ViewEnd.Nanosecond() != 123456789 {
			t.Fatalf("valid zoom rejected: %s %v", value, err)
		}
	}
	if _, _, err := packetGraphOptions(url.Values{"graph_view_end": {"invalid"}}); err == nil {
		t.Fatal("invalid view end accepted")
	}
}

func TestPacketCaptureGraphZoomUsesNativeVisibleFramesWithoutChangingExport(t *testing.T) {
	a := testApplication(t)
	start := time.Date(2026, 10, 27, 0, 0, 0, 123456780, time.UTC)
	records := make([]capture.Record, 11)
	for i := range records {
		records[i] = captureFixtureRecord(443)
		records[i].Time = start.Add(time.Duration(i))
	}
	var input bytes.Buffer
	if err := capture.WritePCAPNG(&input, records); err != nil {
		t.Fatal(err)
	}
	imported := captureRequest(a, "POST", "/api/packet-captures/import?name=zoom-ns.pcapng", input.Bytes(), a.token)
	var info packetCaptureInfo
	if err := json.Unmarshal(imported.Body.Bytes(), &info); err != nil || imported.Code != 201 {
		t.Fatalf("import failed: %v %s", err, imported.Body)
	}
	end := start.Add(10 * time.Nanosecond)
	params := url.Values{"graph_mode": {"condensed"}, "graph_start": {start.Format(time.RFC3339Nano)}, "graph_end": {end.Format(time.RFC3339Nano)}, "graph_zoom": {"2"}}
	w := captureRequest(a, "GET", "/api/packet-captures/"+info.ID+"/statistics?"+params.Encode(), nil, a.token)
	var stats capture.Statistics
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil || w.Code != 200 {
		t.Fatalf("zoom response: %v %s", err, w.Body)
	}
	count := 0
	for _, b := range stats.Timeline {
		count += b.Packets
		if b.Time.Before(start.Add(5 * time.Nanosecond)) {
			t.Fatal("out-of-view frame entered a visible group")
		}
	}
	if count != 6 || stats.Protocols["TCP"] != 11 {
		t.Fatalf("visible/whole counts: %d %+v", count, stats.Protocols)
	}
	exported := captureRequest(a, "GET", "/api/packet-captures/"+info.ID+"/export?format=pcapng", nil, a.token)
	index := 0
	if err := capture.Read(bytes.NewReader(exported.Body.Bytes()), func(r capture.Record) error {
		if !r.Time.Equal(records[index].Time) || !bytes.Equal(r.Data, records[index].Data) {
			t.Fatal("graph zoom changed exported bytes or precision")
		}
		index++
		return nil
	}); err != nil || index != len(records) {
		t.Fatalf("export count: %d %v", index, err)
	}
}
