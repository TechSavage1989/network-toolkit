package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/netip"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const localHostsLimit = 1 << 20

var errLocalToolsUnavailable = errors.New("Hosts editing and IP configuration are available in the Windows app.")
var errLocalConfigConflict = errors.New("The configuration changed since it was loaded. Reload it and review your edits before applying again.")
var errLocalConfigBusy = errors.New("Another local configuration change is still in progress.")

type localIPAddress struct {
	Address      string `json:"address"`
	PrefixLength int    `json:"prefixLength"`
	Origin       string `json:"origin"`
}

type localIPAdapter struct {
	Time            string           `json:"time,omitempty"`
	ID              string           `json:"id"`
	Index           uint32           `json:"index"`
	Name            string           `json:"name"`
	Description     string           `json:"description"`
	Type            string           `json:"type"`
	Status          string           `json:"status"`
	MAC             string           `json:"mac"`
	DHCP            bool             `json:"dhcp"`
	AdminStatus     string           `json:"adminStatus"`
	AdminEditable   bool             `json:"adminEditable"`
	DHCPServer      string           `json:"dhcpServer,omitempty"`
	Addresses       []localIPAddress `json:"addresses"`
	Gateways        []string         `json:"gateways"`
	DNSServers      []string         `json:"dnsServers"`
	DNSAutomatic    bool             `json:"dnsAutomatic"`
	Revision        string           `json:"revision"`
	Editable        bool             `json:"editable"`
	AddressEditable bool             `json:"addressEditable"`
	AddressNote     string           `json:"addressNote,omitempty"`
	Note            string           `json:"note,omitempty"`
	// These are read from Windows, never accepted from an HTTP request. They
	// preserve existing registration/route metadata during a scoped edit.
	dnsRegistration string
	gatewayMetric   *uint32
	configuration   string
	effectiveDNS    []string
}

type localIPSnapshot struct {
	Time      string           `json:"time,omitempty"`
	TimeKind  string           `json:"timeKind,omitempty"`
	Available bool             `json:"available"`
	Elevated  bool             `json:"elevated"`
	Adapters  []localIPAdapter `json:"adapters"`
	Note      string           `json:"note,omitempty"`
}

type localHostsSnapshot struct {
	Available bool     `json:"available"`
	Elevated  bool     `json:"elevated"`
	Path      string   `json:"path"`
	Exists    bool     `json:"exists"`
	Content   string   `json:"content"`
	Revision  string   `json:"revision"`
	Encoding  string   `json:"encoding"`
	Newline   string   `json:"newline"`
	Editable  bool     `json:"editable"`
	Warnings  []string `json:"warnings"`
	Note      string   `json:"note,omitempty"`
}

type localIPRequest struct {
	AdapterID    string   `json:"adapterId"`
	Revision     string   `json:"revision"`
	Mode         string   `json:"mode"`
	Address      string   `json:"address"`
	PrefixLength int      `json:"prefixLength"`
	Gateway      string   `json:"gateway"`
	DNSMode      string   `json:"dnsMode"`
	DNSServers   []string `json:"dnsServers"`
}

type localHostsRequest struct {
	Revision string `json:"revision"`
	Content  string `json:"content"`
}

// Mutation requests accept only fixed, typed local actions. Paths, programs,
// commands, shell text and arbitrary flags are never part of their protocol.
// Wi-Fi key reads have a separate one-profile request and response.
type localToolsRequest struct {
	Kind    string               `json:"kind"`
	IP      *localIPRequest      `json:"ip,omitempty"`
	Hosts   *localHostsRequest   `json:"hosts,omitempty"`
	Network *localNetworkRequest `json:"network,omitempty"`
}

type localToolsResult struct {
	IP    *localIPSnapshot    `json:"ip,omitempty"`
	Hosts *localHostsSnapshot `json:"hosts,omitempty"`
}

