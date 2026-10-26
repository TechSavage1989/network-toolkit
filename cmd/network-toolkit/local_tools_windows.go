//go:build windows

package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"networktoolkit/internal/platform"
)

var localKernel = syscall.NewLazyDLL("kernel32.dll")
var localSystemDirectory, localSystemDirectoryError = localGetSystemDirectory()
var localIPHelper = syscall.NewLazyDLL(filepath.Join(localSystemDirectory, "iphlpapi.dll"))

func localGetSystemDirectory() (string, error) {
	buf := make([]uint16, 32768)
	n, _, e := localKernel.NewProc("GetSystemDirectoryW").Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 || n >= uintptr(len(buf)) {
		return "", fmt.Errorf("Windows system directory is unavailable: %w", e)
	}
	return syscall.UTF16ToString(buf[:n]), nil
}

func localToolsAvailable() bool { return localSystemDirectoryError == nil }
func localPathIsReparse(info os.FileInfo) bool {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return ok && data.FileAttributes&0x400 != 0
}

func localElevated() bool {
	token, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return false
	}
	defer token.Close()
	var elevated, size uint32
	if syscall.GetTokenInformation(token, 20, (*byte)(unsafe.Pointer(&elevated)), 4, &size) != nil {
		return false
	}
	return elevated != 0
}

func localHostsPath() (string, error) {
	if localSystemDirectoryError != nil {
		return "", localSystemDirectoryError
	}
	return filepath.Join(localSystemDirectory, "drivers", "etc", "hosts"), nil
}

func readLocalHosts(ctx context.Context) (localHostsSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return localHostsSnapshot{}, err
	}
	path, err := localHostsPath()
	if err != nil {
		return localHostsSnapshot{}, err
	}
	doc, err := readHostsDocument(path, localDecodeLegacy)
	if err != nil {
		return localHostsSnapshot{Available: true, Path: path, Warnings: []string{}, Note: err.Error()}, nil
	}
	warnings, validationErr := validateHostsText(doc.text)
	if warnings == nil {
		warnings = []string{}
	}
	if validationErr != nil {
		warnings = append(warnings, validationErr.Error())
	}
	s := localHostsSnapshot{Available: true, Elevated: localElevated(), Path: path, Exists: doc.exists,
		Content: normalizeHostsNewlines(doc.text), Revision: localHostsRevision(doc.exists, doc.bytes),
		Encoding: doc.encoding, Newline: doc.newline, Editable: true, Warnings: warnings}
	if !doc.exists {
		s.Note = "The system hosts file does not exist. Save creates it after administrator approval if required."
	}
	return s, nil
}

func localDecodeLegacy(data []byte) (string, string, error) {
	cp, _, _ := localKernel.NewProc("GetACP").Call()
	if len(data) == 0 {
		return "", fmt.Sprintf("Windows code page %d", cp), nil
	}
	convert := localKernel.NewProc("MultiByteToWideChar")
	n, _, err := convert.Call(cp, 0, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), 0, 0)
	if n == 0 || n > localHostsLimit {
		return "", "", fmt.Errorf("Decode the hosts file's Windows code page: %w", err)
	}
	units := make([]uint16, n)
	if n, _, err = convert.Call(cp, 0, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), uintptr(unsafe.Pointer(&units[0])), n); n == 0 {
		return "", "", fmt.Errorf("Decode the hosts file: %w", err)
	}
	text := string(utf16.Decode(units))
	encoding := fmt.Sprintf("Windows code page %d", cp)
	roundTrip, err := localEncodeLegacy(text, encoding)
	if err != nil || !bytes.Equal(roundTrip, data) {
		return "", "", errors.New("The hosts file's text encoding cannot be identified without changing its bytes.")
	}
	return text, encoding, nil
}

