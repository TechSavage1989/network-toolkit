package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"networktoolkit/internal/macros"
)

func TestTerminalMacroHistoryClearingHonorsWindowScopeAndActiveRuns(t *testing.T) {
	a := testApplication(t)
	for _, path := range []string{"/api/terminal-automation/runs", "/api/terminal-automation/runs/unknown"} {
		if response := requestApp(a, http.MethodDelete, path, "", false); response.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated clearing: %d %s", response.Code, response.Body.String())
		}
	}
	one, two := viewTestConnected(t, a), viewTestConnected(t, a)
	refs := map[string]macros.TargetRef{}
	for _, target := range a.terminalAutomation.Targets() {
		refs[target.ID] = macros.TargetRef{ID: target.ID, Generation: target.Generation}
	}
	definition := macroHTTPDefinition("clear-history")
	definition.Steps = []macros.Step{{ID: "pause", Kind: "pause"}}
	start := func(targets []macros.TargetRef) macros.Run {
		run := macroHTTPRun(t, viewTestRequest(a, a.token, http.MethodPost, "/api/terminal-automation/runs", macros.StartRequest{Macro: definition, Targets: targets}, nil), http.StatusCreated)
		viewTestWait(t, func() bool { r, err := a.terminalAutomation.Get(run.ID); return err == nil && r.State == "paused" })
		return run
	}
	stop := func(run macros.Run) {
		macroHTTPRun(t, viewTestRequest(a, a.token, http.MethodPost, "/api/terminal-automation/runs/"+run.ID+"/control", map[string]string{"action": "stop"}, nil), http.StatusOK)
		viewTestWait(t, func() bool { r, err := a.terminalAutomation.Get(run.ID); return err == nil && r.FinishedAt != "" })
	}
	own := start([]macros.TargetRef{refs[one.ID]})
	stop(own)
	group := start([]macros.TargetRef{refs[one.ID], refs[two.ID]})
	stop(group)
	active := start([]macros.TargetRef{refs[one.ID]})
	var launch terminalViewLaunch
	a.launchTerminalView = func(_ context.Context, r terminalViewLaunch) (<-chan error, error) { launch = r; return nil, nil }
	info, err := a.views.create(one.ID, viewTestSnapshot(a, one))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.views.ready(info.ID); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id   string
		code int
	}{{group.ID, http.StatusForbidden}, {active.ID, http.StatusConflict}} {
		response := viewTestRequest(a, launch.ViewToken, http.MethodDelete, "/api/terminal-automation/runs/"+tc.id, nil, nil)
		if response.Code != tc.code {
			t.Fatalf("scoped clear: %d %s", response.Code, response.Body.String())
		}
	}
	clear := func(token, path string) []string {
		response := viewTestRequest(a, token, http.MethodDelete, path, nil, nil)
		if response.Code != http.StatusOK {
			t.Fatalf("clear history: %d %s", response.Code, response.Body.String())
		}
		var body struct {
			Cleared []string `json:"cleared"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Cleared == nil {
			t.Fatal("cleared IDs must be an explicit array")
		}
		return body.Cleared
	}
	if ids := clear(launch.ViewToken, "/api/terminal-automation/runs"); len(ids) != 1 || ids[0] != own.ID {
		t.Fatalf("scoped clear removed wrong history: %v", ids)
	}
	for _, suffix := range []string{"", "/output?format=json"} {
		if response := viewTestRequest(a, a.token, http.MethodGet, "/api/terminal-automation/runs/"+own.ID+suffix, nil, nil); response.Code != http.StatusNotFound {
			t.Fatalf("cleared run or output survived: %d %s", response.Code, response.Body.String())
		}
	}
	if _, err := a.terminalAutomation.Get(group.ID); err != nil {
		t.Fatal("scoped clearing removed the mixed-session run", err)
	}
	if !a.terminalAutomation.OwnsPager(one.ID) {
		t.Fatal("clearing interrupted a paused run")
	}
	if ids := clear(a.token, "/api/terminal-automation/runs"); len(ids) != 1 || ids[0] != group.ID {
		t.Fatalf("owner clear removed wrong history: %v", ids)
	}
	if ids := clear(a.token, "/api/terminal-automation/runs"); len(ids) != 0 {
		t.Fatal("repeated clear should be empty", ids)
	}
	stop(active)
	if ids := clear(launch.ViewToken, "/api/terminal-automation/runs/"+active.ID); len(ids) != 1 || ids[0] != active.ID {
		t.Fatal(ids)
	}
	if response := viewTestRequest(a, a.token, http.MethodDelete, "/api/terminal-automation/runs/"+active.ID, nil, nil); response.Code != http.StatusNotFound {
		t.Fatalf("missing run: %d %s", response.Code, response.Body.String())
	}
	if len(a.terminalAutomation.List()) != 0 {
		t.Fatal("history was not emptied")
	}
	for _, id := range []string{one.ID, two.ID} {
		if session, ok := a.terminalSession(id); !ok || session.State != "connected" {
			t.Fatal("clearing closed a terminal", id)
		}
	}
}
