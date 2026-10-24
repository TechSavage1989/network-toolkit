package diag

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"networktoolkit/internal/deviceinfo"
	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

func fakeSubnetProbes() subnetProbes {
	return subnetProbes{
		ping: func(context.Context, string, time.Duration, int) (platform.PingReply, error) {
			return platform.PingReply{Status: "timeout"}, nil
		},
		tcp: func(_ context.Context, address string, port int, _ time.Duration) PortResult {
			return PortResult{Port: port, Address: net.JoinHostPort(address, fmt.Sprint(port)), State: "timeout"}
		},
		system:    func(context.Context) (map[string]any, error) { return map[string]any{"fixture": true}, nil },
		neighbors: func(context.Context) ([]platform.Neighbor, error) { return nil, nil },
		device: func(context.Context, string, time.Duration) (deviceinfo.Info, error) {
			return deviceinfo.Info{}, errors.New("fixture: no UPnP response")
		},
		hostname: func(context.Context, string) ([]string, error) { return nil, nil },
	}
}

func TestSubnetValidationBeforeAnyOperation(t *testing.T) {
	cases := []model.Request{
		{Tool: "tcp", Target: "10.0.0.0/21"},
		{Tool: "ping", Target: "example.test"},
		{Tool: "tcp", Target: "10.0.0.0/24", Options: map[string]any{"ports": "1-65"}},
		{Tool: "discover", Target: "10.0.0.0/23", Options: map[string]any{"ports": "1-33"}},
		{Tool: "discover", Target: "10.0.0.1", Options: map[string]any{"ports": "1-65"}},
		{Tool: "tcp", Target: "10.0.0.1", Options: map[string]any{"ports": "1-1025"}},
		{Tool: "ping", Target: "10.0.0.1", Options: map[string]any{"timeoutMs": 199}},
		{Tool: "tcp", Target: "10.0.0.1", Options: map[string]any{"concurrency": 65}},
		{Tool: "tcp", Target: "10.0.0.1", Options: map[string]any{"concurrency": 0}},
	}
	for _, request := range cases {
		t.Run(fmt.Sprintf("%s_%s_%v", request.Tool, request.Target, request.Options), func(t *testing.T) {
			// Every injected operation is nil: validation must return before any
			// cache read, adapter lookup, socket, or native ICMP call is attempted.
			if _, err := runSubnetScan(context.Background(), request, nil, subnetProbes{}); err == nil {
				t.Fatalf("accepted invalid request: %+v", request)
			}
		})
	}
	for _, request := range []model.Request{
		{Tool: "tcp", Target: "10.0.0.0/24", Options: map[string]any{"ports": "1-64"}},
		{Tool: "discover", Target: "10.0.0.0/23", Options: map[string]any{"ports": "1-32"}},
		{Tool: "ping", Target: "10.0.0.0/22"},
	} {
		if _, err := parseSubnetOptions(request); err != nil {
			t.Errorf("rejected bounded request %+v: %v", request, err)
		}
	}
}

