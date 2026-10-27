package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"networktoolkit/internal/capture"
)

func (a *application) handlePacketCapture(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/packet-captures")
	m := a.packetCaptures
	if path == "/capabilities" {
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		jsonResponse(w, 200, packetCaptureSupport())
		return
	}
	if path == "/interfaces" {
		if r.Method != http.MethodPost {
			methodError(w)
			return
		}
		var body struct{}
		if e := decodeTerminalBody(w, r, 1024, &body); e != nil {
			jsonError(w, 400, e.Error())
			return
		}
		interfaces, e := packetCaptureInterfaces(r.Context())
		if e != nil {
			jsonError(w, 400, e.Error())
			return
		}
		jsonResponse(w, 200, map[string]any{"interfaces": interfaces})
		return
	}
	if path == "/import" {
		if r.Method != http.MethodPost {
			methodError(w)
			return
		}
		name := filepath.Base(r.URL.Query().Get("name"))
		if name == "." || name == "" {
			name = "Imported capture"
		}
		if len(name) > 200 {
			jsonError(w, 400, "The capture filename is too long.")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, capture.MaxFileBytes)
		info, e := m.importFile(name, r.Body)
		if e != nil {
			jsonError(w, 400, e.Error())
			return
		}
		jsonResponse(w, 201, info)
		return
	}
	if path == "" || path == "/" {
		switch r.Method {
		case http.MethodGet:
			jsonResponse(w, 200, map[string]any{"captures": m.list()})
		case http.MethodPost:
			var o capture.Options
			if e := decodeTerminalBody(w, r, 16<<10, &o); e != nil {
				jsonError(w, 400, e.Error())
				return
			}
			if !packetCaptureSupport().Available {
				jsonError(w, 501, "Live recording is available in the Windows app. Capture-file import and analysis remain available.")
				return
			}
			info, e := m.start(o)
			if e != nil {
				jsonError(w, 409, e.Error())
				return
			}
			jsonResponse(w, 202, info)
		default:
			methodError(w)
		}
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) > 3 || len(parts[0]) != 24 {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	if len(parts) == 1 && r.Method == http.MethodDelete {
		if e := m.remove(id); e != nil {
			jsonError(w, 409, e.Error())
			return
		}
		jsonResponse(w, 200, map[string]bool{"removed": true})
		return
	}
	if len(parts) == 2 && parts[1] == "stop" {
		if r.Method != http.MethodPost {
			methodError(w)
			return
		}
		var body struct{}
		if e := decodeTerminalBody(w, r, 1024, &body); e != nil {
			jsonError(w, 400, e.Error())
			return
		}
		if e := m.stopCapture(id); e != nil {
			jsonError(w, 404, e.Error())
			return
		}
		jsonResponse(w, 202, map[string]bool{"stopping": true})
		return
	}
	if r.Method != http.MethodGet {
		methodError(w)
		return
	}
	// Fast path for the usual live/scrolling packet list. Full filtering, sorting
	// and overview creation are only needed for those explicit operations.
	if len(parts) == 2 && parts[1] == "packets" && r.URL.Query().Get("overview") == "0" &&
		r.URL.Query().Get("filter") == "" && r.URL.Query().Get("query") == "" &&
		(r.URL.Query().Get("protocol") == "" || r.URL.Query().Get("protocol") == "any") &&
		(r.URL.Query().Get("sort") == "" || r.URL.Query().Get("sort") == "number") &&
		r.URL.Query().Get("direction") != "desc" && r.URL.Query().Get("latest") != "1" && r.URL.Query().Get("locate_start") == "" && r.URL.Query().Get("locate_number") == "" {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		info, total, start, page, e := m.windowCapture(id, offset, limit)
		if e != nil {
			jsonError(w, 404, e.Error())
			return
		}
		jsonResponse(w, 200, map[string]any{"capture": info, "total": total, "offset": start, "packets": page, "newestIndex": total - 1})
		return
	}
	if len(parts) == 2 && parts[1] == "graph" {
		q := r.URL.Query()
		options, _, err := packetGraphOptions(q)
		if err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		revision := uint64(0)
		if raw := q.Get("graph_revision"); raw != "" {
			revision, err = strconv.ParseUint(raw, 10, 64)
			if err != nil {
				jsonError(w, 400, "Choose a valid recording revision.")
				return
			}
		}
		view, err := m.graphViewport(id, q.Get("filter"), q.Get("query"), q.Get("protocol"), options, revision)
		if err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		jsonResponse(w, 200, view)
		return
	}
	if len(parts) == 2 && parts[1] == "graph-overview" {
		q := r.URL.Query()
		overview, err := m.navigator(id, q.Get("filter"), q.Get("query"), q.Get("protocol"))
		if err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		jsonResponse(w, 200, overview)
		return
	}
	info, packets, e := m.snapshot(id)
	if e != nil {
		jsonError(w, 404, e.Error())
		return
	}
	if len(parts) == 1 {
		jsonResponse(w, 200, info)
		return
	}
	if parts[1] == "packet" && len(parts) == 3 {
		number, e := strconv.ParseUint(parts[2], 10, 64)
		if e != nil {
			jsonError(w, 400, "Choose a packet number.")
			return
		}
		for index, p := range packets {
			if p.Number == number {
				var previous *capture.Packet
				if index > 0 {
					previous = &packets[index-1]
				}
				layers := append([]capture.Layer{frameCaptureLayer(p, previous)}, capture.Decode(p.Record).Layers...)
				jsonResponse(w, 200, map[string]any{"packet": p, "layers": layers, "hex": capture.HexDump(p.Record.Data), "ascii": capture.ASCII(p.Record.Data), "data": p.Record.Data})
				return
			}
		}
		jsonError(w, 404, "This packet is no longer retained.")
		return
	}
	if parts[1] == "stream" {
		key := r.URL.Query().Get("id")
		if key == "" || len(key) > 1024 {
			jsonError(w, 400, "Choose a TCP or UDP conversation.")
			return
		}
		jsonResponse(w, 200, capture.Follow(packets, key))
		return
	}
	if parts[1] == "troubleshoot" && len(parts) == 2 {
		filter := info.Options.Filter
		filteredCapture := filter.Protocol != "" && filter.Protocol != "any" || filter.Host != "" || len(filter.Ports) != 0 || filter.MAC != "" || filter.VLAN != nil || filter.Direction != "" && filter.Direction != "any"
		jsonResponse(w, 200, capture.Troubleshoot(packets, capture.TroubleshootContext{
			Active: captureActive(info.State), Lost: info.Lost, Overwritten: info.Overwritten,
			KeepFiltered: filteredCapture, FileMode: info.Engine == "file",
		}))
		return
	}
	filtered, e := filterPackets(packets, r.URL.Query().Get("filter"), r.URL.Query().Get("query"), r.URL.Query().Get("protocol"))
	if e != nil {
		jsonError(w, 400, e.Error())
		return
	}
	switch parts[1] {
	case "packets":
		sortField, direction := r.URL.Query().Get("sort"), r.URL.Query().Get("direction")
		sortPackets(filtered, sortField, direction)
		newestIndex := -1
		var newestNumber uint64
		for i, packet := range filtered {
			if newestIndex < 0 || packet.Number > newestNumber {
				newestIndex, newestNumber = i, packet.Number
			}
		}
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 || limit > 500 {
			limit = 100
		}
		if r.URL.Query().Get("latest") == "1" && newestIndex >= 0 {
			offset = max(0, newestIndex-limit/2)
		}
		locateIndex := -1
		if value := r.URL.Query().Get("locate_number"); value != "" {
			number, err := strconv.ParseUint(value, 10, 64)
			if err != nil || number == 0 {
				jsonError(w, 400, "Choose a recorded packet number.")
				return
			}
			for i, p := range filtered {
				if p.Number == number {
					locateIndex = i
					offset = max(0, i-limit/2)
					break
				}
			}
		}
		if value := r.URL.Query().Get("locate_start"); value != "" {
			start, startErr := time.Parse(time.RFC3339Nano, value)
			finish, finishErr := time.Parse(time.RFC3339Nano, r.URL.Query().Get("locate_end"))
			if startErr != nil || finishErr != nil || finish.Before(start) {
				jsonError(w, 400, "Choose recorded interval boundaries.")
				return
			}
			inclusive := r.URL.Query().Get("locate_end_inclusive") == "1"
			for i, packet := range filtered {
				if !packet.Time.IsZero() && !packet.Time.Before(start) && (packet.Time.Before(finish) || inclusive && packet.Time.Equal(finish)) {
					locateIndex = i
					break
				}
			}
			if locateIndex >= 0 {
				offset = max(0, locateIndex-limit/2)
			}
		}
		offset = min(len(filtered), max(0, offset))
		end := min(len(filtered), offset+limit)
		response := map[string]any{"capture": info, "total": len(filtered), "offset": offset, "packets": filtered[offset:end], "newestIndex": newestIndex, "locateIndex": locateIndex}
		if r.URL.Query().Get("overview") != "0" {
			response["overview"] = packetOverviewFor(filtered)
		}
		jsonResponse(w, 200, response)
	case "overview":
		sortPackets(filtered, r.URL.Query().Get("sort"), r.URL.Query().Get("direction"))
		jsonResponse(w, 200, packetOverviewFor(filtered))
	case "statistics":
		options, active, err := packetGraphOptions(r.URL.Query())
		if err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		statistics := capture.Stats(filtered)
		if active {
			capture.ApplyGraph(&statistics, filtered, options)
		}
		jsonResponse(w, 200, statistics)
	case "interval":
		start, err := time.Parse(time.RFC3339Nano, r.URL.Query().Get("interval_start"))
		end, endErr := time.Parse(time.RFC3339Nano, r.URL.Query().Get("interval_end"))
		if err != nil || endErr != nil || end.Before(start) {
			jsonError(w, 400, "Choose valid recorded interval boundaries.")
			return
		}
		inclusive := r.URL.Query().Get("interval_inclusive") == "1"
		matches := make([]capture.Packet, 0)
		for _, p := range filtered {
			if !p.Time.IsZero() && !p.Time.Before(start) && (p.Time.Before(end) || inclusive && p.Time.Equal(end)) {
				matches = append(matches, p)
			}
		}
		sortPackets(matches, r.URL.Query().Get("sort"), r.URL.Query().Get("direction"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 || limit > 500 {
			limit = 50
		}
		offset = min(len(matches), max(0, offset))
		jsonResponse(w, 200, map[string]any{"total": len(matches), "offset": offset, "packets": matches[offset:min(len(matches), offset+limit)]})
	case "export":
		format := r.URL.Query().Get("format")
		packetCaptureExport(w, info, filtered, format)
	default:
		http.NotFound(w, r)
	}
}
func packetCaptureExport(w http.ResponseWriter, info packetCaptureInfo, packets []capture.Packet, format string) {
	if format == "" {
		format = "pcapng"
	}
	if !map[string]bool{"pcap": true, "pcapng": true, "csv": true, "json": true, "txt": true}[format] {
		jsonError(w, 400, "Choose PCAPNG, PCAP, CSV, JSON or text.")
		return
	}
	var out bytes.Buffer
	records := make([]capture.Record, len(packets))
	for i, p := range packets {
		records[i] = p.Record
	}
	var e error
	contentType := "application/octet-stream"
	switch format {
	case "pcap":
		e = capture.WritePCAP(&out, records)
	case "pcapng":
		e = capture.WritePCAPNG(&out, records)
	case "json":
		contentType = "application/json; charset=utf-8"
		e = json.NewEncoder(&out).Encode(map[string]any{"capture": info, "statistics": capture.Stats(packets), "packets": packets})
	case "csv":
		contentType = "text/csv; charset=utf-8"
		writer := csv.NewWriter(&out)
		_ = writer.Write([]string{"Number", "Time UTC", "Source", "Source port", "Destination", "Destination port", "Protocol", "Length", "Captured", "Interface", "Direction", "Info", "Expert"})
		safe := func(s string) string {
			if s != "" && strings.ContainsRune("=+-@", rune(s[0])) {
				return "'" + s
			}
			return s
		}
		for _, p := range packets {
			_ = writer.Write([]string{fmt.Sprint(p.Number), p.Time.Format(time.RFC3339Nano), safe(p.Source), fmt.Sprint(p.SourcePort), safe(p.Destination), fmt.Sprint(p.DestinationPort), p.Protocol, fmt.Sprint(p.Length), fmt.Sprint(p.Captured), safe(p.Interface), p.Direction, safe(p.Info), safe(strings.Join(p.Expert, "; "))})
		}
		writer.Flush()
		e = writer.Error()
	case "txt":
		contentType = "text/plain; charset=utf-8"
		fmt.Fprintf(&out, "Network Toolkit packet capture\n%s\nPackets: %d\n\n", info.Name, len(packets))
		for _, p := range packets {
			fmt.Fprintf(&out, "%d  %s  %s:%d  to  %s:%d  %s  %d bytes\n%s\n", p.Number, p.Time.Format(time.RFC3339Nano), p.Source, p.SourcePort, p.Destination, p.DestinationPort, p.Protocol, p.Length, p.Info)
		}
	}
	if e != nil {
		jsonError(w, 400, e.Error())
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="Network_Toolkit_Capture_`+info.ID+"."+format+`"`)
	w.Header().Set("Content-Length", strconv.Itoa(out.Len()))
	_, _ = io.Copy(w, &out)
}
