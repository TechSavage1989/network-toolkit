package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sharedFilesOf lists the shared files an app of a data folder works with.
func sharedFilesOf(t *testing.T, root, workspace, mode string) []string {
	t.Helper()
	dir := filepath.Join(root, workspace)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	a := newApplicationForRoot(context.Background(), filepath.Join(dir, appSettingsFilename), root)
	a.windowMode = mode
	t.Cleanup(func() {
		a.stop()
		if a.terminals != nil {
			_ = a.terminals.Close()
		}
	})
	settings := a.sharedSettingsPath()
	return []string{
		settings,
		// The credential vault and its configuration identity live beside the
		// shared settings (newCredentialVaultForSettings).
		filepath.Join(filepath.Dir(settings), credentialVaultV2Filename),
		filepath.Join(a.sharedRoot(), knownHostsFilename),
		filepath.Join(a.sharedRoot(), terminalMacroLibraryFilename),
		filepath.Join(a.sharedRoot(), sharedMarkerFilename),
	}
}

func TestSharedDataEveryEditionAndAppUsesTheSameFiles(t *testing.T) {
	user := t.TempDir()
	t.Setenv("LOCALAPPDATA", user)
	t.Setenv("XDG_CONFIG_HOME", user)
	// The portable copy runs from wherever it was unpacked; Setup installs
	// the same program elsewhere. Neither location decides where data lives.
	portable := filepath.Join(t.TempDir(), "Downloads", "Network Toolkit portable")
	installed := filepath.Join(t.TempDir(), "Programs", "Network Toolkit")
	apps := []struct{ name, workspace, mode string }{{"main window", "", "main"}, {"standalone Terminal", terminalWorkspaceName, "terminal"}, {"standalone Text Editor", standaloneEditorDirectory, standaloneEditorMode}}
	check := func(stage, want string) {
		t.Helper()
		var reference []string
		for _, executableDir := range []string{portable, installed} {
			config, err := resolveAppStorage(executableDir, "")
			if err != nil {
				t.Fatal(err)
			}
			if config.RootDir != want || config.Explicit {
				t.Fatalf("%s: %s resolves the data folder %s, want %s", stage, executableDir, config.RootDir, want)
			}
			for _, app := range apps {
				files := sharedFilesOf(t, config.RootDir, app.workspace, app.mode)
				if reference == nil {
					reference = files
				}
				if strings.Join(files, "\n") != strings.Join(reference, "\n") {
					t.Fatalf("%s: the %s started from %s uses\n%s\nbut the first app uses\n%s", stage, app.name, executableDir, strings.Join(files, "\n"), strings.Join(reference, "\n"))
				}
				for _, file := range files {
					if filepath.Dir(file) != want {
						t.Fatalf("%s: the %s keeps shared data outside the data folder: %s", stage, app.name, file)
					}
				}
			}
			// A Terminal window opened from the main window is told the data
			// folder and resolves the same one.
			args, err := terminalChildArguments(config.RootDir, nil, false)
			if err != nil || len(args) != 3 || args[1] != "--data-dir" || args[2] != config.RootDir {
				t.Fatalf("%s: Terminal child arguments: %v %v", stage, args, err)
			}
			if child, err := resolveAppStorage(executableDir, args[2]); err != nil || child.RootDir != config.RootDir {
				t.Fatalf("%s: the Terminal child resolves %+v %v", stage, child, err)
			}
		}
	}
	defaultDir := filepath.Join(user, "NetworkToolkit")
	check("default data folder", defaultDir)

	// Saved data from each app, then the user moves the data folder.
	main := openSharedPage(t, sharedTestApp(t, defaultDir, "", "main"))
	main.add("terminalProfiles", sharedProfile("from-portable", "Core", "192.0.2.1"))
	main.state["theme"] = "light"
	main.save()
	terminal := openSharedPage(t, sharedTestApp(t, defaultDir, terminalWorkspaceName, "terminal"))
	terminal.add("terminalProfiles", sharedProfile("from-terminal", "Edge", "192.0.2.2"))
	terminal.save()
	config, err := resolveAppStorage(portable, "")
	if err != nil {
		t.Fatal(err)
	}
	manager := newAppStorage(config)
	if err := manager.Open(); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(t.TempDir(), "moved data")
	if _, _, err := manager.changeLocation(moved, true); err != nil {
		t.Fatal(err)
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	// Every edition and every app follows the move, and finds the same data.
	check("after moving the data folder", moved)
	for _, app := range apps {
		page := openSharedPage(t, sharedTestApp(t, moved, app.workspace, app.mode))
		if !sameIDs(page.ids("terminalProfiles"), "from-portable", "from-terminal") || page.state["theme"] != "light" {
			t.Fatalf("the %s after the move: %v theme=%v", app.name, page.ids("terminalProfiles"), page.state["theme"])
		}
	}
}

func TestSharedDataFilesMoveAndAreDeletedWithAppData(t *testing.T) {
	manager := storageFixture(t, false)
	root := manager.config.RootDir
	settings := `{"theme":"dark","terminalProfiles":[{"id":"p","name":"Core","type":"ssh","host":"192.0.2.1","port":22}]}`
	files := map[string]string{
		appSettingsFilename:          settings,
		sharedMarkerFilename:         `{"version":1,"sources":{"terminal/settings":{"digest":"abc"}}}`,
		sharedBackupSettings:         `{"theme":"dark"}`,
		sharedBackupMacros:           `{"version":1,"macros":[],"bindings":[]}`,
		sharedBackupKnownHosts:       `{"version":1,"hosts":[]}`,
		savedListsFilename:           `{"version":1,"lists":{"terminalProfiles":[{"id":"p","name":"Core","type":"ssh","host":"192.0.2.1","port":22}]}}`,
		terminalMacroLibraryFilename: `{"version":1,"macros":[],"bindings":[]}`,
		knownHostsFilename:           `{"version":1,"hosts":[]}`,
		filepath.Join(storageTerminalDirectory, appSettingsFilename): `{"activePage":"terminal","terminalProfiles":[{"id":"old"}]}`,
	}
	for name, contents := range files {
		writeStorageFixture(t, root, name, contents)
	}
	vault := writeStorageFixture(t, root, credentialVaultV2Filename, "encrypted vault fixture")
	lock := writeStorageFixture(t, root, sharedDataLockFilename, "")
	lease := writeStorageFixture(t, root, editorSessionLeaseName, "")

	// While another app of the data folder runs, the folder is neither moved
	// nor deleted (the user is asked to close the other windows).
	other := newAppStorage(manager.config)
	if err := other.Open(); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "chosen")
	if _, _, err := manager.changeLocation(destination, true); !errors.Is(err, ErrAppStorageBusy) {
		t.Fatalf("the data folder was moved while a standalone app was running: %v", err)
	}
	if err := manager.scheduleDelete(); !errors.Is(err, ErrAppStorageBusy) {
		t.Fatalf("app data was scheduled for deletion while a standalone app was running: %v", err)
	}
	if err := other.Close(); err != nil {
		t.Fatal(err)
	}

	// Moving: the record of the one-time migration and the untouched copies
	// move with the data, so nothing is migrated a second time over there.
	status, copied, err := manager.changeLocation(destination, true)
	if err != nil || !status.CopyPending {
		t.Fatalf("move: %+v %v", status, err)
	}
	listed := strings.Join(copied, "\n")
	for name, contents := range files {
		if !strings.Contains(listed, name) {
			t.Fatalf("%s was not moved with the data folder: %v", name, copied)
		}
		assertStorageContents(t, filepath.Join(destination, name), contents)
	}
	assertStorageContents(t, filepath.Join(destination, credentialVaultV2Filename), "encrypted vault fixture")
	if err := migrateSharedData(destination); err != nil {
		t.Fatal(err)
	}
	if got := sharedDiskIDs(t, destination, "terminalProfiles"); !sameIDs(got, "p", "old") {
		// The destination's marker names a different digest than the fixture
		// workspace, so its entry is added once, exactly as at the old place.
		t.Fatalf("saved connections at the new place: %v", got)
	}

	// Deleting app data removes the shared files, the migration record and
	// the untouched copies. Lock files stay (ownership is an OS lock on the
	// file, never its presence); the encrypted vault needs its own action.
	if err := manager.scheduleDelete(); err != nil {
		t.Fatal(err)
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	for name := range files {
		if _, err := os.Stat(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s survived deleting app data: %v", name, err)
		}
	}
	for _, kept := range []string{vault, lock, lease} {
		if _, err := os.Stat(kept); err != nil {
			t.Fatalf("%s was removed with app data: %v", kept, err)
		}
	}
}

// "Delete app data" removes everything the one-time move reads, in every
// workspace, so the next start cannot bring saved data or host trust back.
func TestSharedDeleteAppDataIsNotUndoneByTheNextStart(t *testing.T) {
	manager := storageFixture(t, false)
	root := manager.config.RootDir
	editor := filepath.Join(root, standaloneEditorDirectory)
	terminalDir := filepath.Join(root, storageTerminalDirectory)
	writeStorageFixture(t, root, appSettingsFilename, `{"theme":"dark"}`)
	writeStorageFixture(t, editor, appSettingsFilename, `{"activePage":"editor","theme":"light","accentColor":"#112233","editorPreferences":{"wordWrap":true},"terminalProfiles":[{"id":"e1","name":"From editor","type":"ssh","host":"192.0.2.3","port":22}]}`)
	writeKnownHostFixture(t, editor, knownHostFixtureEntry("192.0.2.9:22", knownHostFixtureKey(t)))
	writeStorageFixture(t, editor, terminalMacroLibraryFilename, `{"version":1,"macros":[],"bindings":[]}`)
	writeStorageFixture(t, terminalDir, appSettingsFilename, `{"terminalProfiles":[{"id":"t1","name":"Edge","type":"ssh","host":"192.0.2.2","port":22}]}`)
	writeKnownHostFixture(t, terminalDir, knownHostFixtureEntry("192.0.2.8:22", knownHostFixtureKey(t)))
	// Vaults are never removed with app data: each needs its own action.
	vaults := []string{writeStorageFixture(t, root, credentialVaultV2Filename, "vault"), writeStorageFixture(t, terminalDir, credentialVaultV2Filename, "terminal vault"), writeStorageFixture(t, editor, credentialVaultV2Filename, "editor vault")}
	unrelated := writeStorageFixture(t, editor, "notes-from-user.txt", "kept")
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "e1", "t1") {
		t.Fatalf("fixture: after the one-time move: %v", got)
	}

	if err := manager.scheduleDelete(); err != nil {
		t.Fatal(err)
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{
		filepath.Join(root, appSettingsFilename), filepath.Join(root, knownHostsFilename), filepath.Join(root, sharedMarkerFilename), filepath.Join(root, savedListsFilename),
		filepath.Join(root, sharedBackupSettings), filepath.Join(root, sharedBackupKnownHosts),
		filepath.Join(terminalDir, appSettingsFilename), filepath.Join(terminalDir, knownHostsFilename),
		filepath.Join(editor, appSettingsFilename), filepath.Join(editor, knownHostsFilename), filepath.Join(editor, terminalMacroLibraryFilename),
	} {
		if _, err := os.Stat(gone); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s survived deleting app data: %v", gone, err)
		}
	}
	for _, kept := range append(vaults, unrelated) {
		if _, err := os.Stat(kept); err != nil {
			t.Errorf("%s was removed with app data: %v", kept, err)
		}
	}

	// The next start: nothing is brought back, no host is trusted.
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{appSettingsFilename, knownHostsFilename, terminalMacroLibraryFilename, savedListsFilename} {
		if raw, err := os.ReadFile(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("the next start wrote %s again: %s", name, raw)
		}
	}
	app := sharedTestApp(t, root, "", "main")
	page := openSharedPage(t, app)
	if len(page.ids("terminalProfiles")) != 0 || page.state["theme"] != nil || len(knownHostsOf(t, app)) != 0 {
		t.Fatalf("after deleting app data the next start shows %v, theme %v, trusted hosts %v", page.ids("terminalProfiles"), page.state["theme"], knownHostsOf(t, app))
	}
}

