package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Transfer servers are separate from the short-lived diagnostic job queue.
// They are always OFF after startup and automatically stop on app shutdown.
type transferHostManager struct {
	mu      sync.Mutex
	servers map[string]*transferHost
	roots   map[string]string
	recent  map[string][]transferLog
	base    string
	ctx     context.Context
}
type transferHost struct {
	proto, root, bind, address, access string
	port                               int
	overwrite                          bool
	started                            time.Time
	cancel                             context.CancelFunc
	listener                           io.Closer
	sshFingerprint                     string
	authUser, authPassword             string
	security                           string
	active                             int
	total                              uint64
	history                            []transferLog
	mu                                 sync.Mutex
}
type transferLog struct {
	Time      string `json:"time"`
	Peer      string `json:"peer"`
	Operation string `json:"operation"`
	File      string `json:"file"`
	Bytes     uint64 `json:"bytes"`
	Status    string `json:"status"`
	Detail    string `json:"detail,omitempty"`
}
type transferServerConfig struct {
	Protocol       string `json:"protocol"`
	Bind           string `json:"bind"`
	Port           int    `json:"port"`
	Root           string `json:"root"`
	Access         string `json:"access"`
	Overwrite      bool   `json:"overwrite"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	AuthorizedKeys string `json:"authorizedKeys"`
	Security       string `json:"security"`
}

func newTransferHostManager(ctx context.Context, base string) *transferHostManager {
	t := &transferHostManager{ctx: ctx, base: base, servers: make(map[string]*transferHost), roots: make(map[string]string), recent: make(map[string][]transferLog)}
	for _, proto := range []string{"sftp", "tftp", "ftp"} {
		t.roots[proto] = filepath.Join(base, "Transfers", strings.ToUpper(proto))
	}
	go func() { <-ctx.Done(); t.Close() }()
	return t
}
func (m *transferHostManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, s := range m.servers {
		s.cancel()
		_ = s.listener.Close()
		delete(m.servers, k)
	}
}
func transferProtocol(p string) bool { return p == "sftp" || p == "tftp" || p == "ftp" }
func transferRoot(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" || strings.ContainsRune(dir, 0) || len(dir) > 4096 {
		return "", errors.New("choose a valid absolute server root folder")
	}
	full, e := filepath.Abs(dir)
	if e != nil {
		return "", e
	}
	if e = os.MkdirAll(full, 0700); e != nil {
		return "", fmt.Errorf("create server folder: %w", e)
	}
	full, e = filepath.EvalSymlinks(full)
	if e != nil {
		return "", e
	}
	info, e := os.Stat(full)
	if e != nil || !info.IsDir() {
		return "", errors.New("server root must be a directory")
	}
	return full, nil
}
func bindTransferAddress(host string, port int) (string, error) {
	if port < 1 || port > 65535 {
		return "", errors.New("port must be between 1 and 65535")
	}
	host = strings.TrimSpace(host)
	if host == "" {
		host = "127.0.0.1"
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil {
		return "", errors.New("choose a local IP address (not a hostname)")
	}
	if !ip.IsUnspecified() && !ip.IsLoopback() {
		found := false
		ifaces, e := net.InterfaceAddrs()
		if e != nil {
			return "", e
		}
		for _, i := range ifaces {
			if a, ok := i.(*net.IPNet); ok && a.IP.Equal(ip) {
				found = true
				break
			}
		}
		if !found {
			return "", errors.New("IP address is not assigned to a local network adapter")
		}
	}
	return net.JoinHostPort(ip.String(), strconv.Itoa(port)), nil
}
func (s *transferHost) log(peer, op, file string, n uint64, err error) {
	status := "Success"
	details := ""
	if err != nil {
		status = "Failed"
		details = err.Error()
	}
	if len(details) > 240 {
		details = details[:240]
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.total += n
	s.history = append([]transferLog{{Time: time.Now().UTC().Format(time.RFC3339), Peer: peer, Operation: op, File: file, Bytes: n, Status: status, Detail: details}}, s.history...)
	if len(s.history) > 150 {
		s.history = s.history[:150]
	}
}
func (s *transferHost) activity(delta int) { s.mu.Lock(); s.active += delta; s.mu.Unlock() }
func (s *transferHost) snapshot() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	records := append([]transferLog(nil), s.history...)
	return map[string]any{"running": true, "protocol": s.proto, "bind": s.bind, "port": s.port, "address": s.address, "root": s.root, "access": s.access, "overwrite": s.overwrite, "started": s.started.UTC().Format(time.RFC3339), "active": s.active, "bytes": s.total, "transfers": records, "fingerprint": s.sshFingerprint, "security": s.security}
}
func (m *transferHostManager) status(p string) map[string]any {
	m.mu.Lock()
	s := m.servers[p]
	root := m.roots[p]
	recent := append([]transferLog(nil), m.recent[p]...)
	m.mu.Unlock()
	if s != nil {
		return s.snapshot()
	}
	return map[string]any{"running": false, "protocol": p, "root": root, "port": map[string]int{"sftp": 2222, "tftp": 69, "ftp": 2121}[p], "bind": "127.0.0.1", "access": "readwrite", "overwrite": false, "active": 0, "transfers": recent}
}
func (m *transferHostManager) root(p string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.roots[p]
}

// setRoot changes the local file-manager folder without opening a listening socket.
// A running server's root cannot change until the listener is stopped.
func (m *transferHostManager) setRoot(proto, dir string) (map[string]any, error) {
	if !transferProtocol(proto) {
		return nil, errors.New("unsupported file-transfer protocol")
	}
	root, err := transferRoot(dir)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	if m.servers[proto] != nil {
		m.mu.Unlock()
		return nil, errors.New("stop the server before changing its root folder")
	}
	m.roots[proto] = root
	m.mu.Unlock()
	return m.status(proto), nil
}

func (m *transferHostManager) start(c transferServerConfig) (map[string]any, error) {
	if !transferProtocol(c.Protocol) {
		return nil, errors.New("unsupported file-transfer protocol")
	}
	if c.Access != "read" && c.Access != "write" && c.Access != "readwrite" {
		return nil, errors.New("choose read, write or read/write access")
	}
	if (c.Protocol == "sftp" || c.Protocol == "ftp") && strings.TrimSpace(c.Username) == "" {
		return nil, errors.New("SFTP requires a username")
	}
	if c.Protocol == "ftp" && c.Password == "" {
		return nil, errors.New("FTP requires a username and password")
	}
	if c.Protocol == "sftp" && c.Password == "" && c.AuthorizedKeys == "" {
		return nil, errors.New("SFTP requires a password or authorized SSH public keys file")
	}
	if len(c.Username) > 128 || len(c.Password) > 4096 {
		return nil, errors.New("SFTP credentials exceed length limits")
	}
	if c.Port == 0 {
		if c.Protocol == "sftp" {
			c.Port = 2222
		} else if c.Protocol == "ftp" {
			c.Port = 2121
		} else {
			c.Port = 69
		}
	}
	address, err := bindTransferAddress(c.Bind, c.Port)
	if err != nil {
		return nil, err
	}
	root, err := transferRoot(c.Root)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.servers[c.Protocol] != nil {
		return nil, errors.New("server is already running; stop it before changing settings")
	}
	ctx, cancel := context.WithCancel(m.ctx)
	s := &transferHost{proto: c.Protocol, root: root, bind: c.Bind, address: address, port: c.Port, access: c.Access, overwrite: c.Overwrite, started: time.Now(), cancel: cancel, history: make([]transferLog, 0), security: c.Security}
	if s.bind == "" {
		s.bind = "127.0.0.1"
	}
	if c.Protocol == "ftp" {
		var l net.Listener
		l, err = startFTPServer(ctx, s, m.base, c.Username, c.Password, c.Security)
		if err == nil {
			s.listener = l
		}
	} else if c.Protocol == "tftp" {
		var l *net.UDPConn
		l, err = startTFTPServer(ctx, s)
		if err == nil {
			s.listener = l
		}
	} else {
		var l net.Listener
		l, err = startSFTPServer(ctx, s, m.base, c.Username, c.Password, c.AuthorizedKeys)
		if err == nil {
			s.listener = l
		}
	}
	if err != nil {
		cancel()
		return nil, err
	}
	s.log("local", "START", fmt.Sprintf("%s (%s)", s.address, s.security), 0, nil)
	m.servers[c.Protocol] = s
	m.roots[c.Protocol] = root
	return s.snapshot(), nil
}
func (m *transferHostManager) stop(proto string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.servers[proto]
	if s == nil {
		return errors.New("server is not running")
	}
	delete(m.servers, proto)
	s.log("local", "STOP", s.address, 0, nil)
	s.cancel()
	m.recent[proto] = s.snapshot()["transfers"].([]transferLog)
	return s.listener.Close()
}
func (a *application) handleTransferServer(w http.ResponseWriter, r *http.Request) {
	if a.transfers == nil {
		jsonError(w, 503, "File transfer server manager unavailable.")
		return
	}
	if r.Method == http.MethodGet {
		proto := r.URL.Query().Get("protocol")
		if !transferProtocol(proto) {
			jsonError(w, 400, "Choose SFTP, TFTP or FTP.")
			return
		}
		jsonResponse(w, 200, a.transfers.status(proto))
		return
	}
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	var req struct {
		Action string `json:"action"`
		transferServerConfig
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&req); err != nil {
		jsonError(w, 400, "Invalid server settings.")
		return
	}
	if !transferProtocol(req.Protocol) {
		jsonError(w, 400, "Invalid transfer protocol.")
		return
	}
	switch req.Action {
	case "start":
		v, err := a.transfers.start(req.transferServerConfig)
		if err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		jsonResponse(w, 200, v)
	case "clearlog":
		a.transfers.clearLog(req.Protocol)
		jsonResponse(w, 200, a.transfers.status(req.Protocol))
	case "setroot":
		v, err := a.transfers.setRoot(req.Protocol, req.Root)
		if err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		jsonResponse(w, 200, v)
	case "stop":
		if err := a.transfers.stop(req.Protocol); err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		jsonResponse(w, 200, a.transfers.status(req.Protocol))
	default:
		jsonError(w, 400, "Choose start or stop.")
	}
}

// transferPath rejects absolute paths, traversal, symlink traversal and access
// outside the configured root. This also blocks Windows junctions/reparse links
// exposed through os.ModeSymlink. Operations never follow a symlink within root.
func transferPath(root, sub string, allowRoot bool) (string, error) {
	if len(sub) > 4096 || strings.ContainsRune(sub, 0) {
		return "", errors.New("invalid relative path")
	}
	sub = strings.ReplaceAll(sub, "\\", "/")
	if strings.HasPrefix(sub, "/") || strings.Contains(sub, ":") {
		return "", errors.New("absolute paths are not permitted")
	}
	parts := strings.Split(sub, "/")
	for _, seg := range parts {
		if seg == ".." {
			return "", errors.New("parent traversal is not permitted")
		}
	}
	clean := filepath.Clean(filepath.FromSlash(sub))
	if clean == "." && !allowRoot {
		return "", errors.New("choose a file or subfolder")
	}
	file := filepath.Join(root, clean)
	rel, e := filepath.Rel(root, file)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errors.New("file is outside the server root")
	}
	current := root
	for _, p := range strings.Split(rel, string(os.PathSeparator)) {
		if p == "." || p == "" {
			continue
		}
		current = filepath.Join(current, p)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 {
			return "", errors.New("symbolic links or special files are not accessible")
		}
		// Windows directory junctions can be reported as directories. Confirm
		// that existing paths still resolve inside the configured root.
		if runtime.GOOS == "windows" {
			resolved, err := filepath.EvalSymlinks(current)
			if err == nil {
				inside, e := filepath.Rel(root, resolved)
				if e != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(os.PathSeparator)) {
					return "", errors.New("reparse point escapes the server root")
				}
			}
		}
	}
	return file, nil
}
func randomTransferID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (m *transferHostManager) clearLog(proto string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.recent, proto)
	if s := m.servers[proto]; s != nil {
		s.mu.Lock()
		s.history = nil
		s.mu.Unlock()
	}
}
