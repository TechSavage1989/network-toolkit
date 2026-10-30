package main

import (
	"context"
	"net/http"
	"testing"

	"networktoolkit/internal/macros"
)

func TestTerminalPaste127HTTPAuthenticationOwnershipAndScope(t *testing.T) {
	a := testApplication(t)
	path := "/api/terminal-automation/paste"
	if response := requestApp(a, http.MethodPost, path, `{}`, false); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated paste: %d", response.Code)
	}
	if response := requestApp(a, http.MethodGet, path, "", true); response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET paste: %d", response.Code)
	}
	session, other := viewTestConnected(t, a), viewTestConnected(t, a)
	refs := map[string]macros.TargetRef{}
	for _, target := range a.terminalAutomation.Targets() {
		refs[target.ID] = macros.TargetRef{ID: target.ID, Generation: target.Generation}
	}
	request := macros.PasteRequest{Target: refs[session.ID], Text: "fixture one\nfixture two\n"}
	stale := uint64(999)
	if response := viewTestRequest(a, a.token, http.MethodPost, path, request, &stale); response.Code != http.StatusConflict {
		t.Fatalf("stale ownership accepted: %d %s", response.Code, response.Body.String())
	}
	// The ordinary fixture is Telnet: the opt-in endpoint must return an
	// explicit failed run and not start sending to an unsupported transport.
	run := macroHTTPRun(t, viewTestRequest(a, a.token, http.MethodPost, path, request, nil), http.StatusCreated)
	if run.State != "failed" || len(run.Targets) != 1 || run.Targets[0].State != "failed" {
		t.Fatalf("non-SSH paste was not rejected: %+v", run)
	}
	var launch terminalViewLaunch
	a.launchTerminalView = func(_ context.Context, value terminalViewLaunch) (<-chan error, error) {
		launch = value
		return nil, nil
	}
	view, err := a.views.create(session.ID, viewTestSnapshot(a, session))
	if err != nil {
		t.Fatal(err)
	}
	if response := viewTestRequest(a, launch.ViewToken, http.MethodPost, path, request, nil); response.Code != http.StatusConflict {
		t.Fatalf("opening view accepted paste: %d %s", response.Code, response.Body.String())
	}
	view, err = a.views.ready(view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if response := viewTestRequest(a, a.token, http.MethodPost, path, request, &view.Revision); response.Code != http.StatusConflict {
		t.Fatalf("home sent into detached owner: %d %s", response.Code, response.Body.String())
	}
	if response := viewTestRequest(a, launch.ViewToken, http.MethodPost, path, request, nil); response.Code != http.StatusConflict {
		t.Fatalf("detached stale revision accepted: %d %s", response.Code, response.Body.String())
	}
	outside := macros.PasteRequest{Target: refs[other.ID], Text: request.Text}
	if response := viewTestRequest(a, launch.ViewToken, http.MethodPost, path, outside, &view.Revision); response.Code != http.StatusForbidden {
		t.Fatalf("detached view targeted outside session: %d %s", response.Code, response.Body.String())
	}
	run = macroHTTPRun(t, viewTestRequest(a, launch.ViewToken, http.MethodPost, path, request, &view.Revision), http.StatusCreated)
	if run.State != "failed" {
		t.Fatalf("scoped unsupported transport should fail explicitly: %+v", run)
	}
	request.Text = "unfinished"
	if response := viewTestRequest(a, launch.ViewToken, http.MethodPost, path, request, &view.Revision); response.Code != http.StatusConflict {
		t.Fatalf("unfinished command acquired Enter: %d %s", response.Code, response.Body.String())
	}
}
