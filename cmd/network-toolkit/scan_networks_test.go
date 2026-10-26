package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"networktoolkit/internal/model"
)

func scanNetworkFixtureSystem() map[string]any {
	return map[string]any{"interfaces": []map[string]any{
		{"index": 7, "name": "Ethernet 5", "description": "Intel Ethernet", "up": true, "addresses": []string{"192.168.1.20/24", "2001:db8::20/64"}},
		{"index": 17, "name": "VPN", "description": "Virtual VPN adapter", "up": true, "addresses": []string{"10.8.0.2/32"}},
		{"index": 18, "name": "Offline", "description": "Disconnected adapter", "up": false, "addresses": []string{"172.16.4.5/24"}},
		{"index": 19, "name": "Tentative", "up": true, "addresses": []string{"172.16.5.5/24"}, "addressDetails": []map[string]any{{"address": "172.16.5.5", "state": "tentative"}}},
		{"index": 1, "name": "Loopback", "up": true, "loopback": true, "addresses": []string{"127.0.0.1/8"}},
	}}
}

func TestScanNetworksFilterSystemRoutesAndPreserveRemoteScopes(t *testing.T) {
	rows := []map[string]any{}
	for _, cidr := range []string{
		"0.0.0.0/0", "0.0.0.0/1", "128.0.0.0/1", "0.0.0.0/8", "0.0.0.0/32",
		"127.0.0.0/8", "127.0.0.1/32", "169.254.0.0/16", "169.254.1.1/32",
		"224.0.0.0/4", "239.255.255.250/32", "240.0.0.0/4", "255.255.255.255/32",
		"192.168.1.0/24", "192.168.1.20/32", "192.168.1.255/32", "10.8.0.2/32",
		"172.16.4.5/32", "172.16.4.255/32", "2001:db8::/64",
	} {
		rows = append(rows, map[string]any{"destination": cidr})
	}
	excludedCount := len(rows)
	for _, cidr := range []string{"10.0.0.0/8", "10.8.0.1/32", "10.20.0.7/16", "172.16.4.0/24", "172.16.5.0/24", "192.168.1.1/32", "192.168.2.254/31", "192.168.2.255/32", "100.64.0.0/10", "8.8.8.8/32"} {
		rows = append(rows, map[string]any{"destination": cidr, "gateway": "192.168.1.1", "interface": "192.168.1.20"})
	}
	networks, excluded := scanNetworkSuggestions(rows, scanNetworkFixtureSystem())
	if excluded != excludedCount || len(networks) != 10 {
		t.Fatalf("filtered %d, suggestions %d: %+v", excluded, len(networks), networks)
	}
	want := map[string]uint64{"10.0.0.0/8": 16777214, "10.8.0.1/32": 1, "10.20.0.0/16": 65534, "172.16.4.0/24": 254, "172.16.5.0/24": 254, "192.168.1.1/32": 1, "192.168.2.254/31": 2, "192.168.2.255/32": 1, "100.64.0.0/10": 4194302, "8.8.8.8/32": 1}
	for _, network := range networks {
		count, ok := want[network.CIDR]
		if !ok || count != network.HostCount {
			t.Errorf("unexpected scope/count: %+v", network)
		}
		if len(network.Paths) != 1 || network.Paths[0].InterfaceName != "Ethernet 5" || network.Paths[0].InterfaceDescription != "Intel Ethernet" || network.Paths[0].InterfaceIndex != 7 {
			t.Errorf("adapter identity missing: %+v", network.Paths)
		}
	}
}

func TestScanNetworksCanonicalDedupRetainsDistinctPaths(t *testing.T) {
	rows := []map[string]any{
		{"destination": "10.20.7.8", "mask": "255.255.0.0", "gateway": "192.168.1.1", "interface": "192.168.1.20", "metric": 30},
		{"destination": "10.20.0.0/16", "gateway": "192.168.1.1", "interface": "192.168.1.20", "metric": 30},
		{"cidr": "10.20.9.1/16", "gateway": "10.8.0.1", "interfaceIndex": 17, "metric": 5},
		{"destination": "10.21.1.1/24", "gateway": "0.0.0.0", "interface": "VPN", "metric": 1},
	}
	networks, excluded := scanNetworkSuggestions(rows, scanNetworkFixtureSystem())
	if excluded != 0 || len(networks) != 2 || networks[0].CIDR != "10.20.0.0/16" || len(networks[0].Paths) != 2 {
		t.Fatalf("canonical route/path merge failed: %d %+v", excluded, networks)
	}
	first := networks[0].Paths[0]
	if first.Gateway != "10.8.0.1" || first.Metric != 5 || first.InterfaceIndex != 17 || first.InterfaceName != "VPN" || first.InterfaceDescription != "Virtual VPN adapter" {
		t.Fatalf("preferred route identity lost: %+v", first)
	}
	onLink := networks[1].Paths[0]
	if !onLink.OnLink || onLink.Gateway != "" || onLink.InterfaceIndex != 17 || onLink.InterfaceDescription != "Virtual VPN adapter" {
		t.Fatalf("Linux on-link/alias route not enriched: %+v", onLink)
	}
	// The same snapshot can arrive as native platform maps or JSON-decoded
	// arrays/numbers. Both must produce the same canonical suggestions.
	encoded, _ := json.Marshal(map[string]any{"routes": rows, "system": scanNetworkFixtureSystem()})
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	decodedRows, _ := scanNetworkRows(decoded["routes"])
	fromJSON, excluded := scanNetworkSuggestions(decodedRows, decoded["system"].(map[string]any))
	if excluded != 0 || !reflect.DeepEqual(networks, fromJSON) {
		t.Fatalf("native/JSON data shape mismatch: %+v / %+v", networks, fromJSON)
	}
}

