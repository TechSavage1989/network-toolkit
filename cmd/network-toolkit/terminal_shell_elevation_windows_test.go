//go:build windows

package main

import (
	"context"
	"errors"
	"networktoolkit/internal/terminal"
	"os"
	"os/exec"
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
	entered, release, closed := make(chan struct{}), make(chan struct{}), make(chan syscall.Handle, 1)
	terminalShellLaunch = func(id string, parent uint32) (syscall.Handle, error) {
		if !validLocalHelperID(id) || parent != uint32(os.Getpid()) {
			t.Error("bad private identity")
		}
		close(entered)
		<-release
		h, e := localEvent()
		closed <- h
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
	h := <-closed
	if h == 0 || h == syscall.InvalidHandle {
		t.Fatal("synthetic late-launch handle could not be created")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, err := syscall.WaitForSingleObject(h, 0); err != nil {
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
			var mu sync.Mutex
			var output strings.Builder
			tr, err := openElevatedTerminalShell(ctx, terminal.LocalShellOptions{Type: kind, WorkingDirectory: dir, Cols: 120, Rows: 25}, func(b []byte) {
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
			if !tr.LocalShellState().Elevated || tr.LocalShellState().WorkingDirectory != dir {
				t.Fatal(tr.LocalShellState())
			}
			command := "echo NTK_SHELL117_CWD=%CD%\r"
			if kind == "powershell" {
				command = "Write-Output ('NTK_SHELL117_CWD='+(Get-Location).Path)\r"
			}
			if _, err = tr.Write([]byte(command)); err != nil {
				t.Fatal(err)
			}
			for {
				mu.Lock()
				found := strings.Contains(output.String(), "NTK_SHELL117_CWD="+dir)
				mu.Unlock()
				if found {
					break
				}
				select {
				case <-ctx.Done():
					t.Fatal("synthetic cwd output absent")
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
