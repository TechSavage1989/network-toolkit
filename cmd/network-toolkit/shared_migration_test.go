package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"networktoolkit/internal/macros"
)

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

// snapshotFiles reads every file below dir, so a test can show that a step
// left files exactly as they were.
func snapshotFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || strings.HasSuffix(path, ".lock") {
			return err
		}
		raw, err := os.ReadFile(path)
		rel, _ := filepath.Rel(dir, path)
		out[rel] = string(raw)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func sameFiles(a, b map[string]string) string {
	names := map[string]bool{}
	for name := range a {
		names[name] = true
	}
	for name := range b {
		names[name] = true
	}
	differing := []string{}
	for name := range names {
		if a[name] != b[name] {
			differing = append(differing, name)
		}
	}
	sort.Strings(differing)
	return strings.Join(differing, ", ")
}

func diskRow(t *testing.T, root, key, id string) map[string]any {
	t.Helper()
	list, _ := sharedDisk(t, root)[key].([]any)
	for _, entry := range list {
		if row, _ := entry.(map[string]any); row["id"] == id {
			return row
		}
	}
	return nil
}

const migrationRootSettings = `{"theme":"dark","activePage":"overview","terminalFontSize":13,"forms":{"ping":{"count":4}},
 "terminalAppearance":{"cursor":"block","fontFamily":"Consolas"},
 "terminalSyntax":{"custom":[{"id":"custom-root","name":"Root rules","rules":{}}]},
 "editorPreferences":{"wordWrap":false,"fontSize":13},
 "terminalNameRevision":4,
 "terminalProfiles":[
  {"id":"same","name":"Core","nameRevision":2,"type":"ssh","host":"192.0.2.1","port":22,"username":"admin","credentialId":"cred-root"},
  {"id":"changed","name":"Edge","nameRevision":4,"type":"ssh","host":"192.0.2.2","port":22,"username":"admin"},
  {"id":"root-only","name":"Lab","type":"telnet","host":"192.0.2.3","port":23}],
 "terminalRecent":[{"id":"r-same","name":"Core","type":"ssh","host":"192.0.2.1","port":22,"username":"admin","lastUsed":100}],
 "terminalPromptNames":[{"type":"ssh","host":"192.0.2.1","port":22,"username":"admin","name":"CORE-SW"}],
 "sftpVerifiedHostKeys":{"192.0.2.1:22":"SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","192.0.2.9:22":"SHA256:BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"}}`

const migrationTerminalSettings = `{"theme":"light","activePage":"terminal","terminalFontSize":18,"terminalSidebarWidth":400,"terminalHighlight":false,
 "terminalAppearance":{"cursor":"bar","fontFamily":"Cascadia Mono"},
 "terminalSyntax":{"custom":[{"id":"custom-root","name":"Changed in Terminal","rules":{}},{"id":"custom-terminal","name":"Terminal rules","rules":{}}]},
 "terminalBehavior":{"scrollback":2000,"bell":"visual"},
 "terminalNameRevision":9,
 "terminalProfiles":[
  {"id":"same","name":"Core","nameRevision":2,"type":"ssh","host":"192.0.2.1","port":22,"username":"admin","credentialId":"cred-root"},
  {"id":"changed","name":"Edge","nameRevision":9,"type":"ssh","host":"192.0.2.22","port":2222,"username":"operator","credentialId":"cred-terminal"},
  {"id":"terminal-only","name":"Console","type":"serial","serialPort":"COM3","baudRate":9600,"credentialId":""}],
 "terminalRecent":[{"id":"r-same","name":"Core","type":"ssh","host":"192.0.2.1","port":22,"username":"admin","lastUsed":999},{"id":"r-terminal","name":"Console","type":"serial","serialPort":"COM3","lastUsed":200}],
 "terminalPromptNames":[{"type":"ssh","host":"192.0.2.1","port":22,"username":"admin","name":"OTHER-NAME"},{"type":"serial","serialPort":"COM3","name":"CONSOLE-SW"}],
 "ftpProfiles":[{"id":"ftp-terminal","name":"Files","host":"files.test","username":"operator","port":21,"security":"explicit","credentialId":"cred-ftp"}],
 "sftpVerifiedHostKeys":{"192.0.2.1:22":"SHA256:CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC","192.0.2.50:22":"SHA256:DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD"}}`

func migrationFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, filepath.Join(root, appSettingsFilename), migrationRootSettings)
	writeFixture(t, filepath.Join(root, terminalWorkspaceName, appSettingsFilename), migrationTerminalSettings)
	return root
}

func TestSharedMigrationFreshDataFolderWritesNothing(t *testing.T) {
	root := t.TempDir()
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	if files := snapshotFiles(t, root); len(files) != 0 {
		t.Fatalf("a new data folder was written to: %v", files)
	}
	// Only the main window has been used: still nothing to do.
	writeFixture(t, filepath.Join(root, appSettingsFilename), migrationRootSettings)
	before := snapshotFiles(t, root)
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	if diff := sameFiles(before, snapshotFiles(t, root)); diff != "" {
		t.Fatalf("a data folder without standalone workspaces was changed: %s", diff)
	}
	// A Terminal workspace created by this version holds only window state.
	dest := terminalWorkspace(root)
	if err := os.MkdirAll(dest, 0700); err != nil {
		t.Fatal(err)
	}
	if err := seedTerminalWorkspace(root, dest); err != nil {
		t.Fatal(err)
	}
	before = snapshotFiles(t, root)
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	if diff := sameFiles(before, snapshotFiles(t, root)); diff != "" {
		t.Fatalf("a workspace without data of its own caused changes: %s", diff)
	}
}

