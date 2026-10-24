package identity

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"networktoolkit/internal/model"
)

type registryNativeFixture struct {
	requests         []*registryNativeFixtureRequest
	options          [][2]uint32
	opened           [][2]string
	closed           atomic.Int32
	closedSignal     chan struct{}
	tls13Unsupported bool
}

type registryNativeFixtureRequest struct {
	reader   io.Reader
	status   int
	location string
	options  [][2]uint32
	timeouts []int
	headers  string
	send     func() error
	closed   atomic.Int32
}

func (fixture *registryNativeFixture) SetOption(option, value uint32) error {
	fixture.options = append(fixture.options, [2]uint32{option, value})
	if fixture.tls13Unsupported && value&registryNativeTLS13 != 0 {
		return errors.New("unsupported TLS 1.3 option")
	}
	return nil
}
func (fixture *registryNativeFixture) OpenRequest(host, path string) (registryNativeRequest, error) {
	index := len(fixture.opened)
	fixture.opened = append(fixture.opened, [2]string{host, path})
	if index >= len(fixture.requests) {
		return nil, errors.New("unexpected extra native request")
	}
	return fixture.requests[index], nil
}
func (fixture *registryNativeFixture) Close() {
	fixture.closed.Add(1)
	if fixture.closedSignal != nil {
		close(fixture.closedSignal)
	}
}
func (request *registryNativeFixtureRequest) SetOption(option, value uint32) error {
	request.options = append(request.options, [2]uint32{option, value})
	return nil
}
func (request *registryNativeFixtureRequest) SetTimeouts(value int) error {
	request.timeouts = append(request.timeouts, value)
	return nil
}
func (request *registryNativeFixtureRequest) Send(headers string) error {
	request.headers = headers
	if request.send != nil {
		return request.send()
	}
	return nil
}
func (request *registryNativeFixtureRequest) Receive() error            { return nil }
func (request *registryNativeFixtureRequest) Status() (int, error)      { return request.status, nil }
func (request *registryNativeFixtureRequest) Location() (string, error) { return request.location, nil }
func (request *registryNativeFixtureRequest) Read(value []byte) (int, error) {
	if request.reader == nil {
		return 0, io.EOF
	}
	return request.reader.Read(value)
}
func (request *registryNativeFixtureRequest) Close() { request.closed.Add(1) }

func (fixture *registryNativeFixture) open(agent string, _ registryNativeProxy) (registryNativeSession, error) {
	if agent != "Network-Toolkit/"+model.Version {
		return nil, errors.New("incorrect native user agent")
	}
	return fixture, nil
}

func TestRegistryWindowsFallbackFollowsTLSFailuresWithSameDeadline(t *testing.T) {
	var deadlines []time.Time
	var modes []bool
	calls := 0
	want := csvFixture("MA-L", "001122", "Native HTTPS fixture")
	got, err := downloadRegistryUsingFallback(context.Background(), registrySources[0], func(ctx context.Context, _ registrySource, compatibility bool) ([]byte, error) {
		deadline, _ := ctx.Deadline()
		deadlines = append(deadlines, deadline)
		modes = append(modes, compatibility)
		return nil, http.ErrSchemeMismatch
	}, func(ctx context.Context, source registrySource) ([]byte, error) {
		calls++
		deadline, _ := ctx.Deadline()
		deadlines = append(deadlines, deadline)
		if source != registrySources[0] {
			t.Error("native attempt changed the registry source")
		}
		return want, nil
	})
	if err != nil || !bytes.Equal(got, want) || calls != 1 || !reflect.DeepEqual(modes, []bool{false, true}) || len(deadlines) != 3 || !deadlines[0].Equal(deadlines[2]) || !deadlines[1].Equal(deadlines[2]) {
		t.Fatalf("native fallback order/deadline: calls=%d modes=%v deadlines=%v error=%v", calls, modes, deadlines, err)
	}
}

