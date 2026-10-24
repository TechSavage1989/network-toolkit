package capture

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func troubleTestPacket(number uint64, seconds float64, record Record) Packet {
	record.Time = time.Unix(1000, 0).Add(time.Duration(seconds * float64(time.Second)))
	p := Decode(record)
	p.Number = number
	p.Layers = nil
	return p
}

func troubleTestReverse(record Record) Record {
	r := record
	r.Data = append([]byte(nil), record.Data...)
	copy(r.Data[26:30], record.Data[30:34])
	copy(r.Data[30:34], record.Data[26:30])
	copy(r.Data[34:36], record.Data[36:38])
	copy(r.Data[36:38], record.Data[34:36])
	return r
}

func troubleTestDNS(name string, kind, id uint16, response bool, rcode uint16) Record {
	d := make([]byte, 12)
	binary.BigEndian.PutUint16(d, id)
	flags := uint16(0x100) | rcode&15
	if response {
		flags |= 0x8000
	}
	binary.BigEndian.PutUint16(d[2:], flags)
	if name != "" {
		binary.BigEndian.PutUint16(d[4:], 1)
		for _, label := range strings.Split(name, ".") {
			d = append(d, byte(len(label)))
			d = append(d, label...)
		}
		d = append(d, 0, byte(kind>>8), byte(kind), 0, 1)
	}
	if rcode > 15 {
		binary.BigEndian.PutUint16(d[10:], 1)
		d = append(d, 0, 0, 41, 16, 0, byte(rcode>>4), 0, 0, 0, 0, 0)
	}
	u := make([]byte, 8+len(d))
	binary.BigEndian.PutUint16(u, 53000)
	binary.BigEndian.PutUint16(u[2:], 53)
	binary.BigEndian.PutUint16(u[4:], uint16(len(u)))
	copy(u[8:], d)
	r := testFrame(17, u)
	if response {
		r = troubleTestReverse(r)
	}
	return r
}

func troubleTestTCP(sequence, ack uint32, flags byte, text string, response bool) Record {
	r := testTCP(sequence, flags, text)
	binary.BigEndian.PutUint32(r.Data[42:], ack)
	if response {
		r = troubleTestReverse(r)
	}
	return r
}

func troubleTestDNSTCP(name string, kind, id uint16, response bool, rcode uint16) Record {
	dns := troubleTestDNS(name, kind, id, false, rcode).Data[42:]
	payload := make([]byte, 2+len(dns))
	binary.BigEndian.PutUint16(payload, uint16(len(dns)))
	copy(payload[2:], dns)
	if response {
		payload[4] |= 0x80
	}
	r := troubleTestTCP(100, 1, 0x18, string(payload), false)
	binary.BigEndian.PutUint16(r.Data[34:], 53000)
	binary.BigEndian.PutUint16(r.Data[36:], 53)
	if response {
		r = troubleTestReverse(r)
	}
	return r
}

func troubleTestTail(number uint64, seconds float64) Packet {
	return Packet{Number: number, Time: time.Unix(1000, 0).Add(time.Duration(seconds * float64(time.Second))), Protocol: "Unknown", Interface: "Ethernet"}
}

func troubleTestFinding(t *testing.T, report Troubleshooting, id string) TroubleshootFinding {
	t.Helper()
	for _, f := range report.Findings {
		if f.ID == id {
			return f
		}
	}
	t.Fatalf("missing %s in %+v", id, report.Findings)
	return TroubleshootFinding{}
}

func troubleTestAbsent(t *testing.T, report Troubleshooting, ids ...string) {
	t.Helper()
	for _, f := range report.Findings {
		for _, id := range ids {
			if f.ID == id {
				t.Fatalf("unexpected %s: %+v", id, f)
			}
		}
	}
}

