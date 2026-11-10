package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A portable copy and an installed copy are updated separately, so a version
// from before shared app data (v1.21.38 and older) can run on a data folder
// that this version uses. What that version does, taken from its code:
//
//   - its engine reads the settings file once, at start;
//   - its page keeps the keys it knows (unknown keys are dropped) and at most
//     the first 100 saved sessions and 100 saved FTP connections;
//   - every save writes all of that back, without reading the file again and
//     without merging.
//
// olderMainWindow stands in for it.
type olderMainWindow struct {
	t     *testing.T
	path  string
	state map[string]any
}

func startOlderMainWindow(t *testing.T, root string) *olderMainWindow {
	t.Helper()
	old := &olderMainWindow{t: t, path: filepath.Join(root, appSettingsFilename), state: map[string]any{}}
	raw, err := os.ReadFile(old.path)
	if err != nil || json.Unmarshal(raw, &old.state) != nil {
		t.Fatalf("the older version could not read the settings: %v", err)
	}
	// Its page does not know keys that were added later.
	delete(old.state, sharedWriterField)
	for _, name := range []string{"terminalProfiles", "ftpProfiles"} {
		if list, _ := old.state[name].([]any); len(list) > 100 {
			old.state[name] = list[:100]
		}
	}
	return old
}

func (old *olderMainWindow) ids(name string) []string {
	out := []string{}
	list, _ := old.state[name].([]any)
	for _, entry := range list {
		row, _ := entry.(map[string]any)
		id, _ := row["id"].(string)
		out = append(out, id)
	}
	return out
}

func (old *olderMainWindow) remove(name, id string) {
	kept := []any{}
	list, _ := old.state[name].([]any)
	for _, entry := range list {
		if row, _ := entry.(map[string]any); row["id"] != id {
			kept = append(kept, entry)
		}
	}
	old.state[name] = kept
}

// save writes everything the older version holds, as its PUT /api/state did.
func (old *olderMainWindow) save() {
	old.t.Helper()
	raw, err := json.MarshalIndent(old.state, "", "  ")
	if err != nil {
		old.t.Fatal(err)
	}
	if err := writeAtomic(old.path, raw); err != nil {
		old.t.Fatal(err)
	}
}

func hasID(ids []string, id string) bool {
	for _, have := range ids {
		if have == id {
			return true
		}
	}
	return false
}

// newVersionStart is what every app of this version does first.
func newVersionStart(t *testing.T, root string) {
	t.Helper()
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
}

func olderVersionPoll(t *testing.T, a *application) bool {
	t.Helper()
	var polled struct {
		Older bool `json:"olderVersion"`
	}
	r := requestApp(a, "GET", "/api/shared-data", "", true)
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &polled) != nil {
		t.Fatalf("poll: %d %s", r.Code, r.Body.String())
	}
	return polled.Older
}

