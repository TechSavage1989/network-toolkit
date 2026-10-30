//go:build windows

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// No main GUI is ever created. The real owner backend remains, as in normal
// detached lifetime; the only GUI is the separately built production child.
func TestNativeEditorLocal119InstalledWebView2(t *testing.T) {
	if os.Getenv("NETWORK_TOOLKIT_NATIVE_LOCAL119_TEST") != "1" {
		t.Skip("explicit hosted Windows child-local acceptance only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	private, err := os.MkdirTemp("", "network-toolkit-local119-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(private)
	files := filepath.Join(private, "files")
	if err = os.MkdirAll(files, 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(files, "synthetic-local119.txt")
	if err = os.WriteFile(source, []byte("SYNTHETIC LOCAL119 ORIGINAL"), 0600); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(private, "Network_Toolkit.local119.exe")
	build := exec.CommandContext(ctx, "go", "build", "-trimpath", "-ldflags=-H=windowsgui", "-o", executable, "./cmd/network-toolkit")
	build.Dir = root
	if output, e := build.CombinedOutput(); e != nil {
		t.Fatalf("build child: %v %s", e, output)
	}
	driver, err := os.ReadFile(filepath.Join(root, "scripts", "editor-local119-native-driver.js"))
	if err != nil {
		t.Fatal(err)
	}
	a := newApplication(ctx, filepath.Join(private, "owner-data", "Network_Toolkit.settings.json"))
	defer a.stop()
	a.uiHost = "webview2"
	a.state = json.RawMessage(`{"activePage":"editor","appUpdatePreferences":{"enabled":false}}`)
	output := os.Getenv("TOOLKIT_NATIVE_TEST_OUTPUT")
	if output == "" {
		output = filepath.Join(private, "evidence")
	}
	if err = os.MkdirAll(output, 0700); err != nil {
		t.Fatal(err)
	}
	h := &nativeEditorFixture{ctx: ctx, app: a, root: root, executable: executable, output: output, driver: driver, queues: map[string]chan nativeEditorCommand{}, replies: map[uint64]chan nativeEditorReply{}, tokens: map[string]string{}, children: map[string]*nativeEditorChild{}}
	a.launchEditorView = h.launch
	defer h.cleanup()
	within := func(p string) bool {
		rel, e := filepath.Rel(files, p)
		return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		local := r.URL.Path == "/api/transfer/local" || r.URL.Path == "/api/file-editor/local" || strings.HasPrefix(r.URL.Path, "/api/editor-views/documents/") && strings.HasSuffix(r.URL.Path, "/file")
		if local {
			if r.Method == http.MethodGet {
				q := r.URL.Query()
				p := q.Get("path")
				if p == "" {
					p = files
					q.Set("path", p)
					r.URL.RawQuery = q.Encode()
				}
				if !within(p) {
					http.Error(w, "fixture path outside synthetic root", 403)
					return
				}
			}
			if r.Method == http.MethodPost {
				body, e := io.ReadAll(io.LimitReader(r.Body, 1<<20))
				if e != nil {
					http.Error(w, "fixture body", 400)
					return
				}
				var value struct {
					Path string `json:"path"`
				}
				if json.Unmarshal(body, &value) != nil || !within(value.Path) {
					http.Error(w, "fixture path outside synthetic root", 403)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
			}
			a.ServeHTTP(w, r)
			return
		}
		h.serve(w, r)
	}))
	defer server.Close()
	a.host = strings.TrimPrefix(server.URL, "http://")
	request := editorTransferRequest{ID: randomHex(16), Destination: "new", Batch: editorFixtureBatch()}
	out, err := a.editorViews.offer("", request)
	if err != nil {
		t.Fatal(err)
	}
	child := out["destination"].(string)
	if _, err = h.waitState(child, "production child owns initial document", func(s nativeEditorSnapshot) bool { return s.Window.Owned[editorFixtureUID] >= 2 && !s.Window.Running }); err != nil {
		t.Fatal(err)
	}
	if err = a.editorViews.complete("", request.ID); err != nil {
		t.Fatal(err)
	}
	if err = a.editorViews.beginHomeClose(); err != nil {
		t.Fatal(err)
	}
	if err = a.editorViews.approveHomeClose(); err != nil {
		t.Fatal(err)
	}
	a.editorViews.setHomePresent(false)
	var result nativeEditorSnapshot
	result, err = h.command(child, nativeEditorCommand{Action: "local119", Destination: files})
	if err == nil {
		var got []byte
		got, err = os.ReadFile(source)
		if err == nil && string(got) != "SYNTHETIC LOCAL119 ORIGINAL\nSYNTHETIC SAVED WITHOUT MAIN" {
			err = fmt.Errorf("saved source bytes differ: %q", got)
		}
	}
	if err == nil {
		err = h.capture(child, "editor-local119-child-no-main")
	}
	a.editorViews.mu.Lock()
	remoteApproved := a.editorViews.views[child].explorer != nil && a.editorViews.views[child].explorer.Allowed
	homePresent := a.editorViews.homePresent
	a.editorViews.mu.Unlock()
	if remoteApproved || homePresent {
		err = fmt.Errorf("local access acquired remote approval or required main")
	}
	report := map[string]any{"passed": err == nil, "actualProductionChild": true, "actualWebView2": true, "mainWindowCreated": false, "mainMarkedClosed": !homePresent, "remoteApproved": remoteApproved, "realLocalReadAndLeaseSave": err == nil, "syntheticTemporaryFilesOnly": true, "captures": h.captures, "errors": result.Errors, "scope": "Separate production editor child, actual application backend with home closed; test-only DOM actions; no standalone backend-process-loss claim"}
	if err != nil {
		report["error"] = err.Error()
	}
	raw, _ := json.MarshalIndent(report, "", "  ")
	os.WriteFile(filepath.Join(output, "editor-local119-native-results.json"), raw, 0600)
	if err != nil {
		t.Fatal(err)
	}
}
