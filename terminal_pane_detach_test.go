package main

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"networktoolkit/internal/terminal"
)

func paneDetachFixture(t *testing.T) (*application, terminalViewInfo, terminalViewLaunch, []terminal.SessionInfo, terminalWorkspaceSnapshot) {
	t.Helper()
	a := testApplication(t)
	sessions := []terminal.SessionInfo{viewTestConnected(t, a), viewTestConnected(t, a), viewTestConnected(t, a)}
	workspace := terminalWorkspaceSnapshot{
		Sessions: map[string]terminalScreenSnapshot{}, Panes: []string{sessions[0].ID, sessions[1].ID, sessions[2].ID, ""},
		Active: sessions[0].ID, PaneFocus: 0, Maximized: -1,
		Display: map[string]any{"theme": "light", "terminalFontSize": 16, "terminalLayout": map[string]any{"kind": "grid"}, "terminalProfiles": []any{map[string]any{"name": "Private profile", "host": "profile.example"}}},
	}
	for _, session := range sessions {
		workspace.Sessions[session.ID] = viewTestSnapshot(a, session)
	}
	view, launch := workspaceTestCreate(t, a, workspace)
	if _, err := a.views.ready(view.ID); err != nil {
		t.Fatal(err)
	}
	return a, view, launch, sessions, workspace
}

func TestTerminalPaneDetachFromLinkedWorkspace(t *testing.T) {
	a, source, sourceLaunch, sessions, original := paneDetachFixture(t)
	var childLaunch terminalViewLaunch
	a.launchTerminalView = func(_ context.Context, launch terminalViewLaunch) (<-chan error, error) {
		childLaunch = launch
		return nil, nil
	}
	selected := sessions[1]
	snapshot := viewTestSnapshot(a, selected)
	snapshot.Data = "retained selected pane\r\nswitch# "
	response := workspaceTestDecode(t, viewTestRequest(a, sourceLaunch.ViewToken, http.MethodPost, "/api/terminal-views/current/detach-session", map[string]any{"sessionId": selected.ID, "snapshot": snapshot}, nil), http.StatusCreated)
	if response.View.Workspace || response.View.SessionID != selected.ID || childLaunch.ViewID != response.View.ID || childLaunch.ViewToken == sourceLaunch.ViewToken {
		t.Fatal("selected session did not receive its own isolated view")
	}
	_, remaining, found := a.views.workspaceGet(source.ID)
	if !found || len(remaining.Sessions) != 2 || !reflect.DeepEqual(remaining.Panes, []string{sessions[0].ID, "", sessions[2].ID, ""}) || remaining.Active != original.Active {
		t.Fatalf("other panes or their positions changed: %+v", remaining)
	}
	state := a.views.state()
	for _, session := range []terminal.SessionInfo{sessions[0], sessions[2]} {
		if state.Owners[session.ID].ViewID != source.ID || !state.Owners[session.ID].Ready {
			t.Fatal("another pane lost ownership or input availability")
		}
	}
	if state.Owners[selected.ID].ViewID != response.View.ID || state.Owners[selected.ID].Ready {
		t.Fatal("selected pane input was not paused during transfer")
	}
	if len(response.ViewState.Views) != 1 || response.ViewState.Views[0].ID != source.ID || !response.ViewState.Available {
		t.Fatal("workspace metadata leaked another view capability or lost detachment availability")
	}
	if rejected := viewTestRequest(a, sourceLaunch.ViewToken, "POST", "/api/terminals/"+selected.ID+"/input", map[string]string{"data": "eA=="}, nil); rejected.Code != http.StatusForbidden {
		t.Fatal("source workspace could still write to its detached pane")
	}
	if stale := viewTestRequest(a, sourceLaunch.ViewToken, "POST", "/api/terminal-views/current/checkpoint", map[string]any{"workspace": original}, nil); stale.Code == http.StatusOK {
		t.Fatal("old workspace checkpoint reclaimed the detached session")
	}
	workspaceTestDecode(t, viewTestRequest(a, sourceLaunch.ViewToken, "POST", "/api/terminal-views/current/checkpoint", map[string]any{"workspace": remaining}, nil), 200)
	workspaceTestDecode(t, viewTestRequest(a, childLaunch.ViewToken, "POST", "/api/terminal-views/current/ready", map[string]any{}, nil), 200)
	current := viewTestRequest(a, childLaunch.ViewToken, "GET", "/api/terminal-views/current", nil, nil)
	if !strings.Contains(current.Body.String(), "retained selected pane") {
		t.Fatal("selected pane scrollback did not reach the new window")
	}
	display := viewTestRequest(a, childLaunch.ViewToken, "GET", "/api/state", nil, nil)
	if !strings.Contains(display.Body.String(), `"terminalFontSize":16`) || strings.Contains(display.Body.String(), "profile.example") {
		t.Fatal("single pane display settings were lost or included workspace profiles")
	}
	if rejected := viewTestRequest(a, childLaunch.ViewToken, "POST", "/api/terminal-views/current/detach-session", map[string]any{"sessionId": sessions[0].ID, "snapshot": snapshot}, nil); rejected.Code != http.StatusForbidden {
		t.Fatal("single pane capability could detach a sibling")
	}
	returned := workspaceTestDecode(t, viewTestRequest(a, childLaunch.ViewToken, "POST", "/api/terminal-views/current/reattach", map[string]any{"snapshot": snapshot}, nil), 200)
	if err := a.views.accept(response.View.ID, returned.View.Revision); err != nil {
		t.Fatal(err)
	}
	state = a.views.state()
	if state.Owners[selected.ID].ViewID != "" || !state.Owners[selected.ID].Ready || state.Owners[sessions[0].ID].ViewID != source.ID || state.Owners[sessions[2].ID].ViewID != source.ID {
		t.Fatal("reattaching the selected pane changed sibling ownership")
	}
	if len(a.terminals.List()) != 3 {
		t.Fatal("moving a pane created another connection")
	}
	for _, expected := range sessions {
		if actual, found := a.terminalSession(expected.ID); !found || actual.State != "connected" {
			t.Fatal("moving a pane reconnected or disconnected an existing session")
		}
	}
}

