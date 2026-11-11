package main

import (
	"bytes"
	"debug/pe"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"networktoolkit/internal/model"
)

// One code base builds three programs (docs/SEPARATE_EXECUTABLES.md). The
// build tags ntk_editor and ntk_terminal set buildEdition (edition_*.go);
// without them it is the main program. The edition decides only what the
// program opens without a switch and which switches it accepts. Data folders,
// workspaces, single-instance locks, instance records and peer send are keyed
// by the data root, never by the program, so the same app started from either
// program is the same single instance with the same data.

const (
	mainProgramWindows     = "Network_Toolkit.exe"
	editorProgramWindows   = "Network_Toolkit_TextEditor.exe"
	terminalProgramWindows = "Network_Toolkit_Terminal.exe"
)

// programName is the file name of the program built for an edition on this
// operating system. The Linux edition ships only the main program; the names
// exist there for development builds and tests.
func programName(edition string) string {
	return programNameFor(runtime.GOOS, edition)
}

func programNameFor(goos, edition string) string {
	if goos == "windows" {
		switch edition {
		case standaloneEditorMode:
			return editorProgramWindows
		case "terminal":
			return terminalProgramWindows
		}
		return mainProgramWindows
	}
	switch edition {
	case standaloneEditorMode:
		return "network-toolkit-text-editor"
	case "terminal":
		return "network-toolkit-terminal"
	}
	return "network-toolkit"
}

// editionTitle names the app a separate program opens.
func editionTitle(edition string) string {
	if edition == "terminal" {
		return appName + " Terminal"
	}
	if edition == standaloneEditorMode {
		return standaloneEditorTitle
	}
	return appName
}

// applyBuildEdition makes a separate program open its own app without a
// switch. Its own switches keep working; a switch for another app is refused,
// so the name Windows shows for the program always matches its window. The
// private roles (detached windows, version) run as on every build. The main
// program is unchanged.
func applyBuildEdition(flags *flag.FlagSet, edition string) error {
	if edition == "" {
		return nil
	}
	set := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if set["version"] || set["terminal-view"] || set["editor-view"] {
		return nil
	}
	refuse := func(name string) error {
		other := "the Text Editor with " + programName(standaloneEditorMode) + " or " + programName("") + " --editor-standalone"
		if edition == standaloneEditorMode {
			other = "the Terminal with " + programName("terminal") + " or " + programName("") + " --terminal"
		}
		return fmt.Errorf("%s opens %s only (--%s is not accepted). Open %s.", programName(edition), editionTitle(edition), name, other)
	}
	switch edition {
	case "terminal":
		for _, name := range []string{"editor", standaloneEditorMode, "open-file"} {
			if set[name] {
				return refuse(name)
			}
		}
		return flags.Set("terminal", "true")
	case standaloneEditorMode:
		for _, name := range []string{"terminal", "terminal-preset", terminalLinkFlag} {
			if set[name] {
				return refuse(name)
			}
		}
		if set["editor"] {
			// The Start menu's earlier Text Editor argument: the same app here.
			if err := flags.Set("editor", "false"); err != nil {
				return err
			}
		}
		return flags.Set(standaloneEditorMode, "true")
	}
	return errors.New("Unknown program edition.")
}

// separateProgramMatches reports whether path is a program of this version
// built for edition, safe to start: a regular file, not a link, and on Windows
// a PE file whose version resource names this version and the edition's
// original file name. Tests replace it.
var separateProgramMatches = programFileMatches

func programFileMatches(path, edition string) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || storageIsLink(info) {
		return false
	}
	if runtime.GOOS != "windows" {
		return info.Mode().Perm()&0o111 != 0
	}
	f, err := pe.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	section := f.Section(".rsrc")
	if section == nil || section.Size > 4<<20 {
		return false
	}
	resource, err := section.Data()
	if err != nil {
		return false
	}
	return programResourceMatches(resource, model.Version, programNameFor("windows", edition)) == nil
}

// programResourceMatches checks a version resource for the product, the exact
// version and the program's original file name (each a complete UTF-16 value).
func programResourceMatches(resource []byte, version, original string) error {
	for _, value := range []string{appName, "NetworkToolkit", version, original} {
		if !bytes.Contains(resource, updateUTF16(value)) {
			return errors.New("The program's version resource does not name " + value + ".")
		}
	}
	return nil
}

// standaloneProgram is the program that opens a standalone app (mode is
// "terminal" or standaloneEditorMode) beside the running program, with the
// fixed switch for that app, in this order: the separate program of this
// version; the running program when it is the main program; the main program
// of this version beside a separate program. The switch is accepted by each.
func standaloneProgram(mode string) (string, error) {
	self, err := updateCurrentExecutable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}
	if buildEdition == mode {
		return self, nil
	}
	dir := filepath.Dir(self)
	if candidate := filepath.Join(dir, programName(mode)); separateProgramMatches(candidate, mode) {
		return candidate, nil
	}
	if buildEdition == "" {
		return self, nil
	}
	if candidate := filepath.Join(dir, programName("")); separateProgramMatches(candidate, "") {
		return candidate, nil
	}
	return "", fmt.Errorf("Neither %s nor %s is beside %s.", programName(mode), programName(""), filepath.Base(self))
}

// forwardToSeparateProgram runs a standalone app in its separate program when
// the main program is started for it (an earlier shortcut, a pinned item, Open
// with, a link) and that program of this version is beside it. The arguments
// are passed unchanged; the separate program applies the same rules to them
// (single instance, hand-over, link and file checks). It reports false, and
// the main program runs the app itself as before, when there is no such
// program or it does not start.
func forwardToSeparateProgram(mode string, args []string) bool {
	if buildEdition != "" || (mode != "terminal" && mode != standaloneEditorMode) {
		return false
	}
	self, err := os.Executable()
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}
	target := filepath.Join(filepath.Dir(self), programName(mode))
	if updateNamingSamePath(target, self) || !separateProgramMatches(target, mode) {
		return false
	}
	cmd := exec.Command(target, args...)
	if cwd, err := os.Getwd(); err == nil {
		cmd.Dir = cwd
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		return false
	}
	// This process was started by the user; the window it hands over to may
	// come to the front in its place (peer_send_windows.go).
	peerAllowForeground(cmd.Process.Pid)
	_ = cmd.Process.Release()
	return true
}
