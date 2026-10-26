//go:build windows || linux

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestTerminalWindowLockExclusiveAndReusable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Terminal.instance.lock")
	if err := os.WriteFile(path, []byte("existing lock file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	release, err := acquireTerminalWindowLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	other, err := acquireTerminalWindowLock(path)
	if other != nil || !errors.Is(err, ErrTerminalWindowInUse) {
		if other != nil {
			other()
		}
		t.Fatalf("a second writer acquired the same directory: release=%v err=%v", other != nil, err)
	}
	separate, err := acquireTerminalWindowLock(filepath.Join(t.TempDir(), "Terminal.instance.lock"))
	if err != nil {
		t.Fatalf("an unrelated Terminal directory was blocked: %v", err)
	}
	separate()
	release()
	release() // Release is safe during repeated shutdown paths.
	reopened, err := acquireTerminalWindowLock(path)
	if err != nil {
		t.Fatalf("the persistent lock file prevented reopening: %v", err)
	}
	reopened()
	if data, err := os.ReadFile(path); err != nil || string(data) != "existing lock file\n" {
		t.Fatalf("lock acquisition removed or rewrote the lock file: %q %v", data, err)
	}
}

func TestTerminalWindowLockRejectsNonRegularPaths(t *testing.T) {
	if release, err := acquireTerminalWindowLock(t.TempDir()); err == nil || release != nil {
		if release != nil {
			release()
		}
		t.Fatal("a directory was accepted as the instance lock")
	}
}

func TestTerminalWindowLockReleasedAfterProcessExit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Terminal.instance.lock")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestTerminalWindowLockHelperProcess$")
	cmd.Env = append(os.Environ(), "NETWORK_TOOLKIT_LOCK_HELPER=hold", "NETWORK_TOOLKIT_LOCK_HELPER_PATH="+path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Wait()
	defer cmd.Process.Kill()
	ready := make(chan error, 1)
	go func() {
		data := make([]byte, len("ready\n"))
		_, err := io.ReadFull(stdout, data)
		if err == nil && string(data) != "ready\n" {
			err = fmt.Errorf("unexpected helper output: %q", data)
		}
		ready <- err
	}()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the lock helper did not start")
	}
	if release, err := acquireTerminalWindowLock(path); !errors.Is(err, ErrTerminalWindowInUse) || release != nil {
		if release != nil {
			release()
		}
		t.Fatalf("the second process did not respect ownership: %v", err)
	}
	// Terminate the fixture without executing its release closure. The OS must
	// release ownership; the persistent file must not require manual removal.
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("the fixture unexpectedly exited normally")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		release, err := acquireTerminalWindowLock(path)
		if err == nil {
			release()
			break
		}
		if !errors.Is(err, ErrTerminalWindowInUse) || time.Now().After(deadline) {
			t.Fatalf("an exited process retained the lock: %v; helper: %s", err, stderr.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestTerminalWindowLockHelperProcess(t *testing.T) {
	if os.Getenv("NETWORK_TOOLKIT_LOCK_HELPER") != "hold" {
		return
	}
	release, err := acquireTerminalWindowLock(os.Getenv("NETWORK_TOOLKIT_LOCK_HELPER_PATH"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer release() // Retain ownership; a forced exit deliberately skips this.
	_, _ = fmt.Fprint(os.Stdout, "ready\n")
	// The parent deliberately ends this fixture to verify crash-safe locks.
	for {
		time.Sleep(time.Second)
	}
}