func TestScanNetworksRejectMalformedPrefixes(t *testing.T) {
	for _, row := range []map[string]any{
		nil, {"destination": "not-a-route"}, {"destination": "10.0.0.1"},
		{"destination": "10.0.0.1/33"}, {"destination": "10.0.0.1", "mask": "255.0.255.0"},
		{"destination": "10.0.0.1", "mask": "bad"}, {"destination": "::ffff:10.0.0.1/120"},
		{"destination": "example.invalid/24"}, {"destination": "10.0.0.1;whoami", "mask": "255.255.255.0"},
	} {
		if _, ok := scanRoutePrefix(row); ok {
			t.Errorf("malformed or non-IPv4 route accepted: %#v", row)
		}
	}
	for _, row := range []map[string]any{
		{"destination": "10.0.0.19/24"}, {"destination": " 10.0.0.19 ", "mask": "255.255.255.0"}, {"cidr": "10.0.0.19/24"},
	} {
		prefix, ok := scanRoutePrefix(row)
		if !ok || prefix.String() != "10.0.0.0/24" {
			t.Errorf("valid route not canonicalized: %#v: %v", row, prefix)
		}
	}
}

func TestScanNetworksAmbiguousAdapterAndPointToPointIdentity(t *testing.T) {
	system := map[string]any{"interfaces": []map[string]any{
		{"index": 7, "name": "Ethernet", "description": "First", "up": true, "addresses": []string{"192.168.1.20/24", "10.40.0.0/31"}},
		{"index": 8, "name": "Other", "description": "Second", "up": true, "addresses": []string{"192.168.1.20/24"}},
	}}
	rows := []map[string]any{
		{"destination": "10.60.0.0/24", "gateway": "192.168.1.1", "interface": "192.168.1.20"},
		{"destination": "10.61.0.0/24", "gateway": "192.168.1.1", "interfaceIndex": 8, "interface": "192.168.1.20"},
		{"destination": "10.40.0.1/32", "onLink": true, "gateway": "On-link", "interfaceIndex": 7},
	}
	networks, excluded := scanNetworkSuggestions(rows, system)
	if excluded != 0 || len(networks) != 3 {
		t.Fatalf("point-to-point peer was treated as broadcast: %d %+v", excluded, networks)
	}
	if path := networks[1].Paths[0]; path.InterfaceIndex != 0 || path.InterfaceName != "" || path.InterfaceDescription != "" || path.InterfaceAddress != "192.168.1.20" {
		t.Fatalf("ambiguous local address selected an arbitrary adapter: %+v", path)
	}
	if path := networks[2].Paths[0]; path.InterfaceIndex != 8 || path.InterfaceName != "Other" || path.InterfaceDescription != "Second" {
		t.Fatalf("explicit route interface identity lost: %+v", path)
	}
	context := scanRouteContext(system)
	stale := context.path(map[string]any{"interfaceIndex": 7, "interfaceName": "Replacement adapter", "interface": "192.168.1.20"})
	if stale.InterfaceName != "Replacement adapter" || stale.InterfaceDescription != "" {
		t.Fatalf("cached description attached to a changed adapter identity: %+v", stale)
	}
}

func TestScanNetworksReadOnlySnapshotDeadlineAndPartialErrors(t *testing.T) {
	calls := 0
	response := readScanNetworks(context.Background(), nil, func(ctx context.Context, tool string) (model.Result, error) {
		calls++
		deadline, ok := ctx.Deadline()
		if tool != "routes" || !ok || time.Until(deadline) > scanNetworksTimeout || time.Until(deadline) < time.Second {
			t.Fatalf("routing snapshot missing bounded timeout: %s %v", tool, deadline)
		}
		return model.Result{Data: map[string]any{"routes": []map[string]any{{"destination": "10.9.0.0/24"}}}}, errors.New("fixture partial route read")
	})
	if calls != 1 || len(response.Networks) != 1 || !strings.Contains(response.Error, "fixture partial route read") || !strings.Contains(response.Note, "does not change the source") {
		t.Fatalf("partial snapshot context lost: %+v", response)
	}
	if _, err := time.Parse(time.RFC3339, response.CheckedAt); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	response = readScanNetworks(ctx, nil, func(context.Context, string) (model.Result, error) {
		t.Fatal("canceled snapshot invoked the platform")
		return model.Result{}, nil
	})
	if response.Error == "" || response.Networks == nil || len(response.Networks) != 0 {
		t.Fatalf("cancellation not represented: %+v", response)
	}
	response = readScanNetworks(context.Background(), nil, func(context.Context, string) (model.Result, error) {
		return model.Result{}, nil
	})
	if response.Error == "" || response.Networks == nil {
		t.Fatalf("missing route data claimed success: %+v", response)
	}
}

func TestScanNetworksRejectMutationMethodsBeforePlatformRead(t *testing.T) {
	a := &application{}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		recorder := httptest.NewRecorder()
		a.handleScanNetworks(recorder, httptest.NewRequest(method, "/api/scan-networks", nil))
		if recorder.Code != http.StatusMethodNotAllowed {
			t.Fatalf("mutation method %s accepted: %d", method, recorder.Code)
		}
	}
}
