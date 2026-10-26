//go:build !windows

package platform

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"networktoolkit/internal/model"
)

func ConfigureCommand(cmd *exec.Cmd) { cmd.WaitDelay = 2 * time.Second }

func OpenBrowser(address string) error {
	command := "xdg-open"
	if runtime.GOOS == "darwin" {
		command = "open"
	}
	path, err := exec.LookPath(command)
	if err != nil {
		return errors.New("no browser launcher found; open the printed local address manually")
	}
	cmd := exec.Command(path, address)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

func NotifyFatal(message string) { fmt.Fprintln(os.Stderr, "Network Toolkit:", message) }

func enableBroadcast(conn *net.UDPConn) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var setErr error
	err = raw.Control(func(fd uintptr) { setErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1) })
	if err != nil {
		return err
	}
	return setErr
}

func System(ctx context.Context) (map[string]any, error) {
	out, err := basicSystem(ctx)
	if out == nil {
		return nil, err
	}
	dns := []string{}
	if raw, readErr := os.ReadFile("/etc/resolv.conf"); readErr == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "nameserver" {
				dns = appendUnique(dns, fields[1])
			}
		}
	}
	out["dnsServers"] = dns
	if raw, readErr := os.ReadFile("/proc/net/route"); readErr == nil {
		routes := parseLinuxRoutes(string(raw))
		gateways := []string{}
		for _, r := range routes {
			if r["destination"] != "0.0.0.0" || r["gateway"] == "0.0.0.0" {
				continue
			}
			gateway := fmt.Sprint(r["gateway"])
			gateways = appendUnique(gateways, gateway)
			if interfaces, ok := out["interfaces"].([]map[string]any); ok {
				for _, nic := range interfaces {
					if nic["name"] == r["interface"] {
						nic["gateway"] = gateway
						nic["gateways"] = []string{gateway}
					}
				}
			}
		}
		out["gateways"] = gateways
	}
	normalizeSystem(out)
	return out, err
}

var unixRTT = regexp.MustCompile(`time[=<]([0-9.]+)\s*ms`)
var unixTTL = regexp.MustCompile(`(?:ttl|hlim)=([0-9]+)`)
var unixReplyAddress = regexp.MustCompile(`bytes from\s+([^\s:(]+|[0-9a-fA-F:]+)(?:\s|:)`)
var unixFromAddress = regexp.MustCompile(`(?m)^From\s+([^\s]+)`)

