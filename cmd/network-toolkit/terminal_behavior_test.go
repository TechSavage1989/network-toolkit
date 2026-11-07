package main

import (
	"encoding/json"
	"testing"
)

func TestTerminalBehaviorKeepsValidSettingsAndDefaultsTheRest(t *testing.T) {
	var source map[string]any
	if err := json.Unmarshal([]byte(`{"scrollback":2500,"scrollbar":false,"bell":"sound","bracketedPaste":false,"wordSeparators":" ,;:ab1\u0007|","lfImpliesCr":true,"crImpliesLf":true,"answerback":"lab-term\r\néok","reconnectKeys":false,"closeOnOtherKey":true,"password":"secret-canary"}`), &source); err != nil {
		t.Fatal(err)
	}
	state := map[string]any{"terminalBehavior": source}
	sanitizeTerminalSettings(state)
	clean := state["terminalBehavior"].(map[string]any)
	want := map[string]any{"scrollback": 2500, "scrollbar": false, "bell": "sound", "bracketedPaste": false, "wordSeparators": " ,;:|", "lfImpliesCr": true, "crImpliesLf": true, "answerback": "lab-termok", "reconnectKeys": false, "closeOnOtherKey": true}
	if len(clean) != len(want) {
		t.Fatalf("unexpected keys kept: %#v", clean)
	}
	for key, value := range want {
		if clean[key] != value {
			t.Fatalf("%s = %#v, want %#v (all: %#v)", key, clean[key], value, clean)
		}
	}
	for _, bad := range []any{nil, "x", []any{}, map[string]any{"scrollback": 100000.0, "bell": "loud", "scrollbar": "false", "answerback": 5, "wordSeparators": 7}, map[string]any{"scrollback": 2500.5}} {
		clean := safeTerminalBehavior(bad)
		if clean["scrollback"] != 10000 || clean["scrollbar"] != true || clean["bell"] != "none" || clean["bracketedPaste"] != true || clean["wordSeparators"] != terminalDefaultWordSeparators || clean["lfImpliesCr"] != false || clean["crImpliesLf"] != false || clean["answerback"] != "" || clean["reconnectKeys"] != true || clean["closeOnOtherKey"] != false {
			t.Fatalf("invalid value %#v did not keep today's behavior: %#v", bad, clean)
		}
	}
	long := safeTerminalBehavior(map[string]any{"answerback": string(make([]byte, 200)) + "x"})
	if long["answerback"] != "x" {
		t.Fatalf("control characters must be dropped: %#v", long["answerback"])
	}
}
