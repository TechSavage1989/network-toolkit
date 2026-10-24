//go:build windows

package platform

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"networktoolkit/internal/model"
)

var kernel = syscall.NewLazyDLL("kernel32.dll")
var getSystemDirectory = kernel.NewProc("GetSystemDirectoryW")
var getOEMCP = kernel.NewProc("GetOEMCP")
var multiByteToWideChar = kernel.NewProc("MultiByteToWideChar")
var systemDir = windowsSystemDirectory()
var ipHelper = syscall.NewLazyDLL(filepath.Join(systemDir, "iphlpapi.dll"))
var icmpCreate = ipHelper.NewProc("IcmpCreateFile")
var icmp6Create = ipHelper.NewProc("Icmp6CreateFile")
var icmpClose = ipHelper.NewProc("IcmpCloseHandle")
var icmpSend = ipHelper.NewProc("IcmpSendEcho")
var icmpSendSource = ipHelper.NewProc("IcmpSendEcho2Ex")
var icmp6Send = ipHelper.NewProc("Icmp6SendEcho2")
var getAdapters = ipHelper.NewProc("GetAdaptersAddresses")

func windowsSystemDirectory() string {
	buf := make([]uint16, 32768)
	n, _, _ := getSystemDirectory.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n > 0 && n < uintptr(len(buf)) {
		return syscall.UTF16ToString(buf[:n])
	}
	// The system directory query does not depend on user-modifiable environment
	// variables. This path is only a fallback when the Windows API fails.
	return `C:\Windows\System32`
}

func ConfigureCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	cmd.WaitDelay = 2 * time.Second
}

func OpenBrowser(address string) error {
	parsed, err := url.Parse(address)
	if err != nil || parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" {
		return errors.New("the app browser address must be a local HTTP address")
	}
	op, _ := syscall.UTF16PtrFromString("open")
	file, err := syscall.UTF16PtrFromString(address)
	if err != nil {
		return err
	}
	proc := syscall.NewLazyDLL(filepath.Join(systemDir, "shell32.dll")).NewProc("ShellExecuteW")
	n, _, _ := proc.Call(0, uintptr(unsafe.Pointer(op)), uintptr(unsafe.Pointer(file)), 0, 0, 1)
	runtime.KeepAlive(op)
	runtime.KeepAlive(file)
	if n <= 32 {
		return fmt.Errorf("open default browser: Windows shell code %d", n)
	}
	return nil
}

func NotifyFatal(message string) {
	title, _ := syscall.UTF16PtrFromString("Network Toolkit")
	body, _ := syscall.UTF16PtrFromString(strings.ReplaceAll(message, "\x00", ""))
	proc := syscall.NewLazyDLL(filepath.Join(systemDir, "user32.dll")).NewProc("MessageBoxW")
	_, _, _ = proc.Call(0, uintptr(unsafe.Pointer(body)), uintptr(unsafe.Pointer(title)), 0x10)
	runtime.KeepAlive(title)
	runtime.KeepAlive(body)
}

func enableBroadcast(conn *net.UDPConn) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var setErr error
	err = raw.Control(func(fd uintptr) {
		setErr = syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
	})
	if err != nil {
		return err
	}
	return setErr
}

// The synchronous IcmpSendEcho entry point returns ICMP_ECHO_REPLY with native
// pointer alignment. The separate asynchronous IcmpParseReplies API has a
// different ICMP_ECHO_REPLY32 contract. No option data is supplied, so both the
// 32-bit and native option-pointer locations are zero.
type sockaddr6 struct {
	Family, Port uint16
	FlowInfo     uint32
	Address      [16]byte
	ScopeID      uint32
}

type pingResult struct {
	reply PingReply
	err   error
}

