package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestTerminalPromptLabels127DisplayOnlySanitizer(t *testing.T) {
	state := map[string]any{}
	raw := `{"terminalPromptNames":[{"type":"ssh","host":"192.0.2.5","port":22,"username":"operator","name":"EDGE-SW01","password":"never-retain"},{"type":"ssh","host":"192.0.2.5","port":22,"username":"operator","name":"DUPLICATE"},{"type":"ssh","host":"192.0.2.6","port":22,"username":"operator","name":"<script>"}]}`
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		t.Fatal(err)
	}
	sanitizeTerminalSettings(state)
	got := state["terminalPromptNames"].([]map[string]any)
	if len(got) != 1 || got[0]["name"] != "EDGE-SW01" {
		t.Fatalf("unexpected display data: %+v", got)
	}
	encoded, _ := json.Marshal(state)
	if strings.Contains(string(encoded), "never-retain") {
		t.Fatal("secret retained")
	}
	for _, value := range []string{"password", "login", "error", "name(config)", "name#", "<script>", "abc\n", strings.Repeat("a", 101)} {
		if validTerminalPromptName(value) {
			t.Fatalf("accepted invalid label %q", value)
		}
	}
	if !validTerminalPromptName("EDGE-SW01") {
		t.Fatal("valid device label rejected")
	}
}

func TestTerminalPromptLabels127WorkspaceDisplayIsIdempotent(t *testing.T) {
	input := map[string]any{"terminalPromptNames": []any{map[string]any{"type": "ssh", "host": "192.0.2.5", "port": float64(22), "username": "operator", "name": "EDGE-SW01", "password": "discard"}}}
	first := terminalWorkspaceDisplay(input)
	if len(first["terminalPromptNames"].([]map[string]any)) != 1 {
		t.Fatal("initial learned name was lost")
	}
	second := terminalWorkspaceDisplay(first)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("repeated display sanitation changed learned metadata: %+v -> %+v", first, second)
	}
	encoded, _ := json.Marshal(second)
	if strings.Contains(string(encoded), "discard") {
		t.Fatal("unexpected metadata field survived")
	}
}

func TestTerminalPromptLabels128NumericAndShellHostMetadata(t *testing.T) {
	for _, value := range []string{"7300", "7400", "LAB-PRINTER", "lab.example.test"} {
		if !validTerminalPromptName(value) {
			t.Fatalf("valid numeric or shell hostname rejected: %q", value)
		}
		input := map[string]any{"terminalPromptNames": []any{map[string]any{"type": "ssh", "host": "192.0.2.73", "port": float64(22), "username": "operator", "name": value, "password": "discard"}}}
		first := terminalWorkspaceDisplay(input)
		got := first["terminalPromptNames"].([]map[string]any)
		if len(got) != 1 || got[0]["name"] != value || len(got[0]) != 5 {
			t.Fatalf("invalid scoped learned metadata: %+v", got)
		}
		if !reflect.DeepEqual(first, terminalWorkspaceDisplay(first)) {
			t.Fatal("learned hostname lost during repeated workspace sanitation")
		}
	}
	for _, value := range []string{"192.0.2.73", "127.0.0.1", "1.2", "2001:db8::1", "operator@LAB-PRINTER", "LAB-PRINTER:~", "7300#", "password", "7300\n", "loader", "LOADER-A", "auditor", "console", "bash-5.2", "sh-4.4", "edit"} {
		if validTerminalPromptName(value) {
			t.Fatalf("invalid learned name accepted: %q", value)
		}
	}
}
