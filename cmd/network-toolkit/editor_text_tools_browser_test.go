package main

import (
	"context"
	"encoding/base64"
	"fmt"
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

// TestEditorSessionBrowserTextTools drives the shipped page of the standalone
// Text Editor against its production engine through the Tools menu (owner
// request v1.21.53): the menu and its tooltips, Split at text on page-held
// text, Base64 decode of a whole 1.7 MB single-line document with nothing
// selected, the editor guide's recipe for a one-line encoded settings file
// (Base64 decode stops at the "&&" after the Base64 and offers Decode the
// part before it; Split at text &; URL decode; Undo) on a synthetic 1.5 MB
// file, the Find window's Extended mode, and Undo. The script is
// scripts/editor-text-tools.browser.cjs; afterwards the files on disk must be
// exactly as they were written.
func TestEditorSessionBrowserTextTools(t *testing.T) {
	if os.Getenv("NTK_EDITOR_SESSION_BROWSER") != "1" {
		t.Skip("requires the isolated Playwright browser")
	}
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("requires the platform file lock")
	}
	root := t.TempDir()
	a := sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode)
	files := t.TempDir()
	encodedSettings, _, readable := settingsLine(11000)
	if len(encodedSettings) < 1500000 {
		t.Fatalf("the settings fixture is only %d bytes", len(encodedSettings))
	}
	smallSettings, smallPlain, _ := settingsLine(2)
	var plain strings.Builder
	for i := 0; plain.Len() < 1300000; i++ {
		fmt.Fprintf(&plain, "interface ge-0/0/%d\n description link to 192.0.2.%d\n", i, i%250)
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(plain.String()))
	// 1.5 million unsorted lines: a sort long enough to show its progress and
	// be cancelled.
	var hosts strings.Builder
	for i := 0; i < 1500000; i++ {
		fmt.Fprintf(&hosts, "host-%d.example.com\n", (i*7919)%1500000)
	}
	// One line of 60,000 settings: disk-backed, for Find and Replace.
	var pairs strings.Builder
	for i := 0; i < 60000; i++ {
		fmt.Fprintf(&pairs, "k%d=%d&", i, i%10)
	}
	contents := map[string]string{
		// One line of Base64 followed by "&&": disk-backed, and page-held.
		"settings.b64": encodedSettings + "&&",
		"small.b64":    smallSettings + "&&",
		"large.b64":    encoded,
		"settings.txt": "a=1&b=2&c=3",
		"replace.txt":  "host=a&port=22&user=admin",
		"pairs.txt":    pairs.String(),
		"hosts.txt":    hosts.String(),
		// After Decode the part before it, each step works on the text the
		// step before selected: the "&&" after the Base64 stays as it is.
		"expected-readable.txt": readable + "&&",
		"expected-small.txt":    smallPlain + "&&",
		"expected-b64.txt":      plain.String(),
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/status":
			jsonResponse(w, 200, map[string]any{"version": model.Version, "os": runtime.GOOS, "arch": runtime.GOARCH, "windowMode": a.windowMode, "editorWindowAvailable": true, "editorDiskAvailable": true, "editorSessionAvailable": true, "uiHost": "browser", "system": map[string]any{}})
		case r.URL.Path == "/api/state":
			jsonResponse(w, 200, map[string]any{"activePage": "overview", "appUpdatePreferences": map[string]any{"enabled": false}})
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

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "node", "scripts/editor-text-tools.browser.cjs")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(), "NTK_EDITOR_STANDALONE_URL="+server.URL, "NTK_EDITOR_STANDALONE_TOKEN="+a.token, "NTK_EDITOR_TOOL_FILES="+files)
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	// Nothing was saved: every tool changed only the open documents.
	for _, name := range []string{"settings.b64", "small.b64", "large.b64", "pairs.txt", "hosts.txt"} {
		if got, err := os.ReadFile(filepath.Join(files, name)); err != nil || string(got) != contents[name] {
			t.Errorf("%s changed on disk (%v)", name, err)
		}
	}
}
