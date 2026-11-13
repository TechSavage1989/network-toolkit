package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// An in-memory HKCU with the semantics the plans rely on: key and value names
// are case-insensitive, writing a value creates the key and its parents, and
// DeleteKey refuses a key that still has subkeys (it never deletes a tree).
type fakeRegValue struct {
	name  string
	kind  string // sz, dword, none
	str   string
	dword uint32
}

type fakeRegKey struct {
	path   string
	values map[string]fakeRegValue
}

type fakeRegistry struct {
	keys      map[string]*fakeRegKey
	readFault string // a path whose reads fail
	failWrite string // a path whose writes fail
}

func newFakeRegistry() *fakeRegistry {
	r := &fakeRegistry{keys: map[string]*fakeRegKey{}}
	r.create(`Software\Classes`)
	r.create(editorRegRegistered)
	return r
}

func (r *fakeRegistry) create(path string) *fakeRegKey {
	parts := strings.Split(path, `\`)
	var key *fakeRegKey
	for i := range parts {
		p := strings.Join(parts[:i+1], `\`)
		if key = r.keys[strings.ToLower(p)]; key == nil {
			key = &fakeRegKey{path: p, values: map[string]fakeRegValue{}}
			r.keys[strings.ToLower(p)] = key
		}
	}
	return key
}

func (r *fakeRegistry) read(path string) error {
	if r.readFault != "" && strings.EqualFold(path, r.readFault) {
		return errors.New("synthetic read failure")
	}
	return nil
}

func (r *fakeRegistry) write(path string) error {
	if r.failWrite != "" && strings.EqualFold(path, r.failWrite) {
		return errors.New("synthetic write failure")
	}
	return nil
}

func (r *fakeRegistry) children(path string) int {
	prefix, n := strings.ToLower(path)+`\`, 0
	for k := range r.keys {
		if strings.HasPrefix(k, prefix) && !strings.Contains(k[len(prefix):], `\`) {
			n++
		}
	}
	return n
}

func (r *fakeRegistry) KeyInfo(path string) (bool, int, int, error) {
	if err := r.read(path); err != nil {
		return false, 0, 0, err
	}
	key := r.keys[strings.ToLower(path)]
	if key == nil {
		return false, 0, 0, nil
	}
	return true, r.children(path), len(key.values), nil
}

func (r *fakeRegistry) value(path, name string) (fakeRegValue, bool) {
	key := r.keys[strings.ToLower(path)]
	if key == nil {
		return fakeRegValue{}, false
	}
	v, ok := key.values[strings.ToLower(name)]
	return v, ok
}

func (r *fakeRegistry) String(path, name string) (string, bool, bool, error) {
	if err := r.read(path); err != nil {
		return "", false, false, err
	}
	v, found := r.value(path, name)
	if !found {
		return "", false, false, nil
	}
	return v.str, true, v.kind == "sz", nil
}

func (r *fakeRegistry) DWORD(path, name string) (uint32, bool, error) {
	if err := r.read(path); err != nil {
		return 0, false, err
	}
	v, found := r.value(path, name)
	if !found || v.kind != "dword" {
		return 0, found, nil
	}
	return v.dword, true, nil
}

func (r *fakeRegistry) ValueNames(path string) ([]string, error) {
	if err := r.read(path); err != nil {
		return nil, err
	}
	key := r.keys[strings.ToLower(path)]
	if key == nil {
		return nil, nil
	}
	var names []string
	for _, v := range key.values {
		names = append(names, v.name)
	}
	sort.Strings(names)
	return names, nil
}

func (r *fakeRegistry) set(path string, v fakeRegValue) error {
	if err := r.write(path); err != nil {
		return err
	}
	r.create(path).values[strings.ToLower(v.name)] = v
	return nil
}

func (r *fakeRegistry) SetString(path, name, value string) error {
	return r.set(path, fakeRegValue{name: name, kind: "sz", str: value})
}
func (r *fakeRegistry) SetDWORD(path, name string, value uint32) error {
	return r.set(path, fakeRegValue{name: name, kind: "dword", dword: value})
}
func (r *fakeRegistry) SetNone(path, name string) error {
	return r.set(path, fakeRegValue{name: name, kind: "none"})
}

func (r *fakeRegistry) DeleteValue(path, name string) error {
	if err := r.write(path); err != nil {
		return err
	}
	if key := r.keys[strings.ToLower(path)]; key != nil {
		delete(key.values, strings.ToLower(name))
	}
	return nil
}

func (r *fakeRegistry) DeleteKey(path string) error {
	if err := r.write(path); err != nil {
		return err
	}
	if r.children(path) != 0 {
		return errors.New("key has subkeys")
	}
	delete(r.keys, strings.ToLower(path))
	return nil
}

// A canonical dump of everything, for "exactly as before" comparisons.
func (r *fakeRegistry) dump() string {
	var lines []string
	for _, key := range r.keys {
		lines = append(lines, "["+key.path+"]")
		for _, v := range key.values {
			lines = append(lines, fmt.Sprintf("[%s] %s=%s:%s:%d", key.path, v.name, v.kind, v.str, v.dword))
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func (r *fakeRegistry) str(t *testing.T, path, name string) string {
	t.Helper()
	v, _ := r.value(path, name)
	return v.str
}

func (r *fakeRegistry) has(path string) bool { return r.keys[strings.ToLower(path)] != nil }

const (
	testCopyA = `C:\Tools\Network Toolkit\Network_Toolkit.exe`
	testCopyB = `D:\Portable\NTK\Network_Toolkit.exe`
)

func testEditorPlatform(reg *fakeRegistry, missing ...string) (editorRegPlatform, *int) {
	notified := 0
	return editorRegPlatform{
		registry: reg,
		lock:     func() (func(), error) { return func() {}, nil },
		notify:   func() { notified++ },
		presence: func(path string) editorRegPresence {
			for _, m := range missing {
				if strings.EqualFold(m, path) {
					return editorRegAbsent
				}
			}
			return editorRegPresent
		},
		// Only this user, SYSTEM and Administrators can change the copies.
		access: func(string) (string, error) { return "", nil },
	}, &notified
}

// Another program owns .cfg; another program is in the Open With list of .txt.
func fixtureRegistry() *fakeRegistry {
	reg := newFakeRegistry()
	reg.SetString(`Software\Classes\.cfg`, "", "SyntheticConfigTool.Document")
	reg.SetString(`Software\Classes\.txt\OpenWithProgids`, "Synthetic.Text", "")
	reg.SetString(`Software\Classes\.log`, "Content Type", "text/plain")
	reg.SetString(`Software\Classes\*\shell\SyntheticEdit\command`, "", `"C:\Synthetic\edit.exe" "%1"`)
	return reg
}

func TestEditorRegistrationPlanWritesSetupsKeysForThisCopy(t *testing.T) {
	reg := fixtureRegistry()
	before := map[string]string{}
	for _, ext := range textEditorExtensions {
		before[ext] = reg.str(t, `Software\Classes\`+ext, "")
	}
	platform, notified := testEditorPlatform(reg)
	status, err := registerEditorCopy(platform, testCopyA, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != editorRegThis || !status.Complete || *notified != 1 {
		t.Fatalf("status after register: %+v notified=%d", status, *notified)
	}
	command := `"` + testCopyA + `" --editor-standalone --open-file "%1"`
	checks := map[[2]string]string{
		{editorRegProgKey, ""}:                                       "Network Toolkit text document",
		{editorRegProgKey + `\DefaultIcon`, ""}:                      `"` + testCopyA + `",` + fmt.Sprint(editorRegDocumentIcon),
		{editorRegProgKey + `\shell\open\command`, ""}:               command,
		{editorRegCapabilities, "ApplicationName"}:                   "Network Toolkit Text Editor",
		{editorRegCapabilities, "ApplicationIcon"}:                   `"` + testCopyA + `",1`,
		{editorRegCapabilities, "ApplicationDescription"}:            editorRegDescription,
		{editorRegRegistered, "Network Toolkit Text Editor"}:         `Software\Network Toolkit\Capabilities`,
		{editorRegApplication, "FriendlyAppName"}:                    "Network Toolkit Text Editor",
		{editorRegApplication + `\DefaultIcon`, ""}:                  `"` + testCopyA + `",3`,
		{editorRegApplication + `\shell\open`, "Icon"}:               `"` + testCopyA + `",1`,
		{editorRegApplication + `\shell\open\command`, ""}:           command,
		{editorRegMenuVerb, ""}:                                      "Edit with Network Toolkit Text Editor",
		{editorRegMenuVerb, "Icon"}:                                  `"` + testCopyA + `",1`,
		{editorRegMenuVerb + `\command`, ""}:                         command,
		{editorRegPortable, "Executable"}:                            testCopyA,
		{`Software\Classes\.txt\OpenWithProgids`, "Synthetic.Text"}:  "",
		{`Software\Classes\*\shell\SyntheticEdit\command`, ""}:       `"C:\Synthetic\edit.exe" "%1"`,
		{`Software\Classes\.cfg`, ""}:                                "SyntheticConfigTool.Document",
		{`Software\Classes\.log`, "Content Type"}:                    "text/plain",
		{editorRegCapabilities + `\FileAssociations`, ".dockerfile"}: editorRegProgID,
	}
	for at, want := range checks {
		if got, found := reg.value(at[0], at[1]); !found || got.str != want {
			t.Errorf("%s [%s] = %q (found %v), want %q", at[0], at[1], got.str, found, want)
		}
	}
	if v, _ := reg.value(editorRegProgKey, "AllowSilentDefaultTakeOver"); v.kind != "none" {
		t.Errorf("AllowSilentDefaultTakeOver is %q, want REG_NONE", v.kind)
	}
	if menu, _, _ := reg.DWORD(editorRegPortable, "ContextMenu"); menu != 1 {
		t.Error("the right-click choice is not recorded")
	}
	if names, _ := reg.ValueNames(editorRegCapabilities + `\FileAssociations`); len(names) != len(textEditorExtensions) {
		t.Errorf("%d types in Default apps, want %d", len(names), len(textEditorExtensions))
	}
	records, _ := editorRegRecords(reg)
	for _, ext := range textEditorExtensions {
		key := `Software\Classes\` + ext
		if v, found := reg.value(key+`\OpenWithProgids`, editorRegProgID); !found || v.kind != "sz" || v.str != "" {
			t.Errorf("%s is not in Open with", ext)
		}
		// The extension's own default value is never written.
		if got := reg.str(t, key, ""); got != before[ext] {
			t.Errorf("default value of %s changed to %q", ext, got)
		}
		switch ext {
		case ".txt":
			if records[ext] || records[ext+`\OpenWithProgids`] {
				t.Error(".txt existed with its Open With list and must not be recorded")
			}
		case ".cfg", ".log":
			if records[ext] || !records[ext+`\OpenWithProgids`] {
				t.Errorf("%s existed without an Open With list: only the list is recorded (%v)", ext, records)
			}
		default:
			if !records[ext] || records[ext+`\OpenWithProgids`] {
				t.Errorf("%s did not exist and must be recorded as created", ext)
			}
		}
	}
	if !records[`Applications\Network_Toolkit.exe`] || !records["Applications"] || records["*"] || records[`*\shell`] {
		t.Errorf("records of shared keys: %v", records)
	}
	for k := range reg.keys {
		if strings.Contains(k, "fileexts") || strings.Contains(k, "userchoice") {
			t.Errorf("a registration wrote %s", k)
		}
	}
	if _, found := reg.value(editorRegApplication, "SupportedTypes"); found {
		t.Error("SupportedTypes would hide the editor for unknown types")
	}
}

func TestEditorRegistrationRoundTripLeavesRegistryAsBefore(t *testing.T) {
	for _, menu := range []bool{false, true} {
		reg := fixtureRegistry()
		before := reg.dump()
		platform, notified := testEditorPlatform(reg)
		if _, err := registerEditorCopy(platform, testCopyA, menu, false); err != nil {
			t.Fatal(err)
		}
		// Registering again (a repair) changes nothing about what is recorded.
		if _, err := registerEditorCopy(platform, testCopyA, menu, false); err != nil {
			t.Fatal(err)
		}
		status, err := removeEditorCopy(platform, testCopyA)
		if err != nil {
			t.Fatal(err)
		}
		if status.State != editorRegNone || *notified != 3 {
			t.Fatalf("after remove: %+v notified=%d", status, *notified)
		}
		if after := reg.dump(); after != before {
			t.Fatalf("menu=%v: the registry differs after register and remove:\nbefore:\n%s\nafter:\n%s", menu, before, after)
		}
	}
}

func TestEditorRegistrationFromCleanAccountRemovesEveryKeyItCreated(t *testing.T) {
	reg := newFakeRegistry()
	before := reg.dump()
	platform, _ := testEditorPlatform(reg)
	if _, err := registerEditorCopy(platform, testCopyA, true, false); err != nil {
		t.Fatal(err)
	}
	records, _ := editorRegRecords(reg)
	if !records["*"] || records[`*\shell`] {
		t.Fatalf("a missing all-files key is recorded at its top: %v", records)
	}
	if _, err := removeEditorCopy(platform, testCopyA); err != nil {
		t.Fatal(err)
	}
	if after := reg.dump(); after != before {
		t.Fatalf("keys left behind:\n%s", after)
	}
}

func TestEditorRegistrationRemoveKeepsWhatAnotherProgramAdded(t *testing.T) {
	reg := fixtureRegistry()
	platform, _ := testEditorPlatform(reg)
	if _, err := registerEditorCopy(platform, testCopyA, true, false); err != nil {
		t.Fatal(err)
	}
	// After registration another program joins the Open With list of .json,
	// adds its own verb under *\shell, and Windows gives .ini a value.
	reg.SetString(`Software\Classes\.json\OpenWithProgids`, "Synthetic.Json", "")
	reg.SetString(`Software\Classes\.ini`, "PerceivedType", "text")
	reg.SetString(editorRegMenuVerb, "Extended", "")
	if _, err := removeEditorCopy(platform, testCopyA); err != nil {
		t.Fatal(err)
	}
	if v, found := reg.value(`Software\Classes\.json\OpenWithProgids`, "Synthetic.Json"); !found || v.str != "" {
		t.Error("another program's Open With value was removed")
	}
	if _, found := reg.value(`Software\Classes\.json\OpenWithProgids`, editorRegProgID); found {
		t.Error("the Text Editor is still in Open with for .json")
	}
	if reg.str(t, `Software\Classes\.ini`, "PerceivedType") != "text" || reg.has(`Software\Classes\.ini\OpenWithProgids`) {
		t.Error(".ini: the value added later must keep the key; the empty list goes")
	}
	if !reg.has(editorRegMenuVerb) || reg.has(editorRegMenuVerb+`\command`) {
		t.Error("the verb key holds another value and stays; its command goes")
	}
	if reg.has(editorRegPortable) || reg.has(editorRegProgKey) || reg.has(editorRegCapabilities) {
		t.Error("the record, the ProgID and Default apps entry must be gone")
	}
}

func TestEditorRegistrationLeavesAnotherProgramsOpenWithEntry(t *testing.T) {
	reg := fixtureRegistry()
	reg.SetString(editorRegApplication+`\shell\open\command`, "", `"E:\Elsewhere\Network_Toolkit.exe" "%1"`)
	tree := func() string {
		var lines []string
		for _, line := range strings.Split(reg.dump(), "\n") {
			if strings.Contains(strings.ToLower(line), `applications\network_toolkit.exe`) {
				lines = append(lines, line)
			}
		}
		return strings.Join(lines, "\n")
	}
	before := tree()
	platform, _ := testEditorPlatform(reg)
	status, err := registerEditorCopy(platform, testCopyA, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if tree() != before {
		t.Fatalf("another copy's Open with entry changed:\n%s", tree())
	}
	if status.OpenWithElse != `E:\Elsewhere\Network_Toolkit.exe` {
		t.Errorf("the status should name the other Open with entry: %+v", status)
	}
	records, _ := editorRegRecords(reg)
	if records[`Applications\Network_Toolkit.exe`] || records["Applications"] {
		t.Errorf("another program's entry was recorded: %v", records)
	}
	if _, err := removeEditorCopy(platform, testCopyA); err != nil {
		t.Fatal(err)
	}
	if tree() != before {
		t.Fatal("remove changed another copy's Open with entry")
	}
	// An entry Windows wrote for this copy (bare file name) is taken over.
	reg.SetString(editorRegApplication+`\shell\open\command`, "", `"`+testCopyA+`" "%1"`)
	if _, err := registerEditorCopy(platform, testCopyA, false, false); err != nil {
		t.Fatal(err)
	}
	if got := reg.str(t, editorRegApplication+`\shell\open\command`, ""); got != editorRegCommand(testCopyA) {
		t.Errorf("Windows' own entry for this copy was not updated: %q", got)
	}
	// Taken over, it carries every value of the registration's own entry.
	for _, at := range [][3]string{
		{editorRegApplication + `\DefaultIcon`, "", `"` + testCopyA + `",3`},
		{editorRegApplication + `\shell\open`, "Icon", `"` + testCopyA + `",1`},
		{editorRegApplication, "FriendlyAppName", editorRegAppName},
	} {
		if got := reg.str(t, at[0], at[1]); got != at[2] {
			t.Errorf("%s [%s] = %q after taking over Windows' entry, want %q", at[0], at[1], got, at[2])
		}
	}
}

// When the user picks Network_Toolkit_TextEditor.exe in "Open with", Windows
// writes Applications\Network_Toolkit_TextEditor.exe itself, with a command and
// no DefaultIcon, so Explorer shows that program's own first icon (the editor
// tile) for every file associated through it. A registration gives such an
// entry the document icon, records the value, and takes back exactly that
// value; it never creates the entry and never changes its command.
func TestEditorRegistrationGivesWindowsOwnEditorEntryTheDocumentIcon(t *testing.T) {
	editorA := editorRegJoin(editorRegDir(testCopyA), editorRegEditorFile)
	windowsEntry := func(reg *fakeRegistry, program string) {
		reg.SetString(editorRegEditorApplication+`\shell\open\command`, "", `"`+program+`" "%1"`)
	}
	entry := func(reg *fakeRegistry) string {
		var lines []string
		for _, line := range strings.Split(reg.dump(), "\n") {
			if strings.Contains(strings.ToLower(line), `applications\network_toolkit_texteditor.exe`) {
				lines = append(lines, line)
			}
		}
		return strings.Join(lines, "\n")
	}
	t.Run("Windows' entry for this copy's Text Editor program", func(t *testing.T) {
		reg := fixtureRegistry()
		windowsEntry(reg, editorA)
		before := reg.dump()
		platform, _ := testEditorPlatform(reg)
		if _, err := registerEditorCopy(platform, testCopyA, false, false); err != nil {
			t.Fatal(err)
		}
		want := `"` + testCopyA + `",3`
		if got := reg.str(t, editorRegEditorApplication+`\DefaultIcon`, ""); got != want {
			t.Fatalf("DefaultIcon %q, want %q", got, want)
		}
		if got := reg.str(t, editorRegPortable, editorRegEditorAppIcon); got != want {
			t.Fatalf("the written icon is not recorded: %q", got)
		}
		if got := reg.str(t, editorRegEditorApplication+`\shell\open\command`, ""); got != `"`+editorA+`" "%1"` {
			t.Fatalf("Windows' command changed: %q", got)
		}
		if _, found := reg.value(editorRegEditorApplication, "FriendlyAppName"); found {
			t.Fatal("only the icon is written into Windows' entry")
		}
		if _, found := reg.value(editorRegEditorApplication+`\shell\open`, "Icon"); found {
			t.Fatal("the verb icon belongs to the registration's own entry, not to Windows' entry of the Text Editor program")
		}
		// A repair writes the same and records nothing more.
		snapshot := reg.dump()
		if _, err := registerEditorCopy(platform, testCopyA, false, false); err != nil {
			t.Fatal(err)
		}
		if reg.dump() != snapshot {
			t.Fatalf("registering again changed:\n%s", reg.dump())
		}
		if _, err := removeEditorCopy(platform, testCopyA); err != nil {
			t.Fatal(err)
		}
		if reg.dump() != before {
			t.Fatalf("remove did not leave Windows' entry as it was:\nbefore:\n%s\nafter:\n%s", before, reg.dump())
		}
	})
	t.Run("an icon the user set later stays", func(t *testing.T) {
		reg := fixtureRegistry()
		windowsEntry(reg, editorA)
		platform, _ := testEditorPlatform(reg)
		if _, err := registerEditorCopy(platform, testCopyA, false, false); err != nil {
			t.Fatal(err)
		}
		reg.SetString(editorRegEditorApplication+`\DefaultIcon`, "", `C:\Icons\custom.ico`)
		// Registering again does not overwrite it either.
		if _, err := registerEditorCopy(platform, testCopyA, false, false); err != nil {
			t.Fatal(err)
		}
		if got := reg.str(t, editorRegEditorApplication+`\DefaultIcon`, ""); got != `C:\Icons\custom.ico` {
			t.Fatalf("register replaced the user's icon: %q", got)
		}
		if _, err := removeEditorCopy(platform, testCopyA); err != nil {
			t.Fatal(err)
		}
		if got := reg.str(t, editorRegEditorApplication+`\DefaultIcon`, ""); got != `C:\Icons\custom.ico` {
			t.Fatalf("remove took the user's icon: %q", got)
		}
		if reg.has(editorRegPortable) {
			t.Fatalf("the record stayed:\n%s", reg.dump())
		}
	})
	for _, tc := range []struct {
		name  string
		setup func(reg *fakeRegistry)
	}{
		{"an entry of another copy's program", func(reg *fakeRegistry) { windowsEntry(reg, `E:\Other\Network_Toolkit_TextEditor.exe`) }},
		{"an entry with an icon of its own", func(reg *fakeRegistry) {
			windowsEntry(reg, editorA)
			reg.SetString(editorRegEditorApplication+`\DefaultIcon`, "", `C:\Icons\custom.ico`)
		}},
		{"an icon key someone else created", func(reg *fakeRegistry) {
			windowsEntry(reg, editorA)
			reg.create(editorRegEditorApplication + `\DefaultIcon`)
		}},
		{"an entry without a command", func(reg *fakeRegistry) { reg.SetString(editorRegEditorApplication, "Custom", "1") }},
		{"no entry", func(reg *fakeRegistry) {}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := fixtureRegistry()
			tc.setup(reg)
			before, beforeEntry := reg.dump(), entry(reg)
			platform, _ := testEditorPlatform(reg)
			if _, err := registerEditorCopy(platform, testCopyA, true, false); err != nil {
				t.Fatal(err)
			}
			if entry(reg) != beforeEntry {
				t.Fatalf("the entry changed:\n%s\nwas:\n%s", entry(reg), beforeEntry)
			}
			if _, found := reg.value(editorRegPortable, editorRegEditorAppIcon); found {
				t.Fatal("an icon that was not written is recorded")
			}
			if _, err := removeEditorCopy(platform, testCopyA); err != nil {
				t.Fatal(err)
			}
			if reg.dump() != before {
				t.Fatalf("register and remove changed:\n%s", reg.dump())
			}
		})
	}
	t.Run("takeover keeps the record of the replaced copy's icon", func(t *testing.T) {
		reg := fixtureRegistry()
		editorB := editorRegJoin(editorRegDir(testCopyB), editorRegEditorFile)
		windowsEntry(reg, editorB)
		before := reg.dump()
		platform, _ := testEditorPlatform(reg)
		if _, err := registerEditorCopy(platform, testCopyB, false, false); err != nil {
			t.Fatal(err)
		}
		if got := reg.str(t, editorRegEditorApplication+`\DefaultIcon`, ""); got != `"`+testCopyB+`",3` {
			t.Fatalf("B's entry: %q", got)
		}
		if _, err := registerEditorCopy(platform, testCopyA, false, true); err != nil {
			t.Fatal(err)
		}
		// A leaves the entry of B's program as it is; the record goes on.
		if got := reg.str(t, editorRegEditorApplication+`\DefaultIcon`, ""); got != `"`+testCopyB+`",3` {
			t.Fatalf("A changed B's entry: %q", got)
		}
		if got := reg.str(t, editorRegPortable, editorRegEditorAppIcon); got != `"`+testCopyB+`",3` {
			t.Fatalf("the record of B's icon was lost: %q", got)
		}
		if _, err := removeEditorCopy(platform, testCopyA); err != nil {
			t.Fatal(err)
		}
		if reg.dump() != before {
			t.Fatalf("after takeover and remove:\n%s", reg.dump())
		}
	})
	t.Run("clean up of a copy that is gone", func(t *testing.T) {
		reg := fixtureRegistry()
		editorB := editorRegJoin(editorRegDir(testCopyB), editorRegEditorFile)
		windowsEntry(reg, editorB)
		before := reg.dump()
		platform, _ := testEditorPlatform(reg)
		if _, err := registerEditorCopy(platform, testCopyB, true, false); err != nil {
			t.Fatal(err)
		}
		platform, _ = testEditorPlatform(reg, testCopyB)
		if _, err := cleanupEditorCopy(platform, testCopyA, testCopyB); err != nil {
			t.Fatal(err)
		}
		if reg.dump() != before {
			t.Fatalf("clean up left:\n%s", reg.dump())
		}
	})
	t.Run("a read that fails writes nothing", func(t *testing.T) {
		for _, path := range []string{editorRegEditorApplication, editorRegEditorApplication + `\shell\open\command`, editorRegEditorApplication + `\DefaultIcon`} {
			reg := fixtureRegistry()
			windowsEntry(reg, editorA)
			reg.readFault = path
			before := reg.dump()
			platform, _ := testEditorPlatform(reg)
			if _, err := registerEditorCopy(platform, testCopyA, false, false); err == nil {
				t.Fatalf("register with %s unreadable succeeded", path)
			}
			if reg.dump() != before {
				t.Fatalf("register wrote although %s could not be read", path)
			}
		}
	})
}

func TestEditorRegistrationOwnershipStates(t *testing.T) {
	setup := func(reg *fakeRegistry) {
		reg.SetString(editorRegUninstall, "InstallLocation", `C:\Users\u\AppData\Local\Programs\Network Toolkit`)
		reg.SetString(editorRegProgKey+`\shell\open\command`, "", editorRegCommand(`C:\Users\u\AppData\Local\Programs\Network Toolkit\Network_Toolkit.exe`))
	}
	t.Run("nothing", func(t *testing.T) {
		reg := fixtureRegistry()
		platform, _ := testEditorPlatform(reg)
		status, err := editorRegState(reg, testCopyA, platform.presence)
		if err != nil || status.State != editorRegNone || !status.CanRegister || status.CanRemove || status.NeedsTakeover {
			t.Fatalf("%+v %v", status, err)
		}
		if _, err := removeEditorCopy(platform, testCopyA); !errors.Is(err, errEditorRegNothing) {
			t.Fatalf("remove with nothing registered: %v", err)
		}
	})
	t.Run("installed program", func(t *testing.T) {
		reg := fixtureRegistry()
		setup(reg)
		before := reg.dump()
		platform, notified := testEditorPlatform(reg)
		status, _ := editorRegState(reg, testCopyA, platform.presence)
		if status.State != editorRegSetup || status.CanRegister || status.CanRemove || status.NeedsTakeover || !status.Installed || status.ThisIsInstalled {
			t.Fatalf("%+v", status)
		}
		if !strings.Contains(status.Message, "does not change that registration") {
			t.Errorf("the refusal should say so plainly: %q", status.Message)
		}
		for _, takeover := range []bool{false, true} {
			if _, err := registerEditorCopy(platform, testCopyA, true, takeover); !errors.Is(err, errEditorRegInstalled) {
				t.Fatalf("register over the installed program (takeover=%v): %v", takeover, err)
			}
		}
		if _, err := removeEditorCopy(platform, testCopyA); !errors.Is(err, errEditorRegInstalled) {
			t.Fatalf("remove of the installed registration: %v", err)
		}
		if reg.dump() != before || *notified != 0 {
			t.Fatal("a refused action changed the registry")
		}
		// The installed program itself sees that it is the installed one.
		status, _ = editorRegState(reg, `C:\Users\u\AppData\Local\Programs\Network Toolkit\Network_Toolkit.exe`, platform.presence)
		if !status.ThisIsInstalled || status.CanRegister {
			t.Fatalf("installed copy: %+v", status)
		}
	})
	t.Run("this copy, another copy, takeover", func(t *testing.T) {
		reg := fixtureRegistry()
		platform, _ := testEditorPlatform(reg)
		if _, err := registerEditorCopy(platform, testCopyB, true, false); err != nil {
			t.Fatal(err)
		}
		recordsBefore, _ := editorRegRecords(reg)
		status, _ := editorRegState(reg, testCopyA, platform.presence)
		if status.State != editorRegOther || status.Owner != testCopyB || status.OwnerMissing || !status.NeedsTakeover || status.CanRemove {
			t.Fatalf("other copy: %+v", status)
		}
		if !strings.Contains(status.Message, testCopyB) {
			t.Errorf("the status names where: %q", status.Message)
		}
		snapshot := reg.dump()
		if _, err := registerEditorCopy(platform, testCopyA, false, false); !errors.Is(err, errEditorRegTakeover) {
			t.Fatalf("register without confirmation: %v", err)
		}
		if _, err := removeEditorCopy(platform, testCopyA); err == nil {
			t.Fatal("removing a present copy's registration from another copy")
		}
		if reg.dump() != snapshot {
			t.Fatal("refused actions changed the registry")
		}
		status, err := registerEditorCopy(platform, testCopyA, false, true)
		if err != nil || status.State != editorRegThis {
			t.Fatalf("takeover: %+v %v", status, err)
		}
		for _, at := range [][2]string{{editorRegProgKey + `\shell\open\command`, ""}, {editorRegApplication + `\shell\open\command`, ""}} {
			if got := reg.str(t, at[0], at[1]); got != editorRegCommand(testCopyA) {
				t.Errorf("%s still starts %q", at[0], got)
			}
		}
		// B had the right-click entry; A did not ask for it, so it is gone.
		if reg.has(editorRegMenuVerb) {
			t.Error("the replaced copy's right-click entry is left pointing at it")
		}
		recordsAfter, _ := editorRegRecords(reg)
		for name := range recordsBefore {
			if !recordsAfter[name] && name != "*" && name != `*\shell` {
				t.Errorf("record %s of the replaced registration was lost", name)
			}
		}
		// B now sees A.
		status, _ = editorRegState(reg, testCopyB, platform.presence)
		if status.State != editorRegOther || status.Owner != testCopyA {
			t.Fatalf("B after takeover: %+v", status)
		}
	})
	t.Run("missing program", func(t *testing.T) {
		reg := fixtureRegistry()
		before := reg.dump()
		platform, _ := testEditorPlatform(reg)
		if _, err := registerEditorCopy(platform, testCopyB, true, false); err != nil {
			t.Fatal(err)
		}
		platform, _ = testEditorPlatform(reg, testCopyB)
		status, _ := editorRegState(reg, testCopyA, platform.presence)
		if status.State != editorRegOther || !status.OwnerMissing || !status.CanRemove || !strings.Contains(status.Message, "no longer exists") {
			t.Fatalf("missing: %+v", status)
		}
		// Clean up names the program the user saw (security review 11).
		if _, err := cleanupEditorCopy(platform, testCopyA, testCopyB); err != nil {
			t.Fatal(err)
		}
		if reg.dump() != before {
			t.Fatalf("clean up left:\n%s", reg.dump())
		}
	})
	t.Run("unrecorded registration", func(t *testing.T) {
		reg := fixtureRegistry()
		reg.SetString(editorRegProgKey+`\shell\open\command`, "", editorRegCommand(`F:\Old\Network_Toolkit.exe`))
		platform, _ := testEditorPlatform(reg, `F:\Old\Network_Toolkit.exe`)
		status, _ := editorRegState(reg, testCopyA, platform.presence)
		if status.State != editorRegUnrecorded || status.Owner != `F:\Old\Network_Toolkit.exe` || !status.OwnerMissing || !status.NeedsTakeover || !status.CanRemove {
			t.Fatalf("unrecorded: %+v", status)
		}
		if _, err := cleanupEditorCopy(platform, testCopyA, `F:\Old\Network_Toolkit.exe`); err != nil {
			t.Fatal(err)
		}
		if reg.has(editorRegProgKey) {
			t.Error("the stale ProgID was not cleaned up")
		}
	})
	t.Run("program file name", func(t *testing.T) {
		reg := fixtureRegistry()
		platform, _ := testEditorPlatform(reg)
		before := reg.dump()
		status, _ := editorRegState(reg, `C:\Tools\Network_Toolkit_1.21.44.exe`, platform.presence)
		if status.NameOK || status.CanRegister {
			t.Fatalf("%+v", status)
		}
		if _, err := registerEditorCopy(platform, `C:\Tools\Network_Toolkit_1.21.44.exe`, false, false); !errors.Is(err, errEditorRegName) {
			t.Fatal(err)
		}
		if reg.dump() != before {
			t.Fatal("refused register wrote")
		}
	})
}

func TestEditorRegistrationReadFailureWritesNothing(t *testing.T) {
	for _, path := range []string{editorRegUninstall, editorRegPortable, editorRegProgKey + `\shell\open\command`, `Software\Classes\.json`, editorRegApplication + `\shell\open\command`} {
		reg := fixtureRegistry()
		reg.readFault = path
		before := reg.dump()
		platform, _ := testEditorPlatform(reg)
		if _, err := registerEditorCopy(platform, testCopyA, true, false); err == nil {
			t.Fatalf("register with %s unreadable succeeded", path)
		}
		if reg.dump() != before {
			t.Fatalf("register wrote although %s could not be read", path)
		}
	}
}

func TestEditorRegistrationInterruptedWriteCanBeRemoved(t *testing.T) {
	reg := fixtureRegistry()
	before := reg.dump()
	reg.failWrite = editorRegApplication
	platform, _ := testEditorPlatform(reg)
	status, err := registerEditorCopy(platform, testCopyA, true, false)
	if err == nil {
		t.Fatal("the failed write was not reported")
	}
	if status.State != editorRegThis || status.Complete && reg.has(editorRegMenuVerb) {
		t.Logf("status after the failure: %+v", status)
	}
	reg.failWrite = ""
	if _, err := removeEditorCopy(platform, testCopyA); err != nil {
		t.Fatal(err)
	}
	if reg.dump() != before {
		t.Fatalf("an interrupted registration left keys after remove:\n%s", reg.dump())
	}
}

func TestEditorRegistrationTurnsOffTheRightClickEntry(t *testing.T) {
	reg := fixtureRegistry()
	platform, _ := testEditorPlatform(reg)
	if _, err := registerEditorCopy(platform, testCopyA, true, false); err != nil {
		t.Fatal(err)
	}
	status, err := registerEditorCopy(platform, testCopyA, false, false)
	if err != nil || status.ContextMenu {
		t.Fatalf("%+v %v", status, err)
	}
	if reg.has(editorRegMenuVerb) || reg.str(t, `Software\Classes\*\shell\SyntheticEdit\command`, "") == "" {
		t.Fatal("the right-click entry stayed, or another program's went")
	}
}

// Names and shapes must be Setup's: the portable program writes what
// windows.nsi writes, and the file types come from the same list.
func TestEditorRegistrationMatchesSetup(t *testing.T) {
	nsi, err := os.ReadFile(filepath.Join("..", "..", "scripts", "installer", "windows.nsi"))
	if err != nil {
		t.Fatal(err)
	}
	defines := map[string]string{}
	for _, m := range regexp.MustCompile(`(?m)^!define (\w+) (?:"([^"]*)"|'([^']*)')\s*$`).FindAllStringSubmatch(string(nsi), -1) {
		defines[m[1]] = m[2] + m[3]
	}
	expand := func(s string) string {
		for i := 0; i < 5; i++ {
			s = regexp.MustCompile(`\$\{(\w+)\}`).ReplaceAllStringFunc(s, func(ref string) string {
				if v, ok := defines[ref[2:len(ref)-1]]; ok {
					return v
				}
				return ref
			})
		}
		return s
	}
	for name, want := range map[string]string{
		"EDITOR_NAME": editorRegAppName, "APP_FILENAME": editorRegAppFile, "EDITOR_ARGS": editorRegArgs, "PROG_ID": editorRegProgID,
		"PROG_KEY": editorRegProgKey, "CAPABILITIES": editorRegCapabilities, "APPLICATIONS_KEY": editorRegApplications, "APPLICATION_KEY": editorRegApplication,
		"ALL_FILES_KEY": editorRegAllFiles, "MENU_VERB_KEY": editorRegMenuVerb, "MENU_TEXT": editorRegMenuText, "EDITOR_KEY": editorRegSetupRecord,
		"UNINSTALL_KEY": editorRegUninstall, "PORTABLE_KEY": editorRegPortable,
		"EDITOR_FILENAME": editorRegEditorFile, "EDITOR_APPLICATION_KEY": editorRegEditorApplication, "EDITOR_APPLICATION_ICON": editorRegEditorAppIcon,
	} {
		if got := expand("${" + name + "}"); got != want {
			t.Errorf("windows.nsi %s = %q, the portable program uses %q", name, got, want)
		}
	}
	if got := expand("${EDITOR_COMMAND}"); got != editorRegCommand(`$INSTDIR\Network_Toolkit.exe`) {
		t.Errorf("EDITOR_COMMAND %q", got)
	}
	text := expand(string(nsi))
	for _, line := range []string{
		`WriteRegStr HKCU "` + editorRegProgKey + `" "" "` + editorRegProgName + `"`,
		`WriteRegStr HKCU "` + editorRegCapabilities + `" "ApplicationDescription" "` + editorRegDescription + `"`,
		`WriteRegStr HKCU "Software\RegisteredApplications" "` + editorRegAppName + `" "` + editorRegCapabilities + `"`,
	} {
		if !strings.Contains(text, line) {
			t.Errorf("windows.nsi no longer writes %s", line)
		}
	}
	icon := func(key string) string {
		m := regexp.MustCompile(regexp.QuoteMeta(`WriteRegStr HKCU "`+key+`" "" '"$INSTDIR\Network_Toolkit.exe",`) + `(\d+)'`).FindStringSubmatch(text)
		if m == nil {
			t.Fatalf("windows.nsi writes no icon for %s", key)
		}
		return m[1]
	}
	if got := icon(editorRegProgKey + `\DefaultIcon`); got != fmt.Sprint(editorRegDocumentIcon) {
		t.Errorf("Setup's document icon is %s, the portable program writes %d", got, editorRegDocumentIcon)
	}
	// Explorer shows the DefaultIcon of an Applications entry as the icon of
	// every file the user associated through it: the document icon, as the
	// ProgID's. The program values keep the editor icon.
	if got := icon(editorRegApplication + `\DefaultIcon`); got != fmt.Sprint(editorRegDocumentIcon) {
		t.Errorf("Setup's Open with entry icon is %s, the portable program writes %d", got, editorRegDocumentIcon)
	}
	if got := icon(editorRegEditorApplication + `\DefaultIcon`); got != fmt.Sprint(editorRegDocumentIcon) {
		t.Errorf("Setup gives Windows' own entry of the Text Editor program icon %s, the portable program writes %d", got, editorRegDocumentIcon)
	}
	for _, line := range []string{
		`WriteRegStr HKCU "` + editorRegCapabilities + `" "ApplicationIcon" '"$INSTDIR\Network_Toolkit.exe",` + fmt.Sprint(editorRegEditorIcon) + `'`,
		`WriteRegStr HKCU "` + editorRegMenuVerb + `" "Icon" '"$INSTDIR\Network_Toolkit.exe",` + fmt.Sprint(editorRegEditorIcon) + `'`,
		// Open with shows the program by its verb icon, whatever it reads.
		`WriteRegStr HKCU "` + editorRegApplication + `\shell\open" "Icon" '"$INSTDIR\Network_Toolkit.exe",` + fmt.Sprint(editorRegEditorIcon) + `'`,
		`WriteRegStr HKCU "` + editorRegSetupRecord + `" "` + editorRegEditorAppIcon + `" '"$INSTDIR\Network_Toolkit.exe",` + fmt.Sprint(editorRegDocumentIcon) + `'`,
	} {
		if !strings.Contains(text, line) {
			t.Errorf("windows.nsi no longer writes %s", line)
		}
	}
	list, err := os.ReadFile(filepath.Join("..", "..", "scripts", "installer", "text-extensions.nsh"))
	if err != nil {
		t.Fatal(err)
	}
	var setupTypes []string
	for _, m := range regexp.MustCompile(`(?m)^\s*!insertmacro \$\{ACTION\} "(\.[^"]+)"\s*$`).FindAllStringSubmatch(string(list), -1) {
		setupTypes = append(setupTypes, m[1])
	}
	if strings.Join(setupTypes, " ") != strings.Join(textEditorExtensions, " ") {
		t.Fatal("text_editor_extensions.go differs from text-extensions.nsh: run node scripts/text-extensions.cjs --write")
	}
}

