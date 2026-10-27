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

func TestNetworkListPreferencesSaveAndReloadLongQueries(t *testing.T) {
	a := testApplication(t)
	entries := make([]string, 700)
	for i := range entries {
		entries[i] = fmt.Sprintf("02:00:00:00:%02x:%02x", i/256, i%256)
	}
	query := strings.Join(entries, " ")
	unicodeQuery := strings.Repeat("設備😀 ", 400)
	input := map[string]any{"listPreferences": map[string]any{
		"network:discover": map[string]any{
			"query": query, "sort": "address", "direction": "desc", "status": "open", "respondedOnly": false,
			"password": "secret-filter-canary", "rows": []any{map[string]any{"output": "secret-row-canary"}},
		},
		"network:arp": map[string]any{"query": unicodeQuery},
		"network:bad": map[string]any{"query": map[string]any{"password": "secret-nested-canary"}},
		"result:dns:accepted": map[string]any{"query": strings.Repeat("x", 512)},
		"result:dns:rejected": map[string]any{"query": strings.Repeat("x", 513)},
	}}
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if response := requestApp(a, http.MethodPut, "/api/state", string(body), true); response.Code != http.StatusOK {
		t.Fatalf("save long discovery filters: %d %s", response.Code, response.Body.String())
	}
	check := func(label string, data []byte) {
		t.Helper()
		if bytes.Contains(data, []byte("secret-")) {
			t.Fatalf("%s retained nested or unknown fields", label)
		}
		var state struct {
			Lists map[string]map[string]any `json:"listPreferences"`
		}
		if err := json.Unmarshal(data, &state); err != nil {
			t.Fatal(err)
		}
		discovery := state.Lists["network:discover"]
		if discovery["query"] != query || discovery["sort"] != "address" || discovery["direction"] != "desc" || discovery["status"] != "open" || discovery["respondedOnly"] != false {
			t.Fatalf("%s changed the complete discovery filter or its other controls", label)
		}
		if state.Lists["network:arp"]["query"] != unicodeQuery {
			t.Fatalf("%s changed the Unicode filter", label)
		}
		if state.Lists["result:dns:accepted"]["query"] != strings.Repeat("x", 512) {
			t.Fatalf("%s changed the existing non-network bound", label)
		}
		for _, view := range []string{"network:bad", "result:dns:rejected"} {
			if _, present := state.Lists[view]["query"]; present {
				t.Fatalf("%s retained invalid query for %s", label, view)
			}
		}
	}
	raw, err := os.ReadFile(a.dataPath)
	if err != nil {
		t.Fatal(err)
	}
	check("saved file", raw)
	b := newApplication(context.Background(), a.dataPath)
	defer b.stop()
	b.host = a.host
	response := requestApp(b, http.MethodGet, "/api/state", "", true)
	if response.Code != http.StatusOK {
		t.Fatalf("reload preferences: %d", response.Code)
	}
	check("new application GET", response.Body.Bytes())
	if len(a.terminals.List()) != 0 || len(b.terminals.List()) != 0 {
		t.Fatal("filter persistence created a terminal connection")
	}
}

func TestNetworkListQueryUsesBrowserUTF16Bound(t *testing.T) {
	for _, text := range []string{"", strings.Repeat("x", 32768), strings.Repeat("界", 32768), strings.Repeat("😀", 16384), strings.Repeat("x", 32766) + "😀"} {
		if !validNetworkListQuery(text) {
			t.Fatalf("valid browser-length query rejected: %d UTF-8 bytes", len(text))
		}
	}
	for _, text := range []string{strings.Repeat("x", 32769), strings.Repeat("界", 32769), strings.Repeat("😀", 16384) + "x", string([]byte{0xff})} {
		if validNetworkListQuery(text) {
			t.Fatalf("oversized or invalid query accepted: %d bytes", len(text))
		}
	}
	state := map[string]any{"listPreferences": map[string]any{
		"network:limit": map[string]any{"query": strings.Repeat("界", 32768), "sort": strings.Repeat("x", 129)},
		"network:over": map[string]any{"query": strings.Repeat("😀", 16384) + "x", "status": "all"},
		"other": map[string]any{"query": strings.Repeat("界", 171)},
	}}
	sanitizeListPreferences(state)
	views := state["listPreferences"].(map[string]any)
	if views["network:limit"].(map[string]any)["query"] != strings.Repeat("界", 32768) {
		t.Fatal("network query at the UTF-16 boundary did not survive")
	}
	for _, view := range []string{"network:over", "other"} {
		if _, present := views[view].(map[string]any)["query"]; present {
			t.Fatal("over-limit query was retained or partially truncated")
		}
	}
	if _, present := views["network:limit"].(map[string]any)["sort"]; present {
		t.Fatal("query expansion changed the sort field bound")
	}
}

func TestNetworkListPreferencesDoNotExpandStateRequestLimit(t *testing.T) {
	a := testApplication(t)
	if response := requestApp(a, http.MethodPut, "/api/state", `{"listPreferences":{"network:discover":{"query":"192.0.2.1 192.0.2.30"}}}`, true); response.Code != http.StatusOK {
		t.Fatalf("initial state save: %d", response.Code)
	}
	before := append([]byte(nil), requestApp(a, http.MethodGet, "/api/state", "", true).Body.Bytes()...)
	body := `{"listPreferences":{"network:discover":{"query":"` + strings.Repeat("x", 2<<20) + `"}}}`
	if response := requestApp(a, http.MethodPut, "/api/state", body, true); response.Code != http.StatusBadRequest {
		t.Fatalf("oversized settings request accepted: %d", response.Code)
	}
	after := requestApp(a, http.MethodGet, "/api/state", "", true).Body.Bytes()
	onDisk, err := os.ReadFile(a.dataPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || !bytes.Equal(before, onDisk) {
		t.Fatal("rejected oversized request changed the previous saved query")
	}
}
