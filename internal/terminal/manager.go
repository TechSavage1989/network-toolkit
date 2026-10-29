package terminal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"networktoolkit/internal/diag"
	"networktoolkit/internal/model"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/crypto/ssh"
)

var (
	ErrNotFound               = errors.New("terminal session not found")
	ErrBusy                   = errors.New("terminal session is busy")
	ErrQueueFull              = errors.New("terminal input queue is full; retry without discarding input")
	ErrLimit                  = errors.New("at most eight terminal sessions may be active or shutting down")
	ErrClosed                 = errors.New("terminal manager is closed")
	ErrNotConnected           = errors.New("terminal session is not connected")
	ErrAuthenticationRequired = errors.New("enter authentication again to reconnect; connection secrets are not retained")
	ErrCredentialUnavailable  = errors.New("the saved credential is unavailable for reconnect")
)

type storedEvent struct {
	event Event
	size  int
}

type liveSession struct {
	info            SessionInfo
	request         ConnectRequest // Nonsecret settings and optional credential ID only.
	done            chan struct{}
	reconnecting    bool
	reconnectCancel context.CancelFunc
	ctx             context.Context
	cancel          context.CancelFunc
	conn            transport
	input           chan []byte
	resize          chan struct{}
	controls        chan ControlRequest
	queuedInput     int
	pending         chan PromptReply
	replySent       bool
	finished        bool
	disconnected    bool
	generation      uint64
	automation      map[*AutomationSubscription]struct{}
}

type Manager struct {
	mu                 sync.Mutex
	ctx                context.Context
	cancel             context.CancelFunc
	sessions           map[string]*liveSession
	openers            map[string]transportOpener
	resolveCredential  CredentialResolver
	elevatedLocalShell ElevatedLocalShellOpener
	known              *knownHostStore
	closed             bool
	live               int
	wg                 sync.WaitGroup
	ring               []storedEvent
	ringStart          int
	ringBytes          int
	eventBudget        int
	seq                uint64
	generation         uint64
	changed            chan struct{}
	closeDone          chan struct{}
}

// NewManager always returns a usable manager. A trust-file error disables SSH
// until that file is deliberately repaired, while Telnet and serial remain
// available. Failed loading never resets or overwrites remembered host keys.
func NewManager(parent context.Context, dataDir string) (*Manager, error) {
	known, err := loadKnownHostStore(dataDir)
	m := newManager(parent, known, nil)
	m.openers = map[string]transportOpener{
		"ssh": func(ctx context.Context, req ConnectRequest, cb transportCallbacks) (transport, error) {
			return openSSH(ctx, req, cb, known)
		},
		"telnet":     openTelnet,
		"serial":     openSerial,
		"cmd":        m.openManagedLocalShell,
		"powershell": m.openManagedLocalShell,
	}
	return m, err
}

func newManager(parent context.Context, known *knownHostStore, openers map[string]transportOpener) *Manager {
	ctx, cancel := context.WithCancel(parent)
	m := &Manager{ctx: ctx, cancel: cancel, sessions: map[string]*liveSession{}, openers: openers,
		known: known, eventBudget: maxEventBytes, changed: make(chan struct{}), closeDone: make(chan struct{})}
	go func() { <-ctx.Done(); _ = m.Close() }()
	return m
}

func normalizeRequest(req ConnectRequest) (ConnectRequest, error) {
	return normalizeConnectionRequest(req, false)
}

