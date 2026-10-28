package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestColorPickerPreferencesStrictBoundedAndIdempotent(t *testing.T) {
	input := map[string]any{"recent": []any{"#ABCDEF", "#abcdef", "#12345678", "#fff", "rgb(1,2,3)", " #112233", "#12zz34", 1, map[string]any{"secret": "canary"}}, "saved": []any{"#AABBCC", "#ffffff00", "url(canary)"}, "password": "canary"}
	got := safeColorPickerPreferences(input)
	want := map[string]any{"recent": []string{"#abcdef", "#12345678"}, "saved": []string{"#aabbcc", "#ffffff00"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected colors: %#v", got)
	}
	if !reflect.DeepEqual(got, safeColorPickerPreferences(got)) {
		t.Fatal("palette normalization must be idempotent")
	}
	data, _ := json.Marshal(got)
	if strings.Contains(string(data), "canary") {
		t.Fatal("noncolor field was retained")
	}
	for _, bad := range []any{nil, "secret", []any{}, 12, map[string]any{"saved": "secret"}} {
		if !reflect.DeepEqual(safeColorPickerPreferences(bad), map[string]any{"recent": []string{}, "saved": []string{}}) {
			t.Fatalf("invalid shape did not become empty colors: %#v", bad)
		}
	}
	items := make([]any, 300)
	for i := range items {
		items[i] = fmt.Sprintf("#%06x", i)
	}
	bounded := safeColorPickerPreferences(map[string]any{"recent": items, "saved": items})
	if len(bounded["recent"].([]string)) != 12 || len(bounded["saved"].([]string)) != 24 {
		t.Fatal("palette bounds were not enforced")
	}
}

func TestColorPickerPreferencesDoNotRewriteCustomThemesOrUnsetState(t *testing.T) {
	state := map[string]any{"accentColor": "#987654", "terminalAppearance": map[string]any{"scheme": "custom", "colors": map[string]any{"background": "#123456"}}, "diagnosticColors": map[string]any{"warn": "#aaaa00"}}
	before, _ := json.Marshal(state)
	sanitizeColorPickerPreferences(state)
	after, _ := json.Marshal(state)
	if string(before) != string(after) {
		t.Fatal("unset palette preference altered existing settings")
	}
	state["colorPickerPreferences"] = map[string]any{"saved": []any{"#ABCDEF"}, "other": "canary"}
	sanitizeColorPickerPreferences(state)
	if state["accentColor"] != "#987654" || state["terminalAppearance"].(map[string]any)["scheme"] != "custom" {
		t.Fatal("color history unexpectedly migrated the user's theme")
	}
	data, _ := json.Marshal(state)
	var roundtrip map[string]any
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatal(err)
	}
	sanitizeColorPickerPreferences(roundtrip)
	if !reflect.DeepEqual(state, roundtrip) {
		t.Fatal("synthetic persistence roundtrip changed saved colors")
	}
}