func TestTroubleshootDNSReplyMatchingAndNegativeCodes(t *testing.T) {
	for _, code := range []uint16{0, 2, 3, 5, 16, 19} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			packets := []Packet{troubleTestPacket(1, 0, troubleTestDNS("Example.test", 1, 123, false, 0)), troubleTestPacket(2, .2, troubleTestDNS("example.TEST", 1, 123, true, code)), troubleTestTail(3, 5)}
			r := Troubleshoot(packets, TroubleshootContext{})
			if r.Metrics.DNSQueries != 1 || r.Metrics.DNSResponses != 1 {
				t.Fatal(r.Metrics)
			}
			troubleTestAbsent(t, r, "dns-no-visible-reply")
			if code == 0 {
				if r.Metrics.DNSErrors != 0 || len(r.Findings) != 0 {
					t.Fatal("NOERROR with zero answers is not a DNS error", r)
				}
				return
			}
			if r.Metrics.DNSErrors != 1 {
				t.Fatal(r.Metrics)
			}
			id := map[uint16]string{2: "dns-server-failure", 3: "dns-name-not-found", 5: "dns-refused", 16: "dns-other-error", 19: "dns-other-error"}[code]
			f := troubleTestFinding(t, r, id)
			if !strings.Contains(f.Observation, "198.51.100.2") || !strings.Contains(f.Observation, "example.TEST") {
				t.Fatal("missing understandable DNS context", f)
			}
			if code > 15 {
				troubleTestAbsent(t, r, "dns-name-not-found")
			}
		})
	}
}

func TestTroubleshootDNSUnansweredWindowAndIDReuse(t *testing.T) {
	q := troubleTestPacket(1, 0, troubleTestDNS("missing.test", 1, 42, false, 0))
	for _, end := range []float64{0, 2.99} {
		troubleTestAbsent(t, Troubleshoot([]Packet{q, troubleTestTail(2, end)}, TroubleshootContext{}), "dns-no-visible-reply")
	}
	r := Troubleshoot([]Packet{q, troubleTestTail(2, 3)}, TroubleshootContext{Active: true})
	f := troubleTestFinding(t, r, "dns-no-visible-reply")
	if f.Count != 1 || !strings.Contains(f.Observation, "missing.test") || !strings.Contains(f.Meaning, "not proof") {
		t.Fatal(f)
	}
	// A response with a reused ID on a different question, port or adapter
	// cannot answer the first lookup.
	for _, difference := range []string{"name", "type", "port", "interface", "transport"} {
		t.Run(difference, func(t *testing.T) {
			reply := troubleTestPacket(2, .1, troubleTestDNS("missing.test", 1, 42, true, 0))
			switch difference {
			case "name":
				reply = troubleTestPacket(2, .1, troubleTestDNS("other.test", 1, 42, true, 0))
			case "type":
				reply = troubleTestPacket(2, .1, troubleTestDNS("missing.test", 28, 42, true, 0))
			case "port":
				record := troubleTestDNS("missing.test", 1, 42, true, 0)
				binary.BigEndian.PutUint16(record.Data[36:], 53001)
				reply = troubleTestPacket(2, .1, record)
			case "interface":
				reply.Interface = "Other"
			case "transport":
				reply = troubleTestPacket(2, .1, troubleTestDNSTCP("missing.test", 1, 42, true, 0))
			}
			r := Troubleshoot([]Packet{q, reply, troubleTestTail(3, 5)}, TroubleshootContext{})
			troubleTestFinding(t, r, "dns-no-visible-reply")
		})
	}
	// A question-less response is ambiguous, so do not invent a missing reply.
	empty := troubleTestPacket(2, .1, troubleTestDNS("", 1, 42, true, 0))
	troubleTestAbsent(t, Troubleshoot([]Packet{q, empty, troubleTestTail(4, 5)}, TroubleshootContext{}), "dns-no-visible-reply")
	// Its ID must not suppress a new query captured after that response.
	later := troubleTestPacket(3, 1, troubleTestDNS("later.test", 1, 42, false, 0))
	f = troubleTestFinding(t, Troubleshoot([]Packet{q, empty, later, troubleTestTail(4, 5)}, TroubleshootContext{}), "dns-no-visible-reply")
	if !reflect.DeepEqual(f.PacketNumbers, []uint64{3}) {
		t.Fatal(f)
	}
	later = troubleTestPacket(3, 1, troubleTestDNS("missing.test", 1, 42, false, 0))
	f = troubleTestFinding(t, Troubleshoot([]Packet{q, empty, later, troubleTestTail(4, 5)}, TroubleshootContext{}), "dns-no-visible-reply")
	if !reflect.DeepEqual(f.PacketNumbers, []uint64{3}) {
		t.Fatal("question-less old reply suppressed a later identical lookup", f)
	}
}

