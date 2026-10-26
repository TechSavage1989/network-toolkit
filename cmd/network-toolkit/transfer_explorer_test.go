package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTransferExplorerFileActions(t *testing.T) {
	root := t.TempDir()
	local := t.TempDir()
	source := filepath.Join(local, "ArubaFirmware")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(source, "switches"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "switches", "config.txt"), []byte("switch-name test"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := transferFileAction(root, "upload", "ArubaFirmware", "", source, false, false); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "ArubaFirmware", "switches", "config.txt")); err != nil || string(got) != "switch-name test" {
		t.Fatalf("recursive upload failed: %s %v", got, err)
	}
	if _, err := transferFileAction(root, "copy", "ArubaFirmware", "Backup", "", false, false); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "Backup", "switches", "config.txt")); err != nil || string(got) != "switch-name test" {
		t.Fatalf("server-side folder copy failed: %s %v", got, err)
	}
	if _, err := transferFileAction(root, "copy", "Backup", "Backup/copy", "", false, false); err == nil || !strings.Contains(err.Error(), "itself") {
		t.Fatalf("copy into itself must be blocked: %v", err)
	}
	dest := filepath.Join(t.TempDir(), "LocalCopy")
	if _, err := transferFileAction(root, "download", "Backup", "", dest, false, false); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dest, "switches", "config.txt")); err != nil || string(got) != "switch-name test" {
		t.Fatalf("recursive download failed: %s %v", got, err)
	}
	if _, err := transferFileAction(root, "upload", "Backup", "", source, false, false); err == nil {
		t.Fatal("merge without overwrite should be rejected")
	}
	if _, err := transferFileAction(root, "copy", "../outside", "Oops", "", false, false); err == nil {
		t.Fatal("root traversal accepted")
	}
}
func TestTransferExplorerDoesNotFollowLinks(t *testing.T) {
	local := t.TempDir()
	root := t.TempDir()
	src := filepath.Join(local, "tree")
	if err := os.Mkdir(src, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, filepath.Join(src, "escape")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := transferFileAction(root, "upload", "tree", "", src, false, false); err == nil {
		t.Fatal("link traversal accepted")
	}
}
