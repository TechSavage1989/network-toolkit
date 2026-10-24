package platform

import (
	"context"
	"errors"
	"net"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func sourceFixture() []sourceInterface {
	return sourceInterfacesFromRows([]map[string]any{
		{"index": 7, "ipv6Index": 17, "name": "Ethernet 5", "description": "Fixture Ethernet adapter", "id": "{fixture-adapter}", "up": true,
			"addresses": []string{"192.0.2.20/24", "2001:db8::20/64", "fe80::20%17/64"}, "dns": []string{"192.0.2.53", "fe80::53%17"},
			"addressDetails": []map[string]any{{"address": "192.0.2.20", "state": "preferred"}, {"address": "fe80::20%17", "state": "preferred"}}},
		{"index": 8, "ipv6Index": 18, "name": "Wi-Fi", "up": false, "addresses": []string{"198.51.100.20/24", "fe80::20%18/64"}},
	})
}

func fixtureSourceRead(context.Context) ([]sourceInterface, error) { return sourceFixture(), nil }

func TestSourceBindingValidationAndMetadata(t *testing.T) {
	ctx := context.Background()
	readCalled := false
	automatic, err := resolveSourceBinding(ctx, 0, "", "", func(context.Context) ([]sourceInterface, error) { readCalled = true; return nil, nil })
	if err != nil || automatic != nil || readCalled {
		t.Fatalf("Automatic should not enumerate or bind adapters: %v %v %v", automatic, err, readCalled)
	}
	for _, test := range []struct{ address, canonical, family string }{
		{"192.0.2.20", "192.0.2.20", "ipv4"},
		{"::ffff:192.0.2.20", "192.0.2.20", "ipv4"},
		{"2001:0db8::20", "2001:db8::20", "ipv6"},
		{"fe80::20%17", "fe80::20", "ipv6"},
		{"fe80::20%Ethernet 5", "fe80::20", "ipv6"},
	} {
		binding, err := resolveSourceBinding(ctx, 7, "Ethernet 5", test.address, fixtureSourceRead)
		if err != nil {
			t.Fatalf("%s: %v", test.address, err)
		}
		if binding.Address != test.canonical || binding.Family != test.family || binding.InterfaceIndex != 7 || binding.IPv6InterfaceIndex != 17 || binding.InterfaceDescription != "Fixture Ethernet adapter" || binding.InterfaceID != "{fixture-adapter}" || !reflect.DeepEqual(binding.DNSServers, []string{"192.0.2.53", "fe80::53%17"}) {
			t.Fatalf("metadata or normalization lost: %+v", binding)
		}
		wantIndex := 7
		if test.family == "ipv6" {
			wantIndex = 17
		}
		if binding.socketIndex() != wantIndex {
			t.Fatalf("%s uses interface index %d, want %d", test.family, binding.socketIndex(), wantIndex)
		}
	}
	for _, test := range []struct {
		index                  int
		name, address, message string
	}{
		{0, "", "192.0.2.20", "choose a current"}, {-1, "", "192.0.2.20", "choose a current"}, {7, "", "", "choose a current"},
		{7, "", "fixture.example", "literal"}, {7, "", "192.0.2.20/24", "literal"}, {7, "", "0.0.0.0", "literal"}, {7, "", "224.0.0.1", "literal"}, {7, "", "255.255.255.255", "literal"},
		{7, "Renamed adapter", "192.0.2.20", "has changed"}, {70, "Ethernet 5", "192.0.2.20", "no longer available"},
		{8, "Wi-Fi", "198.51.100.20", "not up"}, {7, "", "192.0.2.21", "no longer assigned"}, {7, "", "fe80::20%18", "scope"},
	} {
		if _, err := resolveSourceBinding(ctx, test.index, test.name, test.address, fixtureSourceRead); err == nil || !strings.Contains(err.Error(), test.message) {
			t.Errorf("%+v returned %v", test, err)
		}
	}
}

func TestSourceBindingRejectsUnavailableOrAmbiguousAddresses(t *testing.T) {
	for _, state := range []string{"invalid", "tentative", "duplicate"} {
		read := func(context.Context) ([]sourceInterface, error) {
			rows := sourceFixture()
			rows[0].addressStates["192.0.2.20"] = state
			return rows, nil
		}
		if _, err := resolveSourceBinding(context.Background(), 7, "", "192.0.2.20", read); err == nil || !strings.Contains(err.Error(), state) {
			t.Fatalf("accepted %s address: %v", state, err)
		}
	}
	read := func(context.Context) ([]sourceInterface, error) {
		rows := sourceFixture()
		rows[1].up = true
		rows[1].addresses = append(rows[1].addresses, "192.0.2.20/24")
		return rows, nil
	}
	if _, err := resolveSourceBinding(context.Background(), 7, "", "192.0.2.20", read); err == nil || !strings.Contains(err.Error(), "more than one") {
		t.Fatalf("ambiguous ICMP source was accepted: %v", err)
	}
	// Equal link-local addresses on different links remain unambiguous with scope.
	if _, err := resolveSourceBinding(context.Background(), 7, "", "fe80::20%17", read); err != nil {
		t.Fatalf("scoped link-local source incorrectly rejected: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := resolveSourceBinding(ctx, 7, "", "192.0.2.20", fixtureSourceRead); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled validation: %v", err)
	}
	want := errors.New("fixture inventory failure")
	if _, err := resolveSourceBinding(context.Background(), 7, "", "192.0.2.20", func(context.Context) ([]sourceInterface, error) { return nil, want }); !errors.Is(err, want) {
		t.Fatalf("adapter errors must not fall back: %v", err)
	}
}

func TestSourceBindingContextCopiesMutableMetadata(t *testing.T) {
	binding, _ := resolveSourceBinding(context.Background(), 7, "", "192.0.2.20", fixtureSourceRead)
	ctx := WithSourceBinding(context.Background(), binding)
	binding.Address, binding.DNSServers[0] = "198.51.100.99", "198.51.100.53"
	first, ok := SourceBindingFromContext(ctx)
	if !ok || first.Address != "192.0.2.20" || first.DNSServers[0] != "192.0.2.53" {
		t.Fatalf("context binding mutated: %+v", first)
	}
	first.DNSServers[0] = "203.0.113.53"
	second, _ := SourceBindingFromContext(ctx)
	if second.DNSServers[0] != "192.0.2.53" {
		t.Fatal("retrieved binding aliases context metadata")
	}
	if _, ok := SourceBindingFromContext(WithSourceBinding(ctx, nil)); ok {
		t.Fatal("Automatic did not clear inherited binding")
	}
}

func TestSourceBindingTargetsNeverResolveOrChangeFamily(t *testing.T) {
	v4, _ := resolveSourceBinding(context.Background(), 7, "", "192.0.2.20", fixtureSourceRead)
	v6, _ := resolveSourceBinding(context.Background(), 7, "", "fe80::20", fixtureSourceRead)
	for _, test := range []struct {
		binding      *SourceBinding
		target, want string
	}{
		{v4, "192.0.2.10", "192.0.2.10"}, {v4, "::ffff:192.0.2.10", "192.0.2.10"},
		{v6, "fe80::10", "fe80::10%17"}, {v6, "[fe80::10%Ethernet 5]", "fe80::10%17"}, {v6, "2001:db8::10", "2001:db8::10"},
	} {
		address, err := resolvePingTarget(WithSourceBinding(context.Background(), test.binding), test.target)
		if err != nil || address.String() != test.want {
			t.Fatalf("%s => %v %v, want %s", test.target, address, err, test.want)
		}
	}
	for _, test := range []struct {
		binding *SourceBinding
		target  string
	}{{v4, "does-not-resolve.example"}, {v4, "2001:db8::10"}, {v6, "192.0.2.10"}, {v6, "fe80::10%7"}, {v6, "fe80::10%Other"}} {
		if _, err := resolvePingTarget(WithSourceBinding(context.Background(), test.binding), test.target); err == nil {
			t.Fatalf("accepted mismatched or unresolved target %s", test.target)
		}
	}
}

func TestSourceBindingPingRejectsInvalidTargetsBeforeProbing(t *testing.T) {
	binding, _ := resolveSourceBinding(context.Background(), 7, "", "192.0.2.20", fixtureSourceRead)
	ctx := WithSourceBinding(context.Background(), binding)
	for _, target := range []string{"fixture.example", "2001:db8::10"} {
		if _, err := Ping(ctx, target, time.Second, 64); err == nil {
			t.Fatalf("bound Ping accepted unresolved or wrong-family target %s", target)
		}
	}
}

type sourceRejectedRawConn struct {
	err   error
	calls int
}

func (raw *sourceRejectedRawConn) Control(func(uintptr)) error    { raw.calls++; return raw.err }
func (raw *sourceRejectedRawConn) Read(func(uintptr) bool) error  { panic("unexpected raw read") }
func (raw *sourceRejectedRawConn) Write(func(uintptr) bool) error { panic("unexpected raw write") }

func TestSourceBindingDialPlanAndSocketFailure(t *testing.T) {
	v4, _ := resolveSourceBinding(context.Background(), 7, "", "192.0.2.20", fixtureSourceRead)
	v6, _ := resolveSourceBinding(context.Background(), 7, "", "fe80::20", fixtureSourceRead)
	for _, test := range []struct {
		binding                                                 *SourceBinding
		network, endpoint, wantNetwork, wantEndpoint, wantLocal string
	}{
		{v4, "tcp", "192.0.2.10:22", "tcp4", "192.0.2.10:22", "192.0.2.20:0"},
		{v4, "udp4", "192.0.2.53:53", "udp4", "192.0.2.53:53", "192.0.2.20:0"},
		{v6, "udp", "[fe80::53]:53", "udp6", "[fe80::53%17]:53", "[fe80::20%17]:0"},
		{v6, "tcp6", "[2001:db8::10]:443", "tcp6", "[2001:db8::10]:443", "[fe80::20%17]:0"},
	} {
		dialer, network, endpoint, err := sourceDialPlan(*test.binding, test.network, test.endpoint, 3*time.Second)
		if err != nil || network != test.wantNetwork || endpoint != test.wantEndpoint || dialer.LocalAddr.String() != test.wantLocal || dialer.Timeout != 3*time.Second {
			t.Fatalf("dial plan %v => %+v %s %s %v", test, dialer, network, endpoint, err)
		}
		if strings.HasPrefix(network, "tcp") {
			if _, ok := dialer.LocalAddr.(*net.TCPAddr); !ok {
				t.Fatal("TCP source has wrong address type")
			}
		} else {
			if _, ok := dialer.LocalAddr.(*net.UDPAddr); !ok {
				t.Fatal("UDP source has wrong address type")
			}
		}
		want := errors.New("fixture socket rejected")
		raw := &sourceRejectedRawConn{err: want}
		err = dialer.ControlContext(context.Background(), network, endpoint, raw)
		if runtime.GOOS == "windows" || runtime.GOOS == "linux" {
			if !errors.Is(err, want) || raw.calls != 1 {
				t.Fatalf("binding failure lost or retried: %v (%d calls)", err, raw.calls)
			}
		} else if err == nil || raw.calls != 0 {
			t.Fatal("unsupported source-interface platform did not fail before socket control")
		}
		before := raw.calls
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := dialer.ControlContext(ctx, network, endpoint, raw); !errors.Is(err, context.Canceled) || raw.calls != before {
			t.Fatal("cancelled socket control was attempted")
		}
	}
	for _, test := range []struct{ network, endpoint string }{{"tcp6", "[2001:db8::1]:22"}, {"tcp", "fixture.example:22"}, {"unix", "/tmp/socket"}, {"tcp", "192.0.2.1"}} {
		if _, _, _, err := sourceDialPlan(*v4, test.network, test.endpoint, time.Second); err == nil {
			t.Fatalf("accepted source-bound plan: %+v", test)
		}
	}
	// These fail before socket creation, proving selected names do not invoke DNS.
	if _, err := DialContext(WithSourceBinding(context.Background(), v4), "tcp", "fixture.example:22", time.Second); err == nil || !strings.Contains(err.Error(), "literal") {
		t.Fatalf("implicit DNS was allowed: %v", err)
	}
	ctx, cancel := context.WithCancel(WithSourceBinding(context.Background(), v4))
	cancel()
	if _, err := DialContext(ctx, "tcp", "192.0.2.1:22", time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled dial: %v", err)
	}
}