func TestTroubleshootDNSCompleteTCPAndOutOfOrderCaptures(t *testing.T) {
	q := troubleTestPacket(1, 0, troubleTestDNSTCP("example.test", 1, 123, false, 0))
	reply := troubleTestPacket(2, .2, troubleTestDNSTCP("example.test", 1, 123, true, 0))
	if q.Protocol != "DNS" || reply.Protocol != "DNS" {
		t.Fatal(q, reply)
	}
	r := Troubleshoot([]Packet{reply, q, troubleTestTail(3, 5)}, TroubleshootContext{})
	if r.Metrics.DNSQueries != 1 || r.Metrics.DNSResponses != 1 {
		t.Fatal(r.Metrics)
	}
	troubleTestAbsent(t, r, "dns-no-visible-reply")
	if !strings.Contains(strings.Join(r.Limitations, " "), "out of order") {
		t.Fatal(r.Limitations)
	}
}

func TestTroubleshootDNSMulticastMalformedAndOtherOperations(t *testing.T) {
	mdns := troubleTestPacket(1, 0, troubleTestDNS("printer.local", 1, 0, false, 0))
	mdns.SourcePort = 5353
	mdns.DestinationPort = 5353
	mdns.Destination = "224.0.0.251"
	update := troubleTestPacket(2, 0, troubleTestDNS("example.test", 1, 99, false, 0))
	update.Record.Data = append([]byte(nil), update.Record.Data...)
	update.Record.Data[44] = 0x29 // DNS UPDATE opcode.
	bad := troubleTestPacket(3, 0, troubleTestDNS("example.test", 1, 100, false, 0))
	bad.PayloadOffset = -1
	r := Troubleshoot([]Packet{mdns, update, bad, troubleTestTail(4, 5)}, TroubleshootContext{})
	if r.Metrics.DNSQueries != 0 {
		t.Fatal(r.Metrics)
	}
	troubleTestAbsent(t, r, "dns-no-visible-reply")
	if !strings.Contains(strings.Join(r.Limitations, " "), "could not be assessed") {
		t.Fatal(r.Limitations)
	}
}

func TestTroubleshootBoundsComplexDNSQuestionAndRecordLists(t *testing.T) {
	base := troubleTestDNS("example.test", 1, 99, false, 0).Data[42:]
	for _, manyQuestions := range []bool{true, false} {
		d := append([]byte(nil), base[:12]...)
		if manyQuestions {
			binary.BigEndian.PutUint16(d[4:], 9)
			for n := 0; n < 9; n++ {
				d = append(d, base[12:]...)
			}
		} else {
			binary.BigEndian.PutUint16(d[4:], 1)
			d = append(d, base[12:]...)
			binary.BigEndian.PutUint16(d[10:], 2049)
			for n := 0; n < 2049; n++ {
				d = append(d, 0, 0, 1, 0, 1, 0, 0, 0, 0, 0, 0)
			}
		}
		u := make([]byte, 8+len(d))
		binary.BigEndian.PutUint16(u, 53000)
		binary.BigEndian.PutUint16(u[2:], 53)
		binary.BigEndian.PutUint16(u[4:], uint16(len(u)))
		copy(u[8:], d)
		p := troubleTestPacket(1, 0, testFrame(17, u))
		if p.Protocol != "DNS" {
			t.Fatal("fixture must be decoded DNS", p.Protocol)
		}
		r := Troubleshoot([]Packet{p, troubleTestTail(2, 5)}, TroubleshootContext{})
		if r.Metrics.DNSQueries != 0 {
			t.Fatal(r.Metrics)
		}
		troubleTestAbsent(t, r, "dns-no-visible-reply")
		if !strings.Contains(strings.Join(r.Limitations, " "), "could not be assessed") {
			t.Fatal(r.Limitations)
		}
	}
}

func TestTroubleshootTCPResponsesResetsAndFastOpen(t *testing.T) {
	for _, test := range []struct {
		name     string
		seq, ack uint32
		flags    byte
		payload  string
		want     string
	}{
		{"accepted", 10, 11, 0x12, "", ""}, {"refused", 10, 11, 0x14, "", "tcp-rejected"}, {"wrong-ack", 10, 12, 0x14, "", "tcp-no-visible-reply"},
		{"sequence-wrap", 0xffffffff, 0, 0x12, "", ""}, {"fast-open-data-accepted", 10, 14, 0x12, "abc", ""}, {"fast-open-syn-only-accepted", 10, 11, 0x12, "abc", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			packets := []Packet{troubleTestPacket(1, 0, troubleTestTCP(test.seq, 0, 2, test.payload, false)), troubleTestPacket(2, .1, troubleTestTCP(100, test.ack, test.flags, "", true)), troubleTestTail(3, 5)}
			r := Troubleshoot(packets, TroubleshootContext{})
			if test.want != "" {
				f := troubleTestFinding(t, r, test.want)
				if !strings.Contains(f.Observation, "192.0.2.1:40000") {
					t.Fatal(f)
				}
			} else {
				troubleTestAbsent(t, r, "tcp-no-visible-reply", "tcp-rejected")
			}
			if test.name == "wrong-ack" {
				troubleTestAbsent(t, r, "tcp-rejected")
				troubleTestFinding(t, r, "tcp-resets")
			}
		})
	}
}

