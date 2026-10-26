package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestTerminalOutputBlocksTransferValidationAndIsolation(t *testing.T) {
	enabled := true
	original := &terminalOutputBlocks{Version: 1, Choice: "last-command", Settings: terminalSessionOutputSettings{AutoPager: &enabled, PagerKey: "auto"}, Blocks: []terminalOutputBlock{{ID: "command-1", Kind: "command", Start: 4, End: 10, Closed: true, At: 1, Echo: []terminalOutputEcho{{Start: 4, End: 5}}}}}
	original.Input = &terminalInputPosition{Line: 9, Offset: 8}
	if err := original.validate(); err != nil {
		t.Fatal(err)
	}
	cloned := original.clone()
	cloned.Input.Offset = 10
	cloned.Blocks[0].Echo[0].End = 8
	*cloned.Settings.AutoPager = false
	if original.Blocks[0].Echo[0].End != 5 || !*original.Settings.AutoPager || original.Input.Offset != 8 {
		t.Fatal("snapshot shares mutable history or settings")
	}
	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var decoded terminalOutputBlocks
	if err = json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if err = decoded.validate(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*terminalOutputBlocks){
		func(s *terminalOutputBlocks) { s.Version = 2 }, func(s *terminalOutputBlocks) { s.Choice = strings.Repeat("x", 129) },
		func(s *terminalOutputBlocks) { s.Settings.PagerKey = "send-arbitrary-input" },
		func(s *terminalOutputBlocks) { s.Input.Line = -1 }, func(s *terminalOutputBlocks) { s.Input.Line = 10301 },
		func(s *terminalOutputBlocks) { s.Input.Offset = -1 }, func(s *terminalOutputBlocks) { s.Input.Offset = 8193 },
		func(s *terminalOutputBlocks) { s.Blocks[0].Start = -1 }, func(s *terminalOutputBlocks) { s.Blocks[0].End = 10301 },
		func(s *terminalOutputBlocks) { s.Blocks[0].End = 3 }, func(s *terminalOutputBlocks) { s.Blocks[0].Kind = "password" },
		func(s *terminalOutputBlocks) { s.Blocks[0].Echo[0].Start = 1 }, func(s *terminalOutputBlocks) { s.Blocks[0].Echo[0].End = 10301 },
		func(s *terminalOutputBlocks) { s.Blocks = append(s.Blocks, s.Blocks[0]) },
	} {
		bad := original.clone()
		change(bad)
		if bad.validate() == nil {
			t.Fatal("invalid output history accepted")
		}
	}
}
func TestTerminalOutputHistoryDoesNotEnterSettings(t *testing.T) {
	state := map[string]any{"terminalInteraction": map[string]any{"autoPager": true, "outputBlocks": "entered password", "rawInput": "hidden input"}, "terminalProfiles": []any{map[string]any{"id": "p", "name": "a", "type": "ssh", "host": "192.0.2.1", "outputBlocks": "typed commands", "password": "secret"}}}
	sanitizeTerminalSettings(state)
	raw, _ := json.Marshal(state)
	for _, secret := range []string{"entered password", "hidden input", "typed commands", "secret"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("sensitive input reached settings: %s", raw)
		}
	}
}

func TestArchivedOutputTransferBoundsAndDeepClone(t *testing.T) {
	original := &terminalOutputBlocks{Version: 1, Choice: "last-command", Blocks: []terminalOutputBlock{{ID: "archived", Kind: "command", Start: -1, End: -1, Closed: true, Archive: &terminalOutputArchive{Screen: "switch# show all\nFIRST\nLAST", Echo: []int{0}, Closed: true}}}}
	if e := original.validate(); e != nil {
		t.Fatal(e)
	}
	cloned := original.clone()
	cloned.Blocks[0].Archive.Screen = "changed"
	cloned.Blocks[0].Archive.Echo[0] = 2
	if original.Blocks[0].Archive.Screen == "changed" || original.Blocks[0].Archive.Echo[0] != 0 {
		t.Fatal("archive clone shares mutable state")
	}
	for _, change := range []func(*terminalOutputBlocks){func(s *terminalOutputBlocks) { s.Blocks[0].Archive.Screen = strings.Repeat("a", (4<<20)+1) }, func(s *terminalOutputBlocks) { s.Blocks[0].Archive.Echo = []int{-1} }, func(s *terminalOutputBlocks) { s.Blocks[0].Archive.Echo = []int{3} }, func(s *terminalOutputBlocks) { s.Blocks[0].Archive.Screen = string([]byte{0xff}) }} {
		bad := original.clone()
		change(bad)
		if bad.validate() == nil {
			t.Fatal("invalid archive accepted")
		}
	}
}

func TestTerminalOutputHistoryRoundTripsThroughLinkedView(t *testing.T) {
	a := testApplication(t)
	var launch terminalViewLaunch
	a.launchTerminalView = func(_ context.Context, l terminalViewLaunch) (<-chan error, error) { launch = l; return nil, nil }
	session := viewTestConnected(t, a)
	snapshot := viewTestSnapshot(a, session)
	enabled := true
	snapshot.OutputBlocks = &terminalOutputBlocks{Version: 1, Choice: "last-command", Settings: terminalSessionOutputSettings{AutoPager: &enabled}, Blocks: []terminalOutputBlock{{ID: "block-1", Kind: "command", Start: 0, End: -1, At: 1, Echo: []terminalOutputEcho{{Start: 0, End: 0}}}}}
	snapshot.OutputBlocks.Input = &terminalInputPosition{Line: 0, Offset: 8}
	created := viewTestRequest(a, a.token, "POST", "/api/terminal-views", map[string]any{"sessionId": session.ID, "snapshot": snapshot}, nil)
	if created.Code != 201 {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	response := viewTestRequest(a, launch.ViewToken, "GET", "/api/terminal-views/current", nil, nil)
	var payload struct {
		Snapshot terminalScreenSnapshot `json:"snapshot"`
	}
	if response.Code != 200 {
		t.Fatalf("read: %d %s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	blocks := payload.Snapshot.OutputBlocks
	if blocks == nil || len(blocks.Blocks) != 1 || blocks.Blocks[0].ID != "block-1" || blocks.Settings.AutoPager == nil || !*blocks.Settings.AutoPager || blocks.Input == nil || blocks.Input.Line != 0 || blocks.Input.Offset != 8 {
		t.Fatalf("output history lost during transfer: %s", response.Body.String())
	}
	a.stateMu.Lock()
	state := string(a.state)
	a.stateMu.Unlock()
	if strings.Contains(state, "block-1") {
		t.Fatal("transient output history was saved in normal settings")
	}
}
