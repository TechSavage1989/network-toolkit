package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"networktoolkit/internal/editorbuffer"
)

func emptyDiskFixture() editorDocumentSnapshot {
	s := editorFixtureBatch().Documents[0]
	s.Content, s.Original, s.Saved = "", "", ""
	s.Dirty, s.Untitled = false, false
	s.Undo, s.Redo = nil, nil
	return s
}

func waitDiskReady(t *testing.T, a *application, lease uint64) *httptest.ResponseRecorder {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		r := diskCall(a, a.token, lease, map[string]any{"action": "status"})
		if r.Code == 200 {
			return r
		}
		if r.Code != 202 || time.Now().After(deadline) {
			t.Fatalf("open: %d %s", r.Code, r.Body.String())
		}
		time.Sleep(time.Millisecond)
	}
}

func TestDiskEditorBrowserReloadExportAndVerification(t *testing.T) {
	a, m, _ := editorFixture(t)
	r := editorHTTP(a, a.token, "/api/editor-views/buffers/open", map[string]any{"document": emptyDiskFixture(), "mode": "upload"})
	if r.Code != 202 {
		t.Fatal(r.Code, r.Body)
	}
	defer m.releaseDocument("", editorFixtureUID, 1)
	call := func(action string, fields map[string]any) *httptest.ResponseRecorder {
		t.Helper()
		if fields == nil {
			fields = map[string]any{}
		}
		fields["action"] = action
		return diskCall(a, a.token, 1, fields)
	}
	raw := []byte("a\r\nλ😀\r\n")
	chunk := map[string]any{"offset": 0, "data": raw[:6]}
	for i := 0; i < 2; i++ {
		if r = call("import-chunk", chunk); r.Code != 200 {
			t.Fatal(r.Body)
		}
	}
	if r = call("import-finish", map[string]any{"size": len(raw)}); r.Code != 409 {
		t.Fatal("accepted an incomplete browser upload")
	}
	if r = call("import-chunk", map[string]any{"offset": 6, "data": raw[6:]}); r.Code != 200 {
		t.Fatal(r.Body)
	}
	if r = call("import-finish", map[string]any{"size": len(raw)}); r.Code != 202 {
		t.Fatal(r.Body)
	}
	if r = waitDiskReady(t, a, 1); !strings.Contains(r.Body.String(), editorDigest(raw)) {
		t.Fatal("wrong imported hash", r.Body)
	}
	if r = call("verify-start", nil); r.Code != 200 {
		t.Fatal(r.Body)
	}
	if r = call("verify-chunk", map[string]any{"offset": 0, "data": raw}); r.Code != 200 {
		t.Fatal(r.Body)
	}
	if r = call("verify-finish", map[string]any{"size": len(raw)}); r.Code != 200 || !strings.Contains(r.Body.String(), editorDigest(raw)) {
		t.Fatal(r.Body)
	}
	format := editorbuffer.Format{Encoding: "UTF-8", EOL: "CRLF"}
	if r = call("export", map[string]any{"revision": 1, "format": format, "range": editorDiskRange{2, 5}}); r.Code != 200 || r.Body.String() != "λ😀" || r.Header().Get("Content-Length") != "6" {
		t.Fatal("wrong streamed selection", r.Code, r.Body)
	}
	if r = call("edit", map[string]any{"revision": 1, "from": 0, "to": 1, "text": "changed"}); r.Code != 200 {
		t.Fatal(r.Body)
	}
	r = call("export", map[string]any{"revision": 2, "format": format})
	if r.Code != 200 || r.Body.String() != "changed\r\nλ😀\r\n" {
		t.Fatal(r.Code, r.Body)
	}
	receipt := r.Header().Get("X-Editor-Export")
	if r = call("edit", map[string]any{"revision": 2, "from": 0, "to": 0, "text": "newer "}); r.Code != 200 {
		t.Fatal(r.Body)
	}
	if r = call("mark-saved", map[string]any{"receipt": receipt}); r.Code != 200 || !strings.Contains(r.Body.String(), `"dirty":true`) {
		t.Fatal("saving the exported revision dropped newer edits", r.Body)
	}
	if r = call("reload-start", map[string]any{"revision": 3}); r.Code != 200 {
		t.Fatal(r.Body)
	}
	// A reload that does not finish leaves the ready document as it was. (A
	// single 0xff byte used to be the unfinished reload here; it is now read
	// as Windows-1252 text, which the next block checks.)
	if r = call("import-chunk", map[string]any{"offset": 0, "data": []byte{0xff}}); r.Code != 200 {
		t.Fatal(r.Body)
	}
	if r = call("cancel", nil); r.Code != 200 {
		t.Fatal(r.Body)
	}
	if r = waitDiskReady(t, a, 1); !strings.Contains(r.Body.String(), "Browser file reload cancelled.") {
		t.Fatal("unfinished reload did not preserve a ready document", r.Body)
	}
	if r = call("window", nil); r.Code != 200 || !strings.Contains(r.Body.String(), "newer changed") {
		t.Fatal("unfinished reload lost original text", r.Body)
	}
	if r = call("reload-start", map[string]any{"revision": 3}); r.Code != 200 {
		t.Fatal(r.Body)
	}
	if r = call("import-chunk", map[string]any{"offset": 0, "data": []byte{0xff}}); r.Code != 200 {
		t.Fatal(r.Body)
	}
	if r = call("import-finish", map[string]any{"size": 1}); r.Code != 202 {
		t.Fatal(r.Body)
	}
	if r = waitDiskReady(t, a, 1); !strings.Contains(r.Body.String(), `"encoding":"Windows-1252"`) || strings.Contains(r.Body.String(), "invalid UTF-8 text") {
		t.Fatal("a byte that is not UTF-8 was not read as Windows-1252", r.Body)
	}
	if r = call("window", nil); r.Code != 200 || !strings.Contains(r.Body.String(), `"text":"ÿ"`) || !strings.Contains(r.Body.String(), `"dirty":false`) {
		t.Fatal("the reloaded byte is not shown as its Windows-1252 character", r.Body)
	}
	var reloaded editorbuffer.Window
	json.Unmarshal(r.Body.Bytes(), &reloaded)
	if r = call("reload-start", map[string]any{"revision": reloaded.Revision}); r.Code != 200 {
		t.Fatal(r.Body)
	}
	if r = call("import-chunk", map[string]any{"offset": 0, "data": []byte("replacement")}); r.Code != 200 {
		t.Fatal(r.Body)
	}
	if r = call("import-finish", map[string]any{"size": len("replacement")}); r.Code != 202 {
		t.Fatal(r.Body)
	}
	waitDiskReady(t, a, 1)
	if r = call("window", nil); r.Code != 200 || !strings.Contains(r.Body.String(), "replacement") || !strings.Contains(r.Body.String(), `"dirty":false`) {
		t.Fatal("valid reload did not replace text", r.Body)
	}
}

