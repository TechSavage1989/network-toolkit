package diag

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

func TestIPv4ScopeFormatsAndUnion(t *testing.T) {
	cases := []struct {
		name, input string
		count       int
		first, last string
		scopes      []string
	}{
		{"literal", "192.0.2.10", 1, "192.0.2.10", "192.0.2.10", []string{"192.0.2.10"}},
		{"cidr", "192.0.2.58/30", 2, "192.0.2.57", "192.0.2.58", []string{"192.0.2.56/30"}},
		{"mask", "192.0.2.58 \t/ \t255.255.255.252", 2, "192.0.2.57", "192.0.2.58", []string{"192.0.2.56/30"}},
		{"short range", "192.0.2.10 - 14", 5, "192.0.2.10", "192.0.2.14", []string{"192.0.2.10-192.0.2.14"}},
		{"cross subnet", "192.0.2.254-192.0.3.1", 4, "192.0.2.254", "192.0.3.1", []string{"192.0.2.254-192.0.3.1"}},
		{"wildcard", "192.0.2.*", 254, "192.0.2.1", "192.0.2.254", []string{"192.0.2.0/24"}},
		{"point to point", "192.0.2.1/31", 2, "192.0.2.0", "192.0.2.1", []string{"192.0.2.0/31"}},
		{"host prefix", "192.0.2.1/32", 1, "192.0.2.1", "192.0.2.1", []string{"192.0.2.1/32"}},
		{"maximum IPv4", "255.255.255.254-255", 2, "255.255.255.254", "255.255.255.255", []string{"255.255.255.254-255.255.255.255"}},
		{"mixed overlap and explicit endpoints", "192.0.2.58/30;192.0.2.57-60\n192.0.2.56\t192.0.2.60", 5, "192.0.2.56", "192.0.2.60", []string{"192.0.2.56/30", "192.0.2.57-192.0.2.60", "192.0.2.56", "192.0.2.60"}},
		{"canonical duplicates", "192.0.2.0/24\n\n192.0.2.* ; 192.0.2.55/255.255.255.0", 254, "192.0.2.1", "192.0.2.254", []string{"192.0.2.0/24"}},
		{"separate networks", "192.0.3.5,192.0.2.0/30;192.0.3.5", 3, "192.0.2.1", "192.0.3.5", []string{"192.0.3.5", "192.0.2.0/30"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := parseIPv4Scopes(tc.input, 1024)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.targets) != tc.count || plan.targets[0].String() != tc.first || plan.targets[len(plan.targets)-1].String() != tc.last || !reflect.DeepEqual(plan.scopes, tc.scopes) {
				t.Fatalf("unexpected scope plan: %+v", plan)
			}
			for i := 1; i < len(plan.targets); i++ {
				if !plan.targets[i-1].Less(plan.targets[i]) {
					t.Fatalf("addresses not unique and numeric-sorted: %v", plan.targets)
				}
			}
		})
	}
}

func TestIPv4ScopeValidationAndCombinedLimits(t *testing.T) {
	for _, input := range []string{
		"", " ,; \n\t", "192.0.2.999", "example.test", "::1", "::ffff:192.0.2.1",
		"192.0.2.1-256", "192.0.2.10-9", "192.0.3.10-192.0.2.1", "192.0.2.1-2-3",
		"192.0.2.1/33", "192.0.2.1/+24", "192.0.2.1/24/24", "192.0.2.1/255.0.255.0",
		"192.0.*.*", "192.0.2.1*", "192.0.2.*-254", "192.0.2.1\ninvalid",
		"0.0.0.0/0", "0.0.0.0-255.255.255.255", "192.0.0.0/22;192.0.0.0;192.0.3.255;192.0.4.0",
		strings.Repeat("192.0.2.1,", maxScanScopeText/10+1),
	} {
		t.Run(input[:min(70, len(input))], func(t *testing.T) {
			if _, err := parseIPv4Scopes(input, 1024); err == nil {
				t.Fatalf("accepted invalid or excessive scope %q", input[:min(70, len(input))])
			}
		})
	}
	// Overlapping CIDRs are counted once, and explicit .0/.255 endpoints remain.
	plan, err := parseIPv4Scopes("192.0.0.0/22,192.0.1.0/24,192.0.0.0,192.0.3.255", 1024)
	if err != nil || len(plan.targets) != 1024 {
		t.Fatalf("combined exact limit failed: %d %v", len(plan.targets), err)
	}
	// A single host keeps the historical network field used by existing reports.
	plan, err = parseIPv4Scopes("192.0.2.3", 1024)
	if err != nil || plan.network != "192.0.2.3/32" {
		t.Fatalf("single host compatibility: %+v %v", plan, err)
	}
	plan, err = parseIPv4Scopes("192.0.2.0/30,198.51.100.0/30", 1024)
	if err != nil || plan.network != "192.0.2.0/30, 198.51.100.0/30" {
		t.Fatalf("combined label misstates scope: %+v %v", plan, err)
	}
}

func TestTCPPortRangesAndListSeparators(t *testing.T) {
	ports, err := parsePorts("22; 80-82\n443\t8000 - 8002,81", 64)
	if err != nil || !reflect.DeepEqual(ports, []int{22, 80, 81, 82, 443, 8000, 8001, 8002}) {
		t.Fatalf("mixed TCP list: %v %v", ports, err)
	}
	for _, input := range []string{",80", "80,", "80,,443", "80,;", "+80", "80-+82", "443/udp", "ssh", "22;whoami", strings.Repeat("80 ", maxScanScopeText/3+1)} {
		if _, err := parsePorts(input, 64); err == nil {
			t.Errorf("accepted invalid TCP list %q", input[:min(70, len(input))])
		}
	}
}

