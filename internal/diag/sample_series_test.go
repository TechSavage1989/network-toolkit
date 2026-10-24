package diag

import (
	"context"
	"errors"
	"net"
	"reflect"
	"strconv"
	"testing"
	"time"

	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

func TestContinuousSamplesPassFormerLimitAndRetainLifetimeStatistics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	series := &sampleSeries{}
	var all []platform.PingReply
	emitted := 0
	err := series.collect(ctx, 0, 0, "icmp", 0, func(event model.Event) {
		emitted++
		data := event.Data.(map[string]any)
		reply := data["value"].(platform.PingReply)
		if _, err := time.Parse(time.RFC3339Nano, reply.Time); err != nil {
			t.Fatal("sample observation time missing", err)
		}
		all = append(all, reply)
		if emitted == MaxRetainedSamples+25 {
			cancel()
		}
	}, func(context.Context) (platform.PingReply, error) {
		return platform.PingReply{Success: emitted%4 != 0, RTTMs: float64(emitted % 31), Status: "reply"}, nil
	})
	if !errors.Is(err, context.Canceled) || emitted != MaxRetainedSamples+25 || len(series.rows) != MaxRetainedSamples {
		t.Fatalf("continuous collection stopped or grew without a bound: %v %d %d", err, emitted, len(series.rows))
	}
	if !reflect.DeepEqual(series.statistics(), statistics(all)) {
		t.Fatalf("lifetime statistics lost older samples: %+v vs %+v", series.statistics(), statistics(all))
	}
	rows := series.samples()
	if rows[0].Sequence != 26 || rows[len(rows)-1].Sequence != emitted {
		t.Fatal("retained sample order is incorrect")
	}
}

func TestFiniteSampleCountAndCancellationRemainExact(t *testing.T) {
	series := &sampleSeries{}
	if err := series.collect(context.Background(), 3, 0, "tcp", 443, nil, func(context.Context) (platform.PingReply, error) { return platform.PingReply{Success: true}, nil }); err != nil || series.statistics().Sent != 3 {
		t.Fatal("finite sample count changed", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := series.collect(ctx, 0, time.Hour, "icmp", 0, nil, func(context.Context) (platform.PingReply, error) {
		t.Fatal("cancelled collection sent a probe")
		return platform.PingReply{}, nil
	}); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled collection did not stop", err)
	}
}

func TestUnlimitedTCPMonitorLoopbackStopsWithCompleteResult(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	_, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	count := 0
	result, err := monitor(ctx, model.Request{Target: "127.0.0.1", Options: map[string]any{"mode": "tcp", "count": 0, "port": port, "intervalMs": 500}}, func(event model.Event) {
		if data, ok := event.Data.(map[string]any); ok && data["type"] == "sample" {
			count++
			if count == 3 {
				cancel()
			}
		}
	})
	data := result.Data.(map[string]any)
	stats := data["statistics"].(Statistics)
	if !errors.Is(err, context.Canceled) || stats.Sent != 3 || stats.Received != 3 || data["continuous"] != true || len(data["samples"].([]platform.PingReply)) != 3 {
		t.Fatalf("unlimited monitor did not stop with its final samples: %v %#v", err, data)
	}
}

func TestContinuousSamplingRetainsProbeErrorsAndRecovers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	series := &sampleSeries{}
	err := series.collect(ctx, 0, 0, "icmp", 0, func(model.Event) {
		if series.sent == 3 {
			cancel()
		}
	}, func(context.Context) (platform.PingReply, error) {
		if series.sent == 0 {
			return platform.PingReply{}, errors.New("temporary network error")
		}
		return platform.PingReply{Success: true, RTTMs: 1}, nil
	})
	if !errors.Is(err, context.Canceled) || series.sent != 3 || series.received != 2 {
		t.Fatal("continuous sampling stopped after a recoverable probe failure", err)
	}
}
