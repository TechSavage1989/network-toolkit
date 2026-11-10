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

// TestEditorSessionBrowserOpenedFileIsActive drives the shipped page against
// two production engines on one data folder: the main window and the
// standalone Text Editor. A file opened from outside (Explorer: a start with
// --open-file, or a second start handed over to the running window through
// the launch request inbox) must be the active tab, whatever the kept
// session holds: at a start that also takes over the other editor's tabs,
// in the running window (new file, already open file, several files), and
// at a start whose file is one of the kept tabs. Every kept tab and its
// unsaved text is still reopened. Second starts use the real path
// (prepareStandaloneEditor to the registered window over loopback).
func TestEditorSessionBrowserOpenedFileIsActive(t *testing.T) {
	if os.Getenv("NTK_EDITOR_SESSION_BROWSER") != "1" {
		t.Skip("requires the isolated Playwright browser")
	}
	root, mainApp, editorApp := editorSessionApps(t)
	files := t.TempDir()
	large, small := filepath.Join(files, "large-log.txt"), filepath.Join(files, "small notes.txt")
	opened, second, third := filepath.Join(files, "opened from explorer.txt"), filepath.Join(files, "second file.txt"), filepath.Join(files, "third file.txt")
	var rows strings.Builder
	for i := 0; i < 6000; i++ {
		rows.WriteString("synthetic log record ")
		rows.WriteString(strings.Repeat("x", i%40))
		rows.WriteString("\r\n")
	}
	contents := map[string]string{large: rows.String(), small: "first saved line\n", opened: "opened from outside\n", second: "second file from outside\n", third: "third file from outside\n"}
	for path, text := range contents {
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	closeBuffers := func(a *application) {
		registry := a.editorViews
		registry.mu.Lock()
		var buffers []*editorDiskDocument
		for _, grant := range registry.documents {
			if grant.buffer != nil {
				buffers = append(buffers, grant.buffer)
			}
		}
		clear(registry.documents)
		registry.homeClosing, registry.homeCloseApproved = false, false
		registry.mu.Unlock()
		for _, buffer := range buffers {
			buffer.close()
		}
	}
	// What main() does with the file of a first start (main.go).
	startWith := func(a *application, path string) error {
		if path == "" {
			return nil
		}
		checked, err := editorLaunchPath(path)
		if err != nil {
			return err
		}
		a.editorLaunchInbox.add(editorLaunchRequest{Path: checked})
		return nil
	}
	serve := func(a *application) *httptest.Server {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fixture := strings.HasPrefix(r.URL.Path, "/api/editor-fixture/")
			var body struct {
				Open string `json:"open"`
			}
			if fixture {
				if r.Header.Get("X-Session-Token") != a.token {
					jsonError(w, http.StatusForbidden, "Fixture session required.")
					return
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
			}
			switch {
			case r.URL.Path == "/api/status":
				jsonResponse(w, 200, map[string]any{"version": model.Version, "os": runtime.GOOS, "arch": runtime.GOARCH, "windowMode": a.windowMode, "editorWindowAvailable": true, "editorDiskAvailable": true, "editorSessionAvailable": true, "editorLaunchAvailable": a.editorLaunchAvailable, "editorLaunchReusable": a.editorLaunchReusable, "uiHost": "browser", "system": map[string]any{}})
			case r.URL.Path == "/api/state":
				jsonResponse(w, 200, map[string]any{"activePage": "overview", "appUpdatePreferences": map[string]any{"enabled": false}})
			case r.URL.Path == "/api/jobs":
				jsonResponse(w, 200, []any{})
			case r.URL.Path == "/api/shutdown":
				jsonResponse(w, 200, map[string]any{"ok": true, "detached": false})
			case r.URL.Path == "/api/editor-fixture/exit":
				// The app was closed: its buffers are gone and its lock on its
				// editor session is released, so the other editor may take over.
				closeBuffers(a)
				a.stop()
				for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
					a.editorSession.leaseMu.Lock()
					released := a.editorSession.lease == nil
					a.editorSession.leaseMu.Unlock()
					if released {
						break
					}
					if time.Now().After(deadline) {
						jsonError(w, 500, "the closed app kept its editor session lock")
						return
					}
				}
				jsonResponse(w, 200, map[string]bool{"ok": true})
			case r.URL.Path == "/api/editor-fixture/restart":
				// The next start of this app, optionally with a file: no buffers
				// and no in-memory session state.
				closeBuffers(a)
				a.editorSession.mu.Lock()
				a.editorSession.pending, a.editorSession.writing, a.editorSession.known, a.editorSession.unreadable = nil, nil, nil, false
				a.editorSession.mu.Unlock()
				a.editorLaunchInbox.take()
				if err := startWith(a, body.Open); err != nil {
					jsonError(w, 500, err.Error())
					return
				}
				jsonResponse(w, 200, map[string]bool{"ok": true})
			case r.URL.Path == "/api/editor-fixture/second-start":
				// Another start with a file while this app runs: the real path of
				// "Network_Toolkit.exe --editor-standalone --open-file <file>".
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
		return server
	}
	mainServer, editorServer := serve(mainApp), serve(editorApp)
	defer mainServer.Close()
	defer editorServer.Close()
	defer closeBuffers(mainApp)
	defer closeBuffers(editorApp)

	// The first start of the standalone Text Editor, with a file from
	// Explorer: it owns its workspace, registers its window and holds the file
	// for its page (main.go).
	dir, release, reused, err := prepareStandaloneEditor(root, &editorLaunchRequest{Path: opened})
	if err != nil || reused || dir != filepath.Dir(editorApp.dataPath) {
		t.Fatalf("standalone Text Editor workspace: %s (%v, reused=%t)", dir, err, reused)
	}
	defer release()
	unregister, err := editorApp.registerWorkspaceWindow(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer unregister()
	editorApp.editorLaunchAvailable, editorApp.editorLaunchReusable = true, true
	if err := startWith(editorApp, opened); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "node", "scripts/editor-session-open-file.browser.cjs")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(), "NTK_EDITOR_MAIN_URL="+mainServer.URL, "NTK_EDITOR_MAIN_TOKEN="+mainApp.token, "NTK_EDITOR_STANDALONE_URL="+editorServer.URL, "NTK_EDITOR_STANDALONE_TOKEN="+editorApp.token,
		"NTK_EDITOR_SESSION_LARGE="+large, "NTK_EDITOR_SESSION_SMALL="+small, "NTK_EDITOR_SESSION_OPENED="+opened, "NTK_EDITOR_SESSION_SECOND="+second, "NTK_EDITOR_SESSION_THIRD="+third)
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	// Opening files from outside and reopening a session never write a file.
	for path, text := range contents {
		if current, err := os.ReadFile(path); err != nil || string(current) != text {
			t.Fatalf("%s changed on disk: %v", filepath.Base(path), err)
		}
	}
	// The main window's tabs were moved to the standalone editor, which still
	// keeps all of them, the unsaved ones with their kept copies.
	if left := diskSession(t, mainApp); len(left.Tabs) != 0 {
		t.Fatalf("tabs stayed with the closed main window: %s", tabSummary(left))
	}
	if kept := tabSummary(diskSession(t, editorApp)); kept != "local:large-log.txt* local:small notes.txt* untitled:Untitled.txt* local:opened from explorer.txt local:second file.txt local:third file.txt" {
		t.Fatalf("the standalone editor's session: %s", kept)
	}
}
