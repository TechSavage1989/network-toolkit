//go:build windows

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

// This test is compiled in cloud checks and executes only on Windows. It uses
// isolated temporary shortcut roots, never the person's actual Desktop/Menu.
func TestUpdateShortcutNativeShellRoundTrip(t *testing.T) {
	for _, iconKind := range []string{"implicit", "old-executable", "custom"} {
		t.Run(iconKind, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			r, _, _ := updateShortcutOLE.NewProc("CoInitializeEx").Call(0, 2|4)
			if int32(r) < 0 {
				t.Fatal(updateShortcutHRESULT("Initialize native shortcut test", r))
			}
			defer updateShortcutOLE.NewProc("CoUninitialize").Call()
			e := newShortcutTestEnvironment(t)
			if err := os.WriteFile(e.old, []byte("previous verified executable"), 0600); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(e.desktop, "Toolkit.lnk")
			iconPath := ""
			if iconKind == "old-executable" {
				iconPath = e.old
			} else if iconKind == "custom" {
				iconPath = filepath.Join(filepath.Dir(e.old), "custom.ico")
			}
			func() {
				link, persist, err := updateShortcutCreate()
				if err != nil {
					t.Fatal(err)
				}
				defer link.call(2)
				defer persist.call(2)
				set := func(method int, value string) {
					p, err := syscall.UTF16PtrFromString(value)
					if err != nil {
						t.Fatal(err)
					}
					if err := updateShortcutHRESULT("Set test shortcut field", link.call(method, uintptr(unsafe.Pointer(p)))); err != nil {
						t.Fatal(err)
					}
					runtime.KeepAlive(p)
				}
				set(20, e.old)
				set(7, "Toolkit customized description")
				set(9, filepath.Dir(e.old))
				set(11, `--profile "my profile" --flag`)
				if err := updateShortcutHRESULT("Set test show mode", link.call(15, 3)); err != nil {
					t.Fatal(err)
				}
				if err := updateShortcutHRESULT("Set test hotkey", link.call(13, 0x241)); err != nil {
					t.Fatal(err)
				}
				if iconPath != "" {
					icon, _ := syscall.UTF16PtrFromString(iconPath)
					if err := updateShortcutHRESULT("Set test icon", link.call(17, uintptr(unsafe.Pointer(icon)), 7)); err != nil {
						t.Fatal(err)
					}
					runtime.KeepAlive(icon)
				}
				file, _ := syscall.UTF16PtrFromString(path)
				if err := updateShortcutHRESULT("Save test shortcut", persist.call(6, uintptr(unsafe.Pointer(file)), 0)); err != nil {
					t.Fatal(err)
				}
				runtime.KeepAlive(file)
			}()
			before, identity, err := updateShortcutSnapshot(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := updateMigrateShortcutsInRoots([]string{e.old}, e.stable, e.settings, []updateShortcutRoot{{e.desktop, 0}}, updateShortcutOps{updateShortcutStageNative, updateShortcutCommitNative}); err != nil {
				t.Fatal(err)
			}
			after, afterIdentity, err := updateShortcutSnapshot(path)
			if err != nil || !os.SameFile(identity, afterIdentity) {
				t.Fatalf("shortcut identity changed: %v", err)
			}
			if err := updateShortcutPreservesSettingsForTarget(before, after, []string{e.old}, e.stable); err != nil {
				t.Fatal(err)
			}
			link, persist, err := updateShortcutLoad(path)
			if err != nil {
				t.Fatal(err)
			}
			defer link.call(2)
			defer persist.call(2)
			target, err := updateShortcutRawTarget(link)
			if err != nil || !updateNamingSamePath(target, e.stable) {
				t.Fatalf("target was not migrated: %q %v", target, err)
			}
			settings, err := updateShortcutParseSettings(after)
			if err != nil {
				t.Fatal(err)
			}
			wantIcon := iconPath
			if iconKind == "old-executable" {
				wantIcon = e.stable
			}
			if actualIcon := updateShortcutString(settings.strings[4], settings.flags&0x80 != 0); actualIcon != wantIcon {
				t.Fatalf("icon got %q, want %q", actualIcon, wantIcon)
			}
		})
	}
}

func TestUpdateShortcutNativeCommitPreservesFileIdentity(t *testing.T) {
	e := newShortcutTestEnvironment(t)
	path := filepath.Join(e.desktop, "Toolkit.lnk")
	e.write(t, path)
	before, identity, err := updateShortcutSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := updateShortcutCommitNative(path, identity, before, e.replacement); err != nil {
		t.Fatal(err)
	}
	after, afterIdentity, err := updateShortcutSnapshot(path)
	if err != nil || !bytes.Equal(after, e.replacement) || !os.SameFile(identity, afterIdentity) {
		t.Fatalf("native write did not preserve identity or bytes: %v", err)
	}
}

func TestUpdateShortcutNativeCommitRejectsHardLinksAndStaleData(t *testing.T) {
	for _, hardLink := range []bool{true, false} {
		e := newShortcutTestEnvironment(t)
		path := filepath.Join(e.desktop, "Toolkit.lnk")
		e.write(t, path)
		before, identity, err := updateShortcutSnapshot(path)
		if err != nil {
			t.Fatal(err)
		}
		want := e.original
		if hardLink {
			if err := os.Link(path, filepath.Join(e.settings, "outside-scope.lnk")); err != nil {
				t.Skipf("hard links unavailable: %v", err)
			}
		} else {
			want = []byte("edited after inspection")
			if err := os.WriteFile(path, want, 0600); err != nil {
				t.Fatal(err)
			}
		}
		if err := updateShortcutCommitNative(path, identity, before, e.replacement); err == nil {
			t.Fatal("native commit did not reject unsafe shortcut")
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(after, want) {
			t.Fatal("native commit modified unsafe shortcut")
		}
	}
}
