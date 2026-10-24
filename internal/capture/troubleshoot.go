package capture

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"
)

// TroubleshootContext describes capture visibility, not the health of a network.
// Troubleshoot never sends traffic and never changes the supplied packets.
type TroubleshootContext struct {
	Active       bool
	Lost         uint64
	Overwritten  uint64
	KeepFiltered bool
	FileMode     bool
}

type TroubleshootMetrics struct {
	DNSQueries            int `json:"dnsQueries"`
	DNSResponses          int `json:"dnsResponses"`
	DNSErrors             int `json:"dnsErrors"`
	TCPConnectionAttempts int `json:"tcpConnectionAttempts"`
	TCPResets             int `json:"tcpResets"`
	ICMPErrors            int `json:"icmpErrors"`
	ARPRequests           int `json:"arpRequests"`
}

type TroubleshootFinding struct {
	ID            string   `json:"id"`
	Severity      string   `json:"severity"`
	Title         string   `json:"title"`
	Observation   string   `json:"observation"`
	Meaning       string   `json:"meaning"`
	NextSteps     []string `json:"nextSteps"`
	Count         int      `json:"count"`
	PacketNumbers []uint64 `json:"packetNumbers"`
	Filter        string   `json:"filter"`
}

type Troubleshooting struct {
	PacketCount int                   `json:"packetCount"`
	Duration    float64               `json:"duration"`
	Summary     string                `json:"summary"`
	Scope       string                `json:"scope"`
	Limitations []string              `json:"limitations"`
	Metrics     TroubleshootMetrics   `json:"metrics"`
	Findings    []TroubleshootFinding `json:"findings"`
}

type troubleFlow struct {
	iface, source, destination  string
	sourcePort, destinationPort uint16
}

func packetFlow(p Packet) troubleFlow {
	return troubleFlow{p.Interface, p.Source, p.Destination, p.SourcePort, p.DestinationPort}
}

func (f troubleFlow) reverse() troubleFlow {
	return troubleFlow{f.iface, f.destination, f.source, f.destinationPort, f.sourcePort}
}

type troubleRequest struct {
	first  time.Time
	number uint64
	count  int
	order  int
	name   string
}

type dnsTroubleKey struct {
	flow      troubleFlow
	transport string
	id        uint16
	question  [32]byte
}

type dnsTroubleExchange struct {
	flow      troubleFlow
	transport string
	id        uint16
}

func (k dnsTroubleKey) exchange() dnsTroubleExchange {
	return dnsTroubleExchange{k.flow, k.transport, k.id}
}

type tcpTroubleKey struct {
	flow     troubleFlow
	sequence uint32
}

type tcpTroubleSegment struct {
	payload []byte
	number  uint64
}

type tcpTroubleRequest struct {
	troubleRequest
	ack, dataAck uint32
	answered     bool
}

type arpTroubleKey struct{ iface, source, target string }

