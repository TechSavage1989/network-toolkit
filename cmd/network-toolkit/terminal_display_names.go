package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"networktoolkit/internal/terminal"
)

type terminalDisplayNameRecord struct {
	ID           string `json:"id"`
	NameRevision uint64 `json:"nameRevision"`
	Name         string `json:"name"`
	NameExplicit *bool  `json:"nameExplicit"`
	Type         string `json:"type"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	SerialPort   string `json:"serialPort"`
}

var terminalDisplayCOMPort130 = regexp.MustCompile(`(?i)^COM[0-9]+$`)

// Identity/display metadata only. This neither resolves nor opens a device.
func terminalDisplaySerialPort(value string) string {
	port := strings.TrimSpace(value)
	if port == "" || len(port) > 256 {
		return ""
	}
	for _, r := range port {
		if r < 0x21 || r > 0x7e {
			return ""
		}
	}
	if terminalDisplayCOMPort130.MatchString(port) {
		return strings.ToUpper(port)
	}
	return port
}

func terminalDisplayPort(kind string, port int) int {
	if port != 0 {
		return port
	}
	if kind == "telnet" {
		return 23
	}
	return 22
}

func (p terminalDisplayNameRecord) matches(s terminal.SessionInfo) bool {
	kind := p.Type
	if kind == "" {
		kind = "ssh"
	}
	if kind == "serial" {
		port := terminalDisplaySerialPort(p.SerialPort)
		return s.Type == "serial" && port != "" && port == terminalDisplaySerialPort(s.SerialPort)
	}
	// Local sessions do not expose a complete working-directory/elevation identity
	// here; never merge unrelated local names by an empty network endpoint.
	if kind != "ssh" && kind != "telnet" {
		return false
	}
	return kind == s.Type && p.Host == s.Host && p.Username == s.Username && terminalDisplayPort(kind, p.Port) == terminalDisplayPort(s.Type, s.Port)
}

func (p terminalDisplayNameRecord) manual() string {
	name := strings.TrimSpace(p.Name)
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 100 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return ""
	}
	if p.NameExplicit != nil {
		if *p.NameExplicit {
			return name
		}
		return ""
	}
	kind := p.Type
	if kind == "" {
		kind = "ssh"
	}
	if kind == "serial" {
		port := terminalDisplaySerialPort(p.SerialPort)
		for _, generated := range []string{port, "Serial " + port, "serial", "Terminal"} {
			if strings.EqualFold(name, generated) {
				return ""
			}
		}
		return name
	}
	if kind == "cmd" && (strings.EqualFold(name, "Command Prompt (local)") || strings.EqualFold(name, "Command Prompt")) {
		return ""
	}
	if kind == "powershell" && (strings.EqualFold(name, "PowerShell (local)") || strings.EqualFold(name, "PowerShell")) {
		return ""
	}
	address := net.JoinHostPort(p.Host, strconv.Itoa(terminalDisplayPort(kind, p.Port)))
	for _, generated := range []string{p.Host, address, p.Host + ":" + strconv.Itoa(terminalDisplayPort(kind, p.Port)), kind, kind + " " + address, "Terminal"} {
		if strings.EqualFold(name, generated) {
			return ""
		}
	}
	return name
}

// Project only the authorized session's display name. This does not rename a
// transport, mutate settings, carry credentials, or disclose the profile list.
// It keeps already detached windows current after a saved-session rename.
func (a *application) terminalDisplaySession(s terminal.SessionInfo) terminal.SessionInfo {
	if s.Type != "ssh" && s.Type != "telnet" && s.Type != "serial" && s.Type != "cmd" && s.Type != "powershell" {
		return s
	}
	a.stateMu.Lock()
	data := append([]byte(nil), a.state...)
	a.stateMu.Unlock()
	var state struct {
		Profiles []terminalDisplayNameRecord `json:"terminalProfiles"`
		Recent   []terminalDisplayNameRecord `json:"terminalRecent"`
	}
	if json.Unmarshal(data, &state) != nil {
		return s
	}
	apply := func(name, source string, revision uint64) terminal.SessionInfo {
		s.ManualName = name
		s.NameRevision = revision
		s.NameExplicit = name != ""
		s.LabelSource = source
		if name == "" {
			s.LabelSource = "default"
		}
		if name != "" {
			s.Label = name
		} else {
			s.Label = strings.ToUpper(s.Type) + " " + net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
			switch s.Type {
			case "serial":
				s.Label = "Serial " + s.SerialPort
			case "cmd":
				s.Label = "Command Prompt (local)"
			case "powershell":
				s.Label = "PowerShell (local)"
			}
		}
		return s
	}
	if s.ProfileID != "" {
		matches := []terminalDisplayNameRecord{}
		for _, p := range state.Profiles {
			if p.ID == s.ProfileID {
				matches = append(matches, p)
			}
		}
		if len(matches) == 1 {
			if matches[0].NameRevision >= s.NameRevision {
				return apply(matches[0].manual(), "profile", matches[0].NameRevision)
			}
			return s
		}
		if s.LabelSource != "manual" {
			return apply("", "default", s.NameRevision)
		}
		// A deleted or ambiguous nonempty saved identity must never fall back to
		// another profile/recent at the same endpoint. Keep this explicit tab name.
		return s
	}
	var selected *terminalDisplayNameRecord
	for _, collection := range [][]terminalDisplayNameRecord{state.Profiles, state.Recent} {
		matches := []terminalDisplayNameRecord{}
		for _, p := range collection {
			if p.matches(s) {
				matches = append(matches, p)
			}
		}
		if len(matches) == 1 && matches[0].NameRevision >= s.NameRevision {
			candidate := matches[0]
			if selected == nil || candidate.NameRevision > selected.NameRevision {
				selected = &candidate
			}
		}
	}
	if selected != nil {
		return apply(selected.manual(), "saved", selected.NameRevision)
	}

	return s
}

func (a *application) terminalDisplayBatch(batch terminal.Batch) terminal.Batch {
	for i := range batch.Sessions {
		batch.Sessions[i] = a.terminalDisplaySession(batch.Sessions[i])
	}
	for i := range batch.Events {
		if batch.Events[i].Session != nil {
			projected := a.terminalDisplaySession(*batch.Events[i].Session)
			batch.Events[i].Session = &projected
		}
	}
	return batch
}

const maxTerminalNameRevision = uint64(9007199254740991)

func terminalNameRevision(value any) (uint64, bool) {
	switch n := value.(type) {
	case float64:
		if n >= 0 && n <= float64(maxTerminalNameRevision) && n == float64(uint64(n)) {
			return uint64(n), true
		}
	case uint64:
		return n, n <= maxTerminalNameRevision
	case int:
		if n >= 0 {
			return uint64(n), uint64(n) <= maxTerminalNameRevision
		}
	}
	return 0, false
}

func terminalNameRows(state map[string]any, key string) []map[string]any {
	if rows, ok := state[key].([]map[string]any); ok {
		return rows
	}
	out := []map[string]any{}
	if rows, ok := state[key].([]any); ok {
		for _, row := range rows {
			if m, ok := row.(map[string]any); ok {
				out = append(out, m)
			}
		}
	}
	return out
}

func terminalNameRecord(row map[string]any) terminalDisplayNameRecord {
	var record terminalDisplayNameRecord
	b, _ := json.Marshal(row)
	_ = json.Unmarshal(b, &record)
	return record
}

func terminalCopyName(to, from map[string]any) {
	for _, key := range []string{"name", "nameExplicit", "nameRevision"} {
		if v, ok := from[key]; ok {
			to[key] = v
		} else {
			delete(to, key)
		}
	}
}

func terminalSameName(a, b map[string]any) bool {
	return a["name"] == b["name"] && a["nameExplicit"] == b["nameExplicit"]
}

func terminalNextNameRevision(state map[string]any) (uint64, error) {
	revision, _ := terminalNameRevision(state["terminalNameRevision"])
	for _, key := range []string{"terminalProfiles", "terminalRecent"} {
		for _, row := range terminalNameRows(state, key) {
			n, _ := terminalNameRevision(row["nameRevision"])
			if n > revision {
				revision = n
			}
		}
	}
	if revision >= maxTerminalNameRevision {
		return 0, errors.New("Display-name revision limit reached.")
	}
	return revision + 1, nil
}

// Called with stateMu held. Only name/provenance/revision fields are reconciled;
// unrelated connection/authentication settings retain their ordinary semantics.
func reconcileTerminalDisplayNames(incoming, current map[string]any) error {
	next, err := terminalNextNameRevision(current)
	if err != nil {
		return err
	}
	changes := []map[string]any{}
	for _, key := range []string{"terminalProfiles", "terminalRecent"} {
		old := map[string]map[string]any{}
		for _, row := range terminalNameRows(current, key) {
			if id, ok := row["id"].(string); ok {
				old[id] = row
			}
		}
		for _, row := range terminalNameRows(incoming, key) {
			id, _ := row["id"].(string)
			before := old[id]
			got, _ := terminalNameRevision(row["nameRevision"])
			want, _ := terminalNameRevision(before["nameRevision"])
			if before == nil { // New entries cannot replay a deleted record revision.
				if got != 0 {
					return errors.New("A new display-name record must start at revision zero. Reload settings before saving.")
				}
				row["nameRevision"] = next
				next++
				if key == "terminalProfiles" {
					changes = append(changes, row)
				}
				continue
			}
			if got > want {
				return errors.New("Display-name revision is newer than the stored record. Reload settings before saving.")
			}
			if got < want {
				terminalCopyName(row, before)
				continue
			}
			if !terminalSameName(row, before) {
				row["nameRevision"] = next
				next++
				if key == "terminalProfiles" {
					changes = append(changes, row)
				}
			} else {
				row["nameRevision"] = want
			}
		}
	}
	for _, changed := range changes {
		p := terminalNameRecord(changed)
		s := terminal.SessionInfo{Type: p.Type, Host: p.Host, Port: p.Port, Username: p.Username, SerialPort: p.SerialPort}
		count := 0
		for _, row := range terminalNameRows(incoming, "terminalProfiles") {
			if terminalNameRecord(row).matches(s) {
				count++
			}
		}
		if count != 1 {
			continue
		} // Same endpoint under distinct saved IDs is ambiguous.
		for _, row := range terminalNameRows(incoming, "terminalRecent") {
			if terminalNameRecord(row).matches(s) {
				terminalCopyName(row, changed)
			}
		}
	}
	if next > maxTerminalNameRevision {
		return errors.New("Display-name revision limit reached.")
	}
	incoming["terminalNameRevision"] = next - 1
	return nil
}

// Invoked under Manager's live-session lock and the existing owner guard. The
// disk write occurs before the manager publishes the new name; failures change
// neither live display metadata nor the in-memory settings snapshot.
func (a *application) persistTerminalDisplayName(s terminal.SessionInfo) (uint64, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	state := map[string]any{}
	if len(a.state) > 0 {
		if err := json.Unmarshal(a.state, &state); err != nil {
			return 0, errors.New("Stored settings could not be read; the terminal name was not changed.")
		}
	}
	revision, err := terminalNextNameRevision(state)
	if err != nil {
		return 0, err
	}
	apply := func(row map[string]any) {
		if s.NameExplicit {
			row["name"] = s.ManualName
		}
		row["nameExplicit"], row["nameRevision"] = s.NameExplicit, revision
	}
	profiles := terminalNameRows(state, "terminalProfiles")
	matchingID, matchingEndpoint := 0, 0
	for _, row := range profiles {
		p := terminalNameRecord(row)
		if p.ID == s.ProfileID && s.ProfileID != "" {
			matchingID++
		}
		if p.matches(s) {
			matchingEndpoint++
		}
	}
	if matchingID > 1 {
		return 0, errors.New("Saved profile identity is ambiguous; the terminal name was not changed.")
	}
	for _, row := range profiles {
		p := terminalNameRecord(row)
		if (s.ProfileID != "" && p.ID == s.ProfileID) || (s.ProfileID == "" && matchingEndpoint == 1 && p.matches(s)) {
			apply(row)
		}
	}
	if matchingEndpoint <= 1 {
		for _, row := range terminalNameRows(state, "terminalRecent") {
			if terminalNameRecord(row).matches(s) {
				apply(row)
			}
		}
	}
	state["terminalNameRevision"] = revision
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil || len(b) > 2<<20 {
		return 0, errors.New("Terminal name would exceed the saved settings limit.")
	}
	release, err := acquireCredentialGuard(filepath.Dir(a.dataPath))
	if err != nil {
		return 0, err
	}
	defer release()
	if err = writeAtomic(a.dataPath, b); err != nil {
		return 0, fmt.Errorf("Terminal name was not saved: %w", err)
	}
	a.state = b
	return revision, nil
}

func terminalDisplayNamesResponse(state map[string]any) map[string]any {
	out := map[string]any{}
	for _, pair := range [][2]string{{"terminalProfiles", "profiles"}, {"terminalRecent", "recent"}} {
		rows := []map[string]any{}
		for _, row := range terminalNameRows(state, pair[0]) {
			revision, _ := terminalNameRevision(row["nameRevision"])
			p := terminalNameRecord(row)
			rows = append(rows, map[string]any{"id": p.ID, "name": p.Name, "nameExplicit": p.manual() != "", "nameRevision": revision})
		}
		out[pair[1]] = rows
	}
	return out
}

// Validate before schema sanitation so an invalid revision cannot silently
// become legacy revision zero and authorize a conflicting name change.
func validateTerminalDisplayRevisions(state map[string]any) error {
	for _, key := range []string{"terminalProfiles", "terminalRecent"} {
		for _, row := range terminalNameRows(state, key) {
			if v, provided := row["nameRevision"]; provided {
				if _, ok := terminalNameRevision(v); !ok {
					return errors.New("Display-name revision must be a nonnegative safe integer.")
				}
			}
		}
	}
	return nil
}
