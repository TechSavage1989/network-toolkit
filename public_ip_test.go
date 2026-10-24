package main

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type publicIPRoundTripFunc func(*http.Request) (*http.Response, error)

func (f publicIPRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPublicIPAddressValidation(t *testing.T) {
	for _, test := range []struct {
		value  string
		family int
		want   string
	}{
		{"8.8.8.8", 4, "8.8.8.8"}, {"100.128.0.1", 4, "100.128.0.1"},
		{"192.0.0.9", 4, "192.0.0.9"}, {"192.0.0.10", 4, "192.0.0.10"},
		{"2606:4700:4700::1111", 6, "2606:4700:4700::1111"},
		{"2001:4860:4860:0:0:0:0:8888", 6, "2001:4860:4860::8888"},
		{"2001:0:4136:e378:8000:63bf:3fff:fdd2", 6, "2001:0:4136:e378:8000:63bf:3fff:fdd2"},
		{"2001:1::1", 6, "2001:1::1"}, {"2001:1::2", 6, "2001:1::2"}, {"2001:1::3", 6, "2001:1::3"},
		{"2001:3::1", 6, "2001:3::1"}, {"2001:4:112::1", 6, "2001:4:112::1"},
		{"2001:20::1", 6, "2001:20::1"}, {"2001:30::1", 6, "2001:30::1"},
	} {
		address, ok := parsePublicIPAddress(test.value, test.family)
		if !ok || address.String() != test.want {
			t.Fatalf("valid %q/%d: %v %v", test.value, test.family, address, ok)
		}
	}
	for _, value := range []string{
		"", "router.example", "8.8.8.8:443", "8.8.8.8/32", "8.8.8.8\n1.1.1.1", "[2606:4700::1]",
		"0.0.0.0", "0.1.2.3", "10.0.0.1", "100.64.0.1", "100.127.255.254", "127.0.0.1", "169.254.0.1",
		"172.16.0.1", "172.31.255.254", "192.0.0.1", "192.0.2.3", "192.88.99.1", "192.168.10.1",
		"198.18.0.1", "198.19.255.254", "198.51.100.1", "203.0.113.1", "224.0.0.1", "240.0.0.1", "255.255.255.255",
		"::", "::1", "::ffff:8.8.8.8", "64:ff9b::808:808", "100::1", "fe80::1", "fe80::1%Ethernet",
		"2606:4700::1%eth0", "fc00::1", "fd12::1", "ff02::1", "2001:db8::1", "3fff::1", "3fff:fff::1",
		"2001:2::1", "2001:10::1", "2001:1f::1", "8000::1", "2001:5::1", "2001:1::4", "2001:4:111::1", "2001:1ff::1",
	} {
		for _, family := range []int{4, 6} {
			if address, ok := parsePublicIPAddress(value, family); ok {
				t.Fatalf("accepted non-public/invalid %q as IPv%d: %v", value, family, address)
			}
		}
	}
	for _, test := range []struct {
		value  string
		family int
	}{{"8.8.8.8", 6}, {"2606:4700::1", 4}, {"8.8.8.8", 0}} {
		if _, ok := parsePublicIPAddress(test.value, test.family); ok {
			t.Fatalf("accepted wrong family: %+v", test)
		}
	}
}

func TestPublicIPHTTPResponseValidation(t *testing.T) {
	for _, test := range []struct {
		name, body, want string
		family, status   int
		length           int64
	}{
		{"ipv4", "8.8.8.8\n", "8.8.8.8", 4, 200, -1},
		{"ipv6", " 2001:4860:4860:0:0:0:0:8888\r\n", "2001:4860:4860::8888", 6, 200, -1},
		{"wrong-family", "8.8.8.8", "", 6, 200, -1},
		{"private", "192.168.1.1", "", 4, 200, -1},
		{"html", "<html>8.8.8.8</html>", "", 4, 200, -1},
		{"json", `{"ip":"8.8.8.8"}`, "", 4, 200, -1},
		{"too-long", strings.Repeat(" ", publicIPResponseLimit) + "8.8.8.8", "", 4, 200, -1},
		{"declared-too-long", "8.8.8.8", "", 4, 200, publicIPResponseLimit + 1},
		{"service-failure", "8.8.8.8", "", 4, 503, -1},
		{"redirect", "8.8.8.8", "", 4, 302, -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			client := publicIPHTTPClient("tcp4", nil)
			client.Transport = publicIPRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				_, expected := publicIPEndpoint(test.family)
				if r.Method != http.MethodGet || r.URL.String() != expected || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
					t.Errorf("unexpected reflector request: %s %s %v", r.Method, r.URL, r.Header)
				}
				return &http.Response{StatusCode: test.status, Header: http.Header{"Location": {"https://must-not-follow.invalid"}}, Body: io.NopCloser(strings.NewReader(test.body)), ContentLength: test.length, Request: r}, nil
			})
			result := fetchPublicIPFamily(context.Background(), test.family, client)
			if calls != 1 || result.Address != test.want || (result.Status == "ok") != (test.want != "") {
				t.Fatalf("response validation: %+v; requests=%d", result, calls)
			}
			if test.want == "" && (result.Status != "unavailable" || result.Error == "") {
				t.Fatalf("missing failure explanation: %+v", result)
			}
		})
	}
}

