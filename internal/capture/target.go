package capture

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// A connection check answers "where did it stop?" for one website or device
// the user names: name lookup, then reaching the device, then the connection,
// then the reply from the service. Each step reads only the recording; nothing
// is sent. A step the capture cannot judge is "unknown", never "ok".

type TargetStep struct {
	ID            string   `json:"id"`     // lookup, reach, connect, respond
	Label         string   `json:"label"`  // short step name
	State         string   `json:"state"`  // ok, fail, warn, unknown, skipped
	Detail        string   `json:"detail"` // plain explanation
	PacketNumbers []uint64 `json:"packetNumbers"`
	Filter        string   `json:"filter"`
}

type TargetCheck struct {
	Target     string       `json:"target"`
	Host       string       `json:"host"`
	Port       uint16       `json:"port,omitempty"`
	Service    string       `json:"service,omitempty"`
	Addresses  []string     `json:"addresses"`
	State      string       `json:"state"` // ok, problem, warn, unknown
	Conclusion string       `json:"conclusion"`
	Steps      []TargetStep `json:"steps"`
}

var schemePorts = map[string]uint16{"http": 80, "https": 443, "ftp": 21, "ssh": 22, "telnet": 23, "rdp": 3389, "smb": 445}

// ParseTarget accepts a website address, host name, host:port or IP address.
func ParseTarget(raw string) (host string, port uint16, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 512 {
		return "", 0, fmt.Errorf("Enter a website or device name, or an IP address.")
	}
	if strings.Contains(raw, "://") {
		u, e := url.Parse(raw)
		if e != nil || u.Hostname() == "" {
			return "", 0, fmt.Errorf("That website address could not be read.")
		}
		host = u.Hostname()
		if p := u.Port(); p != "" {
			n, e := strconv.Atoi(p)
			if e != nil || n < 1 || n > 65535 {
				return "", 0, fmt.Errorf("The port must be from 1 to 65535.")
			}
			port = uint16(n)
		} else {
			port = schemePorts[strings.ToLower(u.Scheme)]
		}
	} else {
		raw = strings.SplitN(raw, "/", 2)[0]
		if h, p, e := net.SplitHostPort(raw); e == nil {
			n, e := strconv.Atoi(p)
			if e != nil || n < 1 || n > 65535 {
				return "", 0, fmt.Errorf("The port must be from 1 to 65535.")
			}
			host, port = h, uint16(n)
		} else {
			host = strings.Trim(raw, "[]")
		}
	}
	host = cleanHost(host)
	if a, e := netip.ParseAddr(host); e == nil {
		return a.Unmap().String(), port, nil
	}
	if len(host) > 253 || host == "" {
		return "", 0, fmt.Errorf("Enter a website or device name, or an IP address.")
	}
	for _, r := range host {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_') {
			return "", 0, fmt.Errorf("Enter a website or device name, or an IP address.")
		}
	}
	return host, port, nil
}

func nameMatches(name, host string) bool {
	name = cleanHost(name)
	return name == host || strings.HasPrefix(name, host+".")
}

func numbersFilter(numbers []uint64) string {
	sort.Slice(numbers, func(i, j int) bool { return numbers[i] < numbers[j] })
	if len(numbers) > 10 {
		numbers = numbers[:10]
	}
	parts := make([]string, len(numbers))
	for i, n := range numbers {
		parts[i] = "frame.number == " + strconv.FormatUint(n, 10)
	}
	return strings.Join(parts, " || ")
}

func step(id, label, state, detail string, numbers ...uint64) TargetStep {
	kept := []uint64{}
	for _, n := range numbers {
		if n > 0 && len(kept) < 10 {
			kept = append(kept, n)
		}
	}
	return TargetStep{ID: id, Label: label, State: state, Detail: detail, PacketNumbers: kept, Filter: numbersFilter(append([]uint64{}, kept...))}
}

// icmpQuotedDestination returns the destination of the packet quoted inside an
// ICMP or ICMPv6 error message.
func icmpQuotedDestination(p Packet) string {
	d := p.Record.Data
	start := p.PayloadOffset + 4 // after the 8-byte ICMP header
	if p.PayloadOffset < 4 || start < 0 || start >= len(d) {
		return ""
	}
	if p.Protocol == "ICMP" && start+20 <= len(d) && d[start]>>4 == 4 {
		return netip.AddrFrom4([4]byte(d[start+16 : start+20])).String()
	}
	if p.Protocol == "ICMPv6" && start+40 <= len(d) && d[start]>>4 == 6 {
		return netip.AddrFrom16([16]byte(d[start+24 : start+40])).Unmap().String()
	}
	return ""
}

