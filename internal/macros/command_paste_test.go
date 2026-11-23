package macros

import (
	"strings"
	"testing"
	"time"
)

func commandPasteFixture(t *testing.T, ids ...string) (*Engine, *fakeBackend) {
	t.Helper()
	e, b := engineFixture(t, ids...)
	b.mu.Lock()
	for _, id := range ids {
		target := b.live[id]
		target.Type = "ssh"
		b.live[id] = target
	}
	b.mu.Unlock()
	return e, b
}

// Group tasks may run in either order. Each target must still receive exactly
// one write, against the generation that was present in the reviewed request.
func commandPasteWrites(t *testing.T, b *fakeBackend, expected map[string]string) {
	t.Helper()
	remaining := make(map[string]string, len(expected))
	for id, text := range expected {
		remaining[id] = text
	}
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for len(remaining) > 0 {
		select {
		case w := <-b.writes:
			want, ok := remaining[w.id]
			if !ok || w.text != want || w.generation != 1 {
				t.Fatalf("unexpected reviewed write: %+v; remaining: %#v", w, remaining)
			}
			delete(remaining, w.id)
		case <-timer.C:
			t.Fatalf("reviewed writes missing: %#v", remaining)
		}
	}
}

func TestCommandPasteRawPreservesReviewedTextAndPerTargetFraming(t *testing.T) {
	for _, execute := range []bool{false, true} {
		for _, input := range []struct {
			name, text, cr, lf, crlf string
		}{
			{"trailing blanks", "  first {{host}}\r\n\r\nsecond\n\n", "  first {{host}}\r\rsecond\r\r", "  first {{host}}\n\nsecond\n\n", "  first {{host}}\r\n\r\nsecond\r\n\r\n"},
			{"unfinished final line", "first\r\nsecond {{session}}", "first\rsecond {{session}}", "first\nsecond {{session}}", "first\r\nsecond {{session}}"},
		} {
			name := "insert " + input.name
			if execute {
				name = "run " + input.name
			}
			t.Run(name, func(t *testing.T) {
				e, b := commandPasteFixture(t, "cr", "lf", "crlf")
				b.mu.Lock()
				for _, id := range []string{"cr", "lf", "crlf"} {
					target := b.live[id]
					target.LineEnding = id
					b.live[id] = target
				}
				b.mu.Unlock()
				result, err := e.Send(SendRequest{Targets: refs("cr", "lf", "crlf"), Text: input.text, Execute: execute, Method: "raw", Bracketed: map[string]bool{"lf": true, "crlf": false}})
				if err != nil || len(result.Targets) != 3 {
					t.Fatalf("raw group: %+v, %v", result, err)
				}
				for _, target := range result.Targets {
					if !target.Sent || target.Error != "" {
						t.Fatalf("raw target did not queue: %+v", target)
					}
				}
				commandPasteWrites(t, b, map[string]string{"cr": input.cr, "lf": "\x1b[200~" + input.lf + "\x1b[201~", "crlf": input.crlf})
				noWrite(t, b)
				if runs := e.List(); len(runs) != 0 {
					t.Fatalf("raw sending created an automation run: %+v", runs)
				}
			})
		}
	}
}

