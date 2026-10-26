package main

import (
	"fmt"
	"strings"

	"networktoolkit/internal/capture"
)

// Packet overview signals are conservative, observable decoder and protocol
// clues. They are not an independent protocol dissector or diagnosis.
type packetOverviewSignal struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Explanation string `json:"explanation"`
	Count       int    `json:"count"`
	Number      uint64 `json:"number"`
	Index       int    `json:"index"`
}
type packetOverviewBin struct {
	Level       string `json:"level"`
	Protocol    string `json:"protocol"`
	Interesting int    `json:"interesting"`
}
type packetOverview struct {
	Bins []packetOverviewBin `json:"bins"`
	// One ASCII code per filtered packet, in current display order. The UI
	// renders a packet-resolution, scrollable traffic strip without loading
	// each packet or adding thousands of DOM elements.
	PacketMarks string                 `json:"packetMarks"`
	Signals     []packetOverviewSignal `json:"signals"`
	Interesting int                    `json:"interesting"`
	Counts      map[string]int         `json:"counts"`
}

// These are packet-level highlights, distinct from the full-capture
// request/reply observations provided by capture.Troubleshoot.
func packetClue(p capture.Packet) (string, string, string, string) {
	info := strings.ToLower(p.Info)
	for _, note := range p.Expert {
		n := strings.ToLower(note)
		switch {
		case strings.Contains(n, "malformed"), strings.Contains(n, "invalid"), strings.Contains(n, "incomplete"):
			return "decoder-error", "error", "Malformed or incomplete header", "The captured decoder reported malformed or incomplete protocol data. Truncation and unsupported formats can look similar."
		case strings.Contains(n, "drop reason"):
			return "capture-drop", "warn", "Windows capture drop indication", "The capture record contains a Windows drop reason. This does not directly measure network packet loss."
		case strings.Contains(n, "truncated"):
			return "truncated", "note", "Truncated capture", "Only part of the original packet was recorded. Header or payload details may be unavailable."
		}
	}
	switch {
	case p.Transport == "TCP" && p.TCPFlags&4 != 0:
		return "tcp-reset", "note", "TCP reset (RST)", "A peer reset a TCP connection. Resets can be expected, so inspect the surrounding conversation."
	case p.Protocol == "DNS" && (strings.Contains(info, "servfail") || strings.Contains(info, "refused")):
		return "dns-failure", "warn", "DNS server error or refusal", "A DNS response reports failure or refusal. Verify the requested name and resolver."
	case p.Protocol == "DNS" && strings.Contains(info, "nxdomain"):
		return "dns-negative", "note", "DNS name not found", "A DNS reply indicates the name does not exist. This can be a normal result."
	case p.Protocol == "DHCP" && (strings.Contains(info, "nak") || strings.Contains(info, "decline")):
		return "dhcp-negative", "warn", "DHCP rejection or decline", "A DHCP rejection or address-decline message was observed. Review the related DHCP exchange."
	case (p.Protocol == "ICMP" || p.Protocol == "ICMPv6") && (strings.Contains(info, "unreachable") || strings.Contains(info, "administratively prohibited")):
		return "icmp-unreachable", "warn", "Network unreachable message", "An ICMP delivery error was seen. Inspect source, destination and the surrounding traffic."
	case p.Transport == "TCP" && (strings.Contains(info, "retransmission") || strings.Contains(info, "out-of-order")):
		return "tcp-sequence", "note", "Possible TCP sequencing issue", "The packet summary notes a TCP sequencing event. Duplicate capture observations can resemble retransmissions."
	}
	if len(p.Expert) > 0 {
		return "decoder-note", "note", "Decoder note", "The packet decoder recorded a detail worth inspecting; it is not necessarily a network problem."
	}
	return "", "", "", ""
}

// Codes are a fixed, ASCII-safe alphabet: 16 protocol families times
// 4 importance levels. A single character represents exactly one packet,
// so a 50,000 frame capture only adds 50 KiB to a full overview response.
const packetMapAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

func packetMapProtocol(p string) int {
	switch strings.ToUpper(p) {
	case "TCP":
		return 1
	case "UDP":
		return 2
	case "DNS":
		return 3
	case "DHCP", "BOOTP", "DHCPV6":
		return 4
	case "ARP":
		return 5
	case "ICMP":
		return 6
	case "ICMPV6":
		return 7
	case "TLS":
		return 8
	case "HTTP", "HTTP2":
		return 9
	case "SSH":
		return 10
	case "MDNS":
		return 11
	case "SMB", "SMB2":
		return 12
	case "NTP":
		return 13
	case "WIREGUARD":
		return 14
	default:
		return 0
	}
}
func packetMapCode(protocol, level string) byte {
	severity := 0
	switch level {
	case "note":
		severity = 1
	case "warn":
		severity = 2
	case "error":
		severity = 3
	}
	return packetMapAlphabet[severity*16+packetMapProtocol(protocol)]
}

