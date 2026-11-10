package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"networktoolkit/internal/model"
)

// Keep and restore a Text Editor session through the shipped page and the
// production editor and session handlers, with real disk-backed documents.
// Between the two runs the fixture closes every buffer and forgets in-memory
// session state, as an app restart would.
func TestEditorSessionBrowser(t *testing.T) {
	if os.Getenv("NTK_EDITOR_SESSION_BROWSER") != "1" {
		t.Skip("requires the isolated Playwright browser")
	}
	a, registry, _ := editorFixture(t)
	a.windowMode = "main"
	dir := t.TempDir()
	large, small := filepath.Join(dir, "large-log.txt"), filepath.Join(dir, "small notes.txt")
	var rows strings.Builder
	for i := 0; i < 6000; i++ {
		rows.WriteString("synthetic log record ")
		rows.WriteString(strings.Repeat("x", i%40))
		rows.WriteString("\r\n")
	}
	if err := os.WriteFile(large, []byte(rows.String()), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(small, []byte("first saved line\n"), 0600); err != nil {
		t.Fatal(err)
	}
	closeBuffers := func() {
		registry.mu.Lock()
		var buffers []*editorDiskDocument
		for _, grant := range registry.documents {
			if grant.buffer != nil {
				buffers = append(buffers, grant.buffer)
			}
		}
		clear(registry.documents)
		registry.mu.Unlock()
		for _, buffer := range buffers {
			buffer.close()
		}
	}
	defer closeBuffers()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixture := strings.HasPrefix(r.URL.Path, "/api/editor-fixture/")
		if fixture && r.Header.Get("X-Session-Token") != a.token {
			jsonError(w, http.StatusForbidden, "Fixture session required.")
			return
		}
		switch {
		case r.URL.Path == "/api/status":
			jsonResponse(w, 200, map[string]any{"version": model.Version, "os": runtime.GOOS, "arch": runtime.GOARCH, "editorWindowAvailable": true, "editorDiskAvailable": true, "editorSessionAvailable": true, "uiHost": "browser", "system": map[string]any{}})
		case r.URL.Path == "/api/state":
			jsonResponse(w, 200, map[string]any{"activePage": "overview", "appUpdatePreferences": map[string]any{"enabled": false}})
		case r.URL.Path == "/api/jobs":
			jsonResponse(w, 200, []any{})
		case r.URL.Path == "/api/shutdown":
			jsonResponse(w, 200, map[string]any{"ok": true, "detached": false})
		case r.URL.Path == "/api/editor-fixture/restart":
			// The next launch: no buffers, no in-memory session state, and the
			// small file was changed by another program.
			closeBuffers()
			registry.mu.Lock()
			registry.homeClosing, registry.homeCloseApproved = false, false
			registry.mu.Unlock()
			a.editorSession.mu.Lock()
			a.editorSession.pending, a.editorSession.writing, a.editorSession.known, a.editorSession.unreadable = nil, nil, nil, false
			a.editorSession.mu.Unlock()
			if err := os.WriteFile(small, []byte("changed by another program\n"), 0600); err != nil {
				jsonError(w, 500, err.Error())
				return
			}
			jsonResponse(w, 200, map[string]bool{"ok": true})
		case strings.HasPrefix(r.URL.Path, "/api/editor-views"), r.URL.Path == "/api/file-editor/local", strings.HasPrefix(r.URL.Path, editorSessionEndpoint), !strings.HasPrefix(r.URL.Path, "/api/"):
			a.ServeHTTP(w, r)
		default:
			jsonResponse(w, 200, map[string]any{"sessions": []any{}, "entries": []any{}})
		}
	}))
	defer server.Close()
	a.host = strings.TrimPrefix(server.URL, "http://")

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "node", "scripts/editor-session-disk.browser.cjs")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(), "NTK_EDITOR_FIXTURE_URL="+server.URL, "NTK_EDITOR_FIXTURE_TOKEN="+a.token, "NTK_EDITOR_SESSION_LARGE="+large, "NTK_EDITOR_SESSION_SMALL="+small)
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(large)
	if err != nil || !strings.HasPrefix(string(saved), "EDITED-HEAD synthetic log record \r\n") || len(saved) != len(rows.String())+len("EDITED-HEAD ") {
		t.Fatalf("the restored large file was not saved over its unchanged original: %d bytes, %v", len(saved), err)
	}
	if current, _ := os.ReadFile(small); string(current) != "changed by another program\n" {
		t.Fatal("restored text replaced a file that changed after the last session")
	}
	// The final session names the two documents that still have unsaved text.
	sessionDir := filepath.Join(filepath.Dir(a.dataPath), editorSessionDirectory)
	raw, err := os.ReadFile(filepath.Join(sessionDir, editorSessionIndexName))
	if err != nil {
		t.Fatal(err)
	}
	var index editorSessionIndex
	if err := json.Unmarshal(raw, &index); err != nil {
		t.Fatal(err)
	}
	var kinds []string
	copies := map[string]bool{}
	for _, tab := range index.Tabs {
		kinds = append(kinds, tab.Kind)
		if tab.Backup != nil {
			copies[editorSessionBackupFile(tab.Backup.ID)] = true
		}
	}
	if strings.Join(kinds, ",") != "local,local,untitled" || index.Tabs[0].Dirty || !index.Tabs[1].Dirty || len(copies) != 2 {
		t.Fatalf("final session: %s", raw)
	}
	entries, _ := os.ReadDir(sessionDir)
	for _, entry := range entries {
		if entry.Name() != editorSessionIndexName && !copies[entry.Name()] {
			t.Fatalf("an unreferenced file stayed in the session folder: %s", entry.Name())
		}
	}
}
