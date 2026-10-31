package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTerminalNameCreationMarkerDoesNotPersist130(t *testing.T) {
	state := map[string]any{}
	if err := json.Unmarshal([]byte(`{"terminalProfiles":[{"id":"new-profile","name":"Automatic","nameExplicit":false,"nameRevision":0,"_nameNew":true,"_namePending":true,"type":"telnet","host":"192.0.2.18","port":23}],"terminalRecent":[{"id":"new-recent","name":"Automatic","nameExplicit":false,"nameRevision":0,"_nameNew":true,"type":"telnet","host":"192.0.2.18","port":23}]}`), &state); err != nil {
		t.Fatal(err)
	}
	sanitizeTerminalSettings(state)
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "_name") {
		t.Fatalf("Transient creation metadata persisted: %s", data)
	}
	if err := reconcileTerminalDisplayNames(state, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	for _, collection := range []string{"terminalProfiles", "terminalRecent"} {
		rows := terminalNameRows(state, collection)
		if len(rows) != 1 {
			t.Fatalf("Missing %s", collection)
		}
		revision, _ := terminalNameRevision(rows[0]["nameRevision"])
		if revision == 0 {
			t.Fatalf("Missing authoritative revision for %s", collection)
		}
	}
}
