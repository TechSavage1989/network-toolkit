package main

import (
	"strings"
	"testing"
)

func TestTerminalSyntaxKeepsOnlySafeCustomRuleSets(t *testing.T) {
	custom := []any{
		map[string]any{"id": "standard", "name": "Clash with a built-in"},
		map[string]any{"id": "Bad ID", "name": "Invalid id"},
		map[string]any{"id": "custom-net", "name": "  Network\nkeywords\x01 ", "regex": true, "caseSensitive": true, "password": "not-a-setting",
			"rules": map[string]any{"red": "(down)\r\n|(error)", "blue": strings.Repeat("x", 9000), "blinking": "ignored"}},
		map[string]any{"id": "custom-net", "name": "Duplicate"},
		map[string]any{"id": "custom-words", "regex": false, "rules": "not a map"},
	}
	for i := 0; i < 60; i++ {
		custom = append(custom, map[string]any{"id": "custom-" + strings.Repeat("a", 1+i%3) + string(rune('a'+i%26)) + string(rune('a'+i/26)), "name": "Filler"})
	}
	state := map[string]any{"terminalSyntax": map[string]any{"active": "custom-net", "custom": custom, "secret": "dropped"}}
	sanitizeTerminalSettings(state)
	clean := state["terminalSyntax"].(map[string]any)
	if len(clean) != 2 || clean["active"] != "custom-net" {
		t.Fatalf("unexpected keys or active rule set: %#v", clean)
	}
	sets := clean["custom"].([]any)
	if len(sets) != terminalSyntaxMaxCustom {
		t.Fatalf("custom rule sets were not capped: %d", len(sets))
	}
	first := sets[0].(map[string]any)
	if first["id"] != "custom-net" || first["name"] != "Network keywords" || first["regex"] != true || first["caseSensitive"] != true || len(first) != 5 {
		t.Fatalf("first rule set not sanitized: %#v", first)
	}
	rules := first["rules"].(map[string]any)
	if rules["red"] != "(down) |(error)" || len([]rune(rules["blue"].(string))) != terminalSyntaxMaxPattern || len(rules) != 7 || rules["blinking"] != nil {
		t.Fatalf("rules not sanitized: %#v", rules)
	}
	second := sets[1].(map[string]any)
	if second["id"] != "custom-words" || second["name"] != "Custom syntax" || second["regex"] != false || second["rules"].(map[string]any)["red"] != "" {
		t.Fatalf("second rule set not sanitized: %#v", second)
	}
	for _, active := range []string{"missing", "", "standard", "smart"} {
		state := map[string]any{"terminalSyntax": map[string]any{"active": active}}
		sanitizeTerminalSettings(state)
		want := active
		if active == "missing" || active == "" {
			want = "smart"
		}
		if got := state["terminalSyntax"].(map[string]any)["active"]; got != want {
			t.Fatalf("active %q became %q, want %q", active, got, want)
		}
	}
}
