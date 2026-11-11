package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// Another program holds the file without sharing it (a running log writer,
// an open document). The editor says so and opens nothing.
func TestEditorExplainsAFileInUse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "in use.log")
	if err := os.WriteFile(path, []byte("held by another program\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(handle)
	a, m, _ := editorFixture(t)
	lease, status, code := openAnyFile(t, a, path)
	if code != 400 || status.Error != "The file is in use by another program and could not be read." {
		t.Fatalf("%d %q", code, status.Error)
	}
	if r := diskCall(a, a.token, lease, map[string]any{"action": "window"}); r.Code == 200 {
		t.Fatal("a file in use returned text")
	}
	m.releaseDocument("", editorFixtureUID, lease)
	m.mu.Lock()
	left := len(m.documents)
	m.mu.Unlock()
	if left != 0 {
		t.Fatal("a failed open left a document behind")
	}
}
