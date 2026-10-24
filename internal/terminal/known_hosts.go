package terminal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

const maxKnownHostFile = 4 * 1024 * 1024
const maxKnownHosts = 2048

type KnownHostInfo struct {
	Host        string `json:"host"`
	KeyType     string `json:"keyType"`
	Fingerprint string `json:"fingerprint"`
	AddedAt     string `json:"addedAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type knownHostEntry struct {
	KnownHostInfo
	Key []byte `json:"key"`
}

type knownHostDocument struct {
	Version int              `json:"version"`
	Hosts   []knownHostEntry `json:"hosts"`
}

type knownHostStore struct {
	mu      sync.Mutex
	path    string
	entries map[string]knownHostEntry
	err     error
}

func loadKnownHostStore(dataDir string) (*knownHostStore, error) {
	store := &knownHostStore{path: filepath.Join(dataDir, "Network_Toolkit.ssh_known_hosts.json"), entries: map[string]knownHostEntry{}}
	err := store.load()
	if err != nil {
		store.err = fmt.Errorf("SSH known-host trust file %s could not be loaded; repair or deliberately rename that file, then restart Network Toolkit before using SSH: %w", store.path, err)
	}
	return store, store.err
}

func (s *knownHostStore) load() error {
	info, err := os.Lstat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("trust file must be a regular file")
	}
	if info.Size() > maxKnownHostFile {
		return errors.New("trust file exceeds the 4 MiB limit")
	}
	file, err := os.Open(s.path)
	if err != nil {
		return err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxKnownHostFile+1))
	if err != nil {
		return err
	}
	if len(raw) > maxKnownHostFile {
		return errors.New("trust file exceeds the 4 MiB limit")
	}
	var document knownHostDocument
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("trust file contains trailing data")
	}
	if document.Version != 1 || len(document.Hosts) > maxKnownHosts {
		return errors.New("unsupported trust-file version or excessive host count")
	}
	for _, entry := range document.Hosts {
		host, portText, err := net.SplitHostPort(entry.Host)
		if err != nil {
			return errors.New("trust file contains an invalid host endpoint")
		}
		normalized, err := normalizeHost(host)
		port, portErr := strconv.Atoi(portText)
		if err != nil || portErr != nil || port < 1 || port > 65535 || entry.Host != net.JoinHostPort(normalized, strconv.Itoa(port)) {
			return errors.New("trust file contains a noncanonical host endpoint")
		}
		if len(entry.Key) == 0 || len(entry.Key) > 16*1024 {
			return errors.New("trust file contains an invalid public key size")
		}
		key, err := ssh.ParsePublicKey(entry.Key)
		if err != nil {
			return errors.New("trust file contains an invalid SSH public key")
		}
		if _, certificate := key.(*ssh.Certificate); certificate {
			return errors.New("SSH host certificate entries are not supported")
		}
		if entry.KeyType != key.Type() || entry.Fingerprint != ssh.FingerprintSHA256(key) {
			return errors.New("trust-file fingerprint does not match its public key")
		}
		if _, err := time.Parse(time.RFC3339Nano, entry.AddedAt); err != nil {
			return errors.New("trust file contains an invalid timestamp")
		}
		if _, err := time.Parse(time.RFC3339Nano, entry.UpdatedAt); err != nil {
			return errors.New("trust file contains an invalid timestamp")
		}
		if _, duplicate := s.entries[entry.Host]; duplicate {
			return errors.New("trust file repeats a host endpoint")
		}
		s.entries[entry.Host] = entry
	}
	return nil
}

func (s *knownHostStore) loadError() error { s.mu.Lock(); defer s.mu.Unlock(); return s.err }

func (m *Manager) KnownHosts() ([]KnownHostInfo, error) {
	if m.known == nil {
		return []KnownHostInfo{}, nil
	}
	return m.known.list()
}

func (s *knownHostStore) list() ([]KnownHostInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	rows := make([]KnownHostInfo, 0, len(s.entries))
	for _, entry := range s.entries {
		rows = append(rows, entry.KnownHostInfo)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Host < rows[j].Host })
	return rows, nil
}

func (m *Manager) RemoveKnownHost(host string) error {
	if m.known == nil {
		return ErrNotFound
	}
	s := m.known
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	if _, exists := s.entries[host]; !exists {
		return ErrNotFound
	}
	next := make(map[string]knownHostEntry, len(s.entries)-1)
	for endpoint, entry := range s.entries {
		if endpoint != host {
			next[endpoint] = entry
		}
	}
	if err := s.persist(next); err != nil {
		return err
	}
	s.entries = next
	return nil
}

func (s *knownHostStore) verify(ctx context.Context, endpoint string, key ssh.PublicKey, prompt func(context.Context, Prompt) (PromptReply, error)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, certificate := key.(*ssh.Certificate); certificate {
		return errors.New("SSH host certificates are not supported; a raw host key is required")
	}
	wire := append([]byte(nil), key.Marshal()...)
	if len(wire) > 16*1024 {
		return errors.New("SSH host public key exceeds its limit")
	}
	s.mu.Lock()
	previous, exists := s.entries[endpoint]
	loadErr := s.err
	s.mu.Unlock()
	if loadErr != nil {
		return loadErr
	}
	if exists && bytes.Equal(previous.Key, wire) {
		return nil
	}
	host, portText, err := net.SplitHostPort(endpoint)
	if err != nil {
		return errors.New("invalid SSH host-key endpoint")
	}
	port, _ := strconv.Atoi(portText)
	fingerprint := ssh.FingerprintSHA256(key)
	challenge := Prompt{Kind: "host-key", Title: "Verify SSH host key", Host: host, Port: port,
		Algorithm: key.Type(), Fingerprint: fingerprint, Changed: exists,
		Message: "This host key is not remembered. Compare the SHA256 fingerprint with a trusted source before accepting it.", Actions: []string{"reject", "once", "save"}}
	if exists {
		challenge.Title = "SSH host key changed"
		challenge.Message = "The server key differs from the remembered key. Verify the new fingerprint independently. Replacement requires typing the exact new SHA256 fingerprint."
		challenge.PreviousFingerprints = []string{previous.Fingerprint}
		challenge.Actions = []string{"reject", "replace"}
	}
	reply, err := prompt(ctx, challenge)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !exists && reply.Action != "once" && reply.Action != "save" {
		return errors.New("SSH host key was not accepted")
	}
	if exists && (reply.Action != "replace" || reply.Fingerprint != fingerprint) {
		return errors.New("changed SSH host key was not explicitly verified")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	current, currentExists := s.entries[endpoint]
	if currentExists && bytes.Equal(current.Key, wire) {
		return nil
	}
	if currentExists != exists || currentExists && !bytes.Equal(current.Key, previous.Key) {
		return errors.New("the remembered SSH host key changed while confirmation was pending; reconnect and verify it again")
	}
	if reply.Action == "once" {
		return nil
	}
	if !exists && len(s.entries) >= maxKnownHosts {
		return errors.New("known-host store reached its 2048-host limit")
	}
	now := timestamp()
	entry := knownHostEntry{KnownHostInfo: KnownHostInfo{Host: endpoint, KeyType: key.Type(), Fingerprint: fingerprint, AddedAt: now, UpdatedAt: now}, Key: wire}
	if exists {
		entry.AddedAt = previous.AddedAt
	}
	next := make(map[string]knownHostEntry, len(s.entries)+1)
	for address, old := range s.entries {
		next[address] = old
	}
	next[endpoint] = entry
	if err := s.persist(next); err != nil {
		return fmt.Errorf("save verified SSH host key: %w", err)
	}
	s.entries = next
	return nil
}

// persist writes a new complete document before replacing the old file. A
// failure leaves the in-memory trust and previous file intact.
func (s *knownHostStore) persist(entries map[string]knownHostEntry) error {
	document := knownHostDocument{Version: 1, Hosts: make([]knownHostEntry, 0, len(entries))}
	for _, entry := range entries {
		document.Hosts = append(document.Hosts, entry)
	}
	sort.Slice(document.Hosts, func(i, j int) bool { return document.Hosts[i].Host < document.Hosts[j].Host })
	raw, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if len(raw) > maxKnownHostFile {
		return errors.New("known-host file would exceed its 4 MiB limit")
	}
	directory := filepath.Dir(s.path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	if info, err := os.Lstat(s.path); err == nil && !info.Mode().IsRegular() {
		return errors.New("trust file must be a regular file")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".ssh-known-hosts-")
	if err != nil {
		return err
	}
	path := temporary.Name()
	defer os.Remove(path)
	if err := temporary.Chmod(0600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(raw); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(path, s.path)
}
