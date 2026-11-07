package capture

import (
	"encoding/binary"
	"strconv"
	"strings"
	"testing"
)

var (
	targetClient = [4]byte{192, 0, 2, 1}
	targetServer = [4]byte{203, 0, 113, 80}
)

// targetAddresses rewrites a test IPv4 frame's addresses (and keeps ports).
func targetAddresses(r Record, src, dst [4]byte) Record {
	out := r
	out.Data = append([]byte(nil), r.Data...)
	copy(out.Data[26:30], src[:])
	copy(out.Data[30:34], dst[:])
	return out
}

func targetPorts(r Record, src, dst uint16) Record {
	out := r
	out.Data = append([]byte(nil), r.Data...)
	binary.BigEndian.PutUint16(out.Data[34:], src)
	binary.BigEndian.PutUint16(out.Data[36:], dst)
	return out
}

// targetDNSAnswer builds a DNS response for name with one A record.
func targetDNSAnswer(name string, id uint16, address [4]byte) Record {
	r := troubleTestDNS(name, 1, id, true, 0)
	d := r.Data[42:]
	binary.BigEndian.PutUint16(d[6:], 1)
	answer := []byte{0xc0, 12, 0, 1, 0, 1, 0, 0, 0, 60, 0, 4, address[0], address[1], address[2], address[3]}
	data := append(append([]byte(nil), r.Data...), answer...)
	binary.BigEndian.PutUint16(data[16:], uint16(len(data)-14))
	binary.BigEndian.PutUint16(data[38:], uint16(len(data)-34))
	r.Data, r.OriginalLength = data, uint32(len(data))
	return r
}

func clientToServer(r Record) Record { return targetAddresses(r, targetClient, targetServer) }
func serverToClient(r Record) Record { return targetAddresses(r, targetServer, targetClient) }

