package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"networktoolkit/internal/terminal"
)

func TestSerialNames130NormalizedCacheIsBoundedAndDisplayOnly(t *testing.T) {
	input := []any{
		map[string]any{"type": "serial", "serialPort": " com3 ", "name": "7300", "host": "must-not-retain", "port": float64(22), "username": "not-retained", "password": "never-retain"},
		map[string]any{"type": "serial", "serialPort": "COM3", "name": "DUPLICATE"},
		map[string]any{"type": "serial", "serialPort": "COM4", "name": "OTHER"},
		map[string]any{"type": "serial", "serialPort": "/dev/ttyUSB0", "name": "UNIX"},
		map[string]any{"type": "serial", "serialPort": "/dev/ttyusb0", "name": "CASE-DIFFERENT"},
		map[string]any{"type": "serial", "serialPort": "COM\n5", "name": "BAD"},
		map[string]any{"type": "serial", "serialPort": "COM 5", "name": "BAD"},
		map[string]any{"type": "serial", "serialPort": "COMé", "name": "BAD"},
		map[string]any{"type": "serial", "serialPort": strings.Repeat("a", 257), "name": "BAD"},
		map[string]any{"type": "serial", "serialPort": "COM6", "name": "password"},
		map[string]any{"type": "cmd", "serialPort": "COM7", "name": "NOT-SERIAL"},
		map[string]any{"type": "ssh", "host": "192.0.2.130", "port": float64(22), "username": "operator", "name": "NETWORK"},
	}
	got := safeTerminalPromptNames(input)
	if len(got) != 5 || got[0]["serialPort"] != "COM3" || len(got[0]) != 3 || got[1]["serialPort"] != "COM4" {
		t.Fatalf("invalid serial cache: %+v", got)
	}
	if got[2]["serialPort"] == got[3]["serialPort"] {
		t.Fatal("Unix paths folded case")
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "never-retain") || strings.Contains(string(raw), "must-not-retain") || strings.Contains(string(raw), "not-retained") {
		t.Fatal("connection/secrets retained in serial learned metadata")
	}
	first := terminalWorkspaceDisplay(map[string]any{"terminalPromptNames": input})
	second := terminalWorkspaceDisplay(first)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("workspace learned serial metadata not idempotent: %+v -> %+v", first, second)
	}
	many := []any{}
	for i := 0; i < 220; i++ {
		many = append(many, map[string]any{"type": "serial", "serialPort": fmt.Sprintf("COM%d", i+1), "name": "SWITCH"})
	}
	if len(safeTerminalPromptNames(many)) != 200 {
		t.Fatal("serial cache bound lost")
	}
}

func TestSerialNames130IdentityAndManualInference(t *testing.T) {
	p := terminalDisplayNameRecord{Type: "serial", SerialPort: "com3", Name: "Serial COM3"}
	if !p.matches(terminal.SessionInfo{Type: "serial", SerialPort: "COM3"}) || p.matches(terminal.SessionInfo{Type: "serial", SerialPort: "COM4"}) || p.matches(terminal.SessionInfo{Type: "ssh", Host: "COM3", Port: 22}) {
		t.Fatal("serial identity collided or lost normalization")
	}
	for _, generated := range []string{"COM3", "com3", "Serial COM3", "serial com3", "serial", "Terminal"} {
		p.Name = generated
		if p.manual() != "" {
			t.Fatalf("generated label inferred manual: %q", generated)
		}
	}
	p.Name = "Lab console"
	if p.manual() != p.Name {
		t.Fatal("legacy genuine custom name lost")
	}
	yes, no := true, false
	p.Name = "COM3"
	p.NameExplicit = &yes
	if p.manual() != "COM3" {
		t.Fatal("explicit generated-looking manual name lost")
	}
	p.NameExplicit = &no
	if p.manual() != "" {
		t.Fatal("clear flag ignored")
	}
	p.SerialPort = ""
	if p.matches(terminal.SessionInfo{Type: "serial"}) {
		t.Fatal("empty serial identity matched")
	}
	p = terminalDisplayNameRecord{Type: "serial", SerialPort: "/dev/ttyUSB0"}
	if p.matches(terminal.SessionInfo{Type: "serial", SerialPort: "/dev/ttyusb0"}) {
		t.Fatal("Unix serial paths folded case")
	}
	for _, kind := range []string{"cmd", "powershell"} {
		if (terminalDisplayNameRecord{Type: kind}).matches(terminal.SessionInfo{Type: kind}) {
			t.Fatal("local sessions matched by incomplete empty network endpoint")
		}
	}
}

const serialState130 = `{"terminalNameRevision":5,"terminalProfiles":[{"id":"p3","name":"COM3","nameExplicit":false,"nameRevision":5,"type":"serial","serialPort":"COM3","baudRate":115200,"dataBits":8,"parity":"none","stopBits":"1","flowControl":"none","credentialId":"unchanged-reference"},{"id":"p4","name":"Other manual","nameExplicit":true,"nameRevision":4,"type":"serial","serialPort":"COM4","baudRate":9600}],"terminalRecent":[{"id":"r3","name":"COM3","nameExplicit":false,"nameRevision":5,"type":"serial","serialPort":"com3","baudRate":115200},{"id":"r4","name":"Other manual","nameExplicit":true,"nameRevision":4,"type":"serial","serialPort":"COM4","baudRate":9600}]}`

