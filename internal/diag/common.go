package diag

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

var errDeviceCheckSubnet = errors.New("Device check supports one target only; use Network discovery to scan an IPv4 subnet")

// Run performs only the diagnostic explicitly requested by the caller.
func Run(ctx context.Context, req model.Request, emit model.Emit) (model.Result, error) {
	return runWithSource(ctx, req, emit, platform.ResolveSourceBinding)
}

func runDiagnostic(ctx context.Context, req model.Request, emit model.Emit) (model.Result, error) {
	if err := ctx.Err(); err != nil {
		return model.Result{}, err
	}
	req.Target = strings.TrimSpace(req.Target)
	if req.Options == nil {
		req.Options = map[string]any{}
	}
	if stringOption(req.Options, "scope", "single") == "subnet" {
		switch req.Tool {
		case "quick":
			return model.Result{}, errDeviceCheckSubnet
		case "ping", "tcp":
			return subnetScan(ctx, req, emit)
		}
	}
	switch req.Tool {
	case "quick":
		return quick(ctx, req, emit)
	case "ping":
		return ping(ctx, req, emit, false)
	case "monitor":
		return monitor(ctx, req, emit)
	case "trace":
		return trace(ctx, req, emit)
	case "dns":
		return dns(ctx, req, emit)
	case "smtp":
		return smtpTest(ctx, req, emit)
	case "sftp":
		return sftpTest(ctx, req, emit)
 case "ftp":
        return ftpTest(ctx,req,emit)
	case "tftp":
		return tftpTest(ctx, req, emit)
	case "tcp":
		return tcp(ctx, req, emit)
	case "http":
		return httpCheck(ctx, req, emit)
	case "tls":
		return tlsCheck(ctx, req, emit)
	case "discover":
		return subnetScan(ctx, req, emit)
	case "identify":
		return identify(ctx, req, emit)
	case "wol":
		port, err := intOption(req.Options, "port", 9, 1, 65535)
		if err != nil {
			return model.Result{}, err
		}
		return platform.Wake(ctx, req.Target, stringOption(req.Options, "broadcast", "255.255.255.255"), port)
	case "routes", "arp", "connections", "wifi", "ipconfig":
		return platform.Snapshot(ctx, req.Tool)
	default:
		return model.Result{}, fmt.Errorf("unknown diagnostic tool: %s", req.Tool)
	}
}

func stringOption(o map[string]any, key, fallback string) string {
	x, ok := o[key]
	if !ok || x == nil {
		return fallback
	}
	if s, ok := x.(string); ok {
		return strings.TrimSpace(s)
	}
	return fallback
}

func boolOption(o map[string]any, key string, fallback bool) bool {
	x, ok := o[key]
	if !ok || x == nil {
		return fallback
	}
	if b, ok := x.(bool); ok {
		return b
	}
	return fallback
}

func intOption(o map[string]any, key string, fallback, min, max int) (int, error) {
	x, ok := o[key]
	if !ok || x == nil || x == "" {
		return fallback, nil
	}
	var n int
	switch v := x.(type) {
	case float64:
		n = int(v)
		if float64(n) != v {
			return 0, fmt.Errorf("%s must be a whole number", key)
		}
	case int:
		n = v
	case string:
		var err error
		n, err = strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("%s must be a whole number", key)
		}
	default:
		return 0, fmt.Errorf("%s must be a whole number", key)
	}
	if n < min || n > max {
		return 0, fmt.Errorf("%s must be between %d and %d", key, min, max)
	}
	return n, nil
}

func durationOption(o map[string]any, key string, fallback, min, max int) (time.Duration, error) {
	n, err := intOption(o, key, fallback, min, max)
	return time.Duration(n) * time.Millisecond, err
}

func validateHost(raw string) (string, error) {
	host := strings.TrimSpace(raw)
	if len(host) == 0 {
		return "", errors.New("enter a hostname or IP address")
	}
	if len(host) > 253 {
		return "", errors.New("hostname is too long")
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = host[1 : len(host)-1]
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return host, nil
	}
	if strings.ContainsAny(host, "/:\\?#@ \t\r\n\x00") {
		return "", errors.New("enter a hostname or IP address without a URL, path, or port")
	}
	for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
		if len(label) == 0 || len(label) > 63 {
			return "", errors.New("hostname contains an empty or overlong label")
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
				return "", errors.New("hostname contains an invalid character; use ASCII or its punycode form")
			}
		}
	}
	return host, nil
}

func parsePorts(spec string, limit int) ([]int, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, errors.New("enter at least one TCP port")
	}
	if len(spec) > maxScanScopeText {
		return nil, fmt.Errorf("TCP port list text is limited to %d bytes", maxScanScopeText)
	}
	spec = scanPortOperators.ReplaceAllString(spec, "-")
	var parts []string
	for _, group := range strings.Split(spec, ",") {
		items := strings.FieldsFunc(group, func(r rune) bool { return scanListSpace(r) || r == ';' })
		if len(items) == 0 {
			return nil, errors.New("port list contains an empty item")
		}
		parts = append(parts, items...)
	}
	seen := map[int]bool{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, errors.New("port list contains an empty item")
		}
		bits := strings.Split(part, "-")
		if len(bits) > 2 {
			return nil, fmt.Errorf("invalid port range: %s", part)
		}
		first, err := strconv.Atoi(strings.TrimSpace(bits[0]))
		if err != nil || !decimalDigits(bits[0]) || first < 1 || first > 65535 {
			return nil, fmt.Errorf("invalid TCP port: %s", part)
		}
		last := first
		if len(bits) == 2 {
			last, err = strconv.Atoi(strings.TrimSpace(bits[1]))
			if err != nil || !decimalDigits(bits[1]) || last < first || last > 65535 {
				return nil, fmt.Errorf("invalid port range: %s", part)
			}
		}
		if last-first+1 > limit {
			return nil, fmt.Errorf("select no more than %d ports per run", limit)
		}
		for p := first; p <= last; p++ {
			seen[p] = true
			if len(seen) > limit {
				return nil, fmt.Errorf("select no more than %d ports per run", limit)
			}
		}
	}
	ports := make([]int, 0, len(seen))
	for p := range seen {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	return ports, nil
}

func ipv4Targets(raw string, limit int) ([]netip.Addr, error) {
	plan, err := parseIPv4Scopes(raw, limit)
	return plan.targets, err
}

func pause(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
func safeError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func resolveHost(ctx context.Context, host string, timeout time.Duration) ([]string, error) {
	if binding, selected := platform.SourceBindingFromContext(ctx); selected {
		return resolveBoundHost(ctx, host, timeout, binding)
	}
	if a, err := netip.ParseAddr(host); err == nil {
		return []string{a.String()}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	var out []string
	seen := map[string]bool{}
	// Prefer IPv4 for a predictable first choice, while retaining IPv6 results.
	for _, v4 := range []bool{true, false} {
		for _, ip := range ips {
			if (ip.IP.To4() != nil) != v4 {
				continue
			}
			v := ip.String()
			if !seen[v] {
				out = append(out, v)
				seen[v] = true
			}
		}
	}
	if len(out) == 0 {
		return nil, errors.New("hostname returned no IP addresses")
	}
	return out, nil
}

func event(emit model.Emit, kind, level, message string, data any) {
	model.Log(emit, level, message, map[string]any{"type": kind, "value": data})
}
