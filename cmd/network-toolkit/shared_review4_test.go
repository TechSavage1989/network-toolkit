package main

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// olderDeleteAppData removes what "Delete app data" of a version from before
// shared app data removes from a data folder (v1.21.39,
// storage_location_files.go): the files it knows, and the window state of
// every detached view. It does not know the copy of the saved lists.
func olderDeleteAppData(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{
		appSettingsFilename, terminalMacroLibraryFilename, "Network_Toolkit.ssh_known_hosts.json",
		"Network_Toolkit.vendors.json.gz", "Network_Toolkit.window.json", "Network_Toolkit.instance.json", credentialRememberFilename,
	} {
		if err := os.Remove(filepath.Join(root, name)); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	views := filepath.Join(root, storageViewsDirectory)
	entries, _ := os.ReadDir(views)
	for _, entry := range entries {
		name := entry.Name()
		if _, err := hex.DecodeString(name); err != nil || len(name) != 32 || name != strings.ToLower(name) {
			continue
		}
		_ = os.Remove(filepath.Join(views, name, "Network_Toolkit.window.json"))
		_ = os.Remove(filepath.Join(views, name))
	}
	_ = os.Remove(views)
}

func noticesOfKind(root, kind string) [][]string {
	var out [][]string
	for _, notice := range sharedNotices(root) {
		if notice.Kind == kind {
			out = append(out, notice.Items)
		}
	}
	return out
}

func hasName(items []string, name string) bool {
	for _, item := range items {
		if item == name {
			return true
		}
	}
	return false
}

// An older version does not know the copy of the saved lists, so its "Delete
// app data" leaves the copy behind. This version must not bring the deleted
// entries back from it, at the next start or later.
func TestSharedOlderDeleteAppDataIsNotUndone(t *testing.T) {
	for _, start := range []string{"start step", "app only", "older version saved again"} {
		root := t.TempDir()
		seed := r3Seed(t, root)
		closeEditorApp(t, seed.app)
		olderDeleteAppData(t, root)
		if _, err := os.Stat(filepath.Join(root, savedListsFilename)); err != nil {
			t.Fatalf("%s: the older delete is expected to leave the copy: %v", start, err)
		}
		switch start {
		case "start step":
			newVersionStart(t, root)
			if _, err := os.Stat(filepath.Join(root, savedListsFilename)); !os.IsNotExist(err) {
				t.Errorf("%s: the copy of deleted data is still there after the start: %v", start, err)
			}
		case "older version saved again":
			// The older version started again with nothing and saved.
			if err := os.WriteFile(filepath.Join(root, appSettingsFilename), []byte(`{"theme":"dark","terminalProfiles":[]}`), 0600); err != nil {
				t.Fatal(err)
			}
			newVersionStart(t, root)
		}
		fresh := openSharedPage(t, sharedTestApp(t, root, "", "main"))
		if got := fresh.ids("terminalProfiles"); len(got) != 0 {
			t.Errorf("%s: deleted saved sessions are shown again: %v", start, got)
		}
		if len(fresh.ids("profiles")) != 0 || len(fresh.ids("tftpProfiles")) != 0 || fresh.state["savedResultFilters"] != nil {
			t.Errorf("%s: deleted main-window data is shown again: %v %v %v", start, fresh.ids("profiles"), fresh.ids("tftpProfiles"), fresh.state["savedResultFilters"])
		}
		fresh.state["activePage"] = "overview"
		fresh.save()
		fresh.poll()
		if got := sharedDiskIDs(t, root, "terminalProfiles"); len(got) != 0 {
			t.Errorf("%s: deleted saved sessions were written again: %v", start, got)
		}
		if strings.Contains(r3Copy(t, root), "192.0.2.1") {
			t.Errorf("%s: the copy still holds deleted entries: %s", start, r3Copy(t, root))
		}
		if len(noticesOfKind(root, "recovered")) != 0 || len(noticesOfKind(root, "older-version")) != 0 {
			t.Errorf("%s: a notice says entries came back: %v", start, sharedNotices(root))
		}
	}
}

// The settings file alone is lost (nothing else that a deletion removes is
// gone): its saved lists come back from the copy, and the user is told what
// came back.
func TestSharedLostSettingsFileIsRecoveredFromTheCopyWithANotice(t *testing.T) {
	for _, mode := range []string{"main", "terminal"} {
		root := t.TempDir()
		seed := r3Seed(t, root)
		closeEditorApp(t, seed.app)
		if err := os.Remove(filepath.Join(root, appSettingsFilename)); err != nil {
			t.Fatal(err)
		}
		newVersionStart(t, root)
		workspace := ""
		if mode == "terminal" {
			workspace = terminalWorkspaceName
		}
		page := openSharedPage(t, sharedTestApp(t, root, workspace, mode))
		if !sameIDs(page.ids("terminalProfiles"), "a") {
			t.Fatalf("%s: saved sessions after the file was lost: %v", mode, page.ids("terminalProfiles"))
		}
		// Nothing was saved by the user: the first refresh writes the file
		// again and records the notice.
		page.poll()
		d := r3Disk(t, root)
		if l, _ := d["profiles"].([]any); len(l) != 1 || d["savedResultFilters"] == nil {
			t.Fatalf("%s: main-window data was not written again: %v %v", mode, d["profiles"], d["savedResultFilters"])
		}
		if !sameIDs(sharedDiskIDs(t, root, "terminalProfiles"), "a") {
			t.Fatalf("%s: saved sessions were not written again: %v", mode, sharedDiskIDs(t, root, "terminalProfiles"))
		}
		notices := noticesOfKind(root, "recovered")
		if len(notices) != 1 || !hasName(notices[0], "A") || !hasName(notices[0], "Core router") || !hasName(notices[0], "Lab TFTP") {
			t.Fatalf("%s: the notice about what was recovered: %v", mode, sharedNotices(root))
		}
		// It is told once.
		page.state["theme"] = "light"
		page.save()
		page.poll()
		if got := noticesOfKind(root, "recovered"); len(got) != 1 {
			t.Fatalf("%s: recovered notices after another save: %v", mode, got)
		}
	}
}

// A Terminal that has not refreshed holds an older list than the copy. When
// the settings file is lost, its save must not write its older view over
// what the main window saved last.
func TestSharedStaleHolderDoesNotBeatANewerCopy(t *testing.T) {
	root := t.TempDir()
	main := r3Seed(t, root)
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	main.add("terminalProfiles", sharedProfile("b", "B", "192.0.2.2"))
	main.remove("terminalProfiles", "a")
	main.state["profiles"] = []any{map[string]any{"id": "target-2", "name": "Edge", "target": "192.0.2.11", "tool": "ping"}}
	main.save()
	closeEditorApp(t, main.app)
	if err := os.Remove(filepath.Join(root, appSettingsFilename)); err != nil {
		t.Fatal(err)
	}
	terminal.state["terminalSidebarWidth"] = float64(300)
	terminal.set("terminalSettings", "fontSize", float64(15))
	if code, body := terminal.trySave(); code != 200 {
		t.Fatalf("the Terminal's save: %d %s", code, body)
	}
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "b") {
		t.Fatalf("saved sessions after the stale Terminal wrote the file: %v, want [b]", got)
	}
	if copied := r3Copy(t, root); strings.Contains(copied, "192.0.2.1\"") || !strings.Contains(copied, "192.0.2.2") {
		t.Fatalf("the copy after the stale Terminal's save: %s", copied)
	}
	d := r3Disk(t, root)
	if l, _ := d["profiles"].([]any); len(l) != 1 || !strings.Contains(pageJSON(l), "target-2") {
		t.Fatalf("the main window's targets after the Terminal's save: %v", d["profiles"])
	}
	// What the Terminal itself adds while the file is missing is kept too.
	_ = os.Remove(filepath.Join(root, appSettingsFilename))
	terminal.add("terminalProfiles", sharedProfile("c", "C", "192.0.2.3"))
	terminal.save()
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "b", "c") {
		t.Fatalf("saved sessions after the Terminal added one: %v", got)
	}
}