// alreadyAuthenticated is used only when adopting an existing SSH transport.
// Opening a new connection must still provide all required authentication.
func normalizeConnectionRequest(req ConnectRequest, alreadyAuthenticated bool) (ConnectRequest, error) {
	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if len([]rune(req.DisplayName)) > 100 || strings.IndexFunc(req.DisplayName, unicode.IsControl) >= 0 {
		return req, errors.New("display name must be at most 100 characters without control characters")
	}
	if len(req.ProfileID) > 80 || strings.IndexFunc(req.ProfileID, unicode.IsControl) >= 0 {
		return req, errors.New("profile ID must be at most 80 bytes without control characters")
	}
	if len(req.CredentialID) > 120 || strings.ContainsAny(req.CredentialID, "/\\") || strings.IndexFunc(req.CredentialID, unicode.IsControl) >= 0 {
		return req, errors.New("credential ID must be at most 120 bytes without path separators or control characters")
	}
	switch req.LineEnding {
	case "":
		if req.Type == "serial" {
			req.LineEnding = "crlf"
		} else {
			req.LineEnding = "cr"
		}
	case "cr", "lf", "crlf":
	default:
		return req, errors.New("line ending must be cr, lf, or crlf")
	}
	if req.Type != "ssh" && req.Type != "telnet" && req.Type != "serial" && req.Type != "cmd" && req.Type != "powershell" {
		return req, errors.New("terminal type must be ssh, telnet, serial, cmd, or powershell")
	}
	if req.Type != "cmd" && req.Type != "powershell" && (req.WorkingDirectory != "" || req.RunAsAdministrator) {
		return req, errors.New("starting directory and administrator mode apply only to local CMD or PowerShell")
	}
	switch req.SSHCompatibility {
	case "":
		req.SSHCompatibility = SSHCompatibilityModern
	case SSHCompatibilityModern, SSHCompatibilityLegacy:
	default:
		return req, errors.New("SSH compatibility must be modern or legacy")
	}
	if req.Type != "ssh" {
		req.SSHCompatibility = SSHCompatibilityModern
	}
	if req.Cols == 0 {
		req.Cols = 80
	}
	if req.Rows == 0 {
		req.Rows = 24
	}
	if err := validDimensions(req.Cols, req.Rows); err != nil {
		return req, err
	}
	if req.Terminal == "" {
		req.Terminal = "xterm-256color"
	}
	if req.Terminal != "xterm-256color" && req.Terminal != "xterm" && req.Terminal != "vt100" {
		return req, errors.New("terminal must be xterm-256color, xterm, or vt100")
	}
	if req.TimeoutMs == 0 {
		req.TimeoutMs = 10000
	}
	if req.TimeoutMs < 1000 || req.TimeoutMs > 30000 {
		return req, errors.New("connection timeout must be between 1000 and 30000 ms")
	}
	if req.Type == "cmd" || req.Type == "powershell" {
		// Local shells never inherit hidden remote credentials or device fields.
		if req.Type == "cmd" && strings.HasPrefix(req.WorkingDirectory, `\\`) {
			return req, errors.New("Command Prompt requires a local drive starting directory, not UNC")
		}
		directory, err := ValidateLocalWorkingDirectory(req.WorkingDirectory)
		if err != nil {
			return req, err
		}
		return ConnectRequest{Type: req.Type, WorkingDirectory: directory, RunAsAdministrator: req.RunAsAdministrator, ProfileID: req.ProfileID, DisplayName: req.DisplayName, Cols: req.Cols, Rows: req.Rows, Terminal: req.Terminal, TimeoutMs: req.TimeoutMs, LineEnding: "cr", SSHCompatibility: SSHCompatibilityModern}, nil
	}
	if req.Type == "serial" {
		req.Username, req.Password, req.PrivateKey, req.Passphrase = "", "", "", ""
		req.CredentialID = ""
		return normalizeSerialRequest(req)
	}
	host, err := normalizeHost(req.Host)
	if err != nil {
		return req, err
	}
	req.Host = host
	if req.Port == 0 {
		if req.Type == "ssh" {
			req.Port = 22
		} else {
			req.Port = 23
		}
	}
	if req.Port < 1 || req.Port > 65535 {
		return req, errors.New("port must be between 1 and 65535")
	}
	if req.Type == "telnet" {
		req.Username, req.Password, req.PrivateKey, req.Passphrase = "", "", "", ""
		req.CredentialID = ""
		return req, nil
	}
	if req.Username == "" || len(req.Username) > 256 || strings.IndexFunc(req.Username, unicode.IsControl) >= 0 {
		return req, errors.New("enter an SSH username of at most 256 bytes without control characters")
	}
	if len(req.Password) > 16*1024 || len(req.Passphrase) > 16*1024 || len(req.PrivateKey) > 256*1024 {
		return req, errors.New("SSH credential input exceeded its size limit")
	}
	if req.AuthMethod == "" {
		if req.PrivateKey != "" {
			req.AuthMethod = "key"
		} else {
			req.AuthMethod = "password"
		}
	}
	if req.AuthMethod != "password" && req.AuthMethod != "key" && req.AuthMethod != "keyboard" {
		return req, errors.New("SSH authentication must be password, key, or keyboard")
	}
	if !alreadyAuthenticated && req.AuthMethod == "key" && strings.TrimSpace(req.PrivateKey) == "" {
		return req, errors.New("select an OpenSSH or PEM private key")
	}
	return req, nil
}

func normalizeHost(raw string) (string, error) {
	host := strings.TrimSpace(raw)
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = host[1 : len(host)-1]
	}
	if address, err := netip.ParseAddr(host); err == nil {
		if address.IsUnspecified() || address.IsMulticast() || address.String() == "255.255.255.255" {
			return "", errors.New("enter a unicast host address")
		}
		return address.Unmap().String(), nil
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "" || len(host) > 253 {
		return "", errors.New("enter a hostname or IP address without a URL or port")
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 {
			return "", errors.New("invalid hostname label")
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
				return "", errors.New("enter an ASCII hostname or IP address without a URL or port")
			}
		}
	}
	return host, nil
}

func validDimensions(cols, rows int) error {
	if cols < 2 || cols > 500 || rows < 2 || rows > 300 {
		return errors.New("terminal dimensions must be 2..500 columns and 2..300 rows")
	}
	return nil
}

func (m *Manager) Connect(request ConnectRequest) (SessionInfo, error) {
	return m.connectWithOpener(request, nil)
}

