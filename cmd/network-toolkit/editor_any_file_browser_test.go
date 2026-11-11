package main

import (
	"bytes"
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

// TestEditorSessionBrowserAnyFile drives the shipped page of the standalone
// Text Editor against its production engine with files Windows would hand to
// a default editor: a program, text without an extension, text in an unknown
// type, Windows-1252 text, UTF-16 text, an empty file and a folder. The first
// file arrives the way Explorer starts the app (--open-file), a later one
// through a real second start. The page must show the program read-only and
// never write it, and every saved text file must hold exactly the bytes that
// were read plus what was typed. The script is
// scripts/editor-any-file.browser.cjs.
func TestEditorSessionBrowserAnyFile(t *testing.T) {
	if os.Getenv("NTK_EDITOR_SESSION_BROWSER") != "1" {
		t.Skip("requires the isolated Playwright browser")
	}
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("requires the platform file lock")
	}
	root := t.TempDir()
	a := sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode)
	files := t.TempDir()
	program := append([]byte("MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xff\xff\x00\x00\xb8\x00"), bytes.Repeat([]byte{0, 0xff, 0x10, '\r', '\n', 'P', 'E'}, 3000)...)
	mixed := "interface a\r\n shutdown\n!\r\ninterface b\r\n"
	legacy := "name=caf\xe9\r\nprice=5\x80\r\n"
	wide := []byte{0xff, 0xfe}
	for _, r := range "wide text\r\nsecond\r\n" {
		wide = append(wide, byte(r), 0)
	}
	contents := map[string][]byte{
		"tool.exe":       program,
		"notes.zz9":      []byte("plain words in an unknown type\n"),
		"running-config": []byte(mixed),
		"legacy.dat":     []byte(legacy),
		"wide.unknown":   wide,
		"empty":          nil,
	}
	paths := map[string]string{}
	for name, raw := range contents {
		paths[name] = filepath.Join(files, name)
		if err := os.WriteFile(paths[name], raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	folder := filepath.Join(files, "a folder.txt")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
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
			jsonResponse(w, 200, map[string]any{"version": model.Version, "os": runtime.GOOS, "arch": runtime.GOARCH, "windowMode": a.windowMode, "editorWindowAvailable": true, "editorDiskAvailable": true, "editorSessionAvailable": true, "editorLaunchAvailable": a.editorLaunchAvailable, "editorLaunchReusable": a.editorLaunchReusable, "uiHost": "browser", "system": map[string]any{}})
		case r.URL.Path == "/api/state":
			jsonResponse(w, 200, map[string]any{"activePage": "overview", "appUpdatePreferences": map[string]any{"enabled": false}})
		case r.URL.Path == "/api/jobs":
			jsonResponse(w, 200, []any{})
		case r.URL.Path == "/api/shutdown":
			jsonResponse(w, 200, map[string]any{"ok": true, "detached": false})
		case r.URL.Path == "/api/editor-fixture/second-start":
			// Another start with a file while this app runs: the real path of
			// "Network_Toolkit.exe --editor-standalone --open-file <file>".
			var body struct {
				Open string `json:"open"`
			}
			if r.Header.Get("X-Session-Token") != a.token {
				jsonError(w, http.StatusForbidden, "Fixture session required.")
				return
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			checked, err := editorLaunchPath(body.Open)
			if err != nil {
				jsonError(w, 500, err.Error())
				return
			}
			_, release, reused, err := prepareStandaloneEditor(root, &editorLaunchRequest{Path: checked})
			if err != nil || !reused {
				if release != nil {
					release()
				}
				jsonError(w, 500, "the second start did not reach the running Text Editor")
				return
			}
			jsonResponse(w, 200, map[string]bool{"ok": true})
		case r.URL.Path == editorLaunchEndpoint, strings.HasPrefix(r.URL.Path, "/api/editor-views"), r.URL.Path == "/api/file-editor/local", strings.HasPrefix(r.URL.Path, editorSessionEndpoint), !strings.HasPrefix(r.URL.Path, "/api/"):
			a.ServeHTTP(w, r)
		default:
			jsonResponse(w, 200, map[string]any{"sessions": []any{}, "entries": []any{}})
		}
	}))
	a.host = strings.TrimPrefix(server.URL, "http://")
	defer server.Close()
	defer closeBuffers()

	// The first start, with the program as Explorer's "%1" (main.go).
	dir, release, reused, err := prepareStandaloneEditor(root, &editorLaunchRequest{Path: paths["tool.exe"]})
	if err != nil || reused || dir != filepath.Dir(a.dataPath) {
		t.Fatalf("standalone Text Editor workspace: %s (%v, reused=%t)", dir, err, reused)
	}
	defer release()
	unregister, err := a.registerWorkspaceWindow(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer unregister()
	a.editorLaunchAvailable, a.editorLaunchReusable = true, true
	checked, err := editorLaunchPath(paths["tool.exe"])
	if err != nil {
		t.Fatal(err)
	}
	a.editorLaunchInbox.add(editorLaunchRequest{Path: checked})
	// A folder is refused before any window is asked to open it.
	if _, err := editorLaunchPath(folder); err == nil {
		t.Fatal("a folder was accepted as a file to open")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "node", "scripts/editor-any-file.browser.cjs")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(), "NTK_EDITOR_STANDALONE_URL="+server.URL, "NTK_EDITOR_STANDALONE_TOKEN="+a.token, "NTK_EDITOR_ANY_FILES="+files)
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	// What the script typed and saved, checked on the bytes.
	want := map[string][]byte{
		"tool.exe":       program,
		"notes.zz9":      contents["notes.zz9"],
		"running-config": []byte("Xinterface a\r\n shutdown\n!\r\ninterface b\r\n"),
		"legacy.dat":     []byte("→éname=café\r\nprice=5€\r\n"),
		"wide.unknown":   wide,
		"empty":          nil,
	}
	for name, expected := range want {
		if got, err := os.ReadFile(paths[name]); err != nil || !bytes.Equal(got, expected) {
			t.Errorf("%s on disk (%v):\n got %q\nwant %q", name, err, clipBytes(got), clipBytes(expected))
		}
	}
	// The save in Windows-1252, before the character it cannot hold was typed.
	if got, err := os.ReadFile(filepath.Join(files, "legacy-saved.dat")); err != nil || string(got) != "\xe9"+legacy {
		t.Errorf("the Windows-1252 save: %q (%v)", got, err)
	}
	entries, _ := os.ReadDir(files)
	if len(entries) != len(contents)+2 {
		t.Errorf("the folder holds %d entries, want the %d fixtures, the folder and one copy", len(entries), len(contents))
	}
}

func clipBytes(b []byte) []byte {
	if len(b) > 96 {
		return b[:96]
	}
	return b
}
