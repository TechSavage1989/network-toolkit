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
	"time"

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
		// The lock on the editor session is released a moment after the app
		// stops. On Windows the temporary folder cannot be removed while a
		// file in it is still open.
		for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
			a.editorSession.leaseMu.Lock()
			released := a.editorSession.lease == nil
			a.editorSession.leaseMu.Unlock()
			if released {
				break
			}
		}
	})
	return a
}

// standaloneTerminalState returns the settings a newly created standalone
// Terminal window loads for a data folder: its own window state plus the
// data folder's shared settings and saved connections.
func standaloneTerminalState(t *testing.T, root string) []byte {
	t.Helper()
	dest := terminalWorkspace(root)
	if err := os.MkdirAll(dest, 0700); err != nil {
		t.Fatal(err)
	}
	if err := seedTerminalWorkspace(root, dest); err != nil {
		t.Fatal(err)
	}
	r := requestApp(sharedTestApp(t, root, terminalWorkspaceName, "terminal"), "GET", "/api/state", "", true)
	if r.Code != 200 {
		t.Fatalf("standalone Terminal settings: %d %s", r.Code, r.Body.String())
	}
	return r.Body.Bytes()
}

// sharedPage behaves like the page of one window (web/assets/shared-data.js):
// it loads its settings once, keeps all of them in memory, writes all of them
// back and says, for each shared key it changed, what it started from.
type sharedPage struct {
	t        *testing.T
	app      *application
	state    map[string]any
	revision string
	// What the page and the engine last agreed on, per shared key.
	baseline map[string]string
	// An older client that does not say what it started from.
	untracked bool
	// Entries of the last save that did not fit in a full list: key to ids.
	refused map[string][]string
}

