package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"networktoolkit/internal/editorbuffer"
)

func diskFixture(t *testing.T) (*application, *editorViewRegistry, string, uint64) {
	t.Helper()
	a, m, _ := editorFixture(t)
	path := filepath.Join(t.TempDir(), "source.txt")
	if e := os.WriteFile(path, []byte("before\r\nλ😀\r\n"), 0600); e != nil {
		t.Fatal(e)
	}
	s := editorFixtureBatch().Documents[0]
	s.Path = path
	s.Binding = editorBinding{Kind: "local"}
	s.Content = ""
	s.Original = ""
	s.Saved = ""
	s.Dirty = false
	s.Undo = nil
	s.Redo = nil
	r := editorHTTP(a, a.token, "/api/editor-views/buffers/open", map[string]any{"document": s})
	if r.Code != 202 {
		t.Fatalf("open: %d %s", r.Code, r.Body.String())
	}
	var opened struct {
		Lease uint64 `json:"lease"`
	}
	json.Unmarshal(r.Body.Bytes(), &opened)
	deadline := time.Now().Add(3 * time.Second)
	for {
		r = diskCall(a, a.token, opened.Lease, map[string]any{"action": "status"})
		if r.Code == 200 {
			break
		}
		if r.Code != 202 || time.Now().After(deadline) {
			t.Fatalf("import: %d %s", r.Code, r.Body.String())
		}
		time.Sleep(time.Millisecond)
	}
	t.Cleanup(func() { m.releaseDocument("", editorFixtureUID, opened.Lease) })
	return a, m, path, opened.Lease
}
func diskCall(a *application, token string, lease uint64, body map[string]any) *httptest.ResponseRecorder {
	body["lease"] = lease
	return editorHTTP(a, token, "/api/editor-views/documents/"+editorFixtureUID+"/buffer", body)
}
func TestDiskEditorLeaseEditSaveAndConflict(t *testing.T) {
	a, m, path, lease := diskFixture(t)
	r := diskCall(a, a.token, lease, map[string]any{"action": "window", "line": 1})
	var window editorbuffer.Window
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	json.Unmarshal(r.Body.Bytes(), &window)
	if window.Text != "λ😀\n" {
		t.Fatal(window)
	}
	if r = diskCall(a, a.token, lease+1, map[string]any{"action": "edit", "revision": window.Revision, "from": 0, "to": 6, "text": "wrong"}); r.Code != 409 {
		t.Fatal("stale lease accepted")
	}
	r = diskCall(a, a.token, lease, map[string]any{"action": "edit", "revision": window.Revision, "from": 0, "to": 6, "text": "after"})
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var meta editorbuffer.Metadata
	json.Unmarshal(r.Body.Bytes(), &meta)
	if r = editorHTTP(a, a.token, "/api/editor-views/documents/"+editorFixtureUID+"/file", map[string]any{"lease": lease, "action": "write-text", "path": path, "content": "truncated"}); r.Code != 409 {
		t.Fatal("full-string write bypassed buffer")
	}
	r = diskCall(a, a.token, lease, map[string]any{"action": "save", "revision": meta.Revision, "format": editorbuffer.Format{"UTF-8", false, "CRLF"}})
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "after\r\nλ😀\r\n" {
		t.Fatalf("saved %q", raw)
	}
	m.mu.Lock()
	snap := m.documents[editorFixtureUID].snapshot
	m.mu.Unlock()
	snap.Disk = false
	if _, e := m.checkpoint("", editorBatch{Documents: []editorDocumentSnapshot{snap}, Active: snap.UID}, map[string]uint64{snap.UID: lease}); e == nil {
		t.Fatal("checkpoint discarded disk handle")
	}
	os.WriteFile(path, []byte("external"), 0600)
	r = diskCall(a, a.token, lease, map[string]any{"action": "save", "revision": meta.Revision, "format": editorbuffer.Format{"UTF-8", false, "LF"}})
	if r.Code != 409 {
		t.Fatal("external modification overwritten")
	}
	raw, _ = os.ReadFile(path)
	if string(raw) != "external" {
		t.Fatal("external bytes lost")
	}
}
func TestDiskEditorSaveAsRequiresConsent(t *testing.T) {
	a, _, path, lease := diskFixture(t)
	dest := filepath.Join(filepath.Dir(path), "other.txt")
	os.WriteFile(dest, []byte("other"), 0600)
	r := diskCall(a, a.token, lease, map[string]any{"action": "save", "revision": 1, "path": dest, "format": editorbuffer.Format{"UTF-8", false, "LF"}})
	if r.Code != 409 {
		t.Fatal("overwrite allowed without consent")
	}
	r = diskCall(a, a.token, lease, map[string]any{"action": "save", "revision": 1, "path": dest, "overwrite": true, "format": editorbuffer.Format{"UTF-8", false, "LF"}})
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	raw, _ := os.ReadFile(dest)
	if !strings.HasPrefix(string(raw), "before\n") {
		t.Fatal("save as wrong text")
	}
}
func TestDiskEditorOwnershipMovesWithBuffer(t *testing.T) {
	a, m, _, lease := diskFixture(t)
	m.mu.Lock()
	snapshot := m.documents[editorFixtureUID].snapshot
	m.mu.Unlock()
	req := editorTransferRequest{ID: randomHex(16), Destination: "new", Batch: editorBatch{Documents: []editorDocumentSnapshot{snapshot}, Active: snapshot.UID}, Leases: map[string]uint64{snapshot.UID: lease}}
	out, e := m.offer("", req)
	if e != nil {
		t.Fatal(e)
	}
	dest := out["destination"].(string)
	m.mu.Lock()
	m.views[dest].info.State = "ready"
	token := m.views[dest].token
	m.mu.Unlock()
	if _, e = m.accept(dest, req.ID); e != nil {
		t.Fatal(e)
	}
	if r := diskCall(a, a.token, lease, map[string]any{"action": "window"}); r.Code != http.StatusConflict {
		t.Fatal("source retained file access")
	}
	m.mu.Lock()
	nextLease := m.documents[snapshot.UID].lease
	m.mu.Unlock()
	r := diskCall(a, token, nextLease, map[string]any{"action": "window"})
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if !strings.Contains(r.Body.String(), "before") {
		t.Fatal("moved buffer lost source")
	}
	m.releaseDocument(dest, snapshot.UID, nextLease)
}
