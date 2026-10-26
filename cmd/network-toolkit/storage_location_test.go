package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func storageFixture(t *testing.T, explicit bool) *appStorageManager {
	t.Helper()
	base := t.TempDir()
	config := appStorageConfig{RootDir: filepath.Join(base, "current"), DefaultDir: filepath.Join(base, "default"), Explicit: explicit}
	config.ConfigPath = filepath.Join(config.DefaultDir, storageSelectionFilename)
	manager := newAppStorage(config)
	if err := manager.Open(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close() })
	return manager
}

func writeStorageFixture(t *testing.T, root, relative, contents string) string {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertStorageContents(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("%s: got %q, error %v; want %q", path, got, err, want)
	}
}

func TestAppStorageDefaultsNeverUseExecutableDirectory(t *testing.T) {
	user := t.TempDir()
	t.Setenv("LOCALAPPDATA", user)
	t.Setenv("XDG_CONFIG_HOME", user)
	executable := t.TempDir()
	legacy := writeStorageFixture(t, executable, "Network_Toolkit.settings.json", `{"legacy":true}`)
	config, err := resolveAppStorage(executable, "")
	if err != nil {
		t.Fatal(err)
	}
	if config.RootDir != filepath.Join(user, "NetworkToolkit") || config.RootDir == executable || config.Explicit {
		t.Fatalf("unexpected default: %+v", config)
	}
	assertStorageContents(t, legacy, `{"legacy":true}`)
	selected := filepath.Join(t.TempDir(), "selected")
	writeStorageFixture(t, config.DefaultDir, storageSelectionFilename, fmt.Sprintf(`{"version":1,"rootDir":%q}`, selected))
	config, err = resolveAppStorage(executable, "")
	if err != nil || config.RootDir != selected {
		t.Fatalf("saved selection: %+v %v", config, err)
	}
	override := filepath.Join(t.TempDir(), "explicit")
	config, err = resolveAppStorage(executable, override)
	if err != nil || config.RootDir != override || !config.Explicit {
		t.Fatalf("explicit override: %+v %v", config, err)
	}
	writeStorageFixture(t, config.DefaultDir, storageSelectionFilename, `{"version":99,"rootDir":"bad"}`)
	if _, err := resolveAppStorage(executable, ""); err == nil {
		t.Fatal("invalid selection silently fell back to another data folder")
	}
	if config, err := resolveAppStorage(executable, override); err != nil || config.RootDir != override {
		t.Fatal("explicit override depended on the saved selector", err)
	}
}

