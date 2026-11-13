package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"networktoolkit/internal/model"
)

// The separate programs are added without asking only in the start that
// follows an update the user approved (docs/SEPARATE_EXECUTABLES.md).
func autoCompanionApplication(t *testing.T, running []byte, healthDir string) (*application, string, *[]string) {
	t.Helper()
	dir := withRunningProgram(t, running)
	original := companionPlatformSupported
	t.Cleanup(func() { companionPlatformSupported = original })
	companionPlatformSupported = func() bool { return true }
	var shortcutCalls []string
	originalShortcuts := updateCompanionShortcuts
	t.Cleanup(func() { updateCompanionShortcuts = originalShortcuts })
	updateCompanionShortcuts = func(d string) error { shortcutCalls = append(shortcutCalls, d); return nil }
	a := testApplication(t)
	a.updateHealthDir = healthDir
	return a, dir, &shortcutCalls
}

func companionFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestSeparateProgramsAreAddedAfterAnApprovedUpdate(t *testing.T) {
	// The signal of the helpers of 1.21.46 and older (the owner's case): they
	// restart the new program with --update-health <stage folder> and confirm
	// with this exact text in update-result.txt (applyUpdatePlanWithOperations
	// at tag v1.21.46), which awaitUpdateCommit accepts.
	if !updateCommittedResult("Installed "+model.Version+" and confirmed the application interface started. Previous executable retained as previous.exe.\n", model.Version) {
		t.Fatal("the released helper's confirmation is not accepted")
	}
	if args := updateRestartArgs([]string{"--update-health", "x", "--data-dir", "d"}); strings.Contains(strings.Join(args, " "), "update-health") {
		t.Fatalf("a later restart would repeat the automatic install: %v", args)
	}
	fixture := newCompanionReleaseFixture(t)
	a, dir, shortcuts := autoCompanionApplication(t, fixture.programs.main, filepath.Join(t.TempDir(), ".network-toolkit-update-x"))
	a.installCompanionsAfterUpdate(context.Background(), true, fixture.client, updateReleaseTagURL+model.Version)
	for name, data := range map[string][]byte{"Network_Toolkit_TextEditor.exe": fixture.programs.editor, "Network_Toolkit_Terminal.exe": fixture.programs.terminal} {
		if b, err := os.ReadFile(filepath.Join(dir, name)); err != nil || !bytes.Equal(b, data) {
			t.Fatalf("%s was not added: %v", name, err)
		}
	}
	a.updater.mu.Lock()
	state := a.updater.companionsInstall
	status := a.companionStatusLocked()
	a.updater.mu.Unlock()
	if state == nil || !state.Automatic || state.State != "added" || state.Reason != "" {
		t.Fatalf("state %+v", state)
	}
	if status == nil || status.Needed {
		t.Fatalf("still needed: %+v", status)
	}
	if len(*shortcuts) != 1 || (*shortcuts)[0] != dir {
		t.Fatalf("Start menu shortcuts of %s not offered: %v", dir, *shortcuts)
	}
	// Once per update: a second call (the same start) does nothing more.
	fixture.requests = nil
	a.installCompanionsAfterUpdate(context.Background(), true, fixture.client, updateReleaseTagURL+model.Version)
	if len(fixture.requests) != 0 || len(*shortcuts) != 1 {
		t.Fatalf("ran twice: %v %v", fixture.requests, *shortcuts)
	}
}

func TestSeparateProgramsAreNeverFetchedWithoutAnApprovedUpdate(t *testing.T) {
	fixture := newCompanionReleaseFixture(t)
	for _, tc := range []struct {
		name      string
		health    string
		confirmed bool
		platform  bool
		mode      string
	}{
		{"a plain start (no update before it)", "", true, true, "main"},
		{"an update the helper has not confirmed", "health", false, true, "main"},
		{"not the Windows x64 main program", "health", true, false, "main"},
		{"a standalone Terminal", "health", true, true, "terminal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			health := ""
			if tc.health != "" {
				health = filepath.Join(t.TempDir(), ".network-toolkit-update-x")
			}
			a, dir, shortcuts := autoCompanionApplication(t, fixture.programs.main, health)
			companionPlatformSupported = func() bool { return tc.platform }
			a.windowMode = tc.mode
			fixture.requests = nil
			a.installCompanionsAfterUpdate(context.Background(), tc.confirmed, fixture.client, updateReleaseTagURL+model.Version)
			if len(fixture.requests) != 0 {
				t.Fatalf("downloaded without an approved update: %v", fixture.requests)
			}
			if names := companionFiles(t, dir); len(names) != 1 {
				t.Fatalf("programs were added: %v", names)
			}
			if a.updater.companionsInstall != nil || len(*shortcuts) != 0 {
				t.Fatalf("state %+v shortcuts %v", a.updater.companionsInstall, *shortcuts)
			}
		})
	}
	t.Run("already current: nothing is fetched", func(t *testing.T) {
		a, dir, _ := autoCompanionApplication(t, fixture.programs.main, "health")
		for name, data := range map[string][]byte{"Network_Toolkit_TextEditor.exe": fixture.programs.editor, "Network_Toolkit_Terminal.exe": fixture.programs.terminal} {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0700); err != nil {
				t.Fatal(err)
			}
		}
		fixture.requests = nil
		a.installCompanionsAfterUpdate(context.Background(), true, fixture.client, updateReleaseTagURL+model.Version)
		if len(fixture.requests) != 0 || a.updater.companionsInstall != nil {
			t.Fatalf("fetched %v, state %+v", fixture.requests, a.updater.companionsInstall)
		}
	})
}

