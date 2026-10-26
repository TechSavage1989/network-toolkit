//go:build windows

package main

import (
	"os/exec"
	"syscall"
	"testing"
)

func TestTerminalChildWindowsWindowAndHandleIsolation(t *testing.T) {
	cmd := exec.Command("fixture-only.exe", "--terminal")
	configureTerminalChild(cmd)
	attr := cmd.SysProcAttr
	if attr == nil || !attr.NoInheritHandles || attr.HideWindow {
		t.Fatal("the native Terminal UI would be hidden or inherit the main app's handles")
	}
	if attr.CreationFlags&0x08000000 == 0 || attr.CreationFlags&syscall.CREATE_NEW_PROCESS_GROUP == 0 {
		t.Fatal("the child would inherit a development console's lifetime signals")
	}
	if attr.CmdLine != "" || attr.ParentProcess != 0 || len(attr.AdditionalInheritedHandles) != 0 {
		t.Fatal("the launcher changed command quoting, parent identity or handle ownership")
	}
}