func TestSerialNames130RenameClearProjectionAndRevisionArePortScoped(t *testing.T) {
	a := &application{state: []byte(serialState130), dataPath: filepath.Join(t.TempDir(), "state.json")}
	s := terminal.SessionInfo{ID: "serial-one", Type: "serial", SerialPort: "COM3", Label: "Serial COM3", NameExplicit: false, LabelSource: "default"}
	initial := a.terminalDisplaySession(s)
	if initial.NameExplicit || initial.ManualName != "" || initial.Label != "Serial COM3" || initial.NameRevision != 5 {
		t.Fatalf("generated serial fallback not projected: %+v", initial)
	}
	s.ManualName = "Console custom"
	s.NameExplicit = true
	s.LabelSource = "manual"
	revision, err := a.persistTerminalDisplayName(s)
	if err != nil || revision != 6 {
		t.Fatalf("rename %d %v", revision, err)
	}
	state := displayState128(t, string(a.state))
	for _, key := range []string{"terminalProfiles", "terminalRecent"} {
		rows := terminalNameRows(state, key)
		if rows[0]["name"] != "Console custom" || rows[0]["nameRevision"] != float64(6) || rows[1]["name"] != "Other manual" || rows[1]["nameRevision"] != float64(4) {
			t.Fatalf("serial ports cross-renamed: %+v", rows)
		}
	}
	p := terminalNameRows(state, "terminalProfiles")[0]
	if p["serialPort"] != "COM3" || p["baudRate"] != float64(115200) || p["credentialId"] != "unchanged-reference" || p["parity"] != "none" {
		t.Fatal("display rename altered serial settings")
	}
	projected := a.terminalDisplaySession(s)
	if projected.ManualName != "Console custom" || projected.Label != "Console custom" || projected.NameRevision != 6 || projected.SerialPort != "COM3" || projected.Host != "" || projected.Port != 0 {
		t.Fatalf("serial projection fabricated network identity: %+v", projected)
	}
	s.NameExplicit = false
	s.ManualName = ""
	s.LabelSource = "default"
	revision, err = a.persistTerminalDisplayName(s)
	if err != nil || revision != 7 {
		t.Fatalf("clear %d %v", revision, err)
	}
	cleared := a.terminalDisplaySession(s)
	raw, _ := json.Marshal(cleared)
	if cleared.NameExplicit || cleared.ManualName != "" || cleared.Label != "Serial COM3" || cleared.NameRevision != 7 || !strings.Contains(string(raw), `"nameExplicit":false`) {
		t.Fatalf("serial clear lost: %s", raw)
	}
	state = displayState128(t, string(a.state))
	if terminalNameRows(state, "terminalProfiles")[0]["name"] != "Console custom" {
		t.Fatal("clear discarded identifying saved text")
	}
	// Stale profile fields cannot restore the manual override after clear.
	stale := displayState128(t, serialState130)
	if err := reconcileTerminalDisplayNames(stale, state); err != nil {
		t.Fatal(err)
	}
	if terminalNameRows(stale, "terminalProfiles")[0]["nameExplicit"] != false || terminalNameRows(stale, "terminalProfiles")[0]["nameRevision"] != float64(7) {
		t.Fatal("stale serial settings restored override")
	}
}

func TestSerialNames130SavedEditSynchronizesOnlyMatchingPortRecent(t *testing.T) {
	current := displayState128(t, serialState130)
	incoming := displayState128(t, serialState130)
	row := terminalNameRows(incoming, "terminalProfiles")[0]
	row["name"] = "New saved console"
	row["nameExplicit"] = true
	if err := reconcileTerminalDisplayNames(incoming, current); err != nil {
		t.Fatal(err)
	}
	recents := terminalNameRows(incoming, "terminalRecent")
	if recents[0]["name"] != "New saved console" || recents[0]["nameRevision"] != row["nameRevision"] || recents[1]["name"] != "Other manual" {
		t.Fatalf("serial saved/recent edit identity wrong: %+v", recents)
	}
}

func TestSerialNames130LocalProjectionRequiresSavedIDAndReadableDefaults(t *testing.T) {
	for _, kind := range []string{"cmd", "powershell"} {
		raw := fmt.Sprintf(`{"terminalProfiles":[{"id":"local-profile","type":%q,"name":"Original custom","nameExplicit":false,"nameRevision":3}]}`, kind)
		a := &application{state: []byte(raw)}
		s := terminal.SessionInfo{ID: "local-session", ProfileID: "local-profile", Type: kind, Label: "Original custom", ManualName: "Original custom", NameExplicit: true, LabelSource: "profile"}
		got := a.terminalDisplaySession(s)
		want := "Command Prompt (local)"
		if kind == "powershell" {
			want = "PowerShell (local)"
		}
		if got.NameExplicit || got.ManualName != "" || got.Label != want || got.NameRevision != 3 {
			t.Fatalf("local clear fallback wrong: %+v", got)
		}
		s.ProfileID = ""
		got = a.terminalDisplaySession(s)
		if got.ManualName != "Original custom" || got.NameRevision != 0 {
			t.Fatal("unbound local session inferred unrelated profile")
		}
		if len(safeTerminalPromptNames([]any{map[string]any{"type": kind, "name": "NODE", "host": "192.0.2.130", "port": float64(22), "username": "operator"}})) != 0 {
			t.Fatal("local session created learned registry record")
		}
	}
}