func TestTroubleshootTCPRetriesDoNotInflateConnectionsAndRepeatedDataIsOnlyClue(t *testing.T) {
	a := troubleTestPacket(1, 0, troubleTestTCP(100, 0, 2, "", false))
	b := troubleTestPacket(2, 1, troubleTestTCP(100, 0, 2, "", false))
	r := Troubleshoot([]Packet{a, b, troubleTestTail(4, 5)}, TroubleshootContext{})
	if r.Metrics.TCPConnectionAttempts != 1 {
		t.Fatal(r.Metrics)
	}
	if troubleTestFinding(t, r, "tcp-no-visible-reply").Count != 1 {
		t.Fatal(r)
	}
	a = troubleTestPacket(1, 0, troubleTestTCP(100, 1, 0x18, "same", false))
	b = troubleTestPacket(2, .5, troubleTestTCP(100, 1, 0x18, "same", false))
	r = Troubleshoot([]Packet{a, b}, TroubleshootContext{})
	f := troubleTestFinding(t, r, "tcp-repeated-data")
	if f.Severity != "info" || !strings.Contains(f.Meaning, "captured more than once") || !strings.Contains(f.Meaning, "not a measured packet-loss percentage") {
		t.Fatal(f)
	}
	b = troubleTestPacket(2, .5, troubleTestTCP(100, 1, 0x18, "different", false))
	troubleTestAbsent(t, Troubleshoot([]Packet{a, b}, TroubleshootContext{}), "tcp-repeated-data")
}

func TestTroubleshootTruncatedFastOpenDoesNotInventMissingReplies(t *testing.T) {
	r := troubleTestTCP(10, 0, 2, "abc", false)
	r.OriginalLength += 20
	p := troubleTestPacket(1, 0, r)
	report := Troubleshoot([]Packet{p, troubleTestPacket(2, .1, troubleTestTCP(100, 34, 0x12, "", true)), troubleTestTail(3, 5)}, TroubleshootContext{})
	troubleTestAbsent(t, report, "tcp-no-visible-reply")
	if !strings.Contains(strings.Join(report.Limitations, " "), "full on-wire payload length is uncertain") {
		t.Fatal(report.Limitations)
	}
	// A later complete retry starts its own assessable three-second window.
	complete := troubleTestPacket(2, 4, troubleTestTCP(10, 0, 2, "abc", false))
	report = Troubleshoot([]Packet{p, complete, troubleTestTail(3, 5)}, TroubleshootContext{})
	troubleTestAbsent(t, report, "tcp-no-visible-reply")
	if report.Metrics.TCPConnectionAttempts != 1 {
		t.Fatal(report.Metrics)
	}
}

func troubleTestARP(operation uint16, source, target [4]byte) Record {
	d := make([]byte, 42)
	copy(d[:14], []byte{255, 255, 255, 255, 255, 255, 0, 1, 2, 3, 4, 5, 8, 6})
	binary.BigEndian.PutUint16(d[14:], 1)
	binary.BigEndian.PutUint16(d[16:], 0x800)
	d[18] = 6
	d[19] = 4
	binary.BigEndian.PutUint16(d[20:], operation)
	copy(d[22:28], []byte{0, 1, 2, 3, 4, 5})
	copy(d[28:32], source[:])
	copy(d[38:42], target[:])
	return Record{Data: d, OriginalLength: 42, LinkType: 1, Interface: "Ethernet"}
}

