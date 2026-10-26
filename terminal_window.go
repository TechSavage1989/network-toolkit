package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const terminalWorkspaceName = "Network_Toolkit.Terminal"
const terminalInstanceFilename = "Network_Toolkit.instance.json"
const terminalWindowRequestLimit = 16

// This is deliberately a separate schema from ConnectRequest. Window launch
// can carry only connection settings, never authentication or terminal input.
type terminalWindowPreset struct {
	Type             string `json:"type"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	Username         string `json:"username"`
	AuthMethod       string `json:"authMethod"`
	SSHCompatibility string `json:"sshCompatibility"`
	TimeoutMs        int    `json:"timeoutMs"`
	SerialPort       string `json:"serialPort"`
	BaudRate         int    `json:"baudRate"`
	DataBits         int    `json:"dataBits"`
	StopBits         string `json:"stopBits"`
	Parity           string `json:"parity"`
	FlowControl      string `json:"flowControl"`
	LocalEcho        bool   `json:"localEcho"`
	LineEnding       string `json:"lineEnding"`
}

type terminalWindowEnvelope struct {
	Config *terminalWindowPreset `json:"config,omitempty"`
}

type terminalWindowRequest struct {
	Seq    uint64               `json:"seq"`
	Config terminalWindowPreset `json:"config"`
}

type terminalWindowInbox struct {
	mu       sync.Mutex
	latest   uint64
	requests []terminalWindowRequest
}

type terminalWindowBatch struct {
	Requests []terminalWindowRequest `json:"requests"`
	Latest   uint64                  `json:"latest"`
	Oldest   uint64                  `json:"oldest"`
	Lost     bool                    `json:"lost"`
	Reset    bool                    `json:"reset"`
}

type terminalWindowInstance struct {
	Version    int    `json:"version"`
	Host       string `json:"host"`
	Token      string `json:"token"`
	InstanceID string `json:"instanceId"`
	PID        int    `json:"pid"`
}

func (p *terminalWindowPreset) normalize() error {
	if p == nil {
		return nil
	}
	p.Type = strings.ToLower(strings.TrimSpace(p.Type))
	if p.Type == "" {
		p.Type = "ssh"
	}
	if p.Type != "ssh" && p.Type != "telnet" && p.Type != "serial" && p.Type != "cmd" && p.Type != "powershell" {
		return errors.New("Choose SSH, Telnet, Serial, Command Prompt, or PowerShell for the terminal window.")
	}
	if p.Type == "cmd" || p.Type == "powershell" {
		*p = terminalWindowPreset{Type: p.Type, TimeoutMs: 10000, SSHCompatibility: "modern", AuthMethod: "password", Port: 22, BaudRate: 9600, DataBits: 8, StopBits: "1", Parity: "none", FlowControl: "none"}
		return nil
	}
	if p.SSHCompatibility == "" {
		p.SSHCompatibility = "modern"
	}
	if p.SSHCompatibility != "modern" && p.SSHCompatibility != "legacy" {
		return errors.New("Choose Modern or Legacy devices for SSH compatibility.")
	}
	if p.Type != "ssh" {
		p.SSHCompatibility = "modern"
	}
	for _, field := range []*string{&p.Host, &p.Username, &p.SerialPort} {
		*field = strings.TrimSpace(*field)
		if len(*field) > 512 || strings.IndexFunc(*field, func(r rune) bool { return r < 32 || r == 127 }) >= 0 {
			return errors.New("A terminal connection field is too long or contains control characters.")
		}
	}
	if p.Port == 0 {
		p.Port = 22
		if p.Type == "telnet" {
			p.Port = 23
		}
	}
	if p.Port < 1 || p.Port > 65535 {
		return errors.New("The terminal port must be between 1 and 65535.")
	}
	if p.AuthMethod == "" {
		p.AuthMethod = "password"
	}
	if p.AuthMethod != "password" && p.AuthMethod != "key" && p.AuthMethod != "keyboard" {
		return errors.New("Choose a supported SSH authentication method.")
	}
	if p.TimeoutMs == 0 {
		p.TimeoutMs = 10000
	}
	if p.TimeoutMs < 1000 || p.TimeoutMs > 30000 {
		return errors.New("Connection timeout must be between 1000 and 30000 milliseconds.")
	}
	if p.BaudRate == 0 {
		p.BaudRate = 9600
	}
	if p.BaudRate < 1 || p.BaudRate > 4000000 {
		return errors.New("Choose a valid serial baud rate.")
	}
	if p.DataBits == 0 {
		p.DataBits = 8
	}
	if p.DataBits < 5 || p.DataBits > 8 {
		return errors.New("Serial data bits must be between 5 and 8.")
	}
	if p.StopBits == "" {
		p.StopBits = "1"
	}
	if p.StopBits != "1" && p.StopBits != "1.5" && p.StopBits != "2" {
		return errors.New("Choose a supported number of stop bits.")
	}
	if p.Parity == "" {
		p.Parity = "none"
	}
	if p.Parity != "none" && p.Parity != "odd" && p.Parity != "even" && p.Parity != "mark" && p.Parity != "space" {
		return errors.New("Choose a supported serial parity.")
	}
	if p.FlowControl == "" {
		p.FlowControl = "none"
	}
	if p.FlowControl != "none" && p.FlowControl != "rtscts" && p.FlowControl != "xonxoff" {
		return errors.New("Choose a supported serial flow-control setting.")
	}
	if p.LineEnding == "" {
		p.LineEnding = "crlf"
	}
	if p.LineEnding != "cr" && p.LineEnding != "lf" && p.LineEnding != "crlf" {
		return errors.New("Choose CR, LF, or CR + LF for Enter.")
	}
	return nil
}

func decodeTerminalWindowPreset(encoded string) (*terminalWindowPreset, error) {
	if encoded == "" {
		return nil, nil
	}
	if len(encoded) > 8192 {
		return nil, errors.New("Terminal window settings are too large.")
	}
	b, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(b) > 4096 {
		return nil, errors.New("Terminal window settings could not be read.")
	}
	var preset *terminalWindowPreset
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&preset); err != nil {
		return nil, fmt.Errorf("Invalid terminal window settings: %w", err)
	}
	if err = dec.Decode(new(any)); err != io.EOF {
		return nil, errors.New("Terminal window settings contain extra data.")
	}
	if err = preset.normalize(); err != nil {
		return nil, err
	}
	return preset, nil
}

func (q *terminalWindowInbox) add(preset *terminalWindowPreset) {
	if preset == nil {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.latest++
	q.requests = append(q.requests, terminalWindowRequest{Seq: q.latest, Config: *preset})
	if len(q.requests) > terminalWindowRequestLimit {
		q.requests = append([]terminalWindowRequest(nil), q.requests[len(q.requests)-terminalWindowRequestLimit:]...)
	}
}

func (q *terminalWindowInbox) batch(after uint64) terminalWindowBatch {
	q.mu.Lock()
	defer q.mu.Unlock()
	b := terminalWindowBatch{Requests: []terminalWindowRequest{}, Latest: q.latest}
	if len(q.requests) > 0 {
		b.Oldest = q.requests[0].Seq
	}
	if after > q.latest {
		b.Reset = true
		after = 0
	}
	b.Lost = b.Oldest > 1 && after < b.Oldest-1
	for _, request := range q.requests {
		if request.Seq > after {
			b.Requests = append(b.Requests, request)
		}
	}
	return b
}

func (a *application) handleTerminalWindow(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/terminal-window/requests" {
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		if a.windowMode != "terminal" {
			jsonError(w, http.StatusConflict, "Connection requests belong to the separate terminal window.")
			return
		}
		after := uint64(0)
		if value := r.URL.Query().Get("after"); value != "" {
			var err error
			after, err = strconv.ParseUint(value, 10, 64)
			if err != nil {
				jsonError(w, http.StatusBadRequest, "Invalid terminal-window request cursor.")
				return
			}
		}
		jsonResponse(w, http.StatusOK, a.windowInbox.batch(after))
		return
	}
	if r.URL.Path != "/api/terminal-window" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	var envelope *terminalWindowEnvelope
	if err := decodeTerminalBody(w, r, 8192, &envelope); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if envelope == nil {
		jsonError(w, http.StatusBadRequest, "Send a JSON object for the terminal-window request.")
		return
	}
	if err := envelope.Config.normalize(); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if a.windowMode == "terminal" {
		if a.shutdownRequested.Load() || a.ctx.Err() != nil {
			jsonError(w, http.StatusServiceUnavailable, "The terminal window is closing. Try again after it closes.")
			return
		}
		a.windowInbox.add(envelope.Config)
		a.activatePrimary()
		jsonResponse(w, http.StatusOK, map[string]any{"ok": true, "reused": true, "windowMode": "terminal", "instanceId": a.windowInstanceID})
		return
	}
	if a.openTerminalWindow == nil {
		jsonError(w, http.StatusNotImplemented, "A separate native terminal window is available in the Windows desktop application.")
		return
	}
	a.windowLaunchMu.Lock()
	defer a.windowLaunchMu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	reused, err := a.openTerminalWindow(ctx, envelope.Config)
	if err != nil {
		jsonError(w, http.StatusServiceUnavailable, "The terminal window could not be opened: "+err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"ok": true, "reused": reused})
}

func terminalWorkspace(root string) string { return filepath.Join(root, terminalWorkspaceName) }

// First-use seeding copies only nonsecret preferences and public SSH trust.
// Once created, the independent workspace is never overwritten by the main UI.
func seedTerminalWorkspace(root, dest string) error {
	settingsPath := filepath.Join(dest, "Network_Toolkit.settings.json")
	if _, err := os.Lstat(settingsPath); errors.Is(err, os.ErrNotExist) {
		state := map[string]any{"activePage": "terminal"}
		b, readErr := readBoundedRegularFile(filepath.Join(root, "Network_Toolkit.settings.json"), 2<<20)
		if readErr == nil {
			var original map[string]any
			if json.Unmarshal(b, &original) == nil {
				for _, key := range []string{"terminalProfiles", "terminalRecent", "terminalSidebarCollapsed", "terminalSidebarTab", "terminalSidebarWidth", "terminalComposerCollapsed", "terminalMacroConfirmation", "terminalConfirmClose", "terminalRecentCollapsed", "terminalSavedCollapsed", "terminalSessionConfirmDelete", "terminalConnection", "terminalLayout", "terminalInteraction", "terminalActiveActionsPosition", "terminalAppearance"} {
					if value, ok := original[key]; ok {
						state[key] = value
					}
				}
				if theme, ok := original["theme"].(string); ok && (theme == "dark" || theme == "light") {
					state["theme"] = theme
				}
				if size, ok := original["terminalFontSize"].(float64); ok && size >= 10 && size <= 24 && size == float64(int(size)) {
					state["terminalFontSize"] = size
				}
				if enabled, ok := original["terminalHighlight"].(bool); ok {
					state["terminalHighlight"] = enabled
				}
			}
		} else if !errors.Is(readErr, os.ErrNotExist) {
			return fmt.Errorf("read the initial terminal preferences: %w", readErr)
		}
		sanitizeTerminalSettings(state)
		b, err = json.MarshalIndent(state, "", "  ")
		if err != nil {
			return err
		}
		if err = writeAtomic(settingsPath, b); err != nil {
			return fmt.Errorf("save the terminal workspace: %w", err)
		}
	} else if err != nil {
		return err
	}
	trustPath := filepath.Join(dest, "Network_Toolkit.ssh_known_hosts.json")
	if _, err := os.Lstat(trustPath); errors.Is(err, os.ErrNotExist) {
		b, readErr := readBoundedRegularFile(filepath.Join(root, "Network_Toolkit.ssh_known_hosts.json"), 4<<20)
		if errors.Is(readErr, os.ErrNotExist) {
			// Persist the intentionally empty first-use trust store too. Leaving
			// it absent would import main-window trust on a later launch.
			b = []byte("{\"version\":1,\"hosts\":[]}\n")
			readErr = nil
		}
		if readErr != nil {
			return fmt.Errorf("read the initial SSH host trust: %w", readErr)
		}
		// Manager construction applies the same strict validation as the main
		// app. An invalid source remains an explicit SSH load error, not empty
		// implicit trust or an automatic replacement with another key.
		if err = writeAtomic(trustPath, b); err != nil {
			return fmt.Errorf("save the initial SSH host trust: %w", err)
		}
	} else if err != nil {
		return err
	}
	if err := seedTerminalMacroLibrary(root, dest); err != nil {
		return fmt.Errorf("save the initial macro library: %w", err)
	}
	return nil
}

func seedTerminalMacroLibrary(source, destination string) error {
	destRoot, err := openStorageRoot(destination, true)
	if err != nil {
		return err
	}
	defer destRoot.Close()
	if _, err := destRoot.Lstat(terminalMacroLibraryFilename); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	sourceRoot, err := openStorageRoot(source, false)
	if err != nil {
		return err
	}
	defer sourceRoot.Close()
	library, err := readTerminalMacroLibrary(sourceRoot)
	if err != nil {
		return err
	}
	library.Revision = ""
	raw, err := json.MarshalIndent(library, "", "  ")
	if err != nil {
		return err
	}
	return writeStorageAtomicLimit(destRoot, terminalMacroLibraryFilename, raw, terminalMacroLibraryLimit)
}

func readBoundedRegularFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, errors.New("The saved file is not a regular file or exceeds its size limit.")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err == nil && int64(len(b)) > limit {
		err = errors.New("The saved file exceeds its size limit.")
	}
	return b, err
}

func readTerminalWindowInstance(dir string) (terminalWindowInstance, error) {
	var record terminalWindowInstance
	b, err := readBoundedRegularFile(filepath.Join(dir, terminalInstanceFilename), 4096)
	if err != nil {
		return record, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&record); err != nil {
		return record, err
	}
	if err = dec.Decode(new(any)); err != io.EOF {
		return record, errors.New("Invalid terminal instance record.")
	}
	host, port, splitErr := net.SplitHostPort(record.Host)
	portNumber, portErr := strconv.Atoi(port)
	token, tokenErr := hex.DecodeString(record.Token)
	id, idErr := hex.DecodeString(record.InstanceID)
	if record.Version != 1 || splitErr != nil || host != "127.0.0.1" || portErr != nil || portNumber < 1 || portNumber > 65535 || record.Host != net.JoinHostPort(host, strconv.Itoa(portNumber)) || tokenErr != nil || len(token) != 32 || idErr != nil || len(id) != 16 || record.PID < 1 {
		return record, errors.New("Invalid terminal instance endpoint or identity.")
	}
	return record, nil
}

func activateTerminalWindow(ctx context.Context, dir string, preset *terminalWindowPreset) error {
	record, err := readTerminalWindowInstance(dir)
	if err != nil {
		return err
	}
	b, err := json.Marshal(terminalWindowEnvelope{Config: preset})
	if err != nil {
		return err
	}
	requestCtx, cancel := context.WithTimeout(ctx, 1200*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, "http://"+record.Host+"/api/terminal-window", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-Token", record.Token)
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: time.Second}).DialContext, DisableKeepAlives: true, MaxResponseHeaderBytes: 16384}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("Terminal activation cannot follow redirects.")
	}}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var result struct {
		OK         bool   `json:"ok"`
		WindowMode string `json:"windowMode"`
		InstanceID string `json:"instanceId"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&result) != nil || !result.OK || result.WindowMode != "terminal" || result.InstanceID != record.InstanceID {
		return errors.New("The existing terminal window did not confirm the request.")
	}
	return nil
}

