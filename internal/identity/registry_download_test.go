package identity

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
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

func registryTestClient(server *httptest.Server, compatibility, trust bool) *http.Client {
	client := registryHTTPClient(compatibility)
	transport := client.Transport.(*http.Transport)
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	transport.TLSClientConfig.ServerName = server.Certificate().DNSNames[0]
	if trust {
		transport.TLSClientConfig.RootCAs = x509.NewCertPool()
		transport.TLSClientConfig.RootCAs.AddCert(server.Certificate())
	}
	return client
}

func TestRegistryPlainHTTPDuringTLSRetriesWithVerifiedTLS12(t *testing.T) {
	data := csvFixture("MA-L", "001122", "TLS compatibility fixture")
	var rejected, accepted atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || r.TLS.Version != tls.VersionTLS12 || r.ProtoMajor != 1 {
			t.Error("compatibility retry did not remain on TLS 1.2 and HTTP/1.1")
		}
		if r.Header.Get("User-Agent") != "Network-Toolkit/"+model.Version {
			t.Error("stale downloader user agent")
		}
		accepted.Add(1)
		_, _ = w.Write(data)
	}))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.TLS = &tls.Config{GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
		for _, version := range hello.SupportedVersions {
			if version > tls.VersionTLS12 {
				rejected.Add(1)
				_, _ = io.WriteString(hello.Conn, "HTTP/1.1 400 Bad Request\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
				return nil, errors.New("fixture endpoint rejects modern TLS with plaintext HTTP")
			}
		}
		return nil, nil
	}}
	server.StartTLS()
	defer server.Close()
	var modes []bool
	var firstError error
	got, err := downloadRegistryUsing(context.Background(), registrySources[0], func(ctx context.Context, source registrySource, compatibility bool) ([]byte, error) {
		modes = append(modes, compatibility)
		client := registryTestClient(server, compatibility, true)
		defer client.CloseIdleConnections()
		result, err := fetchRegistry(ctx, source, client)
		if !compatibility {
			firstError = err
		}
		return result, err
	})
	if err != nil || !bytes.Equal(got, data) || !errors.Is(firstError, http.ErrSchemeMismatch) || !reflect.DeepEqual(modes, []bool{false, true}) || rejected.Load() != 1 || accepted.Load() != 1 {
		t.Fatalf("HTTPS recovery: modes=%v rejected=%d accepted=%d first=%v error=%v", modes, rejected.Load(), accepted.Load(), firstError, err)
	}
	if _, err := parseCSV(got, "MA-L"); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryDownloadRetainsCertificateChecksOnBothProfiles(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("untrusted connection reached the HTTP handler")
	}))
	defer server.Close()
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	for _, compatibility := range []bool{false, true} {
		client := registryTestClient(server, compatibility, false)
		_, err := fetchRegistry(context.Background(), registrySources[0], client)
		client.CloseIdleConnections()
		var certificateError *tls.CertificateVerificationError
		if !errors.As(err, &certificateError) || registryTLSCompatibilityError(err) {
			t.Fatalf("untrusted certificate was accepted/retryable with compatibility=%v: %v", compatibility, err)
		}
		transport := client.Transport.(*http.Transport)
		if transport.TLSClientConfig.InsecureSkipVerify || transport.TLSClientConfig.MinVersion < tls.VersionTLS12 || client.Jar != nil {
			t.Fatal("download profile weakened verification or enabled cookies")
		}
	}
}

