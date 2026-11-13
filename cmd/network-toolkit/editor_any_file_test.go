package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"networktoolkit/internal/editorbuffer"
)

// The Text Editor may be the Windows default for any file, so every test
// here opens a generated file through the same engine route the page uses.

type anyFileStatus struct {
	Metadata   editorbuffer.Metadata `json:"metadata"`
	Format     editorbuffer.Format   `json:"format"`
	Hash       string                `json:"hash"`
	NewLineEOL string                `json:"newLineEol"`
	Error      string                `json:"error"`
}

// openAnyFile opens path as a local document and waits until the engine has
// read it. A failed open returns its message; lease is 0 when the engine
// refused the request itself.
func openAnyFile(t *testing.T, a *application, path string) (uint64, anyFileStatus, int) {
	t.Helper()
	s := emptyDiskFixture()
	s.Path = path
	s.Source = "Local computer"
	s.Binding = editorBinding{Kind: "local"}
	r := editorHTTP(a, a.token, "/api/editor-views/buffers/open", map[string]any{"document": s})
	var status anyFileStatus
	if r.Code != 202 {
		json.Unmarshal(r.Body.Bytes(), &status)
		return 0, status, r.Code
	}
	var opened struct {
		Lease uint64 `json:"lease"`
	}
	json.Unmarshal(r.Body.Bytes(), &opened)
	deadline := time.Now().Add(20 * time.Second)
	for {
		r = diskCall(a, a.token, opened.Lease, map[string]any{"action": "status"})
		if r.Code != 202 {
			json.Unmarshal(r.Body.Bytes(), &status)
			return opened.Lease, status, r.Code
		}
		if time.Now().After(deadline) {
			t.Fatal("the file was still being read after 20 seconds")
		}
		time.Sleep(time.Millisecond)
	}
}

func anyFileEdit(t *testing.T, a *application, lease, revision uint64, from, to int64, text string) uint64 {
	t.Helper()
	r := diskCall(a, a.token, lease, map[string]any{"action": "edit", "revision": revision, "from": from, "to": to, "text": text})
	if r.Code != 200 {
		t.Fatal("edit refused: ", r.Body.String())
	}
	var meta editorbuffer.Metadata
	json.Unmarshal(r.Body.Bytes(), &meta)
	return meta.Revision
}

func utf16LE(text string) []byte {
	out := []byte{0xff, 0xfe}
	for _, r := range text {
		out = append(out, byte(r), byte(r>>8))
	}
	return out
}

func utf16BE(text string) []byte {
	out := []byte{0xfe, 0xff}
	for _, r := range text {
		out = append(out, byte(r>>8), byte(r))
	}
	return out
}

