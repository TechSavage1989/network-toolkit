//go:build windows

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Security review 11, item 1, on Windows: the download folder is owner-only,
// the held Setup cannot be replaced, renamed or deleted, the program that
// starts is the verified file, and a junction in the path refuses to start.

func privateTestFolder(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "NetworkToolkit-Setup")
	if err := fullInstallMakePrivateDir(dir); err != nil {
		t.Fatal(err)
	}
	if err := fullInstallMakePrivateDir(dir); !errors.Is(err, os.ErrExist) {
		t.Fatalf("an existing folder is reported as existing: %v", err)
	}
	return dir
}

func TestFullInstallPrivateFolderIsOwnerOnly(t *testing.T) {
	dir := privateTestFolder(t)
	if err := fullInstallCheckPrivateDir(dir); err != nil {
		t.Fatal(err)
	}
	dacl, err := credentialFileDACL(dir)
	if err != nil {
		t.Fatal(err)
	}
	user, _ := credentialCurrentUserSID()
	// SDDL writes some accounts as aliases (the built-in Administrator, the
	// hosted runner's user, is "LA"), so compare the trustee as a SID.
	if !strings.HasPrefix(dacl, "D:P") || strings.Count(dacl, "(A;") != 1 || sddlTrustee(t, dacl) != user {
		t.Fatalf("the download folder's access list is not owner-only for %s: %s", user, dacl)
	}
	// A file written into it inherits the owner-only list.
	file := filepath.Join(dir, "probe.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if others, err := editorRegWritableByOthers(file); err != nil || others != "" {
		t.Fatalf("a file in the folder can be changed by %q (%v)", others, err)
	}
}

func TestFullInstallRefusesAFolderOthersCanChange(t *testing.T) {
	dir := privateTestFolder(t)
	// BUILTIN\Users may modify: exactly what a custom data folder such as
	// D:\Data can inherit.
	if out, err := exec.Command("icacls", dir, "/grant", "*S-1-5-32-545:(OI)(CI)M").CombinedOutput(); err != nil {
		t.Fatalf("icacls: %v %s", err, out)
	}
	others, err := editorRegWritableByOthers(dir)
	if err != nil || !strings.Contains(others, "Users") {
		t.Fatalf("the folder others can change was not recognized: %q %v", others, err)
	}
	if err := fullInstallCheckPrivateDir(dir); err == nil {
		t.Fatal("a folder others can change was accepted")
	}
}

func TestFullInstallRefusesAJunction(t *testing.T) {
	dir := privateTestFolder(t)
	target := filepath.Join(dir, "real")
	if err := fullInstallMakePrivateDir(target); err != nil {
		t.Fatal(err)
	}
	junction := filepath.Join(dir, "junction")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", junction, target).CombinedOutput(); err != nil {
		t.Fatalf("mklink: %v %s", err, out)
	}
	if err := fullInstallCheckPrivateDir(junction); err == nil {
		t.Fatal("a junction was accepted as the download folder")
	}
	if err := os.WriteFile(filepath.Join(target, "Setup.exe"), []byte("MZ"), 0o600); err != nil {
		t.Fatal(err)
	}
	if held, err := fullInstallOpenHeld(filepath.Join(junction, "Setup.exe")); err == nil {
		held.Close()
		t.Fatal("Setup was opened through a junction")
	}
}

