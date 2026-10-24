package diag

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

func traceResultHops(t *testing.T, result model.Result) []TraceHop {
	t.Helper()
	return result.Data.(map[string]any)["hops"].([]TraceHop)
}

func TestTraceHostnamesAreCachedAndEmittedAlongsideAddresses(t *testing.T) {
	ctx := platform.WithSourceBinding(context.Background(), diagnosticSourceFixture())
	lookupCalls := map[string]int{}
	probeCalls := 0
	var live []TraceHop
	checkSource := func(ctx context.Context) {
		t.Helper()
		binding, selected := platform.SourceBindingFromContext(ctx)
		if !selected || binding.InterfaceIndex != 7 || binding.Address != "192.0.2.20" {
			t.Fatalf("trace or reverse lookup lost selected source: %+v", binding)
		}
	}
	probe := func(ctx context.Context, destination string, _ time.Duration, ttl int) (platform.PingReply, error) {
		checkSource(ctx)
		probeCalls++
		if destination != "192.0.2.9" {
			t.Fatalf("unexpected target %s", destination)
		}
		if ttl < 3 {
			return platform.PingReply{Address: "192.0.2.1", Status: "ttl_expired", RTTMs: float64(ttl)}, nil
		}
		return platform.PingReply{Address: destination, Success: true, Status: "reply", RTTMs: 3}, nil
	}
	lookup := func(ctx context.Context, address string) ([]string, error) {
		checkSource(ctx)
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > hostnameLookupTimeout {
			t.Fatalf("reverse lookup was not bounded: %v %v", deadline, ok)
		}
		lookupCalls[address]++
		if address == "192.0.2.1" {
			return []string{"router.example.", "ROUTER.example."}, nil
		}
		return []string{"destination.example."}, nil
	}
	emit := func(e model.Event) {
		if data, ok := e.Data.(map[string]any); ok && data["type"] == "hop" {
			live = append(live, data["value"].(TraceHop))
		}
	}
	out, err := runTrace(ctx, model.Request{Target: "192.0.2.9", Options: map[string]any{"maxHops": 5}}, emit, probe, lookup)
	hops := traceResultHops(t, out)
	if err != nil || probeCalls != 3 || len(hops) != 3 || len(live) != 3 || lookupCalls["192.0.2.1"] != 1 || lookupCalls["192.0.2.9"] != 1 || out.Data.(map[string]any)["resolveHostnames"] != true {
		t.Fatalf("trace/cache/live coverage: %+v calls=%d/%v live=%+v err=%v", hops, probeCalls, lookupCalls, live, err)
	}
	for i := 0; i < 2; i++ {
		if hops[i].Address != "192.0.2.1" || hops[i].Hostname != "router.example" || hops[i].HostnameSource != "Selected-source reverse lookup" || hops[i].RTTMs == nil || *hops[i].RTTMs != float64(i+1) || live[i].Hostname != hops[i].Hostname || len(hops[i].Samples) != 1 {
			t.Errorf("hop %d lost IP/timing/name: %+v", i, hops[i])
		}
	}
	if !hops[2].Reached || hops[2].Hostname != "destination.example" || out.Data.(map[string]any)["reached"] != true {
		t.Fatalf("destination result changed: %+v", out)
	}
}

