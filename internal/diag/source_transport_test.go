package diag

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

func loopbackDiagnosticSource(t *testing.T) *platform.SourceBinding {
	t.Helper()
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	for _, nic := range interfaces {
		if nic.Flags&net.FlagLoopback == 0 || nic.Flags&net.FlagUp == 0 {
			continue
		}
		addresses, err := nic.Addrs()
		if err != nil {
			t.Fatal(err)
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err == nil && ip.Equal(net.ParseIP("127.0.0.1")) {
				return &platform.SourceBinding{InterfaceIndex: nic.Index, InterfaceName: nic.Name, Address: "127.0.0.1", Family: "ipv4"}
			}
		}
	}
	t.Fatal("loopback source fixture requires an active 127.0.0.1 interface")
	return nil
}

type sourceDNSObservation struct{ network, source, query string }
type sourceDNSFixture struct {
	endpoint     string
	mu           sync.Mutex
	observations []sourceDNSObservation
}

func (f *sourceDNSFixture) observe(network, source, query string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.observations = append(f.observations, sourceDNSObservation{network, source, query})
}

func (f *sourceDNSFixture) snapshot() []sourceDNSObservation {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sourceDNSObservation(nil), f.observations...)
}

// This fixture accepts only loopback DNS. It removes EDNS from the copied
// question so its answer section has valid ordering for the Go resolver too.
func sourceDNSResponse(query []byte, truncated bool) ([]byte, string, error) {
	name, next, err := dnsName(query, 12)
	if err != nil || next+4 > len(query) {
		return nil, "", fmt.Errorf("invalid fixture query: %v", err)
	}
	typ := binary.BigEndian.Uint16(query[next:])
	question := append([]byte(nil), query[:next+4]...)
	for offset := 6; offset <= 10; offset += 2 {
		binary.BigEndian.PutUint16(question[offset:], 0)
	}
	if truncated {
		binary.BigEndian.PutUint16(question[2:], 0x8380)
		return question, name, nil
	}
	switch typ {
	case 1:
		return dnsAnswer(question, 1, []byte{127, 0, 0, 1}), name, nil
	case 12:
		encoded, _ := dnsQuestion("bound-router.example", 1, 0)
		return dnsAnswer(question, 12, encoded[12:len(encoded)-4]), name, nil
	default:
		binary.BigEndian.PutUint16(question[2:], 0x8180)
		return question, name, nil
	}
}

func startSourceDNSFixture(t *testing.T, truncate bool) *sourceDNSFixture {
	t.Helper()
	// TCP and UDP allocate ephemeral ports independently. Reserve TCP first
	// and retry the pair if its matching UDP port is already occupied.
	var udp net.PacketConn
	var tcp net.Listener
	var err error
	for attempt := 0; attempt < 10; attempt++ {
		tcp, err = net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		udp, err = net.ListenPacket("udp4", tcp.Addr().String())
		if err == nil {
			break
		}
		_ = tcp.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	fixture := &sourceDNSFixture{endpoint: udp.LocalAddr().String()}
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		buffer := make([]byte, 4096)
		for {
			n, remote, err := udp.ReadFrom(buffer)
			if err != nil {
				return
			}
			reply, name, err := sourceDNSResponse(buffer[:n], truncate)
			if err != nil {
				t.Error(err)
				continue
			}
			fixture.observe("udp", remote.String(), name)
			if _, err := udp.WriteTo(reply, remote); err != nil {
				return
			}
		}
	}()
	go func() {
		defer workers.Done()
		for {
			conn, err := tcp.Accept()
			if err != nil {
				return
			}
			func() {
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(2 * time.Second))
				var prefix [2]byte
				if _, err := io.ReadFull(conn, prefix[:]); err != nil {
					t.Error(err)
					return
				}
				query := make([]byte, binary.BigEndian.Uint16(prefix[:]))
				if _, err := io.ReadFull(conn, query); err != nil {
					t.Error(err)
					return
				}
				reply, name, err := sourceDNSResponse(query, false)
				if err != nil {
					t.Error(err)
					return
				}
				fixture.observe("tcp", conn.RemoteAddr().String(), name)
				binary.BigEndian.PutUint16(prefix[:], uint16(len(reply)))
				if _, err := conn.Write(append(prefix[:], reply...)); err != nil {
					t.Error(err)
				}
			}()
		}
	}()
	t.Cleanup(func() { udp.Close(); tcp.Close(); workers.Wait() })
	return fixture
}

func requestForBinding(tool, target string, binding *platform.SourceBinding) model.Request {
	return model.Request{Tool: tool, Target: target, Options: map[string]any{"sourceInterfaceIndex": binding.InterfaceIndex, "sourceInterfaceName": binding.InterfaceName, "sourceAddress": binding.Address}}
}

func sourceFixtureResolver(binding *platform.SourceBinding) sourceResolver {
	return func(context.Context, int, string, string) (*platform.SourceBinding, error) { return binding, nil }
}

