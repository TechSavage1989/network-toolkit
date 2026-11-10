package main

// Cases found by the second review of shared app data.
import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// an entry ADDED in this version must survive when the
// older window writes the same bytes twice (for example a save, then its
// save-on-close) and a third app of this version, which was in the
// background, saves afterwards.
func TestSharedOlderWindowWritingTheSameBytesAgainDropsNothingAddedHere(t *testing.T) {
	root := t.TempDir()
	main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	main.add("terminalProfiles", sharedProfile("p1", "One", "192.0.2.1"))
	main.save()
	old := startOlderMainWindow(t, root) // holds [p1]
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	editor := openSharedPage(t, sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode))

	// The older window saves something of its own.
	old.state["theme"] = "light"
	old.save()
	// The Text Editor app sees that, then goes to the background (no polls).
	editor.poll()

	// The user saves a new session in the Terminal app of this version.
	terminal.poll()
	terminal.add("terminalProfiles", sharedProfile("z", "Added in this version", "192.0.2.9"))
	terminal.save()
	if !hasID(sharedDiskIDs(t, root, "terminalProfiles"), "z") {
		t.Fatal("fixture: z not on disk")
	}
	// The older window is closed: it writes what it holds once more, unchanged.
	old.save()

	// The Text Editor app comes to the foreground, polls, and saves a setting.
	changed := editor.poll()
	t.Logf("editor poll reported a change: %v; editor shows z: %v", changed, hasID(editor.ids("terminalProfiles"), "z"))
	editor.set("editorPreferences", "wordWrap", true)
	editor.save()

	disk := sharedDiskIDs(t, root, "terminalProfiles")
	t.Logf("disk after the Text Editor saved: %v", disk)
	var named []string
	for _, notice := range sharedNotices(root) {
		if notice.Kind == "older-version" {
			named = append(named, notice.Items...)
		}
	}
	t.Logf("older-version notices name: %s", strings.Join(named, ", "))
	// A later start of this version: is z anywhere?
	newVersionStart(t, root)
	late := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	t.Logf("an app started later shows: %v", late.ids("terminalProfiles"))
	if !hasID(disk, "z") || !hasID(late.ids("terminalProfiles"), "z") {
		t.Fatalf("a session saved in this version was dropped for good: disk=%v", disk)
	}
}

// an entry removed in this version, which the older
// window never held, must not come back because the older window writes the
// same bytes twice.
func TestSharedOlderWindowWritingTheSameBytesAgainBringsNothingBack(t *testing.T) {
	root := t.TempDir()
	main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	main.add("terminalProfiles", sharedProfile("p1", "One", "192.0.2.1"))
	main.save()
	old := startOlderMainWindow(t, root) // holds [p1]
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	terminal.add("terminalProfiles", sharedProfile("x", "Removed later", "192.0.2.2"))
	terminal.save()
	main.poll()
	if !hasID(main.ids("terminalProfiles"), "x") {
		t.Fatal("fixture: main does not show x")
	}
	// The older window saves what it holds (it never had x).
	old.state["theme"] = "light"
	old.save()
	main.poll()
	terminal.poll()
	if !hasID(main.ids("terminalProfiles"), "x") || !hasID(terminal.ids("terminalProfiles"), "x") {
		t.Fatal("fixture: x was not put back")
	}
	// The user removes x in this version (Terminal app).
	terminal.remove("terminalProfiles", "x")
	terminal.save()
	if hasID(sharedDiskIDs(t, root, "terminalProfiles"), "x") {
		t.Fatal("fixture: x still on disk after its removal")
	}
	// The older window saves again without any change of its own: same bytes.
	old.save()
	// The main window (was in the background) looks again and saves something
	// unrelated.
	changed := main.poll()
	t.Logf("main poll reported a change: %v; main shows x: %v", changed, hasID(main.ids("terminalProfiles"), "x"))
	main.state["activePage"] = "terminal"
	main.save()
	disk := sharedDiskIDs(t, root, "terminalProfiles")
	t.Logf("disk after main saved: %v", disk)
	if hasID(disk, "x") {
		t.Fatalf("an entry removed in this version came back: %v", disk)
	}
}

