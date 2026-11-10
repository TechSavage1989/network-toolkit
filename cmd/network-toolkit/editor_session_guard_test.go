package main

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// "Reopen files next time" is shared by every app of a data folder, so it can
// be off in an app whose last run kept unsaved text. Whatever the page does
// then, the engine never removes a kept copy on behalf of a process that has
// not been given the session in this run.

func keptCopyExists(a *application, backup editorSessionBackup) bool {
	_, err := os.Stat(filepath.Join(sessionDir(a), editorSessionBackupFile(backup.ID)))
	return err == nil
}

func TestEditorSessionNeverRemovesKeptCopiesTheRunWasNotGiven(t *testing.T) {
	root := t.TempDir()
	first := sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode)
	notes := keepText(t, first, "my unsaved notes")
	mustPutSession(t, first, 0, untitledTab("Untitled.txt", notes))
	closeEditorApp(t, first)

	// The next run. Its page has not asked for the session (the preference was
	// off when it started) and now saves the tabs that are open: none.
	second := sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode)
	if r := putSession(t, second, emptyEditorSession()); r.Code != http.StatusConflict {
		t.Fatalf("a session the run was never given was replaced: %d %s", r.Code, r.Body.String())
	}
	if index := diskSession(t, second); len(index.Tabs) != 1 || !keptCopyExists(second, notes) {
		t.Fatalf("the kept text is gone after a refused save: %+v", index.Tabs)
	}
	// A save that names the run's own new copy next to nothing else would
	// still drop the record of the older one.
	draft := keepText(t, second, "a new draft")
	if r := putSession(t, second, map[string]any{"version": 1, "active": 0, "tabs": []any{untitledTab("Draft.txt", draft)}}); r.Code != http.StatusConflict {
		t.Fatalf("a save dropped the record of kept text the run was never given: %d", r.Code)
	}
	if r := requestApp(second, http.MethodDelete, editorSessionEndpoint, "", true); r.Code != http.StatusConflict {
		t.Fatalf("the session was deleted without having been given: %d %s", r.Code, r.Body.String())
	}
	if index := diskSession(t, second); len(index.Tabs) != 1 || !keptCopyExists(second, notes) {
		t.Fatalf("the kept text is gone after a refused delete: %+v", index.Tabs)
	}
	// A save that keeps the older record is fine: nothing is lost by it.
	if r := putSession(t, second, map[string]any{"version": 1, "active": 0, "tabs": []any{untitledTab("Draft.txt", draft), untitledTab("Untitled.txt", notes)}}); r.Code != http.StatusOK {
		t.Fatalf("a save that keeps every record was refused: %d %s", r.Code, r.Body.String())
	}

	// Once the page has been given the session, it decides.
	if index, _ := readSession(t, second); len(index.Tabs) != 2 {
		t.Fatalf("session as given: %+v", index.Tabs)
	}
	if text := keptText(t, second, notes.ID); text != "my unsaved notes" {
		t.Fatalf("kept text: %q", text)
	}
	mustPutSession(t, second, -1)
	if keptCopyExists(second, notes) || keptCopyExists(second, draft) {
		t.Fatal("copies the page no longer names were kept after the session had been given")
	}
}

// The reviewer's case: the Text Editor kept unsaved text with the preference
// on; the data folder says off. After the one-time move the safer value is in
// force, and even with it off the kept copy survives a first save without a
// read.
func TestSharedMigrationKeepsTheSaferEditorClosePreferences(t *testing.T) {
	cases := []struct {
		name, root, workspace string
		restore, ask          any
	}{
		{"root off, editor on", `{"restoreSession":false,"askSaveBeforeClose":false,"wordWrap":false}`, `{"restoreSession":true,"askSaveBeforeClose":true,"wordWrap":true}`, true, true},
		{"root on, editor off", `{"restoreSession":true,"askSaveBeforeClose":true}`, `{"restoreSession":false,"askSaveBeforeClose":false}`, true, true},
		{"root off, editor never changed them", `{"restoreSession":false,"askSaveBeforeClose":false}`, `{"wordWrap":true}`, true, true},
		{"root never changed them, editor off", `{"wordWrap":false}`, `{"restoreSession":false,"askSaveBeforeClose":false}`, true, true},
		{"both off", `{"restoreSession":false,"askSaveBeforeClose":false}`, `{"restoreSession":false,"askSaveBeforeClose":false}`, false, false},
		{"both never changed them", `{"wordWrap":false}`, `{"wordWrap":true}`, nil, nil},
	}
	for _, c := range cases {
		root := t.TempDir()
		writeFixture(t, filepath.Join(root, appSettingsFilename), `{"theme":"dark","editorPreferences":`+c.root+`}`)
		writeFixture(t, filepath.Join(root, standaloneEditorDirectory, appSettingsFilename), `{"activePage":"editor","editorPreferences":`+c.workspace+`}`)
		if err := migrateSharedData(root); err != nil {
			t.Fatal(c.name, err)
		}
		prefs, _ := sharedDisk(t, root)["editorPreferences"].(map[string]any)
		if prefs["restoreSession"] != c.restore || prefs["askSaveBeforeClose"] != c.ask {
			t.Errorf("%s: restoreSession=%v askSaveBeforeClose=%v, want %v and %v", c.name, prefs["restoreSession"], prefs["askSaveBeforeClose"], c.restore, c.ask)
		}
		if c.name == "root off, editor on" && prefs["wordWrap"] != false {
			t.Errorf("%s: another editor preference did not keep the data folder's value: %v", c.name, prefs["wordWrap"])
		}
		// Running it again changes nothing, also after the user chose "off".
		main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
		main.set("editorPreferences", "restoreSession", false)
		main.save()
		if err := migrateSharedData(root); err != nil {
			t.Fatal(c.name, err)
		}
		if prefs, _ := sharedDisk(t, root)["editorPreferences"].(map[string]any); prefs["restoreSession"] != false {
			t.Errorf("%s: a later start turned the preference back on: %v", c.name, prefs["restoreSession"])
		}
		// Nor when an older version of the Text Editor, which still has the
		// preference on, saves its own settings file again.
		writeFixture(t, filepath.Join(root, standaloneEditorDirectory, appSettingsFilename), `{"activePage":"editor","editorPreferences":{"restoreSession":true,"askSaveBeforeClose":true,"fontSize":15}}`)
		if err := migrateSharedData(root); err != nil {
			t.Fatal(c.name, err)
		}
		if prefs, _ := sharedDisk(t, root)["editorPreferences"].(map[string]any); prefs["restoreSession"] != false {
			t.Errorf("%s: an older version's settings turned the preference back on: %v", c.name, prefs)
		}
	}
}

