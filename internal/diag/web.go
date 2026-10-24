package diag

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

type Certificate struct {
	Subject            string   `json:"subject"`
	Issuer             string   `json:"issuer"`
	Serial             string   `json:"serial"`
	DNSNames           []string `json:"dnsNames"`
	IPAddresses        []string `json:"ipAddresses"`
	NotBefore          string   `json:"notBefore"`
	NotAfter           string   `json:"notAfter"`
	ExpiresInDays      int      `json:"expiresInDays"`
	SHA256             string   `json:"sha256"`
	SignatureAlgorithm string   `json:"signatureAlgorithm"`
	PublicKeyAlgorithm string   `json:"publicKeyAlgorithm"`
	IsCA               bool     `json:"isCA"`
}

func certificateDetails(c *x509.Certificate) Certificate {
	ips := make([]string, 0, len(c.IPAddresses))
	for _, ip := range c.IPAddresses {
		ips = append(ips, ip.String())
	}
	fp := sha256.Sum256(c.Raw)
	dns := c.DNSNames
	if dns == nil {
		dns = []string{}
	}
	return Certificate{Subject: c.Subject.String(), Issuer: c.Issuer.String(), Serial: c.SerialNumber.String(), DNSNames: dns, IPAddresses: ips, NotBefore: c.NotBefore.UTC().Format(time.RFC3339), NotAfter: c.NotAfter.UTC().Format(time.RFC3339), ExpiresInDays: int(time.Until(c.NotAfter).Hours() / 24), SHA256: strings.ToUpper(hex.EncodeToString(fp[:])), SignatureAlgorithm: c.SignatureAlgorithm.String(), PublicKeyAlgorithm: c.PublicKeyAlgorithm.String(), IsCA: c.IsCA}
}

func tlsVersion(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	}
	return fmt.Sprintf("0x%04x", v)
}

func tlsMetadata(ctx context.Context, state tls.ConnectionState, name string, verify bool) map[string]any {
	chain := make([]Certificate, 0, len(state.PeerCertificates))
	for _, c := range state.PeerCertificates {
		chain = append(chain, certificateDetails(c))
	}
	valid := false
	validationError := ""
	warnings := []string{}
	if len(state.PeerCertificates) == 0 {
		validationError = "Server returned no certificates"
	} else {
		var err error
		if _, selected := platform.SourceBindingFromContext(ctx); selected {
			err = platform.VerifyOfflineServerCertificate(ctx, state.PeerCertificates, name)
			warnings = append(warnings, "Selected-source certificate validation uses locally available trust data only; missing issuers or roots are not downloaded through an unbound system connection")
		} else {
			intermediates := x509.NewCertPool()
			for _, c := range state.PeerCertificates[1:] {
				intermediates.AddCert(c)
			}
			_, err = state.PeerCertificates[0].Verify(x509.VerifyOptions{DNSName: name, Intermediates: intermediates})
		}
		valid = err == nil
		validationError = safeError(err)
		if time.Until(state.PeerCertificates[0].NotAfter) < 30*24*time.Hour {
			warnings = append(warnings, "Leaf certificate expires within 30 days or has already expired")
		}
	}
	if state.Version < tls.VersionTLS12 {
		warnings = append(warnings, "Server negotiated a legacy TLS version")
	}
	if !verify {
		warnings = append(warnings, "The diagnostic connection was allowed to collect certificates without enforcing validation; the separate validation result is shown")
	}
	return map[string]any{"version": tlsVersion(state.Version), "cipher": tls.CipherSuiteName(state.CipherSuite), "serverName": name, "alpn": state.NegotiatedProtocol, "verified": valid, "verificationError": validationError, "verificationEnforced": verify, "certificates": chain, "warnings": warnings}
}