func Ping(ctx context.Context, target string, timeout time.Duration, ttl int) (PingReply, error) {
	timeout, err := effectiveTimeout(ctx, timeout)
	if err != nil {
		return PingReply{}, err
	}
	lookupCtx, cancelLookup := context.WithTimeout(ctx, timeout)
	targetAddress, err := resolvePingTarget(lookupCtx, target)
	cancelLookup()
	if err != nil {
		return PingReply{}, err
	}
	path, err := exec.LookPath("ping")
	if err != nil {
		return PingReply{Status: "unavailable"}, errors.New("this development host has no ping utility; the Windows executable uses built-in native ICMP APIs")
	}
	if ttl < 0 || ttl > 255 {
		return PingReply{}, errors.New("TTL must be from 0 to 255")
	}
	binding, selected := SourceBindingFromContext(ctx)
	sourceArgs, err := sourcePingArgs(binding, runtime.GOOS)
	if err != nil {
		return PingReply{}, err
	}
	if selected {
		if err := ensureSourcePingUtility(ctx, path); err != nil {
			return PingReply{}, err
		}
	}
	seconds := int(math.Ceil(timeout.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	args := []string{"-n", "-c", "1", "-W", strconv.Itoa(seconds)}
	if targetAddress.IP.To4() == nil {
		args = append(args, "-6")
	}
	if ttl > 0 {
		args = append(args, "-t", strconv.Itoa(ttl))
	}
	args = append(args, sourceArgs...)
	args = append(args, targetAddress.String())
	commandCtx, cancel := context.WithTimeout(ctx, timeout+250*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(commandCtx, path, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	ConfigureCommand(cmd)
	output, runErr := cmd.CombinedOutput()
	if err := ctx.Err(); err != nil {
		return PingReply{}, err
	}
	text := string(output)
	reply := PingReply{Status: "timeout", Error: "No ICMP response before the timeout"}
	if runErr == nil {
		reply.Success = true
		reply.Status = "reply"
		reply.Error = ""
		reply.Address = targetAddress.String()
		if match := unixReplyAddress.FindStringSubmatch(text); len(match) > 1 {
			address := strings.TrimSuffix(match[1], ":")
			if net.ParseIP(address) != nil {
				reply.Address = address
			}
		}
		if match := unixRTT.FindStringSubmatch(text); len(match) > 1 {
			reply.RTTMs, _ = strconv.ParseFloat(match[1], 64)
		}
		if match := unixTTL.FindStringSubmatch(text); len(match) > 1 {
			reply.TTL, _ = strconv.Atoi(match[1])
		}
		return reply, nil
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "time to live exceeded") || strings.Contains(lower, "time exceeded") {
		reply.Status = "ttl_expired"
		reply.Error = "TTL or hop limit expired in transit"
	} else if strings.Contains(lower, "unreachable") {
		reply.Status = "unreachable"
		reply.Error = "Destination unreachable"
	}
	if reply.Status != "timeout" {
		if match := unixFromAddress.FindStringSubmatch(text); len(match) > 1 {
			value := strings.TrimSuffix(match[1], ":")
			if net.ParseIP(value) != nil {
				reply.Address = value
			}
		}
		return reply, nil
	}
	if code, ok := runErr.(*exec.ExitError); ok && code.ExitCode() > 1 && commandCtx.Err() == nil {
		return PingReply{Status: "error", Error: strings.TrimSpace(text)}, fmt.Errorf("ping utility failed: %s", strings.TrimSpace(text))
	}
	return reply, nil
}

func Snapshot(ctx context.Context, tool string) (model.Result, error) {
	if err := ctx.Err(); err != nil {
		return model.Result{}, err
	}
	switch tool {
	case "routes":
		raw, err := os.ReadFile("/proc/net/route")
		recorded := snapshotTime()
		if err != nil {
			return model.Result{}, fmt.Errorf("routing snapshot unavailable on this development host: %w", err)
		}
		return model.Result{Time: recorded, Summary: "IPv4 routing table captured", Data: map[string]any{"time": recorded, "captured": recorded, "timeKind": "snapshot", "routes": stampSnapshotRows(parseLinuxRoutes(string(raw)), recorded), "output": string(raw), "note": "The portable Windows executable shows both IPv4 and IPv6 routes."}}, nil
	case "arp":
		rows, err := Neighbors(ctx)
		data := map[string]any{"neighbors": rows, "note": neighborCacheNote}
		if raw, readErr := os.ReadFile("/proc/net/arp"); readErr == nil {
			data["output"] = string(raw)
		}
		summary := "IPv4 and IPv6 neighbor caches captured"
		if err != nil {
			summary = "Neighbor cache could not be fully read"
			data["warnings"] = []string{err.Error()}
		}
		recorded := snapshotTime()
		data["time"], data["captured"], data["timeKind"] = recorded, recorded, "snapshot"
		return model.Result{Time: recorded, Summary: summary, Data: data}, err
	case "ipconfig":
		data, err := System(ctx)
		recorded := snapshotTime()
		stampSystemSnapshot(data, recorded)
		return model.Result{Time: recorded, Summary: "Local network configuration captured", Data: data}, err
	case "connections":
		outputs := []string{}
		sections := []map[string]any{}
		for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6", "/proc/net/udp", "/proc/net/udp6"} {
			if data, err := os.ReadFile(path); err == nil {
				outputs = append(outputs, path+"\n"+string(data))
				sections = append(sections, map[string]any{"title": path, "output": string(data), "time": snapshotTime(), "timeKind": "snapshot", "success": true})
			}
		}
		if len(outputs) == 0 {
			return model.Result{}, errors.New("connection snapshot unavailable on this development host")
		}
		recorded := snapshotTime()
		return model.Result{Time: recorded, Summary: "Kernel socket tables captured", Data: map[string]any{"time": recorded, "captured": recorded, "timeKind": "snapshot", "sections": sections, "output": strings.Join(outputs, "\n"), "note": "The portable Windows executable presents readable endpoint addresses and process IDs."}}, nil
	case "wifi":
		return model.Result{}, errors.New("wireless adapter inspection is implemented for the portable Windows executable")
	default:
		return model.Result{}, fmt.Errorf("unsupported snapshot: %s", tool)
	}
}

func parseLinuxRoutes(text string) []map[string]any {
	rows := []map[string]any{}
	for _, line := range strings.Split(text, "\n") {
		f := strings.Fields(line)
		if len(f) < 8 || f[0] == "Iface" {
			continue
		}
		decode := func(value string) string {
			raw, err := hex.DecodeString(value)
			if err != nil || len(raw) != 4 {
				return ""
			}
			n := binary.BigEndian.Uint32(raw)
			return net.IPv4(byte(n), byte(n>>8), byte(n>>16), byte(n>>24)).String()
		}
		dest, gw, mask := decode(f[1]), decode(f[2]), decode(f[7])
		if dest == "" || gw == "" || mask == "" {
			continue
		}
		metric, _ := strconv.Atoi(f[6])
		rows = append(rows, map[string]any{"interface": f[0], "destination": dest, "gateway": gw, "mask": mask, "metric": metric})
	}
	return rows
}
