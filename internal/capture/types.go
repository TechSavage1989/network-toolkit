// Package capture reads, decodes and exports packet captures without a driver dependency.
package capture

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"
)

const MaxFileBytes = 128 << 20
const MaxPacketBytes = 262144
const MaxPackets = 200000

type Options struct {
	Engine     string     `json:"engine"`
	Interface  string     `json:"interface"`
	SnapLen    int        `json:"snapLen"`
	Duration   int        `json:"duration"`
	MaxMB      int        `json:"maxMB"`
	MaxPackets int        `json:"maxPackets"`
	Ring       bool       `json:"ring"`
	Filter     KeepFilter `json:"filter"`
}
type KeepFilter struct {
	Protocol  string   `json:"protocol"`
	Host      string   `json:"host"`
	Ports     []uint16 `json:"ports"`
	MAC       string   `json:"mac"`
	VLAN      *uint16  `json:"vlan,omitempty"`
	Direction string   `json:"direction"`
}

func (o *Options) Normalize() error {
	if o.Engine == "" {
		o.Engine = "auto"
	}
	if o.Engine != "auto" && o.Engine != "live" && o.Engine != "file" {
		return errors.New("Choose automatic, live or file capture.")
	}
	if o.Interface == "" {
		o.Interface = "nics"
	}
	if len(o.Interface) > 512 || strings.ContainsAny(o.Interface, "\x00\r\n") {
		return errors.New("Choose a valid capture interface.")
	}
	if o.SnapLen < 0 || o.SnapLen > 65535 {
		return errors.New("Packet length must be 0 through 65535 bytes.")
	}
	if o.Duration == 0 {
		o.Duration = 60
	}
	if o.Duration < 1 || o.Duration > 7200 {
		return errors.New("Capture duration must be 1 through 7200 seconds.")
	}
	if o.MaxMB == 0 {
		o.MaxMB = 32
	}
	if o.MaxMB < 1 || o.MaxMB > 128 {
		return errors.New("Capture memory limit must be 1 through 128 MiB.")
	}
	if o.MaxPackets == 0 {
		o.MaxPackets = 50000
	}
	if o.MaxPackets < 1 || o.MaxPackets > MaxPackets {
		return errors.New("Capture packet limit must be 1 through 200000.")
	}
	o.Filter.Protocol = strings.ToLower(strings.TrimSpace(o.Filter.Protocol))
	if o.Filter.Protocol == "" {
		o.Filter.Protocol = "any"
	}
	switch o.Filter.Protocol {
	case "any", "ipv4", "ipv6", "tcp", "udp", "icmp", "icmpv6", "arp", "dns", "dhcp", "http", "tls", "ssh", "lldp", "cdp":
	default:
		return errors.New("Choose a supported capture protocol.")
	}
	o.Filter.Host = strings.TrimSpace(o.Filter.Host)
	if o.Filter.Host != "" {
		if _, err := netip.ParseAddr(o.Filter.Host); err != nil {
			if _, err = netip.ParsePrefix(o.Filter.Host); err != nil {
				return errors.New("The capture host must be an IP address or CIDR network.")
			}
		}
	}
	if len(o.Filter.Ports) > 32 {
		return errors.New("Use at most 32 capture ports.")
	}
	for _, p := range o.Filter.Ports {
		if p == 0 {
			return errors.New("Capture ports must be 1 through 65535.")
		}
	}
	if o.Filter.MAC != "" {
		m, e := net.ParseMAC(o.Filter.MAC)
		if e != nil || len(m) != 6 {
			return errors.New("Enter a six-byte MAC address.")
		}
		o.Filter.MAC = m.String()
	}
	if o.Filter.VLAN != nil && *o.Filter.VLAN > 4094 {
		return errors.New("VLAN ID must be 0 through 4094.")
	}
	if o.Filter.Direction != "" && o.Filter.Direction != "any" && o.Filter.Direction != "in" && o.Filter.Direction != "out" {
		return errors.New("Choose both directions, inbound or outbound.")
	}
	if o.Engine == "file" && o.Filter.Direction != "" && o.Filter.Direction != "any" {
		return errors.New("Directional filtering requires live capture. Choose both directions for file capture.")
	}
	return nil
}

