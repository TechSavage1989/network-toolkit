package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

const scanNetworksTimeout = 6 * time.Second

type scanNetworkPath struct {
	Gateway              string `json:"gateway"`
	OnLink               bool   `json:"onLink"`
	InterfaceIndex       int    `json:"interfaceIndex,omitempty"`
	InterfaceName        string `json:"interfaceName,omitempty"`
	InterfaceDescription string `json:"interfaceDescription,omitempty"`
	InterfaceAddress     string `json:"interfaceAddress,omitempty"`
	Metric               int    `json:"metric"`
}

type scanNetworkSuggestion struct {
	CIDR      string            `json:"cidr"`
	HostCount uint64            `json:"hostCount"`
	Paths     []scanNetworkPath `json:"paths"`
}

type scanNetworksResponse struct {
	Networks  []scanNetworkSuggestion `json:"networks"`
	Excluded  int                     `json:"excluded"`
	CheckedAt string                  `json:"checkedAt"`
	Note      string                  `json:"note"`
	Error     string                  `json:"error,omitempty"`
}

func (a *application) handleScanNetworks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodError(w)
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	stop := context.AfterFunc(a.ctx, cancel)
	defer stop()
	// This is a read-only snapshot. Route suggestions never resolve names,
	// expand addresses, open sockets, or change the selected source interface.
	result := readScanNetworks(ctx, a.getSystem(false), platform.Snapshot)
	jsonResponse(w, http.StatusOK, result)
}

func readScanNetworks(ctx context.Context, system map[string]any, snapshot func(context.Context, string) (model.Result, error)) scanNetworksResponse {
	response := scanNetworksResponse{
		Networks: []scanNetworkSuggestion{},
		Note:     "IPv4 destinations from this computer's current routing table. Reachability is checked only when you start a scan. Default routes, local interface networks and system-only destinations are omitted. Large networks must be narrowed to the existing scan limits. Selecting a target does not change the source interface.",
	}
	ctx, cancel := context.WithTimeout(ctx, scanNetworksTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		response.Error = "Could not read route suggestions: " + err.Error()
		response.CheckedAt = time.Now().UTC().Format(time.RFC3339)
		return response
	}
	result, err := snapshot(ctx, "routes")
	if err != nil {
		response.Error = "Could not fully read route suggestions: " + err.Error()
	}
	if ctx.Err() != nil {
		response.Error = "Could not fully read route suggestions: " + ctx.Err().Error()
	}
	data, _ := result.Data.(map[string]any)
	if rows, ok := scanNetworkRows(data["routes"]); ok {
		response.Networks, response.Excluded = scanNetworkSuggestions(rows, system)
	} else if response.Error == "" {
		response.Error = "The routing snapshot did not include structured route information."
	}
	response.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	return response
}

// No address expansion occurs here: even very large routes cost one prefix
// operation. Host counts follow the scan planner, including /31 and /32.
func scanNetworkSuggestions(routes []map[string]any, system map[string]any) ([]scanNetworkSuggestion, int) {
	context := scanRouteContext(system)
	networks := []scanNetworkSuggestion{}
	positions := map[netip.Prefix]int{}
	paths := map[netip.Prefix]map[scanNetworkPath]bool{}
	excluded := 0
	for _, route := range routes {
		prefix, ok := scanRoutePrefix(route)
		if !ok || scanRouteExcluded(prefix) || context.connected[prefix] || (prefix.Bits() == 32 && context.localHosts[prefix.Addr()]) {
			excluded++
			continue
		}
		path := context.path(route)
		position, exists := positions[prefix]
		if !exists {
			count := uint64(1) << (32 - prefix.Bits())
			if prefix.Bits() < 31 {
				count -= 2
			}
			position = len(networks)
			positions[prefix] = position
			paths[prefix] = map[scanNetworkPath]bool{}
			networks = append(networks, scanNetworkSuggestion{CIDR: prefix.String(), HostCount: count, Paths: []scanNetworkPath{}})
		}
		if !paths[prefix][path] {
			paths[prefix][path] = true
			networks[position].Paths = append(networks[position].Paths, path)
		}
	}
	for i := range networks {
		sort.SliceStable(networks[i].Paths, func(a, b int) bool {
			x, y := networks[i].Paths[a], networks[i].Paths[b]
			if x.Metric != y.Metric {
				return x.Metric < y.Metric
			}
			if x.InterfaceIndex != y.InterfaceIndex {
				return x.InterfaceIndex < y.InterfaceIndex
			}
			if x.InterfaceName != y.InterfaceName {
				return x.InterfaceName < y.InterfaceName
			}
			return x.Gateway < y.Gateway
		})
	}
	sort.SliceStable(networks, func(a, b int) bool {
		x, _ := netip.ParsePrefix(networks[a].CIDR)
		y, _ := netip.ParsePrefix(networks[b].CIDR)
		if x.Addr() != y.Addr() {
			return x.Addr().Less(y.Addr())
		}
		return x.Bits() < y.Bits()
	})
	return networks, excluded
}