// creating the vault identity from a standalone app must
// not make it forget what another app saved a moment before.
func TestSharedStandaloneIdentityCreationKeepsAnotherAppsSave(t *testing.T) {
	root := t.TempDir()
	main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	main.add("terminalProfiles", sharedProfile("p1", "One", "192.0.2.1"))
	main.save()
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	// The main window saves another connection; the Terminal has not polled.
	main.add("terminalProfiles", sharedProfile("p2", "Two", "192.0.2.2"))
	main.save()
	if id := credentialConfigID(terminal.app.sharedSettingsPath()); id != "" {
		t.Fatalf("fixture: identity already present: %s", id)
	}
	if _, err := terminal.app.ensureCredentialConfigIdentity(); err != nil {
		t.Fatal(err)
	}
	if !hasID(sharedDiskIDs(t, root, "terminalProfiles"), "p2") {
		t.Fatal("fixture: p2 lost by the identity write itself")
	}
	// The Terminal's page changes a setting and saves (no poll in between).
	terminal.state["theme"] = "light"
	terminal.save()
	disk := sharedDiskIDs(t, root, "terminalProfiles")
	t.Logf("disk after the Terminal saved: %v", disk)
	if !hasID(disk, "p2") {
		t.Fatalf("a connection the main window saved was removed by the Terminal's save: %v", disk)
	}
}

// a settings step that fails at its write, in a run in
// which another step succeeds, must not be recorded as done.
func TestSharedMigrationFailedSettingsWriteIsNotRecordedAsHandled(t *testing.T) {
	root := migrationFixture(t)
	workspace := filepath.Join(root, terminalWorkspaceName)
	// The Terminal workspace also has a trusted host, so the known-hosts step
	// has something to do and marks the marker as changed.
	writeKnownHostFixture(t, workspace, knownHostFixtureEntry("edge.test:22", knownHostFixtureKey(t)))
	// The settings write cannot be done in this run: the copy of the saved
	// lists cannot be written (stands in for any failed write).
	blocker := filepath.Join(root, savedListsFilename)
	if err := os.Mkdir(blocker, 0700); err != nil {
		t.Fatal(err)
	}
	err := migrateSharedData(root)
	if err == nil {
		t.Fatal("expected the first run to report the failed settings write")
	}
	t.Logf("first run: %v", err)
	if got := sharedDiskIDs(t, root, "terminalProfiles"); len(got) != 3 {
		t.Fatalf("unexpected: settings were written: %v", got)
	}
	// The obstacle goes away; the next start must complete the migration.
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	if err := migrateSharedData(root); err != nil {
		t.Fatalf("second run: %v", err)
	}
	got := sharedDiskIDs(t, root, "terminalProfiles")
	t.Logf("after second run: %v", got)
	if !hasID(got, "terminal-only") {
		t.Fatalf("the Terminal's saved session was never brought over after a failed first run: %v", got)
	}
}

// the settings file goes missing while the main window
// and a standalone app run. The standalone app saves first (it writes the
// file again with the keys it knows); the main window saves afterwards. What
// only the main window holds (saved targets, TFTP profiles) must survive.
func TestSharedMissingSettingsFileStandaloneSavesFirstKeepsMainOnlyLists(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, appSettingsFilename)
	main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	main.add("terminalProfiles", sharedProfile("a", "A", "192.0.2.1"))
	main.state["profiles"] = []any{map[string]any{"id": "target-1", "name": "Core router", "target": "192.0.2.10", "tool": "ping"}}
	main.state["tftpProfiles"] = []any{map[string]any{"id": "tftp-1", "name": "Lab TFTP", "host": "192.0.2.20", "port": float64(69)}}
	main.state["forms"] = map[string]any{"ping": map[string]any{"count": float64(4)}}
	main.save()
	before, _ := os.ReadFile(path)
	var onDisk map[string]any
	_ = json.Unmarshal(before, &onDisk)
	if onDisk["profiles"] == nil || onDisk["tftpProfiles"] == nil {
		t.Fatalf("fixture: main-only lists not on disk: %s", before)
	}
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	// The Terminal app saves a new session: it writes the file again.
	terminal.add("terminalProfiles", sharedProfile("b", "B", "192.0.2.2"))
	terminal.save()
	// The main window polls and then saves something unrelated.
	main.poll()
	main.state["activePage"] = "overview"
	main.save()

	after, _ := os.ReadFile(path)
	var disk map[string]any
	_ = json.Unmarshal(after, &disk)
	targets, _ := disk["profiles"].([]any)
	tftp, _ := disk["tftpProfiles"].([]any)
	t.Logf("on disk after both saved: profiles=%v tftpProfiles=%v terminalProfiles=%v forms=%v", targets, tftp, sharedDiskIDs(t, root, "terminalProfiles"), disk["forms"])
	// What a restarted main window would load.
	late := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	t.Logf("a main window started later: profiles=%v tftpProfiles=%v", late.state["profiles"], late.state["tftpProfiles"])
	if len(targets) != 1 || len(tftp) != 1 {
		t.Fatalf("saved targets / TFTP profiles that only the main window holds were dropped: %v %v", targets, tftp)
	}
}

