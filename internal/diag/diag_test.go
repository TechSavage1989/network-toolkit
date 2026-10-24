package diag

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

func TestPortValidation(t *testing.T) {
	for _, spec := range []string{"", ",80", "80,", "0", "65536", "20-10", "1-65535", "1-2-3", "22;whoami", "443/udp"} {
		if _, err := parsePorts(spec, 1024); err == nil {
			t.Errorf("accepted invalid ports %q", spec)
		}
	}
	ports, err := parsePorts("443, 20-22,22,80", 1024)
	if err != nil || fmt.Sprint(ports) != "[20 21 22 80 443]" {
		t.Fatalf("unexpected ports %v: %v", ports, err)
	}
	if _, err = parsePorts("1-1024", 1024); err != nil {
		t.Fatal(err)
	}
	if _, err = parsePorts("1-1024,2048", 1024); err == nil {
		t.Fatal("combined range exceeded limit")
	}
}

func TestCIDRBoundaries(t *testing.T) {
	for _, spec := range []string{"0.0.0.0/0", "10.0.0.0/21", "10.0.0.0/33", "::1", "example.com", "192.168.1.999"} {
		if _, err := ipv4Targets(spec, 1024); err == nil {
			t.Errorf("accepted %q", spec)
		}
	}
	cases := []struct {
		spec        string
		count       int
		first, last string
	}{{"192.168.1.58/30", 2, "192.168.1.57", "192.168.1.58"}, {"192.168.1.0/31", 2, "192.168.1.0", "192.168.1.1"}, {"192.168.1.2/32", 1, "192.168.1.2", "192.168.1.2"}, {"192.168.1.2", 1, "192.168.1.2", "192.168.1.2"}, {"192.168.0.0/22", 1022, "192.168.0.1", "192.168.3.254"}}
	for _, tc := range cases {
		out, err := ipv4Targets(tc.spec, 1024)
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != tc.count || out[0].String() != tc.first || out[len(out)-1].String() != tc.last {
			t.Errorf("%s produced %d %v", tc.spec, len(out), out)
		}
	}
}

func dnsAnswer(question []byte, typ uint16, rdata []byte) []byte {
	msg := append([]byte(nil), question...)
	binary.BigEndian.PutUint16(msg[2:], 0x8180)
	binary.BigEndian.PutUint16(msg[6:], 1)
	msg = append(msg, 0xc0, 0x0c, byte(typ>>8), byte(typ), 0, 1, 0, 0, 0, 60, byte(len(rdata)>>8), byte(len(rdata)))
	return append(msg, rdata...)
}

func TestDNSParser(t *testing.T) {
	cases := []struct {
		kind string
		data []byte
		want string
	}{
		{"A", []byte{192, 0, 2, 7}, "192.0.2.7"},
		{"AAAA", net.ParseIP("2001:db8::7").To16(), "2001:db8::7"},
		{"CNAME", []byte{0xc0, 0x0c}, "example.test."},
		{"MX", []byte{0, 10, 0xc0, 0x0c}, "10 example.test."},
		{"TXT", []byte{3, 'a', 'b', 'c', 2, 'd', 'e'}, "abcde"},
		{"SRV", []byte{0, 1, 0, 2, 1, 187, 0xc0, 0x0c}, "1 2 443 example.test."},
		{"SOA", []byte{0xc0, 0x0c, 0xc0, 0x0c, 0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0, 3, 0, 0, 0, 4, 0, 0, 0, 5}, "example.test. example.test. 1 2 3 4 5"},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			q, err := dnsQuestion("example.test", dnsTypes[tc.kind], 55)
			if err != nil {
				t.Fatal(err)
			}
			rows, status, err := parseDNS(dnsAnswer(q, dnsTypes[tc.kind], tc.data), 55, "example.test", dnsTypes[tc.kind])
			if err != nil || status != "NOERROR" || len(rows) != 1 {
				t.Fatalf("rows=%v status=%s err=%v", rows, status, err)
			}
			if rows[0].Value != tc.want || rows[0].TTL == nil || *rows[0].TTL != 60 || rows[0].Section != "answer" {
				t.Fatalf("unexpected row: %+v", rows[0])
			}
		})
	}
}

