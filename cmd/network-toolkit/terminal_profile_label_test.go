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
