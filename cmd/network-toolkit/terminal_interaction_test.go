package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTerminalInteractionPersistenceAndWindowTransfer(t *testing.T) {
	a := testApplication(t)
	a.dataPath = filepath.Join(filepath.Dir(a.dataPath), "Network_Toolkit.settings.json")
	body := `{"terminalInteraction":{"copyOnSelect":false,"copyOnClick":false,"rightClick":"menu","ctrlC":"copy-selection","menuItems":["copy-line","copy-token","paste","paste","not-a-command",{"password":"secret-canary"}],"password":"secret-canary","command":"secret-canary"}}`
	if response := requestApp(a, http.MethodPut, "/api/state", body, true); response.Code != http.StatusOK {
		t.Fatalf("save mouse and keyboard preferences: %d %s", response.Code, response.Body.String())
	}
	want := map[string]any{
		"copyOnSelect": false, "copyOnClick": false, "rightClick": "menu", "ctrlC": "copy-selection",
		"pasteReview": true, "pasteIncludeCommands": false, "autoPager": true, "pagerKey": "auto",
		// Saved before revision 2, so Send to Text Editor is added once.
		"menuItems": []any{"copy-line", "copy-token", "paste", "send-editor"}, "menuRevision": float64(2),
		// Paste warnings (v1.21.9) default on.
		"pasteWarnings": map[string]any{"homograph": true, "pipeToShell": true, "shellProfile": true, "systemArchive": true, "link": true},
	}
	check := func(label string, data []byte) {
		t.Helper()
		if strings.Contains(string(data), "secret-canary") || strings.Contains(string(data), "not-a-command") {
			t.Fatalf("%s retained unrecognized data", label)
		}
		var state map[string]any
		if err := json.Unmarshal(data, &state); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(state["terminalInteraction"], want) {
			t.Fatalf("%s lost terminal preferences: %#v", label, state["terminalInteraction"])
		}
	}
	raw, err := os.ReadFile(a.dataPath)
	if err != nil {
		t.Fatal(err)
	}
	check("saved settings", raw)
	check("linked session", a.terminalViewDisplay())
	var input map[string]any
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	// A workspace may be normalized when accepted and again at checkpoint.
	workspace, err := json.Marshal(terminalWorkspaceDisplay(terminalWorkspaceDisplay(input)))
	if err != nil {
		t.Fatal(err)
	}
	check("linked workspace", workspace)
	dest := terminalWorkspace(filepath.Dir(a.dataPath))
	if err := os.MkdirAll(dest, 0700); err != nil {
		t.Fatal(err)
	}
	if err := seedTerminalWorkspace(filepath.Dir(a.dataPath), dest); err != nil {
		t.Fatal(err)
	}
	seeded, err := os.ReadFile(filepath.Join(dest, "Network_Toolkit.settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	check("independent window", seeded)
	if len(a.terminals.List()) != 0 {
		t.Fatal("saving display preferences created a connection")
	}
}

func TestTerminalInteractionInvalidImportUsesDefaults(t *testing.T) {
	a := testApplication(t)
	body := `{"terminalInteraction":{"copyOnSelect":"false","copyOnClick":{},"rightClick":{"password":"secret-canary"},"ctrlC":"run-command","menuItems":"paste"}}`
	if response := requestApp(a, http.MethodPut, "/api/state", body, true); response.Code != http.StatusOK {
		t.Fatalf("import terminal preferences: %d %s", response.Code, response.Body.String())
	}
	response := requestApp(a, http.MethodGet, "/api/state", "", true)
	var state map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	prefs := state["terminalInteraction"].(map[string]any)
	if prefs["copyOnSelect"] != true || prefs["copyOnClick"] != true || prefs["rightClick"] != "paste" || prefs["ctrlC"] != "copy-selection" || prefs["pasteReview"] != true || prefs["pasteIncludeCommands"] != false || prefs["autoPager"] != true || prefs["pagerKey"] != "auto" || len(prefs["menuItems"].([]any)) != 16 || prefs["menuRevision"] != float64(2) {
		t.Fatalf("invalid import changed default terminal behavior: %#v", prefs)
	}
	if strings.Contains(response.Body.String(), "secret-canary") || strings.Contains(response.Body.String(), "run-command") {
		t.Fatal("invalid preference payload survived normalization")
	}
}

func TestTerminalInteractionDefaultsPreserveExplicitChoices(t *testing.T) {
	for _, value := range []any{nil, map[string]any{}, map[string]any{"autoPager": "false", "ctrlC": "invalid"}} {
		prefs := safeTerminalInteraction(value)
		if prefs["autoPager"] != true || prefs["ctrlC"] != "copy-selection" || prefs["copyOnSelect"] != true || prefs["copyOnClick"] != true || prefs["pasteReview"] != true || prefs["rightClick"] != "paste" || prefs["pagerKey"] != "auto" {
			t.Fatalf("new/unset preferences do not match requested defaults: %#v", prefs)
		}
	}
	saved := map[string]any{"copyOnSelect": false, "copyOnClick": false, "pasteReview": false, "autoPager": false, "ctrlC": "send", "rightClick": "menu", "pagerKey": "enter"}
	prefs := safeTerminalInteraction(saved)
	for key, want := range saved {
		if prefs[key] != want {
			t.Fatalf("explicit saved preference %s changed: got %#v want %#v", key, prefs[key], want)
		}
	}
	if again := safeTerminalInteraction(prefs); !reflect.DeepEqual(again, prefs) {
		t.Fatalf("repeated normalization changed explicit saved choices: %#v", again)
	}
	// Removing the overrides (the normalization/reset path) restores defaults.
	defaults := safeTerminalInteraction(map[string]any{})
	if defaults["autoPager"] != true || defaults["ctrlC"] != "copy-selection" {
		t.Fatalf("empty preferences did not restore defaults: %#v", defaults)
	}
}

func TestTerminalInteractionExplicitPagerOffAndSendCtrlCSurviveStateRoundTrip(t *testing.T) {
	a := testApplication(t)
	body := `{"terminalInteraction":{"autoPager":false,"ctrlC":"send","pasteReview":false,"rightClick":"menu","pagerKey":"enter"}}`
	if response := requestApp(a, http.MethodPut, "/api/state", body, true); response.Code != http.StatusOK {
		t.Fatalf("save explicit choices: %d %s", response.Code, response.Body.String())
	}
	response := requestApp(a, http.MethodGet, "/api/state", "", true)
	var state map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	prefs := state["terminalInteraction"].(map[string]any)
	if prefs["autoPager"] != false || prefs["ctrlC"] != "send" || prefs["pasteReview"] != false || prefs["rightClick"] != "menu" || prefs["pagerKey"] != "enter" {
		t.Fatalf("round trip overwrote explicit choices: %#v", prefs)
	}
}

func TestTerminalMenuAdditionShownOnceThenUserChoiceKept(t *testing.T) {
	old := safeTerminalInteraction(map[string]any{"menuItems": []any{"copy", "paste"}})
	if !reflect.DeepEqual(old["menuItems"], []string{"copy", "paste", "send-editor"}) || old["menuRevision"] != float64(2) {
		t.Fatalf("custom menu saved before revision 2 did not gain Send to Text Editor: %#v", old)
	}
	kept := safeTerminalInteraction(map[string]any{"menuItems": []any{"copy", "paste"}, "menuRevision": float64(2)})
	if !reflect.DeepEqual(kept["menuItems"], []string{"copy", "paste"}) {
		t.Fatalf("unticking Send to Text Editor after revision 2 was not kept: %#v", kept)
	}
}

func TestTerminalPasteWarningsDefaultOnAndKeepTurnedOffKindsAcrossRestart(t *testing.T) {
	for _, value := range []any{nil, map[string]any{}, map[string]any{"pasteWarnings": "off"}, map[string]any{"pasteWarnings": map[string]any{"link": "false", "unknown": false}}} {
		warnings := safeTerminalInteraction(value)["pasteWarnings"].(map[string]any)
		if len(warnings) != 5 {
			t.Fatalf("unexpected paste warning kinds: %#v", warnings)
		}
		for kind, on := range warnings {
			if on != true {
				t.Fatalf("%s must default on for %#v", kind, value)
			}
		}
	}
	a := testApplication(t)
	body := `{"terminalInteraction":{"pasteWarnings":{"link":false,"homograph":true,"secret":"secret-canary"}}}`
	if response := requestApp(a, http.MethodPut, "/api/state", body, true); response.Code != http.StatusOK {
		t.Fatalf("save paste warnings: %d %s", response.Code, response.Body.String())
	}
	response := requestApp(a, http.MethodGet, "/api/state", "", true)
	if strings.Contains(response.Body.String(), "secret-canary") {
		t.Fatal("unknown paste warning key was stored")
	}
	var state map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	warnings := state["terminalInteraction"].(map[string]any)["pasteWarnings"].(map[string]any)
	if warnings["link"] != false || warnings["homograph"] != true || warnings["pipeToShell"] != true {
		t.Fatalf("paste warning choices did not survive a restart: %#v", warnings)
	}
}
