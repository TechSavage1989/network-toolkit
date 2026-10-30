package macros

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// This is a synthetic, in-memory CLI, not a switch configuration or a claim
// about device timing. Hold each prompt until the test explicitly releases it:
// an HTTP/backend enqueue acknowledgement alone must not release the next line.
func TestPaste127PromptPacingPreserves300IndentedLines(t *testing.T) {
	e, b := paste127Fixture(t)
	lines := make([]string, 300)
	for i := range lines {
		lines[i] = fmt.Sprintf("    fixture-command %03d  value-%03d  ", i, i)
	}
	def, err := pasteDefinition(strings.Join(lines, "\n") + "\n")
	if err != nil {
		t.Fatal(err)
	}
	// Prompt withholding is the clock here. Even with fixed pacing set to zero,
	// the production 100ms fresh-prompt stability minimum runs for each line.
	def.PacingMs = 0
	// A retained prompt delivered after subscription is still older than the
	// first write cursor, and cannot authorize a second command.
	b.onSubscribe = func(id string) { b.emit(id, "\r\nfixture-old# ") }
	run, err := e.startPrepared(StartRequest{Targets: refs("synthetic")}, def, true)
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range lines {
		awaitWrite(t, b, "synthetic", line+"\r")
		// Split the echoed line, CR/LF, ANSI colour and prompt across output
		// events. Neither a partial echo nor an incomplete prompt is readiness.
		b.emit("synthetic", line[:12])
		b.emit("synthetic", line[12:]+"\r")
		b.emit("synthetic", "\n\x1b[3")
		b.emit("synthetic", "2mfixture(config-if)")
		noWrite(t, b)
		current, err := e.Get(run.ID)
		if err != nil || current.FinishedAt != "" || current.Targets[0].Step != i+1 {
			t.Fatalf("line %d advanced without a fresh complete prompt: %+v, %v", i, current, err)
		}
		b.emit("synthetic", "#\x1b[0m ")
	}
	result := awaitRun(t, e, run.ID, finished)
	if result.State != "completed" || result.Targets[0].Step != len(lines) {
		t.Fatalf("incomplete synthetic paste: %+v", result)
	}
	noWrite(t, b)
}

func TestPaste127StopDisconnectAndReconnectDoNotSendRemainingLines(t *testing.T) {
	for _, action := range []string{"stop", "disconnect", "reconnect"} {
		t.Run(action, func(t *testing.T) {
			e, b := paste127Fixture(t)
			lines := make([]string, 300)
			for i := range lines {
				lines[i] = fmt.Sprintf("    fixture-command %03d", i)
			}
			run, err := e.StartPaste(PasteRequest{Target: refs("synthetic")[0], Text: strings.Join(lines, "\n") + "\n"})
			if err != nil {
				t.Fatal(err)
			}
			awaitWrite(t, b, "synthetic", lines[0]+"\r")
			b.emit("synthetic", lines[0]+"\r\nprocessing")
			noWrite(t, b)
			if action == "stop" {
				if _, err := e.Control(run.ID, "stop"); err != nil {
					t.Fatal(err)
				}
			} else if action == "reconnect" {
				b.reconnect("synthetic")
			} else {
				b.mu.Lock()
				delete(b.live, "synthetic")
				for _, source := range b.sources["synthetic"] {
					source.Close()
				}
				b.mu.Unlock()
			}
			// A late prompt, including one from a replacement connection, must
			// not resume the old run or enqueue any of its 299 remaining lines.
			b.emit("synthetic", "\r\nfixture(config-if)# ")
			result := awaitRun(t, e, run.ID, finished)
			want := "disconnected"
			if action == "stop" {
				want = "stopped"
			}
			if result.Targets[0].State != want || result.Targets[0].Step != 1 {
				t.Fatalf("unexpected cancellation result: %+v", result)
			}
			noWrite(t, b)
			if action == "reconnect" {
				sent, err := e.Send(SendRequest{Targets: refs("synthetic"), Text: "must-not-reach-replacement", Execute: true})
				if err != nil || len(sent.Targets) != 1 || sent.Targets[0].Sent {
					t.Fatalf("stale generation was not rejected: %+v, %v", sent, err)
				}
				noWrite(t, b)
			}
		})
	}
}

