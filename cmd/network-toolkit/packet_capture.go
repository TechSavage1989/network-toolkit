package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"networktoolkit/internal/capture"
)

type packetCaptureCapabilities struct {
	Available  bool                `json:"available"`
	Live       bool                `json:"live"`
	File       bool                `json:"file"`
	Elevated   bool                `json:"elevated"`
	Note       string              `json:"note"`
	Interfaces []capture.Interface `json:"interfaces"`
}
type packetCaptureMessage struct {
	Version    int                 `json:"version"`
	Type       string              `json:"type"`
	Engine     string              `json:"engine,omitempty"`
	Note       string              `json:"note,omitempty"`
	Error      string              `json:"error,omitempty"`
	Packets    []capture.Record    `json:"packets,omitempty"`
	Lost       uint64              `json:"lost,omitempty"`
	Interfaces []capture.Interface `json:"interfaces,omitempty"`
}
type packetCaptureRequest struct {
	Version   int             `json:"version"`
	Operation string          `json:"operation"`
	Options   capture.Options `json:"options"`
}
type packetCaptureInfo struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	State       string          `json:"state"`
	Engine      string          `json:"engine"`
	Started     time.Time       `json:"started"`
	Finished    *time.Time      `json:"finished,omitempty"`
	Options     capture.Options `json:"options"`
	Count       int             `json:"count"`
	Bytes       int             `json:"bytes"`
	Observed    uint64          `json:"observed"`
	Lost        uint64          `json:"lost"`
	Overwritten uint64          `json:"overwritten"`
	Revision    uint64          `json:"revision"`
	Note        string          `json:"note,omitempty"`
	Error       string          `json:"error,omitempty"`
}
type packetCaptureSession struct {
	info    packetCaptureInfo
	packets []*capture.Packet
	stop    chan struct{}
	once    sync.Once
	done    chan struct{}
	first   time.Time
	next    uint64
}
type packetCaptureManager struct {
	ctx      context.Context
	mu       sync.RWMutex
	sessions map[string]*packetCaptureSession
	order    []string
	run      func(context.Context, capture.Options, <-chan struct{}, func(packetCaptureMessage) error) error
	workers  sync.WaitGroup
	closed   bool
}

