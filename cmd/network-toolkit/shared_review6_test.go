package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The settings file is the user's data; the copy of the saved lists and the
// id beside it are protection. A problem with the protection never stops a
// save: the settings are saved, the copy and the id file that could not be
// read are left as they are, and the user is told which file is the problem.

// pollAnswer returns the raw answer of a refresh.
func pollAnswer(t *testing.T, app *application) map[string]any {
	t.Helper()
	r := requestApp(app, "GET", "/api/shared-data", "", true)
	var answer map[string]any
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &answer) != nil {
		t.Fatalf("refresh: %d %s", r.Code, r.Body.String())
	}
	return answer
}

func r6Mode(workspace string) string {
	if workspace == terminalWorkspaceName {
		return "terminal"
	}
	return "main"
}

// The id file can be neither read nor replaced: a folder is in its place.
func TestSharedIDFileThatIsAFolderNeverBlocksSaving(t *testing.T) {
	for _, workspace := range []string{"", terminalWorkspaceName} {
		mode := r6Mode(workspace)
		root := t.TempDir()
		page := r3Seed(t, root)
		if workspace != "" {
			page = openSharedPage(t, sharedTestApp(t, root, workspace, mode))
		}
		id, _ := r3Disk(t, root)[sharedWriterField].(string)
		witness := savedListsWitnessPath(root)
		if err := os.Remove(witness); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(witness, 0700); err != nil {
			t.Fatal(err)
		}
		want := []string{"a"}
		for i := 0; i < 3; i++ {
			name := "n" + string(rune('0'+i))
			page.add("terminalProfiles", sharedProfile(name, "N", "192.0.2.9"))
			want = append(want, name)
			if code, body := page.trySave(); code != 200 {
				t.Fatalf("%s: save %d: %d %s", mode, i, code, body)
			}
			if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, want...) {
				t.Fatalf("%s: save %d: on disk %v, want %v", mode, i, got, want)
			}
			if got, _ := r3Disk(t, root)[sharedWriterField].(string); got != id {
				t.Fatalf("%s: save %d: the settings file's id changed from %q to %q", mode, i, id, got)
			}
		}
		if copied := r3Copy(t, root); !strings.Contains(copied, `"n2"`) || !strings.Contains(copied, id) {
			t.Errorf("%s: the copy is not kept up to date: %s", mode, copied)
		}
		if info, err := os.Stat(witness); err != nil || !info.IsDir() {
			t.Errorf("%s: what is in the id file's place was touched: %v", mode, err)
		}
		problem, _ := pollAnswer(t, page.app)["protectionProblem"].(string)
		if !strings.Contains(problem, savedListsWitnessFile) || !strings.Contains(problem, storageViewsDirectory) {
			t.Errorf("%s: the user is not told which file is the problem: %q", mode, problem)
		}
		closeEditorApp(t, page.app)
		if err := migrateSharedData(root); err != nil {
			t.Errorf("%s: the next start: %v", mode, err)
		}
		fresh := openSharedPage(t, sharedTestApp(t, root, "", "main"))
		if !sameIDs(fresh.ids("terminalProfiles"), want...) {
			t.Fatalf("%s: after a restart: %v", mode, fresh.ids("terminalProfiles"))
		}
		fresh.state["theme"] = "light"
		if code, body := fresh.trySave(); code != 200 {
			t.Fatalf("%s: a save after a restart: %d %s", mode, code, body)
		}
		// The settings file is lost: the copy is still its copy.
		closeEditorApp(t, fresh.app)
		if err := os.Remove(filepath.Join(root, appSettingsFilename)); err != nil {
			t.Fatal(err)
		}
		if again := openSharedPage(t, sharedTestApp(t, root, "", "main")); !sameIDs(again.ids("terminalProfiles"), want...) {
			t.Errorf("%s: the copy was not used after the settings file was lost: %v", mode, again.ids("terminalProfiles"))
		}
	}
}

