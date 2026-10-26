//go:build linux

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestTerminalChildHasIndependentLinuxSession(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestTerminalChildLinuxSessionHelper$")
	cmd.Env = append(os.Environ(), "NETWORK_TOOLKIT_SESSION_HELPER=1")
	configureTerminalChild(cmd)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the detached QA child did not own its session: %s %v", output, err)
	}
}

func TestTerminalChildLinuxSessionHelper(t *testing.T) {
	if os.Getenv("NETWORK_TOOLKIT_SESSION_HELPER") != "1" {
		return
	}
	session, _, errno := syscall.Syscall(syscall.SYS_GETSID, 0, 0, 0)
	if errno != 0 || int(session) != os.Getpid() || syscall.Getpgrp() != os.Getpid() {
		fmt.Fprintf(os.Stderr, "pid=%d session=%d group=%d error=%v\n", os.Getpid(), session, syscall.Getpgrp(), errno)
		os.Exit(2)
	}
	os.Exit(0)
}