func awaitTerminalWindow(ctx context.Context, dir string, preset *terminalWindowPreset) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		if lastErr = activateTerminalWindow(ctx, dir, preset); lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("The terminal window did not become ready: %w", lastErr)
		case <-ticker.C:
		}
	}
}

func prepareTerminalWindow(root string, preset *terminalWindowPreset) (dir string, release func(), reused bool, err error) {
	dir = terminalWorkspace(root)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", nil, false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	for {
		release, err = acquireTerminalWindowLock(filepath.Join(dir, "Network_Toolkit.window.lock"))
		if !errors.Is(err, ErrTerminalWindowInUse) {
			break
		}
		if activateTerminalWindow(ctx, dir, preset) == nil {
			return dir, nil, true, nil
		}
		select {
		case <-ctx.Done():
			return "", nil, false, errors.New("The existing Terminal workspace is busy and did not respond. Close it before trying again.")
		case <-time.After(100 * time.Millisecond):
			// Retry the lock as well as activation: the previous instance may
			// have been closing when this launch was requested.
		}
	}
	if err != nil {
		return "", nil, false, err
	}
	if err = seedTerminalWorkspace(root, dir); err != nil {
		release()
		return "", nil, false, err
	}
	return dir, release, false, nil
}

func (a *application) registerTerminalWindow(dir string) (func(), error) {
	record := terminalWindowInstance{Version: 1, Host: a.host, Token: a.token, InstanceID: a.windowInstanceID, PID: os.Getpid()}
	b, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, terminalInstanceFilename)
	if err = writeAtomic(path, append(b, '\n')); err != nil {
		return nil, fmt.Errorf("Save the terminal-window instance: %w", err)
	}
	return func() {
		// Only remove our own registration while still holding the process
		// lock. A later instance's endpoint must never be removed by us.
		if current, err := readTerminalWindowInstance(dir); err == nil && current.InstanceID == record.InstanceID {
			_ = os.Remove(path)
		}
	}, nil
}

