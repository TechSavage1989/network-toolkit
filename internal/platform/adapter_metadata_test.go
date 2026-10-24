package platform

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCapturedAdapterMetadataPreservesDiagnosticIdentity(t *testing.T) {
	adapters := []map[string]any{{"index": 8, "ipv6Index": 12, "name": "Ethernet 5", "description": "Intel Ethernet Controller",
		"addresses": []string{"192.0.2.10/24", "192.0.2.10/32", "fe80::10/64"}}}
	context := captureAdapterContext(adapters)
	routes := parseWindowsRoutes("0.0.0.0 0.0.0.0 192.0.2.1 192.0.2.10 25\n12 25 ::/0 fe80::1\n13 25 ::/0 fe80::2\n")
	before := make([]map[string]any, len(routes))
	for i, row := range routes {
		before[i] = map[string]any{}
		for key, value := range row {
			before[i][key] = value
		}
	}
	context.annotateRoutes(routes)
	for i, row := range routes {
		for key, value := range before[i] {
			if !reflect.DeepEqual(row[key], value) {
				t.Fatalf("route %d changed original %s from %v to %v", i, key, value, row[key])
			}
		}
		if i < 2 && (row["interfaceName"] != "Ethernet 5" || row["interfaceDescription"] != "Intel Ethernet Controller") {
			t.Fatalf("route lost collection-time adapter metadata: %v", row)
		}
	}
	if routes[0]["interfaceIndex"] != 8 || routes[1]["interfaceIndex"] != 12 || !reflect.DeepEqual(routes[2], before[2]) {
		t.Fatalf("route index/address identity changed or unknown adapter guessed: %v", routes)
	}
	neighbors := []Neighbor{{Address: "192.0.2.1", MAC: "00:11:22:33:44:55", Interface: "Ethernet 5", InterfaceIndex: 8},
		{Address: "fe80::1%12", Interface: "Ethernet 5", InterfaceIndex: 12}, {Address: "192.0.3.1", Interface: "Interface 99", InterfaceIndex: 99}}
	context.annotateNeighbors(neighbors)
	if neighbors[0].InterfaceDescription != "Intel Ethernet Controller" || neighbors[1].InterfaceDescription != neighbors[0].InterfaceDescription || neighbors[2].InterfaceDescription != "" {
		t.Fatalf("neighbor description was lost or guessed: %+v", neighbors)
	}
	if neighbors[0].Interface != "Ethernet 5" || neighbors[0].InterfaceIndex != 8 || neighbors[1].Address != "fe80::1%12" || neighbors[1].InterfaceIndex != 12 {
		t.Fatalf("neighbor addressing identity changed: %+v", neighbors)
	}
	// Saved results retain the observed label even after the source metadata is
	// reused or renamed. Consumers have no reason to consult current adapters.
	adapters[0]["description"] = "A different adapter now uses this index"
	encoded, err := json.Marshal(neighbors)
	if err != nil {
		t.Fatal(err)
	}
	var restored []Neighbor
	if err = json.Unmarshal(encoded, &restored); err != nil || !reflect.DeepEqual(restored, neighbors) {
		t.Fatalf("recorded neighbor description did not survive export/import: %s, %v", encoded, err)
	}
}

func TestCapturedAdapterMetadataDoesNotGuessAmbiguousAssociations(t *testing.T) {
	context := captureAdapterContext([]map[string]any{
		{"index": 8, "ipv6Index": 12, "name": "Ethernet 5", "description": "First adapter", "addresses": []string{"192.0.2.10/24"}},
		{"index": 9, "ipv6Index": 12, "name": "Ethernet 6", "description": "Second adapter", "addresses": []string{"192.0.2.10/24", "192.0.3.10/24"}},
		{"index": 10, "name": "Ethernet 7", "description": "Third adapter", "addresses": []string{"192.0.2.10/24"}},
	})
	routes := []map[string]any{{"interface": "192.0.2.10"}, {"interface": 12}, {"interface": "Ethernet 5"}, {"interface": "192.0.3.10"}, {"interface": 8}}
	context.annotateRoutes(routes)
	for _, row := range routes[:3] {
		if len(row) != 1 {
			t.Fatalf("ambiguous address/index or untyped alias received guessed metadata: %v", row)
		}
	}
	if routes[3]["interfaceName"] != "Ethernet 6" || routes[4]["interfaceName"] != "Ethernet 5" {
		t.Fatalf("an unrelated ambiguous association erased unique observations: %v", routes)
	}
	neighbors := []Neighbor{{InterfaceIndex: 12}, {InterfaceIndex: 8}}
	context.annotateNeighbors(neighbors)
	if neighbors[0].InterfaceDescription != "" || neighbors[1].InterfaceDescription != "First adapter" {
		t.Fatalf("ambiguous native index was attributed: %+v", neighbors)
	}
	if names := context.interfaceNames(); names[12] != "" || names[8] != "Ethernet 5" {
		t.Fatalf("ambiguous native index supplied a friendly name: %v", names)
	}
}
