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
	root, dest := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Network_Toolkit.settings.json"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	if err := seedTerminalWorkspace(root, dest); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dest, "Network_Toolkit.settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var seeded map[string]any
	if err = json.Unmarshal(data, &seeded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(safeColorPickerPreferences(seeded["colorPickerPreferences"]), want) || strings.Contains(string(data), "palette-canary") {
		t.Fatal("seed lost color preferences or retained noncolor data")
	}
	if err = os.WriteFile(filepath.Join(root, "Network_Toolkit.settings.json"), []byte(`{"colorPickerPreferences":{"saved":["#654321"]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err = seedTerminalWorkspace(root, dest); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(data) {
		t.Fatal("existing independent terminal workspace was overwritten")
	}
}
