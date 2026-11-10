package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"networktoolkit/internal/terminal"
)

func terminalSessionFixture(profileID, name string) terminal.SessionInfo {
	return terminal.SessionInfo{ID: "one", ProfileID: profileID, Type: "ssh", Host: "192.0.2.1", Port: 22, Username: "operator", Label: name, ManualName: name, NameExplicit: true, LabelSource: "manual"}
}

// Several application objects on one temporary data folder stand in for the
// main window, the standalone Terminal and the standalone Text Editor running
// at the same time. Each has its own lock handles, exactly like separate
// processes, so the OS file lock orders their saves.
func sharedTestApp(t *testing.T, root, workspace, mode string) *application {
	t.Helper()
	dir := root
	if workspace != "" {
		dir = filepath.Join(root, workspace)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	a := newApplicationForRoot(context.Background(), filepath.Join(dir, appSettingsFilename), root)
	a.host, a.windowMode = "127.0.0.1:38191", mode
	t.Cleanup(func() {
		a.stop()
		if a.terminals != nil {
			_ = a.terminals.Close()
		}
	})
	return a
}

// sharedPage behaves like the page of one window: it loads its settings once,
// keeps all of them in memory, and writes all of them back.
type sharedPage struct {
	t        *testing.T
	app      *application
	state    map[string]any
	revision string
}

func openSharedPage(t *testing.T, app *application) *sharedPage {
	t.Helper()
	p := &sharedPage{t: t, app: app}
	r := requestApp(app, "GET", "/api/state", "", true)
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &p.state) != nil {
		t.Fatalf("load: %d %s", r.Code, r.Body.String())
	}
	if p.state == nil {
		p.state = map[string]any{}
	}
	for _, name := range []string{"terminalProfiles", "terminalRecent", "ftpProfiles"} {
		if _, ok := p.state[name]; !ok {
			p.state[name] = []any{} // A page always holds every list, empty or not.
		}
	}
	if p.poll() {
		t.Fatal("a page that has just loaded was told its shared values changed")
	}
	return p
}

func (p *sharedPage) apply(shared map[string]any) {
	if revision, _ := shared["revision"].(string); revision != "" {
		p.revision = revision
	}
	if values, ok := shared["values"].(map[string]any); ok {
		for _, key := range sharedDataKeys {
			if !key.Live {
				continue
			}
			if value, present := values[key.Name]; present {
				p.state[key.Name] = value
			} else {
				delete(p.state, key.Name)
			}
		}
	}
}

func (p *sharedPage) poll() bool {
	p.t.Helper()
	r := requestApp(p.app, "GET", "/api/shared-data?revision="+url.QueryEscape(p.revision), "", true)
	var shared map[string]any
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &shared) != nil {
		p.t.Fatalf("poll: %d %s", r.Code, r.Body.String())
	}
	_, changed := shared["values"]
	p.apply(shared)
	return changed
}

func (p *sharedPage) save() {
	p.t.Helper()
	if code, body := p.trySave(); code != 200 {
		p.t.Fatalf("save: %d %s", code, body)
	}
}

func (p *sharedPage) trySave() (int, string) {
	p.t.Helper()
	body := map[string]any{}
	for name, value := range p.state {
		body[name] = value
	}
	if p.revision != "" {
		body[sharedBaseField] = p.revision
	}
	raw, _ := json.Marshal(body)
	r := requestApp(p.app, "PUT", "/api/state", string(raw), true)
	if r.Code != 200 {
		return r.Code, r.Body.String()
	}
	var response struct {
		Shared map[string]any `json:"sharedData"`
	}
	if json.Unmarshal(r.Body.Bytes(), &response) != nil || response.Shared == nil {
		p.t.Fatalf("save response: %s", r.Body.String())
	}
	p.apply(response.Shared)
	return 200, ""
}

