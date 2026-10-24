package terminal

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"networktoolkit/internal/diag"
	"networktoolkit/internal/model"
)

// FileConnection retains one authenticated SSH transport. Each operation opens
// an independent SFTP channel, exactly as BrowseSFTP does for a live terminal.
// It retains reconnect settings only in process memory.
type FileConnection struct {
	mu         sync.Mutex
	transport  *sshTransport
	request    ConnectRequest
	terminalID string
	promoted   atomic.Bool
}

// OpenFileConnection uses Terminal's SSH authentication, algorithm policy,
// client identification and keepalive implementation. The caller must supply
// a fingerprint independently verified through its normal host-key interface.
// Cancelling ctx closes this transport; request contexts belong on Browse.
func OpenFileConnection(ctx context.Context, request ConnectRequest, fingerprint string) (*FileConnection, error) {
	return OpenFileConnectionWithPrompts(ctx, request, fingerprint, nil)
}

// OpenFileConnectionWithPrompts supports the same passphrase and interactive
// authentication challenges as Terminal when its caller provides a prompt UI.
func OpenFileConnectionWithPrompts(ctx context.Context, request ConnectRequest, fingerprint string, prompt func(context.Context, Prompt) (PromptReply, error)) (*FileConnection, error) {
	request.Type = "ssh"
	req, err := normalizeRequest(request)
	if err != nil {
		return nil, err
	}
	fingerprint = strings.TrimSpace(fingerprint)
	if !strings.HasPrefix(fingerprint, "SHA256:") || len(fingerprint) < 25 || len(fingerprint) > 100 {
		return nil, errors.New("verify the SSH server fingerprint before connecting")
	}
	callbacks := transportCallbacks{Prompt: prompt}
	if callbacks.Prompt == nil {
		callbacks.Prompt = func(context.Context, Prompt) (PromptReply, error) {
			return PromptReply{}, errors.New("SSH sign-in requires an interactive challenge; open a Terminal connection and use it for SFTP")
		}
	}

	base, err := openSSHConnection(ctx, req, callbacks, nil, sshTiming{30 * time.Second, 10 * time.Second}, fingerprint)
	if err != nil {
		return nil, err
	}
	return &FileConnection{transport: base, request: req}, nil
}

func (f *FileConnection) Browse(ctx context.Context, req model.Request) (model.Result, error) {
	if f == nil || f.transport == nil || f.transport.ctx.Err() != nil {
		return model.Result{}, ErrNotConnected
	}
	operation, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(f.transport.ctx, cancel)
	defer stop()
	return diag.RunSFTPOnClient(operation, f.transport.client, req, nil)
}

func (f *FileConnection) Done() <-chan struct{} { return f.transport.ctx.Done() }

func (f *FileConnection) Close() error {
	if f == nil || f.transport == nil {
		return nil
	}
	f.mu.Lock()
	f.request.Password, f.request.PrivateKey, f.request.Passphrase = "", "", ""
	f.mu.Unlock()
	return f.transport.Close()
}

// AdoptFileConnection adds a shell to the existing authenticated SSH transport.
// Like Connect it returns a connecting session; Get reports the eventual result.
// A server rejecting the PTY/shell leaves the file connection operational.
// Once connected, the terminal owns the transport: callers must remove the
// old file-registry entry without invoking Close or cancelling its context.
func (m *Manager) AdoptFileConnection(f *FileConnection, request ConnectRequest) (SessionInfo, error) {
	if f == nil || f.transport == nil || f.transport.ctx.Err() != nil {
		return SessionInfo{}, ErrNotConnected
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.terminalID != "" {
		info, err := m.Get(f.terminalID)
		if err == nil && info.State != "closed" && info.State != "error" {
			return info, nil
		}
		if f.promoted.Load() {
			return SessionInfo{}, ErrNotConnected
		}
		f.terminalID = ""
	}
	req := cloneConnectRequest(f.request)
	if request.Host != "" && !strings.EqualFold(request.Host, req.Host) || request.Port != 0 && request.Port != req.Port || request.Username != "" && request.Username != req.Username {
		return SessionInfo{}, errors.New("file connection belongs to a different SSH endpoint or user")
	}
	if request.Cols != 0 {
		req.Cols = request.Cols
	}
	if request.Rows != 0 {
		req.Rows = request.Rows
	}
	if request.Terminal != "" {
		req.Terminal = request.Terminal
	}
	if request.ProfileID != "" {
		req.ProfileID = request.ProfileID
	}
	info, err := m.connectWithOpener(req, func(ctx context.Context, config ConnectRequest, cb transportCallbacks) (transport, error) {
		tr, err := f.transport.openShell(ctx, config, cb)
		if err != nil {
			return nil, err
		}
		tr.onOwned = func() { f.promoted.Store(true) }
		return tr, nil
	})
	if err != nil {
		return SessionInfo{}, err
	}
	f.terminalID = info.ID
	return info, nil
}

func (m *Manager) Get(id string) (SessionInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session := m.sessions[id]
	if session == nil {
		return SessionInfo{}, ErrNotFound
	}
	return cloneSession(session.info), nil
}
