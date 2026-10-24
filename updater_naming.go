package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"networktoolkit/internal/model"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

const updateNamingPointerFile = ".network-toolkit-current.json"
const updateNamingPointerFormat = "NetworkToolkitLauncher/1"
const updateNamingForwardedEnv = "NETWORK_TOOLKIT_COMPATIBILITY_FORWARDED"
const updateNamingPointerLimit = 32 << 10

// Old official filenames remain executable compatibility launchers. Every one
// reads this single pointer, never another launcher or a user-supplied path.
// Aliases are display-only history, not alternative execution targets.
type updateNamingPointer struct {
	Format        string   `json:"format"`
	Executable    string   `json:"executable"`
	Version       string   `json:"version"`
	SHA256        string   `json:"sha256"`
	Compatibility []string `json:"compatibility,omitempty"`
}

type updateExecutableNamingStatus struct {
	Managed       bool
	CurrentPath   string
	CanonicalPath string
	Compatibility []string
	Warning       string
	SHA256        string
}

var updateNamingState struct {
	sync.RWMutex
	status updateExecutableNamingStatus
}
var updateNamingReadyOnce sync.Once

func updateOfficialExecutableName(version string) string {
	return "Network_Toolkit_" + version + "_Windows_x64.exe"
}

// Exact spelling is intentional: app.exe, custom suffixes, and user-renamed
// executables are never enrolled in versioned-name management.
func updateOfficialExecutableVersion(name string) (string, bool) {
	version := strings.TrimSuffix(strings.TrimPrefix(name, "Network_Toolkit_"), "_Windows_x64.exe")
	return version, updateVersionPattern.MatchString(version) && name == updateOfficialExecutableName(version)
}

func updateNamingSamePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func updateNamingCheckDirectory(dir string) error {
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return errors.New("The application folder must be an absolute, clean path")
	}
	if err := validateStoragePlatformPath(dir); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() || storageIsLink(info) {
		return errors.New("The application folder must not be a link or reparse point")
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil || !updateNamingSamePath(real, dir) {
		return errors.New("The application folder must not traverse a link or reparse point")
	}
	return nil
}

// Inspect before opening, then check the opened file's identity and the path
// again. Reject directories, links/reparse points, oversized files, and races.
func updateNamingReadFile(path string, limit int64) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || storageIsLink(before) || before.Size() > limit {
		return nil, errors.New("Launcher metadata and executable targets must be regular, bounded files, not links")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, errors.New("Launcher file changed while it was being opened")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	after, err := os.Lstat(path)
	if err != nil || storageIsLink(after) || !os.SameFile(opened, after) || int64(len(data)) > limit {
		return nil, errors.New("Launcher file changed while it was being read")
	}
	return data, nil
}

