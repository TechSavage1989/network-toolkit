package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

func shortcutFixture(stringsData [5]string, extra ...[]byte) []byte {
	data := make([]byte, 76)
	binary.LittleEndian.PutUint32(data, 76)
	copy(data[4:20], []byte{1, 0x14, 2, 0, 0, 0, 0, 0, 0xc0, 0, 0, 0, 0, 0, 0, 0x46})
	flags := uint32(0x80)
	binary.LittleEndian.PutUint32(data[56:], 9)     // icon index
	binary.LittleEndian.PutUint32(data[60:], 3)     // maximized
	binary.LittleEndian.PutUint16(data[64:], 0x241) // Ctrl+A
	for i, text := range stringsData {
		if text == "" {
			continue
		}
		flags |= 4 << i
		chars := utf16.Encode([]rune(text))
		buf := make([]byte, 2+len(chars)*2)
		binary.LittleEndian.PutUint16(buf, uint16(len(chars)))
		for j, c := range chars {
			binary.LittleEndian.PutUint16(buf[2+j*2:], c)
		}
		data = append(data, buf...)
	}
	binary.LittleEndian.PutUint32(data[20:], flags)
	for _, block := range extra {
		data = append(data, block...)
	}
	return append(data, 0, 0, 0, 0)
}

func shortcutExtra(signature uint32, data []byte) []byte {
	block := make([]byte, 8+len(data))
	binary.LittleEndian.PutUint32(block, uint32(len(block)))
	binary.LittleEndian.PutUint32(block[4:], signature)
	copy(block[8:], data)
	return block
}

func TestUpdateShortcutPreservesAllNonTargetSettings(t *testing.T) {
	fields := [5]string{"Network tools", `..\old.exe`, `C:\working`, `--profile "my profile"`, `C:\custom.ico`}
	extra := shortcutExtra(0x12345678, []byte("opaque custom data"))
	before := shortcutFixture(fields, extra)
	targetChange := append([]byte(nil), before...)
	targetChange[24] = 1 // Target attributes may change.
	if err := updateShortcutPreservesSettings(before, targetChange); err != nil {
		t.Fatal(err)
	}
	changedRelative := fields
	changedRelative[1] = `..\Network_Toolkit.exe`
	if err := updateShortcutPreservesSettings(before, shortcutFixture(changedRelative, extra)); err != nil {
		t.Fatalf("relative target may change: %v", err)
	}
	for _, index := range []int{0, 2, 3, 4} {
		changed := fields
		changed[index] += "changed"
		if err := updateShortcutPreservesSettings(before, shortcutFixture(changed, extra)); err == nil {
			t.Fatalf("setting %d unexpectedly changed", index)
		}
	}
	for _, offset := range []int{21, 56, 60, 64, 68} {
		changed := append([]byte(nil), before...)
		changed[offset] ^= 1
		if err := updateShortcutPreservesSettings(before, changed); err == nil {
			t.Fatalf("header setting at %d unexpectedly changed", offset)
		}
	}
	for _, changed := range [][]byte{
		shortcutFixture(fields),
		shortcutFixture(fields, shortcutExtra(0x12345678, []byte("changed custom data"))),
		shortcutFixture(fields, extra, shortcutExtra(0xa0000009, []byte("new property store"))),
	} {
		if err := updateShortcutPreservesSettings(before, changed); err == nil {
			t.Fatal("custom data unexpectedly changed")
		}
	}
	before = shortcutFixture(fields, extra, shortcutExtra(0xa0000003, []byte("old tracker")))
	after := shortcutFixture(fields, extra, shortcutExtra(0xa0000003, []byte("new target tracker")))
	if err := updateShortcutPreservesSettings(before, after); err != nil {
		t.Fatalf("target tracking may change: %v", err)
	}
}

func TestUpdateShortcutOnlyRetargetsVerifiedExecutableIcon(t *testing.T) {
	old := `C:\Toolkit\Network_Toolkit_1.17.61_Windows_x64.exe`
	newPath := `C:\Toolkit\Network_Toolkit.exe`
	fields := [5]string{"Description", "", `C:\Toolkit`, "--flag", old}
	before := shortcutFixture(fields)
	fields[4] = newPath
	after := shortcutFixture(fields)
	if err := updateShortcutPreservesSettingsForTarget(before, after, []string{old}, newPath); err != nil {
		t.Fatal(err)
	}
	if err := updateShortcutPreservesSettings(before, after); err == nil {
		t.Fatal("unapproved icon rewrite accepted")
	}
	fields[4] = `C:\custom.ico`
	if err := updateShortcutPreservesSettingsForTarget(shortcutFixture(fields), after, []string{old}, newPath); err == nil {
		t.Fatal("custom icon rewrite accepted")
	}
	after[56]++
	if err := updateShortcutPreservesSettingsForTarget(before, after, []string{old}, newPath); err == nil {
		t.Fatal("icon index rewrite accepted")
	}
}