func TestRegistryNativeFallbackDoesNotRetryCertificateOrHTTPRefusals(t *testing.T) {
	certificate := &tls.CertificateVerificationError{Err: errors.New("untrusted certificate")}
	for _, failure := range []error{certificate, registryStatusError(403), registryStatusError(418), registryStatusError(429), context.Canceled} {
		_, err := downloadRegistryUsingFallback(context.Background(), registrySources[0], func(context.Context, registrySource, bool) ([]byte, error) { return nil, failure }, func(context.Context, registrySource) ([]byte, error) {
			t.Error("native fallback bypassed a non-TLS refusal")
			return nil, nil
		})
		if !errors.Is(err, failure) {
			t.Fatalf("lost original refusal: %v", err)
		}
	}
}

func TestRegistryNativeClientEnforcesSecurityLimitsAndOfficialRedirects(t *testing.T) {
	want := csvFixture("MA-L", "001122", "Windows fixture")
	first := &registryNativeFixtureRequest{status: 302, location: "/oui/current.csv"}
	last := &registryNativeFixtureRequest{status: 200, reader: bytes.NewReader(want)}
	fixture := &registryNativeFixture{requests: []*registryNativeFixtureRequest{first, last}, tls13Unsupported: true}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	got, err := downloadRegistryNative(ctx, registrySources[0], registryNativeProxy{mode: registryNativeAutomaticProxy}, fixture.open)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("native download: %v", err)
	}
	if !reflect.DeepEqual(fixture.options, [][2]uint32{{registryNativeSecureProtocols, registryNativeTLS12 | registryNativeTLS13}, {registryNativeSecureProtocols, registryNativeTLS12}}) {
		t.Fatal("native TLS policy changed", fixture.options)
	}
	if !reflect.DeepEqual(fixture.opened, [][2]string{{"standards-oui.ieee.org", "/oui/oui.csv"}, {"standards-oui.ieee.org", "/oui/current.csv"}}) {
		t.Fatal("native request left official destination", fixture.opened)
	}
	for _, request := range fixture.requests {
		if !reflect.DeepEqual(request.options, [][2]uint32{{registryNativeRedirectPolicy, registryNativeNeverRedirect}, {registryNativeDisableFeature, registryNativeDisableCookiesAndAuth}}) {
			t.Fatal("unsafe native request options", request.options)
		}
		if len(request.timeouts) < 2 {
			t.Fatal("native network phases lack timeouts")
		}
		for _, timeout := range request.timeouts {
			if timeout < 1 || timeout > 100 {
				t.Fatal("native timeout exceeded remaining deadline", timeout)
			}
		}
		if !strings.Contains(request.headers, "Accept-Encoding: identity") || request.closed.Load() != 1 {
			t.Fatal("native headers/handle lifetime changed")
		}
	}
	if fixture.closed.Load() != 1 {
		t.Fatal("native session leaked")
	}
}

func TestRegistryNativeRejectsUnsafeRedirectsAndTooManyRedirects(t *testing.T) {
	for _, location := range []string{"http://standards-oui.ieee.org/oui/oui.csv", "https://other.invalid/oui.csv", "https://standards-oui.ieee.org:444/oui.csv", "https://user:secret@standards-oui.ieee.org/oui.csv", ""} {
		request := &registryNativeFixtureRequest{status: 302, location: location}
		fixture := &registryNativeFixture{requests: []*registryNativeFixtureRequest{request}}
		_, err := downloadRegistryNative(context.Background(), registrySources[0], registryNativeProxy{}, fixture.open)
		if err == nil || len(fixture.opened) != 1 || request.closed.Load() != 1 || fixture.closed.Load() != 1 {
			t.Fatalf("unsafe native redirect followed: %q %v", location, err)
		}
	}
	fixture := &registryNativeFixture{requests: []*registryNativeFixtureRequest{{status: 302, location: "/oui/again.csv"}, {status: 302, location: "/oui/again.csv"}, {status: 302, location: "/oui/again.csv"}}}
	_, err := downloadRegistryNative(context.Background(), registrySources[0], registryNativeProxy{}, fixture.open)
	if err == nil || len(fixture.opened) != 3 {
		t.Fatal("native redirects were not bounded", err)
	}
	_, err = downloadRegistryNative(context.Background(), registrySource{"MA-L", "http://standards-oui.ieee.org/oui/oui.csv"}, registryNativeProxy{}, func(string, registryNativeProxy) (registryNativeSession, error) {
		t.Error("unsafe source opened native session")
		return nil, nil
	})
	if err == nil {
		t.Fatal("native initial URL was not checked")
	}
}

