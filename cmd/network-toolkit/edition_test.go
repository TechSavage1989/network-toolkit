package main

import (
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"networktoolkit/internal/model"
)

// editionFlags registers the switches of main() that applyBuildEdition and
// the launch parsers read.
func editionFlags() (*flag.FlagSet, editorLaunchOptions) {
	flags := flag.NewFlagSet("network-toolkit", flag.ContinueOnError)
	flags.SetOutput(new(strings.Builder))
	flags.Bool("terminal", false, "")
	flags.String("terminal-preset", "", "")
	flags.String(terminalLinkFlag, "", "")
	flags.Bool("terminal-view", false, "")
	flags.Bool("editor-view", false, "")
	flags.Bool("version", false, "")
	flags.Bool("no-browser", false, "")
	flags.String("data-dir", "", "")
	return flags, registerEditorLaunchOptions(flags)
}

func TestBuildEditionOpensItsOwnAppAndKeepsItsSwitches(t *testing.T) {
	file := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(file, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		edition, name string
		args          []string
		terminal      bool
		editor        bool
		link, path    bool
		refused       string
	}{
		{edition: "", name: "main without a switch", args: nil},
		{edition: "", name: "main keeps --terminal", args: []string{"--terminal"}, terminal: true},
		{edition: "", name: "main keeps --editor-standalone", args: []string{"--editor-standalone", "--open-file", file}, editor: true, path: true},
		{edition: "terminal", name: "terminal without a switch", args: nil, terminal: true},
		{edition: "terminal", name: "terminal with --terminal", args: []string{"--terminal", "--data-dir", "x"}, terminal: true},
		{edition: "terminal", name: "terminal link alone", args: []string{"--open-url", "ssh://operator@192.0.2.10"}, terminal: true, link: true},
		{edition: "terminal", name: "terminal link as registered", args: []string{"--terminal", "--open-url", "telnet://192.0.2.10:2323"}, terminal: true, link: true},
		{edition: "terminal", name: "terminal refuses the editor", args: []string{"--editor-standalone"}, refused: "--editor-standalone"},
		{edition: "terminal", name: "terminal refuses a file", args: []string{"--open-file", file}, refused: "--open-file"},
		{edition: "terminal", name: "terminal refuses the main editor", args: []string{"--editor"}, refused: "--editor "},
		{edition: "terminal", name: "terminal refuses an editor window", args: []string{"--editor-view"}, refused: "--editor-view"},
		{edition: "terminal", name: "terminal runs its detached Terminal windows", args: []string{"--terminal-view", "--no-browser"}},
		{edition: "terminal", name: "terminal view refuses a file", args: []string{"--terminal-view", "--open-file", file}, refused: "--open-file"},
		{edition: standaloneEditorMode, name: "editor runs its detached editor windows", args: []string{"--editor-view", "--no-browser"}},
		{edition: standaloneEditorMode, name: "editor refuses a Terminal window", args: []string{"--terminal-view"}, refused: "--terminal-view"},
		{edition: standaloneEditorMode, name: "editor view refuses a link", args: []string{"--editor-view", "--open-url", "ssh://192.0.2.10"}, refused: "--open-url"},
		{edition: standaloneEditorMode, name: "editor without a switch", args: nil, editor: true},
		{edition: standaloneEditorMode, name: "editor opens a file", args: []string{"--open-file", file}, editor: true, path: true},
		{edition: standaloneEditorMode, name: "editor with its switch", args: []string{"--editor-standalone", "--open-file", file}, editor: true, path: true},
		{edition: standaloneEditorMode, name: "editor with the earlier shortcut switch", args: []string{"--editor"}, editor: true},
		{edition: standaloneEditorMode, name: "editor refuses the Terminal", args: []string{"--terminal"}, refused: "--terminal "},
		{edition: standaloneEditorMode, name: "editor refuses a link", args: []string{"--open-url", "ssh://192.0.2.10"}, refused: "--open-url"},
		{edition: standaloneEditorMode, name: "editor refuses a preset", args: []string{"--terminal-preset", "e30"}, refused: "--terminal-preset"},
		{edition: standaloneEditorMode, name: "editor prints its version", args: []string{"--version"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flags, options := editionFlags()
			if err := flags.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			err := applyBuildEdition(flags, tc.edition)
			if tc.refused != "" {
				if err == nil || !strings.Contains(err.Error()+" ", tc.refused) || !strings.Contains(err.Error(), programNameFor(runtime.GOOS, tc.edition)) {
					t.Fatalf("want a refusal naming %q and the program, got %v", tc.refused, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := flags.Lookup("terminal").Value.String() == "true"; got != tc.terminal {
				t.Fatalf("terminal mode %v, want %v", got, tc.terminal)
			}
			if got := *options.standalone; got != tc.editor {
				t.Fatalf("standalone editor %v, want %v", got, tc.editor)
			}
			request, err := options.request(flags, flags.Lookup("terminal").Value.String() == "true" || flags.Lookup("version").Value.String() == "true")
			if err != nil {
				t.Fatalf("the editor launch request: %v", err)
			}
			if tc.path && (request == nil || request.Path != file) {
				t.Fatalf("the file to open was lost: %+v", request)
			}
			link, err := terminalLinkLaunch(flags, "windows")
			if err != nil || (link != nil) != tc.link {
				t.Fatalf("link %+v, %v; want a link: %v", link, err, tc.link)
			}
		})
	}
}

func TestProgramNamesAndUpdateMembers(t *testing.T) {
	for edition, want := range map[string]string{"": "Network_Toolkit.exe", standaloneEditorMode: "Network_Toolkit_TextEditor.exe", "terminal": "Network_Toolkit_Terminal.exe"} {
		if got := programNameFor("windows", edition); got != want {
			t.Fatalf("%q: %s, want %s", edition, got, want)
		}
	}
	if programNameFor("linux", "") != "network-toolkit" || programNameFor("linux", "terminal") != "network-toolkit-terminal" || programNameFor("linux", standaloneEditorMode) != "network-toolkit-text-editor" {
		t.Fatal("Linux program names changed")
	}
	// The separate programs' Update ZIP members are never official versioned
	// names of the main program, which released updaters read.
	for _, edition := range updateCompanionEditions {
		member := updateCompanionMember("1.21.45", edition)
		if _, official := updateOfficialExecutableVersion(member); official {
			t.Fatalf("%s reads as the main program's versioned name", member)
		}
		if !strings.HasSuffix(member, "_1.21.45_Windows_x64.exe") {
			t.Fatalf("unexpected member %s", member)
		}
	}
}

// Each program's resource object names its version and original file name
// and no other program's.
func TestProgramResourcesNameTheirProgram(t *testing.T) {
	objects := map[string]string{
		"":                   "resource_windows_amd64.syso",
		standaloneEditorMode: filepath.Join("..", "..", "internal", "winres", "editor", "resource_windows_amd64.syso"),
		"terminal":           filepath.Join("..", "..", "internal", "winres", "terminal", "resource_windows_amd64.syso"),
	}
	for edition, path := range objects {
		resource, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for other := range objects {
			err := programResourceMatches(resource, model.Version, programNameFor("windows", other))
			if (err == nil) != (other == edition) {
				t.Fatalf("resource of %q matched as %q: %v", edition, other, err)
			}
		}
		if programResourceMatches(resource, "0.0.1", programNameFor("windows", edition)) == nil {
			t.Fatalf("resource of %q matched another version", edition)
		}
	}
}

func writeProgram(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0700); err != nil {
		t.Fatal(err)
	}
}

func TestStandaloneProgramPrefersTheSeparateProgramAndFallsBack(t *testing.T) {
	dir := t.TempDir()
	main, editor, terminal := filepath.Join(dir, programName("")), filepath.Join(dir, programName(standaloneEditorMode)), filepath.Join(dir, programName("terminal"))
	writeProgram(t, main)
	got := func(self, edition, mode string) string {
		t.Helper()
		path, err := standaloneProgramFrom(self, edition, mode)
		if err != nil {
			return "error: " + err.Error()
		}
		return path
	}
	// Only Network_Toolkit.exe: it starts both apps itself, as before.
	if got(main, "", "terminal") != main || got(main, "", standaloneEditorMode) != main {
		t.Fatal("the main program alone must start both apps")
	}
	writeProgram(t, editor)
	writeProgram(t, terminal)
	if got(main, "", "terminal") != terminal || got(main, "", standaloneEditorMode) != editor {
		t.Fatal("the separate programs beside the main program must be started")
	}
	// Peer send from a separate program starts the other separate program.
	if got(terminal, "terminal", standaloneEditorMode) != editor || got(editor, standaloneEditorMode, "terminal") != terminal {
		t.Fatal("a separate program must start the other one")
	}
	// A separate program of another version, a link or a folder is skipped.
	original := separateProgramMatches
	t.Cleanup(func() { separateProgramMatches = original })
	separateProgramMatches = func(path, edition string) bool { return path != editor && original(path, edition) }
	if got(main, "", standaloneEditorMode) != main {
		t.Fatal("a separate program that does not match must not be started by the main program")
	}
	if got(terminal, "terminal", standaloneEditorMode) != main {
		t.Fatal("the Terminal program must fall back to Network_Toolkit.exe --editor-standalone")
	}
	separateProgramMatches = original
	if err := os.Remove(editor); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(main, editor); err != nil {
		t.Fatal(err)
	}
	if got(main, "", standaloneEditorMode) != main {
		t.Fatal("a link must never be started as a separate program")
	}
	// A separate program alone: no Network_Toolkit.exe to fall back to.
	if err := os.Remove(main); err != nil {
		t.Fatal(err)
	}
	if path := got(terminal, "terminal", standaloneEditorMode); !strings.HasPrefix(path, "error: ") {
		t.Fatalf("nothing to start, got %s", path)
	}
	// A separate program starting its own app (never the case today) is itself.
	if got(terminal, "terminal", "terminal") != terminal {
		t.Fatal("a separate program is its own app's program")
	}
}

func TestForwardTargetOnlyForMatchingSeparatePrograms(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, programName(""))
	writeProgram(t, main)
	if _, ok := forwardTarget(main, "terminal"); ok {
		t.Fatal("forwarded without a separate program")
	}
	terminal := filepath.Join(dir, programName("terminal"))
	if err := os.WriteFile(terminal, []byte("not executable"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, ok := forwardTarget(main, "terminal"); ok && runtime.GOOS != "windows" {
		t.Fatal("forwarded to a file that is not a program")
	}
	writeProgram(t, filepath.Join(dir, "other"))
	if err := os.Remove(terminal); err != nil {
		t.Fatal(err)
	}
	writeProgram(t, terminal)
	if target, ok := forwardTarget(main, "terminal"); !ok || target != terminal {
		t.Fatalf("not forwarded to the separate Terminal: %s %v", target, ok)
	}
	if _, ok := forwardTarget(main, standaloneEditorMode); ok {
		t.Fatal("forwarded the Text Editor to a missing program")
	}
	// Forwarding never targets the running file itself.
	if _, ok := forwardTarget(terminal, "terminal"); ok {
		t.Fatal("a program must not forward to itself")
	}
}

func TestTerminalLinkRegistrationOfTheProgramFolderIsCurrent(t *testing.T) {
	dir := `C:\Users\operator\AppData\Local\Programs\Network Toolkit`
	main := dir + `\Network_Toolkit.exe`
	terminal := dir + `\Network_Toolkit_Terminal.exe`
	registered := `"` + main + `" --terminal --open-url "%1"`
	if runtime.GOOS != "windows" {
		// filepath on this system does not split Windows paths; check the
		// same rule with this system's separator.
		dir = filepath.Join(t.TempDir(), "Network Toolkit")
		main, terminal = filepath.Join(dir, "Network_Toolkit.exe"), filepath.Join(dir, "Network_Toolkit_Terminal.exe")
		registered = `"` + main + `" --terminal --open-url "%1"`
	}
	for _, exe := range []string{main, terminal} {
		if !terminalLinkCommandIsCurrent(registered, exe) {
			t.Fatalf("Setup's registration is not current for %s", exe)
		}
	}
	if !terminalLinkCommandIsCurrent(`"`+terminal+`" --open-url "%1"`, main) {
		t.Fatal("the separate Terminal's own command is not current")
	}
	other := filepath.Join(filepath.Dir(dir), "Portable", "Network_Toolkit.exe")
	if terminalLinkCommandIsCurrent(`"`+other+`" --terminal --open-url "%1"`, terminal) {
		t.Fatal("another copy's registration counts as this one")
	}
	if terminalLinkCommandIsCurrent(`"`+main+`" --terminal --open-url "%1" --extra`, main) {
		t.Fatal("a changed command counts as Setup's")
	}
}

// Owner request (connected separate program): a detached window runs as the
// separate program of its app, with that program's icon and taskbar name,
// when the program of this exact version is beside the running one and only
// this user and the system can change it and its folder; otherwise the
// running program when that is the main program, else Network_Toolkit.exe of
// this version; always the running program on Linux. The chosen file is held
// open (no write, rename or delete) until the window has started.
func TestDetachedViewProgramPrefersTheSeparateProgramOfThisVersion(t *testing.T) {
	dir := t.TempDir()
	main, editor, terminal := filepath.Join(dir, programNameFor("windows", "")), filepath.Join(dir, programNameFor("windows", standaloneEditorMode)), filepath.Join(dir, programNameFor("windows", "terminal"))
	writeProgram(t, main)
	got := func(goos, self, edition, mode string) string {
		t.Helper()
		path, release, err := detachedViewProgramFrom(goos, self, edition, mode)
		if err != nil {
			return "error: " + err.Error()
		}
		release()
		return path
	}
	if got("windows", main, "", standaloneEditorMode) != main || got("windows", main, "", "terminal") != main {
		t.Fatal("without the separate programs the main program runs both windows")
	}
	writeProgram(t, editor)
	writeProgram(t, terminal)
	if path := got("windows", main, "", standaloneEditorMode); path != editor {
		t.Fatalf("the detached editor must run as %s, got %s", editor, path)
	}
	if path := got("windows", main, "", "terminal"); path != terminal {
		t.Fatalf("the detached terminal must run as %s, got %s", terminal, path)
	}
	// The standalone Terminal's pop-out runs as itself; an editor window it
	// opens runs as the Text Editor program, else as Network_Toolkit.exe
	// (a separate program never runs the other app's window).
	if got("windows", terminal, "terminal", "terminal") != terminal || got("windows", terminal, "terminal", standaloneEditorMode) != editor {
		t.Fatal("a Terminal program's windows")
	}
	// Linux ships only the main program: never a sibling.
	if got("linux", main, "", "terminal") != main {
		t.Fatal("Linux must keep the running program")
	}
	// Another version, a program or folder others can change: never handed a view token.
	matches, writable := separateProgramMatches, detachedViewWritable
	t.Cleanup(func() { separateProgramMatches, detachedViewWritable = matches, writable })
	separateProgramMatches = func(path, edition string) bool { return path != editor && matches(path, edition) }
	if got("windows", main, "", standaloneEditorMode) != main || got("windows", main, "", "terminal") != terminal {
		t.Fatal("a Text Editor program of another version must not run the window")
	}
	if got("windows", terminal, "terminal", standaloneEditorMode) != main {
		t.Fatal("the Terminal program must fall back to Network_Toolkit.exe for an editor window")
	}
	separateProgramMatches = matches
	detachedViewWritable = func(path string) (string, error) {
		if path == terminal {
			return "Users", nil
		}
		return "", nil
	}
	if got("windows", main, "", "terminal") != main {
		t.Fatal("a Terminal program others can change must not run the window")
	}
	detachedViewWritable = func(path string) (string, error) {
		if path == dir {
			return "", os.ErrPermission
		}
		return "", nil
	}
	if got("windows", main, "", "terminal") != main {
		t.Fatal("a folder whose permissions cannot be read must not be trusted")
	}
	if path := got("windows", terminal, "terminal", standaloneEditorMode); !strings.HasPrefix(path, "error: ") {
		t.Fatalf("nothing trusted to run the editor window, got %s", path)
	}
	detachedViewWritable = writable
	if err := os.Remove(terminal); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(main, terminal); err != nil {
		t.Fatal(err)
	}
	if got("windows", main, "", "terminal") != main {
		t.Fatal("a link must never run a detached window")
	}
}

// The separate programs never forward a detached window anywhere: the view
// role runs in the process that was started.
func TestSeparateProgramsDoNotForwardDetachedWindows(t *testing.T) {
	for edition, args := range map[string][]string{"terminal": terminalViewChildArguments(false), standaloneEditorMode: editorViewChildArguments(true)} {
		{
			flags, options := editionFlags()
			if err := flags.Parse(args); err != nil {
				t.Fatal(err)
			}
			if err := applyBuildEdition(flags, edition); err != nil {
				t.Fatalf("%s %v: %v", edition, args, err)
			}
			if flags.Lookup("terminal").Value.String() == "true" || *options.standalone {
				t.Fatalf("%s %v: a detached window must not also open the standalone app", edition, args)
			}
		}
	}
}