func TestSharedMigrationBringsTerminalWorkspaceIntoTheDataFolder(t *testing.T) {
	root := migrationFixture(t)
	workspaceBefore := snapshotFiles(t, filepath.Join(root, terminalWorkspaceName))
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	// The workspace files are only read.
	if diff := sameFiles(workspaceBefore, snapshotFiles(t, filepath.Join(root, terminalWorkspaceName))); diff != "" {
		t.Fatalf("the migration changed workspace files: %s", diff)
	}
	// An untouched copy of the data folder's settings from before.
	if backup, err := os.ReadFile(filepath.Join(root, sharedBackupSettings)); err != nil || string(backup) != migrationRootSettings {
		t.Fatalf("no untouched copy of the main settings was kept: %v", err)
	}
	disk := sharedDisk(t, root)
	// Connections: nothing of the data folder is overwritten; what only the
	// Terminal had is added; one changed on both sides is kept twice.
	copyID := ""
	for _, id := range sharedDiskIDs(t, root, "terminalProfiles") {
		if strings.HasPrefix(id, "changed-terminal-") {
			copyID = id
		}
	}
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "same", "changed", "root-only", "terminal-only", copyID) || copyID == "" {
		t.Fatalf("saved connections after the migration: %v", got)
	}
	if row := diskRow(t, root, "terminalProfiles", "changed"); row["host"] != "192.0.2.2" || row["name"] != "Edge" {
		t.Fatalf("the data folder's connection was overwritten: %v", row)
	}
	if row := diskRow(t, root, "terminalProfiles", copyID); row["host"] != "192.0.2.22" || row["port"] != float64(2222) || row["name"] != "Edge (Terminal)" || row["credentialId"] != "cred-terminal" {
		t.Fatalf("the Terminal's version of a changed connection: %v", row)
	}
	if row := diskRow(t, root, "terminalProfiles", "terminal-only"); row["serialPort"] != "COM3" {
		t.Fatalf("the Terminal's own connection: %v", row)
	}
	if got := sharedDiskIDs(t, root, "ftpProfiles"); !sameIDs(got, "ftp-terminal") {
		t.Fatalf("FTP connections: %v", got)
	}
	// Recent connections and learned names: additions only, the data folder's
	// entry stays where both have one.
	if got := sharedDiskIDs(t, root, "terminalRecent"); !sameIDs(got, "r-same", "r-terminal") || diskRow(t, root, "terminalRecent", "r-same")["lastUsed"] != float64(100) {
		t.Fatalf("recent connections: %v", disk["terminalRecent"])
	}
	names := pageJSON(disk["terminalPromptNames"])
	if !strings.Contains(names, "CORE-SW") || strings.Contains(names, "OTHER-NAME") || !strings.Contains(names, "CONSOLE-SW") {
		t.Fatalf("learned prompt names: %s", names)
	}
	if disk["terminalNameRevision"] != float64(9) {
		t.Fatalf("the display-name counter fell behind a migrated record: %v", disk["terminalNameRevision"])
	}
	// Settings: the data folder's value stays where the two differ; what only
	// the Terminal had set is taken over.
	appearance, _ := disk["terminalAppearance"].(map[string]any)
	behavior, _ := disk["terminalBehavior"].(map[string]any)
	if disk["theme"] != "dark" || disk["terminalFontSize"] != float64(13) || appearance["cursor"] != "block" || appearance["fontFamily"] != "Consolas" {
		t.Fatalf("a setting of the data folder was replaced: theme=%v font=%v appearance=%v", disk["theme"], disk["terminalFontSize"], appearance)
	}
	if disk["terminalHighlight"] != false || behavior["scrollback"] != float64(2000) || behavior["bell"] != "visual" {
		t.Fatalf("settings only the Terminal had were not taken over: highlight=%v behavior=%v", disk["terminalHighlight"], behavior)
	}
	// Highlighting rule sets written in the Terminal are kept next to the
	// data folder's own.
	syntax := pageJSON(disk["terminalSyntax"])
	if !strings.Contains(syntax, "Root rules") || strings.Contains(syntax, "Changed in Terminal") || !strings.Contains(syntax, "Terminal rules") {
		t.Fatalf("highlighting rule sets: %s", syntax)
	}
	// Verified SFTP fingerprints: union; a different fingerprint for the same
	// host is never taken over silently.
	keys, _ := disk["sftpVerifiedHostKeys"].(map[string]any)
	if keys["192.0.2.1:22"] != "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" || keys["192.0.2.50:22"] == nil || keys["192.0.2.9:22"] == nil {
		t.Fatalf("verified fingerprints: %v", keys)
	}
	notices := sharedNotices(root)
	if len(notices) != 1 || notices[0].Kind != "host-key-conflict" || notices[0].Source != "terminal" || len(notices[0].Items) != 1 || notices[0].Items[0] != "192.0.2.1:22" {
		t.Fatalf("the differing fingerprint was not reported: %+v", notices)
	}
	// What only the main window uses is untouched, byte for byte.
	var file map[string]json.RawMessage
	raw, _ := os.ReadFile(filepath.Join(root, appSettingsFilename))
	if json.Unmarshal(raw, &file) != nil || !strings.Contains(string(file["forms"]), `"count": 4`) || string(file["activePage"]) != `"overview"` {
		t.Fatalf("the main window's own settings changed: %s", raw)
	}
	if _, moved := file["terminalSidebarWidth"]; moved {
		t.Fatal("window state of the Terminal was written to the main settings")
	}
	// The marker remembers which saved connections name a credential of the
	// Terminal's previous vault.
	marker, err := readSharedMarker(root)
	if err != nil {
		t.Fatal(err)
	}
	references := map[string]string{}
	for _, reference := range marker.Credentials {
		references[reference.Key+"/"+reference.ID] = reference.Source + ":" + reference.CredentialID
	}
	if len(references) != 2 || references["terminalProfiles/"+copyID] != "terminal:cred-terminal" || references["ftpProfiles/ftp-terminal"] != "terminal:cred-ftp" {
		t.Fatalf("credential references of migrated connections: %v", references)
	}

	// Every app now starts with the same data.
	main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	for _, page := range []*sharedPage{main, terminal} {
		if !sameIDs(page.ids("terminalProfiles"), "same", "changed", "root-only", "terminal-only", copyID) || page.state["theme"] != "dark" {
			t.Fatalf("%s after the migration: %v theme=%v", page.app.windowMode, page.ids("terminalProfiles"), page.state["theme"])
		}
	}
	if terminal.state["terminalSidebarWidth"] != float64(400) || terminal.state["activePage"] != "terminal" {
		t.Fatalf("the Terminal lost its own window state: %v", terminal.state)
	}
}

