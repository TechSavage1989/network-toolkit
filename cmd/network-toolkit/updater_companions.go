package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"networktoolkit/internal/model"
)

// The separate Text Editor and Terminal programs beside Network_Toolkit.exe
// (docs/SEPARATE_EXECUTABLES.md, section 4). A version updated in-app by a
// released updater gets only Network_Toolkit.exe, which runs every app itself
// until the separate programs are there. The start that follows the update
// adds them by itself (updater_companions_auto.go); if that fails, the updater
// page offers to install them: both paths fetch this version's own Update ZIP,
// verify it and place both programs or neither.

const updateReleaseTagURL = "https://api.github.com/repos/" + updateRepository + "/releases/tags/v"

type companionStatus struct {
	Programs []companionProgram `json:"programs"`
	// Needed: at least one is missing or is not a program of this version.
	Needed bool `json:"needed"`
}

type companionProgram struct {
	Name  string `json:"name"`
	State string `json:"state"` // current, missing or other
}

func companionStatusFor(dir string) *companionStatus {
	status := &companionStatus{Programs: []companionProgram{}}
	for _, edition := range updateCompanionEditions {
		name := programNameFor("windows", edition)
		path := filepath.Join(dir, name)
		state := "current"
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			state = "missing"
		} else if !separateProgramMatches(path, edition) {
			state = "other"
		}
		status.Needed = status.Needed || state != "current"
		status.Programs = append(status.Programs, companionProgram{Name: name, State: state})
	}
	return status
}

// companionStatusLocked reports the separate programs where the main window
// installs updates (Windows x64); nil elsewhere. Caller holds updater.mu.
func (a *application) companionStatusLocked() *companionStatus {
	if !companionPlatformSupported() || a.windowMode != "main" || buildEdition != "" {
		return nil
	}
	if a.updater.companions == nil {
		exe, err := updateCurrentExecutable()
		if err != nil {
			return nil
		}
		a.updater.companions = companionStatusFor(filepath.Dir(exe))
	}
	return a.updater.companions
}

// handleUpdateCompanionsLocked installs this version's separate programs on
// the user's explicit request (the automatic install after an approved update
// is updater_companions_auto.go). Caller holds updater.mu.
func (a *application) handleUpdateCompanionsLocked(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != "/api/updates/companions" {
		return false
	}
	var request struct {
		Confirm bool   `json:"confirm"`
		Version string `json:"version"`
	}
	if e := decodeBody(w, r, 4096, &request); e != nil {
		jsonError(w, 400, e.Error())
		return true
	}
	if !request.Confirm || request.Version != model.Version {
		jsonError(w, 409, "Confirm installing the separate programs of version "+model.Version+".")
		return true
	}
	if status := a.companionStatusLocked(); status == nil || !status.Needed {
		jsonError(w, 409, "The separate Text Editor and Terminal programs of this version are already installed.")
		return true
	}
	a.updater.networkBusy = true
	defer func() { a.updater.networkBusy = false }()
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(6 * time.Minute))
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	a.updater.progress = &updateProgress{Phase: "checksums", Version: model.Version}
	finished := false
	defer func() {
		if !finished {
			a.updater.progress.Phase = "failed"
		}
	}()
	err := a.updateInstallCompanionsFromRelease(ctx, updateHTTPClient(), updateReleaseTagURL+model.Version)
	a.updater.companions = nil
	if err != nil {
		jsonError(w, 502, "The separate programs were not installed: "+err.Error())
		return true
	}
	finished = true
	a.updater.progress.Phase = "ready"
	state := &companionInstallState{State: "added"}
	a.companionShortcutsLocked(state)
	a.updater.companionsInstall = state
	jsonResponse(w, 200, a.updateStatusLocked())
	return true
}

// updateInstallCompanionsFromRelease downloads this version's release
// metadata, Update ZIP and SHA256SUMS through the trusted feed checks, and
// installs the separate programs when the package's Network_Toolkit.exe is
// byte-identical to the running one. Caller holds updater.mu.
func (a *application) updateInstallCompanionsFromRelease(ctx context.Context, client *http.Client, metadataURL string) error {
	metadata, err := a.updateFetchUnlocked(ctx, client, metadataURL, updateFeedMaxBody)
	if err != nil {
		return err
	}
	release, err := updateParseReleaseCandidate(metadata, true)
	if err != nil {
		return err
	}
	if release.Version != model.Version {
		return errors.New("The release feed did not return this version")
	}
	sums, err := a.updateFetchUnlocked(ctx, client, release.sumsURL, 64<<10)
	if err != nil {
		return err
	}
	a.updater.progress = &updateProgress{Phase: "download", Version: model.Version}
	archive, err := a.updateDownloadUnlocked(ctx, client, release.packageURL, updateMaxPackage, model.Version)
	if err != nil {
		return err
	}
	a.updater.progress.Phase = "verifying"
	a.updater.mu.Unlock()
	defer a.updater.mu.Lock()
	pkg, err := updateExtractRelease(release.packageName, archive, sums, model.Version, true)
	if err != nil {
		return err
	}
	if len(pkg.companions) == 0 {
		return errors.New("This version's release does not include the separate programs")
	}
	self, err := updateCurrentExecutable()
	if err != nil {
		return err
	}
	running, err := updateNamingReadFile(self, updateMaxEXE)
	if err != nil {
		return err
	}
	if updateHash(running) != updateHash(pkg.main) {
		return errors.New("this copy of " + filepath.Base(self) + " is not the official build of " + model.Version + ", so programs of that build are not added beside it")
	}
	return updateInstallCompanions(filepath.Dir(self), pkg.companions, replaceUpdateExecutable)
}

