package main

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"networktoolkit/internal/capture"
)

func packetGraphOptions(q url.Values) (capture.GraphOptions, bool, error) {
	options := capture.GraphOptions{Recent: time.Minute}
	active := q.Get("graph_mode") != "" || q.Get("graph_start") != "" || q.Get("graph_end") != "" || q.Get("graph_duration_ms") != ""
	if !active {
		return options, false, nil
	}
	if mode := q.Get("graph_mode"); mode != "" && mode != "window" && mode != "progressive" && mode != "condensed" {
		return options, true, fmt.Errorf("choose a supported graph mode")
	}
	options.Condensed = q.Get("graph_mode") == "condensed"
	for _, v := range []struct {
		name   string
		target **time.Time
	}{{"graph_start", &options.Start}, {"graph_end", &options.End}} {
		if value := q.Get(v.name); value != "" {
			t, err := time.Parse(time.RFC3339Nano, value)
			if err != nil || t.IsZero() {
				return options, true, fmt.Errorf("choose valid recorded graph boundaries")
			}
			*v.target = &t
		}
	}
	if options.Start != nil && options.End != nil && options.Start.After(*options.End) {
		return options, true, fmt.Errorf("graph start must not be after its end")
	}
	for _, v := range []struct {
		name    string
		target  *time.Duration
		maximum int64
	}{{"graph_duration_ms", &options.Duration, 315360000000}, {"graph_recent_ms", &options.Recent, 86400000}} {
		if value := q.Get(v.name); value != "" {
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil || n < 1000 || n > v.maximum {
				return options, true, fmt.Errorf("choose a valid graph duration")
			}
			*v.target = time.Duration(n) * time.Millisecond
		}
	}
	return options, true, nil
}
