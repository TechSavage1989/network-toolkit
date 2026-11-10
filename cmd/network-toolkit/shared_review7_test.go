package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The copy of the saved lists cannot be written for long. The settings file
// says so once; refreshes of idle apps try the copy again (with a backoff)
// and never write an identical settings file again.
func TestSharedUnwritableCopyDoesNotRewriteTheSettingsFile(t *testing.T) {
	root := t.TempDir()
	main := r3Seed(t, root)
	copyPath := filepath.Join(root, savedListsFilename)
	if err := os.Remove(copyPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(copyPath, 0700); err != nil {
		t.Fatal(err)
	}
	main.state["theme"] = "light"
	if code, body := main.trySave(); code != 200 {
		t.Fatalf("the save: %d %s", code, body)
	}
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	before, err := os.ReadFile(filepath.Join(root, appSettingsFilename))
	if err != nil {
		t.Fatal(err)
	}
	writes := 0
	writeRootSettingsFault = func(string) error { writes++; return nil }
	t.Cleanup(func() { writeRootSettingsFault = nil })
	for i := 0; i < 20; i++ {
		main.poll()
		terminal.poll()
	}
	if writes != 0 {
		t.Errorf("the settings file was written %d times by 20 refreshes of two idle apps", writes)
	}
	if after, _ := os.ReadFile(filepath.Join(root, appSettingsFilename)); string(after) != string(before) {
		t.Error("the settings file changed while nothing was saved")
	}
	if got, _ := r3Disk(t, root)[sharedCopyBehindField].(string); got != "write" {
		t.Fatalf("the note that the copy is behind: %q", got)
	}
	// The copy can be written again: it is written, and the note is removed
	// from the settings file once.
	if err := os.Remove(copyPath); err != nil {
		t.Fatal(err)
	}
	main.state["terminalSidebarWidth"] = float64(310)
	main.save() // any change of the file makes the other app try at once
	writes = 0
	for i := 0; i < 6; i++ {
		main.poll()
		terminal.poll()
	}
	if _, noted := r3Disk(t, root)[sharedCopyBehindField]; noted {
		t.Error("the note stayed after the copy could be written")
	}
	if copied := r3Copy(t, root); !strings.Contains(copied, "192.0.2.1") || !strings.Contains(copied, "target-1") {
		t.Errorf("the copy after it could be written: %s", copied)
	}
	if writes > 1 {
		t.Errorf("the settings file was written %d times after the copy was written", writes)
	}
}

// The settings file is lost and the copy cannot be read when the main window
// starts. Its page saves its defaults (empty objects for what only the main
// window uses). When the copy can be read, what it holds is put back under
// those defaults, not hidden by them, before the copy is written again.
func TestSharedMainWindowDefaultsDoNotHideWhatTheCopyHolds(t *testing.T) {
	root := t.TempDir()
	seed := r3Seed(t, root)
	seed.state["tableColumns"] = map[string]any{"ping": []any{"host", "loss"}}
	seed.save()
	closeEditorApp(t, seed.app)
	filters := pageJSON(r3Disk(t, root)["savedResultFilters"])
	if err := os.Remove(filepath.Join(root, appSettingsFilename)); err != nil {
		t.Fatal(err)
	}
	failReadsOf(t, savedListsFilename)
	_ = migrateSharedData(root)
	main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	// What the real page holds when nothing was loaded.
	main.state["tableColumns"] = map[string]any{"trace": []any{"hop"}}
	main.state["savedResultFilters"] = map[string]any{"version": float64(1), "scopes": map[string]any{}}
	main.state["transferServerSettings"] = map[string]any{}
	main.state["theme"] = "light"
	if code, body := main.trySave(); code != 200 {
		t.Fatalf("the save: %d %s", code, body)
	}
	readCredentialFault = nil
	main.poll()
	main.poll()
	d := r3Disk(t, root)
	columns, _ := d["tableColumns"].(map[string]any)
	if columns["ping"] == nil || columns["trace"] == nil {
		t.Errorf("chosen columns after the copy could be read: %v", d["tableColumns"])
	}
	if got := pageJSON(d["savedResultFilters"]); got != filters {
		t.Errorf("saved result filters after the copy could be read: %s, want %s", got, filters)
	}
	if server, _ := d["transferServerSettings"].(map[string]any); server["tftpPort"] != float64(6969) {
		t.Errorf("file server settings after the copy could be read: %v", d["transferServerSettings"])
	}
	if l, _ := d["profiles"].([]any); len(l) != 1 || !sameIDs(sharedDiskIDs(t, root, "terminalProfiles"), "a") {
		t.Errorf("saved lists after the copy could be read: %v %v", d["profiles"], sharedDiskIDs(t, root, "terminalProfiles"))
	}
	if copied := r3Copy(t, root); !strings.Contains(copied, "6969") || !strings.Contains(copied, `"ping"`) {
		t.Errorf("the copy lost what only it held: %s", copied)
	}
	named := false
	for _, notice := range sharedNotices(root) {
		named = named || notice.Kind == "copy-unread" && hasName(notice.Items, "A") && hasName(notice.Items, sharedRecoveredSettings)
	}
	if !named {
		t.Errorf("what was restored is not named: %v", sharedNotices(root))
	}
	if main.state["theme"] != "light" {
		t.Errorf("the page's own setting: %v", main.state["theme"])
	}
}

// The save a page makes while the app closes does not wait five seconds for
// another app: the page has to ask the user well inside the time the host
// window waits. A save that can never succeed says so with a code, so that
// the page does not try it again and again.
func TestSharedClosingSaveWaitsBrieflyAndPermanentRefusalsAreMarked(t *testing.T) {
	root := t.TempDir()
	main := r3Seed(t, root)
	release, err := acquireCredentialGuard(root)
	if err != nil {
		t.Fatal(err)
	}
	body := pageJSON(main.state)
	start := time.Now()
	r := requestApp(main.app, "PUT", "/api/state?closing=1", body, true)
	release()
	if r.Code != 409 || time.Since(start) > 2*time.Second {
		t.Fatalf("a closing save while another app saves: %d after %v", r.Code, time.Since(start))
	}
	if r := requestApp(main.app, "PUT", "/api/state?closing=1", body, true); r.Code != 200 {
		t.Fatalf("a closing save: %d %s", r.Code, r.Body.String())
	}
	// A display-name record that is newer than the stored one.
	row := main.row("terminalProfiles", "a")
	row["nameRevision"] = float64(99)
	code, answer := main.trySave()
	if code != 409 || !strings.Contains(answer, `"code":"settings-reload-required"`) {
		t.Fatalf("a save that needs a reload: %d %s", code, answer)
	}
}
