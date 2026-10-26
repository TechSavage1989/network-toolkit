package diag

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"networktoolkit/internal/deviceinfo"
	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

func TestNeighborIdentityUsesExactIPAndPreservesConflicts(t *testing.T) {
	rows := []platform.Neighbor{
		{Address: "192.0.2.1", MAC: "00:11:22:33:44:55", InterfaceIndex: 1, Vendor: "First assignment"},
		{Address: "192.0.2.1", MAC: "00:11:22:33:44:66", InterfaceIndex: 2, Vendor: "Second assignment"},
		{Address: "192.0.2.1", MAC: "00:11:22:33:44:55", InterfaceIndex: 1, Vendor: "First assignment"},
		{Address: "192.0.2.2", MAC: "00:11:22:AA:BB:CC", InterfaceIndex: 1, Vendor: "Observed assignment", VendorSource: "IEEE fixture", MACKind: "universally administered"},
		{Address: "192.0.2.2", MAC: "00-11-22-aa-bb-cc", InterfaceIndex: 2, Vendor: "Observed assignment"},
		{Address: "192.0.2.3", MAC: "00:00:00:00:00:00", Vendor: "Must be ignored"},
		{Address: "192.0.2.4", MAC: "01:00:5e:00:00:01", Vendor: "Must be ignored"},
		{Address: "192.0.2.5", MAC: "ff:ff:ff:ff:ff:ff", Vendor: "Must be ignored"},
		{Address: "192.0.2.6", MAC: "invalid", Vendor: "Must be ignored"},
		{Address: "192.0.2.254", MAC: "00:11:22:12:34:56", IsRouter: true, Vendor: "Gateway assignment"},
		{Address: "::ffff:192.0.2.7", MAC: "02:00:00:00:00:07", MACKind: "locally administered"},
	}
	identities := neighborIdentities(rows)
	if got := identities["192.0.2.1"]; got.MAC != "" || got.Vendor != "" || !strings.Contains(got.IdentityNote, "Conflicting") {
		t.Fatalf("conflicting interfaces attributed an arbitrary device: %+v", got)
	}
	if got := identities["192.0.2.2"]; got.MAC != "00:11:22:aa:bb:cc" || got.Vendor != "Observed assignment" || got.VendorSource != "IEEE fixture" || got.IdentityNote != "" {
		t.Fatalf("same-MAC duplicates were not normalized: %+v", got)
	}
	for _, address := range []string{"192.0.2.3", "192.0.2.4", "192.0.2.5", "192.0.2.6", "192.0.2.100"} {
		if _, exists := identities[address]; exists {
			t.Errorf("invalid/routed target %s inherited a MAC assignment", address)
		}
	}
	if got := identities["192.0.2.7"]; got.MAC != "02:00:00:00:00:07" || got.Vendor != "" {
		t.Fatalf("locally administered MAC changed attribution: %+v", got)
	}
}

func TestNeighborPartialTableIsRetained(t *testing.T) {
	identities, err := readNeighborIdentities(context.Background(), func(context.Context) ([]platform.Neighbor, error) {
		return []platform.Neighbor{{Address: "192.0.2.8", MAC: "00:11:22:33:44:55", Vendor: "Fixture assignment"}}, errors.New("fixture partial table")
	})
	if err == nil || identities["192.0.2.8"].Vendor != "Fixture assignment" {
		t.Fatalf("partial table discarded: %+v %v", identities, err)
	}
}

func TestDiscoveryCanFindUPnPOnlyDevice(t *testing.T) {
	probes := fakeSubnetProbes()
	var calls, neighborCalls atomic.Int32
	probes.neighbors = func(context.Context) ([]platform.Neighbor, error) {
		neighborCalls.Add(1)
		return []platform.Neighbor{{Address: "192.0.2.0", MAC: "00:11:22:33:44:55", Vendor: "MAC assignment"}}, nil
	}
	probes.device = func(_ context.Context, address string, timeout time.Duration) (deviceinfo.Info, error) {
		calls.Add(1)
		if timeout != deviceProbeTimeout {
			t.Errorf("unexpected device timeout %v", timeout)
		}
		if address == "192.0.2.0" {
			return deviceinfo.Info{Responded: true, Manufacturer: "Reported maker", Model: "Model 123", ModelName: "Model", ModelNumber: "123", FriendlyName: "Office device", ModelSource: "UPnP fixture", DescriptionURL: "http://192.0.2.0/device.xml"}, nil
		}
		return deviceinfo.Info{}, deviceinfo.ErrNoResponse
	}
	out, err := runSubnetScan(context.Background(), model.Request{Tool: "discover", Target: "192.0.2.0/31", Options: map[string]any{"ports": "80"}}, nil, probes)
	if err != nil {
		t.Fatal(err)
	}
	data := out.Data.(SubnetResult)
	hosts := data.Hosts
	if calls.Load() != 2 || neighborCalls.Load() != 2 || data.Completed != 2 || data.CompletedProbes != 8 || data.RequestedProbes != 8 || data.Responded != 1 || len(hosts) != 2 {
		t.Fatalf("UPnP-only coverage/counts: %+v calls %d/%d", data, calls.Load(), neighborCalls.Load())
	}
	host := hosts[0]
	if host.Address != "192.0.2.0" || !host.Responded || !host.Completed || host.Model != "Model 123" || host.Manufacturer != "Reported maker" || host.Vendor != "MAC assignment" || len(host.OpenPorts) != 0 || len(host.Evidence) != 1 || host.LatencyMs != nil {
		t.Fatalf("UPnP report changed/inferred base evidence: %+v", host)
	}
	probes.device = nil
	out, err = runSubnetScan(context.Background(), model.Request{Tool: "discover", Target: "192.0.2.0/31", Options: map[string]any{"ports": "80", "identify": false}}, nil, probes)
	if err != nil {
		t.Fatal(err)
	}
	data = out.Data.(SubnetResult)
	if len(data.Hosts) != 2 || data.Responded != 0 || data.Completed != 2 || data.CompletedProbes != 6 || data.RequestedProbes != 6 || data.Identify {
		t.Fatalf("disabled identity changed discovery results: %+v %v", out, err)
	}
}

