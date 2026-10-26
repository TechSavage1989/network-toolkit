package diag

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"networktoolkit/internal/deviceinfo"
	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

func TestHostnameResolutionNormalizesAndBoundsNames(t *testing.T) {
	input := []string{"Zebra.fixture.test.", "alpha.fixture.test.", "ALPHA.fixture.test", "printer_1.", "", "192.0.2.1", "http://wrong.test/", "<script>", "bad..name", "bad..", "bad name", strings.Repeat("a", 64) + ".test", "a\x00b.test"}
	calls := 0
	got := resolveHostname(context.Background(), "::ffff:192.0.2.1", func(ctx context.Context, target string) ([]string, error) {
		calls++
		deadline, ok := ctx.Deadline()
		if target != "192.0.2.1" || !ok || time.Until(deadline) <= 0 || time.Until(deadline) > hostnameLookupTimeout {
			t.Errorf("unbounded or wrong lookup: %q %v", target, deadline)
		}
		return input, nil
	})
	if calls != 1 || got.Hostname != "alpha.fixture.test" || got.HostnameStatus != "resolved" || got.HostnameSource != "Configured reverse lookup" || !slices.Equal(got.Hostnames, []string{"alpha.fixture.test", "printer_1", "Zebra.fixture.test"}) {
		t.Fatalf("unexpected names: %+v", got)
	}
	input[1] = "changed.test"
	if got.Hostnames[0] != "alpha.fixture.test" {
		t.Fatal("resolver output slice was retained")
	}
	got = resolveHostname(context.Background(), "192.0.2.1", func(context.Context, string) ([]string, error) {
		values := make([]string, 100)
		for i := range values {
			values[i] = fmt.Sprintf("host%02d.test.", i)
		}
		return values, nil
	})
	if len(got.Hostnames) != 16 {
		t.Fatalf("unbounded aliases: %d", len(got.Hostnames))
	}
}

