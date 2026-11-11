package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Review finding: a document handed over to the Text Editor must not open a
// second time under another spelling of its path. The engine names one key
// per file: absolute, links resolved, the long form of 8.3 names (Windows
// EvalSymlinks), and case-insensitive on Windows.
func TestEditorCanonicalPathKey(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(file, []byte("hostname lab-switch-01\n"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	a := testApplication(t)
	key := func(path string) string {
		t.Helper()
		raw, _ := json.Marshal(map[string]string{"action": "canonical-path", "path": path})
		r := requestApp(a, "POST", "/api/file-editor/local", string(raw), true)
		var out struct{ Path, Key string }
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &out) != nil || out.Key == "" {
			t.Fatalf("canonical-path %s: %d %s", path, r.Code, r.Body.String())
		}
		return out.Key
	}
	want := key(file)
	for _, other := range []string{link, filepath.Join(dir, ".", "sub", "..", "notes.txt")} {
		if got := key(other); got != want {
			t.Fatalf("%s names %q, want %q", other, got, want)
		}
	}
	if key(filepath.Join(dir, "missing.txt")) == want {
		t.Fatal("another file has the same key")
	}
	if got := editorPathKey("windows", `C:\Lab\Notes.TXT`); got != editorPathKey("windows", `c:\lab\notes.txt`) {
		t.Fatalf("Windows paths differ only in case: %q", got)
	}
	if editorPathKey("linux", "/lab/Notes.txt") == editorPathKey("linux", "/lab/notes.txt") {
		t.Fatal("Linux paths are case-sensitive")
	}
	if r := requestApp(a, "POST", "/api/file-editor/local", `{"action":"canonical-path","path":"relative.txt"}`, true); r.Code != 400 {
		t.Fatalf("a relative path: %d", r.Code)
	}
}