type publicIPBrokenBody struct{}

func (publicIPBrokenBody) Read(p []byte) (int, error) {
	return copy(p, "8.8.8.8"), errors.New("fixture partial response")
}
func (publicIPBrokenBody) Close() error { return nil }

func TestPublicIPErrorsAreBoundedAndReadable(t *testing.T) {
	client := publicIPHTTPClient("tcp4", nil)
	client.Transport = publicIPRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("internal fixture address 192.168.50.1 with sensitive resolver detail")
	})
	result := fetchPublicIPFamily(context.Background(), 4, client)
	if result.Status != "unavailable" || strings.Contains(result.Error, "192.168") || strings.Contains(result.Error, "sensitive") {
		t.Fatalf("raw network details leaked: %+v", result)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	client.Transport = publicIPRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	result = fetchPublicIPFamily(ctx, 6, client)
	if result.Status != "unavailable" || !strings.Contains(result.Error, "timed out") {
		t.Fatalf("deadline was not reported: %+v", result)
	}
	client.Transport = publicIPRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: publicIPBrokenBody{}, ContentLength: -1, Request: r}, nil
	})
	result = fetchPublicIPFamily(context.Background(), 4, client)
	if result.Status != "unavailable" || result.Address != "" {
		t.Fatalf("partial read was accepted: %+v", result)
	}
}

func TestPublicIPClientForcesDirectAddressFamily(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://proxy.invalid:8080")
	t.Setenv("HTTP_PROXY", "http://proxy.invalid:8080")
	for _, network := range []string{"tcp4", "tcp6"} {
		var gotNetwork, gotAddress string
		client := publicIPHTTPClient(network, func(_ context.Context, actualNetwork, address string) (net.Conn, error) {
			gotNetwork, gotAddress = actualNetwork, address
			return nil, errors.New("fixture stops before a real connection")
		})
		transport := client.Transport.(*http.Transport)
		_, _ = transport.DialContext(context.Background(), "tcp", "lookup.fixture.invalid:443")
		if gotNetwork != network || gotAddress != "lookup.fixture.invalid:443" {
			t.Fatalf("family override: %q %q", gotNetwork, gotAddress)
		}
		if transport.Proxy != nil || client.Jar != nil || transport.TLSClientConfig.InsecureSkipVerify || transport.TLSClientConfig.MinVersion < tls.VersionTLS12 || !transport.DisableKeepAlives || client.Timeout != publicIPCheckTimeout {
			t.Fatal("public lookup client lost its direct, bounded HTTPS policy")
		}
	}
}