func localEncodeLegacy(text, encoding string) ([]byte, error) {
	if !strings.HasPrefix(encoding, "Windows code page ") {
		return nil, errors.New("Unsupported hosts text encoding.")
	}
	cp, err := strconv.ParseUint(strings.TrimPrefix(encoding, "Windows code page "), 10, 32)
	if err != nil {
		return nil, err
	}
	if text == "" {
		return []byte{}, nil
	}
	units := utf16.Encode([]rune(text))
	convert := localKernel.NewProc("WideCharToMultiByte")
	flags := uintptr(0x400) // WC_NO_BEST_FIT_CHARS: never replace edited text silently.
	var usedDefault uint32
	defaultPtr := uintptr(unsafe.Pointer(&usedDefault))
	if cp == 65001 {
		flags, defaultPtr = 0, 0
	}
	n, _, callErr := convert.Call(uintptr(cp), flags, uintptr(unsafe.Pointer(&units[0])), uintptr(len(units)), 0, 0, 0, defaultPtr)
	if n == 0 || n > localHostsLimit || usedDefault != 0 {
		return nil, fmt.Errorf("The edited text cannot be represented in the file's original encoding: %v", callErr)
	}
	out := make([]byte, n)
	n, _, callErr = convert.Call(uintptr(cp), flags, uintptr(unsafe.Pointer(&units[0])), uintptr(len(units)), uintptr(unsafe.Pointer(&out[0])), n, 0, defaultPtr)
	if n == 0 || usedDefault != 0 {
		return nil, fmt.Errorf("The edited text cannot be represented in the file's original encoding: %v", callErr)
	}
	return out[:n], nil
}

func localReplaceHosts(path, temporary, backup string) error {
	p, _ := syscall.UTF16PtrFromString(path)
	t, _ := syscall.UTF16PtrFromString(temporary)
	b, _ := syscall.UTF16PtrFromString(backup)
	// flags=0 is deliberate: ReplaceFile preserves the original DACL and other
	// metadata. Never opt into REPLACEFILE_IGNORE_ACL_ERRORS/MERGE_ERRORS.
	// https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-replacefilew
	ok, _, err := localKernel.NewProc("ReplaceFileW").Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(b)), 0, 0, 0)
	if ok == 0 {
		return err
	}
	return nil
}

func localCreateHosts(path, temporary string) error {
	p, _ := syscall.UTF16PtrFromString(path)
	t, _ := syscall.UTF16PtrFromString(temporary)
	// MOVEFILE_WRITE_THROUGH with no REPLACE_EXISTING publishes the already
	// flushed file atomically and fails if another process created hosts first.
	ok, _, err := localKernel.NewProc("MoveFileExW").Call(uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(p)), 8)
	if ok == 0 {
		return err
	}
	return nil
}

type localNativeSocket struct {
	Address unsafe.Pointer
	Length  int32
}
type localNativeAddress struct {
	Length, Flags uint32
	Next          *localNativeAddress
	Socket        localNativeSocket
}
type localNativeUnicast struct {
	Length, Flags                                   uint32
	Next                                            *localNativeUnicast
	Socket                                          localNativeSocket
	PrefixOrigin, SuffixOrigin, DADState            int32
	ValidLifetime, PreferredLifetime, LeaseLifetime uint32
	PrefixLength                                    uint8
}
type localNativeAdapter struct {
	Length, Index                                      uint32
	Next                                               *localNativeAdapter
	AdapterName                                        *byte
	Unicast                                            *localNativeUnicast
	Anycast, Multicast, DNS                            *localNativeAddress
	DNSSuffix, Description, FriendlyName               *uint16
	MAC                                                [8]byte
	MACLength, Flags, MTU, Type, OperStatus, IPv6Index uint32
	ZoneIndices                                        [16]uint32
	FirstPrefix                                        unsafe.Pointer
	TransmitLinkSpeed, ReceiveLinkSpeed                uint64
	WINSServer, Gateway                                *localNativeAddress
	IPv4Metric, IPv6Metric                             uint32
	LUID                                               uint64
	DHCPv4Server                                       localNativeSocket
	CompartmentID                                      uint32
	NetworkGUID                                        [16]byte
	ConnectionType, TunnelType                         uint32
}

