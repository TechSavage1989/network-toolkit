package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"networktoolkit/internal/terminal"
)

// File connections have application lifetimes, not individual HTTP lifetimes.
// Credentials are memory-only, as with Terminal's explicit reconnect support.
type browserFileConnection struct {
	client    *terminal.FileConnection
	request   terminal.ConnectRequest
	cancel    context.CancelFunc
	promotion sync.Mutex
}

func (a *application) browserHomeAvailable(r *http.Request) bool {
	if terminalRequestView(r) != "" || a.shutdownRequested.Load() || a.ctx != nil && a.ctx.Err() != nil {
		return false
	}
	if a.views != nil {
		a.views.mu.Lock()
		defer a.views.mu.Unlock()
		return a.views.homePresent && !a.views.homeClosing
	}
	return true
}

func browserRequestHasLogin(r remoteBrowserRequest) bool {
	return r.Host != "" || r.Port != 0 || r.Username != "" || r.Password != "" || r.PrivateKey != "" || r.KeyPath != "" || r.Passphrase != "" || r.Fingerprint != "" || r.AuthMode != "" || r.Security != "" || r.SSHCompatibility != "" || r.ProfileID != "" || r.TimeoutMs != 0
}

func (a *application) browserConnection(id string) (*browserFileConnection, error) {
	a.browserMu.Lock()
	defer a.browserMu.Unlock()
	entry := a.browserConnections[id]
	if entry == nil {
		return nil, errors.New("SFTP connection closed or unavailable. Reconnect before browsing")
	}
	select {
	case <-entry.client.Done():
		return nil, errors.New("SFTP connection not connected. Reconnect before browsing")
	default:
	}
	return entry, nil
}

func (a *application) openBrowserConnection(request *http.Request, r remoteBrowserRequest) (string, error) {
	if !a.browserHomeAvailable(request) {
		return "", terminal.ErrClosed
	}
	a.browserMu.Lock()
	if len(a.browserConnections)+a.browserConnecting >= 8 {
		a.browserMu.Unlock()
		return "", errors.New("At most eight standalone SFTP connections may be open")
	}
	a.browserConnecting++
	a.browserMu.Unlock()
	defer func() { a.browserMu.Lock(); a.browserConnecting--; a.browserMu.Unlock() }()
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	lifetime, cancel := context.WithCancel(parent)
	stop := context.AfterFunc(request.Context(), cancel)
	defer stop()
	req := terminal.ConnectRequest{Type: "ssh", ProfileID: r.ProfileID, Host: r.Host, Port: r.Port, Username: r.Username, AuthMethod: r.AuthMode, Password: r.Password, PrivateKey: r.PrivateKey, Passphrase: r.Passphrase, SSHCompatibility: r.SSHCompatibility, Cols: 80, Rows: 24, TimeoutMs: r.TimeoutMs}
	if req.AuthMethod == "key" && req.PrivateKey == "" && r.KeyPath != "" {
		file, err := os.Open(r.KeyPath)
		if err != nil {
			cancel()
			return "", err
		}
		defer file.Close()
		b, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
		if err != nil {
			cancel()
			return "", err
		}
		if len(b) > 1<<20 {
			cancel()
			return "", errors.New("SSH private key exceeds 1 MiB")
		}
		req.PrivateKey = string(b)
	}
	client, err := terminal.OpenFileConnection(lifetime, req, strings.TrimSpace(r.Fingerprint))
	if err != nil {
		cancel()
		return "", err
	}
	stop()
	if request.Context().Err() != nil || !a.browserHomeAvailable(request) {
		client.Close()
		cancel()
		return "", terminal.ErrClosed
	}
	id := randomHex(24)
	entry := &browserFileConnection{client: client, request: req, cancel: cancel}
	if a.views != nil {
		a.views.mu.Lock()
	}
	available := !a.shutdownRequested.Load() && (a.ctx == nil || a.ctx.Err() == nil) && (a.views == nil || a.views.homePresent && !a.views.homeClosing)
	if available {
		a.browserMu.Lock()
		if a.browserConnections == nil {
			a.browserConnections = map[string]*browserFileConnection{}
		}
		a.browserConnections[id] = entry
		a.browserMu.Unlock()
	}
	if a.views != nil {
		a.views.mu.Unlock()
	}
	if !available {
		client.Close()
		cancel()
		return "", terminal.ErrClosed
	}

	go func() {
		<-client.Done()
		a.browserMu.Lock()
		if a.browserConnections[id] == entry {
			delete(a.browserConnections, id)
		}
		a.browserMu.Unlock()
	}()
	return id, nil
}