func TestSharedOlderMainWindowCannotDropSavedConnections(t *testing.T) {
	root := t.TempDir()
	// This version: 130 saved sessions and 120 saved FTP connections.
	main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	main.state["terminalProfiles"], main.state["ftpProfiles"] = sharedManyProfiles("ssh", 130), sharedManyFTP("ftp", 120)
	main.state["terminalSavedManualOrder"] = true
	main.save()
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	if olderVersionPoll(t, terminal.app) {
		t.Fatal("an older version was reported although only this version has saved")
	}

	// The older main window starts: it can show the first 100 of each.
	old := startOlderMainWindow(t, root)
	if len(old.ids("terminalProfiles")) != 100 || len(old.ids("ftpProfiles")) != 100 {
		t.Fatalf("fixture: the older version holds %d and %d", len(old.ids("terminalProfiles")), len(old.ids("ftpProfiles")))
	}

	// Meanwhile, in this version: one of the first 100 is removed (so the 101st
	// moves into the first 100), one is added, one far down the list is edited.
	terminal.remove("terminalProfiles", "ssh005")
	terminal.add("terminalProfiles", sharedProfile("added-here", "Added here", "192.0.2.77"))
	terminal.row("terminalProfiles", "ssh120")["username"] = "admin"
	terminal.save()

	// The older main window saves something unrelated: all it holds goes to
	// disk, without the 30 + 20 it never had and without what changed since.
	old.state["theme"] = "light"
	old.remove("ftpProfiles", "ftp007") // and its user removes one on purpose
	old.state["terminalProfiles"] = append(old.state["terminalProfiles"].([]any), sharedProfile("added-in-older", "Added in older", "192.0.2.88"))
	old.save()
	raw, _ := os.ReadFile(filepath.Join(root, appSettingsFilename))
	var written map[string]any
	if json.Unmarshal(raw, &written) != nil || len(written["terminalProfiles"].([]any)) != 101 || len(written["ftpProfiles"].([]any)) != 99 {
		t.Fatalf("fixture: the older version wrote %d and %d", len(written["terminalProfiles"].([]any)), len(written["ftpProfiles"].([]any)))
	}

	// A running app of this version: nothing it shows goes missing, and it is
	// told that an older version is saving to this data folder.
	terminal.poll()
	ids := terminal.ids("terminalProfiles")
	for n := 0; n < 130; n++ {
		id := fmt.Sprintf("ssh%03d", n)
		if n != 5 && !hasID(ids, id) {
			t.Fatalf("%s is gone after the older version saved (%d left)", id, len(ids))
		}
	}
	if !hasID(ids, "ssh100") {
		t.Fatal("the session that had moved into the first 100 after the older version started was dropped")
	}
	if !hasID(ids, "added-here") || !hasID(ids, "added-in-older") {
		t.Fatalf("a session added on either side is missing: here=%v older=%v", hasID(ids, "added-here"), hasID(ids, "added-in-older"))
	}
	if got := len(terminal.ids("ftpProfiles")); got != 120 {
		t.Fatalf("saved FTP connections after the older version saved: %d of 120", got)
	}
	if !olderVersionPoll(t, terminal.app) {
		t.Fatal("the running app was not told that an older version saved to this data folder")
	}
	if terminal.state["theme"] != "light" {
		t.Fatalf("a setting the older version saved is not shown: %v", terminal.state["theme"])
	}

	// Its next save puts everything back on disk, for every version.
	terminal.state["terminalSidebarWidth"] = float64(333)
	terminal.set("terminalAppearance", "cursor", "bar")
	terminal.save()
	// 130 as before (the one this version removed is back: the older window
	// still held it and wrote it again) and the two that were added.
	disk := sharedDiskIDs(t, root, "terminalProfiles")
	if len(disk) != 132 || !hasID(disk, "ssh100") || !hasID(disk, "ssh129") || !hasID(disk, "added-here") || !hasID(disk, "added-in-older") {
		t.Fatalf("data folder after this version saved again: %d saved sessions", len(disk))
	}
	if got := sharedDiskIDs(t, root, "ftpProfiles"); len(got) != 120 {
		t.Fatalf("data folder after this version saved again: %d of 120 FTP connections", len(got))
	}
	if olderVersionPoll(t, terminal.app) {
		t.Fatal("an older version is still reported after this version saved")
	}
	// The user is told what was put back (the one removed in the older
	// version is among them, so it can be removed again here).
	var named []string
	for _, notice := range sharedNotices(root) {
		if notice.Kind == "older-version" {
			named = append(named, notice.Items...)
		}
	}
	if joined := strings.Join(named, "\n"); !strings.Contains(joined, "ftp 7") || !strings.Contains(joined, "ssh 129") {
		t.Fatalf("the user is not told which entries were put back: %v", named)
	}

	// The older window is still open and saves again from what it holds.
	old.state["theme"] = "dark"
	old.save()
	// This version is closed meanwhile and started later: the start repairs
	// the file before any page is served.
	newVersionStart(t, root)
	if got := sharedDiskIDs(t, root, "terminalProfiles"); len(got) != 132 {
		t.Fatalf("data folder after the next start of this version: %d saved sessions", len(got))
	}
	late := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	if len(late.ids("terminalProfiles")) != 132 || len(late.ids("ftpProfiles")) != 120 || late.state["theme"] != "dark" {
		t.Fatalf("an app started later: %d sessions, %d FTP connections, theme %v", len(late.ids("terminalProfiles")), len(late.ids("ftpProfiles")), late.state["theme"])
	}
	// The order the older window wrote is kept; what was put back follows.
	order := late.ids("terminalProfiles")
	if late.state["terminalSavedManualOrder"] != true || order[0] != "ssh000" || order[99] != "ssh099" || order[len(order)-1] != "added-here" {
		t.Fatalf("the user's order was not kept: %s … %s … %s", order[0], order[99], order[len(order)-1])
	}
	// A removal made in this version stays a removal.
	late.remove("terminalProfiles", "ssh129")
	late.save()
	newVersionStart(t, root)
	if got := sharedDiskIDs(t, root, "terminalProfiles"); len(got) != 131 || hasID(got, "ssh129") {
		t.Fatalf("a session removed in this version came back: %d", len(got))
	}
}