func TestTraceMissingPTRDoesNotFailRoute(t *testing.T) {
	for _, tc := range []struct {
		name      string
		lookupErr error
		status    string
	}{
		{"absent", &net.DNSError{Err: "fixture NXDOMAIN", IsNotFound: true}, "not_resolved"},
		{"timeout", &net.DNSError{Err: "fixture timeout", IsTimeout: true}, "timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			out, err := runTrace(context.Background(), model.Request{Target: "192.0.2.9", Options: map[string]any{"maxHops": 3}}, nil,
				func(_ context.Context, _ string, _ time.Duration, ttl int) (platform.PingReply, error) {
					if ttl == 1 {
						return platform.PingReply{Status: "timeout"}, nil
					}
					return platform.PingReply{Address: "192.0.2.9", Status: "reply", Success: true, RTTMs: 2.75}, nil
				},
				func(context.Context, string) ([]string, error) { calls++; return nil, tc.lookupErr })
			hops := traceResultHops(t, out)
			if err != nil || len(hops) != 2 || calls != 1 || hops[0].Address != "" || hops[1].Hostname != "" || hops[1].HostnameStatus != tc.status || hops[1].Address != "192.0.2.9" || hops[1].RTTMs == nil || *hops[1].RTTMs != 2.75 || !hops[1].Reached {
				t.Fatalf("missing PTR changed trace evidence: %+v %v", out, err)
			}
		})
	}
}

func TestTraceHostnameLookupCanBeDisabled(t *testing.T) {
	out, err := runTrace(context.Background(), model.Request{Target: "192.0.2.9", Options: map[string]any{"resolveHostnames": false}}, nil,
		func(context.Context, string, time.Duration, int) (platform.PingReply, error) {
			return platform.PingReply{Address: "192.0.2.9", Status: "reply", Success: true, RTTMs: 1}, nil
		},
		func(context.Context, string) ([]string, error) {
			t.Fatal("disabled trace performed reverse lookup")
			return nil, nil
		})
	hops := traceResultHops(t, out)
	if err != nil || len(hops) != 1 || hops[0].HostnameStatus != "disabled" || out.Data.(map[string]any)["resolveHostnames"] != false {
		t.Fatalf("disabled name option: %+v %v", out, err)
	}
}

func TestTraceCancellationDuringPTRKeepsCollectedHop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := time.Now()
	out, err := runTrace(ctx, model.Request{Target: "192.0.2.9"}, nil,
		func(context.Context, string, time.Duration, int) (platform.PingReply, error) {
			return platform.PingReply{Address: "192.0.2.1", Status: "ttl_expired", RTTMs: 4}, nil
		},
		func(ctx context.Context, _ string) ([]string, error) { cancel(); <-ctx.Done(); return nil, ctx.Err() })
	hops := traceResultHops(t, out)
	if !errors.Is(err, context.Canceled) || len(hops) != 1 || hops[0].Address != "192.0.2.1" || hops[0].RTTMs == nil || *hops[0].RTTMs != 4 || hops[0].HostnameStatus != "cancelled" || time.Since(started) > time.Second {
		t.Fatalf("cancelled PTR discarded or delayed hop: %+v %v", out, err)
	}
}

func TestTraceSelectedSourceWithoutDNSRetainsIPOnly(t *testing.T) {
	binding := diagnosticSourceFixture()
	binding.DNSServers = nil
	ctx := platform.WithSourceBinding(context.Background(), binding)
	out, err := runTrace(ctx, model.Request{Target: "192.0.2.9"}, nil,
		func(context.Context, string, time.Duration, int) (platform.PingReply, error) {
			return platform.PingReply{Address: "192.0.2.9", Status: "reply", Success: true}, nil
		}, systemReverseLookup)
	hops := traceResultHops(t, out)
	if err != nil || len(hops) != 1 || hops[0].Hostname != "" || !hops[0].Reached || !strings.Contains(hops[0].HostnameNote, "no compatible configured DNS") {
		t.Fatalf("selected trace fell back or rejected IP-only trace: %+v %v", out, err)
	}
}

func TestTraceWrongSourceFamilyStopsBeforeProbe(t *testing.T) {
	ctx := platform.WithSourceBinding(context.Background(), diagnosticSourceFixture())
	_, err := runTrace(ctx, model.Request{Target: "2001:db8::9"}, nil,
		func(context.Context, string, time.Duration, int) (platform.PingReply, error) {
			t.Fatal("incompatible source reached ICMP")
			return platform.PingReply{}, nil
		}, nil)
	if err == nil || !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("wrong-family trace error: %v", err)
	}
}
