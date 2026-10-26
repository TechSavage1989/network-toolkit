package main

import (
	"regexp"
	"strings"

	"networktoolkit/internal/capture"
)

// Packet Monitor reports driver components as well as network interfaces.
// A component is selectable only when it can be associated with one real,
// configurable Windows interface. Never guess between NICs sharing a driver
// or a MAC address: capturing the wrong device is worse than omitting a source.
type packetSystemAdapter struct {
	ID, Name, Description, MAC, Driver string
}

var packetAdapterMAC = regexp.MustCompile(`(?i)(?:^|\s)([0-9a-f]{2}(?:[:-][0-9a-f]{2}){5})(?:\s|$)`)

func packetAdapterKey(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
func packetMACKey(s string) string     { return strings.ReplaceAll(packetAdapterKey(s), "-", ":") }

func packetMatchAdapter(name string, adapters []packetSystemAdapter) (packetSystemAdapter, bool) {
	name = strings.TrimSpace(name)
	mac, label := "", name
	if match := packetAdapterMAC.FindStringSubmatchIndex(name); match != nil {
		mac = packetMACKey(name[match[2]:match[3]])
		label = strings.TrimSpace(name[match[1]:])
	}
	matches := []packetSystemAdapter{}
	for _, adapter := range adapters {
		if mac != "" {
			if packetMACKey(adapter.MAC) == mac {
				matches = append(matches, adapter)
			}
			continue
		}
		key := packetAdapterKey(label)
		if key != "" && (key == packetAdapterKey(adapter.ID) || key == packetAdapterKey(adapter.Name) || key == packetAdapterKey(adapter.Description) || key == packetAdapterKey(adapter.Driver)) {
			matches = append(matches, adapter)
		}
	}
	if len(matches) == 1 {
		return matches[0], true
	}
	// A shared MAC can still be disambiguated by an exact interface identity.
	if mac != "" && len(matches) > 1 {
		var exact []packetSystemAdapter
		for _, adapter := range matches {
			key := packetAdapterKey(label)
			if key == packetAdapterKey(adapter.Name) || key == packetAdapterKey(adapter.Description) {
				exact = append(exact, adapter)
			}
		}
		if len(exact) == 1 {
			return exact[0], true
		}
	}
	return packetSystemAdapter{}, false
}

func packetAdapterView(source capture.Interface, adapter packetSystemAdapter) capture.Interface {
	source.Name = adapter.Name
	if source.Name == "" {
		source.Name = adapter.Description
	}
	source.Description = strings.TrimSpace(strings.Join([]string{adapter.Description, adapter.MAC}, " "))
	return source
}

func packetConfigurableAdapters(snapshot localIPSnapshot) []packetSystemAdapter {
	adapters := []packetSystemAdapter{}
	for _, adapter := range snapshot.Adapters {
		if !adapter.Editable || !validAdapterID(adapter.ID) || adapter.Index == 0 || adapter.Status == "Not present" || adapter.Type == "Loopback" || strings.TrimSpace(adapter.Name) == "" {
			continue
		}
		adapters = append(adapters, packetSystemAdapter{ID: adapter.ID, Name: adapter.Name, Description: adapter.Description, MAC: adapter.MAC})
	}
	return adapters
}
