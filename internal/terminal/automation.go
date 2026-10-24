package terminal

import (
	"context"
	"errors"
	"io"
	"sort"
	"sync"
)

// AutomationTarget identifies a particular live transport, rather than merely a
// tab. Generation changes when reconnect replaces that transport.
type AutomationTarget struct {
	ID         string `json:"id"`
	Generation uint64 `json:"generation"`
	Label      string `json:"label"`
	Type       string `json:"type"`
	Host       string `json:"host,omitempty"`
	Username   string `json:"username,omitempty"`
	ProfileID  string `json:"profileId,omitempty"`
	LineEnding string `json:"lineEnding"`
}

var ErrTransportChanged = errors.New("terminal disconnected or reconnected; the original connection is no longer available")

func (m *Manager) AutomationTargets() []AutomationTarget {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]AutomationTarget, 0, m.live)
	for _, s := range m.sessions {
		if s.finished || s.reconnecting || s.info.State != "connected" || s.conn == nil {
			continue
		}
		out = append(out, AutomationTarget{ID: s.info.ID, Generation: s.generation, Label: s.info.Label, Type: s.info.Type, Host: s.info.Host, Username: s.info.Username, ProfileID: s.info.ProfileID, LineEnding: s.info.LineEnding})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Label < out[j].Label || out[i].Label == out[j].Label && out[i].ID < out[j].ID
	})
	return out
}

// SetLineEnding changes the live serial input preference and its retained
// reconnect settings. Transport identity and the active session stay intact.
func (m *Manager) SetLineEnding(id, value string) error {
	switch value {
	case "cr", "lf", "crlf":
	default:
		return errors.New("line ending must be cr, lf, or crlf")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	s := m.sessions[id]
	if s == nil {
		return ErrNotFound
	}
	if s.reconnecting {
		return ErrBusy
	}
	if s.info.Type != "serial" {
		return errors.New("line ending can only be changed for a serial session")
	}
	if s.info.LineEnding == value && s.request.LineEnding == value {
		return nil
	}
	s.info.LineEnding = value
	s.request.LineEnding = value
	m.statusLocked(s, "Serial line ending updated")
	return nil
}

// AutomationWrite only queues input for the generation selected by the user.
func (m *Manager) AutomationWrite(id string, generation uint64, data []byte) error {
	_, err := m.AutomationWriteCursor(id, generation, data)
	return err
}

// AutomationWriteCursor returns the last output sequence before queuing input.
// Wait steps can therefore distinguish fresh command output from an old prompt,
// even when delivery of an older output subscription event was delayed.
func (m *Manager) AutomationWriteCursor(id string, generation uint64, data []byte) (uint64, error) {
	if len(data) > maxQueuedInput {
		return 0, errors.New("automation input must not exceed 256 KiB per step")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s, err := m.connectedLocked(id)
	if err != nil {
		return 0, err
	}
	if s.generation != generation || s.reconnecting {
		return 0, ErrTransportChanged
	}
	cursor := m.seq
	if len(data) == 0 {
		return cursor, nil
	}
	chunks := (len(data) + maxInputChunk - 1) / maxInputChunk
	if s.queuedInput+len(data) > maxQueuedInput || len(s.input)+chunks > cap(s.input) {
		return 0, ErrQueueFull
	}
	for len(data) > 0 {
		n := min(len(data), maxInputChunk)
		copy := append([]byte(nil), data[:n]...)
		s.queuedInput += len(copy)
		s.input <- copy
		data = data[n:]
	}
	return cursor, nil
}

type AutomationOutput struct {
	Seq  uint64
	Data []byte
}

// AutomationSubscription retains every fresh output chunk until it is read.
// Unlike the UI event ring, it cannot silently discard long command output.
// Its queue uses a private mutex and never invokes a subscriber callback while
// the manager mutex is held. Consumers must Close after use.
type AutomationSubscription struct {
	manager *Manager
	session *liveSession
	mu      sync.Mutex
	queue   []AutomationOutput
	head    int
	ended   bool
	changed chan struct{}
}

func (m *Manager) AutomationSubscribe(id string, generation uint64) (*AutomationSubscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, err := m.connectedLocked(id)
	if err != nil {
		return nil, err
	}
	if s.generation != generation || s.reconnecting {
		return nil, ErrTransportChanged
	}
	sub := &AutomationSubscription{manager: m, session: s, changed: make(chan struct{})}
	s.automation[sub] = struct{}{}
	return sub, nil
}

func (s *AutomationSubscription) push(output AutomationOutput) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	output.Data = append([]byte(nil), output.Data...)
	s.queue = append(s.queue, output)
	close(s.changed)
	s.changed = make(chan struct{})
}
func (s *AutomationSubscription) finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ended {
		s.ended = true
		close(s.changed)
	}
}
func (s *AutomationSubscription) Read(ctx context.Context) (AutomationOutput, error) {
	for {
		s.mu.Lock()
		if s.head < len(s.queue) {
			out := s.queue[s.head]
			s.queue[s.head] = AutomationOutput{}
			s.head++
			if s.head == len(s.queue) {
				s.queue = nil
				s.head = 0
			} else if s.head > 1024 && s.head*2 > len(s.queue) {
				s.queue = append([]AutomationOutput(nil), s.queue[s.head:]...)
				s.head = 0
			}
			s.mu.Unlock()
			return out, nil
		}
		ended, changed := s.ended, s.changed
		s.mu.Unlock()
		if ended {
			return AutomationOutput{}, io.EOF
		}
		select {
		case <-ctx.Done():
			return AutomationOutput{}, ctx.Err()
		case <-changed:
		}
	}
}
func (s *AutomationSubscription) Close() {
	s.manager.mu.Lock()
	delete(s.session.automation, s)
	s.finish()
	s.manager.mu.Unlock()
}