func newPacketCaptureManager(ctx context.Context) *packetCaptureManager {
	return &packetCaptureManager{ctx: ctx, sessions: map[string]*packetCaptureSession{}, run: packetCaptureRun}
}
func (m *packetCaptureManager) close() {
	m.mu.Lock()
	m.closed = true
	for _, s := range m.sessions {
		s.once.Do(func() { close(s.stop) })
	}
	m.mu.Unlock()
	done := make(chan struct{})
	go func() { m.workers.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
}
func (m *packetCaptureManager) list() []packetCaptureInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []packetCaptureInfo{}
	for i := len(m.order) - 1; i >= 0; i-- {
		if s := m.sessions[m.order[i]]; s != nil {
			out = append(out, s.info)
		}
	}
	return out
}
func captureActive(state string) bool {
	return state == "starting" || state == "running" || state == "stopping"
}
func (m *packetCaptureManager) register(name string, o capture.Options, engine string) (*packetCaptureSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.ctx.Err() != nil {
		return nil, errors.New("The application is closing.")
	}
	if engine != "import" {
		for _, s := range m.sessions {
			if captureActive(s.info.State) {
				return nil, errors.New("Stop the current packet capture before starting another.")
			}
		}
	}
	for len(m.order) >= 3 {
		removed := false
		for i, id := range m.order {
			if s := m.sessions[id]; s != nil && !captureActive(s.info.State) {
				delete(m.sessions, id)
				m.order = append(m.order[:i], m.order[i+1:]...)
				removed = true
				break
			}
		}
		if !removed {
			return nil, errors.New("All retained captures are active.")
		}
	}
	s := &packetCaptureSession{info: packetCaptureInfo{ID: randomHex(12), Name: name, State: "starting", Engine: engine, Started: time.Now().UTC(), Options: o, Revision: 1}, stop: make(chan struct{}), done: make(chan struct{})}
	m.sessions[s.info.ID] = s
	m.order = append(m.order, s.info.ID)
	if engine != "import" {
		m.workers.Add(1)
	}
	return s, nil
}
func (m *packetCaptureManager) start(o capture.Options) (packetCaptureInfo, error) {
	if e := o.Normalize(); e != nil {
		return packetCaptureInfo{}, e
	}
	s, e := m.register("Packet capture", o, o.Engine)
	if e != nil {
		return packetCaptureInfo{}, e
	}
	go func() {
		defer m.workers.Done()
		defer close(s.done)
		ctx, cancel := context.WithTimeout(m.ctx, time.Duration(o.Duration+90)*time.Second)
		defer cancel()
		e := m.run(ctx, o, s.stop, func(message packetCaptureMessage) error { return m.event(s, message) })
		m.mu.Lock()
		defer m.mu.Unlock()
		now := time.Now().UTC()
		s.info.Finished = &now
		if e != nil {
			s.info.State = "failed"
			s.info.Error = e.Error()
		} else {
			s.info.State = "completed"
		}
		s.info.Revision++
	}()
	m.mu.RLock()
	info := s.info
	m.mu.RUnlock()
	return info, nil
}
func (m *packetCaptureManager) event(s *packetCaptureSession, message packetCaptureMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch message.Type {
	case "ready":
		if s.info.State != "stopping" {
			s.info.State = "running"
		}
		s.info.Engine = message.Engine
		s.info.Note = message.Note
	case "status":
		s.info.Note = message.Note
	case "error":
		return errors.New(message.Error)
	case "packets":
		for _, r := range message.Packets {
			if len(r.Data) > capture.MaxPacketBytes || r.OriginalLength < uint32(len(r.Data)) || len(r.Interface) > 512 {
				return errors.New("The packet capture backend returned invalid packet data.")
			}
			s.info.Observed++
			p := capture.Decode(r)
			_, p.Highlight, _, _ = packetClue(p)
			if !s.info.Options.Filter.Match(p) {
				continue
			}
			// Keep summaries and original bytes. Decode the field tree on demand
			// instead of retaining thousands of strings for every packet.
			p.Layers = nil
			if len(p.Info) > 2048 {
				p.Info = strings.ToValidUTF8(p.Info[:2048], "") + "..."
			}
			if s.first.IsZero() && !p.Time.IsZero() {
				s.first = p.Time
			}
			if !p.Time.IsZero() && !s.first.IsZero() {
				p.Relative = p.Time.Sub(s.first).Seconds()
			}
			s.next++
			p.Number = s.next
			limit := s.info.Options.MaxMB << 20
			if len(p.Record.Data) > limit {
				s.info.Lost++
				continue
			}
			if s.info.Options.Ring {
				for len(s.packets) > 0 && (len(s.packets) >= s.info.Options.MaxPackets || s.info.Bytes+len(p.Record.Data) > limit) {
					s.info.Bytes -= s.packets[0].Captured
					s.packets[0] = nil
					s.packets = s.packets[1:]
					s.info.Overwritten++
				}
			} else if len(s.packets) >= s.info.Options.MaxPackets || s.info.Bytes+len(p.Record.Data) > limit {
				s.info.State = "stopping"
				s.info.Note = "Stopped at the configured packet or memory limit."
				s.once.Do(func() { close(s.stop) })
				break
			}
			s.packets = append(s.packets, &p)
			s.info.Bytes += p.Captured
		}
		s.info.Count = len(s.packets)
	}
	s.info.Lost += message.Lost
	s.info.Revision++
	return nil
}
func (m *packetCaptureManager) importFile(name string, r io.Reader) (packetCaptureInfo, error) {
	o := capture.Options{MaxMB: 128, MaxPackets: capture.MaxPackets}
	_ = o.Normalize()
	staged := []capture.Record{}
	bytes := 0
	e := capture.Read(r, func(record capture.Record) error {
		bytes += len(record.Data)
		if bytes > capture.MaxFileBytes {
			return errors.New("The imported packet data exceeds 128 MiB.")
		}
		staged = append(staged, record)
		return nil
	})
	if e != nil {
		return packetCaptureInfo{}, e
	}
	s, e := m.register(name, o, "import")
	if e != nil {
		return packetCaptureInfo{}, e
	}
	if e = m.event(s, packetCaptureMessage{Type: "packets", Packets: staged}); e != nil {
		m.mu.Lock()
		s.info.State = "failed"
		close(s.done)
		m.mu.Unlock()
		_ = m.remove(s.info.ID)
		return packetCaptureInfo{}, e
	}
	m.mu.Lock()
	s.info.State = "completed"
	now := time.Now().UTC()
	s.info.Finished = &now
	info := s.info
	close(s.done)
	m.mu.Unlock()
	return info, nil
}
func (m *packetCaptureManager) stopCapture(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[id]
	if s == nil {
		return errors.New("Capture not found.")
	}
	if captureActive(s.info.State) {
		s.info.State = "stopping"
		s.once.Do(func() { close(s.stop) })
		s.info.Revision++
	}
	return nil
}
func (m *packetCaptureManager) remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[id]
	if s == nil {
		return errors.New("Capture not found.")
	}
	if captureActive(s.info.State) {
		return errors.New("Stop this capture before clearing it.")
	}
	delete(m.sessions, id)
	for i, v := range m.order {
		if v == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	return nil
}