func TestTroubleshootARPProbeAnnouncementScanAndReply(t *testing.T) {
	source, target := [4]byte{192, 0, 2, 1}, [4]byte{192, 0, 2, 99}
	requests := []Packet{}
	for i := 0; i < 3; i++ {
		requests = append(requests, troubleTestPacket(uint64(i+1), float64(i), troubleTestARP(1, source, target)))
	}
	requests = append(requests, troubleTestTail(5, 5))
	f := troubleTestFinding(t, Troubleshoot(requests, TroubleshootContext{}), "arp-no-visible-reply")
	if f.Severity != "info" || !strings.Contains(f.Meaning, "normal during a network scan") || !strings.Contains(f.Observation, "192.0.2.99") {
		t.Fatal(f)
	}
	answered := append(append([]Packet(nil), requests[:3]...), troubleTestPacket(4, 2.1, troubleTestARP(2, target, source)), requests[3])
	troubleTestAbsent(t, Troubleshoot(answered, TroubleshootContext{}), "arp-no-visible-reply")
	for _, s := range [][4]byte{{0, 0, 0, 0}, target} {
		packets := []Packet{}
		for i := 0; i < 3; i++ {
			packets = append(packets, troubleTestPacket(uint64(i+1), float64(i), troubleTestARP(1, s, target)))
		}
		packets = append(packets, troubleTestTail(4, 5))
		r := Troubleshoot(packets, TroubleshootContext{})
		if r.Metrics.ARPRequests != 0 {
			t.Fatal("address probes and announcements are not unanswered lookups", r)
		}
		troubleTestAbsent(t, r, "arp-no-visible-reply")
	}
}

func TestTroubleshootICMPAndHTTPMeaning(t *testing.T) {
	packets := []Packet{troubleTestPacket(1, 0, testFrame(1, []byte{3, 4, 0, 0, 0, 0, 0, 0})), troubleTestPacket(2, 1, testFrame(1, []byte{11, 0, 0, 0, 0, 0, 0, 0})), troubleTestPacket(3, 2, testTCP(1, 0x18, "HTTP/1.1 404 Not Found\r\n\r\n"))}
	r := Troubleshoot(packets, TroubleshootContext{})
	if r.Metrics.ICMPErrors != 2 {
		t.Fatal(r.Metrics)
	}
	troubleTestFinding(t, r, "icmp-size")
	f := troubleTestFinding(t, r, "icmp-time-exceeded")
	if f.Severity != "info" || !strings.Contains(f.Meaning, "expected during many traceroute") {
		t.Fatal(f)
	}
	f = troubleTestFinding(t, r, "http-errors")
	if !strings.Contains(f.Meaning, "application-level error") || !strings.Contains(f.Meaning, "Encrypted HTTPS") {
		t.Fatal(f)
	}
}

func TestTroubleshootIPv6PacketTooBig(t *testing.T) {
	d := make([]byte, 48)
	d[0] = 0x60
	binary.BigEndian.PutUint16(d[4:], 8)
	d[6] = 58
	d[7] = 64
	copy(d[8:24], []byte{0x20, 1, 0xd, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})
	copy(d[24:40], []byte{0x20, 1, 0xd, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2})
	d[40] = 2
	p := troubleTestPacket(1, 0, Record{Data: d, OriginalLength: uint32(len(d)), LinkType: 229, Interface: "Ethernet"})
	r := Troubleshoot([]Packet{p}, TroubleshootContext{})
	if r.Metrics.ICMPErrors != 1 {
		t.Fatal(r)
	}
	f := troubleTestFinding(t, r, "icmp-size")
	if !strings.Contains(f.Observation, "2001:db8::1") {
		t.Fatal(f)
	}
}

func TestTroubleshootEvidenceIsBoundedDeterministicAndCompiles(t *testing.T) {
	packets := make([]Packet, 0, 31)
	for i := 0; i < 30; i++ {
		p := troubleTestPacket(uint64(i+1), float64(i)/100, troubleTestDNS(fmt.Sprintf("q%d.test", i), 1, uint16(i), false, 0))
		packets = append(packets, p)
	}
	packets = append(packets, troubleTestTail(31, 5))
	first := Troubleshoot(packets, TroubleshootContext{})
	f := troubleTestFinding(t, first, "dns-no-visible-reply")
	if f.Count != 30 || len(f.PacketNumbers) != 10 || f.PacketNumbers[0] != 1 || f.PacketNumbers[9] != 10 {
		t.Fatal(f)
	}
	if !strings.Contains(f.Observation, "q0.test") {
		t.Fatal("example must match retained lowest packet number", f)
	}
	predicate, err := Compile(f.Filter)
	if err != nil {
		t.Fatal(err)
	}
	for i, p := range packets {
		if predicate(p) != (i < 10) {
			t.Fatal("example filter selected incorrect packet", i, p.Number)
		}
	}
	for i := 0; i < 20; i++ {
		if !reflect.DeepEqual(first, Troubleshoot(packets, TroubleshootContext{})) {
			t.Fatal("analysis is nondeterministic")
		}
	}
}