// SOCKADDR_INET is a 28-byte, 4-byte aligned union. The explicit structs keep
// the documented Windows ABI without a cgo runtime or external DLL.
type localSockaddrInet struct {
	Family, Port uint16
	Data         [24]byte
}
type localIPPrefix struct {
	Address localSockaddrInet
	Length  uint8
	Padding [3]byte
}
type localRouteRow struct {
	LUID                                               uint64
	Index                                              uint32
	Destination                                        localIPPrefix
	NextHop                                            localSockaddrInet
	SitePrefixLength                                   uint8
	Padding                                            [3]byte
	ValidLifetime, PreferredLifetime, Metric, Protocol uint32
	Loopback, AutoconfigureAddress, Publish, Immortal  uint8
	Age, Origin                                        uint32
}
type localRouteTable struct {
	Count uint32
	Rows  [1]localRouteRow
}
type localDefaultRoute struct {
	Gateway                  string
	Metric, Protocol, Origin uint32
}

func localDefaultRoutes() (map[uint32][]localDefaultRoute, error) {
	var table *localRouteTable
	status, _, _ := localIPHelper.NewProc("GetIpForwardTable2").Call(2, uintptr(unsafe.Pointer(&table)))
	if status != 0 {
		return nil, syscall.Errno(status)
	}
	if table == nil {
		return nil, errors.New("Windows returned an empty route table pointer.")
	}
	defer localIPHelper.NewProc("FreeMibTable").Call(uintptr(unsafe.Pointer(table)))
	if table.Count > 65536 {
		return nil, errors.New("The IPv4 route table exceeds the supported size.")
	}
	result := map[uint32][]localDefaultRoute{}
	for _, row := range unsafe.Slice(&table.Rows[0], table.Count) {
		if row.Destination.Length != 0 || row.Destination.Address.Family != 2 || row.NextHop.Family != 2 {
			continue
		}
		ip := netip.AddrFrom4([4]byte{row.NextHop.Data[0], row.NextHop.Data[1], row.NextHop.Data[2], row.NextHop.Data[3]})
		result[row.Index] = append(result[row.Index], localDefaultRoute{ip.String(), row.Metric, row.Protocol, row.Origin})
	}
	for _, routes := range result {
		sort.Slice(routes, func(i, j int) bool {
			if routes[i].Gateway == routes[j].Gateway {
				return routes[i].Metric < routes[j].Metric
			}
			return routes[i].Gateway < routes[j].Gateway
		})
	}
	return result, nil
}

func localRegistryStrings(key syscall.Handle, name string) ([]string, error) {
	n, _ := syscall.UTF16PtrFromString(name)
	var kind, size uint32
	err := syscall.RegQueryValueEx(key, n, nil, &kind, nil, &size)
	if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	if size > 64<<10 || size%2 != 0 || (kind != syscall.REG_SZ && kind != syscall.REG_EXPAND_SZ && kind != syscall.REG_MULTI_SZ) {
		return nil, errors.New("Windows returned an unsupported adapter registry value.")
	}
	if size == 0 {
		return []string{}, nil
	}
	data := make([]byte, size)
	if err = syscall.RegQueryValueEx(key, n, nil, &kind, &data[0], &size); err != nil {
		return nil, err
	}
	units := make([]uint16, size/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(data[i*2:])
	}
	text := strings.TrimRight(string(utf16.Decode(units)), "\x00")
	if text == "" {
		return []string{}, nil
	}
	return strings.Split(text, "\x00"), nil
}

type localAdapterRegistry struct{ IP, Mask, Gateway, GatewayMetric, NameServer, DHCPServer []string }