func TestRegistryNativeRejectsRefusalsOversizeAndCertificateFailures(t *testing.T) {
	certificate := errors.New("native certificate verification failed")
	for _, request := range []*registryNativeFixtureRequest{
		{status: 403}, {status: 418}, {status: 429},
		{status: 200, reader: io.LimitReader(registryZeroReader{}, maxCSVBytes+1)},
		{send: func() error { return certificate }},
	} {
		fixture := &registryNativeFixture{requests: []*registryNativeFixtureRequest{request}}
		data, err := downloadRegistryNative(context.Background(), registrySources[0], registryNativeProxy{}, fixture.open)
		if err == nil || len(fixture.opened) != 1 || fixture.closed.Load() != 1 || request.closed.Load() != 1 {
			t.Fatal("native failure accepted/retried or leaked a handle", err)
		}
		if request.send != nil && !errors.Is(err, certificate) {
			t.Fatal("native certificate error was lost", err)
		}
		if request.reader != nil && len(data) > maxCSVBytes+1 {
			t.Fatal("unbounded native response")
		}
	}
}

func TestRegistryNativeRetriesOnlyAnIdentifiedChannelFailure(t *testing.T) {
	for _, flags := range []uint32{registrySecureChannel, 0, registrySecureUntrustedCA, registrySecureWrongHost, registrySecureInvalidDate, registrySecureRevoked, registrySecureRevocationUnknown, registrySecureInvalidCert, registrySecureChannel | registrySecureUntrustedCA, 0x40} {
		t.Run(fmt.Sprintf("%08X", flags), func(t *testing.T) {
			cause := errors.New("native TLS failure fixture")
			failure := &registryNativeSecureError{operation: "WinHttpSendRequest", flags: flags, cause: cause}
			first := &registryNativeFixture{requests: []*registryNativeFixtureRequest{{send: func() error { return failure }}}}
			want := csvFixture("MA-L", "001122", "TLS 1.2 fixture")
			second := &registryNativeFixture{requests: []*registryNativeFixtureRequest{{status: 200, reader: bytes.NewReader(want)}}}
			calls := 0
			open := func(agent string, proxy registryNativeProxy) (registryNativeSession, error) {
				calls++
				if calls == 1 {
					return first.open(agent, proxy)
				}
				if first.closed.Load() != 1 || first.requests[0].closed.Load() != 1 {
					t.Fatal("TLS retry began before the previous handles closed")
				}
				return second.open(agent, proxy)
			}
			got, err := downloadRegistryNative(context.Background(), registrySources[0], registryNativeProxy{}, open)
			if flags == registrySecureChannel {
				if err != nil || !bytes.Equal(got, want) || calls != 2 || !reflect.DeepEqual(second.options, [][2]uint32{{registryNativeSecureProtocols, registryNativeTLS12}}) {
					t.Fatalf("native TLS 1.2 recovery: calls=%d options=%v error=%v", calls, second.options, err)
				}
			} else if !errors.Is(err, cause) || calls != 1 {
				t.Fatalf("retried a certificate/unknown failure: calls=%d error=%v", calls, err)
			}
		})
	}
}