func TestDiscoveryCoversEveryHostAndPort(t *testing.T) {
	probes := fakeSubnetProbes()
	var mu sync.Mutex
	visits := map[string]int{}
	var systemCalls, neighborCalls atomic.Int32
	probes.system = func(context.Context) (map[string]any, error) {
		systemCalls.Add(1)
		return map[string]any{"fixture": true}, nil
	}
	probes.neighbors = func(context.Context) ([]platform.Neighbor, error) {
		neighborCalls.Add(1)
		return nil, nil
	}
	probes.ping = func(_ context.Context, address string, timeout time.Duration, ttl int) (platform.PingReply, error) {
		mu.Lock()
		visits[address+"/icmp"]++
		mu.Unlock()
		if timeout != 750*time.Millisecond || ttl != 128 {
			t.Errorf("unexpected ICMP settings %v/%d", timeout, ttl)
		}
		if strings.HasSuffix(address, ".1") {
			return platform.PingReply{}, errors.New("fixture ICMP unavailable")
		}
		return platform.PingReply{Status: "timeout"}, nil
	}
	probes.tcp = func(_ context.Context, address string, port int, timeout time.Duration) PortResult {
		mu.Lock()
		visits[fmt.Sprintf("%s/%d", address, port)]++
		mu.Unlock()
		if timeout != 750*time.Millisecond {
			t.Errorf("unexpected TCP timeout %v", timeout)
		}
		state := "timeout"
		if port == 80 {
			state = "closed"
			if strings.HasSuffix(address, ".2") {
				state = "open"
			}
		}
		return PortResult{Port: port, State: state, Address: net.JoinHostPort(address, fmt.Sprint(port)), LatencyMs: 2}
	}
	request := model.Request{Tool: "discover", Target: "192.0.2.0/30", Options: map[string]any{"ports": "80,443", "timeoutMs": 750, "identify": false, "count": 1000, "intervalMs": 10000}}
	out, err := runSubnetScan(context.Background(), request, nil, probes)
	if err != nil {
		t.Fatal(err)
	}
	data := out.Data.(SubnetResult)
	if data.Total != 2 || data.Started != 2 || data.Completed != 2 || data.Responded != 2 || data.RequestedProbes != 8 || data.CompletedProbes != 8 || data.NotStarted != 0 {
		t.Fatalf("incorrect counts: %+v", data.SubnetProgress)
	}
	if systemCalls.Load() != 1 || neighborCalls.Load() != 2 {
		t.Fatalf("local context/cache calls = %d/%d", systemCalls.Load(), neighborCalls.Load())
	}
	for _, address := range []string{"192.0.2.1", "192.0.2.2"} {
		for _, kind := range []string{"icmp", "80", "443"} {
			if visits[address+"/"+kind] != 1 {
				t.Errorf("visit count for %s/%s = %d", address, kind, visits[address+"/"+kind])
			}
		}
	}
	if len(data.Hosts) != 2 || data.Hosts[0].Address != "192.0.2.1" || data.Hosts[1].Address != "192.0.2.2" {
		t.Fatalf("host coverage/order: %+v", data.Hosts)
	}
	for _, host := range data.Hosts {
		if len(host.Samples) != 1 || host.Statistics.Sent != 1 || len(host.Ports) != 2 || host.Status != "responded" {
			t.Errorf("incorrect host result: %+v", host)
		}
	}
	if len(data.Hosts[0].OpenPorts) != 0 || len(data.Hosts[1].OpenPorts) != 1 || data.Hosts[0].Ports[0].State != "closed" || !strings.Contains(strings.Join(data.Hosts[0].Evidence, ","), "refused") {
		t.Fatalf("TCP refusal must be response evidence without being open: %+v", data.Hosts)
	}
}

func TestSubnetPingIsOneICMPAttemptPerAddress(t *testing.T) {
	probes := fakeSubnetProbes()
	var calls atomic.Int32
	probes.tcp = nil
	probes.device = nil // identify must never add traffic to Ping subnet mode.
	probes.ping = func(_ context.Context, address string, _ time.Duration, _ int) (platform.PingReply, error) {
		calls.Add(1)
		switch {
		case strings.HasSuffix(address, ".1"):
			return platform.PingReply{Address: address, Success: true, Status: "reply", RTTMs: 1.5}, nil
		case strings.HasSuffix(address, ".2"):
			return platform.PingReply{}, errors.New("fixture ICMP setup error")
		default:
			return platform.PingReply{Status: "timeout"}, nil
		}
	}
	out, err := runSubnetScan(context.Background(), model.Request{Tool: "ping", Target: "192.0.2.0/29", Options: map[string]any{"count": 1000, "intervalMs": 10000, "identify": true}}, nil, probes)
	if err != nil {
		t.Fatal(err)
	}
	data := out.Data.(SubnetResult)
	if calls.Load() != 6 || data.RequestedProbes != 6 || data.CompletedProbes != 6 || data.Completed != 6 || data.Responded != 1 || data.Identify {
		t.Fatalf("single-probe sweep changed by unrelated controls: %+v, calls %d", data, calls.Load())
	}
	if data.Hosts[0].Status != "responded" || data.Hosts[1].Status != "error" || data.Hosts[2].Status != "no_response" || data.Hosts[2].LatencyMs != nil || data.Hosts[2].Statistics.AvgMs != nil {
		t.Fatalf("wrong response/error/latency semantics: %+v", data.Hosts[:3])
	}
}

