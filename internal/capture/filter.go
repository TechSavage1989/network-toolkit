package capture

import (
	"errors"
	"fmt"
	"math"
	"net/netip"
	"strconv"
	"strings"
	"unicode"
)

type Predicate func(Packet) bool
type token struct {
	value  string
	quoted bool
}

func tokenize(text string) ([]token, error) {
	out := []token{}
	for i := 0; i < len(text); {
		if unicode.IsSpace(rune(text[i])) {
			i++
			continue
		}
		if len(out) >= 256 {
			return nil, errors.New("The display filter is too complex.")
		}
		if strings.ContainsRune("()", rune(text[i])) {
			out = append(out, token{value: text[i : i+1]})
			i++
			continue
		}
		if strings.ContainsRune("!<>=&|", rune(text[i])) {
			start := i
			i++
			if i < len(text) && strings.ContainsRune("=&|", rune(text[i])) {
				i++
			}
			out = append(out, token{value: text[start:i]})
			continue
		}
		if text[i] == '"' || text[i] == '\'' {
			quote := text[i]
			i++
			var b strings.Builder
			closed := false
			for i < len(text) {
				if text[i] == quote {
					i++
					closed = true
					break
				}
				if text[i] == '\\' && i+1 < len(text) {
					i++
				}
				b.WriteByte(text[i])
				i++
			}
			if !closed {
				return nil, errors.New("Close the quoted filter value.")
			}
			out = append(out, token{b.String(), true})
			continue
		}
		start := i
		for i < len(text) && !unicode.IsSpace(rune(text[i])) && !strings.ContainsRune("()!<>=&|", rune(text[i])) {
			i++
		}
		out = append(out, token{value: text[start:i]})
	}
	return out, nil
}

type parser struct {
	tokens []token
	index  int
	depth  int
}

