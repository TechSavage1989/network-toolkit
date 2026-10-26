package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func retentionTestStage(t *testing.T, parent, exeDir, oldVersion, newVersion string) completedUpdate {
	t.Helper()
	dir, err := os.MkdirTemp(parent, ".network-toolkit-update-")
	if err != nil {
		t.Fatal(err)
	}
	old := updateTestPE(oldVersion)
	next := updateTestPE(newVersion)
	p := updatePlan{Target: updateOfficialExecutableName(oldVersion), WorkingDir: exeDir, OriginalSHA256: updateHash(old), CandidateSHA256: updateHash(next), Version: newVersion, ParentPID: 1, Nonce: strings.Repeat("a", 64)}
	if parent != exeDir {
		p.TargetDir = exeDir
	}
	b, _ := json.Marshal(p)
	for name, data := range map[string][]byte{"plan.json": b, "previous.exe": old, "installer.exe": old, "ready": []byte(p.Nonce), "helper-ready": []byte("ready"), "update-result.txt": []byte("Installed " + newVersion + " and confirmed the application interface started. Previous executable retained as previous.exe."), "update-progress.txt": []byte("complete")} {
		namingTestWrite(t, filepath.Join(dir, name), data)
	}
	return completedUpdate{dir, p, old}
}

// Simulate the required successful canonical reopen between promotion and cleanup.
func retentionTestRun(root, exe, current, health string) error {
	if err := updateRetainAndName(root, exe, current, health); err != nil {
		return err
	}
	if filepath.Base(exe) != updateStableExecutableName {
		return updateRetainAndName(root, filepath.Join(filepath.Dir(exe), updateStableExecutableName), current, health)
	}
	return nil
}
func TestUpdateRetentionSettingsRootAndIsolation(t *testing.T) {
	retentionTestResetState(t)
	settings := t.TempDir()
	first := t.TempDir()
	second := t.TempDir()
	a, err := updateStorageRoot(settings, first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := updateStorageRoot(settings, second)
	if err != nil {
		t.Fatal(err)
	}
	if a == b || filepath.Dir(filepath.Dir(a)) != settings {
		t.Fatalf("unexpected roots %s %s", a, b)
	}
	link := filepath.Join(t.TempDir(), "settings")
	if os.Symlink(settings, link) == nil {
		if _, err = updateStorageRoot(link, first); err == nil {
			t.Fatal("linked settings accepted")
		}
	}
}
func TestUpdateRetentionMigratesLegacyAndKeepsOnlyPrior(t *testing.T) {
	retentionTestResetState(t)
	exeDir := t.TempDir()
	root, err := updateStorageRoot(t.TempDir(), exeDir)
	if err != nil {
		t.Fatal(err)
	}
	older := retentionTestStage(t, exeDir, exeDir, "1.17.59", "1.17.60")
	latest := retentionTestStage(t, exeDir, exeDir, "1.17.60", "1.17.61")
	alias := filepath.Join(exeDir, updateOfficialExecutableName("1.17.59"))
	namingTestWrite(t, alias, updateTestPE("1.17.60"))
	exe := filepath.Join(exeDir, updateOfficialExecutableName("1.17.60"))
	namingTestWrite(t, exe, updateTestPE("1.17.61"))
	pointer := namingTestManifest("1.17.60", updateTestPE("1.17.60"))
	pointer.Compatibility = []string{filepath.Base(alias)}
	namingTestPointer(t, exeDir, pointer)
	unrelated := filepath.Join(exeDir, updateOfficialExecutableName("1.17.57"))
	namingTestWrite(t, unrelated, updateTestPE("1.17.57"))
	if err = retentionTestRun(root, exe, "1.17.61", latest.dir); err != nil {
		t.Fatal(err)
	}
	if b := namingTestBytes(t, filepath.Join(root, "previous.exe")); updateHash(b) != latest.plan.OriginalSHA256 {
		t.Fatal("wrong previous version")
	}
	record, e := readRetainedUpdate(root)
	if e != nil || !strings.Contains(record.Result, "Installed 1.17.61") || record.Progress != "complete" || record.OriginalFilename != latest.plan.Target {
		t.Fatalf("retained diagnostics missing: %#v %v", record, e)
	}

	for _, path := range []string{alias, exe, older.dir, latest.dir} {
		if _, err = os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("old managed asset retained: %s %v", path, err)
		}
	}
	if _, err = os.Stat(unrelated); err != nil {
		t.Fatal("manual copy deleted")
	}
	canonical := filepath.Join(exeDir, updateStableExecutableName)
	if updateHash(namingTestBytes(t, canonical)) != latest.plan.CandidateSHA256 {
		t.Fatal("current damaged")
	}
	// A later successful upgrade rotates the single rollback slot.
	next := retentionTestStage(t, root, exeDir, "1.17.61", "1.17.62")
	namingTestWrite(t, canonical, updateTestPE("1.17.62"))
	if err = retentionTestRun(root, canonical, "1.17.62", next.dir); err != nil {
		t.Fatal(err)
	}
	if updateHash(namingTestBytes(t, filepath.Join(root, "previous.exe"))) != next.plan.OriginalSHA256 {
		t.Fatal("backup was not rotated")
	}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if e.Name() != "previous.exe" && e.Name() != "previous.json" {
			t.Fatalf("retention leftovers %s", e.Name())
		}
	}
}
func TestUpdateRetentionRejectsUncommittedChangedAndForeignStages(t *testing.T) {
	retentionTestResetState(t)
	for _, kind := range []string{"uncommitted", "wrong-nonce", "changed-backup", "changed-helper", "foreign"} {
		t.Run(kind, func(t *testing.T) {
			exeDir := t.TempDir()
			root, _ := updateStorageRoot(t.TempDir(), exeDir)
			c := retentionTestStage(t, exeDir, exeDir, "1.17.60", "1.17.61")
			exe := filepath.Join(exeDir, updateOfficialExecutableName("1.17.60"))
			namingTestWrite(t, exe, updateTestPE("1.17.61"))
			switch kind {
			case "uncommitted":
				os.Remove(filepath.Join(c.dir, "update-result.txt"))
			case "wrong-nonce":
				namingTestWrite(t, filepath.Join(c.dir, "ready"), []byte("wrong"))
			case "changed-backup":
				namingTestWrite(t, filepath.Join(c.dir, "previous.exe"), []byte("manual"))
			case "changed-helper":
				namingTestWrite(t, filepath.Join(c.dir, "installer.exe"), []byte("manual"))
			case "foreign":
				c.plan.TargetDir = t.TempDir()
				b, _ := json.Marshal(c.plan)
				namingTestWrite(t, filepath.Join(c.dir, "plan.json"), b)
			}
			if err := retentionTestRun(root, exe, "1.17.61", c.dir); err == nil {
				t.Fatal("unsafe migration accepted")
			}
			if _, err := os.Stat(filepath.Join(c.dir, "previous.exe")); err != nil {
				t.Fatal("unverified backup deleted")
			}
			if _, err := os.Stat(exe); err != nil {
				t.Fatal("current renamed before commit/helper proof")
			}
		})
	}
}
func TestUpdateRetentionPreservesModifiedAliasAndUnknownMembers(t *testing.T) {
	retentionTestResetState(t)
	exeDir := t.TempDir()
	root, _ := updateStorageRoot(t.TempDir(), exeDir)
	c := retentionTestStage(t, exeDir, exeDir, "1.17.60", "1.17.61")
	exe := filepath.Join(exeDir, updateOfficialExecutableName("1.17.60"))
	namingTestWrite(t, exe, updateTestPE("1.17.61"))
	alias := filepath.Join(exeDir, updateOfficialExecutableName("1.17.59"))
	namingTestWrite(t, alias, []byte("manual replacement"))
	p := namingTestManifest("1.17.60", updateTestPE("1.17.60"))
	p.Compatibility = []string{filepath.Base(alias)}
	namingTestPointer(t, exeDir, p)
	unknown := filepath.Join(c.dir, "my-document.txt")
	namingTestWrite(t, unknown, []byte("keep"))
	if err := retentionTestRun(root, exe, "1.17.61", c.dir); err == nil {
		t.Fatal("preservation warning missing")
	}
	if string(namingTestBytes(t, alias)) != "manual replacement" || string(namingTestBytes(t, unknown)) != "keep" {
		t.Fatal("unowned data changed")
	}
}
func TestUpdateRetentionTransactionRecovery(t *testing.T) {
	retentionTestResetState(t)
	for _, phase := range []string{"journal", "copy", "old-moved", "new-moved", "recorded"} {
		t.Run(phase, func(t *testing.T) {
			root := t.TempDir()
			old := updateTestPE("1.17.59")
			next := updateTestPE("1.17.60")
			before := retainedUpdate{Format: retainedUpdateFormat, InstalledVersion: "1.17.60", SHA256: updateHash(old)}
			after := retainedUpdate{Format: retainedUpdateFormat, InstalledVersion: "1.17.61", SHA256: updateHash(next)}
			b, _ := json.Marshal(before)
			namingTestWrite(t, filepath.Join(root, "previous.json"), b)
			namingTestWrite(t, filepath.Join(root, "previous.exe"), old)
			b, _ = json.Marshal(retentionTransaction{before, after})
			namingTestWrite(t, filepath.Join(root, "retention-pending.json"), b)
			if phase != "journal" {
				namingTestWrite(t, filepath.Join(root, "previous-next.exe"), next)
			}
			if phase == "old-moved" || phase == "new-moved" || phase == "recorded" {
				os.Rename(filepath.Join(root, "previous.exe"), filepath.Join(root, "previous-old.exe"))
			}
			if phase == "new-moved" || phase == "recorded" {
				os.Rename(filepath.Join(root, "previous-next.exe"), filepath.Join(root, "previous.exe"))
			}
			if phase == "recorded" {
				b, _ = json.Marshal(after)
				namingTestWrite(t, filepath.Join(root, "previous.json"), b)
			}
			if err := recoverRetentionTransaction(root); err != nil {
				t.Fatal(err)
			}
			want := after
			if phase == "journal" {
				want = before
			}
			record, err := readRetainedUpdate(root)
			if err != nil || record != want {
				t.Fatalf("recovery %v %#v", err, record)
			}
			entries, _ := os.ReadDir(root)
			if len(entries) != 2 {
				t.Fatalf("transaction leftovers %v", entries)
			}
		})
	}
}
func TestUpdateNamingCompactDoesNotDuplicateBinary(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, updateOfficialExecutableName("1.17.60"))
	namingTestWrite(t, old, updateTestPE("1.17.61"))
	status, err := updateNamingPromoteCompact(old, "1.17.61")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("full compatibility copy retained")
	}
	if status.CurrentPath != status.CanonicalPath || len(status.Compatibility) != 0 {
		t.Fatalf("status %#v", status)
	}
}