type localApplyResult struct {
	NetworkAction  string           `json:"networkAction,omitempty"`
	NetworkScope   string           `json:"networkScope,omitempty"`
	NetworkTarget  string           `json:"networkTarget,omitempty"`
	Output         string           `json:"output,omitempty"`
	RebootRequired bool             `json:"rebootRequired,omitempty"`
	Message        string           `json:"message,omitempty"`
	Error          string           `json:"error,omitempty"`
	ErrorCode      string           `json:"errorCode,omitempty"`
	Partial        bool             `json:"partial"`
	BackupPath     string           `json:"backupPath,omitempty"`
	Result         localToolsResult `json:"result"`
}

type localToolsOperation struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Status   string `json:"status"`
	Started  string `json:"started"`
	Finished string `json:"finished,omitempty"`
	localApplyResult
}

type localToolsBackend struct {
	available bool
	readHosts func(context.Context) (localHostsSnapshot, error)
	readIP    func(context.Context) (localIPSnapshot, error)
	apply     func(context.Context, localToolsRequest) localApplyResult
}

type localToolsManager struct {
	ctx        context.Context
	cancel     context.CancelFunc
	backend    localToolsBackend
	mu         sync.Mutex
	busy       bool
	wifiBusy   bool
	wifi       localWiFiBackend
	operations map[string]localToolsOperation
	order      []string
	workers    sync.WaitGroup
}

func newLocalToolsManager(ctx context.Context) *localToolsManager {
	m := newLocalToolsManagerWithBackend(ctx, localToolsBackend{localToolsAvailable(), readLocalHosts, readLocalIP, applyLocalTools})
	m.wifi = localWiFiBackend{read: readLocalWiFi, reveal: revealLocalWiFi}
	return m
}
func newLocalToolsManagerWithBackend(ctx context.Context, backend localToolsBackend) *localToolsManager {
	child, cancel := context.WithCancel(ctx)
	return &localToolsManager{ctx: child, cancel: cancel, backend: backend, operations: map[string]localToolsOperation{}}
}