// Troubleshoot inspects all retained packets. Unanswered requests are reported
// only when at least three more seconds are visible in the capture. This is an
// observation window, not a protocol timeout or proof that a device is down.
func Troubleshoot(packets []Packet, context TroubleshootContext) Troubleshooting {
	r := Troubleshooting{PacketCount: len(packets), Scope: "All retained packets; display filters do not change these findings.", Limitations: []string{}, Findings: []TroubleshootFinding{}}
	r.Limitations = append(r.Limitations, "This capture shows traffic visible at the selected capture point. Traffic elsewhere, replies outside the capture window, and unsupported or encrypted content may be absent. A finding is a clue to investigate, not a confirmed root cause.")
	if context.Active {
		r.Limitations = append(r.Limitations, "Capture is still running. Findings can change as replies arrive.")
	}
	if context.KeepFiltered {
		r.Limitations = append(r.Limitations, "Capture keep filters excluded some traffic before analysis. A missing reply may have been filtered out.")
	}
	if context.Lost > 0 {
		r.Limitations = append(r.Limitations, fmt.Sprintf("Capture missed %d packet events. This is capture loss, not a measurement of network packet loss. Missing traffic can affect request/reply matching.", context.Lost))
	}
	if context.Overwritten > 0 {
		r.Limitations = append(r.Limitations, fmt.Sprintf("%d older packets were overwritten. Analysis covers the retained portion only.", context.Overwritten))
	}
	if context.FileMode {
		r.Limitations = append(r.Limitations, "Compatibility file capture can contain only the retained part of its circular recording. Traffic before that portion may be missing.")
	}
	if len(packets) == 0 {
		r.Summary = "No retained packets to analyze. Start a capture, reproduce the problem, then stop it, or open a capture file."
		return r
	}

	ordered := make([]*Packet, len(packets))
	var first, last, previous time.Time
	outOfOrder := false
	for i := range packets {
		p := &packets[i]
		ordered[i] = p
		if p.Time.IsZero() {
			continue
		}
		if first.IsZero() || p.Time.Before(first) {
			first = p.Time
		}
		if p.Time.After(last) {
			last = p.Time
		}
		if !previous.IsZero() && p.Time.Before(previous) {
			outOfOrder = true
		}
		previous = p.Time
	}
	if !first.IsZero() {
		r.Duration = max(0, last.Sub(first).Seconds())
	}
	if outOfOrder {
		sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Time.Before(ordered[j].Time) })
		r.Limitations = append(r.Limitations, "Packet timestamps arrived out of order. Request/reply checks use timestamp order; capture clock differences can still affect the result.")
	}
	if r.Duration < 3 {
		r.Limitations = append(r.Limitations, "Less than three seconds of packet timestamps are retained. Missing-reply checks need a longer observation window.")
	}

	findings := map[string]*TroubleshootFinding{}
	examples := map[string]uint64{}
	add := func(id, severity, title, observation, meaning string, next []string, numbers ...uint64) {
		f := findings[id]
		if f == nil {
			f = &TroubleshootFinding{ID: id, Severity: severity, Title: title, Observation: observation, Meaning: meaning, NextSteps: next, PacketNumbers: []uint64{}}
			findings[id] = f
		}
		f.Count++
		for _, n := range numbers {
			if n > 0 && (examples[id] == 0 || n < examples[id]) {
				examples[id] = n
				f.Observation = observation
			}
		}
		for _, n := range numbers {
			if n == 0 {
				continue
			}
			found := false
			for _, old := range f.PacketNumbers {
				found = found || old == n
			}
			if !found {
				if len(f.PacketNumbers) < 10 {
					f.PacketNumbers = append(f.PacketNumbers, n)
				} else {
					maxIndex := 0
					for i := 1; i < len(f.PacketNumbers); i++ {
						if f.PacketNumbers[i] > f.PacketNumbers[maxIndex] {
							maxIndex = i
						}
					}
					if n < f.PacketNumbers[maxIndex] {
						f.PacketNumbers[maxIndex] = n
					}
				}
			}
		}
	}
	dnsPending := map[dnsTroubleKey]*troubleRequest{}
	dnsOpaqueReplies := map[dnsTroubleExchange]int{}
	tcpPending := map[tcpTroubleKey]*tcpTroubleRequest{}
	tcpReplies := map[tcpTroubleKey]*tcpTroubleRequest{}
	uncertainStarts := map[tcpTroubleKey]bool{}
	arpPending := map[arpTroubleKey]*troubleRequest{}
	segments := map[tcpTroubleKey]tcpTroubleSegment{}
	truncated, malformed, fragments, encrypted, dnsUnsupported, uncertainSYNs := 0, 0, 0, 0, 0, 0
	firstInterface := ordered[0].Interface
	multipleInterfaces := false
	for position, pp := range ordered {
		p := *pp
		fragmented := false
		multipleInterfaces = multipleInterfaces || p.Interface != firstInterface
		if p.Record.DropReason != 0 {
			add("windows-local-drop", "attention", "Windows reported local packet drops", fmt.Sprintf("Windows recorded local drop events. Example: %s, reason code 0x%X.", troubleEndpoints(p), p.Record.DropReason), "This is evidence of a drop reported by the local Windows networking stack at the capture point. It does not measure packet loss across the whole network or identify a remote cause.", []string{"Open an example packet and note the Windows drop reason code.", "Compare the event time with Windows, firewall and adapter logs before changing a rule or driver."}, p.Number)
		}
		if p.Captured < int(p.Length) {
			truncated++
		}
		for _, e := range p.Expert {
			if strings.Contains(e, "fragment") {
				fragments++
				fragmented = true
				break
			}
		}
		for _, e := range p.Expert {
			if e == "Packet truncated by capture length" {
				continue
			}
			if strings.Contains(strings.ToLower(e), "invalid") || strings.Contains(strings.ToLower(e), "incomplete") || strings.Contains(strings.ToLower(e), "truncated") {
				malformed++
				break
			}
		}
		if p.Protocol == "TLS" || p.Protocol == "SSH" {
			encrypted++
		}

		if d, ok := dnsTroubleMessage(p); ok {
			flow := packetFlow(p)
			if d.response {
				r.Metrics.DNSResponses++
				key := dnsTroubleKey{flow.reverse(), p.Transport, d.id, d.question}
				delete(dnsPending, key)
				if !d.hasQuestion { // Some replies omit the echoed question.
					// Do not guess which same-ID question a question-less reply
					// answers, and do not scan every pending query per response.
					dnsOpaqueReplies[key.exchange()] = position
				}
				if d.rcode != 0 {
					r.Metrics.DNSErrors++
					id, title, meaning, next := dnsTroubleError(d.rcode)
					add(id, "attention", title, fmt.Sprintf("DNS error responses were captured. Example: resolver %s returned code %d for %s.", p.Source, d.rcode, troubleName(d.name)), meaning, next, p.Number)
				}
			} else {
				r.Metrics.DNSQueries++
				key := dnsTroubleKey{flow, p.Transport, d.id, d.question}
				q := dnsPending[key]
				replyAt, ambiguous := dnsOpaqueReplies[key.exchange()]
				if q != nil && !(ambiguous && replyAt >= q.order && replyAt < position) {
					q.count++
				} else {
					dnsPending[key] = &troubleRequest{first: p.Time, number: p.Number, count: 1, order: position, name: d.name}
				}
			}
		} else if p.Protocol == "DNS" && (p.SourcePort == 53 || p.DestinationPort == 53) {
			dnsUnsupported++
		}

		if p.Transport == "TCP" {
			flow := packetFlow(p)
			if p.TCPFlags&0x17 == 2 {
				key := tcpTroubleKey{flow, p.Sequence}
				if p.Captured < int(p.Length) || fragmented {
					uncertainSYNs++
					if tcpPending[key] == nil && !uncertainStarts[key] {
						r.Metrics.TCPConnectionAttempts++
						uncertainStarts[key] = true
					}
				} else if q := tcpPending[key]; q != nil && (!q.answered || p.Time.Sub(q.first) <= 3*time.Second) {
					q.count++
				} else {
					if uncertainStarts[key] {
						delete(uncertainStarts, key)
					} else {
						r.Metrics.TCPConnectionAttempts++
					}
					ack := p.Sequence + 1
					dataAck := ack
					if payload, ok := troublePayload(p); ok {
						dataAck += uint32(len(payload))
					}
					q := &tcpTroubleRequest{troubleRequest: troubleRequest{first: p.Time, number: p.Number, count: 1}, ack: ack, dataAck: dataAck}
					tcpPending[key] = q
					tcpReplies[tcpTroubleKey{flow, ack}] = q
					tcpReplies[tcpTroubleKey{flow, dataAck}] = q
				}
			}
			refused := false
			if p.TCPFlags&0x17 == 0x12 || p.TCPFlags&0x17 == 0x14 {
				key := tcpTroubleKey{flow.reverse(), p.Ack}
				if q := tcpReplies[key]; q != nil {
					if p.TCPFlags&4 != 0 {
						refused = true
						add("tcp-rejected", "attention", "A connection attempt was rejected", "A TCP reset acknowledged a captured connection request. Example: "+troubleFlowEndpoints(flow.reverse())+".", "The destination, or a device responding on its behalf, rejected the connection attempt. A service may not be listening on that port, or a policy may be rejecting it.", []string{"Check the destination IP address and service port.", "Use TCP ports to test the same service, then check the service and firewall logs on the destination."}, q.number, p.Number)
					}
					q.answered = true
					for _, ack := range []uint32{q.ack, q.dataAck} {
						candidate := tcpTroubleKey{flow.reverse(), ack}
						if tcpReplies[candidate] == q {
							delete(tcpReplies, candidate)
						}
					}
				}
			}
			if p.TCPFlags&4 != 0 {
				r.Metrics.TCPResets++
				if !refused {
					add("tcp-resets", "info", "Connections were reset", "TCP reset packets were captured. Example: "+troubleEndpoints(p)+".", "A reset abruptly ends or rejects a connection. This can be normal application behavior and does not, by itself, prove a firewall or network fault.", []string{"Check whether the reset happened when the reported problem occurred.", "Inspect the example packet addresses and ports, then compare with the application and firewall logs."}, p.Number)
				}
			}
			if payload, ok := troublePayload(p); ok && len(payload) > 0 {
				key := tcpTroubleKey{flow, p.Sequence}
				if old, exists := segments[key]; exists && bytes.Equal(old.payload, payload) {
					add("tcp-repeated-data", "info", "Repeated TCP data was captured", "The same TCP sequence number and payload appeared more than once in the same direction. Example: "+troubleEndpoints(p)+".", "This may be a retransmission, or the same traffic captured more than once. It is not a measured packet-loss percentage.", []string{"Look for repeated data near the reported delay.", "Capture on one adapter and compare both endpoints when possible. Check link errors and interface utilization before assuming packet loss."}, old.number, p.Number)
				} else {
					segments[key] = tcpTroubleSegment{payload, p.Number}
				}
			}
		}

		if kind, code, ok := troubleICMP(p); ok {
			v6 := p.Protocol == "ICMPv6"
			example := fmt.Sprintf(" Example: %s reported type %d, code %d to %s.", p.Source, kind, code, p.Destination)
			switch {
			case (!v6 && kind == 3 && code == 4) || (v6 && kind == 2):
				r.Metrics.ICMPErrors++
				add("icmp-size", "attention", "A path reported that packets were too large", "ICMP fragmentation-needed or Packet Too Big messages were captured."+example, "A device on the path requested smaller packets. This is useful evidence when some sites or VPN traffic stall while small requests work.", []string{"Check the MTU settings along the path, especially VPNs and tunnels.", "Check that required ICMP messages are allowed. Compare a small request with a larger one before changing MTU values."}, p.Number)
			case (!v6 && kind == 3) || (v6 && kind == 1):
				r.Metrics.ICMPErrors++
				add("icmp-unreachable", "attention", "A device reported unreachable traffic", "ICMP destination-unreachable messages were captured."+example, "A host or router reported that traffic could not be delivered. The ICMP code and quoted original packet help distinguish a route, port or policy issue.", []string{"Open an example packet and inspect its ICMP code and addresses.", "Check the destination service, routing and firewall logs. Use Trace route or TCP ports for a focused follow-up."}, p.Number)
			case (!v6 && kind == 11) || (v6 && kind == 3):
				r.Metrics.ICMPErrors++
				add("icmp-time-exceeded", "info", "Hop-limit messages were captured", "ICMP Time Exceeded messages were captured."+example, "These are expected during many traceroute tests. Outside a path test they may be a clue to a routing loop or an unusually long path.", []string{"Check whether Trace route or a similar path test was running.", "If these messages occur during normal application traffic, review the route and forwarding path."}, p.Number)
			}
		}

		if operation, ok := troubleARP(p); ok {
			if operation == 1 && p.Source != "0.0.0.0" && p.Source != p.Destination {
				r.Metrics.ARPRequests++
				key := arpTroubleKey{p.Interface, p.Source, p.Destination}
				if q := arpPending[key]; q != nil {
					q.count++
				} else {
					arpPending[key] = &troubleRequest{first: p.Time, number: p.Number, count: 1}
				}
			} else if operation == 2 {
				delete(arpPending, arpTroubleKey{p.Interface, p.Destination, p.Source})
			}
		}

		if p.Protocol == "HTTP" {
			words := strings.Fields(p.Info)
			if len(words) >= 2 && strings.HasPrefix(words[0], "HTTP/") {
				if status, err := strconv.Atoi(words[1]); err == nil && status >= 400 && status <= 599 {
					add("http-errors", "attention", "A web server returned error responses", fmt.Sprintf("Plaintext HTTP responses with status 400 through 599 were captured. Example: %s returned HTTP %d to %s.", p.Source, status, p.Destination), "The server returned an application-level error. A missing page or denied request can be expected; the status alone does not identify a network failure. Encrypted HTTPS responses cannot be inspected here.", []string{"Inspect the example status and requested service.", "Use HTTP check for the affected URL and compare the result with the web server or proxy logs."}, p.Number)
				}
			}
		}
	}
	oldEnough := func(q *troubleRequest) bool {
		return !q.first.IsZero() && !last.IsZero() && last.Sub(q.first) >= 3*time.Second
	}
	for key, q := range dnsPending {
		replyAt, ambiguous := dnsOpaqueReplies[key.exchange()]
		if oldEnough(q) && (!ambiguous || replyAt < q.order) {
			add("dns-no-visible-reply", "attention", "Some DNS requests have no visible reply", fmt.Sprintf("DNS requests had no matching response despite at least three more seconds of captured traffic. Example: %s asked resolver %s for %s.", key.flow.source, key.flow.destination, troubleName(q.name)), "The reply may be missing, outside the capture point, or excluded from the recording. This observation is not proof that the DNS server is down.", []string{"Use DNS lookup with the same name and resolver.", "Repeat the capture on the correct adapter with both directions and without a restrictive keep filter. Check resolver reachability and logs."}, q.number)
		}
	}
	for key, q := range tcpPending {
		if !q.answered && oldEnough(&q.troubleRequest) {
			add("tcp-no-visible-reply", "attention", "Some connection requests have no visible reply", "TCP SYN requests had no matching SYN-ACK or reset despite at least three more seconds of captured traffic. Example: "+troubleFlowEndpoints(key.flow)+".", "A response was not seen at this capture point. Possible explanations include a filtered request or reply, the wrong address, a missing route, an unavailable host, or incomplete capture visibility.", []string{"Use TCP ports to test the same destination and port.", "Check routes and firewall logs. If possible, compare captures at both endpoints before identifying where traffic is lost."}, q.number)
		}
	}
	for key, q := range arpPending {
		if q.count >= 3 && oldEnough(q) {
			add("arp-no-visible-reply", "info", "Repeated local address lookups have no visible reply", fmt.Sprintf("Repeated ARP requests had no matching reply in the retained capture. Example: %s repeatedly asked for local IP %s.", key.source, key.target), "The requested IP may be unused, which is normal during a network scan. If it should be active, a local address, VLAN, link or device issue is worth checking.", []string{"Confirm that the requested IP is expected to be online on this local network.", "Check the adapter subnet, VLAN and physical link. Compare the Local network and neighbor information before changing settings."}, q.number)
		}
	}
	if truncated > 0 {
		r.Limitations = append(r.Limitations, fmt.Sprintf("%d packets contain fewer captured bytes than their original length. Missing bytes may prevent complete protocol interpretation.", truncated))
	}
	if malformed > 0 {
		r.Limitations = append(r.Limitations, fmt.Sprintf("%d packets have incomplete or invalid decoded structures. Truncation and capture format can also cause this; it does not by itself prove bad traffic on the wire.", malformed))
	}
	if fragments > 0 {
		r.Limitations = append(r.Limitations, fmt.Sprintf("%d IP fragments were seen. This guide does not reassemble fragmented application messages.", fragments))
	}
	if encrypted > 0 {
		r.Limitations = append(r.Limitations, "Encrypted TLS or SSH traffic was seen. Packet timing and connection details remain visible, but the encrypted application content and HTTPS status are not decrypted.")
	}
	if len(dnsOpaqueReplies) > 0 {
		r.Limitations = append(r.Limitations, "Some DNS responses omit their question. Missing-reply checks skip these ambiguous exchanges instead of guessing which request they answer.")
	}
	if dnsUnsupported > 0 {
		r.Limitations = append(r.Limitations, fmt.Sprintf("%d DNS packets could not be assessed by the supported name-lookup checks. Other DNS operations, complex question lists, malformed records and oversized record lists are not interpreted as successful lookups.", dnsUnsupported))
	}
	if multipleInterfaces {
		r.Limitations = append(r.Limitations, "Request/reply checks match within the same recorded interface. A reply recorded on another interface or an asymmetric path may not be paired with its request.")
	}
	if uncertainSYNs > 0 {
		r.Limitations = append(r.Limitations, fmt.Sprintf("%d incomplete or fragmented TCP connection-start packets were not assessed for missing replies because their full on-wire payload length is uncertain.", uncertainSYNs))
	}
	for _, f := range findings {
		sort.Slice(f.PacketNumbers, func(i, j int) bool { return f.PacketNumbers[i] < f.PacketNumbers[j] })
		filters := make([]string, len(f.PacketNumbers))
		for i, n := range f.PacketNumbers {
			filters[i] = "frame.number == " + strconv.FormatUint(n, 10)
		}
		f.Filter = strings.Join(filters, " || ")
		f.Observation = fmt.Sprintf("%d observed. %s", f.Count, f.Observation)
		r.Findings = append(r.Findings, *f)
	}
	sort.Slice(r.Findings, func(i, j int) bool {
		if r.Findings[i].Severity != r.Findings[j].Severity {
			return r.Findings[i].Severity == "attention"
		}
		return r.Findings[i].ID < r.Findings[j].ID
	})
	if len(r.Findings) == 0 {
		r.Summary = "No supported warning patterns found in the retained traffic. This does not confirm that the network or application is healthy."
	} else {
		r.Summary = fmt.Sprintf("%d types of observation are worth reviewing. Open their example packets and use the suggested checks to investigate.", len(r.Findings))
	}
	return r
}

