//go:build windows

package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"networktoolkit/internal/terminal"
)

// Owner request after v1.21.83: a standard Network Toolkit opens an
// administrator tab through Windows' approval and the private helper, with no
// restart; one that runs as administrator opens it directly.
func TestAdminShellUsesTheHelperOnlyWhenNotElevated(t *testing.T) {
	savedElevated, savedLaunch := adminShellProcessElevated, terminalShellLaunch
	t.Cleanup(func() { adminShellProcessElevated, terminalShellLaunch = savedElevated, savedLaunch })
	launched := 0
	terminalShellLaunch = func(string, uint32) (syscall.Handle, error) { launched++; return 0, syscall.Errno(1223) }
	adminShellProcessElevated = func() bool { return false }
	if _, err := openAdministratorShell(context.Background(), terminal.LocalShellOptions{Type: "cmd", Cols: 80, Rows: 24}, func([]byte) {}); err == nil || launched != 1 {
		t.Fatalf("a standard process did not ask Windows for the helper: %v (%d)", err, launched)
	}
	adminShellProcessElevated = func() bool { return true }
	if conn, err := openAdministratorShell(context.Background(), terminal.LocalShellOptions{Type: "cmd", Cols: 80, Rows: 24}, func([]byte) {}); err == nil {
		conn.Close()
	}
	if launched != 1 {
		t.Fatal("an administrator process started the helper")
	}
}

// The waiting copy holds the old process, says it is ready, and opens only
// once that process has exited; a cancel ends it without opening anything.
func TestAdminRestartWaitsForTheOldCopy(t *testing.T) {
	dir := filepath.Join(t.TempDir(), adminReopenPrefix+"1")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	old := exec.Command("powershell.exe", "-NoProfile", "-Command", "Start-Sleep -Milliseconds 1500")
	if err := old.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- waitAdminParent(old.Process.Pid, 30*time.Second, dir) }()
	go old.Wait()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("wait: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the waiting copy did not open after the old one exited")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "ready")); string(b) != "ready" {
		t.Fatalf("ready marker: %q", b)
	}
	_ = os.Remove(filepath.Join(dir, "ready"))

	// Cancelled: this test process never exits, so only the cancel ends it.
	go func() { done <- waitAdminParent(os.Getpid(), 30*time.Second, dir) }()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(dir, "ready")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no ready marker")
		}
		time.Sleep(20 * time.Millisecond)
	}
	adminRestartCancelDir(dir)
	select {
	case err := <-done:
		if !errors.Is(err, errAdminReopenCancelled) {
			t.Fatalf("cancel: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("a cancelled restart kept waiting")
	}
}

// The restart asks Windows for approval with the run's own switches behind the
// restart switch, visibly, and reports a declined approval without changing anything.
func TestAdminRestartLaunch(t *testing.T) {
	saved, savedElevated := adminRestartLaunch, adminRestartElevated
	t.Cleanup(func() {
		adminRestartLaunch = saved
		adminRestartElevated = savedElevated
		adminRestartState = adminRestart{}
	})
	adminRestartElevated = func() bool { return false } // hosted runners run as administrator
	data, err := filepath.EvalSymlinks(t.TempDir())     // the runner's TEMP is a short (8.3) name
	if err != nil {
		t.Fatal(err)
	}
	a := &application{windowMode: "main", dataPath: filepath.Join(data, "Network_Toolkit.settings.json")}
	// The test binary lives under the runner's short-named TEMP too.
	adminRestartExecutable = func() (string, error) { return filepath.Join(data, "Network_Toolkit.exe"), nil }
	adminRestartLaunch = func(string, []string, string) (syscall.Handle, error) { return 0, errAdminRestartDeclined }
	if err := a.startAdminRestart(); !errors.Is(err, errAdminRestartDeclined) {
		t.Fatalf("declined: %v", err)
	}
	var folder string
	adminRestartLaunch = func(exe string, args []string, cwd string) (syscall.Handle, error) {
		if len(args) < 3 || args[0] != adminReopenFlag || args[1] != strconv.Itoa(os.Getpid()) {
			t.Fatalf("restart switches: %v", args)
		}
		folder = args[2]
		h, err := syscall.OpenProcess(0x00100000, false, uint32(os.Getpid()))
		if err != nil {
			return 0, err
		}
		go func() { _ = waitAdminParent(os.Getpid(), time.Minute, folder) }()
		return h, nil
	}
	if err := a.startAdminRestart(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if adminRestartState.dir != folder {
		t.Fatalf("pending folder %q, launched %q", adminRestartState.dir, folder)
	}
	adminRestartState.cancel()
	if _, err := os.Stat(filepath.Join(folder, "cancel")); err != nil {
		t.Fatalf("cancel marker: %v", err)
	}
}