// CheckTarget follows one website or device through the recording.
func CheckTarget(packets []Packet, target string) (TargetCheck, error) {
	host, port, err := ParseTarget(target)
	if err != nil {
		return TargetCheck{}, err
	}
	c := TargetCheck{Target: strings.TrimSpace(target), Host: host, Port: port, Service: ServiceName(port), Addresses: []string{}, Steps: []TargetStep{}}
	ordered := make([]Packet, len(packets))
	copy(ordered, packets)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Time.Before(ordered[j].Time) })
	var last time.Time
	for _, p := range ordered {
		if p.Time.After(last) {
			last = p.Time
		}
	}
	addresses := map[string]bool{}
	addAddress := func(a string) {
		if a != "" && !addresses[a] {
			addresses[a] = true
			c.Addresses = append(c.Addresses, a)
		}
	}
	_, isIP := netip.ParseAddr(host)
	isAddress := isIP == nil
	who := host
	service := "the service"
	if port != 0 {
		service = ServiceLabel(port)
	}

	// 1. Name lookup.
	if isAddress {
		addAddress(host)
		c.Steps = append(c.Steps, step("lookup", "Name lookup", "skipped", "Not needed: you entered an IP address."))
	} else {
		var answered, failed, empty, queries []uint64
		var failCode uint16
		var firstQuery time.Time
		resolver := ""
		for _, p := range ordered {
			if question, found, code, ok := dnsAnswerAddresses(p); ok && nameMatches(question, host) {
				switch {
				case code != 0:
					failed, failCode = append(failed, p.Number), code
				case len(found) > 0:
					answered, resolver = append(answered, p.Number), p.Source
					for _, a := range found {
						addAddress(a)
					}
				default:
					empty = append(empty, p.Number)
				}
				continue
			}
			if d, ok := dnsTroubleMessage(p); ok && !d.response && nameMatches(d.name, host) {
				if firstQuery.IsZero() {
					firstQuery = p.Time
				}
				queries = append(queries, p.Number)
			}
		}
		// Encrypted websites name themselves in their first message (SNI), so
		// a cached lookup can still be followed.
		var sni []uint64
		for _, p := range ordered {
			if p.Protocol == "TLS" && strings.HasPrefix(p.Info, "TLS Client Hello, SNI ") && nameMatches(strings.TrimPrefix(p.Info, "TLS Client Hello, SNI "), host) {
				sni = append(sni, p.Number)
				if len(answered) == 0 {
					addAddress(p.Destination)
				}
			}
		}
		switch {
		case len(answered) > 0:
			c.Steps = append(c.Steps, step("lookup", "Name lookup", "ok", fmt.Sprintf("The DNS server %s found %s at %s.", resolver, host, strings.Join(c.Addresses, ", ")), append(queries, answered...)...))
		case len(failed) > 0 && failCode == 3:
			c.Steps = append(c.Steps, step("lookup", "Name lookup", "fail", "The DNS server says "+host+" does not exist. Check the spelling, or whether its DNS record is missing.", append(queries, failed...)...))
		case len(failed) > 0:
			c.Steps = append(c.Steps, step("lookup", "Name lookup", "fail", fmt.Sprintf("The DNS server could not look up %s (error %s).", host, dnsCodeName(failCode)), append(queries, failed...)...))
		case len(empty) > 0 && len(c.Addresses) == 0:
			c.Steps = append(c.Steps, step("lookup", "Name lookup", "warn", "The DNS server answered, but gave no address for "+host+".", append(queries, empty...)...))
		case len(queries) > 0 && len(c.Addresses) == 0:
			state := "fail"
			detail := "This computer asked for " + host + ", but no answer came back from the DNS server."
			if last.Sub(firstQuery) < 3*time.Second {
				state, detail = "warn", "This computer asked for "+host+", but the recording ended before an answer could arrive."
			}
			c.Steps = append(c.Steps, step("lookup", "Name lookup", state, detail, queries...))
		case len(sni) > 0:
			c.Steps = append(c.Steps, step("lookup", "Name lookup", "ok", "No lookup was recorded (this computer probably remembered the address), but it connected to "+host+" at "+strings.Join(c.Addresses, ", ")+".", sni...))
		default:
			c.Steps = append(c.Steps, step("lookup", "Name lookup", "unknown", "No lookup of "+host+" was recorded. This computer may have remembered the address from earlier, or used encrypted DNS."))
		}
	}
	if len(c.Addresses) > 0 && !isAddress {
		who = host + " (" + strings.Join(c.Addresses, ", ") + ")"
	}

	// 2. Reaching the device.
	var sent, back, unreachable []uint64
	var firstSent time.Time
	reporter, reason := "", ""
	for _, p := range ordered {
		switch {
		case addresses[p.Source]:
			back = append(back, p.Number)
		case addresses[p.Destination]:
			if firstSent.IsZero() {
				firstSent = p.Time
			}
			sent = append(sent, p.Number)
		}
		if kind, code, ok := troubleICMP(p); ok && ((p.Protocol == "ICMP" && kind == 3) || (p.Protocol == "ICMPv6" && kind == 1)) && addresses[icmpQuotedDestination(p)] {
			unreachable = append(unreachable, p.Number)
			reporter, reason = p.Source, icmpUnreachableReason(p.Protocol == "ICMPv6", code)
		}
	}
	reached := false
	switch {
	case len(c.Addresses) == 0:
		c.Steps = append(c.Steps, step("reach", "Reach the device", "unknown", "There is no address to follow, so this step cannot be checked."))
	case len(back) > 0:
		reached = true
		c.Steps = append(c.Steps, step("reach", "Reach the device", "ok", fmt.Sprintf("%s answered: %d %s came back.", capitalize(who), len(back), plural(len(back), "packet", "packets")), back...))
	case len(unreachable) > 0:
		c.Steps = append(c.Steps, step("reach", "Reach the device", "fail", reporter+" reported that "+who+" cannot be reached: "+reason+".", unreachable...))
	case len(sent) > 0 && last.Sub(firstSent) < 3*time.Second:
		c.Steps = append(c.Steps, step("reach", "Reach the device", "warn", "Traffic was sent to "+who+", but the recording ended before a reply could arrive.", sent...))
	case len(sent) > 0:
		c.Steps = append(c.Steps, step("reach", "Reach the device", "fail", fmt.Sprintf("%d %s went to %s, but nothing came back. It may be switched off or unreachable, the address may be wrong, or a firewall may be dropping the traffic.", len(sent), plural(len(sent), "packet", "packets"), who), sent...))
	default:
		c.Steps = append(c.Steps, step("reach", "Reach the device", "unknown", "No traffic to "+who+" was recorded. Start a capture, then try it while it records."))
	}

	// 3. The connection (TCP), 4. the reply from the service.
	type attempt struct {
		syn, answer uint64
		accepted    bool
		refused     bool
		start       time.Time
		port        uint16
	}
	attempts := map[string]*attempt{}
	var order []string
	var established, serverData, httpOK, httpErr, tlsBack, resets []uint64
	httpStatus := 0
	for _, p := range ordered {
		if p.Transport != "TCP" {
			continue
		}
		toTarget, fromTarget := addresses[p.Destination], addresses[p.Source]
		if !toTarget && !fromTarget {
			continue
		}
		serverPort := p.DestinationPort
		if fromTarget {
			serverPort = p.SourcePort
		}
		if port != 0 && serverPort != port {
			continue
		}
		key := p.Stream
		flags := p.TCPFlags & 0x17
		if toTarget && flags == 0x02 {
			if attempts[key] == nil {
				attempts[key] = &attempt{syn: p.Number, start: p.Time, port: serverPort}
				order = append(order, key)
			}
			continue
		}
		a := attempts[key]
		if fromTarget && a != nil && !a.accepted && !a.refused {
			if flags == 0x12 {
				a.accepted, a.answer = true, p.Number
				continue
			}
			if flags&0x04 != 0 {
				a.refused, a.answer = true, p.Number
				continue
			}
		}
		if a == nil && fromTarget && p.PayloadLength > 0 {
			established = append(established, p.Number)
		}
		if fromTarget && flags&0x04 != 0 && (a == nil || a.accepted) {
			resets = append(resets, p.Number)
		}
		if fromTarget && p.PayloadLength > 0 {
			serverData = append(serverData, p.Number)
			switch {
			case p.Protocol == "HTTP" && strings.HasPrefix(p.Info, "HTTP/"):
				if f := strings.Fields(p.Info); len(f) >= 2 {
					if status, e := strconv.Atoi(f[1]); e == nil {
						if status >= 400 {
							httpErr, httpStatus = append(httpErr, p.Number), status
						} else {
							httpOK = append(httpOK, p.Number)
							if httpStatus == 0 {
								httpStatus = status
							}
						}
					}
				}
			case p.Protocol == "TLS":
				tlsBack = append(tlsBack, p.Number)
			}
		}
	}
	var accepted, refused, silent []uint64
	connectedPort := port
	for _, key := range order {
		a := attempts[key]
		switch {
		case a.accepted:
			accepted = append(accepted, a.syn, a.answer)
			connectedPort = a.port
		case a.refused:
			refused = append(refused, a.syn, a.answer)
			if connectedPort == 0 {
				connectedPort = a.port
			}
		case last.Sub(a.start) >= 3*time.Second:
			silent = append(silent, a.syn)
			if connectedPort == 0 {
				connectedPort = a.port
			}
		}
	}
	if connectedPort != 0 {
		service = ServiceLabel(connectedPort)
		if c.Service == "" {
			c.Service = ServiceName(connectedPort)
		}
	}
	connected := false
	switch {
	case len(accepted) > 0:
		connected = true
		detail := capitalize(service) + " on " + who + " accepted the connection."
		if len(refused)+len(silent) > 0 {
			detail += fmt.Sprintf(" %d other %s refused or not answered.", (len(refused)/2)+len(silent), plural((len(refused)/2)+len(silent), "attempt was", "attempts were"))
		}
		c.Steps = append(c.Steps, step("connect", "Connect", "ok", detail, accepted...))
	case len(refused) > 0:
		c.Steps = append(c.Steps, step("connect", "Connect", "fail", capitalize(service)+" on "+who+" refused the connection. Usually nothing is running on that port, or a firewall or policy is rejecting it.", refused...))
	case len(silent) > 0:
		c.Steps = append(c.Steps, step("connect", "Connect", "fail", capitalize(service)+" on "+who+" did not answer the connection request. A firewall may be dropping it, or the service is not reachable.", silent...))
	case len(established) > 0:
		connected = true
		c.Steps = append(c.Steps, step("connect", "Connect", "ok", "A connection to "+who+" that was already open was used.", established...))
	case len(attempts) > 0:
		c.Steps = append(c.Steps, step("connect", "Connect", "warn", "A connection to "+who+" was started, but the recording ended before it could be answered."))
	case port != 0:
		c.Steps = append(c.Steps, step("connect", "Connect", "unknown", "No connection to "+service+" on "+who+" was recorded."))
	case reached || len(sent) > 0:
		c.Steps = append(c.Steps, step("connect", "Connect", "skipped", "No connection was opened (for example, only Ping was used)."))
	default:
		c.Steps = append(c.Steps, step("connect", "Connect", "unknown", "No connection to "+who+" was recorded."))
	}

	switch {
	case !connected:
		c.Steps = append(c.Steps, step("respond", "Service reply", "skipped", "Not reached: the connection did not open."))
	case len(httpErr) > 0 && len(httpOK) == 0:
		c.Steps = append(c.Steps, step("respond", "Service reply", "fail", "The website answered with an error: "+httpStatusText(httpStatus)+". The network worked; the problem is on the website or its server.", httpErr...))
	case len(httpOK) > 0:
		c.Steps = append(c.Steps, step("respond", "Service reply", "ok", fmt.Sprintf("The website answered normally (status %d).", httpStatus), httpOK...))
	case len(tlsBack) > 0 && len(resets) == 0:
		c.Steps = append(c.Steps, step("respond", "Service reply", "ok", "A secure (encrypted) session started and the server sent data back. Its content is encrypted, so it cannot be read here.", tlsBack...))
	case len(resets) > 0:
		c.Steps = append(c.Steps, step("respond", "Service reply", "warn", capitalize(who)+" cut the connection off after it opened. This can be normal, but if it happened when the problem occurred, check the service or its logs.", resets...))
	case len(serverData) > 0:
		c.Steps = append(c.Steps, step("respond", "Service reply", "ok", capitalize(service)+" sent data back.", serverData...))
	default:
		c.Steps = append(c.Steps, step("respond", "Service reply", "warn", "The connection opened, but no data came back from "+who+" in this recording."))
	}

	c.State, c.Conclusion = "ok", "Everything this capture shows for "+host+" worked. If the problem continues, it may be in the application itself, or it happened outside this recording."
	known := false
	for _, s := range c.Steps {
		if s.State == "ok" || s.State == "fail" || s.State == "warn" {
			known = true
		}
	}
	for _, s := range c.Steps {
		if s.State == "fail" {
			c.State = "problem"
			c.Conclusion = map[string]string{
				"lookup":  "It stops at the name lookup: " + host + " could not be turned into an address. Check the spelling and the DNS settings.",
				"reach":   "It stops before the device answers: " + who + " never replied. Check that it is on, its address, and any firewall in between.",
				"connect": "The device is there, but the connection was not accepted: check that " + service + " is running on it and that no firewall blocks it.",
				"respond": "The network path works; the website or service itself returned an error.",
			}[s.ID]
			return c, nil
		}
	}
	for _, s := range c.Steps {
		if s.State == "warn" {
			c.State, c.Conclusion = "warn", "Nothing clearly failed for "+host+", but one step needs a closer look: "+s.Detail
			return c, nil
		}
	}
	if !known {
		c.State, c.Conclusion = "unknown", "This capture has no traffic for "+host+". Start a capture, then try "+host+" while it records."
	}
	return c, nil
}
