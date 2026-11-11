//go:build !windows

package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The update helper with real child processes and files on Unix: the main
// program and the separate Text Editor (installed, replaced) and Terminal (not
// installed yet, created) in one transaction. ReplaceFileW and the parent
// handle are injected, as in TestUpdateRestartControllerIntegration.
func TestUpdateHelperReplacesTheThreeProgramsTogether(t *testing.T) {
	const editor, terminal = "Network_Toolkit_TextEditor.exe", "Network_Toolkit_Terminal.exe"
	for _, mode := range []string{"healthy", "editor-locked", "terminal-not-created", "exit", "editor-changed", "candidate-changed", "terminal-appeared"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			dir, err := os.MkdirTemp(t.TempDir(), ".network-toolkit-update-")
			if err != nil {
				t.Fatal(err)
			}
			restored := filepath.Join(root, "restored-started")
			argsFile := filepath.Join(root, "new-args")
			nonce := strings.Repeat("b", 64)
			old := []byte("#!/bin/sh\nprintf restored > '" + restored + "'\n")
			newMain := []byte("#!/bin/sh\nexit 7\n")
			if mode != "exit" {
				newMain = []byte("#!/bin/sh\nprintf '%s ' \"$@\" > '" + argsFile + "'\nprintf '" + nonce + "' > '" + filepath.Join(dir, "ready") + "'\nsleep 1\n")
			}
			oldEditor, newEditor, newTerminal := []byte("old editor"), []byte("new editor"), []byte("new terminal")
			files := map[string][]byte{
				filepath.Join(root, "Network_Toolkit.exe"): old, filepath.Join(root, editor): oldEditor,
				filepath.Join(dir, "installer.exe"): old, filepath.Join(dir, "candidate.exe"): newMain,
				filepath.Join(dir, "candidate-"+editor): newEditor, filepath.Join(dir, "candidate-"+terminal): newTerminal,
			}
			for name, b := range files {
				if err = os.WriteFile(name, b, 0700); err != nil {
					t.Fatal(err)
				}
			}
			plan := updatePlan{Target: "Network_Toolkit.exe", TargetDir: root, WorkingDir: root, OriginalSHA256: updateHash(old), CandidateSHA256: updateHash(newMain), Version: "1.21.45", ParentPID: os.Getpid(), Nonce: nonce,
				Args: []string{"--data-dir", "relative data"},
				Companions: []updatePlanCompanion{
					{Target: editor, OriginalSHA256: updateHash(oldEditor), CandidateSHA256: updateHash(newEditor)},
					{Target: terminal, CandidateSHA256: updateHash(newTerminal)},
				}}
			b, _ := json.Marshal(plan)
			if err = os.WriteFile(filepath.Join(dir, "plan.json"), b, 0600); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "editor-changed":
				os.WriteFile(filepath.Join(root, editor), []byte("changed by the user"), 0700)
			case "candidate-changed":
				os.WriteFile(filepath.Join(dir, "candidate-"+terminal), []byte("tampered"), 0700)
			case "terminal-appeared":
				os.WriteFile(filepath.Join(root, terminal), []byte("another program"), 0700)
			}
			var replaced []string
			ops := updateInstallOperations{
				executable: func() (string, error) { return filepath.Join(dir, "installer.exe"), nil },
				waitParent: func(int, time.Duration, string) error { return nil },
				replace: func(target, next, backup string) error {
					if filepath.Dir(target) != filepath.Dir(next) || filepath.Dir(target) != filepath.Dir(backup) {
						t.Fatal("ReplaceFileW participants must share the program's folder")
					}
					if mode == "editor-locked" && filepath.Base(target) == editor && strings.Contains(next, "-candidate.exe") {
						return errors.New("The process cannot access the file because it is being used by another process.")
					}
					replaced = append(replaced, filepath.Base(target))
					if err := os.Rename(target, backup); err != nil {
						return err
					}
					return os.Rename(next, target)
				},
				restoreMissing: os.Rename, startupTimeout: 2 * time.Second,
			}
			if mode == "terminal-not-created" {
				ops.place = func(source, target string) error { return errors.New("Access is denied.") }
			}
			err = applyUpdatePlanWithOperations(dir, ops)
			result, _ := os.ReadFile(filepath.Join(dir, "update-result.txt"))
			read := func(name string) string { b, _ := os.ReadFile(filepath.Join(root, name)); return string(b) }
			if mode == "healthy" {
				if err != nil || !strings.Contains(string(result), "confirmed the application interface") {
					t.Fatalf("healthy: %v %s", err, result)
				}
				if read("Network_Toolkit.exe") != string(newMain) || read(editor) != string(newEditor) || read(terminal) != string(newTerminal) {
					t.Fatal("the three programs are not the new version")
				}
				if b, _ := os.ReadFile(filepath.Join(dir, "previous-"+editor)); string(b) != string(oldEditor) {
					t.Fatal("the replaced Text Editor program is not kept in the stage folder")
				}
				if args, _ := os.ReadFile(argsFile); string(args) != "--data-dir relative data --update-health "+dir+" " {
					t.Fatalf("the program the user installed from must restart with its own arguments: %q", args)
				}
				// Retention removes the separate programs' transaction files with the stage.
				c := completedUpdate{dir: dir, plan: plan}
				os.Remove(filepath.Join(dir, "installer.exe"))
				os.WriteFile(filepath.Join(dir, "previous.exe"), old, 0700)
				if err := cleanupCompletedUpdate(c); err != nil {
					t.Fatalf("cleanup: %v", err)
				}
				if _, err := os.Stat(dir); !os.IsNotExist(err) {
					t.Fatal("stage folder left behind")
				}
				entries, _ := os.ReadDir(root)
				for _, entry := range entries {
					if strings.HasPrefix(entry.Name(), ".network-toolkit-swap-") {
						t.Fatalf("transaction file left beside the programs: %s", entry.Name())
					}
				}
				return
			}
			if err == nil {
				t.Fatalf("%s: failure accepted", mode)
			}
			if read("Network_Toolkit.exe") != string(old) {
				t.Fatalf("%s: Network_Toolkit.exe was not restored", mode)
			}
			wantEditor := string(oldEditor)
			if mode == "editor-changed" {
				wantEditor = "changed by the user"
			}
			if read(editor) != wantEditor {
				t.Fatalf("%s: the Text Editor program is %q", mode, read(editor))
			}
			if mode == "terminal-appeared" {
				if read(terminal) != "another program" {
					t.Fatal("a file that appeared was changed")
				}
			} else if _, err := os.Stat(filepath.Join(root, terminal)); !os.IsNotExist(err) {
				t.Fatalf("%s: the created Terminal program was not removed", mode)
			}
			switch mode {
			case "editor-changed", "candidate-changed", "terminal-appeared":
				if len(replaced) != 0 {
					t.Fatalf("%s: files were replaced before the check: %v", mode, replaced)
				}
				if !strings.Contains(string(result), "was not applied") {
					t.Fatalf("%s: %s", mode, result)
				}
			default:
				deadline := time.Now().Add(2 * time.Second)
				for time.Now().Before(deadline) {
					if _, err = os.Stat(restored); err == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if _, err = os.Stat(restored); err != nil {
					t.Fatalf("%s: the previous version was not restarted", mode)
				}
			}
			entries, _ := os.ReadDir(root)
			for _, entry := range entries {
				if strings.Contains(entry.Name(), "-candidate.exe") || strings.Contains(entry.Name(), "-previous.exe") {
					t.Fatalf("%s: transaction file left beside the programs: %s", mode, entry.Name())
				}
			}
		})
	}
}

// The restart after an update starts the program the user installed from with
// that program's own arguments; one-shot requests are not repeated.
func TestUpdateRestartArgumentsOfEachApp(t *testing.T) {
	for _, tc := range []struct{ args, want []string }{
		{nil, []string{}},
		{[]string{"--terminal", "--open-url", "ssh://192.0.2.10"}, []string{"--terminal"}},
		{[]string{"--editor-standalone", "--open-file", "notes.txt", "--data-dir", "d"}, []string{"--editor-standalone", "--data-dir", "d"}},
		{[]string{"--open-url=telnet://192.0.2.10"}, []string{}},
		{[]string{"--no-browser"}, []string{"--no-browser"}},
	} {
		if got := updateRestartArgs(tc.args); strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Fatalf("%v: %v, want %v", tc.args, got, tc.want)
		}
	}
}