func TestDiscoveryRetainsPartialRespondingHost(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	probes := fakeSubnetProbes()
	probes.ping = func(context.Context, string, time.Duration, int) (platform.PingReply, error) {
		cancel()
		return platform.PingReply{Success: true, Status: "reply", RTTMs: 1}, nil
	}
	probes.tcp = nil
	probes.device = nil
	out, err := runSubnetScan(ctx, model.Request{Tool: "discover", Target: "192.0.2.1", Options: map[string]any{"concurrency": 1, "ports": "80"}}, nil, probes)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	data := out.Data.(SubnetResult)
	hosts := data.Hosts
	if len(hosts) != 1 || hosts[0].Status != "cancelled" || hosts[0].Completed || !hosts[0].Responded || data.Completed != 0 || data.Started != 1 {
		t.Fatalf("partial discovery evidence lost: %+v", data)
	}
}

func TestIdentifyValidationBeforeProbes(t *testing.T) {
	for _, target := range []string{"", "example.test", "192.0.2.1/32", "http://192.0.2.1", "192.0.2.1:80", "::1", "0.0.0.0", "224.0.0.1", "255.255.255.255"} {
		if _, err := runIdentify(context.Background(), model.Request{Target: target}, nil, nil, nil, nil); err == nil {
			t.Errorf("accepted non-device target %q", target)
		}
	}
	if _, err := runIdentify(context.Background(), model.Request{Target: "192.0.2.1", Options: map[string]any{"timeoutMs": 5001}}, nil, nil, nil, nil); err == nil {
		t.Fatal("accepted unbounded identification timeout")
	}
}

func TestIdentifyRetainsCacheWithoutInventingModel(t *testing.T) {
	neighbors := func(context.Context) ([]platform.Neighbor, error) {
		return []platform.Neighbor{{Address: "192.0.2.1", MAC: "00:11:22:33:44:55", Vendor: "MAC assignment holder", VendorSource: "IEEE fixture"}}, nil
	}
	probe := func(_ context.Context, address string, timeout time.Duration) (deviceinfo.Info, error) {
		if address != "192.0.2.1" || timeout != deviceProbeTimeout {
			t.Errorf("unexpected explicit target/budget %s %v", address, timeout)
		}
		return deviceinfo.Info{}, deviceinfo.ErrNoResponse
	}
	out, err := runIdentify(context.Background(), model.Request{Target: "192.0.2.1"}, nil, neighbors, probe, nil)
	if err != nil {
		t.Fatal(err)
	}
	data := out.Data.(IdentifyResult)
	if data.Model != "" || data.Manufacturer != "" || data.ModelSource != "" || data.Vendor != "MAC assignment holder" || data.Responded || data.Status != "no_response" || data.IdentityError == "" {
		t.Fatalf("silent device fabricated model or erased neighbor evidence: %+v", data)
	}
	probe = func(context.Context, string, time.Duration) (deviceinfo.Info, error) {
		return deviceinfo.Info{Responded: true}, errors.New("fixture invalid description")
	}
	out, err = runIdentify(context.Background(), model.Request{Target: "192.0.2.1"}, nil, neighbors, probe, nil)
	if err != nil {
		t.Fatal(err)
	}
	data = out.Data.(IdentifyResult)
	if !data.Responded || data.Status != "responded" || len(data.Evidence) != 1 || data.Model != "" || data.IdentityError != "fixture invalid description" {
		t.Fatalf("reply evidence conflated with a reported model: %+v", data)
	}
	probe = func(context.Context, string, time.Duration) (deviceinfo.Info, error) {
		return deviceinfo.Info{}, errors.New("fixture socket setup failed")
	}
	out, err = runIdentify(context.Background(), model.Request{Target: "192.0.2.1"}, nil, neighbors, probe, nil)
	if err != nil {
		t.Fatal(err)
	}
	data = out.Data.(IdentifyResult)
	if data.Responded || data.Status != "error" || data.Vendor != "MAC assignment holder" {
		t.Fatalf("local probe error was reported as target silence: %+v", data)
	}
}