func TestTerminalPaneDetachFailureRestoresSourceAndScope(t *testing.T) {
	a, source, launch, sessions, original := paneDetachFixture(t)
	outsider := viewTestConnected(t, a)
	request := map[string]any{"sessionId": outsider.ID, "snapshot": viewTestSnapshot(a, outsider)}
	if result := viewTestRequest(a, launch.ViewToken, "POST", "/api/terminal-views/current/detach-session", request, nil); result.Code != http.StatusConflict {
		t.Fatalf("workspace detached an unrelated session: %d", result.Code)
	}
	a.launchTerminalView = func(context.Context, terminalViewLaunch) (<-chan error, error) {
		return nil, errors.New("test launch failed")
	}
	request = map[string]any{"sessionId": sessions[1].ID, "snapshot": viewTestSnapshot(a, sessions[1])}
	if result := viewTestRequest(a, launch.ViewToken, "POST", "/api/terminal-views/current/detach-session", request, nil); result.Code != http.StatusBadRequest {
		t.Fatalf("failed pane launch reported success: %d", result.Code)
	}
	_, restored, _ := a.views.workspaceGet(source.ID)
	if len(restored.Sessions) != 3 || !reflect.DeepEqual(restored.Panes, original.Panes) || restored.Active != original.Active || a.views.count() != 1 {
		t.Fatal("failed pane launch changed the source workspace")
	}
	owner := a.views.state().Owners[sessions[1].ID]
	if owner.ViewID != source.ID || !owner.Ready {
		t.Fatal("failed pane launch did not restore source input ownership")
	}
}
