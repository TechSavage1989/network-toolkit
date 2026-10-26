package diag

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

func TestDeviceCheckRejectsSubnetScope(t *testing.T) {
	for _, target := range []string{"127.0.0.1", "127.0.0.1/32", "not a valid host"} {
		t.Run(target, func(t *testing.T) {
			request := model.Request{Tool: "quick", Target: target, Options: map[string]any{"scope": "subnet"}}
			var emitted []model.Event
			out, err := Run(context.Background(), request, func(e model.Event) { emitted = append(emitted, e) })
			data, ok := out.Data.(map[string]any)
			// Source selection is recorded on failed jobs too. No diagnostic
			// operation or event may run for the obsolete scope request.
			if !errors.Is(err, errDeviceCheckSubnet) || !strings.Contains(err.Error(), "use Network discovery") || !ok || len(data) != 1 || len(emitted) != 1 {
				t.Fatalf("Device check did not reject obsolete subnet request before work: %+v, %v, %d events", out, err, len(emitted))
			}
			if source := data["source"].(*SourceMetadata); source.Mode != "automatic" || emitted[0].Data.(map[string]any)["type"] != "source" {
				t.Fatalf("unexpected work before scope validation: %+v", emitted)
			}
			// The shared engine also rejects old internal callers before any
			// injected cache read or probe, rather than silently treating Quick
			// as Discovery with different defaults.
			if _, err := runSubnetScan(context.Background(), request, nil, subnetProbes{}); !errors.Is(err, errDeviceCheckSubnet) {
				t.Fatalf("shared engine accepted obsolete Quick scope: %v", err)
			}
		})
	}
}

func TestDiscoveryDefaultsAndExpandedLimits(t *testing.T) {
	options, err := parseSubnetOptions(model.Request{Tool: "discover", Target: "192.0.2.17/30"})
	if err != nil {
		t.Fatal(err)
	}
	if options.network != "192.0.2.16/30" || len(options.targets) != 2 || options.targets[0].String() != "192.0.2.17" || options.targets[1].String() != "192.0.2.18" || !options.icmp || !options.identify || !options.resolveHostnames || options.timeout != 400*time.Millisecond || options.concurrency != 16 || !slices.Equal(options.ports, []int{22, 80, 443, 445, 3389}) {
		t.Fatalf("Discovery defaults or CIDR normalization changed: %+v", options)
	}
	for _, request := range []model.Request{
		{Tool: "discover", Target: "192.0.2.0/24", Options: map[string]any{"ports": "1-64", "timeoutMs": 10000, "concurrency": 64, "identify": false}},
		{Tool: "discover", Target: "192.0.2.0/23", Options: map[string]any{"ports": "1-32"}},
		{Tool: "discover", Target: "192.0.0.0/22", Options: map[string]any{"ports": "1-16"}},
		{Tool: "discover", Target: "192.0.2.0/31", Options: map[string]any{"timeoutMs": 100, "concurrency": 1}},
		{Tool: "discover", Target: "192.0.2.1/32"},
	} {
		if _, err := parseSubnetOptions(request); err != nil {
			t.Errorf("rejected bounded Discovery request %+v: %v", request, err)
		}
	}
	for _, request := range []model.Request{
		{Tool: "discover", Target: "192.0.0.0/21"},
		{Tool: "discover", Target: "192.0.2.1", Options: map[string]any{"ports": "1-65"}},
		{Tool: "discover", Target: "192.0.2.0/23", Options: map[string]any{"ports": "1-33"}},
		{Tool: "discover", Target: "192.0.0.0/22", Options: map[string]any{"ports": "1-17"}},
		{Tool: "discover", Target: "192.0.2.1", Options: map[string]any{"timeoutMs": 99}},
		{Tool: "discover", Target: "192.0.2.1", Options: map[string]any{"timeoutMs": 10001}},
		{Tool: "discover", Target: "192.0.2.1", Options: map[string]any{"concurrency": 0}},
		{Tool: "discover", Target: "192.0.2.1", Options: map[string]any{"concurrency": 65}},
		{Tool: "discover", Target: "device.example"},
		{Tool: "discover", Target: "2001:db8::/64"},
	} {
		// Nil operations make validation-before-work part of the regression.
		if _, err := runSubnetScan(context.Background(), request, nil, subnetProbes{}); err == nil {
			t.Errorf("accepted unbounded or unsupported Discovery request %+v", request)
		}
	}
}