func Ping(ctx context.Context, target string, timeout time.Duration, ttl int) (PingReply, error) {
	timeout, err := effectiveTimeout(ctx, timeout)
	if err != nil {
		return PingReply{}, err
	}
	lookupCtx, cancel := context.WithTimeout(ctx, timeout)
	address, err := resolvePingTarget(lookupCtx, target)
	cancel()
	if err != nil {
		return PingReply{}, err
	}
	if ttl < 0 || ttl > 255 {
		return PingReply{}, errors.New("TTL must be from 0 to 255")
	}
	if ttl == 0 {
		ttl = 128
	}
	timeout, err = effectiveTimeout(ctx, timeout)
	if err != nil {
		return PingReply{}, err
	}
	result := make(chan pingResult, 1)
	binding, _ := SourceBindingFromContext(ctx)
	// Synchronous IP Helper calls have a bounded native timeout. A cancelled
	// caller returns immediately; the outstanding call retains its own buffers
	// and handle until Windows returns, avoiding pointer/handle lifetime races.
	go func() {
		reply, err := nativePing(address, timeout, ttl, binding)
		result <- pingResult{reply, err}
	}()
	select {
	case <-ctx.Done():
		return PingReply{}, ctx.Err()
	case r := <-result:
		return r.reply, r.err
	}
}

func nativePing(address net.IPAddr, timeout time.Duration, ttl int, binding *SourceBinding) (PingReply, error) {
	var selectedIP net.IP
	if binding != nil {
		ip, err := binding.sourceIP()
		if err != nil {
			return PingReply{}, err
		}
		selectedIP = net.IP(ip.AsSlice())
	}
	ipv4 := address.IP.To4()
	create, send := icmpCreate, icmpSend
	if ipv4 == nil {
		create, send = icmp6Create, icmp6Send
	} else if binding != nil {
		send = icmpSendSource
	}
	if err := create.Find(); err != nil {
		return PingReply{}, fmt.Errorf("ICMP API is unavailable: %w", err)
	}
	if err := send.Find(); err != nil {
		return PingReply{}, fmt.Errorf("ICMP API is unavailable: %w", err)
	}
	handle, _, err := create.Call()
	if handle == 0 || handle == ^uintptr(0) {
		return PingReply{}, fmt.Errorf("open ICMP handle: %w", err)
	}
	defer icmpClose.Call(handle)
	payload := []byte("NetworkToolkit diagnostic probe")
	response := make([]byte, 2048)
	options := icmpOptions{TTL: uint8(ttl)}
	timeoutMs := uintptr((timeout + time.Millisecond - 1) / time.Millisecond)
	var n uintptr
	if ipv4 != nil {
		if binding == nil {
			n, _, err = send.Call(handle, uintptr(binary.LittleEndian.Uint32(ipv4)),
				uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)), uintptr(unsafe.Pointer(&options)),
				uintptr(unsafe.Pointer(&response[0])), uintptr(len(response)), timeoutMs)
		} else {
			// IcmpSendEcho2Ex selects the IPv4 source without requiring a raw socket.
			// Event/APC are nil, preserving the synchronous buffer-lifetime contract.
			source := selectedIP.To4()
			n, _, err = send.Call(handle, 0, 0, 0, uintptr(binary.LittleEndian.Uint32(source)), uintptr(binary.LittleEndian.Uint32(ipv4)),
				uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)), uintptr(unsafe.Pointer(&options)),
				uintptr(unsafe.Pointer(&response[0])), uintptr(len(response)), timeoutMs)
		}
	} else {
		source := sockaddr6{Family: 23}
		dest := sockaddr6{Family: 23}
		if binding != nil {
			copy(source.Address[:], selectedIP.To16())
			if selectedIP.IsLinkLocalUnicast() {
				source.ScopeID = uint32(binding.socketIndex())
			}
		}
		copy(dest.Address[:], address.IP.To16())
		if address.Zone != "" {
			index, convErr := strconv.ParseUint(address.Zone, 10, 32)
			if convErr != nil {
				nic, nicErr := net.InterfaceByName(address.Zone)
				if nicErr != nil {
					return PingReply{}, fmt.Errorf("IPv6 scope: %w", nicErr)
				}
				index = uint64(nic.Index)
			}
			dest.ScopeID = uint32(index)
			if binding == nil {
				source.ScopeID = uint32(index)
			}
		}
		n, _, err = send.Call(handle, 0, 0, 0, uintptr(unsafe.Pointer(&source)), uintptr(unsafe.Pointer(&dest)),
			uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)), uintptr(unsafe.Pointer(&options)),
			uintptr(unsafe.Pointer(&response[0])), uintptr(len(response)), timeoutMs)
		runtime.KeepAlive(source)
		runtime.KeepAlive(dest)
	}
	runtime.KeepAlive(payload)
	runtime.KeepAlive(options)
	runtime.KeepAlive(response)
	if n == 0 {
		code := uint32(0)
		if e, ok := err.(syscall.Errno); ok {
			code = uint32(e)
		}
		if code >= 11001 && code <= 11050 {
			// Windows can expose an ICMP error reply while returning zero
			// successful echo replies. Retain that router's address for trace.
			statusOffset := 4
			if ipv4 == nil {
				statusOffset = 28
			}
			if binary.LittleEndian.Uint32(response[statusOffset:statusOffset+4]) == code && code != 11010 {
				var reply PingReply
				if ipv4 != nil {
					reply = parseICMP4Reply(response)
				} else {
					reply = parseICMP6Reply(response)
				}
				if reply.Address != "" {
					return reply, nil
				}
			}
			status, message := icmpStatus(code)
			return PingReply{Status: status, Error: message}, nil
		}
		return PingReply{Status: "error", Error: err.Error()}, fmt.Errorf("ICMP request: %w", err)
	}
	if ipv4 != nil {
		return parseICMP4Reply(response), nil
	}
	return parseICMP6Reply(response), nil
}