func (p *sharedPage) ids(name string) []string {
	out := []string{}
	list, _ := p.state[name].([]any)
	for _, entry := range list {
		row, _ := entry.(map[string]any)
		id, _ := row["id"].(string)
		out = append(out, id)
	}
	return out
}

func (p *sharedPage) row(name, id string) map[string]any {
	list, _ := p.state[name].([]any)
	for _, entry := range list {
		if row, _ := entry.(map[string]any); row["id"] == id {
			return row
		}
	}
	return nil
}

func (p *sharedPage) add(name string, row map[string]any) {
	list, _ := p.state[name].([]any)
	p.state[name] = append(append([]any{}, list...), row)
}

func (p *sharedPage) remove(name, id string) {
	list, _ := p.state[name].([]any)
	kept := []any{}
	for _, entry := range list {
		if row, _ := entry.(map[string]any); row["id"] != id {
			kept = append(kept, entry)
		}
	}
	p.state[name] = kept
}

func sharedProfile(id, name, host string) map[string]any {
	return map[string]any{"id": id, "name": name, "nameExplicit": true, "nameRevision": float64(0), "type": "ssh", "host": host, "port": float64(22), "username": "operator", "authMethod": "password", "sshCompatibility": "modern", "timeoutMs": float64(10000)}
}

func sharedDiskIDs(t *testing.T, root, name string) []string {
	t.Helper()
	settings, err := readRootSettings(filepath.Join(root, appSettingsFilename))
	if err != nil {
		t.Fatal(err)
	}
	out := []string{}
	list, _ := settings.shared()[name].([]any)
	for _, entry := range list {
		row, _ := entry.(map[string]any)
		id, _ := row["id"].(string)
		out = append(out, id)
	}
	return out
}

func sameIDs(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]int{}
	for _, id := range got {
		seen[id]++
	}
	for _, id := range want {
		seen[id]--
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}

func TestSharedStateStalePageNeverRemovesAnotherAppsProfile(t *testing.T) {
	root := t.TempDir()
	main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	editor := openSharedPage(t, sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode))

	// The standalone Terminal saves a connection.
	terminal.add("terminalProfiles", sharedProfile("from-terminal", "Core switch", "192.0.2.10"))
	terminal.save()
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "from-terminal") {
		t.Fatalf("the Terminal's profile is not in the data folder: %v", got)
	}

	// The main window has not heard of it and saves its whole (stale) state,
	// for example after a sidebar was resized.
	main.state["theme"] = "light"
	main.save()
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "from-terminal") {
		t.Fatalf("a stale save removed another app's profile: %v", got)
	}
	// The save's answer gave the page what it was missing.
	if got := main.ids("terminalProfiles"); !sameIDs(got, "from-terminal") {
		t.Fatalf("the saving page did not receive the merged list: %v", got)
	}

	// The other direction, and a third app.
	main.add("terminalProfiles", sharedProfile("from-main", "Edge router", "192.0.2.20"))
	main.save()
	editor.state["editorFilesPreferences"] = map[string]any{"width": float64(320)}
	editor.save()
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "from-terminal", "from-main") {
		t.Fatalf("profiles after saves from three apps: %v", got)
	}

	// Idle pages notice the change.
	if !terminal.poll() || !sameIDs(terminal.ids("terminalProfiles"), "from-terminal", "from-main") {
		t.Fatalf("the Terminal did not notice the main window's profile: %v", terminal.ids("terminalProfiles"))
	}
	if terminal.poll() {
		t.Fatal("an unchanged list was reported as changed")
	}
	if !sameIDs(editor.ids("terminalProfiles"), "from-terminal", "from-main") {
		t.Fatalf("the Text Editor's save did not return the saved connections: %v", editor.ids("terminalProfiles"))
	}

	// A removal is a removal: it reaches the other apps and is not undone by
	// their next save.
	terminal.remove("terminalProfiles", "from-main")
	terminal.save()
	main.state["theme"] = "dark"
	main.save() // stale: still lists from-main
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "from-terminal") {
		t.Fatalf("a removal was undone by a stale save: %v", got)
	}
	if got := main.ids("terminalProfiles"); !sameIDs(got, "from-terminal") {
		t.Fatalf("the main window still lists a removed profile: %v", got)
	}

	// A new process of any app starts with the shared list.
	late := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	if got := late.ids("terminalProfiles"); !sameIDs(got, "from-terminal") {
		t.Fatalf("a newly started Terminal lists %v", got)
	}
}

