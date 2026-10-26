//go:build windows

package main

import (
	"os/exec"
	"syscall"
	"testing"
)

func TestDetachedTerminalViewWindowsStdinAndWindow(t *testing.T) {
	cmd := exec.Command("fixture-only.exe", "--terminal-view")
	configureTerminalViewChild(cmd)
	attr := cmd.SysProcAttr
	if attr == nil || attr.NoInheritHandles || attr.HideWindow {
		t.Fatal("the designated stdin bootstrap pipe or native window is suppressed")
	}
	if attr.CreationFlags&0x08000000 == 0 || attr.CreationFlags&syscall.CREATE_NEW_PROCESS_GROUP == 0 {
		t.Fatal("the child inherits console lifetime signals")
	}
	if attr.CmdLine != "" || attr.ParentProcess != 0 || len(attr.AdditionalInheritedHandles) != 0 {
		t.Fatal("the launcher changed command quoting, parent identity or designated handle ownership")
	}
}
