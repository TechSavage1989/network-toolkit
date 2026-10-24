// Package deviceinfo reads identity fields explicitly reported by a device.
// It sends only one unicast SSDP query to the selected literal IP and can fetch
// one description from that same IP. It does not invoke UPnP actions.
// Protocol reference: UPnP Device Architecture 2.0, sections 1.3.2 and 2.3:
// https://upnp.org/specs/arch/UPnP-arch-DeviceArchitecture-v2.0-20140901.pdf
package deviceinfo

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"networktoolkit/internal/platform"
)

// Info contains only device-reported description values. Responded records a
// valid SSDP 200 response even if fetching its description subsequently fails.
// A missing model stays empty; a friendly name or HTTP Server header is never
// substituted for it. This information is self-reported, not authenticated.
type Info struct {
	Responded      bool   `json:"responded"`
	Manufacturer   string `json:"manufacturer,omitempty"`
	Model          string `json:"model,omitempty"`
	ModelName      string `json:"modelName,omitempty"`
	ModelNumber    string `json:"modelNumber,omitempty"`
	FriendlyName   string `json:"friendlyName,omitempty"`
	ModelSource    string `json:"modelSource,omitempty"`
	DescriptionURL string `json:"descriptionUrl,omitempty"`
}

var ErrNoResponse = errors.New("no response to unicast UPnP discovery")

const maxDescriptionBytes = 512 * 1024
const maxSSDPBytes = 16 * 1024
const ssdpReplyWindow = 1100 * time.Millisecond

// Probe has a single total UDP-plus-HTTP timeout of 1200ms..5000ms. Parent
// cancellation takes priority. Nil error means a valid UPnP device description
// was read; Responded may still be true on a subsequent description error.
func Probe(ctx context.Context, address string, timeout time.Duration) (Info, error) {
	return probe(ctx, address, timeout, 1900)
}

func probe(parent context.Context, address string, timeout time.Duration, port int) (Info, error) {
	if err := parent.Err(); err != nil {
		return Info{}, err
	}
	if timeout < 1200*time.Millisecond || timeout > 5*time.Second {
		return Info{}, errors.New("device identification timeout must be between 1200 and 5000 ms")
	}
	target, err := netip.ParseAddr(strings.TrimSpace(address))
	if err != nil || target.IsUnspecified() || target.IsMulticast() {
		return Info{}, errors.New("device identification requires one literal unicast IP address")
	}
	target = target.Unmap()
	if target == netip.AddrFrom4([4]byte{255, 255, 255, 255}) || (target.Is6() && target.IsLinkLocalUnicast() && target.Zone() == "") {
		return Info{}, errors.New("device identification requires a unicast IP with its interface scope when applicable")
	}
	if port < 1 || port > 65535 {
		return Info{}, errors.New("invalid discovery port")
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	location, replied, err := discoverDescription(ctx, target, port)
	info := Info{Responded: replied}
	if err != nil {
		return info, err
	}
	u, err := descriptionLocation(location, target)
	if err != nil {
		return info, err
	}
	info.DescriptionURL = u.String()
	description, err := fetchDescription(ctx, target, u)
	if err != nil {
		return info, err
	}
	description.Responded = true
	description.DescriptionURL = u.String()
	return description, nil
}

func discoverDescription(ctx context.Context, target netip.Addr, port int) (string, bool, error) {
	network := "udp4"
	if target.Is6() {
		network = "udp6"
	}
	endpoint := net.JoinHostPort(target.String(), strconv.Itoa(port))
	conn, err := platform.DialContext(ctx, network, endpoint, 0)
	if err != nil {
		return "", false, fmt.Errorf("open unicast device discovery: %w", err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	deadline := time.Now().Add(ssdpReplyWindow)
	if limit, ok := ctx.Deadline(); ok && limit.Before(deadline) {
		deadline = limit
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return "", false, err
	}
	// UPnP Device Architecture 2.0, section 1.3.2: unicast HOST is the selected
	// device IP and port. MX is for multicast searches and is not sent here.
	request := "M-SEARCH * HTTP/1.1\r\nHOST: " + endpoint + "\r\nMAN: \"ssdp:discover\"\r\nST: upnp:rootdevice\r\n\r\n"
	if _, err := io.WriteString(conn, request); err != nil {
		if ctx.Err() != nil {
			return "", false, ctx.Err()
		}
		return "", false, fmt.Errorf("send unicast device discovery: %w", err)
	}
	buffer := make([]byte, maxSSDPBytes+1)
	var responseErr error
	for received := 0; received < 8; received++ {
		n, err := conn.Read(buffer)
		if err != nil {
			if ctx.Err() != nil {
				return "", false, ctx.Err()
			}
			if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
				if responseErr != nil {
					return "", false, responseErr
				}
				return "", false, ErrNoResponse
			}
			return "", false, fmt.Errorf("read unicast device discovery: %w", err)
		}
		if n > maxSSDPBytes {
			responseErr = errors.New("device discovery response exceeded the 16 KiB limit")
			continue
		}
		location, err := parseSSDPResponse(buffer[:n])
		if err == nil {
			// The connected UDP socket accepts packets only from the selected
			// target endpoint, so this cannot identify a different responding IP.
			return location, true, nil
		}
		responseErr = err
	}
	return "", false, errors.New("device returned too many unusable discovery responses")
}

func parseSSDPResponse(raw []byte) (string, error) {
	if len(raw) > maxSSDPBytes || !bytes.Contains(raw, []byte("\r\n\r\n")) {
		return "", errors.New("invalid device discovery response framing")
	}
	response, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(raw)), nil)
	if err != nil {
		return "", errors.New("malformed device discovery response")
	}
	defer response.Body.Close()
	if response.ProtoMajor != 1 || response.StatusCode != http.StatusOK || len(response.Header.Values("ST")) != 1 || response.Header.Get("ST") != "upnp:rootdevice" {
		return "", errors.New("device did not return a matching UPnP root-device response")
	}
	locations := response.Header.Values("Location")
	if len(locations) != 1 || strings.TrimSpace(locations[0]) == "" {
		return "", errors.New("device discovery response has no single description URL")
	}
	return strings.TrimSpace(locations[0]), nil
}