func TestCommandPasteESXiGroupPreservesBlanksAndLiteralTemplates(t *testing.T) {
	e, b := commandPasteFixture(t, "a", "b")
	b.mu.Lock()
	target := b.live["b"]
	target.LineEnding = "lf"
	b.live["b"] = target
	b.mu.Unlock()
	// Retained prompts cannot authorize subsequent lines, even when their
	// subscription delivery occurs immediately before the first group writes.
	b.onSubscribe = func(id string) { b.emit(id, "\r\n[root@old-host:~] ") }
	lines := []string{"  echo {{host}}  ", "", "echo {{session}} {{unknown}}"}
	run, err := e.StartPasteGroup(SendRequest{Targets: refs("a", "b"), Text: strings.Join(lines, "\n") + "\n", Execute: true, Method: "paced"})
	if err != nil || len(run.Targets) != 2 {
		t.Fatalf("literal group: %+v, %v", run, err)
	}
	for i, line := range lines {
		commandPasteWrites(t, b, map[string]string{"a": line + "\r", "b": line + "\n"})
		if i == 0 {
			paste127NoWriteFor(t, b, 250*time.Millisecond)
		} else {
			noWrite(t, b)
		}
		b.emit("a", line+"\r\n[root@esxi-a:~] ")
		b.emit("b", line+"\r\n[root@esxi-b:/vmfs/volumes/Shared Datastore] ")
	}
	result := awaitRun(t, e, run.ID, finished)
	if result.State != "completed" || len(e.List()) != 1 {
		t.Fatalf("literal group did not complete as one run: %+v", result)
	}
	for _, target := range result.Targets {
		if target.State != "completed" || target.Total != len(lines) || target.Step != len(lines) {
			t.Fatalf("group lost a literal line: %+v", target)
		}
		capture, err := e.PasteCapture(run.ID, target.ID)
		if err != nil || len(capture.Steps) != len(lines) {
			t.Fatalf("group capture: %+v, %v", capture, err)
		}
		for i, step := range capture.Steps {
			if step.Command != lines[i] {
				t.Fatalf("group capture expanded or dropped line %d: %+v", i, step)
			}
		}
	}
	noWrite(t, b)
}

func TestCommandPasteGroupPagerDefaultAndOverride(t *testing.T) {
	yes, no := true, false
	for _, tc := range []struct {
		name, key, reply string
		auto            *bool
	}{{"default", "", " ", nil}, {"disabled", "", "", &no}, {"enter", "enter", "\r", &yes}} {
		t.Run(tc.name, func(t *testing.T) {
			e, b := commandPasteFixture(t, "a")
			run, err := e.StartPasteGroup(SendRequest{Targets: refs("a"), Text: "show one\nshow two\n", Method: "paced", AutoPager: tc.auto, PagerKey: tc.key})
			if err != nil {
				t.Fatal(err)
			}
			awaitWrite(t, b, "a", "show one\r")
			b.emit("a", "show one\r\n--More--")
			if tc.reply != "" {
				awaitWrite(t, b, "a", tc.reply)
			}
			noWrite(t, b)
			b.emit("a", "\r\n[root@esxi:~] ")
			awaitWrite(t, b, "a", "show two\r")
			b.emit("a", "\r\n[root@esxi:~] ")
			if result := awaitRun(t, e, run.ID, finished); result.State != "completed" {
				t.Fatal(result)
			}
			noWrite(t, b)
		})
	}
}

func TestCommandPasteRejectsInvalidReviewBeforeAnyWrites(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  SendRequest
	}{
		{"unknown method", SendRequest{Targets: refs("a"), Text: "one\n", Method: "other"}},
		{"foreign framing", SendRequest{Targets: refs("a"), Text: "one\n", Method: "raw", Bracketed: map[string]bool{"outside": true}}},
		{"foreign false framing", SendRequest{Targets: refs("a"), Text: "one\n", Method: "raw", Bracketed: map[string]bool{"outside": false}}},
		{"paced framing", SendRequest{Targets: refs("a"), Text: "one\n", Method: "paced", Bracketed: map[string]bool{"a": true}}},
		{"unfinished paced", SendRequest{Targets: refs("a"), Text: "one\ntwo", Method: "paced"}},
		{"control paced", SendRequest{Targets: refs("a"), Text: "one\t\n", Method: "paced"}},
		{"empty raw", SendRequest{Targets: refs("a"), Method: "raw"}},
		{"nul raw", SendRequest{Targets: refs("a"), Text: "one\x00\n", Method: "raw"}},
		{"invalid UTF8", SendRequest{Targets: refs("a"), Text: string([]byte{0xff}), Method: "raw"}},
		{"oversize raw", SendRequest{Targets: refs("a"), Text: strings.Repeat("x", MaxStepTextBytes+1), Method: "raw"}},
		{"duplicate raw targets", SendRequest{Targets: refs("a", "a"), Text: "one\n", Method: "raw"}},
		{"duplicate paced targets", SendRequest{Targets: refs("a", "a"), Text: "one\n", Method: "paced"}},
		{"missing generation", SendRequest{Targets: []TargetRef{{ID: "a"}}, Text: "one\n", Method: "raw"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, b := commandPasteFixture(t, "a")
			var err error
			if tc.req.Method == "paced" {
				_, err = e.StartPasteGroup(tc.req)
			} else {
				_, err = e.Send(tc.req)
			}
			if err == nil {
				t.Fatal("invalid reviewed input was accepted")
			}
			if len(e.List()) != 0 {
				t.Fatal("invalid review created an automation run")
			}
			noWrite(t, b)
		})
	}
}

