//go:build !windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Exercise the complete controller, real child processes and files on Unix.
// Replacement and parent handles are injected because ReplaceFileW requires Windows.
func TestUpdateRestartControllerIntegration(t *testing.T) {
	for _, variant := range []string{"legacy", "settings-custom", "settings-versioned", "settings-cross-volume"} {
		modern := variant != "legacy"
		for _, mode := range []string{"healthy", "exit", "timeout", "changed", "wait-failed"} {
			t.Run(fmt.Sprintf("%s/%s", mode, variant), func(t *testing.T) {
				root := t.TempDir()
				stageRoot := root
				if modern {
					stageRoot = t.TempDir()
				}
				if variant == "settings-cross-volume" {
					var err error
					stageRoot, err = os.MkdirTemp("/dev/shm", "network-toolkit-test-")
					if err != nil {
						t.Skip("separate temporary volume unavailable")
					}
					t.Cleanup(func() { os.RemoveAll(stageRoot) })
				}
				dir, err := os.MkdirTemp(stageRoot, ".network-toolkit-update-")
				if err != nil {
					t.Fatal(err)
				}
				targetName := "app.exe"
				if variant == "settings-versioned" {
					targetName = updateOfficialExecutableName("1.17.59")
				}
				launched := filepath.Join(root, "restored-started")
				old := []byte("#!/bin/sh\nprintf restored > '" + launched + "'\n")
				nonce := strings.Repeat("a", 64)
				newBody := "#!/bin/sh\nexit 7\n"
				if mode == "healthy" {
					newBody = "#!/bin/sh\nprintf '" + nonce + "' > '" + filepath.Join(dir, "ready") + "'\nsleep 1\n"
				}
				if mode == "timeout" {
					newBody = "#!/bin/sh\nexec sleep 5\n"
				}
				candidate := []byte(newBody)
				for name, b := range map[string][]byte{filepath.Join(root, targetName): old, filepath.Join(dir, "installer.exe"): old, filepath.Join(dir, "candidate.exe"): candidate} {
					if err = os.WriteFile(name, b, 0700); err != nil {
						t.Fatal(err)
					}
				}
				plan := updatePlan{Target: targetName, WorkingDir: root, OriginalSHA256: updateHash(old), CandidateSHA256: updateHash(candidate), Version: "1.17.60", ParentPID: os.Getpid(), Nonce: nonce}
				if modern {
					plan.TargetDir = root
				}
				if variant == "settings-versioned" {
					plan.LaunchTarget = updateOfficialExecutableName(plan.Version)
				}
				b, _ := json.Marshal(plan)
				os.WriteFile(filepath.Join(dir, "plan.json"), b, 0600)
				ops := updateInstallOperations{
					executable: func() (string, error) { return filepath.Join(dir, "installer.exe"), nil },
					waitParent: func(int, time.Duration, string) error {
						if mode == "wait-failed" {
							return os.ErrPermission
						}
						return nil
					},
					replace: func(target, next, backup string) error {
						if modern && (filepath.Dir(target) != filepath.Dir(next) || filepath.Dir(target) != filepath.Dir(backup)) {
							t.Fatal("cross-volume replacement participants")
						}
						if err := os.Rename(target, backup); err != nil {
							return err
						}
						return os.Rename(next, target)
					},
					restoreMissing: os.Rename, startupTimeout: 500 * time.Millisecond,
				}
				if mode == "changed" {
					os.WriteFile(filepath.Join(dir, "candidate.exe"), []byte("tampered"), 0700)
				}
				err = applyUpdatePlanWithOperations(dir, ops)
				result, _ := os.ReadFile(filepath.Join(dir, "update-result.txt"))
				if mode == "healthy" {
					if err != nil || !strings.Contains(string(result), "confirmed the application interface") {
						t.Fatalf("healthy: %v %s", err, result)
					}
					if variant == "settings-versioned" {
						if _, err := os.Stat(filepath.Join(root, plan.LaunchTarget)); err != nil {
							t.Fatal("canonical missing", err)
						}
						if _, err := os.Stat(filepath.Join(root, targetName)); !os.IsNotExist(err) {
							t.Fatal("old filename retained")
						}
					}
					previous, _ := os.ReadFile(filepath.Join(dir, "previous.exe"))
					if string(previous) != string(old) {
						t.Fatal("backup changed")
					}
				} else {
					if err == nil {
						t.Fatal("failure accepted")
					}
					installed, _ := os.ReadFile(filepath.Join(root, targetName))
					if string(installed) != string(old) {
						t.Fatal("original not preserved/restored")
					}
					if mode == "exit" || mode == "timeout" {
						deadline := time.Now().Add(time.Second)
						for time.Now().Before(deadline) {
							if _, err = os.Stat(launched); err == nil {
								break
							}
							time.Sleep(10 * time.Millisecond)
						}
						if _, err = os.Stat(launched); err != nil {
							t.Fatal("rollback did not restart original")
						}
						if _, err = os.Stat(filepath.Join(dir, "failed.exe")); err != nil {
							t.Fatal("failed binary not retained")
						}
					}
				}
				if len(result) == 0 {
					t.Fatal("no diagnostic outcome")
				}
				if _, err = os.Stat(filepath.Join(dir, "update-progress.txt")); err != nil {
					t.Fatal("no progress log")
				}
			})
		}
	}

}