func TestSharedMigrationRepeatsAsNoOpAndNeverBringsBackWhatTheUserRemoved(t *testing.T) {
	root := migrationFixture(t)
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	after := snapshotFiles(t, root)
	for i := 0; i < 3; i++ {
		if err := migrateSharedData(root); err != nil {
			t.Fatal(err)
		}
	}
	if diff := sameFiles(after, snapshotFiles(t, root)); diff != "" {
		t.Fatalf("a repeated migration changed: %s", diff)
	}
	// The standalone Terminal runs and saves: its own file keeps the copy
	// from before sharing as it was, so nothing is migrated again.
	terminalApp := sharedTestApp(t, root, terminalWorkspaceName, "terminal")
	terminal := openSharedPage(t, terminalApp)
	terminal.remove("terminalProfiles", "terminal-only")
	terminal.state["terminalSidebarWidth"] = float64(444)
	terminal.save()
	var own map[string]any
	raw, _ := os.ReadFile(terminalApp.dataPath)
	if json.Unmarshal(raw, &own) != nil || own["terminalSidebarWidth"] != float64(444) || !strings.Contains(pageJSON(own["terminalProfiles"]), "terminal-only") || own["theme"] != "light" {
		t.Fatalf("the workspace file lost its window state or its copy from before sharing: %s", raw)
	}
	settings := snapshotFiles(t, root)[appSettingsFilename]
	marker := snapshotFiles(t, root)[sharedMarkerFilename]
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	now := snapshotFiles(t, root)
	if now[appSettingsFilename] != settings || now[sharedMarkerFilename] != marker {
		t.Fatal("a connection the user removed was migrated again, or the marker changed")
	}
	if got := sharedDiskIDs(t, root, "terminalProfiles"); len(got) != 4 {
		t.Fatalf("saved connections after a removal and a repeated migration: %v", got)
	}

	// An older version of the Terminal is used afterwards and saves a new
	// connection into its own file. Only that connection is added; the one
	// the user removed stays removed.
	var legacy map[string]any
	if err := json.Unmarshal([]byte(migrationTerminalSettings), &legacy); err != nil {
		t.Fatal(err)
	}
	legacy["terminalProfiles"] = append(legacy["terminalProfiles"].([]any), map[string]any{"id": "added-by-older-version", "name": "Later", "type": "ssh", "host": "192.0.2.90", "port": float64(22)})
	encoded, _ := json.Marshal(legacy)
	writeFixture(t, terminalApp.dataPath, string(encoded))
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	got := sharedDiskIDs(t, root, "terminalProfiles")
	hasLater, hasRemoved := false, false
	for _, id := range got {
		hasLater = hasLater || id == "added-by-older-version"
		hasRemoved = hasRemoved || id == "terminal-only"
	}
	if len(got) != 5 || !hasLater || hasRemoved {
		t.Fatalf("saved connections after an older version added one: %v", got)
	}
}