func (r *parser) peek() string {
	if r.index >= len(r.tokens) {
		return ""
	}
	return strings.ToLower(r.tokens[r.index].value)
}
func (r *parser) take() token { t := r.tokens[r.index]; r.index++; return t }
func (r *parser) expression() (Predicate, error) {
	left, e := r.and()
	if e != nil {
		return nil, e
	}
	for r.peek() == "||" || r.peek() == "or" {
		r.take()
		right, e := r.and()
		if e != nil {
			return nil, e
		}
		a, b := left, right
		left = func(p Packet) bool { return a(p) || b(p) }
	}
	return left, nil
}
func (r *parser) and() (Predicate, error) {
	left, e := r.unary()
	if e != nil {
		return nil, e
	}
	for r.peek() == "&&" || r.peek() == "and" {
		r.take()
		right, e := r.unary()
		if e != nil {
			return nil, e
		}
		a, b := left, right
		left = func(p Packet) bool { return a(p) && b(p) }
	}
	return left, nil
}
func (r *parser) unary() (Predicate, error) {
	r.depth++
	defer func() { r.depth-- }()
	if r.depth > 32 {
		return nil, errors.New("The display filter is nested too deeply.")
	}
	if r.peek() == "!" || r.peek() == "not" {
		r.take()
		a, e := r.unary()
		return func(p Packet) bool { return !a(p) }, e
	}
	if r.peek() == "(" {
		r.take()
		a, e := r.expression()
		if e != nil {
			return nil, e
		}
		if r.peek() != ")" {
			return nil, errors.New("Close the filter parenthesis.")
		}
		r.take()
		return a, nil
	}
	if r.peek() == "" || r.peek() == ")" {
		return nil, errors.New("A filter condition is missing.")
	}
	name := strings.ToLower(r.take().value)
	op := r.peek()
	switch op {
	case "==", "!=", ">", ">=", "<", "<=", "contains":
		r.take()
		if r.index >= len(r.tokens) {
			return nil, errors.New("A comparison value is missing.")
		}
		value := r.take().value
		return comparison(name, op, value)
	default:
		return present(name)
	}
}
func present(name string) (Predicate, error) {
	switch name {
	case "tcp", "udp":
		return func(p Packet) bool { return strings.EqualFold(p.Transport, name) }, nil
	case "ip", "ipv4":
		return func(p Packet) bool { return p.IPVersion == 4 }, nil
	case "ipv6":
		return func(p Packet) bool { return p.IPVersion == 6 }, nil
	case "arp", "icmp", "icmpv6", "dns", "dhcp", "http", "tls", "ssh", "lldp", "cdp":
		return func(p Packet) bool { return strings.EqualFold(p.Protocol, name) }, nil
	case "tcp.flags.syn", "tcp.flags.ack", "tcp.flags.fin", "tcp.flags.reset", "tcp.flags.rst":
		bits := map[string]uint8{"syn": 2, "ack": 16, "fin": 1, "reset": 4, "rst": 4}
		flag := bits[name[strings.LastIndexByte(name, '.')+1:]]
		return func(p Packet) bool { return p.Transport == "TCP" && p.TCPFlags&flag != 0 }, nil
	case "dns.request", "dns.response":
		return func(p Packet) bool {
			message, ok := dnsTroubleMessage(p)
			return ok && message.response == (name == "dns.response")
		}, nil
	case "icmp.error":
		return func(p Packet) bool {
			kind, _, ok := troubleICMP(p)
			return ok && ((p.Protocol == "ICMP" && (kind == 3 || kind == 11)) || (p.Protocol == "ICMPv6" && (kind == 1 || kind == 2 || kind == 3)))
		}, nil
	case "frame.time_recorded":
		return func(p Packet) bool { return !p.Time.IsZero() }, nil
	case "expert", "_ws.expert":
		return func(p Packet) bool { return len(p.Expert) > 0 }, nil
	}
	return nil, fmt.Errorf("Unknown display field %q. Use the filter help for supported fields.", name)
}
func stringCompare(value, op, want string) bool {
	switch op {
	case "==":
		return strings.EqualFold(value, want)
	case "!=":
		return !strings.EqualFold(value, want)
	case "contains":
		return strings.Contains(strings.ToLower(value), strings.ToLower(want))
	}
	return false
}
func comparison(name, op, want string) (Predicate, error) {
	if strings.HasPrefix(name, "tcp.flags.") {
		a, e := present(name)
		if e != nil {
			return nil, e
		}
		value := want == "1" || strings.EqualFold(want, "true")
		if want != "0" && want != "1" && !strings.EqualFold(want, "true") && !strings.EqualFold(want, "false") {
			return nil, errors.New("TCP flag values are true, false, 1 or 0.")
		}
		if op != "==" && op != "!=" {
			return nil, errors.New("TCP flags use == or !=.")
		}
		return func(p Packet) bool {
			match := a(p) == value
			if op == "!=" {
				match = !match
			}
			return match
		}, nil
	}
	numeric := func(get func(Packet) float64) (Predicate, error) {
		n, e := strconv.ParseFloat(want, 64)
		if e != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return nil, errors.New("This display field requires a number.")
		}
		switch op {
		case "==", "!=", ">", ">=", "<", "<=":
		default:
			return nil, errors.New("Numeric fields use ==, !=, <, <=, > or >=.")
		}
		return func(p Packet) bool {
			v := get(p)
			switch op {
			case "==":
				return v == n
			case "!=":
				return v != n
			case ">":
				return v > n
			case ">=":
				return v >= n
			case "<":
				return v < n
			case "<=":
				return v <= n
			}
			return false
		}, nil
	}
	switch name {
	case "frame.number", "number":
		return numeric(func(p Packet) float64 { return float64(p.Number) })
	case "frame.len", "length":
		return numeric(func(p Packet) float64 { return float64(p.Length) })
	case "frame.cap_len":
		return numeric(func(p Packet) float64 { return float64(p.Captured) })
	case "frame.time_relative", "time":
		return numeric(func(p Packet) float64 { return p.Relative })
	case "tcp.port", "udp.port", "port", "tcp.srcport", "udp.srcport", "tcp.dstport", "udp.dstport":
		n, e := strconv.ParseUint(want, 10, 16)
		if e != nil {
			return nil, errors.New("A port filter requires a number from 0 to 65535.")
		}
		if op != "==" && op != "!=" {
			return nil, errors.New("Port filters use == or !=.")
		}
		return func(p Packet) bool {
			if strings.HasPrefix(name, "tcp.") && p.Transport != "TCP" || strings.HasPrefix(name, "udp.") && p.Transport != "UDP" {
				return false
			}
			match := p.SourcePort == uint16(n) || p.DestinationPort == uint16(n)
			if strings.HasSuffix(name, "srcport") {
				match = p.SourcePort == uint16(n)
			}
			if strings.HasSuffix(name, "dstport") {
				match = p.DestinationPort == uint16(n)
			}
			if op == "!=" {
				match = !match
			}
			return match
		}, nil
	case "vlan.id":
		n, e := strconv.ParseUint(want, 10, 12)
		if e != nil {
			return nil, errors.New("VLAN filters require a VLAN number.")
		}
		if op != "==" && op != "!=" {
			return nil, errors.New("VLAN filters use == or !=.")
		}
		return func(p Packet) bool {
			matched := false
			for _, v := range p.VLANs {
				matched = matched || v == uint16(n)
			}
			if op == "!=" {
				matched = !matched
			}
			return matched
		}, nil
	}
	if op != "==" && op != "!=" && op != "contains" {
		return nil, errors.New("Text and address fields use ==, != or contains.")
	}
	switch name {
	case "ip.addr", "ipv6.addr", "ip.src", "ipv6.src", "ip.dst", "ipv6.dst":
		if op != "contains" {
			if _, e := netip.ParseAddr(want); e != nil {
				if _, e = netip.ParsePrefix(want); e != nil {
					return nil, errors.New("Enter an IP address or CIDR network for this field.")
				}
			}
		}
		return func(p Packet) bool {
			if strings.HasPrefix(name, "ipv6") && p.IPVersion != 6 || strings.HasPrefix(name, "ip.") && p.IPVersion != 4 {
				return false
			}
			check := func(v string) bool {
				if op == "contains" {
					return strings.Contains(strings.ToLower(v), strings.ToLower(want))
				}
				return MatchesAddress(v, want)
			}
			matched := check(p.Source) || check(p.Destination)
			if strings.HasSuffix(name, ".src") {
				matched = check(p.Source)
			}
			if strings.HasSuffix(name, ".dst") {
				matched = check(p.Destination)
			}
			if op == "!=" {
				matched = !matched
			}
			return matched
		}, nil
	case "eth.addr", "eth.src", "eth.dst":
		return func(p Packet) bool {
			matched := stringCompare(p.SrcMAC, "==", want) || stringCompare(p.DstMAC, "==", want)
			if op == "contains" {
				matched = stringCompare(p.SrcMAC, op, want) || stringCompare(p.DstMAC, op, want)
			}
			if name == "eth.src" {
				return stringCompare(p.SrcMAC, op, want)
			}
			if name == "eth.dst" {
				return stringCompare(p.DstMAC, op, want)
			}
			if op == "!=" {
				matched = !matched
			}
			return matched
		}, nil
	case "frame.severity":
		if op != "==" && op != "!=" {
			return nil, errors.New("Severity filters use == or !=.")
		}
		if want != "error" && want != "warn" && want != "note" && want != "normal" {
			return nil, errors.New("Severity must be error, warn, note or normal.")
		}
		return func(p Packet) bool { return stringCompare(PacketSeverity(p), op, want) }, nil
	case "protocol", "frame.protocols":
		return func(p Packet) bool { return stringCompare(p.Protocol, op, want) }, nil
	case "info":
		return func(p Packet) bool { return stringCompare(p.Info, op, want) }, nil
	case "interface", "frame.interface_name":
		return func(p Packet) bool { return stringCompare(p.Interface, op, want) }, nil
	case "direction":
		return func(p Packet) bool { return stringCompare(p.Direction, op, want) }, nil
	case "tcp.stream", "udp.stream", "stream":
		return func(p Packet) bool { return stringCompare(p.Stream, op, want) }, nil
	case "expert":
		return func(p Packet) bool { return stringCompare(strings.Join(p.Expert, "; "), op, want) }, nil
	}
	return nil, fmt.Errorf("Unknown display field %q.", name)
}
func Compile(text string) (Predicate, error) {
	if len(text) > 4096 {
		return nil, errors.New("Display filters are limited to 4096 characters.")
	}
	tokens, e := tokenize(text)
	if e != nil {
		return nil, e
	}
	if len(tokens) == 0 {
		return func(Packet) bool { return true }, nil
	}
	r := parser{tokens: tokens}
	p, e := r.expression()
	if e != nil {
		return nil, e
	}
	if r.index != len(tokens) {
		return nil, errors.New("Join filter conditions with && or ||.")
	}
	return p, nil
}

