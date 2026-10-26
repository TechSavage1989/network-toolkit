package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

const (
	publicIPCacheTTL      = 5 * time.Minute
	publicIPCheckTimeout  = 8 * time.Second
	publicIPResponseLimit = 128
)

type publicIPFamilyResult struct {
	Status  string `json:"status"`
	Address string `json:"address,omitempty"`
	Error   string `json:"error,omitempty"`
}

type publicIPSnapshot struct {
	Provider  string               `json:"provider"`
	CheckedAt string               `json:"checkedAt"`
	ExpiresAt string               `json:"expiresAt"`
	IPv4      publicIPFamilyResult `json:"ipv4"`
	IPv6      publicIPFamilyResult `json:"ipv6"`
}

type publicIPCall struct {
	done   chan struct{}
	result publicIPSnapshot
	err    error
}

// The cache belongs to one application process and is never added to app
// settings, system snapshots, reports or terminal-window bootstrap data.
type publicIPManager struct {
	ctx      context.Context
	mu       sync.Mutex
	cached   publicIPSnapshot
	expires  time.Time
	inflight *publicIPCall
	now      func() time.Time
	lookup   func(context.Context, int) publicIPFamilyResult
}

func newPublicIPManager(ctx context.Context) *publicIPManager {
	return &publicIPManager{ctx: ctx, now: time.Now, lookup: lookupPublicIPFamily}
}

func (m *publicIPManager) get(ctx context.Context, force bool) (publicIPSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return publicIPSnapshot{}, err
	}
	if err := m.ctx.Err(); err != nil {
		return publicIPSnapshot{}, err
	}
	m.mu.Lock()
	call := m.inflight
	if call == nil {
		if !force && !m.expires.IsZero() && m.now().Before(m.expires) {
			result := m.cached
			m.mu.Unlock()
			return result, nil
		}
		call = &publicIPCall{done: make(chan struct{})}
		m.inflight = call
		go m.refresh(call)
	}
	m.mu.Unlock()
	select {
	case <-ctx.Done():
		return publicIPSnapshot{}, ctx.Err()
	case <-call.done:
		return call.result, call.err
	}
}

func (m *publicIPManager) refresh(call *publicIPCall) {
	// A disconnected renderer must not cancel other callers or turn its
	// canceled request into a cached "unavailable" result. The shared work
	// instead has one bounded lifetime tied to the owning application.
	ctx, cancel := context.WithTimeout(m.ctx, publicIPCheckTimeout)
	defer cancel()
	type familyResult struct {
		family int
		result publicIPFamilyResult
	}
	results := make(chan familyResult, 2)
	for _, family := range []int{4, 6} {
		go func(family int) { results <- familyResult{family, m.lookup(ctx, family)} }(family)
	}
	result := publicIPSnapshot{Provider: "ipify"}
	for range 2 {
		received := <-results
		if received.family == 4 {
			result.IPv4 = received.result
		} else {
			result.IPv6 = received.result
		}
	}
	checked := m.now().UTC()
	result.CheckedAt = checked.Format(time.RFC3339)
	result.ExpiresAt = checked.Add(publicIPCacheTTL).Format(time.RFC3339)
	m.mu.Lock()
	if err := m.ctx.Err(); err != nil {
		call.err = err
	} else {
		// Replace both outcomes, including failures. An old address is never
		// presented as the result of a newly failed lookup.
		m.cached = result
		m.expires = checked.Add(publicIPCacheTTL)
		call.result = result
	}
	if m.inflight == call {
		m.inflight = nil
	}
	close(call.done)
	m.mu.Unlock()
}

func publicIPEndpoint(family int) (network, endpoint string) {
	switch family {
	case 4:
		return "tcp4", "https://api.ipify.org"
	case 6:
		return "tcp6", "https://api6.ipify.org"
	default:
		return "", ""
	}
}