func targetStep(t *testing.T, c TargetCheck, id string) TargetStep {
	t.Helper()
	for _, s := range c.Steps {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("missing step %s in %#v", id, c.Steps)
	return TargetStep{}
}

func workingWebsite() []Packet {
	return []Packet{
		troubleTestPacket(1, 0, troubleTestDNS("www.example.test", 1, 7, false, 0)),
		troubleTestPacket(2, 0.02, targetDNSAnswer("www.example.test", 7, targetServer)),
		troubleTestPacket(3, 0.05, clientToServer(troubleTestTCP(100, 0, 0x02, "", false))),
		troubleTestPacket(4, 0.07, serverToClient(troubleTestReverse(troubleTestTCP(900, 101, 0x12, "", false)))),
		troubleTestPacket(5, 0.08, clientToServer(troubleTestTCP(101, 901, 0x18, "GET / HTTP/1.1\r\nHost: www.example.test\r\n\r\n", false))),
		troubleTestPacket(6, 0.12, serverToClient(troubleTestReverse(troubleTestTCP(901, 140, 0x18, "HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n", false)))),
		troubleTestTail(7, 5),
	}
}

func TestCheckTargetFollowsAWorkingWebsite(t *testing.T) {
	c, err := CheckTarget(workingWebsite(), "http://www.example.test/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if c.Host != "www.example.test" || c.Port != 80 || c.State != "ok" || len(c.Addresses) != 1 || c.Addresses[0] != "203.0.113.80" {
		t.Fatalf("working website: %#v", c)
	}
	for _, id := range []string{"lookup", "reach", "connect", "respond"} {
		if s := targetStep(t, c, id); s.State != "ok" || s.Filter == "" {
			t.Fatalf("step %s should be ok with example packets: %#v", id, s)
		}
	}
	if !strings.Contains(targetStep(t, c, "connect").Detail, "Web (HTTP) (port 80)") {
		t.Fatalf("service not named: %s", targetStep(t, c, "connect").Detail)
	}
}

func TestCheckTargetStopsAtTheFailingStep(t *testing.T) {
	missing := []Packet{
		troubleTestPacket(1, 0, troubleTestDNS("missing.example.test", 1, 9, false, 0)),
		troubleTestPacket(2, 0.03, troubleTestDNS("missing.example.test", 1, 9, true, 3)),
		troubleTestTail(3, 5),
	}
	c, _ := CheckTarget(missing, "missing.example.test")
	if c.State != "problem" || targetStep(t, c, "lookup").State != "fail" || !strings.Contains(c.Conclusion, "name lookup") {
		t.Fatalf("NXDOMAIN should stop at the lookup: %#v", c)
	}

	refused := []Packet{
		troubleTestPacket(1, 0, clientToServer(targetPorts(troubleTestTCP(100, 0, 0x02, "", false), 40000, 3389))),
		troubleTestPacket(2, 0.01, serverToClient(targetPorts(troubleTestTCP(0, 101, 0x14, "", false), 3389, 40000))),
		troubleTestTail(3, 5),
	}
	c, _ = CheckTarget(refused, "203.0.113.80:3389")
	if c.State != "problem" || targetStep(t, c, "lookup").State != "skipped" || targetStep(t, c, "reach").State != "ok" || targetStep(t, c, "connect").State != "fail" {
		t.Fatalf("refused connection: %#v", c)
	}
	if !strings.Contains(targetStep(t, c, "connect").Detail, "Remote Desktop (port 3389)") || !strings.Contains(c.Conclusion, "not accepted") {
		t.Fatalf("refused wording: %s / %s", targetStep(t, c, "connect").Detail, c.Conclusion)
	}

	silent := []Packet{
		troubleTestPacket(1, 0, clientToServer(troubleTestTCP(100, 0, 0x02, "", false))),
		troubleTestPacket(2, 1, clientToServer(troubleTestTCP(100, 0, 0x02, "", false))),
		troubleTestTail(3, 6),
	}
	c, _ = CheckTarget(silent, "203.0.113.80")
	if c.State != "problem" || targetStep(t, c, "reach").State != "fail" || !strings.Contains(c.Conclusion, "never replied") {
		t.Fatalf("no reply should stop before the device answers: %#v", c)
	}

	short := []Packet{troubleTestPacket(1, 0, clientToServer(troubleTestTCP(100, 0, 0x02, "", false))), troubleTestTail(2, 1)}
	c, _ = CheckTarget(short, "203.0.113.80")
	if c.State != "warn" || targetStep(t, c, "reach").State != "warn" {
		t.Fatalf("a recording that ends too soon must not be called a failure: %#v", c)
	}
}

func TestCheckTargetWithoutTrafficAndInvalidInput(t *testing.T) {
	c, err := CheckTarget(workingWebsite(), "other.example.test")
	if err != nil || c.State != "unknown" || !strings.Contains(c.Conclusion, "no traffic") {
		t.Fatalf("unrelated target: %#v %v", c, err)
	}
	for _, bad := range []string{"", "bad host!", "example.test:99999", strings.Repeat("a", 600)} {
		if _, err := CheckTarget(nil, bad); err == nil {
			t.Fatalf("%q was accepted", bad)
		}
	}
	for raw, want := range map[string]string{"https://Example.TEST/x": "example.test:443", "[2001:db8::1]:22": "2001:db8::1:22", "192.0.2.5": "192.0.2.5:0", "printer.local.": "printer.local:0"} {
		host, port, err := ParseTarget(raw)
		if err != nil || host+":"+strconv.Itoa(int(port)) != want {
			t.Fatalf("ParseTarget(%q) = %s:%d %v, want %s", raw, host, port, err, want)
		}
	}
}

func TestNamesServicesOutcomesAndVerdict(t *testing.T) {
	packets := workingWebsite()
	names := HostNames(packets)
	if names["203.0.113.80"] != "www.example.test" {
		t.Fatalf("DNS-learned names: %#v", names)
	}
	stats := Stats(packets)
	var web *Conversation
	for i := range stats.Conversations {
		if stats.Conversations[i].ServicePort == 80 {
			web = &stats.Conversations[i]
		}
	}
	if web == nil || web.Outcome != "worked" || web.Service != "Web (HTTP)" || web.NameB != "www.example.test" {
		t.Fatalf("web conversation: %#v", web)
	}
	refused := Stats([]Packet{
		troubleTestPacket(1, 0, clientToServer(targetPorts(troubleTestTCP(100, 0, 0x02, "", false), 40000, 445))),
		troubleTestPacket(2, 0.01, serverToClient(targetPorts(troubleTestTCP(0, 101, 0x14, "", false), 445, 40000))),
	})
	if len(refused.Conversations) != 1 || refused.Conversations[0].Outcome != "refused" || refused.Conversations[0].Service != "File sharing (SMB)" {
		t.Fatalf("refused conversation: %#v", refused.Conversations)
	}
	if v := Troubleshoot(nil, TroubleshootContext{}).Verdict; v.State != "empty" {
		t.Fatalf("empty verdict: %#v", v)
	}
	if v := Troubleshoot(packets[:3], TroubleshootContext{}).Verdict; v.State != "little" {
		t.Fatalf("small capture verdict: %#v", v)
	}
	nx := append(workingWebsite(), troubleTestPacket(8, 1, troubleTestDNS("missing.example.test", 1, 9, true, 3)))
	report := Troubleshoot(nx, TroubleshootContext{})
	f := troubleTestFinding(t, report, "dns-name-not-found")
	if report.Verdict.State != "problems" || report.Verdict.Headline != "1 problem found" || f.Headline != "The name missing.example.test does not exist" || !strings.Contains(f.Technical, "NXDOMAIN") {
		t.Fatalf("plain finding: %#v %#v", report.Verdict, f)
	}
	if strings.Contains(f.Meaning, "NXDOMAIN") {
		t.Fatalf("jargon in the plain explanation: %s", f.Meaning)
	}
}
