package diag

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"networktoolkit/internal/deviceinfo"
	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

func selectedSourceRequest(tool, target string) model.Request {
	return model.Request{Tool: tool, Target: target, Options: map[string]any{
		"sourceInterfaceIndex": 7, "sourceInterfaceName": "Fixture Ethernet", "sourceAddress": "192.0.2.20",
	}}
}

func diagnosticSourceFixture() *platform.SourceBinding {
	return &platform.SourceBinding{InterfaceIndex: 7, IPv6InterfaceIndex: 17, InterfaceName: "Fixture Ethernet", InterfaceDescription: "Fixture adapter description", InterfaceID: "fixture-id", Address: "192.0.2.20", Family: "ipv4", DNSServers: []string{"2001:db8::53", "192.0.2.53"}}
}

func TestDiagnosticSourceOptionValidation(t *testing.T) {
	for _, tool := range []string{"quick", "ping", "trace", "tcp", "monitor", "http", "tls", "discover", "identify", "dns"} {
		if !SupportsSourceBinding(tool) || ValidateSourceOptions(selectedSourceRequest(tool, "192.0.2.1")) != nil {
			t.Errorf("supported tool %q rejected a valid selection", tool)
		}
	}
	for _, tool := range []string{"wol", "routes", "arp", "connections", "wifi", "ipconfig", "terminal", "unknown"} {
		if SupportsSourceBinding(tool) || ValidateSourceOptions(selectedSourceRequest(tool, "192.0.2.1")) == nil {
			t.Errorf("unsupported tool %q accepted an explicit source", tool)
		}
		if err := ValidateSourceOptions(model.Request{Tool: tool}); err != nil {
			t.Errorf("Automatic %q was affected: %v", tool, err)
		}
	}
	for _, tc := range []struct {
		key   string
		value any
	}{
		{"sourceInterfaceIndex", nil}, {"sourceInterfaceIndex", "7"}, {"sourceInterfaceIndex", 0}, {"sourceInterfaceIndex", -1}, {"sourceInterfaceIndex", 7.5}, {"sourceInterfaceIndex", float64(2147483648)},
		{"sourceAddress", nil}, {"sourceAddress", 7}, {"sourceAddress", ""}, {"sourceAddress", "host.invalid"}, {"sourceAddress", "192.0.2.20/24"}, {"sourceAddress", "0.0.0.0"}, {"sourceAddress", "::"}, {"sourceAddress", "224.0.0.1"}, {"sourceAddress", "255.255.255.255"},
		{"sourceInterfaceName", 7}, {"sourceInterfaceName", "bad\nname"},
	} {
		req := selectedSourceRequest("ping", "192.0.2.1")
		req.Options[tc.key] = tc.value
		if err := ValidateSourceOptions(req); err == nil {
			t.Errorf("accepted malformed selection %s=%#v", tc.key, tc.value)
		}
	}
	for _, key := range []string{"sourceInterfaceIndex", "sourceAddress", "sourceInterfaceName"} {
		if err := ValidateSourceOptions(model.Request{Tool: "ping", Options: map[string]any{key: nil}}); err == nil {
			t.Errorf("partial source option %s silently became Automatic", key)
		}
	}
	for _, tool := range []string{"discover", "identify", "ping", "tcp"} {
		req := selectedSourceRequest(tool, "192.0.2.0/30")
		req.Options["sourceAddress"] = "2001:db8::20"
		req.Options["scope"] = "subnet"
		if err := ValidateSourceOptions(req); err == nil || !strings.Contains(err.Error(), "IPv4") {
			t.Errorf("IPv6 source accepted for IPv4 scan %s: %v", tool, err)
		}
	}
	req := selectedSourceRequest("trace", "2001:db8::1")
	req.Options["sourceAddress"], req.Options["sourceInterfaceIndex"] = "2001:db8::20", float64(7)
	if err := ValidateSourceOptions(req); err != nil {
		t.Fatalf("JSON integer/IPv6 trace source rejected: %v", err)
	}
}