// The copy could not be read when the app started and can be read at its
// first save: the copy is taken in before it is written.
func TestSharedCopyUnreadableAtStartIsTakenInBeforeTheFirstWrite(t *testing.T) {
	for _, mode := range []string{"terminal", "main"} {
		root := t.TempDir()
		seed := r3Seed(t, root)
		closeEditorApp(t, seed.app)
		if err := os.Remove(filepath.Join(root, appSettingsFilename)); err != nil {
			t.Fatal(err)
		}
		good, err := os.ReadFile(filepath.Join(root, savedListsFilename))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, savedListsFilename), []byte("{not json"), 0600); err != nil {
			t.Fatal(err)
		}
		workspace := ""
		if mode == "terminal" {
			workspace = terminalWorkspaceName
		}
		page := openSharedPage(t, sharedTestApp(t, root, workspace, mode))
		if err := os.WriteFile(filepath.Join(root, savedListsFilename), good, 0600); err != nil {
			t.Fatal(err)
		}
		page.add("terminalProfiles", sharedProfile("n", "N", "192.0.2.9"))
		if code, body := page.trySave(); code != 200 {
			t.Fatalf("%s: save: %d %s", mode, code, body)
		}
		if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "a", "n") {
			t.Fatalf("%s: saved sessions on disk: %v, want [a n]", mode, got)
		}
		d := r3Disk(t, root)
		if l, _ := d["profiles"].([]any); len(l) != 1 || d["savedResultFilters"] == nil {
			t.Fatalf("%s: main-window data of the copy was overwritten: %v %v", mode, d["profiles"], d["savedResultFilters"])
		}
		if copied := r3Copy(t, root); !strings.Contains(copied, "192.0.2.1\"") || !strings.Contains(copied, "target-1") {
			t.Fatalf("%s: the copy lost what it held: %s", mode, copied)
		}
		page.poll()
		if notices := noticesOfKind(root, "recovered"); len(notices) != 1 || !hasName(notices[0], "A") || hasName(notices[0], "N") {
			t.Fatalf("%s: the notice about what was recovered: %v", mode, sharedNotices(root))
		}
	}
}

