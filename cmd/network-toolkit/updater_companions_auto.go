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
// confirmed: the document icon, the file types the installed uninstaller
// knows (text_editor_types.go), then the separate programs.
func (a *application) postUpdateRepairs(confirmed bool) {
	defer a.clearCompanionsPending()
	if a.updateHealthDir == "" || !confirmed || a.windowMode != "main" || buildEdition != "" || a.ctx.Err() != nil {
		return
	}
	if platform, ok := postUpdateEditorPlatform(); ok {
		if exe, err := postUpdateExecutable(); err == nil {
			if _, err := editorRegRepairDocumentIcon(platform, exe); err != nil {
				log.Print("Text Editor document icon: ", err)
			}
			added, err := refreshEditorTypes(platform, exe)
			if err != nil {
				log.Print("Text Editor file types: ", err)
			}
			a.editorTypesAdded.Store(int64(added))
		}
	}
	a.installCompanionsAfterUpdate(a.ctx, true, updateHTTPClient(), updateReleaseTagURL+model.Version)
}

// The registry and program path postUpdateRepairs uses (tests replace them).
var (
	postUpdateEditorPlatform = editorRegDefaultPlatform
	postUpdateExecutable     = updateCurrentExecutable
)

// editorRegEditorAppIconUninstallerVersion is the first Setup whose
// uninstaller takes back the document icon a registration writes into
// Windows' own entry of the separate Text Editor program
// (RemoveEditorApplicationIcon in windows.nsi). The start after an in-app
// update writes that icon for the installed program only when the installed
// uninstaller (DisplayVersion, written only by Setup) is at least this
// version; otherwise the next Setup writes it. It is the version that follows
// 1.21.51, the first to ship this; if that release gets another number, set
// it here in the same change.
const editorRegEditorAppIconUninstallerVersion = "1.21.52"

func editorRegUninstallerRemovesAppIcon(version string) bool {
	return updateVersionPattern.MatchString(version) && !updateNewer(editorRegEditorAppIconUninstallerVersion, version)
}

// editorRegRepairDocumentIcon gives the files of the running program's Text
// Editor registration the document icon (index 3), as Setup does on every
// install and upgrade (DOCUMENT_ICON). An in-app update does not run Setup, so
// a registration written before 1.21.45 kept the editor icon on the ProgID,
// and one written before 1.21.52 on its "Open with" entry. Three values, each
// only where this program's registration owns it (planEditorIconRepair);
// nothing of another program is touched. Every read comes first: a read that
// fails writes nothing.
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
	plan, err := planEditorIconRepair(p.registry, exe)
	if err != nil || len(plan) == 0 {
		return false, err
	}
	err = plan.apply(p.registry)
	// SHCNE_ASSOCCHANGED after any change, also a partial one.
	p.notify()
	return err == nil, err
}

// planEditorIconRepair computes the writes of editorRegRepairDocumentIcon:
//   - the ProgID's DefaultIcon, when the ProgID's command starts exe;
//   - the DefaultIcon of Applications\Network_Toolkit.exe and the program
//     icon on its open verb (shell\open, Icon), when its command is the Text
//     Editor command of exe, so Setup or the portable registration wrote it
//     and removes it with its other values (an entry Windows wrote itself,
//     with the bare file name, is nobody's to take back, so it is left alone).
//     Each only where it is absent or exactly what 1.21.44 to 1.21.51 wrote
//     (the editor icon as DefaultIcon, no verb Icon): an icon the user set
//     there stays;
//   - Windows' own entry of the separate Text Editor program beside exe
//     (planEditorAppIcon), recorded in the record of this program's
//     registration: Setup's, when exe is the installed program and the
//     installed uninstaller takes it back (editorRegUninstallerRemovesAppIcon),
//     or the portable record, when it names exe and nothing is installed.
func planEditorIconRepair(reg editorRegistry, exe string) (editorRegPlan, error) {
	var plan editorRegPlan
	// owned reads a key's command: the program it starts, when that is exe
	// and accept agrees.
	owned := func(key string, accept func(command string) bool) (string, error) {
		command, found, ok, err := reg.String(key+`\shell\open\command`, "")
		if err != nil || !found || !ok || !editorRegStartsWith(command, exe) || !accept(command) {
			return "", err
		}
		return editorRegCommandProgram(command), nil
	}
	// set adds want unless the value is already want. With keepOthers, only
	// where the value is absent or one of replaceable: anything else stays.
	set := func(key, name, want string, keepOthers bool, replaceable ...string) error {
		current, found, ok, err := reg.String(key, name)
		if err != nil || found && ok && current == want {
			return err
		}
		if found && keepOthers {
			mine := false
			for _, r := range replaceable {
				mine = mine || ok && current == r
			}
			if !mine {
				return nil
			}
		}
		plan.str(key, name, want)
		return nil
	}
	progID, err := editorRegExists(reg, editorRegProgKey)
	if err != nil {
		return nil, err
	}
	if progID {
		program, err := owned(editorRegProgKey, func(string) bool { return true })
		if err != nil {
			return nil, err
		}
		if program != "" {
			if err := set(editorRegProgKey+`\DefaultIcon`, "", editorRegIcon(program, editorRegDocumentIcon), false); err != nil {
				return nil, err
			}
		}
	}
	program, err := owned(editorRegApplication, func(command string) bool {
		return strings.EqualFold(command, editorRegCommand(editorRegCommandProgram(command)))
	})
	if err != nil {
		return nil, err
	}
	if program != "" {
		if err := set(editorRegApplication+`\DefaultIcon`, "", editorRegIcon(program, editorRegDocumentIcon), true, editorRegIcon(program, editorRegEditorIcon)); err != nil {
			return nil, err
		}
		// No released version wrote a verb icon: only an absent one is added.
		if err := set(editorRegApplication+`\shell\open`, "Icon", editorRegIcon(program, editorRegEditorIcon), true); err != nil {
			return nil, err
		}
	}
	location, found, ok, err := reg.String(editorRegUninstall, "InstallLocation")
	if err != nil {
		return nil, err
	}
	if found && ok && strings.TrimSpace(location) != "" {
		installed, version, err := editorRegInstalledHere(reg, exe)
		if err != nil || !installed || !editorRegUninstallerRemovesAppIcon(version) {
			return plan, err
		}
		record, err := editorRegExists(reg, editorRegSetupRecord)
		if err != nil || !record {
			return plan, err
		}
		// Setup's own path, as Setup writes DOCUMENT_ICON with $INSTDIR.
		err = planEditorAppIcon(reg, &plan, editorRegJoin(strings.TrimSpace(location), editorRegAppFile), editorRegSetupRecord)
		return plan, err
	}
	recorded, found, ok, err := reg.String(editorRegPortable, "Executable")
	if err != nil {
		return nil, err
	}
	if found && ok && editorRegSamePath(recorded, exe) {
		if err := planEditorAppIcon(reg, &plan, recorded, editorRegPortable); err != nil {
			return nil, err
		}
	}
	return plan, nil
}