func TestRegistryNativeTLSFailureExplainsAllReportedReasons(t *testing.T) {
	for _, item := range []struct {
		flags uint32
		want  string
	}{
		{registrySecureUntrustedCA, "authority is not trusted"},
		{registrySecureWrongHost, "does not match standards-oui.ieee.org"},
		{registrySecureInvalidDate, "PC date and time"},
		{registrySecureRevoked, "has been revoked"},
		{registrySecureRevocationUnknown, "revocation status could not be checked"},
		{registrySecureInvalidCert, "certificate is invalid"},
		{registrySecureChannel, "TLS security channel"},
		{0, "without providing"},
		{0x40, "additional TLS failure"},
	} {
		message := (&registryNativeSecureError{operation: "WinHttpSendRequest", flags: item.flags}).Error()
		if !strings.Contains(message, item.want) || !strings.Contains(message, fmt.Sprintf("TLS flags 0x%08X", item.flags)) || !strings.Contains(message, "12175") {
			t.Fatal("missing native TLS diagnosis", message)
		}
	}
	combined := (&registryNativeSecureError{operation: "WinHttpReceiveResponse", flags: registrySecureUntrustedCA | registrySecureWrongHost}).Error()
	if !strings.Contains(combined, "authority is not trusted") || !strings.Contains(combined, "does not match") {
		t.Fatal("lost a combined TLS failure", combined)
	}
}

type registryZeroReader struct{}

func (registryZeroReader) Read(buffer []byte) (int, error) { clear(buffer); return len(buffer), nil }

func TestRegistryNativeCancellationReturnsWithoutClosingAnActiveSynchronousHandle(t *testing.T) {
	started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	request := &registryNativeFixtureRequest{send: func() error { close(started); <-release; return nil }}
	fixture := &registryNativeFixture{requests: []*registryNativeFixtureRequest{request}, closedSignal: finished}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := downloadRegistryNative(ctx, registrySources[0], registryNativeProxy{}, fixture.open)
		done <- err
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("native cancellation blocked the caller")
	}
	if request.closed.Load() != 0 || fixture.closed.Load() != 0 {
		t.Fatal("cancellation closed a synchronous handle from another thread")
	}
	close(release)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("native worker did not release its handles")
	}
	if request.closed.Load() != 1 || fixture.closed.Load() != 1 {
		t.Fatal("native worker leaked or double-closed a handle")
	}
}

func TestRegistryNativeProxyRoutingPreservesExplicitEnvironmentSettings(t *testing.T) {
	automatic, err := registryNativeProxyConfig(nil, false)
	if err != nil || automatic.mode != registryNativeAutomaticProxy {
		t.Fatal("Windows proxy discovery was not selected")
	}
	direct, err := registryNativeProxyConfig(nil, true)
	if err != nil || direct.mode != registryNativeDirect {
		t.Fatal("NO_PROXY was ignored")
	}
	location, _ := url.Parse("http://proxy.example.invalid:8080")
	named, err := registryNativeProxyConfig(location, true)
	if err != nil || named.mode != registryNativeNamedProxy || named.server != location.Host {
		t.Fatal("explicit proxy route was changed")
	}
	for _, value := range []string{"https://proxy.example.invalid:8443", "http://user:secret-canary@proxy.example.invalid:8080", "http://proxy.example.invalid/other"} {
		location, _ := url.Parse(value)
		_, err := registryNativeProxyConfig(location, true)
		if err == nil || strings.Contains(err.Error(), "secret-canary") {
			t.Fatal("unsupported proxy was bypassed or leaked credentials", err)
		}
	}
}

func TestRegistryNativeFailurePreservesStoredAndActiveDatabase(t *testing.T) {
	store := fixtureStore(t)
	if err := store.persist(context.Background(), store.current.Load()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.dir, DatabaseFilename)
	before, _ := os.ReadFile(path)
	previous := store.current.Load()
	calls := 0
	err := store.update(context.Background(), func(ctx context.Context, source registrySource) ([]byte, error) {
		return downloadRegistryUsingFallback(ctx, source, func(context.Context, registrySource, bool) ([]byte, error) {
			calls++
			return nil, http.ErrSchemeMismatch
		}, func(context.Context, registrySource) ([]byte, error) {
			calls++
			return nil, errors.New("native TLS failure")
		})
	})
	after, _ := os.ReadFile(path)
	if err == nil || calls != 3 || store.current.Load() != previous || !bytes.Equal(before, after) || store.status().Updating {
		t.Fatalf("native failure changed the database or left updating status: %v", err)
	}
}