func TestSharedMigrationRecoversFromInterruptedRuns(t *testing.T) {
	root := migrationFixture(t)
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	complete := snapshotFiles(t, root)
	// The process stopped after the settings were written but before the
	// marker was: the next start does the work again and adds nothing twice.
	if err := os.Remove(filepath.Join(root, sharedMarkerFilename)); err != nil {
		t.Fatal(err)
	}
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	again := snapshotFiles(t, root)
	if again[appSettingsFilename] != complete[appSettingsFilename] || again[sharedBackupSettings] != migrationRootSettings {
		t.Fatal("repeating an interrupted migration changed the settings or replaced the untouched copy")
	}
	if got := sharedDiskIDs(t, root, "terminalProfiles"); len(got) != 5 {
		t.Fatalf("an interrupted migration duplicated connections: %v", got)
	}

	// One part cannot be done (the data folder's macro library is damaged):
	// the other parts are done, the failure is reported, and a later start
	// completes the rest.
	second := migrationFixture(t)
	writeFixture(t, filepath.Join(second, terminalMacroLibraryFilename), `{"version":1,"macros":[`)
	library, _ := json.Marshal(terminalMacroLibrary{Version: 1, Macros: []macros.Definition{macroHTTPDefinition("from-terminal")}})
	writeFixture(t, filepath.Join(second, terminalWorkspaceName, terminalMacroLibraryFilename), string(library))
	if err := migrateSharedData(second); err == nil {
		t.Fatal("a damaged macro library was not reported")
	}
	if got := sharedDiskIDs(t, second, "terminalProfiles"); len(got) != 5 {
		t.Fatalf("a failure in one part held back the saved connections: %v", got)
	}
	if raw, _ := os.ReadFile(filepath.Join(second, terminalMacroLibraryFilename)); string(raw) != `{"version":1,"macros":[` {
		t.Fatal("the damaged macro library was replaced")
	}
	writeFixture(t, filepath.Join(second, terminalMacroLibraryFilename), `{"version":1,"macros":[],"bindings":[]}`)
	if err := migrateSharedData(second); err != nil {
		t.Fatal(err)
	}
	storage, err := openStorageRoot(second, false)
	if err != nil {
		t.Fatal(err)
	}
	merged, err := readTerminalMacroLibrary(storage)
	if err != nil || len(merged.Macros) != 1 || merged.Macros[0].ID != "from-terminal" {
		t.Fatalf("the macros were not migrated once the library was repaired: %+v %v", merged.Macros, err)
	}
	if got := sharedDiskIDs(t, second, "terminalProfiles"); len(got) != 5 {
		t.Fatalf("completing the migration duplicated connections: %v", got)
	}
}

