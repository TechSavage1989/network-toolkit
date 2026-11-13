package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"networktoolkit/internal/model"
)

// TestEditorSessionBrowserFindWindow drives the shipped page of the
// standalone Text Editor against its production engine through the Find
// window (owner request v1.21.55): each tab and option, the search history,
// swap, transparency (kept across a reload), Count, Find All in the current
// document and in all open documents with the Search results panel (jump,
// collapse, clear, rows drawn only in view for a document with 200,000
// hits), Replace All in all open documents with Undo in each, Mark All with
// its marks, bookmarks and Copy Marked Text, In selection on page-held and
// disk-backed text, wrap-around off, backward search and ". matches
// newline". The script is scripts/editor-find-window.browser.cjs; afterwards
// no file on disk may have changed: Replace All never saves.
func TestEditorSessionBrowserFindWindow(t *testing.T) {
	if os.Getenv("NTK_EDITOR_SESSION_BROWSER") != "1" {
		t.Skip("requires the isolated Playwright browser")
	}
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("requires the platform file lock")
	}
	root := t.TempDir()
	a := sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode)
	files := t.TempDir()
	var config strings.Builder
	config.WriteString("hostname lab-switch-01\n")
	for i := 1; i <= 12; i++ {
		fmt.Fprintf(&config, "interface ge-0/0/%d\n description uplink to 192.0.2.%d\n\tvlan %d\n", i, i, 10*i)
	}
	config.WriteString("end\n")
	// 100,000 lines with two hits each: disk-backed, 200,000 hits.
	var hits strings.Builder
	for i := 0; i < 100000; i++ {
		fmt.Fprintf(&hits, "vlan %d name vlan-%d\n", i%4000, i)
	}
	// About 5 MB of text: disk-backed, for In selection.
	var large strings.Builder
	for i := 0; large.Len() < 5<<20; i++ {
		fmt.Fprintf(&large, "host-%06d.example.com 192.0.2.%d port %d\n", i, i%250, 1000+i%9000)
	}
	contents := map[string]string{
		"config.txt": config.String(),
		"second.txt": "vlan 99\n name lab\nvlan 100\n name voice\n",
		"hits.txt":   hits.String(),
		"large.txt":  large.String(),
		"image.bin":  "\x89PNG\r\n\x1a\n\x00vlan\x00data\x00vlan\x00",
		"blank.txt":  "a\n\nb\n\nc\n",
	}
	for name, text := range contents {
		if err := os.WriteFile(filepath.Join(files, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	closeBuffers := func() {
		registry := a.editorViews
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
	// The saved settings are kept here, so a reload of the page reads what
	// the page saved (the Find window's transparency).
	var stateMu sync.Mutex
	state := []byte(`{"activePage":"overview","appUpdatePreferences":{"enabled":false}}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/status":
			jsonResponse(w, 200, map[string]any{"version": model.Version, "os": runtime.GOOS, "arch": runtime.GOARCH, "windowMode": a.windowMode, "editorWindowAvailable": true, "editorDiskAvailable": true, "editorSessionAvailable": true, "uiHost": "browser", "system": map[string]any{}})
		case r.URL.Path == "/api/state" && r.Method == http.MethodGet:
			stateMu.Lock()
			body := append([]byte(nil), state...)
			stateMu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		case r.URL.Path == "/api/state":
			body, _ := io.ReadAll(io.LimitReader(r.Body, 8<<20))
			if json.Valid(body) {
				stateMu.Lock()
				state = body
				stateMu.Unlock()
			}
			jsonResponse(w, 200, map[string]any{"ok": true})
		case r.URL.Path == "/api/jobs":
			jsonResponse(w, 200, []any{})
		case r.URL.Path == "/api/shutdown":
			jsonResponse(w, 200, map[string]any{"ok": true, "detached": false})
		case strings.HasPrefix(r.URL.Path, "/api/editor-views"), r.URL.Path == "/api/file-editor/local", strings.HasPrefix(r.URL.Path, editorSessionEndpoint), !strings.HasPrefix(r.URL.Path, "/api/"):
			a.ServeHTTP(w, r)
		default:
			jsonResponse(w, 200, map[string]any{"sessions": []any{}, "entries": []any{}})
		}
	}))
	a.host = strings.TrimPrefix(server.URL, "http://")
	defer server.Close()
	defer closeBuffers()
	dir, release, reused, err := prepareStandaloneEditor(root, nil)
	if err != nil || reused || dir != filepath.Dir(a.dataPath) {
		t.Fatalf("standalone Text Editor workspace: %s (%v, reused=%t)", dir, err, reused)
	}
	defer release()
	unregister, err := a.registerWorkspaceWindow(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer unregister()

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "node", "scripts/editor-find-window.browser.cjs")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(), "NTK_EDITOR_STANDALONE_URL="+server.URL, "NTK_EDITOR_STANDALONE_TOKEN="+a.token, "NTK_EDITOR_FIND_FILES="+files)
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	// Replace All changes open documents only: nothing was saved.
	for name, text := range contents {
		if got, err := os.ReadFile(filepath.Join(files, name)); err != nil || string(got) != text {
			t.Errorf("%s changed on disk (%v)", name, err)
		}
	}
	// The search history never reaches the saved settings.
	stateMu.Lock()
	saved := string(state)
	stateMu.Unlock()
	for _, secret := range []string{"uplink to", "lab-switch", "vlan-9999"} {
		if strings.Contains(saved, secret) {
			t.Errorf("the saved settings hold search text %q", secret)
		}
	}
	if !strings.Contains(saved, `"findTransparency"`) {
		t.Error("the Find window's transparency was not saved")
	}
}
