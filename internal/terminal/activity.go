package terminal

// ConnectedCount includes all owned live transports, including detached-window
// sessions. It reads state only: no dial, keepalive, authentication or event drain.
func (m *Manager) ConnectedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.ctx.Err() != nil {
		return 0
	}
	count := 0
	for _, session := range m.sessions {
		if session.info.State == "connected" && session.conn != nil &&
			!session.finished && !session.disconnected && session.ctx.Err() == nil {
			count++
		}
	}
	return count
}