func TestEditorRegistrationCommandLine(t *testing.T) {
	reg := fixtureRegistry()
	platform, _ := testEditorPlatform(reg)
	oldPlatform, oldExe := windowsIntegrationPlatform, windowsIntegrationExecutable
	t.Cleanup(func() { windowsIntegrationPlatform, windowsIntegrationExecutable = oldPlatform, oldExe })
	windowsIntegrationPlatform = func() (editorRegPlatform, bool) { return platform, true }
	exe := testCopyA
	windowsIntegrationExecutable = func() (string, error) { return exe, nil }
	run := runTextEditorRegistrationCommand
	if code := run("remove"); code != 6 {
		t.Fatalf("remove with nothing registered: %d", code)
	}
	if code := run("register"); code != 0 {
		t.Fatalf("register: %d", code)
	}
	exe = testCopyB
	if code := run("register"); code != 4 {
		t.Fatalf("second copy: %d", code)
	}
	if code := run("takeover-menu"); code != 0 {
		t.Fatalf("takeover: %d", code)
	}
	if code := run("cleanup"); code != 6 {
		t.Fatalf("cleanup of a present registration: %d", code)
	}
	if code := run("remove"); code != 0 {
		t.Fatalf("remove: %d", code)
	}
	if code := run("bogus"); code != 2 {
		t.Fatal("an unknown action was accepted")
	}
}
