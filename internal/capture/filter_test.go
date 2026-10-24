package capture

import (
	"strings"
	"testing"
)

func TestDisplayFilters(t *testing.T) {
	p := Decode(testTCP(10, 0x12, "GET / HTTP/1.1\r\n"))
	p.Number = 9
	for _, expression := range []string{"tcp && ip.addr == 192.0.2.0/24", "(http || dns) && !tcp.flags.rst", "tcp.dstport == 80 && frame.number >= 9", "info contains 'GET'", "tcp.flags.syn && tcp.flags.ack", "eth.src == 06:07:08:09:0a:0b", "ip.src != 203.0.113.2", "not udp and frame.cap_len > 0"} {
		fn, e := Compile(expression)
		if e != nil || !fn(p) {
			t.Fatalf("%s: %v", expression, e)
		}
	}
	for _, expression := range []string{"udp", "ip.dst == 192.0.2.0/24", "tcp.flags.rst", "tcp.port != 80", "ipv6", "info contains missing"} {
		fn, e := Compile(expression)
		if e != nil || fn(p) {
			t.Fatalf("unexpected match %s: %v", expression, e)
		}
	}
	for _, expression := range []string{"tcp and", "unknown", "ip.addr == invalid", "tcp.port == 65536", "frame.len == NaN", "frame.len < +Inf", "tcp udp", "info contains 'unclosed", strings.Repeat("(", 40) + "tcp" + strings.Repeat(")", 40)} {
		if _, e := Compile(expression); e == nil {
			t.Fatal("invalid expression accepted", expression)
		}
	}
}
func TestKeepFiltersAndOptions(t *testing.T) {
	vlan := uint16(42)
	o := Options{Filter: KeepFilter{Host: "192.0.2.0/24", Protocol: "tcp", Ports: []uint16{80}, MAC: "06-07-08-09-0a-0b", VLAN: &vlan, Direction: "out"}}
	if e := o.Normalize(); e != nil {
		t.Fatal(e)
	}
	p := Decode(testTCP(1, 2, ""))
	p.VLANs = []uint16{42}
	p.Direction = "out"
	if !o.Filter.Match(p) {
		t.Fatal("keep filter did not match")
	}
	p.Direction = "in"
	if o.Filter.Match(p) {
		t.Fatal("direction ignored")
	}
	o.Engine = "file"
	if o.Normalize() == nil {
		t.Fatal("file direction accepted")
	}
	for _, o := range []Options{{Engine: "shell"}, {MaxMB: 129}, {Duration: 7201}, {SnapLen: -1}, {Filter: KeepFilter{Ports: []uint16{0}}}, {Filter: KeepFilter{MAC: "invalid"}}, {Filter: KeepFilter{Host: "bad"}}} {
		if o.Normalize() == nil {
			t.Fatal("invalid options accepted", o)
		}
	}
}