func TestSubnetOneGlobalPoolForAllProbeTypes(t *testing.T) {
	probes := fakeSubnetProbes()
	var active, maximum, total atomic.Int32
	entered := make(chan struct{}, 4)
	gate := make(chan struct{})
	var mu sync.Mutex
	visits := map[string]int{}
	operation := func(address, kind string) {
		current := active.Add(1)
		defer active.Add(-1)
		for previous := maximum.Load(); current > previous && !maximum.CompareAndSwap(previous, current); previous = maximum.Load() {
		}
		if total.Add(1) <= 4 {
			entered <- struct{}{}
			<-gate
		}
		mu.Lock()
		visits[address+"/"+kind]++
		mu.Unlock()
	}
	probes.ping = func(_ context.Context, address string, _ time.Duration, _ int) (platform.PingReply, error) {
		operation(address, "icmp")
		return platform.PingReply{Status: "timeout"}, nil
	}
	probes.tcp = func(_ context.Context, address string, port int, _ time.Duration) PortResult {
		operation(address, fmt.Sprint(port))
		return PortResult{Port: port, State: "timeout"}
	}
	probes.device = func(_ context.Context, address string, timeout time.Duration) (deviceinfo.Info, error) {
		operation(address, "device")
		if timeout != deviceProbeTimeout {
			t.Errorf("device timeout = %v", timeout)
		}
		return deviceinfo.Info{Responded: true, Model: "Fixture device", ModelSource: "UPnP fixture"}, nil
	}
	probes.hostname = func(_ context.Context, address string) ([]string, error) {
		operation(address, "hostname")
		return []string{"fixture.test."}, nil
	}
	finished := make(chan model.Result, 1)
	go func() {
		out, err := runSubnetScan(context.Background(), model.Request{Tool: "discover", Target: "192.0.2.0/28", Options: map[string]any{"ports": "80,443,8080", "concurrency": 4, "identify": true}}, nil, probes)
		if err != nil {
			t.Error(err)
		}
		finished <- out
	}()
	for i := 0; i < 4; i++ {
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			close(gate)
			t.Fatal("configured host workers did not become active")
		}
	}
	close(gate)
	var out model.Result
	select {
	case out = <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("bounded scan did not finish")
	}
	data := out.Data.(SubnetResult)
	if maximum.Load() != 4 || active.Load() != 0 || total.Load() != 84 || data.CompletedProbes != 84 || data.RequestedProbes != 84 || data.Completed != 14 || data.Responded != 14 {
		t.Fatalf("global bounds/counts: peak %d active %d total %d %+v", maximum.Load(), active.Load(), total.Load(), data.SubnetProgress)
	}
	for _, host := range data.Hosts {
		for _, kind := range []string{"icmp", "80", "443", "8080", "device", "hostname"} {
			if visits[host.Address+"/"+kind] != 1 {
				t.Errorf("probe skipped/duplicated: %s/%s", host.Address, kind)
			}
		}
		if host.Model != "Fixture device" || !host.IdentityResponded {
			t.Errorf("reported identity was lost: %+v", host)
		}
	}
}

func TestSubnetCancelledBeforeStartDoesNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, err := runSubnetScan(ctx, model.Request{Tool: "tcp", Target: "192.0.2.0/30"}, nil, subnetProbes{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v", err)
	}
	data := out.Data.(SubnetResult)
	if data.Total != 2 || data.Started != 0 || data.CompletedProbes != 0 || !data.Cancelled || data.NotStarted != 2 || len(data.Hosts) != 0 {
		t.Fatalf("unexpected cancelled result: %+v", data)
	}
}

