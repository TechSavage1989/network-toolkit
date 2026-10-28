package main

import (
	"context"
	"net/http"
	"sync"
)

// Activity is deliberately a counts-only projection of live backend ownership.
// Never return connection metadata, job input, credentials, or historical results.
// Components are sampled under their own locks, without network or helper calls.
type activityCounts struct {
	Connections int `json:"connections,omitempty"`
	Servers     int `json:"servers,omitempty"`
	Operations  int `json:"operations,omitempty"`
	Captures    int `json:"captures,omitempty"`
}

type activitySnapshot struct {
	Tools map[string]activityCounts `json:"tools"`
}

func activityPage(tool string) string {
	switch tool {
	case "quick", "ping", "trace", "dns", "smtp", "tcp", "http", "tls", "identify", "discover", "monitor", "wol", "terminal", "sftp", "tftp", "capture", "hosts":
		return tool
	case "ftp", "ftps", "ssh":
		return "sftp"
	case "ip", "ipconfig", "routes", "arp", "connections", "wifi":
		return "local"
	default:
		return ""
	}
}

func (s *activitySnapshot) add(page string, counts activityCounts) {
	if page == "" || counts == (activityCounts{}) {
		return
	}
	previous := s.Tools[page]
	previous.Connections += counts.Connections
	previous.Servers += counts.Servers
	previous.Operations += counts.Operations
	previous.Captures += counts.Captures
	s.Tools[page] = previous
}

// Short-lived file operations (including FTP/FTPS) have no idle retained socket.
// Track only dispatch lifetime; merely reading a cached file listing is inactive.
type activityOperation struct {
	ctx  context.Context
	page string
}

type activityOperations struct {
	mu      sync.Mutex
	entries map[*activityOperation]struct{}
}

func (m *activityOperations) begin(ctx context.Context, tool string) func() {
	page := activityPage(tool)
	if page == "" || ctx.Err() != nil {
		return func() {}
	}
	op := &activityOperation{ctx: ctx, page: page}
	m.mu.Lock()
	if m.entries == nil {
		m.entries = make(map[*activityOperation]struct{})
	}
	m.entries[op] = struct{}{}
	m.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			m.mu.Lock()
			delete(m.entries, op)
			m.mu.Unlock()
		})
	}
}

func (m *activityOperations) appendTo(snapshot *activitySnapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for op := range m.entries {
		if op.ctx.Err() == nil {
			snapshot.add(op.page, activityCounts{Operations: 1})
		}
	}
}

func (a *application) activitySnapshot() activitySnapshot {
	snapshot := activitySnapshot{Tools: make(map[string]activityCounts)}
	if a.shutdownRequested.Load() || (a.ctx != nil && a.ctx.Err() != nil) {
		return snapshot
	}
	if a.terminals != nil {
		snapshot.add("terminal", activityCounts{Connections: a.terminals.ConnectedCount()})
	}
	a.browserMu.Lock()
	for _, entry := range a.browserConnections {
		if entry == nil || entry.client == nil {
			continue
		}
		select {
		case <-entry.client.Done():
		default:
			snapshot.add("sftp", activityCounts{Connections: 1})
		}
	}
	a.browserMu.Unlock()
	a.mu.RLock()
	for _, job := range a.jobs {
		if job != nil && job.Status == "running" && (job.activityContext == nil || job.activityContext.Err() == nil) {
			snapshot.add(activityPage(job.Tool), activityCounts{Operations: 1})
		}
	}
	a.mu.RUnlock()
	a.activity.appendTo(&snapshot)
	if m := a.packetCaptures; m != nil {
		m.mu.RLock()
		if !m.closed && m.ctx.Err() == nil {
			for _, session := range m.sessions {
				if session != nil && session.info.State == "running" && session.info.Engine != "import" {
					snapshot.add("capture", activityCounts{Captures: 1})
				}
			}
		}
		m.mu.RUnlock()
	}
	if m := a.transfers; m != nil {
		m.mu.Lock()
		if m.ctx.Err() == nil {
			for _, server := range m.servers {
				if server != nil && server.listener != nil && server.isServing() {
					snapshot.add(activityPage(server.proto), activityCounts{Servers: 1})
				}
			}
		}
		m.mu.Unlock()
	}
	if m := a.localTools; m != nil {
		m.mu.Lock()
		if m.ctx.Err() == nil {
			for _, op := range m.operations {
				if op.Status == "running" {
					snapshot.add(activityPage(op.Kind), activityCounts{Operations: 1})
				}
			}
		}
		m.mu.Unlock()
	}
	return snapshot
}

func (a *application) handleActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodError(w)
		return
	}
	jsonResponse(w, http.StatusOK, a.activitySnapshot())
}
