package main

import (
	"errors"
	"fmt"
	"strings"
)

// The file types Windows offers the installed Text Editor for, after an in-app
// update (owner request, 2026-11-12).
//
// The installed copy's registrations come from the Setup that installed it:
// Setup up to 1.21.43 registered five types, 1.21.44 and later every type of
// scripts/installer/text-extensions.nsh, and an in-app update replaces only
// the programs. So a copy installed long ago and updated in the app offers the
// Text Editor for .txt but not, for example, for .json.
//
//   - When the installed uninstaller records the keys Setup creates and
//     removes every type of the current list (DisplayVersion of Setup's
//     uninstall key, written only by Setup, is a strict x.y.z of at least
//     editorRegTypesUninstallerVersion), the start that follows an update the
//     user approved adds the missing types itself, exactly as Setup's
//     RegisterExtension writes them, recording created keys in Setup's own
//     record (refreshEditorTypes, called from postUpdateRepairs).
//   - Otherwise nothing is written: an older uninstaller would leave what was
//     added behind. App settings shows a notice instead (setupNeeded) with a
//     button that downloads and starts this version's verified Setup through
//     "Install the full version" (full_install.go), and the page then closes
//     the app through its normal close, so that Setup can replace the programs.
//
// Never a UserChoice, nothing under FileExts, never the default value of an
// extension, never a key another program wrote: only the ProgID as a value of
// <ext>\OpenWithProgids and the type in Capabilities\FileAssociations, as Setup.

// editorRegTypesUninstallerVersion is the first Setup whose uninstaller
// records the keys it created (EDITOR_KEY\CreatedKeys) and removes every type
// of text-extensions.nsh as the list is today (1.21.44 added both; the list
// has not changed since). A type added to the list later is removed only by
// an uninstaller of the version that first lists it: raise this constant to
// that version in the same change (TestEditorTypesUninstallerKnowsTheList
// fails until then).
const editorRegTypesUninstallerVersion = "1.21.44"

// Setup's record of the keys it created (CREATED_KEYS in windows.nsi).
const editorRegSetupKeys = editorRegSetupRecord + `\CreatedKeys`

// editorRegTypesStatus is what App settings shows about the types of the
// installed program; only the installed program (the running copy is the one
// in the uninstall key's InstallLocation) reports it.
type editorRegTypesStatus struct {
	Total   int `json:"total"`
	Missing int `json:"missing"`
	// SetupVersion is the installed uninstaller's version (DisplayVersion).
	SetupVersion string `json:"setupVersion,omitempty"`
	// UninstallerRecords: that uninstaller records and removes every type, so
	// the start after an approved update adds missing types itself.
	UninstallerRecords bool `json:"uninstallerRecords"`
	// SetupNeeded: types are missing; App settings offers to run Setup.
	SetupNeeded bool `json:"setupNeeded"`
	// Added: types this run added after the update.
	Added int `json:"added,omitempty"`
}

func editorRegUninstallerRecords(version string) bool {
	return updateVersionPattern.MatchString(version) && !updateNewer(editorRegTypesUninstallerVersion, version)
}

