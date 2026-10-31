//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type nativeProfile130Badge struct {
	Text                    string
	Visible, Noninteractive bool
}
type nativeProfiles130Snapshot struct {
	BootAt                               int64
	StoredName, ProfileID                string
	Explicit, Pending, IdentityUnchanged bool
	TabBadge, SavedBadge, RecentBadge    nativeProfile130Badge
}

func TestNativeTerminalProfiles130InstalledWebView2(t *testing.T) {
	if os.Getenv("NETWORK_TOOLKIT_NATIVE_PROFILE130") != "1" {
		t.Skip("explicit synthetic native profile130 acceptance only")
	}
	t.Setenv("NETWORK_TOOLKIT_NATIVE_TERMINAL_TEST", "1")
	TestNativeTerminalTransferInstalledWebView2(t)
}
func (h *nativeTerminalFixture) profile130Identity() (map[string]any, error) {
	h.app.stateMu.Lock()
	raw := append([]byte(nil), h.app.state...)
	h.app.stateMu.Unlock()
	var state struct {
		Profiles []map[string]any `json:"terminalProfiles"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	for _, p := range state.Profiles {
		if p["id"] == "native-profile114" {
			identity := map[string]any{}
			for _, k := range []string{"id", "type", "host", "port", "username", "credentialId", "password", "privateKey", "serialPort", "folder"} {
				v := p[k]
				if v == nil {
					v = ""
				}
				identity[k] = v
			}

			return identity, nil
		}
	}
	return nil, errors.New("persisted synthetic profile missing")
}
func (h *nativeTerminalFixture) exerciseProfiles130(accepted *atomic.Int32) (result error) {
	phases := []string{}
	defer func() {
		r := map[string]any{"passed": result == nil, "phases": phases, "connections": accepted.Load(), "scope": "Actual native WebView2 with synthetic loopback Telnet only; no physical serial port, remote device or real credentials"}
		if result != nil {
			r["error"] = result.Error()
		}
		b, _ := json.MarshalIndent(r, "", "  ")
		_ = os.WriteFile(filepath.Join(h.output, "native-profiles130-report.json"), b, 0600)
	}()
	initial, err := h.wait("", func(s nativeTerminalSnapshot) bool {
		return s.Editable && s.GeometryFits && strings.Contains(s.Transcript, "SYNTHETIC 079")
	})
	if err != nil {
		return err
	}
	if _, err = h.command("", "profiles130:baseline"); err != nil {
		return err
	}
	identity, err := h.profile130Identity()
	if err != nil {
		return err
	}
	check := func(label string, explicit bool) (nativeTerminalSnapshot, error) {
		return h.wait("", func(s nativeTerminalSnapshot) bool {
			p := s.Profiles130
			return p != nil && !p.Pending && p.Explicit == explicit && p.IdentityUnchanged && s.TabPrimary == label && s.Editable && s.GeometryFits && p.TabBadge.Text == "Telnet" && p.TabBadge.Visible && p.TabBadge.Noninteractive
		})
	}
	if _, err = h.command("", "profiles130:save-empty"); err != nil {
		return err
	}
	if _, err = check("127.0.0.1", false); err != nil {
		return err
	}
	phases = append(phases, "Real blank Save before prompt keeps automatic IP fallback")
	close(h.promptReady)
	if _, err = check("7300", false); err != nil {
		return err
	}
	if _, err = h.command("", "profiles130:save-empty"); err != nil {
		return err
	}
	s, err := check("7300", false)
	if err != nil {
		return err
	}
	if s.Profiles130.StoredName == "7300" {
		return errors.New("learned prompt frozen into stored manual name")
	}
	phases = append(phases, "Blank Save after actual prompt remains automatic, learned name not frozen")
	if _, err = h.command("", "profiles130:save-manual"); err != nil {
		return err
	}
	if _, err = check("Native manual130", true); err != nil {
		return err
	}
	if _, err = h.command("", "clear-name"); err != nil {
		return err
	}
	if _, err = check("7300", false); err != nil {
		return err
	}
	phases = append(phases, "Manual Save wins; explicit clear restores learned name")
	if _, err = h.command("", "profiles130:saved"); err != nil {
		return err
	}
	s, err = h.wait("", func(s nativeTerminalSnapshot) bool {
		return s.Profiles130 != nil && s.Profiles130.SavedBadge.Visible && s.Profiles130.SavedBadge.Text == "Telnet" && s.SavedPrimary == "7300"
	})
	if err != nil {
		return err
	}
	if err = h.capture("", "profiles130-saved"); err != nil {
		return err
	}
	if _, err = h.command("", "profiles130:recent"); err != nil {
		return err
	}
	if _, err = h.wait("", func(s nativeTerminalSnapshot) bool {
		return s.Profiles130 != nil && s.Profiles130.RecentBadge.Visible && s.Profiles130.RecentBadge.Text == "Telnet"
	}); err != nil {
		return err
	}
	phases = append(phases, "Actual tab, Saved and Recent render noninteractive Telnet badges")
	oldBoot := s.Profiles130.BootAt
	if _, err = h.command("", "profiles130:reload-preferences"); err != nil {
		return err
	}
	if _, err = h.wait("", func(s nativeTerminalSnapshot) bool {
		return s.Profiles130 != nil && s.Profiles130.BootAt != oldBoot && s.Editable && s.TabPrimary == "7300"
	}); err != nil {
		return err
	}
	phases = append(phases, "Actual page reload restores automatic naming from persisted settings")
	if _, err = h.command("", "selection"); err != nil {
		return err
	}
	if _, err = h.wait("", func(s nativeTerminalSnapshot) bool {
		return s.Selection != "" && s.Viewport == 10 && s.ViewportDOM == 10
	}); err != nil {
		return err
	}
	before, err := check("7300", false)
	if err != nil {
		return err
	}
	if _, err = h.command("", "detach"); err != nil {
		return err
	}
	view := ""
	until := time.Now().Add(20 * time.Second)
	for time.Now().Before(until) {
		for _, v := range h.app.views.state().Views {
			if v.SessionID == h.session && v.State != "returned" {
				view = v.ID
				break
			}
		}
		if view != "" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if view == "" {
		return errors.New("profile130 actual child missing")
	}
	child, err := h.wait(view, func(s nativeTerminalSnapshot) bool {
		return s.Editable && s.TabPrimary == "7300" && s.Profiles130 != nil && !s.Profiles130.Explicit && s.Transcript == before.Transcript && s.Selection == before.Selection && s.Viewport == before.Viewport && s.ViewportDOM == float64(before.Viewport)
	})
	if err != nil {
		return err
	}
	if child.TabVisible {
		return errors.New("single child unexpectedly exposes hidden strip")
	}
	if _, err = h.command(view, "reattach"); err != nil {
		return err
	}
	if _, err = h.wait("", func(s nativeTerminalSnapshot) bool {
		return s.Editable && s.TabVisible && s.TabPrimary == "7300" && s.Transcript == before.Transcript && s.Selection == before.Selection && s.Viewport == before.Viewport && s.ViewportDOM == float64(before.Viewport)
	}); err != nil {
		return err
	}
	if accepted.Load() != 1 {
		return errors.New("detach/reload unexpectedly reconnected")
	}
	phases = append(phases, "Actual detached child/return preserve automatic metadata and transcript without reconnect")
	if _, err = h.command("", "profiles130:reconnect"); err != nil {
		return err
	}
	if _, err = check("7300", false); err != nil {
		return err
	}
	until = time.Now().Add(5 * time.Second)
	for accepted.Load() != 2 && time.Now().Before(until) {
		time.Sleep(25 * time.Millisecond)
	}
	if accepted.Load() != 2 {
		return errors.New("explicit reconnect did not create exactly one replacement loopback connection")
	}
	after, err := h.profile130Identity()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(identity, after) {
		return fmt.Errorf("profile configuration changed during naming (synthetic identity only): before=%v after=%v", identity, after)
	}
	if initial.Session != h.session {
		return errors.New("profile acceptance changed session identity")
	}
	phases = append(phases, "Explicit reconnect retains original session/profile identity and configuration")
	return h.capture("", "profiles130-final")
}
