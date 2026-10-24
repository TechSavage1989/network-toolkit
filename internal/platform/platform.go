package platform

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"networktoolkit/internal/model"
)

// PingReply reports only observed ICMP information. An empty Address means
// that no ICMP reply was received. TTL is zero when the platform omits it.
type PingReply struct {
	Time     string  `json:"time,omitempty"`
	Address  string  `json:"address"`
	Sequence int     `json:"sequence"`
	Success  bool    `json:"success"`
	RTTMs    float64 `json:"rttMs"`
	TTL      int     `json:"ttl"`
	Status   string  `json:"status"`
	Error    string  `json:"error,omitempty"`
}

func basicSystem(ctx context.Context) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	hostname, _ := os.Hostname()
	out := map[string]any{"hostname": hostname, "os": runtime.GOOS, "arch": runtime.GOARCH,
		"interfaces": []map[string]any{}, "dnsServers": []string{}, "gateways": []string{}, "warnings": []string{}}
	list, err := net.Interfaces()
	if err != nil {
		return out, fmt.Errorf("read network interfaces: %w", err)
	}
	interfaces := make([]map[string]any, 0, len(list))
	warnings := []string{}
	for _, v := range list {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		addresses := []string{}
		addrs, aerr := v.Addrs()
		if aerr != nil {
			warnings = append(warnings, "Could not read addresses for "+v.Name+": "+aerr.Error())
		}
		for _, a := range addrs {
			addresses = append(addresses, a.String())
		}
		interfaces = append(interfaces, map[string]any{"index": v.Index, "name": v.Name, "mac": v.HardwareAddr.String(),
			"mtu": v.MTU, "up": v.Flags&net.FlagUp != 0, "loopback": v.Flags&net.FlagLoopback != 0,
			"addresses": addresses, "dns": []string{}, "gateways": []string{}})
	}
	out["interfaces"] = interfaces
	out["warnings"] = warnings
	return out, nil
}

func normalizeSystem(data map[string]any) {
	if list, ok := data["interfaces"].([]map[string]any); ok {
		sort.SliceStable(list, func(i, j int) bool {
			a, b := list[i], list[j]
			if a["loopback"] != b["loopback"] {
				return a["loopback"] == false
			}
			if a["up"] != b["up"] {
				return a["up"] == true
			}
			return fmt.Sprint(a["name"]) < fmt.Sprint(b["name"])
		})
	}
}

func appendUnique(items []string, value string) []string {
	if value == "" {
		return items
	}
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func resolvePingTarget(ctx context.Context, target string) (net.IPAddr, error) {
	if binding, selected := SourceBindingFromContext(ctx); selected {
		return sourceTarget(*binding, target)
	}
	target = strings.TrimSpace(target)
	if target == "" || len(target) > 253 || strings.ContainsAny(target, "\x00\r\n\t /\\") {
		return net.IPAddr{}, errors.New("enter one valid hostname or IP address")
	}
	target = strings.TrimPrefix(strings.TrimSuffix(target, "]"), "[")
	if ip := net.ParseIP(target); ip != nil {
		return net.IPAddr{IP: ip}, nil
	}
	if p := strings.LastIndexByte(target, '%'); p > 0 {
		if ip := net.ParseIP(target[:p]); ip != nil && ip.To4() == nil {
			return net.IPAddr{IP: ip, Zone: target[p+1:]}, nil
		}
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, target)
	if err != nil {
		return net.IPAddr{}, fmt.Errorf("resolve target: %w", err)
	}
	if len(addresses) == 0 {
		return net.IPAddr{}, errors.New("target resolved to no addresses")
	}
	for _, addr := range addresses {
		if addr.IP.To4() != nil {
			return addr, nil
		}
	}
	return addresses[0], nil
}

func effectiveTimeout(ctx context.Context, timeout time.Duration) (time.Duration, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	if timeout > 10*time.Second {
		timeout = 10 * time.Second
	}
	if deadline, ok := ctx.Deadline(); ok {
		if left := time.Until(deadline); left < timeout {
			timeout = left
		}
	}
	if timeout <= 0 {
		return 0, context.DeadlineExceeded
	}
	return timeout, nil
}

func magicPacket(mac string) ([]byte, net.HardwareAddr, error) {
	mac = strings.TrimSpace(mac)
	// Also accept the plain 12-hex-digit notation printed by some devices.
	if len(mac) == 12 && !strings.ContainsAny(mac, ":-.") {
		if raw, err := hex.DecodeString(mac); err == nil {
			mac = net.HardwareAddr(raw).String()
		}
	}
	addr, err := net.ParseMAC(mac)
	if err != nil || len(addr) != 6 {
		return nil, nil, errors.New("enter a valid 6-byte MAC address")
	}
	zero := true
	for _, b := range addr {
		if b != 0 {
			zero = false
		}
	}
	if zero || addr[0]&1 != 0 {
		return nil, nil, errors.New("Wake-on-LAN requires an individual device MAC address, not a zero, multicast, or broadcast address")
	}
	packet := make([]byte, 102)
	for i := 0; i < 6; i++ {
		packet[i] = 0xff
	}
	for i := 0; i < 16; i++ {
		copy(packet[6+i*6:], addr)
	}
	return packet, addr, nil
}

// Wake sends a standard UDP magic packet. Delivery and waking cannot be
// confirmed by sending alone, so the result deliberately reports only sent.
func Wake(ctx context.Context, mac, broadcast string, port int) (model.Result, error) {
	packet, addr, err := magicPacket(mac)
	if err != nil {
		return model.Result{}, err
	}
	if broadcast == "" {
		broadcast = "255.255.255.255"
	}
	ip := net.ParseIP(strings.TrimSpace(broadcast))
	if ip == nil || ip.To4() == nil || ip.IsUnspecified() || ip.IsMulticast() {
		return model.Result{}, errors.New("enter a valid IPv4 broadcast or destination address")
	}
	if port < 1 || port > 65535 {
		return model.Result{}, errors.New("UDP port must be from 1 to 65535")
	}
	if err := ctx.Err(); err != nil {
		return model.Result{}, err
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero})
	if err != nil {
		return model.Result{}, fmt.Errorf("open UDP socket: %w", err)
	}
	defer conn.Close()
	// Windows requires SO_BROADCAST before sending to a broadcast address.
	if err = enableBroadcast(conn); err != nil {
		return model.Result{}, fmt.Errorf("enable UDP broadcast: %w", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetWriteDeadline(deadline)
	n, err := conn.WriteToUDP(packet, &net.UDPAddr{IP: ip.To4(), Port: port})
	if err != nil {
		return model.Result{}, fmt.Errorf("send magic packet: %w", err)
	}
	return model.Result{Summary: "Wake-on-LAN packet sent. Device wake-up has not been confirmed.", Data: map[string]any{
		"mac": addr.String(), "destination": ip.String(), "port": port, "bytesSent": n, "status": "sent",
		"note": "The device must support Wake-on-LAN, have it enabled, and be reachable from this broadcast network."}}, nil
}
