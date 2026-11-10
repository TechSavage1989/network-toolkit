package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"networktoolkit/internal/macros"
)

type knownHostFixture struct {
	Host        string `json:"host"`
	KeyType     string `json:"keyType"`
	Fingerprint string `json:"fingerprint"`
	AddedAt     string `json:"addedAt"`
	UpdatedAt   string `json:"updatedAt"`
	Key         []byte `json:"key"`
}

func knownHostFixtureKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func sshFingerprint(key ssh.PublicKey) string { return ssh.FingerprintSHA256(key) }

func knownHostFixtureEntry(host string, key ssh.PublicKey) knownHostFixture {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return knownHostFixture{Host: host, KeyType: key.Type(), Fingerprint: ssh.FingerprintSHA256(key), AddedAt: now, UpdatedAt: now, Key: key.Marshal()}
}

func writeKnownHostFixture(t *testing.T, dir string, entries ...knownHostFixture) {
	t.Helper()
	if entries == nil {
		entries = []knownHostFixture{}
	}
	raw, err := json.MarshalIndent(map[string]any{"version": 1, "hosts": entries}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Network_Toolkit.ssh_known_hosts.json"), append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}

func knownHostsOf(t *testing.T, a *application) []string {
	t.Helper()
	r := requestApp(a, "GET", "/api/terminals/known-hosts", "", true)
	var listed struct {
		Hosts []struct {
			Host string `json:"host"`
		} `json:"hosts"`
	}
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &listed) != nil {
		t.Fatalf("known hosts: %d %s", r.Code, r.Body.String())
	}
	out := []string{}
	for _, row := range listed.Hosts {
		out = append(out, row.Host)
	}
	return out
}

func TestSharedKnownHostsOneTrustFilePerDataFolder(t *testing.T) {
	root := t.TempDir()
	core, edge, lab := knownHostFixtureKey(t), knownHostFixtureKey(t), knownHostFixtureKey(t)
	writeKnownHostFixture(t, root, knownHostFixtureEntry("core.test:22", core), knownHostFixtureEntry("edge.test:22", edge))
	// A trust file left in the standalone Terminal's own folder by an earlier
	// version is not what the Terminal uses any more.
	writeKnownHostFixture(t, filepath.Join(root, terminalWorkspaceName), knownHostFixtureEntry("old-only.test:22", lab))
	main := sharedTestApp(t, root, "", "main")
	terminal := sharedTestApp(t, root, terminalWorkspaceName, "terminal")
	editor := sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode)
	for _, app := range []*application{main, terminal, editor} {
		if got := knownHostsOf(t, app); !sameIDs(got, "core.test:22", "edge.test:22") {
			t.Fatalf("%s trusts %v", app.windowMode, got)
		}
	}
	// Removing a host in one app removes it for all of them, at once.
	if r := requestApp(terminal, "DELETE", "/api/terminals/known-hosts", `{"host":"core.test:22"}`, true); r.Code != 200 {
		t.Fatalf("remove: %d %s", r.Code, r.Body.String())
	}
	for _, app := range []*application{main, terminal, editor} {
		if got := knownHostsOf(t, app); !sameIDs(got, "edge.test:22") {
			t.Fatalf("%s still trusts %v after a removal in the Terminal", app.windowMode, got)
		}
	}
	// A host trusted meanwhile by another app is kept by a later removal here:
	// removals work on the file as it is, not on what this app loaded.
	writeKnownHostFixture(t, root, knownHostFixtureEntry("edge.test:22", edge), knownHostFixtureEntry("lab.test:22", lab))
	if r := requestApp(main, "DELETE", "/api/terminals/known-hosts", `{"host":"edge.test:22"}`, true); r.Code != 200 {
		t.Fatalf("remove: %d %s", r.Code, r.Body.String())
	}
	for _, app := range []*application{main, terminal, editor} {
		if got := knownHostsOf(t, app); !sameIDs(got, "lab.test:22") {
			t.Fatalf("%s trusts %v", app.windowMode, got)
		}
	}
	// The old workspace file is left exactly as it was.
	if _, err := os.Stat(filepath.Join(root, terminalWorkspaceName, "Network_Toolkit.ssh_known_hosts.json")); err != nil {
		t.Fatalf("the Terminal's previous trust file was removed: %v", err)
	}
	// Interleaved removals from two apps: every removal lands, nothing else
	// goes missing.
	entries := []knownHostFixture{knownHostFixtureEntry("keep.test:22", core)}
	for n := 0; n < 16; n++ {
		entries = append(entries, knownHostFixtureEntry(fmt.Sprintf("drop%d.test:22", n), edge))
	}
	writeKnownHostFixture(t, root, entries...)
	var wg sync.WaitGroup
	for n := 0; n < 16; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			app := main
			if n%2 == 1 {
				app = terminal
			}
			if r := requestApp(app, "DELETE", "/api/terminals/known-hosts", fmt.Sprintf(`{"host":"drop%d.test:22"}`, n), true); r.Code != 200 {
				t.Errorf("remove drop%d: %d %s", n, r.Code, r.Body.String())
			}
		}(n)
	}
	wg.Wait()
	if got := knownHostsOf(t, editor); !sameIDs(got, "keep.test:22") {
		t.Fatalf("after interleaved removals: %v", got)
	}
}