// Network_Toolkit.Views is a file: the id cannot be kept there at all. On
// every platform that reads as "there is no id file", not as "could not be
// read".
func TestSharedViewsThatIsAFileNeverBlocksSaving(t *testing.T) {
	root := t.TempDir()
	main := r3Seed(t, root)
	id, _ := r3Disk(t, root)[sharedWriterField].(string)
	if err := os.RemoveAll(filepath.Join(root, storageViewsDirectory)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, storageViewsDirectory), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, exists, err := readSavedListsWitness(root); got != "" || exists || err != nil {
		t.Fatalf("an id file below a file: %q exists=%v err=%v, want a confirmed \"does not exist\"", got, exists, err)
	}
	for i := 0; i < 3; i++ {
		main.add("terminalProfiles", sharedProfile("n"+string(rune('0'+i)), "N", "192.0.2.9"))
		if code, body := main.trySave(); code != 200 {
			t.Fatalf("save %d: %d %s", i, code, body)
		}
		if got, _ := r3Disk(t, root)[sharedWriterField].(string); got != id {
			t.Fatalf("save %d: the settings file's id changed from %q to %q", i, id, got)
		}
	}
	if raw, err := os.ReadFile(filepath.Join(root, storageViewsDirectory)); err != nil || string(raw) != "x" {
		t.Errorf("the file in the folder's place was touched: %q %v", raw, err)
	}
	if problem, _ := pollAnswer(t, main.app)["protectionProblem"].(string); !strings.Contains(problem, storageViewsDirectory) {
		t.Errorf("the user is not told which file is the problem: %q", problem)
	}
	// The copy cannot be paired here, so it is written the way v1.21.40
	// wrote it and is still used when the settings file is lost.
	closeEditorApp(t, main.app)
	if err := os.Remove(filepath.Join(root, appSettingsFilename)); err != nil {
		t.Fatal(err)
	}
	_ = migrateSharedData(root)
	if again := openSharedPage(t, sharedTestApp(t, root, "", "main")); !sameIDs(again.ids("terminalProfiles"), "a", "n0", "n1", "n2") {
		t.Errorf("the copy was not used after the settings file was lost: %v", again.ids("terminalProfiles"))
	}
}

