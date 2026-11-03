package macros

import (
	"strings"
	"testing"
)

// Owner request (v1.19.7): Last command, Last paste, Select output and Copy
// output must work for macro runs, not only paced pastes. The capture that
// backs those tools is therefore recorded for every run's command steps.
func TestMacro136CaptureAssociatesCommandsAndOutputForMacroRuns(t *testing.T) {
	e, b := engineFixture(t, "a")
	run, err := e.Start(StartRequest{Macro: definition("show one", "show two"), Targets: refs("a")})
	if err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "a", "show one\r")
	b.emit("a", "show one\r\nONE\r\nfixture# ")
	awaitWrite(t, b, "a", "show two\r")
	b.emit("a", "show two\r\nTWO\r\nfixture# ")
	result := awaitRun(t, e, run.ID, finished)
	if result.State != "completed" {
		t.Fatalf("run: %+v", result)
	}
	capture, err := e.PasteCapture(run.ID, "a")
	if err != nil {
		t.Fatal(err)
	}
	if capture.RunID != run.ID || capture.SessionID != "a" || capture.State != "completed" || len(capture.Steps) != 2 {
		t.Fatalf("capture: %+v", capture)
	}
	for i, command := range []string{"show one", "show two"} {
		if capture.Steps[i].Index != i+1 || capture.Steps[i].Command != command {
			t.Fatalf("step %d: %+v", i, capture.Steps[i])
		}
	}
	if !strings.Contains(capture.Steps[0].Output, "ONE") || strings.Contains(capture.Steps[0].Output, "TWO") || !strings.Contains(capture.Steps[1].Output, "TWO") {
		t.Fatalf("crossed boundaries: %+v", capture.Steps)
	}
	if _, err := e.PasteCapture(run.ID, "other"); err == nil {
		t.Fatal("cross-session capture allowed")
	}
	if _, err := e.PasteCapture("missing", "a"); err == nil {
		t.Fatal("unknown run captured")
	}
}
