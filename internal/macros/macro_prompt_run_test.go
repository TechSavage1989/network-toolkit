package macros

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestEngineESXiMacrosRunEveryCommandWithNewAndSavedDefaults(t *testing.T) {
	for _, savedDefault := range []bool{false, true} {
		for _, multilineStep := range []bool{false, true} {
			name := "new default"
			if savedDefault {
				name = "saved old default"
			}
			if multilineStep {
				name += " multiline command step"
			} else {
				name += " separate command steps"
			}
			t.Run(name, func(t *testing.T) {
				e, b := engineFixture(t, "esxi")
				def := definition("show one", " show two ", "show three")
				if savedDefault {
					def.Prompt = legacyDefaultPrompt
					data, _ := json.Marshal(def)
					if err := json.Unmarshal(data, &def); err != nil {
						t.Fatal(err)
					}
				}
				if multilineStep {
					def.Steps = []Step{{ID: "batch", Kind: "command", Text: "show one\r\n\r\n show two \rshow three\n", TimeoutMs: 1500}}
				}
				b.onSubscribe = func(id string) { b.emit(id, "\r\n[root@VMHOST002:~] ") }
				run, err := e.Start(StartRequest{Macro: def, Targets: refs("esxi")})
				if err != nil {
					t.Fatal(err)
				}
				awaitWrite(t, b, "esxi", "show one\r")
				noWrite(t, b) // The retained prompt must not release command two.
				b.emit("esxi", "show one\r\nONE\r\n[completed]\r\n[root@VMHOST002:/vmfs/volumes/Fixture Store")
				noWrite(t, b) // An unrelated bracket and a partial prompt are insufficient.
				b.emit("esxi", "] ")
				awaitWrite(t, b, "esxi", " show two \r")
				noWrite(t, b)
				b.emit("esxi", " show two \r\nTWO\r\n\x1b[3")
				b.emit("esxi", "2m[root@VMHOST002:~/fixture dir")
				noWrite(t, b)
				b.emit("esxi", "]\x1b[0m ")
				awaitWrite(t, b, "esxi", "show three\r")
				b.emit("esxi", "show three\r\nTHREE\r\n[root@VMHOST002:~] ")
				result := awaitRun(t, e, run.ID, finished)
				if result.State != "completed" || result.Targets[0].Step != 3 || result.Targets[0].Total != 3 {
					t.Fatalf("macro stopped before all commands completed: %+v", result)
				}
				capture, err := e.PasteCapture(run.ID, "esxi")
				if err != nil || len(capture.Steps) != 3 {
					t.Fatalf("command capture is incomplete: %+v, %v", capture, err)
				}
				for i, command := range []string{"show one", " show two ", "show three"} {
					if capture.Steps[i].Command != command || !strings.Contains(capture.Steps[i].Output, []string{"ONE", "TWO", "THREE"}[i]) {
						t.Fatalf("capture step %d lost its command/output: %+v", i+1, capture.Steps[i])
					}
				}
				noWrite(t, b)
			})
		}
	}
}

func TestEngineESXiDefaultDoesNotOverrideCustomMacroPattern(t *testing.T) {
	e, b := engineFixture(t, "esxi")
	def := definition("show one", "show two")
	def.Prompt = `CUSTOM-READY$`
	run, err := e.Start(StartRequest{Macro: def, Targets: refs("esxi")})
	if err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "esxi", "show one\r")
	b.emit("esxi", "\r\n[root@VMHOST002:~] ")
	noWrite(t, b)
	b.emit("esxi", "\r\nCUSTOM-READY")
	awaitWrite(t, b, "esxi", "show two\r")
	b.emit("esxi", "\r\nCUSTOM-READY")
	if result := awaitRun(t, e, run.ID, finished); result.State != "completed" {
		t.Fatalf("custom pattern behavior changed: %+v", result)
	}
}

func TestMatchOutputLegacyWaitPatternKeepsOriginalGrammarAndByteLimit(t *testing.T) {
	re := regexp.MustCompile(legacyDefaultPrompt)
	for _, tc := range []struct {
		name, text string
		want       bool
	}{
		{"legacy prompt", "output\r\nrouter# ", true},
		{"ESXi prompt", "output\r\n[root@esxi:~] ", false},
		{"later output", "router#\r\nstill working", false},
		{"164 bytes", strings.Repeat("x", 160) + "#   ", true},
		{"165 bytes", strings.Repeat("x", 160) + "#    ", false},
		{"UTF8 byte bound", strings.Repeat("é", 82) + "#", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchOutput(re, legacyDefaultPrompt, tc.text); got != tc.want {
				t.Fatalf("legacy Wait accepted=%v, want %v for %q", got, tc.want, tc.text)
			}
		})
	}
}

func TestEngineLegacyWaitStepDoesNotInheritMigratedESXiDefault(t *testing.T) {
	e, b := engineFixture(t, "esxi")
	def := definition("one", "two")
	def.Prompt = legacyDefaultPrompt
	def.Steps = []Step{
		{ID: "one", Kind: "command", Text: "one"},
		{ID: "explicit-wait", Kind: "wait", Text: legacyDefaultPrompt, TimeoutMs: 1500},
		{ID: "two", Kind: "command", Text: "two"},
	}
	run, err := e.Start(StartRequest{Macro: def, Targets: refs("esxi")})
	if err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "esxi", "one\r")
	// This releases the migrated command prompt wait, but not the explicitly
	// configured legacy Wait step that follows it.
	b.emit("esxi", "one\r\n[root@esxi:~] ")
	awaitRun(t, e, run.ID, func(r Run) bool { return r.Targets[0].Step == 2 && r.Targets[0].Waiting != "" })
	noWrite(t, b)
	b.emit("esxi", "\r\nrouter# ")
	awaitWrite(t, b, "esxi", "two\r")
	b.emit("esxi", "two\r\n[root@esxi:~] ")
	if result := awaitRun(t, e, run.ID, finished); result.State != "completed" {
		t.Fatalf("explicit legacy Wait or migrated command prompt changed: %+v", result)
	}
	noWrite(t, b)
}