type Interface struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Engine      string `json:"engine"`
}
type Record struct {
	Time           time.Time `json:"time"`
	Data           []byte    `json:"data"`
	OriginalLength uint32    `json:"originalLength"`
	LinkType       uint16    `json:"linkType"`
	Interface      string    `json:"interface"`
	Direction      string    `json:"direction,omitempty"`
	DropReason     uint32    `json:"dropReason,omitempty"`
}
type Field struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
}
type Layer struct {
	Name   string  `json:"name"`
	Offset int     `json:"offset"`
	Length int     `json:"length"`
	Fields []Field `json:"fields"`
}
type Packet struct {
	Number          uint64    `json:"number"`
	Time            time.Time `json:"time"`
	Relative        float64   `json:"relative"`
	Source          string    `json:"source"`
	Destination     string    `json:"destination"`
	SourcePort      uint16    `json:"sourcePort,omitempty"`
	DestinationPort uint16    `json:"destinationPort,omitempty"`
	Protocol        string    `json:"protocol"`
	Transport       string    `json:"transport,omitempty"`
	Length          uint32    `json:"length"`
	Captured        int       `json:"captured"`
	Interface       string    `json:"interface"`
	Direction       string    `json:"direction,omitempty"`
	Info            string    `json:"info"`
	Highlight       string    `json:"highlight,omitempty"`
	Stream          string    `json:"stream,omitempty"`
	Expert          []string  `json:"expert,omitempty"`
	SrcMAC          string    `json:"srcMAC,omitempty"`
	DstMAC          string    `json:"dstMAC,omitempty"`
	VLANs           []uint16  `json:"vlans,omitempty"`
	IPVersion       int       `json:"ipVersion,omitempty"`
	TCPFlags        uint8     `json:"tcpFlags,omitempty"`
	Sequence        uint32    `json:"sequence,omitempty"`
	Ack             uint32    `json:"ack,omitempty"`
	PayloadOffset   int       `json:"payloadOffset,omitempty"`
	PayloadLength   int       `json:"payloadLength,omitempty"`
	Record          Record    `json:"-"`
	Layers          []Layer   `json:"-"`
}

func (f KeepFilter) Match(p Packet) bool {
	proto := strings.ToLower(p.Protocol)
	switch f.Protocol {
	case "", "any":
	case "ipv4":
		if p.IPVersion != 4 {
			return false
		}
	case "ipv6":
		if p.IPVersion != 6 {
			return false
		}
	case "tcp", "udp":
		if !strings.EqualFold(p.Transport, f.Protocol) {
			return false
		}
	default:
		if proto != f.Protocol {
			return false
		}
	}
	if f.Host != "" && !MatchesAddress(p.Source, f.Host) && !MatchesAddress(p.Destination, f.Host) {
		return false
	}
	if len(f.Ports) > 0 {
		found := false
		for _, port := range f.Ports {
			found = found || p.SourcePort == port || p.DestinationPort == port
		}
		if !found {
			return false
		}
	}
	if f.MAC != "" && !strings.EqualFold(f.MAC, p.SrcMAC) && !strings.EqualFold(f.MAC, p.DstMAC) {
		return false
	}
	if f.VLAN != nil {
		found := false
		for _, v := range p.VLANs {
			found = found || v == *f.VLAN
		}
		if !found {
			return false
		}
	}
	return f.Direction == "" || f.Direction == "any" || f.Direction == p.Direction
}
func MatchesAddress(value, filter string) bool {
	a, e := netip.ParseAddr(value)
	if e != nil {
		return false
	}
	if b, e := netip.ParseAddr(filter); e == nil {
		return a.Unmap() == b.Unmap()
	}
	prefix, e := netip.ParsePrefix(filter)
	return e == nil && prefix.Contains(a)
}
func StreamKey(p Packet) string {
	if p.Transport != "TCP" && p.Transport != "UDP" {
		return ""
	}
	a, b := net.JoinHostPort(p.Source, fmt.Sprint(p.SourcePort)), net.JoinHostPort(p.Destination, fmt.Sprint(p.DestinationPort))
	if a > b {
		a, b = b, a
	}
	return p.Transport + " " + a + " | " + b
}
