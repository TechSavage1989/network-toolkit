package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testApplication(t *testing.T) *application {
	t.Helper()
	a := newApplication(context.Background(), filepath.Join(t.TempDir(), "settings.json"))
	a.host = "127.0.0.1:38191"
	t.Cleanup(func() {
		a.stop()
		if a.terminals != nil {
			_ = a.terminals.Close()
		}
	})
	return a
}
func requestApp(a *application, method, path, body string, auth bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://"+a.host+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if auth {
		r.Header.Set("X-Session-Token", a.token)
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}

func TestAPISessionAndHostGuards(t *testing.T) {
	a := testApplication(t)
	if r := requestApp(a, "GET", "/api/state", "", false); r.Code != http.StatusUnauthorized {
		t.Fatalf("Unauthenticated API status %d", r.Code)
	}
	r := httptest.NewRequest("PUT", "http://attacker.invalid/api/state", strings.NewReader(`{"theme":"light"}`))
	r.Header.Set("X-Session-Token", a.token)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("Rebinding host status %d", w.Code)
	}
	r = httptest.NewRequest("PUT", "http://"+a.host+"/api/state", strings.NewReader(`{"theme":"light"}`))
	r.Header.Set("X-Session-Token", a.token)
	r.Header.Set("Origin", "https://attacker.invalid")
	w = httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("Cross-origin status %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("Unexpected public CORS")
	}
}

func TestStatePersistenceAndReplacement(t *testing.T) {
	a := testApplication(t)
	for _, body := range []string{`{"theme":"dark","targets":[{"name":"Lab","host":"192.0.2.1"}]}`, `{"theme":"light","targets":[]}`} {
		r := requestApp(a, "PUT", "/api/state", body, true)
		if r.Code != 200 {
			t.Fatalf("State write %d: %s", r.Code, r.Body.String())
		}
	}
	b := newApplication(context.Background(), a.dataPath)
	defer b.stop()
	b.host = a.host
	r := requestApp(b, "GET", "/api/state", "", true)
	var data map[string]any
	if err := json.Unmarshal(r.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if data["theme"] != "light" {
		t.Fatal("Settings did not survive a new application instance")
	}
	files, err := os.ReadDir(filepath.Dir(a.dataPath))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("Unexpected temporary files remain: %d", len(files))
	}
}

func TestStateRejectsOversizeAndExtraValues(t *testing.T) {
	a := testApplication(t)
	for _, body := range []string{`{} {}`, `null`, `[1,2]`, `{"value":"` + strings.Repeat("x", 2<<20) + `"}`} {
		r := requestApp(a, "PUT", "/api/state", body, true)
		if r.Code != 400 {
			t.Fatalf("Invalid state accepted: %d", r.Code)
		}
	}
	r := requestApp(a, "GET", "/api/state", "", true)
	if !bytes.Equal(r.Body.Bytes(), []byte(`{}`)) {
		t.Fatal("Invalid state changed saved values")
	}
}

func TestJobValidationAndHistoryPruning(t *testing.T) {
	a := testApplication(t)
	for _, body := range []string{`{"tool":"shell","target":"whoami"}`, `{"tool":"tcp","target":"localhost\nanything"}`, `{"tool":"tcp"} {}`} {
		r := requestApp(a, "POST", "/api/jobs", body, true)
		if r.Code != 400 {
			t.Fatalf("Invalid job accepted: %d", r.Code)
		}
	}
	for i := 0; i < maxConcurrent; i++ {
		id := strings.Repeat(string(rune('a'+i)), 24)
		a.jobs[id] = &Job{ID: id, Status: "running", cancel: func() {}}
	}
	r := requestApp(a, "POST", "/api/jobs", `{"tool":"tcp","target":"127.0.0.1","options":{"ports":"1"}}`, true)
	if r.Code != 429 {
		t.Fatalf("Job concurrency limit status %d", r.Code)
	}
	a.jobs["done"] = &Job{ID: "done", Status: "completed"}
	r = requestApp(a, "DELETE", "/api/jobs", "", true)
	if r.Code != 200 || len(a.jobs) != maxConcurrent {
		t.Fatal("History clear affected running jobs")
	}
}