func TestMultiScopeScanDeduplicatesAndRecordsCombinedProgress(t *testing.T) {
	request := model.Request{Tool: "discover", Target: "192.0.2.0/30\n192.0.2.2-3;198.51.100.7", Options: map[string]any{"ports": "80-81\n443", "identify": false, "resolveHostnames": false}}
	probes := fakeSubnetProbes()
	var mu sync.Mutex
	visited := map[string]int{}
	var systemCalls atomic.Int32
	probes.system = func(context.Context) (map[string]any, error) {
		systemCalls.Add(1)
		return map[string]any{"fixture": true}, nil
	}
	probes.ping = func(_ context.Context, address string, _ time.Duration, _ int) (platform.PingReply, error) {
		mu.Lock()
		visited[address+"/icmp"]++
		mu.Unlock()
		return platform.PingReply{Status: "timeout"}, nil
	}
	probes.tcp = func(_ context.Context, address string, port int, _ time.Duration) PortResult {
		mu.Lock()
		visited[fmt.Sprintf("%s/%d", address, port)]++
		mu.Unlock()
		return PortResult{Port: port, State: "closed"}
	}
	var progress []discoveryProgress
	emit := func(e model.Event) {
		if data, ok := e.Data.(map[string]any); ok && data["type"] == "progress" {
			progress = append(progress, data["value"].(discoveryProgress))
		}
	}
	result, err := runSubnetScan(context.Background(), request, emit, probes)
	if err != nil {
		t.Fatal(err)
	}
	data := result.Data.(SubnetResult)
	if data.Range != request.Target || !reflect.DeepEqual(data.Scopes, []string{"192.0.2.0/30", "192.0.2.2-192.0.2.3", "198.51.100.7"}) || data.Total != 4 || data.Started != 4 || data.Completed != 4 || data.RequestedProbes != 16 || data.CompletedProbes != 16 || systemCalls.Load() != 1 {
		t.Fatalf("combined metadata/progress incorrect: %+v; system calls %d", data, systemCalls.Load())
	}
	if len(visited) != 16 {
		t.Fatalf("wrong probe count: %v", visited)
	}
	for key, count := range visited {
		if count != 1 {
			t.Errorf("duplicate probe %s: %d", key, count)
		}
	}
	if len(progress) < 2 || progress[0].Started != 0 || progress[len(progress)-1].SubnetProgress != data.SubnetProgress {
		t.Fatalf("combined initial/final progress missing: %+v", progress)
	}
	for _, event := range progress {
		if event.Total != 4 || event.RequestedProbes != 16 || event.Scope != "subnet" {
			t.Errorf("progress was split into individual scopes: %+v", event)
		}
	}
}

func TestMultiScopeValidationAndCancellationBeforeOperations(t *testing.T) {
	for _, request := range []model.Request{
		{Tool: "discover", Target: "192.0.2.0/30\nexample.test"},
		{Tool: "discover", Target: "192.0.0.0/22;192.0.4.0/24"},
		{Tool: "discover", Target: "192.0.2.0/24;198.51.100.0/24", Options: map[string]any{"ports": "1-33"}},
		{Tool: "discover", Target: "192.0.2.1;192.0.2.2", Options: map[string]any{"ports": "80-1"}},
		{Tool: "discover", Target: "192.0.2.1", Options: map[string]any{"ports": "80" + strings.Repeat(" ", maxScanScopeText)}},
	} {
		if _, err := runSubnetScan(context.Background(), request, nil, subnetProbes{}); err == nil {
			t.Fatalf("invalid list reached work: %+v", request)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, err := runSubnetScan(ctx, model.Request{Tool: "discover", Target: "192.0.2.0/30;198.51.100.7", Options: map[string]any{"ports": "80-81", "identify": false, "resolveHostnames": false}}, nil, subnetProbes{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation before operations: %v", err)
	}
	data := out.Data.(SubnetResult)
	if data.Total != 3 || data.Started != 0 || data.NotStarted != 3 || data.RequestedProbes != 9 || !data.Cancelled || len(data.Scopes) != 2 {
		t.Fatalf("cancelled combined scope lost its plan: %+v", data)
	}
}

func TestMultiScopeCancellationStopsRemainingScopes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	probes := fakeSubnetProbes()
	var tcpCalls int
	probes.tcp = func(_ context.Context, address string, port int, _ time.Duration) PortResult {
		tcpCalls++
		cancel()
		return PortResult{Address: address, Port: port, State: "open"}
	}
	request := model.Request{Tool: "discover", Target: "192.0.2.1;198.51.100.4;203.0.113.9", Options: map[string]any{"ports": "80-81", "identify": false, "resolveHostnames": false, "concurrency": 1}}
	result, err := runSubnetScan(ctx, request, nil, probes)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation: %v", err)
	}
	data := result.Data.(SubnetResult)
	if tcpCalls != 1 || data.Total != 3 || data.Started != 1 || data.Completed != 0 || data.NotStarted != 2 || data.CompletedProbes != 2 || data.RequestedProbes != 9 || !data.Cancelled || len(data.Scopes) != 3 || len(data.Hosts) != 1 {
		t.Fatalf("cancellation lost the combined plan or dispatched remaining scopes: %+v; TCP calls %d", data, tcpCalls)
	}
	if host := data.Hosts[0]; host.Address != "192.0.2.1" || host.Completed || !host.Responded || host.Status != "cancelled" || len(host.OpenPorts) != 1 || host.OpenPorts[0] != 80 {
		t.Fatalf("partial host evidence lost: %+v", host)
	}
}
