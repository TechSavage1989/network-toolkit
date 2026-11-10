package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func r3Disk(t *testing.T, root string) map[string]any {
	raw, _ := os.ReadFile(filepath.Join(root, appSettingsFilename))
	var disk map[string]any
	_ = json.Unmarshal(raw, &disk)
	return disk
}
func r3Copy(t *testing.T, root string) string {
	raw, _ := os.ReadFile(filepath.Join(root, savedListsFilename))
	return string(raw)
}
func r3Seed(t *testing.T, root string) *sharedPage {
	main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	main.add("terminalProfiles", sharedProfile("a", "A", "192.0.2.1"))
	main.state["profiles"] = []any{map[string]any{"id": "target-1", "name": "Core router", "target": "192.0.2.10", "tool": "ping"}}
	main.state["tftpProfiles"] = []any{map[string]any{"id": "tftp-1", "name": "Lab TFTP", "host": "192.0.2.20", "port": float64(69)}}
	main.state["savedResultFilters"] = map[string]any{"ping": []any{map[string]any{"id": "f1", "name": "Slow", "query": "ms>100"}}}
	main.state["transferServerSettings"] = map[string]any{"tftpPort": float64(6969)}
	main.save()
	d := r3Disk(t, root)
	t.Logf("seed disk: filters=%v server=%v", d["savedResultFilters"], d["transferServerSettings"])
	return main
}

// file missing, standalone creates the vault identity (the natural step
// after the vault locked because the identity is gone), main not running.
func TestSharedMissingFileIdentityCreationOnMissingFile(t *testing.T) {
	root := t.TempDir()
	r3Seed(t, root)
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	if err := os.Remove(filepath.Join(root, appSettingsFilename)); err != nil {
		t.Fatal(err)
	}
	terminal.poll()
	if _, err := terminal.app.ensureCredentialConfigIdentity(); err != nil {
		t.Fatal(err)
	}
	t.Logf("after identity: disk=%v\ncopy=%s", r3Disk(t, root), r3Copy(t, root))
	terminal.state["terminalSidebarWidth"] = float64(321)
	terminal.save()
	terminal.poll()
	d := r3Disk(t, root)
	t.Logf("after per-window save: terminalProfiles=%v profiles=%v tftp=%v\ncopy=%s", d["terminalProfiles"], d["profiles"], d["tftpProfiles"], r3Copy(t, root))
	late := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	t.Logf("terminal restarted: terminalProfiles=%v", late.state["terminalProfiles"])
	if l, _ := d["terminalProfiles"].([]any); len(l) != 1 {
		t.Errorf("saved sessions not on disk: %v", d["terminalProfiles"])
	}
	if l, _ := d["profiles"].([]any); len(l) != 1 {
		t.Errorf("saved targets not on disk: %v", d["profiles"])
	}
	// now a real change
	terminal.add("terminalProfiles", sharedProfile("b", "B", "192.0.2.2"))
	terminal.save()
	d = r3Disk(t, root)
	t.Logf("after shared save: terminalProfiles=%v profiles=%v tftp=%v\ncopy=%s", d["terminalProfiles"], d["profiles"], d["tftpProfiles"], r3Copy(t, root))
	if l, _ := d["profiles"].([]any); len(l) != 1 {
		t.Errorf("saved targets not on disk after shared save: %v", d["profiles"])
	}
}

// main window did not poll while the file was missing.
func TestSharedMissingFileMainDidNotSeeMissing(t *testing.T) {
	root := t.TempDir()
	main := r3Seed(t, root)
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	if err := os.Remove(filepath.Join(root, appSettingsFilename)); err != nil {
		t.Fatal(err)
	}
	terminal.add("terminalProfiles", sharedProfile("b", "B", "192.0.2.2"))
	terminal.save()
	main.poll()
	main.state["activePage"] = "overview"
	main.save()
	d := r3Disk(t, root)
	t.Logf("disk: profiles=%v tftp=%v filters=%v server=%v", d["profiles"], d["tftpProfiles"], d["savedResultFilters"], d["transferServerSettings"])
	if d["savedResultFilters"] == nil || d["transferServerSettings"] == nil {
		t.Errorf("main-only settings lost: filters=%v server=%v", d["savedResultFilters"], d["transferServerSettings"])
	}
	if l, _ := d["profiles"].([]any); len(l) != 1 {
		t.Errorf("saved targets lost")
	}
}

