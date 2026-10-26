package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"networktoolkit/internal/capture"
)

func captureFixtureRecord(port uint16) capture.Record {
	d := make([]byte, 54)
	d[12] = 8
	d[14] = 0x45
	binary.BigEndian.PutUint16(d[16:], 40)
	d[22] = 64
	d[23] = 6
	copy(d[26:], []byte{192, 0, 2, 1, 198, 51, 100, 2})
	binary.BigEndian.PutUint16(d[34:], 45000)
	binary.BigEndian.PutUint16(d[36:], port)
	d[46] = 0x50
	d[47] = 2
	return capture.Record{Time: time.Unix(1780000000, 123456789), Data: d, OriginalLength: 54, LinkType: 1, Interface: "Ethernet"}
}
func TestPacketCaptureRetentionFilteringStopAndConcurrentReads(t *testing.T) {
	for _, ring := range []bool{false, true} {
		t.Run(map[bool]string{false: "stop", true: "ring"}[ring], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			m := newPacketCaptureManager(ctx)
			defer m.close()
			ready := make(chan struct{})
			m.run = func(ctx context.Context, o capture.Options, stop <-chan struct{}, emit func(packetCaptureMessage) error) error {
				if e := emit(packetCaptureMessage{Type: "ready", Engine: "fixture"}); e != nil {
					return e
				}
				records := []capture.Record{}
				for i := 0; i < 8; i++ {
					r := captureFixtureRecord(443)
					if i == 0 {
						r = captureFixtureRecord(22)
					}
					records = append(records, r)
				}
				if e := emit(packetCaptureMessage{Type: "packets", Packets: records, Lost: 2}); e != nil {
					return e
				}
				close(ready)
				select {
				case <-stop:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			info, e := m.start(capture.Options{MaxMB: 1, MaxPackets: 3, Ring: ring, Filter: capture.KeepFilter{Ports: []uint16{443}}})
			if e != nil {
				t.Fatal(e)
			}
			<-ready
			if ring {
				if _, e = m.start(capture.Options{}); e == nil {
					t.Fatal("concurrent recording accepted")
				}
			}
			snapshot, packets, e := m.snapshot(info.ID)
			if e != nil || snapshot.Count != 3 || snapshot.Bytes != 162 || snapshot.Lost != 2 || len(packets) != 3 {
				t.Fatalf("%+v %v", snapshot, e)
			}
			if ring && (snapshot.Overwritten != 4 || packets[0].Number != 5) {
				t.Fatal(snapshot, packets[0].Number)
			}
			if !ring && snapshot.Overwritten != 0 {
				t.Fatal(snapshot)
			}
			var readers sync.WaitGroup
			for i := 0; i < 4; i++ {
				readers.Add(1)
				go func() {
					defer readers.Done()
					for j := 0; j < 20; j++ {
						_, p, _ := m.snapshot(info.ID)
						_ = capture.Stats(p)
						_ = capture.Follow(p, packets[0].Stream)
					}
				}()
			}
			if e = m.stopCapture(info.ID); e != nil {
				t.Fatal(e)
			}
			m.mu.RLock()
			done := m.sessions[info.ID].done
			m.mu.RUnlock()
			<-done
			readers.Wait()
			snapshot, _, _ = m.snapshot(info.ID)
			if snapshot.State != "completed" {
				t.Fatal(snapshot)
			}
			if e = m.remove(info.ID); e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(packets[0].Record.Data, captureFixtureRecord(443).Data) {
				t.Fatal("removal mutated an existing snapshot")
			}
		})
	}
}
func captureRequest(a *application, method, path string, data []byte, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://"+a.host+path, bytes.NewReader(data))
	r.Header.Set("X-Session-Token", token)
	r.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func TestPacketCaptureImportInspectFilterExportAndAuthorization(t *testing.T) {
	a := testApplication(t)
	var data bytes.Buffer
	records := []capture.Record{captureFixtureRecord(443), captureFixtureRecord(22)}
	if e := capture.WritePCAPNG(&data, records); e != nil {
		t.Fatal(e)
	}
	if w := captureRequest(a, "POST", "/api/packet-captures/import", data.Bytes(), ""); w.Code != http.StatusUnauthorized {
		t.Fatal("unauthenticated import", w.Code)
	}
	w := captureRequest(a, "POST", "/api/packet-captures/import?name=fixture.pcapng", data.Bytes(), a.token)
	if w.Code != 201 {
		t.Fatalf("import: %d %s", w.Code, w.Body)
	}
	var info packetCaptureInfo
	if e := json.Unmarshal(w.Body.Bytes(), &info); e != nil || info.Count != 2 || info.State != "completed" {
		t.Fatal(e, info)
	}
	base := "/api/packet-captures/" + info.ID
	w = captureRequest(a, "GET", base+"/packets?filter="+url.QueryEscape("tcp.port == 443 && ip.addr == 192.0.2.0/24"), nil, a.token)
	var page struct {
		Total   int              `json:"total"`
		Packets []capture.Packet `json:"packets"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &page)
	if w.Code != 200 || page.Total != 1 || len(page.Packets) != 1 || page.Packets[0].DestinationPort != 443 {
		t.Fatalf("filtered packets: %s", w.Body)
	}
	// Continuous-scroll window requests omit the expensive full-capture map;
	// the initial request still includes the overview by default.
	for _, check := range []struct {
		suffix   string
		overview bool
	}{{"/packets?overview=0", false}, {"/packets?overview=1", true}, {"/packets", true}} {
		pageResponse := captureRequest(a, "GET", base+check.suffix, nil, a.token)
		var values map[string]json.RawMessage
		if e := json.Unmarshal(pageResponse.Body.Bytes(), &values); e != nil || pageResponse.Code != 200 {
			t.Fatalf("page overview request %s: %v / %d", check.suffix, e, pageResponse.Code)
		}
		_, present := values["overview"]
		if present != check.overview {
			t.Fatalf("overview presence for %s: got %v want %v", check.suffix, present, check.overview)
		}
	}
	// The fast unfiltered window must return just the requested frame, and
	// the independently fetched overview must retain the same protocol and
	// severity as the packet list (including zero severity for normal frames).
	w = captureRequest(a, "GET", base+"/packets?overview=0&offset=1&limit=1&sort=number&direction=asc", nil, a.token)
	var window struct {
		Total   int              `json:"total"`
		Offset  int              `json:"offset"`
		Packets []capture.Packet `json:"packets"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &window); w.Code != 200 || e != nil || window.Total != 2 || window.Offset != 1 || len(window.Packets) != 1 || window.Packets[0].Number != 2 {
		t.Fatalf("window-only response: code=%d err=%v body=%s", w.Code, e, w.Body)
	}
	w = captureRequest(a, "GET", base+"/overview?sort=number&direction=asc", nil, a.token)
	var marks packetOverview
	if e := json.Unmarshal(w.Body.Bytes(), &marks); w.Code != 200 || e != nil || len(marks.PacketMarks) != 2 {
		t.Fatalf("independent map response: code=%d err=%v body=%s", w.Code, e, w.Body)
	}
	if marks.PacketMarks[1] != packetMapCode(window.Packets[0].Protocol, window.Packets[0].Highlight) {
		t.Fatalf("map severity and row highlight disagree: map=%q, protocol=%q, highlight=%q", marks.PacketMarks[1], window.Packets[0].Protocol, window.Packets[0].Highlight)
	}
	w = captureRequest(a, "GET", base+"/packet/1", nil, a.token)
	var detail struct {
		Data   []byte          `json:"data"`
		Layers []capture.Layer `json:"layers"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &detail)
	if w.Code != 200 || !bytes.Equal(detail.Data, records[0].Data) || len(detail.Layers) < 3 {
		t.Fatal("packet detail missing bytes or fields", w.Body)
	}
	for _, format := range []string{"pcap", "pcapng"} {
		w = captureRequest(a, "GET", base+"/export?format="+format+"&filter="+url.QueryEscape("tcp.port == 443"), nil, a.token)
		count := 0
		e := capture.Read(bytes.NewReader(w.Body.Bytes()), func(r capture.Record) error {
			count++
			if !r.Time.Equal(records[0].Time) || !bytes.Equal(r.Data, records[0].Data) {
				t.Fatal("export changed captured data")
			}
			return nil
		})
		if w.Code != 200 || e != nil || count != 1 {
			t.Fatal(w.Code, e, count)
		}
	}
	for _, format := range []string{"csv", "json", "txt"} {
		w = captureRequest(a, "GET", base+"/export?format="+format, nil, a.token)
		if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Disposition"), "attachment;") || w.Body.Len() == 0 {
			t.Fatal(format, w.Code)
		}
	}
	if w = captureRequest(a, "GET", base+"/packets?filter="+url.QueryEscape("bad filter"), nil, a.token); w.Code != 400 {
		t.Fatal("invalid filter accepted")
	}
	if w = captureRequest(a, "POST", "/api/packet-captures/import", []byte("bad file"), a.token); w.Code != 400 || len(a.packetCaptures.list()) != 1 {
		t.Fatal("failed import changed retained captures")
	}
	var launch terminalViewLaunch
	a.launchTerminalView = func(_ context.Context, l terminalViewLaunch) (<-chan error, error) { launch = l; return nil, nil }
	session := viewTestConnected(t, a)
	if w := viewTestRequest(a, a.token, "POST", "/api/terminal-views", map[string]any{"sessionId": session.ID, "snapshot": viewTestSnapshot(a, session)}, nil); w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	if w = captureRequest(a, "GET", base, nil, launch.ViewToken); w.Code != 403 {
		t.Fatal("terminal token reached capture", w.Code)
	}
	if w = captureRequest(a, "DELETE", base, nil, a.token); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if len(a.packetCaptures.list()) != 0 {
		t.Fatal("capture was not cleared")
	}
}
func TestPacketCaptureImportEvictionAndFailedImportIsolation(t *testing.T) {
	m := newPacketCaptureManager(context.Background())
	defer m.close()
	var b bytes.Buffer
	_ = capture.WritePCAP(&b, []capture.Record{captureFixtureRecord(22)})
	var first string
	for i := 0; i < 4; i++ {
		s, e := m.importFile("capture", bytes.NewReader(b.Bytes()))
		if e != nil {
			t.Fatal(e)
		}
		if i == 0 {
			first = s.ID
		}
	}
	if len(m.list()) != 3 {
		t.Fatal("retained capture limit ignored")
	}
	if _, _, e := m.snapshot(first); e == nil {
		t.Fatal("oldest capture was not evicted")
	}
	_, e := m.importFile("invalid", io.LimitReader(strings.NewReader("bad capture"), 100))
	if e == nil || len(m.list()) != 3 {
		t.Fatal("invalid import changed retained captures")
	}
}
