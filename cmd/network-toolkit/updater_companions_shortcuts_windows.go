//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

// Setup's Start menu shortcuts of the installed copy are pointed at the
// separate programs once they are placed (updateRetargetCompanionShortcuts
// decides which). The shortcut is changed through the Shell's own IShellLinkW:
// only its target, arguments and icon; everything else it holds stays. A copy
// is saved outside the Start menu, checked, and written over the original
// only while the original still holds the bytes that were read
// (updateShortcutCommitNative, shared with the stable-filename migration).
func updateCompanionShortcutsNative(dir string) error {
	platform, ok := editorRegDefaultPlatform()
	if !ok {
		return nil
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	r, _, _ := updateShortcutOLE.NewProc("CoInitializeEx").Call(0, 2|4)
	if int32(r) < 0 {
		return updateShortcutHRESULT("Prepare the Start menu shortcuts", r)
	}
	defer updateShortcutOLE.NewProc("CoUninitialize").Call()
	return updateRetargetCompanionShortcuts(dir, companionShortcutNativeOps(platform.registry))
}

// FOLDERID_Programs: this user's Start menu Programs folder ($SMPROGRAMS with
// SetShellVarContext current, as Setup uses it).
var companionProgramsFolderID = syscall.GUID{Data1: 0xa77f5d77, Data2: 0x2e2b, Data3: 0x44c3, Data4: [8]byte{0xa6, 0xa2, 0xab, 0xa6, 0x01, 0x05, 0x4a, 0x51}}

func companionShortcutNativeOps(reg editorRegistry) companionShortcutOps {
	return companionShortcutOps{
		installLocation: func() (string, bool, error) {
			value, found, ok, err := reg.String(editorRegUninstall, "InstallLocation")
			return value, found && ok, err
		},
		setupVersion: func() (string, bool, error) {
			value, found, ok, err := reg.String(editorRegUninstall, "DisplayVersion")
			return value, found && ok, err
		},
		programsFolder: func() (string, error) { return updateShortcutKnownFolder(companionProgramsFolderID) },
		exists:         companionShortcutExists,
		read:           companionShortcutReadNative,
		retarget:       companionShortcutRetargetNative,
	}
}

// companionShortcutExists: a regular file; a link or reparse point is never
// followed or changed.
func companionShortcutExists(link string) (bool, error) {
	info, err := os.Lstat(link)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.Mode().IsRegular() && !storageIsLink(info), nil
}

func companionShortcutArguments(link *updateShortcutCOM) (string, error) {
	var buffer [32768]uint16
	// IShellLinkW::GetArguments
	if err := updateShortcutHRESULT("Read shortcut arguments", link.call(10, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))); err != nil {
		return "", err
	}
	return syscall.UTF16ToString(buffer[:]), nil
}

func companionShortcutReadNative(path string) (string, string, error) {
	link, persist, err := updateShortcutLoad(path)
	if err != nil {
		return "", "", err
	}
	defer link.call(2)
	defer persist.call(2)
	target, err := updateShortcutRawTarget(link)
	if err != nil {
		return "", "", err
	}
	args, err := companionShortcutArguments(link)
	return target, args, err
}

func companionShortcutRetargetNative(path, target, args, program string) error {
	if ok, err := companionShortcutExists(program); err != nil || !ok {
		return errors.New(filepath.Base(program) + " is not there")
	}
	original, identity, err := updateShortcutSnapshot(path)
	if err != nil {
		return err
	}
	link, persist, err := updateShortcutLoad(path)
	if err != nil {
		return err
	}
	defer link.call(2)
	defer persist.call(2)
	gotTarget, err := updateShortcutRawTarget(link)
	if err != nil {
		return err
	}
	gotArgs, err := companionShortcutArguments(link)
	if err != nil {
		return err
	}
	if !strings.EqualFold(gotTarget, target) || gotArgs != args {
		return errors.New("the shortcut changed while it was read; left unchanged")
	}
	programW, err := syscall.UTF16PtrFromString(program)
	if err != nil {
		return err
	}
	empty, _ := syscall.UTF16PtrFromString("")
	if err = updateShortcutHRESULT("Set shortcut target", link.call(20, uintptr(unsafe.Pointer(programW)))); err != nil {
		return err
	}
	if err = updateShortcutHRESULT("Set shortcut arguments", link.call(11, uintptr(unsafe.Pointer(empty)))); err != nil {
		return err
	}
	if err = updateShortcutHRESULT("Set shortcut icon", link.call(17, uintptr(unsafe.Pointer(programW)), 0)); err != nil {
		return err
	}
	runtime.KeepAlive(programW)
	runtime.KeepAlive(empty)
	stage, err := os.MkdirTemp("", "network-toolkit-shortcut-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	staged := filepath.Join(stage, "shortcut.lnk")
	stagedW, err := syscall.UTF16PtrFromString(staged)
	if err != nil {
		return err
	}
	// FALSE: Save a Copy As; the original .lnk is not written here.
	if err = updateShortcutHRESULT("Save shortcut copy", persist.call(6, uintptr(unsafe.Pointer(stagedW)), 0)); err != nil {
		return err
	}
	runtime.KeepAlive(stagedW)
	checkTarget, checkArgs, err := companionShortcutReadNative(staged)
	if err != nil {
		return err
	}
	if !strings.EqualFold(checkTarget, program) || checkArgs != "" {
		return errors.New("the changed shortcut could not be verified; left unchanged")
	}
	replacement, err := updateNamingReadFile(staged, updateShortcutLimit)
	if err != nil {
		return err
	}
	return updateShortcutCommitNative(path, identity, original, replacement)
}

// companionLongPath is the long form of an existing path: a folder may be
// written with 8.3 short names (C:\Users\RUNNER~1\...), while a shortcut's
// target reads back in the long form.
func companionLongPath(path string) string { return privateLongPath(path) }