// The preference is switched off in the main window while the Text Editor is
// closed with unsaved kept text, and while it is running. Nothing the engine
// does for either removes that text.
func TestEditorSessionKeptTextSurvivesThePreferenceBeingSwitchedOffElsewhere(t *testing.T) {
	root, main, editor := editorSessionApps(t)
	mainPage := openSharedPage(t, main)

	// While the Text Editor is running with an unsaved document.
	notes := keepText(t, editor, "typed in the Text Editor")
	mustPutSession(t, editor, 0, untitledTab("Untitled.txt", notes))
	editorPage := openSharedPage(t, editor)
	mainPage.set("editorPreferences", "restoreSession", false)
	mainPage.save()
	if !editorPage.poll() || editorPage.member("editorPreferences", "restoreSession") != false {
		t.Fatalf("the running Text Editor was not told that the preference is off: %v", editorPage.state["editorPreferences"])
	}
	if !keptCopyExists(editor, notes) || len(diskSession(t, editor).Tabs) != 1 {
		t.Fatal("switching the preference off elsewhere removed the running editor's kept text")
	}

	// The Text Editor is closed with that text kept; the main window keeps
	// running and reads its own session. It does not take the text of a
	// closed app away for good either: it reopens it here.
	closeEditorApp(t, editor)
	index, _ := readSession(t, main)
	if len(index.Tabs) != 1 || index.Tabs[0].Backup == nil || keptText(t, main, index.Tabs[0].Backup.ID) != "typed in the Text Editor" {
		t.Fatalf("the closed editor's unsaved text was not offered to the running app: %+v", index.Tabs)
	}

	// A Text Editor that is closed while the preference is switched off, then
	// started: its page is given the session whatever the preference is.
	root2 := t.TempDir()
	main2 := sharedTestApp(t, root2, "", "main")
	editor2 := sharedTestApp(t, root2, standaloneEditorDirectory, standaloneEditorMode)
	kept := keepText(t, editor2, "kept while closed")
	mustPutSession(t, editor2, 0, untitledTab("Untitled.txt", kept))
	closeEditorApp(t, editor2)
	page2 := openSharedPage(t, main2)
	page2.set("editorPreferences", "restoreSession", false)
	page2.save()
	closeEditorApp(t, main2) // the main window never looked at editor sessions
	restarted := sharedTestApp(t, root2, standaloneEditorDirectory, standaloneEditorMode)
	if page := openSharedPage(t, restarted); page.member("editorPreferences", "restoreSession") != false {
		t.Fatalf("fixture: the preference should be off: %v", page.state["editorPreferences"])
	}
	// Whatever the page sends before it has read the session is refused.
	if r := putSession(t, restarted, emptyEditorSession()); r.Code != http.StatusConflict {
		t.Fatalf("the kept session was replaced unread: %d", r.Code)
	}
	if r := requestApp(restarted, http.MethodDelete, editorSessionEndpoint, "", true); r.Code != http.StatusConflict {
		t.Fatalf("the kept session was deleted unread: %d", r.Code)
	}
	given, _ := readSession(t, restarted)
	if len(given.Tabs) != 1 || keptText(t, restarted, kept.ID) != "kept while closed" {
		t.Fatalf("the restarted Text Editor was not given its unsaved text: %+v", given.Tabs)
	}
	_ = root
}
