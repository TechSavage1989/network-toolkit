package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"networktoolkit/internal/desktop"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadTextBytesPreservesEncodingAndRejectsBinary(t *testing.T) {
	original := append([]byte{0xef, 0xbb, 0xbf}, []byte("line 1\r\nline 2\n日本語\n")...)
	got, err := readTextBytes(original)
	if err != nil {
		t.Fatal(err)
	}
	if got["bom"] != true || got["content"] != "line 1\r\nline 2\n日本語\n" || got["hash"] != editorDigest(original) {
		t.Fatalf("incorrect BOM/content/revision: %#v", got)
	}
	for _, b := range [][]byte{[]byte("hello\x00world"), []byte{0xff, 0xfe, 0x00, 0x00}, bytes.Repeat([]byte{'a'}, textEditorLimit+1)} {
		if _, err := readTextBytes(b); err == nil {
			t.Errorf("unsafe text accepted: %d bytes", len(b))
		}
	}
}

func TestLocalTextEditorCreateModifyConflictAndPermissions(t *testing.T) {
	name := filepath.Join(t.TempDir(), "configuration.txt")
	first, err := saveEditableLocal(name, []byte("one\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	if first["hash"] != editorDigest([]byte("one\n")) {
		t.Fatal("initial revision missing")
	}
	if _, err := saveEditableLocal(name, []byte("overwrite\n"), ""); err == nil {
		t.Fatal("overwrote an existing file without its revision")
	}
	if err := os.Chmod(name, 0640); err != nil {
		t.Fatal(err)
	}
	saved, err := saveEditableLocal(name, []byte("two\r\n"), first["hash"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if saved["hash"] == first["hash"] {
		t.Fatal("revision did not change")
	}
	info, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0640 {
		t.Errorf("lost existing permissions: %v", info.Mode().Perm())
	}
	if _, err := saveEditableLocal(name, []byte("third"), first["hash"].(string)); err == nil {
		t.Fatal("stale revision overwrote a changed file")
	}
	current, err := os.ReadFile(name)
	if err != nil || string(current) != "two\r\n" {
		t.Fatalf("file changed after stale save: %q, %v", current, err)
	}
	if err := os.Remove(name); err != nil {
		t.Fatal(err)
	}
	if _, err := saveEditableLocal(name, []byte("resurrect"), saved["hash"].(string)); err == nil {
		t.Fatal("recreated a deleted file without consent")
	}
}

func TestLocalTextEditorHTTPOperations(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.txt")
	send := func(action, content, hash string, bom bool) (int, map[string]any) {
		t.Helper()
		encoded, err := json.Marshal(map[string]any{"action": action, "path": p, "content": content, "expectedHash": hash, "bom": bom})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/file-editor/local", bytes.NewReader(encoded))
		rec := httptest.NewRecorder()
		(&application{}).handleLocalTextEditor(rec, req)
		var data map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&data); err != nil {
			t.Fatal(err)
		}
		return rec.Code, data
	}
	status, created := send("write-text", "first\r\n", "", true)
	if status != 200 || created["hash"] == "" {
		t.Fatalf("cannot create new text file: %d %#v", status, created)
	}
	status, read := send("read-text", "", "", false)
	if status != 200 || read["content"] != "first\r\n" || read["bom"] != true || read["hash"] != created["hash"] {
		t.Fatalf("text read did not roundtrip: %d %#v", status, read)
	}
	status, updated := send("write-text", "second\n", created["hash"].(string), false)
	if status != 200 || updated["hash"] == created["hash"] {
		t.Fatalf("cannot save revision: %d %#v", status, updated)
	}
	status, _ = send("write-text", "stale", created["hash"].(string), false)
	if status != 409 {
		t.Fatalf("expected conflict, got %d", status)
	}
	status, _ = send("write-text", strings.Repeat("x", textEditorLimit+1), updated["hash"].(string), false)
	if status == 200 {
		t.Fatal("accepted oversized editor save")
	}
}

// Save As consent: an existing destination is replaced only with the explicit
// overwrite flag (the Windows dialog prompt or the editor's Replace confirm).
func TestSaveEditableLocalOverwriteConsent(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(existing, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := saveEditableLocalWith(existing, []byte("new\n"), "", false); err == nil {
		t.Fatal("existing destination replaced without consent")
	}
	if data, _ := os.ReadFile(existing); string(data) != "old\n" {
		t.Fatalf("refused save changed the file: %q", data)
	}
	result, err := saveEditableLocalWith(existing, []byte("new\n"), "", true)
	if err != nil || result["ok"] != true {
		t.Fatalf("consented overwrite failed: %v %v", err, result)
	}
	if data, _ := os.ReadFile(existing); string(data) != "new\n" {
		t.Fatalf("consented overwrite did not replace contents: %q", data)
	}
	fresh := filepath.Join(dir, "fresh.txt")
	if _, err := saveEditableLocalWith(fresh, []byte("x\n"), "", true); err != nil {
		t.Fatalf("new file with consent: %v", err)
	}
	if _, err := saveEditableLocalWith(existing, []byte("later\n"), "0000000000000000000000000000000000000000000000000000000000000000", true); err == nil {
		t.Fatal("stale expected hash must still be rejected even with overwrite")
	}
}

func TestEditorSaveDialogHandler(t *testing.T) {
	dir := t.TempDir()
	chosen := filepath.Join(dir, "chosen.txt")
	cases := []struct {
		name   string
		prompt func(context.Context, desktop.SaveFileRequest) (desktop.SaveFileResponse, error)
		status int
		body   string
	}{
		{"path", func(context.Context, desktop.SaveFileRequest) (desktop.SaveFileResponse, error) {
			return desktop.SaveFileResponse{Path: chosen}, nil
		}, 200, `"path"`},
		{"cancelled", func(context.Context, desktop.SaveFileRequest) (desktop.SaveFileResponse, error) {
			return desktop.SaveFileResponse{Cancelled: true}, nil
		}, 200, `"cancelled":true`},
		{"busy", func(context.Context, desktop.SaveFileRequest) (desktop.SaveFileResponse, error) {
			return desktop.SaveFileResponse{}, desktop.ErrSaveDialogBusy
		}, 409, "already open"},
		{"unavailable", func(context.Context, desktop.SaveFileRequest) (desktop.SaveFileResponse, error) {
			return desktop.SaveFileResponse{}, desktop.ErrSaveDialogUnavailable
		}, 400, "Windows application"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var seen desktop.SaveFileRequest
			a := &application{promptSaveFile: func(ctx context.Context, request desktop.SaveFileRequest) (desktop.SaveFileResponse, error) {
				seen = request
				return tc.prompt(ctx, request)
			}}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/file-editor/save-dialog", strings.NewReader(`{"suggested":"notes.txt","directory":"`+strings.ReplaceAll(dir, `\`, `\\`)+`"}`))
			a.handleEditorSaveDialog(rec, req)
			if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.body) {
				t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
			}
			if seen.Suggested != "notes.txt" || seen.Directory != dir || seen.Title != "Save As" {
				t.Fatalf("dialog request not forwarded: %+v", seen)
			}
		})
	}
	rec := httptest.NewRecorder()
	(&application{}).handleEditorSaveDialog(rec, httptest.NewRequest(http.MethodGet, "/api/file-editor/save-dialog", nil))
	if rec.Code != 405 {
		t.Fatalf("GET status %d", rec.Code)
	}
}