func TestFullInstallHeldSetupCannotBeReplacedAndStartsVerified(t *testing.T) {
	dir := privateTestFolder(t)
	sub := filepath.Join(dir, ".network-toolkit-setup-test")
	if err := fullInstallMakePrivateDir(sub); err != nil {
		t.Fatal(err)
	}
	// A harmless program that exits at once stands in for Setup.
	program, err := os.ReadFile(filepath.Join(os.Getenv("SystemRoot"), "System32", "HOSTNAME.EXE"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sub, "Network_Toolkit_9.9.9_Windows_x64_Setup.exe")
	if err := os.WriteFile(path, program, 0o700); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(program)
	download := &fullInstallDownload{Version: "9.9.9", Name: filepath.Base(path), Size: int64(len(program)), SHA256: hex.EncodeToString(sum[:]), path: path, dir: sub}
	oldRoot, oldLaunch := fullInstallRoot, fullInstallLaunch
	t.Cleanup(func() { fullInstallRoot, fullInstallLaunch = oldRoot, oldLaunch })
	fullInstallRoot = func() (string, error) { return dir, fullInstallCheckPrivateDir(dir) }
	var exitCode uint32 = 1234
	launched := 0
	fullInstallLaunch = func(ctx context.Context, p string) error {
		launched++
		// While Setup is held: nobody can write, rename or delete it.
		if f, err := os.OpenFile(p, os.O_WRONLY, 0); err == nil {
			f.Close()
			t.Error("the held Setup could be opened for writing")
		}
		if err := os.Rename(p, p+".moved"); err == nil {
			t.Error("the held Setup could be renamed")
		}
		if err := os.Remove(p); err == nil {
			t.Error("the held Setup could be deleted")
		}
		if err := os.WriteFile(p, []byte("MZ replaced"), 0o700); err == nil {
			t.Error("the held Setup could be replaced")
		}
		// The real start, while the file is held.
		process, err := fullInstallCreateProcess(p)
		if err != nil {
			return err
		}
		defer syscall.CloseHandle(process)
		if event, _ := syscall.WaitForSingleObject(process, uint32((20 * time.Second).Milliseconds())); event != syscall.WAIT_OBJECT_0 {
			return errors.New("the started program did not finish")
		}
		return syscall.GetExitCodeProcess(process, &exitCode)
	}
	if err := fullInstallLaunchVerified(context.Background(), download); err != nil {
		t.Fatal(err)
	}
	if launched != 1 || exitCode != 0 {
		t.Fatalf("launched=%d exit code %d", launched, exitCode)
	}
	// After the start the file is still the verified one.
	after, err := os.ReadFile(path)
	if err != nil || sha256.Sum256(after) != sum {
		t.Fatal("the file changed while it was held")
	}
	// Changed bytes are never started.
	if err := os.WriteFile(path, append(program, 0), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := fullInstallLaunchVerified(context.Background(), download); !errors.Is(err, errFullInstallChanged) || launched != 1 {
		t.Fatalf("a changed Setup: %v launched=%d", err, launched)
	}
}

// Item 4 on Windows: a copy in a folder Users can change is refused.
func TestEditorRegistrationRefusesAFolderOthersCanChangeOnWindows(t *testing.T) {
	dir := privateTestFolder(t)
	exe := filepath.Join(dir, "Network_Toolkit.exe")
	if err := os.WriteFile(exe, []byte("MZ"), 0o700); err != nil {
		t.Fatal(err)
	}
	if others, err := editorRegWritableByOthers(exe); err != nil || others != "" {
		t.Fatalf("a private copy: %q %v", others, err)
	}
	if out, err := exec.Command("icacls", dir, "/grant", "*S-1-5-32-545:(OI)(CI)M").CombinedOutput(); err != nil {
		t.Fatalf("icacls: %v %s", err, out)
	}
	platform, _ := testEditorPlatform(fixtureRegistry())
	platform.access = editorRegWritableByOthers
	status, err := editorRegStatus(platform, exe)
	if err != nil || status.CanRegister || !strings.Contains(status.LocationProblem, "Users") {
		t.Fatalf("%+v %v", status, err)
	}
}

// sddlTrustee returns, as a SID string, the trustee of the only ACE in dacl.
func sddlTrustee(t *testing.T, dacl string) string {
	t.Helper()
	open := strings.Index(dacl, "(A;")
	if open < 0 {
		return ""
	}
	end := strings.Index(dacl[open:], ")")
	if end < 0 {
		return ""
	}
	fields := strings.Split(dacl[open+1:open+end], ";")
	sid, err := syscall.StringToSid(fields[len(fields)-1])
	if err != nil {
		t.Fatalf("trustee %q of %s: %v", fields[len(fields)-1], dacl, err)
	}
	text, err := sid.String()
	if err != nil {
		t.Fatal(err)
	}
	return text
}
