package diag

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"runtime"
	"sort"
	"strings"
	"time"

	"networktoolkit/internal/platform"
)

const hostnameLookupTimeout = 1500 * time.Millisecond

// HostnameIdentity is a resolver observation, separate from device-reported
// friendly names and reachability evidence. A cached/PTR name cannot establish
// that the target itself replied to any probe.
type HostnameIdentity struct {
	Hostname       string   `json:"hostname,omitempty"`
	Hostnames      []string `json:"hostnames,omitempty"`
	HostnameSource string   `json:"hostnameSource,omitempty"`
	HostnameStatus string   `json:"hostnameStatus"`
	HostnameNote   string   `json:"hostnameNote,omitempty"`
}

type reverseLookup func(context.Context, string) ([]string, error)

// Automatic Windows lookups retain the OS resolver's enterprise DNS policies.
// Native DnsQuery can ignore cancellation, so its caller uses a bounded wait and
// shared admission slots which remain occupied until the native call returns.
var hostnameResolver = &net.Resolver{PreferGo: true}

func systemReverseLookup(ctx context.Context, address string) ([]string, error) {
	if _, selected := platform.SourceBindingFromContext(ctx); selected {
		resolver, err := boundResolver(ctx)
		if err != nil {
			return nil, err
		}
		return resolver.LookupAddr(ctx, address)
	}
	// Windows tracert uses the operating system DNS client, including its
	// suffixes, policies and NRPT configuration. Prefer that resolution path
	// in automatic mode so reverse lookups match the local system.
	if runtime.GOOS == "windows" {
		return boundedNativeReverseLookup(ctx, address, net.DefaultResolver.LookupAddr)
	}
	return hostnameResolver.LookupAddr(ctx, address)
}

func resolveHostname(parent context.Context, address string, lookup reverseLookup) HostnameIdentity {
	result := HostnameIdentity{HostnameStatus: "not_resolved", HostnameNote: "No hostname was returned by the configured reverse lookup."}
	if parent.Err() != nil {
		result.HostnameStatus, result.HostnameNote = "cancelled", "Hostname lookup was cancelled."
		return result
	}
	target, err := netip.ParseAddr(address)
	if err != nil || target.IsUnspecified() || target.IsMulticast() || target.Unmap().String() == "255.255.255.255" {
		result.HostnameNote = "Hostname lookup requires a literal unicast IP address."
		return result
	}
	if lookup == nil {
		result.HostnameNote = "Hostname lookup is unavailable."
		return result
	}
	ctx, cancel := context.WithTimeout(parent, hostnameLookupTimeout)
	defer cancel()
	names, lookupErr := lookup(ctx, target.Unmap().WithZone("").String())
	if parent.Err() != nil {
		result.HostnameStatus, result.HostnameNote = "cancelled", "Hostname lookup was cancelled."
		return result
	}
	// Keep only bounded hostname strings, never URLs, literal addresses, XML or
	// resolver error text. The UI escapes them; transports continue to use IPs.
	seen := map[string]bool{}
	for i, raw := range names {
		if i >= 64 {
			break
		}
		name := strings.TrimSuffix(strings.TrimSpace(raw), ".")
		if name == "" || len(name) > 253 || strings.HasSuffix(name, ".") {
			continue
		}
		if _, err := netip.ParseAddr(name); err == nil {
			continue
		}
		if _, err := validateHost(name); err != nil {
			continue
		}
		key := strings.ToLower(name)
		if !seen[key] {
			seen[key] = true
			result.Hostnames = append(result.Hostnames, name)
		}
		if len(result.Hostnames) == 16 {
			break
		}
	}
	if len(result.Hostnames) != 0 {
		sort.Slice(result.Hostnames, func(i, j int) bool {
			return strings.ToLower(result.Hostnames[i]) < strings.ToLower(result.Hostnames[j])
		})
		result.Hostname = result.Hostnames[0]
		result.HostnameSource = "Configured reverse lookup"
		if _, selected := platform.SourceBindingFromContext(ctx); selected {
			result.HostnameSource = "Selected-source reverse lookup"
		}
		result.HostnameStatus = "resolved"
		result.HostnameNote = "The resolver name may be cached and is not evidence that the device responded."
		return result
	}
	var dnsError *net.DNSError
	if _, selected := platform.SourceBindingFromContext(ctx); selected && lookupErr != nil {
		result.HostnameNote = "Selected-source reverse lookup: " + lookupErr.Error()
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(lookupErr, context.DeadlineExceeded) || (errors.As(lookupErr, &dnsError) && dnsError.IsTimeout) {
		result.HostnameStatus, result.HostnameNote = "timeout", "Hostname lookup timed out after its bounded lookup window."
	} else if errors.Is(lookupErr, context.Canceled) {
		result.HostnameStatus, result.HostnameNote = "cancelled", "Hostname lookup was cancelled."
	}
	return result
}

func disabledHostname() HostnameIdentity {
	return HostnameIdentity{HostnameStatus: "disabled", HostnameNote: "Hostname lookup was not requested."}
}

type hostnameCheckData struct {
	Address string `json:"address"`
	HostnameIdentity
}

func makeHostnameCheck(ctx context.Context, address string, lookup reverseLookup) QuickCheck {
	identity := resolveHostname(ctx, address, lookup)
	detail := "Not resolved for " + address + ". " + identity.HostnameNote
	if identity.Hostname != "" {
		detail = strings.Join(identity.Hostnames, ", ") + " (" + address + ")"
	}
	return QuickCheck{Name: "Hostname", Status: "info", Detail: detail, Data: hostnameCheckData{Address: address, HostnameIdentity: identity}}
}

var nativeReverseSlots = make(chan struct{}, 64)

func boundedNativeReverseLookup(ctx context.Context, address string, lookup reverseLookup) ([]string, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case nativeReverseSlots <- struct{}{}:
	}
	type answer struct {
		names []string
		err   error
	}
	ready := make(chan answer, 1)
	go func() {
		defer func() { <-nativeReverseSlots }()
		names, err := lookup(ctx, address)
		ready <- answer{names, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-ready:
		return result.names, result.err
	}
}
