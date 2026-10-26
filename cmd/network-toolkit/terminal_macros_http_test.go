package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"networktoolkit/internal/macros"
)

func macroHTTPDefinition(id string) macros.Definition {
	return macros.Definition{ID: id, Name: "Read device state", Steps: []macros.Step{{ID: "show", Kind: "command", Text: "show version"}}, Prompt: macros.DefaultPrompt, TimeoutMs: macros.DefaultTimeoutMs}
}

func macroHTTPLibrary(t *testing.T, response *httptest.ResponseRecorder, code int) terminalMacroLibrary {
	t.Helper()
	if response.Code != code {
		t.Fatalf("macro library status %d, want %d: %s", response.Code, code, response.Body.String())
	}
	var library terminalMacroLibrary
	if err := json.Unmarshal(response.Body.Bytes(), &library); err != nil {
		t.Fatal(err)
	}
	return library
}

func macroHTTPRun(t *testing.T, response *httptest.ResponseRecorder, code int) macros.Run {
	t.Helper()
	if response.Code != code {
		t.Fatalf("macro run status %d, want %d: %s", response.Code, code, response.Body.String())
	}
	var decoded struct {
		Run macros.Run `json:"run"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded.Run
}

func TestTerminalMacroLibrarySeparateStorageRevisionAndSecretDefaults(t *testing.T) {
	a := testApplication(t)
	path := "/api/terminal-automation/library"
	initial := macroHTTPLibrary(t, requestApp(a, http.MethodGet, path, "", true), http.StatusOK)
	if initial.Version != 1 || initial.Revision != "" || initial.Macros == nil || initial.Bindings == nil {
		t.Fatalf("first-use library must be an explicit empty versioned library: %#v", initial)
	}
	if response := requestApp(a, http.MethodPut, "/api/state", `{"theme":"light","reportNotes":"preserved settings"}`, true); response.Code != http.StatusOK {
		t.Fatal(response.Body.String())
	}
	settingsBefore, err := os.ReadFile(a.dataPath)
	if err != nil {
		t.Fatal(err)
	}
	definition := macroHTTPDefinition("audit")
	definition.Parameters = []macros.Parameter{{Name: "enablePassword", Label: "Enable password", Secret: true, DefaultValue: "secret-default-canary"}, {Name: "interfaceName", DefaultValue: "ethernet1"}}
	definition.Steps[0].Text = "show interface {{interfaceName}}"
	library := terminalMacroLibrary{Version: 1, Macros: []macros.Definition{definition}, Bindings: []terminalMacroBinding{{ProfileID: "saved-core", MacroID: definition.ID, OnConnect: true, OnReconnect: true}}}
	saved := macroHTTPLibrary(t, viewTestRequest(a, a.token, http.MethodPut, path, library, nil), http.StatusOK)
	if len(saved.Revision) != sha256.Size*2 || saved.Macros[0].Parameters[0].DefaultValue != "" || saved.Macros[0].Parameters[1].DefaultValue != "ethernet1" {
		t.Fatalf("saved defaults or revision are incorrect: %#v", saved)
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(a.dataPath), terminalMacroLibraryFilename))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	if saved.Revision != hex.EncodeToString(digest[:]) || bytes.Contains(raw, []byte("secret-default-canary")) || bytes.Contains(raw, []byte(`"revision"`)) {
		t.Fatalf("disk library exposed a secret or revision does not describe its exact bytes: %s", raw)
	}
	settingsAfter, err := os.ReadFile(a.dataPath)
	if err != nil || !bytes.Equal(settingsBefore, settingsAfter) {
		t.Fatal("macro save changed workspace settings", err)
	}
	reloaded := macroHTTPLibrary(t, requestApp(a, http.MethodGet, path, "", true), http.StatusOK)
	if reloaded.Revision != saved.Revision || len(reloaded.Bindings) != 1 || !reloaded.Bindings[0].OnReconnect {
		t.Fatalf("library round trip lost revision or startup bindings: %#v", reloaded)
	}
}

func TestTerminalMacroLibraryWritesBeyondMetadataLimitAndRejectsStaleWindows(t *testing.T) {
	a := testApplication(t)
	definition := macroHTTPDefinition("long-audit")
	definition.Steps[0].Text = strings.Repeat("show ", 6000)
	library := terminalMacroLibrary{Version: 1, Macros: []macros.Definition{definition}}
	saved := macroHTTPLibrary(t, viewTestRequest(a, a.token, http.MethodPut, "/api/terminal-automation/library", library, nil), http.StatusOK)
	filename := filepath.Join(filepath.Dir(a.dataPath), terminalMacroLibraryFilename)
	before, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) <= 16<<10 {
		t.Fatal("fixture must exercise a library larger than ordinary storage metadata")
	}
	if saved.Macros[0].Steps[0].Text != definition.Steps[0].Text {
		t.Fatal("large macro was truncated")
	}
	// Both windows read the same revision. Exactly one may replace it.
	library.Revision = saved.Revision
	var wait sync.WaitGroup
	responses := make(chan *httptest.ResponseRecorder, 2)
	for _, name := range []string{"First window", "Second window"} {
		wait.Add(1)
		go func(name string) {
			defer wait.Done()
			copy := library
			copy.Macros = append([]macros.Definition(nil), library.Macros...)
			copy.Macros[0].Name = name
			responses <- viewTestRequest(a, a.token, http.MethodPut, "/api/terminal-automation/library", copy, nil)
		}(name)
	}
	wait.Wait()
	close(responses)
	statuses := map[int]int{}
	for response := range responses {
		statuses[response.Code]++
	}
	if statuses[http.StatusOK] != 1 || statuses[http.StatusConflict] != 1 {
		t.Fatalf("concurrent library saves: %v", statuses)
	}
	latest := macroHTTPLibrary(t, requestApp(a, http.MethodGet, "/api/terminal-automation/library", "", true), http.StatusOK)
	if latest.Revision == saved.Revision {
		t.Fatal("successful update did not advance its content revision")
	}
	current, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if response := viewTestRequest(a, a.token, http.MethodPut, "/api/terminal-automation/library", library, nil); response.Code != http.StatusConflict {
		t.Fatalf("stale save: %d %s", response.Code, response.Body.String())
	}
	after, err := os.ReadFile(filename)
	if err != nil || !bytes.Equal(current, after) {
		t.Fatal("stale save modified the current library", err)
	}
}

func TestTerminalMacroLibraryRejectsConflictsAndMalformedRequestsWithoutWriting(t *testing.T) {
	a := testApplication(t)
	definition := macroHTTPDefinition("audit")
	base := terminalMacroLibrary{Version: 1, Macros: []macros.Definition{definition}}
	saved := macroHTTPLibrary(t, viewTestRequest(a, a.token, http.MethodPut, "/api/terminal-automation/library", base, nil), http.StatusOK)
	second := macroHTTPDefinition("other")
	firstShortcut, secondShortcut := definition, second
	firstShortcut.Shortcut, secondShortcut.Shortcut = "Ctrl+Shift+1", "shift+control+1"
	cases := map[string]terminalMacroLibrary{
		"duplicate ID":                  {Version: 1, Macros: []macros.Definition{definition, definition}},
		"duplicate normalized shortcut": {Version: 1, Macros: []macros.Definition{firstShortcut, secondShortcut}},
		"duplicate binding":             {Version: 1, Macros: []macros.Definition{definition}, Bindings: []terminalMacroBinding{{ProfileID: "saved", MacroID: "audit"}, {ProfileID: "saved", MacroID: "audit", OnReconnect: true}}},
		"unknown macro binding":         {Version: 1, Macros: []macros.Definition{definition}, Bindings: []terminalMacroBinding{{ProfileID: "saved", MacroID: "missing"}}},
		"empty saved session":           {Version: 1, Macros: []macros.Definition{definition}, Bindings: []terminalMacroBinding{{ProfileID: " ", MacroID: "audit"}}},
		"future version":                {Version: 2},
	}
	for name, library := range cases {
		t.Run(name, func(t *testing.T) {
			library.Revision = saved.Revision
			if response := viewTestRequest(a, a.token, http.MethodPut, "/api/terminal-automation/library", library, nil); response.Code != http.StatusBadRequest {
				t.Fatalf("invalid library: %d %s", response.Code, response.Body.String())
			}
		})
	}
	for _, body := range []string{`{"version":1,"unexpected":"retained"}`, `{"version":1} {}`, strings.Repeat(" ", terminalMacroLibraryLimit+1) + `{}`} {
		if response := requestApp(a, http.MethodPut, "/api/terminal-automation/library", body, true); response.Code != http.StatusBadRequest {
			t.Fatalf("malformed macro library: %d %.200s", response.Code, response.Body.String())
		}
	}
	if current := macroHTTPLibrary(t, requestApp(a, http.MethodGet, "/api/terminal-automation/library", "", true), http.StatusOK); current.Revision != saved.Revision || len(current.Macros) != 1 {
		t.Fatal("rejected saves changed the library")
	}
}

func TestTerminalMacroLibraryRejectsLinkedFilesAndStorageAncestors(t *testing.T) {
	for _, location := range []string{"library", "root"} {
		t.Run(location, func(t *testing.T) {
			a := testApplication(t)
			outside := t.TempDir()
			outsidePath := filepath.Join(outside, terminalMacroLibraryFilename)
			original := []byte(`{"version":1,"macros":[],"bindings":[]}`)
			if err := os.WriteFile(outsidePath, original, 0600); err != nil {
				t.Fatal(err)
			}
			if location == "library" {
				if err := os.Symlink(outsidePath, filepath.Join(filepath.Dir(a.dataPath), terminalMacroLibraryFilename)); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			} else {
				linked := filepath.Join(t.TempDir(), "linked-root")
				if err := os.Symlink(outside, linked); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				a.dataPath = filepath.Join(linked, "settings.json")
			}
			for _, method := range []string{http.MethodGet, http.MethodPut} {
				if response := requestApp(a, method, "/api/terminal-automation/library", `{"version":1}`, true); response.Code != http.StatusInternalServerError {
					t.Fatalf("linked %s %s accepted: %d %s", location, method, response.Code, response.Body.String())
				}
			}
			if current, err := os.ReadFile(outsidePath); err != nil || !bytes.Equal(current, original) {
				t.Fatal("linked source was modified", err)
			}
		})
	}
}

func TestTerminalMacroLibrarySeedIsSanitizedIndependentAndDoesNotOverwrite(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	definition := macroHTTPDefinition("seeded")
	definition.Parameters = []macros.Parameter{{Name: "password", Secret: true, DefaultValue: "seed-secret-canary"}}
	library := terminalMacroLibrary{Version: 1, Macros: []macros.Definition{definition}, Bindings: []terminalMacroBinding{{ProfileID: "saved", MacroID: "seeded", OnConnect: true}}}
	raw, err := json.Marshal(library)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, terminalMacroLibraryFilename), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := seedTerminalWorkspace(source, destination); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(destination, terminalMacroLibraryFilename)
	seeded, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(seeded, []byte("seed-secret-canary")) || !bytes.Contains(seeded, []byte("seeded")) || !bytes.Contains(seeded, []byte("saved")) {
		t.Fatalf("seeded library lost bindings or retained secrets: %s", seeded)
	}
	independent := []byte(`{"version":1,"macros":[],"bindings":[]}`)
	if err := os.WriteFile(filename, independent, 0600); err != nil {
		t.Fatal(err)
	}
	if err := seedTerminalWorkspace(source, destination); err != nil {
		t.Fatal(err)
	}
	if current, err := os.ReadFile(filename); err != nil || !bytes.Equal(current, independent) {
		t.Fatal("later launch overwrote the independent library", err)
	}
	if current, err := os.ReadFile(filepath.Join(source, terminalMacroLibraryFilename)); err != nil || !bytes.Equal(current, raw) {
		t.Fatal("seeding changed the source library", err)
	}
	// Persist an intentionally empty first-use library, so a later source
	// library cannot unexpectedly introduce macros into an existing workspace.
	emptySource, emptyDestination := t.TempDir(), t.TempDir()
	if err := seedTerminalMacroLibrary(emptySource, emptyDestination); err != nil {
		t.Fatal(err)
	}
	emptyBefore, err := os.ReadFile(filepath.Join(emptyDestination, terminalMacroLibraryFilename))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(emptySource, terminalMacroLibraryFilename), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := seedTerminalMacroLibrary(emptySource, emptyDestination); err != nil {
		t.Fatal(err)
	}
	if current, err := os.ReadFile(filepath.Join(emptyDestination, terminalMacroLibraryFilename)); err != nil || !bytes.Equal(current, emptyBefore) {
		t.Fatal("later source macros replaced an intentionally empty library", err)
	}
}

func TestTerminalAutomationHTTPAuthenticationAndDetachedTargetScope(t *testing.T) {
	a := testApplication(t)
	for _, path := range []string{"library", "targets", "runs", "send", "runs/example/control", "runs/example/output"} {
		if response := requestApp(a, http.MethodGet, "/api/terminal-automation/"+path, "", false); response.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated automation %s: %d", path, response.Code)
		}
	}
	session, other := viewTestConnected(t, a), viewTestConnected(t, a)
	var launch terminalViewLaunch
	a.launchTerminalView = func(_ context.Context, request terminalViewLaunch) (<-chan error, error) {
		launch = request
		return nil, nil
	}
	info, err := a.views.create(session.ID, viewTestSnapshot(a, session))
	if err != nil {
		t.Fatal(err)
	}
	if response := viewTestRequest(a, launch.ViewToken, http.MethodGet, "/api/terminal-automation/targets", nil, nil); response.Code != http.StatusConflict {
		t.Fatalf("opening view automated a session: %d %s", response.Code, response.Body.String())
	}
	if _, err := a.views.ready(info.ID); err != nil {
		t.Fatal(err)
	}
	response := viewTestRequest(a, launch.ViewToken, http.MethodGet, "/api/terminal-automation/targets", nil, nil)
	if response.Code != http.StatusOK {
		t.Fatal(response.Body.String())
	}
	var targets struct {
		Targets []macros.Target `json:"targets"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &targets); err != nil {
		t.Fatal(err)
	}
	if len(targets.Targets) != 1 || targets.Targets[0].ID != session.ID || strings.Contains(response.Body.String(), other.ID) {
		t.Fatalf("detached targets leaked another session: %s", response.Body.String())
	}
	refs := map[string]macros.TargetRef{}
	for _, target := range a.terminalAutomation.Targets() {
		refs[target.ID] = macros.TargetRef{ID: target.ID, Generation: target.Generation}
	}
	mixed := []macros.TargetRef{refs[session.ID], refs[other.ID]}
	for _, execute := range []bool{false, true} {
		response := viewTestRequest(a, launch.ViewToken, http.MethodPost, "/api/terminal-automation/send", macros.SendRequest{Targets: mixed, Text: "show version", Execute: execute}, nil)
		if response.Code != http.StatusForbidden {
			t.Fatalf("detached group send execute=%v: %d %s", execute, response.Code, response.Body.String())
		}
	}
	definition := macroHTTPDefinition("scoped")
	definition.Steps = []macros.Step{{ID: "pause", Kind: "delay", DelayMs: 30000}}
	if response := viewTestRequest(a, launch.ViewToken, http.MethodPost, "/api/terminal-automation/runs", macros.StartRequest{Macro: definition, Targets: mixed}, nil); response.Code != http.StatusForbidden {
		t.Fatalf("detached view started a mixed run: %d %s", response.Code, response.Body.String())
	}
	response = viewTestRequest(a, launch.ViewToken, http.MethodPost, "/api/terminal-automation/send", macros.SendRequest{Targets: []macros.TargetRef{refs[session.ID]}, Text: "show version"}, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("assigned session send failed: %d %s", response.Code, response.Body.String())
	}
	own := macroHTTPRun(t, viewTestRequest(a, launch.ViewToken, http.MethodPost, "/api/terminal-automation/runs", macros.StartRequest{Macro: definition, Targets: []macros.TargetRef{refs[session.ID]}}, nil), http.StatusCreated)
	macroHTTPRun(t, viewTestRequest(a, launch.ViewToken, http.MethodGet, "/api/terminal-automation/runs/"+own.ID, nil, nil), http.StatusOK)
	macroHTTPRun(t, viewTestRequest(a, launch.ViewToken, http.MethodPost, "/api/terminal-automation/runs/"+own.ID+"/control", map[string]string{"action": "stop"}, nil), http.StatusOK)
	viewTestWait(t, func() bool { run, err := a.terminalAutomation.Get(own.ID); return err == nil && run.State == "stopped" })
	group := macroHTTPRun(t, viewTestRequest(a, a.token, http.MethodPost, "/api/terminal-automation/runs", macros.StartRequest{Macro: definition, Targets: mixed}, nil), http.StatusCreated)
	for _, action := range []struct {
		method, suffix string
		body           any
	}{{http.MethodGet, "", nil}, {http.MethodGet, "/output?sessionId=" + session.ID, nil}, {http.MethodGet, "/output?sessionId=" + other.ID + "&format=json", nil}, {http.MethodPost, "/control", map[string]string{"action": "stop"}}} {
		response := viewTestRequest(a, launch.ViewToken, action.method, "/api/terminal-automation/runs/"+group.ID+action.suffix, action.body, nil)
		if response.Code != http.StatusForbidden {
			t.Fatalf("detached view reached mixed run %s: %d %s", action.suffix, response.Code, response.Body.String())
		}
	}
	response = viewTestRequest(a, launch.ViewToken, http.MethodGet, "/api/terminal-automation/runs", nil, nil)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), group.ID) || !strings.Contains(response.Body.String(), own.ID) {
		t.Fatalf("detached run listing lost own run or leaked group: %d %s", response.Code, response.Body.String())
	}
	macroHTTPRun(t, viewTestRequest(a, a.token, http.MethodPost, "/api/terminal-automation/runs/"+group.ID+"/control", map[string]string{"action": "stop"}, nil), http.StatusOK)
}

