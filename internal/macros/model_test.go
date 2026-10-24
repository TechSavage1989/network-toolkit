package macros

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func modelDefinition() Definition {
	return Definition{ID: "macro-1", Name: "Check interfaces", WaitForPrompt: true, PacingMs: DefaultPacingMs, Steps: []Step{{ID: "first", Kind: "command", Text: "show interfaces"}}}
}

func TestModelJSONDefaultsAndUnknownFields(t *testing.T) {
	var definition Definition
	if err := json.Unmarshal([]byte(`{"id":"one","name":"One","parameters":[{"name":"value"}],"steps":[{"kind":"command","text":"show version"}],"runtimeValues":{"password":"never-save"},"script":"local-program"}`), &definition); err != nil {
		t.Fatal(err)
	}
	if !definition.WaitForPrompt || definition.TimeoutMs != DefaultTimeoutMs || definition.PacingMs != DefaultPacingMs || !definition.Parameters[0].Required {
		t.Fatalf("defaults were not applied: %+v", definition)
	}
	encoded, err := json.Marshal(definition)
	if err != nil || strings.Contains(string(encoded), "never-save") || strings.Contains(string(encoded), "local-program") {
		t.Fatalf("unknown fields were retained: %s, %v", encoded, err)
	}
	if err := json.Unmarshal([]byte(`{"waitForPrompt":false,"pacingMs":0,"parameters":[{"required":false}]}`), &definition); err != nil {
		t.Fatal(err)
	}
	if definition.WaitForPrompt || definition.PacingMs != 0 || definition.Parameters[0].Required {
		t.Fatal("explicit false and zero must survive decoding")
	}
	for _, input := range []string{`{"steps":"command"}`, `{"timeoutMs":1.5}`, `{"waitForPrompt":"false"}`, `{"parameters":[{"name":3}]}`} {
		if err := json.Unmarshal([]byte(input), &definition); err == nil {
			t.Fatalf("accepted incorrect JSON field type: %s", input)
		}
	}
}

func TestModelValidationCanonicalizesDetachedCopy(t *testing.T) {
	definition := modelDefinition()
	definition.Name, definition.Folder, definition.Shortcut = "  Inspect  ", " Network ", "shift+control+KeyM"
	definition.Tags = []string{" network ", "network", "", "check"}
	definition.Parameters = []Parameter{{Name: "password", Label: " Password ", DefaultValue: "never-store-this", Secret: true, Required: true}}
	definition.Steps = []Step{{ID: "existing-step", Kind: "command", Text: "echo <b>{{host}}</b> & {{password}}"}, {Kind: "pause", Text: "Review <details> before continuing"}}
	clean, err := Validate(definition)
	if err != nil {
		t.Fatal(err)
	}
	if clean.ID != "macro-1" || clean.Name != "Inspect" || clean.Folder != "Network" || clean.Shortcut != "Ctrl+Shift+M" || clean.Prompt != DefaultPrompt || clean.TimeoutMs != DefaultTimeoutMs {
		t.Fatalf("unexpected canonical definition: %+v", clean)
	}
	if clean.Parameters[0].DefaultValue != "" || definition.Parameters[0].DefaultValue != "never-store-this" {
		t.Fatal("secret defaults must be stripped without mutating source")
	}
	if clean.Steps[0].ID != "existing-step" || clean.Steps[1].ID != "step-2" || clean.Steps[0].Text != definition.Steps[0].Text || !reflect.DeepEqual(clean.Tags, []string{"network", "check"}) {
		t.Fatalf("lost stable IDs, literal HTML text or tags: %+v", clean)
	}
	clean.Steps[0].Text, clean.Tags[0] = "changed", "changed"
	if definition.Steps[0].Text == "changed" || definition.Tags[0] == "changed" {
		t.Fatal("validation must return detached slices")
	}
}

func TestModelRejectsMalformedDefinitions(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Definition)
	}{
		{"empty ID", func(d *Definition) { d.ID = "" }},
		{"empty name", func(d *Definition) { d.Name = " " }},
		{"name control", func(d *Definition) { d.Name = "name\x1b" }},
		{"unknown kind", func(d *Definition) { d.Steps[0].Kind = "script" }},
		{"unsupported key", func(d *Definition) { d.Steps[0] = Step{Kind: "key", Key: "RunProgram"} }},
		{"unresolved parameter", func(d *Definition) { d.Steps[0].Text = "show {{interface}}" }},
		{"malformed placeholder", func(d *Definition) { d.Steps[0].Text = "show {{bad-name}}" }},
		{"builtin parameter", func(d *Definition) { d.Parameters = []Parameter{{Name: "HOST"}} }},
		{"duplicate parameter", func(d *Definition) { d.Parameters = []Parameter{{Name: "value"}, {Name: "value"}} }},
		{"multiline parameter", func(d *Definition) { d.Parameters = []Parameter{{Name: "value", DefaultValue: "a\nb"}} }},
		{"duplicate step ID", func(d *Definition) { d.Steps = append(d.Steps, d.Steps[0]) }},
		{"all disabled", func(d *Definition) { d.Steps[0].Disabled = true }},
		{"multiline command", func(d *Definition) { d.Steps[0].Text = "one\ntwo" }},
		{"NUL command", func(d *Definition) { d.Steps[0].Text = "one\x00two" }},
		{"long step", func(d *Definition) { d.Steps[0].Text = strings.Repeat("é", MaxStepTextBytes/2+1) }},
		{"bad prompt", func(d *Definition) { d.Prompt = "(" }},
		{"unsupported lookaround", func(d *Definition) { d.Steps[0] = Step{Kind: "wait", Text: "(?=ready)"} }},
		{"bad wait", func(d *Definition) { d.Steps[0] = Step{Kind: "wait", Text: "["} }},
		{"negative timeout", func(d *Definition) { d.TimeoutMs = -1 }},
		{"short timeout", func(d *Definition) { d.Steps[0].TimeoutMs = 99 }},
		{"long delay", func(d *Definition) { d.Steps[0] = Step{Kind: "delay", DelayMs: MaxTimeoutMs + 1} }},
		{"unmodified shortcut", func(d *Definition) { d.Shortcut = "m" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definition := modelDefinition()
			test.edit(&definition)
			if _, err := Validate(definition); err == nil {
				t.Fatal("accepted invalid definition")
			}
		})
	}
}