// same, main saves without polling first.
func TestSharedMissingFileMainDidNotSeeMissingNoPoll(t *testing.T) {
	root := t.TempDir()
	main := r3Seed(t, root)
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	_ = os.Remove(filepath.Join(root, appSettingsFilename))
	terminal.add("terminalProfiles", sharedProfile("b", "B", "192.0.2.2"))
	terminal.save()
	main.state["activePage"] = "overview"
	main.save()
	d := r3Disk(t, root)
	t.Logf("disk: profiles=%v filters=%v server=%v", d["profiles"], d["savedResultFilters"], d["transferServerSettings"])
	if d["savedResultFilters"] == nil || d["transferServerSettings"] == nil {
		t.Errorf("main-only settings lost: filters=%v server=%v", d["savedResultFilters"], d["transferServerSettings"])
	}
}

// engine-side update (updateSharedLocked) from a standalone app on a missing file.
func TestSharedMissingFileEngineUpdateOnMissingFile(t *testing.T) {
	root := t.TempDir()
	r3Seed(t, root)
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	_ = os.Remove(filepath.Join(root, appSettingsFilename))
	terminal.app.stateMu.Lock()
	err := terminal.app.updateSharedLocked(func(shared map[string]any) error { return nil })
	terminal.app.stateMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	d := r3Disk(t, root)
	t.Logf("disk: terminalProfiles=%v profiles=%v tftp=%v\ncopy=%s", d["terminalProfiles"], d["profiles"], d["tftpProfiles"], r3Copy(t, root))
	if l, _ := d["profiles"].([]any); len(l) != 1 {
		t.Errorf("saved targets lost by engine update")
	}
}

// as T1 but the main window is running and polling.
func TestSharedMissingFileIdentityCreationOnMissingFileMainRunning(t *testing.T) {
	root := t.TempDir()
	main := r3Seed(t, root)
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	_ = os.Remove(filepath.Join(root, appSettingsFilename))
	main.poll()
	terminal.poll()
	if _, err := terminal.app.ensureCredentialConfigIdentity(); err != nil {
		t.Fatal(err)
	}
	main.poll()
	t.Logf("main page after poll: terminalProfiles=%v profiles=%v", main.ids("terminalProfiles"), main.state["profiles"])
	main.state["activePage"] = "overview"
	main.save()
	terminal.poll()
	t.Logf("terminal page after poll: terminalProfiles=%v", terminal.ids("terminalProfiles"))
	d := r3Disk(t, root)
	t.Logf("disk: terminalProfiles=%v profiles=%v\ncopy=%s", d["terminalProfiles"], d["profiles"], r3Copy(t, root))
	if l, _ := d["terminalProfiles"].([]any); len(l) != 1 {
		t.Errorf("saved sessions lost everywhere")
	}
}

// "lost" stays set after an engine-side write; a later removal by another main window comes back.
func TestSharedMissingFileLostStaysSet(t *testing.T) {
	root := t.TempDir()
	mainA := r3Seed(t, root)
	mainA.state["overviewLayout"] = map[string]any{"cards": []any{"a", "b"}}
	mainA.save()
	_ = os.Remove(filepath.Join(root, appSettingsFilename))
	mainA.poll()
	mainA.app.stateMu.Lock()
	err := mainA.app.updateSharedLocked(func(shared map[string]any) error { return nil })
	lost := mainA.app.shared.lost
	mainA.app.stateMu.Unlock()
	t.Logf("after engine write: err=%v lost=%v disk overview=%v", err, lost, r3Disk(t, root)["overviewLayout"])
	if lost {
		t.Errorf("lost still set after this app wrote the whole file itself")
	}
}

