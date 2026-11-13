package main

import (
	"errors"
	"testing"
)

// Each editor window keeps its own share of jobs: a new search drops the
// window's oldest finished job, never a running one and never another
// window's; with every job running it is refused in plain words.
func TestEditorFindJobsPerWindow(t *testing.T) {
	var f editorFind
	job := func(owner string) *editorFindJob {
		return &editorFindJob{id: newJobID(), owner: owner, kind: "all", cancel: func() {}}
	}
	var first *editorFindJob
	for i := 0; i < editorFindJobs; i++ {
		j := job("a")
		if i == 0 {
			first = j
		}
		if err := f.add(j); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.add(job("a")); !errors.Is(err, errFindBusy) {
		t.Fatalf("a window full of running searches started another: %v", err)
	}
	if err := f.add(job("b")); err != nil {
		t.Fatalf("another window's searches took this window's share: %v", err)
	}
	if _, err := f.job("a", first.id); err != nil {
		t.Fatal("a running search was dropped")
	}
	first.finish(map[string]any{"count": 1}, nil)
	if err := f.add(job("a")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.job("a", first.id); err == nil {
		t.Fatal("the finished search did not make room")
	}
}