// updateInstallCompanions places verified separate programs beside the main
// program: both or neither. A missing program is created without replacing
// anything; a program of another build is replaced with ReplaceFileW (fails
// while it is open). A failure undoes every step already done.
func updateInstallCompanions(dir string, programs map[string][]byte, replace func(target, candidate, backup string) error) error {
	if err := updateNamingCheckDirectory(dir); err != nil {
		return err
	}
	unlock, err := lockUpdateNamingDirectory(dir)
	if err != nil {
		return err
	}
	defer unlock()
	type step struct {
		name, target, backup string
		replaced             bool
		hash, previous       string
	}
	nonce := randomHex(16)
	transaction := func(name, role string) string {
		return filepath.Join(dir, ".network-toolkit-companion-"+nonce+"-"+strings.TrimSuffix(name, ".exe")+"-"+role+".exe")
	}
	var done []step
	undo := func(cause error) error {
		var failures []string
		for i := len(done) - 1; i >= 0; i-- {
			s := done[i]
			if s.replaced {
				failed := transaction(s.name, "failed")
				if err := replace(s.target, s.backup, failed); err != nil {
					failures = append(failures, s.name+": "+err.Error())
					continue
				}
				_ = updateRemoveVerified(failed, s.hash)
			} else if err := updateRemoveVerified(s.target, s.hash); err != nil {
				failures = append(failures, s.name+": "+err.Error())
			}
		}
		if len(failures) > 0 {
			return fmt.Errorf("%v; undoing the earlier steps also failed (%s)", cause, strings.Join(failures, "; "))
		}
		return cause
	}
	for _, edition := range updateCompanionEditions {
		name := programNameFor("windows", edition)
		b, ok := programs[name]
		if !ok || len(b) == 0 {
			return undo(errors.New(name + " is missing from the package"))
		}
		target := filepath.Join(dir, name)
		hash := updateHash(b)
		existing, err := updateNamingReadFile(target, updateMaxEXE)
		switch {
		case os.IsNotExist(err):
			if err = updateNamingWriteNewFile(target, b, 0700); err != nil {
				return undo(fmt.Errorf("%s could not be created: %w", name, err))
			}
			done = append(done, step{name: name, target: target, hash: hash})
			if got, err := updateNamingReadFile(target, updateMaxEXE); err != nil || updateHash(got) != hash {
				return undo(errors.New(name + " could not be verified after it was written"))
			}
		case err != nil:
			return undo(fmt.Errorf("%s could not be read: %w", name, err))
		case updateHash(existing) == hash:
			// Already this build.
		default:
			candidate, backup := transaction(name, "candidate"), transaction(name, "previous")
			if err = updateNamingWriteNewFile(candidate, b, 0700); err != nil {
				return undo(err)
			}
			if err = replace(target, candidate, backup); err != nil {
				_ = updateRemoveVerified(candidate, hash)
				return undo(fmt.Errorf("%s could not be replaced (close it if it is open): %w", name, err))
			}
			done = append(done, step{name: name, target: target, backup: backup, replaced: true, hash: hash, previous: updateHash(existing)})
		}
	}
	for _, s := range done {
		if s.replaced {
			_ = updateRemoveVerified(s.backup, s.previous)
		}
	}
	return nil
}

// standaloneAppsOpen names the separate Text Editor and Terminal windows of
// this data root that are open: their programs are replaced by an update.
func standaloneAppsOpen(root string) []string {
	if root == "" {
		return nil
	}
	var open []string
	for _, mode := range []string{standaloneEditorMode, "terminal"} {
		lock := filepath.Join(peerWorkspaceDir(root, mode), "Network_Toolkit.window.lock")
		if info, err := os.Lstat(lock); err != nil || !info.Mode().IsRegular() {
			continue // never started from this data root: nothing to create here
		}
		release, err := acquireTerminalWindowLock(lock)
		if errors.Is(err, ErrTerminalWindowInUse) {
			open = append(open, editionTitle(mode)+" window")
			continue
		}
		if err == nil {
			release()
		}
	}
	return open
}
