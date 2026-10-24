package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestActualBrowserWorkspacePayloadDetachesActiveTerminal(t *testing.T) {
	node, e := exec.LookPath("node")
	if e != nil {
		t.Skip("Node is required for the bundled browser payload fixture")
	}
	a := testApplication(t)
	var launched terminalViewLaunch
	a.launchTerminalView = func(_ context.Context, l terminalViewLaunch) (<-chan error, error) { launched = l; return nil, nil }
	session := viewTestConnected(t, a)
	raw, e := exec.Command(node, "scripts/terminal-transfer-fixture.cjs", session.ID, session.Host, strconv.Itoa(session.Port)).Output()
	if e != nil {
		if exit, ok := e.(*exec.ExitError); ok {
			t.Fatalf("browser snapshot: %s", exit.Stderr)
		}
		t.Fatal(e)
	}
	for _, secret := range []string{"credentialId", "saved-reference-canary", "password-canary"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatal("connection secret/reference leaked into handoff", secret)
		}
	}
	r := httptest.NewRequest("POST", "http://"+a.host+"/api/terminal-views/workspace", bytes.NewReader(raw))
	r.Header.Set("X-Session-Token", a.token)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatalf("real browser handoff rejected: %d %s", w.Code, w.Body)
	}
	if launched.ViewID == "" {
		t.Fatal("workspace view was not launched")
	}
	response := viewTestRequest(a, launched.ViewToken, "GET", "/api/terminal-views/current", nil, nil)
	if response.Code != 200 || !strings.Contains(response.Body.String(), "configuration 0") || !strings.Contains(response.Body.String(), "configuration 11999") {
		t.Fatal("long archived command output was not transferred", response.Code)
	}
	var envelope map[string]any
	if e = json.Unmarshal(raw, &envelope); e != nil {
		t.Fatal(e)
	}
	workspace := envelope["workspace"].(map[string]any)
	snapshot := workspace["sessions"].(map[string]any)[session.ID].(map[string]any)
	snapshot["config"].(map[string]any)["credentialId"] = ""
	regression, _ := json.Marshal(envelope)
	decoder := json.NewDecoder(bytes.NewReader(regression))
	decoder.DisallowUnknownFields()
	var typed struct {
		Workspace terminalWorkspaceSnapshot `json:"workspace"`
	}
	if decoder.Decode(&typed) == nil {
		t.Fatal("fixture did not exercise strict credential-field rejection")
	}
}
