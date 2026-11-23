package macros

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestModelMigratesOnlyShippedDefaultPrompt(t *testing.T) {
	for _, prompt := range []string{"", legacyDefaultPrompt, DefaultPrompt} {
		source := modelDefinition()
		source.Prompt = prompt
		encoded, err := json.Marshal(source)
		if err != nil {
			t.Fatal(err)
		}
		var restored Definition
		if err := json.Unmarshal(encoded, &restored); err != nil {
			t.Fatal(err)
		}
		clean, err := Validate(restored)
		if err != nil || clean.Prompt != DefaultPrompt {
			t.Fatalf("saved default prompt did not migrate: %q, %v", clean.Prompt, err)
		}
		if restored.Prompt != prompt || source.Prompt != prompt {
			t.Fatal("validation mutated the saved source")
		}
	}
	for _, custom := range []string{`READY$`, `(?:^|[\r\n])router[#>]$`, `(?m)^\[waiting\]$`} {
		source := modelDefinition()
		source.Prompt = custom
		clean, err := Validate(source)
		if err != nil || clean.Prompt != custom {
			t.Fatalf("custom prompt changed: %q, %v", clean.Prompt, err)
		}
	}
}

func TestModelMultilineCommandsPreserveOrderOptionsAndIDs(t *testing.T) {
	longID := strings.Repeat("x", 80)
	source := modelDefinition()
	source.Steps = []Step{
		{ID: "batch", Kind: "command", Text: "  show {{host}}  \r\n\r\nshow two\rshow three\n", TimeoutMs: 1200},
		{ID: "batch-line-2", Kind: "wait", Text: "READY", TimeoutMs: 600},
		{ID: longID, Kind: "command", Text: "disabled one\n disabled two ", TimeoutMs: 500, Disabled: true},
		{ID: "tail", Kind: "text", Text: "one\ntwo"},
	}
	original, _ := json.Marshal(source)
	clean, err := Validate(source)
	if err != nil {
		t.Fatal(err)
	}
	want := []Step{
		{ID: "batch", Kind: "command", Text: "  show {{host}}  ", TimeoutMs: 1200},
		{ID: "batch-line-2-2", Kind: "command", Text: "show two", TimeoutMs: 1200},
		{ID: "batch-line-3", Kind: "command", Text: "show three", TimeoutMs: 1200},
		{ID: "batch-line-2", Kind: "wait", Text: "READY", TimeoutMs: 600},
		{ID: longID, Kind: "command", Text: "disabled one", TimeoutMs: 500, Disabled: true},
		{ID: strings.Repeat("x", 73) + "-line-2", Kind: "command", Text: " disabled two ", TimeoutMs: 500, Disabled: true},
		{ID: "tail", Kind: "text", Text: "one\ntwo"},
	}
	if !reflect.DeepEqual(clean.Steps, want) {
		t.Fatalf("multiline steps lost content, options or ordering:\ngot %+v\nwant %+v", clean.Steps, want)
	}
	after, _ := json.Marshal(source)
	if string(original) != string(after) {
		t.Fatal("validation mutated the original definition")
	}
	again, err := Validate(clean)
	if err != nil || !reflect.DeepEqual(clean, again) {
		t.Fatalf("saved command expansion was not stable: %+v, %v", again, err)
	}
}

func TestModelMultilineCommandsKeepExistingLimitsAndValidation(t *testing.T) {
	source := modelDefinition()
	source.Steps[0].Text = strings.Repeat("show clock\n", MaxSteps)
	clean, err := Validate(source)
	if err != nil || len(clean.Steps) != MaxSteps {
		t.Fatalf("1024 expanded commands were rejected: %d, %v", len(clean.Steps), err)
	}
	source.Steps[0].Text += "show version"
	if _, err := Validate(source); err == nil || !strings.Contains(err.Error(), "1024") {
		t.Fatalf("expanded step bound was not enforced: %v", err)
	}
	source.Steps[0].Text = strings.Repeat("x\n", MaxStepTextBytes/2+1)
	if _, err := Validate(source); err == nil || !strings.Contains(err.Error(), "64 KiB") {
		t.Fatalf("original multiline field bypassed its byte bound: %v", err)
	}
	for _, invalid := range []string{"one\nshow {{missing}}", "one\nshow {{bad-name}}", "one\n\x00two"} {
		source.Steps[0].Text = invalid
		if _, err := Validate(source); err == nil {
			t.Fatalf("invalid text disappeared during splitting: %q", invalid)
		}
	}
	source.Steps = []Step{
		{ID: "keep", Kind: "command", Text: "show version"},
		{ID: "inactive", Kind: "command", Text: " \r\n\n", Disabled: true, TimeoutMs: 500},
	}
	clean, err = Validate(source)
	if err != nil || len(clean.Steps) != 2 || clean.Steps[1].ID != "inactive" || !clean.Steps[1].Disabled || clean.Steps[1].TimeoutMs != 500 || clean.Steps[1].Text != "" {
		t.Fatalf("inactive empty step was lost: %+v, %v", clean.Steps, err)
	}
}
