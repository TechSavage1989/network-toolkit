package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sessionApp(t *testing.T, workspace, mode string) *application {
	t.Helper()
	a := testApplication(t)
	a.dataPath = filepath.Join(workspace, "Network_Toolkit.settings.json")
	a.windowMode = mode
	return a
}

func untitledSession(b editorSessionBackup) editorSessionIndex {
	return editorSessionIndex{Version: 1, Active: 0, Tabs: []editorSessionTab{{Kind: "untitled", Name: "Untitled.txt", Dirty: true, Format: sessionFormat(), Backup: &b}}}
}

func TestAppStorageMoveCarriesEditorSessionsAndDeleteRemovesThem(t *testing.T) {
	manager := storageFixture(t, false)
	root := manager.config.RootDir
	if err := os.MkdirAll(filepath.Join(root, standaloneEditorDirectory), 0700); err != nil {
		t.Fatal(err)
	}
	main := sessionApp(t, root, "main")
	editor := sessionApp(t, filepath.Join(root, standaloneEditorDirectory), standaloneEditorMode)
	first := keepText(t, main, "main window draft")
	editorCopy := keepText(t, editor, "standalone draft")
	if r := putSession(t, main, untitledSession(first)); r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	if r := putSession(t, editor, untitledSession(editorCopy)); r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	orphan := writeStorageFixture(t, root, filepath.Join(editorSessionDirectory, editorSessionBackupFile(strings.Repeat("f", 32))), "not named by the index")
	destination := filepath.Join(t.TempDir(), "chosen")
	if _, _, err := manager.changeLocation(destination, true); err != nil {
		t.Fatal(err)
	}
	moved := func(workspace, id string) string {
		return filepath.Join(destination, workspace, editorSessionDirectory, editorSessionBackupFile(id))
	}
	assertStorageContents(t, moved("", first.ID), "\ufeffmain window draft")
	assertStorageContents(t, moved(standaloneEditorDirectory, editorCopy.ID), "\ufeffstandalone draft")
	if _, err := os.Stat(filepath.Join(destination, editorSessionDirectory, filepath.Base(orphan))); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a copy that no session names was moved")
	}
	// The session kept at close replaces the copy made when the folder changed.
	second := keepText(t, main, "newer draft kept at close")
	if r := putSession(t, main, untitledSession(second)); r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	assertStorageContents(t, moved("", second.ID), "\ufeffnewer draft kept at close")
	if _, err := os.Stat(moved("", first.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the replaced copy stayed in the new folder")
	}
	source, err := os.ReadFile(filepath.Join(root, editorSessionDirectory, editorSessionIndexName))
	if err != nil {
		t.Fatal(err)
	}
	assertStorageContents(t, filepath.Join(destination, editorSessionDirectory, editorSessionIndexName), string(source))
	assertStorageContents(t, moved(standaloneEditorDirectory, editorCopy.ID), "\ufeffstandalone draft")
	if _, err := os.Stat(filepath.Join(destination, editorSessionMigrating)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a temporary migration folder remained")
	}

	// A destination that already has a session is refused.
	again := storageFixture(t, false)
	writeStorageFixture(t, again.config.RootDir, "Network_Toolkit.settings.json", "{}")
	occupied := filepath.Join(t.TempDir(), "occupied")
	writeStorageFixture(t, occupied, filepath.Join(standaloneEditorDirectory, editorSessionDirectory, editorSessionIndexName), "{}")
	if _, _, err := again.changeLocation(occupied, true); err == nil {
		t.Fatal("copy overwrote an existing Text Editor session")
	}

	// Deleting app data removes the main workspace's session.
	doomed := storageFixture(t, false)
	writeStorageFixture(t, doomed.config.RootDir, filepath.Join(editorSessionDirectory, editorSessionIndexName), "{}")
	writeStorageFixture(t, doomed.config.RootDir, filepath.Join(editorSessionDirectory, editorSessionBackupFile(strings.Repeat("a", 32))), "unsaved")
	if err := doomed.scheduleDelete(); err != nil {
		t.Fatal(err)
	}
	if err := doomed.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(doomed.config.RootDir, editorSessionDirectory)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("deleting app data kept the Text Editor session", err)
	}
}

func TestAppStorageMoveKeepsAChangedDestinationSession(t *testing.T) {
	manager := storageFixture(t, false)
	main := sessionApp(t, manager.config.RootDir, "main")
	kept := keepText(t, main, "draft")
	if r := putSession(t, main, untitledSession(kept)); r.Code != http.StatusOK {
		t.Fatal(r.Body.String())
	}
	destination := filepath.Join(t.TempDir(), "chosen")
	if _, _, err := manager.changeLocation(destination, true); err != nil {
		t.Fatal(err)
	}
	edited := writeStorageFixture(t, destination, filepath.Join(editorSessionDirectory, editorSessionIndexName), `{"edited":"elsewhere"}`)
	if err := manager.Close(); err == nil {
		t.Fatal("a session changed in the destination was overwritten")
	}
	assertStorageContents(t, edited, `{"edited":"elsewhere"}`)
	if _, err := os.Stat(filepath.Join(manager.config.RootDir, editorSessionDirectory, editorSessionBackupFile(kept.ID))); err != nil {
		t.Fatal("the original session was changed", err)
	}
}

func TestEditorSessionCapabilityIsReportedOnlyForPrimaryWorkspaces(t *testing.T) {
	for mode, want := range map[string]bool{"main": true, standaloneEditorMode: true, "terminal": false} {
		a := testApplication(t)
		a.windowMode = mode
		r := requestApp(a, http.MethodGet, "/api/status", "", true)
		var status struct {
			EditorSessionAvailable bool `json:"editorSessionAvailable"`
		}
		if r.Code != http.StatusOK || json.Unmarshal(r.Body.Bytes(), &status) != nil || status.EditorSessionAvailable != want {
			t.Fatalf("%s: %d %s", mode, r.Code, r.Body.String()[:min(160, r.Body.Len())])
		}
	}
}
