package diag

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

func parsedMeasurement(t *testing.T, stamp string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil || parsed.Location() != time.UTC {
		t.Fatalf("missing or non-UTC acquisition time %q: %v", stamp, err)
	}
	return parsed
}

func TestMeasurementTraceKeepsOriginalProbeClocksInLiveAndFinalRows(t *testing.T) {
	clocks := []string{"2026-10-26T12:34:30Z", "2026-10-26T12:34:30.250123456Z"}
	index := 0
	var live TraceHop
	out, err := runTrace(context.Background(), model.Request{Target: "192.0.2.9", Options: map[string]any{"probes": 2, "resolveHostnames": false}}, func(e model.Event) {
		if data, ok := e.Data.(map[string]any); ok && data["type"] == "hop" {
			live = data["value"].(TraceHop)
		}
	}, func(context.Context, string, time.Duration, int) (platform.PingReply, error) {
		stamp := clocks[index]
		index++
		return platform.PingReply{Time: stamp, Address: "192.0.2.9", Success: true, Status: "reply"}, nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	row := out.Data.(map[string]any)["hops"].([]TraceHop)[0]
	if row.Time != clocks[1] || live.Time != row.Time || len(row.Samples) != 2 {
		t.Fatalf("hop/latest observation was retimed: live=%+v final=%+v", live, row)
	}
	for i, sample := range row.Samples {
		if sample.Time != clocks[i] || live.Samples[i].Time != clocks[i] {
			t.Fatalf("probe %d lost its original clock: %+v", i, sample)
		}
	}
}

func TestMeasurementContinuousSamplesRetainNativeTimeThroughCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	series := &sampleSeries{}
	const nativeTime = "2026-10-26T12:34:30.123456789Z"
	err := series.collect(ctx, 0, 0, "icmp", 0, func(event model.Event) {
		row := event.Data.(map[string]any)["value"].(platform.PingReply)
		if row.Time != nativeTime {
			t.Fatalf("native timestamp was replaced: %+v", row)
		}
		cancel()
	}, func(context.Context) (platform.PingReply, error) {
		return platform.PingReply{Time: nativeTime, Success: true, Status: "reply"}, nil
	})
	if !errors.Is(err, context.Canceled) || len(series.samples()) != 1 || series.samples()[0].Time != nativeTime {
		t.Fatalf("cancelled result lost acquisition time: %+v %v", series.samples(), err)
	}
}

func TestMeasurementPortRowsMatchLiveEventsAndAcquisitionWindow(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, e := listener.Accept()
		if e == nil {
			conn.Close()
		}
	}()
	_, rawPort, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(rawPort)
	before := time.Now().UTC()
	var live PortResult
	rows := checkPorts(context.Background(), "127.0.0.1", []int{port}, time.Second, 1, func(event model.Event) {
		live = event.Data.(map[string]any)["value"].(PortResult)
	})
	after := time.Now().UTC()
	if len(rows) != 1 || rows[0].State != "open" || rows[0].Time != live.Time {
		t.Fatalf("port observation changed between event and result: %+v %+v", live, rows)
	}
	stamp := parsedMeasurement(t, rows[0].Time)
	if stamp.Before(before) || stamp.After(after) {
		t.Fatalf("port time %s outside measurement %s..%s", stamp, before, after)
	}
}

func TestMeasurementDiscoveryPreservesPerProbeTimeAcrossFinalCacheEnrichment(t *testing.T) {
	probes := fakeSubnetProbes()
	const pingTime = "2026-10-26T12:34:30Z"
	const portTime = "2026-10-26T12:34:30.5Z"
	probes.ping = func(context.Context, string, time.Duration, int) (platform.PingReply, error) {
		return platform.PingReply{Time: pingTime, Success: true, Status: "reply"}, nil
	}
	probes.tcp = func(_ context.Context, address string, port int, _ time.Duration) PortResult {
		return PortResult{Time: portTime, Port: port, Address: address, State: "open"}
	}
	var live SubnetHost
	out, err := runSubnetScan(context.Background(), model.Request{Tool: "discover", Target: "192.0.2.1", Options: map[string]any{"ports": "443", "identify": false, "resolveHostnames": false}}, func(event model.Event) {
		if data, ok := event.Data.(map[string]any); ok && data["type"] == "host" {
			live = data["value"].(SubnetHost)
		}
	}, probes)
	if err != nil {
		t.Fatal(err)
	}
	row := out.Data.(SubnetResult).Hosts[0]
	if row.Time != portTime || live.Time != row.Time || row.Samples[0].Time != pingTime || row.Ports[0].Time != portTime {
		t.Fatalf("host/probe acquisition association lost: %+v", row)
	}
	parsedMeasurement(t, out.Data.(SubnetResult).Time)
}

