package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
)

func savedFilterState(t *testing.T, data string) map[string]any {
	t.Helper()
	var state map[string]any
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func TestSavedResultFiltersNarrowSchemaAndMigration(t *testing.T) {
	old := savedFilterState(t, `{"listPreferences":{"network:discover":{"query":"keep","respondedOnly":false}},"terminalFontSize":17}`)
	before, _ := json.Marshal(old)
	sanitizeSavedResultFilters(old)
	after, _ := json.Marshal(old)
	if !bytes.Equal(before, after) {
		t.Fatal("migration changed existing preferences")
	}
	state := savedFilterState(t, `{"savedResultFilters":{"version":1,"secret":"canary-root","scopes":{"network:discover":[{"id":"one","name":"Office","password":"canary-password","filter":{"query":"192.0.2.1\nAA-BB-CC-00-00-01","status":"open","respondedOnly":false,"rows":["canary-result"],"credentialId":"canary-credential"}},{"id":"two","name":"OFFICE","filter":{"query":"duplicate"}}],"result:ping:samples":[{"id":"loss","name":"Loss","filter":{"query":"router","status":"timeout","legacyStatus":"all","type":"tcp","statistic":"loss","respondedOnly":true}}],"network:__proto__":[{"id":"bad","name":"bad","filter":{"query":"canary-prototype"}}]}}}`)
	sanitizeSavedResultFilters(state)
	raw, _ := json.Marshal(state)
	if bytes.Contains(raw, []byte("canary")) {
		t.Fatalf("unknown or secret fields survived: %s", raw)
	}
	scopes := state["savedResultFilters"].(map[string]any)["scopes"].(map[string]any)
	network := scopes["network:discover"].([]any)
	if len(network) != 1 {
		t.Fatal("duplicate names survived")
	}
	filter := network[0].(map[string]any)["filter"].(map[string]any)
	if filter["query"] != "192.0.2.1 AA-BB-CC-00-00-01" || filter["status"] != "open" || filter["respondedOnly"] != false {
		t.Fatal("saved options changed")
	}
	result := scopes["result:ping:samples"].([]any)[0].(map[string]any)["filter"].(map[string]any)
	if result["type"] != "tcp" || result["statistic"] != "loss" || result["legacyStatus"] != "all" {
		t.Fatal("result controls lost")
	}
	if _, ok := result["respondedOnly"]; ok {
		t.Fatal("incompatible option survived")
	}
}

func TestSavedResultFiltersBoundsAndInvalidRecords(t *testing.T) {
	entries := []any{}
	for i := 0; i < 30; i++ {
		entries = append(entries, map[string]any{"id": fmt.Sprintf("f%d", i), "name": fmt.Sprintf("Filter %d", i), "filter": map[string]any{"query": strings.Repeat("😀", 16384), "status": "all"}})
	}
	state := map[string]any{"savedResultFilters": map[string]any{"version": float64(1), "scopes": map[string]any{"network:discover": entries}}}
	sanitizeSavedResultFilters(state)
	rows := state["savedResultFilters"].(map[string]any)["scopes"].(map[string]any)["network:discover"].([]any)
	if len(rows) != 8 {
		t.Fatalf("combined 262144 UTF16 unit bound not enforced: %d", len(rows))
	}
	for _, scope := range []string{"network:__proto__", "result:constructor", "result:ping:prototype", "terminal:profile", "result:ping/bad"} {
		if validSavedFilterScope(scope) {
			t.Fatalf("unsafe scope accepted: %s", scope)
		}
	}
	for _, test := range []struct{ scope, query string }{{"network:discover", strings.Repeat("x", 32769)}, {"result:ping:samples", strings.Repeat("x", 301)}} {
		if _, ok := cleanSavedFilter(map[string]any{"query": test.query}, test.scope); ok {
			t.Fatal("oversized query accepted")
		}
	}
	for _, field := range []string{"query", "status", "type", "legacyStatus", "statistic"} {
		if _, ok := cleanSavedFilter(map[string]any{field: map[string]any{"secret": "canary"}}, "result:ping:samples"); ok {
			t.Fatalf("nested %s accepted", field)
		}
	}
}

func TestSavedResultFiltersSaveRestartAndNoConnections(t *testing.T) {
	a := testApplication(t)
	body := `{"listPreferences":{"network:discover":{"query":"existing","respondedOnly":false}},"savedResultFilters":{"version":1,"scopes":{"network:discover":[{"id":"office","name":"Office","filter":{"query":"192.0.2.1, AA-BB-CC-00-00-01","status":"open","respondedOnly":false,"password":"filter-secret-canary","rows":["result-secret-canary"]}}],"result:ping:samples":[{"id":"loss","name":"Loss","filter":{"query":"router","status":"timeout","type":"tcp","legacyStatus":"all","statistic":"loss"}}]}}}`
	response := requestApp(a, http.MethodPut, "/api/state", body, true)
	if response.Code != http.StatusOK {
		t.Fatalf("save: %d %s", response.Code, response.Body.String())
	}
	raw, err := os.ReadFile(a.dataPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("secret-canary")) {
		t.Fatal("save retained unallowlisted result or credential fields")
	}
	b := newApplication(context.Background(), a.dataPath)
	defer b.stop()
	b.host = a.host
	response = requestApp(b, http.MethodGet, "/api/state", "", true)
	if response.Code != http.StatusOK {
		t.Fatalf("reload: %d", response.Code)
	}
	if !bytes.Equal(bytes.TrimSpace(response.Body.Bytes()), bytes.TrimSpace(raw)) {
		var got, want any
		json.Unmarshal(response.Body.Bytes(), &got)
		json.Unmarshal(raw, &want)
		a, _ := json.Marshal(got)
		b, _ := json.Marshal(want)
		if !bytes.Equal(a, b) {
			t.Fatal("restart changed saved filters")
		}
	}
	data := savedFilterState(t, response.Body.String())
	scopes := data["savedResultFilters"].(map[string]any)["scopes"].(map[string]any)
	if len(scopes) != 2 {
		t.Fatal("tool scoped saved filters lost")
	}
	if len(a.terminals.List()) != 0 || len(b.terminals.List()) != 0 {
		t.Fatal("saved filter persistence started a connection")
	}
}
