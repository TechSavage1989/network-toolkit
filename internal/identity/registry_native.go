package identity

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"networktoolkit/internal/model"
)

// WinHTTP settings are kept in the tested driver, away from the syscall layer.
const (
	registryNativeAutomaticProxy        = 4
	registryNativeDirect                = 1
	registryNativeNamedProxy            = 3
	registryNativeSecureProtocols       = 84
	registryNativeTLS12                 = 0x800
	registryNativeTLS13                 = 0x2000
	registryNativeRedirectPolicy        = 88
	registryNativeNeverRedirect         = 0
	registryNativeDisableFeature        = 63
	registryNativeDisableCookiesAndAuth = 1 | 4
)

type registryNativeProxy struct {
	mode   uint32
	server string
}

func registryNativeProxyConfig(proxy *url.URL, explicitEnvironment bool) (registryNativeProxy, error) {
	if proxy == nil {
		if explicitEnvironment {
			// ProxyFromEnvironment returned nil because NO_PROXY matched.
			return registryNativeProxy{mode: registryNativeDirect}, nil
		}
		return registryNativeProxy{mode: registryNativeAutomaticProxy}, nil
	}
	if proxy.Scheme != "http" || proxy.Host == "" || proxy.User != nil || (proxy.Path != "" && proxy.Path != "/") || proxy.RawQuery != "" || proxy.Fragment != "" {
		return registryNativeProxy{}, errors.New("the environment proxy cannot be represented by the Windows fallback; its route and credentials were not changed")
	}
	return registryNativeProxy{mode: registryNativeNamedProxy, server: proxy.Host}, nil
}

type registryNativeSession interface {
	SetOption(uint32, uint32) error
	OpenRequest(host, path string) (registryNativeRequest, error)
	Close()
}

type registryNativeRequest interface {
	io.Reader
	SetOption(uint32, uint32) error
	SetTimeouts(int) error
	Send(string) error
	Receive() error
	Status() (int, error)
	Location() (string, error)
	Close()
}

type registryNativeOpen func(string, registryNativeProxy) (registryNativeSession, error)

func downloadRegistryNative(ctx context.Context, source registrySource, proxy registryNativeProxy, open registryNativeOpen) ([]byte, error) {
	location, err := url.Parse(source.url)
	if err != nil || !officialRegistryURL(location) {
		return nil, errors.New("vendor updates must use the official HTTPS registry")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, registryDownloadTimeout)
	defer cancel()
	// Synchronous WinHTTP handles must never be closed from a cancelling thread.
	// The worker owns every handle and buffer until its native calls return.
	// Cancellation returns to the caller immediately; the worker checks context
	// between calls, uses finite native timeouts and then closes its own handles.
	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		data, err := registryNativeWork(ctx, location, proxy, open)
		done <- result{data, err}
	}()
	select {
	case value := <-done:
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return value.data, value.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func registryNativeWork(ctx context.Context, location *url.URL, proxy registryNativeProxy, open registryNativeOpen) ([]byte, error) {
	data, err := registryNativeWorkTLS(ctx, location, proxy, open, registryNativeTLS12|registryNativeTLS13)
	if err == nil || ctx.Err() != nil || !registryNativeChannelFailure(err) {
		return data, err
	}
	// Only a positively identified Windows channel failure gets one native
	// TLS 1.2 retry. Certificate/unknown failures stop here. The first session
	// is already closed, and the retry shares the original deadline and route.
	data, retryErr := registryNativeWorkTLS(ctx, location, proxy, open, registryNativeTLS12)
	if retryErr != nil {
		return nil, fmt.Errorf("Windows TLS 1.2 retry failed: %w (initial failure: %v)", retryErr, err)
	}
	return data, nil
}

func registryNativeWorkTLS(ctx context.Context, location *url.URL, proxy registryNativeProxy, open registryNativeOpen, protocols uint32) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	session, err := open("Network-Toolkit/"+model.Version, proxy)
	if err != nil {
		return nil, err
	}
	defer session.Close()
	if err := session.SetOption(registryNativeSecureProtocols, protocols); err != nil {
		// Older supported Windows builds may not recognize the TLS 1.3 flag.
		if err := session.SetOption(registryNativeSecureProtocols, registryNativeTLS12); err != nil {
			return nil, err
		}
	}
	for redirects := 0; ; redirects++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data, next, err := registryNativeOne(ctx, session, location)
		if err != nil {
			return nil, err
		}
		if next == nil {
			return data, nil
		}
		if redirects >= 2 {
			return nil, errors.New("too many vendor registry redirects")
		}
		location = next
	}
}

func registryNativeOne(ctx context.Context, session registryNativeSession, location *url.URL) ([]byte, *url.URL, error) {
	if !officialRegistryURL(location) {
		return nil, nil, errors.New("vendor update redirected outside the official HTTPS registry")
	}
	request, err := session.OpenRequest(location.Hostname(), location.RequestURI())
	if err != nil {
		return nil, nil, err
	}
	defer request.Close()
	if err := request.SetOption(registryNativeRedirectPolicy, registryNativeNeverRedirect); err != nil {
		return nil, nil, err
	}
	if err := request.SetOption(registryNativeDisableFeature, registryNativeDisableCookiesAndAuth); err != nil {
		return nil, nil, err
	}
	if err := registryNativeTimeout(ctx, request); err != nil {
		return nil, nil, err
	}
	if err := request.Send("Accept: text/csv, application/octet-stream;q=0.9\r\nAccept-Encoding: identity\r\n"); err != nil {
		return nil, nil, err
	}
	if err := registryNativeTimeout(ctx, request); err != nil {
		return nil, nil, err
	}
	if err := request.Receive(); err != nil {
		return nil, nil, err
	}
	status, err := request.Status()
	if err != nil {
		return nil, nil, err
	}
	if status == 301 || status == 302 || status == 303 || status == 307 || status == 308 {
		value, err := request.Location()
		if err != nil {
			return nil, nil, err
		}
		if value == "" {
			return nil, nil, errors.New("empty vendor registry redirect")
		}
		next, err := location.Parse(value)
		if err != nil || !officialRegistryURL(next) {
			return nil, nil, errors.New("vendor update redirected outside the official HTTPS registry")
		}
		return nil, next, nil
	}
	if status != http.StatusOK {
		return nil, nil, registryStatusError(status)
	}
	data, err := io.ReadAll(io.LimitReader(registryNativeReader{ctx, request}, maxCSVBytes+1))
	if err == nil && len(data) > maxCSVBytes {
		err = errors.New("registry download is too large")
	}
	return data, nil, err
}

func registryNativeTimeout(ctx context.Context, request registryNativeRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	remaining := 5 * time.Second
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < remaining {
		remaining = time.Until(deadline)
	}
	if remaining <= 0 {
		return context.DeadlineExceeded
	}
	millis := max(1, int(remaining/time.Millisecond))
	if err := request.SetTimeouts(millis); err != nil {
		return fmt.Errorf("Windows HTTPS timeout configuration: %w", err)
	}
	return nil
}

type registryNativeReader struct {
	ctx     context.Context
	request registryNativeRequest
}

func (reader registryNativeReader) Read(buffer []byte) (int, error) {
	if err := registryNativeTimeout(reader.ctx, reader.request); err != nil {
		return 0, err
	}
	return reader.request.Read(buffer)
}