func TestPaste127CommandConversionIsNotRawPaste(t *testing.T) {
	// Explicitly document the existing converter contract: nonblank command
	// bytes retain indentation/trailing spaces, but blank lines are omitted
	// and every command receives the selected terminal's Enter sequence.
	input := "    fixture one  \r\n \t\r\n\tfixture two\rfixture three\n"
	steps := FromCommands(input)
	want := []string{"    fixture one  ", "\tfixture two", "fixture three"}
	if len(steps) != len(want) {
		t.Fatalf("unexpected command count: %d", len(steps))
	}
	for i, step := range steps {
		if step.Kind != "command" || step.Text != want[i] || strings.ContainsAny(step.Text, "\r\n") {
			t.Fatalf("command %d changed: %+v", i, step)
		}
	}
}

// Keep the shared macro fixture unchanged: only this opt-in path requires SSH.
func paste127Fixture(t *testing.T) (*Engine, *fakeBackend) {
	e, b := engineFixture(t, "synthetic")
	b.mu.Lock()
	target := b.live["synthetic"]
	target.Type = "ssh"
	b.live["synthetic"] = target
	b.mu.Unlock()
	return e, b
}

func TestPaste127LiteralTemplatesBlanksAndLineBoundaries(t *testing.T) {
	e, b := paste127Fixture(t)
	text := "    literal {{host}} {{unknown}} {{broken  \r\n\r\n literal }}\r"
	run, err := e.StartPaste(PasteRequest{Target: refs("synthetic")[0], Text: text})
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"    literal {{host}} {{unknown}} {{broken  ", "", " literal }}"} {
		awaitWrite(t, b, "synthetic", line+"\r")
		noWrite(t, b)
		b.emit("synthetic", line+"\r\nfixture# ")
	}
	if result := awaitRun(t, e, run.ID, finished); result.State != "completed" {
		t.Fatal(result)
	}
	noWrite(t, b)
}

func TestPaste127ValidationRejectsUnsupportedInputWithoutWrites(t *testing.T) {
	for name, text := range map[string]string{
		"empty": "", "unfinished": "fixture", "tab": "\tfixture\n",
		"escape": "fixture\x1b[31m\n", "nul": "fixture\x00\n",
		"delete": "fixture\x7f\n", "C1": "fixture\u009b\n",
		"invalid UTF8": string([]byte{0xff, '\n'}),
		"too many lines": strings.Repeat("\n", MaxSteps+1),
		"too many bytes": strings.Repeat("x", 64<<10)+"\n",
	} {
		t.Run(name, func(t *testing.T) {
			e, b := paste127Fixture(t)
			if _, err := e.StartPaste(PasteRequest{Target: refs("synthetic")[0], Text: text}); err == nil {
				t.Fatal("unsupported paste accepted")
			}
			noWrite(t, b)
		})
	}
	for _, text := range []string{strings.Repeat("\n", MaxSteps), strings.Repeat("x", (64<<10)-1)+"\n", "fixture café\n"} {
		def, err := pasteDefinition(text)
		if err != nil || !utf8.ValidString(text) || len(def.Steps) == 0 || !def.WaitForPrompt || def.AutoPager || def.PacingMs != DefaultPacingMs || def.TimeoutMs != DefaultTimeoutMs {
			t.Fatalf("valid boundary or fixed safety defaults changed: %+v, %v", def, err)
		}
	}
}

func TestPaste127RejectsOtherTransportsAndStaleGeneration(t *testing.T) {
	for _, kind := range []string{"telnet", "serial", "local", "stale"} {
		t.Run(kind, func(t *testing.T) {
			e, b := paste127Fixture(t)
			b.mu.Lock()
			target := b.live["synthetic"]
			if kind == "stale" {
				target.Generation++
			} else {
				target.Type = kind
			}
			b.live["synthetic"] = target
			b.mu.Unlock()
			run, err := e.StartPaste(PasteRequest{Target: refs("synthetic")[0], Text: "fixture\n"})
			if err != nil || run.State != "failed" || len(run.Targets) != 1 || run.Targets[0].State != "failed" {
				t.Fatalf("expected explicit failed run: %+v, %v", run, err)
			}
			noWrite(t, b)
		})
	}
}