func TestCommandPasteGroupReportsBusyStaleAndUnsupportedTargets(t *testing.T) {
	e, b := commandPasteFixture(t, "busy", "stale", "ready", "telnet")
	b.mu.Lock()
	target := b.live["telnet"]
	target.Type = "telnet"
	b.live["telnet"] = target
	b.mu.Unlock()
	busy, err := e.StartPasteGroup(SendRequest{Targets: refs("busy"), Text: "hold\n", Method: "paced"})
	if err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "busy", "hold\r")
	b.reconnect("stale")
	sent, err := e.Send(SendRequest{Targets: refs("busy", "stale", "ready"), Text: "literal {{host}}\n", Method: "raw"})
	if err != nil || len(sent.Targets) != 3 || sent.Targets[0].Sent || sent.Targets[0].Error == "" || sent.Targets[1].Sent || sent.Targets[1].Error == "" || !sent.Targets[2].Sent {
		t.Fatalf("raw partial result: %+v, %v", sent, err)
	}
	awaitWrite(t, b, "ready", "literal {{host}}\r")
	run, err := e.StartPasteGroup(SendRequest{Targets: refs("busy", "stale", "ready", "telnet"), Text: "one\ntwo\n", Method: "paced"})
	if err != nil || len(run.Targets) != 4 {
		t.Fatalf("paced partial group: %+v, %v", run, err)
	}
	for _, line := range []string{"one", "two"} {
		awaitWrite(t, b, "ready", line+"\r")
		noWrite(t, b)
		b.emit("ready", "\r\n[root@esxi:~] ")
	}
	result := awaitRun(t, e, run.ID, finished)
	for _, target := range result.Targets {
		if target.ID == "ready" {
			if target.State != "completed" || target.Step != 2 {
				t.Fatalf("eligible destination did not complete: %+v", target)
			}
		} else if target.State != "failed" || target.Error == "" || target.Step != 0 {
			t.Fatalf("ineligible destination received a command: %+v", target)
		}
	}
	if _, err := e.Control(busy.ID, "stop"); err != nil {
		t.Fatal(err)
	}
	awaitRun(t, e, busy.ID, finished)
	noWrite(t, b)
}

func TestCommandPasteRawFencesReconnectAfterTargetSnapshot(t *testing.T) {
	e, b := commandPasteFixture(t, "a", "b")
	b.onWrite = func(w written) {
		if w.id == "a" {
			b.reconnect("b")
		}
	}
	result, err := e.Send(SendRequest{Targets: refs("a", "b"), Text: "one\ntwo", Method: "raw"})
	if err != nil || len(result.Targets) != 2 || !result.Targets[0].Sent || result.Targets[1].Sent || result.Targets[1].Error == "" {
		t.Fatalf("reconnected target was not fenced: %+v, %v", result, err)
	}
	commandPasteWrites(t, b, map[string]string{"a": "one\rtwo"})
	noWrite(t, b)
}
