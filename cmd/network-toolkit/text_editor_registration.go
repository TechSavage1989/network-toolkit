package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// "Use this copy's Text Editor in Windows" (App settings > Editor, portable
// program on Windows). On the user's click the portable program writes the same
// per-user registrations as Setup (scripts/installer/windows.nsi), with every
// command pointing at this copy, and records what it created so that Remove
// takes back only that. Nothing is written at startup.
//
// Ownership, in one place (AGENTS.md "Portable Text Editor registration"):
//   - The installed program wins. While Setup's uninstall key names an install
//     folder, a portable copy changes nothing and only says so.
//   - Setup takes over a portable registration: it adopts the portable record's
//     list of created keys, rewrites every command for the installed program,
//     carries over the right-click choice and deletes the portable record.
//   - One portable copy at a time. Another copy replaces a portable
//     registration only after the user confirms (takeover).
//   - Never a UserChoice, never anything under FileExts, never the default
//     value of an extension, never a key another program wrote; keys under
//     shared parents are recorded only when this registration created them and
//     removed only once they are empty (editorRegDeleteEmpty).
//
// The registry is reached through editorRegistry so that every decision is
// tested on any platform (text_editor_registration_test.go).

// These names are Setup's (windows.nsi defines); a test compares them.
const (
	editorRegAppName      = "Network Toolkit Text Editor" // EDITOR_NAME
	editorRegAppFile      = "Network_Toolkit.exe"         // APP_FILENAME
	editorRegArgs         = "--editor-standalone"         // EDITOR_ARGS
	editorRegProgID       = "NetworkToolkit.TextDocument" // PROG_ID
	editorRegProgKey      = `Software\Classes\` + editorRegProgID
	editorRegProgName     = "Network Toolkit text document"
	editorRegVendorKey    = `Software\Network Toolkit`
	editorRegCapabilities = editorRegVendorKey + `\Capabilities` // CAPABILITIES
	editorRegDescription  = "Open text, log, configuration, script and source files in Network Toolkit Text Editor."
	editorRegRegistered   = `Software\RegisteredApplications`
	editorRegApplications = `Software\Classes\Applications` // APPLICATIONS_KEY
	editorRegApplication  = editorRegApplications + `\` + editorRegAppFile
	editorRegAllFiles     = `Software\Classes\*` // ALL_FILES_KEY
	editorRegMenuVerb     = editorRegAllFiles + `\shell\NetworkToolkit.TextEditor`
	editorRegMenuText     = "Edit with " + editorRegAppName    // MENU_TEXT
	editorRegSetupRecord  = editorRegVendorKey + `\TextEditor` // EDITOR_KEY
	editorRegUninstall    = `Software\Microsoft\Windows\CurrentVersion\Uninstall\NetworkToolkit`
	// The portable registration's own record (PORTABLE_KEY in windows.nsi):
	// Executable (the copy it opens), ContextMenu (1 when the right-click
	// entry was chosen) and CreatedKeys (value names as in Setup's CreatedKeys).
	editorRegPortable     = editorRegVendorKey + `\PortableTextEditor`
	editorRegPortableKeys = editorRegPortable + `\CreatedKeys`
	// Zero-based icon groups of the program file (ICON_GROUPS in
	// scripts/build_windows_resources.py). The program icon of the Text Editor
	// stands for the application; the document icon for files of its types.
	editorRegEditorIcon   = 1
	editorRegDocumentIcon = 3
)

// The separate Text Editor program's own Open with entry
// (EDITOR_APPLICATION_KEY in windows.nsi): its file name, its icon 0 (the APP
// group of that program) and its own command (editorRegEditorCommand).
const (
	editorRegEditorFile        = editorProgramWindows // EDITOR_FILENAME
	editorRegEditorApplication = editorRegApplications + `\` + editorRegEditorFile
	editorRegEditorProgramIcon = 0
)

// The command every registration starts the Text Editor with: the program, the
// mode and the file as the one quoted value of --open-file (EDITOR_COMMAND).
func editorRegCommand(exe string) string {
	return `"` + exe + `" ` + editorRegArgs + ` --open-file "%1"`
}

// The command of the separate Text Editor program's Open with entry
// (EDITOR_PROGRAM_COMMAND): that program opens the Text Editor without a mode
// switch, and the file is again the one quoted value of --open-file.
func editorRegEditorCommand(program string) string {
	return `"` + program + `" --open-file "%1"`
}

// The separate Text Editor program beside a copy's Network_Toolkit.exe.
func editorRegEditorProgram(exe string) string {
	return editorRegJoin(editorRegDir(exe), editorRegEditorFile)
}

func editorRegIcon(exe string, index int) string {
	return `"` + exe + `",` + strconv.Itoa(index)
}

// editorRegistry is the current user's registry (HKCU). Paths are relative to
// HKEY_CURRENT_USER. "Absent" and "could not be read" are different answers:
// a read error is returned, never reported as absence.
type editorRegistry interface {
	// KeyInfo reports whether the key exists and how many subkeys and values
	// (the unnamed default value included) it holds.
	KeyInfo(path string) (exists bool, subkeys, values int, err error)
	// String reads a value. found is false when the key or value is absent; a
	// value of another type is found with ok false.
	String(path, name string) (value string, found, ok bool, err error)
	DWORD(path, name string) (value uint32, found bool, err error)
	// ValueNames lists the values of a key; an absent key has none.
	ValueNames(path string) ([]string, error)
	SetString(path, name, value string) error
	SetDWORD(path, name string, value uint32) error
	SetNone(path, name string) error
	// DeleteValue removes one value; an absent key or value is not an error.
	DeleteValue(path, name string) error
	// DeleteKey removes one key that has no subkeys (never a tree); an absent
	// key is not an error.
	DeleteKey(path string) error
}

// The ownership states of the Text Editor registration.
const (
	editorRegNone       = "none"       // nothing registered
	editorRegSetup      = "setup"      // the installed program (Setup)
	editorRegThis       = "this"       // this portable copy
	editorRegOther      = "other"      // another portable copy
	editorRegUnrecorded = "unrecorded" // a Network Toolkit registration that no record names
)

type editorRegistrationStatus struct {
	Supported  bool   `json:"supported"`
	Executable string `json:"executable"`
	NameOK     bool   `json:"nameOk"`
	// PathOK is false when the program path holds a "%": Explorer would
	// expand %1, %L and the like inside the program path as well.
	PathOK bool `json:"pathOk"`
	// LocationProblem says why this copy may not be registered from where it
	// is: someone other than this user, SYSTEM and Administrators can change
	// the program or its folder (editorRegAccessCheck).
	LocationProblem string `json:"locationProblem,omitempty"`
	Installed       bool   `json:"installed"`
	InstallLocation string `json:"installLocation,omitempty"`
	ThisIsInstalled bool   `json:"thisIsInstalled"`
	State           string `json:"state"`
	Owner           string `json:"owner,omitempty"`
	OwnerMissing    bool   `json:"ownerMissing"`
	// OwnerUnreachable: the drive or share of that program is not connected
	// (or could not be checked), so whether it exists is unknown.
	OwnerUnreachable bool   `json:"ownerUnreachable"`
	Complete         bool   `json:"complete"`
	ContextMenu      bool   `json:"contextMenu"`
	OpenWithElse     string `json:"openWithElsewhere,omitempty"`
	CanRegister      bool   `json:"canRegister"`
	NeedsTakeover    bool   `json:"needsTakeover"`
	CanRemove        bool   `json:"canRemove"`
	Message          string `json:"message"`
	// EditorProgram is the separate Text Editor program beside this copy that
	// a registration also offers under its own name in Open with: present,
	// of this version, and changeable only by this user and the system
	// (editorRegPlatform.separate). Empty when there is none.
	EditorProgram string `json:"editorProgram,omitempty"`
}

var (
	errEditorRegInstalled = errors.New("The installed Network Toolkit has registered its Text Editor with Windows. This copy does not change that registration. To use this copy instead, uninstall the installed program first.")
	errEditorRegTakeover  = errors.New("Another copy of Network Toolkit is registered. Confirm that this copy should replace it.")
	errEditorRegName      = errors.New("Windows lists the Text Editor under the program file name Network_Toolkit.exe. Rename this copy's program file to Network_Toolkit.exe, start it again, then register.")
	errEditorRegNothing   = errors.New("This copy is not registered, so there is nothing to remove.")
	errEditorRegPercent   = errors.New("This copy's folder has a % sign in its path. Windows would read it as part of the command that opens files, so this copy cannot be registered from there. Move the folder to a path without %, start it from there, then register.")
	errEditorRegUnsafe    = errors.New("This copy cannot be registered from where it is.")
	errEditorRegStale     = errors.New("The registration changed since it was shown. Look at it again before cleaning up.")
)

// Windows paths, handled the same way on every platform so that the tests of
// these decisions run anywhere: either slash separates, case does not matter.
func editorRegWinPath(p string) string {
	p = strings.ReplaceAll(p, "/", `\`)
	for len(p) > 3 && strings.HasSuffix(p, `\`) {
		p = p[:len(p)-1]
	}
	return p
}

func editorRegBase(p string) string {
	p = editorRegWinPath(p)
	return p[strings.LastIndex(p, `\`)+1:]
}

func editorRegDir(p string) string {
	p = editorRegWinPath(p)
	if i := strings.LastIndex(p, `\`); i >= 0 {
		return p[:i]
	}
	return ""
}

func editorRegJoin(dir, name string) string { return editorRegWinPath(dir) + `\` + name }

func editorRegSamePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return strings.EqualFold(editorRegWinPath(a), editorRegWinPath(b))
}

// The program a registration command starts: the first quoted part.
func editorRegCommandProgram(command string) string {
	if !strings.HasPrefix(command, `"`) {
		return ""
	}
	end := strings.Index(command[1:], `"`)
	if end < 0 {
		return ""
	}
	return command[1 : end+1]
}