// Each row is one kind of text file under a name Windows has no editor for.
// An untouched save writes the bytes that were read; a save after typing "X"
// at the start changes those bytes only.
func TestEditorOpensAnyTextFileWithoutChangingItsBytes(t *testing.T) {
	mark := "\xef\xbb\xbf"
	cases := []struct {
		name, file  string
		raw, edited []byte
		format      editorbuffer.Format
	}{
		{"no extension", "running-config", []byte("hostname core-1\n!\nend\n"), []byte("Xhostname core-1\n!\nend\n"), editorbuffer.Format{Encoding: "UTF-8", EOL: "LF"}},
		{"unknown extension", "export.zz9", []byte("naïve λ 😀\r\nsecond\r\n"), []byte("Xnaïve λ 😀\r\nsecond\r\n"), editorbuffer.Format{Encoding: "UTF-8", EOL: "CRLF"}},
		{"dot file", ".settings", []byte("a=1\rb=2\r"), []byte("Xa=1\rb=2\r"), editorbuffer.Format{Encoding: "UTF-8", EOL: "CR"}},
		{"empty file", "empty.unknown", nil, []byte("X"), editorbuffer.Format{Encoding: "UTF-8", EOL: "LF"}},
		{"UTF-8 with a byte order mark", "bom.dat", []byte(mark + "ключ=значение\r\n"), []byte(mark + "Xключ=значение\r\n"), editorbuffer.Format{Encoding: "UTF-8", BOM: true, EOL: "CRLF"}},
		{"UTF-16 little endian", "le.reg", utf16LE("Windows Registry\r\n[HKEY]\r\n"), utf16LE("XWindows Registry\r\n[HKEY]\r\n"), editorbuffer.Format{Encoding: "UTF-16LE", BOM: true, EOL: "CRLF"}},
		{"UTF-16 big endian", "be.strings", utf16BE("\"k\" = \"v\";\n"), utf16BE("X\"k\" = \"v\";\n"), editorbuffer.Format{Encoding: "UTF-16BE", BOM: true, EOL: "LF"}},
		{"Windows-1252", "legacy.cfg", []byte("name=caf\xe9\r\nprice=5\x80\r\n"), []byte("Xname=caf\xe9\r\nprice=5\x80\r\n"), editorbuffer.Format{Encoding: "Windows-1252", EOL: "CRLF"}},
		{"invalid UTF-8 among valid UTF-8", "mixed-encoding.log", []byte("ok λ\nbroken \xc3\x28 \xff\n"), []byte("Xok λ\nbroken \xc3\x28 \xff\n"), editorbuffer.Format{Encoding: "Windows-1252", EOL: "LF"}},
		{"mixed line endings", "mixed.conf", []byte("one\r\ntwo\nthree\rfour\r\n"), []byte("Xone\r\ntwo\nthree\rfour\r\n"), editorbuffer.Format{Encoding: "UTF-8", EOL: "Mixed"}},
		{"mixed line endings in UTF-16", "mixed.inf", utf16LE("a\nb\r\nc\n"), utf16LE("Xa\nb\r\nc\n"), editorbuffer.Format{Encoding: "UTF-16LE", BOM: true, EOL: "Mixed"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, m, _ := editorFixture(t)
			dir := t.TempDir()
			path, copyPath := filepath.Join(dir, c.file), filepath.Join(dir, "copy of "+c.file)
			if e := os.WriteFile(path, c.raw, 0600); e != nil {
				t.Fatal(e)
			}
			lease, status, code := openAnyFile(t, a, path)
			if code != 200 {
				t.Fatalf("not opened: %d %s", code, status.Error)
			}
			defer m.releaseDocument("", editorFixtureUID, lease)
			if status.Format != c.format || status.Hash != editorDigest(c.raw) || status.Metadata.Dirty {
				t.Fatalf("opened as %+v dirty=%v, want %+v", status.Format, status.Metadata.Dirty, c.format)
			}
			// Untouched: Save As writes the same bytes to a new file.
			r := diskCall(a, a.token, lease, map[string]any{"action": "save", "revision": status.Metadata.Revision, "path": copyPath, "format": status.Format})
			if r.Code != 200 {
				t.Fatal("untouched save refused: ", r.Body.String())
			}
			if got, _ := os.ReadFile(copyPath); !bytes.Equal(got, c.raw) {
				t.Fatalf("an untouched save changed the bytes:\n got %q\nwant %q", got, c.raw)
			}
			// Edited: only the typed character is new.
			revision := anyFileEdit(t, a, lease, status.Metadata.Revision, 0, 0, "X")
			r = diskCall(a, a.token, lease, map[string]any{"action": "save", "revision": revision, "format": status.Format})
			if r.Code != 200 {
				t.Fatal("edited save refused: ", r.Body.String())
			}
			if got, _ := os.ReadFile(copyPath); !bytes.Equal(got, c.edited) {
				t.Fatalf("an edited save changed more than the edit:\n got %q\nwant %q", got, c.edited)
			}
			if got, _ := os.ReadFile(path); !bytes.Equal(got, c.raw) {
				t.Fatal("the file that was opened changed although the save went to its copy")
			}
		})
	}
}

// Lines the user did not retype keep their own ending; a typed line gets the
// file's most common one. Picking a line ending converts every line.
func TestEditorKeepsEachLineEndingOfAMixedFile(t *testing.T) {
	a, m, _ := editorFixture(t)
	path := filepath.Join(t.TempDir(), "startup-config")
	raw := "interface a\r\n shutdown\n!\r\ninterface b\r\n"
	os.WriteFile(path, []byte(raw), 0600)
	lease, status, code := openAnyFile(t, a, path)
	if code != 200 || status.Format.EOL != "Mixed" || status.NewLineEOL != "CRLF" {
		t.Fatalf("%d %+v new lines %q", code, status.Format, status.NewLineEOL)
	}
	defer m.releaseDocument("", editorFixtureUID, lease)
	// "interface a\n shutdown\n!\n...": add a line after " shutdown".
	revision := anyFileEdit(t, a, lease, status.Metadata.Revision, 21, 21, "\n description x")
	r := diskCall(a, a.token, lease, map[string]any{"action": "save", "revision": revision, "format": status.Format})
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if got, _ := os.ReadFile(path); string(got) != "interface a\r\n shutdown\r\n description x\n!\r\ninterface b\r\n" {
		t.Fatalf("saved %q", got)
	}
	r = diskCall(a, a.token, lease, map[string]any{"action": "save", "revision": revision, "format": editorbuffer.Format{Encoding: "UTF-8", EOL: "LF"}})
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if got, _ := os.ReadFile(path); string(got) != "interface a\n shutdown\n description x\n!\ninterface b\n" {
		t.Fatalf("an explicit conversion saved %q", got)
	}
}