// A settings file that was replaced by hand, or a crash between the two
// files, never loses an entry either, and a file written by this version is
// taken as it is.
func TestSharedSavedListsCopyFollowsEverySaveOfThisVersion(t *testing.T) {
	root := t.TempDir()
	main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	main.state["terminalProfiles"] = sharedManyProfiles("ssh", 12)
	main.state["profiles"] = []any{map[string]any{"id": "t1", "name": "Gateway", "target": "192.0.2.1"}, map[string]any{"id": "t2", "name": "Core", "target": "192.0.2.2"}}
	main.save()
	copyPath := filepath.Join(root, savedListsFilename)
	read := func() map[string][]string {
		raw, err := os.ReadFile(copyPath)
		var stored struct {
			Lists map[string][]map[string]any `json:"lists"`
		}
		if err != nil || json.Unmarshal(raw, &stored) != nil {
			t.Fatalf("the copy of the saved lists: %v %s", err, raw)
		}
		out := map[string][]string{}
		for name, list := range stored.Lists {
			for _, row := range list {
				id, _ := row["id"].(string)
				out[name] = append(out[name], id)
			}
		}
		return out
	}
	if got := read(); len(got["terminalProfiles"]) != 12 || len(got["profiles"]) != 2 {
		t.Fatalf("the copy after the first save: %v", got)
	}
	// It holds nothing but the saved lists.
	raw, _ := os.ReadFile(copyPath)
	if strings.Contains(string(raw), "theme") || strings.Contains(string(raw), credentialConfigIdentityField) || strings.Contains(string(raw), "terminalAppearance") {
		t.Fatalf("the copy holds more than the saved lists: %s", raw)
	}
	// A removal in this version reaches the copy with the same save.
	main.remove("terminalProfiles", "ssh003")
	main.remove("profiles", "t2")
	main.save()
	if got := read(); len(got["terminalProfiles"]) != 11 || len(got["profiles"]) != 1 {
		t.Fatalf("the copy after a removal: %v", got)
	}
	// The standalone Terminal changes a saved session: main-window lists it
	// does not know stay in the copy.
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	terminal.remove("terminalProfiles", "ssh004")
	terminal.save()
	if got := read(); len(got["terminalProfiles"]) != 10 || len(got["profiles"]) != 1 {
		t.Fatalf("the copy after a save of the Terminal: %v", got)
	}
	// A file this version wrote is taken as it is, whatever the copy says: an
	// interrupted save (copy written, settings not) changes nothing.
	settings, _ := os.ReadFile(filepath.Join(root, appSettingsFilename))
	if err := os.WriteFile(copyPath, []byte(`{"version":1,"lists":{"terminalProfiles":[{"id":"never-saved","name":"Never saved","type":"ssh","host":"192.0.2.9","port":22}]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got := openSharedPage(t, sharedTestApp(t, root, "", "main")).ids("terminalProfiles"); len(got) != 10 || hasID(got, "never-saved") {
		t.Fatalf("an entry of an interrupted save appeared: %v", got)
	}
	if now, _ := os.ReadFile(filepath.Join(root, appSettingsFilename)); string(now) != string(settings) {
		t.Fatal("reading changed the settings file")
	}
	// A damaged copy is no reason to refuse anything.
	if err := os.WriteFile(copyPath, []byte(`{not json`), 0600); err != nil {
		t.Fatal(err)
	}
	old := startOlderMainWindow(t, root)
	old.save()
	page := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	if got := page.ids("terminalProfiles"); len(got) != 10 {
		t.Fatalf("with a damaged copy: %v", got)
	}
	page.state["theme"] = "light"
	page.save()
	if got := read(); len(got["terminalProfiles"]) != 10 {
		t.Fatalf("the copy was not written again: %v", got)
	}
}

// What an older standalone app adds to its own workspace files after the
// one-time move is picked up at the next start of this version; what the user
// removed in this version does not come back.
func TestSharedMigrationPicksUpWhatAnOlderStandaloneAppAddsLater(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, terminalWorkspaceName)
	first, removed, later := knownHostFixtureKey(t), knownHostFixtureKey(t), knownHostFixtureKey(t)
	writeFixture(t, filepath.Join(root, appSettingsFilename), `{"theme":"dark"}`)
	rawSettings := func(ids ...string) string {
		list := []any{}
		for _, id := range ids {
			list = append(list, sharedProfile(id, "Device "+id, "192.0.2.1"))
		}
		raw, _ := json.Marshal(map[string]any{"activePage": "terminal", "terminalProfiles": list})
		return string(raw)
	}
	writeFixture(t, filepath.Join(workspace, appSettingsFilename), rawSettings("one", "two"))
	writeKnownHostFixture(t, workspace, knownHostFixtureEntry("first.test:22", first), knownHostFixtureEntry("removed.test:22", removed))
	newVersionStart(t, root)
	app := sharedTestApp(t, root, terminalWorkspaceName, "terminal")
	page := openSharedPage(t, app)
	if !sameIDs(page.ids("terminalProfiles"), "one", "two") || !sameIDs(knownHostsOf(t, app), "first.test:22", "removed.test:22") {
		t.Fatalf("after the one-time move: %v %v", page.ids("terminalProfiles"), knownHostsOf(t, app))
	}
	// In this version the user removes a saved session and a trusted host.
	page.remove("terminalProfiles", "two")
	page.save()
	if r := requestApp(app, "DELETE", "/api/terminals/known-hosts", `{"host":"removed.test:22"}`, true); r.Code != 200 {
		t.Fatalf("remove host: %d %s", r.Code, r.Body.String())
	}
	closeEditorApp(t, app)

	// The older standalone Terminal runs: it still has both, and its user adds
	// a session and trusts a new host.
	writeFixture(t, filepath.Join(workspace, appSettingsFilename), rawSettings("one", "two", "three"))
	writeKnownHostFixture(t, workspace, knownHostFixtureEntry("first.test:22", first), knownHostFixtureEntry("removed.test:22", removed), knownHostFixtureEntry("later.test:22", later))

	newVersionStart(t, root)
	again := sharedTestApp(t, root, "", "main")
	if got := openSharedPage(t, again).ids("terminalProfiles"); !sameIDs(got, "one", "three") {
		t.Fatalf("saved sessions after the older Terminal added one: %v", got)
	}
	if got := knownHostsOf(t, again); !sameIDs(got, "first.test:22", "later.test:22") {
		t.Fatalf("trusted hosts after the older Terminal trusted one: %v", got)
	}
	// And once more without any change: nothing moves.
	before := snapshotFiles(t, root)
	newVersionStart(t, root)
	if diff := sameFiles(before, snapshotFiles(t, root)); diff != "" {
		t.Fatalf("a start without changes changed: %s", diff)
	}
}

// The data folder's settings file disappears, or cannot be read, while apps
// run. That is not "another app removed everything": pages keep what they
// show, and the next save writes all of it again.
func TestSharedSettingsFileMissingOrUnreadableWhileAppsRun(t *testing.T) {
	for _, mode := range []struct{ workspace, mode string }{{"", "main"}, {terminalWorkspaceName, "terminal"}} {
		root := t.TempDir()
		path := filepath.Join(root, appSettingsFilename)
		seed := openSharedPage(t, sharedTestApp(t, root, "", "main"))
		seed.add("terminalProfiles", sharedProfile("a", "A", "192.0.2.1"))
		seed.add("terminalProfiles", sharedProfile("b", "B", "192.0.2.2"))
		seed.state["theme"] = "light"
		seed.set("terminalAppearance", "cursor", "bar")
		seed.save()
		page := openSharedPage(t, sharedTestApp(t, root, mode.workspace, mode.mode))

		// Missing.
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if page.poll() || !sameIDs(page.ids("terminalProfiles"), "a", "b") || page.state["theme"] != "light" {
			t.Fatalf("%s: a missing settings file emptied the page: %v theme=%v", mode.mode, page.ids("terminalProfiles"), page.state["theme"])
		}
		page.add("terminalProfiles", sharedProfile("c", "C", "192.0.2.3"))
		page.save()
		if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "a", "b", "c") {
			t.Fatalf("%s: the save after the file went missing wrote %v", mode.mode, got)
		}
		disk := sharedDisk(t, root)
		if appearance, _ := disk["terminalAppearance"].(map[string]any); disk["theme"] != "light" || appearance["cursor"] != "bar" {
			t.Fatalf("%s: settings the page had not changed were not written again: theme=%v appearance=%v", mode.mode, disk["theme"], appearance)
		}
		if !sameIDs(page.ids("terminalProfiles"), "a", "b", "c") {
			t.Fatalf("%s: the page after that save: %v", mode.mode, page.ids("terminalProfiles"))
		}

		// Missing, and the page saves something that is not shared.
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		page.state["terminalSidebarWidth"] = float64(410)
		page.save()
		if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "a", "b", "c") {
			t.Fatalf("%s: a save of a window preference did not write the saved sessions again: %v", mode.mode, got)
		}

		// Unreadable: nothing the page shows changes.
		good, _ := os.ReadFile(path)
		if err := os.WriteFile(path, []byte(`{"terminalProfiles":[`), 0600); err != nil {
			t.Fatal(err)
		}
		if page.poll() || !sameIDs(page.ids("terminalProfiles"), "a", "b", "c") {
			t.Fatalf("%s: an unreadable settings file emptied the page: %v", mode.mode, page.ids("terminalProfiles"))
		}
		page.add("terminalProfiles", sharedProfile("d", "D", "192.0.2.4"))
		code, body := page.trySave()
		if mode.mode == "main" {
			// The main window replaces its own unreadable file, as it always did,
			// with everything it holds.
			if code != 200 || !sameIDs(sharedDiskIDs(t, root, "terminalProfiles"), "a", "b", "c", "d") {
				t.Fatalf("main: save over an unreadable file: %d %s %v", code, body, sharedDiskIDs(t, root, "terminalProfiles"))
			}
			continue
		}
		// A standalone app does not replace a file it cannot read: it says so,
		// keeps what the page holds, and saves it once the file is back.
		if now, _ := os.ReadFile(path); code == 200 || string(now) != `{"terminalProfiles":[` {
			t.Fatalf("%s: an unreadable settings file was replaced: %d %s", mode.mode, code, now)
		}
		if err := os.WriteFile(path, good, 0600); err != nil {
			t.Fatal(err)
		}
		page.save()
		if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "a", "b", "c", "d") {
			t.Fatalf("%s: after the file was readable again: %v", mode.mode, got)
		}
	}
}

// The copy of the saved lists is only right if every write of the data
// folder's settings file goes through writeSharedSettings.
func TestSharedSettingsFileHasOneWriter(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		source := string(raw)
		if n := strings.Count(source, "writeRootSettings("); n > 0 {
			calls += n
			if name != "shared_data.go" && name != "shared_older_version.go" {
				t.Errorf("%s writes the main settings file directly; use writeSharedSettings", name)
			}
		}
		// The main window's own settings file is the data folder's.
		if strings.Contains(source, "writeAtomic(a.dataPath, full)") || strings.Contains(source, "writeAtomic(a.sharedSettingsPath()") || strings.Contains(source, "writeAtomic(settingsPath") && name == "shared_migration.go" {
			t.Errorf("%s writes the main settings file without the copy of the saved lists", name)
		}
	}
	if calls != 2 {
		t.Fatalf("writeRootSettings is defined once and called once (by writeSharedSettings); found %d uses", calls)
	}
}
