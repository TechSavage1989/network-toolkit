package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestTransferLocalFileBrowser(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "sample.bin")
	if e := os.WriteFile(file, []byte{1, 2, 3}, 0600); e != nil {
		t.Fatal(e)
	}
	app := &application{}
	req := httptest.NewRequest(http.MethodGet, "/api/transfer/local?path="+file, nil)
	rec := httptest.NewRecorder()
	app.handleTransferLocal(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var data struct {
		Path    string               `json:"path"`
		Entries []localTransferEntry `json:"entries"`
	}
	if e := json.Unmarshal(rec.Body.Bytes(), &data); e != nil {
		t.Fatal(e)
	}
	if data.Path != root || len(data.Entries) != 1 || data.Entries[0].Name != "sample.bin" || data.Entries[0].Size != 3 {
		t.Fatalf("bad browser: %+v", data)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/transfer/local", nil)
	rec = httptest.NewRecorder()
	app.handleTransferLocal(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("mutation allowed: %d", rec.Code)
	}
}