func TestSharedMacroLibraryOnePerDataFolderWithRevisions(t *testing.T) {
	root := t.TempDir()
	main := sharedTestApp(t, root, "", "main")
	terminal := sharedTestApp(t, root, terminalWorkspaceName, "terminal")
	const path = "/api/terminal-automation/library"
	load := func(a *application) terminalMacroLibrary {
		return macroHTTPLibrary(t, requestApp(a, http.MethodGet, path, "", true), http.StatusOK)
	}
	library := load(terminal)
	library.Macros = append(library.Macros, macroHTTPDefinition("from-terminal"))
	saved := macroHTTPLibrary(t, viewTestRequest(terminal, terminal.token, http.MethodPut, path, library, nil), http.StatusOK)
	if _, err := os.Stat(filepath.Join(root, terminalMacroLibraryFilename)); err != nil {
		t.Fatalf("the Terminal's macro was not saved in the data folder: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, terminalWorkspaceName, terminalMacroLibraryFilename)); !os.IsNotExist(err) {
		t.Fatalf("a macro library was written to the standalone workspace: %v", err)
	}
	inMain := load(main)
	if len(inMain.Macros) != 1 || inMain.Macros[0].ID != "from-terminal" || inMain.Revision != saved.Revision {
		t.Fatalf("the main window does not see the Terminal's macro: %+v", inMain)
	}
	// The main window saves; the Terminal, still holding the older revision,
	// is refused instead of overwriting the main window's macro.
	inMain.Macros = append(inMain.Macros, macroHTTPDefinition("from-main"))
	macroHTTPLibrary(t, viewTestRequest(main, main.token, http.MethodPut, path, inMain, nil), http.StatusOK)
	saved.Macros = append(saved.Macros, macroHTTPDefinition("lost-update"))
	if r := viewTestRequest(terminal, terminal.token, http.MethodPut, path, saved, nil); r.Code != http.StatusConflict {
		t.Fatalf("a save from an old revision was accepted: %d %s", r.Code, r.Body.String())
	}
	if got := load(terminal); len(got.Macros) != 2 {
		t.Fatalf("library after a refused save: %+v", got.Macros)
	}

	// Both apps add macros at the same time, each reloading and retrying when
	// its revision is stale, as the page does. Nothing is lost.
	const perApp = 10
	var wg sync.WaitGroup
	for index, app := range []*application{main, terminal} {
		wg.Add(1)
		go func(index int, app *application) {
			defer wg.Done()
			for n := 0; n < perApp; n++ {
				for attempt := 0; ; attempt++ {
					current := load(app)
					current.Macros = append(current.Macros, macroHTTPDefinition(fmt.Sprintf("app%d-macro%d", index, n)))
					r := viewTestRequest(app, app.token, http.MethodPut, path, current, nil)
					if r.Code == http.StatusOK {
						break
					}
					if r.Code != http.StatusConflict || attempt > 200 {
						t.Errorf("save: %d %s", r.Code, r.Body.String())
						return
					}
				}
			}
		}(index, app)
	}
	wg.Wait()
	final := load(sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode))
	ids := map[string]bool{}
	for _, definition := range final.Macros {
		ids[definition.ID] = true
	}
	if len(final.Macros) != 2+2*perApp || len(ids) != len(final.Macros) {
		t.Fatalf("interleaved macro saves kept %d of %d macros", len(final.Macros), 2+2*perApp)
	}
	var _ macros.Definition
}