// A character Windows-1252 cannot hold stops the save; the file on disk is
// not touched and no temporary file is left beside it.
func TestEditorRefusesToReplaceCharactersOfALegacyFile(t *testing.T) {
	a, m, _ := editorFixture(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.ini")
	raw := []byte("title=caf\xe9\r\n")
	os.WriteFile(path, raw, 0600)
	lease, status, code := openAnyFile(t, a, path)
	if code != 200 || status.Format.Encoding != "Windows-1252" {
		t.Fatal(code, status.Format)
	}
	defer m.releaseDocument("", editorFixtureUID, lease)
	revision := anyFileEdit(t, a, lease, status.Metadata.Revision, 0, 0, "→")
	r := diskCall(a, a.token, lease, map[string]any{"action": "save", "revision": revision, "format": status.Format})
	if r.Code != 409 || !strings.Contains(r.Body.String(), "Windows-1252") || !strings.Contains(r.Body.String(), "UTF-8") {
		t.Fatal("the save should name the encoding and the way out: ", r.Code, r.Body.String())
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, raw) {
		t.Fatalf("the refused save changed the file: %q", got)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatal("the refused save left a temporary file")
	}
	r = diskCall(a, a.token, lease, map[string]any{"action": "save", "revision": revision, "format": editorbuffer.Format{Encoding: "UTF-8", EOL: "CRLF"}})
	if got, _ := os.ReadFile(path); r.Code != 200 || string(got) != "→title=café\r\n" {
		t.Fatalf("choosing UTF-8 should save every character: %d %q", r.Code, got)
	}
}

// A program, an archive, an image and a file of NUL bytes open for reading
// only. Every way of changing or writing them is refused by the engine, and
// their bytes on disk never change.
func TestEditorShowsBinaryFilesReadOnlyAndNeverWritesThem(t *testing.T) {
	cases := map[string][]byte{
		"tool.exe":   append([]byte("MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xff\xff\x00\x00\xb8"), bytes.Repeat([]byte{0, 0xff, '\r', '\n'}, 40000)...),
		"backup.zip": []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00\x00\x00!\x00config.txt\x8b\x00\x01"),
		"logo.png":   []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x10\x00\x00\x00\x10"),
		"zeros":      bytes.Repeat([]byte{0}, 100000),
		"bad.txt":    {0xff, 0xfe, 0x00, 0xd8, 0x41, 0x00},
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			a, m, _ := editorFixture(t)
			dir := t.TempDir()
			path := filepath.Join(dir, name)
			os.WriteFile(path, raw, 0600)
			lease, status, code := openAnyFile(t, a, path)
			if code != 200 || status.Format.Encoding != "Binary" || status.Metadata.Dirty || status.Hash != editorDigest(raw) {
				t.Fatalf("not opened as a read-only file that is not text: %d %+v %s", code, status.Format, status.Error)
			}
			defer m.releaseDocument("", editorFixtureUID, lease)
			r := diskCall(a, a.token, lease, map[string]any{"action": "window"})
			var window editorbuffer.Window
			json.Unmarshal(r.Body.Bytes(), &window)
			if r.Code != 200 || window.Text == "" || strings.ContainsRune(window.Text, 0) {
				t.Fatal("nothing to show: ", r.Code, r.Body.String())
			}
			revision := status.Metadata.Revision
			text := editorbuffer.Format{Encoding: "UTF-8", EOL: "LF"}
			refused := []map[string]any{
				{"action": "edit", "revision": revision, "from": 0, "to": 1, "text": "x"},
				{"action": "edit", "revision": revision, "from": 0, "to": 0, "text": ""},
				{"action": "edit-begin", "revision": revision, "from": 0, "to": 1, "requestId": "bulk-1"},
				{"action": "replace-all", "revision": revision, "query": "P", "replacement": "Q", "matchCase": true},
				{"action": "replace-text", "revision": revision, "query": "P", "replacement": "Q", "from": 0, "to": 1, "regex": true},
				{"action": "tool-start", "revision": revision, "tool": "upper"},
				{"action": "tool-start", "revision": revision, "tool": "base64-encode", "from": 0, "to": 1},
				// Decode the part before it: the same tool over a range.
				{"action": "tool-start", "revision": revision, "tool": "base64-decode", "from": 0, "to": 3},
				{"action": "save", "revision": revision, "format": status.Format},
				{"action": "save", "revision": revision, "format": text},
				{"action": "save", "revision": revision, "format": text, "path": filepath.Join(dir, "saved-as.txt")},
				{"action": "save", "revision": revision, "format": text, "path": filepath.Join(dir, "saved-as.txt"), "overwrite": true},
				{"action": "export", "revision": revision, "format": text},
				{"action": "export", "revision": revision, "format": editorbuffer.Format{Encoding: "Windows-1252", EOL: "LF"}},
			}
			for _, request := range refused {
				r = diskCall(a, a.token, lease, request)
				if r.Code == 200 {
					t.Fatalf("%v was accepted for a file that is not text: %s", request["action"], r.Body.String())
				}
				if action := request["action"]; (action == "edit" || action == "save") && !strings.Contains(r.Body.String(), "not a text file") {
					t.Fatalf("%v should say that this is not a text file: %s", action, r.Body.String())
				}
			}
			if got, _ := os.ReadFile(path); !bytes.Equal(got, raw) {
				t.Fatal("the file changed on disk")
			}
			if entries, _ := os.ReadDir(dir); len(entries) != 1 {
				t.Fatalf("a refused save left %d extra files beside the original", len(entries)-1)
			}
			r = diskCall(a, a.token, lease, map[string]any{"action": "status"})
			var after anyFileStatus
			json.Unmarshal(r.Body.Bytes(), &after)
			if after.Metadata.Dirty || after.Metadata.Revision != revision || after.Metadata.CanUndo || after.Format.Encoding != "Binary" {
				t.Fatalf("the refused requests left a trace: %+v", after)
			}
			// Reading it again from disk gives the same read-only document.
			if r = diskCall(a, a.token, lease, map[string]any{"action": "reload"}); r.Code != 200 || !strings.Contains(r.Body.String(), `"encoding":"Binary"`) {
				t.Fatal("reload: ", r.Code, r.Body.String())
			}
		})
	}
}