type socketAddress struct {
	Address unsafe.Pointer
	Length  int32
}
type addressNode struct {
	Length, Flags uint32
	Next          *addressNode
	Socket        socketAddress
}
type unicastNode struct {
	Length, Flags                                   uint32
	Next                                            *unicastNode
	Socket                                          socketAddress
	PrefixOrigin, SuffixOrigin, DADState            int32
	ValidLifetime, PreferredLifetime, LeaseLifetime uint32
	PrefixLength                                    uint8
}
type adapterAddress struct {
	Length, Index                                      uint32
	Next                                               *adapterAddress
	AdapterName                                        *byte
	Unicast                                            *unicastNode
	Anycast, Multicast                                 *addressNode
	DNS                                                *addressNode
	DNSSuffix, Description, FriendlyName               *uint16
	MAC                                                [8]byte
	MACLength, Flags, MTU, Type, OperStatus, IPv6Index uint32
	ZoneIndices                                        [16]uint32
	FirstPrefix                                        unsafe.Pointer
	TransmitLinkSpeed, ReceiveLinkSpeed                uint64
	WINSServer, Gateway                                *addressNode
}

func System(ctx context.Context) (map[string]any, error) {
	out, baseErr := basicSystem(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		return nil, baseErr
	}
	native, err := readWindowsAdapters()
	if err != nil {
		warnings, _ := out["warnings"].([]string)
		out["warnings"] = append(warnings, "Additional adapter information unavailable: "+err.Error())
		normalizeSystem(out)
		return out, baseErr
	}
	out["interfaces"] = native
	dns, gateways := []string{}, []string{}
	for _, nic := range native {
		if nic["up"] != true {
			continue
		}
		for _, value := range nic["dns"].([]string) {
			dns = appendUnique(dns, value)
		}
		for _, value := range nic["gateways"].([]string) {
			gateways = appendUnique(gateways, value)
		}
	}
	out["dnsServers"], out["gateways"] = dns, gateways
	normalizeSystem(out)
	return out, nil
}