func TestTerminalAutomationHTTPLinkedWorkspaceUsesItsAssignedSessions(t *testing.T) {
	a, source, launch, sessions, _ := paneDetachFixture(t)
	other := viewTestConnected(t, a)
	response := viewTestRequest(a, launch.ViewToken, http.MethodGet, "/api/terminal-automation/targets", nil, nil)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), other.ID) {
		t.Fatalf("linked workspace listed an outside session: %d %s", response.Code, response.Body.String())
	}
	var decoded struct {
		Targets []macros.Target `json:"targets"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Targets) != len(sessions) {
		t.Fatalf("linked workspace lost assigned sessions: %s", response.Body.String())
	}
	refs := []macros.TargetRef{}
	for _, target := range decoded.Targets {
		refs = append(refs, macros.TargetRef{ID: target.ID, Generation: target.Generation})
	}
	response = viewTestRequest(a, launch.ViewToken, http.MethodPost, "/api/terminal-automation/send", macros.SendRequest{Targets: refs, Text: "show version"}, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("linked workspace could not send to its group: %d %s", response.Code, response.Body.String())
	}
	// Moving one pane to its own window updates this workspace's capability;
	// a stale group selection must not continue sending into that moved pane.
	moved := sessions[1]
	if result := viewTestRequest(a, launch.ViewToken, http.MethodPost, "/api/terminal-views/current/detach-session", map[string]any{"sessionId": moved.ID, "snapshot": viewTestSnapshot(a, moved)}, nil); result.Code != http.StatusCreated {
		t.Fatalf("detach fixture pane: %d %s", result.Code, result.Body.String())
	}
	response = viewTestRequest(a, launch.ViewToken, http.MethodPost, "/api/terminal-automation/send", macros.SendRequest{Targets: refs, Text: "show version"}, nil)
	if response.Code != http.StatusForbidden {
		t.Fatalf("stale workspace group reached its detached pane: %d %s", response.Code, response.Body.String())
	}
	info, ok := a.views.info(source.ID)
	if !ok || len(info.SessionIDs) != 2 {
		t.Fatalf("workspace assignments changed unexpectedly: %#v", info)
	}
	response = viewTestRequest(a, launch.ViewToken, http.MethodGet, "/api/terminal-automation/targets", nil, nil)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), moved.ID) || strings.Contains(response.Body.String(), other.ID) {
		t.Fatalf("workspace targets did not follow pane ownership: %d %s", response.Code, response.Body.String())
	}
}