// A copy that is damaged (empty, cut off) never blocks a save either.
func TestSharedDamagedCopyNeverBlocksSaving(t *testing.T) {
	for _, content := range []string{"", `{"version":1,"lists":{"terminalPro`} {
		for _, older := range []bool{false, true} {
			root := t.TempDir()
			seed := r3Seed(t, root)
			closeEditorApp(t, seed.app)
			if older {
				if err := os.WriteFile(filepath.Join(root, appSettingsFilename), []byte(`{"theme":"dark","terminalProfiles":[]}`), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(root, savedListsFilename), []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			if err := migrateSharedData(root); err != nil {
				t.Errorf("content=%q older=%v: the start: %v", content, older, err)
			}
			for _, workspace := range []string{"", terminalWorkspaceName} {
				page := openSharedPage(t, sharedTestApp(t, root, workspace, r6Mode(workspace)))
				page.add("terminalProfiles", sharedProfile("z"+workspace, "Z", "192.0.2.7"))
				if code, body := page.trySave(); code != 200 {
					t.Errorf("content=%q older=%v %s: %d %s", content, older, r6Mode(workspace), code, body)
				}
			}
			if !strings.Contains(r3Copy(t, root), "192.0.2.7") {
				t.Errorf("content=%q older=%v: the copy was not written again: %s", content, older, r3Copy(t, root))
			}
		}
	}
}

// An older version saved over the settings file and dropped an entry, and
// the copy (or the id beside it) cannot be read, now or at the next start.
// The file is served and saved as it is and the copy is left untouched.
// When the copy can be read again, the dropped entry is put back and named.
func TestSharedOlderFileAndProtectionThatCannotBeReadForLong(t *testing.T) {
	for _, which := range []string{savedListsFilename, savedListsWitnessFile} {
		root := t.TempDir()
		seed := r3Seed(t, root)
		closeEditorApp(t, seed.app)
		if err := os.WriteFile(filepath.Join(root, appSettingsFilename), []byte(`{"theme":"dark","terminalProfiles":[]}`), 0600); err != nil {
			t.Fatal(err)
		}
		before := r3Copy(t, root)
		failReadsOf(t, which)
		if err := migrateSharedData(root); err != nil {
			t.Errorf("%s: the start: %v", which, err)
		}
		problem := ""
		for round := 0; round < 2; round++ {
			for _, workspace := range []string{"", terminalWorkspaceName} {
				mode := r6Mode(workspace)
				app := sharedTestApp(t, root, workspace, mode)
				page := loadSharedPage(t, app)
				name := "z" + string(rune('0'+round)) + mode
				page.add("terminalProfiles", sharedProfile(name, "Z "+name, "192.0.2.7"))
				if code, body := page.trySave(); code != 200 {
					t.Fatalf("%s: round %d %s: %d %s", which, round, mode, code, body)
				}
				if !hasID(sharedDiskIDs(t, root, "terminalProfiles"), name) {
					t.Fatalf("%s: round %d %s: not on disk: %v", which, round, mode, sharedDiskIDs(t, root, "terminalProfiles"))
				}
				problem, _ = pollAnswer(t, app)["protectionProblem"].(string)
				closeEditorApp(t, app)
			}
		}
		if r3Copy(t, root) != before {
			t.Fatalf("%s: a copy that could not be read was written over:\n%s", which, r3Copy(t, root))
		}
		if !strings.Contains(problem, which) {
			t.Errorf("%s: the user is not told which file is the problem: %q", which, problem)
		}
		readCredentialFault = nil
		// The read works again: at the next start, or while an app runs.
		if which == savedListsFilename {
			newVersionStart(t, root)
		}
		page := openSharedPage(t, sharedTestApp(t, root, "", "main"))
		want := []string{"a", "z0main", "z0terminal", "z1main", "z1terminal"}
		if !sameIDs(page.ids("terminalProfiles"), want...) {
			t.Fatalf("%s: after the read worked again: %v", which, page.ids("terminalProfiles"))
		}
		page.state["theme"] = "light"
		page.save()
		page.poll()
		if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, want...) {
			t.Fatalf("%s: on disk after the read worked again: %v", which, got)
		}
		if l, _ := r3Disk(t, root)["profiles"].([]any); len(l) != 1 {
			t.Errorf("%s: the main window's targets were not put back: %v", which, l)
		}
		named := false
		for _, notice := range sharedNotices(root) {
			named = named || notice.Kind == "copy-unread" && hasName(notice.Items, "A")
		}
		if !named {
			t.Errorf("%s: what was put back is not named: %v", which, sharedNotices(root))
		}
		if problem, _ := pollAnswer(t, page.app)["protectionProblem"].(string); problem != "" {
			t.Errorf("%s: the problem is still reported: %q", which, problem)
		}
		// A removal is a removal again.
		page.remove("terminalProfiles", "a")
		page.save()
		closeEditorApp(t, page.app)
		newVersionStart(t, root)
		if again := openSharedPage(t, sharedTestApp(t, root, "", "main")); hasID(again.ids("terminalProfiles"), "a") {
			t.Errorf("%s: an entry removed afterwards came back", which)
		}
	}
}

// The settings file is missing and the copy cannot be read for long: the app
// still saves what it holds, the copy is left untouched, and what the copy
// holds comes back when it can be read.
func TestSharedMissingFileAndCopyThatCannotBeReadForLong(t *testing.T) {
	root := t.TempDir()
	seed := r3Seed(t, root)
	closeEditorApp(t, seed.app)
	if err := os.Remove(filepath.Join(root, appSettingsFilename)); err != nil {
		t.Fatal(err)
	}
	before := r3Copy(t, root)
	failReadsOf(t, savedListsFilename)
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	terminal.add("terminalProfiles", sharedProfile("n", "N", "192.0.2.9"))
	if code, body := terminal.trySave(); code != 200 {
		t.Fatalf("the Terminal's save: %d %s", code, body)
	}
	if !sameIDs(sharedDiskIDs(t, root, "terminalProfiles"), "n") || r3Copy(t, root) != before {
		t.Fatalf("disk %v; copy untouched: %v", sharedDiskIDs(t, root, "terminalProfiles"), r3Copy(t, root) == before)
	}
	readCredentialFault = nil
	terminal.poll()
	terminal.poll()
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "a", "n") {
		t.Fatalf("saved sessions after the copy could be read: %v", got)
	}
	d := r3Disk(t, root)
	if l, _ := d["profiles"].([]any); len(l) != 1 || d["savedResultFilters"] == nil {
		t.Fatalf("the main window's data after the copy could be read: %v %v", d["profiles"], d["savedResultFilters"])
	}
	if !strings.Contains(r3Copy(t, root), "192.0.2.9") || !strings.Contains(r3Copy(t, root), "target-1") {
		t.Fatalf("the copy in the end: %s", r3Copy(t, root))
	}
}

// What an app holds beyond the file is never dropped because saving it
// failed, however often: it stays pending and is saved when saving works.
func TestSharedHeldEntriesAreNeverDroppedByFailedSaves(t *testing.T) {
	root := t.TempDir()
	seed := r3Seed(t, root)
	closeEditorApp(t, seed.app)
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	terminal.add("terminalProfiles", sharedProfile("b", "B", "192.0.2.2"))
	terminal.save()
	// An older version deletes the app data; a main window starts with
	// nothing and saves.
	olderDeleteAppData(t, root)
	_ = migrateSharedData(root)
	main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	main.add("terminalProfiles", sharedProfile("x", "X", "192.0.2.8"))
	main.save()
	release, err := acquireCredentialGuard(root)
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 7; n++ {
		terminal.poll()
	}
	release()
	terminal.app.stateMu.Lock()
	message := terminal.app.storageError
	terminal.app.stateMu.Unlock()
	if !sameIDs(terminal.ids("terminalProfiles"), "a", "b", "x") {
		t.Fatalf("the Terminal after its saves failed: %v", terminal.ids("terminalProfiles"))
	}
	if !strings.Contains(message, "could not be saved") {
		t.Errorf("the failure is not shown while the entries are pending: %q", message)
	}
	// Another app removes an entry the Terminal does not hold as pending:
	// that removal is accepted, the pending entries stay.
	main.remove("terminalProfiles", "x")
	main.state["theme"] = "light"
	main.save()
	terminal.poll()
	terminal.poll()
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "a", "b") {
		t.Fatalf("on disk in the end: %v, want [a b]", got)
	}
	if got := terminal.ids("terminalProfiles"); !sameIDs(got, "a", "b") {
		t.Fatalf("the Terminal in the end: %v, want [a b]", got)
	}
	terminal.app.stateMu.Lock()
	message = terminal.app.storageError
	terminal.app.stateMu.Unlock()
	if strings.Contains(message, "could not be saved") {
		t.Errorf("the failure is still shown after the save worked: %q", message)
	}
}

