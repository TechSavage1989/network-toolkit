package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// In the shared-data code "does not exist", "could not be read" and "read
// and different" are three distinct answers. These tests make one read fail
// the way it does while another program holds the file.

func failReadsOf(t *testing.T, name string) {
	t.Helper()
	readCredentialFault = func(path string) error {
		if filepath.Base(path) == name {
			return errors.New("sharing violation")
		}
		return nil
	}
	t.Cleanup(func() { readCredentialFault = nil })
}

// idBesideCopy reads the id that pairs the copy with its settings file
// straight from its file.
func idBesideCopy(t *testing.T, root string) string {
	t.Helper()
	raw, err := os.ReadFile(savedListsWitnessPath(root))
	if os.IsNotExist(err) {
		return ""
	}
	var witness struct {
		SavedLists string `json:"savedLists"`
	}
	if err != nil || json.Unmarshal(raw, &witness) != nil {
		t.Fatalf("the id beside the copy: %v %s", err, raw)
	}
	return witness.SavedLists
}

// A read of the copy, or of the id beside it, fails once during a save. The
// settings file that was just read carries its id: the save keeps it. With a
// new id every other running app would take the file for written anew and
// add back what the save removed.
func TestSharedReadFaultDuringSaveKeepsTheFilesID(t *testing.T) {
	for _, which := range []string{savedListsFilename, savedListsWitnessFile} {
		root := t.TempDir()
		main := r3Seed(t, root)
		main.add("terminalProfiles", sharedProfile("b", "B", "192.0.2.2"))
		main.save()
		terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
		main.poll()
		before, _ := r3Disk(t, root)[sharedWriterField].(string)
		failReadsOf(t, which)
		main.remove("terminalProfiles", "a")
		main.state["theme"] = "light"
		code, body := main.trySave()
		readCredentialFault = nil
		if code != 200 {
			t.Fatalf("%s: the save: %d %s", which, code, body)
		}
		if after, _ := r3Disk(t, root)[sharedWriterField].(string); before == "" || after != before {
			t.Errorf("%s: the settings file's id changed from %q to %q", which, before, after)
		}
		if got := idBesideCopy(t, root); got != before {
			t.Errorf("%s: the id beside the copy is %q, want %q", which, got, before)
		}
		terminal.poll()
		terminal.poll()
		main.poll()
		if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "b") {
			t.Errorf("%s: the removal of a was undone: disk %v, terminal %v, main %v", which, got, terminal.ids("terminalProfiles"), main.ids("terminalProfiles"))
		}
		closeEditorApp(t, main.app)
		closeEditorApp(t, terminal.app)
	}
}

// An older version saved over the settings file and dropped an entry. At the
// next start of this version the id beside the copy, or the copy itself,
// cannot be read. That is not "the id is gone": the copy stays, and the
// entry is put back once the read works.
func TestSharedUnreadableIDOrCopyAtStartNeverCostsTheCopy(t *testing.T) {
	for _, which := range []string{savedListsWitnessFile, savedListsFilename, "control"} {
		root := t.TempDir()
		seed := r3Seed(t, root)
		closeEditorApp(t, seed.app)
		if err := os.WriteFile(filepath.Join(root, appSettingsFilename), []byte(`{"theme":"dark","terminalProfiles":[]}`), 0600); err != nil {
			t.Fatal(err)
		}
		failReadsOf(t, which)
		_ = migrateSharedData(root)
		readCredentialFault = nil
		if _, err := os.Stat(filepath.Join(root, savedListsFilename)); err != nil {
			t.Fatalf("%s: the copy after the start: %v", which, err)
		}
		if !strings.Contains(r3Copy(t, root), "192.0.2.1") {
			t.Fatalf("%s: the copy lost the entry: %s", which, r3Copy(t, root))
		}
		page := openSharedPage(t, sharedTestApp(t, root, "", "main"))
		if !sameIDs(page.ids("terminalProfiles"), "a") {
			t.Errorf("%s: the entry the older version dropped was not put back: %v", which, page.ids("terminalProfiles"))
		}
		closeEditorApp(t, page.app)
	}
}