func TestSeparateProgramsFailureKeepsTheButtonAndIsNotRetried(t *testing.T) {
	fixture := newCompanionReleaseFixture(t)
	for _, tc := range []struct {
		name    string
		prepare func(f *companionReleaseFixture) []byte // returns the running program
		want    string
	}{
		{"offline", func(f *companionReleaseFixture) []byte { f.offline = true; return f.programs.main }, "synthetic: no network connection"},
		{"a changed download", func(f *companionReleaseFixture) []byte {
			f.serve = append(append([]byte{}, f.archive...), 0)
			return f.programs.main
		}, "Checksum"},
		{"another build of the main program", func(f *companionReleaseFixture) []byte {
			return append(updateTestProgramPE(model.Version, "Network_Toolkit.exe"), 0)
		}, "not the official build"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() { fixture.offline, fixture.serve = false, fixture.archive }()
			running := tc.prepare(fixture)
			a, dir, shortcuts := autoCompanionApplication(t, running, "health")
			a.installCompanionsAfterUpdate(context.Background(), true, fixture.client, updateReleaseTagURL+model.Version)
			if names := companionFiles(t, dir); len(names) != 1 {
				t.Fatalf("programs were placed after a failed check: %v", names)
			}
			a.updater.mu.Lock()
			state, status := a.updater.companionsInstall, a.companionStatusLocked()
			a.updater.mu.Unlock()
			if state == nil || state.State != "failed" || !strings.Contains(state.Reason, tc.want) {
				t.Fatalf("state %+v, want a failure with %q", state, tc.want)
			}
			if status == nil || !status.Needed {
				t.Fatal("the manual button is not offered after a failure")
			}
			if len(*shortcuts) != 0 {
				t.Fatalf("shortcuts changed after a failure: %v", *shortcuts)
			}
			// Not retried until the next update.
			fixture.offline, fixture.serve, fixture.requests = false, fixture.archive, nil
			a.installCompanionsAfterUpdate(context.Background(), true, fixture.client, updateReleaseTagURL+model.Version)
			if len(fixture.requests) != 0 {
				t.Fatalf("retried: %v", fixture.requests)
			}
			if a.updater.progress != nil {
				t.Fatalf("progress left behind: %+v", a.updater.progress)
			}
		})
	}
}

func TestSeparateProgramsShortcutFailureIsReportedNotFatal(t *testing.T) {
	fixture := newCompanionReleaseFixture(t)
	a, dir, _ := autoCompanionApplication(t, fixture.programs.main, "health")
	updateCompanionShortcuts = func(string) error { return errors.New("synthetic shortcut failure") }
	a.installCompanionsAfterUpdate(context.Background(), true, fixture.client, updateReleaseTagURL+model.Version)
	if names := companionFiles(t, dir); len(names) != 3 {
		t.Fatalf("programs: %v", names)
	}
	state := a.updater.companionsInstall
	if state == nil || state.State != "added" || !strings.Contains(state.Shortcuts, "synthetic shortcut failure") || state.StartMenu != "" {
		t.Fatalf("state %+v", state)
	}
	// An uninstaller of 1.21.46 or older: recorded, not an error.
	a, _, _ = autoCompanionApplication(t, fixture.programs.main, "health")
	updateCompanionShortcuts = func(string) error { return errCompanionShortcutsNextSetup }
	a.installCompanionsAfterUpdate(context.Background(), true, fixture.client, updateReleaseTagURL+model.Version)
	state = a.updater.companionsInstall
	if state == nil || state.State != "added" || state.Shortcuts != "" || state.StartMenu != "next-setup" {
		t.Fatalf("state %+v", state)
	}
}

// While the programs are being added, no other update action that uses the
// network starts (the network slot is reserved, as for the manual button);
// the release page and the status still work. While they are pending (the
// update is not confirmed yet), only the manual button is refused.
func TestSeparateProgramsRunningBlocksOtherUpdateActions(t *testing.T) {
	a := testApplication(t)
	if msg := a.companionsBlockLocked("/api/updates/download"); msg != "" {
		t.Fatalf("blocked while idle: %s", msg)
	}
	a.updater.companionsInstall = &companionInstallState{Automatic: true, State: "pending"}
	if a.companionsBlockLocked("/api/updates/companions") == "" {
		t.Fatal("the manual install allowed while the automatic one is pending")
	}
	for _, path := range []string{"/api/updates/check", "/api/updates/download", "/api/updates/open-releases"} {
		if msg := a.companionsBlockLocked(path); msg != "" {
			t.Fatalf("%s blocked while pending: %s", path, msg)
		}
	}
	a.updater.companionsInstall = &companionInstallState{Automatic: true, State: "running"}
	for _, path := range []string{"/api/updates/check", "/api/updates/download", "/api/updates/import", "/api/updates/install", "/api/updates/companions"} {
		if a.companionsBlockLocked(path) == "" {
			t.Fatalf("%s allowed while the separate programs are being added", path)
		}
	}
	if msg := a.companionsBlockLocked("/api/updates/open-releases"); msg != "" {
		t.Fatalf("release page blocked: %s", msg)
	}
	status := a.updateStatusLocked()
	if got, ok := status["companionsInstall"].(*companionInstallState); !ok || got.State != "running" {
		t.Fatalf("status does not report the running install: %#v", status["companionsInstall"])
	}
}