func TestTroubleshootContextEmptyMalformedAndPureConcurrentRead(t *testing.T) {
	r := Troubleshoot(nil, TroubleshootContext{})
	if r.Findings == nil || r.Limitations == nil || !strings.Contains(r.Summary, "No retained packets") {
		t.Fatal(r)
	}
	encoded, err := json.Marshal(r)
	if err != nil || strings.Contains(string(encoded), `"findings":null`) {
		t.Fatal(string(encoded), err)
	}
	p := troubleTestPacket(1, 0, testTCP(1, 0x18, "encrypted"))
	p.Protocol = "TLS"
	p.Expert = []string{"IPv4 fragment; transport data may require reassembly", "Packet truncated by capture length"}
	p.Length += 10
	p.Record.DropReason = 0x1234
	packets := []Packet{p, troubleTestTail(2, 5)}
	before := append([]Packet(nil), packets...)
	before[0].Record.Data = append([]byte(nil), p.Record.Data...)
	before[0].Expert = append([]string(nil), p.Expert...)
	r = Troubleshoot(packets, TroubleshootContext{Active: true, Lost: 3, Overwritten: 4, KeepFiltered: true, FileMode: true})
	joined := strings.Join(r.Limitations, " ")
	for _, term := range []string{"still running", "keep filters", "capture loss, not a measurement of network packet loss", "overwritten", "circular", "fewer captured bytes", "fragments", "not decrypted"} {
		if !strings.Contains(joined, term) {
			t.Fatal("missing context", term, r.Limitations)
		}
	}
	f := troubleTestFinding(t, r, "windows-local-drop")
	if !strings.Contains(f.Observation, "0x1234") {
		t.Fatal(f)
	}
	var wait sync.WaitGroup
	for i := 0; i < 5; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for n := 0; n < 10; n++ {
				_ = Troubleshoot(packets, TroubleshootContext{})
			}
		}()
	}
	wait.Wait()
	if !reflect.DeepEqual(before, packets) {
		t.Fatal("analysis changed source packets")
	}
	for _, proto := range []string{"DNS", "ARP", "ICMP", "ICMPv6", "TCP"} {
		for _, offset := range []int{-1, 0, 1, 1000000} {
			bad := Packet{Protocol: proto, Transport: proto, Source: "192.0.2.1", Destination: "192.0.2.2", SourcePort: 53, DestinationPort: 53, PayloadOffset: offset, PayloadLength: 100, Record: Record{Data: []byte{1}, LinkType: 1}}
			_ = Troubleshoot([]Packet{bad}, TroubleshootContext{})
		}
	}
}

func BenchmarkTroubleshoot200000(b *testing.B) {
	packets := make([]Packet, 200000)
	for i := range packets {
		p := troubleTestPacket(uint64(i+1), float64(i)/1000, troubleTestTCP(uint32(i), 1, 0x18, "x", false))
		packets[i] = p
	}
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		r := Troubleshoot(packets, TroubleshootContext{})
		if r.PacketCount != len(packets) {
			b.Fatal(r.PacketCount)
		}
	}
}

func BenchmarkTroubleshootQuestionlessDNS200000(b *testing.B) {
	packets := make([]Packet, 200000)
	for i := 0; i < 100000; i++ {
		query := troubleTestDNS("example.test", 1, uint16(i), false, 0)
		reply := troubleTestDNS("", 1, uint16(i), true, 0)
		port := uint16(53000 + i/65536)
		binary.BigEndian.PutUint16(query.Data[34:], port)
		binary.BigEndian.PutUint16(reply.Data[36:], port)
		packets[i] = troubleTestPacket(uint64(i+1), float64(i)/1000, query)
		packets[i+100000] = troubleTestPacket(uint64(i+100001), 200+float64(i)/1000, reply)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		r := Troubleshoot(packets, TroubleshootContext{})
		if r.PacketCount != len(packets) {
			b.Fatal(r.PacketCount)
		}
	}
}