// A path that cannot be opened gives one plain sentence and no document.
func TestEditorExplainsFilesItCannotOpen(t *testing.T) {
	dir := t.TempDir()
	locked := filepath.Join(dir, "locked.txt")
	os.WriteFile(locked, []byte("secret"), 0600)
	os.Chmod(locked, 0)
	defer os.Chmod(locked, 0600)
	cases := []struct{ name, path, message string }{
		{"folder", dir, "This is a folder, not a file."},
		{"missing file", filepath.Join(dir, "gone.cfg"), "The file does not exist."},
		{"unreadable file", locked, "The file could not be read: access was denied."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.name == "unreadable file" {
				if f, e := os.Open(locked); e == nil {
					f.Close()
					t.Skip("this account reads files without permission (administrator); the Windows runner covers a file in use")
				}
			}
			a, m, _ := editorFixture(t)
			lease, status, code := openAnyFile(t, a, c.path)
			if code != 400 || status.Error != c.message {
				t.Fatalf("%d %q, want %q", code, status.Error, c.message)
			}
			// The page releases a document that failed to open; until then it
			// has no text, and afterwards nothing of it remains.
			if r := diskCall(a, a.token, lease, map[string]any{"action": "window"}); r.Code == 200 {
				t.Fatal("a document that failed to open returned text")
			}
			m.releaseDocument("", editorFixtureUID, lease)
			m.mu.Lock()
			left := len(m.documents)
			m.mu.Unlock()
			if left != 0 {
				t.Fatal("a failed open left a document behind")
			}
		})
	}
}

// Only the engine's own documents may carry the new formats between windows
// or into the kept session; text held by a page stays UTF-8 or UTF-16.
func TestEditorAnyFileFormatsStayWithDiskDocuments(t *testing.T) {
	disk := emptyDiskFixture()
	disk.Disk, disk.Path, disk.Binding = true, filepath.Join(t.TempDir(), "x"), editorBinding{Kind: "local"}
	for _, encoding := range []string{"Windows-1252", "Binary"} {
		d := disk
		d.Encoding, d.SavedEncoding = encoding, encoding
		if e := validateEditorBatch(editorBatch{Documents: []editorDocumentSnapshot{d}, Active: d.UID}); e != nil {
			t.Fatal(encoding, e)
		}
		d.Disk = false
		if e := validateEditorBatch(editorBatch{Documents: []editorDocumentSnapshot{d}, Active: d.UID}); e == nil {
			t.Fatal("a page-held document claimed ", encoding)
		}
	}
	d := disk
	d.EOL, d.SavedEOL = "Mixed", "Mixed"
	if e := validateEditorBatch(editorBatch{Documents: []editorDocumentSnapshot{d}, Active: d.UID}); e != nil {
		t.Fatal(e)
	}
	d.Disk = false
	if e := validateEditorBatch(editorBatch{Documents: []editorDocumentSnapshot{d}, Active: d.UID}); e == nil {
		t.Fatal("a page-held document claimed per-line endings")
	}
	for format, valid := range map[editorbuffer.Format]bool{
		{Encoding: "Windows-1252", EOL: "Mixed"}:           true,
		{Encoding: "UTF-16LE", BOM: true, EOL: "Mixed"}:    true,
		{Encoding: "Windows-1252", BOM: true, EOL: "CRLF"}: false,
		{Encoding: "Binary", EOL: "LF"}:                    false,
	} {
		if validEditorDiskFormat(format) != valid {
			t.Fatalf("kept-session format %+v valid=%v", format, !valid)
		}
	}
}