func TestMeasurementHTTPRedirectEntriesRetainDistinctResponseTimes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Location", "/next")
			w.WriteHeader(http.StatusFound)
			return
		}
		fmt.Fprint(w, "fixture")
	}))
	defer server.Close()
	var live []HTTPHop
	out, err := httpCheck(context.Background(), model.Request{Target: server.URL, Options: map[string]any{"redirects": true}}, func(event model.Event) {
		if data, ok := event.Data.(map[string]any); ok && data["type"] == "http" {
			live = append(live, data["value"].(HTTPHop))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	rows := out.Data.(map[string]any)["redirects"].([]HTTPHop)
	if len(rows) != 2 || len(live) != 2 {
		t.Fatalf("redirect fixture missing observations: %+v", rows)
	}
	previous := time.Time{}
	for i, row := range rows {
		stamp := parsedMeasurement(t, row.Time)
		if row.Time != live[i].Time || stamp.Before(previous) {
			t.Fatalf("response clock association/order changed: %+v %+v", live, rows)
		}
		previous = stamp
	}
}

func TestMeasurementDNSRecordsKeepTheirOwningQueryClock(t *testing.T) {
	fixture := startSourceDNSFixture(t, false)
	var live []DNSQueryResult
	out, err := dns(context.Background(), model.Request{Target: "clock.fixture.test", Options: map[string]any{"server": fixture.endpoint, "recordType": "ALL"}}, func(event model.Event) {
		if data, ok := event.Data.(map[string]any); ok && data["type"] == "dns" {
			live = append(live, data["value"].(map[string]any)["query"].(DNSQueryResult))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	data := out.Data.(map[string]any)
	queries := data["queries"].([]DNSQueryResult)
	records := data["records"].([]DNSRecord)
	if len(queries) != 11 || len(live) != len(queries) || len(records) != 1 {
		t.Fatalf("DNS fixture did not exercise separate queries: %+v %+v", queries, records)
	}
	byType := map[string]string{}
	previous := time.Time{}
	for i, query := range queries {
		stamp := parsedMeasurement(t, query.Time)
		if stamp.Before(previous) || query.Time != live[i].Time {
			t.Fatalf("query completion changed between live and final: %+v %+v", live, queries)
		}
		previous = stamp
		byType[query.Type] = query.Time
	}
	for _, record := range records {
		if record.Time != byType[record.Type] {
			t.Fatalf("record associated with a different query time: %+v", record)
		}
	}
}

func TestMeasurementResultFinalizationDoesNotRetimestampHistoryOrMutateEventMap(t *testing.T) {
	const existing = "2026-10-26T12:34:30.123456Z"
	legacyRow := map[string]any{"address": "192.0.2.1"}
	published := map[string]any{"rows": []map[string]any{legacyRow}}
	out := recordResultTime(model.Result{Data: published})
	parsedMeasurement(t, out.Time)
	if _, changed := published["time"]; changed {
		t.Fatal("published event snapshot was mutated")
	}
	if _, fabricated := legacyRow["time"]; fabricated {
		t.Fatal("missing historical row timestamp was fabricated")
	}
	if out.Data.(map[string]any)["time"] != out.Time {
		t.Fatal("result finalization metadata differs")
	}
	preserved := recordResultTime(model.Result{Time: existing, Data: map[string]any{"time": existing}})
	if preserved.Time != existing || preserved.Data.(map[string]any)["time"] != existing {
		t.Fatal("existing authoritative time replaced")
	}
}