func updateNamingReadPointer(dir string) (updateNamingPointer, []byte, error) {
	var p updateNamingPointer
	data, err := updateNamingReadFile(filepath.Join(dir, updateNamingPointerFile), updateNamingPointerLimit)
	if err != nil {
		return p, nil, err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err = d.Decode(&p); err != nil {
		return p, data, errors.New("The current-version pointer is not valid launcher metadata")
	}
	if err = d.Decode(new(any)); err != io.EOF {
		return p, data, errors.New("The current-version pointer contains trailing data")
	}
	version, official := updateOfficialExecutableVersion(p.Executable)
	digest, digestErr := hex.DecodeString(p.SHA256)
	if p.Format != updateNamingPointerFormat || !official || version != p.Version || digestErr != nil || len(digest) != 32 || strings.ToLower(p.SHA256) != p.SHA256 || len(p.Compatibility) > 128 {
		return p, data, errors.New("The current-version pointer has an invalid filename, version, or checksum")
	}
	seen := map[string]bool{}
	for _, alias := range p.Compatibility {
		v, ok := updateOfficialExecutableVersion(alias)
		if !ok || !updateNewer(p.Version, v) || seen[alias] {
			return p, data, errors.New("The current-version pointer contains invalid compatibility filenames")
		}
		seen[alias] = true
	}
	return p, data, nil
}

func updateNamingValidateTarget(dir string, p updateNamingPointer) (string, error) {
	// Recheck even callers which constructed the pointer themselves. A manifest
	// can only name the canonical basename for exactly its recorded version.
	if !updateVersionPattern.MatchString(p.Version) || p.Executable != updateOfficialExecutableName(p.Version) {
		return "", errors.New("The launcher target is not an exact official versioned filename")
	}
	if err := updateNamingCheckDirectory(dir); err != nil {
		return "", err
	}
	target := filepath.Join(dir, p.Executable)
	data, err := updateNamingReadFile(target, updateMaxEXE)
	if err != nil {
		return "", fmt.Errorf("Cannot read the current executable: %w", err)
	}
	if updateHash(data) != p.SHA256 {
		return "", errors.New("The current executable does not match its recorded SHA256 checksum")
	}
	if err = updateValidatePE(data, p.Version); err != nil {
		return "", err
	}
	return target, nil
}

func updateNamingPrivateLaunch(args []string) bool {
	for _, arg := range args {
		key := strings.SplitN(arg, "=", 2)[0]
		switch key {
		case "--update-health", "-update-health", "--apply-update", "-apply-update", "--local-tools-helper", "-local-tools-helper", "--packet-capture-helper", "-packet-capture-helper", "--terminal-view", "-terminal-view":
			return true
		}
	}
	return false
}

// A health launch must stay at the replaced path: 1.17.58/59 installers only
// accept their nonce there. Naming is committed later, after genuine readiness.
// Return a warning only when this is already a correctly named, known binary;
// a broken pointer must never silently fall back to an outdated alias.
func updateNamingForwardTarget(exe, current string, args []string, forwarded bool) (target, warning string, err error) {
	base := filepath.Base(exe)
	if _, official := updateOfficialExecutableVersion(base); !official || updateNamingPrivateLaunch(args) {
		return "", "", nil
	}
	if !updateVersionPattern.MatchString(current) {
		return "", "", errors.New("Invalid running application version")
	}
	dir := filepath.Dir(exe)
	if err = updateNamingCheckDirectory(dir); err != nil {
		return "", "", err
	}
	p, _, err := updateNamingReadPointer(dir)
	if os.IsNotExist(err) {
		return "", "", nil // First updated launch; no pointer exists yet.
	}
	correctlyNamed := base == updateOfficialExecutableName(current)
	if err != nil {
		if correctlyNamed {
			return "", err.Error(), nil
		}
		return "", "", err
	}
	// A healthy new binary may have been copied over the previous canonical
	// filename. Never launch the pointer's older version or reject this upgrade
	// solely because its old hash describes the bytes which were just replaced.
	if updateNewer(current, p.Version) && p.Executable == base {
		return "", "", nil
	}
	target, err = updateNamingValidateTarget(dir, p)
	if err != nil {
		if correctlyNamed && updateNewer(current, p.Version) {
			return "", err.Error(), nil
		}
		return "", "", err
	}
	if updateNewer(current, p.Version) || updateNamingSamePath(target, exe) {
		return "", "", nil
	}
	if forwarded {
		return "", "", errors.New("The current version changed while reopening. Close this launcher and open the latest version directly")
	}
	return target, "", nil
}

func updateNamingForwardCommand(target string, args []string, cwd string, env []string) *exec.Cmd {
	cmd := exec.Command(target, args...)
	cmd.Dir = cwd
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	for _, value := range env {
		if key := strings.SplitN(value, "=", 2)[0]; !strings.EqualFold(key, updateNamingForwardedEnv) {
			cmd.Env = append(cmd.Env, value)
		}
	}
	cmd.Env = append(cmd.Env, updateNamingForwardedEnv+"=1")
	configureUpdateAppProcess(cmd)
	return cmd
}

// Stable-name builds never depend on an executable-side pointer. Legacy code
// remains solely for validating and migrating old, app-owned metadata.
func runUpdateCompatibilityLauncherIfRequested() bool { return false }

// Write complete bytes to a private same-folder temporary file first. The final
// no-replace move is atomic and cannot truncate an unrelated existing file.
func updateNamingWriteNewFile(path string, data []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".network-toolkit-name-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return moveUpdateNamingNoReplace(temp, path)
}

func updateNamingPublishPointer(dir string, p updateNamingPointer, prior []byte) error {
	encoded, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	path := filepath.Join(dir, updateNamingPointerFile)
	if prior == nil {
		return updateNamingWriteNewFile(path, encoded, 0600)
	}
	current, err := updateNamingReadFile(path, updateNamingPointerLimit)
	if err != nil || !bytes.Equal(current, prior) {
		return errors.New("The current-version pointer changed; it was not overwritten")
	}
	if bytes.Equal(current, encoded) {
		return nil
	}
	f, err := os.CreateTemp(dir, ".network-toolkit-pointer-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, err = f.Write(encoded); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	// All participating processes hold the same folder-specific lock. Recheck
	// immediately before atomic replacement, including nonparticipating edits.
	current, err = updateNamingReadFile(path, updateNamingPointerLimit)
	if err != nil || !bytes.Equal(current, prior) {
		return errors.New("The current-version pointer changed; it was not overwritten")
	}
	return replaceSettingsFile(temp, path)
}

func updateNamingPromote(exe, current string) (status updateExecutableNamingStatus, result error) {
	return updateNamingPromoteWithMode(exe, current, false, nil)
}

// Compact retention uses a rename, never another full compatibility executable.
func updateNamingPromoteCompact(exe, current string, proofs ...completedUpdate) (status updateExecutableNamingStatus, result error) {
	return updateNamingPromoteWithMode(exe, current, true, proofs)
}

func updateNamingPromoteWithMode(exe, current string, compact bool, proofs []completedUpdate) (status updateExecutableNamingStatus, result error) {
	status.CurrentPath = exe
	base := filepath.Base(exe)
	namedVersion, official := updateOfficialExecutableVersion(base)
	status.Managed = official
	if !official {
		return status, nil
	}
	if !updateVersionPattern.MatchString(current) || updateNewer(namedVersion, current) {
		return status, errors.New("The executable filename is newer than the running version; it was not renamed")
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
	p, prior, err := updateNamingReadPointer(dir)
	if err != nil && !os.IsNotExist(err) {
		return status, err // Never claim/overwrite an unrelated pointer file.
	}
	if err == nil {
		// The only exception to validating prior target bytes is the exact
		// canonical path just replaced with this successfully started upgrade.
		replacedPrior := p.Executable == base && p.Version == namedVersion && updateNewer(current, p.Version)
		if !replacedPrior {
			if _, err = updateNamingValidateTarget(dir, p); err != nil {
				// A crash between compact rename and pointer publication leaves
				// the old path absent and the running correctly named new image.
				// Do not apply this recovery to a changed or still-existing file.
				_, oldErr := os.Lstat(filepath.Join(dir, p.Executable))
				recoverRename := false
				if compact && base == updateOfficialExecutableName(current) && updateNewer(current, p.Version) && os.IsNotExist(oldErr) {
					self, readErr := updateNamingReadFile(exe, updateMaxEXE)
					if readErr == nil && updateValidatePE(self, current) == nil {
						for _, proof := range proofs {
							verified, e := inspectCompletedUpdate(proof.dir, dir)
							if e == nil && verified.plan.Version == current && verified.plan.Target == p.Executable && verified.plan.OriginalSHA256 == p.SHA256 && verified.plan.CandidateSHA256 == updateHash(self) {
								recoverRename = true
								break
							}
						}
					}
				}
				if !recoverRename {
					return status, fmt.Errorf("The previous current-version pointer was preserved: %w", err)
				}
			}
		}
		if updateNewer(p.Version, current) {
			status.CanonicalPath = filepath.Join(dir, p.Executable)
			for _, alias := range p.Compatibility {
				status.Compatibility = append(status.Compatibility, filepath.Join(dir, alias))
			}
			return status, nil // An older process must never roll back the pointer.
		}
	}
	self, err := updateNamingReadFile(exe, updateMaxEXE)
	if err != nil {
		return status, err
	}
	if err = updateValidatePE(self, current); err != nil {
		return status, err
	}
	canonical := filepath.Join(dir, updateOfficialExecutableName(current))
	desired := updateNamingPointer{Format: updateNamingPointerFormat, Executable: filepath.Base(canonical), Version: current, SHA256: updateHash(self)}
	if !updateNamingSamePath(exe, canonical) {
		if _, statErr := os.Lstat(canonical); os.IsNotExist(statErr) {
			if compact {
				err = moveUpdateNamingNoReplace(exe, canonical)
			} else {
				err = updateNamingWriteNewFile(canonical, self, 0700)
			}
			if err != nil && !os.IsExist(err) {
				return status, fmt.Errorf("Could not create the current version's filename: %w", err)
			}
		}
		if _, err = updateNamingValidateTarget(dir, desired); err != nil {
			return status, fmt.Errorf("The canonical filename already exists or could not be verified; it was not overwritten: %w", err)
		}
	}
	status.CanonicalPath = canonical
	status.SHA256 = updateHash(self)
	if compact {
		status.CurrentPath = canonical
	}
	aliases := map[string]bool{}
	for _, alias := range append(append([]string{}, p.Compatibility...), base, p.Executable) {
		if v, ok := updateOfficialExecutableVersion(alias); ok && updateNewer(current, v) {
			aliases[alias] = true
		}
	}
	for alias := range aliases {
		if compact {
			if _, e := os.Lstat(filepath.Join(dir, alias)); os.IsNotExist(e) {
				continue
			}
		}
		desired.Compatibility = append(desired.Compatibility, alias)
	}
	sort.Strings(desired.Compatibility)
	if len(desired.Compatibility) > 128 {
		return status, errors.New("Too many compatibility filenames; the previous pointer was preserved")
	}
	for _, alias := range desired.Compatibility {
		status.Compatibility = append(status.Compatibility, filepath.Join(dir, alias))
	}
	if err = updateNamingPublishPointer(dir, desired, prior); err != nil {
		return status, fmt.Errorf("The current executable is ready, but its compatibility pointer could not be saved: %w", err)
	}
	return status, nil
}

// Call only after the native UI is genuinely ready (or the explicitly headless
// service is ready), and after signaling the update helper's original-path
// health marker. Naming failure is a warning, never an update rollback trigger.
func recordUpdateNamingAfterHealthyStartup() (result error) {
	if !updateInstallSupported() {
		return nil
	}
	updateNamingReadyOnce.Do(func() {
		exe, err := os.Executable()
		status := updateExecutableNamingStatus{CurrentPath: exe}
		if err == nil {
			status, err = updateNamingPromote(exe, model.Version)
		}
		if err != nil {
			status.Warning = err.Error()
		}
		updateNamingState.Lock()
		updateNamingState.status = status
		updateNamingState.Unlock()
		result = err
	})
	return result
}

func updateNamingStatus() map[string]any {
	updateNamingState.RLock()
	status := updateNamingState.status
	status.Compatibility = append([]string{}, status.Compatibility...)
	updateNamingState.RUnlock()
	if status.CurrentPath == "" {
		status.CurrentPath, _ = os.Executable()
		_, status.Managed = updateOfficialExecutableVersion(filepath.Base(status.CurrentPath))
		status.Managed = status.Managed || filepath.Base(status.CurrentPath) == updateStableExecutableName
	}
	compatibilityNames := make([]string, 0, len(status.Compatibility))
	for _, path := range status.Compatibility {
		compatibilityNames = append(compatibilityNames, filepath.Base(path))
	}
	canonicalName := ""
	if status.CanonicalPath != "" {
		canonicalName = filepath.Base(status.CanonicalPath)
	}
	return map[string]any{
		"managedName": status.Managed, "currentPath": status.CurrentPath, "currentFilename": filepath.Base(status.CurrentPath),
		"canonicalPath": status.CanonicalPath, "canonicalFilename": canonicalName,
		"compatibilityPaths": status.Compatibility, "compatibilityFilenames": compatibilityNames, "warning": status.Warning,
	}
}

// The OS can continue reporting the original image pathname after a running
// image is renamed. Only use a promoted pathname recorded by this process and
// verified against the bytes it validated, never an arbitrary pointer target.
func updateCurrentExecutable() (string, error) {
	updateNamingState.RLock()
	status := updateNamingState.status
	updateNamingState.RUnlock()
	if status.CanonicalPath != "" && status.SHA256 != "" {
		b, err := updateNamingReadFile(status.CanonicalPath, updateMaxEXE)
		if err != nil {
			return "", err
		}
		if updateHash(b) != status.SHA256 || updateValidatePE(b, model.Version) != nil {
			return "", errors.New("The current executable changed after startup")
		}
		return status.CanonicalPath, nil
	}
	return os.Executable()
}
