package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalEditorRoundTripPreservesUTF16(t *testing.T) {
	path := filepath.Join(t.TempDir(), "utf16.conf")
	original, err := encodeEditableText("first: äΩ", "UTF-16LE", true)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	app := &application{}
	send := func(req map[string]any) (int, map[string]any) {
		b, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		resp := httptest.NewRecorder()
		app.handleLocalTextEditor(resp, httptest.NewRequest(http.MethodPost, "/api/file-editor/local", strings.NewReader(string(b))))
		var data map[string]any
		if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
			t.Fatalf("not json: %s", resp.Body.String())
		}
		return resp.Code, data
	}
	code, initial := send(map[string]any{"action": "read-text", "path": path})
	if code != 200 || initial["encoding"] != "UTF-16LE" || initial["bom"] != true || initial["content"] != "first: äΩ" {
		t.Fatalf("initial read %d %#v", code, initial)
	}
	code, save := send(map[string]any{"action": "write-text", "path": path, "content": "updated: ü雪", "expectedHash": initial["hash"], "encoding": "UTF-16LE", "bom": true})
	if code != 200 {
		t.Fatalf("save %d %#v", code, save)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(got, []byte{0xff, 0xfe}) {
		t.Fatalf("missing UTF-16 BOM: %x", got[:2])
	}
	text, enc, bom, err := decodeEditableText(got)
	if err != nil || text != "updated: ü雪" || enc != "UTF-16LE" || !bom {
		t.Fatalf("text %q encoding %s bom %v err %v", text, enc, bom, err)
	}
	code, _ = send(map[string]any{"action": "write-text", "path": path, "content": "overwrite", "expectedHash": initial["hash"], "encoding": "UTF-16LE", "bom": true})
	if code != http.StatusConflict {
		t.Fatalf("stale version was accepted: %d", code)
	}
}