func troublePayload(p Packet) ([]byte, bool) {
	if p.PayloadOffset < 0 || p.PayloadLength < 0 || p.PayloadOffset > len(p.Record.Data) || p.PayloadLength > len(p.Record.Data)-p.PayloadOffset {
		return nil, false
	}
	return p.Record.Data[p.PayloadOffset : p.PayloadOffset+p.PayloadLength], true
}

func troubleFlowEndpoints(f troubleFlow) string {
	if f.source == "" {
		f.source = "unknown source"
	}
	if f.destination == "" {
		f.destination = "unknown destination"
	}
	if f.sourcePort == 0 && f.destinationPort == 0 {
		return f.source + " to " + f.destination
	}
	return net.JoinHostPort(f.source, strconv.Itoa(int(f.sourcePort))) + " to " + net.JoinHostPort(f.destination, strconv.Itoa(int(f.destinationPort)))
}

func troubleEndpoints(p Packet) string { return troubleFlowEndpoints(packetFlow(p)) }

func troubleName(name string) string {
	if name == "" {
		return "a name not included in the response question"
	}
	runes := []rune(name)
	if len(runes) > 160 {
		return string(runes[:160]) + "..."
	}
	return name
}

type troubleDNS struct {
	id          uint16
	response    bool
	rcode       uint16
	question    [32]byte
	name        string
	hasQuestion bool
}