func TestDiscoveryRetainsAllHostsWithRichLiveResults(t *testing.T) {
	probes := fakeSubnetProbes()
	probes.device = nil // This run explicitly disables additional device traffic.
	probes.ping = func(_ context.Context, address string, _ time.Duration, _ int) (platform.PingReply, error) {
		switch address {
		case "192.0.2.1":
			return platform.PingReply{Address: address, Success: true, Status: "reply", RTTMs: 3.5, TTL: 64}, nil
		case "192.0.2.4":
			return platform.PingReply{}, errors.New("fixture ICMP setup error")
		default:
			return platform.PingReply{Status: "timeout"}, nil
		}
	}
	probes.tcp = func(_ context.Context, address string, port int, _ time.Duration) PortResult {
		result := PortResult{Port: port, Address: fmt.Sprintf("%s:%d", address, port), State: "timeout"}
		if address == "192.0.2.2" && port == 80 {
			result.State, result.LatencyMs = "open", 2
		}
		if address == "192.0.2.2" && port == 443 {
			result.State, result.LatencyMs = "closed", 1
		}
		if address == "192.0.2.5" && port == 80 {
			result.State, result.Error = "error", "fixture dial setup error"
		}
		return result
	}
	var progressEvents []discoveryProgress
	var hostEvents []SubnetHost
	emit := func(e model.Event) {
		// Inspect the actual JSON envelope used by browser/report consumers.
		raw, err := json.Marshal(e.Data)
		if err != nil {
			t.Fatal(err)
		}
		var envelope struct {
			Type  string          `json:"type"`
			Value json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			t.Fatal(err)
		}
		switch envelope.Type {
		case "progress":
			var progress discoveryProgress
			if err := json.Unmarshal(envelope.Value, &progress); err != nil {
				t.Fatal(err)
			}
			if progress.Scope != "subnet" || progress.Total != 6 || progress.RequestedProbes != 24 {
				t.Errorf("live format marker/counts missing: %+v", progress)
			}
			progressEvents = append(progressEvents, progress)
		case "host":
			if len(progressEvents) == 0 || progressEvents[0].Started != 0 {
				t.Error("host emitted before initial all-host progress marker")
			}
			var host SubnetHost
			if err := json.Unmarshal(envelope.Value, &host); err != nil {
				t.Fatal(err)
			}
			hostEvents = append(hostEvents, host)
		case "subnet-host", "subnet-progress":
			t.Errorf("Discovery changed its established event name to %s", envelope.Type)
		}
	}
	out, err := runSubnetScan(context.Background(), model.Request{Tool: "discover", Target: "192.0.2.0/29", Options: map[string]any{"ports": "80,443", "identify": false}}, emit, probes)
	if err != nil {
		t.Fatal(err)
	}
	data := out.Data.(SubnetResult)
	if data.Scope != "subnet" || data.Tool != "discover" || len(data.Hosts) != 6 || data.Started != 6 || data.Completed != 6 || data.Responded != 2 || data.NotStarted != 0 || data.CompletedProbes != 24 || data.RequestedProbes != 24 || data.LocalContext["fixture"] != true {
		t.Fatalf("silent/error hosts or completed work were lost: %+v", data)
	}
	if len(hostEvents) < 6 || len(hostEvents) > 12 || len(progressEvents) < 2 || progressEvents[len(progressEvents)-1].SubnetProgress != data.SubnetProgress {
		t.Fatalf("live coverage/final progress mismatch: %d host events, %+v progress", len(hostEvents), progressEvents)
	}
	expected := []string{"responded", "responded", "no_response", "error", "error", "no_response"}
	for i, host := range data.Hosts {
		if host.Status != expected[i] || !host.Completed || len(host.Samples) != 1 || host.Samples[0].Sequence != 1 || host.Statistics == nil || host.Statistics.Sent != 1 || len(host.Ports) != 2 {
			t.Errorf("rich result mismatch for %s: %+v", host.Address, host)
		}
		if host.Responded != (i < 2) {
			t.Errorf("fabricated/missing response evidence for %s", host.Address)
		}
		if i >= 2 && (host.LatencyMs != nil || host.Statistics.AvgMs != nil || len(host.Evidence) != 0) {
			t.Errorf("unsuccessful probes acquired a response latency/evidence: %+v", host)
		}
	}
	if data.Hosts[0].Statistics.Received != 1 || *data.Hosts[0].Statistics.AvgMs != 3.5 || data.Hosts[0].Samples[0].TTL != 64 || !slices.Equal(data.Hosts[1].OpenPorts, []int{80}) || !slices.Contains(data.Hosts[1].Evidence, "TCP 443 refused") {
		t.Fatalf("ICMP details or open/refused distinction lost: %+v", data.Hosts[:2])
	}
}

