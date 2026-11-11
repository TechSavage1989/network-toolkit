package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"networktoolkit/internal/model"
)

// Adding the separate programs after an approved update (owner decision,
// 2026-11-11; docs/SEPARATE_EXECUTABLES.md, "Users on released versions").
//
// An update helper of 1.21.46 or older replaces only Network_Toolkit.exe. The
// new version then adds its separate Text Editor and Terminal programs by
// itself, in the background, in the start that follows the update: the user's
// approval of the update is the consent. The signal is the one every released
// helper gives: it starts the new program with --update-health <stage folder>
// and, once the new window reported that it started, writes its confirmation
// to update-result.txt in that folder (awaitUpdateCommit). Only a confirmed
// update counts, so an update that is still being put back never adds
// anything. A plain start (no --update-health) never downloads.
//
// It runs once per update: the --update-health start happens once, and the
// result is kept in this run's memory only (no saved state). A failure keeps
// the app running as before (Network_Toolkit.exe runs the apps itself) and
// leaves the manual button, with the reason; it is not retried.

// companionInstallState is the last install of the separate programs in this
// run: Automatic after an update, otherwise the manual button.
type companionInstallState struct {
	Automatic bool   `json:"automatic"`
	State     string `json:"state"` // running, added or failed
	Reason    string `json:"reason,omitempty"`
	// Shortcuts: why Setup's Start menu shortcuts could not be pointed at the
	// separate programs (they keep working through Network_Toolkit.exe).
	Shortcuts string `json:"shortcuts,omitempty"`
}

// companionPlatformSupported: the separate programs exist for the Windows x64
// release only (tests replace it).
var companionPlatformSupported = func() bool { return runtime.GOOS == "windows" && runtime.GOARCH == "amd64" }

// updateCompanionShortcuts points Setup's own Start menu shortcuts of the
// installed copy in dir at the separate programs (platform file; tests replace it).
var updateCompanionShortcuts = updateCompanionShortcutsNative

// companionsBlockLocked refuses every update action that downloads, stages or
// installs while the separate programs are being added. Caller holds updater.mu.
func (a *application) companionsBlockLocked(path string) string {
	if s := a.updater.companionsInstall; s == nil || s.State != "running" {
		return ""
	}
	switch path {
	case "/api/updates/check", "/api/updates/open-releases":
		return ""
	}
	return "The separate Text Editor and Terminal programs are being added. Try again when that has finished."
}

// installCompanionsAfterUpdate adds the separate programs in the start that
// follows an update the user approved and the helper confirmed. confirmed is
// awaitUpdateCommit's answer for this run's --update-health folder.
func (a *application) installCompanionsAfterUpdate(ctx context.Context, confirmed bool, client *http.Client, metadataURL string) {
	if a.updateHealthDir == "" || !confirmed {
		return
	}
	a.updater.mu.Lock()
	defer a.updater.mu.Unlock()
	if a.updater.companionsInstall != nil {
		return // once per update, whatever the result
	}
	status := a.companionStatusLocked()
	if status == nil || !status.Needed {
		return
	}
	// A check or a manual action that started first finishes first (a check
	// takes at most about 25 s); a manual install of the same programs makes
	// this one unnecessary.
	for deadline := time.Now().Add(40 * time.Second); a.updater.networkBusy || a.updater.installing; {
		if time.Now().After(deadline) || ctx.Err() != nil {
			return
		}
		a.updater.mu.Unlock()
		select {
		case <-ctx.Done():
		case <-time.After(200 * time.Millisecond):
		}
		a.updater.mu.Lock()
	}
	if a.updater.companionsInstall != nil || a.updater.staged != nil {
		return
	}
	a.updater.companions = nil
	if status = a.companionStatusLocked(); status == nil || !status.Needed {
		return
	}
	state := &companionInstallState{Automatic: true, State: "running"}
	a.updater.companionsInstall = state
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	err := a.updateInstallCompanionsFromRelease(ctx, client, metadataURL)
	a.updater.companions = nil
	a.updater.progress = nil
	if err != nil {
		state.State, state.Reason = "failed", err.Error()
		log.Print("Separate programs were not added after the update: ", err)
		return
	}
	state.State = "added"
	a.companionShortcutsLocked(state)
}

// companionShortcutsLocked points Setup's shortcuts at the separate programs
// just placed; a failure is reported, never fatal. Caller holds updater.mu.
func (a *application) companionShortcutsLocked(state *companionInstallState) {
	exe, err := updateCurrentExecutable()
	if err == nil {
		err = updateCompanionShortcuts(filepath.Dir(exe))
	}
	if err != nil {
		state.Shortcuts = err.Error()
		log.Print("Start menu shortcuts of the separate programs: ", err)
	}
}