// SetCredentialResolver configures backend-only saved-credential lookup for
// reconnect. Initial connection requests are resolved by the application before
// Connect. A missing resolver fails closed rather than reusing an old secret.
func (m *Manager) SetCredentialResolver(resolve CredentialResolver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resolveCredential = resolve
}

func (m *Manager) connectWithOpener(request ConnectRequest, override transportOpener) (SessionInfo, error) {
	req, err := normalizeConnectionRequest(request, override != nil)
	if err != nil {
		return SessionInfo{}, err
	}
	if req.Type == "ssh" && m.known != nil {
		if err := m.known.loadError(); err != nil {
			return SessionInfo{}, err
		}
	}
	id, err := randomID()
	if err != nil {
		return SessionInfo{}, err
	}
	m.mu.Lock()
	if m.closed || m.ctx.Err() != nil {
		m.mu.Unlock()
		return SessionInfo{}, ErrClosed
	}
	if m.live >= maxLiveSessions {
		m.mu.Unlock()
		return SessionInfo{}, ErrLimit
	}
	opener := m.openers[req.Type]
	if override != nil {
		opener = override
	}
	if opener == nil {
		m.mu.Unlock()
		return SessionInfo{}, errors.New("terminal transport is unavailable")
	}
	if len(m.sessions) >= maxSessionHistory {
		var oldest *liveSession
		for _, candidate := range m.sessions {
			if candidate.finished && !candidate.reconnecting && (oldest == nil || candidate.info.CreatedAt < oldest.info.CreatedAt) {
				oldest = candidate
			}
		}
		if oldest == nil {
			m.mu.Unlock()
			return SessionInfo{}, ErrLimit
		}
		removed := cloneSession(oldest.info)
		removed.State = "removed"
		m.appendLocked(Event{SessionID: oldest.info.ID, Type: "status", Session: &removed, Message: "Oldest closed session removed from retained history"})
		oldest.request = ConnectRequest{}
		delete(m.sessions, oldest.info.ID)
	}
	s := newLiveSession(m.ctx, id, req)
	m.generation++
	s.generation = m.generation
	m.sessions[id] = s
	m.live++
	m.statusLocked(s, "Connecting")
	info := cloneSession(s.info)
	m.wg.Add(1)
	m.mu.Unlock()
	go m.runSession(s, req, opener)
	return info, nil
}

func newLiveSession(parent context.Context, id string, req ConnectRequest) *liveSession {
	ctx, cancel := context.WithCancel(parent)
	now := timestamp()
	label := strings.ToUpper(req.Type) + " " + net.JoinHostPort(req.Host, fmt.Sprint(req.Port))
	if req.Type == "serial" {
		label = "Serial " + req.SerialPort
	} else if req.Type == "cmd" {
		label = "Command Prompt (local)"
	} else if req.Type == "powershell" {
		label = "PowerShell (local)"
	}
	if req.DisplayName != "" {
		label = req.DisplayName
	}
	return &liveSession{automation: make(map[*AutomationSubscription]struct{}), ctx: ctx, cancel: cancel, request: reconnectSettings(req), done: make(chan struct{}), input: make(chan []byte, 64), resize: make(chan struct{}, 1), controls: make(chan ControlRequest, 16),
		info: SessionInfo{ID: id, ProfileID: req.ProfileID, LineEnding: req.LineEnding, Type: req.Type, Label: label, Host: req.Host, Port: req.Port, Username: req.Username,
			SSHCompatibility: req.SSHCompatibility,
			SerialPort:       req.SerialPort, State: "connecting", CreatedAt: now, UpdatedAt: now, Cols: req.Cols, Rows: req.Rows}}
}