func scanRoutePrefix(route map[string]any) (netip.Prefix, bool) {
	destination := scanNetworkText(route["destination"])
	if destination == "" {
		destination = scanNetworkText(route["cidr"])
	}
	if prefix, err := netip.ParsePrefix(destination); err == nil {
		return prefix.Masked(), prefix.Addr().Is4()
	}
	address, err := netip.ParseAddr(destination)
	if err != nil || !address.Is4() {
		return netip.Prefix{}, false
	}
	mask := scanNetworkText(route["mask"])
	maskAddress, err := netip.ParseAddr(mask)
	if err != nil || !maskAddress.Is4() {
		return netip.Prefix{}, false
	}
	bytes := maskAddress.As4()
	bits, size := net.IPMask(bytes[:]).Size()
	if size != 32 {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(address, bits).Masked(), true
}

func scanRouteExcluded(prefix netip.Prefix) bool {
	if !prefix.IsValid() || !prefix.Addr().Is4() || prefix.Bits() <= 1 {
		return true
	}
	address := prefix.Addr().As4()
	return address[0] == 0 || address[0] == 127 || address[0] >= 224 || (address[0] == 169 && address[1] == 254)
}

type scanRouteAdapter struct {
	index             int
	name, description string
}

type scanRouteLocalContext struct {
	connected  map[netip.Prefix]bool
	localHosts map[netip.Addr]bool
	byIndex    map[int]*scanRouteAdapter
	byName     map[string]*scanRouteAdapter
	byAddress  map[netip.Addr]*scanRouteAdapter
}

func scanRouteContext(system map[string]any) scanRouteLocalContext {
	context := scanRouteLocalContext{connected: map[netip.Prefix]bool{}, localHosts: map[netip.Addr]bool{}, byIndex: map[int]*scanRouteAdapter{}, byName: map[string]*scanRouteAdapter{}, byAddress: map[netip.Addr]*scanRouteAdapter{}}
	adapters, _ := scanNetworkRows(system["interfaces"])
	for _, adapter := range adapters {
		info := &scanRouteAdapter{index: scanNetworkInt(adapter["index"]), name: scanNetworkText(adapter["name"]), description: scanNetworkText(adapter["description"])}
		if info.index > 0 {
			if _, exists := context.byIndex[info.index]; exists {
				context.byIndex[info.index] = nil
			} else {
				context.byIndex[info.index] = info
			}
		}
		if info.name != "" {
			if _, exists := context.byName[info.name]; exists {
				context.byName[info.name] = nil
			} else {
				context.byName[info.name] = info
			}
		}
		up, _ := adapter["up"].(bool)
		loopback, _ := adapter["loopback"].(bool)
		states := map[netip.Addr]string{}
		details, _ := scanNetworkRows(adapter["addressDetails"])
		for _, detail := range details {
			if address, err := netip.ParseAddr(strings.SplitN(scanNetworkText(detail["address"]), "/", 2)[0]); err == nil {
				states[address] = strings.ToLower(scanNetworkText(detail["state"]))
			}
		}
		for _, raw := range scanNetworkAddresses(adapter["addresses"]) {
			address, err := netip.ParseAddr(strings.SplitN(raw, "/", 2)[0])
			if err != nil || !address.Is4() {
				continue
			}
			context.localHosts[address] = true
			if previous, exists := context.byAddress[address]; exists && previous != info {
				context.byAddress[address] = nil
			} else if !exists {
				context.byAddress[address] = info
			}
			prefix, err := netip.ParsePrefix(raw)
			if err != nil {
				continue
			}
			prefix = prefix.Masked()
			if prefix.Bits() <= 30 {
				bytes := prefix.Addr().As4()
				upper := binary.BigEndian.Uint32(bytes[:]) | uint32((uint64(1)<<(32-prefix.Bits()))-1)
				binary.BigEndian.PutUint32(bytes[:], upper)
				context.localHosts[netip.AddrFrom4(bytes)] = true
			}
			state := states[address]
			if up && !loopback && !address.IsLoopback() && state != "invalid" && state != "tentative" && state != "duplicate" {
				context.connected[prefix] = true
			}
		}
	}
	return context
}

func (context scanRouteLocalContext) path(route map[string]any) scanNetworkPath {
	path := scanNetworkPath{Gateway: scanNetworkText(route["gateway"]), InterfaceIndex: scanNetworkInt(route["interfaceIndex"]), InterfaceName: scanNetworkText(route["interfaceName"]), InterfaceDescription: scanNetworkText(route["interfaceDescription"]), Metric: scanNetworkInt(route["metric"])}
	path.OnLink, _ = route["onLink"].(bool)
	if gateway, err := netip.ParseAddr(path.Gateway); err == nil {
		if gateway.IsUnspecified() {
			path.Gateway, path.OnLink = "", true
		} else {
			path.Gateway = gateway.String()
		}
	}
	var info *scanRouteAdapter
	if path.InterfaceIndex == 0 {
		path.InterfaceIndex = scanNetworkInt(route["interface"])
	}
	if path.InterfaceIndex > 0 {
		info = context.byIndex[path.InterfaceIndex]
	}
	rawInterface := scanNetworkText(route["interface"])
	if address, err := netip.ParseAddr(rawInterface); err == nil && address.Is4() {
		path.InterfaceAddress = address.String()
		if info == nil && path.InterfaceIndex == 0 {
			info = context.byAddress[address]
		}
	} else if rawInterface != "" && scanNetworkInt(route["interface"]) == 0 {
		if path.InterfaceName == "" {
			path.InterfaceName = rawInterface
		}
	}
	if info == nil && path.InterfaceIndex == 0 && path.InterfaceName != "" {
		info = context.byName[path.InterfaceName]
	}
	// The system snapshot may be cached while routes carry freshly captured
	// labels. Do not attach an old adapter description after an index changed.
	if info != nil && path.InterfaceName != "" && info.name != "" && path.InterfaceName != info.name {
		info = nil
	}
	if info != nil {
		if path.InterfaceIndex == 0 {
			path.InterfaceIndex = info.index
		}
		if path.InterfaceName == "" {
			path.InterfaceName = info.name
		}
		if path.InterfaceDescription == "" {
			path.InterfaceDescription = info.description
		}
	}
	return path
}

func scanNetworkRows(value any) ([]map[string]any, bool) {
	switch rows := value.(type) {
	case []map[string]any:
		return rows, true
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, item := range rows {
			row, _ := item.(map[string]any)
			out = append(out, row)
		}
		return out, true
	}
	return nil, false
}

func scanNetworkAddresses(value any) []string {
	if addresses, ok := value.([]string); ok {
		return addresses
	}
	addresses, _ := value.([]any)
	out := make([]string, 0, len(addresses))
	for _, value := range addresses {
		if object, ok := value.(map[string]any); ok {
			value = object["address"]
		}
		if address := scanNetworkText(value); address != "" {
			out = append(out, address)
		}
	}
	return out
}

func scanNetworkText(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func scanNetworkInt(value any) int {
	switch number := value.(type) {
	case int:
		return number
	case float64:
		if number >= 0 && number <= math.MaxInt32 && number == math.Trunc(number) {
			return int(number)
		}
	case uint32:
		return int(number)
	case json.Number:
		parsed, _ := strconv.ParseInt(string(number), 10, 32)
		return int(parsed)
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(number), 10, 32)
		return int(parsed)
	}
	return 0
}
