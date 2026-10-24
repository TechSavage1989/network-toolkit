package platform

import (
	"net/netip"
	"strings"
)

// Adapter labels are captured with a diagnostic result. A saved/imported result
// must not resolve an old interface index against the computer viewing it.
type capturedAdapterInfo struct {
	index             int
	name, description string
}

type capturedAdapterContext struct {
	byIndex map[int]*capturedAdapterInfo
	byIPv4  map[string]*capturedAdapterInfo
}

func captureAdapterContext(rows []map[string]any) capturedAdapterContext {
	context := capturedAdapterContext{byIndex: map[int]*capturedAdapterInfo{}, byIPv4: map[string]*capturedAdapterInfo{}}
	for _, row := range rows {
		index, _ := row["index"].(int)
		name, _ := row["name"].(string)
		description, _ := row["description"].(string)
		info := &capturedAdapterInfo{index: index, name: name, description: description}
		ipv6Index, _ := row["ipv6Index"].(int)
		for _, value := range []int{index, ipv6Index} {
			if value <= 0 {
				continue
			}
			previous, exists := context.byIndex[value]
			if !exists {
				context.byIndex[value] = info
			} else if previous != info {
				// Even equal labels cannot establish that two rows are one adapter.
				context.byIndex[value] = nil
			}
		}
		addresses, _ := row["addresses"].([]string)
		for _, raw := range addresses {
			address, err := netip.ParseAddr(strings.SplitN(raw, "/", 2)[0])
			if err != nil || !address.Is4() || address.IsUnspecified() {
				continue
			}
			key := address.String()
			previous, exists := context.byIPv4[key]
			if !exists {
				context.byIPv4[key] = info
			} else if previous != info {
				// Duplicate local addresses occur across virtual/network interfaces.
				context.byIPv4[key] = nil
			}
		}
	}
	return context
}

func (context capturedAdapterContext) interfaceNames() map[int]string {
	names := map[int]string{}
	for index, info := range context.byIndex {
		if info != nil && info.name != "" {
			names[index] = info.name
		}
	}
	return names
}

func (context capturedAdapterContext) annotateNeighbors(rows []Neighbor) {
	for i := range rows {
		if info := context.byIndex[rows[i].InterfaceIndex]; info != nil {
			rows[i].InterfaceDescription = info.description
		}
	}
}

func (context capturedAdapterContext) annotateRoutes(rows []map[string]any) {
	for _, row := range rows {
		var info *capturedAdapterInfo
		index := 0
		switch value := row["interface"].(type) {
		case int:
			info, index = context.byIndex[value], value
		case string:
			// Windows route print uses the local IPv4 address, not the alias.
			address, err := netip.ParseAddr(value)
			if err == nil && address.Is4() {
				info = context.byIPv4[address.String()]
				if info != nil {
					index = info.index
				}
			}
		}
		if info == nil {
			continue
		}
		if info.name != "" {
			row["interfaceName"] = info.name
		}
		if info.description != "" {
			row["interfaceDescription"] = info.description
		}
		if index > 0 {
			row["interfaceIndex"] = index
		}
	}
}
