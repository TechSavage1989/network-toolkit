package main

import "testing"

func TestTerminalDisplayNamesLegacyAcknowledgement129(t *testing.T) {
	rows := []map[string]any{
		{"id": "legacy", "type": "ssh", "host": "192.0.2.23", "port": float64(22), "name": "Custom West"},
		{"id": "generated", "type": "ssh", "host": "192.0.2.23", "port": float64(22), "name": "SSH 192.0.2.23:22"},
		{"id": "cleared", "type": "ssh", "host": "192.0.2.23", "port": float64(22), "name": "Retained identifier", "nameExplicit": false},
		{"id": "explicit-default", "type": "ssh", "host": "192.0.2.23", "port": float64(22), "name": "192.0.2.23", "nameExplicit": true},
	}
	state := map[string]any{"terminalProfiles": rows, "terminalRecent": rows}
	for _, key := range []string{"profiles", "recent"} {
		got := terminalDisplayNamesResponse(state)[key].([]map[string]any)
		for i, want := range []bool{true, false, false, true} {
			if got[i]["nameExplicit"] != want || got[i]["name"] != rows[i]["name"] || got[i]["nameRevision"] != uint64(0) {
				t.Fatalf("%s row %d legacy acknowledgement mismatch: %#v", key, i, got[i])
			}
		}
	}
	if _, mutated := rows[0]["nameExplicit"]; mutated {
		t.Fatal("response construction changed the stored legacy profile")
	}
}