// windowCapture copies only the requested page, not the entire retained capture.
// It is safe for concurrent live capture; packet summaries are immutable after append.
func (m *packetCaptureManager) windowCapture(id string, offset, limit int) (packetCaptureInfo, int, int, []capture.Packet, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session := m.sessions[id]
	if session == nil {
		return packetCaptureInfo{}, 0, 0, nil, errors.New("Capture not found.")
	}
	total := len(session.packets)
	offset = min(total, max(0, offset))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	end := min(total, offset+limit)
	page := make([]capture.Packet, 0, end-offset)
	for _, pointer := range session.packets[offset:end] {
		page = append(page, *pointer)
	}
	return session.info, total, offset, page, nil
}

func (m *packetCaptureManager) snapshot(id string) (packetCaptureInfo, []capture.Packet, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := m.sessions[id]
	if s == nil {
		return packetCaptureInfo{}, nil, errors.New("Capture not found.")
	}
	packets := make([]capture.Packet, len(s.packets))
	for i, p := range s.packets {
		packets[i] = *p
	}
	return s.info, packets, nil
}
func filterPackets(packets []capture.Packet, expression, query, protocol string) ([]capture.Packet, error) {
	match, e := capture.Compile(expression)
	if e != nil {
		return nil, e
	}
	if len(query) > 1024 {
		return nil, errors.New("Search text is limited to 1024 characters.")
	}
	query = strings.ToLower(query)
	out := make([]capture.Packet, 0, len(packets))
	for _, p := range packets {
		if protocol != "" && protocol != "any" && !strings.EqualFold(p.Protocol, protocol) {
			continue
		}
		if !match(p) {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(fmt.Sprintf("%d %s %s %d %d %s %s %s %s %s", p.Number, p.Source, p.Destination, p.SourcePort, p.DestinationPort, p.Protocol, p.Info, p.Interface, p.SrcMAC, p.DstMAC)), query) {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}
func sortPackets(packets []capture.Packet, field, direction string) {
	// Packet numbering follows the retained capture order. The common default
	// does not need a full stable sort for every continuous-scroll page.
	if field == "" || field == "number" {
		if direction == "desc" {
			for i, j := 0, len(packets)-1; i < j; i, j = i+1, j-1 {
				packets[i], packets[j] = packets[j], packets[i]
			}
		}
		return
	}
	less := func(i, j int) bool {
		a, b := packets[i], packets[j]
		switch field {
		case "source":
			return a.Source < b.Source
		case "destination":
			return a.Destination < b.Destination
		case "protocol":
			return a.Protocol < b.Protocol
		case "length":
			return a.Length < b.Length
		case "time":
			if !a.Time.Equal(b.Time) {
				return a.Time.Before(b.Time)
			}
		}
		return a.Number < b.Number
	}
	if direction == "desc" {
		sort.SliceStable(packets, func(i, j int) bool { return less(j, i) })
	} else {
		sort.SliceStable(packets, less)
	}
}