// Close cancels a pending UAC handshake and waits for any started fixed-purpose
// helper to stop. Requests are never sent after the app context is cancelled.
func (m *localToolsManager) Close(ctx context.Context) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	m.cancel()
	m.mu.Unlock()
	done := make(chan struct{})
	go func() { m.workers.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func localRevision(value any) string {
	b, _ := json.Marshal(value)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func localHostsRevision(exists bool, b []byte) string {
	if !exists {
		return localRevision("hosts-file-missing")
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func validLocalRevision(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
func localAdapterRevision(a localIPAdapter) string {
	// Do not hash link status, counters, display labels or other incidental UI
	// fields. The identity and complete settings are the optimistic guard.
	return localRevision(struct {
		ID                          string
		Index                       uint32
		DHCP                        bool
		Addresses                   []localIPAddress
		Gateways, DNSServers        []string
		DNSAutomatic                bool
		Registration, Configuration string
		GatewayMetric               *uint32
		AdminStatus                 string
		AdminEditable               bool
	}{strings.ToLower(a.ID), a.Index, a.DHCP, a.Addresses, a.Gateways, a.DNSServers, a.DNSAutomatic, a.dnsRegistration, a.configuration, a.gatewayMetric, a.AdminStatus, a.AdminEditable})
}

func localErrorCode(err error) string {
	switch {
	case errors.Is(err, errLocalConfigConflict):
		return "conflict"
	case errors.Is(err, errLocalToolsUnavailable):
		return "unsupported"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "cancelled"
	case os.IsPermission(err), errors.Is(err, os.ErrPermission):
		return "permission"
	default:
		return "failed"
	}
}

func decodeLocalJSON(w http.ResponseWriter, r *http.Request, target any, limit int64) error {
	mediaType, _, mediaErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaErr != nil || mediaType != "application/json" {
		return errors.New("Send an application/json request.")
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return errors.New("The local configuration request is too large or incomplete.")
	}
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.New("Send a local configuration object.")
	}
	// Reject duplicate and case-variant keys before normal Go struct decoding.
	var seen map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return errors.New("Send a local configuration object.")
	}
	allowedBytes, _ := json.Marshal(target)
	allowed := map[string]json.RawMessage{}
	_ = json.Unmarshal(allowedBytes, &allowed)
	keys := map[string]bool{}
	for dec.More() {
		t, e := dec.Token()
		k, ok := t.(string)
		if _, known := allowed[k]; e != nil || !ok || !known || keys[strings.ToLower(k)] {
			return errors.New("Use each configuration field exactly once.")
		}
		keys[strings.ToLower(k)] = true
		var v json.RawMessage
		if dec.Decode(&v) != nil {
			return errors.New("Invalid configuration value.")
		}
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return errors.New("Configuration fields cannot be null. Send an explicit value.")
		}
	}
	if len(keys) != len(allowed) {
		return errors.New("Send every configuration field explicitly.")
	}
	if _, err = dec.Token(); err != nil {
		return err
	}
	if err = dec.Decode(&seen); !errors.Is(err, io.EOF) {
		return errors.New("Send exactly one configuration object.")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(target); err != nil {
		return fmt.Errorf("Invalid configuration: %w", err)
	}
	return nil
}

func (m *localToolsManager) prepare(ctx context.Context, request localToolsRequest) error {
	if !m.backend.available {
		return errLocalToolsUnavailable
	}
	if err := validateLocalToolsRequest(request); err != nil {
		return err
	}
	if request.Kind == "hosts" {
		s, e := m.backend.readHosts(ctx)
		if e != nil {
			return e
		}
		if !s.Editable {
			return errors.New(s.Note)
		}
		if s.Revision != request.Hosts.Revision {
			return errLocalConfigConflict
		}
		return nil
	}
	s, e := m.backend.readIP(ctx)
	if e != nil {
		return e
	}
	if request.Kind == "network" {
		_, e = planLocalNetworkAction(s, *request.Network)
		return e
	}
	_, _, e = planLocalIP(s, *request.IP)
	return e
}
func (m *localToolsManager) start(request localToolsRequest) (localToolsOperation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.busy {
		return localToolsOperation{}, errLocalConfigBusy
	}
	if err := m.ctx.Err(); err != nil {
		return localToolsOperation{}, err
	}
	m.busy = true
	op := localToolsOperation{ID: randomHex(16), Kind: request.Kind, Status: "running", Started: time.Now().UTC().Format(time.RFC3339), localApplyResult: localApplyResult{Message: "Applying the reviewed change. Approve the Windows administrator prompt if requested."}}
	if request.Kind == "network" {
		op.NetworkAction, op.NetworkScope = string(request.Network.Action), request.Network.Scope
		op.NetworkTarget = "This computer — all adapters/applications"
		if localNetworkScoped(request.Network.Action) {
			op.NetworkTarget = request.Network.AdapterID
		}
		op.Message = "Running " + localNetworkActionLabel(request.Network.Action) + ". Approve Windows administrator access if requested; wait for confirmation before retrying."
	}
	for len(m.order) >= 16 {
		delete(m.operations, m.order[0])
		m.order = m.order[1:]
	}
	m.operations[op.ID] = op
	m.order = append(m.order, op.ID)
	m.workers.Add(1)
	go func() {
		defer m.workers.Done()
		ctx, cancel := context.WithTimeout(m.ctx, 3*time.Minute)
		defer cancel()
		result := m.backend.apply(ctx, request)
		m.mu.Lock()
		defer m.mu.Unlock()
		op.localApplyResult = result
		op.Status = "completed"
		if result.Error != "" {
			op.Status = "failed"
		}
		op.Finished = time.Now().UTC().Format(time.RFC3339)
		m.operations[op.ID] = op
		m.busy = false
	}()
	return op, nil
}
func (m *localToolsManager) operation(id string) (localToolsOperation, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	op, ok := m.operations[id]
	return op, ok
}

func (a *application) handleLocalTools(w http.ResponseWriter, r *http.Request) {
	a.handleLocalToolsWithManager(w, r, a.localTools)
}
func (a *application) handleLocalToolsWithManager(w http.ResponseWriter, r *http.Request, m *localToolsManager) {
	if m == nil || !a.homeCanConnect() {
		jsonError(w, http.StatusConflict, "The workspace is closing.")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/local-tools/")
	if path == "wifi" || path == "wifi/reveal" {
		a.handleLocalWiFi(w, r, m, path)
		return
	}
	if strings.HasPrefix(path, "operations/") {
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		id := strings.TrimPrefix(path, "operations/")
		if len(id) != 32 {
			http.NotFound(w, r)
			return
		}
		op, ok := m.operation(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		jsonResponse(w, 200, map[string]any{"operation": op})
		return
	}
	if path != "ip" && path != "hosts" && path != "network" {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodGet {
		if path == "network" {
			methodError(w)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		if path == "ip" {
			value, err := m.backend.readIP(ctx)
			if err != nil {
				jsonError(w, 500, err.Error())
				return
			}
			stampLocalIPSnapshot(&value, time.Now())
			jsonResponse(w, 200, value)
		} else {
			value, err := m.backend.readHosts(ctx)
			if err != nil {
				jsonError(w, 500, err.Error())
				return
			}
			jsonResponse(w, 200, value)
		}
		return
	}
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	request := localToolsRequest{Kind: path}
	var err error
	if path == "ip" {
		request.IP = &localIPRequest{}
		err = decodeLocalJSON(w, r, request.IP, 16<<10)
	} else if path == "network" {
		request.Network = &localNetworkRequest{}
		err = decodeLocalJSON(w, r, request.Network, 4096)
	} else {
		request.Hosts = &localHostsRequest{}
		err = decodeLocalJSON(w, r, request.Hosts, 6*localHostsLimit+4096)
	}
	if err == nil {
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		err = m.prepare(ctx, request)
		cancel()
	}
	if err != nil {
		code := 400
		errorCode := "validation"
		if errors.Is(err, errLocalConfigConflict) {
			code = 409
			errorCode = "conflict"
		} else if errors.Is(err, errLocalToolsUnavailable) {
			code = 501
			errorCode = "unsupported"
		}
		jsonResponse(w, code, map[string]any{"error": err.Error(), "errorCode": errorCode})
		return
	}
	if !a.homeCanConnect() {
		jsonError(w, http.StatusConflict, "The workspace is closing.")
		return
	}
	op, err := m.start(request)
	if err != nil {
		jsonError(w, 409, err.Error())
		return
	}
	jsonResponse(w, http.StatusAccepted, map[string]any{"operation": op})
}

func validateLocalToolsRequest(r localToolsRequest) error {
	switch r.Kind {
	case "hosts":
		if r.Hosts == nil || r.IP != nil || r.Network != nil || !validLocalRevision(r.Hosts.Revision) {
			return errors.New("Reload the hosts file before saving.")
		}
		_, err := validateHostsText(r.Hosts.Content)
		return err
	case "ip":
		if r.IP == nil || r.Hosts != nil || r.Network != nil {
			return errors.New("Specify one adapter configuration.")
		}
		return validateLocalIPRequest(*r.IP)
	case "network":
		if r.Network == nil || r.IP != nil || r.Hosts != nil {
			return errors.New("Specify exactly one network action.")
		}
		return validateLocalNetworkRequest(*r.Network)
	default:
		return errors.New("Unsupported local configuration action.")
	}
}

func localIPv4(s string, allowLoopback bool) (netip.Addr, error) {
	a, e := netip.ParseAddr(s)
	if e != nil || !a.Is4() || a.IsUnspecified() || a.IsMulticast() || a == netip.AddrFrom4([4]byte{255, 255, 255, 255}) || (!allowLoopback && a.IsLoopback()) {
		return netip.Addr{}, errors.New("Enter a usable IPv4 address.")
	}
	return a, nil
}
func validateLocalIPRequest(r localIPRequest) error {
	if !validLocalRevision(r.Revision) || !validAdapterID(r.AdapterID) {
		return errors.New("Reload and select a network adapter before applying changes.")
	}
	if r.Mode != "dhcp" && r.Mode != "static" {
		return errors.New("Choose DHCP or static IPv4 addressing.")
	}
	// A DNS-only adapter can have no current address while disconnected. Its
	// unchanged empty snapshot is represented by address "" and prefix 0.
	// planLocalIP proves that this cannot create an address mutation.
	emptyStaticSnapshot := r.Mode == "static" && r.Address == "" && r.PrefixLength == 0
	if r.Mode == "static" && !emptyStaticSnapshot {
		a, e := localIPv4(r.Address, false)
		if e != nil {
			return fmt.Errorf("IP address: %w", e)
		}
		if r.PrefixLength < 1 || r.PrefixLength > 32 {
			return errors.New("The IPv4 prefix length must be between 1 and 32.")
		}
		if r.PrefixLength <= 30 {
			p := netip.PrefixFrom(a, r.PrefixLength).Masked()
			b := a.As4()
			n := p.Addr().As4()
			v := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
			nv := uint32(n[0])<<24 | uint32(n[1])<<16 | uint32(n[2])<<8 | uint32(n[3])
			if v == nv || v == nv|(^uint32(0)>>r.PrefixLength) {
				return errors.New("Choose a usable host address, not the subnet or broadcast address.")
			}
		}
		if r.Gateway != "" {
			g, e := localIPv4(r.Gateway, false)
			if e != nil {
				return fmt.Errorf("Gateway: %w", e)
			}
			if g == a {
				return errors.New("The default gateway cannot be the adapter's own IP address.")
			}
		}
	}
	if r.DNSMode != "automatic" && r.DNSMode != "manual" {
		return errors.New("Choose automatic or manual IPv4 DNS servers.")
	}
	if len(r.DNSServers) > 6 {
		return errors.New("Specify no more than six IPv4 DNS servers.")
	}
	if r.DNSMode == "manual" {
		seen := map[string]bool{}
		for _, v := range r.DNSServers {
			a, e := localIPv4(v, true)
			if e != nil {
				return fmt.Errorf("DNS server %q: %w", v, e)
			}
			if seen[a.String()] {
				return errors.New("List each DNS server only once.")
			}
			seen[a.String()] = true
		}
	}
	return nil
}
func validAdapterID(s string) bool {
	if len(s) != 38 || s[0] != '{' || s[37] != '}' {
		return false
	}
	for i, c := range s[1:37] {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

func primaryLocalIP(a localIPAdapter) (string, int, string) {
	address := ""
	prefix := 0
	gateway := ""
	if len(a.Addresses) > 0 {
		address = a.Addresses[0].Address
		prefix = a.Addresses[0].PrefixLength
	}
	if len(a.Gateways) > 0 {
		gateway = a.Gateways[0]
	}
	return address, prefix, gateway
}
func equalLocalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type localNetshStep struct {
	Label     string
	Arguments []string
}

func planLocalIP(snapshot localIPSnapshot, r localIPRequest) (localIPAdapter, []localNetshStep, error) {
	if err := validateLocalIPRequest(r); err != nil {
		return localIPAdapter{}, nil, err
	}
	var a localIPAdapter
	found := false
	for _, row := range snapshot.Adapters {
		if strings.EqualFold(row.ID, r.AdapterID) {
			a = row
			found = true
			break
		}
	}
	if !found {
		return a, nil, fmt.Errorf("The selected adapter is no longer present. %w", errLocalConfigConflict)
	}
	if a.Revision != r.Revision {
		return a, nil, errLocalConfigConflict
	}
	if !a.Editable {
		return a, nil, fmt.Errorf("This adapter cannot be changed: %s", a.Note)
	}
	address, prefix, gateway := primaryLocalIP(a)
	addressChange := (r.Mode == "dhcp") != a.DHCP || (r.Mode == "static" && (r.Address != address || r.PrefixLength != prefix || r.Gateway != gateway))
	if addressChange && r.Mode == "static" && r.Address == "" {
		return a, nil, errors.New("An empty address snapshot can only preserve existing addressing during a DNS-only change. Choose DHCP or a valid static IPv4 address to change addressing.")
	}
	if addressChange && !a.AddressEditable {
		return a, nil, fmt.Errorf("IPv4 address changes are unavailable for this adapter: %s DNS can still be changed independently.", a.AddressNote)
	}
	name := fmt.Sprintf("name=%d", a.Index)
	steps := []localNetshStep{}
	if addressChange {
		args := []string{"interface", "ipv4", "set", "address", name, "source=" + r.Mode}
		if r.Mode == "static" {
			mask := ^uint32(0) << (32 - r.PrefixLength)
			args = append(args, "address="+r.Address, fmt.Sprintf("mask=%d.%d.%d.%d", byte(mask>>24), byte(mask>>16), byte(mask>>8), byte(mask)))
			if r.Gateway == "" {
				args = append(args, "gateway=none")
			} else {
				args = append(args, "gateway="+r.Gateway)
				if r.Gateway == gateway && a.gatewayMetric != nil {
					args = append(args, fmt.Sprintf("gwmetric=%d", *a.gatewayMetric))
				}
			}
		}
		args = append(args, "store=persistent")
		steps = append(steps, localNetshStep{"IPv4 addressing", args})
	}
	dnsChange := (r.DNSMode == "automatic") != a.DNSAutomatic || (r.DNSMode == "manual" && !equalLocalStrings(r.DNSServers, a.DNSServers))
	if dnsChange {
		args := []string{"interface", "ipv4", "set", "dnsservers", name}
		if r.DNSMode == "automatic" {
			args = append(args, "source=dhcp")
		} else {
			address := "none"
			if len(r.DNSServers) > 0 {
				address = r.DNSServers[0]
			}
			args = append(args, "source=static", "address="+address, "validate=no")
		}
		if a.dnsRegistration == "none" || a.dnsRegistration == "primary" || a.dnsRegistration == "both" {
			args = append(args, "register="+a.dnsRegistration)
		}
		steps = append(steps, localNetshStep{"IPv4 DNS configuration", args})
		for i := 1; r.DNSMode == "manual" && i < len(r.DNSServers); i++ {
			steps = append(steps, localNetshStep{fmt.Sprintf("DNS server %d", i+1), []string{"interface", "ipv4", "add", "dnsservers", name, "address=" + r.DNSServers[i], fmt.Sprintf("index=%d", i+1), "validate=no"}})
		}
	}
	return a, steps, nil
}

func validateHostsText(content string) ([]string, error) {
	if !utf8.ValidString(content) || len(content) > localHostsLimit {
		return nil, errors.New("Hosts text must be valid text no larger than 1 MiB.")
	}
	for _, c := range content {
		if c < 32 && c != '\t' && c != '\r' && c != '\n' {
			return nil, errors.New("Hosts text cannot contain NUL or other control characters.")
		}
	}
	warnings := []string{}
	names := map[string]int{}
	for i, line := range strings.Split(normalizeHostsNewlines(content), "\n") {
		active := strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		if active == "" {
			continue
		}
		fields := strings.Fields(active)
		if len(fields) < 2 {
			return nil, fmt.Errorf("Line %d needs an IP address followed by at least one hostname.", i+1)
		}
		address, err := netip.ParseAddr(fields[0])
		if err != nil || address.Zone() != "" {
			return nil, fmt.Errorf("Line %d has an invalid IPv4 or IPv6 address.", i+1)
		}
		for _, name := range fields[1:] {
			if !validHostsName(name) {
				return nil, fmt.Errorf("Line %d has an invalid hostname: %s. Use ASCII hostnames or punycode.", i+1, name)
			}
			key := strings.ToLower(strings.TrimSuffix(name, "."))
			if first, ok := names[key]; ok && len(warnings) < 50 {
				warnings = append(warnings, fmt.Sprintf("%s also appears on line %d.", name, first))
			} else {
				names[key] = i + 1
			}
		}
	}
	return warnings, nil
}
func validHostsName(s string) bool {
	s = strings.TrimSuffix(s, ".")
	if s == "" || len(s) > 253 {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
				return false
			}
		}
	}
	return true
}
func normalizeHostsNewlines(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

func finalizeLocalAdapters(rows []localIPAdapter) []localIPAdapter {
	sort.SliceStable(rows, func(i, j int) bool {
		upI, upJ := strings.EqualFold(rows[i].Status, "up"), strings.EqualFold(rows[j].Status, "up")
		if upI != upJ {
			return upI
		}
		return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
	})
	for i := range rows {
		rows[i].Revision = localAdapterRevision(rows[i])
	}
	return rows
}
