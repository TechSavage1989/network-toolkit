package capture

import (
	"encoding/binary"
	"net/netip"
	"strconv"
	"strings"
)

// Plain names for the troubleshooting view (owner request: a capture should
// be understandable without packet-analysis knowledge). Service names are
// conventional port assignments: a service can use another port, so the UI
// presents them as "usually" when it explains them.

var serviceNames = map[uint16]string{
	20: "FTP data", 21: "FTP", 22: "SSH", 23: "Telnet", 25: "Mail (SMTP)", 53: "DNS",
	67: "DHCP", 68: "DHCP", 69: "TFTP", 80: "Web (HTTP)", 88: "Kerberos sign-in",
	110: "Mail (POP3)", 123: "Time (NTP)", 135: "Windows RPC", 137: "Windows names",
	138: "Windows browsing", 139: "File sharing (NetBIOS)", 143: "Mail (IMAP)",
	161: "SNMP", 162: "SNMP traps", 389: "Directory (LDAP)", 443: "Secure web (HTTPS)",
	445: "File sharing (SMB)", 465: "Mail (SMTPS)", 500: "VPN (IPsec)", 514: "Syslog",
	515: "Printing (LPD)", 546: "DHCPv6", 547: "DHCPv6", 587: "Mail submission",
	631: "Printing (IPP)", 636: "Directory (LDAPS)", 853: "DNS over TLS", 993: "Mail (IMAPS)",
	995: "Mail (POP3S)", 1194: "VPN (OpenVPN)", 1433: "SQL Server", 1723: "VPN (PPTP)",
	1812: "RADIUS", 1813: "RADIUS accounting", 1900: "Device discovery (SSDP)",
	3268: "Directory (Global Catalog)", 3306: "MySQL", 3389: "Remote Desktop",
	4500: "VPN (IPsec)", 5060: "Phone calls (SIP)", 5061: "Phone calls (SIP TLS)",
	5353: "Local name discovery (mDNS)", 5355: "Local name lookup (LLMNR)", 5432: "PostgreSQL",
	5900: "Remote screen (VNC)", 5985: "Windows remote management", 5986: "Windows remote management",
	8080: "Web (alternate)", 8443: "Secure web (alternate)", 9100: "Printing (raw)",
	51820: "VPN (WireGuard)",
}

// ServiceName returns a plain name for a well-known port, or "".
func ServiceName(port uint16) string { return serviceNames[port] }

// servicePort picks the server side of a connection: the well-known port when
// only one side has one, otherwise the lower port.
func servicePort(a, b uint16) uint16 {
	_, knownA := serviceNames[a]
	_, knownB := serviceNames[b]
	switch {
	case knownB && !knownA:
		return b
	case knownA && !knownB:
		return a
	case a == 0:
		return b
	case b == 0 || a < b:
		return a
	}
	return b
}

// ServiceLabel describes a port for people: "Remote Desktop (port 3389)" or
// "port 50123".
func ServiceLabel(port uint16) string {
	if name := ServiceName(port); name != "" {
		return name + " (port " + strconv.Itoa(int(port)) + ")"
	}
	return "port " + strconv.Itoa(int(port))
}

const maxHostNames = 4096

// HostNames maps IP addresses to the names this capture's DNS answers gave
// for them (A and AAAA records answering a standard query). It is learned from
// the recording only; nothing is looked up.
func HostNames(packets []Packet) map[string]string {
	names := map[string]string{}
	for _, p := range packets {
		question, addresses, _, ok := dnsAnswerAddresses(p)
		if !ok || question == "" {
			continue
		}
		for _, address := range addresses {
			if _, exists := names[address]; !exists && len(names) < maxHostNames {
				names[address] = question
			}
		}
	}
	return names
}

// dnsMessage returns the DNS message carried by a standard DNS packet.
func dnsMessage(p Packet) ([]byte, bool) {
	if p.Protocol != "DNS" || p.SourcePort != 53 && p.DestinationPort != 53 {
		return nil, false
	}
	d, ok := troublePayload(p)
	if !ok {
		return nil, false
	}
	if p.Transport == "TCP" {
		if len(d) < 2 {
			return nil, false
		}
		size := int(binary.BigEndian.Uint16(d))
		if size > len(d)-2 {
			return nil, false
		}
		d = d[2 : 2+size]
	} else if p.Transport != "UDP" {
		return nil, false
	}
	if len(d) < 12 {
		return nil, false
	}
	return d, true
}

// dnsAnswerAddresses reads a DNS response: its first question name, the
// IPv4/IPv6 addresses in its answer section and its response code.
func dnsAnswerAddresses(p Packet) (string, []string, uint16, bool) {
	d, ok := dnsMessage(p)
	if !ok || p.SourcePort != 53 {
		return "", nil, 0, false
	}
	flags := binary.BigEndian.Uint16(d[2:])
	if flags&0x8000 == 0 || flags>>11&15 != 0 {
		return "", nil, 0, false
	}
	questions, answers := int(binary.BigEndian.Uint16(d[4:])), int(binary.BigEndian.Uint16(d[6:]))
	if questions > 8 || answers > 256 {
		return "", nil, 0, false
	}
	off, question := 12, ""
	for i := 0; i < questions; i++ {
		name, next, err := dnsName(d, off)
		if err != nil || next > len(d)-4 {
			return "", nil, 0, false
		}
		if i == 0 {
			question = strings.TrimSuffix(strings.ToLower(name), ".")
		}
		off = next + 4
	}
	addresses := []string{}
	for i := 0; i < answers; i++ {
		_, next, err := dnsName(d, off)
		if err != nil || next > len(d)-10 {
			break
		}
		kind, length := binary.BigEndian.Uint16(d[next:]), int(binary.BigEndian.Uint16(d[next+8:]))
		start := next + 10
		if length > len(d)-start {
			break
		}
		if kind == 1 && length == 4 {
			addresses = append(addresses, netip.AddrFrom4([4]byte(d[start:start+4])).String())
		} else if kind == 28 && length == 16 {
			addresses = append(addresses, netip.AddrFrom16([16]byte(d[start:start+16])).String())
		}
		off = start + length
	}
	return question, addresses, flags & 15, true
}