// A data folder from before shared app data (no mark, no copy), only the
// Terminal is used, and the settings file disappears while it runs. The
// Terminal is not stuck: it writes what it holds, saved credentials stay
// usable, and a main window that starts later is told that what only it
// uses is absent, not removed.
func TestSharedStandaloneAloneWritesAMissingFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, appSettingsFilename), []byte(`{"terminalProfiles":[{"id":"a","name":"A","type":"ssh","host":"192.0.2.1","port":22}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	if err := os.Remove(filepath.Join(root, appSettingsFilename)); err != nil {
		t.Fatal(err)
	}
	terminal.add("terminalProfiles", sharedProfile("b", "B", "192.0.2.2"))
	if code, body := terminal.trySave(); code != 200 {
		t.Fatalf("the Terminal's save: %d %s", code, body)
	}
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "a", "b") {
		t.Fatalf("saved sessions on disk: %v", got)
	}
	if _, marked := r3Disk(t, root)[sharedMainOnlyAbsentField]; !marked {
		t.Fatal("the file does not say that the main window's data is absent")
	}
	if id, err := terminal.app.ensureCredentialConfigIdentity(); err != nil || id == "" {
		t.Fatalf("the vault identity: %q %v", id, err)
	}
	late := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	if !sameIDs(late.ids("terminalProfiles"), "a", "b") {
		t.Fatalf("a Terminal started later: %v", late.ids("terminalProfiles"))
	}
	// The identity is created on a missing file as well.
	_ = os.Remove(filepath.Join(root, appSettingsFilename))
	_ = os.Remove(filepath.Join(root, savedListsFilename))
	if id, err := terminal.app.ensureCredentialConfigIdentity(); err != nil || id == "" {
		t.Fatalf("the vault identity on a missing file: %q %v", id, err)
	}
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "a", "b") {
		t.Fatalf("saved sessions after the identity was written: %v", got)
	}
}

// The Terminal holds [a]; the settings file and its copy are gone; the main
// window is not running. Whatever the user does next (save in the Terminal,
// or open the main window first, which starts with nothing and saves), the
// Terminal's entries are on disk in the end and shown in both.
func TestSharedStandaloneKeepsWhatItHoldsWhenTheFileIsWrittenAnew(t *testing.T) {
	for _, order := range []string{"terminal saves first", "terminal refreshed while missing", "terminal saw nothing"} {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, appSettingsFilename), []byte(`{"terminalProfiles":[{"id":"a","name":"A","type":"ssh","host":"192.0.2.1","port":22,"username":"operator"}]}`), 0600); err != nil {
			t.Fatal(err)
		}
		terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
		terminal.set("editorPreferences", "wordWrap", true)
		if order == "terminal saw nothing" {
			// This version has saved here before (the file carries its id):
			// that is what lets an app see that a file was written anew.
			terminal.save()
		}
		if err := os.Remove(filepath.Join(root, appSettingsFilename)); err != nil {
			t.Fatal(err)
		}
		_ = os.Remove(filepath.Join(root, savedListsFilename))
		want := []string{"a"}
		switch order {
		case "terminal saves first":
			terminal.add("terminalProfiles", sharedProfile("b", "B", "192.0.2.2"))
			if code, body := terminal.trySave(); code != 200 {
				t.Fatalf("%s: the Terminal's save: %d %s", order, code, body)
			}
			want = []string{"a", "b"}
		case "terminal refreshed while missing":
			terminal.poll()
		}
		main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
		main.state["activePage"] = "overview"
		main.save()
		if order != "terminal saves first" {
			if got := sharedDiskIDs(t, root, "terminalProfiles"); len(got) != 0 {
				t.Fatalf("%s: a main window started with nothing wrote %v", order, got)
			}
		}
		// The Terminal's next refresh puts back what it holds.
		terminal.poll()
		if got := terminal.ids("terminalProfiles"); !sameIDs(got, want...) {
			t.Fatalf("%s: the Terminal after the main window wrote the file: %v, want %v", order, got, want)
		}
		if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, want...) {
			t.Fatalf("%s: on disk after the Terminal's refresh: %v, want %v", order, got, want)
		}
		terminal.state["terminalSidebarWidth"] = float64(280)
		terminal.save()
		main.poll()
		if got := main.ids("terminalProfiles"); !sameIDs(got, want...) {
			t.Fatalf("%s: the main window after its refresh: %v, want %v", order, got, want)
		}
		if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, want...) {
			t.Fatalf("%s: on disk in the end: %v, want %v", order, got, want)
		}
		if order == "terminal saw nothing" && sharedDisk(t, root)["editorPreferences"] == nil {
			t.Errorf("%s: a setting the Terminal held is gone in the end", order)
		}
		// From here on a removal is a removal again.
		main.remove("terminalProfiles", "a")
		main.save()
		terminal.poll()
		terminal.state["terminalSidebarWidth"] = float64(300)
		terminal.save()
		if got := sharedDiskIDs(t, root, "terminalProfiles"); hasID(got, "a") {
			t.Fatalf("%s: an entry removed afterwards came back: %v", order, got)
		}
	}
}

// A running main window that has not noticed the loss reads the file a
// standalone app wrote anew: what only the main window uses is absent there,
// not removed, and the main window writes it again.
func TestSharedMainKeepsItsDataWhenAStandaloneAppWroteTheFileAnew(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, appSettingsFilename), []byte(`{"terminalProfiles":[{"id":"a","name":"A","type":"ssh","host":"192.0.2.1","port":22}],"profiles":[{"id":"target-1","name":"Core router","target":"192.0.2.10","tool":"ping"}],"savedResultFilters":{"ping":[{"id":"f1","name":"Slow","query":"ms>100"}]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	if err := os.Remove(filepath.Join(root, appSettingsFilename)); err != nil {
		t.Fatal(err)
	}
	terminal.add("terminalProfiles", sharedProfile("b", "B", "192.0.2.2"))
	terminal.save()
	main.poll()
	d := r3Disk(t, root)
	if l, _ := d["profiles"].([]any); len(l) != 1 || d["savedResultFilters"] == nil {
		t.Fatalf("the main window's data after its refresh: %v %v", d["profiles"], d["savedResultFilters"])
	}
	if _, marked := d[sharedMainOnlyAbsentField]; marked {
		t.Fatal("the note that main-window data is absent stayed after the main window wrote the file")
	}
	if !sameIDs(sharedDiskIDs(t, root, "terminalProfiles"), "a", "b") || !sameIDs(main.ids("terminalProfiles"), "a", "b") {
		t.Fatalf("saved sessions: disk %v main %v", sharedDiskIDs(t, root, "terminalProfiles"), main.ids("terminalProfiles"))
	}
}

// The copy of the saved lists and the settings file belong together: the
// same id is in the file's mark, in the copy and in a place that every
// version's "Delete app data" removes.
func TestSharedSavedListsCopyIsPairedWithTheSettingsFile(t *testing.T) {
	root := t.TempDir()
	seed := r3Seed(t, root)
	var copied savedListsCopy
	if err := json.Unmarshal([]byte(r3Copy(t, root)), &copied); err != nil || len(copied.Pair) != 32 {
		t.Fatalf("the copy's id: %q %v", copied.Pair, err)
	}
	if got, _ := r3Disk(t, root)[sharedWriterField].(string); got != copied.Pair {
		t.Fatalf("the settings file's mark %q is not the copy's id %q", got, copied.Pair)
	}
	if got := idBesideCopy(t, root); got != copied.Pair {
		t.Fatalf("the id beside the copy is %q, want %q", got, copied.Pair)
	}
	// It stays the same from save to save.
	seed.add("terminalProfiles", sharedProfile("b", "B", "192.0.2.2"))
	seed.save()
	var later savedListsCopy
	if err := json.Unmarshal([]byte(r3Copy(t, root)), &later); err != nil || later.Pair != copied.Pair {
		t.Fatalf("the copy's id changed from %q to %q", copied.Pair, later.Pair)
	}
	// v1.21.40 writes the copy without the id. The next save of this version
	// writes it again under the same id, so no app takes the file for new.
	var plainCopy map[string]json.RawMessage
	_ = json.Unmarshal([]byte(r3Copy(t, root)), &plainCopy)
	delete(plainCopy, "pair")
	withoutID, _ := json.Marshal(plainCopy)
	if err := os.WriteFile(filepath.Join(root, savedListsFilename), withoutID, 0600); err != nil {
		t.Fatal(err)
	}
	seed.add("terminalProfiles", sharedProfile("c", "C", "192.0.2.3"))
	seed.save()
	if err := json.Unmarshal([]byte(r3Copy(t, root)), &later); err != nil || later.Pair != copied.Pair {
		t.Fatalf("after a save of v1.21.40 the id changed from %q to %q", copied.Pair, later.Pair)
	}
	if got, _ := r3Disk(t, root)[sharedWriterField].(string); got != copied.Pair {
		t.Fatalf("after a save of v1.21.40 the settings file's mark is %q, want %q", got, copied.Pair)
	}
	// This version's own "Delete app data" removes all of it.
	closeEditorApp(t, seed.app)
	storage, err := openStorageRoot(root, false)
	if err != nil {
		t.Fatal(err)
	}
	err = deleteStorageDataOnce(storage, defaultStorageDeleteOps())
	_ = storage.Close()
	if err != nil {
		t.Fatal(err)
	}
	left, _ := os.ReadDir(root)
	for _, entry := range left {
		if !strings.HasSuffix(entry.Name(), ".lock") {
			t.Fatalf("left after deleting app data: %s", entry.Name())
		}
	}
	// A data folder that was moved has the copy but not the id beside it;
	// the next start writes it again, so the pairing survives a move.
	moved := t.TempDir()
	other := r3Seed(t, moved)
	closeEditorApp(t, other.app)
	if err := os.RemoveAll(filepath.Join(moved, storageViewsDirectory)); err != nil {
		t.Fatal(err)
	}
	newVersionStart(t, moved)
	if idBesideCopy(t, moved) == "" {
		t.Fatal("the id beside the copy was not written again for a settings file that carries it")
	}
	// A copy from v1.21.40 has no id. It is used as that version used it.
	legacy := t.TempDir()
	first := r3Seed(t, legacy)
	closeEditorApp(t, first.app)
	var plain map[string]json.RawMessage
	_ = json.Unmarshal([]byte(r3Copy(t, legacy)), &plain)
	delete(plain, "pair")
	raw, _ := json.Marshal(plain)
	if err := os.WriteFile(filepath.Join(legacy, savedListsFilename), raw, 0600); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(legacy, appSettingsFilename))
	if page := openSharedPage(t, sharedTestApp(t, legacy, "", "main")); !sameIDs(page.ids("terminalProfiles"), "a") {
		t.Fatalf("a copy without an id was not used: %v", page.ids("terminalProfiles"))
	}
}
