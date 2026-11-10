package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// The tests in this file run the test program again as separate operating
// system processes on one data folder, the way the main window, the Terminal
// and the Text Editor run. An OS file lock is only proven between processes,
// so the lock, the wait and the merge are exercised across real process
// boundaries here, not only between application objects of one process.

const (
	sharedProcessMode      = "NTK_SHARED_PROCESS_MODE"
	sharedProcessRoot      = "NTK_SHARED_PROCESS_ROOT"
	sharedProcessWorkspace = "NTK_SHARED_PROCESS_WORKSPACE"
	sharedProcessWindow    = "NTK_SHARED_PROCESS_WINDOW"
	sharedProcessCount     = "NTK_SHARED_PROCESS_COUNT"
)

// TestSharedProcessHelper is the other process. It does nothing unless a test
// of this file started it.
func TestSharedProcessHelper(t *testing.T) {
	mode := os.Getenv(sharedProcessMode)
	if mode == "" {
		return
	}
	root := os.Getenv(sharedProcessRoot)
	waitForParent := func() { _, _ = io.Copy(io.Discard, os.Stdin) }
	switch mode {
	case "hold-settings", "hold-shared":
		// Another app in the middle of a save: it holds the lock until the
		// test closes this process's input.
		var release func()
		var err error
		if mode == "hold-shared" {
			release, err = acquireSharedDataLock(root)
		} else {
			release, err = acquireCredentialGuard(root)
		}
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("ready")
		waitForParent()
		release()
	case "save":
		// One app: its page loads the settings, then saves a new connection
		// with every save, never refreshing in between, and one setting.
		window := os.Getenv(sharedProcessWindow)
		count, _ := strconv.Atoi(os.Getenv(sharedProcessCount))
		page := loadSharedPage(t, sharedTestApp(t, root, os.Getenv(sharedProcessWorkspace), window))
		fmt.Println("ready")
		// Every app has loaded before the first one saves.
		if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
			t.Fatal(err)
		}
		for n := 0; n < count; n++ {
			page.add("terminalProfiles", sharedProfile(fmt.Sprintf("%s-%d", window, n), fmt.Sprintf("%s %d", window, n), "192.0.2.10"))
			if n == count/2 {
				switch window {
				case "main":
					page.state["theme"] = "light"
				case "terminal":
					page.state["terminalFontSize"] = float64(16)
				default:
					page.set("editorPreferences", "wordWrap", true)
				}
			}
			page.save()
		}
	default:
		t.Fatalf("unknown mode %q", mode)
	}
}

type sharedProcess struct {
	t       *testing.T
	cmd     *exec.Cmd
	input   io.WriteCloser
	output  *sharedProcessOutput
	drained chan struct{}
	done    bool
}

// sharedProcessOutput collects what the other process printed, for a failure
// message.
type sharedProcessOutput struct {
	mu   sync.Mutex
	text bytes.Buffer
}

func (o *sharedProcessOutput) add(text string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.text.WriteString(text)
}

func (o *sharedProcessOutput) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.text.String()
}

// startSharedProcess starts the helper and returns once it reports that it is
// ready (it holds its lock, or its page has loaded).
func startSharedProcess(t *testing.T, root, mode string, env ...string) *sharedProcess {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestSharedProcessHelper$", "-test.count=1")
	cmd.Env = append(append(os.Environ(), sharedProcessMode+"="+mode, sharedProcessRoot+"="+root), env...)
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = cmd.Stdout // One pipe for everything the other process prints.
	p := &sharedProcess{t: t, cmd: cmd, input: input, output: &sharedProcessOutput{}, drained: make(chan struct{})}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !p.done {
			_ = p.input.Close()
			_ = p.cmd.Process.Kill()
			<-p.drained
			_ = p.cmd.Wait()
		}
	})
	reader := bufio.NewReader(stdout)
	ready := make(chan error, 1)
	go func() {
		defer close(p.drained)
		told := false
		for {
			line, err := reader.ReadString('\n')
			p.output.add(line)
			if !told && strings.TrimSpace(line) == "ready" {
				told = true
				ready <- nil
			}
			if err != nil {
				if !told {
					ready <- err
				}
				return
			}
		}
	}()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatalf("the other process did not start: %v\n%s", err, p.output.String())
		}
	case <-time.After(60 * time.Second):
		t.Fatalf("the other process did not become ready\n%s", p.output.String())
	}
	return p
}

