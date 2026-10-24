package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"networktoolkit/internal/capture"
)

func captureUDPDetailFixture(port uint16, payload []byte) capture.Record {
	r := captureFixtureRecord(port)
	r.Data = append(append([]byte{}, r.Data[:34]...), make([]byte, 8+len(payload))...)
	r.Data[23] = 17
	binary.BigEndian.PutUint16(r.Data[16:], uint16(len(r.Data)-14))
	binary.BigEndian.PutUint16(r.Data[34:], 45000)
	binary.BigEndian.PutUint16(r.Data[36:], port)
	binary.BigEndian.PutUint16(r.Data[38:], uint16(8+len(payload)))
	copy(r.Data[42:], payload)
	r.OriginalLength = uint32(len(r.Data))
	return r
}

// Exercise the actual import and detail HTTP paths, including unstructured
// layers whose fields can be null in JSON. The inspector must still show the
// decoded headers and original bytes when no application decoder is available.
func TestPacketCaptureUDPDetailRetainsHeadersAndBytes(t *testing.T) {
	dns := []byte{0x12, 0x34, 1, 0, 0, 1, 0, 0, 0, 0, 0, 0, 7, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 4, 't', 'e', 's', 't', 0, 0, 1, 0, 1}
	truncatedPayload := captureUDPDetailFixture(9999, []byte("complete payload"))
	truncatedPayload.Data = truncatedPayload.Data[:45]
	truncatedHeader := captureUDPDetailFixture(9999, nil)
	truncatedHeader.Data = truncatedHeader.Data[:38]
	cases := []struct {
		name        string
		record      capture.Record
		protocol    string
		wantLayers  []string
		wantPayload int
		wantExpert  string
	}{
		{"generic payload", captureUDPDetailFixture(9999, bytes.Repeat([]byte("UDP payload "), 420)), "UDP", []string{"Ethernet", "Internet Protocol v4", "User Datagram Protocol", "Payload"}, 5040, ""},
		{"empty datagram", captureUDPDetailFixture(9999, nil), "UDP", []string{"Ethernet", "Internet Protocol v4", "User Datagram Protocol"}, 0, ""},
		{"DNS query", captureUDPDetailFixture(53, dns), "DNS", []string{"Ethernet", "Internet Protocol v4", "User Datagram Protocol", "Domain Name System"}, len(dns), ""},
		{"UDP 443 encrypted payload", captureUDPDetailFixture(443, []byte{0xc0, 0, 0, 0, 1, 0, 0, 0x45, 0x67, 0x89, 0xab}), "UDP", []string{"Ethernet", "Internet Protocol v4", "User Datagram Protocol", "Payload"}, 11, ""},
		{"truncated payload", truncatedPayload, "UDP", []string{"Ethernet", "Internet Protocol v4", "User Datagram Protocol", "Payload"}, 3, "Packet truncated by capture length"},
		{"truncated UDP header", truncatedHeader, "UDP", []string{"Ethernet", "Internet Protocol v4"}, 0, "Incomplete UDP header"},
	}
	a := testApplication(t)
	records := make([]capture.Record, len(cases))
	for i, tc := range cases {
		records[i] = tc.record
	}
	var file bytes.Buffer
	if err := capture.WritePCAPNG(&file, records); err != nil {
		t.Fatal(err)
	}
	w := captureRequest(a, http.MethodPost, "/api/packet-captures/import?name=udp-details.pcapng", file.Bytes(), a.token)
	if w.Code != http.StatusCreated {
		t.Fatalf("import: %d %s", w.Code, w.Body)
	}
	var info packetCaptureInfo
	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil || info.Count != len(cases) {
		t.Fatalf("capture: %v %+v", err, info)
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := captureRequest(a, http.MethodGet, fmt.Sprintf("/api/packet-captures/%s/packet/%d", info.ID, i+1), nil, a.token)
			if w.Code != http.StatusOK {
				t.Fatalf("detail: %d %s", w.Code, w.Body)
			}
			var detail struct {
				Packet capture.Packet  `json:"packet"`
				Layers []capture.Layer `json:"layers"`
				Data   []byte          `json:"data"`
				Hex    string          `json:"hex"`
				ASCII  string          `json:"ascii"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(detail.Data, tc.record.Data) || detail.Hex != capture.HexDump(tc.record.Data) || detail.ASCII != capture.ASCII(tc.record.Data) {
				t.Fatal("packet details changed or omitted captured bytes")
			}
			p := detail.Packet
			if p.Number != uint64(i+1) || p.Transport != "UDP" || p.Protocol != tc.protocol || p.Captured != len(tc.record.Data) || p.Length != tc.record.OriginalLength || p.PayloadLength != tc.wantPayload {
				t.Fatalf("unexpected UDP summary: %+v", p)
			}
			if p.Source != "192.0.2.1" || p.Destination != "198.51.100.2" {
				t.Fatalf("IP endpoints missing: %+v", p)
			}
			if tc.wantExpert != "" && !slices.Contains(p.Expert, tc.wantExpert) {
				t.Fatalf("missing capture/decode limitation %q: %v", tc.wantExpert, p.Expert)
			}
			var layerNames []string
			for _, layer := range detail.Layers {
				layerNames = append(layerNames, layer.Name)
				if layer.Offset < 0 || layer.Length < 0 || layer.Offset+layer.Length > len(detail.Data) {
					t.Fatalf("invalid layer byte range: %+v", layer)
				}
				for _, field := range layer.Fields {
					if field.Offset < 0 || field.Length < 0 || field.Offset+field.Length > len(detail.Data) {
						t.Fatalf("invalid field byte range: %+v", field)
					}
				}
				if layer.Name == "User Datagram Protocol" && (len(layer.Fields) < 3 || p.SourcePort != 45000 || p.DestinationPort == 0) {
					t.Fatalf("UDP header fields missing: %+v %+v", p, layer)
				}
				if layer.Name == "Payload" && len(layer.Fields) != 0 {
					t.Fatalf("expected raw payload without application fields: %+v", layer)
				}
			}
			if len(layerNames) == 0 || !strings.HasPrefix(layerNames[0], "Frame ") || !slices.Equal(layerNames[1:], tc.wantLayers) {
				t.Fatalf("packet layers: got %v, want %v", layerNames, tc.wantLayers)
			}
			if tc.protocol == "DNS" && !strings.Contains(p.Info, "example.test") {
				t.Fatalf("DNS query details missing: %+v", p)
			}
			t.Logf("HTTP 200: %d captured bytes, layers %v", len(detail.Data), layerNames)
		})
	}
}