// editorRegMissingTypes lists the types of the list for which Windows does not
// offer the Text Editor the way Setup registers it: Default apps
// (Capabilities\FileAssociations names the ProgID) and Open with (the ProgID
// is a value of <ext>\OpenWithProgids). A read error is returned, never
// counted as missing.
func editorRegMissingTypes(reg editorRegistry) ([]string, error) {
	var missing []string
	for _, ext := range textEditorExtensions {
		association, found, ok, err := reg.String(editorRegCapabilities+`\FileAssociations`, ext)
		if err != nil {
			return nil, err
		}
		_, listed, _, err := reg.String(`Software\Classes\`+ext+`\OpenWithProgids`, editorRegProgID)
		if err != nil {
			return nil, err
		}
		if !found || !ok || association != editorRegProgID || !listed {
			missing = append(missing, ext)
		}
	}
	return missing, nil
}

// editorRegInstalledHere reads Setup's uninstall key: whether exe is the
// installed program (its folder is InstallLocation) and the version of the
// installed uninstaller.
func editorRegInstalledHere(reg editorRegistry, exe string) (installed bool, version string, err error) {
	location, found, ok, err := reg.String(editorRegUninstall, "InstallLocation")
	if err != nil {
		return false, "", fmt.Errorf("Windows could not read the installed program's registration: %w", err)
	}
	location = strings.TrimSpace(location)
	if !found || !ok || location == "" || !editorRegSamePath(companionLongPath(location), companionLongPath(editorRegDir(exe))) {
		return false, "", nil
	}
	version, found, ok, err = reg.String(editorRegUninstall, "DisplayVersion")
	if err != nil {
		return false, "", fmt.Errorf("Windows could not read the installed program's registration: %w", err)
	}
	if !found || !ok {
		version = ""
	}
	return true, strings.TrimSpace(version), nil
}

// editorRegTypesState is the status App settings shows, or nil when exe is not
// the installed program.
func editorRegTypesState(reg editorRegistry, exe string) (*editorRegTypesStatus, error) {
	if !strings.EqualFold(editorRegBase(exe), editorRegAppFile) {
		return nil, nil
	}
	installed, version, err := editorRegInstalledHere(reg, exe)
	if err != nil || !installed {
		return nil, err
	}
	missing, err := editorRegMissingTypes(reg)
	if err != nil {
		return nil, fmt.Errorf("Windows could not read the Text Editor registration: %w", err)
	}
	return &editorRegTypesStatus{
		Total:              len(textEditorExtensions),
		Missing:            len(missing),
		SetupVersion:       version,
		UninstallerRecords: editorRegUninstallerRecords(version),
		SetupNeeded:        len(missing) > 0,
	}, nil
}

// planEditorAddTypes is Setup's RegisterExtension for each missing type: the
// record of a key it is about to create first (the type's key when absent,
// else its Open With list when absent), then the type in Default apps and the
// ProgID in the type's Open With list. Nothing else of the type is written.
func planEditorAddTypes(reg editorRegistry, missing []string) (editorRegPlan, error) {
	var plan editorRegPlan
	for _, ext := range missing {
		key := `Software\Classes\` + ext
		extExists, err := editorRegExists(reg, key)
		if err != nil {
			return nil, err
		}
		listExists, err := editorRegExists(reg, key+`\OpenWithProgids`)
		if err != nil {
			return nil, err
		}
		switch {
		case !extExists:
			plan.dword(editorRegSetupKeys, ext, 1)
		case !listExists:
			plan.dword(editorRegSetupKeys, ext+`\OpenWithProgids`, 1)
		}
		plan.str(editorRegCapabilities+`\FileAssociations`, ext, editorRegProgID)
		plan.str(key+`\OpenWithProgids`, editorRegProgID, "")
	}
	return plan, nil
}

var errEditorTypesNotSetups = errors.New("the Text Editor registration is not the installed program's")

// refreshEditorTypes adds the types of the list that the installed Text Editor
// registration lacks, when exe is the installed Network_Toolkit.exe, the
// installed uninstaller records and removes them (editorRegUninstallerRecords),
// and Setup's registration is in place: the ProgID starts exe and Setup's
// record exists. It holds Setup's lock and notifies the shell after a change.
// It reports how many types it added: 0 and no error when nothing is missing,
// exe is not the installed program or its uninstaller is older, and
// errEditorTypesNotSetups when the registration is not Setup's for exe.
func refreshEditorTypes(p editorRegPlatform, exe string) (int, error) {
	if !strings.EqualFold(editorRegBase(exe), editorRegAppFile) {
		return 0, nil
	}
	editorRegActionMu.Lock()
	defer editorRegActionMu.Unlock()
	release, err := p.lock()
	if err != nil {
		return 0, err
	}
	defer release()
	installed, version, err := editorRegInstalledHere(p.registry, exe)
	if err != nil || !installed || !editorRegUninstallerRecords(version) {
		return 0, err
	}
	command, found, ok, err := p.registry.String(editorRegProgKey+`\shell\open\command`, "")
	if err != nil {
		return 0, err
	}
	record, err := editorRegExists(p.registry, editorRegSetupRecord)
	if err != nil {
		return 0, err
	}
	if !found || !ok || !editorRegStartsWith(command, exe) || !record {
		return 0, errEditorTypesNotSetups
	}
	missing, err := editorRegMissingTypes(p.registry)
	if err != nil || len(missing) == 0 {
		return 0, err
	}
	plan, err := planEditorAddTypes(p.registry, missing)
	if err != nil {
		return 0, err
	}
	applyErr := plan.apply(p.registry)
	p.notify()
	if applyErr != nil {
		return 0, applyErr
	}
	return len(missing), nil
}
