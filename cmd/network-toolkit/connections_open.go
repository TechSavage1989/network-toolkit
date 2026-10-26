package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

var errConnectionLaunchUnavailable = errors.New("Device connection launch is available in the Windows app.")

// A connection request describes only a destination. It never accepts a URL,
// executable, credential, remote command, file, or additional client arguments.
type connectionOpenRequest struct {
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port,omitempty"`
}

type connectionOpenTarget struct {
	Protocol string
	Host     string
	Port     int
	Target   string
}

type connectionOpenCommand struct {
	File      string
	Arguments []string
}

func buildConnectionOpenTarget(request connectionOpenRequest) (connectionOpenTarget, error) {
	var target connectionOpenTarget
	var defaultPort int
	switch request.Protocol {
	case "http":
		defaultPort = 80
	case "https":
		defaultPort = 443
	case "rdp":
		defaultPort = 3389
	default:
		return target, errors.New("Choose HTTP, HTTPS, or RDP for the device connection.")
	}
	if request.Port < 0 || request.Port > 65535 {
		return target, errors.New("The connection port must be between 1 and 65535, or 0 for the protocol default.")
	}
	host, err := connectionOpenHost(request.Host)
	if err != nil {
		return target, err
	}
	port := request.Port
	if port == 0 {
		port = defaultPort
	}
	target = connectionOpenTarget{Protocol: request.Protocol, Host: host, Port: port}
	if request.Protocol == "rdp" {
		// An explicit port also makes an IPv6 destination unambiguous to mstsc.
		target.Target = net.JoinHostPort(host, strconv.Itoa(port))
		return target, nil
	}
	authority := host
	if strings.Contains(host, ":") {
		authority = "[" + host + "]"
	}
	if port != defaultPort {
		authority = net.JoinHostPort(host, strconv.Itoa(port))
	}
	target.Target = (&url.URL{Scheme: request.Protocol, Host: authority, Path: "/"}).String()
	return target, nil
}

func connectionOpenHost(host string) (string, error) {
	invalid := errors.New("Enter an IP address or ASCII DNS hostname without a URL, path, credentials, whitespace, or port.")
	if host == "" || len(host) > 254 || strings.TrimSpace(host) != host {
		return "", invalid
	}
	bracketed := strings.HasPrefix(host, "[") || strings.HasSuffix(host, "]")
	if bracketed {
		if !strings.HasPrefix(host, "[") || !strings.HasSuffix(host, "]") {
			return "", invalid
		}
		host = host[1 : len(host)-1]
	}
	if address, err := netip.ParseAddr(host); err == nil {
		if bracketed && !address.Is6() {
			return "", invalid
		}
		if address.Zone() != "" {
			return "", errors.New("Connection launch does not support IPv6 scope IDs. Use the device's DNS name or an IPv4 or unscoped IPv6 address.")
		}
		address = address.Unmap()
		if address.IsUnspecified() || address.IsMulticast() || address == netip.AddrFrom4([4]byte{255, 255, 255, 255}) {
			return "", errors.New("Choose a device address; unspecified, multicast, and broadcast addresses cannot open a device connection.")
		}
		return address.String(), nil
	}
	if bracketed {
		return "", invalid
	}
	// RFC host labels, including ASCII punycode, are sufficient for Windows
	// device names. No DNS lookup or destination connection is made here.
	name := strings.TrimSuffix(host, ".")
	if name == "" || len(name) > 253 {
		return "", invalid
	}
	labels := strings.Split(name, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", invalid
		}
		for _, ch := range label {
			if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-') {
				return "", invalid
			}
		}
	}
	// Browsers recognize legacy integer, short, octal and hexadecimal IPv4
	// forms. They must not reinterpret a supposedly literal DNS destination.
	// Strict, four-octet IPv4 addresses were already accepted above.
	last := strings.ToLower(labels[len(labels)-1])
	if connectionNumericLabel(last, 10) || last == "0x" || strings.HasPrefix(last, "0x") && connectionNumericLabel(last[2:], 16) {
		return "", errors.New("Use a full IPv4 address, an IPv6 address, or a DNS hostname; abbreviated and numeric address aliases are not supported.")
	}
	return strings.ToLower(host), nil
}