func (a *application) closeBrowserConnection(id string) {
	a.browserMu.Lock()
	entry := a.browserConnections[id]
	a.browserMu.Unlock()
	if entry == nil {
		return
	}
	entry.promotion.Lock()
	defer entry.promotion.Unlock()
	a.browserMu.Lock()
	current := a.browserConnections[id]
	if current == entry {
		delete(a.browserConnections, id)
	}
	a.browserMu.Unlock()
	if current == entry {
		entry.client.Close()
		entry.cancel()
	}
}
func (a *application) closeBrowserConnections() {
	a.browserMu.Lock()
	ids := make([]string, 0, len(a.browserConnections))
	for id := range a.browserConnections {
		ids = append(ids, id)
	}
	a.browserMu.Unlock()
	for _, id := range ids {
		a.closeBrowserConnection(id)
	}
}

func (a *application) promoteBrowserConnection(r *http.Request, id string) (terminal.SessionInfo, error) {
	entry, err := a.browserConnection(id)
	if err != nil {
		return terminal.SessionInfo{}, err
	}
	entry.promotion.Lock()
	defer entry.promotion.Unlock()
	if a.terminals == nil || a.views == nil || !a.browserHomeAvailable(r) {
		return terminal.SessionInfo{}, terminal.ErrClosed
	}
	a.views.mu.Lock()
	if !a.views.homePresent || a.views.homeClosing {
		a.views.mu.Unlock()
		return terminal.SessionInfo{}, terminal.ErrClosed
	}
	info, err := a.terminals.AdoptFileConnection(entry.client, entry.request)
	a.views.mu.Unlock()
	if err != nil {
		return info, err
	}
	timer := time.NewTimer(20 * time.Second)
	defer timer.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		info, err = a.terminals.Get(info.ID)
		if err != nil {
			return info, err
		}
		if info.State == "connected" {
			if !a.browserHomeAvailable(r) {
				_ = a.terminals.Disconnect(info.ID)
				return info, terminal.ErrClosed
			}
			a.browserMu.Lock()
			if a.browserConnections[id] == entry {
				delete(a.browserConnections, id)
			}
			a.browserMu.Unlock()
			return info, nil
		}
		if info.State == "error" || info.State == "closed" {
			message := info.Error
			if message == "" {
				message = "The SSH server does not provide an interactive terminal"
			}
			_ = a.terminals.Remove(info.ID)
			return info, errors.New(message)
		}
		select {
		case <-r.Context().Done():
			_ = a.terminals.Disconnect(info.ID)
			return info, r.Context().Err()
		case <-timer.C:
			_ = a.terminals.Disconnect(info.ID)
			return info, errors.New("Opening the SSH terminal timed out")
		case <-tick.C:
		}
	}
}

func (a *application) browserConnectionInfos() []map[string]any {
	infos := []map[string]any{}
	a.browserMu.Lock()
	defer a.browserMu.Unlock()
	for id, entry := range a.browserConnections {
		select {
		case <-entry.client.Done():
			continue
		default:
		}
		infos = append(infos, map[string]any{"id": id, "host": entry.request.Host, "port": entry.request.Port, "username": entry.request.Username, "connected": true})
	}
	return infos
}