// The network slot is reserved for the whole automatic install, so "Install
// with Setup" (fullInstallReserveNetwork) cannot run beside it.
func TestSeparateProgramsReserveTheNetworkSlot(t *testing.T) {
	fixture := newCompanionReleaseFixture(t)
	a, _, _ := autoCompanionApplication(t, fixture.programs.main, "health")
	var refusals, requests int
	fixture.onRequest = func() {
		requests++
		if a.fullInstallReserveNetwork() != nil {
			refusals++
		} else {
			a.fullInstallReleaseNetwork()
		}
	}
	a.installCompanionsAfterUpdate(context.Background(), true, fixture.client, updateReleaseTagURL+model.Version)
	if requests == 0 || refusals != requests {
		t.Fatalf("Install with Setup could start during %d of %d requests", requests-refusals, requests)
	}
	if a.updater.networkBusy {
		t.Fatal("the network slot was not released")
	}
	if a.updater.companionsInstall == nil || a.updater.companionsInstall.State != "added" {
		t.Fatalf("state %+v", a.updater.companionsInstall)
	}
}

// The page sees the install coming from the first status read: "pending"
// from the start of a run that qualifies, until it runs or does not.
func TestSeparateProgramsArePendingFromTheStart(t *testing.T) {
	fixture := newCompanionReleaseFixture(t)
	t.Run("an update start that qualifies", func(t *testing.T) {
		a, _, _ := autoCompanionApplication(t, fixture.programs.main, "health")
		a.markCompanionsPending()
		if s := a.updater.companionsInstall; s == nil || s.State != "pending" || !s.Automatic {
			t.Fatalf("state %+v", s)
		}
		a.installCompanionsAfterUpdate(context.Background(), true, fixture.client, updateReleaseTagURL+model.Version)
		if s := a.updater.companionsInstall; s == nil || s.State != "added" {
			t.Fatalf("pending did not run: %+v", s)
		}
	})
	t.Run("a plain start", func(t *testing.T) {
		a, _, _ := autoCompanionApplication(t, fixture.programs.main, "")
		a.markCompanionsPending()
		if a.updater.companionsInstall != nil {
			t.Fatalf("pending without an update: %+v", a.updater.companionsInstall)
		}
	})
	for _, end := range []string{"unconfirmed", "cleared", "already current"} {
		t.Run("pending ends: "+end, func(t *testing.T) {
			a, dir, _ := autoCompanionApplication(t, fixture.programs.main, "health")
			a.markCompanionsPending()
			fixture.requests = nil
			switch end {
			case "unconfirmed":
				a.installCompanionsAfterUpdate(context.Background(), false, fixture.client, updateReleaseTagURL+model.Version)
			case "cleared":
				a.clearCompanionsPending()
			case "already current":
				for name, data := range map[string][]byte{"Network_Toolkit_TextEditor.exe": fixture.programs.editor, "Network_Toolkit_Terminal.exe": fixture.programs.terminal} {
					if err := os.WriteFile(filepath.Join(dir, name), data, 0700); err != nil {
						t.Fatal(err)
					}
				}
				a.installCompanionsAfterUpdate(context.Background(), true, fixture.client, updateReleaseTagURL+model.Version)
			}
			if a.updater.companionsInstall != nil || len(fixture.requests) != 0 {
				t.Fatalf("state %+v, requests %v", a.updater.companionsInstall, fixture.requests)
			}
		})
	}
}

// Start menu shortcuts: only Setup's own, of the installed copy, by the rule
// of IsOwnedShortcut in scripts/installer/windows.nsi.
type fakeCompanionShortcut struct{ target, args string }

