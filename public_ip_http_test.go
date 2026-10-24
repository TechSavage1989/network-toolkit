package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPublicIPAPIGuardsAndMainWindowScope(t *testing.T) {
	a := testApplication(t)
	var calls atomic.Int32
	a.publicIP.lookup = func(context.Context, int) publicIPFamilyResult {
		calls.Add(1)
		return publicIPFamilyResult{Status: "ok", Address: "8.8.8.8"}
	}
	for _, path := range []string{"/api/public-ip", "/api/public-ip/refresh"} {
		if r := requestApp(a, "GET", path, "", false); r.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated lookup: %d", r.Code)
		}
		for _, bad := range []string{"host", "origin", "fetch-site"} {
			r := httptest.NewRequest("POST", "http://"+a.host+path, strings.NewReader(`{}`))
			r.Header.Set("X-Session-Token", a.token)
			switch bad {
			case "host":
				r.Host = "unrelated.invalid"
			case "origin":
				r.Header.Set("Origin", "https://unrelated.invalid")
			case "fetch-site":
				r.Header.Set("Sec-Fetch-Site", "cross-site")
			}
			response := httptest.NewRecorder()
			a.ServeHTTP(response, r)
			if response.Code != http.StatusForbidden {
				t.Fatalf("%s lookup bypassed guard: %d", bad, response.Code)
			}
		}
	}
	for _, test := range []struct{ method, path string }{{"POST", "/api/public-ip"}, {"GET", "/api/public-ip/refresh"}, {"DELETE", "/api/public-ip/refresh"}} {
		if r := requestApp(a, test.method, test.path, `{}`, true); r.Code != http.StatusMethodNotAllowed {
			t.Fatalf("wrong method %s %s: %d", test.method, test.path, r.Code)
		}
	}
	if r := requestApp(a, "GET", "/api/public-ip/unknown", "", true); r.Code != http.StatusNotFound {
		t.Fatal("unknown public lookup route was accepted")
	}
	a.windowMode = "terminal"
	for _, test := range []struct{ method, path string }{{"GET", "/api/public-ip"}, {"POST", "/api/public-ip/refresh"}} {
		if r := requestApp(a, test.method, test.path, `{}`, true); r.Code != http.StatusNotFound {
			t.Fatalf("independent terminal reached public lookup: %d", r.Code)
		}
	}
	a.windowMode = "main"
	_, launch := workspaceTestCreate(t, a, terminalWorkspaceSnapshot{})
	for _, test := range []struct{ method, path string }{{"GET", "/api/public-ip"}, {"POST", "/api/public-ip/refresh"}} {
		if r := viewTestRequest(a, launch.ViewToken, test.method, test.path, nil, nil); r.Code != http.StatusForbidden {
			t.Fatalf("scoped linked terminal reached public lookup: %d", r.Code)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("denied request performed external lookup: %d calls", calls.Load())
	}
}

func TestPublicIPAPIResponsesAndNoSystemCoupling(t *testing.T) {
	a := testApplication(t)
	var calls atomic.Int32
	var fail atomic.Bool
	a.publicIP.lookup = func(_ context.Context, family int) publicIPFamilyResult {
		calls.Add(1)
		if family == 6 || fail.Load() {
			return publicIPFamilyResult{Status: "unavailable", Error: "fixture service unavailable"}
		}
		return publicIPFamilyResult{Status: "ok", Address: "8.8.8.8"}
	}
	a.system, a.systemAt = map[string]any{"hostname": "fixture", "interfaces": []any{}}, time.Now()
	for _, path := range []string{"/api/status", "/api/system", "/api/state", "/api/report"} {
		if r := requestApp(a, "GET", path, "", true); r.Code != http.StatusOK {
			t.Fatalf("existing %s response: %d", path, r.Code)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("startup/system/state/report initiated a public lookup")
	}
	first := requestApp(a, "GET", "/api/public-ip", "", true)
	var result publicIPSnapshot
	if first.Code != http.StatusOK || json.Unmarshal(first.Body.Bytes(), &result) != nil || result.Provider != "ipify" || result.IPv4.Address != "8.8.8.8" || result.IPv6.Status != "unavailable" || result.IPv6.Address != "" || result.CheckedAt == "" || result.ExpiresAt == "" || calls.Load() != 2 {
		t.Fatalf("lazy partial result: %d %s", first.Code, first.Body.String())
	}
	if first.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("local HTTP cache may retain public addresses")
	}
	if cached := requestApp(a, "GET", "/api/public-ip", "", true); cached.Code != http.StatusOK || calls.Load() != 2 {
		t.Fatal("GET did not reuse application cache")
	}
	fail.Store(true)
	refreshed := requestApp(a, "POST", "/api/public-ip/refresh", `{}`, true)
	if refreshed.Code != http.StatusOK || json.Unmarshal(refreshed.Body.Bytes(), &result) != nil || result.IPv4.Status != "unavailable" || strings.Contains(refreshed.Body.String(), "8.8.8.8") || calls.Load() != 4 {
		t.Fatalf("failed forced refresh should be a readable family result: %d %s", refreshed.Code, refreshed.Body.String())
	}
	for _, path := range []string{"/api/system", "/api/state", "/api/report"} {
		response := requestApp(a, "GET", path, "", true)
		if strings.Contains(response.Body.String(), "8.8.8.8") || strings.Contains(response.Body.String(), "ipify") {
			t.Fatalf("public address persisted into %s", path)
		}
	}
}
