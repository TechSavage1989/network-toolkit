package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const updateStableExecutableName = "Network_Toolkit.exe"

type updateStableMigration struct {
	Format   string            `json:"format"`
	Version  string            `json:"version"`
	SHA256   string            `json:"sha256"`
	OldPaths map[string]string `json:"oldPaths"`
	Pointer  []byte            `json:"pointer,omitempty"`
}

// Only official names are normalized. The journal belongs to this installation's
// settings-local update root, never the executable directory.
func updateStablePromote(root, exe, current string, completed []completedUpdate) (updateExecutableNamingStatus, error) {
	status := updateExecutableNamingStatus{CurrentPath: exe}
	base := filepath.Base(exe)
	named, official := updateOfficialExecutableVersion(base)
	status.Managed = official || base == updateStableExecutableName
	if !status.Managed {
		return status, nil
	}
	if !updateVersionPattern.MatchString(current) || (official && updateNewer(named, current)) {
		return status, errors.New("The executable filename is newer than this application")
	}
	dir := filepath.Dir(exe)
	if err := updateNamingCheckDirectory(dir); err != nil {
		return status, err
	}
	unlock, err := lockUpdateNamingDirectory(dir)
	if err != nil {
		return status, err
	}
	defer unlock()
	self, err := updateNamingReadFile(exe, updateMaxEXE)
	if err != nil {
		return status, err
	}
	if err = updateValidatePE(self, current); err != nil {
		return status, err
	}
	canonical := filepath.Join(dir, updateStableExecutableName)
	status.SHA256 = updateHash(self)
	path := filepath.Join(root, "stable-name-migration.json")
	migration := updateStableMigration{Format: "NetworkToolkitStableName/1", Version: current, SHA256: status.SHA256, OldPaths: map[string]string{}}
	if b, e := updateNamingReadFile(path, 64<<10); e == nil {
		if json.Unmarshal(b, &migration) != nil || migration.Format != "NetworkToolkitStableName/1" || !updateVersionPattern.MatchString(migration.Version) || len(migration.SHA256) != 64 || migration.OldPaths == nil {
			return status, errors.New("Stable-name migration metadata changed; files were preserved")
		}
		if migration.Version != current || migration.SHA256 != status.SHA256 {
			proved := false
			for _, c := range completed {
				if updateNewer(current, migration.Version) && c.plan.Version == current && c.plan.OriginalSHA256 == migration.SHA256 && c.plan.CandidateSHA256 == status.SHA256 {
					proved = true
					break
				}
			}
			if !proved {
				return status, errors.New("An unfinished naming migration does not match the verified update chain; files were preserved")
			}
			migration.Version = current
			migration.SHA256 = status.SHA256
		}
		for name, hash := range migration.OldPaths {
			if _, ok := updateOfficialExecutableVersion(name); !ok || len(hash) != 64 {
				return status, errors.New("Invalid stable-name migration path")
			}
		}
	} else if !os.IsNotExist(e) {
		return status, e
	}
	known := map[string]bool{status.SHA256: true}
	for _, hash := range migration.OldPaths {
		known[hash] = true
	}
	for _, c := range completed {
		known[c.plan.OriginalSHA256] = true
		known[c.plan.CandidateSHA256] = true
		if _, ok := updateOfficialExecutableVersion(c.plan.Target); ok {
			migration.OldPaths[c.plan.Target] = c.plan.OriginalSHA256
		}
		if _, ok := updateOfficialExecutableVersion(c.plan.LaunchTarget); ok {
			migration.OldPaths[c.plan.LaunchTarget] = c.plan.CandidateSHA256
		}
	}
	if data, e := updateNamingReadFile(filepath.Join(root, "pending-aliases.json"), 32<<10); e == nil {
		var pending map[string]string
		if json.Unmarshal(data, &pending) != nil {
			return status, errors.New("Pending alias record is invalid")
		}
		for name, hash := range pending {
			if _, ok := updateOfficialExecutableVersion(name); !ok || len(hash) != 64 {
				return status, errors.New("Invalid pending alias")
			}
			known[hash] = true
			migration.OldPaths[name] = hash
		}
	}
	if p, raw, e := updateNamingReadPointer(dir); e == nil {
		// The pointer itself is owned only when tied to verified current/previous bytes.
		if known[p.SHA256] || (len(migration.Pointer) > 0 && bytes.Equal(migration.Pointer, raw)) {
			migration.Pointer = raw
			for _, name := range append(p.Compatibility, p.Executable) {
				b, e := updateNamingReadFile(filepath.Join(dir, name), updateMaxEXE)
				if e == nil && known[updateHash(b)] && updateValidatePE(b, "") == nil {
					migration.OldPaths[name] = updateHash(b)
				}
			}
		} else {
			return status, errors.New("Legacy pointer ownership could not be verified; the file was preserved")
		}

	} else if !os.IsNotExist(e) {
		return status, fmt.Errorf("Legacy pointer preserved: %w", e)
	}
	if official {
		migration.OldPaths[base] = status.SHA256
	}
	if len(migration.OldPaths) > 0 || len(migration.Pointer) > 0 {
		encoded, _ := json.Marshal(migration)
		if err = writeRetentionJSON(path, encoded); err != nil {
			return status, err
		}
	}
	if !updateNamingSamePath(exe, canonical) {
		if _, e := os.Lstat(canonical); !os.IsNotExist(e) {
			return status, errors.New("Network_Toolkit.exe already exists; it was not overwritten. Move the other copy before retrying")
		}
		if err = moveUpdateNamingNoReplace(exe, canonical); err != nil {
			return status, err
		}
		status.CurrentPath = canonical
	}
	status.CanonicalPath = canonical
	for name := range migration.OldPaths {
		status.Compatibility = append(status.Compatibility, filepath.Join(dir, name))
	}
	// Reopen from the canonical image before touching shortcuts, old binaries or
	// pointer state. This avoids cleanup before a genuinely healthy stable launch.
	if !updateNamingSamePath(exe, canonical) {
		return status, nil
	}
	oldPaths := append([]string{}, status.Compatibility...)
	if len(oldPaths) > 0 {
		if err = updateMigrateShortcuts(oldPaths, canonical, root); err != nil {
			return status, err
		}
	}

	return status, nil
}

func updateStableFinish(root, exe, current string, status updateExecutableNamingStatus) error {
	if filepath.Base(exe) != updateStableExecutableName || len(status.Compatibility) > 0 {
		return nil
	}
	path := filepath.Join(root, "stable-name-migration.json")
	if _, err := updateNamingReadFile(path, 64<<10); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	b, err := updateNamingReadFile(path, 64<<10)
	if err != nil {
		return err
	}
	var migration updateStableMigration
	if json.Unmarshal(b, &migration) != nil || migration.Format != "NetworkToolkitStableName/1" || migration.Version != current || migration.SHA256 != status.SHA256 {
		return errors.New("Migration metadata changed; preserved")
	}
	dir := filepath.Dir(exe)
	if len(migration.Pointer) > 0 {
		old, err := updateNamingReadFile(filepath.Join(dir, updateNamingPointerFile), updateNamingPointerLimit)
		if err == nil {
			if !bytes.Equal(old, migration.Pointer) {
				return errors.New("Legacy pointer changed and was preserved")
			}
			if err = os.Remove(filepath.Join(dir, updateNamingPointerFile)); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return os.Remove(path)
}