func publicIPHTTPClient(network string, dial func(context.Context, string, string) (net.Conn, error)) *http.Client {
	if dial == nil {
		dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: -1}
		dial = dialer.DialContext
	}
	transport := &http.Transport{
		// Direct, family-specific connections observe this computer's current
		// route. Ambient HTTP_PROXY/HTTPS_PROXY values must not change it.
		Proxy: nil,
		DialContext: func(ctx context.Context, _ string, address string) (net.Conn, error) {
			return dial(ctx, network, address)
		},
		TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:    4 * time.Second,
		ResponseHeaderTimeout:  5 * time.Second,
		MaxResponseHeaderBytes: 8 << 10,
		DisableKeepAlives:      true,
		DisableCompression:     true,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   publicIPCheckTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func lookupPublicIPFamily(ctx context.Context, family int) publicIPFamilyResult {
	network, _ := publicIPEndpoint(family)
	if network == "" {
		return publicIPFamilyResult{Status: "unavailable", Error: "The address family is not supported."}
	}
	client := publicIPHTTPClient(network, nil)
	defer client.CloseIdleConnections()
	return fetchPublicIPFamily(ctx, family, client)
}

func fetchPublicIPFamily(ctx context.Context, family int, client *http.Client) publicIPFamilyResult {
	_, endpoint := publicIPEndpoint(family)
	unavailable := func(message string) publicIPFamilyResult {
		return publicIPFamilyResult{Status: "unavailable", Error: message}
	}
	if endpoint == "" {
		return unavailable("The address family is not supported.")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return unavailable("The public IP check could not be prepared.")
	}
	request.Header.Set("Accept", "text/plain")
	request.Header.Set("Cache-Control", "no-cache")
	request.Header.Set("User-Agent", "NetworkToolkit-PublicIP/1")
	response, err := client.Do(request)
	if err != nil {
		var networkError net.Error
		if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &networkError) && networkError.Timeout() {
			return unavailable(fmt.Sprintf("The IPv%d public IP check timed out. Try Refresh.", family))
		}
		return unavailable(fmt.Sprintf("Could not reach the IPv%d lookup service. Check the Internet connection, VPN, or firewall.", family))
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return unavailable(fmt.Sprintf("The IPv%d lookup service returned HTTP %d. Try Refresh later.", family, response.StatusCode))
	}
	if response.ContentLength > publicIPResponseLimit {
		return unavailable("The lookup service returned an unexpected response.")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, publicIPResponseLimit+1))
	if err != nil {
		return unavailable("The lookup service response could not be read completely. Try Refresh.")
	}
	if len(body) > publicIPResponseLimit {
		return unavailable("The lookup service returned an unexpected response.")
	}
	address, ok := parsePublicIPAddress(strings.TrimSpace(string(body)), family)
	if !ok {
		return unavailable(fmt.Sprintf("The lookup service did not return a valid public IPv%d address.", family))
	}
	return publicIPFamilyResult{Status: "ok", Address: address.String()}
}

var publicIPExcludedIPv4 = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
}

var publicIPGlobalIPv6 = netip.MustParsePrefix("2000::/3")
var publicIPProtocolIPv6 = netip.MustParsePrefix("2001::/23")
var publicIPReachableProtocolIPv6 = []netip.Prefix{
	// The IANA protocol-assignment block is not a general source range.
	// Preserve its reachable specific allocations and Teredo addresses.
	netip.MustParsePrefix("2001::/32"),
	netip.MustParsePrefix("2001:1::1/128"),
	netip.MustParsePrefix("2001:1::2/128"),
	netip.MustParsePrefix("2001:1::3/128"),
	netip.MustParsePrefix("2001:3::/32"),
	netip.MustParsePrefix("2001:4:112::/48"),
	netip.MustParsePrefix("2001:20::/28"),
	netip.MustParsePrefix("2001:30::/28"),
}
var publicIPExcludedIPv6 = []netip.Prefix{
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("3fff::/20"),
}

func parsePublicIPAddress(value string, family int) (netip.Addr, bool) {
	address, err := netip.ParseAddr(value)
	if err != nil || address.Zone() != "" || address.Is4In6() || !address.IsGlobalUnicast() || address.IsPrivate() {
		return netip.Addr{}, false
	}
	if family == 4 && address.Is4() {
		// These two special-purpose anycast addresses are globally reachable.
		if value == "192.0.0.9" || value == "192.0.0.10" {
			return address, true
		}
		for _, prefix := range publicIPExcludedIPv4 {
			if prefix.Contains(address) {
				return netip.Addr{}, false
			}
		}
		return address, true
	}
	if family != 6 || !address.Is6() || !publicIPGlobalIPv6.Contains(address) {
		return netip.Addr{}, false
	}
	if publicIPProtocolIPv6.Contains(address) {
		for _, prefix := range publicIPReachableProtocolIPv6 {
			if prefix.Contains(address) {
				return address, true
			}
		}
		return netip.Addr{}, false
	}
	for _, prefix := range publicIPExcludedIPv6 {
		if prefix.Contains(address) {
			return netip.Addr{}, false
		}
	}
	return address, true
}