func TestUpdateShortcutParserRejectsMalformedData(t *testing.T) {
	valid := shortcutFixture([5]string{"name", "", "", "arguments", ""})
	for n := 0; n < len(valid); n++ {
		if _, err := updateShortcutParseSettings(valid[:n]); err == nil {
			t.Fatalf("accepted truncated shortcut at %d", n)
		}
	}
	if _, err := updateShortcutParseSettings(append(valid, 0)); err == nil {
		t.Fatal("accepted trailing bytes")
	}
	for _, offset := range []int{0, 4} {
		bad := append([]byte(nil), valid...)
		bad[offset] = 0
		if _, err := updateShortcutParseSettings(bad); err == nil {
			t.Fatal("accepted invalid header")
		}
	}
	bad := shortcutFixture([5]string{}, shortcutExtra(0x12345678, nil))
	binary.LittleEndian.PutUint32(bad[76:], 0xffffffff)
	if _, err := updateShortcutParseSettings(bad); err == nil {
		t.Fatal("accepted overflowing block length")
	}
}

type shortcutTestEnvironment struct {
	old, stable, settings, desktop, menu string
	original, replacement                []byte
}

func newShortcutTestEnvironment(t *testing.T) shortcutTestEnvironment {
	t.Helper()
	base := t.TempDir()
	e := shortcutTestEnvironment{
		old: filepath.Join(base, "Network_Toolkit_1.17.61_Windows_x64.exe"), stable: filepath.Join(base, "Network_Toolkit.exe"),
		settings: filepath.Join(base, "settings"), desktop: filepath.Join(base, "Desktop"), menu: filepath.Join(base, "StartMenu"),
	}
	for _, dir := range []string{e.settings, e.desktop, e.menu} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(e.stable, []byte("already-verified-stable-executable"), 0600); err != nil {
		t.Fatal(err)
	}
	e.original = shortcutFixture([5]string{"Toolkit", "old target", base, "--profile test", "custom.ico"}, shortcutExtra(0x12345678, []byte("custom")))
	e.replacement = append([]byte(nil), e.original...)
	e.replacement[24]++
	return e
}

func (e shortcutTestEnvironment) write(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, e.original, 0600); err != nil {
		t.Fatal(err)
	}
}

func (e shortcutTestEnvironment) ops(matching map[string]bool, calls *[]string) updateShortcutOps {
	return updateShortcutOps{
		stage: func(source, stage, newPath string, oldPaths []string) (bool, error) {
			*calls = append(*calls, source)
			if !matching[source] {
				return false, nil
			}
			return true, os.WriteFile(stage, e.replacement, 0600)
		},
		commit: func(path string, identity os.FileInfo, before, after []byte) error {
			current, info, err := updateShortcutSnapshot(path)
			if err != nil || !os.SameFile(identity, info) || !bytes.Equal(current, before) {
				return errors.New("test commit identity mismatch")
			}
			return os.WriteFile(path, after, 0600)
		},
	}
}