func editorRegStartsWith(command, exe string) bool {
	prefix := `"` + exe + `"`
	return exe != "" && len(command) >= len(prefix) && strings.EqualFold(command[:len(prefix)], prefix)
}

// Whether a registered program exists. Only a file that is confirmed absent
// on a drive that is there is missing; a drive or share that is not connected,
// or a file that could not be checked, is unreachable (unknown).
type editorRegPresence int

const (
	editorRegPresent editorRegPresence = iota
	editorRegAbsent
	editorRegUnreachable
)

type editorRegFileCheck func(path string) editorRegPresence

func editorRegFileState(path string) editorRegPresence {
	return editorRegClassify(path, os.Stat)
}

func editorRegFileMissing(path string) bool { return editorRegFileState(path) == editorRegAbsent }

// The root of a Windows path's volume: "D:\" or "\\server\share\".
func editorRegVolumeRoot(path string) string {
	p := editorRegWinPath(path)
	if len(p) >= 2 && p[1] == ':' {
		return p[:2] + `\`
	}
	if strings.HasPrefix(p, `\\`) {
		parts := strings.SplitN(p[2:], `\`, 3)
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			return `\\` + parts[0] + `\` + parts[1] + `\`
		}
	}
	return ""
}

func editorRegClassify(path string, stat func(string) (os.FileInfo, error)) editorRegPresence {
	_, err := stat(path)
	switch {
	case err == nil:
		return editorRegPresent
	case !errors.Is(err, os.ErrNotExist):
		return editorRegUnreachable
	}
	if root := editorRegVolumeRoot(path); root != "" {
		if _, err := stat(root); err != nil {
			return editorRegUnreachable
		}
	}
	return editorRegAbsent
}

// editorRegAccessCheck names who other than this user, SYSTEM and
// Administrators (and TrustedInstaller) can change a file or folder; "" when
// nobody. Windows reads the object's owner and DACL.
type editorRegAccessCheck func(path string) (others string, err error)

// editorRegState reads who owns the registration. exe is this copy's program.
func editorRegState(reg editorRegistry, exe string, presence editorRegFileCheck) (editorRegistrationStatus, error) {
	owner := func(path string) (missing, unreachable bool) {
		if path == "" {
			return true, false
		}
		switch presence(path) {
		case editorRegAbsent:
			return true, false
		case editorRegUnreachable:
			return false, true
		}
		return false, false
	}
	status := editorRegistrationStatus{Supported: true, Executable: exe, NameOK: strings.EqualFold(editorRegBase(exe), editorRegAppFile), PathOK: !strings.Contains(exe, "%"), State: editorRegNone}
	location, found, ok, err := reg.String(editorRegUninstall, "InstallLocation")
	if err != nil {
		return status, fmt.Errorf("Windows could not read the installed program's registration: %w", err)
	}
	if found && ok && strings.TrimSpace(location) != "" {
		status.Installed, status.InstallLocation = true, location
		status.ThisIsInstalled = editorRegSamePath(editorRegDir(exe), location)
		status.State, status.Owner = editorRegSetup, editorRegJoin(location, editorRegAppFile)
	}
	command, _, _, err := reg.String(editorRegProgKey+`\shell\open\command`, "")
	if err != nil {
		return status, fmt.Errorf("Windows could not read the Text Editor registration: %w", err)
	}
	recordExists, _, _, err := reg.KeyInfo(editorRegPortable)
	if err != nil {
		return status, fmt.Errorf("Windows could not read this program's registration record: %w", err)
	}
	if status.State == editorRegNone && recordExists {
		recorded, _, _, err := reg.String(editorRegPortable, "Executable")
		if err != nil {
			return status, fmt.Errorf("Windows could not read this program's registration record: %w", err)
		}
		status.Owner = recorded
		if editorRegSamePath(recorded, exe) {
			status.State = editorRegThis
			status.Complete = command == editorRegCommand(exe)
		} else {
			status.State = editorRegOther
			status.OwnerMissing, status.OwnerUnreachable = owner(recorded)
		}
		menu, _, err := reg.DWORD(editorRegPortable, "ContextMenu")
		if err != nil {
			return status, fmt.Errorf("Windows could not read this program's registration record: %w", err)
		}
		status.ContextMenu = menu == 1
	}
	if status.State == editorRegNone {
		progExists, _, _, err := reg.KeyInfo(editorRegProgKey)
		if err != nil {
			return status, fmt.Errorf("Windows could not read the Text Editor registration: %w", err)
		}
		setupRecord, _, _, err := reg.KeyInfo(editorRegSetupRecord)
		if err != nil {
			return status, fmt.Errorf("Windows could not read the Text Editor registration: %w", err)
		}
		if progExists || setupRecord {
			status.State = editorRegUnrecorded
			status.Owner = editorRegCommandProgram(command)
			status.OwnerMissing, status.OwnerUnreachable = owner(status.Owner)
		}
	}
	if status.State == editorRegThis {
		app, found, _, err := reg.String(editorRegApplication+`\shell\open\command`, "")
		if err != nil {
			return status, fmt.Errorf("Windows could not read the Open with registration: %w", err)
		}
		if found && !editorRegStartsWith(app, exe) {
			status.OpenWithElse = editorRegCommandProgram(app)
		}
	}
	switch status.State {
	case editorRegNone:
		status.CanRegister = status.NameOK
		status.Message = "Windows does not offer this copy's Text Editor."
	case editorRegSetup:
		if status.ThisIsInstalled {
			status.Message = "This is the installed program. Setup registered its Text Editor with Windows; run Setup again or uninstall to change that."
		} else {
			status.Message = "The installed Network Toolkit at " + location + " is registered with Windows. This copy does not change that registration. To use this copy instead, uninstall the installed program first."
		}
	case editorRegThis:
		status.CanRegister, status.CanRemove = status.NameOK, true
		if status.Complete {
			status.Message = "Windows offers this copy's Text Editor."
		} else {
			status.Message = "This copy's registration is incomplete. Register again to repair it, or remove it."
		}
	case editorRegOther, editorRegUnrecorded:
		status.CanRegister, status.NeedsTakeover = status.NameOK, true
		status.CanRemove = status.OwnerMissing
		where := status.Owner
		if where == "" {
			where = "an unknown location"
		}
		if status.State == editorRegUnrecorded {
			status.Message = "Windows has a Network Toolkit Text Editor registration that no copy recorded. It opens " + where + "."
		} else {
			status.Message = "Another copy of Network Toolkit is registered, at " + where + "."
		}
		if status.OwnerMissing {
			status.Message += " That program no longer exists, so Windows cannot open files with it. Clean up removes that registration."
		}
		if status.OwnerUnreachable {
			status.Message += " Its drive or network location is not connected (or could not be checked), so this copy cannot tell whether that program still exists. Connect it, or remove the registration from that copy."
		}
	}
	if status.State != editorRegSetup {
		if !status.NameOK {
			status.Message += " " + errEditorRegName.Error()
		}
		if !status.PathOK {
			status.Message += " " + errEditorRegPercent.Error()
		}
	}
	status.CanRegister = status.CanRegister && status.PathOK
	return status, nil
}

// editorRegStatus is editorRegState plus the check of where this copy is:
// registering makes Windows run this program for every file opened through
// it, so a program or folder that other users can change is refused.
func editorRegStatus(p editorRegPlatform, exe string) (editorRegistrationStatus, error) {
	status, err := editorRegState(p.registry, exe, p.presence)
	if err != nil || !status.CanRegister {
		return status, err
	}
	for _, path := range []string{exe, editorRegDir(exe)} {
		others, err := p.access(path)
		if err != nil {
			status.LocationProblem = "Windows could not check who can change " + path + " (" + err.Error() + "), so this copy is not registered from there."
		} else if others != "" {
			status.LocationProblem = others + " can change " + path + ". Once registered, Windows would run this program for every text file you double-click, so anyone who can change it could run their own program as you. Move this copy into a folder under your user folder (for example Documents or %LOCALAPPDATA%), start it from there, then register."
		}
		if status.LocationProblem != "" {
			status.CanRegister = false
			status.Message += " " + status.LocationProblem
			break
		}
	}
	if status.CanRegister && p.separate != nil {
		status.EditorProgram = p.separate(exe)
	}
	return status, nil
}

// editorRegOp is one step of a plan. Plans are computed from reads only and
// applied in order; "delete if empty" is decided when it runs.
type editorRegOp struct {
	Kind  string // set-string, set-dword, set-none, delete-value, delete-empty
	Path  string
	Name  string
	Value string
	DWord uint32
}

func (op editorRegOp) String() string {
	switch op.Kind {
	case "set-string":
		return fmt.Sprintf("set %s [%s] = %q", op.Path, op.Name, op.Value)
	case "set-dword":
		return fmt.Sprintf("set %s [%s] = dword:%d", op.Path, op.Name, op.DWord)
	case "set-none":
		return fmt.Sprintf("set %s [%s] = none", op.Path, op.Name)
	case "delete-value":
		return fmt.Sprintf("delete %s [%s]", op.Path, op.Name)
	default:
		return "delete-if-empty " + op.Path
	}
}

type editorRegPlan []editorRegOp

func (p *editorRegPlan) str(path, name, value string) {
	*p = append(*p, editorRegOp{Kind: "set-string", Path: path, Name: name, Value: value})
}
func (p *editorRegPlan) dword(path, name string, value uint32) {
	*p = append(*p, editorRegOp{Kind: "set-dword", Path: path, Name: name, DWord: value})
}
func (p *editorRegPlan) none(path, name string) {
	*p = append(*p, editorRegOp{Kind: "set-none", Path: path, Name: name})
}
func (p *editorRegPlan) del(path, name string) {
	*p = append(*p, editorRegOp{Kind: "delete-value", Path: path, Name: name})
}
func (p *editorRegPlan) empty(path string) {
	*p = append(*p, editorRegOp{Kind: "delete-empty", Path: path})
}

// editorRegDeleteEmpty is DeleteEmptyUserKey of windows.nsi: a key is removed
// only when nothing is left in it, no subkey and no value, not even an unnamed
// one. What another program added keeps the key.
func editorRegDeleteEmpty(reg editorRegistry, path string) error {
	exists, subkeys, values, err := reg.KeyInfo(path)
	if err != nil || !exists || subkeys != 0 || values != 0 {
		return err
	}
	return reg.DeleteKey(path)
}

func (p editorRegPlan) apply(reg editorRegistry) error {
	for _, op := range p {
		var err error
		switch op.Kind {
		case "set-string":
			err = reg.SetString(op.Path, op.Name, op.Value)
		case "set-dword":
			err = reg.SetDWORD(op.Path, op.Name, op.DWord)
		case "set-none":
			err = reg.SetNone(op.Path, op.Name)
		case "delete-value":
			err = reg.DeleteValue(op.Path, op.Name)
		case "delete-empty":
			err = editorRegDeleteEmpty(reg, op.Path)
		default:
			err = errors.New("unknown registry step")
		}
		if err != nil {
			return fmt.Errorf("Windows refused a registry change (%s): %w", op, err)
		}
	}
	return nil
}

func editorRegExists(reg editorRegistry, path string) (bool, error) {
	exists, _, _, err := reg.KeyInfo(path)
	return exists, err
}

// The records of created keys this registration holds, by value name.
func editorRegRecords(reg editorRegistry) (map[string]bool, error) {
	names, err := reg.ValueNames(editorRegPortableKeys)
	if err != nil {
		return nil, err
	}
	records := make(map[string]bool, len(names))
	for _, name := range names {
		if value, found, err := reg.DWORD(editorRegPortableKeys, name); err != nil {
			return nil, err
		} else if found && value == 1 {
			records[name] = true
		}
	}
	return records, nil
}

// planEditorRegister computes the writes that register this copy. status is
// what editorRegState read a moment ago; takeover is the user's confirmation
// that another copy's registration may be replaced.
func planEditorRegister(reg editorRegistry, status editorRegistrationStatus, contextMenu, takeover bool) (editorRegPlan, error) {
	exe := status.Executable
	switch {
	case status.State == editorRegSetup:
		return nil, errEditorRegInstalled
	case !status.NameOK:
		return nil, errEditorRegName
	case !status.PathOK:
		return nil, errEditorRegPercent
	case status.LocationProblem != "":
		return nil, fmt.Errorf("%w %s", errEditorRegUnsafe, status.LocationProblem)
	case status.NeedsTakeover && !takeover:
		return nil, errEditorRegTakeover
	}
	previous := ""
	if status.State == editorRegOther || status.State == editorRegUnrecorded {
		previous = status.Owner
	}
	records, err := editorRegRecords(reg)
	if err != nil {
		return nil, err
	}
	var plan editorRegPlan
	// The record comes first: an interrupted registration can always be
	// removed, and a key is recorded before it is created.
	plan.str(editorRegPortable, "Executable", exe)
	type probe struct{ ext, open bool }
	probes := make([]probe, len(textEditorExtensions))
	for i, ext := range textEditorExtensions {
		if probes[i].ext, err = editorRegExists(reg, `Software\Classes\`+ext); err != nil {
			return nil, err
		}
		if probes[i].open, err = editorRegExists(reg, `Software\Classes\`+ext+`\OpenWithProgids`); err != nil {
			return nil, err
		}
	}
	for i, ext := range textEditorExtensions {
		if !probes[i].ext && !records[ext] {
			plan.dword(editorRegPortableKeys, ext, 1)
		} else if probes[i].ext && !probes[i].open && !records[ext] && !records[ext+`\OpenWithProgids`] {
			plan.dword(editorRegPortableKeys, ext+`\OpenWithProgids`, 1)
		}
	}
	// "Open with" for any file: written when absent, when it already starts
	// this copy (Windows writes such an entry itself), or when it is the
	// replaced registration's own command.
	appCommand, appFound, _, err := reg.String(editorRegApplication+`\shell\open\command`, "")
	if err != nil {
		return nil, err
	}
	appExists, err := editorRegExists(reg, editorRegApplication)
	if err != nil {
		return nil, err
	}
	ownApplication := !appExists || (appFound && (editorRegStartsWith(appCommand, exe) || (previous != "" && appCommand == editorRegCommand(previous))))
	applicationsExists, err := editorRegExists(reg, editorRegApplications)
	if err != nil {
		return nil, err
	}
	// The separate Text Editor program's own Open with entry, by the same
	// rules, when that program is beside this copy (status.EditorProgram).
	// Without it, an entry this registration (or the replaced one) recorded
	// and that still starts its program is taken back.
	editorCommand, editorFound, _, err := reg.String(editorRegEditorApplication+`\shell\open\command`, "")
	if err != nil {
		return nil, err
	}
	editorExists, err := editorRegExists(reg, editorRegEditorApplication)
	if err != nil {
		return nil, err
	}
	editorPrevious := editorFound && (editorCommand == editorRegEditorCommand(editorRegEditorProgram(exe)) || (previous != "" && editorCommand == editorRegEditorCommand(editorRegEditorProgram(previous))))
	ownEditorApplication := status.EditorProgram != "" && (!editorExists || (editorFound && (editorRegStartsWith(editorCommand, status.EditorProgram) || editorPrevious)))
	if ownApplication {
		plan.dword(editorRegPortableKeys, `Applications\`+editorRegAppFile, 1)
	}
	if ownEditorApplication {
		plan.dword(editorRegPortableKeys, `Applications\`+editorRegEditorFile, 1)
	}
	if (ownApplication || ownEditorApplication) && !applicationsExists {
		plan.dword(editorRegPortableKeys, "Applications", 1)
	}
	allFiles, err := editorRegExists(reg, editorRegAllFiles)
	if err != nil {
		return nil, err
	}
	allFilesShell, err := editorRegExists(reg, editorRegAllFiles+`\shell`)
	if err != nil {
		return nil, err
	}
	if contextMenu {
		if !allFiles && !records["*"] {
			plan.dword(editorRegPortableKeys, "*", 1)
		} else if allFiles && !allFilesShell && !records["*"] && !records[`*\shell`] {
			plan.dword(editorRegPortableKeys, `*\shell`, 1)
		}
	}
	command := editorRegCommand(exe)
	plan.str(editorRegProgKey, "", editorRegProgName)
	plan.none(editorRegProgKey, "AllowSilentDefaultTakeOver")
	plan.str(editorRegProgKey+`\DefaultIcon`, "", editorRegIcon(exe, editorRegDocumentIcon))
	plan.str(editorRegProgKey+`\shell\open\command`, "", command)
	plan.str(editorRegCapabilities, "ApplicationName", editorRegAppName)
	plan.str(editorRegCapabilities, "ApplicationDescription", editorRegDescription)
	plan.str(editorRegCapabilities, "ApplicationIcon", editorRegIcon(exe, editorRegEditorIcon))
	// The extension's own key and default value belong to whichever program
	// registered the type; only the ProgID is added to OpenWithProgids.
	for _, ext := range textEditorExtensions {
		plan.str(editorRegCapabilities+`\FileAssociations`, ext, editorRegProgID)
		plan.str(`Software\Classes\`+ext+`\OpenWithProgids`, editorRegProgID, "")
	}
	plan.str(editorRegRegistered, editorRegAppName, editorRegCapabilities)
	if ownApplication {
		plan.str(editorRegApplication, "FriendlyAppName", editorRegAppName)
		plan.str(editorRegApplication+`\DefaultIcon`, "", editorRegIcon(exe, editorRegEditorIcon))
		plan.str(editorRegApplication+`\shell\open\command`, "", command)
	}
	switch {
	case ownEditorApplication:
		plan.str(editorRegEditorApplication, "FriendlyAppName", editorRegAppName)
		plan.str(editorRegEditorApplication+`\DefaultIcon`, "", editorRegIcon(status.EditorProgram, editorRegEditorProgramIcon))
		plan.str(editorRegEditorApplication+`\shell\open\command`, "", editorRegEditorCommand(status.EditorProgram))
	case status.EditorProgram == "" && editorPrevious && records[`Applications\`+editorRegEditorFile]:
		planEditorRemoveApplication(&plan, editorRegEditorApplication)
		plan.del(editorRegPortableKeys, `Applications\`+editorRegEditorFile)
	}
	verbCommand, verbFound, _, err := reg.String(editorRegMenuVerb+`\command`, "")
	if err != nil {
		return nil, err
	}
	if contextMenu {
		plan.str(editorRegMenuVerb, "", editorRegMenuText)
		plan.str(editorRegMenuVerb, "Icon", editorRegIcon(exe, editorRegEditorIcon))
		plan.str(editorRegMenuVerb+`\command`, "", command)
		plan.dword(editorRegPortable, "ContextMenu", 1)
	} else if verbFound && (verbCommand == command || (previous != "" && verbCommand == editorRegCommand(previous))) {
		// The entry was chosen earlier (by this or the replaced copy) and is
		// not wanted now.
		planEditorRemoveMenu(&plan, records)
		plan.del(editorRegPortable, "ContextMenu")
	} else {
		plan.del(editorRegPortable, "ContextMenu")
	}
	return plan, nil
}

// An Open with entry under Applications: the values a registration wrote, then
// each key once it is empty.
func planEditorRemoveApplication(plan *editorRegPlan, key string) {
	plan.del(key+`\shell\open\command`, "")
	plan.empty(key + `\shell\open\command`)
	plan.empty(key + `\shell\open`)
	plan.empty(key + `\shell`)
	plan.del(key+`\DefaultIcon`, "")
	plan.empty(key + `\DefaultIcon`)
	plan.del(key, "FriendlyAppName")
	plan.empty(key)
}

// The right-click entry: the values this program wrote, then each key once it
// is empty, then the shared parents that this registration created.
func planEditorRemoveMenu(plan *editorRegPlan, records map[string]bool) {
	plan.del(editorRegMenuVerb+`\command`, "")
	plan.empty(editorRegMenuVerb + `\command`)
	plan.del(editorRegMenuVerb, "")
	plan.del(editorRegMenuVerb, "Icon")
	plan.empty(editorRegMenuVerb)
	if records["*"] {
		plan.empty(editorRegAllFiles + `\shell`)
		plan.empty(editorRegAllFiles)
	} else if records[`*\shell`] {
		plan.empty(editorRegAllFiles + `\shell`)
	}
	plan.del(editorRegPortableKeys, "*")
	plan.del(editorRegPortableKeys, `*\shell`)
}

// planEditorRemove computes what Remove (or Clean up, for a registration whose
// program no longer exists) takes back: only what the record says this
// registration created, and only values that still say what it wrote.
func planEditorRemove(reg editorRegistry, status editorRegistrationStatus) (editorRegPlan, error) {
	switch {
	case status.State == editorRegSetup:
		return nil, errEditorRegInstalled
	case status.State == editorRegThis:
	case (status.State == editorRegOther || status.State == editorRegUnrecorded) && status.OwnerMissing:
	case status.State == editorRegOther || status.State == editorRegUnrecorded:
		return nil, errors.New("Another copy of Network Toolkit is registered and its program exists. Remove the registration from that copy.")
	default:
		return nil, errEditorRegNothing
	}
	owner := status.Owner
	if status.State == editorRegThis {
		owner = status.Executable
	}
	command := editorRegCommand(owner)
	records, err := editorRegRecords(reg)
	if err != nil {
		return nil, err
	}
	var plan editorRegPlan
	// File types: the list, and any type the record names (a later list never
	// drops a type, but the record is what was created).
	extensions := append([]string(nil), textEditorExtensions...)
	seen := map[string]bool{}
	for _, ext := range extensions {
		seen[ext] = true
	}
	var extra []string
	for name := range records {
		ext := strings.TrimSuffix(name, `\OpenWithProgids`)
		if strings.HasPrefix(ext, ".") && !strings.Contains(ext, `\`) && !seen[ext] {
			seen[ext] = true
			extra = append(extra, ext)
		}
	}
	sort.Strings(extra)
	extensions = append(extensions, extra...)
	for _, ext := range extensions {
		key := `Software\Classes\` + ext
		plan.del(key+`\OpenWithProgids`, editorRegProgID)
		if records[ext] {
			plan.empty(key + `\OpenWithProgids`)
			plan.empty(key)
		} else if records[ext+`\OpenWithProgids`] {
			plan.empty(key + `\OpenWithProgids`)
		}
	}
	// "Open with" for any file, while it still starts this registration's program.
	appCommand, _, _, err := reg.String(editorRegApplication+`\shell\open\command`, "")
	if err != nil {
		return nil, err
	}
	if records[`Applications\`+editorRegAppFile] && appCommand == command {
		planEditorRemoveApplication(&plan, editorRegApplication)
	}
	// The separate Text Editor program's entry, while it still starts the
	// Text Editor program beside this registration's program.
	editorCommand, _, _, err := reg.String(editorRegEditorApplication+`\shell\open\command`, "")
	if err != nil {
		return nil, err
	}
	if records[`Applications\`+editorRegEditorFile] && editorCommand == editorRegEditorCommand(editorRegEditorProgram(owner)) {
		planEditorRemoveApplication(&plan, editorRegEditorApplication)
	}
	if records["Applications"] {
		plan.empty(editorRegApplications)
	}
	verbCommand, verbFound, _, err := reg.String(editorRegMenuVerb+`\command`, "")
	if err != nil {
		return nil, err
	}
	if verbFound && verbCommand == command {
		planEditorRemoveMenu(&plan, records)
	} else if records["*"] || records[`*\shell`] {
		// The entry is gone or another program's now; the shared parents
		// still go once they are empty.
		if records["*"] {
			plan.empty(editorRegAllFiles + `\shell`)
			plan.empty(editorRegAllFiles)
		} else {
			plan.empty(editorRegAllFiles + `\shell`)
		}
		plan.del(editorRegPortableKeys, "*")
		plan.del(editorRegPortableKeys, `*\shell`)
	}
	// The ProgID, while it still starts this registration's program.
	progCommand, progFound, _, err := reg.String(editorRegProgKey+`\shell\open\command`, "")
	if err != nil {
		return nil, err
	}
	if !progFound || progCommand == command {
		plan.del(editorRegProgKey+`\shell\open\command`, "")
		plan.empty(editorRegProgKey + `\shell\open\command`)
		plan.empty(editorRegProgKey + `\shell\open`)
		plan.empty(editorRegProgKey + `\shell`)
		plan.del(editorRegProgKey+`\DefaultIcon`, "")
		plan.empty(editorRegProgKey + `\DefaultIcon`)
		plan.del(editorRegProgKey, "")
		plan.del(editorRegProgKey, "AllowSilentDefaultTakeOver")
		plan.empty(editorRegProgKey)
		// Default apps: the types that name this ProgID, then the description.
		associations, err := reg.ValueNames(editorRegCapabilities + `\FileAssociations`)
		if err != nil {
			return nil, err
		}
		for _, name := range associations {
			value, _, ok, err := reg.String(editorRegCapabilities+`\FileAssociations`, name)
			if err != nil {
				return nil, err
			}
			if ok && value == editorRegProgID {
				plan.del(editorRegCapabilities+`\FileAssociations`, name)
			}
		}
		plan.empty(editorRegCapabilities + `\FileAssociations`)
		plan.del(editorRegCapabilities, "ApplicationName")
		plan.del(editorRegCapabilities, "ApplicationDescription")
		plan.del(editorRegCapabilities, "ApplicationIcon")
		plan.empty(editorRegCapabilities)
		registered, _, ok, err := reg.String(editorRegRegistered, editorRegAppName)
		if err != nil {
			return nil, err
		}
		if ok && registered == editorRegCapabilities {
			plan.del(editorRegRegistered, editorRegAppName)
		}
	}
	// The record itself, last.
	names, err := reg.ValueNames(editorRegPortableKeys)
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	for _, name := range names {
		plan.del(editorRegPortableKeys, name)
	}
	plan.empty(editorRegPortableKeys)
	plan.del(editorRegPortable, "Executable")
	plan.del(editorRegPortable, "ContextMenu")
	plan.empty(editorRegPortable)
	plan.empty(editorRegVendorKey)
	return plan, nil
}

// The platform pieces (text_editor_registration_windows.go): the registry, the
// Setup lock shared with windows.nsi, and the shell notification.
type editorRegPlatform struct {
	registry editorRegistry
	lock     func() (func(), error)
	notify   func()
	presence editorRegFileCheck
	access   editorRegAccessCheck
	// separate names the separate Text Editor program beside exe when it may
	// be registered (EditorProgram), or "". Nil: never.
	separate func(exe string) string
}

// registerEditorCopy and removeEditorCopy run one action under the lock that
// Setup and Uninstall hold, re-reading the state first so that the decision
// never rests on an older status.
func registerEditorCopy(p editorRegPlatform, exe string, contextMenu, takeover bool) (editorRegistrationStatus, error) {
	return p.run(exe, func(status editorRegistrationStatus) (editorRegPlan, error) {
		return planEditorRegister(p.registry, status, contextMenu, takeover)
	})
}

// removeEditorCopy takes back this copy's own registration only.
func removeEditorCopy(p editorRegPlatform, exe string) (editorRegistrationStatus, error) {
	return p.run(exe, func(status editorRegistrationStatus) (editorRegPlan, error) {
		switch status.State {
		case editorRegThis:
			return planEditorRemove(p.registry, status)
		case editorRegSetup:
			return nil, errEditorRegInstalled
		}
		return nil, errEditorRegNothing
	})
}

// cleanupEditorCopy removes another copy's registration whose program is
// confirmed missing. owner is the program the user saw: the registration must
// still name it, and it must still be missing, when the action runs.
func cleanupEditorCopy(p editorRegPlatform, exe, owner string) (editorRegistrationStatus, error) {
	return p.run(exe, func(status editorRegistrationStatus) (editorRegPlan, error) {
		if status.State == editorRegSetup {
			return nil, errEditorRegInstalled
		}
		if (status.State != editorRegOther && status.State != editorRegUnrecorded) || !status.OwnerMissing ||
			!(editorRegSamePath(owner, status.Owner) || owner == "" && status.Owner == "") {
			return nil, errEditorRegStale
		}
		return planEditorRemove(p.registry, status)
	})
}

// One action at a time in this process; the Setup mutex covers other processes.
var editorRegActionMu sync.Mutex

func (p editorRegPlatform) run(exe string, plan func(editorRegistrationStatus) (editorRegPlan, error)) (editorRegistrationStatus, error) {
	editorRegActionMu.Lock()
	defer editorRegActionMu.Unlock()
	release, err := p.lock()
	if err != nil {
		return editorRegistrationStatus{}, err
	}
	defer release()
	status, err := editorRegStatus(p, exe)
	if err != nil {
		return status, err
	}
	steps, err := plan(status)
	if err != nil {
		return status, err
	}
	applyErr := steps.apply(p.registry)
	// SHCNE_ASSOCCHANGED after any change, also a partial one.
	p.notify()
	after, err := editorRegStatus(p, exe)
	if applyErr != nil {
		return after, applyErr
	}
	return after, err
}
