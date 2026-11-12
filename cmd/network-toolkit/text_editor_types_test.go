package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

const testInstalled = `C:\Users\operator\AppData\Local\Programs\Network Toolkit\Network_Toolkit.exe`

// releasedTypes are the five types Setup up to 1.21.43 registered.
var releasedTypes = []string{".txt", ".log", ".ini", ".cfg", ".conf"}

// installedRegistry is what Setup of setupVersion left for the installed
// program, when only the five released types were registered (Setup up to
// 1.21.43 registered no more; a later Setup would have registered them all):
// the ProgID, Default apps and Open with for those types, and, from 1.21.44,
// its record. Another program owns .cfg and .yaml and is in the Open With
// list of .txt and .md (fixtureRegistry plus .yaml and .md).
func installedRegistry(setupVersion string) *fakeRegistry {
	reg := fixtureRegistry()
	reg.SetString(`Software\Classes\.yaml`, "", "SyntheticYaml.Document")
	reg.SetString(`Software\Classes\.md\OpenWithProgids`, "Synthetic.Markdown", "")
	reg.SetString(editorRegUninstall, "InstallLocation", editorRegDir(testInstalled))
	if setupVersion != "" {
		reg.SetString(editorRegUninstall, "DisplayVersion", setupVersion)
	}
	command := editorRegCommand(testInstalled)
	reg.SetString(editorRegProgKey, "", editorRegProgName)
	reg.SetString(editorRegProgKey+`\DefaultIcon`, "", editorRegIcon(testInstalled, editorRegDocumentIcon))
	reg.SetString(editorRegProgKey+`\shell\open\command`, "", command)
	reg.SetString(editorRegCapabilities, "ApplicationName", editorRegAppName)
	reg.SetString(editorRegRegistered, editorRegAppName, editorRegCapabilities)
	for _, ext := range releasedTypes {
		reg.SetString(editorRegCapabilities+`\FileAssociations`, ext, editorRegProgID)
		reg.SetString(`Software\Classes\`+ext+`\OpenWithProgids`, editorRegProgID, "")
	}
	if editorRegUninstallerRecords(setupVersion) {
		reg.SetDWORD(editorRegSetupRecord, "Records", 1)
	}
	return reg
}

// simulateUninstall is Setup's RemoveExtension for each of types and the
// removal of its Default apps entry and record values: what the installed
// uninstaller of 1.21.44 or later does for them.
func simulateUninstall(t *testing.T, reg *fakeRegistry, types []string) {
	t.Helper()
	records := map[string]bool{}
	names, _ := reg.ValueNames(editorRegSetupKeys)
	for _, name := range names {
		if v, _, _ := reg.DWORD(editorRegSetupKeys, name); v == 1 {
			records[name] = true
		}
	}
	for _, ext := range types {
		key := `Software\Classes\` + ext
		reg.DeleteValue(key+`\OpenWithProgids`, editorRegProgID)
		if records[ext] {
			editorRegDeleteEmpty(reg, key+`\OpenWithProgids`)
			editorRegDeleteEmpty(reg, key)
		} else if records[ext+`\OpenWithProgids`] {
			editorRegDeleteEmpty(reg, key+`\OpenWithProgids`)
		}
		reg.DeleteValue(editorRegCapabilities+`\FileAssociations`, ext)
		reg.DeleteValue(editorRegSetupKeys, ext)
		reg.DeleteValue(editorRegSetupKeys, ext+`\OpenWithProgids`)
	}
	editorRegDeleteEmpty(reg, editorRegSetupKeys)
}

func TestEditorRegistrationTypesAreAddedWhenTheUninstallerRemovesThem(t *testing.T) {
	reg := installedRegistry("1.21.44")
	before := reg.dump()
	platform, notified := testEditorPlatform(reg)
	status, err := editorRegTypesState(reg, testInstalled)
	if err != nil || status == nil || status.Missing != len(textEditorExtensions)-len(releasedTypes) || !status.SetupNeeded || !status.UninstallerRecords || status.SetupVersion != "1.21.44" {
		t.Fatalf("status before: %+v %v", status, err)
	}
	missing, _ := editorRegMissingTypes(reg)
	added, err := refreshEditorTypes(platform, strings.ToLower(testInstalled))
	if err != nil || added != len(missing) || *notified != 1 {
		t.Fatalf("added %d of %d: %v (notified %d)", added, len(missing), err, *notified)
	}
	if after, _ := editorRegMissingTypes(reg); len(after) != 0 {
		t.Fatalf("still missing: %v", after)
	}
	if status, _ := editorRegTypesState(reg, testInstalled); status.Missing != 0 || status.SetupNeeded {
		t.Fatalf("status after: %+v", status)
	}
	// Exactly what Setup writes: the ProgID in the Open With list and the type
	// in Default apps, the record of a key that was created. Nothing else.
	records := map[string]bool{}
	names, _ := reg.ValueNames(editorRegSetupKeys)
	for _, name := range names {
		records[name] = true
	}
	for _, ext := range missing {
		key := `Software\Classes\` + ext
		if v, found := reg.value(key+`\OpenWithProgids`, editorRegProgID); !found || v.kind != "sz" || v.str != "" {
			t.Errorf("%s is not in Open with", ext)
		}
		switch ext {
		case ".yaml":
			if reg.str(t, key, "") != "SyntheticYaml.Document" || records[ext] || !records[ext+`\OpenWithProgids`] {
				t.Errorf(".yaml belongs to another program: only its new Open With list is recorded (%v)", records[ext+`\OpenWithProgids`])
			}
		case ".md":
			if records[ext] || records[ext+`\OpenWithProgids`] {
				t.Error(".md had an Open With list: nothing is recorded")
			}
		default:
			if !records[ext] || records[ext+`\OpenWithProgids`] {
				t.Errorf("%s was created and must be recorded", ext)
			}
		}
	}
	for _, ext := range releasedTypes {
		if records[ext] || records[ext+`\OpenWithProgids`] {
			t.Errorf("a type that was registered already was recorded: %s", ext)
		}
	}
	for k := range reg.keys {
		if strings.Contains(k, "fileexts") || strings.Contains(k, "userchoice") || strings.HasPrefix(k, strings.ToLower(editorRegPortable)) {
			t.Errorf("wrote %s", k)
		}
	}
	// Nothing missing: nothing is written and the shell is not told.
	again := reg.dump()
	reg.failWrite = editorRegCapabilities + `\FileAssociations`
	if added, err := refreshEditorTypes(platform, testInstalled); err != nil || added != 0 || *notified != 1 || reg.dump() != again {
		t.Fatalf("a second run wrote: %d %v", added, err)
	}
	reg.failWrite = ""
	// The installed uninstaller takes everything added back.
	simulateUninstall(t, reg, missing)
	if after := reg.dump(); after != before {
		t.Fatalf("the uninstaller would leave keys behind:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestEditorRegistrationTypesAreNotWrittenWhereSetupMustRun(t *testing.T) {
	for _, tc := range []struct {
		name, version, exe string
		change             func(*fakeRegistry)
		notice             bool  // the page shows the Run Setup notice
		err                error // refreshEditorTypes reports it
	}{
		{name: "an uninstaller of 1.21.43", version: "1.21.43", exe: testInstalled, notice: true},
		{name: "an uninstaller of 1.20.99", version: "1.20.99", exe: testInstalled, notice: true},
		{name: "no DisplayVersion", version: "", exe: testInstalled, notice: true},
		{name: "a version that is not x.y.z", version: "1.21.50-beta", exe: testInstalled, notice: true},
		{name: "a version with spaces inside", version: "1. 21.50", exe: testInstalled, notice: true},
		{name: "a portable copy", version: "1.21.50", exe: testCopyA},
		{name: "the Text Editor program", version: "1.21.50", exe: editorRegEditorProgram(testInstalled)},
		{name: "a ProgID of another program", version: "1.21.50", exe: testInstalled, notice: true, err: errEditorTypesNotSetups, change: func(reg *fakeRegistry) {
			reg.SetString(editorRegProgKey+`\shell\open\command`, "", editorRegCommand(testCopyB))
		}},
		{name: "no ProgID", version: "1.21.50", exe: testInstalled, notice: true, err: errEditorTypesNotSetups, change: func(reg *fakeRegistry) {
			reg.DeleteValue(editorRegProgKey+`\shell\open\command`, "")
		}},
		{name: "no record of Setup", version: "1.21.50", exe: testInstalled, notice: true, err: errEditorTypesNotSetups, change: func(reg *fakeRegistry) {
			reg.DeleteValue(editorRegSetupRecord, "Records")
			editorRegDeleteEmpty(reg, editorRegSetupRecord)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := installedRegistry(tc.version)
			if tc.version == "1.21.50" && !reg.has(editorRegSetupRecord) {
				t.Fatal("fixture: Setup of 1.21.50 keeps a record")
			}
			if tc.change != nil {
				tc.change(reg)
			}
			before := reg.dump()
			platform, notified := testEditorPlatform(reg)
			added, err := refreshEditorTypes(platform, tc.exe)
			if added != 0 || !errors.Is(err, tc.err) || *notified != 0 || reg.dump() != before {
				t.Fatalf("added %d, %v (want %v), notified %d", added, err, tc.err, *notified)
			}
			status, err := editorRegTypesState(reg, tc.exe)
			if err != nil || (status != nil && status.SetupNeeded) != tc.notice {
				t.Fatalf("status %+v %v, want a notice: %v", status, err, tc.notice)
			}
			if status != nil && status.UninstallerRecords != editorRegUninstallerRecords(tc.version) {
				t.Fatalf("uninstaller records: %+v", status)
			}
		})
	}
	t.Run("a read that fails writes nothing and shows no notice", func(t *testing.T) {
		reg := installedRegistry("1.21.50")
		reg.readFault = `Software\Classes\.json\OpenWithProgids`
		before := reg.dump()
		platform, notified := testEditorPlatform(reg)
		if added, err := refreshEditorTypes(platform, testInstalled); err == nil || added != 0 || *notified != 0 || reg.dump() != before {
			t.Fatalf("wrote after a failed read: %d %v", added, err)
		}
		if status, err := editorRegTypesState(reg, testInstalled); err == nil || status != nil {
			t.Fatalf("a failed read must not read as missing types: %+v %v", status, err)
		}
	})
	t.Run("a write that fails leaves a record for every key it created", func(t *testing.T) {
		reg := installedRegistry("1.21.50")
		reg.failWrite = `Software\Classes\.json\OpenWithProgids`
		platform, notified := testEditorPlatform(reg)
		if _, err := refreshEditorTypes(platform, testInstalled); err == nil || *notified != 1 {
			t.Fatalf("a failed write: %v (notified %d)", err, *notified)
		}
		if v, _, _ := reg.DWORD(editorRegSetupKeys, ".json"); v != 1 {
			t.Fatal("the key about to be created was not recorded first")
		}
	})
	t.Run("a Setup that registered every type shows no notice", func(t *testing.T) {
		reg := installedRegistry("1.21.43")
		for _, ext := range textEditorExtensions {
			reg.SetString(editorRegCapabilities+`\FileAssociations`, ext, editorRegProgID)
			reg.SetString(`Software\Classes\`+ext+`\OpenWithProgids`, editorRegProgID, "")
		}
		if status, err := editorRegTypesState(reg, testInstalled); err != nil || status == nil || status.SetupNeeded || status.Missing != 0 {
			t.Fatalf("status %+v %v", status, err)
		}
	})
}

// The in-app addition relies on the installed uninstaller removing every type
// of the list. 1.21.44 is the first such uninstaller for the list as it is.
// A type added to text-extensions.nsh is removed only by the uninstaller of
// the version that first lists it: raise editorRegTypesUninstallerVersion to
// that version, then update this pin.
func TestEditorRegistrationTypesUninstallerKnowsTheList(t *testing.T) {
	sum := sha256.Sum256([]byte(strings.Join(textEditorExtensions, " ")))
	pins := map[string]string{
		"1.21.44": "1ca84a626966f0a8f94c34d551031d8e00a28a901a4d68bb00bf7ddcbc3cb4ae", // 108 types
	}
	if pins[editorRegTypesUninstallerVersion] != hex.EncodeToString(sum[:]) {
		t.Fatalf("text-extensions.nsh changed (%d types): set editorRegTypesUninstallerVersion to the first version whose uninstaller removes every type of the new list, and pin the list for it here", len(textEditorExtensions))
	}
	for version, want := range map[string]bool{"1.21.44": true, "1.21.50": true, "1.22.0": true, "2.0.0": true, "1.21.43": false, "1.9.99": false, "": false, "1.21": false, "v1.21.50": false, "1.21.50 ": false} {
		if got := editorRegUninstallerRecords(version); got != want {
			t.Errorf("%q: %v", version, got)
		}
	}
}

// Only the start that follows an update the user approved and the helper
// confirmed adds types, and only in the main window.
func TestEditorRegistrationTypesAfterAnApprovedUpdateOnly(t *testing.T) {
	originalPlatform, originalExe, originalCompanions := postUpdateEditorPlatform, postUpdateExecutable, companionPlatformSupported
	t.Cleanup(func() {
		postUpdateEditorPlatform, postUpdateExecutable, companionPlatformSupported = originalPlatform, originalExe, originalCompanions
	})
	companionPlatformSupported = func() bool { return false } // no download in this test
	postUpdateExecutable = func() (string, error) { return testInstalled, nil }
	for _, tc := range []struct {
		name      string
		health    string
		confirmed bool
		mode      string
		added     bool
	}{
		{"an approved and confirmed update", "stage", true, "main", true},
		{"a plain start", "", true, "main", false},
		{"an update the helper did not confirm", "stage", false, "main", false},
		{"the standalone Text Editor", "stage", true, standaloneEditorMode, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := installedRegistry("1.21.50")
			before := reg.dump()
			platform, _ := testEditorPlatform(reg)
			postUpdateEditorPlatform = func() (editorRegPlatform, bool) { return platform, true }
			a := testApplication(t)
			a.updateHealthDir, a.windowMode = tc.health, tc.mode
			a.postUpdateRepairs(tc.confirmed)
			missing, _ := editorRegMissingTypes(reg)
			if tc.added != (len(missing) == 0) || tc.added != (a.editorTypesAdded.Load() == int64(len(textEditorExtensions)-len(releasedTypes))) {
				t.Fatalf("missing %d, reported %d", len(missing), a.editorTypesAdded.Load())
			}
			if !tc.added && reg.dump() != before {
				t.Fatal("the registry changed")
			}
		})
	}
	// The page reads the result through the Windows integration status.
	reg := installedRegistry("1.21.43")
	platform, _ := testEditorPlatform(reg)
	originalIntegration, originalIntegrationExe := windowsIntegrationPlatform, windowsIntegrationExecutable
	t.Cleanup(func() {
		windowsIntegrationPlatform, windowsIntegrationExecutable = originalIntegration, originalIntegrationExe
	})
	windowsIntegrationPlatform = func() (editorRegPlatform, bool) { return platform, true }
	windowsIntegrationExecutable = func() (string, error) { return testInstalled, nil }
	a := testApplication(t)
	a.windowMode = "main"
	data := a.windowsIntegrationStatus()
	if data.Types == nil || !data.Types.SetupNeeded || data.Types.UninstallerRecords || data.Types.SetupVersion != "1.21.43" || !data.TextEditor.ThisIsInstalled {
		t.Fatalf("the page is not told to offer Setup: %+v", data.Types)
	}
}
