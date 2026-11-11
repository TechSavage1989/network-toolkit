//go:build linux

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// buildSeparatePrograms builds the three programs of this checkout with the
// same tags as build.sh (Linux builds: no resource objects are linked) into
// dir, under the names programName gives them.
func buildSeparatePrograms(t *testing.T, dir string) {
	t.Helper()
	goTool := filepath.Join(runtime.GOROOT(), "bin", "go")
	for _, edition := range []string{"", standaloneEditorMode, "terminal"} {
		args := []string{"build", "-mod=vendor", "-o", filepath.Join(dir, programName(edition))}
		switch edition {
		case standaloneEditorMode:
			args = append(args, "-tags", "ntk_editor")
		case "terminal":
			args = append(args, "-tags", "ntk_terminal")
		}
		cmd := exec.Command(goTool, append(args, ".")...)
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", programName(edition), err, out)
		}
	}
}

type programRun struct {
	cmd    *exec.Cmd
	output *bytes.Buffer
}

// runProgram starts a program. Its output is collected, so wait returns only
// when the program and any program it forwarded to (which shares its output)
// have finished.
func runProgram(t *testing.T, env []string, path string, args ...string) *programRun {
	t.Helper()
	return startProgram(t, env, new(bytes.Buffer), path, args...)
}

// runQuiet starts a program whose output is discarded: wait returns when that
// process ends, while a program it forwarded to keeps running.
func runQuiet(t *testing.T, env []string, path string, args ...string) *programRun {
	t.Helper()
	return startProgram(t, env, nil, path, args...)
}

func startProgram(t *testing.T, env []string, output *bytes.Buffer, path string, args ...string) *programRun {
	t.Helper()
	cmd := exec.Command(path, args...)
	cmd.Env = env
	run := &programRun{cmd: cmd, output: output}
	if output != nil {
		cmd.Stdout, cmd.Stderr = output, output
	} else {
		run.output = new(bytes.Buffer)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return run
}

func (r *programRun) wait(t *testing.T, limit time.Duration) string {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- r.cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("%s: %v\n%s", filepath.Base(r.cmd.Path), err, r.output)
		}
	case <-time.After(limit):
		_ = r.cmd.Process.Kill()
		t.Fatalf("%s %v did not finish\n%s", filepath.Base(r.cmd.Path), r.cmd.Args[1:], r.output)
	}
	return r.output.String()
}

func (r *programRun) stop(t *testing.T) {
	t.Helper()
	if r.cmd.ProcessState != nil {
		return
	}
	_ = r.cmd.Process.Signal(syscall.SIGINT)
	done := make(chan struct{})
	go func() { _ = r.cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		_ = r.cmd.Process.Kill()
		<-done
	}
}

