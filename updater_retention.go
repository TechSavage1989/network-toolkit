package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"networktoolkit/internal/model"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Each installation has its own rollback slot, even when several portable
// copies use one settings directory. Never claim an existing unrelated folder.
func updateStorageRoot(settingsDir, exeDir string) (string, error) {
	settingsDir, err := filepath.Abs(settingsDir)
	if err != nil {
		return "", err
	}
	if err = updateNamingCheckDirectory(settingsDir); err != nil {
		return "", err
	}
	exeDir, err = filepath.Abs(exeDir)
	if err != nil {
		return "", err
	}
	if err = updateNamingCheckDirectory(exeDir); err != nil {
		return "", err
	}
	key := exeDir
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	root := filepath.Join(settingsDir, "updates", updateHash([]byte(key)))
	for _, dir := range []string{filepath.Dir(root), root} {
		if err = os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
			return "", err
		}
		if err = updateNamingCheckDirectory(dir); err != nil {
			return "", err
		}
	}
	return root, nil
}

type retainedUpdate struct {
	Format           string `json:"format"`
	InstalledVersion string `json:"installedVersion"`
	SHA256           string `json:"sha256"`
	OriginalFilename string `json:"originalFilename,omitempty"`
	Result           string `json:"result,omitempty"`
	Progress         string `json:"progress,omitempty"`
}

const retainedUpdateFormat = "NetworkToolkitRollback/1"

type retentionTransaction struct {
	Previous retainedUpdate `json:"previous"`
	Next     retainedUpdate `json:"next"`
}

type completedUpdate struct {
	dir      string
	plan     updatePlan
	previous []byte
}

func inspectCompletedUpdate(dir, exeDir string) (completedUpdate, error) {
	var c completedUpdate
	p, err := updateReadPlan(dir)
	if err != nil {
		return c, err
	}
	if !updateNamingSamePath(filepath.Dir(updatePlanTarget(dir, p)), exeDir) {
		return c, errors.New("Update belongs to another application folder")
	}
	result, err := updateNamingReadFile(filepath.Join(dir, "update-result.txt"), 64<<10)
	if err != nil || !updateCommittedResult(string(result), p.Version) {
		return c, errors.New("Update is not committed")
	}
	ready, err := updateNamingReadFile(filepath.Join(dir, "ready"), 1024)
	if err != nil || string(ready) != p.Nonce {
		return c, errors.New("Update startup evidence is missing")
	}
	previous, err := updateNamingReadFile(filepath.Join(dir, "previous.exe"), updateMaxEXE)
	if (err != nil || updateHash(previous) != p.OriginalSHA256 || updateValidatePE(previous, "") != nil) && p.TargetDir != "" {
		// A healthy install may have committed while a cross-volume backup
		// transfer was blocked. Read its exact journaled same-volume backup;
		// do not touch it until the helper has released and retention commits.
		previous, err = updateNamingReadFile(filepath.Join(p.TargetDir, ".network-toolkit-swap-"+p.Nonce+"-previous.exe"), updateMaxEXE)
	}
	if err != nil || updateHash(previous) != p.OriginalSHA256 || updateValidatePE(previous, "") != nil {
		return c, errors.New("Previous application backup could not be verified")
	}
	return completedUpdate{dir, p, previous}, nil
}

// A mapped Windows executable cannot be deleted. Success here is also proof
// that the old helper has released its image; a committed result alone is not.
// Missing is accepted for resumption after an interrupted prior cleanup.
func releaseCompletedUpdateHelper(c completedUpdate) error {
	path := filepath.Join(c.dir, "installer.exe")
	b, err := updateNamingReadFile(path, updateMaxEXE)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || updateHash(b) != c.plan.OriginalSHA256 || updateValidatePE(b, "") != nil {
		return errors.New("Update helper could not be verified; cleanup is pending")
	}
	if err = os.Remove(path); err != nil {
		return fmt.Errorf("Update helper is still in use; cleanup is pending: %w", err)
	}
	return nil
}

func readRetainedUpdate(root string) (retainedUpdate, error) {
	var r retainedUpdate
	b, err := updateNamingReadFile(filepath.Join(root, "previous.json"), 32<<10)
	if err != nil {
		return r, err
	}
	if err = json.Unmarshal(b, &r); err != nil || r.Format != retainedUpdateFormat || !updateVersionPattern.MatchString(r.InstalledVersion) || len(r.SHA256) != 64 {
		return r, errors.New("Rollback metadata is invalid")
	}
	old, err := updateNamingReadFile(filepath.Join(root, "previous.exe"), updateMaxEXE)
	if err != nil || updateHash(old) != r.SHA256 || updateValidatePE(old, "") != nil {
		return r, errors.New("Rollback backup was changed; it has been preserved")
	}
	return r, nil
}