func TestCompanionShortcutsRetargetOnlySetupsOwnShortcuts(t *testing.T) {
	install := `C:\Users\Example\AppData\Local\Programs\Network Toolkit`
	programs := `C:\Users\Example\AppData\Roaming\Microsoft\Windows\Start Menu\Programs`
	folder := filepath.Join(programs, "Network Toolkit")
	editorLink, terminalLink := filepath.Join(folder, "Network Toolkit Text Editor.lnk"), filepath.Join(folder, "Network Toolkit Terminal.lnk")
	main := install + `\Network_Toolkit.exe`
	type result struct {
		retargeted map[string]string
		err        error
	}
	setupVersion, setupVersionFound := companionShortcutUninstallerVersion, true
	run := func(location string, found bool, links map[string]fakeCompanionShortcut, readErr error) result {
		r := result{retargeted: map[string]string{}}
		ops := companionShortcutOps{
			installLocation: func() (string, bool, error) { return location, found, nil },
			setupVersion:    func() (string, bool, error) { return setupVersion, setupVersionFound, nil },
			programsFolder:  func() (string, error) { return programs, nil },
			exists: func(link string) (bool, error) {
				_, ok := links[link]
				return ok, nil
			},
			read: func(link string) (string, string, error) {
				if readErr != nil {
					return "", "", readErr
				}
				l := links[link]
				return l.target, l.args, nil
			},
			retarget: func(link, target, args, program string) error {
				if l := links[link]; l.target != target || l.args != args {
					return errors.New("retarget was not given what was read")
				}
				r.retargeted[link] = program
				return nil
			},
		}
		r.err = updateRetargetCompanionShortcuts(install, ops)
		return r
	}
	owned := map[string]fakeCompanionShortcut{editorLink: {main, "--editor-standalone"}, terminalLink: {main, "--terminal"}}
	r := run(install, true, owned, nil)
	if r.err != nil || r.retargeted[editorLink] != install+`\Network_Toolkit_TextEditor.exe` || r.retargeted[terminalLink] != install+`\Network_Toolkit_Terminal.exe` {
		t.Fatalf("owned shortcuts: %+v", r)
	}
	// The released --editor form and a differently cased path are Setup's too.
	r = run(strings.ToLower(install)+`\`, true, map[string]fakeCompanionShortcut{editorLink: {strings.ToUpper(main), "--editor"}}, nil)
	if r.err != nil || len(r.retargeted) != 1 {
		t.Fatalf("legacy form: %+v", r)
	}
	for _, tc := range []struct {
		name  string
		links map[string]fakeCompanionShortcut
	}{
		{"arguments the user changed", map[string]fakeCompanionShortcut{editorLink: {main, "--editor-standalone --data-dir D:\\Data"}, terminalLink: {main, ""}}},
		{"another program", map[string]fakeCompanionShortcut{editorLink: {`D:\Portable\Network_Toolkit.exe`, "--editor-standalone"}, terminalLink: {`C:\Windows\System32\cmd.exe`, "--terminal"}}},
		{"already the separate programs", map[string]fakeCompanionShortcut{editorLink: {install + `\Network_Toolkit_TextEditor.exe`, ""}, terminalLink: {install + `\Network_Toolkit_Terminal.exe`, ""}}},
		{"swapped arguments", map[string]fakeCompanionShortcut{editorLink: {main, "--terminal"}, terminalLink: {main, "--editor-standalone"}}},
		{"no shortcuts", map[string]fakeCompanionShortcut{}},
	} {
		if r := run(install, true, tc.links, nil); r.err != nil || len(r.retargeted) != 0 {
			t.Fatalf("%s: %+v", tc.name, r)
		}
	}
	// A portable copy (no Setup, or Setup installed elsewhere) has no Setup shortcuts.
	if r := run("", false, owned, nil); r.err != nil || len(r.retargeted) != 0 {
		t.Fatalf("portable: %+v", r)
	}
	if r := run(`D:\Elsewhere`, true, owned, nil); r.err != nil || len(r.retargeted) != 0 {
		t.Fatalf("other install: %+v", r)
	}
	// A shortcut that cannot be read is left alone and reported.
	if r := run(install, true, owned, errors.New("synthetic read failure")); r.err == nil || len(r.retargeted) != 0 {
		t.Fatalf("unreadable: %+v", r)
	}
	// The uninstaller in the folder is Setup's, never replaced by an in-app
	// update. One of 1.21.46 or older removes only shortcuts that start
	// Network_Toolkit.exe, so the shortcuts wait for the next Setup.
	for _, tc := range []struct {
		version string
		found   bool
	}{{"1.21.46", true}, {"1.17.63", true}, {"", false}, {"", true}, {"1.21.047", true}, {"1.21.47 ", true}, {"v1.21.47", true}, {"1.21", true}} {
		setupVersion, setupVersionFound = tc.version, tc.found
		if r := run(install, true, owned, nil); !errors.Is(r.err, errCompanionShortcutsNextSetup) || len(r.retargeted) != 0 {
			t.Fatalf("Setup %q (found %v): %+v", tc.version, tc.found, r)
		}
		// Nothing to change: nothing to report.
		if r := run(install, true, map[string]fakeCompanionShortcut{}, nil); r.err != nil {
			t.Fatalf("Setup %q without shortcuts: %+v", tc.version, r)
		}
	}
	for _, version := range []string{"1.21.47", "1.21.48", "1.22.0", "2.0.0"} {
		setupVersion, setupVersionFound = version, true
		if r := run(install, true, owned, nil); r.err != nil || len(r.retargeted) != 2 {
			t.Fatalf("Setup %s: %+v", version, r)
		}
	}
}

// The document icon (index 3) of files of the Text Editor's types, written by
// Setup; an in-app update's first start repairs it for the registration of
// this program only.
func TestDocumentIconRepairTouchesOnlyThisProgramsProgID(t *testing.T) {
	command := `"` + testCopyA + `" --editor-standalone --open-file "%1"`
	t.Run("this program's ProgID with the editor icon", func(t *testing.T) {
		reg := fixtureRegistry()
		reg.SetString(editorRegProgKey, "", editorRegProgName)
		reg.SetString(editorRegProgKey+`\DefaultIcon`, "", `"`+testCopyA+`",1`)
		reg.SetString(editorRegProgKey+`\shell\open\command`, "", command)
		before := reg.dump()
		platform, notified := testEditorPlatform(reg)
		changed, err := editorRegRepairDocumentIcon(platform, strings.ToLower(testCopyA))
		if err != nil || !changed || *notified != 1 {
			t.Fatalf("changed=%v err=%v notified=%d", changed, err, *notified)
		}
		if got := reg.str(t, editorRegProgKey+`\DefaultIcon`, ""); got != `"`+testCopyA+`",3` {
			t.Fatalf("DefaultIcon %q", got)
		}
		// Nothing else changed.
		reg.SetString(editorRegProgKey+`\DefaultIcon`, "", `"`+testCopyA+`",1`)
		if reg.dump() != before {
			t.Fatalf("other values changed:\n%s\n%s", before, reg.dump())
		}
		// Already right: nothing is written, no notification.
		reg.SetString(editorRegProgKey+`\DefaultIcon`, "", `"`+testCopyA+`",3`)
		reg.failWrite = editorRegProgKey + `\DefaultIcon`
		if changed, err := editorRegRepairDocumentIcon(platform, testCopyA); err != nil || changed || *notified != 1 {
			t.Fatalf("rewrote a current icon: %v %v", changed, err)
		}
	})
	t.Run("another program's ProgID", func(t *testing.T) {
		reg := fixtureRegistry()
		reg.SetString(editorRegProgKey+`\DefaultIcon`, "", `"`+testCopyB+`",1`)
		reg.SetString(editorRegProgKey+`\shell\open\command`, "", `"`+testCopyB+`" --editor-standalone --open-file "%1"`)
		before := reg.dump()
		platform, notified := testEditorPlatform(reg)
		if changed, err := editorRegRepairDocumentIcon(platform, testCopyA); err != nil || changed || *notified != 0 || reg.dump() != before {
			t.Fatalf("changed another program's registration: %v %v", changed, err)
		}
	})
	t.Run("no ProgID", func(t *testing.T) {
		reg := fixtureRegistry()
		before := reg.dump()
		platform, notified := testEditorPlatform(reg)
		if changed, err := editorRegRepairDocumentIcon(platform, testCopyA); err != nil || changed || *notified != 0 || reg.dump() != before {
			t.Fatalf("created a registration: %v %v", changed, err)
		}
	})
	t.Run("a read that fails changes nothing", func(t *testing.T) {
		reg := fixtureRegistry()
		reg.SetString(editorRegProgKey+`\DefaultIcon`, "", `"`+testCopyA+`",1`)
		reg.SetString(editorRegProgKey+`\shell\open\command`, "", command)
		reg.readFault = editorRegProgKey + `\shell\open\command`
		before := reg.dump()
		platform, _ := testEditorPlatform(reg)
		if changed, err := editorRegRepairDocumentIcon(platform, testCopyA); err == nil || changed || reg.dump() != before {
			t.Fatalf("changed after a failed read: %v %v", changed, err)
		}
	})
	t.Run("the program file must be Network_Toolkit.exe", func(t *testing.T) {
		reg := fixtureRegistry()
		other := `C:\Tools\Network Toolkit\Network_Toolkit_TextEditor.exe`
		reg.SetString(editorRegProgKey+`\DefaultIcon`, "", `"`+other+`",1`)
		reg.SetString(editorRegProgKey+`\shell\open\command`, "", `"`+other+`" --editor-standalone --open-file "%1"`)
		reg.SetString(editorRegApplication+`\DefaultIcon`, "", `"`+other+`",1`)
		reg.SetString(editorRegApplication+`\shell\open\command`, "", `"`+other+`" --editor-standalone --open-file "%1"`)
		platform, _ := testEditorPlatform(reg)
		if changed, err := editorRegRepairDocumentIcon(platform, other); err != nil || changed {
			t.Fatalf("repaired a registration of another program file: %v %v", changed, err)
		}
	})
}

// Explorer shows the DefaultIcon of Applications\Network_Toolkit.exe as the
// icon of every file the user associated through that "Open with" entry, so
// Setup and the portable registration write the document icon there too
// (1.21.51 and older wrote the editor icon, 1). The first start after an
// update repairs the entry Setup or the portable registration wrote for this
// program: its command is the Text Editor command of the running program.
func TestDocumentIconRepairOfTheOpenWithEntry(t *testing.T) {
	command := editorRegCommand(testCopyA)
	t.Run("the entry a registration wrote for this program", func(t *testing.T) {
		reg := fixtureRegistry()
		reg.SetString(editorRegApplication, "FriendlyAppName", editorRegAppName)
		reg.SetString(editorRegApplication+`\DefaultIcon`, "", `"`+testCopyA+`",1`)
		reg.SetString(editorRegApplication+`\shell\open\command`, "", command)
		before := reg.dump()
		platform, notified := testEditorPlatform(reg)
		changed, err := editorRegRepairDocumentIcon(platform, strings.ToLower(testCopyA))
		if err != nil || !changed || *notified != 1 {
			t.Fatalf("changed=%v err=%v notified=%d", changed, err, *notified)
		}
		if got := reg.str(t, editorRegApplication+`\DefaultIcon`, ""); got != `"`+testCopyA+`",3` {
			t.Fatalf("DefaultIcon %q", got)
		}
		// The verb icon keeps the program icon in Open with (1.21.51 and
		// older wrote none).
		if got := reg.str(t, editorRegApplication+`\shell\open`, "Icon"); got != `"`+testCopyA+`",1` {
			t.Fatalf("verb Icon %q", got)
		}
		reg.SetString(editorRegApplication+`\DefaultIcon`, "", `"`+testCopyA+`",1`)
		reg.DeleteValue(editorRegApplication+`\shell\open`, "Icon")
		if reg.dump() != before {
			t.Fatalf("other values changed:\n%s\n%s", before, reg.dump())
		}
		reg.SetString(editorRegApplication+`\DefaultIcon`, "", `"`+testCopyA+`",3`)
		reg.SetString(editorRegApplication+`\shell\open`, "Icon", `"`+testCopyA+`",1`)
		if changed, err := editorRegRepairDocumentIcon(platform, testCopyA); err != nil || changed || *notified != 1 {
			t.Fatalf("rewrote current icons: %v %v", changed, err)
		}
		// Only the verb icon missing: only it is written.
		reg.DeleteValue(editorRegApplication+`\shell\open`, "Icon")
		if changed, err := editorRegRepairDocumentIcon(platform, testCopyA); err != nil || !changed || *notified != 2 {
			t.Fatalf("missing verb icon: %v %v", changed, err)
		}
		if got := reg.str(t, editorRegApplication+`\shell\open`, "Icon"); got != `"`+testCopyA+`",1` {
			t.Fatalf("verb Icon %q", got)
		}
	})
	// Only a value that is absent or exactly what 1.21.44 to 1.21.51 wrote is
	// written: the editor icon as DefaultIcon, no verb Icon. An icon the user
	// set on the entry stays.
	t.Run("icons the user set stay", func(t *testing.T) {
		reg := fixtureRegistry()
		reg.SetString(editorRegApplication, "FriendlyAppName", editorRegAppName)
		reg.SetString(editorRegApplication+`\DefaultIcon`, "", `C:\Icons\files.ico`)
		reg.SetString(editorRegApplication+`\shell\open`, "Icon", `C:\Icons\program.ico`)
		reg.SetString(editorRegApplication+`\shell\open\command`, "", command)
		before := reg.dump()
		platform, notified := testEditorPlatform(reg)
		if changed, err := editorRegRepairDocumentIcon(platform, testCopyA); err != nil || changed || *notified != 0 || reg.dump() != before {
			t.Fatalf("replaced the user's icons: %v %v\n%s", changed, err, reg.dump())
		}
		// The document icon of another index the user chose stays too.
		reg.SetString(editorRegApplication+`\DefaultIcon`, "", `"`+testCopyA+`",0`)
		reg.SetString(editorRegApplication+`\shell\open`, "Icon", `"`+testCopyA+`",2`)
		before = reg.dump()
		if changed, err := editorRegRepairDocumentIcon(platform, testCopyA); err != nil || changed || reg.dump() != before {
			t.Fatalf("replaced the user's choice of index: %v %v", changed, err)
		}
	})
	t.Run("a user's file icon stays while the missing verb icon is added", func(t *testing.T) {
		reg := fixtureRegistry()
		reg.SetString(editorRegApplication+`\DefaultIcon`, "", `C:\Icons\files.ico`)
		reg.SetString(editorRegApplication+`\shell\open\command`, "", command)
		platform, _ := testEditorPlatform(reg)
		if changed, err := editorRegRepairDocumentIcon(platform, testCopyA); err != nil || !changed {
			t.Fatalf("changed=%v err=%v", changed, err)
		}
		if got := reg.str(t, editorRegApplication+`\DefaultIcon`, ""); got != `C:\Icons\files.ico` {
			t.Fatalf("DefaultIcon %q", got)
		}
		if got := reg.str(t, editorRegApplication+`\shell\open`, "Icon"); got != `"`+testCopyA+`",1` {
			t.Fatalf("verb Icon %q", got)
		}
	})
	t.Run("an entry without an icon gets both", func(t *testing.T) {
		reg := fixtureRegistry()
		reg.SetString(editorRegApplication+`\shell\open\command`, "", command)
		platform, _ := testEditorPlatform(reg)
		if changed, err := editorRegRepairDocumentIcon(platform, testCopyA); err != nil || !changed {
			t.Fatalf("changed=%v err=%v", changed, err)
		}
		if reg.str(t, editorRegApplication+`\DefaultIcon`, "") != `"`+testCopyA+`",3` || reg.str(t, editorRegApplication+`\shell\open`, "Icon") != `"`+testCopyA+`",1` {
			t.Fatalf("after repair:\n%s", reg.dump())
		}
	})
	for _, tc := range []struct {
		name    string
		command string
	}{
		// Windows' own entry (bare file name): nobody would take the icon back.
		{"Windows' own entry for this program", `"` + testCopyA + `" "%1"`},
		{"another copy's entry", editorRegCommand(testCopyB)},
		{"another program's entry", `"E:\Elsewhere\Network_Toolkit.exe" "%1"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := fixtureRegistry()
			reg.SetString(editorRegApplication+`\shell\open\command`, "", tc.command)
			before := reg.dump()
			platform, notified := testEditorPlatform(reg)
			if changed, err := editorRegRepairDocumentIcon(platform, testCopyA); err != nil || changed || *notified != 0 || reg.dump() != before {
				t.Fatalf("changed %s: %v %v\n%s", tc.name, changed, err, reg.dump())
			}
		})
	}
	t.Run("a read that fails changes nothing", func(t *testing.T) {
		for _, path := range []string{editorRegApplication + `\DefaultIcon`, editorRegApplication + `\shell\open`} {
			reg := fixtureRegistry()
			reg.SetString(editorRegProgKey+`\DefaultIcon`, "", `"`+testCopyA+`",1`)
			reg.SetString(editorRegProgKey+`\shell\open\command`, "", command)
			reg.SetString(editorRegApplication+`\DefaultIcon`, "", `"`+testCopyA+`",1`)
			reg.SetString(editorRegApplication+`\shell\open\command`, "", command)
			reg.readFault = path
			before := reg.dump()
			platform, notified := testEditorPlatform(reg)
			if changed, err := editorRegRepairDocumentIcon(platform, testCopyA); err == nil || changed || *notified != 0 || reg.dump() != before {
				t.Fatalf("%s unreadable: changed after a failed read: %v %v", path, changed, err)
			}
		}
	})
}

