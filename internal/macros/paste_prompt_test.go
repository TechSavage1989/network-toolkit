package macros

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestPastePromptSharedFixtures(t *testing.T) {
	data, err := os.ReadFile("../../scripts/fixtures/terminal-paced-prompts.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		MaxBytes int    `json:"maxBytes"`
		Pattern  string `json:"pattern"`
		Cases    []struct {
			Name     string `json:"name"`
			Line     string `json:"line"`
			Accepted bool   `json:"accepted"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if fixtures.MaxBytes != pastePromptMaxBytes || fixtures.Pattern != pastePromptPattern {
		t.Fatal("paced prompt grammar or byte limit differs from the shared browser fixtures")
	}
	if len(fixtures.Cases) == 0 {
		t.Fatal("missing shared prompt cases")
	}
	for _, tc := range fixtures.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			if strings.ContainsAny(tc.Line, "\r\n") {
				t.Fatal("a shared browser fixture must be one visible terminal line")
			}
			for _, prefix := range []string{"", "first output\r\nsecond output\r\n", strings.Repeat("x", 256*1024) + "\r\n"} {
				if got := matchPastePrompt(prefix + tc.Line); got != tc.Accepted {
					t.Fatalf("accepted=%v for final line %q, want %v", got, tc.Line, tc.Accepted)
				}
			}
		})
	}
}

func TestPasteAndOrdinaryMacroDefaultsRecognizeESXi(t *testing.T) {
	for _, pattern := range []string{DefaultPrompt, legacyDefaultPrompt} {
		def := modelDefinition()
		def.Prompt = pattern
		clean, err := Validate(def)
		if err != nil || !matchOutput(regexp.MustCompile(clean.Prompt), clean.Prompt, "\r\n[root@esxi-lab:~] ") {
			t.Fatalf("new and already-saved default macros must recognize ESXi prompts: %v", err)
		}
	}
	var def Definition
	if err := json.Unmarshal([]byte(`{"id":"fixture","name":"Fixture"}`), &def); err != nil {
		t.Fatal(err)
	}
	if def.Prompt != DefaultPrompt {
		t.Fatal("ordinary macro JSON defaults did not use the current recognizer")
	}
	paste, err := pasteDefinition("fixture first\nfixture second\n")
	if err != nil || paste.Prompt != DefaultPrompt {
		t.Fatalf("paste and macros do not share the default prompt grammar: %+v, %v", paste, err)
	}
}

func TestPasteESXiTwoCommandsBlanksChangingPathsAndStalePrompt(t *testing.T) {
	e, b := paste127Fixture(t)
	first := "lsof | grep -i 'fixture-vm'"
	second := "ps -c | grep -i 'fixture-vm'"
	// A retained prompt arrives on subscription, before the first command's
	// output cursor. It must not release the blank or second command.
	b.onSubscribe = func(id string) { b.emit(id, "\r\n[root@esxi-lab:~] ") }
	run, err := e.StartPaste(PasteRequest{Target: refs("synthetic")[0], Text: first + "\n\n" + second + "\n"})
	if err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "synthetic", first+"\r")
	paste127NoWriteFor(t, b, 2*pastePromptStable)
	// A split prompt (and split ANSI color) remains incomplete until its final
	// bracket; even a very long output line does not hide the final prompt.
	b.emit("synthetic", first+"\r\n"+strings.Repeat("x", 128*1024)+"\r\n\x1b[3")
	b.emit("synthetic", "2m[root@esxi-lab:/vmfs/volumes/Fixture Store")
	paste127NoWriteFor(t, b, 2*pastePromptStable)
	b.emit("synthetic", "]\x1b[0m ")
	awaitWrite(t, b, "synthetic", "\r")
	noWrite(t, b)
	b.emit("synthetic", "\r\n[root@esxi-lab:/vmfs/volumes/Fixture Store/Fixture VM (Copy)]")
	awaitWrite(t, b, "synthetic", second+"\r")
	noWrite(t, b)
	b.emit("synthetic", second+"\r\nfixture process output\r\n[root@esxi-lab:~/fixture dir] ")
	result := awaitRun(t, e, run.ID, finished)
	if result.State != "completed" || result.Targets[0].Step != 3 {
		t.Fatalf("ESXi paste did not complete its two commands and blank line: %+v", result)
	}
	capture, err := e.PasteCapture(run.ID, "synthetic")
	if err != nil || len(capture.Steps) != 3 {
		t.Fatalf("ESXi paste capture missing lines: %+v, %v", capture, err)
	}
	for i, want := range []string{first, "", second} {
		if capture.Steps[i].Command != want {
			t.Fatalf("command %d changed: %q, want %q", i+1, capture.Steps[i].Command, want)
		}
	}
	noWrite(t, b)
}

func TestPasteESXiNegativeAndIntermediateOutputCannotAdvance(t *testing.T) {
	for _, scenario := range []string{"no fresh line", "bracketed output", "unfinished command", "intermediate prompt", "delayed continuation"} {
		t.Run(scenario, func(t *testing.T) {
			e, b := paste127Fixture(t)
			run, err := e.StartPaste(PasteRequest{Target: refs("synthetic")[0], Text: "fixture first\nfixture next\n"})
			if err != nil {
				t.Fatal(err)
			}
			awaitWrite(t, b, "synthetic", "fixture first\r")
			switch scenario {
			case "no fresh line":
				b.emit("synthetic", "[root@esxi-lab:~] ")
			case "bracketed output":
				b.emit("synthetic", "fixture first\r\n[operation completed]")
			case "unfinished command":
				b.emit("synthetic", "fixture first\r\n[root@esxi-lab:~] echo unfinished")
			case "intermediate prompt":
				b.emit("synthetic", "fixture first\r\n[root@esxi-lab:~]")
				b.emit("synthetic", " still processing")
			case "delayed continuation":
				b.emit("synthetic", "fixture first\r\n[root@esxi-lab:~]")
				paste127NoWriteFor(t, b, 30*time.Millisecond)
				b.emit("synthetic", " still processing")
			}
			paste127NoWriteFor(t, b, 2*pastePromptStable)
			current, err := e.Get(run.ID)
			if err != nil || current.Targets[0].Step != 1 || current.FinishedAt != "" {
				t.Fatalf("non-prompt output advanced the ESXi paste: %+v, %v", current, err)
			}
			b.emit("synthetic", "\r\n[root@esxi-lab:/tmp/fixture path] ")
			awaitWrite(t, b, "synthetic", "fixture next\r")
			b.emit("synthetic", "fixture next\r\n[root@esxi-lab:~] ")
			if result := awaitRun(t, e, run.ID, finished); result.State != "completed" {
				t.Fatal(result)
			}
			noWrite(t, b)
		})
	}
}