func localReadAdapterRegistry(id string) (localAdapterRegistry, error) {
	var result localAdapterRegistry
	if !validAdapterID(id) {
		return result, errors.New("The adapter does not have a Windows GUID.")
	}
	path, _ := syscall.UTF16PtrFromString(`SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces\` + id)
	var key syscall.Handle
	if err := syscall.RegOpenKeyEx(syscall.HKEY_LOCAL_MACHINE, path, 0, syscall.KEY_READ, &key); err != nil {
		return result, err
	}
	defer syscall.RegCloseKey(key)
	for _, item := range []struct {
		name string
		out  *[]string
	}{{"IPAddress", &result.IP}, {"SubnetMask", &result.Mask}, {"DefaultGateway", &result.Gateway}, {"DefaultGatewayMetric", &result.GatewayMetric}, {"NameServer", &result.NameServer}, {"DhcpServer", &result.DHCPServer}} {
		v, err := localRegistryStrings(key, item.name)
		if err != nil {
			return result, fmt.Errorf("Read %s: %w", item.name, err)
		}
		*item.out = v
	}
	return result, nil
}

func readLocalIP(ctx context.Context) (snapshot localIPSnapshot, resultErr error) {
	snapshot = localIPSnapshot{Available: true, Elevated: localElevated(), Adapters: []localIPAdapter{}}
	defer func() {
		if resultErr == nil {
			stampLocalIPSnapshot(&snapshot, time.Now())
		}
	}()
	if localSystemDirectoryError != nil {
		return snapshot, localSystemDirectoryError
	}
	if err := ctx.Err(); err != nil {
		return snapshot, err
	}
	size := uint32(15 << 10)
	var buf []byte
	complete := false
	for attempt := 0; attempt < 4; attempt++ {
		if size < 1 || size > 16<<20 {
			return snapshot, errors.New("Adapter information exceeds the supported size.")
		}
		buf = make([]byte, size)
		status, _, _ := localIPHelper.NewProc("GetAdaptersAddresses").Call(0, 0x02|0x04|0x80|0x100, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
		if status == 232 {
			return snapshot, nil
		}
		if status == 111 {
			continue
		}
		if status != 0 {
			return snapshot, fmt.Errorf("Read Windows network adapters: %w", syscall.Errno(status))
		}
		complete = true
		break
	}
	if !complete || int(size) > len(buf) {
		return snapshot, errors.New("Adapter information changed too quickly. Reload adapters.")
	}
	routes, routeErr := localDefaultRoutes()
	start := uintptr(unsafe.Pointer(&buf[0]))
	end := start + uintptr(len(buf))
	inBuffer := func(p unsafe.Pointer, n uintptr) bool {
		addr := uintptr(p)
		return addr >= start && addr <= end && n <= end-addr
	}
	readUTF16 := func(p *uint16) string {
		if p == nil {
			return ""
		}
		units := []uint16{}
		for i := 0; i < 32768; i++ {
			pos := unsafe.Add(unsafe.Pointer(p), i*2)
			if !inBuffer(pos, 2) {
				break
			}
			v := *(*uint16)(pos)
			if v == 0 {
				break
			}
			units = append(units, v)
		}
		return string(utf16.Decode(units))
	}
	readID := func(p *byte) string {
		out := []byte{}
		for i := 0; p != nil && i < 128; i++ {
			pos := unsafe.Add(unsafe.Pointer(p), i)
			if !inBuffer(pos, 1) {
				break
			}
			v := *(*byte)(pos)
			if v == 0 {
				break
			}
			out = append(out, v)
		}
		return string(out)
	}
	readIPv4 := func(socket localNativeSocket) string {
		if socket.Length < 8 || !inBuffer(socket.Address, 8) {
			return ""
		}
		raw := unsafe.Slice((*byte)(socket.Address), 8)
		if binary.LittleEndian.Uint16(raw) != 2 {
			return ""
		}
		return netip.AddrFrom4([4]byte{raw[4], raw[5], raw[6], raw[7]}).String()
	}
	readAddressList := func(first *localNativeAddress) []string {
		out := []string{}
		seen := map[string]bool{}
		for p, count := first, 0; p != nil && count < 4096; p, count = p.Next, count+1 {
			if !inBuffer(unsafe.Pointer(p), unsafe.Sizeof(*p)) {
				break
			}
			address := readIPv4(p.Socket)
			if address != "" && address != "0.0.0.0" && !seen[address] {
				out = append(out, address)
				seen[address] = true
			}
		}
		return out
	}
	for p, count := (*localNativeAdapter)(unsafe.Pointer(&buf[0])), 0; p != nil && count < 4096; p, count = p.Next, count+1 {
		if !inBuffer(unsafe.Pointer(p), unsafe.Sizeof(*p)) || p.Length < uint32(unsafe.Sizeof(*p)) {
			return snapshot, errors.New("Windows returned an unsupported adapter information layout.")
		}
		a := localIPAdapter{ID: readID(p.AdapterName), Index: p.Index, Name: readUTF16(p.FriendlyName), Description: readUTF16(p.Description), DHCP: p.Flags&4 != 0, Addresses: []localIPAddress{}, Gateways: readAddressList(p.Gateway), DNSServers: readAddressList(p.DNS), Editable: true, AddressEditable: true}
		a.effectiveDNS = append([]string(nil), a.DNSServers...)
		a.Type = map[uint32]string{6: "Ethernet", 71: "Wi-Fi", 24: "Loopback", 23: "PPP", 131: "Tunnel"}[p.Type]
		if a.Type == "" {
			a.Type = "Other"
		}
		a.Status = map[uint32]string{1: "Up", 2: "Down", 3: "Testing", 4: "Unknown", 5: "Dormant", 6: "Not present", 7: "Lower layer down"}[p.OperStatus]
		if a.Status == "" {
			a.Status = "Unknown"
		}
		if p.MACLength <= 8 {
			a.MAC = strings.ToUpper(net.HardwareAddr(p.MAC[:p.MACLength]).String())
		}
		a.dnsRegistration = "none"
		if p.Flags&1 != 0 {
			a.dnsRegistration = "primary"
			if p.Flags&2 != 0 {
				a.dnsRegistration = "both"
			}
		}
		for u, n := p.Unicast, 0; u != nil && n < 4096; u, n = u.Next, n+1 {
			if !inBuffer(unsafe.Pointer(u), unsafe.Sizeof(*u)) {
				break
			}
			ip := readIPv4(u.Socket)
			if ip == "" || ip == "0.0.0.0" {
				continue
			}
			origin := map[int32]string{1: "manual", 2: "well-known", 3: "dhcp", 4: "router advertisement"}[u.PrefixOrigin]
			if origin == "" {
				origin = "other"
			}
			a.Addresses = append(a.Addresses, localIPAddress{ip, int(u.PrefixLength), origin})
		}
		registry, regErr := localReadAdapterRegistry(a.ID)
		if len(registry.DHCPServer) > 0 {
			if ip, e := netip.ParseAddr(strings.TrimSpace(registry.DHCPServer[0])); e == nil && ip.Is4() && !ip.IsUnspecified() {
				a.DHCPServer = ip.String()
			}
		}
		if regErr != nil {
			a.Editable = false
			a.AddressEditable = false
			a.Note = "The adapter's stored configuration could not be read: " + regErr.Error()
		} else {
			// Preserve an explicitly nonempty value, even if it contains only
			// separators. Treating that as DHCP would erase an explicit clear.
			nameServer := strings.Join(registry.NameServer, " ")
			a.DNSAutomatic = nameServer == ""
			if !a.DNSAutomatic {
				a.DNSServers = []string{}
				for _, value := range strings.FieldsFunc(nameServer, func(c rune) bool { return c == ',' || c == ';' || c == ' ' || c == '\t' || c == '\r' || c == '\n' }) {
					ip, e := netip.ParseAddr(value)
					if e != nil || !ip.Is4() {
						a.Editable = false
						a.Note = "This adapter has an IPv4 DNS configuration that the editor cannot preserve safely."
						continue
					}
					a.DNSServers = append(a.DNSServers, ip.String())
				}
			}
			stored := []localIPAddress{}
			for i, value := range registry.IP {
				if value == "" || value == "0.0.0.0" {
					continue
				}
				ip, e := netip.ParseAddr(value)
				if e != nil || !ip.Is4() || i >= len(registry.Mask) {
					a.AddressEditable = false
					a.AddressNote = "The stored IPv4 address configuration has unsupported entries."
					continue
				}
				mask := net.ParseIP(registry.Mask[i]).To4()
				if mask == nil {
					a.AddressEditable = false
					a.AddressNote = "The stored IPv4 subnet mask could not be read."
					continue
				}
				prefix, bits := net.IPMask(mask).Size()
				if bits != 32 {
					a.AddressEditable = false
					a.AddressNote = "The stored IPv4 subnet mask is not contiguous."
					continue
				}
				stored = append(stored, localIPAddress{ip.String(), prefix, "manual"})
			}
			if !a.DHCP && len(stored) > 0 {
				a.Addresses = stored
			}
			if a.DHCP && len(stored) > 0 {
				a.AddressEditable = false
				a.AddressNote = "DHCP with additional stored static addresses is preserved. Use Windows advanced adapter settings to edit addresses."
			}
			if !a.DHCP && len(registry.Gateway) > 0 {
				a.Gateways = []string{}
				for _, v := range registry.Gateway {
					ip, e := netip.ParseAddr(v)
					if e == nil && ip.Is4() && !ip.IsUnspecified() {
						a.Gateways = append(a.Gateways, ip.String())
					}
				}
			}
			a.configuration = localRevision(struct {
				Registry localAdapterRegistry
				Routes   []localDefaultRoute
			}{registry, routes[a.Index]})
		}
		if len(a.Addresses) > 1 || len(a.Gateways) > 1 || len(routes[a.Index]) > 1 {
			a.AddressEditable = false
			a.AddressNote = "Additional IPv4 addresses or default routes are preserved. Use Windows advanced adapter settings to edit addresses."
		}
		if routeErr != nil {
			a.AddressEditable = false
			a.AddressNote = "The route table could not be read safely. DNS can still be changed independently."
		}
		if len(routes[a.Index]) == 1 {
			metric := routes[a.Index][0].Metric
			a.gatewayMetric = &metric
		}
		if a.gatewayMetric == nil && len(registry.GatewayMetric) == 1 {
			if metric, e := strconv.ParseUint(registry.GatewayMetric[0], 10, 32); e == nil {
				v := uint32(metric)
				a.gatewayMetric = &v
			}
		}
		if !a.DHCP && len(a.Gateways) > 0 && a.gatewayMetric == nil {
			a.AddressEditable = false
			a.AddressNote = "The existing default gateway metric could not be read. DNS can still be changed independently."
		}
		if !localIPv4Configurable(a.ID, p.Index, p.Flags, p.Type, p.OperStatus, p.TunnelType) {
			a.Editable = false
			a.AddressEditable = false
			a.Note = "IPv4 configuration is unavailable for this interface."
		} else if a.Editable && p.Type == 131 {
			a.AddressEditable = false
			a.AddressNote = "This tunnel exposes editable IPv4 DNS settings. Its tunnel-managed addresses and routes are preserved."
		}
		sort.Slice(a.Addresses, func(i, j int) bool { return a.Addresses[i].Address < a.Addresses[j].Address })
		snapshot.Adapters = append(snapshot.Adapters, a)
	}
	runtime.KeepAlive(buf)
	snapshot.Adapters = finalizeLocalAdapters(snapshot.Adapters)
	return snapshot, ctx.Err()
}

func localRunNetsh(ctx context.Context, step localNetshStep) (string, error) {
	if localSystemDirectoryError != nil {
		return "", localSystemDirectoryError
	}
	commandCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(commandCtx, filepath.Join(localSystemDirectory, "netsh.exe"), step.Arguments...)
	platform.ConfigureCommand(cmd)
	var output localOutputBuffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	text := string(output.data)
	if decoded, _, decodeErr := localDecodeLegacy(output.data); decodeErr == nil {
		text = decoded
	}
	if commandCtx.Err() != nil {
		err = commandCtx.Err()
	}
	return strings.TrimSpace(text), err
}

func localPerformHosts(ctx context.Context, request localHostsRequest) localApplyResult {
	path, err := localHostsPath()
	if err != nil {
		return localApplyResult{Error: err.Error(), ErrorCode: localErrorCode(err)}
	}
	backup, err := saveHostsFile(ctx, path, request, localHostsFileOps{decode: localDecodeLegacy, encode: localEncodeLegacy, replace: localReplaceHosts, create: localCreateHosts})
	if backup != "" {
		if _, statErr := os.Stat(backup); statErr != nil {
			backup = ""
		}
	}
	result := localApplyResult{BackupPath: backup}
	if err != nil {
		result.Error = err.Error()
		result.ErrorCode = localErrorCode(err)
		result.Partial = backup != ""
	} else {
		result.Message = "The hosts file was saved."
	}
	refreshCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	fresh, refreshErr := readLocalHosts(refreshCtx)
	if refreshErr == nil {
		result.Result.Hosts = &fresh
		if err != nil && fresh.Editable && fresh.Revision != request.Revision {
			if errors.Is(err, errLocalConfigConflict) || localErrorCode(err) == "permission" {
				result.Error, result.ErrorCode, result.Partial = errLocalConfigConflict.Error(), "conflict", false
			} else {
				result.Partial = true
			}
		}
	}
	if err == nil && (refreshErr != nil || !fresh.Editable || !fresh.Exists || fresh.Content != normalizeHostsNewlines(request.Content)) {
		result.Error = "The save completed, but the hosts file could not be verified. Reload it before making another change."
		result.ErrorCode = "failed"
		result.Partial = true
	}
	if err == nil && result.Error == "" {
		ok, _, _ := syscall.NewLazyDLL(filepath.Join(localSystemDirectory, "dnsapi.dll")).NewProc("DnsFlushResolverCache").Call()
		if ok == 0 {
			result.Message += " Windows did not confirm a DNS cache refresh; existing application caches may take time to update."
		}
	}
	return result
}

func localPerformTools(ctx context.Context, request localToolsRequest) localApplyResult {
	if err := validateLocalToolsRequest(request); err != nil {
		return localApplyResult{Error: err.Error(), ErrorCode: "validation"}
	}
	if err := ctx.Err(); err != nil {
		return localApplyResult{Error: err.Error(), ErrorCode: localErrorCode(err)}
	}
	if request.Kind == "hosts" {
		return localPerformHosts(ctx, *request.Hosts)
	}
	return executeLocalIP(ctx, *request.IP, localIPApplyOps{read: readLocalIP, run: localRunNetsh})
}

func applyLocalTools(ctx context.Context, request localToolsRequest) localApplyResult {
	if err := validateLocalToolsRequest(request); err != nil {
		return localApplyResult{Error: err.Error(), ErrorCode: "validation"}
	}
	if localElevated() {
		return localPerformTools(ctx, request)
	}
	// Avoid a UAC prompt for a no-op. Hosts writes first try the caller's actual
	// permissions, so a user-granted hosts ACL does not trigger needless UAC.
	if request.Kind == "hosts" {
		result := localPerformHosts(ctx, *request.Hosts)
		if result.ErrorCode != "permission" || result.Partial {
			return result
		}
	} else {
		snapshot, err := readLocalIP(ctx)
		if err != nil {
			return localApplyResult{Error: err.Error(), ErrorCode: localErrorCode(err)}
		}
		_, steps, err := planLocalIP(snapshot, *request.IP)
		if err != nil {
			return localApplyResult{Error: err.Error(), ErrorCode: localErrorCode(err), Result: localToolsResult{IP: &snapshot}}
		}
		if len(steps) == 0 {
			return localApplyResult{Message: "No changes were needed.", Result: localToolsResult{IP: &snapshot}}
		}
	}
	return localElevatedApply(ctx, request)
}
