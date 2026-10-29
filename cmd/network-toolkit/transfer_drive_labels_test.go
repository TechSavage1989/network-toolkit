package main

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDriveLabels115CachedBoundedAndNetworkFree(t *testing.T) {
	var calls atomic.Int32
	c := newDriveLabelCache(func(path string) string { calls.Add(1); return "Synthetic label" })
	d := []localTransferDrive{{Path: "C:/", Kind: "local"}, {Path: "Z:/", Kind: "network", Label: "Network drive"}}
	c.enrich(d, 75*time.Millisecond)
	if d[0].Label != "Synthetic label" || d[1].Label != "Network drive" || calls.Load() != 1 {
		t.Fatalf("bad labels/calls %#v %d", d, calls.Load())
	}
	c.enrich(d, 75*time.Millisecond)
	if calls.Load() != 1 {
		t.Fatal("repeated inventory reprobed cached label")
	}
}
func TestDriveLabels115UnavailableDoesNotBlockOrGrowWorkers(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	var calls atomic.Int32
	c := newDriveLabelCache(func(string) string { calls.Add(1); <-release; return "late" })
	d := make([]localTransferDrive, 100)
	for i := range d {
		d[i] = localTransferDrive{Path: string(rune('a' + i)), Kind: "removable", Label: "Removable drive"}
	}
	start := time.Now()
	for range 3 {
		c.enrich(d, 10*time.Millisecond)
	}
	if time.Since(start) > time.Second {
		t.Fatal("listing waited on unavailable volume")
	}
	if calls.Load() > 4 || len(c.entries) > 4 {
		t.Fatalf("unbounded pending calls %d entries %d", calls.Load(), len(c.entries))
	}
	for _, v := range d {
		if v.Label != "Removable drive" {
			t.Fatal("fallback lost")
		}
	}
}