func TestHostnameResolutionUnresolvedTimeoutAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"missing", &net.DNSError{IsNotFound: true, Err: "fixture no record"}, "not_resolved"},
		{"deadline", context.DeadlineExceeded, "timeout"},
		{"DNS timeout", &net.DNSError{IsTimeout: true, Err: "fixture timeout"}, "timeout"},
		{"cancelled", context.Canceled, "cancelled"},
		{"other failure", errors.New("fixture raw resolver output must not be copied"), "not_resolved"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveHostname(context.Background(), "192.0.2.1", func(context.Context, string) ([]string, error) { return nil, tc.err })
			if got.HostnameStatus != tc.want || got.Hostname != "" || got.HostnameSource != "" || strings.Contains(got.HostnameNote, "fixture") {
				t.Fatalf("unexpected failure result: %+v", got)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	noCall := func(context.Context, string) ([]string, error) {
		t.Fatal("lookup started when prohibited")
		return nil, nil
	}
	if got := resolveHostname(ctx, "192.0.2.1", noCall); got.HostnameStatus != "cancelled" {
		t.Fatalf("pre-cancelled lookup: %+v", got)
	}
	for _, address := range []string{"not-an-address", "0.0.0.0", "255.255.255.255", "224.0.0.1", "::"} {
		if got := resolveHostname(context.Background(), address, noCall); got.Hostname != "" {
			t.Fatalf("invalid target resolved: %+v", got)
		}
	}
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()
	start := time.Now()
	got := resolveHostname(ctx, "192.0.2.1", func(ctx context.Context, _ string) ([]string, error) {
		<-ctx.Done()
		return []string{"late.test"}, ctx.Err()
	})
	if got.HostnameStatus != "cancelled" || got.Hostname != "" || time.Since(start) > time.Second {
		t.Fatalf("parent cancellation did not win: %+v", got)
	}
}

func TestDiscoveryHostnameDefaultsProgressAndNoResponseEvidence(t *testing.T) {
	probes := fakeSubnetProbes()
	var lookedUp []string
	probes.hostname = func(_ context.Context, address string) ([]string, error) {
		lookedUp = append(lookedUp, address)
		if strings.HasSuffix(address, ".1") {
			return []string{"printer.fixture.test."}, nil
		}
		return nil, &net.DNSError{IsNotFound: true}
	}
	probes.neighbors = func(context.Context) ([]platform.Neighbor, error) {
		return []platform.Neighbor{{Address: "192.0.2.1", MAC: "00:11:22:33:44:55", Vendor: "Fixture vendor"}}, nil
	}
	var hostJSON []json.RawMessage
	emit := func(e model.Event) {
		data, err := json.Marshal(e.Data)
		if err != nil {
			t.Fatal(err)
		}
		var envelope struct {
			Type  string          `json:"type"`
			Value json.RawMessage `json:"value"`
		}
		if err = json.Unmarshal(data, &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Type == "host" {
			hostJSON = append(hostJSON, envelope.Value)
		}
	}
	out, err := runSubnetScan(context.Background(), model.Request{Tool: "discover", Target: "192.0.2.0/30", Options: map[string]any{"ports": "443", "identify": false, "concurrency": 1}}, emit, probes)
	if err != nil {
		t.Fatal(err)
	}
	data := out.Data.(SubnetResult)
	if !data.ResolveHostnames || data.RequestedProbes != 6 || data.CompletedProbes != 6 || data.Completed != 2 || data.Responded != 0 || !slices.Equal(lookedUp, []string{"192.0.2.1", "192.0.2.2"}) {
		t.Fatalf("hostname coverage/progress: %+v %v", data, lookedUp)
	}
	if data.Hosts[0].Hostname != "printer.fixture.test" || data.Hosts[0].Vendor != "Fixture vendor" || data.Hosts[1].HostnameStatus != "not_resolved" {
		t.Fatalf("hostname or refreshed neighbor lost: %+v", data.Hosts)
	}
	for _, host := range data.Hosts {
		if host.Responded || len(host.Evidence) != 0 || host.Status != "no_response" {
			t.Fatalf("resolver name became response evidence: %+v", host)
		}
	}
	if len(hostJSON) != 2 || !strings.Contains(string(hostJSON[0]), `"hostname":"printer.fixture.test"`) || !strings.Contains(string(hostJSON[0]), `"hostnameSource":"Configured reverse lookup"`) {
		t.Fatalf("live hostname serialization missing: %s", hostJSON)
	}
}

func TestDiscoveryHostnameDisableAndSharedWorkerLimit(t *testing.T) {
	probes := fakeSubnetProbes()
	probes.hostname = func(context.Context, string) ([]string, error) { t.Fatal("disabled name lookup ran"); return nil, nil }
	out, err := runSubnetScan(context.Background(), model.Request{Tool: "discover", Target: "192.0.2.1", Options: map[string]any{"ports": "443", "identify": false, "resolveHostnames": false}}, nil, probes)
	if err != nil {
		t.Fatal(err)
	}
	data := out.Data.(SubnetResult)
	if data.ResolveHostnames || data.RequestedProbes != 2 || data.Hosts[0].HostnameStatus != "disabled" {
		t.Fatalf("disable failed: %+v", data)
	}
	var calls, active, maximum atomic.Int32
	probes.hostname = func(context.Context, string) ([]string, error) {
		calls.Add(1)
		n := active.Add(1)
		for {
			old := maximum.Load()
			if n <= old || maximum.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
		active.Add(-1)
		return nil, nil
	}
	out, err = runSubnetScan(context.Background(), model.Request{Tool: "discover", Target: "192.0.2.0/28", Options: map[string]any{"ports": "443", "identify": false, "concurrency": 4}}, nil, probes)
	if err != nil {
		t.Fatal(err)
	}
	data = out.Data.(SubnetResult)
	if calls.Load() != 14 || maximum.Load() < 2 || maximum.Load() > 4 || active.Load() != 0 || data.CompletedProbes != 42 || data.RequestedProbes != 42 {
		t.Fatalf("lookup worker bound/count: %d/%d %+v", calls.Load(), maximum.Load(), data)
	}
}

func TestDiscoveryHostnameCancellationRetainsProbeEvidence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	probes := fakeSubnetProbes()
	probes.ping = func(context.Context, string, time.Duration, int) (platform.PingReply, error) {
		return platform.PingReply{Success: true, Status: "reply", RTTMs: 2}, nil
	}
	probes.hostname = func(context.Context, string) ([]string, error) {
		cancel()
		return []string{"late.test"}, context.Canceled
	}
	out, err := runSubnetScan(ctx, model.Request{Tool: "discover", Target: "192.0.2.1", Options: map[string]any{"concurrency": 1, "ports": "443", "identify": false}}, nil, probes)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	data := out.Data.(SubnetResult)
	if len(data.Hosts) != 1 || !data.Cancelled || data.Completed != 0 || data.Responded != 1 || data.Hosts[0].Hostname != "" || data.Hosts[0].HostnameStatus != "cancelled" || data.Hosts[0].Status != "cancelled" || len(data.Hosts[0].Evidence) != 1 {
		t.Fatalf("cancelled hostname lost probe evidence: %+v", data)
	}
}

func TestDeviceDetailsAndCheckKeepHostnameIndependent(t *testing.T) {
	neighbors := func(context.Context) ([]platform.Neighbor, error) {
		return []platform.Neighbor{{Address: "192.0.2.1", MAC: "00:11:22:33:44:55", Vendor: "Fixture vendor"}}, nil
	}
	probe := func(context.Context, string, time.Duration) (deviceinfo.Info, error) {
		return deviceinfo.Info{}, deviceinfo.ErrNoResponse
	}
	lookup := func(context.Context, string) ([]string, error) { return []string{"switch.fixture.test."}, nil }
	out, err := runIdentify(context.Background(), model.Request{Target: "192.0.2.1"}, nil, neighbors, probe, lookup)
	if err != nil {
		t.Fatal(err)
	}
	data := out.Data.(IdentifyResult)
	if data.Hostname != "switch.fixture.test" || data.Model != "" || data.FriendlyName != "" || data.Responded || data.Status != "no_response" || data.Vendor != "Fixture vendor" {
		t.Fatalf("hostname was conflated with identity/reachability: %+v", data)
	}
	out, err = runIdentify(context.Background(), model.Request{Target: "192.0.2.1", Options: map[string]any{"resolveHostnames": false}}, nil, neighbors, probe, func(context.Context, string) ([]string, error) {
		t.Fatal("disabled device name lookup ran")
		return nil, nil
	})
	if err != nil || out.Data.(IdentifyResult).HostnameStatus != "disabled" {
		t.Fatalf("device disable failed: %+v %v", out, err)
	}
	check := makeHostnameCheck(context.Background(), "192.0.2.1", lookup)
	name := check.Data.(hostnameCheckData)
	if check.Name != "Hostname" || check.Status != "info" || name.Address != "192.0.2.1" || name.Hostname != "switch.fixture.test" {
		t.Fatalf("device check omitted name/IP: %+v", check)
	}
}