func TestAppStorageSharedLeaseBlocksOtherWorkspaceMaintenance(t *testing.T) {
	first := storageFixture(t, false)
	second := newAppStorage(first.config)
	if err := second.Open(); err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	settings := writeStorageFixture(t, first.config.RootDir, "Network_Toolkit.settings.json", `{"saved":true}`)
	if err := first.scheduleDelete(); !errors.Is(err, ErrAppStorageBusy) {
		t.Fatalf("deletion ignored another workspace: %v", err)
	}
	if _, _, err := first.changeLocation(filepath.Join(t.TempDir(), "next"), true); !errors.Is(err, ErrAppStorageBusy) {
		t.Fatalf("migration ignored another workspace: %v", err)
	}
	assertStorageContents(t, settings, `{"saved":true}`)
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.scheduleDelete(); err != nil {
		t.Fatal("shared lease was not restored after refused maintenance:", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(settings); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("deferred deletion did not complete", err)
	}
}

func TestAppStorageCopyRefreshesOnlyItsUnchangedFilesOnClose(t *testing.T) {
	manager := storageFixture(t, false)
	root := manager.config.RootDir
	settings := writeStorageFixture(t, root, "Network_Toolkit.settings.json", `{"theme":"dark"}`)
	vendors := writeStorageFixture(t, root, "Network_Toolkit.vendors.json.gz", "vendor fixture")
	writeStorageFixture(t, root, filepath.Join(storageTerminalDirectory, "Network_Toolkit.settings.json"), `{"terminal":true}`)
	writeStorageFixture(t, root, filepath.Join(storageBrowserDirectory, "cache.bin"), "must not copy")
	writeStorageFixture(t, root, "report.json", "user report")
	destination := filepath.Join(t.TempDir(), "chosen")
	unrelated := writeStorageFixture(t, destination, "notes.txt", "keep destination notes")
	status, copied, err := manager.changeLocation(destination, true)
	if err != nil {
		t.Fatal(err)
	}
	if !status.RestartRequired || !status.CopyPending || status.RootDir != root || status.NextRootDir != destination || len(copied) != 3 {
		t.Fatalf("bad location result: %+v %#v", status, copied)
	}
	assertStorageContents(t, filepath.Join(destination, "Network_Toolkit.settings.json"), `{"theme":"dark"}`)
	if _, err := os.Stat(filepath.Join(destination, storageBrowserDirectory)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a live WebView cache was copied")
	}
	if _, err := os.Stat(filepath.Join(destination, "report.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("an unrelated source file was copied")
	}
	writeStorageFixture(t, root, "Network_Toolkit.settings.json", `{"theme":"light","late":true}`)
	writeStorageFixture(t, root, "Network_Toolkit.ssh_known_hosts.json", `{"version":1,"hosts":[]}`)
	if err := os.Remove(vendors); err != nil {
		t.Fatal(err)
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	assertStorageContents(t, filepath.Join(destination, "Network_Toolkit.settings.json"), `{"theme":"light","late":true}`)
	assertStorageContents(t, filepath.Join(destination, "Network_Toolkit.ssh_known_hosts.json"), `{"version":1,"hosts":[]}`)
	if _, err := os.Stat(filepath.Join(destination, "Network_Toolkit.vendors.json.gz")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a reset vendor database was restored by the migration")
	}
	assertStorageContents(t, settings, `{"theme":"light","late":true}`)
	assertStorageContents(t, unrelated, "keep destination notes")
}

func TestAppStorageCopyConflictKeepsBothLocations(t *testing.T) {
	manager := storageFixture(t, false)
	source := writeStorageFixture(t, manager.config.RootDir, "Network_Toolkit.settings.json", `{"source":"before"}`)
	destination := filepath.Join(t.TempDir(), "chosen")
	if _, _, err := manager.changeLocation(destination, true); err != nil {
		t.Fatal(err)
	}
	writeStorageFixture(t, manager.config.RootDir, "Network_Toolkit.settings.json", `{"source":"latest"}`)
	target := writeStorageFixture(t, destination, "Network_Toolkit.settings.json", `{"destination":"edited separately"}`)
	if err := manager.Close(); err == nil {
		t.Fatal("a modified migration destination was overwritten")
	}
	assertStorageContents(t, source, `{"source":"latest"}`)
	assertStorageContents(t, target, `{"destination":"edited separately"}`)
}

func TestAppStorageCopyRefusesExistingOrActiveDestination(t *testing.T) {
	manager := storageFixture(t, false)
	writeStorageFixture(t, manager.config.RootDir, "Network_Toolkit.settings.json", `{"source":true}`)
	destination := filepath.Join(t.TempDir(), "chosen")
	existing := writeStorageFixture(t, destination, "Network_Toolkit.settings.json", `{"existing":true}`)
	if _, _, err := manager.changeLocation(destination, true); err == nil {
		t.Fatal("copy overwrote an existing app-data slot")
	}
	assertStorageContents(t, existing, `{"existing":true}`)
	if _, err := os.Stat(manager.config.ConfigPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a failed copy changed the saved selection")
	}
	other := newAppStorage(appStorageConfig{RootDir: destination, DefaultDir: destination, ConfigPath: filepath.Join(destination, storageSelectionFilename), Explicit: true})
	if err := other.Open(); err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, _, err := manager.changeLocation(destination, false); !errors.Is(err, ErrAppStorageBusy) {
		t.Fatalf("active destination was accepted: %v", err)
	}
}

func TestAppStorageDeleteIsDeferredAndPreservesUnrelatedFiles(t *testing.T) {
	manager := storageFixture(t, false)
	root := manager.config.RootDir
	var removed []string
	for _, relative := range []string{
		"Network_Toolkit.settings.json", "Network_Toolkit.ssh_known_hosts.json", "Network_Toolkit.vendors.json.gz", "Network_Toolkit.window.json",
		filepath.Join(storageTerminalDirectory, "Network_Toolkit.settings.json"), filepath.Join(storageTerminalDirectory, "Network_Toolkit.instance.json"),
		filepath.Join(storageBrowserDirectory, "Default", "Cache", "data"),
		filepath.Join(storageTerminalDirectory, storageBrowserDirectory, "cache"),
		filepath.Join(storageViewsDirectory, strings.Repeat("ab", 16), "Network_Toolkit.window.json"),
		filepath.Join(storageViewsDirectory, strings.Repeat("ab", 16), storageBrowserDirectory, "cache"),
	} {
		removed = append(removed, writeStorageFixture(t, root, relative, "app data"))
	}
	var kept []string
	for _, relative := range []string{"report.json", "notes.txt", filepath.Join(storageTerminalDirectory, "notes.txt"), filepath.Join(storageViewsDirectory, "unrelated", "report.txt"), filepath.Join(storageViewsDirectory, strings.Repeat("ab", 16), "notes.txt")} {
		kept = append(kept, writeStorageFixture(t, root, relative, "unrelated content"))
	}
	selector := writeStorageFixture(t, manager.config.DefaultDir, storageSelectionFilename, fmt.Sprintf(`{"version":1,"rootDir":%q}`, root))
	if err := manager.scheduleDelete(); err != nil {
		t.Fatal(err)
	}
	for _, path := range removed {
		assertStorageContents(t, path, "app data")
	}
	newcomer := newAppStorage(manager.config)
	if err := newcomer.Open(); !errors.Is(err, ErrAppStorageBusy) {
		_ = newcomer.Close()
		t.Fatalf("new process entered a pending deletion: %v", err)
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range removed {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("app data remained: %s %v", path, err)
		}
	}
	for _, path := range kept {
		assertStorageContents(t, path, "unrelated content")
	}
	assertStorageContents(t, selector, fmt.Sprintf(`{"version":1,"rootDir":%q}`, root))
	for _, name := range []string{storageGuardFilename, storageLeaseFilename} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatal("a coordination lock was unlinked:", err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, storageDeleteFilename)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("completed deletion marker remained")
	}
}

func TestAppStoragePendingDeleteRecoversAfterOwnerCrash(t *testing.T) {
	manager := storageFixture(t, true)
	settings := writeStorageFixture(t, manager.config.RootDir, "Network_Toolkit.settings.json", `{"old":true}`)
	if err := writeStorageAtomic(manager.root, storageDeleteFilename, []byte(`{"version":1,"application":"NetworkToolkit"}`)); err != nil {
		t.Fatal(err)
	}
	// Simulate a process ending after the durable marker, before its orderly
	// Close callback learned the intent. The next process must clean first.
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	next := newAppStorage(manager.config)
	if err := next.Open(); err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	if _, err := os.Stat(settings); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("pending deletion was not completed before startup")
	}
}

func TestAppStorageRefusesLinksBeforeDeletingAnything(t *testing.T) {
	manager := storageFixture(t, true)
	settings := writeStorageFixture(t, manager.config.RootDir, "Network_Toolkit.settings.json", "keep settings")
	outside := t.TempDir()
	protected := writeStorageFixture(t, outside, "protected.txt", "outside data")
	if err := os.Mkdir(filepath.Join(manager.config.RootDir, storageBrowserDirectory), 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(manager.config.RootDir, storageBrowserDirectory, "linked")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	if err := manager.scheduleDelete(); err == nil {
		t.Fatal("cache link was followed")
	}
	assertStorageContents(t, settings, "keep settings")
	assertStorageContents(t, protected, "outside data")
	if _, err := os.Stat(filepath.Join(manager.config.RootDir, storageDeleteFilename)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("unsafe deletion was acknowledged")
	}
	linkedRoot := filepath.Join(t.TempDir(), "linked-root")
	if err := os.Symlink(manager.config.RootDir, linkedRoot); err != nil {
		t.Fatal(err)
	}
	if opened, err := openStorageRoot(linkedRoot, true); err == nil {
		_ = opened.Close()
		t.Fatal("a linked storage root was accepted")
	}
}

func TestAppStorageExplicitOverrideCannotChangeGlobalSelection(t *testing.T) {
	manager := storageFixture(t, true)
	if _, _, err := manager.changeLocation(filepath.Join(t.TempDir(), "next"), false); err == nil {
		t.Fatal("explicit override changed the global selection")
	}
	if _, err := os.Stat(manager.config.DefaultDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("explicit storage unnecessarily created the default control folder")
	}
}

func TestAppStorageLeaseIsCrossProcessAndCrashSafe(t *testing.T) {
	manager := storageFixture(t, true)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestAppStorageLeaseHelperProcess$")
	cmd.Env = append(os.Environ(), "NETWORK_TOOLKIT_STORAGE_HELPER="+manager.config.RootDir)
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	finished := false
	t.Cleanup(func() {
		if !finished {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil || line != "ready\n" {
		t.Fatalf("child lease did not start: %q %v", line, err)
	}
	if err := manager.scheduleDelete(); !errors.Is(err, ErrAppStorageBusy) {
		t.Fatalf("cross-process lease was ignored: %v", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	finished = true
	if err := manager.scheduleDelete(); err != nil {
		t.Fatal("crashed owner left a stale app-data lock:", err)
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestAppStorageLeaseHelperProcess(t *testing.T) {
	root := os.Getenv("NETWORK_TOOLKIT_STORAGE_HELPER")
	if root == "" {
		return
	}
	manager := newAppStorage(appStorageConfig{RootDir: root, DefaultDir: root, ConfigPath: filepath.Join(root, storageSelectionFilename), Explicit: true})
	if err := manager.Open(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	_, _ = fmt.Fprintln(os.Stdout, "ready")
	_, _ = io.Copy(io.Discard, os.Stdin)
	if err := manager.Close(); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