func TestPaste127DoesNotAutoAnswerPagerAndRawSendRemainsLiteral(t *testing.T) {
	e, b := paste127Fixture(t)
	run, err := e.StartPaste(PasteRequest{Target: refs("synthetic")[0], Text: "fixture one\nfixture two\n"})
	if err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "synthetic", "fixture one\r")
	b.emit("synthetic", "fixture one\r\n--More--")
	noWrite(t, b)
	if _, err := e.Control(run.ID, "stop"); err != nil {
		t.Fatal(err)
	}
	awaitRun(t, e, run.ID, finished)
	noWrite(t, b)
	// The existing unexecuted send is still raw terminal input, including
	// placeholders and control bytes; the new paste validator is not global.
	raw := "literal {{host}}\t\x1b[31m"
	result, err := e.Send(SendRequest{Targets: refs("synthetic"), Text: raw})
	if err != nil || len(result.Targets) != 1 || !result.Targets[0].Sent {
		t.Fatalf("raw send changed: %+v, %v", result, err)
	}
	awaitWrite(t, b, "synthetic", raw)
}

func paste127NoWriteFor(t *testing.T, b *fakeBackend, duration time.Duration) {
	t.Helper()
	select {
	case w := <-b.writes:
		t.Fatalf("unstable output authorized another write: %+v", w)
	case <-time.After(duration):
	}
}

func TestPaste127EchoAndIntermediatePromptFragmentsCannotAdvance(t *testing.T) {
	for _, scenario := range []string{"echo suffix", "intermediate prompt", "delayed continuation"} {
		t.Run(scenario, func(t *testing.T) {
			e, b := paste127Fixture(t)
			def, err := pasteDefinition("description budget $\nfixture next\n")
			if err != nil {
				t.Fatal(err)
			}
			def.PacingMs = 0 // Isolate prompt stability from post-prompt pacing.
			run, err := e.startPrepared(StartRequest{Targets: refs("synthetic")}, def, true)
			if err != nil {
				t.Fatal(err)
			}
			awaitWrite(t, b, "synthetic", "description budget $\r")
			switch scenario {
			case "echo suffix":
				// A command fragment ending in a prompt character is not a
				// fresh line boundary, even if it remains quiet for a while.
				b.emit("synthetic", "description budget $")
			case "intermediate prompt":
				b.emit("synthetic", "description budget $\r\nfixture#")
				b.emit("synthetic", " not-a-prompt")
			case "delayed continuation":
				b.emit("synthetic", "description budget $\r\nfixture#")
				paste127NoWriteFor(t, b, 30*time.Millisecond)
				b.emit("synthetic", " still processing")
			}
			paste127NoWriteFor(t, b, 2*pastePromptStable)
			current, err := e.Get(run.ID)
			if err != nil || current.Targets[0].Step != 1 || current.FinishedAt != "" {
				t.Fatalf("fragment advanced the paste: %+v, %v", current, err)
			}
			b.emit("synthetic", "\r\nfixture# ")
			awaitWrite(t, b, "synthetic", "fixture next\r")
			b.emit("synthetic", "fixture next\r\nfixture# ")
			if result := awaitRun(t, e, run.ID, finished); result.State != "completed" {
				t.Fatal(result)
			}
		})
	}
}

func TestPaste127StopDuringPromptStabilityDiscardsRemainingLines(t *testing.T) {
	e, b := paste127Fixture(t)
	run, err := e.StartPaste(PasteRequest{Target: refs("synthetic")[0], Text: "fixture one\nfixture two\n"})
	if err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "synthetic", "fixture one\r")
	b.emit("synthetic", "fixture one\r\nfixture# ")
	paste127NoWriteFor(t, b, 30*time.Millisecond)
	if _, err := e.Control(run.ID, "stop"); err != nil {
		t.Fatal(err)
	}
	if result := awaitRun(t, e, run.ID, finished); result.State != "stopped" || result.Targets[0].Step != 1 {
		t.Fatalf("stop lost the stability race: %+v", result)
	}
	paste127NoWriteFor(t, b, 2*pastePromptStable)
}