// PacketSeverity describes recorded decoder/capture clues. A normal packet is
// not a claim that its conversation or the network succeeded.
func PacketSeverity(p Packet) string {
	if p.Highlight == "error" || p.Highlight == "warn" || p.Highlight == "note" {
		return p.Highlight
	}
	notes, info := strings.ToLower(strings.Join(p.Expert, " ")), strings.ToLower(p.Info)
	if strings.Contains(notes, "malformed") || strings.Contains(notes, "invalid") || strings.Contains(notes, "incomplete") {
		return "error"
	}
	if strings.Contains(notes, "drop reason") || p.Protocol == "DNS" && (strings.Contains(info, "servfail") || strings.Contains(info, "refused")) || p.Protocol == "DHCP" && (strings.Contains(info, "nak") || strings.Contains(info, "decline")) || (p.Protocol == "ICMP" || p.Protocol == "ICMPv6") && (strings.Contains(info, "unreachable") || strings.Contains(info, "administratively prohibited")) {
		return "warn"
	}
	if p.Transport == "TCP" && p.TCPFlags&4 != 0 || notes != "" || p.Protocol == "DNS" && strings.Contains(info, "nxdomain") || p.Transport == "TCP" && (strings.Contains(info, "retransmission") || strings.Contains(info, "out-of-order")) {
		return "note"
	}
	return "normal"
}
