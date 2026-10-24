package capture

import (
	"encoding/binary"
	"strings"
	"testing"
	"time"
)

func testFrame(proto byte, payload []byte) Record {
	d := make([]byte, 14+20+len(payload))
	copy(d, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 8, 0})
	d[14] = 0x45
	binary.BigEndian.PutUint16(d[16:], uint16(20+len(payload)))
	d[22] = 64
	d[23] = proto
	copy(d[26:], []byte{192, 0, 2, 1, 198, 51, 100, 2})
	copy(d[34:], payload)
	return Record{Time: time.Unix(100, 123), Data: d, OriginalLength: uint32(len(d)), LinkType: 1, Interface: "Ethernet"}
}
func testTCP(seq uint32, flags byte, text string) Record {
	p := make([]byte, 20+len(text))
	binary.BigEndian.PutUint16(p, 40000)
	binary.BigEndian.PutUint16(p[2:], 80)
	binary.BigEndian.PutUint32(p[4:], seq)
	p[12] = 0x50
	p[13] = flags
	copy(p[20:], text)
	return testFrame(6, p)
}
func TestDecodeProtocolsAndFields(t *testing.T) {
	p := Decode(testTCP(42, 0x18, "GET / HTTP/1.1\r\nHost: example.test\r\n\r\n"))
	if p.Protocol != "HTTP" || p.Transport != "TCP" || p.Source != "192.0.2.1" || p.DestinationPort != 80 || p.Sequence != 42 || !strings.Contains(p.Info, "GET /") || len(p.Layers) < 4 || p.Stream == "" {
		t.Fatalf("bad HTTP packet: %+v", p)
	}
	for _, l := range p.Layers {
		if l.Offset < 0 || l.Offset+l.Length > len(p.Record.Data) {
			t.Fatal("invalid layer offset", l)
		}
	}
	dns := []byte{0x12, 0x34, 1, 0, 0, 1, 0, 0, 0, 0, 0, 0, 7, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 4, 't', 'e', 's', 't', 0, 0, 1, 0, 1}
	udp := make([]byte, 8+len(dns))
	binary.BigEndian.PutUint16(udp, 53000)
	binary.BigEndian.PutUint16(udp[2:], 53)
	binary.BigEndian.PutUint16(udp[4:], uint16(len(udp)))
	copy(udp[8:], dns)
	p = Decode(testFrame(17, udp))
	if p.Protocol != "DNS" || !strings.Contains(p.Info, "example.test") {
		t.Fatal(p)
	}
	p = Decode(testFrame(1, []byte{8, 0, 0, 0, 0, 1, 0, 2}))
	if p.Protocol != "ICMP" {
		t.Fatal(p)
	}
	r := testTCP(1, 4, "")
	r.OriginalLength += 100
	p = Decode(r)
	if len(p.Expert) < 2 {
		t.Fatal("missing truncation/reset notes", p.Expert)
	}
	vlan := testTCP(1, 2, "")
	vlan.Data = append(append(append([]byte{}, vlan.Data[:12]...), 0x81, 0, 0, 42), vlan.Data[12:]...)
	vlan.OriginalLength = uint32(len(vlan.Data))
	p = Decode(vlan)
	if len(p.VLANs) != 1 || p.VLANs[0] != 42 || p.Transport != "TCP" {
		t.Fatal(p)
	}
}
func TestDecodeEveryTruncatedPrefix(t *testing.T) {
	r := testTCP(100, 0x18, "HTTP/1.1 200 OK\r\n")
	for n := 0; n <= len(r.Data); n++ {
		v := r
		v.Data = r.Data[:n]
		p := Decode(v)
		if p.Captured != n {
			t.Fatal(n)
		}
	}
	cyclic := []byte{0, 1, 1, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0xc0, 0x0c, 0, 1, 0, 1}
	_, _, _ = decodeDNS(cyclic, 0)
}
func FuzzPacketDecode(f *testing.F) {
	f.Add(testTCP(1, 0x18, "GET / HTTP/1.1\r\n").Data, uint16(1))
	f.Add([]byte{0x60}, uint16(101))
	f.Fuzz(func(t *testing.T, data []byte, link uint16) {
		if len(data) > 1<<16 {
			t.Skip()
		}
		p := Decode(Record{Data: data, OriginalLength: uint32(len(data)), LinkType: link})
		if p.Captured != len(data) || p.PayloadOffset < 0 || p.PayloadLength < 0 || p.PayloadOffset+p.PayloadLength > len(data) {
			t.Fatal("invalid decoded payload bounds")
		}
		_ = Stats([]Packet{p})
		if p.Stream != "" {
			_ = Follow([]Packet{p}, p.Stream)
		}
	})
}