// No main window is running. The settings file is lost; the Terminal app and
// then the Text Editor app each save. A main window started afterwards has
// everything, including what only it uses, because the copy holds that.
func TestSharedMissingFileTwoStandaloneAppsRecreateAndMainStartsLater(t *testing.T) {
	root := t.TempDir()
	seed := r3Seed(t, root)
	seed.state["overviewLayout"] = map[string]any{"cards": []any{"a", "b"}}
	seed.save()
	closeEditorApp(t, seed.app)
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	editor := openSharedPage(t, sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode))
	if err := os.Remove(filepath.Join(root, appSettingsFilename)); err != nil {
		t.Fatal(err)
	}
	terminal.add("terminalProfiles", sharedProfile("b", "B", "192.0.2.2"))
	terminal.save()
	editor.set("editorPreferences", "wordWrap", true)
	editor.save()
	main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	if !sameIDs(main.ids("terminalProfiles"), "a", "b") || len(main.ids("profiles")) != 1 || len(main.ids("tftpProfiles")) != 1 {
		t.Fatalf("lists in a main window started later: %v %v %v", main.ids("terminalProfiles"), main.ids("profiles"), main.ids("tftpProfiles"))
	}
	for _, name := range []string{"savedResultFilters", "transferServerSettings", "overviewLayout"} {
		if main.state[name] == nil {
			t.Errorf("%s is gone in a main window started later", name)
		}
	}
	if main.member("editorPreferences", "wordWrap") != true {
		t.Error("the Text Editor's change is gone")
	}
}

// An app that starts while the settings file is missing but its copy is
// there does not start empty and does not empty the copy with its first save.
func TestSharedMissingFileAppStartedWithoutTheFileUsesTheCopy(t *testing.T) {
	root := t.TempDir()
	seed := r3Seed(t, root)
	closeEditorApp(t, seed.app)
	if err := os.Remove(filepath.Join(root, appSettingsFilename)); err != nil {
		t.Fatal(err)
	}
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	if !sameIDs(terminal.ids("terminalProfiles"), "a") {
		t.Fatalf("a Terminal started without the settings file shows %v", terminal.ids("terminalProfiles"))
	}
	terminal.state["theme"] = "light"
	terminal.save()
	d := r3Disk(t, root)
	if l, _ := d["terminalProfiles"].([]any); len(l) != 1 {
		t.Fatalf("saved sessions after its first save: %v", d["terminalProfiles"])
	}
	if l, _ := d["profiles"].([]any); len(l) != 1 || d["savedResultFilters"] == nil {
		t.Fatalf("main-window data after its first save: %v %v", d["profiles"], d["savedResultFilters"])
	}
	// Deleting app data removes the copy too: then an app does start empty.
	_ = os.Remove(filepath.Join(root, appSettingsFilename))
	_ = os.Remove(filepath.Join(root, savedListsFilename))
	if fresh := openSharedPage(t, sharedTestApp(t, root, "", "main")); len(fresh.ids("terminalProfiles")) != 0 {
		t.Fatalf("an app started without file and copy shows %v", fresh.ids("terminalProfiles"))
	}
}