func TestSharedStateKeepsWindowPreferencesPerWorkspace(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, appSettingsFilename)
	// Main settings with members only the main window uses; one has a number
	// format a careless rewrite would change.
	original := `{"theme":"dark","forms":{"ping":{"count":4}},"bandwidth":{"size":1e3},"_credentialVaultConfigID":"0123456789abcdef0123456789abcdef","terminalProfiles":[{"id":"p","name":"Core","type":"ssh","host":"192.0.2.1","port":22}],"terminalSidebarWidth":300}`
	if err := os.WriteFile(mainPath, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	terminalApp := sharedTestApp(t, root, terminalWorkspaceName, "terminal")
	terminal := openSharedPage(t, terminalApp)
	if !sameIDs(terminal.ids("terminalProfiles"), "p") {
		t.Fatalf("the Terminal does not list the data folder's profile: %v", terminal.state)
	}
	if _, leaked := terminal.state["forms"]; leaked || terminal.state["theme"] != nil || terminal.state[credentialConfigIdentityField] != nil {
		t.Fatalf("main-window preferences reached the Terminal: %v", terminal.state)
	}
	terminal.state["theme"], terminal.state["terminalSidebarWidth"] = "light", float64(420)
	terminal.add("terminalProfiles", sharedProfile("q", "Edge", "192.0.2.2"))
	terminal.save()

	var disk map[string]json.RawMessage
	raw, err := os.ReadFile(mainPath)
	if err != nil || json.Unmarshal(raw, &disk) != nil {
		t.Fatal(err)
	}
	compact := func(name string) string {
		var out bytes.Buffer
		if err := json.Compact(&out, disk[name]); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	if compact("theme") != `"dark"` || compact("terminalSidebarWidth") != `300` || compact("bandwidth") != `{"size":1e3}` || compact("forms") != `{"ping":{"count":4}}` || compact(credentialConfigIdentityField) != `"0123456789abcdef0123456789abcdef"` {
		t.Fatalf("a standalone save changed the main window's own settings: %s", raw)
	}
	if _, added := disk["firstLaunchWelcomeSeen"]; added {
		t.Fatal("an existing main settings file must not gain a first-launch marker")
	}
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "p", "q") {
		t.Fatalf("shared profiles on disk: %v", got)
	}
	own, err := os.ReadFile(terminalApp.dataPath)
	if err != nil {
		t.Fatal(err)
	}
	var workspace map[string]any
	if json.Unmarshal(own, &workspace) != nil || workspace["theme"] != "light" || workspace["terminalSidebarWidth"] != float64(420) {
		t.Fatalf("the Terminal's window preferences were not saved in its own file: %s", own)
	}
	for _, key := range sharedDataKeys {
		if _, present := workspace[key.Name]; present {
			t.Fatalf("the standalone workspace file holds shared data (%s): %s", key.Name, own)
		}
	}
	// The main window keeps its own preferences and sees the new profile.
	main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	if main.state["theme"] != "dark" || !sameIDs(main.ids("terminalProfiles"), "p", "q") {
		t.Fatalf("main window state: %v", main.state)
	}
}

