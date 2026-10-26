package diag

import (
	"context"
	"errors"
	"networktoolkit/internal/deviceinfo"
	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
	"sync"
	"testing"
	"time"
)

func TestDiscoveryPublishesReachabilityBeforeSlowMetadata(t *testing.T) {
	probes := fakeSubnetProbes()
	release := make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	probes.ping = func(context.Context, string, time.Duration, int) (platform.PingReply, error) {
		return platform.PingReply{Success: true, Status: "reply", RTTMs: 1}, nil
	}
	probes.device = func(context.Context, string, time.Duration) (deviceinfo.Info, error) {
		<-release
		return deviceinfo.Info{Responded: true, Model: "Fixture model"}, nil
	}
	probes.hostname = func(context.Context, string) ([]string, error) { <-release; return []string{"fixture.test."}, nil }
	early := make(chan SubnetHost, 8)
	done := make(chan SubnetResult, 1)
	go func() {
		out, err := runSubnetScan(context.Background(), model.Request{Tool: "discover", Target: "192.0.2.0/30", Options: map[string]any{"ports": "80,443", "concurrency": 2}}, func(e model.Event) {
			if data, ok := e.Data.(map[string]any); ok && data["type"] == "host" {
				host := data["value"].(SubnetHost)
				if host.Enriching {
					early <- host
				}
			}
		}, probes)
		if err != nil {
			t.Error(err)
		}
		done <- out.Data.(SubnetResult)
	}()
	var first SubnetHost
	select {
	case first = <-early:
	case <-time.After(time.Second):
		t.Fatal("reachability waited for optional metadata")
	}
	if !first.Responded || first.Completed || first.Status != "scanning" || len(first.Ports) != 2 || len(first.Samples) != 1 {
		t.Fatalf("early evidence: %+v", first)
	}
	once.Do(func() { close(release) })
	select {
	case final := <-done:
		if final.Started != 2 || final.Completed != 2 || final.Responded != 2 || len(final.Hosts) != 2 || final.CompletedProbes != 10 {
			t.Fatalf("duplicate rows/counts: %+v", final)
		}
		for _, host := range final.Hosts {
			if host.Enriching || !host.Completed || host.Model != "Fixture model" || host.Hostname != "fixture.test" {
				t.Fatalf("metadata lost: %+v", host)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("metadata completion blocked")
	}
	if first.Model != "" || first.Hostname != "" || !first.Enriching {
		t.Fatal("published snapshot was mutated")
	}
}
func TestNativeHostnameDeadlineDoesNotReleaseOutstandingAdmission(t *testing.T) {
	release := make(chan struct{})
	exited := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := boundedNativeReverseLookup(ctx, "192.0.2.1", func(context.Context, string) ([]string, error) { defer close(exited); <-release; return nil, nil })
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 500*time.Millisecond {
		t.Fatalf("lookup ignored caller deadline: %v", err)
	}
	if len(nativeReverseSlots) != 1 {
		t.Fatal("outstanding native request lost its admission slot")
	}
	close(release)
	<-exited
	deadline := time.Now().Add(time.Second)
	for len(nativeReverseSlots) > 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(nativeReverseSlots) != 0 {
		t.Fatal("native admission was leaked")
	}
}

func TestParallelDiscoveryCancellationKeepsCompletedResponseEvidence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	probes := fakeSubnetProbes()
	started := make(chan struct{}, 2)
	probes.ping = func(context.Context, string, time.Duration, int) (platform.PingReply, error) {
		started <- struct{}{}
		return platform.PingReply{Success: true, Status: "reply", RTTMs: 1}, nil
	}
	probes.tcp = func(ctx context.Context, address string, port int, _ time.Duration) PortResult {
		<-ctx.Done()
		return PortResult{Address: address, Port: port, State: "cancelled"}
	}
	probes.device = func(ctx context.Context, _ string, _ time.Duration) (deviceinfo.Info, error) {
		<-ctx.Done()
		return deviceinfo.Info{}, ctx.Err()
	}
	probes.hostname = func(ctx context.Context, _ string) ([]string, error) { <-ctx.Done(); return nil, ctx.Err() }
	done := make(chan SubnetResult, 1)
	go func() {
		out, err := runSubnetScan(ctx, model.Request{Tool: "discover", Target: "192.0.2.0/30", Options: map[string]any{"ports": "80,443", "concurrency": 2}}, nil, probes)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("scan error: %v", err)
		}
		done <- out.Data.(SubnetResult)
	}()
	for n := 0; n < 2; n++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("parallel hosts did not start")
		}
	}
	cancel()
	select {
	case data := <-done:
		if !data.Cancelled || data.Completed != 0 || data.Responded != 2 || len(data.Hosts) != 2 {
			t.Fatalf("partial rows/counts: %+v", data)
		}
		for _, host := range data.Hosts {
			if !host.Responded || host.Completed || host.Enriching || host.Status != "cancelled" || len(host.Samples) != 1 {
				t.Fatalf("partial response lost: %+v", host)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("parallel scan did not cancel")
	}
}