func connectionNumericLabel(label string, base int) bool {
	if label == "" {
		return false
	}
	for _, ch := range label {
		if ch >= '0' && ch <= '9' {
			continue
		}
		if base == 16 && ch >= 'a' && ch <= 'f' {
			continue
		}
		return false
	}
	return true
}

// systemMSTSC is supplied only by the Windows system-directory API. This pure
// planner is also used by no-launch tests; request fields never select a file.
func buildConnectionOpenCommand(target connectionOpenTarget, systemMSTSC string) (connectionOpenCommand, error) {
	canonical, err := buildConnectionOpenTarget(connectionOpenRequest{Protocol: target.Protocol, Host: target.Host, Port: target.Port})
	if err != nil || canonical != target {
		return connectionOpenCommand{}, errors.New("The device connection target is invalid.")
	}
	if target.Protocol == "rdp" {
		if systemMSTSC == "" {
			return connectionOpenCommand{}, errors.New("Windows Remote Desktop Connection (mstsc.exe) was not found.")
		}
		return connectionOpenCommand{File: systemMSTSC, Arguments: []string{"/v:" + target.Target}}, nil
	}
	return connectionOpenCommand{File: target.Target}, nil
}

// Decode the small public envelope exactly: no duplicate/case-variant fields,
// unknown options, null fields, trailing values, or credentials are accepted.
func decodeConnectionOpenRequest(w http.ResponseWriter, r *http.Request) (connectionOpenRequest, error) {
	var request connectionOpenRequest
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return request, errors.New("Send an application/json request.")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	decoder := json.NewDecoder(r.Body)
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return request, errors.New("Send a JSON object with protocol, host, and an optional numeric port.")
	}
	seen := make(map[string]bool, 3)
	for decoder.More() {
		token, tokenErr := decoder.Token()
		key, ok := token.(string)
		if tokenErr != nil || !ok || seen[key] || key != "protocol" && key != "host" && key != "port" {
			return request, errors.New("Use each of protocol, host, and port at most once, with no other fields.")
		}
		seen[key] = true
		var raw json.RawMessage
		if err = decoder.Decode(&raw); err != nil || string(raw) == "null" {
			return request, errors.New("Connection fields must contain a protocol, host, or numeric port; null is not supported.")
		}
		switch key {
		case "protocol":
			err = json.Unmarshal(raw, &request.Protocol)
		case "host":
			err = json.Unmarshal(raw, &request.Host)
		case "port":
			err = json.Unmarshal(raw, &request.Port)
		}
		if err != nil {
			return request, errors.New("Protocol and host must be strings; port must be an integer.")
		}
	}
	if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
		return request, errors.New("The connection request is incomplete.")
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return request, errors.New("Send exactly one connection request.")
	}
	if !seen["protocol"] || !seen["host"] {
		return request, errors.New("Specify both a protocol and a device host.")
	}
	return request, nil
}

func (a *application) handleConnectionOpen(w http.ResponseWriter, r *http.Request) {
	a.handleConnectionOpenWithLauncher(w, r, launchDeviceConnection)
}

// Keeping the launcher an argument lets HTTP tests verify actual acceptance and
// failure behavior without opening a client, mutating a global, or networking.
func (a *application) handleConnectionOpenWithLauncher(w http.ResponseWriter, r *http.Request, launch func(context.Context, connectionOpenTarget) error) {
	if r.URL.Path != "/api/connections/open" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	if !a.homeCanConnect() {
		jsonError(w, http.StatusConflict, "The workspace is closing.")
		return
	}
	request, err := decodeConnectionOpenRequest(w, r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	target, err := buildConnectionOpenTarget(request)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	// A body can arrive slowly while the native home starts closing. Its engine
	// may remain alive for detached sessions, so context alone is insufficient.
	if !a.homeCanConnect() {
		jsonError(w, http.StatusConflict, "The workspace is closing.")
		return
	}
	if err = launch(a.ctx, target); err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, errConnectionLaunchUnavailable) {
			code = http.StatusNotImplemented
		} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			code = http.StatusConflict
		}
		jsonError(w, code, fmt.Sprintf("The %s connection could not be opened: %s", strings.ToUpper(target.Protocol), err))
		return
	}
	jsonResponse(w, http.StatusAccepted, map[string]any{"ok": true, "protocol": target.Protocol, "target": target.Target})
}