func TestSelectedDNSAndHostnameResolutionUseBoundUDPAndTCP(t *testing.T) {
	fixture := startSourceDNSFixture(t, true)
	binding := loopbackDiagnosticSource(t)
	binding.DNSServers = []string{"2001:db8::53", fixture.endpoint}
	ctx := platform.WithSourceBinding(context.Background(), binding)
	addresses, err := resolveHost(ctx, "bound-only.invalid.", time.Second)
	if err != nil || fmt.Sprint(addresses) != "[127.0.0.1]" {
		t.Fatalf("bound forward lookup: %v %v", addresses, err)
	}
	identity := resolveHostname(ctx, "192.0.2.77", systemReverseLookup)
	if identity.Hostname != "bound-router.example" || identity.HostnameSource != "Selected-source reverse lookup" {
		t.Fatalf("bound PTR lookup: %+v", identity)
	}
	req := requestForBinding("dns", "bound-only.invalid", binding)
	req.Options["recordType"] = "A"
	out, err := runWithSource(context.Background(), req, nil, sourceFixtureResolver(binding))
	if err != nil {
		t.Fatal(err)
	}
	data := out.Data.(map[string]any)
	records := data["records"].([]DNSRecord)
	if len(records) != 1 || records[0].Value != "127.0.0.1" || records[0].TTL == nil || data["server"] != fixture.endpoint || data["source"].(*SourceMetadata).DNSServer != fixture.endpoint {
		t.Fatalf("selected DNS result did not use its configured server: %+v", data)
	}
	counts := map[string]int{}
	for _, observed := range fixture.snapshot() {
		host, _, err := net.SplitHostPort(observed.source)
		if err != nil || host != binding.Address {
			t.Errorf("DNS source changed: %+v", observed)
		}
		counts[observed.network]++
	}
	if counts["udp"] != 3 || counts["tcp"] != 3 {
		t.Fatalf("UDP/TCP retry transport coverage: %+v", fixture.snapshot())
	}
}

func TestSelectedDNSExplicitServerOverridesAdapterAndSilentServerDoesNotFallback(t *testing.T) {
	fixture := startSourceDNSFixture(t, false)
	binding := loopbackDiagnosticSource(t)
	// No adapter DNS is configured. The explicit literal server is sufficient
	// for the DNS tool without bootstrapping a system DNS lookup.
	req := requestForBinding("dns", "bound-only.invalid", binding)
	req.Options["server"] = fixture.endpoint
	out, err := runWithSource(context.Background(), req, nil, sourceFixtureResolver(binding))
	if err != nil || out.Data.(map[string]any)["source"].(*SourceMetadata).NameResolution != "bound_dns" {
		t.Fatalf("explicit bound DNS: %+v %v", out, err)
	}
	silent, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer silent.Close()
	req.Target = "localhost"
	req.Options["server"], req.Options["timeoutMs"] = silent.LocalAddr().String(), 100
	started := time.Now()
	out, err = runWithSource(context.Background(), req, nil, sourceFixtureResolver(binding))
	if err == nil || time.Since(started) > time.Second || len(out.Data.(map[string]any)["records"].([]DNSRecord)) != 0 {
		t.Fatalf("silent explicit server used hosts/system fallback: %+v %v", out, err)
	}
}