func TestDiskEditorPromotionBulkEditAndReplaceAll(t *testing.T) {
	a, m, _ := editorFixture(t)
	s := editorFixtureBatch().Documents[0]
	s.Source = "Desktop"
	s.Undo = []string{s.Saved, "undo-first"}
	s.Redo = []string{"redo-next"}
	leases, err := m.checkpoint("", editorBatch{Documents: []editorDocumentSnapshot{s}, Active: s.UID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	promotion := map[string]any{"document": s, "lease": leases[s.UID]}
	for i := 0; i < 2; i++ {
		if r := editorHTTP(a, a.token, "/api/editor-views/buffers/promote", promotion); r.Code != 200 {
			t.Fatal(r.Body)
		}
	}
	defer m.releaseDocument("", s.UID, leases[s.UID])
	call := func(action string, fields map[string]any) *httptest.ResponseRecorder {
		t.Helper()
		fields["action"] = action
		r := diskCall(a, a.token, leases[s.UID], fields)
		if r.Code != 200 {
			t.Fatal(action, r.Code, r.Body)
		}
		return r
	}
	call("verify-start", map[string]any{})
	call("undo", map[string]any{"revision": s.EditRevision})
	if r := call("window", map[string]any{}); !strings.Contains(r.Body.String(), "undo-first") {
		t.Fatal("promotion dropped undo state", r.Body)
	}
	call("redo", map[string]any{"revision": s.EditRevision + 1})
	id := "fixture-chunk-edit"
	call("edit-begin", map[string]any{"requestId": id, "revision": s.EditRevision + 2, "from": 0, "to": len(s.Content)})
	piece := strings.Repeat("😀", 150000)
	for i := 0; i < 2; i++ {
		call("edit-chunk", map[string]any{"requestId": id, "offset": 0, "text": piece})
	}
	call("edit-chunk", map[string]any{"requestId": id, "offset": 300000, "text": piece + " test\ntest"})
	var meta editorbuffer.Metadata
	for i := 0; i < 2; i++ {
		r := call("edit-commit", map[string]any{"requestId": id})
		if err := json.Unmarshal(r.Body.Bytes(), &meta); err != nil || meta.Units != 600010 || meta.Revision != s.EditRevision+3 {
			t.Fatal("incorrect bulk revision", r.Body, err)
		}
	}
	if r := call("find", map[string]any{"query": "test", "start": meta.Units, "backwards": true}); !strings.Contains(r.Body.String(), `"line":1`) {
		t.Fatal("did not find the closest match from the end", r.Body)
	}
	if r := call("replace-all", map[string]any{"query": "test", "replacement": "λ", "revision": meta.Revision}); !strings.Contains(r.Body.String(), `"count":2`) {
		t.Fatal("did not replace both matches", r.Body)
	}
	if r := call("window", map[string]any{"start": 600000}); !strings.Contains(r.Body.String(), " λ\\nλ") {
		t.Fatal("wrong replaced text", r.Body)
	}
	call("undo", map[string]any{"revision": meta.Revision + 1})
	call("undo", map[string]any{"revision": meta.Revision + 2})
	if r := call("window", map[string]any{}); !strings.Contains(r.Body.String(), "private-document-canary\\nedit") {
		t.Fatal("bulk operation did not undo as one edit", r.Body)
	}
}

func TestDiskEditorFTPSOpenSaveAndEndpointGuard(t *testing.T) {
	a, m, _ := editorFixture(t)
	base := t.TempDir()
	manager := newTransferHostManager(context.Background(), base)
	defer manager.Close()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	root := filepath.Join(base, "share")
	status, err := manager.start(transferServerConfig{Protocol: "ftp", Security: "explicit", Root: root, Bind: "127.0.0.1", Port: port, Access: "readwrite", Username: "client", Password: "fixture-secret", Overwrite: true})
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "large.txt")
	// Generate a file beyond the earlier editor ceiling using modest chunks.
	f, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	part := strings.Repeat("fixture\r\n", 64<<10)
	for i := 0; i < 30; i++ {
		if _, err := f.WriteString(part); err != nil {
			f.Close()
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	remote := remoteBrowserRequest{Protocol: "ftp", Security: "explicit", Host: "127.0.0.1", Port: port, Username: "client", Password: "fixture-secret", Fingerprint: status["fingerprint"].(string)}
	s := emptyDiskFixture()
	s.Path, s.Binding = "large.txt", editorBinding{Kind: "ftp"}
	if r := editorHTTP(a, a.token, "/api/editor-views/buffers/open", map[string]any{"document": s, "remote": remote}); r.Code != 202 {
		t.Fatal(r.Code, r.Body)
	}
	defer m.releaseDocument("", s.UID, 1)
	ready := waitDiskReady(t, a, 1)
	if strings.Contains(ready.Body.String(), "fixture-secret") {
		t.Fatal("FTP secret escaped the request")
	}
	if r := diskCall(a, a.token, 1, map[string]any{"action": "edit", "revision": 1, "from": 0, "to": 7, "text": "changed"}); r.Code != 200 {
		t.Fatal(r.Body)
	}
	save := map[string]any{"action": "save", "revision": 2, "format": editorbuffer.Format{Encoding: "UTF-8", EOL: "CRLF"}}
	wrong := remote
	wrong.Port++
	save["remote"] = wrong
	if r := diskCall(a, a.token, 1, save); r.Code != 409 {
		t.Fatal("accepted changed remote endpoint", r.Body)
	}
	save["remote"] = remote
	if r := diskCall(a, a.token, 1, save); r.Code != 200 {
		t.Fatal(r.Code, r.Body)
	}
	f, err = os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	check := make([]byte, len("changed\r\n"))
	if _, err := f.Read(check); err != nil || string(check) != "changed\r\n" {
		t.Fatal("wrong streamed save", string(check), err)
	}
	info, err := f.Stat()
	if err != nil || info.Size() != 30*int64(len(part)) {
		t.Fatal("FTP save truncated the file", info, err)
	}
}