// Moving the data folder takes the Text Editor workspace's files along too,
// so the record of the one-time move and what it was made from stay together.
func TestSharedMoveTakesEveryWorkspaceTheOneTimeMoveReads(t *testing.T) {
	manager := storageFixture(t, false)
	root := manager.config.RootDir
	files := map[string]string{
		appSettingsFilename: `{"theme":"dark"}`,
		filepath.Join(standaloneEditorDirectory, appSettingsFilename):          `{"activePage":"editor","terminalProfiles":[{"id":"e1","name":"From editor","type":"ssh","host":"192.0.2.3","port":22}]}`,
		filepath.Join(standaloneEditorDirectory, terminalMacroLibraryFilename): `{"version":1,"macros":[],"bindings":[]}`,
		filepath.Join(storageTerminalDirectory, appSettingsFilename):           `{"terminalProfiles":[{"id":"t1","name":"Edge","type":"ssh","host":"192.0.2.2","port":22}]}`,
	}
	for name, contents := range files {
		writeStorageFixture(t, root, name, contents)
	}
	writeKnownHostFixture(t, filepath.Join(root, standaloneEditorDirectory), knownHostFixtureEntry("192.0.2.9:22", knownHostFixtureKey(t)))
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "chosen")
	status, copied, err := manager.changeLocation(destination, true)
	if err != nil || !status.CopyPending {
		t.Fatalf("move: %+v %v", status, err)
	}
	listed := strings.Join(copied, "\n")
	for _, name := range []string{filepath.Join(standaloneEditorDirectory, appSettingsFilename), filepath.Join(standaloneEditorDirectory, knownHostsFilename), filepath.Join(standaloneEditorDirectory, terminalMacroLibraryFilename), filepath.Join(storageTerminalDirectory, appSettingsFilename), sharedMarkerFilename, savedListsFilename} {
		if !strings.Contains(listed, name) {
			t.Errorf("%s was not moved with the data folder: %v", name, copied)
		}
	}
	// At the new place the next start has nothing left to do.
	before := snapshotFiles(t, destination)
	if err := migrateSharedData(destination); err != nil {
		t.Fatal(err)
	}
	if diff := sameFiles(before, snapshotFiles(t, destination)); diff != "" {
		t.Fatalf("the next start at the new place changed: %s", diff)
	}
	if got := sharedDiskIDs(t, destination, "terminalProfiles"); !sameIDs(got, "e1", "t1") {
		t.Fatalf("saved sessions at the new place: %v", got)
	}
}
