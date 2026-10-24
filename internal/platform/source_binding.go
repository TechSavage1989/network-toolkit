package platform

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// SourceBinding is a validated snapshot of one local adapter and source address.
// InterfaceIndex is the adapter's primary index; Windows may have a different
// IPv6InterfaceIndex. Re-resolve a saved selection before beginning a new job.
type SourceBinding struct {
	InterfaceIndex       int      `json:"interfaceIndex"`
	IPv6InterfaceIndex   int      `json:"ipv6InterfaceIndex,omitempty"`
	InterfaceName        string   `json:"interfaceName"`
	InterfaceDescription string   `json:"interfaceDescription,omitempty"`
	InterfaceID          string   `json:"interfaceId,omitempty"`
	Address              string   `json:"address"`
	Family               string   `json:"family"`
	DNSServers           []string `json:"dnsServers,omitempty"`
}

type sourceBindingKey struct{}

func copySourceBinding(binding *SourceBinding) *SourceBinding {
	if binding == nil {
		return nil
	}
	copy := *binding
	copy.DNSServers = append([]string(nil), binding.DNSServers...)
	return &copy
}

// WithSourceBinding preserves Automatic routing when binding is nil. It copies
// the selection so concurrent probes cannot mutate each other's source choice.
func WithSourceBinding(ctx context.Context, binding *SourceBinding) context.Context {
	return context.WithValue(ctx, sourceBindingKey{}, copySourceBinding(binding))
}

func SourceBindingFromContext(ctx context.Context) (*SourceBinding, bool) {
	binding, ok := ctx.Value(sourceBindingKey{}).(*SourceBinding)
	return copySourceBinding(binding), ok && binding != nil
}

type sourceInterface struct {
	index, ipv6Index      int
	name, description, id string
	up                    bool
	addresses, dns        []string
	addressStates         map[string]string
}

// ResolveSourceBinding checks current address ownership and operational state.
// Empty options mean Automatic; incomplete, stale, or invalid options are errors.
func ResolveSourceBinding(ctx context.Context, index int, name, address string) (*SourceBinding, error) {
	return resolveSourceBinding(ctx, index, name, address, readSourceInterfaces)
}