func pageJSON(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func openSharedPage(t *testing.T, app *application) *sharedPage {
	t.Helper()
	p := loadSharedPage(t, app)
	if p.poll() {
		t.Fatal("a page that has just loaded was told its shared values changed")
	}
	return p
}

// loadSharedPage loads a page without asking for changes afterwards, for a
// test in which another app may save at that very moment.
func loadSharedPage(t *testing.T, app *application) *sharedPage {
	t.Helper()
	p := &sharedPage{t: t, app: app, baseline: map[string]string{}}
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
	for _, key := range sharedDataKeys {
		p.baseline[key.Name] = pageJSON(p.state[key.Name])
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
			p.baseline[key.Name] = pageJSON(p.state[key.Name])
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
	body, sent := map[string]any{}, map[string]string{}
	for name, value := range p.state {
		body[name] = value
	}
	if !p.untracked {
		started := map[string]any{}
		for _, key := range sharedDataKeys {
			sent[key.Name] = pageJSON(p.state[key.Name])
			if sent[key.Name] != p.baseline[key.Name] {
				var before any
				_ = json.Unmarshal([]byte(p.baseline[key.Name]), &before)
				started[key.Name] = before
			}
		}
		body[sharedBaseField] = started
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
	for name, value := range sent {
		p.baseline[name] = value
	}
	p.apply(response.Shared)
	// Entries that did not fit in a full list were not saved. The page takes
	// them out of its own list, as web/assets/shared-data.js does.
	p.refused = map[string][]string{}
	if refused, ok := response.Shared["refused"].(map[string]any); ok {
		for name, entries := range refused {
			list, _ := entries.([]any)
			for _, entry := range list {
				row, _ := entry.(map[string]any)
				id, _ := row["id"].(string)
				p.refused[name] = append(p.refused[name], id)
				p.remove(name, id)
			}
			p.baseline[name] = pageJSON(p.state[name])
		}
	}
	return 200, ""
}

// saveRefused saves and returns the entries that did not fit in a full list.
func (p *sharedPage) saveRefused() map[string][]string {
	p.t.Helper()
	p.save()
	return p.refused
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

// set changes one member of a settings object the way a settings dialog does:
// the page replaces the whole object.
func (p *sharedPage) set(name, member string, value any) {
	next := map[string]any{}
	current, _ := p.state[name].(map[string]any)
	for k, v := range current {
		next[k] = v
	}
	next[member] = value
	p.state[name] = next
}

func (p *sharedPage) member(name, member string) any {
	current, _ := p.state[name].(map[string]any)
	return current[member]
}

func sharedProfile(id, name, host string) map[string]any {
	return map[string]any{"id": id, "name": name, "nameExplicit": true, "nameRevision": float64(0), "type": "ssh", "host": host, "port": float64(22), "username": "operator", "authMethod": "password", "sshCompatibility": "modern", "timeoutMs": float64(10000)}
}

func sharedDisk(t *testing.T, root string) map[string]any {
	t.Helper()
	settings, err := readRootSettings(filepath.Join(root, appSettingsFilename))
	if err != nil {
		t.Fatal(err)
	}
	return settings.shared(sharedDataKeys)
}

func sharedDiskIDs(t *testing.T, root, name string) []string {
	t.Helper()
	out := []string{}
	list, _ := sharedDisk(t, root)[name].([]any)
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
	// for example after a panel was resized.
	main.state["toolLayout"] = map[string]any{"width": float64(340), "collapsed": false}
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
	editor.state["activePage"] = "editor"
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
	main.state["activePage"] = "terminal"
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

func TestSharedSettingsCarryBetweenAppsInBothDirections(t *testing.T) {
	root := t.TempDir()
	main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	editor := openSharedPage(t, sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode))

	// Settings made in the main window: theme, accent, terminal look and
	// behaviour, confirmations, editor preferences.
	main.state["theme"], main.state["accentColor"] = "light", "#286dbb"
	main.state["terminalFontSize"], main.state["terminalHighlight"] = float64(16), false
	main.set("terminalAppearance", "cursor", "bar")
	main.set("terminalBehavior", "scrollback", float64(2000))
	main.state["terminalConfirmClose"], main.state["terminalMacroConfirmation"], main.state["terminalSidebarSide"] = false, "never", "left"
	main.set("editorPreferences", "wordWrap", true)
	main.set("editorPreferences", "fontSize", float64(15))
	main.save()
	want := sharedDisk(t, root)
	for _, page := range []*sharedPage{terminal, editor} {
		if !page.poll() {
			t.Fatalf("%s was not told about the main window's settings", page.app.windowMode)
		}
		for _, name := range []string{"theme", "accentColor", "terminalFontSize", "terminalHighlight", "terminalAppearance", "terminalBehavior", "terminalConfirmClose", "terminalMacroConfirmation", "terminalSidebarSide", "editorPreferences"} {
			if !sharedEqual(page.state[name], want[name]) || want[name] == nil {
				t.Fatalf("%s: %s is %s, the data folder has %s", page.app.windowMode, name, pageJSON(page.state[name]), pageJSON(want[name]))
			}
		}
	}
	if terminal.state["theme"] != "light" || terminal.state["terminalFontSize"] != float64(16) || terminal.state["terminalHighlight"] != false || terminal.member("editorPreferences", "wordWrap") != true {
		t.Fatalf("the Terminal did not take over the main window's settings: %v", terminal.state)
	}

	// The other direction: a change in the standalone Terminal shows in the
	// main window, and in an app started later.
	terminal.state["theme"] = "dark"
	terminal.set("terminalAppearance", "boldBright", false)
	terminal.save()
	if !main.poll() || main.state["theme"] != "dark" || main.member("terminalAppearance", "boldBright") != false {
		t.Fatalf("the main window did not take over the Terminal's settings: theme=%v appearance=%v", main.state["theme"], main.state["terminalAppearance"])
	}
	if main.member("terminalAppearance", "cursor") != "bar" {
		t.Fatal("a setting changed in one app reverted another setting of the same dialog")
	}
	late := openSharedPage(t, sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode))
	if late.state["theme"] != "dark" || late.member("editorPreferences", "fontSize") != float64(15) {
		t.Fatalf("a Text Editor started later does not use the shared settings: %v", late.state)
	}

	// A stale page never reverts a setting changed elsewhere. The editor page
	// still holds the light theme and the old cursor settings; it changes one
	// editor preference and saves its whole state.
	if editor.state["theme"] != "light" {
		t.Fatal("fixture: the editor page should still be stale")
	}
	editor.set("editorPreferences", "syntax", false)
	editor.save()
	disk := sharedDisk(t, root)
	if disk["theme"] != "dark" {
		t.Fatalf("a stale page reverted the theme to %v", disk["theme"])
	}
	appearance, _ := disk["terminalAppearance"].(map[string]any)
	prefs, _ := disk["editorPreferences"].(map[string]any)
	if appearance["boldBright"] != false || appearance["cursor"] != "bar" || prefs["syntax"] != false || prefs["wordWrap"] != true || prefs["fontSize"] != float64(15) {
		t.Fatalf("settings after a stale save: appearance=%v editor=%v", appearance, prefs)
	}
	if editor.state["theme"] != "dark" {
		t.Fatal("the stale page was not given the current theme by its save")
	}

	// Two apps change different members of one settings object at the same
	// time: both changes are kept.
	main.poll()
	terminal.poll()
	main.set("terminalBehavior", "bell", "visual")
	terminal.set("terminalBehavior", "scrollback", float64(5000))
	main.save()
	terminal.save()
	behavior, _ := sharedDisk(t, root)["terminalBehavior"].(map[string]any)
	if behavior["bell"] != "visual" || behavior["scrollback"] != float64(5000) {
		t.Fatalf("two apps changed one settings object: %v", behavior)
	}
	// The same setting changed in both: the later save (that page's change)
	// is kept.
	main.poll()
	terminal.poll()
	main.state["terminalFontSize"] = float64(18)
	terminal.state["terminalFontSize"] = float64(11)
	main.save()
	terminal.save()
	if got := sharedDisk(t, root)["terminalFontSize"]; got != float64(11) {
		t.Fatalf("font size after two apps changed it: %v", got)
	}
}

func TestSharedStateKeepsWindowStatePerWorkspace(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, appSettingsFilename)
	// Main settings with members only the main window uses; one has a number
	// format a careless rewrite would change.
	original := `{"theme":"dark","activePage":"overview","forms":{"ping":{"count":4}},"bandwidth":{"size":1e3},"profiles":[{"id":"t","name":"Gateway","target":"192.0.2.1"}],"_credentialVaultConfigID":"0123456789abcdef0123456789abcdef","terminalProfiles":[{"id":"p","name":"Core","type":"ssh","host":"192.0.2.1","port":22}],"terminalSidebarWidth":300}`
	if err := os.WriteFile(mainPath, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	terminalApp := sharedTestApp(t, root, terminalWorkspaceName, "terminal")
	terminal := openSharedPage(t, terminalApp)
	if !sameIDs(terminal.ids("terminalProfiles"), "p") || terminal.state["theme"] != "dark" {
		t.Fatalf("the Terminal does not start with the data folder's connections and settings: %v", terminal.state)
	}
	for _, name := range []string{"forms", "bandwidth", "activePage", "terminalSidebarWidth", "profiles", credentialConfigIdentityField} {
		if _, leaked := terminal.state[name]; leaked {
			t.Fatalf("%s of the main window reached the Terminal: %v", name, terminal.state)
		}
	}
	// What is about this window stays in its own file.
	terminal.state["activePage"], terminal.state["terminalSidebarWidth"], terminal.state["terminalSidebarCollapsed"] = "terminal", float64(420), true
	terminal.state["terminalConnection"] = map[string]any{"type": "ssh", "host": "192.0.2.77"}
	// A page holds a default for every key; one for a main-only key must not
	// reach the main window's data.
	terminal.state["profiles"] = []any{}
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
	if compact("activePage") != `"overview"` || compact("terminalSidebarWidth") != `300` || compact("bandwidth") != `{"size":1e3}` || compact("forms") != `{"ping":{"count":4}}` || compact(credentialConfigIdentityField) != `"0123456789abcdef0123456789abcdef"` || compact("profiles") != `[{"id":"t","name":"Gateway","target":"192.0.2.1"}]` {
		t.Fatalf("a standalone save changed the main window's own settings: %s", raw)
	}
	if _, added := disk["firstLaunchWelcomeSeen"]; added {
		t.Fatal("an existing main settings file must not gain a first-launch marker")
	}
	if _, added := disk["terminalConnection"]; added {
		t.Fatal("the Terminal's connection form draft was written to the main settings")
	}
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "p", "q") {
		t.Fatalf("shared profiles on disk: %v", got)
	}
	own, err := os.ReadFile(terminalApp.dataPath)
	if err != nil {
		t.Fatal(err)
	}
	var workspace map[string]any
	if json.Unmarshal(own, &workspace) != nil || workspace["activePage"] != "terminal" || workspace["terminalSidebarWidth"] != float64(420) || workspace["terminalSidebarCollapsed"] != true || workspace["terminalConnection"] == nil {
		t.Fatalf("the Terminal's window state was not saved in its own file: %s", own)
	}
	for _, key := range sharedDataKeys {
		if _, present := workspace[key.Name]; present {
			t.Fatalf("the standalone workspace file holds shared data (%s): %s", key.Name, own)
		}
	}
	// The main window keeps its own window state and sees the new profile.
	main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	if main.state["activePage"] != "overview" || main.state["terminalSidebarWidth"] != float64(300) || !sameIDs(main.ids("terminalProfiles"), "p", "q") || !sameIDs(main.ids("profiles"), "t") {
		t.Fatalf("main window state: %v", main.state)
	}
	// A restarted Terminal has its own window state back.
	again := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	if again.state["terminalSidebarWidth"] != float64(420) || again.state["activePage"] != "terminal" {
		t.Fatalf("the Terminal's window state after a restart: %v", again.state)
	}
}

func TestSharedKeyTableClassifiesEveryPageKey(t *testing.T) {
	shared, perWindow := map[string]bool{}, map[string]bool{}
	for _, key := range sharedDataKeys {
		if key.Name == "" || shared[key.Name] {
			t.Fatalf("shared key %q is empty or repeated", key.Name)
		}
		shared[key.Name] = true
		if key.Kind == sharedCounter && key.Live {
			t.Fatalf("%s: a counter is kept by the engine, not refreshed in pages", key.Name)
		}
	}
	for _, name := range perWindowStateKeys {
		if name == "" || perWindow[name] || shared[name] {
			t.Fatalf("per-window key %q is empty, repeated or also shared", name)
		}
		perWindow[name] = true
	}
	// Every key the page declares in its initial settings object, and every
	// key a module adds later, must be classified.
	html, err := os.ReadFile(filepath.Join("..", "..", "web", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	var declared string
	for _, line := range strings.Split(string(html), "\n") {
		if strings.HasPrefix(line, "let state={") {
			declared = line
		}
	}
	if declared == "" {
		t.Fatal("the page's settings object was not found")
	}
	depth, start := 0, len("let state={")
	names := []string{}
	for i := start; i < len(declared) && depth >= 0; i++ {
		switch declared[i] {
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		case ':':
			if depth == 0 {
				j := i
				for j > start && declared[j-1] != ',' && declared[j-1] != '{' {
					j--
				}
				names = append(names, strings.Trim(declared[j:i], `'" `))
			}
		}
	}
	if len(names) < 60 {
		t.Fatalf("only %d settings keys were read from the page: %v", len(names), names)
	}
	names = append(names, "terminalSyntax", "terminalBehavior", "terminalInteraction", "terminalPromptNames", "terminalSavedManualOrder", "terminalNameRevision", "editorFilesPreferences", "transferServerSettings", "transferServerSidebar", "transferClientLayout", "transferModes", "testTabOrder", "overviewLayout", "remoteBrowserSettings", "resultFilters", "graphPlotPreferences", "graphTimeRange", "graphWorkspacePreferences", "packetGraphPreferences", "packetMapSettings")
	for _, name := range names {
		if !shared[name] && !perWindow[name] {
			t.Errorf("%s is not classified: add it to sharedDataKeys (a preference or saved data) or perWindowStateKeys (about one window) in shared_data.go", name)
		}
	}
	// What the user sets on purpose is shared; what is about one window is not.
	for _, name := range []string{"terminalProfiles", "terminalRecent", "ftpProfiles", "sftpRecent", "sftpVerifiedHostKeys", "theme", "accentColor", "colorPickerPreferences", "terminalAppearance", "terminalSyntax", "terminalHighlight", "terminalBehavior", "terminalInteraction", "terminalFontSize", "terminalMacroConfirmation", "terminalSidebarSide", "terminalConfirmClose", "terminalSessionConfirmDelete", "editorPreferences", "editorFilesPreferences"} {
		if key, ok := sharedKeyNamed(name); !ok || key.MainOnly {
			t.Errorf("%s must be shared by every app", name)
		}
	}
	for _, name := range []string{"activePage", "terminalSidebarWidth", "terminalSidebarCollapsed", "terminalSidebarTab", "terminalLayout", "terminalConnection", "toolLayout", "firstLaunchWelcomeSeen", "appUpdatePreferences", credentialConfigIdentityField} {
		if !perWindow[name] {
			t.Errorf("%s is about one window or one run and must stay per window", name)
		}
	}
}

func TestSharedStateStandaloneCreatesMainSettingsWithoutHidingFirstLaunch(t *testing.T) {
	root := t.TempDir()
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	terminal.state["terminalSidebarWidth"] = float64(400)
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
	if strings.Contains(string(raw), "terminalSidebarWidth") {
		t.Fatalf("window state of the Terminal was written to the main settings: %s", raw)
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
				// Each app also keeps changing its own member of one settings object.
				page.set("editorPreferences", fmt.Sprintf("app%d", index), float64(n))
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
	prefs, _ := sharedDisk(t, root)["editorPreferences"].(map[string]any)
	for index := range apps {
		if prefs[fmt.Sprintf("app%d", index)] != float64(perApp-1) {
			t.Fatalf("interleaved settings saves lost a change: %v", prefs)
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

func TestSharedStateRevisionSignalsAndBusyPages(t *testing.T) {
	root := t.TempDir()
	mainApp := sharedTestApp(t, root, "", "main")
	main := openSharedPage(t, mainApp)
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	first := main.revision
	if first == "" || terminal.revision == "" {
		t.Fatalf("a loaded page has no revision: %q %q", first, terminal.revision)
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
	// Window state is not shared data: no signal.
	terminal.state["terminalSidebarWidth"] = float64(400)
	terminal.save()
	if main.poll() || main.revision != first {
		t.Fatal("window state changed the shared revision")
	}
	// Data only the main window uses does not make a standalone app refresh.
	before := terminal.revision
	main.add("profiles", map[string]any{"id": "t", "name": "Gateway", "target": "192.0.2.1"})
	main.save()
	if terminal.poll() || terminal.revision != before {
		t.Fatal("main-only data made the Terminal refresh")
	}
	first = main.revision
	terminal.add("terminalProfiles", sharedProfile("a", "A", "192.0.2.1"))
	terminal.save()

	// The main page is busy (a dialog is open): it asks, is told about the
	// change, but does not apply it.
	r := requestApp(mainApp, "GET", "/api/shared-data?revision="+first, "", true)
	var told struct {
		Revision string         `json:"revision"`
		Values   map[string]any `json:"values"`
	}
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &told) != nil || told.Revision == first || told.Values == nil {
		t.Fatalf("the change was not signalled: %s", r.Body.String())
	}
	for _, key := range sharedDataKeys {
		if _, sent := told.Values[key.Name]; sent && !key.Live {
			t.Fatalf("%s is read when a page starts, not pushed into a running page", key.Name)
		}
	}
	// Asked again with the same old revision, the values are offered again.
	if r := requestApp(mainApp, "GET", "/api/shared-data?revision="+first, "", true); !strings.Contains(r.Body.String(), `"values"`) {
		t.Fatalf("a page that could not apply a change must be offered it again: %s", r.Body.String())
	}
	// It then saves its own new profile, still without the Terminal's. The
	// Terminal's profile must survive although the engine has since shown the
	// page a newer list.
	main.add("terminalProfiles", sharedProfile("b", "B", "192.0.2.2"))
	main.save()
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "a", "b") {
		t.Fatalf("a save from a page that had not applied a change lost a profile: %v", got)
	}
	// An older client that does not say what it started from is taken to
	// hold what the engine last gave it or accepted from it.
	old := openSharedPage(t, sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode))
	old.untracked = true
	terminal.add("terminalProfiles", sharedProfile("c", "C", "192.0.2.3"))
	terminal.save()
	old.add("terminalProfiles", sharedProfile("d", "D", "192.0.2.4"))
	old.save()
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "a", "b", "c", "d") {
		t.Fatalf("a save from an older client lost a profile: %v", got)
	}
	old.remove("terminalProfiles", "a")
	old.save()
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "b", "c", "d") {
		t.Fatalf("an older client could not remove a profile it was given: %v", got)
	}
}

func TestSharedStateEditsRenamesAndEditorPreferences(t *testing.T) {
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

	// Editor preferences merge one by one: each editor writes back all of its
	// preferences, and only what its user changed counts as a change.
	editor := openSharedPage(t, sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode))
	if editor.member("editorPreferences", "fontSize") != float64(13) {
		t.Fatalf("the standalone Text Editor does not start with the shared preferences: %v", editor.state["editorPreferences"])
	}
	editor.state["editorPreferences"] = map[string]any{"wordWrap": true, "fontSize": float64(13)}
	editor.save()
	// The main window's editor still holds its old values and saves all of
	// them, having changed only the font size.
	main.state["editorPreferences"] = map[string]any{"wordWrap": false, "fontSize": float64(16)}
	main.save()
	prefs, _ := sharedDisk(t, root)["editorPreferences"].(map[string]any)
	if prefs["wordWrap"] != true || prefs["fontSize"] != float64(16) {
		t.Fatalf("editor preferences from two editors: %v", prefs)
	}
	// The save's answer brings the main window's editor up to date.
	if main.member("editorPreferences", "wordWrap") != true {
		t.Fatalf("the main window's editor was not given the other editor's change: %v", main.state["editorPreferences"])
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
	for _, body := range []string{`{"terminalProfiles":"junk"}`, `{"terminalProfiles":{"id":"p"}}`, `{"terminalRecent":7}`, `{"ftpProfiles":"x"}`, `{"ftpRecent":{}}`, `{"sftpRecent":true}`, `{"sftpVerifiedHostKeys":[]}`, `{"editorPreferences":"x"}`, `{"terminalPromptNames":{}}`, `{"terminalAppearance":[]}`, `{"terminalSyntax":"x"}`,
		`{"_sharedBase":"token"}`, `{"_sharedBase":[]}`, `{"terminalProfiles":[],"_sharedBase":{"terminalProfiles":"junk"}}`, `{"_sharedBase":{"editorPreferences":7}}`} {
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
