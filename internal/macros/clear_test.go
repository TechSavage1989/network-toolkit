package macros

import (
	"errors"
	"sync"
	"testing"
)

func TestClearFinishedRunReleasesOutputAndKeepsTerminalUsable(t *testing.T) {
	e, b := engineFixture(t, "one")
	run, err := e.Start(StartRequest{Macro: definition("show version"), Targets: refs("one")})
	if err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "one", "show version\r")
	b.emit("one", "show version\r\nretained device output\r\nswitch# ")
	awaitRun(t, e, run.ID, func(r Run) bool { return r.FinishedAt != "" })
	if out, err := e.Output(run.ID, ""); err != nil || len(out) == 0 {
		t.Fatal("run did not retain output before clearing", err)
	}
	if err := e.Clear(run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Get(run.ID); err == nil {
		t.Fatal("cleared run remains available")
	}
	if _, err := e.Output(run.ID, "one"); err == nil {
		t.Fatal("cleared output remains available")
	}
	if len(e.List()) != 0 || len(e.runs) != 0 || len(e.order) != 0 {
		t.Fatal("clearing left run history behind")
	}
	if err := e.Clear(run.ID); err == nil {
		t.Fatal("clearing an unknown run should report that it was not found")
	}
	if _, err := e.Send(SendRequest{Targets: refs("one"), Text: "still connected"}); err != nil {
		t.Fatal("clearing closed the terminal", err)
	}
	awaitWrite(t, b, "one", "still connected")
}

func TestClearFinishedSelectionKeepsActivePausedAndUnselectedRuns(t *testing.T) {
	e, b := engineFixture(t, "one", "two", "three")
	finished := []string{}
	for _, id := range []string{"one", "two"} {
		run, err := e.Start(StartRequest{Macro: definition("show clock"), Targets: refs(id)})
		if err != nil {
			t.Fatal(err)
		}
		awaitWrite(t, b, id, "show clock\r")
		b.emit(id, "show clock\r\n12:00\r\nswitch# ")
		awaitRun(t, e, run.ID, func(r Run) bool { return r.FinishedAt != "" })
		finished = append(finished, run.ID)
	}
	def := definition("show never sent")
	def.Steps = []Step{{ID: "pause", Kind: "pause"}}
	active, err := e.Start(StartRequest{Macro: def, Targets: refs("three")})
	if err != nil {
		t.Fatal(err)
	}
	awaitRun(t, e, active.ID, func(r Run) bool { return r.State == "paused" })
	if err := e.Clear(active.ID); !errors.Is(err, ErrRunActive) {
		t.Fatalf("active clear: %v", err)
	}
	if ids := e.ClearFinished(nil); len(ids) != 0 {
		t.Fatal("empty selection cleared history", ids)
	}
	if ids := e.ClearFinished([]string{finished[0], active.ID, "unknown", finished[0]}); len(ids) != 1 || ids[0] != finished[0] {
		t.Fatal("clear did not respect selection or included active runs", ids)
	}
	if len(e.List()) != 2 || !e.OwnsPager("three") {
		t.Fatal("clear changed active ownership or an unselected finished run")
	}
	if _, err := e.Output(finished[1], ""); err != nil {
		t.Fatal("unselected output disappeared", err)
	}
	// Concurrent readers must see either the intact run or a not-found result.
	var readers sync.WaitGroup
	for range 4 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for range 100 {
				e.List()
				e.Get(finished[1])
				e.Output(finished[1], "")
			}
		}()
	}
	if ids := e.ClearFinished([]string{finished[1], active.ID}); len(ids) != 1 || ids[0] != finished[1] {
		t.Fatal(ids)
	}
	readers.Wait()
	if ids := e.ClearFinished(finished); len(ids) != 0 {
		t.Fatal("repeated clear is not idempotent", ids)
	}
	if _, err := e.Control(active.ID, "stop"); err != nil {
		t.Fatal(err)
	}
	awaitRun(t, e, active.ID, func(r Run) bool { return r.FinishedAt != "" })
	if err := e.Clear(active.ID); err != nil {
		t.Fatal("stopped run could not be cleared", err)
	}
	if len(e.List()) != 0 || e.OwnsPager("three") {
		t.Fatal("clear left stale history or terminal ownership")
	}
}
