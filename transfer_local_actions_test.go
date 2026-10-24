package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalExplorerActionsAndSecurity(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(src, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := localExplorerAction("copy", src, filepath.Join(root, "copy.txt"), false, false); err != nil {
		t.Fatal(err)
	}
	if err := localExplorerAction("rename", filepath.Join(root, "copy.txt"), filepath.Join(root, "moved.txt"), false, false); err != nil {
		t.Fatal(err)
	}
	if err := localExplorerAction("delete", filepath.Join(root, "moved.txt"), "", false, false); err == nil {
		t.Fatal("local deletion without confirmation accepted")
	}
	if err := localExplorerAction("delete", filepath.Join(root, "moved.txt"), "", false, true); err != nil {
		t.Fatal(err)
	}
	if err := localExplorerAction("mkdir", filepath.Join(root, "newfolder"), "", false, false); err != nil {
		t.Fatal(err)
	}
	if err := localExplorerAction("copy", root, filepath.Join(root, "nested"), false, false); err == nil {
		t.Fatal("local self-copy accepted")
	}
	if err := localExplorerAction("delete", string(filepath.Separator), "", false, true); err == nil {
		t.Fatal("filesystem root deletion accepted")
	}
	app := testApplication(t)
	if r := requestApp(app, http.MethodPost, "/api/transfer/local-files", `{"action":"delete","path":"`+strings.ReplaceAll(src, `\`, `\\`)+`","confirm":true}`, false); r.Code != http.StatusUnauthorized {
		t.Fatalf("unauthed local delete: %d", r.Code)
	}
	if r := requestApp(app, http.MethodGet, "/api/transfer/local-files", "", true); r.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET local actions allowed: %d", r.Code)
	}
}