// Windows' own entry of the separate Text Editor program (written when the
// user picks Network_Toolkit_TextEditor.exe in "Open with"): the first start
// after an update gives it the document icon when the record that holds the
// value is one whose uninstaller or Remove takes it back.
func TestDocumentIconRepairOfWindowsOwnEditorEntry(t *testing.T) {
	windowsEntry := func(reg *fakeRegistry, program string) {
		reg.SetString(editorRegEditorApplication+`\shell\open\command`, "", `"`+program+`" "%1"`)
	}
	installedEditor := editorRegJoin(editorRegDir(testInstalled), editorRegEditorFile)
	installedIcon := editorRegIcon(testInstalled, editorRegDocumentIcon)
	t.Run("the installed program, with an uninstaller that takes it back", func(t *testing.T) {
		for _, version := range []string{editorRegEditorAppIconUninstallerVersion, "1.22.0", "2.0.0"} {
			reg := installedRegistry(version)
			windowsEntry(reg, installedEditor)
			platform, notified := testEditorPlatform(reg)
			changed, err := editorRegRepairDocumentIcon(platform, testInstalled)
			if err != nil || !changed || *notified != 1 {
				t.Fatalf("Setup %s: changed=%v err=%v notified=%d", version, changed, err, *notified)
			}
			if got := reg.str(t, editorRegEditorApplication+`\DefaultIcon`, ""); got != installedIcon {
				t.Fatalf("Setup %s: DefaultIcon %q", version, got)
			}
			if got := reg.str(t, editorRegSetupRecord, editorRegEditorAppIcon); got != installedIcon {
				t.Fatalf("Setup %s: Setup's record holds %q", version, got)
			}
			if got := reg.str(t, editorRegEditorApplication+`\shell\open\command`, ""); got != `"`+installedEditor+`" "%1"` {
				t.Fatalf("Windows' command changed: %q", got)
			}
			if _, found := reg.value(editorRegEditorApplication+`\shell\open`, "Icon"); found {
				t.Fatal("a verb icon was written into Windows' entry of the Text Editor program")
			}
			// What the uninstaller does: the value, while it is what was written.
			simulateUninstallEditorAppIcon(reg)
			if reg.has(editorRegEditorApplication+`\DefaultIcon`) || !reg.has(editorRegEditorApplication+`\shell\open\command`) {
				t.Fatalf("after uninstall:\n%s", reg.dump())
			}
		}
	})
	t.Run("an older uninstaller would leave it behind", func(t *testing.T) {
		for _, version := range []string{"1.21.51", "1.21.44", "", "1.21.052", "v1.21.52"} {
			reg := installedRegistry(version)
			reg.SetDWORD(editorRegSetupRecord, "Records", 1)
			windowsEntry(reg, installedEditor)
			before := reg.dump()
			platform, notified := testEditorPlatform(reg)
			if changed, err := editorRegRepairDocumentIcon(platform, testInstalled); err != nil || changed || *notified != 0 || reg.dump() != before {
				t.Fatalf("Setup %q: changed=%v err=%v\n%s", version, changed, err, reg.dump())
			}
		}
	})
	t.Run("the registered portable copy", func(t *testing.T) {
		reg := fixtureRegistry()
		platform, _ := testEditorPlatform(reg)
		if _, err := registerEditorCopy(platform, testCopyA, false, false); err != nil {
			t.Fatal(err)
		}
		// The user picks this copy's Text Editor program in Open with later.
		editorA := editorRegJoin(editorRegDir(testCopyA), editorRegEditorFile)
		windowsEntry(reg, editorA)
		changed, err := editorRegRepairDocumentIcon(platform, testCopyA)
		if err != nil || !changed {
			t.Fatalf("changed=%v err=%v", changed, err)
		}
		if got := reg.str(t, editorRegPortable, editorRegEditorAppIcon); got != `"`+testCopyA+`",3` {
			t.Fatalf("the portable record holds %q", got)
		}
		// Remove takes back exactly that.
		if _, err := removeEditorCopy(platform, testCopyA); err != nil {
			t.Fatal(err)
		}
		if reg.has(editorRegEditorApplication+`\DefaultIcon`) || !reg.has(editorRegEditorApplication+`\shell\open\command`) || reg.has(editorRegPortable) {
			t.Fatalf("after remove:\n%s", reg.dump())
		}
	})
	for _, tc := range []struct {
		name  string
		setup func(reg *fakeRegistry)
		exe   string
	}{
		{"a copy that is not registered", func(reg *fakeRegistry) {
			windowsEntry(reg, editorRegJoin(editorRegDir(testCopyA), editorRegEditorFile))
		}, testCopyA},
		{"another portable copy's registration", func(reg *fakeRegistry) {
			reg.SetString(editorRegPortable, "Executable", testCopyB)
			windowsEntry(reg, editorRegJoin(editorRegDir(testCopyA), editorRegEditorFile))
		}, testCopyA},
		{"a portable copy while the program is installed", func(reg *fakeRegistry) {
			reg.SetString(editorRegUninstall, "InstallLocation", editorRegDir(testInstalled))
			reg.SetString(editorRegUninstall, "DisplayVersion", "2.0.0")
			reg.SetDWORD(editorRegSetupRecord, "Records", 1)
			windowsEntry(reg, editorRegJoin(editorRegDir(testCopyA), editorRegEditorFile))
		}, testCopyA},
		{"the installed program without Setup's record", func(reg *fakeRegistry) {
			reg.SetString(editorRegUninstall, "InstallLocation", editorRegDir(testInstalled))
			reg.SetString(editorRegUninstall, "DisplayVersion", "2.0.0")
			windowsEntry(reg, installedEditor)
		}, testInstalled},
		{"an entry of another folder's program", func(reg *fakeRegistry) {
			reg.SetString(editorRegUninstall, "InstallLocation", editorRegDir(testInstalled))
			reg.SetString(editorRegUninstall, "DisplayVersion", "2.0.0")
			reg.SetDWORD(editorRegSetupRecord, "Records", 1)
			windowsEntry(reg, `E:\Other\Network_Toolkit_TextEditor.exe`)
		}, testInstalled},
		{"an entry with an icon of its own", func(reg *fakeRegistry) {
			reg.SetString(editorRegUninstall, "InstallLocation", editorRegDir(testInstalled))
			reg.SetString(editorRegUninstall, "DisplayVersion", "2.0.0")
			reg.SetDWORD(editorRegSetupRecord, "Records", 1)
			windowsEntry(reg, installedEditor)
			reg.SetString(editorRegEditorApplication+`\DefaultIcon`, "", `C:\Icons\custom.ico`)
		}, testInstalled},
		{"no entry", func(reg *fakeRegistry) {
			reg.SetString(editorRegUninstall, "InstallLocation", editorRegDir(testInstalled))
			reg.SetString(editorRegUninstall, "DisplayVersion", "2.0.0")
			reg.SetDWORD(editorRegSetupRecord, "Records", 1)
		}, testInstalled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := fixtureRegistry()
			tc.setup(reg)
			before := reg.dump()
			platform, notified := testEditorPlatform(reg)
			if changed, err := editorRegRepairDocumentIcon(platform, tc.exe); err != nil || changed || *notified != 0 || reg.dump() != before {
				t.Fatalf("changed=%v err=%v\n%s", changed, err, reg.dump())
			}
		})
	}
	t.Run("a read that fails changes nothing", func(t *testing.T) {
		for _, path := range []string{editorRegUninstall, editorRegSetupRecord, editorRegEditorApplication + `\shell\open\command`, editorRegEditorApplication + `\DefaultIcon`} {
			reg := installedRegistry("2.0.0")
			reg.SetDWORD(editorRegSetupRecord, "Records", 1)
			windowsEntry(reg, installedEditor)
			reg.readFault = path
			before := reg.dump()
			platform, notified := testEditorPlatform(reg)
			if changed, err := editorRegRepairDocumentIcon(platform, testInstalled); err == nil || changed || *notified != 0 || reg.dump() != before {
				t.Fatalf("%s unreadable: changed=%v err=%v", path, changed, err)
			}
		}
	})
}