// Neither the settings file nor a usable copy. A standalone app used to
// refuse to save then ("open the main window"); that left what it holds
// nowhere but in its memory and was no help when the main window was not
// running (shared_review4_test.go). It now writes what it holds and notes
// that the main window's data is absent. What this test is about stays: the
// main window's data is never lost, and the Terminal's change is saved.
func TestSharedMissingFileAndCopyStandaloneWritesAndMainKeepsItsData(t *testing.T) {
	for _, damage := range []string{"missing", "unreadable"} {
		root := t.TempDir()
		main := r3Seed(t, root)
		terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
		_ = os.Remove(filepath.Join(root, appSettingsFilename))
		if damage == "missing" {
			_ = os.Remove(filepath.Join(root, savedListsFilename))
		} else if err := os.WriteFile(filepath.Join(root, savedListsFilename), []byte("{not json"), 0600); err != nil {
			t.Fatal(err)
		}
		terminal.add("terminalProfiles", sharedProfile("b", "B", "192.0.2.2"))
		if code, body := terminal.trySave(); code != 200 {
			t.Fatalf("%s copy: the Terminal's save: %d %s", damage, code, body)
		}
		d := r3Disk(t, root)
		if _, noted := d[sharedMainOnlyAbsentField]; !noted || d["profiles"] != nil {
			t.Fatalf("%s copy: the file the Terminal wrote does not say that main-window data is absent: %v", damage, d)
		}
		if strings.Contains(r3Copy(t, root), "not json") {
			t.Fatalf("%s copy: the copy was not written again", damage)
		}
		terminal.app.stateMu.Lock()
		err := terminal.app.updateSharedLocked(func(map[string]any) error { return nil })
		terminal.app.stateMu.Unlock()
		if err != nil {
			t.Fatalf("%s copy: an engine-side save: %v", damage, err)
		}
		if _, err := terminal.app.ensureCredentialConfigIdentity(); err != nil {
			t.Fatalf("%s copy: the vault identity: %v", damage, err)
		}
		main.state["activePage"] = "overview"
		main.save()
		d = r3Disk(t, root)
		if l, _ := d["profiles"].([]any); len(l) != 1 || d["savedResultFilters"] == nil {
			t.Fatalf("%s copy: the main window did not write everything it holds: %v", damage, d["profiles"])
		}
		if _, noted := d[sharedMainOnlyAbsentField]; noted {
			t.Fatalf("%s copy: the note stayed after the main window wrote its data", damage)
		}
		terminal.save()
		if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "a", "b") {
			t.Fatalf("%s copy: after the main window wrote the file: %v", damage, got)
		}
		if !strings.Contains(r3Copy(t, root), "target-1") {
			t.Fatalf("%s copy: the copy lacks the main window's targets in the end: %s", damage, r3Copy(t, root))
		}
	}
}

// A notice that could not be recorded is kept and recorded later.
func TestSharedNoticeThatCouldNotBeRecordedIsKept(t *testing.T) {
	root := t.TempDir()
	app := sharedTestApp(t, root, "", "main")
	openSharedPage(t, app)
	app.stateMu.Lock()
	app.shared.pendingPutBack = []string{"Core switch"}
	app.stateMu.Unlock()
	// The record cannot be written: a folder is in its place.
	if err := os.Mkdir(filepath.Join(root, sharedMarkerFilename), 0700); err != nil {
		t.Fatal(err)
	}
	app.flushSharedNotices()
	app.stateMu.Lock()
	kept := append([]string{}, app.shared.pendingPutBack...)
	app.stateMu.Unlock()
	if len(kept) != 1 {
		t.Fatalf("the names of a notice that was not recorded were dropped: %v", kept)
	}
	if err := os.Remove(filepath.Join(root, sharedMarkerFilename)); err != nil {
		t.Fatal(err)
	}
	app.flushSharedNotices()
	if notices := sharedNotices(root); len(notices) != 1 || notices[0].Items[0] != "Core switch" {
		t.Fatalf("the notice after the record could be written: %+v", notices)
	}
}

// Every writer of the data folder's settings file reads it with
// readSharedSettings (which fills a missing file from the copy) and, where
// the file may be missing, calls recreateRootLocked. A new writer changes
// these numbers and has to be looked at.
func TestSharedSettingsWritersAreKnown(t *testing.T) {
	want := map[string][2]int{ // file: calls of writeSharedSettings, calls of recreateRootLocked
		"shared_state_http.go":        {3, 2}, // putState (main, standalone), updateSharedLocked
		"credential_vault_v2_http.go": {1, 1}, // ensureCredentialConfigIdentity
		"credential_vault_v2.go":      {1, 0}, // invalidateCredentialConfigIdentity: only for a file that exists
		"shared_migration.go":         {1, 0}, // settings(): members come from readSharedSettings, nothing held
		"shared_older_version.go":     {1, 0}, // olderVersion(): only for a file that exists
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, _ := os.ReadFile(name)
		source := string(raw)
		writes := strings.Count(source, "writeSharedSettings(") - strings.Count(source, "func writeSharedSettings(")
		recreates := strings.Count(source, "recreateRootLocked(") - strings.Count(source, ") recreateRootLocked(")
		if expected := want[name]; writes != expected[0] || recreates != expected[1] {
			t.Errorf("%s: %d writes of the settings file and %d calls of recreateRootLocked, expected %v", name, writes, recreates, expected)
		}
		if writes > 0 && !strings.Contains(source, "readSharedSettings(") {
			t.Errorf("%s writes the settings file without reading it with readSharedSettings", name)
		}
	}
}
