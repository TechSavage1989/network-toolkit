package main

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"time"

	"networktoolkit/internal/capture"
)

func packetGraphOptions(q url.Values) (capture.GraphOptions, bool, error) {
	options := capture.GraphOptions{Recent: time.Minute}
	active := q.Get("graph_mode") != "" || q.Get("graph_start") != "" || q.Get("graph_end") != "" || q.Get("graph_duration_ms") != "" || q.Get("graph_zoom") != "" || q.Get("graph_view_end") != ""
	if !active {
		return options, false, nil
	}
	if mode := q.Get("graph_mode"); mode != "" && mode != "linear" && mode != "window" && mode != "progressive" && mode != "condensed" {
		return options, true, fmt.Errorf("choose a supported graph mode")
	}
	options.Condensed = q.Get("graph_mode") == "condensed"
	for _, v := range []struct {
		name   string
		target **time.Time
	}{{"graph_start", &options.Start}, {"graph_end", &options.End}, {"graph_view_end", &options.ViewEnd}} {
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
	if value := q.Get("graph_zoom"); value != "" {
		zoom, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsNaN(zoom) || math.IsInf(zoom, 0) || zoom < .25 || zoom > 16 {
			return options, true, fmt.Errorf("choose a supported graph zoom")
		}
		options.Zoom = zoom
	}
	return options, true, nil
}