func terminalChildArguments(root string, preset *terminalWindowPreset, development bool) ([]string, error) {
	args := []string{"--terminal", "--data-dir", root}
	if preset != nil {
		if err := preset.normalize(); err != nil {
			return nil, err
		}
		b, err := json.Marshal(preset)
		if err != nil || len(b) > 4096 {
			return nil, errors.New("Terminal window settings are too large.")
		}
		args = append(args, "--terminal-preset", base64.RawURLEncoding.EncodeToString(b))
	}
	if development {
		args = append(args, "--no-browser")
	}
	return args, nil
}

func launchIndependentTerminal(ctx context.Context, root string, preset *terminalWindowPreset) (bool, error) {
	dir := terminalWorkspace(root)
	if err := activateTerminalWindow(ctx, dir, preset); err == nil {
		return true, nil
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	exe, err := updateCurrentExecutable()
	if err != nil {
		return false, err
	}
	args, err := terminalChildArguments(root, preset, runtime.GOOS != "windows")
	if err != nil {
		return false, err
	}
	// No shell, credentials, parent-owned pipes or parent cancellation context
	// are passed to the independent GUI process.
	cmd := exec.Command(exe, args...)
	configureTerminalChild(cmd)
	if err = cmd.Start(); err != nil {
		return false, err
	}
	go func() { _ = cmd.Wait() }()
	// The child carries the initial preset itself, including if it discovers
	// another instance under the OS lock. Poll with nil to avoid duplicating it.
	if err = awaitTerminalWindow(ctx, dir, nil); err != nil {
		return false, err
	}
	return false, nil
}
