package main

import (
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Saved names are display metadata only. Resolve them from the current saved
// collection, never from a remote OSC title or a client-supplied display label.
func (a *application) terminalProfileLabel(id string) string {
	if id == "" {
		return ""
	}
	a.stateMu.Lock()
	data := append([]byte(nil), a.state...)
	a.stateMu.Unlock()
	var state struct {
		Profiles []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"terminalProfiles"`
	}
	if json.Unmarshal(data, &state) != nil {
		return ""
	}
	found, name := false, ""
	for _, profile := range state.Profiles {
		if profile.ID != id {
			continue
		}
		if found {
			return ""
		}
		found, name = true, strings.TrimSpace(profile.Name)
	}
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) > 100 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return ""
	}
	return name
}