// Publish immutable content under its digest before switching the small
// rollback record. This avoids a cross-volume rename and never overwrites the
// only recoverable previous version. Once committed, old managed content goes.
func retainCompletedUpdate(root string, c completedUpdate) error {
	if err := recoverRetentionTransaction(root); err != nil {
		return err
	}
	record, err := readRetainedUpdate(root)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && updateNewer(record.InstalledVersion, c.plan.Version) {
		return nil
	}
	next := retainedUpdate{Format: retainedUpdateFormat, InstalledVersion: c.plan.Version, SHA256: c.plan.OriginalSHA256, OriginalFilename: c.plan.Target}
	if b, e := updateNamingReadFile(filepath.Join(c.dir, "update-result.txt"), 64<<10); e == nil {
		if len(b) > 2048 {
			b = b[len(b)-2048:]
		}
		next.Result = string(b)
	}
	if b, e := updateNamingReadFile(filepath.Join(c.dir, "update-progress.txt"), 64<<10); e == nil {
		if len(b) > 2048 {
			b = b[len(b)-2048:]
		}
		next.Progress = string(b)
	}
	nextData, _ := json.Marshal(next)
	oldPath := filepath.Join(root, "previous.exe")
	if err == nil && record.SHA256 == next.SHA256 {
		return writeRetentionMetadata(root, nextData)
	}
	// Use an explicit transaction journal so a crash never makes an older file
	// eligible for deletion merely because its filename looks managed.
	journal := filepath.Join(root, "retention-pending.json")
	pending := retentionTransaction{record, next}
	journalData, _ := json.Marshal(pending)
	if err = updateWriteExclusive(journal, journalData, 0600); err != nil {
		return errors.New("A previous rollback-file transaction needs recovery; no backup was removed")
	}
	temp := filepath.Join(root, "previous-next.exe")
	if err = updateWriteExclusive(temp, c.previous, 0700); err != nil {
		_ = os.Remove(journal)
		return err
	}
	if record.Format != "" {
		if err = moveUpdateNamingNoReplace(oldPath, filepath.Join(root, "previous-old.exe")); err != nil {
			_ = os.Remove(temp)
			_ = os.Remove(journal)
			return err
		}
	}
	if err = moveUpdateNamingNoReplace(temp, oldPath); err != nil {
		return err
	}
	if err = writeRetentionMetadata(root, nextData); err != nil {
		return err
	}
	if record.Format != "" {
		if err = removeRetentionHash(filepath.Join(root, "previous-old.exe"), record.SHA256); err != nil {
			return err
		}
	}
	return os.Remove(journal)
}

func writeRetentionMetadata(root string, data []byte) error {
	path := filepath.Join(root, "previous.json")
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return updateWriteExclusive(path, data, 0600)
	}
	temp, err := os.CreateTemp(root, ".rollback-record-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, err = temp.Write(data); err == nil {
		err = temp.Sync()
	}
	closeErr := temp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return replaceSettingsFile(name, path)
}
func removeRetentionHash(path, hash string) error {
	b, err := updateNamingReadFile(path, updateMaxEXE)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if updateHash(b) != hash {
		return errors.New("Managed update file changed; it was preserved")
	}
	return os.Remove(path)
}

