//go:build !windows

package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
}

// A separate program placed by the update helper must start: its --version run
// exits 0 and names the version that was installed.
func TestUpdateProgramStartCheck(t *testing.T) {
	dir := t.TempDir()
	program := filepath.Join(dir, "Network_Toolkit_Terminal.exe")
	for _, tc := range []struct {
		body string
		ok   bool
	}{
		{`[ "$1" = --version ] && echo "Network Toolkit 1.21.47 windows amd64"`, true},
		{`echo "Network Toolkit 1.21.46 windows amd64"`, false},
		{`echo "Network Toolkit 1.21.47 windows amd64"; exit 3`, false},
		{`exit 0`, false},
		{`sleep 30`, false},
	} {
		writeScript(t, program, tc.body)
		start := time.Now()
		err := checkUpdateProgramWithin(program, "1.21.47", 2*time.Second)
		if (err == nil) != tc.ok {
			t.Fatalf("%q: %v", tc.body, err)
		}
		if time.Since(start) > 10*time.Second {
			t.Fatalf("%q: the check did not stop at its time limit", tc.body)
		}
	}
}

// Network_Toolkit.exe hands a standalone app to its separate program, but runs
// the app itself when that program exits with a failure at once. A program
// that keeps running, or finishes successfully (a hand-over), is not run twice.
func TestForwardingFallsBackWhenTheSeparateProgramFails(t *testing.T) {
	original := forwardStartWindow
	forwardStartWindow = 1500 * time.Millisecond
	t.Cleanup(func() { forwardStartWindow = original })
	dir := t.TempDir()
	self := filepath.Join(dir, programName(""))
	writeScript(t, self, "exit 0")
	target := filepath.Join(dir, programName("terminal"))
	marker := filepath.Join(dir, "started")
	for _, tc := range []struct {
		name, body string
		forwarded  bool
	}{
		{"fails at once", "exit 2", false},
		{"crashes after starting", "sleep 0.3; exit 1", false},
		{"hands over", "exit 0", true},
		{"keeps running", "touch '" + marker + "'; sleep 5", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writeScript(t, target, tc.body)
			start := time.Now()
			if got := forwardFrom(self, "terminal", []string{"--terminal"}); got != tc.forwarded {
				t.Fatalf("forwarded %v, want %v", got, tc.forwarded)
			}
			if tc.name == "keeps running" {
				if time.Since(start) > 4*time.Second {
					t.Fatal("waited for a running program to end")
				}
				if _, err := os.Stat(marker); err != nil {
					t.Fatal("the separate program was not started")
				}
			}
		})
	}
}

// While the update helper has not confirmed the update, the restarted main
// window starts no separate program: undoing the update replaces their files.
func TestNoSeparateTerminalWindowWhileTheUpdateIsUnconfirmed(t *testing.T) {
	a := testApplication(t)
	opened := 0
	a.openTerminalWindow = func(context.Context, *terminalWindowPreset) (bool, error) { opened++; return false, nil }
	a.updateCommitPending.Store(true)
	r := requestApp(a, http.MethodPost, "/api/terminal-window", `{}`, true)
	if r.Code != http.StatusServiceUnavailable || !strings.Contains(r.Body.String(), "update") || opened != 0 {
		t.Fatalf("%d %s (opened %d)", r.Code, r.Body.String(), opened)
	}
	a.updateCommitPending.Store(false)
	if r = requestApp(a, http.MethodPost, "/api/terminal-window", `{}`, true); r.Code != http.StatusOK || opened != 1 {
		t.Fatalf("%d %s", r.Code, r.Body.String())
	}
}