func TestSelectedHTTPRedirectsKeepSourceAndOriginalHost(t *testing.T) {
	fixture := startSourceDNSFixture(t, false)
	binding := loopbackDiagnosticSource(t)
	binding.DNSServers = []string{fixture.endpoint}
	var mu sync.Mutex
	var hosts, sources []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hosts = append(hosts, r.Host)
		sources = append(sources, r.RemoteAddr)
		mu.Unlock()
		if r.URL.Path == "/start" {
			_, port, _ := net.SplitHostPort(r.Host)
			http.Redirect(w, r, "http://second-bound.invalid:"+port+"/end", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	port := server.Listener.Addr().(*net.TCPAddr).Port
	req := requestForBinding("http", fmt.Sprintf("http://first-bound.invalid:%d/start", port), binding)
	calls := 0
	out, err := runWithSource(context.Background(), req, nil, func(context.Context, int, string, string) (*platform.SourceBinding, error) {
		calls++
		return binding, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	data := out.Data.(map[string]any)
	hops := data["redirects"].([]HTTPHop)
	mu.Lock()
	gotHosts, gotSources := append([]string(nil), hosts...), append([]string(nil), sources...)
	mu.Unlock()
	if calls != 1 || len(hops) != 2 || data["statusCode"] != http.StatusNoContent || len(gotHosts) != 2 || gotHosts[0] != fmt.Sprintf("first-bound.invalid:%d", port) || gotHosts[1] != fmt.Sprintf("second-bound.invalid:%d", port) {
		t.Fatalf("redirect source/host context: %+v hosts=%v calls=%d", data, gotHosts, calls)
	}
	for _, source := range gotSources {
		host, _, _ := net.SplitHostPort(source)
		if host != binding.Address {
			t.Errorf("HTTP used source %s", source)
		}
	}
	// A stale binding must fail in socket setup even though Automatic routing
	// could reach this local listener. No unbound retry is permitted.
	bad := *binding
	bad.InterfaceName = "missing-toolkit-fixture-interface"
	bad.InterfaceIndex = 2147483646
	bad.DNSServers = nil
	badRequest := requestForBinding("http", server.URL+"/end", &bad)
	_, err = runWithSource(context.Background(), badRequest, nil, sourceFixtureResolver(&bad))
	if err == nil || !strings.Contains(err.Error(), "bind diagnostic socket") {
		t.Fatalf("stale source used Automatic HTTP: %v", err)
	}
	mu.Lock()
	count := len(hosts)
	mu.Unlock()
	if count != 2 {
		t.Fatalf("stale source sent an unbound request: %d", count)
	}
}

func TestSelectedTCPAndMonitorUseBoundResolvedDestination(t *testing.T) {
	fixture := startSourceDNSFixture(t, false)
	binding := loopbackDiagnosticSource(t)
	binding.DNSServers = []string{fixture.endpoint}
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	listener.SetDeadline(time.Now().Add(3 * time.Second))
	sources := make(chan string, 2)
	go func() {
		for i := 0; i < 2; i++ {
			conn, err := listener.AcceptTCP()
			if err != nil {
				sources <- "error:" + err.Error()
				return
			}
			sources <- conn.RemoteAddr().String()
			conn.Close()
		}
	}()
	port := listener.Addr().(*net.TCPAddr).Port
	for _, tool := range []string{"tcp", "monitor"} {
		req := requestForBinding(tool, "tcp-bound.invalid.", binding)
		req.Options["ports"] = strconv.Itoa(port)
		req.Options["mode"], req.Options["port"], req.Options["count"] = "tcp", port, 1
		out, err := runWithSource(context.Background(), req, nil, sourceFixtureResolver(binding))
		if err != nil {
			t.Fatalf("%s: %v", tool, err)
		}
		data := out.Data.(map[string]any)
		if tool == "tcp" {
			ports := data["ports"].([]PortResult)
			if len(ports) != 1 || ports[0].State != "open" || ports[0].Address != listener.Addr().String() {
				t.Fatalf("TCP lost bound resolved endpoint: %+v", data)
			}
		} else {
			samples := data["samples"].([]platform.PingReply)
			if len(samples) != 1 || !samples[0].Success || samples[0].Address != listener.Addr().String() {
				t.Fatalf("Monitor lost bound resolved endpoint: %+v", data)
			}
		}
		host, _, err := net.SplitHostPort(<-sources)
		if err != nil || host != binding.Address {
			t.Fatalf("%s used source %q: %v", tool, host, err)
		}
	}
}

func TestSelectedTLSKeepsSNIAndCertificateValidation(t *testing.T) {
	fixture := startSourceDNSFixture(t, false)
	binding := loopbackDiagnosticSource(t)
	binding.DNSServers = []string{fixture.endpoint}
	hellos := make(chan string, 8)
	sources := make(chan string, 8)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	server.TLS = &tls.Config{GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
		hellos <- hello.ServerName
		sources <- hello.Conn.RemoteAddr().String()
		return nil, nil
	}}
	server.StartTLS()
	defer server.Close()
	port := server.Listener.Addr().(*net.TCPAddr).Port
	req := requestForBinding("tls", "tls-bound.invalid", binding)
	req.Options["port"] = port
	out, err := runWithSource(context.Background(), req, nil, sourceFixtureResolver(binding))
	if err != nil {
		t.Fatal(err)
	}
	data := out.Data.(map[string]any)
	if data["serverName"] != "tls-bound.invalid" || data["verified"] != false || data["verificationError"] == "" || data["verificationEnforced"] != false {
		t.Fatalf("TLS diagnostics lost original identity or verification failure: %+v", data)
	}
	httpReq := requestForBinding("http", "https://tls-bound.invalid:"+strconv.Itoa(port), binding)
	if _, err := runWithSource(context.Background(), httpReq, nil, sourceFixtureResolver(binding)); err == nil {
		t.Fatal("selected-source HTTPS silently accepted an untrusted certificate")
	}
	// Literal IP requests do not send TLS SNI. Their actual URL IP must still
	// reach certificate trust checking rather than become an empty identity.
	ipReq := requestForBinding("http", server.URL, binding)
	if _, err := runWithSource(context.Background(), ipReq, nil, sourceFixtureResolver(binding)); err == nil || strings.Contains(err.Error(), "requires the server hostname") || strings.Contains(err.Error(), "offline server identity check") {
		t.Fatalf("literal HTTPS IP was lost during offline verification: %v", err)
	}
	httpReq.Options["verifyTLS"] = false
	out, err = runWithSource(context.Background(), httpReq, nil, sourceFixtureResolver(binding))
	if err != nil || out.Data.(map[string]any)["tls"].(map[string]any)["verified"] != false {
		t.Fatalf("explicit diagnostic certificate collection changed: %+v %v", out, err)
	}
	for i := 0; i < 4; i++ {
		want := "tls-bound.invalid"
		if i == 2 {
			want = ""
		}
		if name := <-hellos; name != want {
			t.Errorf("SNI replaced with IP: %q", name)
		}
		host, _, _ := net.SplitHostPort(<-sources)
		if host != binding.Address {
			t.Errorf("TLS source changed to %q", host)
		}
	}
}
