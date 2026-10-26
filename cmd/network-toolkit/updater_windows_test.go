//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestUpdateWindowsAtomicReplacementAndRollback(t *testing.T) {
	d := t.TempDir()
	target := filepath.Join(d, "app.exe")
	next := filepath.Join(d, "candidate.exe")
	backup := filepath.Join(d, "previous.exe")
	failed := filepath.Join(d, "failed.exe")
	os.WriteFile(target, []byte("old"), 0600)
	os.WriteFile(next, []byte("new"), 0600)
	if e := replaceUpdateExecutable(target, next, backup); e != nil {
		t.Fatal(e)
	}
	a, _ := os.ReadFile(target)
	b, _ := os.ReadFile(backup)
	if string(a) != "new" || string(b) != "old" {
		t.Fatal("replacement lost contents")
	}
	if e := replaceUpdateExecutable(target, backup, failed); e != nil {
		t.Fatal(e)
	}
	a, _ = os.ReadFile(target)
	b, _ = os.ReadFile(failed)
	if string(a) != "old" || string(b) != "new" {
		t.Fatal("rollback lost contents")
	}
}

func TestUpdateGUIAndHelperLaunchFlagsAreSeparate(t *testing.T) {
	helper := exec.Command("installer.exe")
	configureUpdateHelperProcess(helper)
	if helper.SysProcAttr == nil || !helper.SysProcAttr.HideWindow {
		t.Fatal("helper must stay hidden")
	}
	app := exec.Command("app.exe")
	configureUpdateAppProcess(app)
	if app.SysProcAttr == nil || app.SysProcAttr.HideWindow {
		t.Fatal("GUI restart must not inherit SW_HIDE")
	}
	if app.SysProcAttr.CreationFlags&syscall.CREATE_NEW_PROCESS_GROUP == 0 {
		t.Fatal("restart is not independent of the helper console group")
	}
}