func TestDiscoveryCancellationRetainsSilentAndPartialHosts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	probes := fakeSubnetProbes()
	probes.device = nil
	probes.ping = func(_ context.Context, address string, _ time.Duration, _ int) (platform.PingReply, error) {
		if address == "192.0.2.2" {
			cancel()
			return platform.PingReply{}, context.Canceled
		}
		return platform.PingReply{Status: "timeout"}, nil
	}
	var tcpCalls int
	probes.tcp = func(_ context.Context, address string, port int, _ time.Duration) PortResult {
		tcpCalls++
		if address != "192.0.2.1" {
			t.Errorf("started another TCP probe after cancellation: %s", address)
		}
		return PortResult{Port: port, State: "timeout"}
	}
	out, err := runSubnetScan(ctx, model.Request{Tool: "discover", Target: "192.0.2.0/29", Options: map[string]any{"ports": "80,443", "identify": false, "concurrency": 1}}, nil, probes)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	data := out.Data.(SubnetResult)
	if !data.Cancelled || data.Started != 2 || data.Completed != 1 || data.Responded != 0 || data.NotStarted != 4 || data.CompletedProbes != 5 || data.RequestedProbes != 24 || len(data.Hosts) != 2 || tcpCalls != 2 {
		t.Fatalf("cancelled Discovery lost silent work or started extra probes: %+v; TCP calls %d", data, tcpCalls)
	}
	if data.Hosts[0].Status != "no_response" || !data.Hosts[0].Completed || len(data.Hosts[0].Ports) != 2 || data.Hosts[1].Status != "cancelled" || data.Hosts[1].Completed || len(data.Hosts[1].Samples) != 1 || len(data.Hosts[1].Ports) != 0 {
		t.Fatalf("silent/partial rows changed on cancellation: %+v", data.Hosts)
	}
}

func TestDiscoveryLocalContextFailureDoesNotSuppressHosts(t *testing.T) {
	probes := fakeSubnetProbes()
	var calls int
	probes.system = func(context.Context) (map[string]any, error) {
		calls++
		return map[string]any{"partial": true}, errors.New("fixture partial local context")
	}
	out, err := runSubnetScan(context.Background(), model.Request{Tool: "discover", Target: "192.0.2.0/31", Options: map[string]any{"ports": "80", "identify": false}}, nil, probes)
	if err != nil {
		t.Fatal(err)
	}
	data := out.Data.(SubnetResult)
	if calls != 1 || data.LocalContext["partial"] != true || data.LocalContextError != "fixture partial local context" || data.Completed != 2 || len(data.Hosts) != 2 || data.Responded != 0 {
		t.Fatalf("local context error suppressed network results: %+v; calls %d", data, calls)
	}
}
