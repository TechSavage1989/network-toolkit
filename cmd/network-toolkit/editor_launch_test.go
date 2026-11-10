package main

import (
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestEditorLaunchCLI(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "résumé & notes %[v2].txt")
	if err := os.WriteFile(path, []byte("Synthetic text\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		name         string
		args         []string
		incompatible bool
		want         *editorLaunchRequest
		wantError    bool
	}{
		{name: "ordinary app"},
		{name: "editor shortcut", args: []string{"--editor"}, want: &editorLaunchRequest{}},
		{name: "Explorer file", args: []string{"--open-file", path}, want: &editorLaunchRequest{Path: path}},
		{name: "missing file", args: []string{"--open-file", filepath.Join(root, "missing.txt")}, wantError: true},
		{name: "empty filename", args: []string{"--open-file", ""}, wantError: true},
		{name: "directory", args: []string{"--open-file", root}, wantError: true},
		{name: "another mode", args: []string{"--open-file", path}, incompatible: true, wantError: true},
		{name: "unexpected arguments", args: []string{"--editor", path}, wantError: true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			flags := flag.NewFlagSet("editor launch fixture", flag.ContinueOnError)
			options := registerEditorLaunchOptions(flags)
			if err := flags.Parse(fixture.args); err != nil {
				t.Fatal(err)
			}
			got, err := options.request(flags, fixture.incompatible)
			if (err != nil) != fixture.wantError || !reflect.DeepEqual(got, fixture.want) {
				t.Fatalf("got %+v, %v, want %+v (error=%t)", got, err, fixture.want, fixture.wantError)
			}
		})
	}
	args := updateRestartArgs([]string{"--editor", "--open-file", path, "--data-dir", root, "-open-file=" + path})
	if !reflect.DeepEqual(args, []string{"--data-dir", root}) {
		t.Fatalf("restart arguments retained a one-shot launch: %v", args)
	}
	if args := updateRestartArgs([]string{"--data-dir", "--open-file", "--editor"}); !reflect.DeepEqual(args, []string{"--data-dir", "--open-file"}) {
		t.Fatalf("restart arguments lost a literal settings folder: %v", args)
	}
}

func TestEditorLaunchReusesRegisteredWorkspace(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("requires the platform workspace lock")
	}
	a := testApplication(t)
	a.editorLaunchAvailable, a.editorLaunchReusable = true, true
	server := httptest.NewServer(a)
	defer server.Close()
	a.host = strings.TrimPrefix(server.URL, "http://")
	root := t.TempDir()
	release, reused, err := prepareEditorWorkspace(root, nil)
	if err != nil || reused || release == nil {
		t.Fatalf("first workspace ownership: %v (reused=%t)", err, reused)
	}
	defer release()
	unregister, err := a.registerWorkspaceWindow(root)
	if err != nil {
		t.Fatal(err)
	}
	defer unregister()

	file := filepath.Join(root, "external & synthetic.txt")
	if err := os.WriteFile(file, []byte("Synthetic text\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, request := range []editorLaunchRequest{{Path: file}, {Path: file}, {}} {
		nextRelease, nextReused, err := prepareEditorWorkspace(root, &request)
		if err != nil || !nextReused || nextRelease != nil {
			t.Fatalf("later launch failed to reuse workspace: %v (reused=%t)", err, nextReused)
		}
	}
	select {
	case <-a.windowActivate:
	default:
		t.Fatal("the live window was not raised")
	}
	if response := requestApp(a, http.MethodPost, editorLaunchEndpoint, "", false); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated editor inbox returned %d", response.Code)
	}
	activation := httptest.NewRequest(http.MethodPost, server.URL+"/api/state", strings.NewReader("{}"))
	activation.Header.Set("X-Window-Activation-Token", a.windowActivationToken)
	protected := httptest.NewRecorder()
	a.ServeHTTP(protected, activation)
	if protected.Code != http.StatusUnauthorized {
		t.Fatalf("activation token accessed regular API: %d", protected.Code)
	}
	if html := requestApp(a, http.MethodGet, "/", "", false); strings.Contains(html.Body.String(), filepath.Base(file)) {
		t.Fatal("the shell file name appeared in the page bootstrap")
	}
	a.updateFinalizing.Store(true)
	if response := requestApp(a, http.MethodPost, editorLaunchEndpoint, "", true); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("launch inbox during update returned %d", response.Code)
	}
	a.updateFinalizing.Store(false)
	next := func() []editorLaunchRequest {
		t.Helper()
		var result struct{ Requests []editorLaunchRequest }
		response := requestApp(a, http.MethodPost, editorLaunchEndpoint, "", true)
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &result) != nil {
			t.Fatalf("editor inbox returned %d: %s", response.Code, response.Body)
		}
		return result.Requests
	}
	if got := next(); !reflect.DeepEqual(got, []editorLaunchRequest{{Path: file}, {}}) {
		t.Fatalf("unexpected inbox: %+v", got)
	}
	if len(next()) != 0 {
		t.Fatal("launch requests must be consumed only once")
	}
}
