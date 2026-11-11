package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// The shipped pages of the main Toolkit window and of the standalone Text
// Editor in Chromium, each against its own real engine of one data root (every
// production handler, real loopback HTTP between the engines, real protected
// instance records, real local files under a temporary folder). Popping the
// main window's editor out must add its local and untitled tabs to the running
// Text Editor, keep their unsaved text there before the main window lets go,
// and still send a tab that cannot be carried to a connected window. Only the
// update routes are synthetic, and a connected window is recorded instead of
// started.
func TestEditorHandoverBrowser(t *testing.T) {
	if os.Getenv("NTK_PEER_SEND_BROWSER") != "1" {
		t.Skip("requires the isolated Playwright browser")
	}
	quietUpdates := func(a *application) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/updates/") {
				jsonResponse(w, http.StatusOK, map[string]any{})
				return
			}
			a.ServeHTTP(w, r)
		})
	}
	root := t.TempDir()
	engine := func(mode string) *application {
		t.Helper()
		dir := peerWorkspaceDir(root, mode)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		settings := `{"activePage":"editor","theme":"light","welcomeDismissed":true,"firstLaunchWelcome":{"dismissed":true},"appUpdatePreferences":{"enabled":false}}`
		if err := os.WriteFile(filepath.Join(dir, "Network_Toolkit.settings.json"), []byte(settings), 0600); err != nil {
			t.Fatal(err)
		}
		a, _ := peerTestAppWith(t, root, mode, quietUpdates)
		return a
	}
	main := engine("main")
	var launchMu sync.Mutex
	launches := 0
	main.launchEditorView = func(context.Context, editorViewLaunch) (<-chan error, error) {
		launchMu.Lock()
		launches++
		launchMu.Unlock()
		return nil, nil
	}
	editor := engine(standaloneEditorMode)
	peerRegister(t, main, root)
	peerRegister(t, editor, root)

	folder := t.TempDir()
	files := map[string]string{"notes": "hostname lab-switch-01\r\ninterface Gi0/1\r\n", "clean": "hostname lab-core-01\n", "changed": "before\n"}
	paths := map[string]string{}
	for name, text := range files {
		paths[name] = filepath.Join(folder, name+".txt")
		if err := os.WriteFile(paths[name], []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	encoded, _ := json.Marshal(paths)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "node", "scripts/editor-handover-live.browser.cjs")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(),
		"NTK_HANDOVER_MAIN_URL=http://"+main.host, "NTK_HANDOVER_MAIN_TOKEN="+main.token,
		"NTK_HANDOVER_EDITOR_URL=http://"+editor.host, "NTK_HANDOVER_EDITOR_TOKEN="+editor.token,
		"NTK_HANDOVER_FILES="+string(encoded))
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	launchMu.Lock()
	started := launches
	launchMu.Unlock()
	if started < 1 {
		t.Fatal("the tab that could not be carried never asked for a connected window")
	}
	// The receiving editor kept the unsaved text in its own session.
	raw, err := os.ReadFile(filepath.Join(peerWorkspaceDir(root, standaloneEditorMode), editorSessionDirectory, editorSessionIndexName))
	if err != nil {
		t.Fatalf("the Text Editor kept no session: %v", err)
	}
	var index editorSessionIndex
	if err = json.Unmarshal(raw, &index); err != nil {
		t.Fatal(err)
	}
	kept := map[string]bool{}
	for _, tab := range index.Tabs {
		if tab.Backup != nil {
			name := tab.Name
			if tab.Path != "" {
				name = filepath.Base(tab.Path)
			}
			kept[tab.Kind+":"+name] = true
		}
	}
	if !kept["local:notes.txt"] || !kept["untitled:Untitled.txt"] {
		t.Fatalf("the unsaved text of the handed-over tabs is not kept: %s", raw)
	}
	// Nothing was saved on the way.
	for _, name := range []string{"notes", "clean"} {
		if got, _ := os.ReadFile(paths[name]); string(got) != files[name] {
			t.Fatalf("%s changed on disk: %q", name, got)
		}
	}
	for _, secret := range []string{main.token, main.windowActivationToken, editor.token, editor.windowActivationToken} {
		if bytes.Contains(output, []byte(secret)) {
			t.Fatal("a token appeared in the browser run")
		}
	}
}