func TestDNSRejectsMalformedReplies(t *testing.T) {
	q, _ := dnsQuestion("example.test", 1, 77)
	valid := dnsAnswer(q, 1, []byte{192, 0, 2, 9})
	if _, _, err := parseDNS(valid, 78, "example.test", 1); err == nil {
		t.Fatal("accepted wrong ID")
	}
	if _, _, err := parseDNS(valid, 77, "different.test", 1); err == nil {
		t.Fatal("accepted wrong question")
	}
	if _, _, err := parseDNS(valid, 77, "example.test", 28); err == nil {
		t.Fatal("accepted wrong question type")
	}
	if _, _, err := parseDNS(valid[:len(valid)-1], 77, "example.test", 1); err == nil {
		t.Fatal("accepted truncated data")
	}
	loop := append([]byte(nil), valid...)
	loop[12] = 0xc0
	loop[13] = 12
	if _, _, err := parseDNS(loop, 77, "example.test", 1); err == nil {
		t.Fatal("accepted pointer loop")
	}
	wrongLength := dnsAnswer(q, 1, []byte{1, 2, 3})
	if _, _, err := parseDNS(wrongLength, 77, "example.test", 1); err == nil {
		t.Fatal("accepted short IPv4")
	}
	if _, err := dnsQuestion(strings.Repeat("a", 64)+".test", 1, 1); err == nil {
		t.Fatal("accepted oversized label")
	}
	for _, input := range [][]byte{{}, {1}, {0, 1, 2}, make([]byte, 12)} {
		if _, _, err := parseDNS(input, 1, "x.test", 1); err == nil {
			t.Fatal("accepted short malformed message")
		}
	}
}