func TestEmbeddedUIContainsSessionAndNoRemoteDependencies(t *testing.T) {
	a := testApplication(t)
	r := requestApp(a, "GET", "/", "", false)
	if r.Code != 200 {
		t.Fatal(r.Code)
	}
	if !strings.Contains(r.Body.String(), httpBootstrapToken(a.token)) {
		t.Fatal("Expected platform-specific interface bootstrap was missing")
	}
	if httpBootstrapToken(a.token) != a.token && strings.Contains(r.Body.String(), a.token) {
		t.Fatal("Native capability was exposed in HTTP interface HTML")
	}
	if strings.Contains(r.Body.String(), "__WINDOW_MODE__") || !strings.Contains(r.Body.String(), "WINDOW_MODE='main'") {
		t.Fatal("Main-window bootstrap mode was not replaced")
	}
	a.windowMode = "terminal"
	terminalResponse := requestApp(a, "GET", "/", "", false)
	if terminalResponse.Code != http.StatusOK || !strings.Contains(terminalResponse.Body.String(), "WINDOW_MODE='terminal'") || strings.Contains(terminalResponse.Body.String(), "__WINDOW_MODE__") {
		t.Fatal("Independent-window bootstrap mode was not replaced")
	}
	if strings.Contains(r.Body.String(), `<script src="http`) || strings.Contains(r.Body.String(), `<link rel="stylesheet" href="http`) {
		t.Fatal("Remote UI dependency")
	}
	if !strings.Contains(r.Header().Get("Content-Security-Policy"), "connect-src 'self'") {
		t.Fatal("UI may make external requests")
	}
}

func TestJobTargetListsOnlyForNetworkScans(t *testing.T) {
	a := testApplication(t)
	// Filling the running slots exercises HTTP validation without launching a
	// diagnostic. A valid target envelope reaches the concurrency guard (429).
	for i := 0; i < maxConcurrent; i++ {
		id := strings.Repeat(string(rune('a'+i)), 24)
		a.jobs[id] = &Job{ID: id, Status: "running", cancel: func() {}}
	}
	for _, tc := range []struct {
		name, tool, target string
		options            map[string]any
		want               int
	}{
		{"discovery lines", "discover", "192.0.2.1\r\n198.51.100.1-4", nil, 429},
		{"network ping lines", "ping", "192.0.2.0/30\n198.51.100.1", map[string]any{"scope": "subnet"}, 429},
		{"network tcp lines", "tcp", "192.0.2.0/30\n198.51.100.1", map[string]any{"scope": " subnet "}, 429},
		{"larger scope list", "discover", strings.Repeat("192.0.2.1\n", 1200), nil, 429},
		{"oversized scope list", "discover", strings.Repeat("192.0.2.1\n", 2000), nil, 400},
		{"nul in scope list", "discover", "192.0.2.1\x00198.51.100.1", nil, 400},
		{"single ping stays strict", "ping", "192.0.2.1\n198.51.100.1", nil, 400},
		{"single tcp stays strict", "tcp", "192.0.2.1\n198.51.100.1", map[string]any{"scope": "single"}, 400},
		{"invalid scope type stays strict", "tcp", "192.0.2.1\n198.51.100.1", map[string]any{"scope": []string{"subnet"}}, 400},
		{"http stays strict", "http", "https://example.test\nhttps://other.test", nil, 400},
		{"device check stays strict", "quick", "192.0.2.1\n198.51.100.1", map[string]any{"scope": "subnet"}, 400},
		{"single target length stays strict", "tcp", strings.Repeat("a", 2049), nil, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(map[string]any{"tool": tc.tool, "target": tc.target, "options": tc.options})
			if err != nil {
				t.Fatal(err)
			}
			response := requestApp(a, "POST", "/api/jobs", string(body), true)
			if response.Code != tc.want {
				t.Fatalf("target guard status %d, want %d: %s", response.Code, tc.want, response.Body.String())
			}
		})
	}
}
