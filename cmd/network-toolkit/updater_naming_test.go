package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func namingTestWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0700); err != nil {
		t.Fatal(err)
	}
}
func namingTestPointer(t *testing.T, dir string, p updateNamingPointer) []byte {
	t.Helper()
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	namingTestWrite(t, filepath.Join(dir, updateNamingPointerFile), data)
	return data
}
func namingTestManifest(version string, data []byte) updateNamingPointer {
	return updateNamingPointer{Format: updateNamingPointerFormat, Executable: updateOfficialExecutableName(version), Version: version, SHA256: updateHash(data)}
}
func namingTestBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func TestUpdateNamingOfficialNamesOnly(t *testing.T) {
	for _, name := range []string{"app.exe", "network_toolkit_1.17.60_Windows_x64.exe", "Network_Toolkit_1.17.60_Windows_x64.EXE", "Network_Toolkit_01.17.60_Windows_x64.exe", "Network_Toolkit_1.17.60_Windows_x64 (1).exe", "Network_Toolkit_1.17.60_Windows_x64.exe:evil", "../Network_Toolkit_1.17.60_Windows_x64.exe", `C:\Network_Toolkit_1.17.60_Windows_x64.exe`, "Network_Toolkit_1.17.60-beta_Windows_x64.exe"} {
		if _, ok := updateOfficialExecutableVersion(name); ok {
			t.Errorf("claimed custom or unsafe name %q", name)
		}
	}
	if v, ok := updateOfficialExecutableVersion("Network_Toolkit_1.17.60_Windows_x64.exe"); !ok || v != "1.17.60" {
		t.Fatalf("official name rejected: %q %v", v, ok)
	}
}
func TestUpdateNamingFirstMigrationRetainsOriginalAndForwards(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, updateOfficialExecutableName("1.17.58"))
	data := updateTestPE("1.17.60")
	namingTestWrite(t, old, data)
	if target, _, err := updateNamingForwardTarget(old, "1.17.60", nil, false); err != nil || target != "" {
		t.Fatalf("first launch must run in place: %q %v", target, err)
	}
	status, err := updateNamingPromote(old, "1.17.60")
	if err != nil {
		t.Fatal(err)
	}
	canonical := filepath.Join(dir, updateOfficialExecutableName("1.17.60"))
	if !status.Managed || status.CanonicalPath != canonical || !reflect.DeepEqual(status.Compatibility, []string{old}) {
		t.Fatalf("wrong migration status: %#v", status)
	}
	if !bytes.Equal(namingTestBytes(t, old), data) || !bytes.Equal(namingTestBytes(t, canonical), data) {
		t.Fatal("canonical or original bytes changed")
	}
	for _, launch := range []struct{ path, want string }{{old, canonical}, {canonical, ""}} {
		if target, warning, err := updateNamingForwardTarget(launch.path, "1.17.60", []string{"--data-dir", "relative settings"}, false); err != nil || warning != "" || target != launch.want {
			t.Fatalf("forward %s: %q %q %v", launch.path, target, warning, err)
		}
	}
	before := namingTestBytes(t, filepath.Join(dir, updateNamingPointerFile))
	if _, err := updateNamingPromote(old, "1.17.60"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, namingTestBytes(t, filepath.Join(dir, updateNamingPointerFile))) {
		t.Fatal("migration not idempotent")
	}
}
func TestUpdateNamingMultipleUpgradesForwardDirectly(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, updateOfficialExecutableName("1.17.58"))
	namingTestWrite(t, first, updateTestPE("1.17.60"))
	if _, err := updateNamingPromote(first, "1.17.60"); err != nil {
		t.Fatal(err)
	}
	previous := "1.17.60"
	for _, version := range []string{"1.17.61", "1.17.62"} {
		replaced := filepath.Join(dir, updateOfficialExecutableName(previous))
		namingTestWrite(t, replaced, updateTestPE(version))
		for _, args := range [][]string{{"--update-health", "staging"}, nil} {
			if target, _, err := updateNamingForwardTarget(replaced, version, args, false); err != nil || target != "" {
				t.Fatalf("replaced canonical startup: %q %v", target, err)
			}
		}
		if _, err := updateNamingPromote(replaced, version); err != nil {
			t.Fatal(err)
		}
		previous = version
	}
	latest := filepath.Join(dir, updateOfficialExecutableName("1.17.62"))
	for _, tc := range []struct{ nameVersion, binaryVersion string }{{"1.17.58", "1.17.60"}, {"1.17.60", "1.17.61"}, {"1.17.61", "1.17.62"}} {
		alias := filepath.Join(dir, updateOfficialExecutableName(tc.nameVersion))
		if target, _, err := updateNamingForwardTarget(alias, tc.binaryVersion, nil, false); err != nil || target != latest {
			t.Fatalf("alias not direct to latest: %q %v", target, err)
		}
	}
	p, _, err := updateNamingReadPointer(dir)
	if err != nil || p.Version != "1.17.62" || len(p.Compatibility) != 3 {
		t.Fatalf("history: %#v %v", p, err)
	}
	if target, _, err := updateNamingForwardTarget(latest, "1.17.62", nil, true); err != nil || target != "" {
		t.Fatalf("forwarded target recursively launched: %q %v", target, err)
	}
	if _, _, err := updateNamingForwardTarget(first, "1.17.60", nil, true); err == nil {
		t.Fatal("permitted second hop")
	}
}
func TestUpdateNamingCustomNamesAndSettingsUntouched(t *testing.T) {
	dir := t.TempDir()
	custom := filepath.Join(dir, "my-app.exe")
	data := updateTestPE("1.17.60")
	namingTestWrite(t, custom, data)
	settings := filepath.Join(dir, "settings.json")
	namingTestWrite(t, settings, []byte(`{"keep":"settings"}`))
	namingTestWrite(t, filepath.Join(dir, updateNamingPointerFile), []byte("unrelated"))
	if target, _, err := updateNamingForwardTarget(custom, "1.17.60", nil, false); err != nil || target != "" {
		t.Fatalf("custom path used pointer: %q %v", target, err)
	}
	status, err := updateNamingPromote(custom, "1.17.60")
	if err != nil || status.Managed || status.CanonicalPath != "" {
		t.Fatalf("custom name changed: %#v %v", status, err)
	}
	if !bytes.Equal(namingTestBytes(t, custom), data) || string(namingTestBytes(t, settings)) != `{"keep":"settings"}` || string(namingTestBytes(t, filepath.Join(dir, updateNamingPointerFile))) != "unrelated" {
		t.Fatal("custom file or settings modified")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 3 {
		t.Fatal("custom executable generated files")
	}
}
func TestUpdateNamingCanonicalCollisionNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, updateOfficialExecutableName("1.17.58"))
	canonical := filepath.Join(dir, updateOfficialExecutableName("1.17.60"))
	namingTestWrite(t, old, updateTestPE("1.17.60"))
	namingTestWrite(t, canonical, []byte("unrelated existing file"))
	if _, err := updateNamingPromote(old, "1.17.60"); err == nil {
		t.Fatal("accepted collision")
	}
	if string(namingTestBytes(t, canonical)) != "unrelated existing file" {
		t.Fatal("collision changed")
	}
	if _, err := os.Lstat(filepath.Join(dir, updateNamingPointerFile)); !os.IsNotExist(err) {
		t.Fatal("published pointer despite collision")
	}
	namingTestWrite(t, canonical, updateTestPE("1.17.60"))
	if _, err := updateNamingPromote(old, "1.17.60"); err != nil {
		t.Fatal(err)
	}
}
func TestUpdateNamingFailedUpgradePreservesPriorPointer(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, updateOfficialExecutableName("1.17.58"))
	namingTestWrite(t, old, updateTestPE("1.17.60"))
	if _, err := updateNamingPromote(old, "1.17.60"); err != nil {
		t.Fatal(err)
	}
	pointer := filepath.Join(dir, updateNamingPointerFile)
	prior := namingTestBytes(t, pointer)
	replaced := filepath.Join(dir, updateOfficialExecutableName("1.17.60"))
	namingTestWrite(t, replaced, updateTestPE("1.17.61"))
	if target, _, err := updateNamingForwardTarget(replaced, "1.17.61", []string{"--update-health=staging"}, false); err != nil || target != "" {
		t.Fatal("health launch forwarded")
	}
	if !bytes.Equal(prior, namingTestBytes(t, pointer)) {
		t.Fatal("pointer changed before readiness")
	}
	namingTestWrite(t, replaced, updateTestPE("1.17.60"))
	if target, _, err := updateNamingForwardTarget(old, "1.17.60", nil, false); err != nil || target != replaced {
		t.Fatalf("rollback lost known current: %q %v", target, err)
	}
	namingTestWrite(t, replaced, updateTestPE("1.17.61"))
	collision := filepath.Join(dir, updateOfficialExecutableName("1.17.61"))
	namingTestWrite(t, collision, []byte("unrelated"))
	if _, err := updateNamingPromote(replaced, "1.17.61"); err == nil {
		t.Fatal("collision not reported")
	}
	if !bytes.Equal(prior, namingTestBytes(t, pointer)) || string(namingTestBytes(t, collision)) != "unrelated" {
		t.Fatal("failed promotion lost prior pointer/unrelated file")
	}
	if _, _, err := updateNamingForwardTarget(old, "1.17.60", nil, false); err == nil {
		t.Fatal("outdated alias silently fell back")
	}
}
func TestUpdateNamingMalformedPointersFailClosedForAliases(t *testing.T) {
	good := namingTestManifest("1.17.60", updateTestPE("1.17.60"))
	valid, _ := json.Marshal(good)
	bads := [][]byte{[]byte("unrelated user text"), append(append([]byte{}, valid...), []byte(` {}`)...), bytes.Replace(valid, []byte(`"format":`), []byte(`"unknown":1,"format":`), 1)}
	for _, change := range []func(*updateNamingPointer){
		func(p *updateNamingPointer) { p.Format = "OtherApp/1" }, func(p *updateNamingPointer) { p.Executable = "../" + p.Executable }, func(p *updateNamingPointer) { p.Executable = `C:\` + p.Executable }, func(p *updateNamingPointer) { p.Executable += ":evil" }, func(p *updateNamingPointer) { p.Version = "1.17.61" }, func(p *updateNamingPointer) { p.SHA256 = strings.Repeat("z", 64) }, func(p *updateNamingPointer) { p.SHA256 = strings.ToUpper(p.SHA256) }, func(p *updateNamingPointer) { p.Compatibility = []string{"app.exe"} }, func(p *updateNamingPointer) { p.Compatibility = []string{updateOfficialExecutableName("1.17.61")} }, func(p *updateNamingPointer) {
			p.Compatibility = []string{updateOfficialExecutableName("1.17.58"), updateOfficialExecutableName("1.17.58")}
		},
	} {
		p := good
		change(&p)
		b, _ := json.Marshal(p)
		bads = append(bads, b)
	}
	for i, bad := range bads {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			dir := t.TempDir()
			alias := filepath.Join(dir, updateOfficialExecutableName("1.17.58"))
			canonical := filepath.Join(dir, good.Executable)
			namingTestWrite(t, alias, updateTestPE("1.17.60"))
			namingTestWrite(t, canonical, updateTestPE("1.17.60"))
			pointer := filepath.Join(dir, updateNamingPointerFile)
			namingTestWrite(t, pointer, bad)
			if _, _, err := updateNamingForwardTarget(alias, "1.17.60", nil, false); err == nil {
				t.Fatal("alias accepted invalid pointer")
			}
			if target, warning, err := updateNamingForwardTarget(canonical, "1.17.60", nil, false); err != nil || warning == "" || target != "" {
				t.Fatalf("correctly named known binary blocked: %q %q %v", target, warning, err)
			}
			if _, err := updateNamingPromote(alias, "1.17.60"); err == nil {
				t.Fatal("invalid pointer overwritten")
			}
			if !bytes.Equal(bad, namingTestBytes(t, pointer)) {
				t.Fatal("unrelated pointer changed")
			}
		})
	}
}
func TestUpdateNamingTargetValidation(t *testing.T) {
	for _, kind := range []string{"missing", "corrupt hash", "not PE", "wrong version", "wrong product", "directory"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			alias := filepath.Join(dir, updateOfficialExecutableName("1.17.58"))
			namingTestWrite(t, alias, updateTestPE("1.17.60"))
			data := updateTestPE("1.17.61")
			p := namingTestManifest("1.17.61", data)
			path := filepath.Join(dir, p.Executable)
			switch kind {
			case "not PE":
				data = []byte("not Windows")
				p.SHA256 = updateHash(data)
			case "wrong version":
				data = updateTestPE("1.17.60")
				p.SHA256 = updateHash(data)
			case "wrong product":
				data = bytes.Replace(data, updateUTF16("NetworkToolkit"), updateUTF16("AnotherProduct"), 1)
				p.SHA256 = updateHash(data)
			case "corrupt hash":
				data = append(data, 'x')
			}
			if kind == "directory" {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			} else if kind != "missing" {
				namingTestWrite(t, path, data)
			}
			namingTestPointer(t, dir, p)
			if target, _, err := updateNamingForwardTarget(alias, "1.17.60", nil, false); err == nil || target != "" {
				t.Fatalf("unsafe target accepted: %q %v", target, err)
			}
		})
	}
}
func TestUpdateNamingRejectsLinkedFilesAndFolders(t *testing.T) {
	for _, kind := range []string{"pointer", "target", "folder"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			data := updateTestPE("1.17.61")
			p := namingTestManifest("1.17.61", data)
			alias := filepath.Join(dir, updateOfficialExecutableName("1.17.58"))
			namingTestWrite(t, alias, updateTestPE("1.17.60"))
			target := filepath.Join(dir, p.Executable)
			namingTestWrite(t, target, data)
			namingTestPointer(t, dir, p)
			if kind == "folder" {
				linked := filepath.Join(t.TempDir(), "linked")
				if err := os.Symlink(dir, linked); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				alias = filepath.Join(linked, filepath.Base(alias))
			} else {
				path := target
				if kind == "pointer" {
					path = filepath.Join(dir, updateNamingPointerFile)
				}
				real := path + ".actual"
				if err := os.Rename(path, real); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(real, path); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			if _, _, err := updateNamingForwardTarget(alias, "1.17.60", nil, false); err == nil {
				t.Fatal("linked target/pointer/folder accepted")
			}
		})
	}
}
func TestUpdateNamingNoDowngradeOrNewerPointerOverwrite(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, updateOfficialExecutableName("1.17.60"))
	latest := filepath.Join(dir, updateOfficialExecutableName("1.17.61"))
	namingTestWrite(t, old, updateTestPE("1.17.60"))
	data := updateTestPE("1.17.61")
	namingTestWrite(t, latest, data)
	before := namingTestPointer(t, dir, namingTestManifest("1.17.61", data))
	if target, _, err := updateNamingForwardTarget(old, "1.17.60", nil, false); err != nil || target != latest {
		t.Fatalf("older canonical not forwarded: %q %v", target, err)
	}
	status, err := updateNamingPromote(old, "1.17.60")
	if err != nil || status.CanonicalPath != latest || !bytes.Equal(before, namingTestBytes(t, filepath.Join(dir, updateNamingPointerFile))) {
		t.Fatalf("older process downgraded pointer: %#v %v", status, err)
	}
	before = namingTestPointer(t, dir, namingTestManifest("1.17.59", updateTestPE("1.17.59")))
	if target, warning, err := updateNamingForwardTarget(latest, "1.17.61", nil, false); err != nil || target != "" || warning == "" {
		t.Fatalf("new download blocked by obsolete pointer: %q %q %v", target, warning, err)
	}
	if _, err := updateNamingPromote(latest, "1.17.61"); err == nil || !bytes.Equal(before, namingTestBytes(t, filepath.Join(dir, updateNamingPointerFile))) {
		t.Fatal("invalid older pointer overwritten")
	}
}
func TestUpdateNamingPrivateLaunchesNeverForward(t *testing.T) {
	dir := t.TempDir()
	alias := filepath.Join(dir, updateOfficialExecutableName("1.17.58"))
	namingTestWrite(t, alias, updateTestPE("1.17.60"))
	namingTestWrite(t, filepath.Join(dir, updateNamingPointerFile), []byte("bad pointer"))
	for _, args := range [][]string{{"--update-health", "staging"}, {"-update-health", "staging"}, {"--update-health=staging"}, {"-update-health=staging"}, {"--apply-update", "staging"}, {"--terminal-view"}, {"--local-tools-helper=pipe"}, {"--packet-capture-helper", "pipe"}} {
		if target, _, err := updateNamingForwardTarget(alias, "1.17.60", args, false); err != nil || target != "" {
			t.Fatalf("private launch forwarded: %#v %q %v", args, target, err)
		}
	}
}
func TestUpdateNamingForwardPreservesArgumentsCWDAndStreams(t *testing.T) {
	target := filepath.Join(t.TempDir(), updateOfficialExecutableName("1.17.61"))
	cwd := t.TempDir()
	args := []string{"--data-dir", `..\My settings\ユニコード`, "--terminal", "--terminal-preset=nonsecret=with spaces", "--port", "8765"}
	cmd := updateNamingForwardCommand(target, args, cwd, []string{"KEY=val", updateNamingForwardedEnv + "=old", strings.ToLower(updateNamingForwardedEnv) + "=old2"})
	if cmd.Path != target || cmd.Dir != cwd || !reflect.DeepEqual(cmd.Args, append([]string{target}, args...)) || cmd.Stdin != os.Stdin || cmd.Stdout != os.Stdout || cmd.Stderr != os.Stderr {
		t.Fatalf("launch context changed: %#v", cmd)
	}
	if !reflect.DeepEqual(cmd.Env, []string{"KEY=val", updateNamingForwardedEnv + "=1"}) {
		t.Fatalf("env not preserved/guard duplicated: %#v", cmd.Env)
	}
}
func TestUpdateNamingAtomicPublicationNeverClobbersUnrelatedFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing")
	namingTestWrite(t, path, []byte("keep"))
	if err := updateNamingWriteNewFile(path, []byte("replace"), 0700); err == nil || string(namingTestBytes(t, path)) != "keep" {
		t.Fatal("exclusive publication clobbered file")
	}
	p := namingTestManifest("1.17.60", updateTestPE("1.17.60"))
	pointer := filepath.Join(dir, updateNamingPointerFile)
	prior := namingTestPointer(t, dir, p)
	namingTestWrite(t, pointer, []byte("someone else changed it"))
	p.Version, p.Executable = "1.17.61", updateOfficialExecutableName("1.17.61")
	if err := updateNamingPublishPointer(dir, p, prior); err == nil || string(namingTestBytes(t, pointer)) != "someone else changed it" {
		t.Fatal("pointer compare-before-replace failed")
	}
	if err := updateNamingPublishPointer(dir, p, nil); err == nil || string(namingTestBytes(t, pointer)) != "someone else changed it" {
		t.Fatal("initial pointer clobbered file")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatal("temporary publication files leaked")
	}
}
func TestUpdateNamingConcurrentHealthyLaunchesKeepNewest(t *testing.T) {
	dir := t.TempDir()
	older := filepath.Join(dir, updateOfficialExecutableName("1.17.58"))
	newer := filepath.Join(dir, updateOfficialExecutableName("1.17.59"))
	namingTestWrite(t, older, updateTestPE("1.17.60"))
	namingTestWrite(t, newer, updateTestPE("1.17.61"))
	var wg sync.WaitGroup
	for _, tc := range []struct{ path, version string }{{older, "1.17.60"}, {newer, "1.17.61"}} {
		wg.Add(1)
		go func(path, version string) {
			defer wg.Done()
			if _, err := updateNamingPromote(path, version); err != nil {
				t.Errorf("concurrent promotion: %v", err)
			}
		}(tc.path, tc.version)
	}
	wg.Wait()
	p, _, err := updateNamingReadPointer(dir)
	if err != nil || p.Version != "1.17.61" {
		t.Fatalf("newest promotion lost: %#v %v", p, err)
	}
	if _, err := updateNamingValidateTarget(dir, p); err != nil {
		t.Fatal(err)
	}
}