// Reconnect replaces only the transport. The session ID, owning window and
// retained display remain the same. Saved credentials are resolved afresh;
// manual password/private-key authentication is never retained for reconnect.
func (m *Manager) Reconnect(ctx context.Context, id string) (SessionInfo, error) {
	if err := ctx.Err(); err != nil {
		return SessionInfo{}, err
	}
	m.mu.Lock()
	if m.closed || m.ctx.Err() != nil {
		m.mu.Unlock()
		return SessionInfo{}, ErrClosed
	}
	s := m.sessions[id]
	if s == nil {
		m.mu.Unlock()
		return SessionInfo{}, ErrNotFound
	}
	if s.reconnecting {
		m.mu.Unlock()
		return SessionInfo{}, ErrBusy
	}
	req := cloneConnectRequest(s.request)
	req.Cols, req.Rows = s.info.Cols, s.info.Rows
	resolve := m.resolveCredential
	if req.Type == "ssh" && req.CredentialID == "" && req.AuthMethod != "keyboard" {
		m.mu.Unlock()
		return SessionInfo{}, ErrAuthenticationRequired
	}
	if req.CredentialID != "" && resolve == nil {
		m.mu.Unlock()
		return SessionInfo{}, ErrCredentialUnavailable
	}
	opener := m.openers[req.Type]
	if opener == nil || s.done == nil {
		m.mu.Unlock()
		return SessionInfo{}, errors.New("the previous connection settings are unavailable")
	}
	if req.Type == "ssh" && m.known != nil {
		if err := m.known.loadError(); err != nil {
			m.mu.Unlock()
			return SessionInfo{}, err
		}
	}
	waitCtx, cancel := context.WithCancel(ctx)
	s.reconnecting, s.reconnectCancel = true, cancel
	m.mu.Unlock()
	stopManagerCancellation := context.AfterFunc(m.ctx, cancel)
	defer func() {
		stopManagerCancellation()
		cancel()
		m.mu.Lock()
		s.reconnecting, s.reconnectCancel = false, nil
		m.mu.Unlock()
	}()
	if req.CredentialID != "" {
		resolved, err := resolve(waitCtx, cloneConnectRequest(req))
		if err != nil {
			return SessionInfo{}, err
		}
		if resolved.Type != req.Type || resolved.Host != req.Host || resolved.Port != req.Port || resolved.Username != req.Username || resolved.CredentialID != req.CredentialID {
			return SessionInfo{}, errors.New("saved credential resolution cannot change the reconnect destination or user")
		}
		req.AuthMethod, req.Password, req.PrivateKey, req.Passphrase = resolved.AuthMethod, resolved.Password, resolved.PrivateKey, resolved.Passphrase
		// The resolver supplies the current authenticated vault binding, including
		// a pin that may have been explicitly renewed since the previous session.
		req.ExpectedHostFingerprint = resolved.ExpectedHostFingerprint
		resolved.Password, resolved.PrivateKey, resolved.Passphrase = "", "", ""
		if req.AuthMethod != "password" && req.AuthMethod != "key" || req.AuthMethod == "password" && req.Password == "" {
			return SessionInfo{}, ErrCredentialUnavailable
		}
		if _, err := normalizeRequest(req); err != nil {
			return SessionInfo{}, ErrCredentialUnavailable
		}
	}
	if err := waitCtx.Err(); err != nil {
		return SessionInfo{}, err
	}
	m.mu.Lock()
	if m.closed || m.ctx.Err() != nil || m.sessions[id] != s {
		m.mu.Unlock()
		return SessionInfo{}, ErrClosed
	}
	s.disconnected = true
	m.mu.Unlock()
	m.finish(s, nil)
	select {
	case <-s.done:
	case <-waitCtx.Done():
		return SessionInfo{}, waitCtx.Err()
	case <-m.ctx.Done():
		return SessionInfo{}, ErrClosed
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.ctx.Err() != nil {
		return SessionInfo{}, ErrClosed
	}
	if err := waitCtx.Err(); err != nil {
		return SessionInfo{}, err
	}
	if m.sessions[id] != s {
		return SessionInfo{}, ErrNotFound
	}
	if m.live >= maxLiveSessions {
		return SessionInfo{}, ErrLimit
	}
	next := newLiveSession(m.ctx, id, req)
	m.generation++
	next.generation = m.generation
	next.info.CreatedAt = s.info.CreatedAt
	next.info.BytesReceived, next.info.BytesSent = s.info.BytesReceived, s.info.BytesSent
	m.sessions[id] = next
	s.request = ConnectRequest{}
	m.live++
	m.wg.Add(1)
	m.statusLocked(next, "Reconnecting using previous connection settings")
	// Save the active cursor before leaving the alternate screen. In the normal
	// buffer, this replaces a stale saved cursor with the current input position.
	// In the alternate buffer, its separate save leaves the normal cursor intact
	// so DECRST 1049 restores the position preceding the full-screen application.
	m.appendLocked(Event{SessionID: id, Type: "output", Data: []byte("\x18\x1b7\x1b[?1049l\x1b[!p\x1b[?1000l\x1b[?1002l\x1b[?1003l\x1b[?1004l\x1b[?1006l\x1b[?1015l\x1b[?2004l\x1b[0m\r\n[Network Toolkit: Reconnecting using previous settings.]\r\n")})
	info := cloneSession(next.info)
	go m.runSession(next, req, opener)
	return info, nil
}

func (m *Manager) runSession(s *liveSession, req ConnectRequest, opener transportOpener) {
	defer func() {
		m.mu.Lock()
		m.live--
		close(s.done)
		m.mu.Unlock()
		m.wg.Done()
	}()
	callbacks := transportCallbacks{
		Output: func(data []byte) { m.output(s, data) },
		Prompt: func(ctx context.Context, prompt Prompt) (PromptReply, error) { return m.ask(s, ctx, prompt) },
	}
	conn, err := opener(s.ctx, req, callbacks)
	// Release the opener's copy. The retained session request contains only safe
	// connection settings and an optional backend credential reference.
	req.Password, req.PrivateKey, req.Passphrase = "", "", ""
	if err != nil {
		m.finish(s, err)
		return
	}
	if conn == nil {
		m.finish(s, errors.New("terminal transport returned no connection"))
		return
	}
	m.mu.Lock()
	if s.finished || s.ctx.Err() != nil {
		m.mu.Unlock()
		_ = conn.Close()
		m.finish(s, s.ctx.Err())
		return
	}
	if accepted, ok := conn.(interface{ commitOwnership() error }); ok {
		if err := accepted.commitOwnership(); err != nil {
			m.mu.Unlock()
			_ = conn.Close()
			m.finish(s, err)
			return
		}
	}
	s.conn = conn
	if local, ok := conn.(interface{ LocalShellState() LocalShellState }); ok {
		state := local.LocalShellState()
		s.info.WorkingDirectory, s.info.Elevated = state.WorkingDirectory, state.Elevated
	}
	s.info.State = "connected"
	m.statusLocked(s, "Connected")
	m.mu.Unlock()
	writerDone := make(chan struct{})
	go func() { defer close(writerDone); m.writeLoop(s, conn) }()
	select {
	case err = <-conn.Done():
	case <-s.ctx.Done():
		err = s.ctx.Err()
	}
	m.finish(s, err)
	<-writerDone
}

func (m *Manager) writeLoop(s *liveSession, conn transport) {
	for {
		if s.ctx.Err() != nil {
			return
		}
		select {
		case <-s.ctx.Done():
			return
		case data := <-s.input:
			written := 0
			var err error
			for written < len(data) && s.ctx.Err() == nil {
				var n int
				n, err = conn.Write(data[written:])
				if n < 0 || n > len(data)-written {
					err = errors.New("invalid terminal write result")
					break
				}
				written += n
				if err != nil {
					break
				}
				if n == 0 {
					err = io.ErrNoProgress
					break
				}
			}
			m.mu.Lock()
			s.queuedInput -= len(data)
			s.info.BytesSent += uint64(written)
			m.mu.Unlock()
			clear(data)
			if err != nil {
				m.finish(s, err)
				return
			}
		case <-s.resize:
			m.mu.Lock()
			cols, rows := s.info.Cols, s.info.Rows
			m.mu.Unlock()
			if err := conn.Resize(cols, rows); err != nil {
				m.finish(s, fmt.Errorf("resize terminal: %w", err))
				return
			}
		case control := <-s.controls:
			serial, ok := conn.(serialControl)
			if !ok {
				m.finish(s, errors.New("serial controls are unavailable"))
				return
			}
			var err error
			if control.DTR != nil || control.RTS != nil {
				err = serial.SetSignals(control.DTR, control.RTS)
			}
			if err == nil && control.Break != nil {
				err = serial.SetBreak(*control.Break)
			}
			if err != nil {
				m.finish(s, fmt.Errorf("serial control: %w", err))
				return
			}
		}
	}
}

func (m *Manager) finish(s *liveSession, err error) {
	m.mu.Lock()
	if s.finished {
		m.mu.Unlock()
		return
	}
	s.finished = true
	for subscriber := range s.automation {
		subscriber.finish()
	}
	s.info.Prompt = nil
	s.pending = nil
	s.info.State = "closed"
	message := "Disconnected"
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) && !s.disconnected && !m.closed {
		s.info.State = "error"
		s.info.Error = safeMessage(err.Error(), 2048)
		var legacyRequired *sshLegacyRequiredError
		if s.info.Type == "ssh" && s.info.SSHCompatibility == SSHCompatibilityModern && errors.As(err, &legacyRequired) {
			s.info.ErrorCode = "ssh_legacy_required"
		}
		message = s.info.Error
	}
	conn := s.conn
	s.conn = nil
	s.cancel()
	m.statusLocked(s, message)
	// Drop unsent bytes from live memory without ever publishing their contents.
	for {
		select {
		case data := <-s.input:
			clear(data)
			s.queuedInput -= len(data)
		default:
			goto drained
		}
	}
drained:
	m.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}