// waitForRecord waits for a workspace's instance record naming another
// instance than notID, and returns it with the file of its process.
func waitForRecord(t *testing.T, dir, notID string) (terminalWindowInstance, string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		record, err := readPeerInstance(dir)
		if err == nil && record.InstanceID != notID {
			exe, _ := os.Readlink("/proc/" + strconv.Itoa(record.PID) + "/exe")
			return record, exe
		}
		if time.Now().After(deadline) {
			t.Fatalf("no instance record in %s: %v", dir, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

var sessionTokenPattern = regexp.MustCompile(`__networkToolkitNativeSession:'([a-f0-9]{64})'`)

// pageCall reads the session token from the app's page (Linux headless mode
// serves it in the page) and makes one API request with it.
func pageCall(t *testing.T, host, method, path, body string) []byte {
	t.Helper()
	page, err := http.Get("http://" + host + "/")
	if err != nil {
		t.Fatal(err)
	}
	html, _ := io.ReadAll(page.Body)
	page.Body.Close()
	match := sessionTokenPattern.FindSubmatch(html)
	if match == nil {
		t.Fatal("the page carries no session token")
	}
	req, _ := http.NewRequest(method, "http://"+host+path, strings.NewReader(body))
	req.Header.Set("X-Session-Token", string(match[1]))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		t.Fatalf("%s %s: %d %s", method, path, resp.StatusCode, b)
	}
	return b
}

// The three programs of one folder, and Network_Toolkit.exe of another folder
// without separate programs, against one data root: the same standalone app
// is one single instance whichever program starts it, requests are handed
// over across program names, and Network_Toolkit.exe forwards to the separate
// program beside it.
func TestSeparateProgramsShareOneInstanceWithRealBinaries(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the three programs")
	}
	programs, mainOnly := t.TempDir(), t.TempDir()
	buildSeparatePrograms(t, programs)
	main, editor, terminal := filepath.Join(programs, programName("")), filepath.Join(programs, programName(standaloneEditorMode)), filepath.Join(programs, programName("terminal"))
	soloMain := filepath.Join(mainOnly, programName(""))
	if b, err := os.ReadFile(main); err != nil || os.WriteFile(soloMain, b, 0700) != nil {
		t.Fatal("copy the main program", err)
	}
	// The default data root of a home of the test's own: a link is opened with
	// exactly "--terminal --open-url <link>", without --data-dir.
	home := t.TempDir()
	env := append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, ".config"), "XDG_DATA_HOME="+filepath.Join(home, ".local", "share"))
	root := filepath.Join(home, ".config", "NetworkToolkit")
	data := []string{"--no-browser"}
	terminalDir, editorDir := peerWorkspaceDir(root, "terminal"), peerWorkspaceDir(root, standaloneEditorMode)

	// 1. Network_Toolkit.exe --terminal with the separate program beside it
	// forwards: the Terminal that opens is the separate program.
	runQuiet(t, env, main, append([]string{"--terminal"}, data...)...).wait(t, 20*time.Second)
	record, exe := waitForRecord(t, terminalDir, "")
	if exe != terminal {
		t.Fatalf("the Terminal runs in %s, want the separate program %s", exe, terminal)
	}
	defer func() { _ = syscall.Kill(record.PID, syscall.SIGINT) }()

	// 2. Network_Toolkit.exe of a folder without separate programs, and the
	// separate Terminal program itself with a link: both hand over to the
	// running Terminal and finish. No second Terminal opens.
	for _, handover := range [][]string{{soloMain, "--terminal", "--no-browser"}, {terminal, "--open-url", "ssh://operator@192.0.2.10:2222"}, {main, "--terminal", "--open-url", "telnet://192.0.2.10:2323"}} {
		if out := runProgram(t, env, handover[0], handover[1:]...).wait(t, 20*time.Second); strings.Contains(out, "Network Toolkit:") {
			t.Fatalf("%s %v: %s", filepath.Base(handover[0]), handover[1:], out)
		}
	}
	if again, _ := waitForRecord(t, terminalDir, ""); again.InstanceID != record.InstanceID {
		t.Fatal("a second Terminal took over the workspace")
	}
	var inbox terminalWindowBatch
	if err := json.Unmarshal(pageCall(t, record.Host, http.MethodGet, "/api/terminal-window/requests", ""), &inbox); err != nil {
		t.Fatal(err)
	}
	var links []string
	for _, request := range inbox.Requests {
		if request.Link != nil {
			links = append(links, request.Config.Type+" "+request.Config.Host+":"+strconv.Itoa(request.Config.Port))
		}
	}
	if strings.Join(links, ", ") != "ssh 192.0.2.10:2222, telnet 192.0.2.10:2323" {
		t.Fatalf("the links that reached the running Terminal: %v", links)
	}

	// 3. The other way round: Network_Toolkit.exe runs the Terminal itself
	// (no separate program beside it); the separate program hands over to it.
	_ = syscall.Kill(record.PID, syscall.SIGINT)
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if _, err := os.Stat(filepath.Join(terminalDir, terminalInstanceFilename)); os.IsNotExist(err) {
			break
		}
	}
	soloOwner := runProgram(t, env, soloMain, append([]string{"--terminal"}, data...)...)
	defer soloOwner.stop(t)
	record, exe = waitForRecord(t, terminalDir, record.InstanceID)
	if exe != soloMain {
		t.Fatalf("the Terminal runs in %s, want %s", exe, soloMain)
	}
	runProgram(t, env, terminal, data...).wait(t, 20*time.Second)
	if again, _ := waitForRecord(t, terminalDir, ""); again.InstanceID != record.InstanceID {
		t.Fatal("the separate program opened a second Terminal")
	}

	// 4. The Text Editor: Network_Toolkit.exe --editor-standalone --open-file
	// forwards to the separate program, which opens the file; a second file
	// started through Network_Toolkit.exe elsewhere is handed to it.
	first, second := filepath.Join(home, "first.txt"), filepath.Join(home, "second.log")
	for _, file := range []string{first, second} {
		if err := os.WriteFile(file, []byte("synthetic"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	runQuiet(t, env, main, append([]string{"--editor-standalone", "--open-file", first}, data...)...).wait(t, 20*time.Second)
	editorRecord, exe := waitForRecord(t, editorDir, "")
	if exe != editor {
		t.Fatalf("the Text Editor runs in %s, want the separate program %s", exe, editor)
	}
	defer func() { _ = syscall.Kill(editorRecord.PID, syscall.SIGINT) }()
	runProgram(t, env, soloMain, append([]string{"--editor-standalone", "--open-file", second}, data...)...).wait(t, 20*time.Second)
	var launches struct {
		Requests []editorLaunchRequest `json:"requests"`
	}
	if err := json.Unmarshal(pageCall(t, editorRecord.Host, http.MethodPost, editorLaunchEndpoint, "{}"), &launches); err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, request := range launches.Requests {
		paths = append(paths, request.Path)
	}
	if strings.Join(paths, ", ") != first+", "+second {
		t.Fatalf("the files that reached the Text Editor: %v", paths)
	}

	// 5. A separate program refuses the other app's switch; nothing changes.
	refused := runProgram(t, env, terminal, append([]string{"--editor-standalone"}, data...)...).wait(t, 20*time.Second)
	if !strings.Contains(refused, programName("terminal")+" opens Network Toolkit Terminal only") {
		t.Fatalf("no refusal: %s", refused)
	}
	refused = runProgram(t, env, editor, append([]string{"--terminal"}, data...)...).wait(t, 20*time.Second)
	if !strings.Contains(refused, programName(standaloneEditorMode)+" opens Network Toolkit Text Editor only") {
		t.Fatalf("no refusal: %s", refused)
	}
	if again, _ := waitForRecord(t, editorDir, ""); again.InstanceID != editorRecord.InstanceID {
		t.Fatal("the refused start changed the Text Editor")
	}
	// Each separate program prints its version like Network_Toolkit.exe.
	for _, program := range []string{editor, terminal} {
		if out := runProgram(t, env, program, "--version").wait(t, 20*time.Second); !strings.HasPrefix(out, appName+" ") {
			t.Fatalf("%s --version: %q", filepath.Base(program), out)
		}
	}
}