func TestSubnetCancellationRetainsEvidenceAndImmutableEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	probes := fakeSubnetProbes()
	probes.ping = func(context.Context, string, time.Duration, int) (platform.PingReply, error) {
		return platform.PingReply{Address: "192.0.2.9", Success: true, RTTMs: 1, Status: "reply"}, nil
	}
	var tcpCalls, neighborCalls int
	probes.tcp = func(_ context.Context, address string, port int, _ time.Duration) PortResult {
		tcpCalls++
		cancel()
		return PortResult{Address: address, Port: port, State: "open", LatencyMs: 2}
	}
	probes.neighbors = func(readCtx context.Context) ([]platform.Neighbor, error) {
		neighborCalls++
		vendor := "Initial vendor"
		if neighborCalls == 2 {
			vendor = "Refreshed vendor"
			if readCtx.Err() != nil {
				t.Error("final read-only cache refresh inherited cancelled probe context")
			}
		}
		return []platform.Neighbor{{Address: "192.0.2.9", MAC: "00:11:22:33:44:55", Vendor: vendor}}, nil
	}
	var events []SubnetHost
	emit := func(e model.Event) {
		if data, ok := e.Data.(map[string]any); ok && data["type"] == "host" {
			events = append(events, data["value"].(SubnetHost))
		}
	}
	out, err := runSubnetScan(ctx, model.Request{Tool: "discover", Target: "192.0.2.9", Options: map[string]any{"ports": "80,443", "identify": false}}, emit, probes)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v", err)
	}
	data := out.Data.(SubnetResult)
	if tcpCalls != 1 || data.Started != 1 || data.Completed != 0 || data.Responded != 1 || data.CompletedProbes != 2 || data.RequestedProbes != 4 || len(data.Hosts) != 1 || !data.Cancelled {
		t.Fatalf("partial result or probe dispatch wrong: %+v tcp %d", data, tcpCalls)
	}
	host := data.Hosts[0]
	if host.Status != "cancelled" || host.Completed || !host.Responded || len(host.Ports) != 1 || len(host.Samples) != 1 || len(host.Evidence) != 2 || host.Vendor != "Refreshed vendor" {
		t.Fatalf("partial evidence lost: %+v", host)
	}
	if len(events) != 2 || events[0].Vendor != "Initial vendor" || events[1].Vendor != "Refreshed vendor" || len(events[0].Evidence) != 2 {
		t.Fatalf("published snapshots mutated or refresh missing: %+v", events)
	}
}

func TestSubnetCancelStopsActivePoolAndNewDispatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	probes := fakeSubnetProbes()
	entered := make(chan struct{}, 4)
	var calls atomic.Int32
	probes.ping = func(ctx context.Context, _ string, _ time.Duration, _ int) (platform.PingReply, error) {
		calls.Add(1)
		entered <- struct{}{}
		<-ctx.Done()
		return platform.PingReply{}, ctx.Err()
	}
	type completed struct {
		out model.Result
		err error
	}
	finished := make(chan completed, 1)
	go func() {
		out, err := runSubnetScan(ctx, model.Request{Tool: "ping", Target: "192.0.2.0/26", Options: map[string]any{"concurrency": 4}}, nil, probes)
		finished <- completed{out, err}
	}()
	for i := 0; i < 4; i++ {
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			t.Fatal("active workers did not start")
		}
	}
	cancel()
	select {
	case result := <-finished:
		if !errors.Is(result.err, context.Canceled) {
			t.Fatal(result.err)
		}
		data := result.out.Data.(SubnetResult)
		if calls.Load() != 4 || data.Started != 4 || data.Completed != 0 || data.CompletedProbes != 4 || data.NotStarted != 58 || len(data.Hosts) != 4 {
			t.Fatalf("cancellation started extra work or dropped partial hosts: %+v calls %d", data, calls.Load())
		}
		for _, host := range data.Hosts {
			if host.Status != "cancelled" || host.Completed {
				t.Errorf("unfinished host counted as completed: %+v", host)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled workers did not drain")
	}
}

func TestSubnetLocalTCPDispatch(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	openPort := listener.Addr().(*net.TCPAddr).Port
	refused, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedPort := refused.Addr().(*net.TCPAddr).Port
	refused.Close()
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			connection.Close()
		}
	}()
	out, err := Run(context.Background(), model.Request{Tool: "tcp", Target: "127.0.0.1/32", Options: map[string]any{"scope": "subnet", "ports": fmt.Sprintf("%d,%d", openPort, closedPort), "timeoutMs": 500}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	data := out.Data.(SubnetResult)
	if len(data.Hosts) != 1 || data.Completed != 1 || !data.Hosts[0].Responded || len(data.Hosts[0].OpenPorts) != 1 || data.Hosts[0].OpenPorts[0] != openPort {
		t.Fatalf("local subnet integration: %+v", data)
	}
	for _, port := range data.Hosts[0].Ports {
		if port.Port == openPort && port.State != "open" || port.Port == closedPort && port.State != "closed" {
			t.Errorf("open/refused classification changed: %+v", port)
		}
	}
}
