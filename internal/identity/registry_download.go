package identity

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"networktoolkit/internal/model"
)

const registryDownloadTimeout = 20 * time.Second

type registryDownloadAttempt func(context.Context, registrySource, bool) ([]byte, error)

func officialRegistryURL(value *url.URL) bool {
	return value != nil && value.Scheme == "https" && value.Host == "standards-oui.ieee.org" && value.User == nil
}

func registryHTTPClient(compatibility bool) *http.Client {
	config := &tls.Config{MinVersion: tls.VersionTLS12}
	transport := &http.Transport{
		Proxy:                  http.ProxyFromEnvironment,
		DialContext:            (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		TLSClientConfig:        config,
		TLSHandshakeTimeout:    5 * time.Second,
		ResponseHeaderTimeout:  10 * time.Second,
		MaxResponseHeaderBytes: 64 << 10,
		DisableKeepAlives:      true,
		ForceAttemptHTTP2:      !compatibility,
	}
	if compatibility {
		// Some TLS endpoints/intermediaries answer a modern ClientHello with
		// plaintext HTTP. Retry this download with a smaller TLS 1.2 hello and
		// HTTP/1.1. Certificate/hostname checks and proxy routing stay enabled.
		config.MaxVersion = tls.VersionTLS12
		config.CurvePreferences = []tls.CurveID{tls.X25519, tls.CurveP256, tls.CurveP384}
		config.NextProtos = []string{"http/1.1"}
		transport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	}
	return &http.Client{
		Transport: transport,
		Timeout:   registryDownloadTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 || !officialRegistryURL(req.URL) {
				return errors.New("vendor update redirected outside the official HTTPS registry")
			}
			return nil
		},
	}
}

func downloadRegistry(ctx context.Context, source registrySource) ([]byte, error) {
	return downloadRegistryUsingFallback(ctx, source, func(ctx context.Context, source registrySource, compatibility bool) ([]byte, error) {
		client := registryHTTPClient(compatibility)
		defer client.CloseIdleConnections()
		return fetchRegistry(ctx, source, client)
	}, registryPlatformFallback())
}

func downloadRegistryUsing(ctx context.Context, source registrySource, attempt registryDownloadAttempt) ([]byte, error) {
	return downloadRegistryUsingFallback(ctx, source, attempt, nil)
}

func downloadRegistryUsingFallback(ctx context.Context, source registrySource, attempt registryDownloadAttempt, fallback func(context.Context, registrySource) ([]byte, error)) ([]byte, error) {
	location, err := url.Parse(source.url)
	if err != nil || !officialRegistryURL(location) {
		return nil, errors.New("vendor updates must use the official HTTPS registry")
	}
	ctx, cancel := context.WithTimeout(ctx, registryDownloadTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := attempt(ctx, source, false)
	if err == nil {
		return data, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if !registryTLSCompatibilityError(err) {
		return nil, err
	}
	data, err = attempt(ctx, source, true)
	if err == nil {
		return data, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if registryTLSCompatibilityError(err) {
		if fallback != nil {
			data, nativeErr := fallback(ctx, source)
			if nativeErr == nil {
				return data, nil
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("Windows HTTPS download also failed: %w (earlier Go TLS failure: %v)", nativeErr, err)
		}
		return nil, fmt.Errorf("a secure HTTPS connection could not be established after a compatibility retry. A proxy, network filter, or the download service may be interrupting TLS. Check HTTPS access to standards-oui.ieee.org, or download the IEEE CSV in your browser and use Import IEEE CSV: %w", err)
	}
	return nil, err
}

func registryTLSCompatibilityError(err error) bool {
	var header tls.RecordHeaderError
	return errors.Is(err, http.ErrSchemeMismatch) || errors.As(err, &header) ||
		strings.Contains(err.Error(), "net/http: TLS handshake timeout") ||
		strings.Contains(err.Error(), "remote error: tls: handshake failure") ||
		strings.Contains(err.Error(), "remote error: tls: protocol version not supported")
}

func fetchRegistry(ctx context.Context, source registrySource, client *http.Client) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.url, nil)
	if err != nil {
		return nil, err
	}
	if !officialRegistryURL(req.URL) {
		return nil, errors.New("vendor updates must use the official HTTPS registry")
	}
	req.Header.Set("User-Agent", "Network-Toolkit/"+model.Version)
	req.Header.Set("Accept", "text/csv, application/octet-stream;q=0.9")
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, registryStatusError(response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxCSVBytes+1))
	if err == nil && len(data) > maxCSVBytes {
		err = errors.New("registry download is too large")
	}
	return data, err
}

func registryStatusError(status int) error {
	if status == http.StatusForbidden || status == http.StatusTeapot || status == http.StatusTooManyRequests {
		return fmt.Errorf("IEEE refused the download (HTTP %d). Try again later, or download the IEEE CSV in your browser and use Import IEEE CSV", status)
	}
	return fmt.Errorf("IEEE returned HTTP %d", status)
}
