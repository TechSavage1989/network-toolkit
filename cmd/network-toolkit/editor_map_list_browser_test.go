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

// TestEditorSessionBrowserMapList runs scripts/editor-map-list.browser.cjs:
// the Document Map and the Document List (owner request, like Notepad++) in
// the shipped page, and then in the standalone Text Editor against its
// production engine: a 16 MB disk-backed file is mapped by line and scrolled
// through the map with no long task over 50 ms, a 1.7 MB single-line Base64
// export with Wrap on is mapped by character offset, and after a reload the
// engine has kept both settings in the data folder's shared settings.
func TestEditorSessionBrowserMapList(t *testing.T) {
	if os.Getenv("NTK_EDITOR_SESSION_BROWSER") != "1" {
		t.Skip("requires the isolated Playwright browser")
	}
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("requires the platform file lock")
	}
	root := t.TempDir()
	writeFixture(t, filepath.Join(root, appSettingsFilename), `{"theme":"dark","appUpdatePreferences":{"enabled":false},"editorPreferences":{"restoreSession":false}}`)
	a := sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode)
	files := t.TempDir()
	var config strings.Builder
	for i := 0; config.Len() < 16*1024*1024; i++ {
		if i%5 == 0 {
			fmt.Fprintf(&config, "interface GigabitEthernet0/%d\n", i%48)
		} else if i%5 == 4 {
			config.WriteString("!\n")
		} else {
			fmt.Fprintf(&config, " description synthetic uplink %d to 192.0.2.%d\n", i, i%250)
		}
	}
	var plain strings.Builder
	for i := 0; plain.Len() < 1300000; i++ {
		fmt.Fprintf(&plain, "interface ge-0/0/%d\n description link to 192.0.2.%d\n", i, i%250)
	}
	contents := map[string]string{
		"large.cfg":  config.String(),
		"export.b64": base64.StdEncoding.EncodeToString([]byte(plain.String())),
	}
	if len(contents["export.b64"]) < 1700000 {
		t.Fatalf("the Base64 fixture is only %d bytes", len(contents["export.b64"]))
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
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/status":
			jsonResponse(w, 200, map[string]any{"version": model.Version, "os": runtime.GOOS, "arch": runtime.GOARCH, "windowMode": a.windowMode, "editorWindowAvailable": true, "editorDiskAvailable": true, "editorSessionAvailable": true, "uiHost": "browser", "system": map[string]any{}})
		case r.URL.Path == "/api/jobs":
			jsonResponse(w, 200, []any{})
		case r.URL.Path == "/api/shutdown":
			jsonResponse(w, 200, map[string]any{"ok": true, "detached": false})
		case r.URL.Path == "/api/state", strings.HasPrefix(r.URL.Path, "/api/editor-views"), r.URL.Path == "/api/file-editor/local", strings.HasPrefix(r.URL.Path, editorSessionEndpoint), !strings.HasPrefix(r.URL.Path, "/api/"):
			a.ServeHTTP(w, r)
		default:
			jsonResponse(w, 200, map[string]any{"sessions": []any{}, "entries": []any{}})
		}
	}))
	a.host = server.Listener.Addr().String()
	server.Start()
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "node", "scripts/editor-map-list.browser.cjs")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(), "NTK_EDITOR_STANDALONE_URL="+server.URL, "NTK_EDITOR_STANDALONE_TOKEN="+a.token, "NTK_EDITOR_MAP_FILES="+files)
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	// Both settings are shared editor preferences, kept in the data folder's
	// settings file (sharedDataKeys: editorPreferences, editorFilesPreferences).
	disk := sharedDisk(t, root)
	prefs, _ := disk["editorPreferences"].(map[string]any)
	sidebar, _ := disk["editorFilesPreferences"].(map[string]any)
	if prefs["documentMap"] != true || sidebar["panel"] != "documents" {
		t.Fatalf("the shared settings do not keep the map and the list: editorPreferences=%v editorFilesPreferences=%v", prefs, sidebar)
	}
	for name, text := range contents {
		if got, err := os.ReadFile(filepath.Join(files, name)); err != nil || string(got) != text {
			t.Errorf("%s changed on disk (%v)", name, err)
		}
	}
}