// Setup's Start menu shortcuts of the Text Editor and the Terminal (windows.nsi:
// CreateStartShortcut, IsOwnedShortcut). owned are the arguments of the
// released form, which started Network_Toolkit.exe.
var companionShortcuts = []struct {
	link, program string
	owned         []string
}{
	{"Network Toolkit Text Editor.lnk", editorProgramWindows, []string{"--editor-standalone", "--editor"}},
	{"Network Toolkit Terminal.lnk", terminalProgramWindows, []string{"--terminal"}},
}

type companionShortcutOps struct {
	// installLocation: InstallLocation of Setup's uninstall key (HKCU).
	installLocation func() (string, bool, error)
	// programsFolder: this user's Start menu Programs folder ($SMPROGRAMS).
	programsFolder func() (string, error)
	// exists: a regular file (not a link) is at that path.
	exists func(link string) (bool, error)
	// read: the shortcut's exact target and arguments.
	read func(link string) (target, args string, err error)
	// retarget: start program, without arguments, with its icon 0; only when
	// the shortcut still has target and args.
	retarget func(link, target, args, program string) error
}

// updateRetargetCompanionShortcuts changes Setup's own shortcuts of the
// installed copy in dir only: target $INSTDIR\Network_Toolkit.exe with the
// released arguments, as IsOwnedShortcut decides. A portable copy, a shortcut
// that points elsewhere or that the user changed, is left alone.
func updateRetargetCompanionShortcuts(dir string, ops companionShortcutOps) error {
	location, found, err := ops.installLocation()
	if err != nil {
		return fmt.Errorf("Windows could not read the installed program's registration: %w", err)
	}
	location = strings.TrimSpace(location)
	if !found || location == "" || !editorRegSamePath(location, dir) {
		return nil
	}
	menu, err := ops.programsFolder()
	if err != nil {
		return err
	}
	main := editorRegJoin(location, mainProgramWindows)
	var failures []error
	for _, s := range companionShortcuts {
		link := filepath.Join(menu, "Network Toolkit", s.link)
		ok, err := ops.exists(link)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", s.link, err))
			continue
		}
		if !ok {
			continue
		}
		target, args, err := ops.read(link)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", s.link, err))
			continue
		}
		if !editorRegSamePath(target, main) || !companionShortcutArgsOwned(args, s.owned) {
			continue
		}
		if err := ops.retarget(link, target, args, editorRegJoin(location, s.program)); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", s.link, err))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("Start menu shortcuts still open the apps through Network_Toolkit.exe: %w", errors.Join(failures...))
	}
	return nil
}

func companionShortcutArgsOwned(args string, owned []string) bool {
	for _, o := range owned {
		if strings.EqualFold(args, o) { // NSIS compares strings without case
			return true
		}
	}
	return false
}

// postUpdateRepairs runs in the start that follows an update the helper
// confirmed: the separate programs, then the document icon.
func (a *application) postUpdateRepairs(confirmed bool) {
	if a.updateHealthDir == "" || !confirmed || a.windowMode != "main" || buildEdition != "" || a.ctx.Err() != nil {
		return
	}
	if platform, ok := editorRegDefaultPlatform(); ok {
		if exe, err := updateCurrentExecutable(); err == nil {
			if _, err := editorRegRepairDocumentIcon(platform, exe); err != nil {
				log.Print("Text Editor document icon: ", err)
			}
		}
	}
	a.installCompanionsAfterUpdate(a.ctx, true, updateHTTPClient(), updateReleaseTagURL+model.Version)
}

// editorRegRepairDocumentIcon points the ProgID's DefaultIcon at the document
// icon (index 3) when the ProgID's command starts exe, as Setup does on every
// install and upgrade (DOCUMENT_ICON). An in-app update does not run Setup, so
// a registration written before 1.21.45 kept the editor icon. Nothing else is
// written, and a ProgID of another program is never touched.
func editorRegRepairDocumentIcon(p editorRegPlatform, exe string) (bool, error) {
	if !strings.EqualFold(editorRegBase(exe), editorRegAppFile) {
		return false, nil
	}
	editorRegActionMu.Lock()
	defer editorRegActionMu.Unlock()
	release, err := p.lock()
	if err != nil {
		return false, err
	}
	defer release()
	exists, _, _, err := p.registry.KeyInfo(editorRegProgKey)
	if err != nil || !exists {
		return false, err
	}
	command, found, ok, err := p.registry.String(editorRegProgKey+`\shell\open\command`, "")
	if err != nil || !found || !ok || !editorRegStartsWith(command, exe) {
		return false, err
	}
	want := editorRegIcon(editorRegCommandProgram(command), editorRegDocumentIcon)
	current, found, ok, err := p.registry.String(editorRegProgKey+`\DefaultIcon`, "")
	if err != nil || found && ok && current == want {
		return false, err
	}
	if err := p.registry.SetString(editorRegProgKey+`\DefaultIcon`, "", want); err != nil {
		return false, err
	}
	p.notify()
	return true, nil
}