// The same while apps run: a standalone app saves over a file an older
// version wrote, and cannot read the id beside the copy (or the copy) at
// that moment. It must not write the copy without the dropped entry. The
// save itself is not refused (shared_review6_test.go): the settings are
// saved, the copy is left as it is, and the entry comes back afterwards.
func TestSharedUnreadableIDOrCopyDuringASaveNeverCostsTheCopy(t *testing.T) {
	for _, which := range []string{savedListsWitnessFile, savedListsFilename} {
		root := t.TempDir()
		seed := r3Seed(t, root)
		closeEditorApp(t, seed.app)
		terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
		if err := os.WriteFile(filepath.Join(root, appSettingsFilename), []byte(`{"theme":"dark","terminalProfiles":[]}`), 0600); err != nil {
			t.Fatal(err)
		}
		failReadsOf(t, which)
		terminal.poll()
		terminal.set("editorPreferences", "wordWrap", true)
		before := r3Copy(t, root)
		code, body := terminal.trySave()
		readCredentialFault = nil
		if code != 200 || r3Disk(t, root)["editorPreferences"] == nil {
			t.Errorf("%s: the save while the copy cannot be used: %d %s", which, code, body)
		}
		if r3Copy(t, root) != before {
			t.Errorf("%s: the copy was written although it could not be taken into account", which)
		}
		if !strings.Contains(r3Copy(t, root), "192.0.2.1") {
			t.Fatalf("%s: the copy lost the entry the older version dropped: %s", which, r3Copy(t, root))
		}
		terminal.poll()
		terminal.save()
		if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "a") {
			t.Errorf("%s: after the read worked again: %v", which, got)
		}
		closeEditorApp(t, terminal.app)
	}
}

// The settings file is missing and the copy cannot be read at this moment
// (not: is damaged). No app writes over a copy it could not read. The save
// itself is not refused (shared_review6_test.go).
func TestSharedMissingFileAndCopyThatCannotBeReadRightNow(t *testing.T) {
	for _, which := range []string{savedListsWitnessFile, savedListsFilename} {
		root := t.TempDir()
		seed := r3Seed(t, root)
		closeEditorApp(t, seed.app)
		if err := os.Remove(filepath.Join(root, appSettingsFilename)); err != nil {
			t.Fatal(err)
		}
		failReadsOf(t, which)
		_ = migrateSharedData(root)
		page := openSharedPage(t, sharedTestApp(t, root, "", "main"))
		page.add("terminalProfiles", sharedProfile("n", "N", "192.0.2.9"))
		code, body := page.trySave()
		readCredentialFault = nil
		if code != 200 || !sameIDs(sharedDiskIDs(t, root, "terminalProfiles"), "n") {
			t.Errorf("%s: the save while the copy could not be read: %d %s, disk %v", which, code, body, sharedDiskIDs(t, root, "terminalProfiles"))
		}
		if !strings.Contains(r3Copy(t, root), "192.0.2.1") || !strings.Contains(r3Copy(t, root), "target-1") {
			t.Fatalf("%s: the copy was written over: %s", which, r3Copy(t, root))
		}
		page.poll()
		page.state["theme"] = "light"
		page.save()
		if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "a", "n") {
			t.Errorf("%s: after the read worked again: %v", which, got)
		}
		if l, _ := r3Disk(t, root)["profiles"].([]any); len(l) != 1 {
			t.Errorf("%s: the main window's targets after the read worked again: %v", which, l)
		}
		closeEditorApp(t, page.app)
	}
}

// An id file that is there but damaged was not removed by a deletion: the
// copy is used and the id is written again.
func TestSharedDamagedIDFileIsNotADeletion(t *testing.T) {
	root := t.TempDir()
	seed := r3Seed(t, root)
	closeEditorApp(t, seed.app)
	id := idBesideCopy(t, root)
	if err := os.WriteFile(savedListsWitnessPath(root), []byte("{half"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, appSettingsFilename)); err != nil {
		t.Fatal(err)
	}
	newVersionStart(t, root)
	page := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	if !sameIDs(page.ids("terminalProfiles"), "a") {
		t.Fatalf("saved sessions with a damaged id file: %v", page.ids("terminalProfiles"))
	}
	page.poll()
	if got := idBesideCopy(t, root); got != id {
		t.Fatalf("the id beside the copy is %q, want %q", got, id)
	}
}

// A standalone app that is first to save in a brand-new data folder never
// saw a settings file. Nothing of the main window's is absent from the file
// it writes, so it writes no note about that.
func TestSharedStandaloneFirstInANewFolderWritesNoNote(t *testing.T) {
	root := t.TempDir()
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	terminal.add("terminalProfiles", sharedProfile("a", "A", "192.0.2.1"))
	terminal.save()
	if _, noted := r3Disk(t, root)[sharedMainOnlyAbsentField]; noted {
		t.Error("the first save in a new folder says that main-window data is absent")
	}
	main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	main.poll()
	terminal.remove("terminalProfiles", "a")
	terminal.add("terminalProfiles", sharedProfile("b", "B", "192.0.2.2"))
	terminal.save()
	main.poll()
	main.poll()
	terminal.poll()
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "b") {
		t.Errorf("saved sessions: %v", got)
	}
	main.state["profiles"] = []any{map[string]any{"id": "target-1", "name": "Core router", "target": "192.0.2.10", "tool": "ping"}}
	main.save()
	main.state["profiles"] = []any{}
	main.save()
	terminal.state["theme"] = "light"
	terminal.save()
	main.poll()
	main.poll()
	d := r3Disk(t, root)
	if l, _ := d["profiles"].([]any); len(l) != 0 {
		t.Errorf("a removed target is back: %v", l)
	}
	if _, noted := d[sharedMainOnlyAbsentField]; noted {
		t.Error("the note is in the file in the end")
	}
}