func TestDNSExplicitServerAndTCPFallback(t *testing.T) {
	udp, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	tcp, err := net.Listen("tcp4", udp.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer tcp.Close()
	errs := make(chan error, 2)
	go func() {
		buf := make([]byte, 1024)
		n, addr, e := udp.ReadFrom(buf)
		if e != nil {
			errs <- e
			return
		}
		reply := append([]byte(nil), buf[:n]...)
		binary.BigEndian.PutUint16(reply[2:], 0x8380)
		_, e = udp.WriteTo(reply, addr)
		errs <- e
	}()
	go func() {
		c, e := tcp.Accept()
		if e != nil {
			errs <- e
			return
		}
		defer c.Close()
		c.SetDeadline(time.Now().Add(time.Second * 3))
		var prefix [2]byte
		if _, e = io.ReadFull(c, prefix[:]); e != nil {
			errs <- e
			return
		}
		q := make([]byte, binary.BigEndian.Uint16(prefix[:]))
		if _, e = io.ReadFull(c, q); e != nil {
			errs <- e
			return
		}
		reply := dnsAnswer(q, 1, []byte{192, 0, 2, 101})
		binary.BigEndian.PutUint16(prefix[:], uint16(len(reply)))
		_, e = c.Write(append(prefix[:], reply...))
		errs <- e
	}()
	rows, status, err := wireLookup(context.Background(), "unique-test.invalid", 1, udp.LocalAddr().String(), time.Second*2)
	if err != nil || status != "NOERROR" || len(rows) != 1 || rows[0].Value != "192.0.2.101" {
		t.Fatalf("explicit DNS failed: %v %s %v", rows, status, err)
	}
	for i := 0; i < 2; i++ {
		if e := <-errs; e != nil {
			t.Fatal(e)
		}
	}
}

func TestDNSServerTimeoutNoFallback(t *testing.T) {
	udp, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	start := time.Now()
	_, _, err = wireLookup(context.Background(), "localhost", 1, udp.LocalAddr().String(), 100*time.Millisecond)
	if err == nil {
		t.Fatal("silent explicit server unexpectedly resolved localhost")
	}
	if time.Since(start) > time.Second {
		t.Fatal("DNS timeout was not enforced")
	}
}

func TestTCPStatesAndCancellation(t *testing.T) {
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	p := dialPort(context.Background(), "127.0.0.1", port, time.Second)
	if p.State != "open" {
		t.Fatalf("open listener reported %s: %s", p.State, p.Error)
	}
	l.Close()
	p = dialPort(context.Background(), "127.0.0.1", port, time.Second)
	if p.State != "closed" {
		t.Fatalf("closed listener reported %s: %s", p.State, p.Error)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	rows := checkPorts(ctx, "127.0.0.1", []int{1, 2, 3}, time.Second, 2, nil)
	if len(rows) != 0 || time.Since(start) > time.Second {
		t.Fatalf("cancelled work ran: %+v", rows)
	}
	if _, err = Run(ctx, model.Request{Tool: "tcp", Target: "127.0.0.1"}, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run did not preserve cancellation: %v", err)
	}
}

func TestHTTPRedirectTimingBodyLimitAndHeaderRedaction(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/done", http.StatusFound)
			return
		}
		w.Header().Set("X-Diagnostic", "present")
		w.Header().Set("Set-Cookie", "session=private-token")
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, strings.Repeat("A", 100000))
	}))
	defer s.Close()
	result, err := httpCheck(context.Background(), model.Request{Target: s.URL + "/start", Options: map[string]any{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d := result.Data.(map[string]any)
	if d["statusCode"] != 201 || len(d["redirects"].([]HTTPHop)) != 2 || d["bodyTruncated"] != true || d["bodyBytesRead"] != int64(65536) {
		t.Fatalf("unexpected HTTP result: %+v", d)
	}
	for _, h := range d["headers"].([]HTTPHeader) {
		if h.Name == "Set-Cookie" && h.Value != "[redacted]" {
			t.Fatal("cookie value was not redacted")
		}
	}
	hops := d["redirects"].([]HTTPHop)
	if hops[1].FirstByteMs == nil || hops[1].TotalMs < 0 || hops[1].RemoteAddress == "" {
		t.Fatalf("missing timing: %+v", hops[1])
	}
	result, err = httpCheck(context.Background(), model.Request{Target: s.URL + "/start", Options: map[string]any{"redirects": false}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Data.(map[string]any)["statusCode"] != 302 {
		t.Fatal("redirect option was ignored")
	}
}

func TestHTTPFailuresRemainFailures(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/loop" {
			http.Redirect(w, r, "/loop", http.StatusFound)
			return
		}
		time.Sleep(250 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer s.Close()
	_, err := httpCheck(context.Background(), model.Request{Target: s.URL + "/loop", Options: map[string]any{}}, nil)
	if err == nil || !strings.Contains(err.Error(), "loop") {
		t.Fatalf("redirect loop not detected: %v", err)
	}
	_, err = httpCheck(context.Background(), model.Request{Target: s.URL, Options: map[string]any{"timeoutMs": 100}}, nil)
	if err == nil {
		t.Fatal("HTTP timeout was ignored")
	}
	for _, url := range []string{"file:///etc/passwd", "ftp://example.com", "http://user:password@example.com", "http://127.0.0.1:0", "http://127.0.0.1:65536"} {
		if _, err := diagnosticURL(url); err == nil {
			t.Errorf("accepted %s", url)
		}
	}
}

func TestTLSInvalidCertificateIsCollectedAndReported(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer s.Close()
	_, err := httpCheck(context.Background(), model.Request{Target: s.URL, Options: map[string]any{}}, nil)
	if err == nil {
		t.Fatal("HTTP trusted self-signed certificate by default")
	}
	result, err := httpCheck(context.Background(), model.Request{Target: s.URL, Options: map[string]any{"verifyTLS": false}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	meta := result.Data.(map[string]any)["tls"].(map[string]any)
	if meta["verified"] != false || meta["verificationError"] == "" {
		t.Fatalf("invalid certificate not reported: %+v", meta)
	}
	port := s.Listener.Addr().(*net.TCPAddr).Port
	result, err = tlsCheck(context.Background(), model.Request{Target: "127.0.0.1", Options: map[string]any{"port": port}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	meta = result.Data.(map[string]any)
	certs := meta["certificates"].([]Certificate)
	if meta["verified"] != false || meta["verificationError"] == "" || len(certs) == 0 || len(certs[0].SHA256) != 64 {
		t.Fatalf("TLS result incorrect: %+v", meta)
	}
}

func TestStatisticsDoesNotInventLatency(t *testing.T) {
	s := statistics([]platform.PingReply{{Success: false}, {Success: false}})
	if s.LossPercent != 100 || s.AvgMs != nil || s.MinMs != nil || s.MaxMs != nil {
		t.Fatalf("unanswered samples produced RTT: %+v", s)
	}
	s = statistics([]platform.PingReply{{Success: true, RTTMs: 10}, {Success: true, RTTMs: 20}, {Success: false}, {Success: true, RTTMs: 40}})
	if s.Received != 3 || s.Lost != 1 || s.LossPercent != 25 || s.JitterMs == nil || *s.JitterMs != 10 || s.AvgMs == nil || *s.AvgMs != 70.0/3 {
		t.Fatalf("bad statistics: %+v", s)
	}
}

func FuzzDNSParserNeverPanics(f *testing.F) {
	q, _ := dnsQuestion("example.test", 1, 77)
	f.Add(dnsAnswer(q, 1, []byte{192, 0, 2, 1}))
	f.Add([]byte{})
	f.Add([]byte{0, 77, 0x81, 0x80})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65535 {
			return
		}
		parseDNS(data, 77, "example.test", 1)
	})
}
