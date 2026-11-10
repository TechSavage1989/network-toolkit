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

func TestColorPickerPreferencesSurviveStateAndNarrowTerminalDisplays(t *testing.T) {
	a := testApplication(t)
	body := `{"accentColor":"#112233","colorPickerPreferences":{"recent":["#ABCDEF"],"saved":["#12345678"],"password":"palette-canary"}}`
	response := requestApp(a, http.MethodPut, "/api/state", body, true)
	if response.Code != http.StatusOK {
		t.Fatalf("state write: %d %s", response.Code, response.Body.String())
	}
	response = requestApp(a, http.MethodGet, "/api/state", "", true)
	var original map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &original); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(response.Body.String(), "palette-canary") {
		t.Fatal("arbitrary palette field persisted")
	}
	want := safeColorPickerPreferences(original["colorPickerPreferences"])
	var single map[string]any
	if err := json.Unmarshal(terminalSingleViewDisplay(original), &single); err != nil {
		t.Fatal(err)
	}
	for name, display := range map[string]map[string]any{"single": single, "workspace": terminalWorkspaceDisplay(original)} {
		if !reflect.DeepEqual(safeColorPickerPreferences(display["colorPickerPreferences"]), want) {
			t.Fatalf("%s display lost saved colors", name)
		}
		if _, exists := display["accentColor"]; exists {
			t.Fatalf("%s broadened unrelated display preferences", name)
		}
	}
	// Color preferences are shared by every app of the data folder: a
	// standalone Terminal uses the data folder's, and keeps no copy.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Network_Toolkit.settings.json"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	data := standaloneTerminalState(t, root)
	var loaded map[string]any
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(safeColorPickerPreferences(loaded["colorPickerPreferences"]), want) || strings.Contains(string(data), "palette-canary") {
		t.Fatal("the standalone Terminal lost color preferences or received noncolor data")
	}
	path := filepath.Join(terminalWorkspace(root), "Network_Toolkit.settings.json")
	own, err := os.ReadFile(path)
	if err != nil || strings.Contains(string(own), "colorPickerPreferences") {
		t.Fatalf("the Terminal workspace keeps its own copy of shared color preferences: %s %v", own, err)
	}
	if err = os.WriteFile(filepath.Join(root, "Network_Toolkit.settings.json"), []byte(`{"colorPickerPreferences":{"saved":["#654321"]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err = seedTerminalWorkspace(root, terminalWorkspace(root)); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(own) {
		t.Fatal("existing terminal workspace was overwritten")
	}
}
