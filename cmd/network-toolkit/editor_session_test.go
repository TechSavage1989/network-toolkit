package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"networktoolkit/internal/editorbuffer"
)

func sessionJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func sessionDir(a *application) string {
	return filepath.Join(filepath.Dir(a.dataPath), editorSessionDirectory)
}

func keepText(t *testing.T, a *application, text string) editorSessionBackup {
	t.Helper()
	r := requestApp(a, http.MethodPost, editorSessionEndpoint+"/backup", sessionJSON(t, map[string]any{"source": "text", "text": text}), true)
	if r.Code != http.StatusOK {
		t.Fatalf("keep text: %d %s", r.Code, r.Body.String())
	}
	var backup editorSessionBackup
	if err := json.Unmarshal(r.Body.Bytes(), &backup); err != nil {
		t.Fatal(err)
	}
	return backup
}

func putSession(t *testing.T, a *application, index any) *httptest.ResponseRecorder {
	t.Helper()
	return requestApp(a, http.MethodPut, editorSessionEndpoint, sessionJSON(t, index), true)
}

func readSession(t *testing.T, a *application) (editorSessionIndex, string) {
	t.Helper()
	r := requestApp(a, http.MethodGet, editorSessionEndpoint, "", true)
	if r.Code != http.StatusOK {
		t.Fatalf("read session: %d %s", r.Code, r.Body.String())
	}
	var out struct {
		editorSessionIndex
		Warning string `json:"warning"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.editorSessionIndex, out.Warning
}

func sessionFormat() *editorbuffer.Format {
	return &editorbuffer.Format{Encoding: "UTF-8", EOL: "CRLF"}
}

func TestEditorSessionValidationRefusesUnsafeRecords(t *testing.T) {
	a := testApplication(t)
	local := filepath.Join(t.TempDir(), "notes.txt")
	backup := keepText(t, a, "unsaved")
	good := editorSessionTab{Kind: "untitled", Name: "Untitled.txt", Dirty: true, Format: sessionFormat(), Backup: &backup}
	cases := map[string]any{
		"unknown kind":          editorSessionIndex{Version: 1, Active: -1, Tabs: []editorSessionTab{{Kind: "remote", Name: "x", Dirty: true, Format: sessionFormat(), Backup: &backup}}},
		"relative local path":   editorSessionIndex{Version: 1, Active: -1, Tabs: []editorSessionTab{{Kind: "local", Path: "notes.txt"}}},
		"dirty without copy":    editorSessionIndex{Version: 1, Active: -1, Tabs: []editorSessionTab{{Kind: "local", Path: local, Dirty: true, Format: sessionFormat(), SavedFormat: sessionFormat()}}},
		"clean with copy":       editorSessionIndex{Version: 1, Active: -1, Tabs: []editorSessionTab{{Kind: "local", Path: local, Backup: &backup}}},
		"untitled with path":    editorSessionIndex{Version: 1, Active: -1, Tabs: []editorSessionTab{{Kind: "untitled", Path: local, Name: "x", Dirty: true, Format: sessionFormat(), Backup: &backup}}},
		"copy without origin":   editorSessionIndex{Version: 1, Active: -1, Tabs: []editorSessionTab{{Kind: "copy", Name: "x", Dirty: true, Format: sessionFormat(), Backup: &backup}}},
		"untitled with origin":  editorSessionIndex{Version: 1, Active: -1, Tabs: []editorSessionTab{{Kind: "untitled", Name: "x", Origin: "SFTP", Dirty: true, Format: sessionFormat(), Backup: &backup}}},
		"active out of range":   editorSessionIndex{Version: 1, Active: 1, Tabs: []editorSessionTab{good}},
		"duplicate copy":        editorSessionIndex{Version: 1, Active: -1, Tabs: []editorSessionTab{good, good}},
		"stored file status":    editorSessionIndex{Version: 1, Active: -1, Tabs: []editorSessionTab{{Kind: "local", Path: local, Exists: new(bool)}}},
		"traversing copy":       editorSessionIndex{Version: 1, Active: -1, Tabs: []editorSessionTab{{Kind: "untitled", Name: "x", Dirty: true, Format: sessionFormat(), Backup: &editorSessionBackup{ID: "../../Network_Toolkit.settings", Size: 3, SHA256: backup.SHA256}}}},
		"unsupported format":    editorSessionIndex{Version: 1, Active: -1, Tabs: []editorSessionTab{{Kind: "untitled", Name: "x", Dirty: true, Format: &editorbuffer.Format{Encoding: "Latin-1", EOL: "LF"}, Backup: &backup}}},
		"control in name":       editorSessionIndex{Version: 1, Active: -1, Tabs: []editorSessionTab{{Kind: "untitled", Name: "a\nb", Dirty: true, Format: sessionFormat(), Backup: &backup}}},
		"unsupported version":   editorSessionIndex{Version: 2, Active: -1, Tabs: []editorSessionTab{}},
		"connection binding":    map[string]any{"version": 1, "active": -1, "tabs": []any{map[string]any{"kind": "local", "path": local, "dirty": false, "binding": map[string]any{"kind": "sshConnection", "connectionId": "abc"}}}},
		"stored password":       map[string]any{"version": 1, "active": -1, "tabs": []any{map[string]any{"kind": "local", "path": local, "dirty": false, "password": "secret"}}},
		"unknown top-level key": map[string]any{"version": 1, "active": -1, "tabs": []any{}, "connections": []any{}},
	}
	too := editorSessionIndex{Version: 1, Active: -1}
	for i := 0; i <= editorSessionTabLimit; i++ {
		too.Tabs = append(too.Tabs, editorSessionTab{Kind: "local", Path: filepath.Join(t.TempDir(), "f"+strings.Repeat("x", i%5)+".txt")})
	}
	cases["too many tabs"] = too
	for name, index := range cases {
		if r := putSession(t, a, index); r.Code != http.StatusBadRequest {
			t.Errorf("%s: accepted with %d %s", name, r.Code, r.Body.String())
		}
	}
	if _, err := os.Stat(filepath.Join(sessionDir(a), editorSessionIndexName)); !os.IsNotExist(err) {
		t.Fatal("a refused session was written", err)
	}
	if r := putSession(t, a, editorSessionIndex{Version: 1, Active: 0, Tabs: []editorSessionTab{good}}); r.Code != http.StatusOK {
		t.Fatal(r.Code, r.Body.String())
	}
	for _, id := range []string{"", "../session", "..%2F..%2FNetwork_Toolkit.settings", strings.Repeat("A", 32)} {
		if r := requestApp(a, http.MethodGet, editorSessionEndpoint+"/backup?id="+id, "", true); r.Code != http.StatusNotFound {
			t.Errorf("backup %q: %d %s", id, r.Code, r.Body.String())
		}
	}
	if r := requestApp(a, http.MethodGet, editorSessionEndpoint, "", false); r.Code != http.StatusUnauthorized {
		t.Fatal("the session was readable without the app session token", r.Code)
	}
	a.windowMode = "terminal"
	if r := requestApp(a, http.MethodGet, editorSessionEndpoint, "", true); r.Code != http.StatusForbidden {
		t.Fatal("the independent Terminal workspace reached the editor session", r.Code)
	}
}

func TestEditorSessionRoundTripCleanupAndDelete(t *testing.T) {
	a := testApplication(t)
	local := filepath.Join(t.TempDir(), "kept.txt")
	if err := os.WriteFile(local, []byte("on disk\n"), 0600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "gone.txt")
	first := keepText(t, a, "\ufeffstarts with a BOM character\nλ😀")
	copied := keepText(t, a, "remote unsaved")
	index := editorSessionIndex{Version: 1, Active: 2, Tabs: []editorSessionTab{
		{Kind: "local", Path: local},
		{Kind: "local", Path: missing, Dirty: true, Hash: strings.Repeat("a", 64), Format: sessionFormat(), SavedFormat: &editorbuffer.Format{Encoding: "UTF-16LE", BOM: true, EOL: "LF"}, Backup: &first},
		{Kind: "copy", Name: "router.cfg", Origin: "SFTP files · /etc/router.cfg", Language: "network-fortios", Dirty: true, Format: sessionFormat(), Backup: &copied},
	}}
	if r := putSession(t, a, index); r.Code != http.StatusOK {
		t.Fatal(r.Code, r.Body.String())
	}
	got, warning := readSession(t, a)
	if warning != "" || got.Active != 2 || len(got.Tabs) != 3 || got.Tabs[2].Origin != index.Tabs[2].Origin || got.Tabs[1].SavedFormat.Encoding != "UTF-16LE" {
		t.Fatalf("round trip changed the session: %+v %q", got, warning)
	}
	if got.Tabs[0].Exists == nil || !*got.Tabs[0].Exists || got.Tabs[1].Exists == nil || *got.Tabs[1].Exists || got.Tabs[2].Exists != nil {
		t.Fatalf("wrong local file status: %+v", got.Tabs)
	}
	raw, err := os.ReadFile(filepath.Join(sessionDir(a), editorSessionIndexName))
	if err != nil || bytes.Contains(raw, []byte("remote unsaved")) || bytes.Contains(raw, []byte("exists")) {
		t.Fatalf("the index stored document text or file status: %s %v", raw, err)
	}
	r := requestApp(a, http.MethodGet, editorSessionEndpoint+"/backup?id="+first.ID, "", true)
	var text struct{ Text string }
	if r.Code != http.StatusOK || json.Unmarshal(r.Body.Bytes(), &text) != nil || text.Text != "\ufeffstarts with a BOM character\nλ😀" {
		t.Fatalf("kept text changed: %d %s", r.Code, r.Body.String())
	}

	// A newer copy replaces the old one; leftovers from an interrupted write
	// and copies no index refers to are removed when the next index is saved.
	second := keepText(t, a, "newer unsaved text")
	recent := keepText(t, a, "written but not saved in an index yet")
	stale := filepath.Join(sessionDir(a), editorSessionBackupFile(strings.Repeat("b", 32)))
	temp := filepath.Join(sessionDir(a), ".backup-"+strings.Repeat("c", 32)+".tmp")
	unrelated := filepath.Join(sessionDir(a), "notes-from-user.txt")
	for _, path := range []string{stale, temp, unrelated} {
		if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	index.Tabs[2].Backup = &second
	if r := putSession(t, a, index); r.Code != http.StatusOK {
		t.Fatal(r.Code, r.Body.String())
	}
	exists := func(id string) bool {
		_, err := os.Stat(filepath.Join(sessionDir(a), editorSessionBackupFile(id)))
		return err == nil
	}
	if exists(copied.ID) || !exists(second.ID) || !exists(first.ID) || !exists(recent.ID) {
		t.Fatal("cleanup removed a referenced or pending copy, or kept a replaced one")
	}
	for _, path := range []string{stale, temp} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("cleanup kept an unreferenced leftover", path)
		}
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Fatal("cleanup removed a file it does not own")
	}
	a.editorSession.mu.Lock()
	a.editorSession.pending[recent.ID] = time.Now().Add(-2 * editorSessionGrace)
	a.editorSession.mu.Unlock()
	if r := putSession(t, a, index); r.Code != http.StatusOK || exists(recent.ID) {
		t.Fatal("an expired unreferenced copy was kept", r.Code)
	}
	index.Tabs[2].Backup = &editorSessionBackup{ID: strings.Repeat("d", 32), Size: 9, SHA256: second.SHA256}
	if r := putSession(t, a, index); r.Code != http.StatusConflict {
		t.Fatal("an index naming a missing copy was saved", r.Code)
	}

	if r := requestApp(a, http.MethodDelete, editorSessionEndpoint, "", true); r.Code != http.StatusOK {
		t.Fatal(r.Code, r.Body.String())
	}
	if got, _ := readSession(t, a); len(got.Tabs) != 0 || exists(first.ID) || exists(second.ID) {
		t.Fatal("turning restore off kept session data")
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Fatal("removing the session deleted an unrelated file")
	}
	os.Remove(unrelated)
	if r := requestApp(a, http.MethodDelete, editorSessionEndpoint, "", true); r.Code != http.StatusOK {
		t.Fatal(r.Code)
	}
	if _, err := os.Stat(sessionDir(a)); !os.IsNotExist(err) {
		t.Fatal("the empty session folder was kept", err)
	}
}

func TestEditorSessionSetsAsideAnUnreadableIndexWithoutRemovingCopies(t *testing.T) {
	a := testApplication(t)
	orphan := keepText(t, a, "text from an older session")
	a.editorSession = editorSessionStore{} // as after a restart
	if err := os.WriteFile(filepath.Join(sessionDir(a), editorSessionIndexName), []byte(`{"version":1,"tabs":[{"kind":"local","path":"relative"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	got, warning := readSession(t, a)
	if len(got.Tabs) != 0 || !strings.Contains(warning, "could not be read") {
		t.Fatalf("an unreadable session was not reported: %+v %q", got, warning)
	}
	matches, _ := filepath.Glob(filepath.Join(sessionDir(a), "session.unreadable-*.json"))
	if len(matches) != 1 {
		t.Fatal("the unreadable index was not kept aside", matches)
	}
	if r := putSession(t, a, emptyEditorSession()); r.Code != http.StatusOK {
		t.Fatal(r.Code, r.Body.String())
	}
	if _, err := os.Stat(filepath.Join(sessionDir(a), editorSessionBackupFile(orphan.ID))); err != nil {
		t.Fatal("a copy that the unreadable index may have named was removed")
	}
}

func TestEditorSessionRefusesSavesWhileAppDataDeletionIsPending(t *testing.T) {
	a := testApplication(t)
	root := t.TempDir()
	a.storage = newAppStorage(appStorageConfig{RootDir: root, DefaultDir: root, Explicit: true})
	if err := a.storage.Open(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.storage.Close() })
	a.storage.mu.Lock()
	a.storage.deletePending = true
	a.storage.mu.Unlock()
	if r := requestApp(a, http.MethodPost, editorSessionEndpoint+"/backup", `{"source":"text","text":"x"}`, true); r.Code != http.StatusConflict {
		t.Fatal("a copy was kept while app data deletion was pending", r.Code)
	}
	if r := putSession(t, a, emptyEditorSession()); r.Code != http.StatusConflict {
		t.Fatal("an index was saved while app data deletion was pending", r.Code)
	}
	a.storage.mu.Lock()
	a.storage.deletePending = false
	a.storage.mu.Unlock()
}

func sessionWaitReady(t *testing.T, a *application, uid string, lease uint64) map[string]any {
	t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for {
		r := editorHTTP(a, a.token, "/api/editor-views/documents/"+uid+"/buffer", map[string]any{"action": "status", "lease": lease})
		if r.Code == http.StatusOK {
			var out map[string]any
			json.Unmarshal(r.Body.Bytes(), &out)
			return out
		}
		if r.Code != http.StatusAccepted || time.Now().After(deadline) {
			t.Fatalf("open: %d %s", r.Code, r.Body.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func sessionOpen(t *testing.T, a *application, document editorDocumentSnapshot, restore any) uint64 {
	t.Helper()
	body := map[string]any{"document": document}
	if restore != nil {
		body["mode"], body["restore"] = "restore", restore
	}
	r := editorHTTP(a, a.token, "/api/editor-views/buffers/open", body)
	if r.Code != http.StatusAccepted {
		t.Fatalf("open: %d %s", r.Code, r.Body.String())
	}
	var opened struct{ Lease uint64 }
	json.Unmarshal(r.Body.Bytes(), &opened)
	return opened.Lease
}

func fileDigest(t *testing.T, path string) string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// A large disk-backed document is kept by streaming its buffer snapshot to the
// session folder, restored as unsaved text with its original save baseline and
// format, and saved only when the file is unchanged since the last session.
func TestEditorSessionStreamsAndRestoresLargeDiskDocument(t *testing.T) {
	a, m, _ := editorFixture(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "large.log")
	const lines = 1 << 20 // 64 MiB of generated text
	row := strings.Repeat("generated session record ", 2) + "0123456789abc\r\n"
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	block := []byte(strings.Repeat(row, 4096))
	for i := 0; i < lines/4096; i++ {
		if _, err := file.Write(block); err != nil {
			t.Fatal(err)
		}
	}
	file.Close()
	size := int64(len(row) * lines)
	original := fileDigest(t, path)
	document := emptyDiskFixture()
	document.Path, document.Binding, document.Source = path, editorBinding{Kind: "local"}, "Local computer"
	lease := sessionOpen(t, a, document, nil)
	status := sessionWaitReady(t, a, editorFixtureUID, lease)
	revision := uint64(status["metadata"].(map[string]any)["revision"].(float64))
	r := diskCall(a, a.token, lease, map[string]any{"action": "edit", "revision": revision, "from": 0, "to": 0, "text": "UNSAVED-HEAD\n"})
	if r.Code != http.StatusOK {
		t.Fatal(r.Body)
	}

	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	r = requestApp(a, http.MethodPost, editorSessionEndpoint+"/backup", sessionJSON(t, map[string]any{"source": "buffer", "uid": editorFixtureUID, "lease": lease}), true)
	runtime.ReadMemStats(&after)
	if r.Code != http.StatusOK {
		t.Fatal(r.Code, r.Body.String())
	}
	var kept struct {
		editorSessionBackup
		Revision uint64 `json:"revision"`
	}
	json.Unmarshal(r.Body.Bytes(), &kept)
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 16<<20 {
		t.Fatalf("keeping a %d byte document allocated %d bytes", size, allocated)
	}
	normalized := size - lines + int64(len("UNSAVED-HEAD\n")+len(editorSessionBOM))
	if kept.Revision != revision+1 || kept.Size != normalized {
		t.Fatalf("wrong kept copy: %+v want size %d", kept, normalized)
	}
	if fileDigest(t, filepath.Join(sessionDir(a), editorSessionBackupFile(kept.ID))) != kept.SHA256 {
		t.Fatal("the kept copy checksum does not match its bytes")
	}
	if r := requestApp(a, http.MethodPost, editorSessionEndpoint+"/backup", sessionJSON(t, map[string]any{"source": "buffer", "uid": editorFixtureUID2, "lease": lease}), true); r.Code != http.StatusConflict {
		t.Fatal("kept a document this workspace does not own", r.Code)
	}
	backup := kept.editorSessionBackup
	index := editorSessionIndex{Version: 1, Active: 0, Tabs: []editorSessionTab{{Kind: "local", Path: path, Dirty: true, Hash: original, Format: &editorbuffer.Format{Encoding: "UTF-8", EOL: "LF"}, SavedFormat: &editorbuffer.Format{Encoding: "UTF-8", EOL: "CRLF"}, Backup: &backup}}}
	if r := putSession(t, a, index); r.Code != http.StatusOK {
		t.Fatal(r.Code, r.Body.String())
	}
	if err := m.releaseDocument("", editorFixtureUID, lease); err != nil {
		t.Fatal(err)
	}

	restore := map[string]any{"backup": backup.ID, "hash": original, "format": index.Tabs[0].SavedFormat}
	child := editorHTTP(a, "wrong-token", "/api/editor-views/buffers/open", map[string]any{"document": document, "mode": "restore", "restore": restore})
	if child.Code != http.StatusUnauthorized {
		t.Fatal("restore accepted an unauthenticated caller", child.Code)
	}
	bad := map[string]any{"backup": "../" + backup.ID[3:], "hash": original, "format": index.Tabs[0].SavedFormat}
	if r := editorHTTP(a, a.token, "/api/editor-views/buffers/open", map[string]any{"document": document, "mode": "restore", "restore": bad}); r.Code != http.StatusBadRequest {
		t.Fatal("restore accepted a traversing copy name", r.Code)
	}
	lease = sessionOpen(t, a, document, restore)
	status = sessionWaitReady(t, a, editorFixtureUID, lease)
	meta := status["metadata"].(map[string]any)
	if meta["dirty"] != true || status["hash"] != original || status["format"].(map[string]any)["eol"] != "CRLF" || status["restore"] != nil && status["restore"].(map[string]any)["changed"] == true {
		t.Fatalf("restored document state: %v", status)
	}
	r = diskCall(a, a.token, lease, map[string]any{"action": "window", "start": 0, "limit": 64})
	if r.Code != http.StatusOK || !strings.HasPrefix(gjsonText(t, r.Body.Bytes()), "UNSAVED-HEAD\ngenerated session record") {
		t.Fatal("restored text differs", r.Body.String()[:min(200, r.Body.Len())])
	}
	revision = uint64(meta["revision"].(float64))
	r = diskCall(a, a.token, lease, map[string]any{"action": "save", "revision": revision, "format": index.Tabs[0].SavedFormat})
	if r.Code != http.StatusOK {
		t.Fatal("saving restored text over the unchanged file failed", r.Code, r.Body.String())
	}
	if info, err := os.Stat(path); err != nil || info.Size() != size+int64(len("UNSAVED-HEAD\r\n")) {
		t.Fatal("saved restored file has the wrong size", info, err)
	}
	if err := m.releaseDocument("", editorFixtureUID, lease); err != nil {
		t.Fatal(err)
	}

	// The same kept copy over a file that changed meanwhile is shown, marked as
	// changed, and refuses to overwrite the newer file.
	saved := fileDigest(t, path)
	lease = sessionOpen(t, a, document, restore)
	status = sessionWaitReady(t, a, editorFixtureUID, lease)
	if status["restore"].(map[string]any)["changed"] != true || saved == original {
		t.Fatalf("a file changed since the session was not reported: %v", status["restore"])
	}
	revision = uint64(status["metadata"].(map[string]any)["revision"].(float64))
	if r := diskCall(a, a.token, lease, map[string]any{"action": "save", "revision": revision, "format": index.Tabs[0].SavedFormat}); r.Code != http.StatusConflict || !strings.Contains(r.Body.String(), "changed") {
		t.Fatal("restored text replaced a file that changed after the session", r.Code, r.Body.String())
	}
	if fileDigest(t, path) != saved {
		t.Fatal("the newer file was modified")
	}
	m.releaseDocument("", editorFixtureUID, lease)

	// An unsaved document restores from the same copy without a file baseline.
	draft := emptyDiskFixture()
	lease = sessionOpen(t, a, draft, map[string]any{"backup": backup.ID, "hash": "", "format": map[string]any{"encoding": "UTF-8", "eol": "LF"}})
	status = sessionWaitReady(t, a, editorFixtureUID, lease)
	if status["metadata"].(map[string]any)["dirty"] != true || status["hash"] != "" || status["restore"] == nil {
		t.Fatalf("restored draft state: %v", status)
	}
	m.releaseDocument("", editorFixtureUID, lease)
	if r := editorHTTP(a, a.token, "/api/editor-views/buffers/open", map[string]any{"document": draft, "mode": "restore", "restore": map[string]any{"backup": strings.Repeat("e", 32), "hash": "", "format": map[string]any{"encoding": "UTF-8", "eol": "LF"}}}); r.Code != http.StatusNotFound {
		t.Fatal("restored a copy the session does not name", r.Code)
	}
}

func gjsonText(t *testing.T, raw []byte) string {
	t.Helper()
	var window struct{ Text string }
	if err := json.Unmarshal(raw, &window); err != nil {
		t.Fatal(err)
	}
	return window.Text
}