func resolveSourceBinding(ctx context.Context, index int, name, address string, read func(context.Context) ([]sourceInterface, error)) (*SourceBinding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	name, address = strings.TrimSpace(name), strings.TrimSpace(address)
	if index == 0 && name == "" && address == "" {
		return nil, nil
	}
	if index <= 0 || uint64(index) > uint64(^uint32(0)) || address == "" {
		return nil, errors.New("choose a current source adapter and one of its IP addresses, or choose Automatic")
	}
	ip, err := netip.ParseAddr(address)
	if err != nil || ip.IsUnspecified() || ip.IsMulticast() || ip.Unmap() == netip.MustParseAddr("255.255.255.255") {
		return nil, errors.New("source address must be a literal, assigned unicast IP address")
	}
	ip = ip.Unmap()
	interfaces, err := read(ctx)
	if err != nil {
		return nil, fmt.Errorf("validate source adapter: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var selected *sourceInterface
	for i := range interfaces {
		if interfaces[i].index == index {
			if selected != nil {
				return nil, errors.New("source adapter index is ambiguous; refresh the adapter list")
			}
			selected = &interfaces[i]
		}
	}
	if selected == nil {
		return nil, errors.New("selected source adapter is no longer available; refresh the adapter list")
	}
	if name != "" && name != selected.name {
		return nil, errors.New("selected source adapter has changed; refresh the adapter list and select it again")
	}
	if !selected.up {
		return nil, fmt.Errorf("source adapter %q is not up", selected.name)
	}
	binding := &SourceBinding{InterfaceIndex: selected.index, IPv6InterfaceIndex: selected.ipv6Index,
		InterfaceName: selected.name, InterfaceDescription: selected.description, InterfaceID: selected.id,
		Address: ip.WithZone("").String(), Family: "ipv6", DNSServers: append([]string(nil), selected.dns...)}
	if ip.Is4() {
		binding.Family = "ipv4"
	}
	if !binding.matchesZone(ip.Zone()) {
		return nil, errors.New("source IPv6 scope does not match the selected adapter")
	}
	if !sourceInterfaceHasIP(*selected, ip) {
		return nil, fmt.Errorf("source address %s is no longer assigned to %q; refresh the adapter list", binding.Address, selected.name)
	}
	if state := selected.addressStates[binding.Address]; state == "invalid" || state == "tentative" || state == "duplicate" {
		return nil, fmt.Errorf("source address %s is %s and cannot be used", binding.Address, state)
	}
	// Windows IPv4 ICMP selects a source address rather than a socket interface.
	// A duplicate non-scoped source cannot identify one adapter unambiguously.
	if !ip.IsLinkLocalUnicast() || ip.Is4() {
		for _, other := range interfaces {
			if other.index != selected.index && other.up && sourceInterfaceHasIP(other, ip) {
				return nil, fmt.Errorf("source address %s belongs to more than one active adapter; choose a unique source address", binding.Address)
			}
		}
	}
	return binding, nil
}

func sourceInterfaceHasIP(nic sourceInterface, address netip.Addr) bool {
	for _, raw := range nic.addresses {
		ip, err := netip.ParseAddr(strings.SplitN(raw, "/", 2)[0])
		if err == nil && ip.Unmap().WithZone("") == address.Unmap().WithZone("") {
			return true
		}
	}
	return false
}

func (binding SourceBinding) socketIndex() int {
	if binding.Family == "ipv6" && binding.IPv6InterfaceIndex > 0 {
		return binding.IPv6InterfaceIndex
	}
	return binding.InterfaceIndex
}

func (binding SourceBinding) matchesZone(zone string) bool {
	return zone == "" || zone == binding.InterfaceName || zone == strconv.Itoa(binding.socketIndex())
}

func (binding SourceBinding) sourceIP() (netip.Addr, error) {
	ip, err := netip.ParseAddr(binding.Address)
	if err != nil || binding.InterfaceIndex <= 0 || uint64(binding.InterfaceIndex) > uint64(^uint32(0)) || binding.IPv6InterfaceIndex < 0 || uint64(binding.IPv6InterfaceIndex) > uint64(^uint32(0)) || binding.InterfaceName == "" || ip.IsUnspecified() || ip.IsMulticast() || ip.Unmap() == netip.MustParseAddr("255.255.255.255") {
		return netip.Addr{}, errors.New("invalid source binding; select a current adapter address")
	}
	ip = ip.Unmap()
	if (binding.Family != "ipv4" && binding.Family != "ipv6") || (ip.Is4() != (binding.Family == "ipv4")) || !binding.matchesZone(ip.Zone()) {
		return netip.Addr{}, errors.New("source binding address family or scope is inconsistent")
	}
	return ip.WithZone(""), nil
}

// sourceTarget never resolves a hostname. Diagnostics using an explicit source
// must resolve names through that adapter's DNS before invoking a probe.
func sourceTarget(binding SourceBinding, target string) (net.IPAddr, error) {
	if _, err := binding.sourceIP(); err != nil {
		return net.IPAddr{}, err
	}
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "[") && strings.HasSuffix(target, "]") {
		target = target[1 : len(target)-1]
	}
	ip, err := netip.ParseAddr(target)
	if err != nil {
		return net.IPAddr{}, errors.New("an explicit source requires a literal destination IP resolved through the selected adapter's DNS")
	}
	ip = ip.Unmap()
	if ip.Is4() != (binding.Family == "ipv4") {
		return net.IPAddr{}, fmt.Errorf("destination address does not match the selected %s source", binding.Family)
	}
	if !binding.matchesZone(ip.Zone()) {
		return net.IPAddr{}, errors.New("destination IPv6 scope does not match the selected source adapter")
	}
	zone := ip.Zone()
	if !ip.Is4() && (zone != "" || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()) {
		zone = strconv.Itoa(binding.socketIndex())
	}
	return net.IPAddr{IP: net.IP(ip.WithZone("").AsSlice()), Zone: zone}, nil
}

// DialContext binds TCP/UDP diagnostics to the validated source address and
// adapter. Explicit source failures are returned without an automatic retry.
// A selected source accepts literal endpoints only; the caller owns bound DNS.
func DialContext(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	binding, selected := SourceBindingFromContext(ctx)
	if !selected {
		return (&net.Dialer{Timeout: timeout}).DialContext(ctx, network, address)
	}
	dialer, network, address, err := sourceDialPlan(*binding, network, address, timeout)
	if err != nil {
		return nil, err
	}
	return dialer.DialContext(ctx, network, address)
}

func sourceDialPlan(binding SourceBinding, network, address string, timeout time.Duration) (*net.Dialer, string, string, error) {
	ip, err := binding.sourceIP()
	if err != nil {
		return nil, "", "", err
	}
	base := strings.TrimSuffix(strings.TrimSuffix(network, "4"), "6")
	if base != "tcp" && base != "udp" {
		return nil, "", "", fmt.Errorf("source binding does not support %q sockets", network)
	}
	family := "6"
	if ip.Is4() {
		family = "4"
	}
	if network != base && network != base+family {
		return nil, "", "", errors.New("socket family does not match the selected source address")
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, "", "", fmt.Errorf("source-bound endpoint: %w", err)
	}
	destination, err := sourceTarget(binding, host)
	if err != nil {
		return nil, "", "", err
	}
	zone := ""
	if ip.IsLinkLocalUnicast() {
		zone = strconv.Itoa(binding.socketIndex())
	}
	dialer := &net.Dialer{Timeout: timeout}
	if base == "tcp" {
		dialer.LocalAddr = &net.TCPAddr{IP: net.IP(ip.AsSlice()), Zone: zone}
	} else {
		dialer.LocalAddr = &net.UDPAddr{IP: net.IP(ip.AsSlice()), Zone: zone}
	}
	dialer.ControlContext = func(ctx context.Context, network, address string, raw syscall.RawConn) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := setSourceInterface(raw, binding); err != nil {
			return fmt.Errorf("bind diagnostic socket to %q (%s): %w", binding.InterfaceName, binding.Address, err)
		}
		return nil
	}
	return dialer, base + family, net.JoinHostPort(destination.String(), port), nil
}
