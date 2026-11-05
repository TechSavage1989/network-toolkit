package main

import (
	"net/http"
	"strings"
	"testing"
)

// Owner report (v1.20.9): test tab order and the Overview layout reset on restart.
// The server must keep both preferences exactly as saved.
func TestStateKeepsTestTabOrderAndOverviewLayout(t *testing.T) {
	a := testApplication(t)
	body := `{"testTabOrder":{"target":["ping","monitor","dns"],"web":["tls","http"]},"overviewLayout":{"order":["activity","stats"],"hidden":["publicIP"]}}`
	if r := requestApp(a, http.MethodPut, "/api/state", body, true); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	r := requestApp(a, http.MethodGet, "/api/state", "", true)
	got := r.Body.String()
	for _, want := range []string{`"testTabOrder":{"target":["ping","monitor","dns"],"web":["tls","http"]}`, `"overviewLayout":{"hidden":["publicIP"],"order":["activity","stats"]}`} {
		if !strings.Contains(strings.NewReplacer(" ", "", "\n", "").Replace(got), want) {
			t.Fatalf("state lost %s: %s", want, got)
		}
	}
}

// v1.20.10 is the first two-digit patch number: it must compare as newer than 1.20.9.
func TestUpdateNewerHandlesTwoDigitPatch(t *testing.T) {
	if !updateNewer("1.20.10", "1.20.9") || updateNewer("1.20.9", "1.20.10") || !updateNewer("1.20.10", "1.20.2") || !updateNewer("1.20.11", "1.20.10") || !updateNewer("1.20.12", "1.20.11") || !updateNewer("1.20.13", "1.20.12") || !updateNewer("1.20.14", "1.20.13") || !updateNewer("1.20.15", "1.20.14") || !updateNewer("1.20.16", "1.20.15") {
		t.Fatal("two-digit patch versions compared as text")
	}
}