func (m *Manager) List() []SessionInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.listLocked()
}

func (m *Manager) listLocked() []SessionInfo {
	list := make([]SessionInfo, 0, len(m.sessions))
	for _, s := range m.sessions {
		list = append(list, cloneSession(s.info))
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt < list[j].CreatedAt })
	return list
}

func (m *Manager) Write(id string, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if len(data) > maxInputChunk {
		return errors.New("terminal input must not exceed 64 KiB per request")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s, err := m.connectedLocked(id)
	if err != nil {
		return err
	}
	if s.queuedInput+len(data) > maxQueuedInput || len(s.input) == cap(s.input) {
		return ErrQueueFull
	}
	copy := append([]byte(nil), data...)
	s.queuedInput += len(copy)
	s.input <- copy
	return nil
}

func (m *Manager) Resize(id string, cols, rows int) error {
	if err := validDimensions(cols, rows); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s, err := m.connectedLocked(id)
	if err != nil {
		return err
	}
	s.info.Cols, s.info.Rows = cols, rows
	select {
	case s.resize <- struct{}{}:
	default:
	}
	return nil
}

func (m *Manager) Control(id string, req ControlRequest) error {
	if req.DTR == nil && req.RTS == nil && req.Break == nil {
		return errors.New("select DTR, RTS, or break control")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s, err := m.connectedLocked(id)
	if err != nil {
		return err
	}
	if _, ok := s.conn.(serialControl); !ok {
		return errors.New("controls apply only to supported serial sessions")
	}
	req = cloneControl(req)
	select {
	case s.controls <- req:
		if req.DTR != nil {
			s.request.SerialDTR = req.DTR
		}
		if req.RTS != nil {
			s.request.SerialRTS = req.RTS
		}
		return nil
	default:
		return ErrBusy
	}
}

func (m *Manager) Rename(id, label string) error {
	label = strings.TrimSpace(label)
	if label == "" || len([]rune(label)) > 100 || strings.IndexFunc(label, unicode.IsControl) >= 0 {
		return errors.New("Use a terminal name of 1 through 100 characters without control characters.")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[id]
	if s == nil {
		return ErrNotFound
	}
	if m.closed {
		return ErrClosed
	}
	s.info.Label = label
	m.statusLocked(s, "")
	return nil
}

func (m *Manager) connectedLocked(id string) (*liveSession, error) {
	if m.closed {
		return nil, ErrClosed
	}
	s := m.sessions[id]
	if s == nil {
		return nil, ErrNotFound
	}
	if s.finished || s.info.State != "connected" || s.conn == nil {
		return nil, ErrNotConnected
	}
	return s, nil
}

// SFTPClient returns the current SSH transport and its lifetime for an
// explicitly authorized browser operation. The caller must not close the
// returned client. The terminal manager owns it and terminates it on disconnect.
func (m *Manager) SFTPClient(id string) (*ssh.Client, context.Context, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, err := m.connectedLocked(id)
	if err != nil {
		return nil, nil, err
	}
	if s.info.Type != "ssh" {
		return nil, nil, errors.New("Only connected SSH terminal sessions can browse SFTP files")
	}
	transport, ok := s.conn.(*sshTransport)
	if !ok || transport.client == nil {
		return nil, nil, errors.New("This SSH transport does not support shared file browsing")
	}
	return transport.client, s.ctx, nil
}

// BrowseSFTP opens a new SSH subsystem channel without changing or closing
// the shell. Both the HTTP request and the terminal lifetime cancel the SFTP
// channel. No credentials or private key bytes are passed to the browser.
func (m *Manager) BrowseSFTP(ctx context.Context, id string, req model.Request) (model.Result, error) {
	client, lifetime, err := m.SFTPClient(id)
	if err != nil {
		return model.Result{}, err
	}
	operationCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(lifetime, cancel)
	defer stop()
	return diag.RunSFTPOnClient(operationCtx, client, req, nil)
}

func (m *Manager) Disconnect(id string) error {
	m.mu.Lock()
	s := m.sessions[id]
	if s == nil {
		m.mu.Unlock()
		return ErrNotFound
	}
	s.disconnected = true
	if s.reconnectCancel != nil {
		s.reconnectCancel()
	}
	m.mu.Unlock()
	m.finish(s, nil)
	return nil
}

func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[id]
	if s == nil {
		return ErrNotFound
	}
	if !s.finished || s.reconnecting {
		return fmt.Errorf("%w: disconnect the session before removing it", ErrBusy)
	}
	delete(m.sessions, id)
	s.request = ConnectRequest{}
	info := cloneSession(s.info)
	info.State = "removed"
	m.appendLocked(Event{SessionID: id, Type: "status", Session: &info, Message: "Session removed"})
	return nil
}

func (m *Manager) Close() error {
	m.mu.Lock()
	if m.closed {
		done := m.closeDone
		m.mu.Unlock()
		<-done
		return nil
	}
	m.closed = true
	m.cancel()
	sessions := make([]*liveSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		s.request = ConnectRequest{}
		sessions = append(sessions, s)
	}
	close(m.changed)
	m.changed = make(chan struct{})
	m.mu.Unlock()
	for _, s := range sessions {
		m.finish(s, nil)
	}
	m.wg.Wait()
	close(m.closeDone)
	return nil
}

func (m *Manager) output(s *liveSession, data []byte) {
	for len(data) > 0 {
		n := min(len(data), maxOutputChunk)
		chunk := append([]byte(nil), data[:n]...)
		data = data[n:]
		m.mu.Lock()
		if s.finished {
			m.mu.Unlock()
			return
		}
		s.info.BytesReceived += uint64(n)
		m.appendLocked(Event{SessionID: s.info.ID, Type: "output", Data: chunk})
		for subscriber := range s.automation {
			subscriber.push(AutomationOutput{Seq: m.seq, Data: chunk})
		}
		m.mu.Unlock()
	}
}

func (m *Manager) statusLocked(s *liveSession, message string) {
	s.info.UpdatedAt = timestamp()
	info := cloneSession(s.info)
	m.appendLocked(Event{SessionID: s.info.ID, Type: "status", Session: &info, Message: message})
}

func (m *Manager) appendLocked(event Event) {
	m.seq++
	event.Seq, event.Time = m.seq, timestamp()
	cost := eventSize(event)
	m.ring = append(m.ring, storedEvent{event, cost})
	m.ringBytes += cost
	for m.ringStart < len(m.ring) && (m.ringBytes > m.eventBudget || len(m.ring)-m.ringStart > maxEvents) {
		m.ringBytes -= m.ring[m.ringStart].size
		m.ring[m.ringStart] = storedEvent{}
		m.ringStart++
	}
	if m.ringStart > 1024 && m.ringStart*2 > len(m.ring) {
		m.ring = append([]storedEvent(nil), m.ring[m.ringStart:]...)
		m.ringStart = 0
	}
	close(m.changed)
	m.changed = make(chan struct{})
}

// Events is a single manager-wide feed. The caller should maintain one cursor
// for all sessions rather than opening a long poll per terminal tab.
func (m *Manager) EventBounds() (oldest, latest uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	oldest = m.seq + 1
	if m.ringStart < len(m.ring) {
		oldest = m.ring[m.ringStart].event.Seq
	}
	return oldest, m.seq
}

func (m *Manager) Events(ctx context.Context, after uint64, wait time.Duration) (Batch, error) {
	if err := ctx.Err(); err != nil {
		return Batch{}, err
	}
	if wait < 0 || wait > 25*time.Second {
		return Batch{}, errors.New("event wait must be between zero and 25 seconds")
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	for {
		m.mu.Lock()
		batch := m.batchLocked(after)
		changed, closed := m.changed, m.closed
		m.mu.Unlock()
		if len(batch.Events) > 0 || batch.Lost || batch.Reset || wait == 0 || closed {
			return batch, nil
		}
		select {
		case <-ctx.Done():
			return Batch{}, ctx.Err()
		case <-m.ctx.Done():
			return batch, nil
		case <-timer.C:
			m.mu.Lock()
			batch = m.batchLocked(after)
			m.mu.Unlock()
			return batch, nil
		case <-changed:
		}
	}
}

func (m *Manager) batchLocked(after uint64) Batch {
	oldest := m.seq + 1
	if m.ringStart < len(m.ring) {
		oldest = m.ring[m.ringStart].event.Seq
	}
	batch := Batch{Events: []Event{}, Sessions: m.listLocked(), Next: after, Oldest: oldest, Latest: m.seq}
	if after > m.seq {
		batch.Reset = true
		after = 0
		batch.Next = 0
	}
	if after < oldest-1 {
		batch.Lost = true
		batch.LostEvents = oldest - 1 - after
		after = oldest - 1
		batch.Next = after
	}
	bytes := 0
	for _, stored := range m.ring[m.ringStart:] {
		if stored.event.Seq <= after {
			continue
		}
		if len(batch.Events) >= maxBatchEvents || len(batch.Events) > 0 && bytes+stored.size > maxBatchBytes {
			break
		}
		batch.Events = append(batch.Events, cloneEvent(stored.event))
		batch.Next = stored.event.Seq
		bytes += stored.size
	}
	if len(batch.Events) == 0 {
		batch.Next = m.seq
	}
	return batch
}

func (m *Manager) ask(s *liveSession, parent context.Context, prompt Prompt) (PromptReply, error) {
	if err := validatePrompt(prompt); err != nil {
		return PromptReply{}, err
	}
	id, err := randomID()
	if err != nil {
		return PromptReply{}, err
	}
	prompt.ID = id
	ctx, cancel := context.WithTimeout(parent, promptTimeout)
	defer cancel()
	replies := make(chan PromptReply, 1)
	m.mu.Lock()
	if s.finished || s.ctx.Err() != nil {
		m.mu.Unlock()
		return PromptReply{}, context.Canceled
	}
	if s.pending != nil {
		m.mu.Unlock()
		return PromptReply{}, errors.New("overlapping authentication prompts are not supported")
	}
	s.pending = replies
	s.replySent = false
	s.info.Prompt = clonePrompt(&prompt)
	s.info.State = "awaiting_input"
	m.statusLocked(s, "Waiting for user input")
	m.appendLocked(Event{SessionID: s.info.ID, Type: "prompt", Prompt: clonePrompt(&prompt)})
	m.mu.Unlock()
	var reply PromptReply
	select {
	case reply = <-replies:
	case <-ctx.Done():
		err = ctx.Err()
	case <-s.ctx.Done():
		err = s.ctx.Err()
	}
	m.mu.Lock()
	if s.pending == replies {
		s.pending = nil
		s.info.Prompt = nil
		if !s.finished {
			s.info.State = "connecting"
			if s.conn != nil {
				s.info.State = "connected"
			}
			m.statusLocked(s, "Continuing connection")
		}
	}
	m.mu.Unlock()
	if err != nil {
		return PromptReply{}, err
	}
	if reply.Action == "cancel" || reply.Action == "reject" {
		return PromptReply{}, errors.New("connection prompt declined")
	}
	return reply, nil
}

func (m *Manager) Reply(id string, reply PromptReply) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[id]
	if s == nil {
		return ErrNotFound
	}
	if s.finished || s.pending == nil || s.info.Prompt == nil {
		return fmt.Errorf("%w: no prompt is pending", ErrBusy)
	}
	if s.replySent {
		return fmt.Errorf("%w: prompt was already answered", ErrBusy)
	}
	prompt := s.info.Prompt
	if reply.PromptID != prompt.ID {
		return errors.New("prompt is no longer current")
	}
	allowed := false
	for _, action := range prompt.Actions {
		if action == reply.Action {
			allowed = true
			break
		}
	}
	if !allowed {
		return errors.New("invalid prompt action")
	}
	if reply.Action == "submit" {
		if len(reply.Answers) != len(prompt.Fields) {
			return errors.New("answer every authentication challenge")
		}
		total := 0
		for _, answer := range reply.Answers {
			total += len(answer)
			if len(answer) > 16*1024 {
				return errors.New("authentication answer exceeds its limit")
			}
		}
		if total > 64*1024 {
			return errors.New("authentication answers exceed their total limit")
		}
	} else if len(reply.Answers) != 0 {
		return errors.New("this prompt does not accept authentication answers")
	}
	if reply.Action == "replace" && reply.Fingerprint != prompt.Fingerprint {
		return errors.New("type the exact new SHA256 fingerprint to replace the remembered key")
	}
	reply.Answers = append([]string(nil), reply.Answers...)
	select {
	case s.pending <- reply:
		s.replySent = true
		return nil
	default:
		return fmt.Errorf("%w: prompt was already answered", ErrBusy)
	}
}

func validatePrompt(prompt Prompt) error {
	if len(prompt.Title) > 1024 || len(prompt.Message) > 16*1024 || len(prompt.Fields) > 16 || len(prompt.Actions) > 4 {
		return errors.New("remote authentication prompt exceeded its limit")
	}
	total := 0
	for _, field := range prompt.Fields {
		total += len(field.Label)
		if len(field.Label) > 4096 {
			return errors.New("remote authentication question exceeded its limit")
		}
	}
	if total > 16*1024 {
		return errors.New("remote authentication questions exceeded their limit")
	}
	return nil
}

func randomID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
func timestamp() string { return time.Now().UTC().Format(time.RFC3339Nano) }
func safeMessage(text string, limit int) string {
	if len(text) > limit {
		text = text[:limit]
	}
	return strings.ToValidUTF8(text, "�")
}

func clonePrompt(p *Prompt) *Prompt {
	if p == nil {
		return nil
	}
	copy := *p
	copy.Fields = append([]PromptField(nil), p.Fields...)
	copy.Actions = append([]string(nil), p.Actions...)
	copy.PreviousFingerprints = append([]string(nil), p.PreviousFingerprints...)
	return &copy
}
func cloneSession(s SessionInfo) SessionInfo { s.Prompt = clonePrompt(s.Prompt); return s }
func cloneEvent(e Event) Event {
	e.Data = append([]byte(nil), e.Data...)
	e.Prompt = clonePrompt(e.Prompt)
	if e.Session != nil {
		copy := cloneSession(*e.Session)
		e.Session = &copy
	}
	return e
}
func cloneControl(c ControlRequest) ControlRequest {
	if c.DTR != nil {
		v := *c.DTR
		c.DTR = &v
	}
	if c.RTS != nil {
		v := *c.RTS
		c.RTS = &v
	}
	if c.Break != nil {
		v := *c.Break
		c.Break = &v
	}
	return c
}

func cloneConnectRequest(req ConnectRequest) ConnectRequest {
	lines := cloneControl(ControlRequest{DTR: req.SerialDTR, RTS: req.SerialRTS})
	req.SerialDTR, req.SerialRTS = lines.DTR, lines.RTS
	return req
}

func reconnectSettings(req ConnectRequest) ConnectRequest {
	req = cloneConnectRequest(req)
	req.Password, req.PrivateKey, req.Passphrase = "", "", ""
	return req
}
func eventSize(e Event) int {
	n := 256 + len(e.Data) + len(e.Message)
	addPrompt := func(p *Prompt) {
		if p != nil {
			n += 256 + len(p.Title) + len(p.Message) + len(p.Host) + len(p.Fingerprint)
			for _, f := range p.Fields {
				n += 64 + len(f.Label)
			}
		}
	}
	addPrompt(e.Prompt)
	if e.Session != nil {
		n += 512 + len(e.Session.Label) + len(e.Session.Host) + len(e.Session.Username) + len(e.Session.Error) + len(e.Session.ErrorCode) + len(e.Session.SSHCompatibility)
		addPrompt(e.Session.Prompt)
	}
	return n
}