func dnsTroubleMessage(p Packet) (troubleDNS, bool) {
	if p.Protocol != "DNS" || p.SourcePort != 53 && p.DestinationPort != 53 {
		return troubleDNS{}, false
	}
	for _, value := range []string{p.Source, p.Destination} {
		address, err := netip.ParseAddr(value)
		if err != nil || address.IsMulticast() {
			return troubleDNS{}, false
		}
	}
	d, ok := troublePayload(p)
	if !ok {
		return troubleDNS{}, false
	}
	if p.Transport == "TCP" {
		if len(d) < 2 {
			return troubleDNS{}, false
		}
		size := int(binary.BigEndian.Uint16(d))
		if size > len(d)-2 {
			return troubleDNS{}, false
		}
		d = d[2 : 2+size]
	} else if p.Transport != "UDP" {
		return troubleDNS{}, false
	}
	if len(d) < 12 {
		return troubleDNS{}, false
	}
	flags := binary.BigEndian.Uint16(d[2:])
	if flags>>11&15 != 0 {
		return troubleDNS{}, false
	} // Standard name lookups only.
	response := flags&0x8000 != 0
	if !response && p.DestinationPort != 53 || response && p.SourcePort != 53 {
		return troubleDNS{}, false
	}
	count := int(binary.BigEndian.Uint16(d[4:]))
	if count > 8 {
		return troubleDNS{}, false
	}
	// Keep the correlation key fixed-size even when compressed DNS names
	// expand to much more text than their original wire representation.
	questions := sha256.New()
	firstName := ""
	off := 12
	for i := 0; i < count; i++ {
		name, next, err := dnsName(d, off)
		if err != nil || next > len(d)-4 {
			return troubleDNS{}, false
		}
		if i == 0 {
			firstName = name
			if name == "" {
				firstName = "(DNS root)"
			}
		}
		prefix := [2]byte{byte(len(name) >> 8), byte(len(name))}
		questions.Write(prefix[:])
		questions.Write([]byte(strings.ToLower(name)))
		questions.Write(d[next : next+4])
		off = next + 4
	}
	rcode, ok := troubleDNSCode(d, off, flags&15)
	if !ok {
		return troubleDNS{}, false
	}
	var question [32]byte
	if count > 0 {
		copy(question[:], questions.Sum(question[:0]))
	}
	return troubleDNS{id: binary.BigEndian.Uint16(d), response: response, rcode: rcode, question: question, name: firstName, hasQuestion: count > 0}, true
}

