package main

import (
	"net/http"
	"strings"
	"testing"
)

// Saturated job slots make this an entirely local validation test: a valid
// shape reaches the concurrency guard, and a malformed shape must fail first.
func TestSourceOptionsRejectedBeforeJobAllocation(t *testing.T) {
	a := testApplication(t)
	for i := 0; i < maxConcurrent; i++ {
		id := strings.Repeat(string(rune('a'+i)), 24)
		a.jobs[id] = &Job{ID: id, Status: "running", cancel: func() {}}
	}
	for _, body := range []string{
		`{"tool":"ping","target":"192.0.2.1","options":{"sourceInterfaceIndex":"7","sourceAddress":"192.0.2.20"}}`,
		`{"tool":"trace","target":"192.0.2.1","options":{"sourceInterfaceIndex":7.5,"sourceAddress":"192.0.2.20"}}`,
		`{"tool":"tcp","target":"192.0.2.1","options":{"sourceInterfaceIndex":7}}`,
		`{"tool":"http","target":"https://example.invalid","options":{"sourceInterfaceIndex":7,"sourceAddress":"adapter-name"}}`,
		`{"tool":"dns","target":"example.invalid","options":{"sourceInterfaceIndex":7,"sourceAddress":"192.0.2.20","sourceInterfaceName":["Ethernet"]}}`,
		`{"tool":"routes","options":{"sourceInterfaceIndex":7,"sourceAddress":"192.0.2.20"}}`,
	} {
		response := requestApp(a, http.MethodPost, "/api/jobs", body, true)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid source shape reached allocation: %d %s", response.Code, response.Body.String())
		}
		if len(a.jobs) != maxConcurrent {
			t.Fatal("invalid source request allocated a job")
		}
	}
	for _, options := range []string{`{}`, `{"sourceInterfaceIndex":7,"sourceInterfaceName":"Ethernet 5","sourceAddress":"192.0.2.20"}`} {
		response := requestApp(a, http.MethodPost, "/api/jobs", `{"tool":"ping","target":"192.0.2.1","options":`+options+`}`, true)
		if response.Code != http.StatusTooManyRequests {
			t.Fatalf("valid source shape failed before concurrency guard: %d %s", response.Code, response.Body.String())
		}
	}
}