func tlsCheck(ctx context.Context, req model.Request, emit model.Emit) (model.Result, error) {
	host, err := validateHost(req.Target)
	if err != nil {
		return model.Result{}, err
	}
	port, err := intOption(req.Options, "port", 443, 1, 65535)
	if err != nil {
		return model.Result{}, err
	}
	timeout, err := durationOption(req.Options, "timeoutMs", 5000, 100, 30000)
	if err != nil {
		return model.Result{}, err
	}
	name := stringOption(req.Options, "serverName", host)
	if name == "" {
		name = host
	}
	if _, err = validateHost(name); err != nil {
		return model.Result{}, fmt.Errorf("server name: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	started := time.Now()
	conn, err := diagnosticDialContext(ctx, "tcp", net.JoinHostPort(host, fmt.Sprint(port)), timeout)
	if err != nil {
		return model.Result{}, fmt.Errorf("TCP connection: %w", err)
	}
	defer conn.Close()
	connected := time.Now()
	// This tool deliberately collects the presented certificate chain even when
	// it is invalid. tlsMetadata performs trust, name, and time validation and
	// always exposes the independent result to the caller.
	tc := tls.Client(conn, &tls.Config{ServerName: name, InsecureSkipVerify: true, MinVersion: tls.VersionTLS10, NextProtos: []string{"h2", "http/1.1"}})
	if err = tc.HandshakeContext(ctx); err != nil {
		return model.Result{}, fmt.Errorf("TLS handshake: %w", err)
	}
	state := tc.ConnectionState()
	data := tlsMetadata(ctx, state, name, false)
	data["target"] = host
	data["port"] = port
	data["remoteAddress"] = conn.RemoteAddr().String()
	data["tcpMs"] = ms(connected.Sub(started))
	data["handshakeMs"] = ms(time.Since(connected))
	data["totalMs"] = ms(time.Since(started))
	valid, _ := data["verified"].(bool)
	level, summary := "success", "Certificate validation passed"
	if !valid {
		level = "warning"
		summary = "TLS handshake succeeded; certificate validation failed"
	}
	event(emit, "tls", level, summary, data)
	return model.Result{Summary: summary, Data: data}, nil
}

type HTTPHop struct {
	URL           string   `json:"url"`
	Method        string   `json:"method"`
	RemoteAddress string   `json:"remoteAddress"`
	DNSMs         *float64 `json:"dnsMs"`
	ConnectMs     *float64 `json:"connectMs"`
	TLSMs         *float64 `json:"tlsMs"`
	FirstByteMs   *float64 `json:"firstByteMs"`
	TotalMs       float64  `json:"totalMs"`
	StatusCode    int      `json:"statusCode"`
	Status        string   `json:"status"`
	Location      string   `json:"location,omitempty"`
	Protocol      string   `json:"protocol"`
	Error         string   `json:"error,omitempty"`
}

type HTTPHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func cleanHeaders(h http.Header) []HTTPHeader {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rows := make([]HTTPHeader, 0, len(keys))
	bytes := 0
	for _, k := range keys {
		// A troubleshooting report must not export authentication cookies or
		// credentials returned by an endpoint.
		value := strings.Join(h.Values(k), ", ")
		if strings.EqualFold(k, "Set-Cookie") || strings.EqualFold(k, "Authorization") || strings.EqualFold(k, "Proxy-Authorization") {
			value = "[redacted]"
		}
		if len(value) > 4096 {
			value = value[:4096] + " [truncated]"
		}
		bytes += len(k) + len(value)
		if bytes > 64*1024 {
			rows = append(rows, HTTPHeader{Name: "Note", Value: "Additional headers omitted after 64 KiB"})
			break
		}
		rows = append(rows, HTTPHeader{Name: k, Value: value})
	}
	return rows
}

func diagnosticURL(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, errors.New("enter an HTTP or HTTPS URL")
	}
	if len(raw) > 8192 {
		return nil, errors.New("URL is too long")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("only http:// and https:// URLs are supported")
	}
	if u.Host == "" {
		return nil, errors.New("URL requires a hostname")
	}
	if u.User != nil {
		return nil, errors.New("URLs with embedded usernames or passwords are not supported")
	}
	if _, err = validateHost(u.Hostname()); err != nil {
		return nil, err
	}
	if p := u.Port(); p != "" {
		if _, err = parsePorts(p, 1); err != nil {
			return nil, errors.New("URL port must be between 1 and 65535")
		}
	}
	u.Fragment = ""
	return u, nil
}

func httpCheck(ctx context.Context, req model.Request, emit model.Emit) (model.Result, error) {
	u, err := diagnosticURL(req.Target)
	if err != nil {
		return model.Result{}, err
	}
	method := strings.ToUpper(stringOption(req.Options, "method", "GET"))
	if method != "GET" && method != "HEAD" {
		return model.Result{}, errors.New("HTTP method must be GET or HEAD")
	}
	timeout, err := durationOption(req.Options, "timeoutMs", 10000, 100, 30000)
	if err != nil {
		return model.Result{}, err
	}
	verify := boolOption(req.Options, "verifyTLS", true)
	redirects := boolOption(req.Options, "redirects", true)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	transport := &http.Transport{Proxy: nil, DialContext: func(dialCtx context.Context, network, endpoint string) (net.Conn, error) {
		return diagnosticDialContext(dialCtx, network, endpoint, timeout)
	}, TLSClientConfig: &tls.Config{InsecureSkipVerify: !verify, MinVersion: tls.VersionTLS10}, TLSHandshakeTimeout: timeout, ResponseHeaderTimeout: timeout, MaxResponseHeaderBytes: 256 * 1024, DisableKeepAlives: true, ForceAttemptHTTP2: false}
	_, sourceSelected := platform.SourceBindingFromContext(ctx)
	defer transport.CloseIdleConnections()
	// Follow redirects explicitly, preserving each request's independent timing.
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	started := time.Now()
	hops := make([]HTTPHop, 0)
	var headers []HTTPHeader
	var tlsData map[string]any
	var contentLength int64
	var sampledBytes int64
	bodyTruncated := false
	finalURL := u.String()
	finalStatus := 0
	finalProtocol := ""
	seen := map[string]bool{}
	result := func() model.Result {
		return model.Result{Summary: fmt.Sprintf("HTTP %d, %d request(s), %.1f ms total", finalStatus, len(hops), ms(time.Since(started))), Data: map[string]any{"target": req.Target, "finalURL": finalURL, "method": method, "statusCode": finalStatus, "protocol": finalProtocol, "headers": headers, "redirects": hops, "tls": tlsData, "contentLength": contentLength, "bodyBytesRead": sampledBytes, "bodyTruncated": bodyTruncated, "elapsedMs": ms(time.Since(started)), "proxy": "Direct connection; system and environment proxies are not used", "note": "Timings include the direct request path. Response bodies are discarded after at most 64 KiB per response. Cookies are not retained and Set-Cookie headers are redacted."}}
	}
	for redirect := 0; redirect <= 10; redirect++ {
		if err = ctx.Err(); err != nil {
			return result(), err
		}
		finalURL = u.String()
		if seen[finalURL] {
			return result(), errors.New("redirect loop detected")
		}
		seen[finalURL] = true
		hop := HTTPHop{URL: finalURL, Method: method}
		hopStart := time.Now()
		var mu sync.Mutex
		var dnsStart, tcpStart, tlsStart time.Time
		trace := &httptrace.ClientTrace{
			DNSStart:          func(httptrace.DNSStartInfo) { mu.Lock(); dnsStart = time.Now(); mu.Unlock() },
			DNSDone:           func(httptrace.DNSDoneInfo) { mu.Lock(); v := ms(time.Since(dnsStart)); hop.DNSMs = &v; mu.Unlock() },
			ConnectStart:      func(string, string) { mu.Lock(); tcpStart = time.Now(); mu.Unlock() },
			ConnectDone:       func(string, string, error) { mu.Lock(); v := ms(time.Since(tcpStart)); hop.ConnectMs = &v; mu.Unlock() },
			TLSHandshakeStart: func() { mu.Lock(); tlsStart = time.Now(); mu.Unlock() },
			TLSHandshakeDone: func(tls.ConnectionState, error) {
				mu.Lock()
				v := ms(time.Since(tlsStart))
				hop.TLSMs = &v
				mu.Unlock()
			},
			GotConn: func(i httptrace.GotConnInfo) {
				mu.Lock()
				hop.RemoteAddress = i.Conn.RemoteAddr().String()
				mu.Unlock()
			},
			GotFirstResponseByte: func() { mu.Lock(); v := ms(time.Since(hopStart)); hop.FirstByteMs = &v; mu.Unlock() },
		}
		r, requestErr := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), method, finalURL, nil)
		if requestErr != nil {
			return result(), requestErr
		}
		r.Header.Set("User-Agent", "NetworkToolkit/"+model.Version)
		r.Header.Set("Accept", "*/*")
		requestClient := client
		if sourceSelected && verify && u.Scheme == "https" {
			// Keep mandatory chain and identity checks, but prevent the default
			// Windows verifier from fetching trust data through another source.
			// Capture the actual URL host: TLS SNI is empty for literal IPs.
			// A fresh transport avoids mutating TLS configuration already in use
			// and gives every redirected hostname its own verification identity.
			serverName := u.Hostname()
			hopTransport := transport.Clone()
			hopTransport.TLSClientConfig.InsecureSkipVerify = true
			hopTransport.TLSClientConfig.VerifyConnection = func(state tls.ConnectionState) error {
				return platform.VerifyOfflineServerCertificate(ctx, state.PeerCertificates, serverName)
			}
			defer hopTransport.CloseIdleConnections()
			requestClient = &http.Client{Transport: hopTransport, CheckRedirect: client.CheckRedirect}
		}
		resp, requestErr := requestClient.Do(r)
		if requestErr != nil {
			mu.Lock()
			hop.TotalMs = ms(time.Since(hopStart))
			hop.Error = requestErr.Error()
			hops = append(hops, hop)
			snapshot := hop
			mu.Unlock()
			event(emit, "http", "error", requestErr.Error(), snapshot)
			return result(), requestErr
		}
		mu.Lock()
		hop.StatusCode = resp.StatusCode
		hop.Status = resp.Status
		hop.Protocol = resp.Proto
		hop.Location = resp.Header.Get("Location")
		mu.Unlock()
		contentLength = resp.ContentLength
		headers = cleanHeaders(resp.Header)
		finalStatus = resp.StatusCode
		finalProtocol = resp.Proto
		if resp.TLS != nil {
			tlsData = tlsMetadata(ctx, *resp.TLS, u.Hostname(), verify)
		} else {
			tlsData = nil
		}
		n, readErr := io.Copy(io.Discard, io.LimitReader(resp.Body, 65537))
		resp.Body.Close()
		sampledBytes = n
		if n > 65536 {
			sampledBytes = 65536
			bodyTruncated = true
		}
		mu.Lock()
		hop.TotalMs = ms(time.Since(hopStart))
		if readErr != nil {
			hop.Error = readErr.Error()
		}
		hops = append(hops, hop)
		snapshot := hop
		mu.Unlock()
		level := "success"
		if resp.StatusCode >= 400 {
			level = "warning"
		}
		event(emit, "http", level, fmt.Sprintf("%s %s: %s", method, finalURL, resp.Status), snapshot)
		if readErr != nil {
			return result(), fmt.Errorf("read response body: %w", readErr)
		}
		isRedirect := resp.StatusCode == 301 || resp.StatusCode == 302 || resp.StatusCode == 303 || resp.StatusCode == 307 || resp.StatusCode == 308
		if !redirects || !isRedirect || hop.Location == "" {
			return result(), nil
		}
		if redirect == 10 {
			return result(), errors.New("stopped after 10 redirects")
		}
		location, locationErr := url.Parse(hop.Location)
		if locationErr != nil {
			return result(), fmt.Errorf("invalid redirect URL: %w", locationErr)
		}
		next := u.ResolveReference(location)
		if _, err = diagnosticURL(next.String()); err != nil {
			return result(), fmt.Errorf("redirect rejected: %w", err)
		}
		u = next
		if resp.StatusCode == 303 && method != "HEAD" {
			method = "GET"
		}
	}
	return result(), nil
}