// finish lets the other process end (closing its input) and waits for it.
func (p *sharedProcess) finish() {
	p.t.Helper()
	if p.done {
		return
	}
	_ = p.input.Close()
	<-p.drained
	err := p.cmd.Wait()
	p.done = true
	if err != nil {
		p.t.Fatalf("the other process failed: %v\n%s", err, p.output.String())
	}
}

func TestSharedSavesFromSeparateProcessesLoseNothing(t *testing.T) {
	root := t.TempDir()
	const perApp = 12
	apps := []struct{ workspace, window string }{
		{"", "main"},
		{terminalWorkspaceName, "terminal"},
		{standaloneEditorDirectory, standaloneEditorMode},
	}
	processes, want := []*sharedProcess{}, []string{}
	for _, app := range apps {
		processes = append(processes, startSharedProcess(t, root, "save",
			sharedProcessWorkspace+"="+app.workspace, sharedProcessWindow+"="+app.window, sharedProcessCount+"="+strconv.Itoa(perApp)))
		for n := 0; n < perApp; n++ {
			want = append(want, fmt.Sprintf("%s-%d", app.window, n))
		}
	}
	// Every page has loaded the empty settings. Now all three save at once.
	for _, p := range processes {
		if _, err := io.WriteString(p.input, "go\n"); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range processes {
		p.finish()
	}
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, want...) {
		t.Fatalf("connections after three processes saved at once: %d of %d: %v", len(got), len(want), got)
	}
	disk := sharedDisk(t, root)
	preferences, _ := disk["editorPreferences"].(map[string]any)
	if disk["theme"] != "light" || disk["terminalFontSize"] != float64(16) || preferences["wordWrap"] != true {
		t.Fatalf("settings after three processes saved at once: theme=%v font=%v editor=%v", disk["theme"], disk["terminalFontSize"], preferences)
	}
	// An app started afterwards has all of it, and no workspace kept a copy.
	late := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	if !sameIDs(late.ids("terminalProfiles"), want...) || late.state["theme"] != "light" || late.member("editorPreferences", "wordWrap") != true {
		t.Fatalf("an app started afterwards: %v theme=%v", late.ids("terminalProfiles"), late.state["theme"])
	}
	for _, workspace := range []string{terminalWorkspaceName, standaloneEditorDirectory} {
		raw, err := os.ReadFile(filepath.Join(root, workspace, appSettingsFilename))
		if err != nil || bytes.Contains(raw, []byte("terminalProfiles")) || bytes.Contains(raw, []byte(`"theme"`)) {
			t.Fatalf("%s holds shared data in its own settings file: %s %v", workspace, raw, err)
		}
	}
}

func TestSharedSaveWaitsForAnotherProcessAndReportsALongWait(t *testing.T) {
	root := t.TempDir()
	settings := filepath.Join(root, appSettingsFilename)
	page := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	page.add("terminalProfiles", sharedProfile("first", "First", "192.0.2.1"))
	page.save()

	// Another process is saving. This save waits for it instead of failing.
	holder := startSharedProcess(t, root, "hold-settings")
	const held = 300 * time.Millisecond
	released := make(chan struct{})
	started := time.Now()
	go func() {
		time.Sleep(held)
		_ = holder.input.Close()
		close(released)
	}()
	page.add("terminalProfiles", sharedProfile("second", "Second", "192.0.2.2"))
	page.save()
	if waited := time.Since(started); waited < held {
		t.Fatalf("a save went through after %v while another process held the lock for %v", waited, held)
	}
	<-released
	holder.finish()
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "first", "second") {
		t.Fatalf("after waiting for the other process: %v", got)
	}

	// The other process does not finish in time: the save is refused with a
	// plain message, the file is untouched, and the page still holds the
	// change, so the same save goes through afterwards.
	holder = startSharedProcess(t, root, "hold-settings")
	before, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	wait := sharedLockWait
	sharedLockWait = 200 * time.Millisecond
	t.Cleanup(func() { sharedLockWait = wait })
	page.add("terminalProfiles", sharedProfile("third", "Third", "192.0.2.3"))
	code, body := page.trySave()
	if code != http.StatusConflict || !strings.Contains(body, "another Network Toolkit app is saving") || strings.Contains(body, "credential") {
		t.Fatalf("a save while another process held the lock: %d %s", code, body)
	}
	if after, err := os.ReadFile(settings); err != nil || !bytes.Equal(before, after) {
		t.Fatalf("the settings file changed while another process held the lock: %v", err)
	}
	// Reading is not blocked by another app's save.
	if r := requestApp(page.app, "GET", "/api/state", "", true); r.Code != 200 {
		t.Fatalf("settings could not be read while another process was saving: %d", r.Code)
	}
	holder.finish()
	sharedLockWait = wait
	page.save()
	if got := sharedDiskIDs(t, root, "terminalProfiles"); !sameIDs(got, "first", "second", "third") {
		t.Fatalf("after the other process finished: %v", got)
	}
}