func TestSharedStateStandaloneCreatesMainSettingsWithoutHidingFirstLaunch(t *testing.T) {
	root := t.TempDir()
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	terminal.state["theme"] = "light"
	terminal.save()
	if _, err := os.Lstat(filepath.Join(root, appSettingsFilename)); !os.IsNotExist(err) {
		t.Fatalf("a save without shared data created the main settings file: %v", err)
	}
	terminal.add("terminalProfiles", sharedProfile("p", "Core", "192.0.2.1"))
	terminal.save()
	raw, err := os.ReadFile(filepath.Join(root, appSettingsFilename))
	if err != nil {
		t.Fatal(err)
	}
	var disk map[string]any
	if json.Unmarshal(raw, &disk) != nil || disk["firstLaunchWelcomeSeen"] != false {
		t.Fatalf("the main window would skip its first-launch welcome: %s", raw)
	}
	if strings.Contains(string(raw), "theme") {
		t.Fatalf("a window preference of the Terminal was written to the main settings: %s", raw)
	}
}

func TestSharedStateConcurrentSavesFromThreeAppsLoseNothing(t *testing.T) {
	root := t.TempDir()
	apps := []*application{sharedTestApp(t, root, "", "main"), sharedTestApp(t, root, terminalWorkspaceName, "terminal"), sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode)}
	const perApp = 12
	var wg sync.WaitGroup
	failures := make(chan string, len(apps)*perApp)
	for index, app := range apps {
		wg.Add(1)
		go func(index int, app *application) {
			defer wg.Done()
			page := openSharedPage(t, app)
			for n := 0; n < perApp; n++ {
				id := fmt.Sprintf("app%d-profile%d", index, n)
				page.add("terminalProfiles", sharedProfile(id, id, "192.0.2.50"))
				page.add("ftpProfiles", map[string]any{"id": id, "name": id, "host": "files.test", "username": "operator", "port": float64(21), "security": "explicit", "fingerprint": "", "credentialId": ""})
				if n%3 == 2 {
					// Sometimes a page polls and applies what the others saved,
					// sometimes it saves without having looked.
					page.poll()
				}
				if code, body := page.trySave(); code != 200 {
					failures <- fmt.Sprintf("%s: %d %s", id, code, body)
					return
				}
			}
		}(index, app)
	}
	wg.Wait()
	close(failures)
	for failure := range failures {
		t.Fatal(failure)
	}
	want := []string{}
	for index := range apps {
		for n := 0; n < perApp; n++ {
			want = append(want, fmt.Sprintf("app%d-profile%d", index, n))
		}
	}
	for _, name := range []string{"terminalProfiles", "ftpProfiles"} {
		if got := sharedDiskIDs(t, root, name); !sameIDs(got, want...) {
			t.Fatalf("%s after interleaved saves: %d of %d kept: %v", name, len(got), len(want), got)
		}
	}
	// Every app converges on the same list.
	for _, app := range apps {
		page := openSharedPage(t, app)
		if !sameIDs(page.ids("terminalProfiles"), want...) {
			t.Fatalf("%s lists %d profiles", app.windowMode, len(page.ids("terminalProfiles")))
		}
	}
}

