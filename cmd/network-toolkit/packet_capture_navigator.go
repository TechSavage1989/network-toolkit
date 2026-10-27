package main

import (
	"errors"
	"networktoolkit/internal/capture"
	"sort"
	"time"
)

type packetGraphView struct {
	Overview capture.Navigator
	Timed    []capture.Packet
}
type packetGraphKey struct{ expression, query, protocol string }

type packetNavigatorCache struct {
	revision uint64
	views    map[packetGraphKey]packetGraphView
}

// Cache immutable chronological contributors by recording revision and filters.
// Viewport navigation reuses this index; it does not copy or analyze the whole
// recording on each pointer movement. Keep at most two display-filter views.
func (m *packetCaptureManager) graphView(id, expression, query, protocol string) (packetGraphView, error) {
	key := packetGraphKey{expression, query, protocol}
	m.mu.RLock()
	session := m.sessions[id]
	if session == nil {
		m.mu.RUnlock()
		return packetGraphView{}, errors.New("Capture not found.")
	}
	revision := session.info.Revision
	if session.navigator.revision == revision {
		if cached, ok := session.navigator.views[key]; ok {
			m.mu.RUnlock()
			return cached, nil
		}
	}
	m.mu.RUnlock()
	info, packets, err := m.snapshot(id)
	if err != nil {
		return packetGraphView{}, err
	}
	matching, err := filterPackets(packets, expression, query, protocol)
	if err != nil {
		return packetGraphView{}, err
	}
	result := packetGraphView{Overview: capture.TimelineNavigator(packets, matching)}
	result.Overview.Revision = info.Revision
	for _, p := range matching {
		if !p.Time.IsZero() {
			result.Timed = append(result.Timed, p)
		}
	}
	sort.SliceStable(result.Timed, func(i, j int) bool { return result.Timed[i].Time.Before(result.Timed[j].Time) })
	m.mu.Lock()
	defer m.mu.Unlock()
	session = m.sessions[id]
	if session != nil && session.info.Revision == info.Revision {
		if session.navigator.revision != info.Revision || len(session.navigator.views) >= 2 {
			session.navigator = packetNavigatorCache{revision: info.Revision, views: map[packetGraphKey]packetGraphView{}}
		}
		session.navigator.views[key] = result
	}
	return result, nil
}
func (m *packetCaptureManager) navigator(id, expression, query, protocol string) (capture.Navigator, error) {
	v, err := m.graphView(id, expression, query, protocol)
	return v.Overview, err
}

type packetViewport struct {
	Revision uint64           `json:"revision,omitempty"`
	Timeline []capture.Bucket `json:"timeline"`
	Start    *time.Time       `json:"graphStarted,omitempty"`
	End      *time.Time       `json:"graphFinished,omitempty"`
	Note     string           `json:"graphNote"`
}

func (m *packetCaptureManager) graphViewport(id, expression, query, protocol string, options capture.GraphOptions, revision uint64) (packetViewport, error) {
	// A gesture uses the overview's immutable recording revision. Live arrivals
	// do not rebuild the full index for every movement. The regular overview
	// refresh advances the snapshot and then subsequent viewport requests.
	var v packetGraphView
	var err error
	found := false
	if revision > 0 {
		m.mu.RLock()
		if session := m.sessions[id]; session != nil && session.navigator.revision == revision {
			v, found = session.navigator.views[packetGraphKey{expression, query, protocol}]
		}
		m.mu.RUnlock()
	}
	if !found {
		v, err = m.graphView(id, expression, query, protocol)
	}
	if err != nil {
		return packetViewport{}, err
	}
	if options.Start == nil || options.End == nil {
		return packetViewport{}, errors.New("Choose both displayed graph boundaries.")
	}
	// Explicit bounds are the actual viewport, so duration/zoom must not apply twice.
	options.Zoom = 1
	options.Duration = 0
	options.ViewEnd = nil
	first := sort.Search(len(v.Timed), func(i int) bool { return !v.Timed[i].Time.Before(*options.Start) })
	last := sort.Search(len(v.Timed), func(i int) bool { return v.Timed[i].Time.After(*options.End) })
	s := capture.Statistics{}
	capture.ApplyGraph(&s, v.Timed[first:last], options)
	return packetViewport{Revision: v.Overview.Revision, Timeline: s.Timeline, Start: options.Start, End: options.End, Note: s.GraphNote}, nil
}