// A data folder last used by v1.21.40 (mark 1, copy without id, no id file),
// then ordinary use with three apps of this version: every removal stays.
func TestSharedUpgradeFromTheVersionWithoutIDs(t *testing.T) {
	root := t.TempDir()
	seed := r3Seed(t, root)
	seed.add("terminalProfiles", sharedProfile("b", "B", "192.0.2.2"))
	seed.save()
	closeEditorApp(t, seed.app)
	var members, copied map[string]json.RawMessage
	_ = json.Unmarshal([]byte(pageJSON(r3Disk(t, root))), &members)
	members[sharedWriterField] = json.RawMessage("1")
	raw, _ := json.MarshalIndent(members, "", "  ")
	if err := os.WriteFile(filepath.Join(root, appSettingsFilename), raw, 0600); err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal([]byte(r3Copy(t, root)), &copied)
	delete(copied, "pair")
	raw, _ = json.Marshal(copied)
	if err := os.WriteFile(filepath.Join(root, savedListsFilename), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, storageViewsDirectory)); err != nil {
		t.Fatal(err)
	}
	newVersionStart(t, root)
	main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	terminal.remove("terminalProfiles", "a")
	terminal.save()
	main.poll()
	if !sameIDs(main.ids("terminalProfiles"), "b") {
		t.Errorf("the main window after the Terminal removed a: %v", main.ids("terminalProfiles"))
	}
	main.poll()
	terminal.poll()
	main.state["profiles"] = []any{}
	main.state["theme"] = "light"
	main.add("terminalProfiles", sharedProfile("c", "C", "192.0.2.3"))
	main.save()
	editor := openSharedPage(t, sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode))
	terminal.poll()
	editor.poll()
	terminal.state["theme"] = "dark"
	terminal.save()
	main.poll()
	editor.poll()
	main.poll()
	d := r3Disk(t, root)
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "b", "c") {
		t.Errorf("saved sessions: %v", got)
	}
	if l, _ := d["profiles"].([]any); len(l) != 0 {
		t.Errorf("a removed target is back: %v", l)
	}
	if d["theme"] != "dark" || main.state["theme"] != "dark" {
		t.Errorf("theme on disk %v, in the main window %v", d["theme"], main.state["theme"])
	}
	if _, noted := d[sharedMainOnlyAbsentField]; noted {
		t.Error("the note that main-window data is absent was written")
	}
}

// What happens when the refresh's save keeps failing is tested in
// shared_review6_test.go (TestSharedHeldEntriesAreNeverDroppedByFailedSaves):
// the values stay pending. An earlier version of this file tested that the
// app gave up after five failures, which dropped them.

// The id that pairs the copy with the settings file is refreshed like the
// files of a pending move: an id that was first written after the move was
// chosen is at the new place when the app closes.
func TestSharedMoveRefreshesTheIDBesideTheCopy(t *testing.T) {
	manager := storageFixture(t, false)
	root := manager.config.RootDir
	writeStorageFixture(t, root, appSettingsFilename, `{"theme":"dark"}`)
	destination := filepath.Join(t.TempDir(), "chosen")
	if status, _, err := manager.changeLocation(destination, true); err != nil || !status.CopyPending {
		t.Fatalf("move: %+v %v", status, err)
	}
	if idBesideCopy(t, root) != "" {
		t.Fatal("the fixture has an id already")
	}
	page := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	page.add("terminalProfiles", sharedProfile("a", "A", "192.0.2.1"))
	page.save()
	closeEditorApp(t, page.app)
	id := idBesideCopy(t, root)
	if id == "" {
		t.Fatal("the save wrote no id")
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	if got := idBesideCopy(t, destination); got != id {
		t.Fatalf("the id at the new place is %q, want %q", got, id)
	}
	if !strings.Contains(r3Copy(t, destination), id) {
		t.Fatalf("the copy at the new place: %s", r3Copy(t, destination))
	}
}
