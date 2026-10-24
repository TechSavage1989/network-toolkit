package capture

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"unicode"
)

func Decode(record Record) Packet {
	p := Packet{Time: record.Time, Length: record.OriginalLength, Captured: len(record.Data), Interface: record.Interface, Direction: record.Direction, Protocol: "Unknown", Record: record, Layers: []Layer{}, Expert: []string{}}
	if p.Length == 0 {
		p.Length = uint32(len(record.Data))
	}
	if p.Captured < int(p.Length) {
		p.Expert = append(p.Expert, "Packet truncated by capture length")
	}
	if record.DropReason != 0 {
		p.Expert = append(p.Expert, fmt.Sprintf("Windows drop reason 0x%X", record.DropReason))
	}
	d := record.Data
	offset, ether := 0, uint16(0)
	layer := func(name string, off, n int, fields ...Field) {
		p.Layers = append(p.Layers, Layer{Name: name, Offset: off, Length: n, Fields: fields})
	}
	field := func(name, value string, off, n int) Field {
		return Field{Name: name, Value: value, Offset: off, Length: n}
	}
	malformed := func(text string) Packet {
		p.Expert = append(p.Expert, text)
		if p.Info == "" {
			p.Info = text
		}
		return p
	}
	switch record.LinkType {
	case 1:
		if len(d) < 14 {
			return malformed("Incomplete Ethernet header")
		}
		p.DstMAC = net.HardwareAddr(d[:6]).String()
		p.SrcMAC = net.HardwareAddr(d[6:12]).String()
		p.Source, p.Destination = p.SrcMAC, p.DstMAC
		ether = binary.BigEndian.Uint16(d[12:])
		offset = 14
		layer("Ethernet", 0, 14, field("Destination", p.DstMAC, 0, 6), field("Source", p.SrcMAC, 6, 6), field("EtherType", fmt.Sprintf("0x%04X", ether), 12, 2))
		for i := 0; i < 4 && (ether == 0x8100 || ether == 0x88a8 || ether == 0x9100); i++ {
			if len(d) < offset+4 {
				return malformed("Incomplete VLAN header")
			}
			tci := binary.BigEndian.Uint16(d[offset:])
			v := tci & 0xfff
			p.VLANs = append(p.VLANs, v)
			layer("802.1Q VLAN", offset, 4, field("VLAN", fmt.Sprint(v), offset, 2), field("Priority", fmt.Sprint(tci>>13), offset, 2))
			ether = binary.BigEndian.Uint16(d[offset+2:])
			offset += 4
		}
		if ether <= 1500 && len(d) >= offset+8 && d[offset] == 0xaa && d[offset+1] == 0xaa {
			oui := d[offset+3 : offset+6]
			pid := binary.BigEndian.Uint16(d[offset+6:])
			if oui[0] == 0 && oui[1] == 0 && oui[2] == 0x0c && pid == 0x2000 {
				p.Protocol = "CDP"
				p.Info = decodeCDP(d[offset+8:])
				layer("Cisco Discovery Protocol", offset+8, len(d)-offset-8, field("Summary", p.Info, offset+8, len(d)-offset-8))
				return p
			}
			ether = pid
			offset += 8
		}
	case 101, 12:
		if len(d) == 0 {
			return malformed("Empty IP packet")
		}
		if d[0]>>4 == 4 {
			ether = 0x800
		} else if d[0]>>4 == 6 {
			ether = 0x86dd
		}
	case 228:
		ether = 0x800
	case 229:
		ether = 0x86dd
	case 0, 108:
		if len(d) < 4 {
			return malformed("Incomplete loopback header")
		}
		family := binary.LittleEndian.Uint32(d)
		if record.LinkType == 108 {
			family = binary.BigEndian.Uint32(d)
		}
		offset = 4
		if family == 2 {
			ether = 0x800
		} else if family == 23 || family == 24 || family == 28 || family == 30 {
			ether = 0x86dd
		}
		layer("Loopback", 0, 4, field("Address family", fmt.Sprint(family), 0, 4))
	case 113:
		if len(d) < 16 {
			return malformed("Incomplete Linux cooked header")
		}
		ether = binary.BigEndian.Uint16(d[14:])
		offset = 16
		layer("Linux cooked capture", 0, 16)
	case 276:
		if len(d) < 20 {
			return malformed("Incomplete Linux cooked v2 header")
		}
		ether = binary.BigEndian.Uint16(d)
		offset = 20
		layer("Linux cooked capture v2", 0, 20)
	default:
		p.Info = fmt.Sprintf("Link type %d, %d captured bytes", record.LinkType, len(d))
		return p
	}
	if ether == 0x88cc {
		p.Protocol = "LLDP"
		p.Info = decodeLLDP(d[offset:])
		layer("Link Layer Discovery Protocol", offset, len(d)-offset, field("Summary", p.Info, offset, len(d)-offset))
		return p
	}
	if ether == 0x806 {
		p.Protocol = "ARP"
		if len(d) < offset+8 {
			return malformed("Incomplete ARP header")
		}
		hlen, plen := int(d[offset+4]), int(d[offset+5])
		need := 8 + 2*(hlen+plen)
		if len(d) < offset+need {
			return malformed("Incomplete ARP addresses")
		}
		op := binary.BigEndian.Uint16(d[offset+6:])
		if hlen == 6 && plen == 4 {
			sender := net.IP(d[offset+8+hlen : offset+8+hlen+4]).String()
			target := net.IP(d[offset+8+2*hlen+plen : offset+need]).String()
			p.Source, p.Destination = sender, target
			if op == 1 {
				p.Info = "Who has " + target + "? Tell " + sender
			} else {
				p.Info = sender + " is at " + net.HardwareAddr(d[offset+8:offset+8+hlen]).String()
			}
		} else {
			p.Info = fmt.Sprintf("ARP operation %d", op)
		}
		layer("Address Resolution Protocol", offset, need, field("Operation", fmt.Sprint(op), offset+6, 2), field("Sender IP", p.Source, offset+8+hlen, plen), field("Target IP", p.Destination, offset+8+2*hlen+plen, plen))
		return p
	}
	proto, end, fragment := byte(0), len(d), false
	switch ether {
	case 0x800:
		p.IPVersion = 4
		p.Protocol = "IPv4"
		if len(d) < offset+20 || d[offset]>>4 != 4 {
			return malformed("Incomplete or invalid IPv4 header")
		}
		ihl := int(d[offset]&15) * 4
		total := int(binary.BigEndian.Uint16(d[offset+2:]))
		if ihl < 20 || total < ihl || offset+ihl > len(d) {
			return malformed("Invalid IPv4 header length")
		}
		src, _ := netip.AddrFromSlice(d[offset+12 : offset+16])
		dst, _ := netip.AddrFromSlice(d[offset+16 : offset+20])
		p.Source, p.Destination = src.String(), dst.String()
		proto = d[offset+9]
		flags := binary.BigEndian.Uint16(d[offset+6:])
		fragment = flags&0x1fff != 0
		if flags&0x3fff != 0 {
			p.Expert = append(p.Expert, "IPv4 fragment; transport data may require reassembly")
		}
		if offset+total < end {
			end = offset + total
		}
		layer("Internet Protocol v4", offset, ihl, field("Source", p.Source, offset+12, 4), field("Destination", p.Destination, offset+16, 4), field("TTL", fmt.Sprint(d[offset+8]), offset+8, 1), field("Protocol", fmt.Sprint(proto), offset+9, 1), field("Total length", fmt.Sprint(total), offset+2, 2), field("Identification", fmt.Sprint(binary.BigEndian.Uint16(d[offset+4:])), offset+4, 2), field("Flags / fragment offset", fmt.Sprintf("0x%04X", flags), offset+6, 2))
		offset += ihl
	case 0x86dd:
		p.IPVersion = 6
		p.Protocol = "IPv6"
		if len(d) < offset+40 || d[offset]>>4 != 6 {
			return malformed("Incomplete or invalid IPv6 header")
		}
		src, _ := netip.AddrFromSlice(d[offset+8 : offset+24])
		dst, _ := netip.AddrFromSlice(d[offset+24 : offset+40])
		p.Source, p.Destination = src.String(), dst.String()
		proto = d[offset+6]
		total := 40 + int(binary.BigEndian.Uint16(d[offset+4:]))
		if offset+total < end && total > 40 {
			end = offset + total
		}
		layer("Internet Protocol v6", offset, 40, field("Source", p.Source, offset+8, 16), field("Destination", p.Destination, offset+24, 16), field("Hop limit", fmt.Sprint(d[offset+7]), offset+7, 1), field("Next header", fmt.Sprint(proto), offset+6, 1))
		offset += 40
		for n := 0; n < 8 && (proto == 0 || proto == 43 || proto == 44 || proto == 60 || proto == 51); n++ {
			if end-offset < 8 {
				return malformed("Incomplete IPv6 extension header")
			}
			next := d[offset]
			length := 8
			if proto == 44 {
				bits := binary.BigEndian.Uint16(d[offset+2:])
				fragment = bits&0xfff8 != 0
				p.Expert = append(p.Expert, "IPv6 fragment; transport data may require reassembly")
			} else if proto == 51 {
				length = (int(d[offset+1]) + 2) * 4
			} else {
				length = (int(d[offset+1]) + 1) * 8
			}
			if offset+length > end {
				return malformed("Invalid IPv6 extension length")
			}
			layer("IPv6 extension", offset, length, field("Next header", fmt.Sprint(next), offset, 1))
			offset += length
			proto = next
		}
	default:
		p.Protocol = fmt.Sprintf("EtherType 0x%04X", ether)
		p.Info = fmt.Sprintf("%d captured bytes", len(d))
		return p
	}
	if fragment {
		p.Info = "Non-initial IP fragment"
		return p
	}
	if offset > end {
		return malformed("IP payload offset exceeds the packet")
	}
	payload := offset
	switch proto {
	case 6:
		p.Transport = "TCP"
		p.Protocol = "TCP"
		if end-offset < 20 {
			return malformed("Incomplete TCP header")
		}
		n := int(d[offset+12]>>4) * 4
		if n < 20 || offset+n > end {
			return malformed("Invalid or truncated TCP header length")
		}
		p.SourcePort = binary.BigEndian.Uint16(d[offset:])
		p.DestinationPort = binary.BigEndian.Uint16(d[offset+2:])
		p.Sequence = binary.BigEndian.Uint32(d[offset+4:])
		p.Ack = binary.BigEndian.Uint32(d[offset+8:])
		p.TCPFlags = d[offset+13]
		flags := []string{}
		for i, name := range []string{"FIN", "SYN", "RST", "PSH", "ACK", "URG", "ECE", "CWR"} {
			if p.TCPFlags&(1<<i) != 0 {
				flags = append(flags, name)
			}
		}
		p.Info = fmt.Sprintf("%d to %d [%s] Seq=%d Ack=%d Len=%d", p.SourcePort, p.DestinationPort, strings.Join(flags, ", "), p.Sequence, p.Ack, end-offset-n)
		if p.TCPFlags&4 != 0 {
			p.Expert = append(p.Expert, "TCP reset")
		}
		layer("Transmission Control Protocol", offset, n, field("Source port", fmt.Sprint(p.SourcePort), offset, 2), field("Destination port", fmt.Sprint(p.DestinationPort), offset+2, 2), field("Sequence", fmt.Sprint(p.Sequence), offset+4, 4), field("Acknowledgment", fmt.Sprint(p.Ack), offset+8, 4), field("Flags", strings.Join(flags, ", "), offset+13, 1), field("Window", fmt.Sprint(binary.BigEndian.Uint16(d[offset+14:])), offset+14, 2))
		payload = offset + n
	case 17:
		p.Transport = "UDP"
		p.Protocol = "UDP"
		if end-offset < 8 {
			return malformed("Incomplete UDP header")
		}
		p.SourcePort = binary.BigEndian.Uint16(d[offset:])
		p.DestinationPort = binary.BigEndian.Uint16(d[offset+2:])
		n := int(binary.BigEndian.Uint16(d[offset+4:]))
		if n < 8 {
			return malformed("Invalid UDP length")
		}
		if offset+n < end {
			end = offset + n
		}
		p.Info = fmt.Sprintf("%d to %d Len=%d", p.SourcePort, p.DestinationPort, end-offset-8)
		layer("User Datagram Protocol", offset, 8, field("Source port", fmt.Sprint(p.SourcePort), offset, 2), field("Destination port", fmt.Sprint(p.DestinationPort), offset+2, 2), field("Length", fmt.Sprint(n), offset+4, 2))
		payload = offset + 8
	case 1, 58:
		p.Protocol = "ICMP"
		if proto == 58 {
			p.Protocol = "ICMPv6"
		}
		if end-offset < 4 {
			return malformed("Incomplete ICMP header")
		}
		kind, code := d[offset], d[offset+1]
		names := map[byte]string{0: "Echo reply", 3: "Destination unreachable", 8: "Echo request", 11: "Time exceeded"}
		if proto == 58 {
			names = map[byte]string{1: "Destination unreachable", 2: "Packet too big", 3: "Time exceeded", 128: "Echo request", 129: "Echo reply", 133: "Router solicitation", 134: "Router advertisement", 135: "Neighbor solicitation", 136: "Neighbor advertisement"}
		}
		name := names[kind]
		if name == "" {
			name = "Type " + fmt.Sprint(kind)
		}
		p.Info = name + ", code " + fmt.Sprint(code)
		if (proto == 1 && (kind == 3 || kind == 11)) || (proto == 58 && kind < 128) {
			p.Expert = append(p.Expert, name)
		}
		layer(p.Protocol, offset, end-offset, field("Type", fmt.Sprint(kind), offset, 1), field("Code", fmt.Sprint(code), offset+1, 1))
		payload = offset + 4
	default:
		p.Protocol = fmt.Sprintf("IP protocol %d", proto)
		p.Info = fmt.Sprintf("%d payload bytes", end-offset)
	}
	p.PayloadOffset = payload
	p.PayloadLength = end - payload
	p.Stream = StreamKey(p)
	if payload > end {
		return malformed("Invalid transport payload")
	}
	app := d[payload:end]
	port := func(n uint16) bool { return p.SourcePort == n || p.DestinationPort == n }
	if p.Transport == "UDP" && (port(53) || port(5353) || port(5355)) || p.Transport == "TCP" && port(53) {
		dns := app
		extra := 0
		if p.Transport == "TCP" {
			if len(app) < 2 {
				return p
			}
			length := int(binary.BigEndian.Uint16(app))
			if length > len(app)-2 {
				p.Expert = append(p.Expert, "DNS message spans TCP segments")
				return p
			}
			dns = app[2 : 2+length]
			extra = 2
		}
		info, fields, e := decodeDNS(dns, payload+extra)
		if e == nil {
			p.Protocol = "DNS"
			p.Info = info
			layer("Domain Name System", payload+extra, len(dns), fields...)
		} else {
			p.Expert = append(p.Expert, e.Error())
		}
	}
	if p.Transport == "UDP" && (port(67) || port(68)) {
		if len(app) >= 240 && binary.BigEndian.Uint32(app[236:]) == 0x63825363 {
			p.Protocol = "DHCP"
			p.Info = decodeDHCP(app)
			layer("DHCP / BOOTP", payload, len(app), field("Summary", p.Info, payload, len(app)))
		}
	}
	if p.Transport == "TCP" && len(app) > 0 {
		line := string(app[:min(len(app), 8192)])
		if i := strings.IndexAny(line, "\r\n"); i >= 0 {
			line = line[:i]
		}
		if strings.HasPrefix(line, "HTTP/") || strings.HasPrefix(line, "GET ") || strings.HasPrefix(line, "POST ") || strings.HasPrefix(line, "HEAD ") || strings.HasPrefix(line, "PUT ") || strings.HasPrefix(line, "DELETE ") || strings.HasPrefix(line, "OPTIONS ") {
			p.Protocol = "HTTP"
			p.Info = clean(line)
			layer("Hypertext Transfer Protocol", payload, len(app), field("Start line", p.Info, payload, len(line)))
		} else if strings.HasPrefix(line, "SSH-") {
			p.Protocol = "SSH"
			p.Info = clean(line)
			layer("Secure Shell", payload, len(app), field("Identification", p.Info, payload, len(line)))
		} else if len(app) >= 5 && app[0] >= 20 && app[0] <= 23 && app[1] == 3 && app[2] <= 4 {
			p.Protocol = "TLS"
			kind := map[byte]string{20: "Change cipher spec", 21: "Alert", 22: "Handshake", 23: "Application data"}[app[0]]
			p.Info = fmt.Sprintf("TLS %s, record length %d", kind, binary.BigEndian.Uint16(app[3:]))
			fields := []Field{field("Record type", kind, payload, 1), field("Record version", fmt.Sprintf("0x%02X%02X", app[1], app[2]), payload+1, 2)}
			if app[0] == 22 && len(app) > 5 && app[5] == 1 {
				if host := tlsServerName(app[5:]); host != "" {
					p.Info = "TLS Client Hello, SNI " + host
					fields = append(fields, field("Server name", host, payload+5, len(app)-5))
				}
			}
			layer("Transport Layer Security", payload, len(app), fields...)
		}
	}
	if p.PayloadLength > 0 && p.Protocol == p.Transport {
		layer("Payload", payload, p.PayloadLength)
	}
	return p
}
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\t' {
			return ' '
		}
		return r
	}, s)
}
func dnsName(d []byte, offset int) (string, int, error) {
	parts := []string{}
	next := offset
	seen := map[int]bool{}
	jumped := false
	for n := 0; n < 64; n++ {
		if offset >= len(d) || seen[offset] {
			return "", 0, fmt.Errorf("Invalid DNS name or compression loop")
		}
		seen[offset] = true
		length := int(d[offset])
		if length == 0 {
			if !jumped {
				next = offset + 1
			}
			return strings.Join(parts, "."), next, nil
		}
		if length&0xc0 == 0xc0 {
			if offset+1 >= len(d) {
				break
			}
			pointer := int(binary.BigEndian.Uint16(d[offset:]) & 0x3fff)
			if !jumped {
				next = offset + 2
			}
			jumped = true
			offset = pointer
			continue
		}
		if length > 63 || offset+1+length > len(d) {
			break
		}
		parts = append(parts, clean(string(d[offset+1:offset+1+length])))
		offset += 1 + length
		if !jumped {
			next = offset
		}
		if len(strings.Join(parts, ".")) > 253 {
			break
		}
	}
	return "", 0, fmt.Errorf("Truncated or invalid DNS name")
}
func decodeDNS(d []byte, base int) (string, []Field, error) {
	if len(d) < 12 {
		return "", nil, fmt.Errorf("Incomplete DNS header")
	}
	id, flags := binary.BigEndian.Uint16(d), binary.BigEndian.Uint16(d[2:])
	questions, answers := int(binary.BigEndian.Uint16(d[4:])), int(binary.BigEndian.Uint16(d[6:]))
	if questions > 128 || answers > 512 {
		return "", nil, fmt.Errorf("DNS record count exceeds the decoder limit")
	}
	fields := []Field{{"Transaction ID", fmt.Sprintf("0x%04X", id), base, 2}, {"Flags", fmt.Sprintf("0x%04X", flags), base + 2, 2}, {"Questions", fmt.Sprint(questions), base + 4, 2}, {"Answers", fmt.Sprint(answers), base + 6, 2}}
	off := 12
	names := []string{}
	kind := func(n uint16) string {
		if s := map[uint16]string{1: "A", 2: "NS", 5: "CNAME", 6: "SOA", 12: "PTR", 15: "MX", 16: "TXT", 28: "AAAA", 33: "SRV", 65: "HTTPS"}[n]; s != "" {
			return s
		}
		return fmt.Sprint(n)
	}
	for n := 0; n < questions; n++ {
		name, next, e := dnsName(d, off)
		if e != nil || next+4 > len(d) {
			return "", nil, fmt.Errorf("Incomplete DNS question")
		}
		typ := binary.BigEndian.Uint16(d[next:])
		names = append(names, kind(typ)+" "+name)
		fields = append(fields, Field{"Question", kind(typ) + " " + name, base + off, next + 4 - off})
		off = next + 4
	}
	values := []string{}
	for n := 0; n < answers; n++ {
		name, next, e := dnsName(d, off)
		if e != nil || next+10 > len(d) {
			return "", nil, fmt.Errorf("Incomplete DNS answer")
		}
		typ, length := binary.BigEndian.Uint16(d[next:]), int(binary.BigEndian.Uint16(d[next+8:]))
		start := next + 10
		if start+length > len(d) {
			return "", nil, fmt.Errorf("Truncated DNS answer")
		}
		value := ""
		switch typ {
		case 1:
			if length == 4 {
				value = net.IP(d[start : start+4]).String()
			}
		case 28:
			if length == 16 {
				value = net.IP(d[start : start+16]).String()
			}
		case 2, 5, 12:
			value, _, _ = dnsName(d, start)
		case 15:
			if length >= 3 {
				value, _, _ = dnsName(d, start+2)
			}
		case 16:
			if length > 0 && int(d[start])+1 <= length {
				value = clean(string(d[start+1 : start+1+int(d[start])]))
			}
		}
		if value != "" {
			values = append(values, kind(typ)+" "+value)
			fields = append(fields, Field{"Answer " + name, value, base + off, start + length - off})
		}
		off = start + length
	}
	text := "Query"
	if flags&0x8000 != 0 {
		text = "Response"
	}
	text += fmt.Sprintf(" 0x%04X ", id) + strings.Join(names, ", ")
	if flags&15 != 0 {
		text += " RCODE=" + fmt.Sprint(flags&15)
	}
	if len(values) > 0 {
		text += " " + strings.Join(values, ", ")
	}
	return text, fields, nil
}
func decodeDHCP(d []byte) string {
	kinds := map[byte]string{1: "Discover", 2: "Offer", 3: "Request", 4: "Decline", 5: "ACK", 6: "NAK", 7: "Release", 8: "Inform"}
	parts := []string{}
	for off := 240; off < len(d); {
		code := d[off]
		off++
		if code == 255 {
			break
		}
		if code == 0 {
			continue
		}
		if off >= len(d) {
			break
		}
		n := int(d[off])
		off++
		if off+n > len(d) {
			break
		}
		v := d[off : off+n]
		switch code {
		case 53:
			if len(v) > 0 {
				parts = append(parts, kinds[v[0]])
			}
		case 12:
			parts = append(parts, "Host "+clean(string(v)))
		case 50, 54:
			if n == 4 {
				parts = append(parts, net.IP(v).String())
			}
		}
		off += n
	}
	return "DHCP " + strings.Join(parts, ", ") + fmt.Sprintf(" Transaction 0x%08X", binary.BigEndian.Uint32(d[4:]))
}
func decodeLLDP(d []byte) string {
	parts := []string{}
	for off, n := 0, 0; off+2 <= len(d) && n < 128; n++ {
		header := binary.BigEndian.Uint16(d[off:])
		off += 2
		typ, length := header>>9, int(header&511)
		if off+length > len(d) {
			break
		}
		v := d[off : off+length]
		if typ == 0 {
			break
		}
		switch typ {
		case 1, 2:
			if len(v) > 1 {
				value := clean(string(v[1:]))
				if (typ == 1 && v[0] == 4 || typ == 2 && v[0] == 3) && len(v) == 7 {
					value = net.HardwareAddr(v[1:]).String()
				}
				prefix := "Chassis "
				if typ == 2 {
					prefix = "Port "
				}
				parts = append(parts, prefix+value)
			}
		case 5:
			parts = append(parts, "System "+clean(string(v)))
		}
		off += length
	}
	return strings.Join(parts, ", ")
}
func decodeCDP(d []byte) string {
	if len(d) < 4 {
		return "Incomplete CDP header"
	}
	parts := []string{}
	for off, n := 4, 0; off+4 <= len(d) && n < 128; n++ {
		typ, length := binary.BigEndian.Uint16(d[off:]), int(binary.BigEndian.Uint16(d[off+2:]))
		if length < 4 || off+length > len(d) {
			break
		}
		if name := map[uint16]string{1: "Device", 3: "Port", 5: "Software", 6: "Platform", 10: "Native VLAN"}[typ]; name != "" {
			v := d[off+4 : off+length]
			text := clean(string(v))
			if typ == 10 && len(v) == 2 {
				text = fmt.Sprint(binary.BigEndian.Uint16(v))
			}
			parts = append(parts, name+" "+text)
		}
		off += length
	}
	return strings.Join(parts, ", ")
}
func tlsServerName(d []byte) string {
	if len(d) < 4+2+32+1 {
		return ""
	}
	length := int(d[1])<<16 | int(d[2])<<8 | int(d[3])
	if length > len(d)-4 {
		return ""
	}
	d = d[4 : 4+length]
	off := 34
	if off >= len(d) {
		return ""
	}
	off += 1 + int(d[off])
	if off+2 > len(d) {
		return ""
	}
	off += 2 + int(binary.BigEndian.Uint16(d[off:]))
	if off >= len(d) {
		return ""
	}
	off += 1 + int(d[off])
	if off+2 > len(d) {
		return ""
	}
	end := off + 2 + int(binary.BigEndian.Uint16(d[off:]))
	off += 2
	if end > len(d) {
		return ""
	}
	for off+4 <= end {
		typ, n := binary.BigEndian.Uint16(d[off:]), int(binary.BigEndian.Uint16(d[off+2:]))
		off += 4
		if off+n > end {
			return ""
		}
		if typ == 0 && n >= 5 {
			v := d[off : off+n]
			declared := int(binary.BigEndian.Uint16(v))
			if declared > n-2 {
				return ""
			}
			for i := 2; i+3 <= 2+declared; {
				kind, length := v[i], int(binary.BigEndian.Uint16(v[i+1:]))
				i += 3
				if i+length > 2+declared {
					return ""
				}
				if kind == 0 {
					return clean(string(v[i : i+length]))
				}
				i += length
			}
		}
		off += n
	}
	return ""
}
func HexDump(data []byte) string { return hex.Dump(data) }
func ASCII(data []byte) string {
	var b strings.Builder
	for _, v := range data {
		if v >= 32 && v < 127 || v == '\r' || v == '\n' || v == '\t' {
			b.WriteByte(v)
		} else {
			b.WriteByte('.')
		}
	}
	return b.String()
}
func Port(value string) (uint16, error) {
	n, e := strconv.ParseUint(value, 10, 16)
	return uint16(n), e
}