func cleanupCompletedUpdate(c completedUpdate) error {
	if c.plan.TargetDir != "" {
		for name, hash := range map[string]string{"candidate": c.plan.CandidateSHA256, "previous": c.plan.OriginalSHA256, "failed": c.plan.CandidateSHA256} {
			path := filepath.Join(c.plan.TargetDir, ".network-toolkit-swap-"+c.plan.Nonce+"-"+name+".exe")
			if err := removeRetentionHash(path, hash); err != nil {
				return err
			}
		}
	}
	// Only exact members written by our updater are removed. Any unknown file,
	// directory, link, or changed binary is preserved, and rmdir will leave it.
	for name, hash := range map[string]string{"previous.exe": c.plan.OriginalSHA256, "candidate.exe": c.plan.CandidateSHA256, "failed.exe": c.plan.CandidateSHA256} {
		if err := removeRetentionHash(filepath.Join(c.dir, name), hash); err != nil {
			return err
		}
	}
	for _, name := range []string{"ready", "helper-ready", "update-progress.txt", "update-result.txt", "plan.json"} {
		path := filepath.Join(c.dir, name)
		if _, err := updateNamingReadFile(path, 64<<10); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return os.Remove(c.dir)
}

func (a *application) updateRetentionAfterHealthyStartup() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	root, err := updateStorageRoot(filepath.Dir(a.dataPath), filepath.Dir(exe))
	if err != nil {
		return err
	}
	// The old helper may still be returning immediately after its commit write.
	// Retry while the application remains alive, rather than permanently leaving
	// its mapped image and staging directory after this normal short race.
	deadline := time.Now().Add(15 * time.Second)
	for {
		if current, e := updateCurrentExecutable(); e == nil {
			exe = current
		}
		err = updateRetainAndName(root, exe, model.Version, a.updateHealthDir)
		if err == nil || !strings.Contains(err.Error(), "still in use") || time.Now().After(deadline) {
			return err
		}
		select {
		case <-a.ctx.Done():
			return a.ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func updateRetainAndName(root, exe, current, healthDir string) error {
	exeDir := filepath.Dir(exe)
	unlock, err := lockUpdateRetentionDirectory(root)
	if err != nil {
		return err
	}
	defer unlock()
	var completed []completedUpdate
	var pendingWarnings []string
	for _, parent := range []string{exeDir, root} {
		entries, err := os.ReadDir(parent)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), ".network-toolkit-update-") {
				continue
			}
			dir := filepath.Join(parent, entry.Name())
			c, err := inspectCompletedUpdate(dir, exeDir)
			if err != nil {
				if updateNamingSamePath(dir, healthDir) {
					return err
				}
				if updateManagedPendingStage(dir, exeDir) {
					pendingWarnings = append(pendingWarnings, "An unfinished or failed update is preserved for recovery: "+dir)
				}
				continue
			}
			if updateNewer(c.plan.Version, current) {
				continue
			}
			completed = append(completed, c)
		}
	}
	sort.Slice(completed, func(i, j int) bool { return updateNewer(completed[j].plan.Version, completed[i].plan.Version) })
	if len(completed) > 0 {
		latest := completed[len(completed)-1]
		if err = releaseCompletedUpdateHelper(latest); err != nil {
			return err
		}
		if err = retainCompletedUpdate(root, latest); err != nil {
			return err
		}
	}
	// Compact promotion never creates a second full launcher executable.
	status, err := updateStablePromote(root, exe, current, completed)
	if err != nil {
		status.Warning = err.Error()
	}
	updateNamingState.Lock()
	updateNamingState.status = status
	updateNamingState.Unlock()
	if err != nil {
		return err
	}
	// A renamed running image must reopen successfully before cleanup.
	if status.CanonicalPath != "" && !updateNamingSamePath(exe, status.CanonicalPath) {
		return nil
	}
	known := map[string]bool{}
	for _, c := range completed {
		known[c.plan.OriginalSHA256] = true
		known[c.plan.CandidateSHA256] = true
	}
	if p, _, e := updateNamingReadPointer(exeDir); e == nil {
		known[p.SHA256] = true
	}
	if data, e := updateNamingReadFile(filepath.Join(root, "stable-name-migration.json"), 64<<10); e == nil {
		var migration updateStableMigration
		if json.Unmarshal(data, &migration) == nil && migration.Format == "NetworkToolkitStableName/1" && migration.Version == current && migration.SHA256 == status.SHA256 {
			for _, hash := range migration.OldPaths {
				known[hash] = true
			}
		}
	}
	warnings := pendingWarnings
	pendingPath := filepath.Join(root, "pending-aliases.json")
	pending := map[string]string{}
	if data, e := updateNamingReadFile(pendingPath, 32<<10); e == nil {
		if e = json.Unmarshal(data, &pending); e != nil {
			return errors.New("Pending executable cleanup metadata is invalid; files were preserved")
		}
		for name, digest := range pending {
			if _, ok := updateOfficialExecutableVersion(name); !ok || len(digest) != 64 {
				return errors.New("Pending executable cleanup metadata contains an invalid file")
			}
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	for _, alias := range status.Compatibility {
		if b, e := updateNamingReadFile(alias, updateMaxEXE); e == nil && known[updateHash(b)] && updateValidatePE(b, "") == nil {
			pending[filepath.Base(alias)] = updateHash(b)
		}
	}
	if len(pending) > 0 {
		data, _ := json.Marshal(pending)
		if e := writeRetentionJSON(pendingPath, data); e != nil {
			return e
		}
	}
	for _, alias := range status.Compatibility {
		b, e := updateNamingReadFile(alias, updateMaxEXE)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil || (!known[updateHash(b)] && pending[filepath.Base(alias)] != updateHash(b)) || updateValidatePE(b, "") != nil {
			warnings = append(warnings, "An older executable could not be verified and was preserved: "+filepath.Base(alias))
			continue
		}
		if e = os.Remove(alias); e != nil {
			warnings = append(warnings, "An older executable is still in use; cleanup is pending: "+filepath.Base(alias))
		} else {
			delete(pending, filepath.Base(alias))
		}
	}
	for name := range pending {
		if _, e := os.Lstat(filepath.Join(exeDir, name)); os.IsNotExist(e) {
			delete(pending, name)
		}
	}
	if len(pending) == 0 {
		_ = os.Remove(pendingPath)
	} else {
		data, _ := json.Marshal(pending)
		if e := writeRetentionJSON(pendingPath, data); e != nil {
			return e
		}
	}
	for _, c := range completed {
		if e := releaseCompletedUpdateHelper(c); e != nil {
			warnings = append(warnings, e.Error())
			continue
		}
		if e := cleanupCompletedUpdate(c); e != nil {
			warnings = append(warnings, "Some update files were preserved: "+e.Error())
		}
	}
	remaining := []string{}
	for _, alias := range status.Compatibility {
		if _, e := os.Lstat(alias); !os.IsNotExist(e) {
			remaining = append(remaining, alias)
		}
	}
	status.Compatibility = remaining
	if e := updateStableFinish(root, exe, current, status); e != nil {
		warnings = append(warnings, e.Error())
	}
	status.Warning = strings.Join(warnings, "; ")
	updateNamingState.Lock()
	updateNamingState.status = status
	updateNamingState.Unlock()
	if status.Warning != "" {
		return errors.New(status.Warning)
	}
	return nil
}

func recoverRetentionTransaction(root string) error {
	journal := filepath.Join(root, "retention-pending.json")
	b, err := updateNamingReadFile(journal, 64<<10)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var tx retentionTransaction
	if err = json.Unmarshal(b, &tx); err != nil || tx.Next.Format != retainedUpdateFormat || !updateVersionPattern.MatchString(tx.Next.InstalledVersion) || len(tx.Next.SHA256) != 64 {
		return errors.New("Unrecognized rollback transaction; files were preserved")
	}
	if tx.Previous.Format != "" && (tx.Previous.Format != retainedUpdateFormat || !updateVersionPattern.MatchString(tx.Previous.InstalledVersion) || len(tx.Previous.SHA256) != 64) {
		return errors.New("Invalid previous rollback transaction")
	}
	current := filepath.Join(root, "previous.exe")
	next := filepath.Join(root, "previous-next.exe")
	old := filepath.Join(root, "previous-old.exe")
	matches := func(path, hash string) bool {
		data, e := updateNamingReadFile(path, updateMaxEXE)
		return e == nil && updateHash(data) == hash && updateValidatePE(data, "") == nil
	}
	absent := func(path string) bool { _, e := os.Lstat(path); return os.IsNotExist(e) }
	if !matches(current, tx.Next.SHA256) {
		if !matches(next, tx.Next.SHA256) {
			if absent(next) && absent(old) && (tx.Previous.Format == "" && absent(current) || matches(current, tx.Previous.SHA256)) {
				return os.Remove(journal)
			}
			return errors.New("Rollback recovery requires verified files; no files were removed")
		}
		if !absent(current) {
			if !matches(current, tx.Previous.SHA256) || !absent(old) {
				return errors.New("Rollback recovery collision; files were preserved")
			}
			if err = moveUpdateNamingNoReplace(current, old); err != nil {
				return err
			}
		}
		if err = moveUpdateNamingNoReplace(next, current); err != nil {
			return err
		}
	}
	data, _ := json.Marshal(tx.Next)
	if err = writeRetentionMetadata(root, data); err != nil {
		return err
	}
	if !absent(old) {
		if tx.Previous.Format == "" {
			return errors.New("Unrecognized older rollback file was preserved")
		}
		if err = removeRetentionHash(old, tx.Previous.SHA256); err != nil {
			return err
		}
	}
	if !absent(next) {
		if err = removeRetentionHash(next, tx.Next.SHA256); err != nil {
			return err
		}
	}
	return os.Remove(journal)
}

func writeRetentionJSON(path string, data []byte) error {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return updateWriteExclusive(path, data, 0600)
	}
	if _, err := updateNamingReadFile(path, 32<<10); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".retention-record-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return replaceSettingsFile(name, path)
}

// Failure diagnostics are not successful-install cleanup candidates. Report
// them explicitly so the UI never implies the one-backup target was achieved
// while a failed attempt still needs investigation or recovery.
func updateManagedPendingStage(dir, exeDir string) bool {
	p, err := updateReadPlan(dir)
	if err != nil || !updateNamingSamePath(filepath.Dir(updatePlanTarget(dir, p)), exeDir) {
		return false
	}
	for name, digest := range map[string]string{"installer.exe": p.OriginalSHA256, "previous.exe": p.OriginalSHA256, "candidate.exe": p.CandidateSHA256, "failed.exe": p.CandidateSHA256} {
		if b, e := updateNamingReadFile(filepath.Join(dir, name), updateMaxEXE); e == nil && updateHash(b) == digest && updateValidatePE(b, "") == nil {
			return true
		}
	}
	return false
}