func TestUpdateShortcutMigrationScopeAndBackups(t *testing.T) {
	e := newShortcutTestEnvironment(t)
	matching := map[string]bool{
		filepath.Join(e.desktop, "Toolkit.lnk"):               true,
		filepath.Join(e.menu, "Programs", "Toolkit.LNK"):      true,
		filepath.Join(e.desktop, "nested", "not-scanned.lnk"): true,
	}
	for path := range matching {
		e.write(t, path)
	}
	other := filepath.Join(e.desktop, "Other application.lnk")
	e.write(t, other)
	var calls []string
	err := updateMigrateShortcutsInRoots([]string{e.old}, e.stable, e.settings, []updateShortcutRoot{{e.desktop, 0}, {e.menu, 8}}, e.ops(matching, &calls))
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 {
		t.Fatalf("unexpected scan scope: %v", calls)
	}
	for path := range matching {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		want := e.replacement
		if strings.Contains(path, "not-scanned") {
			want = e.original
		}
		if !bytes.Equal(data, want) {
			t.Fatalf("unexpected shortcut content %s", path)
		}
	}
	data, _ := os.ReadFile(other)
	if !bytes.Equal(data, e.original) {
		t.Fatal("unrelated shortcut changed")
	}
	backups, err := os.ReadDir(filepath.Join(e.settings, "shortcut-backups"))
	if err != nil || len(backups) != 4 {
		t.Fatalf("missing original backups and path records: %v %v", backups, err)
	}
	for _, backup := range backups {
		if strings.HasSuffix(backup.Name(), ".lnk") {
			data, _ := os.ReadFile(filepath.Join(e.settings, "shortcut-backups", backup.Name()))
			if !bytes.Equal(data, e.original) {
				t.Fatal("backup did not retain original bytes")
			}
		}
	}
	children, _ := os.ReadDir(e.settings)
	if len(children) != 1 || children[0].Name() != "shortcut-backups" {
		t.Fatal("staging files were not cleaned")
	}
}

func TestUpdateShortcutMigrationRejectsContentAndIdentityRaces(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "content", true: "identity"}[replace], func(t *testing.T) {
			e := newShortcutTestEnvironment(t)
			path := filepath.Join(e.desktop, "Toolkit.lnk")
			e.write(t, path)
			committed := false
			ops := updateShortcutOps{
				stage: func(source, stage, newPath string, oldPaths []string) (bool, error) {
					if replace {
						if err := os.Rename(path, path+".moved"); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(path, e.original, 0600); err != nil {
							t.Fatal(err)
						}
					} else if err := os.WriteFile(path, []byte("user edit"), 0600); err != nil {
						t.Fatal(err)
					}
					return true, os.WriteFile(stage, e.replacement, 0600)
				},
				commit: func(string, os.FileInfo, []byte, []byte) error { committed = true; return nil },
			}
			err := updateMigrateShortcutsInRoots([]string{e.old}, e.stable, e.settings, []updateShortcutRoot{{e.desktop, 0}}, ops)
			if err == nil || committed {
				t.Fatalf("race was not rejected: %v", err)
			}
		})
	}
}

func TestUpdateShortcutMigrationReportsPartialFailure(t *testing.T) {
	e := newShortcutTestEnvironment(t)
	good := filepath.Join(e.desktop, "good.lnk")
	bad := filepath.Join(e.desktop, "bad.lnk")
	e.write(t, good)
	e.write(t, bad)
	var calls []string
	ops := e.ops(map[string]bool{good: true, bad: true}, &calls)
	originalStage := ops.stage
	ops.stage = func(source, stage, newPath string, oldPaths []string) (bool, error) {
		if source == bad {
			return false, errors.New("locked shortcut")
		}
		return originalStage(source, stage, newPath, oldPaths)
	}
	err := updateMigrateShortcutsInRoots([]string{e.old}, e.stable, e.settings, []updateShortcutRoot{{e.desktop, 0}}, ops)
	var partial *updateShortcutMigrationError
	if !errors.As(err, &partial) || partial.updated != 1 || partial.failed != 1 {
		t.Fatalf("missing partial result: %v", err)
	}
	for path, expected := range map[string][]byte{good: e.replacement, bad: e.original} {
		data, _ := os.ReadFile(path)
		if !bytes.Equal(data, expected) {
			t.Fatalf("unexpected result %s", path)
		}
	}
}

