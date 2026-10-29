package main

import (
	"strings"
	"sync"
	"time"
)

// Labels are optional presentation metadata. Slow/unavailable devices never hold
// a listing indefinitely, and repeated refreshes cannot create unbounded workers.
type driveLabelEntry struct {
	label   string
	ready   chan struct{}
	expires time.Time
}
type driveLabelCache struct {
	mu      sync.Mutex
	entries map[string]*driveLabelEntry
	slots   chan struct{}
	read    func(string) string
}

func newDriveLabelCache(read func(string) string) *driveLabelCache {
	return &driveLabelCache{entries: map[string]*driveLabelEntry{}, slots: make(chan struct{}, 4), read: read}
}
func (c *driveLabelCache) request(path string) *driveLabelEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := strings.ToLower(path)
	if e := c.entries[key]; e != nil && (e.expires.IsZero() || time.Now().Before(e.expires)) {
		return e
	}
	if len(c.entries) >= 256 {
		for k, old := range c.entries {
			if !old.expires.IsZero() && time.Now().After(old.expires) {
				delete(c.entries, k)
			}
		}
		if len(c.entries) >= 256 {
			return nil
		}
	}
	select {
	case c.slots <- struct{}{}:
	default:
		return nil
	}
	e := &driveLabelEntry{ready: make(chan struct{})}
	c.entries[key] = e
	go func() {
		label := c.read(path)
		c.mu.Lock()
		e.label = label
		e.expires = time.Now().Add(30 * time.Second)
		close(e.ready)
		c.mu.Unlock()
		<-c.slots
	}()
	return e
}
func (c *driveLabelCache) enrich(drives []localTransferDrive, wait time.Duration) {
	entries := make([]*driveLabelEntry, len(drives))
	for i, d := range drives {
		if d.Kind != "network" {
			entries[i] = c.request(d.Path)
		}
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	for _, e := range entries {
		if e != nil {
			select {
			case <-e.ready:
			case <-timer.C:
				goto collect
			}
		}
	}
collect:
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, e := range entries {
		if e != nil && e.label != "" {
			drives[i].Label = e.label
		}
	}
}