func TestSharedStoresRefuseChangesWhileAnotherProcessHoldsTheirLock(t *testing.T) {
	root := t.TempDir()
	key := knownHostFixtureKey(t)
	writeKnownHostFixture(t, root, knownHostFixtureEntry("core.test:22", key))
	app := sharedTestApp(t, root, terminalWorkspaceName, "terminal")
	const macroPath = "/api/terminal-automation/library"
	library := macroHTTPLibrary(t, requestApp(app, http.MethodGet, macroPath, "", true), http.StatusOK)
	library.Macros = append(library.Macros, macroHTTPDefinition("first"))
	library = macroHTTPLibrary(t, viewTestRequest(app, app.token, http.MethodPut, macroPath, library, nil), http.StatusOK)
	macroFile, hostFile := filepath.Join(root, terminalMacroLibraryFilename), filepath.Join(root, "Network_Toolkit.ssh_known_hosts.json")
	macrosBefore, err := os.ReadFile(macroFile)
	if err != nil {
		t.Fatal(err)
	}
	hostsBefore, err := os.ReadFile(hostFile)
	if err != nil {
		t.Fatal(err)
	}

	holder := startSharedProcess(t, root, "hold-shared")
	wait := sharedLockWait
	sharedLockWait = 200 * time.Millisecond
	t.Cleanup(func() { sharedLockWait = wait })
	library.Macros = append(library.Macros, macroHTTPDefinition("second"))
	if r := viewTestRequest(app, app.token, http.MethodPut, macroPath, library, nil); r.Code != http.StatusConflict {
		t.Fatalf("a macro save while another process held the lock: %d %s", r.Code, r.Body.String())
	}
	if r := requestApp(app, "DELETE", "/api/terminals/known-hosts", `{"host":"core.test:22"}`, true); r.Code == 200 {
		t.Fatalf("a trusted host was removed while another process held the lock: %s", r.Body.String())
	}
	if after, err := os.ReadFile(macroFile); err != nil || !bytes.Equal(macrosBefore, after) {
		t.Fatalf("the macro library changed while another process held the lock: %v", err)
	}
	if after, err := os.ReadFile(hostFile); err != nil || !bytes.Equal(hostsBefore, after) {
		t.Fatalf("SSH host trust changed while another process held the lock: %v", err)
	}
	// Reading both, and checking a host key, is not blocked.
	if got := knownHostsOf(t, app); !sameIDs(got, "core.test:22") {
		t.Fatalf("trusted hosts while another process held the lock: %v", got)
	}
	if got := macroHTTPLibrary(t, requestApp(app, http.MethodGet, macroPath, "", true), http.StatusOK); len(got.Macros) != 1 {
		t.Fatalf("macros while another process held the lock: %+v", got.Macros)
	}

	holder.finish()
	sharedLockWait = wait
	if got := macroHTTPLibrary(t, viewTestRequest(app, app.token, http.MethodPut, macroPath, library, nil), http.StatusOK); len(got.Macros) != 2 {
		t.Fatalf("the macro save after the other process finished: %+v", got.Macros)
	}
	if r := requestApp(app, "DELETE", "/api/terminals/known-hosts", `{"host":"core.test:22"}`, true); r.Code != 200 {
		t.Fatalf("remove after the other process finished: %d %s", r.Code, r.Body.String())
	}
	if got := knownHostsOf(t, app); len(got) != 0 {
		t.Fatalf("trusted hosts after the removal: %v", got)
	}
}
