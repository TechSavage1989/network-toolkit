package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTerminalAppearanceAndActionsPositionPersistAndTransfer(t *testing.T) {
	a := testApplication(t)
	a.dataPath = filepath.Join(filepath.Dir(a.dataPath), "Network_Toolkit.settings.json")
	body := `{"terminalConfirmClose":false,"terminalActiveActionsPosition":"bottom","terminalAppearance":{"fontFamily":"Cascadia Mono","fontWeight":"bold","lineHeight":1.3,"scheme":"custom","highlightUnderlines":false,"highlightColors":{"address":"#AABBCC","command":"#112233","field":"#223344","protocol":"#334455","value":"#445566","number":"#556677","comment":"#667788","error":"url(secret-canary)","unknown":"secret-canary"},"colors":{"background":"#112233","foreground":"#FAFAFA","brightRed":"#FF8888","selectionBackground":"#12345688","command":"secret-canary"},"shells":{"powershell":{"enabled":false,"colors":{"command":"#AABBCC","outputAddress":"#CC99EE","string":"url(secret-canary)","unknown":"#ffffff"}},"cmd":{"enabled":true,"colors":{"variable":"#112233"}},"unknown":"secret-canary"},"password":"secret-canary"}}`
	r := requestApp(a, http.MethodPut, "/api/state", body, true)
	if r.Code != http.StatusOK {
		t.Fatalf("save terminal appearance: %d %s", r.Code, r.Body.String())
	}
	disk, err := os.ReadFile(a.dataPath)
	if err != nil {
		t.Fatal(err)
	}
	check := func(data []byte) {
		t.Helper()
		if bytes.Contains(data, []byte("secret-canary")) {
			t.Fatal("appearance retained an unknown settings field")
		}
		var state map[string]any
		if err := json.Unmarshal(data, &state); err != nil {
			t.Fatal(err)
		}
		appearance, ok := state["terminalAppearance"].(map[string]any)
		if !ok || appearance["fontFamily"] != "Cascadia Mono" || appearance["lineHeight"] != 1.3 || state["terminalActiveActionsPosition"] != "bottom" || appearance["colors"].(map[string]any)["foreground"] != "#fafafa" || appearance["highlightUnderlines"] != false || appearance["highlightColors"].(map[string]any)["address"] != "#aabbcc" {
			t.Fatalf("appearance or toolbar position was lost: %#v", state)
		}
		if state["terminalConfirmClose"] != false {
			t.Fatal("close confirmation preference was lost")
		}
		shells := appearance["shells"].(map[string]any)
		want := map[string]any{
			"powershell": map[string]any{"enabled": false, "colors": map[string]any{"command": "#aabbcc", "outputAddress": "#cc99ee"}},
			"cmd":        map[string]any{"enabled": true, "colors": map[string]any{"variable": "#112233"}},
		}
		if !reflect.DeepEqual(shells, want) {
			t.Fatalf("shell preferences did not survive persistence/transfer: %#v", shells)
		}
		highlights := appearance["highlightColors"].(map[string]any)
		for key, color := range map[string]string{"command": "#112233", "field": "#223344", "protocol": "#334455", "value": "#445566", "number": "#556677", "comment": "#667788"} {
			if highlights[key] != color {
				t.Fatalf("configuration highlight color was lost: %s=%v", key, highlights[key])
			}
		}
	}
	check(disk)
	check(a.terminalViewDisplay())
	var state map[string]any
	_ = json.Unmarshal(disk, &state)
	workspace := terminalWorkspaceDisplay(state)
	if !reflect.DeepEqual(workspace, terminalWorkspaceDisplay(workspace)) {
		t.Fatal("appearance normalization changed on repeated workspace transfer")
	}
	data, _ := json.Marshal(workspace)
	check(data)
	dest := terminalWorkspace(filepath.Dir(a.dataPath))
	if err := os.MkdirAll(dest, 0700); err != nil {
		t.Fatal(err)
	}
	if err := seedTerminalWorkspace(filepath.Dir(a.dataPath), dest); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(dest, "Network_Toolkit.settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	check(data)
}

func TestInvalidTerminalAppearanceAndPositionUseDefaults(t *testing.T) {
	state := map[string]any{"terminalActiveActionsPosition": map[string]any{"command": "secret-canary"}, "terminalAppearance": map[string]any{"fontFamily": "url(secret-canary)", "fontWeight": "900", "lineHeight": -1.0, "scheme": "unknown", "highlightUnderlines": "false", "highlightColors": map[string]any{"address": "#ABCDEF", "error": "url(secret-canary)", "unknown": "#112233"}, "colors": map[string]any{"background": "url(secret-canary)", "foreground": "#12345678", "blue": "#ABCDEF"}}}
	sanitizeTerminalSettings(state)
	appearance := state["terminalAppearance"].(map[string]any)
	if state["terminalActiveActionsPosition"] != "top" || appearance["fontFamily"] != "Consolas" || appearance["scheme"] != "app" || appearance["lineHeight"] != 1.15 || appearance["highlightUnderlines"] != true {
		t.Fatalf("invalid appearance did not use defaults: %#v", state)
	}
	highlights := appearance["highlightColors"].(map[string]any)
	if len(highlights) != 1 || highlights["address"] != "#abcdef" {
		t.Fatalf("invalid highlight colors were accepted: %#v", highlights)
	}
	colors := appearance["colors"].(map[string]any)
	if len(colors) != 1 || colors["blue"] != "#abcdef" {
		t.Fatalf("invalid color values were accepted: %#v", colors)
	}
}

func TestLocalShellSettingsUseSafeIndependentDefaults(t *testing.T) {
	for _, value := range []any{nil, "false", []any{}, map[string]any{"powershell": map[string]any{"enabled": "false", "colors": map[string]any{"command": "#12345678"}}, "cmd": false}} {
		clean := safeTerminalShellAppearance(value)
		for _, kind := range []string{"powershell", "cmd"} {
			pref := clean[kind].(map[string]any)
			if pref["enabled"] != true || len(pref["colors"].(map[string]any)) != 0 {
				t.Fatalf("invalid shell settings changed defaults: %#v", clean)
			}
		}
	}
	for _, value := range []any{nil, "false", 0, []any{}} {
		state := map[string]any{"terminalConfirmClose": value}
		sanitizeTerminalSettings(state)
		if _, exists := state["terminalConfirmClose"]; exists {
			t.Fatalf("nonboolean close preference survived: %#v", state)
		}
	}
}
