//go:build windows

package main

import (
	"context"
	"errors"
	"networktoolkit/internal/terminal"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// No real UAC is invoked: the launcher seam models Cancel and delayed consent.
func TestTerminalShellExplicitApprovalAndLateCancellation(t *testing.T) {
	original := terminalShellLaunch
	t.Cleanup(func() { terminalShellLaunch = original })
	var calls atomic.Int32
	terminalShellLaunch = func(string, uint32) (syscall.Handle, error) { calls.Add(1); return 0, syscall.Errno(1223) }
	o := terminal.LocalShellOptions{Type: "cmd", WorkingDirectory: t.TempDir(), Cols: 80, Rows: 24}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := openElevatedTerminalShell(cancelled, o, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatal("cancelled request launched")
	}
	bad := o
	bad.Type = "arbitrary.exe"
	if _, err := openElevatedTerminalShell(context.Background(), bad, nil); err == nil {
		t.Fatal("invalid shell accepted")
	}
	bad = o
	bad.WorkingDirectory += "\\missing"
	if _, err := openElevatedTerminalShell(context.Background(), bad, nil); err == nil {
		t.Fatal("invalid folder accepted")
	}
	if calls.Load() != 0 {
		t.Fatal("invalid request launched")
	}
	if _, err := openElevatedTerminalShell(context.Background(), o, nil); err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatal("explicit Cancel automatically retried")
	}
	// Windows hands a freed handle value to the next handle the process opens,
	// and the Go runtime opens thread and timer handles at any moment, so
	// polling the old value cannot tell "closed" from "closed and reused". The
	// fake keeps its own duplicate of the event and the check below compares
	// kernel objects instead of handle values.
	if localSystemDirectoryError != nil {
		t.Fatal(localSystemDirectoryError)
	}
	compare := syscall.NewLazyDLL(filepath.Join(localSystemDirectory, "kernelbase.dll")).NewProc("CompareObjectHandles")
	if err := compare.Find(); err != nil {
		t.Fatalf("CompareObjectHandles (Windows 10 1607 or later) is unavailable: %v", err)
	}
	sameObject := func(a, b syscall.Handle) bool {
		same, _, _ := compare.Call(uintptr(a), uintptr(b))
		return same != 0
	}
	type lateLaunch struct {
		h, dup syscall.Handle
		err    error
	}
	entered, release, launched := make(chan struct{}), make(chan struct{}), make(chan lateLaunch, 1)
	terminalShellLaunch = func(id string, parent uint32) (syscall.Handle, error) {
		if !validLocalHelperID(id) || parent != uint32(os.Getpid()) {
			t.Error("bad private identity")
		}
		close(entered)
		<-release
		h, e := localEvent()
		l := lateLaunch{h: h, err: e}
		if e == nil {
			// Duplicate before production code receives h, while h is certainly open.
			self, _ := syscall.GetCurrentProcess()
			l.err = syscall.DuplicateHandle(self, h, self, &l.dup, 0, false, syscall.DUPLICATE_SAME_ACCESS)
			if l.err == nil && !sameObject(h, l.dup) {
				l.err = errors.New("CompareObjectHandles did not match the open event with its duplicate")
			}
		}
		launched <- l
		return h, e
	}
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := openElevatedTerminalShell(ctx, o, nil); done <- err }()
	<-entered
	stop()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	close(release)
	l := <-launched
	if l.dup != 0 {
		defer syscall.CloseHandle(l.dup)
	}
	if l.err != nil || l.h == 0 || l.h == syscall.InvalidHandle || l.dup == 0 {
		t.Fatal("synthetic late-launch handle could not be created", l.err)
	}
	// While production code keeps l.h open it still names the event. Once it
	// closes l.h, the value is invalid or names a different, reused object;
	// both mean the late-launch handle was closed.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !sameObject(l.h, l.dup) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("late-launch handle leaked")
}

