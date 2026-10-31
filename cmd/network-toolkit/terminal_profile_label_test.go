package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestSavedTerminalLabelUsesCurrentNonsecretName(t *testing.T) {
	for _, tt := range []struct{ name, state, id, want string }{
		{"named", `{"terminalProfiles":[{"id":"a","name":"Core <west> & lab","password":"ignored"}]}`, "a", "Core <west> & lab"},
		{"trim", `{"terminalProfiles":[{"id":"a","name":"  Core  "}]}`, "a", "Core"},
		{"missing", `{"terminalProfiles":[{"id":"a","name":"Core"}]}`, "b", ""},
		{"empty", `{"terminalProfiles":[{"id":"a","name":"  "}]}`, "a", ""},
		{"ambiguous", `{"terminalProfiles":[{"id":"a","name":"Core"},{"id":"a","name":"Other"}]}`, "a", ""},
		{"control", `{"terminalProfiles":[{"id":"a","name":"Core\nOther"}]}`, "a", ""},
		{"bad state", `not JSON`, "a", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := &application{state: []byte(tt.state)}
			if got := a.terminalProfileLabel(tt.id); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestSavedTerminalProfileWindowMetadataRoundTrip(t *testing.T) {
	for _, kind := range []string{"ssh", "telnet", "cmd", "powershell"} {
		p := terminalWindowPreset{Type: kind, ProfileID: "saved-profile"}
		if err := p.normalize(); err != nil {
			t.Fatal(err)
		}
		if p.ProfileID != "saved-profile" {
			t.Fatalf("%s dropped saved label identity", kind)
		}
	}
	for _, id := range []string{"bad\nvalue", strings.Repeat("a", 81)} {
		p := terminalWindowPreset{ProfileID: id}
		if p.normalize() == nil {
			t.Fatal("invalid profile accepted")
		}
	}
}
func TestTerminalWorkspaceOrderIsBoundedNonsecretMetadata(t *testing.T) {
	got := terminalWorkspaceDisplay(map[string]any{"terminalTabOrder": []any{"b", "a", "b", "bad\n", 1, ""}, "password": "never"})
	if !reflect.DeepEqual(got["terminalTabOrder"], []string{"b", "a"}) {
		t.Fatalf("order: %#v", got)
	}
	if _, ok := got["password"]; ok {
		t.Fatal("unknown metadata retained")
	}
	ids := make([]string, 140)
	for i := range ids {
		ids[i] = fmt.Sprint(i)
	}
	if len(terminalWorkspaceDisplay(map[string]any{"terminalTabOrder": ids})["terminalTabOrder"].([]string)) != 128 {
		t.Fatal("unbounded order")
	}
}

func TestTerminalExplicitNameMetadata128(t *testing.T) {
	for _, explicit := range []any{true, false, "true", map[string]any{"password": "never"}} {
		profile := map[string]any{"id": "saved", "name": "192.0.2.23", "type": "ssh", "host": "192.0.2.23", "port": float64(22), "nameExplicit": explicit, "credentialId": "reference-only", "password": "never"}
		clean := safeTerminalProfile(profile, true)
		if value, ok := explicit.(bool); ok {
			if clean["nameExplicit"] != value {
				t.Fatal("boolean provenance lost")
			}
		} else if _, ok := clean["nameExplicit"]; ok {
			t.Fatal("invalid provenance retained")
		}
		if clean["credentialId"] != "reference-only" || clean["host"] != profile["host"] {
			t.Fatal("credential reference or address changed")
		}
		if _, ok := clean["password"]; ok {
			t.Fatal("password retained")
		}
		if _, ok := safeTerminalProfile(profile, false)["nameExplicit"]; ok {
			t.Fatal("name provenance leaked into connection config")
		}
	}
	a := &application{state: []byte(`{"terminalProfiles":[{"id":"saved","name":"192.0.2.23","nameExplicit":true}]}`)}
	if !a.terminalProfileNameExplicit("saved") || a.terminalProfileNameExplicit("missing") {
		t.Fatal("explicit profile lookup mismatch")
	}
	a.state = []byte(`{"terminalProfiles":[{"id":"saved","nameExplicit":true},{"id":"saved","nameExplicit":true}]}`)
	if a.terminalProfileNameExplicit("saved") {
		t.Fatal("ambiguous profile provenance accepted")
	}
}
