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
	Automatic bool `json:"automatic"`
	// State: pending (an update start that qualifies, waiting for the helper's
	// confirmation), running, added or failed.
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
	// Shortcuts: why Setup's Start menu shortcuts could not be pointed at the
	// separate programs (they keep working through Network_Toolkit.exe).
	Shortcuts string `json:"shortcuts,omitempty"`
	// StartMenu "next-setup": the installed uninstaller is older than the
	// separate programs, so the next Setup points the shortcuts at them.
	StartMenu string `json:"startMenu,omitempty"`
}

// companionPlatformSupported: the separate programs exist for the Windows x64
// release only (tests replace it).
var companionPlatformSupported = func() bool { return runtime.GOOS == "windows" && runtime.GOARCH == "amd64" }

// updateCompanionShortcuts points Setup's own Start menu shortcuts of the
// installed copy in dir at the separate programs (platform file; tests replace it).
var updateCompanionShortcuts = updateCompanionShortcutsNative

// companionsBlockLocked: while the separate programs are being added, every
// update action that uses the network is refused (the slot is reserved as for
// the manual button); while they are pending, the manual button only. Caller
// holds updater.mu.
func (a *application) companionsBlockLocked(path string) string {
	s := a.updater.companionsInstall
	switch {
	case s == nil:
	case s.State == "pending" && path == "/api/updates/companions":
		return "The separate Text Editor and Terminal programs will be added in a moment."
	case s.State == "running" && path != "/api/updates/open-releases":
		return "The separate Text Editor and Terminal programs are being added. Try again when that has finished."
	}
	return ""
}

// markCompanionsPending: from the start of a run that follows an update and
// lacks its separate programs, the Updates page shows that they will be added
// (and offers no button) until the install runs or does not.
func (a *application) markCompanionsPending() {
	if a.updateHealthDir == "" {
		return
	}
	a.updater.mu.Lock()
	defer a.updater.mu.Unlock()
	status := a.companionStatusLocked()
	a.updater.companions = nil // read again once the program's name is settled
	if a.updater.companionsInstall == nil && status != nil && status.Needed {
		a.updater.companionsInstall = &companionInstallState{Automatic: true, State: "pending"}
	}
}

// clearCompanionsPending ends a pending install that will not run.
func (a *application) clearCompanionsPending() {
	a.updater.mu.Lock()
	defer a.updater.mu.Unlock()
	a.clearCompanionsPendingLocked()
}

func (a *application) clearCompanionsPendingLocked() {
	if s := a.updater.companionsInstall; s != nil && s.State == "pending" {
		a.updater.companionsInstall = nil
	}
}

// installCompanionsAfterUpdate adds the separate programs in the start that
// follows an update the user approved and the helper confirmed. confirmed is
// awaitUpdateCommit's answer for this run's --update-health folder.
func (a *application) installCompanionsAfterUpdate(ctx context.Context, confirmed bool, client *http.Client, metadataURL string) {
	a.updater.mu.Lock()
	defer a.updater.mu.Unlock()
	defer a.clearCompanionsPendingLocked() // whatever did not start
	if a.updateHealthDir == "" || !confirmed {
		return
	}
	if s := a.updater.companionsInstall; s != nil && s.State != "pending" {
		return // once per update, whatever the result
	}
	status := a.companionStatusLocked()
	if status == nil || !status.Needed {
		return
	}
	// A check or a manual action that started first finishes first (a check
	// takes at most about 25 s).
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
	if s := a.updater.companionsInstall; s != nil && s.State != "pending" || a.updater.staged != nil {
		return
	}
	a.updater.companions = nil
	if status = a.companionStatusLocked(); status == nil || !status.Needed {
		return
	}
	state := &companionInstallState{Automatic: true, State: "running"}
	a.updater.companionsInstall = state
	// The network slot, as for the manual button: no check, download or
	// "Install with Setup" (fullInstallReserveNetwork) runs beside it.
	a.updater.networkBusy = true
	defer func() { a.updater.networkBusy = false }()
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
	switch {
	case errors.Is(err, errCompanionShortcutsNextSetup):
		state.StartMenu = "next-setup"
	case err != nil:
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

// companionShortcutUninstallerVersion is the first Setup whose uninstaller
// recognises and removes Start menu shortcuts that start the separate programs
// (IsOwnedShortcut with PROGRAM in windows.nsi; 1.21.46 has no such form).
const companionShortcutUninstallerVersion = "1.21.47"

// errCompanionShortcutsNextSetup: Setup's shortcuts were left for the next
// Setup to change, because the installed uninstaller is older than
// companionShortcutUninstallerVersion (or its version could not be read).
// Nothing is lost: Network_Toolkit.exe forwards to the separate programs.
var errCompanionShortcutsNextSetup = errors.New("The Start menu shortcuts will be pointed at the separate programs by the next Setup.")

type companionShortcutOps struct {
	// installLocation: InstallLocation of Setup's uninstall key (HKCU).
	installLocation func() (string, bool, error)
	// setupVersion: DisplayVersion of Setup's uninstall key, written only by
	// Setup (never by the in-app updater): the version of its uninstaller.
	setupVersion func() (string, bool, error)
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
	if location != "" {
		location = companionLongPath(location)
	}
	if !found || location == "" || !editorRegSamePath(location, companionLongPath(dir)) {
		return nil
	}
	menu, err := ops.programsFolder()
	if err != nil {
		return err
	}
	// The folder checks before a write resolve links and short names and
	// refuse a path that differs from what was resolved: use the long form.
	menu = companionLongPath(menu)
	main := editorRegJoin(location, mainProgramWindows)
	type owned struct{ name, link, target, args, program string }
	var failures []error
	var change []owned
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
		if !editorRegSamePath(companionLongPath(target), main) || !companionShortcutArgsOwned(args, s.owned) {
			continue
		}
		change = append(change, owned{s.link, link, target, args, editorRegJoin(location, s.program)})
	}
	if len(change) > 0 {
		// The installed uninstaller removes only the shortcut forms it knows.
		// An in-app update never replaces it, and only Setup writes
		// DisplayVersion: an uninstaller older than the separate programs would
		// leave retargeted shortcuts behind, so they wait for the next Setup.
		version, found, err := ops.setupVersion()
		if err != nil || !found || !updateVersionPattern.MatchString(version) || updateNewer(companionShortcutUninstallerVersion, version) {
			return errCompanionShortcutsNextSetup
		}
	}
	for _, c := range change {
		if err := ops.retarget(c.link, c.target, c.args, c.program); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", c.name, err))
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
	defer a.clearCompanionsPending()
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
