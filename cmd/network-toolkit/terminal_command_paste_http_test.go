package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"networktoolkit/internal/macros"
	"networktoolkit/internal/terminal"
)

// Observe only a loopback fixture. Reading through the successful payload also
// proves that rejected earlier requests did not leave input queued ahead of it.
func commandPasteHTTPConnected(t *testing.T, a *application) (terminal.SessionInfo, <-chan string) {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	received := make(chan string, 128)
	go func() {
		defer close(received)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
		_, _ = io.WriteString(conn, "Command review fixture\r\nrouter> ")
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if n > 0 {
				received <- string(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()
	session, err := a.terminals.Connect(terminal.ConnectRequest{Type: "telnet", Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, LineEnding: "lf"})
	if err != nil {
		t.Fatal(err)
	}
	viewTestWait(t, func() bool {
		current, ok := a.terminalSession(session.ID)
		if ok && current.State == "connected" {
			session = current
			return true
		}
		return false
	})
	return session, received
}

func commandPasteHTTPReadThrough(t *testing.T, received <-chan string, ending string) string {
	t.Helper()
	var data strings.Builder
	timer := time.NewTimer(4 * time.Second)
	defer timer.Stop()
	for {
		select {
		case chunk, ok := <-received:
			if !ok {
				t.Fatalf("fixture closed before reviewed input arrived: %q", data.String())
			}
			data.WriteString(chunk)
			if strings.Contains(data.String(), ending) {
				return data.String()
			}
		case <-timer.C:
			t.Fatalf("reviewed input did not arrive: %q", data.String())
		}
	}
}

func commandPasteHTTPRefs(a *application, ids ...string) []macros.TargetRef {
	byID := map[string]macros.TargetRef{}
	for _, target := range a.terminalAutomation.Targets() {
		byID[target.ID] = macros.TargetRef{ID: target.ID, Generation: target.Generation}
	}
	refs := make([]macros.TargetRef, 0, len(ids))
	for _, id := range ids {
		refs = append(refs, byID[id])
	}
	return refs
}

func TestTerminalCommandPasteHTTPRejectsWholeMixedScopeBeforeAnyInput(t *testing.T) {
	a := testApplication(t)
	session, ownInput := commandPasteHTTPConnected(t, a)
	other, otherInput := commandPasteHTTPConnected(t, a)
	var launch terminalViewLaunch
	a.launchTerminalView = func(_ context.Context, request terminalViewLaunch) (<-chan error, error) {
		launch = request
		return nil, nil
	}
	view, err := a.views.create(session.ID, viewTestSnapshot(a, session))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.views.ready(view.ID); err != nil {
		t.Fatal(err)
	}
	refs := commandPasteHTTPRefs(a, session.ID, other.ID)
	for _, method := range []string{"raw", "paced"} {
		response := viewTestRequest(a, launch.ViewToken, http.MethodPost, "/api/terminal-automation/send", macros.SendRequest{Targets: refs, Text: "FORBIDDEN-REVIEW\n", Method: method}, nil)
		if response.Code != http.StatusForbidden {
			t.Fatalf("mixed %s review: %d %s", method, response.Code, response.Body.String())
		}
	}
	if len(a.terminalAutomation.List()) != 0 {
		t.Fatal("out-of-scope paced review created a partial run")
	}
	// Main-window group capability intentionally includes a session whose
	// display is detached. No single-owner revision header is required here.
	text := "REVIEWED {{host}}\n\nREVIEW-END"
	response := viewTestRequest(a, a.token, http.MethodPost, "/api/terminal-automation/send", macros.SendRequest{Targets: refs, Text: text, Execute: true, Method: "raw", Bracketed: map[string]bool{session.ID: true}}, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("main group review: %d %s", response.Code, response.Body.String())
	}
	var result macros.SendResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || len(result.Targets) != 2 || !result.Targets[0].Sent || !result.Targets[1].Sent {
		t.Fatalf("main group result: %+v, %v", result, err)
	}
	// This loopback transport uses Telnet NVT, which encodes each input LF as
	// CRLF on the wire. Engine tests separately check every target line ending.
	wireText := strings.ReplaceAll(text, "\n", "\r\n")
	for _, target := range []struct {
		input    <-chan string
		expected string
	}{{ownInput, "\x1b[200~" + wireText + "\x1b[201~"}, {otherInput, wireText}} {
		wire := commandPasteHTTPReadThrough(t, target.input, target.expected)
		if strings.Contains(wire, "FORBIDDEN-REVIEW") || strings.Count(wire, wireText) != 1 || !strings.HasSuffix(wire, target.expected) {
			t.Fatalf("scope or reviewed text changed on the wire: %q", wire)
		}
	}
}

func TestTerminalCommandPasteHTTPRejectsInvalidReviewAndStaleGeneration(t *testing.T) {
	a := testApplication(t)
	session, received := commandPasteHTTPConnected(t, a)
	refs := commandPasteHTTPRefs(a, session.ID)
	for _, request := range []macros.SendRequest{
		{Targets: refs, Text: "INVALID-REVIEW\n", Method: "unknown"},
		{Targets: refs, Text: "INVALID-REVIEW\n", Method: "raw", Bracketed: map[string]bool{"outside": true}},
		{Targets: refs, Text: "INVALID-REVIEW", Method: "paced"},
		{Targets: refs, Text: "INVALID-REVIEW\n", Method: "paced", Bracketed: map[string]bool{session.ID: true}},
	} {
		response := viewTestRequest(a, a.token, http.MethodPost, "/api/terminal-automation/send", request, nil)
		if response.Code != http.StatusConflict {
			t.Fatalf("invalid review: %d %s", response.Code, response.Body.String())
		}
	}
	stale := append([]macros.TargetRef(nil), refs...)
	stale[0].Generation++
	response := viewTestRequest(a, a.token, http.MethodPost, "/api/terminal-automation/send", macros.SendRequest{Targets: stale, Text: "STALE-REVIEW\n", Method: "raw"}, nil)
	var result macros.SendResult
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &result) != nil || len(result.Targets) != 1 || result.Targets[0].Sent || result.Targets[0].Error == "" {
		t.Fatalf("stale review did not report a per-target failure: %d %s", response.Code, response.Body.String())
	}
	if len(a.terminalAutomation.List()) != 0 {
		t.Fatal("invalid review created a run")
	}
	text := "APPROVED first\n\nAPPROVED-END\n\n"
	response = viewTestRequest(a, a.token, http.MethodPost, "/api/terminal-automation/send", macros.SendRequest{Targets: refs, Text: text, Method: "raw"}, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("valid review: %d %s", response.Code, response.Body.String())
	}
	wireText := strings.ReplaceAll(text, "\n", "\r\n")
	wire := commandPasteHTTPReadThrough(t, received, wireText)
	if strings.Contains(wire, "INVALID-REVIEW") || strings.Contains(wire, "STALE-REVIEW") || strings.Count(wire, wireText) != 1 || !strings.HasSuffix(wire, wireText) {
		t.Fatalf("rejected or modified text reached fixture: %q", wire)
	}
}

func TestTerminalCommandPasteHTTPPacedCreatesOneGroupWithPerTargetTransportErrors(t *testing.T) {
	a := testApplication(t)
	first, second := viewTestConnected(t, a), viewTestConnected(t, a)
	refs := commandPasteHTTPRefs(a, first.ID, second.ID)
	// Execute is deliberately false. An explicitly reviewed method controls
	// delivery, and paced Telnet destinations must retain the SSH-only error.
	run := macroHTTPRun(t, viewTestRequest(a, a.token, http.MethodPost, "/api/terminal-automation/send", macros.SendRequest{Targets: refs, Text: "one {{host}}\n\ntwo\n", Method: "paced"}, nil), http.StatusCreated)
	if len(a.terminalAutomation.List()) != 1 || run.State != "failed" || len(run.Targets) != 2 {
		t.Fatalf("paced group was not retained as one failed run: %+v", run)
	}
	for _, target := range run.Targets {
		if target.State != "failed" || target.Total != 3 || target.Step != 0 || !strings.Contains(target.Error, "only for SSH") {
			t.Fatalf("paced transport error or literal line count lost: %+v", target)
		}
	}
}

func TestTerminalCommandPasteHTTPAcceptsEscaped64KiBLiteralPayload(t *testing.T) {
	a := testApplication(t)
	session, received := commandPasteHTTPConnected(t, a)
	// JSON escaping pushes this valid 64 KiB review past the former 128 KiB
	// envelope limit. Its three trailing Enter boundaries must all survive.
	text := strings.Repeat("\"", macros.MaxStepTextBytes-3) + "\n\n\n"
	request := macros.SendRequest{Targets: commandPasteHTTPRefs(a, session.ID), Text: text, Execute: true, Method: "raw"}
	encoded, err := json.Marshal(request)
	if err != nil || len(encoded) <= 128<<10 {
		t.Fatalf("fixture does not exceed the previous envelope cap: %d, %v", len(encoded), err)
	}
	response := viewTestRequest(a, a.token, http.MethodPost, "/api/terminal-automation/send", request, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("valid escaped review: %d %s", response.Code, response.Body.String())
	}
	wireText := strings.ReplaceAll(text, "\n", "\r\n")
	wire := commandPasteHTTPReadThrough(t, received, wireText)
	start := strings.Index(wire, strings.Repeat("\"", 32))
	if start < 0 || wire[start:] != wireText || strings.Count(wire[start:], "\n") != 3 {
		t.Fatalf("64 KiB reviewed payload changed: received %d bytes", len(wire))
	}
}