// The post-update repair trusts an installed uninstaller of at least
// editorRegEditorAppIconUninstallerVersion to take the icon back: that version
// must be this build's or older (a released one) and its Setup must hold the
// removal, inserted where uninstall and a rolled back install unregister.
func TestEditorAppIconUninstallerVersion(t *testing.T) {
	v := editorRegEditorAppIconUninstallerVersion
	if !updateVersionPattern.MatchString(v) || updateNewer(v, model.Version) {
		t.Fatalf("editorRegEditorAppIconUninstallerVersion %q must be a strict x.y.z no newer than this build (%s)", v, model.Version)
	}
	nsi, err := os.ReadFile(filepath.Join("..", "..", "scripts", "installer", "windows.nsi"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(nsi)
	if !strings.Contains(text, "\n!macro RemoveEditorApplicationIcon\n") {
		t.Fatal("windows.nsi has no RemoveEditorApplicationIcon")
	}
	start := strings.Index(text, "\n!macro UnregisterShell\n")
	if start < 0 {
		t.Fatal("windows.nsi has no UnregisterShell")
	}
	unregister := text[start : start+strings.Index(text[start:], "\n!macroend")]
	if !strings.Contains(unregister, "!insertmacro RemoveEditorApplicationIcon") {
		t.Fatal("UnregisterShell does not remove the icon of Windows' own entry of the Text Editor program")
	}
}

// Giving Windows' own entry of the Text Editor program the document icon is a
// nicety: a write that fails is skipped (no record of it) and never rolls back
// an install or stops an upgrade.
func TestSetupEditorApplicationIconIsNotFatal(t *testing.T) {
	nsi, err := os.ReadFile(filepath.Join("..", "..", "scripts", "installer", "windows.nsi"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(nsi), "\r\n", "\n")
	start := strings.Index(text, "\n!macro RegisterEditorApplicationIcon\n")
	if start < 0 {
		t.Fatal("windows.nsi has no RegisterEditorApplicationIcon")
	}
	body := text[start : start+strings.Index(text[start:], "\n!macroend")]
	var lines []string
	for _, line := range strings.Split(body, "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, ";") {
			lines = append(lines, line)
		}
	}
	for _, fatal := range []string{"install_error", "RegistrationFailed", "Abort", "StopSetup"} {
		if strings.Contains(body, fatal) {
			t.Errorf("RegisterEditorApplicationIcon uses %s", fatal)
		}
	}
	if last := lines[len(lines)-1]; last != "ClearErrors" {
		t.Errorf("RegisterEditorApplicationIcon must end with ClearErrors, ends with %q", last)
	}
	icon := strings.Index(body, `WriteRegStr HKCU "${EDITOR_APPLICATION_KEY}\DefaultIcon" ""`)
	record := strings.Index(body, `WriteRegStr HKCU "${EDITOR_KEY}" "${EDITOR_APPLICATION_ICON}"`)
	if icon < 0 || record < 0 || record < icon || !strings.Contains(body[icon:record], "${IfNot} ${Errors}") {
		t.Error("the icon is recorded only after its write succeeded")
	}
	// The step in the main section is not followed by an error check.
	section := text[strings.Index(text, `Section "Network Toolkit"`):]
	at := strings.Index(section, "!insertmacro RegisterEditorApplicationIcon")
	if at < 0 {
		t.Fatal("Setup does not give Windows' own entry the document icon")
	}
	for _, line := range strings.Split(section[at:], "\n")[1:] {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.Contains(line, "install_error") {
			t.Errorf("a failed icon write would stop Setup: %q follows the step", line)
		}
		break
	}
}

// simulateUninstallEditorAppIcon is Setup's RemoveEditorApplicationIcon.
func simulateUninstallEditorAppIcon(reg *fakeRegistry) {
	recorded, found, ok, _ := reg.String(editorRegSetupRecord, editorRegEditorAppIcon)
	if found && ok && recorded != "" {
		if current, found, ok, _ := reg.String(editorRegEditorApplication+`\DefaultIcon`, ""); found && ok && current == recorded {
			reg.DeleteValue(editorRegEditorApplication+`\DefaultIcon`, "")
			editorRegDeleteEmpty(reg, editorRegEditorApplication+`\DefaultIcon`)
		}
	}
	reg.DeleteValue(editorRegSetupRecord, editorRegEditorAppIcon)
}