func descriptionLocation(raw string, target netip.Addr) (*url.URL, error) {
	if len(raw) == 0 || len(raw) > 2048 {
		return nil, errors.New("invalid device description URL length")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" || u.User != nil || u.Fragment != "" || u.Host == "" || strings.HasSuffix(u.Host, ":") {
		return nil, errors.New("invalid device description URL")
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("device descriptions must use HTTP or HTTPS")
	}
	host, err := netip.ParseAddr(u.Hostname())
	if err != nil || host.Unmap().WithZone("") != target.Unmap().WithZone("") || (host.Zone() != "" && host.Zone() != target.Zone()) {
		return nil, errors.New("device description URL must contain the same literal IP as the selected target")
	}
	if rawPort := u.Port(); rawPort != "" {
		port, err := strconv.Atoi(rawPort)
		if err != nil || port < 1 || port > 65535 {
			return nil, errors.New("invalid device description port")
		}
	}
	return u, nil
}

func fetchDescription(ctx context.Context, target netip.Addr, location *url.URL) (Info, error) {
	port := location.Port()
	if port == "" {
		port = "80"
		if location.Scheme == "https" {
			port = "443"
		}
	}
	network := "tcp4"
	if target.Is6() {
		network = "tcp6"
	}
	transport := &http.Transport{
		Proxy:                  nil,
		DisableKeepAlives:      true,
		DisableCompression:     true,
		MaxResponseHeaderBytes: 16 * 1024,
		DialContext: func(dialCtx context.Context, _ string, _ string) (net.Conn, error) {
			// Ignore DNS and the transport-provided host. Every connection is
			// pinned to the one literal IP validated by the caller.
			return platform.DialContext(dialCtx, network, net.JoinHostPort(target.String(), port), 0)
		},
	}
	if _, selected := platform.SourceBindingFromContext(ctx); selected {
		// Keep HTTPS certificate validation mandatory while preventing Windows
		// trust-chain retrieval from escaping the selected diagnostic source.
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true, VerifyConnection: func(state tls.ConnectionState) error {
			return platform.VerifyOfflineServerCertificate(ctx, state.PeerCertificates, location.Hostname())
		}}
	}
	defer transport.CloseIdleConnections()
	client := http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, location.String(), nil)
	if err != nil {
		return Info{}, err
	}
	request.Header.Set("Accept", "text/xml, application/xml")
	// UPnP's optional User-Agent has a defined OS/version token format. Omit it
	// rather than asserting an operating system version this package has not read.
	request.Header.Set("User-Agent", "")
	response, err := client.Do(request)
	if err != nil {
		return Info{}, fmt.Errorf("read device description: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Info{}, fmt.Errorf("device description returned HTTP %d; redirects are not followed", response.StatusCode)
	}
	if response.ContentLength > maxDescriptionBytes {
		return Info{}, errors.New("device description exceeded the 512 KiB limit")
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxDescriptionBytes+1))
	if err != nil {
		return Info{}, fmt.Errorf("read device description body: %w", err)
	}
	if len(raw) > maxDescriptionBytes {
		return Info{}, errors.New("device description exceeded the 512 KiB limit")
	}
	return parseDescription(raw)
}