// A move is not refused because the id file cannot be read or is not a
// file: the new place pairs the copy again at its next start.
func TestSharedMoveIsNotRefusedForTheIDFile(t *testing.T) {
	for _, problem := range []string{"unreadable", "folder"} {
		manager := storageFixture(t, false)
		root := manager.config.RootDir
		page := openSharedPage(t, sharedTestApp(t, root, "", "main"))
		page.add("terminalProfiles", sharedProfile("a", "A", "192.0.2.1"))
		page.save()
		closeEditorApp(t, page.app)
		id := idBesideCopy(t, root)
		if problem == "folder" {
			if err := os.Remove(savedListsWitnessPath(root)); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(savedListsWitnessPath(root), 0700); err != nil {
				t.Fatal(err)
			}
		} else {
			failReadsOf(t, savedListsWitnessFile)
		}
		destination := filepath.Join(t.TempDir(), "chosen")
		status, _, err := manager.changeLocation(destination, true)
		if err != nil || !status.CopyPending {
			t.Fatalf("%s: move: %+v %v", problem, status, err)
		}
		if err := manager.Close(); err != nil {
			t.Fatalf("%s: closing with the move pending: %v", problem, err)
		}
		readCredentialFault = nil
		if err := migrateSharedData(destination); err != nil {
			t.Fatalf("%s: the start at the new place: %v", problem, err)
		}
		if got := idBesideCopy(t, destination); got != id {
			t.Fatalf("%s: the id at the new place after its first start is %q, want %q", problem, got, id)
		}
	}
}

// The copy cannot be written (something else is in its place). The settings
// are saved all the same and say that the copy is behind; nothing is put
// back from it later, because the saves knew everything: a removal made
// meanwhile stays a removal, and the copy is written again when it can be.
func TestSharedCopyThatCannotBeWrittenNeverBlocksSaving(t *testing.T) {
	root := t.TempDir()
	main := r3Seed(t, root)
	main.add("terminalProfiles", sharedProfile("b", "B", "192.0.2.2"))
	main.save()
	copyPath := filepath.Join(root, savedListsFilename)
	if err := os.Remove(copyPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(copyPath, 0700); err != nil {
		t.Fatal(err)
	}
	main.remove("terminalProfiles", "a")
	if code, body := main.trySave(); code != 200 {
		t.Fatalf("the save: %d %s", code, body)
	}
	if got, _ := r3Disk(t, root)[sharedCopyBehindField].(string); got != "write" {
		t.Fatalf("the settings file does not say that the copy is behind: %q", got)
	}
	if problem, _ := pollAnswer(t, main.app)["protectionProblem"].(string); !strings.Contains(problem, savedListsFilename) || !strings.Contains(problem, "Your settings are saved") {
		t.Errorf("the user is not told which file is the problem: %q", problem)
	}
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	if !sameIDs(terminal.ids("terminalProfiles"), "b") {
		t.Fatalf("another app: %v", terminal.ids("terminalProfiles"))
	}
	if err := os.Remove(copyPath); err != nil {
		t.Fatal(err)
	}
	terminal.poll()
	d := r3Disk(t, root)
	if _, noted := d[sharedCopyBehindField]; noted || !sameIDs(sharedDiskIDs(t, root, "terminalProfiles"), "b") {
		t.Fatalf("after the copy could be written: note=%v disk=%v", noted, sharedDiskIDs(t, root, "terminalProfiles"))
	}
	if copied := r3Copy(t, root); !strings.Contains(copied, "192.0.2.2") || strings.Contains(copied, "192.0.2.1\"") || !strings.Contains(copied, "target-1") {
		t.Fatalf("the copy after it could be written: %s", copied)
	}
	if problem, _ := pollAnswer(t, terminal.app)["protectionProblem"].(string); problem != "" {
		t.Errorf("the problem is still reported: %q", problem)
	}
}