func TestDiagnosticSourcePreparationAndErrorMetadata(t *testing.T) {
	req := selectedSourceRequest("tcp", "192.0.2.1")
	calls := 0
	resolve := func(_ context.Context, index int, name, address string) (*platform.SourceBinding, error) {
		calls++
		if index != 7 || name != "Fixture Ethernet" || address != "192.0.2.20" {
			t.Fatalf("requested selection changed: %d %q %q", index, name, address)
		}
		return diagnosticSourceFixture(), nil
	}
	ctx, meta, err := prepareDiagnosticSource(context.Background(), req, resolve)
	binding, ok := platform.SourceBindingFromContext(ctx)
	if err != nil || calls != 1 || !ok || binding.Address != "192.0.2.20" || meta.Status != "validated" || meta.DNSServer != "192.0.2.53:53" || meta.NameResolution != "bound_dns" || meta.InterfaceID != "fixture-id" || sourceMetadata(ctx) != meta {
		t.Fatalf("prepared source or metadata: %+v %+v calls=%d err=%v", binding, meta, calls, err)
	}
	ctx, automatic, err := prepareDiagnosticSource(ctx, model.Request{Tool: "tcp"}, resolve)
	if _, selected := platform.SourceBindingFromContext(ctx); selected || calls != 1 || err != nil || automatic.Status != "automatic" || automatic.NameResolution != "system" {
		t.Fatalf("Automatic inherited a source or enumerated adapters: %+v %v", automatic, err)
	}
	var events []model.Event
	wantErr := errors.New("fixture source address is no longer assigned")
	out, err := runWithSource(context.Background(), req, func(e model.Event) { events = append(events, e) }, func(context.Context, int, string, string) (*platform.SourceBinding, error) { return nil, wantErr })
	if !errors.Is(err, wantErr) || len(events) != 1 {
		t.Fatalf("failed validation ran extra operations: %+v %v", events, err)
	}
	failed := out.Data.(map[string]any)["source"].(*SourceMetadata)
	data := events[0].Data.(map[string]any)
	if failed.Status != "invalid" || failed.Address != req.Options["sourceAddress"] || failed.InterfaceName != req.Options["sourceInterfaceName"] || failed.Error != wantErr.Error() || data["type"] != "source" || data["value"] != failed {
		t.Fatalf("requested source was lost on failure: %+v %+v", failed, events)
	}
	// Malformed source options must be rejected before adapter enumeration.
	req.Options["sourceAddress"] = "invalid"
	_, err = runWithSource(context.Background(), req, nil, func(context.Context, int, string, string) (*platform.SourceBinding, error) {
		t.Fatal("invalid shape reached adapter resolver")
		return nil, nil
	})
	if err == nil {
		t.Fatal("invalid source succeeded")
	}
	// A later target validation error still includes the successfully validated source.
	req = selectedSourceRequest("ping", "http://invalid-target")
	out, err = runWithSource(context.Background(), req, nil, resolve)
	if err == nil || out.Data.(map[string]any)["source"].(*SourceMetadata).Status != "validated" {
		t.Fatalf("source lost on diagnostic error: %+v %v", out, err)
	}
}

func TestDiagnosticSourceMetadataPreservesResultShapes(t *testing.T) {
	source := &SourceMetadata{Mode: "selected", Status: "validated", Address: "192.0.2.20"}
	for _, data := range []any{nil, map[string]any{"retained": true}, SubnetResult{Network: "192.0.2.0/24"}, IdentifyResult{Address: "192.0.2.1"}} {
		out := attachSource(model.Result{Summary: "retained summary", Data: data}, source)
		encoded, err := json.Marshal(out)
		if err != nil || !strings.Contains(string(encoded), `"source":{"mode":"selected"`) || out.Summary != "retained summary" {
			t.Fatalf("source missing from %T: %s %v", data, encoded, err)
		}
		switch data.(type) {
		case map[string]any:
			if _, mutated := data.(map[string]any)["source"]; mutated {
				t.Fatal("attaching final source metadata mutated an activity snapshot")
			}
		case SubnetResult:
			if out.Data.(SubnetResult).Network != "192.0.2.0/24" {
				t.Fatal("subnet shape changed")
			}
		case IdentifyResult:
			if out.Data.(IdentifyResult).Address != "192.0.2.1" {
				t.Fatal("identity shape changed")
			}
		}
	}
}