func TestModelValidActionsAndBounds(t *testing.T) {
	definition := modelDefinition()
	definition.Parameters = []Parameter{{Name: "interface", Required: true}}
	definition.Steps = []Step{
		{Kind: "command", Text: "show interface {{interface}}"},
		{Kind: "text", Text: "one\ntwo"},
		{Kind: "key", Key: "Ctrl+C", Text: "discard", DelayMs: 1},
		{Kind: "wait", Text: `(?im){{host}}.*ready$`, TimeoutMs: 100},
		{Kind: "delay", DelayMs: MaxTimeoutMs},
		{Kind: "pause", Text: "Review {{session}}"},
	}
	clean, err := Validate(definition)
	if err != nil {
		t.Fatal(err)
	}
	if clean.Steps[2].Text != "" || clean.Steps[2].DelayMs != 0 || clean.Steps[2].TimeoutMs != 0 {
		t.Fatal("irrelevant key fields were retained")
	}
	definition = modelDefinition()
	definition.Steps[0].Text = strings.Repeat("a", MaxStepTextBytes)
	if _, err := Validate(definition); err != nil {
		t.Fatalf("rejected 64 KiB boundary: %v", err)
	}
	definition.Steps = make([]Step, MaxSteps)
	for index := range definition.Steps {
		definition.Steps[index] = Step{Kind: "command", Text: "show clock"}
	}
	if _, err := Validate(definition); err != nil {
		t.Fatalf("rejected 1024-step boundary: %v", err)
	}
	definition.Steps = append(definition.Steps, Step{Kind: "command", Text: "show clock"})
	if _, err := Validate(definition); err == nil {
		t.Fatal("accepted too many steps")
	}
	definition.Steps = make([]Step, 65)
	for index := range definition.Steps {
		definition.Steps[index] = Step{Kind: "text", Text: strings.Repeat("a", MaxStepTextBytes)}
	}
	if _, err := Validate(definition); err == nil || !strings.Contains(err.Error(), "4 MiB") {
		t.Fatalf("definition size limit not enforced: %v", err)
	}
}

func TestModelExpandLiteralParameters(t *testing.T) {
	text := `show {{interface}} on {{Host}}: {{interface}}`
	values := map[string]string{"interface": `<tag>$HOME;$(touch file) {{username}}`, "host": "router-1"}
	got, err := Expand(text, values)
	want := `show <tag>$HOME;$(touch file) {{username}} on router-1: <tag>$HOME;$(touch file) {{username}}`
	if err != nil || got != want {
		t.Fatalf("Expand must substitute once, literally: %q, %v", got, err)
	}
	if got := DetectParams(text); !reflect.DeepEqual(got, []string{"interface", "Host"}) {
		t.Fatalf("unexpected parameter order: %v", got)
	}
	for _, input := range []struct{ text, value string }{
		{"{{missing}}", ""}, {"{{value", ""}, {"{{bad-name}}", ""}, {"{{value}}", "one\ntwo"}, {"{{value}}", "one\rtwo"}, {"{{value}}", "one\x00two"}, {"{{value}}", strings.Repeat("a", MaxStepTextBytes+1)},
	} {
		if _, err := Expand(input.text, map[string]string{"value": input.value}); err == nil {
			t.Fatalf("accepted invalid expansion %q", input.text)
		}
	}
}

func TestModelFromCommandsAndKeys(t *testing.T) {
	got := FromCommands(" show version \r\n\r\necho <details>&value\rshow clock\n")
	want := []Step{{ID: "step-1", Kind: "command", Text: " show version "}, {ID: "step-2", Kind: "command", Text: "echo <details>&value"}, {ID: "step-3", Kind: "command", Text: "show clock"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("split lost literal command text: %+v", got)
	}
	for name, want := range map[string]string{"Enter": "\r", "Tab": "\t", "Escape": "\x1b", "Ctrl+C": "\x03", "Ctrl+Z": "\x1a", "Ctrl+D": "\x04", "Up": "\x1b[A", "Down": "\x1b[B", "Left": "\x1b[D", "Right": "\x1b[C", "Backspace": "\x7f"} {
		if got, ok := KeyBytes(name); !ok || got != want {
			t.Fatalf("key %s: %q %t", name, got, ok)
		}
	}
	if _, ok := KeyBytes("unknown"); ok {
		t.Fatal("accepted unknown key")
	}
}
