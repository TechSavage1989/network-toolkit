package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicBootstrapNeverCarriesNativeCapability(t *testing.T) {
	secret := strings.Repeat("ab", 32)
	if value := publicBootstrapToken(secret, true); value != "__SESSION_TOKEN__" || strings.Contains(value, secret) {
		t.Fatal("native capability escaped into public HTML bootstrap")
	}
	if value := publicBootstrapToken(secret, false); value != secret {
		t.Fatal("isolated development bootstrap changed")
	}
}

func TestTerminalActivationRecordIsNotGeneralAPICapability(t *testing.T) {
	a := testApplication(t)
	a.windowMode = "terminal"
	dir := t.TempDir()
	cleanup, err := a.registerTerminalWindow(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	raw, err := os.ReadFile(filepath.Join(dir, terminalInstanceFilename))
	if err != nil || strings.Contains(string(raw), a.token) {
		t.Fatal("general API capability was written to an instance record")
	}
	record, err := readTerminalWindowInstance(dir)
	if err != nil || record.Version != 2 || record.Token != a.windowActivationToken || record.Token == a.token {
		t.Fatal("instance activation capability is not separate")
	}
	for _, path := range []string{"/api/credentials", "/api/state", "/api/terminal-views", "/api/terminals"} {
		for _, header := range []string{"X-Window-Activation-Token", "X-Session-Token"} {
			r := httptest.NewRequest(http.MethodGet, "http://"+a.host+path, nil)
			r.Header.Set(header, record.Token)
			w := httptest.NewRecorder()
			a.ServeHTTP(w, r)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("activation capability authorized %s with %s: %d", path, header, w.Code)
			}
		}
	}
	activate := func(origin string) int {
		r := httptest.NewRequest(http.MethodPost, "http://"+a.host+"/api/terminal-window", strings.NewReader(`{}`))
		r.Header.Set("X-Window-Activation-Token", record.Token)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		return w.Code
	}
	if status := activate(""); status != http.StatusOK {
		t.Fatalf("bounded activation failed: %d", status)
	}
	if status := activate("https://attacker.invalid"); status != http.StatusForbidden {
		t.Fatal("activation bypassed Origin checks")
	}
	a.windowMode = "main"
	if status := activate(""); status != http.StatusUnauthorized {
		t.Fatal("activation capability was accepted by a non-Terminal workspace")
	}
	// Version 1 persisted the broad session token. Never reuse that old record
	// as the new least-privilege protocol, including during mixed-version runs.
	record.Version = 1
	old, _ := json.Marshal(record)
	if err := os.WriteFile(filepath.Join(dir, terminalInstanceFilename), old, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readTerminalWindowInstance(dir); err == nil {
		t.Fatal("legacy general-capability activation record was accepted")
	}
}