func TestSharedMigrationKnownHostsAndMacros(t *testing.T) {
	root := t.TempDir()
	core, edge, changed, replaced := knownHostFixtureKey(t), knownHostFixtureKey(t), knownHostFixtureKey(t), knownHostFixtureKey(t)
	writeKnownHostFixture(t, root, knownHostFixtureEntry("core.test:22", core), knownHostFixtureEntry("router.test:22", changed))
	workspace := filepath.Join(root, terminalWorkspaceName)
	writeKnownHostFixture(t, workspace, knownHostFixtureEntry("core.test:22", core), knownHostFixtureEntry("edge.test:22", edge), knownHostFixtureEntry("router.test:22", replaced))
	rootTrust, _ := os.ReadFile(filepath.Join(root, knownHostsFilename))

	shared := macroHTTPDefinition("shared")
	edited := macroHTTPDefinition("edited")
	rootOnly := macroHTTPDefinition("root-only")
	rootOnly.Shortcut = "Ctrl+Alt+1"
	rootLibrary, _ := json.Marshal(terminalMacroLibrary{Version: 1, Macros: []macros.Definition{shared, edited, rootOnly}})
	writeFixture(t, filepath.Join(root, terminalMacroLibraryFilename), string(rootLibrary))
	editedInTerminal := macroHTTPDefinition("edited")
	editedInTerminal.Name = "Read device state v2"
	editedInTerminal.Steps[0].Text = "show running-config"
	own := macroHTTPDefinition("terminal-macro")
	own.Shortcut = "Ctrl+Alt+1" // taken by a macro of the data folder
	workspaceLibrary, _ := json.Marshal(terminalMacroLibrary{Version: 1, Macros: []macros.Definition{shared, editedInTerminal, own}, Bindings: []terminalMacroBinding{{ProfileID: "changed", MacroID: "edited", OnConnect: true}, {ProfileID: "terminal-only", MacroID: "terminal-macro", OnConnect: true}}})
	writeFixture(t, filepath.Join(workspace, terminalMacroLibraryFilename), string(workspaceLibrary))
	writeFixture(t, filepath.Join(root, appSettingsFilename), migrationRootSettings)
	writeFixture(t, filepath.Join(workspace, appSettingsFilename), migrationTerminalSettings)
	workspaceBefore := snapshotFiles(t, workspace)

	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	if diff := sameFiles(workspaceBefore, snapshotFiles(t, workspace)); diff != "" {
		t.Fatalf("the migration changed workspace files: %s", diff)
	}
	// SSH host trust: a union. The key the data folder trusts for a host
	// stays; the Terminal's different key is not trusted, and the user is told.
	app := sharedTestApp(t, root, terminalWorkspaceName, "terminal")
	if got := knownHostsOf(t, app); !sameIDs(got, "core.test:22", "edge.test:22", "router.test:22") {
		t.Fatalf("trusted hosts after the migration: %v", got)
	}
	trust, _ := os.ReadFile(filepath.Join(root, knownHostsFilename))
	if !strings.Contains(string(trust), strings.TrimPrefix(sshFingerprint(changed), "SHA256:")) || strings.Contains(string(trust), strings.TrimPrefix(sshFingerprint(replaced), "SHA256:")) {
		t.Fatal("a different host key from the Terminal workspace replaced the trusted one")
	}
	if backup, _ := os.ReadFile(filepath.Join(root, sharedBackupKnownHosts)); string(backup) != string(rootTrust) {
		t.Fatal("no untouched copy of the data folder's trust file was kept")
	}
	reported := map[string]bool{}
	for _, notice := range sharedNotices(root) {
		if notice.Kind == "host-key-conflict" {
			for _, host := range notice.Items {
				reported[host] = true
			}
		}
	}
	if !reported["router.test:22"] || reported["core.test:22"] || reported["edge.test:22"] {
		t.Fatalf("the host with a different key was not reported (or others were): %v", reported)
	}

	// Macros: additions; one changed on both sides is kept twice; a shortcut
	// already used in the data folder is not taken; startup macros follow
	// their session and macro.
	storage, err := openStorageRoot(root, false)
	if err != nil {
		t.Fatal(err)
	}
	library, err := readTerminalMacroLibrary(storage)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]macros.Definition{}
	copyID := ""
	for _, definition := range library.Macros {
		byID[definition.ID] = definition
		if strings.HasPrefix(definition.ID, "edited-terminal-") {
			copyID = definition.ID
		}
	}
	if len(library.Macros) != 5 || copyID == "" || byID["edited"].Steps[0].Text != "show version" || byID[copyID].Steps[0].Text != "show running-config" || byID[copyID].Name != "Read device state v2 (Terminal)" {
		t.Fatalf("macros after the migration: %+v", library.Macros)
	}
	if byID["root-only"].Shortcut != "Ctrl+Alt+1" || byID["terminal-macro"].Shortcut != "" {
		t.Fatalf("shortcuts after the migration: %q %q", byID["root-only"].Shortcut, byID["terminal-macro"].Shortcut)
	}
	profileCopy := ""
	for _, id := range sharedDiskIDs(t, root, "terminalProfiles") {
		if strings.HasPrefix(id, "changed-terminal-") {
			profileCopy = id
		}
	}
	bindings := map[string]bool{}
	for _, binding := range library.Bindings {
		bindings[binding.ProfileID+" -> "+binding.MacroID] = true
	}
	if len(bindings) != 2 || !bindings[profileCopy+" -> "+copyID] || !bindings["terminal-only -> terminal-macro"] {
		t.Fatalf("startup macros after the migration: %v (connection copy %s, macro copy %s)", bindings, profileCopy, copyID)
	}
	if backup, _ := os.ReadFile(filepath.Join(root, sharedBackupMacros)); string(backup) != string(rootLibrary) {
		t.Fatal("no untouched copy of the data folder's macro library was kept")
	}
	// Done once.
	after := snapshotFiles(t, root)
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	if diff := sameFiles(after, snapshotFiles(t, root)); diff != "" {
		t.Fatalf("a repeated migration changed: %s", diff)
	}
	// A notice is shown until the user acknowledges it.
	ids := []string{}
	for _, notice := range sharedNotices(root) {
		ids = append(ids, notice.ID)
	}
	if err := acknowledgeSharedNotices(root, ids); err != nil || len(sharedNotices(root)) != 0 {
		t.Fatalf("notices after acknowledging: %v %v", sharedNotices(root), err)
	}
	if err := migrateSharedData(root); err != nil || len(sharedNotices(root)) != 0 {
		t.Fatal("an acknowledged notice came back")
	}
}

