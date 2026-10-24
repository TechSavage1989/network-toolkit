package diag

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

// SourceMetadata describes the requested and validated local source. It does
// not claim that a remote device replied or that a physical egress was observed.
type SourceMetadata struct {
	Mode                 string `json:"mode"`
	Status               string `json:"status"`
	InterfaceIndex       int    `json:"interfaceIndex,omitempty"`
	InterfaceName        string `json:"interfaceName,omitempty"`
	InterfaceDescription string `json:"interfaceDescription,omitempty"`
	InterfaceID          string `json:"interfaceId,omitempty"`
	Address              string `json:"address,omitempty"`
	Family               string `json:"family,omitempty"`
	DNSServer            string `json:"dnsServer,omitempty"`
	NameResolution       string `json:"nameResolution"`
	Note                 string `json:"note"`
	Error                string `json:"error,omitempty"`
}

type sourceMetadataKey struct{}
type sourceResolver func(context.Context, int, string, string) (*platform.SourceBinding, error)

// SupportsSourceBinding lists outbound diagnostics whose every probe uses the
// selected source. Read-only snapshots, external launchers, and Wake-on-LAN do
// not accept these options.
func SupportsSourceBinding(tool string) bool {
	switch tool {
	case "quick", "ping", "trace", "tcp", "monitor", "http", "tls", "discover", "identify", "dns":
		return true
	default:
		return false
	}
}

func sourceOptionsPresent(options map[string]any) bool {
	for _, key := range []string{"sourceInterfaceIndex", "sourceAddress", "sourceInterfaceName"} {
		if _, present := options[key]; present {
			return true
		}
	}
	return false
}

func sourceIndex(value any) (int, error) {
	var n int
	switch v := value.(type) {
	case int:
		n = v
	case float64:
		if v < 1 || v > 2147483647 || v != float64(int(v)) {
			return 0, errors.New("sourceInterfaceIndex must be an integer between 1 and 2147483647")
		}
		n = int(v)
	default:
		return 0, errors.New("sourceInterfaceIndex must be an integer between 1 and 2147483647")
	}
	if n < 1 || n > 2147483647 {
		return 0, errors.New("sourceInterfaceIndex must be an integer between 1 and 2147483647")
	}
	return n, nil
}

// ValidateSourceOptions checks only request shape and tool/address-family
// compatibility. Run revalidates adapter availability and address ownership
// immediately before starting probes, so a stale dropdown cannot select a
// different adapter or silently return to automatic routing.
func ValidateSourceOptions(req model.Request) error {
	if !sourceOptionsPresent(req.Options) {
		return nil
	}
	if !SupportsSourceBinding(req.Tool) {
		return fmt.Errorf("%s does not support selecting a diagnostic source; use Automatic routing", req.Tool)
	}
	if _, err := sourceIndex(req.Options["sourceInterfaceIndex"]); err != nil {
		return err
	}
	text, ok := req.Options["sourceAddress"].(string)
	if !ok || len(text) > 256 {
		return errors.New("sourceAddress must be one literal local IP address")
	}
	address, err := netip.ParseAddr(strings.TrimSpace(text))
	if err != nil || address.IsUnspecified() || address.IsMulticast() || address.Unmap().String() == "255.255.255.255" {
		return errors.New("sourceAddress must be one literal unicast local IP address")
	}
	if raw, present := req.Options["sourceInterfaceName"]; present {
		name, ok := raw.(string)
		if !ok || len(name) > 512 || strings.ContainsAny(name, "\x00\r\n\t") {
			return errors.New("sourceInterfaceName must be a valid adapter name")
		}
	}
	if address.Unmap().Is6() && (req.Tool == "discover" || req.Tool == "identify" || (req.Tool == "ping" || req.Tool == "tcp") && stringOption(req.Options, "scope", "single") == "subnet") {
		return errors.New("this network/device scan supports IPv4 targets; select an IPv4 source address")
	}
	return nil
}

func requestedSource(req model.Request) *SourceMetadata {
	if !sourceOptionsPresent(req.Options) {
		return &SourceMetadata{Mode: "automatic", Status: "automatic", NameResolution: "system", Note: "The operating system chooses the source interface and address. Existing diagnostic resolver settings apply."}
	}
	index, _ := sourceIndex(req.Options["sourceInterfaceIndex"])
	meta := &SourceMetadata{Mode: "selected", Status: "invalid", InterfaceIndex: index, InterfaceName: stringOption(req.Options, "sourceInterfaceName", ""), Address: stringOption(req.Options, "sourceAddress", ""), NameResolution: "unavailable", Note: "The requested source has not been validated. No probe uses Automatic routing as a fallback."}
	if address, err := netip.ParseAddr(meta.Address); err == nil {
		meta.Family = "ipv6"
		if address.Unmap().Is4() {
			meta.Family = "ipv4"
		}
	}
	return meta
}

