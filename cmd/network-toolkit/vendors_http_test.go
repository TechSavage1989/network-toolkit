package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"networktoolkit/internal/identity"
)

func TestVendorAPIGuardsAndStatus(t *testing.T) {
	a := testApplication(t)
	for _, path := range []string{"status", "update", "import", "reset"} {
		if response := requestApp(a, http.MethodPost, "/api/vendors/"+path, `{}`, false); response.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated %s: %d", path, response.Code)
		}
	}
	for _, path := range []string{"update", "import", "reset"} {
		if response := requestApp(a, http.MethodGet, "/api/vendors/"+path, "", true); response.Code != http.StatusMethodNotAllowed {
			t.Fatalf("GET mutated %s: %d", path, response.Code)
		}
		r := httptest.NewRequest(http.MethodPost, "http://"+a.host+"/api/vendors/"+path, strings.NewReader(`{}`))
		r.Header.Set("X-Session-Token", a.token)
		r.Header.Set("Origin", "https://unrelated.invalid")
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatalf("cross-origin %s: %d", path, w.Code)
		}
	}
	r := requestApp(a, http.MethodGet, "/api/vendors/status", "", true)
	var status identity.Status
	if r.Code != http.StatusOK || json.Unmarshal(r.Body.Bytes(), &status) != nil || status.Entries < 50000 || !status.UpdateAvailable {
		t.Fatalf("vendor status: %d %s", r.Code, r.Body.String())
	}
	if r := requestApp(a, http.MethodPost, "/api/vendors/status", "", true); r.Code != http.StatusMethodNotAllowed {
		t.Fatal("status allowed a mutation method")
	}
	if r := requestApp(a, http.MethodPost, "/api/vendors/unknown", "", true); r.Code != http.StatusNotFound {
		t.Fatal("unknown vendor route was accepted")
	}
}

func TestVendorImportRejectsMalformedEnvelope(t *testing.T) {
	a := testApplication(t)
	before := identity.DatabaseStatus()
	for _, body := range []string{`{} {}`, `{"csv":42}`, `[]`, `{"unexpected":true}`} {
		r := requestApp(a, http.MethodPost, "/api/vendors/import", body, true)
		if r.Code != http.StatusBadRequest {
			t.Fatalf("invalid import envelope accepted: %d %s", r.Code, r.Body.String())
		}
	}
	after := identity.DatabaseStatus()
	if before.Entries != after.Entries || before.Source != after.Source {
		t.Fatal("malformed import changed the database")
	}
}