func TestSharedStateRevisionSignalsAndNamedBase(t *testing.T) {
	root := t.TempDir()
	mainApp := sharedTestApp(t, root, "", "main")
	main := openSharedPage(t, mainApp)
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	first := main.revision
	if first == "" || first != terminal.revision {
		t.Fatalf("two apps of one data folder disagree on the revision: %q %q", first, terminal.revision)
	}
	if r := requestApp(mainApp, "POST", "/api/shared-data", "{}", true); r.Code != http.StatusMethodNotAllowed {
		t.Fatalf("the revision endpoint accepted a write: %d", r.Code)
	}
	if r := requestApp(mainApp, "GET", "/api/shared-data", "", false); r.Code != http.StatusUnauthorized {
		t.Fatalf("the revision endpoint answered without the session token: %d", r.Code)
	}
	if r := requestApp(mainApp, "GET", "/api/shared-data?revision="+strings.Repeat("a", 65), "", true); r.Code != http.StatusBadRequest {
		t.Fatalf("an oversized revision was accepted: %d", r.Code)
	}
	// A window preference is not shared data: no signal.
	terminal.state["terminalSidebarWidth"] = float64(400)
	terminal.save()
	if main.poll() || main.revision != first {
		t.Fatal("a window preference changed the shared revision")
	}
	terminal.add("terminalProfiles", sharedProfile("a", "A", "192.0.2.1"))
	terminal.save()

	// The main page is busy (a dialog is open): it asks, is told about the
	// change, but does not apply it and keeps naming its old revision.
	r := requestApp(mainApp, "GET", "/api/shared-data?revision="+first, "", true)
	var told struct {
		Revision string         `json:"revision"`
		Values   map[string]any `json:"values"`
	}
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &told) != nil || told.Revision == first || told.Values == nil {
		t.Fatalf("the change was not signalled: %s", r.Body.String())
	}
	if _, deferredKey := told.Values["editorPreferences"]; deferredKey {
		t.Fatal("editor preferences are read when an editor starts, not pushed into a running page")
	}
	if _, counter := told.Values["terminalNameRevision"]; counter {
		t.Fatal("the name revision counter belongs to the engine")
	}
	// Asked again with the same old revision, the values are offered again.
	if r := requestApp(mainApp, "GET", "/api/shared-data?revision="+first, "", true); !strings.Contains(r.Body.String(), `"values"`) {
		t.Fatalf("a page that could not apply a change must be offered it again: %s", r.Body.String())
	}
	// It then saves its own new profile, still from the old revision. The
	// Terminal's profile must survive although the engine has since shown the
	// page a newer list.
	main.add("terminalProfiles", sharedProfile("b", "B", "192.0.2.2"))
	main.save()
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "a", "b") {
		t.Fatalf("saving from an older revision lost a profile: %v", got)
	}
	// A revision the engine no longer knows removes nothing either.
	main.remove("terminalProfiles", "a")
	main.revision = "0000000000000000000000000000dead"
	main.save()
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "a", "b") {
		t.Fatalf("a save from an unknown revision removed a profile: %v", got)
	}
}

func TestSharedStateEditsRenamesAndDeferredPreferences(t *testing.T) {
	root := t.TempDir()
	mainApp := sharedTestApp(t, root, "", "main")
	main := openSharedPage(t, mainApp)
	terminalApp := sharedTestApp(t, root, terminalWorkspaceName, "terminal")
	terminal := openSharedPage(t, terminalApp)
	main.add("terminalProfiles", sharedProfile("p", "Core", "192.0.2.1"))
	main.state["editorPreferences"] = map[string]any{"wordWrap": false, "fontSize": float64(13)}
	main.save()
	terminal.poll()
	if terminal.row("terminalProfiles", "p") == nil {
		t.Fatal("the Terminal did not receive the profile")
	}

	// The main window renames the profile; the Terminal (not refreshed yet)
	// moves it to a folder. Both changes are kept: the folder from the page's
	// edit, the newer name from the display-name revisions.
	main.row("terminalProfiles", "p")["name"] = "Core switch"
	main.save()
	terminal.row("terminalProfiles", "p")["folder"] = "MDF"
	terminal.save()
	fresh := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	row := fresh.row("terminalProfiles", "p")
	if row == nil || row["name"] != "Core switch" || row["folder"] != "MDF" {
		t.Fatalf("a rename and a folder change from two apps did not both survive: %v", row)
	}

	// A name the engine itself saves (renaming a live tab) reaches the list on
	// disk from a standalone process too.
	if _, err := terminalApp.persistTerminalDisplayName(terminalSessionFixture("p", "Renamed in a tab")); err != nil {
		t.Fatal(err)
	}
	main.poll()
	if row := main.row("terminalProfiles", "p"); row == nil || row["name"] != "Renamed in a tab" || row["folder"] != "MDF" {
		t.Fatalf("the engine's rename did not reach the other app: %v", row)
	}

	// Editor preferences merge one by one, and each editor writes back only
	// what its user changed.
	editor := openSharedPage(t, sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode))
	if prefs, _ := editor.state["editorPreferences"].(map[string]any); prefs["fontSize"] != float64(13) {
		t.Fatalf("the standalone Text Editor does not start with the shared preferences: %v", editor.state["editorPreferences"])
	}
	editor.state["editorPreferences"] = map[string]any{"wordWrap": true, "fontSize": float64(13)}
	editor.save()
	// The main window's editor still holds its old values and saves all of
	// them, having changed only the font size.
	main.state["editorPreferences"] = map[string]any{"wordWrap": false, "fontSize": float64(16)}
	main.save()
	settings, err := readRootSettings(filepath.Join(root, appSettingsFilename))
	if err != nil {
		t.Fatal(err)
	}
	prefs, _ := settings.shared()["editorPreferences"].(map[string]any)
	if prefs["wordWrap"] != true || prefs["fontSize"] != float64(16) {
		t.Fatalf("editor preferences from two editors: %v", prefs)
	}
	if prefs, _ := main.state["editorPreferences"].(map[string]any); prefs["wordWrap"] != false {
		t.Fatal("a running editor's preferences were replaced under it")
	}
}

