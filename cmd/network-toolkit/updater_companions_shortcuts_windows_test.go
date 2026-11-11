//go:build windows

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

// Real .lnk files in a temporary folder that stands for the Start menu (never
// the person's own): Setup's shortcuts are pointed at the separate programs,
// with no arguments and icon 0; others are not touched.
func TestCompanionShortcutsNativeRetarget(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	r, _, _ := updateShortcutOLE.NewProc("CoInitializeEx").Call(0, 2|4)
	if int32(r) < 0 {
		t.Fatal(updateShortcutHRESULT("Initialize native shortcut test", r))
	}
	defer updateShortcutOLE.NewProc("CoUninitialize").Call()
	install, menu := t.TempDir(), t.TempDir()
	folder := filepath.Join(menu, "Network Toolkit")
	if err := os.MkdirAll(folder, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Network_Toolkit.exe", "Network_Toolkit_TextEditor.exe", "Network_Toolkit_Terminal.exe"} {
		if err := os.WriteFile(filepath.Join(install, name), []byte("synthetic program"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	main := filepath.Join(install, "Network_Toolkit.exe")
	create := func(path, target, args, description string, icon int) {
		t.Helper()
		link, persist, err := updateShortcutCreate()
		if err != nil {
			t.Fatal(err)
		}
		defer link.call(2)
		defer persist.call(2)
		set := func(method int, value string) {
			p, _ := syscall.UTF16PtrFromString(value)
			if err := updateShortcutHRESULT("Set test shortcut field", link.call(method, uintptr(unsafe.Pointer(p)))); err != nil {
				t.Fatal(err)
			}
			runtime.KeepAlive(p)
		}
		set(20, target)
		set(11, args)
		set(9, install)
		set(7, description)
		iconPath, _ := syscall.UTF16PtrFromString(target)
		if err := updateShortcutHRESULT("Set test icon", link.call(17, uintptr(unsafe.Pointer(iconPath)), uintptr(icon))); err != nil {
			t.Fatal(err)
		}
		runtime.KeepAlive(iconPath)
		file, _ := syscall.UTF16PtrFromString(path)
		if err := updateShortcutHRESULT("Save test shortcut", persist.call(6, uintptr(unsafe.Pointer(file)), 0)); err != nil {
			t.Fatal(err)
		}
		runtime.KeepAlive(file)
	}
	editor, terminal := filepath.Join(folder, "Network Toolkit Text Editor.lnk"), filepath.Join(folder, "Network Toolkit Terminal.lnk")
	create(editor, main, "--editor-standalone", "Synthetic description", 1)
	create(terminal, main, "--terminal --data-dir D:\\Data", "", 2) // changed by the user
	terminalBefore, err := os.ReadFile(terminal)
	if err != nil {
		t.Fatal(err)
	}
	ops := companionShortcutNativeOps(nil)
	ops.installLocation = func() (string, bool, error) { return install, true, nil }
	ops.setupVersion = func() (string, bool, error) { return "1.21.46", true, nil }
	editorBefore, _ := os.ReadFile(editor)
	if err := updateRetargetCompanionShortcuts(install, ops); err != errCompanionShortcutsNextSetup {
		t.Fatalf("an uninstaller of 1.21.46 would leave the changed shortcuts behind: %v", err)
	}
	if after, _ := os.ReadFile(editor); string(after) != string(editorBefore) {
		t.Fatal("shortcut changed for an older uninstaller")
	}
	ops.setupVersion = func() (string, bool, error) { return companionShortcutUninstallerVersion, true, nil }
	ops.programsFolder = func() (string, error) { return menu, nil }
	if err := updateRetargetCompanionShortcuts(install, ops); err != nil {
		t.Fatal(err)
	}
	target, args, err := companionShortcutReadNative(editor)
	if err != nil || !strings.EqualFold(target, filepath.Join(install, "Network_Toolkit_TextEditor.exe")) || args != "" {
		t.Fatalf("editor shortcut: %q %q %v", target, args, err)
	}
	link, persist, err := updateShortcutLoad(editor)
	if err != nil {
		t.Fatal(err)
	}
	var icon, description [1024]uint16
	var index int32
	if err := updateShortcutHRESULT("Read icon", link.call(16, uintptr(unsafe.Pointer(&icon[0])), uintptr(len(icon)), uintptr(unsafe.Pointer(&index)))); err != nil {
		t.Fatal(err)
	}
	if err := updateShortcutHRESULT("Read description", link.call(6, uintptr(unsafe.Pointer(&description[0])), uintptr(len(description)))); err != nil {
		t.Fatal(err)
	}
	link.call(2)
	persist.call(2)
	if !strings.EqualFold(syscall.UTF16ToString(icon[:]), filepath.Join(install, "Network_Toolkit_TextEditor.exe")) || index != 0 {
		t.Fatalf("editor icon %q,%d", syscall.UTF16ToString(icon[:]), index)
	}
	if syscall.UTF16ToString(description[:]) != "Synthetic description" {
		t.Fatalf("other fields lost: %q", syscall.UTF16ToString(description[:]))
	}
	if after, err := os.ReadFile(terminal); err != nil || string(after) != string(terminalBefore) {
		t.Fatalf("a shortcut the user changed was rewritten: %v", err)
	}
	// A portable copy: nothing changes.
	create(editor, main, "--editor", "", 1)
	editorBefore, _ = os.ReadFile(editor)
	ops.installLocation = func() (string, bool, error) { return "", false, nil }
	if err := updateRetargetCompanionShortcuts(install, ops); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(editor); string(after) != string(editorBefore) {
		t.Fatal("a portable copy changed Setup's shortcut")
	}
}