func TestSharedMigrationTextEditorPreferences(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, filepath.Join(root, appSettingsFilename), `{"theme":"dark","editorPreferences":{"wordWrap":false,"fontSize":13}}`)
	editorSettings := `{"activePage":"editor","theme":"light","accentColor":"#286dbb","editorPreferences":{"wordWrap":true,"fontSize":13,"palette":"custom","colors":{"keyword":"#112233"}},"editorFilesPreferences":{"width":340,"dock":"right"}}`
	writeFixture(t, filepath.Join(root, standaloneEditorDirectory, appSettingsFilename), editorSettings)
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	disk := sharedDisk(t, root)
	prefs, _ := disk["editorPreferences"].(map[string]any)
	files, _ := disk["editorFilesPreferences"].(map[string]any)
	// The data folder's values stay where the two differ; what only the
	// editor had set is taken over.
	if disk["theme"] != "dark" || prefs["wordWrap"] != false || prefs["fontSize"] != float64(13) {
		t.Fatalf("settings of the data folder were replaced: %v %v", disk["theme"], prefs)
	}
	if disk["accentColor"] != "#286dbb" || prefs["palette"] != "custom" || prefs["colors"] == nil || files["width"] != float64(340) || files["dock"] != "right" {
		t.Fatalf("settings only the Text Editor had were not taken over: accent=%v editor=%v files=%v", disk["accentColor"], prefs, files)
	}
	if raw, _ := os.ReadFile(filepath.Join(root, standaloneEditorDirectory, appSettingsFilename)); string(raw) != editorSettings {
		t.Fatal("the Text Editor's own file was changed")
	}
	editor := openSharedPage(t, sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode))
	if editor.state["theme"] != "dark" || editor.member("editorPreferences", "palette") != "custom" || editor.state["activePage"] != "editor" {
		t.Fatalf("the standalone Text Editor after the migration: %v", editor.state)
	}
}

// Saved sessions whose id another entry of the same list also has cannot be
// matched by id. They are still brought over, each with an id of its own, and
// the workspace is recorded as done only because nothing was left out.
func TestSharedMigrationBringsOverEntriesThatShareAnID(t *testing.T) {
	root := t.TempDir()
	profile := func(id, name, host string) map[string]any { return sharedProfile(id, name, host) }
	rootRaw, _ := json.Marshal(map[string]any{"terminalProfiles": []any{profile("dup", "Root one", "192.0.2.1"), profile("plain", "Plain", "192.0.2.2")}})
	workspaceRaw, _ := json.Marshal(map[string]any{"terminalProfiles": []any{profile("dup", "Workspace one", "192.0.2.11"), profile("dup", "Workspace two", "192.0.2.12"), profile("plain", "Plain", "192.0.2.2"), profile("fine", "Fine", "192.0.2.13")}})
	writeFixture(t, filepath.Join(root, appSettingsFilename), string(rootRaw))
	writeFixture(t, filepath.Join(root, terminalWorkspaceName, appSettingsFilename), string(workspaceRaw))
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	list, _ := sharedDisk(t, root)["terminalProfiles"].([]any)
	names, ids := []string{}, map[string]bool{}
	for _, entry := range list {
		row := entry.(map[string]any)
		names = append(names, row["name"].(string))
		id := row["id"].(string)
		if id == "" || ids[id] {
			t.Fatalf("an entry was brought over without an id of its own: %v", row)
		}
		ids[id] = true
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "Fine,Plain,Root one,Workspace one,Workspace two" {
		t.Fatalf("saved sessions after the move: %v", names)
	}
	marker, err := readSharedMarker(root)
	if err != nil || marker.Sources["terminal/settings"] == nil || marker.Sources["terminal/settings"].Digest == "" {
		t.Fatalf("the workspace is not recorded as done although everything was brought over: %+v %v", marker.Sources, err)
	}
	before := snapshotFiles(t, root)
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	if diff := sameFiles(before, snapshotFiles(t, root)); diff != "" {
		t.Fatalf("a second start changed: %s", diff)
	}
}