func TestUpdateShortcutMigrationDoesNotTraverseLinks(t *testing.T) {
	e := newShortcutTestEnvironment(t)
	outside := filepath.Join(t.TempDir(), "outside.lnk")
	e.write(t, outside)
	if err := os.Symlink(outside, filepath.Join(e.desktop, "linked.lnk")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if err := os.Symlink(filepath.Dir(outside), filepath.Join(e.menu, "linked-directory")); err != nil {
		t.Fatal(err)
	}
	var calls []string
	err := updateMigrateShortcutsInRoots([]string{e.old}, e.stable, e.settings, []updateShortcutRoot{{e.desktop, 0}, {e.menu, 8}}, e.ops(map[string]bool{}, &calls))
	if err == nil || len(calls) != 0 {
		t.Fatalf("link scope not rejected: %v, calls: %v", err, calls)
	}
	data, _ := os.ReadFile(outside)
	if !bytes.Equal(data, e.original) {
		t.Fatal("outside shortcut changed")
	}
}

func TestUpdateShortcutBackupCollisionFailsClosed(t *testing.T) {
	e := newShortcutTestEnvironment(t)
	path := filepath.Join(e.desktop, "Toolkit.lnk")
	e.write(t, path)
	backup := filepath.Join(e.settings, "shortcut-backups", updateHash([]byte(path))+"-"+updateHash(e.original)+".lnk")
	if err := os.Mkdir(filepath.Dir(backup), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, []byte("unrelated file"), 0600); err != nil {
		t.Fatal(err)
	}
	var calls []string
	err := updateMigrateShortcutsInRoots([]string{e.old}, e.stable, e.settings, []updateShortcutRoot{{e.desktop, 0}}, e.ops(map[string]bool{path: true}, &calls))
	if err == nil {
		t.Fatal("claimed unrelated backup")
	}
	data, _ := os.ReadFile(path)
	if !bytes.Equal(data, e.original) {
		t.Fatal("changed shortcut without verified backup")
	}
}

func TestUpdateShortcutMigrationRejectsUnscopedPaths(t *testing.T) {
	e := newShortcutTestEnvironment(t)
	for _, old := range []string{filepath.Join(filepath.Dir(e.old), "custom.exe"), filepath.Join(t.TempDir(), filepath.Base(e.old)), "relative.exe"} {
		if err := updateShortcutValidatePaths([]string{old}, e.stable, e.settings); err == nil {
			t.Fatalf("accepted unscoped path %s", old)
		}
	}
	if err := updateShortcutValidatePaths([]string{e.old}, e.old, e.settings); err == nil {
		t.Fatal("accepted unstable replacement")
	}
}

func TestUpdateShortcutMigrationBoundsStartMenuDepth(t *testing.T) {
	e := newShortcutTestEnvironment(t)
	deep := e.menu
	for i := 0; i < updateShortcutMaxDepth+1; i++ {
		deep = filepath.Join(deep, "nested")
	}
	path := filepath.Join(deep, "Toolkit.lnk")
	e.write(t, path)
	var calls []string
	err := updateMigrateShortcutsInRoots([]string{e.old}, e.stable, e.settings, []updateShortcutRoot{{e.menu, updateShortcutMaxDepth}}, e.ops(map[string]bool{path: true}, &calls))
	if err == nil || len(calls) != 0 || !strings.Contains(err.Error(), "nesting limit") {
		t.Fatalf("deep subtree was not bounded: %v %v", err, calls)
	}
}

func TestUpdateShortcutMigrationReportsEntryBudgetBeforeNextRoot(t *testing.T) {
	e := newShortcutTestEnvironment(t)
	for i := 0; i < updateShortcutScanLimit; i++ {
		if err := os.WriteFile(filepath.Join(e.desktop, fmt.Sprintf("file-%04d.txt", i)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(e.menu, "not-yet-scanned.lnk")
	e.write(t, path)
	var calls []string
	err := updateMigrateShortcutsInRoots([]string{e.old}, e.stable, e.settings, []updateShortcutRoot{{e.desktop, 0}, {e.menu, 8}}, e.ops(map[string]bool{path: true}, &calls))
	if err == nil || len(calls) != 0 || !strings.Contains(err.Error(), "remaining folders were not inspected") {
		t.Fatalf("unscanned second root was not reported: %v %v", err, calls)
	}
	data, _ := os.ReadFile(path)
	if !bytes.Equal(data, e.original) {
		t.Fatal("shortcut outside scan budget changed")
	}
}

func TestUpdateShortcutMigrationRejectsSerializerDataLoss(t *testing.T) {
	e := newShortcutTestEnvironment(t)
	path := filepath.Join(e.desktop, "Toolkit.lnk")
	e.write(t, path)
	e.replacement = shortcutFixture([5]string{"Toolkit", "new target", "", "", ""})
	var calls []string
	err := updateMigrateShortcutsInRoots([]string{e.old}, e.stable, e.settings, []updateShortcutRoot{{e.desktop, 0}}, e.ops(map[string]bool{path: true}, &calls))
	if err == nil {
		t.Fatal("accepted serializer data loss")
	}
	data, _ := os.ReadFile(path)
	if !bytes.Equal(data, e.original) {
		t.Fatal("original was not preserved")
	}
	if _, err := os.Stat(filepath.Join(e.settings, "shortcut-backups")); !os.IsNotExist(err) {
		t.Fatal("backed up or attempted write despite failed preservation check")
	}
}