// On an already elevated isolated runner, this exercises the actual same-image
// pipe/ConPTY helper without a consent dialog. It never bypasses Windows UAC;
// unelevated runners skip this high-token case while running the Cancel spy.
func TestTerminalShellNativeHelper(t *testing.T) {
	if id := os.Getenv("NTK_SYNTHETIC_SHELL117_HELPER"); id != "" {
		parent, _ := strconv.ParseUint(os.Getenv("NTK_SYNTHETIC_SHELL117_PARENT"), 10, 32)
		if err := runTerminalShellHelper(id, uint32(parent)); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	if !localElevated() {
		t.Skip("actual high-token helper requires an already elevated isolated runner; no UAC initiated")
	}
	original := terminalShellLaunch
	t.Cleanup(func() { terminalShellLaunch = original })
	terminalShellLaunch = func(id string, parent uint32) (syscall.Handle, error) {
		exe, err := os.Executable()
		if err != nil {
			return 0, err
		}
		cmd := exec.Command(exe, "-test.run=^TestTerminalShellNativeHelper$")
		cmd.Env = append(os.Environ(), "NTK_SYNTHETIC_SHELL117_HELPER="+id, "NTK_SYNTHETIC_SHELL117_PARENT="+strconv.FormatUint(uint64(parent), 10))
		if err = cmd.Start(); err != nil {
			return 0, err
		}
		h, err := syscall.OpenProcess(0x1000|syscall.SYNCHRONIZE, false, uint32(cmd.Process.Pid))
		go cmd.Wait()
		return h, err
	}
	for _, kind := range []string{"cmd", "powershell"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			dir := t.TempDir()
			canonical, err := filepath.EvalSymlinks(dir)
			if err != nil {
				t.Fatal(err)
			}
			var mu sync.Mutex
			var output strings.Builder
			tr, err := openElevatedTerminalShell(ctx, terminal.LocalShellOptions{Type: kind, WorkingDirectory: dir, Cols: 500, Rows: 25}, func(b []byte) {
				mu.Lock()
				defer mu.Unlock()
				if output.Len() < 128<<10 {
					output.Write(b)
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			defer tr.Close()
			if !tr.LocalShellState().Elevated || tr.LocalShellState().WorkingDirectory != canonical {
				t.Fatal(tr.LocalShellState())
			}
			control := regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07]*(?:\x07|\x1b\\))`)
			readOutput := func() string { mu.Lock(); defer mu.Unlock(); return control.ReplaceAllString(output.String(), "") }
			for !strings.Contains(readOutput(), ">") {
				select {
				case <-ctx.Done():
					t.Fatalf("initial helper prompt absent; requested=%q canonical=%q state=%+v output=%q", dir, canonical, tr.LocalShellState(), readOutput())
				case <-time.After(20 * time.Millisecond):
				}
			}
			mu.Lock()
			from := output.Len()
			mu.Unlock()
			command := "echo \"NTK_SHELL117_CWD=%CD%\"\r"
			if kind == "powershell" {
				command = "Write-Output ('NTK_SHELL117_CWD='+(Get-Location).Path)\r"
			}
			if _, err = tr.Write([]byte(command)); err != nil {
				t.Fatal(err)
			}
			for {
				mu.Lock()
				found := strings.Contains(control.ReplaceAllString(output.String()[from:], ""), "NTK_SHELL117_CWD="+canonical)
				mu.Unlock()
				if found {
					break
				}
				select {
				case <-ctx.Done():
					t.Fatalf("synthetic cwd output absent; requested=%q canonical=%q state=%+v output=%q", dir, canonical, tr.LocalShellState(), readOutput())
				case <-time.After(20 * time.Millisecond):
				}
			}
			if err = tr.Resize(100, 30); err != nil {
				t.Fatal(err)
			}
			tr.Close()
			select {
			case <-tr.Done():
			case <-time.After(8 * time.Second):
				t.Fatal("helper did not cancel")
			}
			t.Logf("shell=%s parentElevated=%t childElevated=%t cwdConfirmed=true UACInvoked=false", kind, localElevated(), tr.LocalShellState().Elevated)
		})
	}
}