func TestSharedStateRejectsInvalidAndOversizedSharedData(t *testing.T) {
	root := t.TempDir()
	main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	main.add("terminalProfiles", sharedProfile("p", "Core", "192.0.2.1"))
	main.state["reportNotes"] = strings.Repeat("n", 1400<<10) // the main window's own data
	main.save()
	terminalApp := sharedTestApp(t, root, terminalWorkspaceName, "terminal")
	before, err := os.ReadFile(filepath.Join(root, appSettingsFilename))
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{`{"terminalProfiles":"junk"}`, `{"terminalProfiles":{"id":"p"}}`, `{"terminalRecent":7}`, `{"ftpProfiles":"x"}`, `{"ftpRecent":{}}`, `{"sftpRecent":true}`, `{"sftpVerifiedHostKeys":[]}`, `{"editorPreferences":"x"}`, `{"terminalPromptNames":{}}`} {
		if r := requestApp(terminalApp, "PUT", "/api/state", body, true); r.Code != http.StatusBadRequest {
			t.Fatalf("invalid shared data accepted (%d): %s", r.Code, body)
		}
	}
	// A list that would push the main settings file over its 2 MiB limit is
	// refused as a whole; nothing is written.
	terminal := openSharedPage(t, terminalApp)
	for n := 0; n < 100; n++ {
		row := sharedProfile(fmt.Sprintf("big%d", n), "big", strings.Repeat("h", 1000))
		for _, field := range []string{"username", "serialPort", "credentialId", "stopBits", "parity", "flowControl", "lineEnding"} {
			row[field] = strings.Repeat("x", 1000)
		}
		terminal.add("terminalProfiles", row)
	}
	if code, body := terminal.trySave(); code != http.StatusBadRequest || !strings.Contains(body, "2 MiB") {
		t.Fatalf("an oversized shared list was not refused: %d %s", code, body)
	}
	after, err := os.ReadFile(filepath.Join(root, appSettingsFilename))
	if err != nil || string(after) != string(before) {
		t.Fatal("a refused save changed the main settings file")
	}
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "p") {
		t.Fatalf("profiles after refused saves: %v", got)
	}
	// A broken main settings file is never replaced by a standalone app.
	terminal = openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	if err := os.WriteFile(filepath.Join(root, appSettingsFilename), []byte(`{"terminalProfiles":[`), 0600); err != nil {
		t.Fatal(err)
	}
	terminal.add("terminalProfiles", sharedProfile("q", "Edge", "192.0.2.2"))
	if code, _ := terminal.trySave(); code != http.StatusConflict {
		t.Fatalf("a standalone app saved over an unreadable main settings file: %d", code)
	}
	if raw, _ := os.ReadFile(filepath.Join(root, appSettingsFilename)); string(raw) != `{"terminalProfiles":[` {
		t.Fatal("the unreadable main settings file was changed")
	}
}