func readWindowsAdapters() ([]map[string]any, error) {
	if err := getAdapters.Find(); err != nil {
		return nil, err
	}
	size := uint32(15000)
	var buf []byte
	for attempt := 0; attempt < 3; attempt++ {
		if size == 0 || size > 16*1024*1024 {
			return nil, errors.New("unexpected adapter buffer size")
		}
		buf = make([]byte, size)
		// Skip multicast/anycast; include gateways. Read every address family.
		result, _, _ := getAdapters.Call(0, 0x02|0x04|0x80, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
		if result == 111 {
			continue
		}
		if result == 232 {
			return []map[string]any{}, nil
		}
		if result != 0 {
			return nil, fmt.Errorf("GetAdaptersAddresses: %w", syscall.Errno(result))
		}
		adapters := []map[string]any{}
		count := 0
		for ptr := (*adapterAddress)(unsafe.Pointer(&buf[0])); ptr != nil && count < 4096; ptr = ptr.Next {
			count++
			if uintptr(ptr.Length) < unsafe.Sizeof(adapterAddress{}) {
				return nil, errors.New("unsupported adapter structure layout")
			}
			index := ptr.Index
			if index == 0 {
				index = ptr.IPv6Index
			}
			name := readUTF16(ptr.FriendlyName)
			if name == "" {
				name = fmt.Sprintf("Interface %d", index)
			}
			macLength := int(ptr.MACLength)
			if macLength > len(ptr.MAC) {
				macLength = len(ptr.MAC)
			}
			dns, gateways, addresses := []string{}, []string{}, []string{}
			for p, n := ptr.DNS, 0; p != nil && n < 512; p, n = p.Next, n+1 {
				dns = appendUnique(dns, readSocketIP(p.Socket))
			}
			for p, n := ptr.Gateway, 0; p != nil && n < 512; p, n = p.Next, n+1 {
				gateways = appendUnique(gateways, readSocketIP(p.Socket))
			}
			addressDetails := []map[string]any{}
			for p, n := ptr.Unicast, 0; p != nil && n < 2048; p, n = p.Next, n+1 {
				ip := readSocketIP(p.Socket)
				if ip == "" {
					continue
				}
				prefix := int(p.PrefixLength)
				addresses = append(addresses, ip+"/"+strconv.Itoa(prefix))
				origin := "other"
				switch p.PrefixOrigin {
				case 1:
					origin = "manual"
				case 3:
					origin = "DHCP"
				case 4:
					origin = "router advertisement"
				}
				state := "unknown"
				switch p.DADState {
				case 0:
					state = "invalid"
				case 1:
					state = "tentative"
				case 2:
					state = "duplicate"
				case 3:
					state = "deprecated"
				case 4:
					state = "preferred"
				}
				addressDetails = append(addressDetails, map[string]any{"address": ip, "prefix": prefix, "origin": origin, "state": state})
			}
			kind := "other"
			switch ptr.Type {
			case 6:
				kind = "Ethernet"
			case 24:
				kind = "Loopback"
			case 71:
				kind = "Wi-Fi"
			case 131:
				kind = "Tunnel"
			case 23:
				kind = "PPP"
			}
			state := "unknown"
			switch ptr.OperStatus {
			case 1:
				state = "up"
			case 2:
				state = "down"
			case 3:
				state = "testing"
			case 5:
				state = "dormant"
			case 6:
				state = "not present"
			case 7:
				state = "lower layer down"
			}
			row := map[string]any{"index": int(index), "name": name, "description": readUTF16(ptr.Description),
				"mac": net.HardwareAddr(ptr.MAC[:macLength]).String(), "mtu": ptr.MTU, "up": ptr.OperStatus == 1,
				"loopback": ptr.Type == 24, "status": state, "type": kind, "addresses": addresses,
				"addressDetails": addressDetails, "dns": dns, "gateways": gateways, "dnsSuffix": readUTF16(ptr.DNSSuffix),
				"dhcpEnabled": ptr.Flags&4 != 0, "ipv4Enabled": ptr.Flags&128 != 0, "ipv6Enabled": ptr.Flags&256 != 0}
			if id := readAdapterID(buf, ptr.AdapterName); id != "" {
				row["id"] = id
			}
			if ptr.IPv6Index > 0 {
				row["ipv6Index"] = int(ptr.IPv6Index)
			}
			if len(gateways) > 0 {
				row["gateway"] = gateways[0]
			}
			if ptr.TransmitLinkSpeed > 0 && ptr.TransmitLinkSpeed != ^uint64(0) {
				row["speedMbps"] = float64(ptr.TransmitLinkSpeed) / 1e6
				row["transmitMbps"] = float64(ptr.TransmitLinkSpeed) / 1e6
			}
			if ptr.ReceiveLinkSpeed > 0 && ptr.ReceiveLinkSpeed != ^uint64(0) {
				row["receiveMbps"] = float64(ptr.ReceiveLinkSpeed) / 1e6
			}
			adapters = append(adapters, row)
		}
		runtime.KeepAlive(buf)
		return adapters, nil
	}
	return nil, errors.New("adapter list changed repeatedly while being read")
}

// AdapterName is the permanent native identifier, separately from the mutable
// friendly name and the hardware description. Keep reads inside the API buffer.
func readAdapterID(buffer []byte, pointer *byte) string {
	if len(buffer) == 0 || pointer == nil {
		return ""
	}
	start := uintptr(unsafe.Pointer(&buffer[0]))
	address := uintptr(unsafe.Pointer(pointer))
	if address < start || address-start >= uintptr(len(buffer)) {
		return ""
	}
	remaining := buffer[int(address-start):]
	if len(remaining) > 128 {
		remaining = remaining[:128]
	}
	for i, value := range remaining {
		if value == 0 {
			return string(remaining[:i])
		}
		if value > 127 {
			return ""
		}
	}
	return ""
}

func readUTF16(ptr *uint16) string {
	if ptr == nil {
		return ""
	}
	chars := []uint16{}
	for i := 0; i < 32768; i++ {
		value := *(*uint16)(unsafe.Add(unsafe.Pointer(ptr), i*2))
		if value == 0 {
			break
		}
		chars = append(chars, value)
	}
	return string(utf16.Decode(chars))
}

func readSocketIP(socket socketAddress) string {
	if socket.Address == nil || socket.Length < 2 {
		return ""
	}
	family := *(*uint16)(socket.Address)
	if family == 2 && socket.Length >= 16 {
		address := (*syscall.RawSockaddrInet4)(socket.Address)
		return net.IP(address.Addr[:]).String()
	}
	if family == 23 && socket.Length >= 28 {
		address := (*syscall.RawSockaddrInet6)(socket.Address)
		ip := net.IP(address.Addr[:])
		text := ip.String()
		if ip.IsLinkLocalUnicast() && address.Scope_id != 0 {
			text += "%" + strconv.FormatUint(uint64(address.Scope_id), 10)
		}
		return text
	}
	return ""
}

func decodeCommandOutput(raw []byte) string {
	if len(raw) >= 2 && raw[0] == 0xff && raw[1] == 0xfe {
		chars := make([]uint16, (len(raw)-2)/2)
		for i := range chars {
			chars[i] = binary.LittleEndian.Uint16(raw[2+i*2:])
		}
		return string(utf16.Decode(chars))
	}
	if utf8.Valid(raw) {
		return strings.TrimPrefix(string(raw), "\ufeff")
	}
	if len(raw) == 0 {
		return ""
	}
	page, _, _ := getOEMCP.Call()
	count, _, _ := multiByteToWideChar.Call(page, 0, uintptr(unsafe.Pointer(&raw[0])), uintptr(len(raw)), 0, 0)
	if count == 0 || count > 4*1024*1024 {
		return strings.ToValidUTF8(string(raw), "�")
	}
	chars := make([]uint16, int(count))
	n, _, _ := multiByteToWideChar.Call(page, 0, uintptr(unsafe.Pointer(&raw[0])), uintptr(len(raw)), uintptr(unsafe.Pointer(&chars[0])), count)
	runtime.KeepAlive(raw)
	if n == 0 {
		return strings.ToValidUTF8(string(raw), "�")
	}
	return string(utf16.Decode(chars[:n]))
}

type commandSpec struct {
	title, exe string
	args       []string
}

func Snapshot(ctx context.Context, tool string) (model.Result, error) {
	commands := []commandSpec{}
	title := ""
	switch tool {
	case "routes":
		title = "Routing table"
		commands = []commandSpec{{"IPv4 and IPv6 routes", "route.exe", []string{"print"}}}
	case "arp":
		title = "Neighbor cache"
		commands = []commandSpec{{"IPv4 ARP cache", "arp.exe", []string{"-a"}}, {"IPv6 neighbor cache", "netsh.exe", []string{"interface", "ipv6", "show", "neighbors"}}}
	case "connections":
		title = "Listening ports and active connections"
		commands = []commandSpec{{"TCP and UDP endpoints with process IDs", "netstat.exe", []string{"-a", "-n", "-o"}}}
	case "wifi":
		title = "Wi-Fi information"
		commands = []commandSpec{{"Wireless interfaces", "netsh.exe", []string{"wlan", "show", "interfaces"}}, {"Visible wireless networks", "netsh.exe", []string{"wlan", "show", "networks", "mode=bssid"}}}
	case "ipconfig":
		title = "IP configuration"
		commands = []commandSpec{{"Full adapter configuration", "ipconfig.exe", []string{"/all"}}}
	default:
		return model.Result{}, fmt.Errorf("unsupported local network snapshot: %s", tool)
	}
	sections := []map[string]any{}
	warnings := []string{}
	var outputs []string
	succeeded := false
	for _, item := range commands {
		if err := ctx.Err(); err != nil {
			return model.Result{}, err
		}
		commandCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		cmd := exec.CommandContext(commandCtx, filepath.Join(systemDir, item.exe), item.args...)
		ConfigureCommand(cmd)
		var buffer limitedBuffer
		buffer.limit = 2 * 1024 * 1024
		cmd.Stdout = &buffer
		cmd.Stderr = &buffer
		err := cmd.Run()
		contextErr := commandCtx.Err()
		cancel()
		output := strings.TrimSpace(decodeCommandOutput(buffer.bytes))
		section := map[string]any{"title": item.title, "command": item.exe + " " + strings.Join(item.args, " "), "output": output, "success": err == nil}
		if err != nil {
			message := err.Error()
			if contextErr != nil {
				message = contextErr.Error()
			}
			section["error"] = message
			warnings = append(warnings, item.title+": "+message)
		} else {
			succeeded = true
		}
		if buffer.truncated {
			section["truncated"] = true
			warnings = append(warnings, item.title+": output limited to 2 MiB")
		}
		sections = append(sections, section)
		outputs = append(outputs, item.title+"\n\n"+output)
	}
	data := map[string]any{"sections": sections, "output": strings.Join(outputs, "\n\n"), "warnings": warnings, "captured": time.Now().UTC().Format(time.RFC3339)}
	if tool == "connections" && len(sections) > 0 {
		data["connections"] = parseNetstat(fmt.Sprint(sections[0]["output"]))
	}
	if tool == "routes" && len(sections) > 0 {
		routes := parseWindowsRoutes(fmt.Sprint(sections[0]["output"]))
		if adapters, err := readWindowsAdapters(); err == nil {
			captureAdapterContext(adapters).annotateRoutes(routes)
		}
		data["routes"] = routes
		data["note"] = "The table lists active routes. Persistent routes and full interface details remain in the captured command output."
	}
	if tool == "arp" {
		neighbors, err := Neighbors(ctx)
		data["neighbors"] = neighbors
		data["note"] = neighborCacheNote
		if err != nil {
			warnings = append(warnings, "Structured neighbor information: "+err.Error())
			data["warnings"] = warnings
		} else {
			succeeded = true
		}
	}
	if tool == "ipconfig" {
		if system, err := System(ctx); err == nil {
			data["system"] = system
		} else {
			warnings = append(warnings, "Structured adapter information: "+err.Error())
			data["warnings"] = warnings
		}
	}
	if tool == "wifi" {
		data["note"] = "Windows may limit wireless details when the wireless adapter is unavailable, WLAN AutoConfig is stopped, or location access is disabled. No wireless passwords are collected."
	}
	summary := title + " captured"
	if !succeeded {
		summary = title + " could not be fully read; review command output and errors"
		return model.Result{Summary: summary, Data: data}, errors.New(summary)
	}
	return model.Result{Summary: summary, Data: data}, ctx.Err()
}

type limitedBuffer struct {
	bytes     []byte
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := b.limit - len(b.bytes)
	if remaining > n {
		remaining = n
	}
	if remaining > 0 {
		b.bytes = append(b.bytes, p[:remaining]...)
	}
	if remaining < n {
		b.truncated = true
	}
	return n, nil
}

func parseNetstat(output string) []map[string]any {
	rows := []map[string]any{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		proto := strings.ToUpper(fields[0])
		if proto != "TCP" && proto != "UDP" {
			continue
		}
		row := map[string]any{"protocol": proto, "local": fields[1], "remote": fields[2]}
		pidIndex := 3
		if proto == "TCP" {
			if len(fields) < 5 {
				continue
			}
			row["state"] = fields[3]
			pidIndex = 4
		} else {
			row["state"] = "bound"
		}
		if pid, err := strconv.Atoi(fields[pidIndex]); err == nil {
			row["pid"] = pid
		} else {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}
