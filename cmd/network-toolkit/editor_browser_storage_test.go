package main

import (
	"bytes"
	"context"
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

// Browser file and draft operations use the real editor routes and generated
// data. Only the browser's file handle and unrelated application startup
// requests are supplied by the fixture.
func TestEditorBrowserStorage(t *testing.T) {
	if os.Getenv("NTK_EDITOR_LARGE_BROWSER") != "1" {
		t.Skip("requires the isolated Playwright browser")
	}
	a, _, _ := editorFixture(t)
	a.windowMode = "main"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/status":
			jsonResponse(w, 200, map[string]any{
				"version": model.Version, "os": runtime.GOOS, "arch": runtime.GOARCH,
				"editorWindowAvailable": true, "editorDiskAvailable": true, "uiHost": "browser",
			})
		case r.URL.Path == "/api/state":
			jsonResponse(w, 200, map[string]any{"activePage": "editor", "appUpdatePreferences": map[string]any{"enabled": false}})
		case r.URL.Path == "/api/jobs":
			jsonResponse(w, 200, []any{})
		case strings.HasPrefix(r.URL.Path, "/api/editor-views"), r.URL.Path == "/api/file-editor/local", !strings.HasPrefix(r.URL.Path, "/api/"):
			a.ServeHTTP(w, r)
		default:
			jsonResponse(w, 200, map[string]any{"sessions": []any{}, "entries": []any{}})
		}
	}))
	defer server.Close()
	a.host = strings.TrimPrefix(server.URL, "http://")
	destination := filepath.Join(t.TempDir(), "promoted-draft.txt")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "node", "scripts/editor-browser-storage.browser.cjs")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(),
		"NTK_EDITOR_FIXTURE_URL="+server.URL,
		"NTK_EDITOR_FIXTURE_TOKEN="+a.token,
		"NTK_EDITOR_DRAFT_DESTINATION="+destination,
	)
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	expected := "seed\r\n" + strings.Repeat("draft fixture row\r\n", 65536) + "DRAFT-EOF"
	if !bytes.Equal(saved, []byte(expected)) {
		t.Fatalf("Save As changed the draft bytes: got %d bytes, want %d", len(saved), len(expected))
	}
}