// EDNS adds eight upper response-code bits in an OPT record's TTL. Walking
// bounded RR headers avoids calling BADVERS (16) success or code 19 NXDOMAIN.
func troubleDNSCode(d []byte, offset int, code uint16) (uint16, bool) {
	counts := [3]int{int(binary.BigEndian.Uint16(d[6:])), int(binary.BigEndian.Uint16(d[8:])), int(binary.BigEndian.Uint16(d[10:]))}
	if counts[0]+counts[1]+counts[2] > 2048 {
		return 0, false
	}
	optSeen := false
	for section, count := range counts {
		for n := 0; n < count; n++ {
			name, next, err := dnsName(d, offset)
			if err != nil || next > len(d)-10 {
				return 0, false
			}
			length := int(binary.BigEndian.Uint16(d[next+8:]))
			start := next + 10
			if length > len(d)-start {
				return 0, false
			}
			if section == 2 && binary.BigEndian.Uint16(d[next:]) == 41 {
				if optSeen || name != "" {
					return 0, false
				}
				optSeen = true
				code = uint16(d[next+4])<<4 | code
			}
			offset = start + length
		}
	}
	return code, true
}

func dnsTroubleError(code uint16) (string, string, string, []string) {
	switch code {
	case 3:
		return "dns-name-not-found", "A DNS server reported that a name does not exist", "The resolver returned NXDOMAIN. This can be a misspelled name, a missing DNS record or an expected background lookup; it is not the same as no DNS response.", []string{"Check the requested name for a typo and confirm that its DNS record should exist.", "Use DNS lookup against the same resolver and check the correct internal or public DNS zone."}
	case 2:
		return "dns-server-failure", "A DNS server could not complete a lookup", "The resolver returned SERVFAIL. The server answered, but could not complete that DNS lookup. Upstream resolution, DNSSEC validation or server configuration may need checking.", []string{"Use DNS lookup with the same name and resolver.", "Check resolver logs and upstream DNS access. Compare another permitted resolver without changing the client's DNS settings."}
	case 5:
		return "dns-refused", "A DNS server refused a lookup", "The resolver returned REFUSED. Its access or query policy may not permit this request.", []string{"Check that this client is using the intended DNS resolver.", "Review recursion, zone access and resolver policy for the client network."}
	default:
		return "dns-other-error", "DNS returned other error responses", "The DNS server answered with a nonzero response code. Inspect the code and query before deciding whether it relates to the reported problem.", []string{"Open the example packet and inspect its DNS flags and requested name.", "Repeat that lookup with DNS lookup and review the resolver logs."}
	}
}

