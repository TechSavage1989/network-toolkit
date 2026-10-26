package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStableNameNoSidecars(t *testing.T) {
	dir := t.TempDir()
	root, _ := updateStorageRoot(t.TempDir(), dir)
	old := filepath.Join(dir, updateOfficialExecutableName("1.17.62"))
	namingTestWrite(t, old, updateTestPE("1.17.62"))
	status, err := updateStablePromote(root, old, "1.17.62", nil)
	if err != nil {
		t.Fatal(err)
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 1 || files[0].Name() != updateStableExecutableName {
		t.Fatalf("unexpected files: %v", files)
	}
	if status.CanonicalPath != filepath.Join(dir, updateStableExecutableName) {
		t.Fatal(status)
	}
	if _, err = os.Stat(filepath.Join(root, "stable-name-migration.json")); err != nil {
		t.Fatal(err)
	}
}
func TestStableNameCollisionAndCustomPreserved(t *testing.T) {
	for _, custom := range []bool{false, true} {
		dir := t.TempDir()
		root, _ := updateStorageRoot(t.TempDir(), dir)
		name := updateOfficialExecutableName("1.17.62")
		if custom {
			name = "MyTool.exe"
		}
		old := filepath.Join(dir, name)
		namingTestWrite(t, old, updateTestPE("1.17.62"))
		canonical := filepath.Join(dir, updateStableExecutableName)
		namingTestWrite(t, canonical, []byte("unrelated"))
		status, err := updateStablePromote(root, old, "1.17.62", nil)
		if custom && (err != nil || status.Managed) {
			t.Fatal(status, err)
		}
		if !custom && err == nil {
			t.Fatal("collision accepted")
		}
		if string(namingTestBytes(t, canonical)) != "unrelated" {
			t.Fatal("collision overwritten")
		}
		if _, err = os.Stat(old); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStablePendingMigrationSurvivesVerifiedUpgrade(t *testing.T) {
	dir := t.TempDir()
	root, _ := updateStorageRoot(t.TempDir(), dir)
	old := filepath.Join(dir, updateOfficialExecutableName("1.17.62"))
	namingTestWrite(t, old, updateTestPE("1.17.62"))
	status, err := updateStablePromote(root, old, "1.17.62", nil)
	if err != nil {
		t.Fatal(err)
	}
	c := retentionTestStage(t, root, dir, "1.17.62", "1.17.63")
	namingTestWrite(t, status.CanonicalPath, updateTestPE("1.17.63"))
	if _, err = updateStablePromote(root, status.CanonicalPath, "1.17.63", nil); err == nil {
		t.Fatal("unproved upgrade accepted")
	}
	got, err := updateStablePromote(root, status.CanonicalPath, "1.17.63", []completedUpdate{c})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Compatibility) == 0 {
		t.Fatal("pending shortcut history lost")
	}
}
