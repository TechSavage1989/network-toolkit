package main

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"os"
	"reflect"
	"testing"
)

func TestSessionHistoryAndDeletionPreferencesPersistWithoutSecrets(t *testing.T) {
	a := testApplication(t)
	body := `{"terminalRecent":[{"id":"r1","name":"Lab SSH","type":"ssh","host":"192.0.2.20","port":2222,"username":"operator","credentialId":"vault-reference","lastUsed":1792660000000,"password":"secret-canary","privateKey":"secret-canary","output":"secret-canary","sessionId":"secret-canary"},{"id":"r1","name":"duplicate"},null],"terminalSidebarCollapsed":true,"terminalSidebarTab":"macros","terminalComposerCollapsed":true,"terminalMacroConfirmation":"multiple","terminalRecentCollapsed":true,"terminalSavedCollapsed":false,"terminalSessionConfirmDelete":false}`
	response := requestApp(a, http.MethodPut, "/api/state", body, true)
	if response.Code != http.StatusOK {
		t.Fatalf("save history: %d %s", response.Code, response.Body.String())
	}
	disk, err := os.ReadFile(a.dataPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(disk, []byte("secret-canary")) {
		t.Fatal("session history persisted authentication or transcript data")
	}
	var state map[string]any
	if err := json.Unmarshal(disk, &state); err != nil {
		t.Fatal(err)
	}
	list := state["terminalRecent"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["credentialId"] != "vault-reference" || state["terminalSessionConfirmDelete"] != false || state["terminalSidebarCollapsed"] != true || state["terminalSidebarTab"] != "macros" || state["terminalComposerCollapsed"] != true || state["terminalMacroConfirmation"] != "multiple" {
		t.Fatalf("history/preferences lost: %#v", state)
	}
	first := terminalWorkspaceDisplay(state)
	if first["terminalSidebarTab"] != "macros" || first["terminalComposerCollapsed"] != true || first["terminalMacroConfirmation"] != "multiple" || !reflect.DeepEqual(first, terminalWorkspaceDisplay(first)) || len(first["terminalRecent"].([]map[string]any)) != 1 {
		t.Fatal("linked workspace history is not stable across repeated sanitization")
	}
	if len(a.terminals.List()) != 0 {
		t.Fatal("saving recent connections opened a transport")
	}
}

func TestSessionHistoryBoundsAndInvalidPreferences(t *testing.T) {
	items := make([]any, 60)
	for i := range items {
		items[i] = map[string]any{"id": string(rune('A' + i)), "name": "Serial", "type": "serial", "serialPort": "COM4", "lastUsed": -1.0, "password": "secret-canary"}
	}
	state := map[string]any{"terminalRecent": items, "terminalSessionConfirmDelete": "false", "terminalSidebarCollapsed": map[string]any{"password": "secret-canary"}, "terminalSidebarTab": "unknown", "terminalComposerCollapsed": "false", "terminalMacroConfirmation": map[string]any{"password": "secret-canary"}}
	sanitizeTerminalSettings(state)
	if _, exists := state["terminalSessionConfirmDelete"]; exists {
		t.Fatal("invalid imported confirmation setting disabled safe default")
	}
	if _, exists := state["terminalSidebarCollapsed"]; exists {
		t.Fatal("invalid sidebar setting was retained")
	}
	if state["terminalMacroConfirmation"] != "always" {
		t.Fatal("invalid macro confirmation policy did not use Always")
	}
	if state["terminalSidebarTab"] != "saved" {
		t.Fatal("invalid sidebar tab did not use Saved")
	}
	if _, exists := state["terminalComposerCollapsed"]; exists {
		t.Fatal("invalid composer setting was retained")
	}
	list := state["terminalRecent"].([]map[string]any)
	if len(list) != 50 || list[0]["lastUsed"] != float64(0) || list[0]["password"] != nil {
		t.Fatalf("invalid history was not bounded/sanitized: %#v", list)
	}
}

func TestTerminalSidebarWidthRejectsMalformedPreferencesAndKeepsBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input any
		want  float64
	}{
		{"minimum", float64(260), 260},
		{"default", float64(300), 300},
		{"custom", float64(372), 372},
		{"maximum", float64(480), 480},
		{"below", float64(259), 300},
		{"above", float64(481), 300},
		{"fraction", 372.5, 300},
		{"string", "372", 300},
		{"boolean", false, 300},
		{"object", map[string]any{"password": "secret-canary"}, 300},
		{"nil", nil, 300},
		{"not a number", math.NaN(), 300},
		{"positive infinity", math.Inf(1), 300},
		{"negative infinity", math.Inf(-1), 300},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := map[string]any{"terminalSidebarWidth": tc.input}
			sanitizeTerminalSettings(state)
			if state["terminalSidebarWidth"] != tc.want {
				t.Fatalf("width=%#v, want %v", state["terminalSidebarWidth"], tc.want)
			}
			transferred := terminalWorkspaceDisplay(state)
			if transferred["terminalSidebarWidth"] != tc.want || !reflect.DeepEqual(transferred, terminalWorkspaceDisplay(transferred)) {
				t.Fatalf("width changed during repeated transfer: %#v", transferred)
			}
		})
	}
	empty := map[string]any{}
	sanitizeTerminalSettings(empty)
	if _, exists := empty["terminalSidebarWidth"]; exists {
		t.Fatal("missing width should retain the client default")
	}
}

func TestTerminalSidebarWidthPersistsAndReloads(t *testing.T) {
	a := testApplication(t)
	response := requestApp(a, http.MethodPut, "/api/state", `{"terminalSidebarWidth":372}`, true)
	if response.Code != http.StatusOK {
		t.Fatalf("save width: %d %s", response.Code, response.Body.String())
	}
	disk, err := os.ReadFile(a.dataPath)
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err = json.Unmarshal(disk, &saved); err != nil || saved["terminalSidebarWidth"] != float64(372) {
		t.Fatalf("saved width: %s %v", disk, err)
	}
	response = requestApp(a, http.MethodGet, "/api/state", "", true)
	var loaded map[string]any
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &loaded) != nil || loaded["terminalSidebarWidth"] != float64(372) {
		t.Fatalf("reload width: %d %s", response.Code, response.Body.String())
	}
}

func TestTerminalConnectionSettingsTabPersists(t *testing.T) {
	a := testApplication(t)
	response := requestApp(a, http.MethodPut, "/api/state", `{"terminalSidebarTab":"settings","terminalSidebarCollapsed":true}`, true)
	if response.Code != http.StatusOK {
		t.Fatalf("save connection tab: %d %s", response.Code, response.Body.String())
	}
	saved := requestApp(a, http.MethodGet, "/api/state", "", true)
	var state map[string]any
	if err := json.Unmarshal(saved.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if state["terminalSidebarTab"] != "settings" || state["terminalSidebarCollapsed"] != true {
		t.Fatalf("connection tab preference lost: %#v", state)
	}
}