func TestPublicIPCacheRefreshAndFailureReplacement(t *testing.T) {
	m := newPublicIPManager(context.Background())
	now := time.Date(2026, 10, 21, 20, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }
	var calls atomic.Int32
	var fail atomic.Bool
	m.lookup = func(_ context.Context, family int) publicIPFamilyResult {
		calls.Add(1)
		if fail.Load() {
			return publicIPFamilyResult{Status: "unavailable", Error: "fixture service unavailable"}
		}
		if family == 4 {
			return publicIPFamilyResult{Status: "ok", Address: "8.8.8.8"}
		}
		return publicIPFamilyResult{Status: "ok", Address: "2001:4860:4860::8888"}
	}
	first, err := m.get(context.Background(), false)
	if err != nil || calls.Load() != 2 || first.CheckedAt != now.Format(time.RFC3339) || first.ExpiresAt != now.Add(publicIPCacheTTL).Format(time.RFC3339) {
		t.Fatalf("initial snapshot: %+v, calls=%d, error=%v", first, calls.Load(), err)
	}
	cached, err := m.get(context.Background(), false)
	if err != nil || calls.Load() != 2 || cached != first {
		t.Fatal("fresh snapshot was not cached")
	}
	now = now.Add(time.Minute)
	forced, err := m.get(context.Background(), true)
	if err != nil || calls.Load() != 4 || forced.CheckedAt == first.CheckedAt {
		t.Fatal("manual refresh did not bypass cache")
	}
	now = now.Add(publicIPCacheTTL)
	if _, err = m.get(context.Background(), false); err != nil || calls.Load() != 6 {
		t.Fatal("cache was not refreshed at expiry")
	}
	fail.Store(true)
	failed, err := m.get(context.Background(), true)
	if err != nil || calls.Load() != 8 || failed.IPv4.Address != "" || failed.IPv6.Address != "" || failed.IPv4.Status != "unavailable" || failed.IPv6.Status != "unavailable" {
		t.Fatalf("failed refresh retained an old address: %+v %v", failed, err)
	}
	if _, err = m.get(context.Background(), false); err != nil || calls.Load() != 8 {
		t.Fatal("failed results were immediately re-requested")
	}
	other := newPublicIPManager(context.Background())
	other.lookup = m.lookup
	if _, err = other.get(context.Background(), false); err != nil || calls.Load() != 10 {
		t.Fatal("another application incorrectly reused this process cache")
	}
}

type publicIPObservedContext struct {
	context.Context
	joined chan struct{}
	once   sync.Once
}

func (c *publicIPObservedContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.joined) })
	return c.Context.Done()
}

func TestPublicIPConcurrentRefreshAndCanceledCaller(t *testing.T) {
	m := newPublicIPManager(context.Background())
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var calls atomic.Int32
	m.lookup = func(ctx context.Context, family int) publicIPFamilyResult {
		calls.Add(1)
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > publicIPCheckTimeout {
			t.Error("shared lookup has no bounded application lifetime")
		}
		started <- struct{}{}
		<-release
		if ctx.Err() != nil {
			return publicIPFamilyResult{Status: "unavailable", Error: "fixture context canceled"}
		}
		return publicIPFamilyResult{Status: "ok", Address: "fixture-family-result"}
	}
	cancelCtx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() { _, err := m.get(cancelCtx, true); first <- err }()
	for range 2 {
		<-started
	}
	results := make(chan publicIPSnapshot, 8)
	for i := range 8 {
		ctx := &publicIPObservedContext{Context: context.Background(), joined: make(chan struct{})}
		go func(force bool) {
			result, err := m.get(ctx, force)
			if err != nil {
				t.Error(err)
			}
			results <- result
		}(i%2 == 0)
		<-ctx.joined
	}
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled renderer kept waiting: %v", err)
	}
	close(release)
	for range 8 {
		if result := <-results; result.IPv4.Status != "ok" || result.IPv6.Status != "ok" {
			t.Fatalf("one canceled renderer poisoned other callers: %+v", result)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("concurrent requests did not coalesce: %d family calls", calls.Load())
	}
	if result, err := m.get(context.Background(), false); err != nil || result.IPv4.Status != "ok" || calls.Load() != 2 {
		t.Fatal("canceled caller poisoned cached result")
	}
}

func TestPublicIPApplicationCancellationDoesNotPublish(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := newPublicIPManager(ctx)
	started := make(chan struct{}, 2)
	m.lookup = func(ctx context.Context, _ int) publicIPFamilyResult {
		started <- struct{}{}
		<-ctx.Done()
		return publicIPFamilyResult{Status: "unavailable", Error: "fixture canceled"}
	}
	done := make(chan error, 1)
	go func() { _, err := m.get(context.Background(), false); done <- err }()
	for range 2 {
		<-started
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("application cancellation: %v", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.expires.IsZero() || m.inflight != nil || m.cached.CheckedAt != "" {
		t.Fatal("shutdown published a canceled lookup as a current result")
	}
}