func TestDiagnosticSourceNoDNSAndWrongFamilyCannotFallBack(t *testing.T) {
	binding := diagnosticSourceFixture()
	binding.DNSServers = nil
	req := selectedSourceRequest("ping", "192.0.2.1")
	ctx, meta, err := prepareDiagnosticSource(context.Background(), req, func(context.Context, int, string, string) (*platform.SourceBinding, error) { return binding, nil })
	if err != nil || meta.Status != "validated" || meta.NameResolution != "unavailable" || !strings.Contains(meta.Note, "Literal IP targets can still") {
		t.Fatalf("no-DNS adapter should retain literal probes: %+v %v", meta, err)
	}
	if ips, err := resolveHost(ctx, "192.0.2.1", time.Second); err != nil || fmt.Sprint(ips) != "[192.0.2.1]" {
		t.Fatalf("literal target depended on DNS: %v %v", ips, err)
	}
	for _, target := range []string{"localhost", "source-fixture.invalid", "2001:db8::1", "0.0.0.0"} {
		if _, err := resolveHost(ctx, target, time.Second); err == nil {
			t.Errorf("incompatible/no-DNS target %q used a fallback", target)
		}
	}
	identity := resolveHostname(ctx, "192.0.2.1", systemReverseLookup)
	if identity.Hostname != "" || identity.HostnameStatus != "not_resolved" || !strings.Contains(identity.HostnameNote, "no compatible configured DNS") {
		t.Fatalf("missing selected-source reverse DNS was hidden: %+v", identity)
	}
	for _, server := range []string{"localhost:53", "2001:db8::53", "224.0.0.1", "255.255.255.255"} {
		if _, err := boundServerEndpoint(binding, server); err == nil {
			t.Errorf("invalid selected DNS server %q accepted", server)
		}
	}
	if _, err := dns(ctx, model.Request{Target: "source-fixture.invalid", Options: map[string]any{"server": "localhost"}}, nil); err == nil {
		t.Fatal("selected DNS used a hostname server bootstrap")
	}
	if _, err := dns(ctx, model.Request{Target: "source-fixture.invalid"}, nil); err == nil {
		t.Fatal("selected DNS with no configured server used system resolver")
	}
	if _, err := diagnosticDialContext(ctx, "tcp", "[2001:db8::1]:80", time.Second); err == nil {
		t.Fatal("wrong-family TCP target succeeded")
	}
}

func TestSelectedNeighborIdentityUsesOnlyMatchingInterface(t *testing.T) {
	rows := []platform.Neighbor{
		{Address: "192.0.2.1", InterfaceIndex: 8, MAC: "00:11:22:33:44:88", Vendor: "Other adapter"},
		{Address: "192.0.2.1", InterfaceIndex: 7, MAC: "00:11:22:33:44:77", Vendor: "Selected adapter"},
		{Address: "192.0.2.2", InterfaceIndex: 0, MAC: "00:11:22:33:44:66", Vendor: "Unknown adapter"},
		{Address: "::ffff:192.0.2.3", InterfaceIndex: 7, MAC: "00:11:22:33:44:33", Vendor: "Mapped IPv4"},
		{Address: "2001:db8::1", InterfaceIndex: 17, MAC: "00:11:22:33:44:55", Vendor: "Selected IPv6 index"},
		{Address: "2001:db8::1", InterfaceIndex: 7, MAC: "00:11:22:33:44:44", Vendor: "Wrong IPv6 index"},
	}
	read := func(context.Context) ([]platform.Neighbor, error) { return rows, nil }
	binding := diagnosticSourceFixture()
	ctx, cancel := context.WithCancel(platform.WithSourceBinding(context.Background(), binding))
	cancel()
	identities, err := refreshNeighborIdentities(ctx, read)
	if err != nil || len(identities) != 2 || identities["192.0.2.1"].Vendor != "Selected adapter" || identities["192.0.2.3"].Vendor != "Mapped IPv4" {
		t.Fatalf("cancelled-result refresh mixed interfaces: %+v %v", identities, err)
	}
	binding.Address, binding.Family = "2001:db8::20", "ipv6"
	identities, err = readNeighborIdentities(platform.WithSourceBinding(context.Background(), binding), read)
	if err != nil || len(identities) != 1 || identities["2001:db8::1"].Vendor != "Selected IPv6 index" {
		t.Fatalf("IPv6 neighbor index was not respected: %+v %v", identities, err)
	}
}

