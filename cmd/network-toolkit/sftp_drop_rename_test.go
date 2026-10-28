package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSFTPDropRenameDoesNotOverwrite(t *testing.T) {
	root := t.TempDir()
	src, dst := filepath.Join(root, "stage"), filepath.Join(root, "target")
	os.WriteFile(src, []byte("new"), 0600)
	os.WriteFile(dst, []byte("old"), 0600)
	if err := renameSFTPFileNoReplace(src, dst); err == nil {
		t.Fatal("replaced destination")
	}
	for name, want := range map[string]string{src: "new", dst: "old"} {
		b, e := os.ReadFile(name)
		if e != nil || string(b) != want {
			t.Fatalf("changed %s: %s %v", name, b, e)
		}
	}
	os.Remove(dst)
	if e := renameSFTPFileNoReplace(src, dst); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(src); !os.IsNotExist(e) {
		t.Fatal("source stage not cleaned")
	}
	if e := renameSFTPFileNoReplace(filepath.Join(root, "missing"), filepath.Join(root, "not-created")); e == nil {
		t.Fatal("missing source accepted")
	}
}
