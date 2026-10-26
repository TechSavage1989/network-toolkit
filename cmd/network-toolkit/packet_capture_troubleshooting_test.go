package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"networktoolkit/internal/capture"
)

func TestPacketTroubleshootingReviewsWholeCaptureAndKeepsEvidenceInspectable(t *testing.T) {
	a := testApplication(t)
	syn := captureFixtureRecord(443)
	binary.BigEndian.PutUint32(syn.Data[38:], 100)
	reset := captureFixtureRecord(443)
	copy(reset.Data[26:34], []byte{198, 51, 100, 2, 192, 0, 2, 1})
	binary.BigEndian.PutUint16(reset.Data[34:], 443)
	binary.BigEndian.PutUint16(reset.Data[36:], 45000)
	binary.BigEndian.PutUint32(reset.Data[42:], 101)
	reset.Data[47] = 0x14
	reset.Time = syn.Time.Add(10 * time.Millisecond)
	unanswered := captureFixtureRecord(22)
	unanswered.Time = syn.Time.Add(100 * time.Millisecond)
	later := captureFixtureRecord(80)
	later.Data[47] = 0x10
	later.Time = syn.Time.Add(5 * time.Second)
	var data bytes.Buffer
	if err := capture.WritePCAPNG(&data, []capture.Record{syn, reset, unanswered, later}); err != nil {
		t.Fatal(err)
	}
	response := captureRequest(a, http.MethodPost, "/api/packet-captures/import?name=troubleshooting.pcapng", data.Bytes(), a.token)
	if response.Code != http.StatusCreated {
		t.Fatal(response.Code, response.Body.String())
	}
	var info packetCaptureInfo
	if err := json.Unmarshal(response.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	base := "/api/packet-captures/" + info.ID
	var report struct {
		PacketCount int      `json:"packetCount"`
		Scope       string   `json:"scope"`
		Limitations []string `json:"limitations"`
		Metrics     struct {
			TCPResets int `json:"tcpResets"`
		} `json:"metrics"`
		Findings []struct {
			Filter        string   `json:"filter"`
			PacketNumbers []uint64 `json:"packetNumbers"`
		} `json:"findings"`
	}
	for _, suffix := range []string{"/troubleshoot", "/troubleshoot?filter=" + url.QueryEscape("frame.number == 1") + "&query=not-present&protocol=UDP"} {
		response = captureRequest(a, http.MethodGet, base+suffix, nil, a.token)
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &report) != nil || report.PacketCount != 4 || report.Metrics.TCPResets != 1 || report.Scope == "" {
			t.Fatalf("whole-capture review: %d %s", response.Code, response.Body.String())
		}
	}
	if len(report.Findings) == 0 {
		t.Fatal("reset and unanswered connection evidence was omitted")
	}
	for _, finding := range report.Findings {
		if len(finding.PacketNumbers) == 0 {
			continue
		}
		response = captureRequest(a, http.MethodGet, base+"/packets?filter="+url.QueryEscape(finding.Filter), nil, a.token)
		var examples struct {
			Packets []capture.Packet `json:"packets"`
		}
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &examples) != nil || len(examples.Packets) == 0 {
			t.Fatalf("evidence filter cannot be opened: %d %s", response.Code, response.Body.String())
		}
	}
	a.packetCaptures.mu.Lock()
	session := a.packetCaptures.sessions[info.ID]
	session.info.Lost = 2
	session.info.Overwritten = 7
	session.info.Engine = "file"
	session.info.Options.Filter.Host = "192.0.2.1"
	a.packetCaptures.mu.Unlock()
	response = captureRequest(a, http.MethodGet, base+"/troubleshoot", nil, a.token)
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &report) != nil || len(report.Limitations) < 3 {
		t.Fatalf("capture-quality limitations missing: %d %s", response.Code, response.Body.String())
	}
	if response = captureRequest(a, http.MethodGet, base+"/troubleshoot", nil, ""); response.Code != http.StatusUnauthorized {
		t.Fatal("unauthenticated capture review", response.Code)
	}
	if response = captureRequest(a, http.MethodPost, base+"/troubleshoot", []byte("{}"), a.token); response.Code != http.StatusMethodNotAllowed {
		t.Fatal("review accepted a write", response.Code)
	}
	if response = captureRequest(a, http.MethodGet, base+"/troubleshoot/extra", nil, a.token); response.Code != http.StatusNotFound {
		t.Fatal("unexpected nested review route", response.Code)
	}
	if response = captureRequest(a, http.MethodGet, "/api/packet-captures/"+strings.Repeat("0", 24)+"/troubleshoot", nil, a.token); response.Code != http.StatusNotFound {
		t.Fatal("unknown capture review", response.Code)
	}
}