func TestRegistryRetryUsesOneDeadlineAndHonorsCancellation(t *testing.T) {
	var deadlines []time.Time
	_, err := downloadRegistryUsing(context.Background(), registrySources[0], func(ctx context.Context, _ registrySource, _ bool) ([]byte, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > registryDownloadTimeout {
			t.Error("download was not bounded")
		}
		deadlines = append(deadlines, deadline)
		return nil, &url.Error{Op: "Get", URL: registrySources[0].url, Err: http.ErrSchemeMismatch}
	})
	if len(deadlines) != 2 || !deadlines[0].Equal(deadlines[1]) || !errors.Is(err, http.ErrSchemeMismatch) || !strings.Contains(err.Error(), "Import IEEE CSV") {
		t.Fatalf("retry did not preserve its deadline/actionable error: %v %v", deadlines, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	_, err = downloadRegistryUsing(ctx, registrySources[0], func(context.Context, registrySource, bool) ([]byte, error) {
		calls++
		cancel()
		return nil, http.ErrSchemeMismatch
	})
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("cancellation started a retry: calls=%d error=%v", calls, err)
	}
}

type registryRoundTrip func(*http.Request) (*http.Response, error)

func (f registryRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestRegistryRejectsHTTPExternalRedirectsAndRefusalsWithoutRetry(t *testing.T) {
	for _, location := range []string{"http://standards-oui.ieee.org/oui/oui.csv", "https://unrelated.invalid/oui.csv", "https://standards-oui.ieee.org:444/oui.csv", "https://user:password@standards-oui.ieee.org/oui.csv"} {
		client := registryHTTPClient(false)
		calls := 0
		client.Transport = registryRoundTrip(func(req *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {location}}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
		})
		_, err := fetchRegistry(context.Background(), registrySources[0], client)
		if err == nil || calls != 1 || registryTLSCompatibilityError(err) {
			t.Fatalf("unsafe redirect followed: %s calls=%d error=%v", location, calls, err)
		}
		if _, err := downloadRegistryUsing(context.Background(), registrySource{"MA-L", location}, func(context.Context, registrySource, bool) ([]byte, error) {
			t.Error("unsafe initial URL reached downloader")
			return nil, nil
		}); err == nil {
			t.Fatal("unsafe initial URL accepted")
		}
	}
	for _, status := range []int{403, 418, 429, 500} {
		calls := 0
		_, err := downloadRegistryUsing(context.Background(), registrySources[0], func(ctx context.Context, source registrySource, _ bool) ([]byte, error) {
			calls++
			client := registryHTTPClient(false)
			client.Transport = registryRoundTrip(func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("blocked")), Request: req}, nil
			})
			return fetchRegistry(ctx, source, client)
		})
		if err == nil || calls != 1 || !strings.Contains(err.Error(), fmt.Sprint(status)) {
			t.Fatalf("HTTP refusal retried/ignored: status=%d calls=%d error=%v", status, calls, err)
		}
	}
}

func TestRegistryTLSFailurePreservesDatabaseOnDiskAndInMemory(t *testing.T) {
	s := fixtureStore(t)
	if err := s.persist(context.Background(), s.current.Load()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.dir, DatabaseFilename)
	before, _ := os.ReadFile(path)
	previous := s.current.Load()
	calls := 0
	err := s.update(context.Background(), func(ctx context.Context, source registrySource) ([]byte, error) {
		return downloadRegistryUsing(ctx, source, func(context.Context, registrySource, bool) ([]byte, error) {
			calls++
			return nil, http.ErrSchemeMismatch
		})
	})
	after, _ := os.ReadFile(path)
	if err == nil || calls != 2 || s.current.Load() != previous || !bytes.Equal(before, after) || s.status().Updating || strings.Count(err.Error(), "previous database was kept") != 1 {
		t.Fatalf("failed secure download changed database or left an invalid status: calls=%d error=%v", calls, err)
	}
}

func TestRegistryLiveHTTPS(t *testing.T) {
	if os.Getenv("NETWORK_TOOLKIT_IEEE_LIVE_TEST") != "1" {
		t.Skip("explicit live IEEE download check; ordinary tests use local fixtures")
	}
	db, err := decodeDatabase(bundledBytes, true)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, source := range registrySources {
		t.Run(source.registry, func(t *testing.T) {
			data, err := downloadRegistry(ctx, source)
			if err != nil {
				t.Fatal(err)
			}
			rows, err := parseCSV(data, source.registry)
			if err != nil || len(rows) < db.counts[source.registry]*3/4 {
				t.Fatalf("invalid/unexpectedly small live registry: entries=%d error=%v", len(rows), err)
			}
			t.Logf("verified HTTPS CSV: bytes=%d entries=%d", len(data), len(rows))
		})
	}
}