func TestDiagnosticSourceIPv6DNSScopeUsesSelectedFamilyIndex(t *testing.T) {
	binding := diagnosticSourceFixture()
	binding.Address, binding.Family = "fe80::20", "ipv6"
	binding.DNSServers = []string{"fe80::53%99", "fe80::53%Fixture Ethernet"}
	ctx := platform.WithSourceBinding(context.Background(), binding)
	server, err := selectedDNSServer(ctx)
	if err != nil || server != "[fe80::53%17]:53" {
		t.Fatalf("wrong DNS scope accepted or alias not normalized: %q %v", server, err)
	}
	if _, err := resolveHost(ctx, "fe80::1%99", time.Second); err == nil {
		t.Fatal("incompatible IPv6 target scope was accepted")
	}
	addresses, err := resolveHost(ctx, "fe80::1", time.Second)
	if err != nil || fmt.Sprint(addresses) != "[fe80::1%17]" {
		t.Fatalf("link-local target lost selected IPv6 scope: %v %v", addresses, err)
	}
}

func TestDiscoverySourceReachesEveryCombinedProbe(t *testing.T) {
	req := selectedSourceRequest("discover", "192.0.2.1")
	req.Options["ports"] = "80"
	ctx, meta, err := prepareDiagnosticSource(context.Background(), req, func(context.Context, int, string, string) (*platform.SourceBinding, error) {
		return diagnosticSourceFixture(), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	calls := map[string]int{}
	check := func(ctx context.Context, name string) {
		binding, selected := platform.SourceBindingFromContext(ctx)
		if !selected || binding.InterfaceIndex != 7 || binding.Address != "192.0.2.20" {
			t.Errorf("%s lost selected source: %+v", name, binding)
		}
		mu.Lock()
		calls[name]++
		mu.Unlock()
	}
	probes := fakeSubnetProbes()
	probes.ping = func(ctx context.Context, target string, _ time.Duration, _ int) (platform.PingReply, error) {
		check(ctx, "icmp")
		return platform.PingReply{Address: target, Status: "reply", Success: true}, nil
	}
	probes.tcp = func(ctx context.Context, target string, port int, _ time.Duration) PortResult {
		check(ctx, "tcp")
		return PortResult{Address: net.JoinHostPort(target, fmt.Sprint(port)), Port: port, State: "open"}
	}
	probes.system = func(ctx context.Context) (map[string]any, error) { check(ctx, "system"); return map[string]any{}, nil }
	probes.neighbors = func(ctx context.Context) ([]platform.Neighbor, error) {
		check(ctx, "neighbors")
		return []platform.Neighbor{{Address: "192.0.2.1", InterfaceIndex: 7, MAC: "00:11:22:33:44:55", Vendor: "Fixture"}}, nil
	}
	probes.device = func(ctx context.Context, _ string, _ time.Duration) (deviceinfo.Info, error) {
		check(ctx, "upnp")
		return deviceinfo.Info{Responded: true}, nil
	}
	probes.hostname = func(ctx context.Context, _ string) ([]string, error) {
		check(ctx, "ptr")
		return []string{"fixture.example."}, nil
	}
	out, err := runSubnetScan(ctx, req, nil, probes)
	data := out.Data.(SubnetResult)
	if err != nil || data.Source != meta || len(data.Hosts) != 1 || data.Hosts[0].Hostname != "fixture.example" || data.Hosts[0].Vendor != "Fixture" {
		t.Fatalf("combined result/source: %+v %v", data, err)
	}
	for name, count := range map[string]int{"icmp": 1, "tcp": 1, "system": 1, "neighbors": 2, "upnp": 1, "ptr": 1} {
		if calls[name] != count {
			t.Errorf("%s calls %d, want %d", name, calls[name], count)
		}
	}
	// Cancelling between sequential combined probes retains source and response evidence.
	req.Options["concurrency"] = 1
	ctx, cancel := context.WithCancel(ctx)
	probes.ping = func(ctx context.Context, target string, _ time.Duration, _ int) (platform.PingReply, error) {
		check(ctx, "cancelled-icmp")
		cancel()
		return platform.PingReply{Address: target, Status: "reply", Success: true}, nil
	}
	out, err = runSubnetScan(ctx, req, nil, probes)
	data = out.Data.(SubnetResult)
	if !errors.Is(err, context.Canceled) || data.Source != meta || len(data.Hosts) != 1 || !data.Hosts[0].Responded || data.Hosts[0].Completed {
		t.Fatalf("partial source/evidence lost: %+v %v", data, err)
	}
}