func troubleICMP(p Packet) (byte, byte, bool) {
	if p.Protocol != "ICMP" && p.Protocol != "ICMPv6" || p.PayloadOffset < 4 || p.PayloadOffset > len(p.Record.Data) {
		return 0, 0, false
	}
	offset := p.PayloadOffset - 4
	return p.Record.Data[offset], p.Record.Data[offset+1], true
}

func troubleARP(p Packet) (uint16, bool) {
	if p.Protocol != "ARP" {
		return 0, false
	}
	d := p.Record.Data
	offset := 0
	switch p.Record.LinkType {
	case 1:
		if len(d) < 14 {
			return 0, false
		}
		offset = 14
		ether := binary.BigEndian.Uint16(d[12:])
		for i := 0; i < 4 && (ether == 0x8100 || ether == 0x88a8 || ether == 0x9100); i++ {
			if len(d) < offset+4 {
				return 0, false
			}
			ether = binary.BigEndian.Uint16(d[offset+2:])
			offset += 4
		}
		if ether != 0x806 {
			return 0, false
		}
	case 113:
		offset = 16
	case 276:
		offset = 20
	default:
		return 0, false
	}
	if offset > len(d)-28 || d[offset+4] != 6 || d[offset+5] != 4 || binary.BigEndian.Uint16(d[offset+2:]) != 0x800 {
		return 0, false
	}
	return binary.BigEndian.Uint16(d[offset+6:]), true
}