func prepareDiagnosticSource(ctx context.Context, req model.Request, resolve sourceResolver) (context.Context, *SourceMetadata, error) {
	if !SupportsSourceBinding(req.Tool) && !sourceOptionsPresent(req.Options) {
		return ctx, nil, nil
	}
	meta := requestedSource(req)
	if err := ValidateSourceOptions(req); err != nil {
		meta.Error = err.Error()
		return ctx, meta, err
	}
	if err := ctx.Err(); err != nil {
		meta.Error = err.Error()
		return ctx, meta, err
	}
	if meta.Mode == "automatic" {
		ctx = platform.WithSourceBinding(ctx, nil)
		return context.WithValue(ctx, sourceMetadataKey{}, meta), meta, nil
	}
	binding, err := resolve(ctx, meta.InterfaceIndex, meta.InterfaceName, meta.Address)
	if err != nil {
		meta.Error = err.Error()
		meta.Note = "The selected source could not be validated. No probes were sent and Automatic routing was not used."
		return ctx, meta, err
	}
	if binding == nil {
		err = errors.New("selected source validation returned no adapter")
		meta.Error = err.Error()
		return ctx, meta, err
	}
	meta.Status, meta.InterfaceIndex, meta.InterfaceName = "validated", binding.InterfaceIndex, binding.InterfaceName
	meta.InterfaceDescription, meta.InterfaceID = binding.InterfaceDescription, binding.InterfaceID
	meta.Address, meta.Family = binding.Address, binding.Family
	ctx = platform.WithSourceBinding(ctx, binding)
	dnsServer, dnsErr := selectedDNSServer(ctx)
	if req.Tool == "dns" && stringOption(req.Options, "server", "") != "" {
		dnsServer, dnsErr = boundServerEndpoint(binding, stringOption(req.Options, "server", ""))
	}
	meta.Note = "Probes use the selected local source and interface. A connection or routing failure does not fall back to Automatic routing."
	if req.Tool == "http" || req.Tool == "tls" || req.Tool == "discover" || req.Tool == "identify" {
		meta.Note += " HTTPS/TLS trust checks use locally available certificates only; missing trust data is reported instead of downloaded through another source."
	}
	if dnsErr != nil {
		meta.NameResolution = "unavailable"
		meta.Note += " " + dnsErr.Error() + " Literal IP targets can still be tested."
	} else {
		meta.DNSServer, meta.NameResolution = dnsServer, "bound_dns"
		meta.Note += " Network name lookups use " + dnsServer + " through the same source; local hosts-file entries may also resolve names."
	}
	return context.WithValue(ctx, sourceMetadataKey{}, meta), meta, nil
}

func sourceMetadata(ctx context.Context) *SourceMetadata {
	meta, _ := ctx.Value(sourceMetadataKey{}).(*SourceMetadata)
	return meta
}

func runWithSource(ctx context.Context, req model.Request, emit model.Emit, resolve sourceResolver) (result model.Result, runErr error) {
	ctx, meta, err := prepareDiagnosticSource(ctx, req, resolve)
	if meta != nil {
		defer func() { result = attachSource(result, meta) }()
		level, message := "info", "Source: Automatic routing"
		if meta.Mode == "selected" {
			message = fmt.Sprintf("Source: %s, %s (%s)", meta.InterfaceName, meta.Address, meta.Family)
		}
		if err != nil {
			level, message = "error", "Source selection rejected: "+err.Error()
		}
		event(emit, "source", level, message, meta)
	}
	if err != nil {
		return model.Result{Summary: "Diagnostic source could not be prepared"}, err
	}
	return runDiagnostic(ctx, req, emit)
}

func attachSource(result model.Result, source *SourceMetadata) model.Result {
	switch data := result.Data.(type) {
	case nil:
		result.Data = map[string]any{"source": source}
	case map[string]any:
		// Some diagnostics also publish the data map as an activity event.
		// Add final metadata without mutating an already emitted snapshot.
		copy := make(map[string]any, len(data)+1)
		for key, value := range data {
			copy[key] = value
		}
		copy["source"] = source
		result.Data = copy
	case SubnetResult:
		data.Source = source
		result.Data = data
	case IdentifyResult:
		data.Source = source
		result.Data = data
	}
	return result
}