// The settings file cannot be read for a moment (another program holds it).
// That is not "the configuration was reset": the vault stays unlocked, no
// new identity is created, and a remembered unlock is not called expired.
func TestSharedVaultSettingsThatCannotBeReadNeverLockOrRekeyTheVault(t *testing.T) {
	root, main, terminal := sharedVaultApps(t)
	settings := filepath.Join(root, appSettingsFilename)
	identity := credentialConfigID(settings)
	if identity == "" {
		t.Fatal("fixture: no identity")
	}
	failures := 0
	fail := func(times int) {
		failures = times
		readCredentialFault = func(path string) error {
			if path == settings && failures != 0 {
				if failures > 0 {
					failures--
				}
				return errors.New("held by another program")
			}
			return nil
		}
	}
	t.Cleanup(func() { readCredentialFault = nil })

	// Every use of a credential checks the identity.
	fail(-1)
	if err := terminal.credentials.requireUnlocked(); !errors.Is(err, errCredentialConfigUnreadable) {
		t.Fatalf("a use of the vault while the settings could not be read: %v", err)
	}
	if status := terminal.credentials.status(); status.State != "unlocked" || strings.Contains(status.Warning, "reset") {
		t.Fatalf("the vault was locked because the settings could not be read: %+v", status)
	}
	fail(0)
	if err := terminal.credentials.requireUnlocked(); err != nil {
		t.Fatalf("the vault after the settings were readable again: %v", err)
	}

	// The first read fails, the next would succeed: no new identity is made.
	fail(1)
	if id, err := main.ensureCredentialConfigIdentity(); err == nil {
		t.Fatalf("an identity (%s) was returned although the settings could not be read", id)
	}
	fail(0)
	if now := credentialConfigID(settings); now != identity {
		t.Fatalf("the identity was replaced after a failed read: %s, was %s", now, identity)
	}
	if err := terminal.credentials.requireUnlocked(); err != nil {
		t.Fatalf("the other app's vault after that: %v", err)
	}

	// An app that starts in such a moment: its remembered unlock is used as
	// soon as the settings can be read, not called expired.
	if err := main.credentials.remember(identity, true); err != nil {
		t.Fatal(err)
	}
	fail(-1)
	started := reloadFixtureVault(t, main.credentials)
	if status := started.status(); status.State == "unlocked" || strings.Contains(status.Warning, "expired") {
		t.Fatalf("a vault loaded while the settings could not be read: %+v", status)
	}
	fail(0)
	if status := started.status(); status.State != "unlocked" || !status.Remembered {
		t.Fatalf("the remembered unlock was not used once the settings were readable: %+v", status)
	}
}

// A list that cannot be brought over (its value is not a list) must leave
// the workspace "not done", so the next start looks at it again. Today every
// shared list has a sanitizer that turns anything else into a list, so the
// test adds a list key without one, as a later key could be.
func TestSharedMigrationPartThatCannotBeBroughtOverIsNotRecordedAsDone(t *testing.T) {
	saved := sharedDataKeys
	sharedDataKeys = append(append([]sharedKey{}, saved...), sharedKey{Name: "futureList", Kind: sharedList, MaxSaved: 10})
	t.Cleanup(func() { sharedDataKeys = saved })
	root := t.TempDir()
	writeFixture(t, filepath.Join(root, appSettingsFilename), `{"theme":"dark"}`)
	workspace := filepath.Join(root, terminalWorkspaceName, appSettingsFilename)
	writeFixture(t, workspace, `{"futureList":"not a list","terminalProfiles":[{"id":"t1","name":"Edge","type":"ssh","host":"192.0.2.2","port":22}]}`)
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "t1") {
		t.Fatalf("the part that could be brought over: %v", got)
	}
	marker, err := readSharedMarker(root)
	if err != nil || marker.Sources["terminal/settings"] == nil {
		t.Fatalf("marker: %+v %v", marker, err)
	}
	if marker.Sources["terminal/settings"].Digest != "" {
		t.Fatal("a workspace with a list that was left out is recorded as done")
	}
	// Once the list can be read, the next start brings it over.
	writeFixture(t, workspace, `{"futureList":[{"id":"f1","name":"Later"}],"terminalProfiles":[{"id":"t1","name":"Edge","type":"ssh","host":"192.0.2.2","port":22}]}`)
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	if got := sharedDiskIDs(t, root, "futureList"); !sameIDs(got, "f1") {
		t.Fatalf("the list after it could be read: %v", got)
	}
}
