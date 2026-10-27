package main

import (
	"bytes"
	"encoding/json"
	"net/url"
	"networktoolkit/internal/capture"
	"testing"
	"time"
)

func TestPacketNavigatorCacheAndExactViewportKeepRecordedData(t *testing.T) {
	a := testApplication(t)
	start := time.Date(2026, 10, 27, 0, 0, 0, 123456789, time.UTC)
	records := make([]capture.Record, 601)
	for i := range records {
		records[i] = captureFixtureRecord(443)
		records[i].Time = start.Add(time.Duration(i) * time.Second)
	}
	var source bytes.Buffer
	if err := capture.WritePCAPNG(&source, records); err != nil {
		t.Fatal(err)
	}
	imported := captureRequest(a, "POST", "/api/packet-captures/import?name=navigator.pcapng", source.Bytes(), a.token)
	var info packetCaptureInfo
	if err := json.Unmarshal(imported.Body.Bytes(), &info); err != nil || imported.Code != 201 {
		t.Fatalf("import: %v %s", err, imported.Body)
	}
	base := "/api/packet-captures/" + info.ID
	before := captureRequest(a, "GET", base+"/export?format=csv", nil, a.token).Body.String()
	w := captureRequest(a, "GET", base+"/graph-overview?query=does-not-match", nil, a.token)
	var overview capture.Navigator
	if err := json.Unmarshal(w.Body.Bytes(), &overview); err != nil || w.Code != 200 || !overview.Start.Equal(start) || !overview.End.Equal(records[600].Time) {
		t.Fatalf("full extent lost through empty filters: %v %s", err, w.Body)
	}
	for _, b := range overview.Bins {
		if b.Packets != 0 {
			t.Fatal("nonmatching frames included")
		}
	}
	first, err := a.packetCaptures.graphView(info.ID, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	second, _ := a.packetCaptures.graphView(info.ID, "", "", "")
	if &first.Timed[0] != &second.Timed[0] {
		t.Fatal("unchanged recording rebuilt its cached chronological index")
	}
	params := url.Values{"graph_mode": {"condensed"}, "graph_start": {records[500].Time.Format(time.RFC3339Nano)}, "graph_end": {records[510].Time.Format(time.RFC3339Nano)}}
	w = captureRequest(a, "GET", base+"/graph?"+params.Encode(), nil, a.token)
	var viewport packetViewport
	if err := json.Unmarshal(w.Body.Bytes(), &viewport); err != nil || w.Code != 200 {
		t.Fatalf("viewport: %v %s", err, w.Body)
	}
	count := 0
	for _, b := range viewport.Timeline {
		count += b.Packets
		if b.Time.Before(records[500].Time) || b.EndTime.After(records[510].Time) {
			t.Fatal("outside contributor in viewport")
		}
	}
	if count != 11 || !viewport.Start.Equal(records[500].Time) || !viewport.End.Equal(records[510].Time) {
		t.Fatalf("inexact viewport: %+v count %d", viewport, count)
	}
	// Delayed callers cannot mutate source summaries, capture fields or exports.
	after := captureRequest(a, "GET", base+"/export?format=csv", nil, a.token).Body.String()
	if before != after {
		t.Fatal("navigation changed recorded export")
	}
	for _, query := range []string{"graph_start=bad&graph_end=bad", "graph_start=" + url.QueryEscape(start.Format(time.RFC3339Nano)), "graph_start=" + url.QueryEscape(records[10].Time.Format(time.RFC3339Nano)) + "&graph_end=" + url.QueryEscape(start.Format(time.RFC3339Nano))} {
		if response := captureRequest(a, "GET", base+"/graph?"+query, nil, a.token); response.Code != 400 {
			t.Fatal("invalid viewport accepted", query, response.Code)
		}
	}
	m := a.packetCaptures
	m.mu.Lock()
	session := m.sessions[info.ID]
	p := *session.packets[len(session.packets)-1]
	p.Number++
	p.Time = p.Time.Add(time.Second)
	session.packets = append(session.packets, &p)
	session.info.Revision++
	m.mu.Unlock()
	from, to := records[500].Time, records[510].Time
	oldView, err := m.graphViewport(info.ID, "", "", "", capture.GraphOptions{Start: &from, End: &to}, first.Overview.Revision)
	if err != nil || oldView.Revision != first.Overview.Revision {
		t.Fatal("drag did not reuse the overview recording revision", err, oldView.Revision)
	}
	m.mu.RLock()
	cached := m.sessions[info.ID].navigator.views[packetGraphKey{}]
	m.mu.RUnlock()
	if &cached.Timed[0] != &first.Timed[0] {
		t.Fatal("live arrival rebuilt the graph index during a gesture")
	}
	updated, _ := m.graphView(info.ID, "", "", "")
	if !updated.Overview.End.Equal(p.Time) || &updated.Timed[0] == &first.Timed[0] {
		t.Fatal("new recording revision did not advance overview/index")
	}
}