func TestUpdateRetentionRecoversDeferredCrossVolumeBackup(t *testing.T) {
	retentionTestResetState(t)
	exeDir := t.TempDir()
	root, _ := updateStorageRoot(t.TempDir(), exeDir)
	c := retentionTestStage(t, root, exeDir, "1.17.60", "1.17.61")
	temp := filepath.Join(exeDir, ".network-toolkit-swap-"+c.plan.Nonce+"-previous.exe")
	if err := os.Rename(filepath.Join(c.dir, "previous.exe"), temp); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(exeDir, updateOfficialExecutableName("1.17.61"))
	namingTestWrite(t, exe, updateTestPE("1.17.61"))
	if err := retentionTestRun(root, exe, "1.17.61", c.dir); err != nil {
		t.Fatal(err)
	}
	if updateHash(namingTestBytes(t, filepath.Join(root, "previous.exe"))) != c.plan.OriginalSHA256 {
		t.Fatal("deferred backup lost")
	}
	if _, err := os.Stat(temp); !os.IsNotExist(err) {
		t.Fatal("transient backup not removed")
	}
}
func TestUpdateRetentionResumesPendingAliasWithoutOldStage(t *testing.T) {
	retentionTestResetState(t)
	exeDir := t.TempDir()
	root, _ := updateStorageRoot(t.TempDir(), exeDir)
	exe := filepath.Join(exeDir, updateOfficialExecutableName("1.17.61"))
	namingTestWrite(t, exe, updateTestPE("1.17.61"))
	alias := filepath.Join(exeDir, updateOfficialExecutableName("1.17.59"))
	old := updateTestPE("1.17.60")
	namingTestWrite(t, alias, old)
	p := namingTestManifest("1.17.61", updateTestPE("1.17.61"))
	p.Compatibility = []string{filepath.Base(alias)}
	namingTestPointer(t, exeDir, p)
	data, _ := json.Marshal(map[string]string{filepath.Base(alias): updateHash(old)})
	namingTestWrite(t, filepath.Join(root, "pending-aliases.json"), data)
	if err := retentionTestRun(root, exe, "1.17.61", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(alias); !os.IsNotExist(err) {
		t.Fatal("pending alias not removed")
	}
	if _, err := os.Stat(filepath.Join(root, "pending-aliases.json")); !os.IsNotExist(err) {
		t.Fatal("completed cleanup metadata retained")
	}
}
func TestUpdateNamingRecoversCompactRenameBeforePointer(t *testing.T) {
	dir := t.TempDir()
	old := updateOfficialExecutableName("1.17.60")
	p := namingTestManifest("1.17.60", updateTestPE("1.17.60"))
	namingTestPointer(t, dir, p)
	current := filepath.Join(dir, updateOfficialExecutableName("1.17.61"))
	namingTestWrite(t, current, updateTestPE("1.17.61"))
	proof := retentionTestStage(t, dir, dir, "1.17.60", "1.17.61")
	status, err := updateNamingPromoteCompact(current, "1.17.61", proof)
	if err != nil {
		t.Fatal(err)
	}
	if status.CanonicalPath != current {
		t.Fatal("compact recovery failed")
	}
	if _, err := os.Stat(filepath.Join(dir, old)); !os.IsNotExist(err) {
		t.Fatal("old binary recreated")
	}
}

func TestUpdateRetentionModernLaunchAlreadyRenamed(t *testing.T) {
	retentionTestResetState(t)
	exeDir := t.TempDir()
	root, _ := updateStorageRoot(t.TempDir(), exeDir)
	c := retentionTestStage(t, root, exeDir, "1.17.61", "1.17.62")
	c.plan.LaunchTarget = updateOfficialExecutableName("1.17.62")
	data, _ := json.Marshal(c.plan)
	namingTestWrite(t, filepath.Join(c.dir, "plan.json"), data)
	namingTestPointer(t, exeDir, namingTestManifest("1.17.61", updateTestPE("1.17.61")))
	current := filepath.Join(exeDir, c.plan.LaunchTarget)
	namingTestWrite(t, current, updateTestPE("1.17.62"))
	if err := retentionTestRun(root, current, "1.17.62", c.dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(exeDir, updateNamingPointerFile)); !os.IsNotExist(err) {
		t.Fatalf("legacy pointer retained: %v", err)
	}
	if updateHash(namingTestBytes(t, filepath.Join(root, "previous.exe"))) != c.plan.OriginalSHA256 {
		t.Fatal("wrong rollback")
	}
}
func TestUpdateNamingMissingPointerTargetWithoutCommitPreserved(t *testing.T) {
	dir := t.TempDir()
	prior := namingTestPointer(t, dir, namingTestManifest("1.17.60", updateTestPE("1.17.60")))
	current := filepath.Join(dir, updateOfficialExecutableName("1.17.61"))
	namingTestWrite(t, current, updateTestPE("1.17.61"))
	if _, err := updateNamingPromoteCompact(current, "1.17.61"); err == nil {
		t.Fatal("missing prior target without verified commit accepted")
	}
	if string(namingTestBytes(t, filepath.Join(dir, updateNamingPointerFile))) != string(prior) {
		t.Fatal("unproven pointer overwritten")
	}
}

func TestUpdateRetentionRecoversSwapDespitePartialStageBackup(t *testing.T) {
	retentionTestResetState(t)
	exeDir := t.TempDir()
	root, _ := updateStorageRoot(t.TempDir(), exeDir)
	c := retentionTestStage(t, root, exeDir, "1.17.60", "1.17.61")
	namingTestWrite(t, filepath.Join(exeDir, ".network-toolkit-swap-"+c.plan.Nonce+"-previous.exe"), c.previous)
	partial := filepath.Join(c.dir, "previous.exe")
	namingTestWrite(t, partial, []byte("partial transfer"))
	exe := filepath.Join(exeDir, updateOfficialExecutableName("1.17.61"))
	namingTestWrite(t, exe, updateTestPE("1.17.61"))
	if err := retentionTestRun(root, exe, "1.17.61", c.dir); err == nil {
		t.Fatal("preservation warning expected")
	}
	if updateHash(namingTestBytes(t, filepath.Join(root, "previous.exe"))) != c.plan.OriginalSHA256 {
		t.Fatal("verified rollback not retained")
	}
	if string(namingTestBytes(t, partial)) != "partial transfer" {
		t.Fatal("unverified partial file removed")
	}
}
func TestUpdateRetentionReportsFailedAttempts(t *testing.T) {
	retentionTestResetState(t)
	exeDir := t.TempDir()
	root, _ := updateStorageRoot(t.TempDir(), exeDir)
	c := retentionTestStage(t, root, exeDir, "1.17.60", "1.17.61")
	namingTestWrite(t, filepath.Join(c.dir, "update-result.txt"), []byte("Rollback completed"))
	exe := filepath.Join(exeDir, updateOfficialExecutableName("1.17.60"))
	namingTestWrite(t, exe, updateTestPE("1.17.60"))
	if err := retentionTestRun(root, exe, "1.17.60", ""); err == nil || !strings.Contains(err.Error(), "unfinished or failed") {
		t.Fatalf("failed attempt not surfaced: %v", err)
	}
	if _, err := os.Stat(filepath.Join(c.dir, "previous.exe")); err != nil {
		t.Fatal("failed-attempt backup removed")
	}
}

func retentionTestResetState(t *testing.T) {
	t.Helper()
	updateNamingState.Lock()
	before := updateNamingState.status
	updateNamingState.status = updateExecutableNamingStatus{}
	updateNamingState.Unlock()
	t.Cleanup(func() { updateNamingState.Lock(); updateNamingState.status = before; updateNamingState.Unlock() })
}