func boundServerEndpoint(binding *platform.SourceBinding, server string) (string, error) {
	endpoint, err := serverEndpoint(server)
	if err != nil {
		return "", err
	}
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "", err
	}
	address, err := compatibleSourceTarget(binding, host)
	if err != nil {
		return "", fmt.Errorf("DNS server must be a literal IP compatible with the selected source: %w", err)
	}
	return net.JoinHostPort(address, port), nil
}

func selectedDNSServer(ctx context.Context) (string, error) {
	binding, selected := platform.SourceBindingFromContext(ctx)
	if !selected {
		return "", errors.New("no diagnostic source was selected")
	}
	for _, server := range binding.DNSServers {
		if endpoint, err := boundServerEndpoint(binding, server); err == nil {
			return endpoint, nil
		}
	}
	return "", errors.New("The selected adapter has no compatible configured DNS server; enter a literal IP target or select another source")
}

func compatibleSourceTarget(binding *platform.SourceBinding, host string) (string, error) {
	address, err := netip.ParseAddr(host)
	if err != nil {
		return "", errors.New("a literal IP address is required")
	}
	address = address.Unmap()
	if address.IsUnspecified() || address.IsMulticast() || address.String() == "255.255.255.255" {
		return "", errors.New("a unicast IP address is required")
	}
	if (binding.Family == "ipv4") != address.Is4() {
		return "", fmt.Errorf("target %s is incompatible with the selected %s source", host, binding.Family)
	}
	if address.Is6() {
		index := binding.IPv6InterfaceIndex
		if index == 0 {
			index = binding.InterfaceIndex
		}
		zone := address.Zone()
		if zone != "" && zone != binding.InterfaceName && zone != strconv.Itoa(index) {
			return "", errors.New("target IPv6 scope does not match the selected source adapter")
		}
		if address.IsLinkLocalUnicast() || zone != "" {
			address = address.WithZone(strconv.Itoa(index))
		}
	}
	return address.String(), nil
}

func boundResolver(ctx context.Context) (*net.Resolver, error) {
	endpoint, err := selectedDNSServer(ctx)
	if err != nil {
		return nil, err
	}
	// PreferGo and a custom Dial keep both UDP queries and TCP retries on the
	// selected source. The resolver may use the local hosts file, but it never
	// invokes the native system DNS transport as an error fallback.
	// https://pkg.go.dev/net#Resolver
	return &net.Resolver{PreferGo: true, StrictErrors: true, Dial: func(dialCtx context.Context, network, _ string) (net.Conn, error) {
		return platform.DialContext(dialCtx, network, endpoint, 0)
	}}, nil
}

func resolveBoundHost(ctx context.Context, host string, timeout time.Duration, binding *platform.SourceBinding) ([]string, error) {
	if _, err := netip.ParseAddr(host); err == nil {
		address, err := compatibleSourceTarget(binding, host)
		if err != nil {
			return nil, err
		}
		return []string{address}, nil
	}
	resolver, err := boundResolver(ctx)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	network := "ip4"
	if binding.Family == "ipv6" {
		network = "ip6"
	}
	ips, err := resolver.LookupIP(ctx, network, host)
	if err != nil {
		return nil, fmt.Errorf("resolve target through selected source: %w", err)
	}
	addresses, seen := []string{}, map[string]bool{}
	for _, ip := range ips {
		address, err := compatibleSourceTarget(binding, ip.String())
		if err == nil && !seen[address] {
			addresses, seen[address] = append(addresses, address), true
		}
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("hostname returned no addresses compatible with selected %s source", binding.Family)
	}
	return addresses, nil
}

// diagnosticDialContext preserves automatic routing exactly. When a source is
// selected it resolves through that adapter's DNS and passes only literal
// endpoints to the platform binder, including for every HTTP redirect.
func diagnosticDialContext(ctx context.Context, network, endpoint string, timeout time.Duration) (net.Conn, error) {
	if _, selected := platform.SourceBindingFromContext(ctx); !selected {
		return platform.DialContext(ctx, network, endpoint, timeout)
	}
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	addresses, err := resolveHost(ctx, host, timeout)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, address := range addresses {
		conn, err := platform.DialContext(ctx, network, net.JoinHostPort(address, port), timeout)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}