func packetOverviewFor(packets []capture.Packet) packetOverview {
	result := packetOverview{Bins: []packetOverviewBin{}, Signals: []packetOverviewSignal{}, Counts: map[string]int{"error": 0, "warn": 0, "note": 0}}
	if len(packets) == 0 {
		return result
	}
	const maxBins = 180
	n := min(maxBins, len(packets))
	result.Bins = make([]packetOverviewBin, n)
	signals := map[string]*packetOverviewSignal{}
	marks := make([]byte, len(packets))
	rank := func(level string) int {
		switch level {
		case "error":
			return 3
		case "warn":
			return 2
		case "note":
			return 1
		}
		return 0
	}
	for i, p := range packets {
		bin := min(n-1, i*n/len(packets))
		b := &result.Bins[bin]
		if b.Protocol == "" {
			b.Protocol = p.Protocol
		}
		id, level, title, explanation := packetClue(p)
		if p.Highlight != "" {
			level = p.Highlight
		}
		marks[i] = packetMapCode(p.Protocol, level)
		if id == "" {
			continue
		}
		if rank(level) > rank(b.Level) {
			b.Level = level
		}
		b.Interesting++
		result.Interesting++
		result.Counts[level]++
		item := signals[id]
		if item == nil {
			item = &packetOverviewSignal{ID: id, Severity: level, Title: title, Explanation: explanation, Number: p.Number, Index: i}
			signals[id] = item
		}
		item.Count++
	}
	result.PacketMarks = string(marks)
	// Stable priority ordering, with the earliest representative frame first.
	for _, id := range []string{"decoder-error", "capture-drop", "dns-failure", "dhcp-negative", "icmp-unreachable", "tcp-reset", "dns-negative", "tcp-sequence", "truncated", "decoder-note"} {
		if s := signals[id]; s != nil {
			result.Signals = append(result.Signals, *s)
		}
	}
	return result
}

func frameCaptureLayer(p capture.Packet, previous *capture.Packet) capture.Layer {
	fields := []capture.Field{}
	add := func(name, value string) {
		fields = append(fields, capture.Field{Name: name, Value: value, Offset: 0, Length: 0})
	}
	t := p.Time
	if !t.IsZero() {
		add("Arrival time (local)", t.Local().Format("2006-01-02 15:04:05.000000000 MST"))
		add("Arrival time (UTC)", t.UTC().Format("2006-01-02 15:04:05.000000000 MST"))
		add("Epoch time", fmt.Sprintf("%d.%09d", t.Unix(), t.Nanosecond()))
	}
	add("Time since first captured frame", fmt.Sprintf("%.9f seconds", p.Relative))
	if previous != nil && !t.IsZero() && !previous.Time.IsZero() {
		add("Time since previous captured frame", fmt.Sprintf("%.9f seconds", t.Sub(previous.Time).Seconds()))
	}
	add("Frame number", fmt.Sprint(p.Number))
	add("Frame length", fmt.Sprintf("%d bytes (%d bits)", p.Length, uint64(p.Length)*8))
	add("Captured length", fmt.Sprintf("%d bytes (%d bits)", p.Captured, uint64(p.Captured)*8))
	add("Interface", p.Interface)
	if p.Direction != "" {
		add("Direction", p.Direction)
	}
	add("Encapsulation", frameLinkType(p.Record.LinkType))
	names := make([]string, 0, len(p.Layers))
	for _, layer := range p.Layers {
		names = append(names, layer.Name)
	}
	add("Protocols in frame", strings.Join(names, " / "))
	return capture.Layer{Name: fmt.Sprintf("Frame %d: %d bytes on wire, %d bytes captured", p.Number, p.Length, p.Captured), Offset: 0, Length: p.Captured, Fields: fields}
}
func frameLinkType(t uint16) string {
	switch t {
	case 1:
		return "Ethernet (1)"
	case 0, 108:
		return "Loopback"
	case 101, 12:
		return "Raw IP"
	case 228:
		return "IPv4"
	case 229:
		return "IPv6"
	case 113:
		return "Linux cooked capture"
	case 276:
		return "Linux cooked v2"
	}
	return fmt.Sprintf("Link type %d", t)
}

// The packet capture record timestamp already carries its original time zone;
// local display conversion is presentation-only.
