package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func linkEnvelopeJSON(t *testing.T, raw string) string {
	t.Helper()
	link, err := parseTerminalLink(raw, "windows")
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(terminalWindowEnvelope{Config: &link.Preset, Link: &link.Link})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestTerminalLinkRequestQueuesOnlyAReviewForm(t *testing.T) {
	a := testApplication(t)
	a.windowMode = "terminal"
	r := requestApp(a, "POST", "/api/terminal-window", linkEnvelopeJSON(t, "ssh://operator:Secr3t@192.0.2.10:2222"), true)
	if r.Code != 200 {
		t.Fatalf("Link request failed: %d %s", r.Code, r.Body.String())
	}
	batch := a.windowInbox.batch(0)
	if len(batch.Requests) != 1 || batch.Requests[0].Link == nil || !batch.Requests[0].Link.PasswordIgnored || batch.Requests[0].Link.Scheme != "ssh" || batch.Requests[0].Config.Host != "192.0.2.10" || batch.Requests[0].Config.Port != 2222 || batch.Requests[0].Config.Username != "operator" {
		t.Fatalf("Link request was not queued for review: %+v", batch)
	}
	if len(a.terminals.List()) != 0 || len(a.jobs) != 0 {
		t.Fatal("A link connected or ran something without the user's confirmation")
	}
	response := requestApp(a, "GET", "/api/terminal-window/requests", "", true).Body.String()
	if strings.Contains(response, "Secr3t") || !strings.Contains(response, `"passwordIgnored":true`) {
		t.Fatalf("Window request response: %s", response)
	}
	if b, err := os.ReadFile(a.dataPath); err == nil && strings.Contains(string(b), "192.0.2.10") {
		t.Fatal("A link was written to the settings file")
	}
}

func TestTerminalLinkRequestRejectsForgedLinkMetadata(t *testing.T) {
	a := testApplication(t)
	a.windowMode = "terminal"
	for _, body := range []string{
		`{"link":{"scheme":"ssh"}}`,
		`{"config":{"type":"ssh","host":"192.0.2.1"},"link":{"scheme":"telnet"}}`,
		`{"config":{"type":"ssh","host":"192.0.2.1"},"link":{"scheme":"http"}}`,
		`{"config":{"type":"ssh","host":""},"link":{"scheme":"ssh"}}`,
		`{"config":{"type":"ssh","host":"192.0.2.1","profileId":"core"},"link":{"scheme":"ssh"}}`,
		`{"config":{"type":"ssh","host":"192.0.2.1","serialPort":"COM3"},"link":{"scheme":"ssh"}}`,
		`{"config":{"type":"telnet","host":"192.0.2.1","username":"admin"},"link":{"scheme":"telnet"}}`,
		`{"config":{"type":"serial","serialPort":"COM3","host":"192.0.2.1"},"link":{"scheme":"serial"}}`,
		`{"config":{"type":"serial"},"link":{"scheme":"serial"}}`,
		`{"config":{"type":"powershell"},"link":{"scheme":"powershell"}}`,
		`{"config":{"type":"ssh","host":"192.0.2.1"},"link":{"scheme":"ssh","password":"secret"}}`,
		`{"config":{"type":"ssh","host":"192.0.2.1"},"link":{"scheme":"ssh","url":"ssh://x"}}`,
	} {
		if r := requestApp(a, "POST", "/api/terminal-window", body, true); r.Code != 400 {
			t.Fatalf("Forged link request accepted (%d): %s", r.Code, body)
		}
	}
	if len(a.windowInbox.batch(0).Requests) != 0 {
		t.Fatal("A rejected link reached the inbox")
	}
	a.windowMode = "main"
	a.openTerminalWindow = func(context.Context, *terminalWindowPreset) (bool, error) {
		t.Fatal("A link request launched a window from the main workspace")
		return false, nil
	}
	if r := requestApp(a, "POST", "/api/terminal-window", linkEnvelopeJSON(t, "ssh://192.0.2.10"), true); r.Code != 400 {
		t.Fatalf("The main workspace accepted link metadata: %d", r.Code)
	}
}

func TestTerminalLinkDuplicatesAreIgnoredBrieflyAndQueueIsBounded(t *testing.T) {
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	terminalWindowNow = func() time.Time { return now }
	t.Cleanup(func() { terminalWindowNow = time.Now })
	var q terminalWindowInbox
	link, err := parseTerminalLink("telnet://192.0.2.20", "windows")
	if err != nil {
		t.Fatal(err)
	}
	if !q.addRequest(&link.Preset, &link.Link) {
		t.Fatal("First link was not queued")
	}
	now = now.Add(time.Second)
	if q.addRequest(&link.Preset, &link.Link) {
		t.Fatal("An immediate duplicate link was queued")
	}
	other, _ := parseTerminalLink("telnet://192.0.2.21", "windows")
	if !q.addRequest(&other.Preset, &other.Link) {
		t.Fatal("A different link was treated as a duplicate")
	}
	if !q.addRequest(&link.Preset, &link.Link) {
		t.Fatal("A repeated link after another link was dropped")
	}
	now = now.Add(terminalLinkDuplicateWindow)
	if !q.addRequest(&link.Preset, &link.Link) {
		t.Fatal("A deliberate later click on the same link was dropped")
	}
	// Presets from the main window are never collapsed: each is a user action.
	preset := link.Preset
	q.add(&preset)
	q.add(&preset)
	if got := len(q.batch(0).Requests); got != 6 {
		t.Fatalf("Unexpected queue length %d", got)
	}
	for i := 0; i < terminalWindowRequestLimit+8; i++ {
		now = now.Add(time.Millisecond)
		next, err := parseTerminalLink(fmt.Sprintf("ssh://192.0.2.%d", i+30), "windows")
		if err != nil {
			t.Fatal(err)
		}
		q.addRequest(&next.Preset, &next.Link)
	}
	batch := q.batch(0)
	if len(batch.Requests) != terminalWindowRequestLimit || !batch.Lost || batch.Requests[len(batch.Requests)-1].Config.Host != fmt.Sprintf("192.0.2.%d", terminalWindowRequestLimit+8+29) {
		t.Fatalf("Queue was not bounded to the newest requests: %d lost=%v", len(batch.Requests), batch.Lost)
	}
	// The queued link metadata is a copy and cannot be changed by the caller.
	link.Link.PasswordIgnored = true
	for _, request := range q.batch(0).Requests {
		if request.Link != nil && request.Link.PasswordIgnored {
			t.Fatal("Queued link metadata aliased the caller's value")
		}
	}
}

func TestTerminalLinkSecondLaunchHandsOffToRunningWindow(t *testing.T) {
	root := t.TempDir()
	dir := terminalWorkspace(root)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	release, err := acquireTerminalWindowLock(filepath.Join(dir, "Network_Toolkit.window.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	a := newApplication(t.Context(), filepath.Join(dir, "Network_Toolkit.settings.json"))
	t.Cleanup(func() {
		a.stop()
		if a.terminals != nil {
			_ = a.terminals.Close()
		}
	})
	a.windowMode = "terminal"
	server := httptest.NewServer(a)
	defer server.Close()
	a.host = strings.TrimPrefix(server.URL, "http://")
	unregister, err := a.registerTerminalWindow(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer unregister()
	link, err := parseTerminalLink("serial:COM7?baud=115200&parity=n", "windows")
	if err != nil {
		t.Fatal(err)
	}
	request := terminalWindowEnvelope{Config: &link.Preset, Link: &link.Link}
	for attempt := 0; attempt < 2; attempt++ {
		_, releaseSecond, reused, err := prepareTerminalWindowRequest(root, request)
		if err != nil || !reused || releaseSecond != nil {
			t.Fatalf("Second launch did not hand off: reused=%v err=%v", reused, err)
		}
	}
	batch := a.windowInbox.batch(0)
	if len(batch.Requests) != 1 || batch.Requests[0].Link == nil || batch.Requests[0].Link.Scheme != "serial" || batch.Requests[0].Config.SerialPort != "COM7" || batch.Requests[0].Config.BaudRate != 115200 {
		t.Fatalf("Running window did not receive exactly one link request: %+v", batch)
	}
	select {
	case <-a.windowActivate:
	default:
		t.Fatal("The running Terminal window was not brought forward")
	}
	if len(a.terminals.List()) != 0 {
		t.Fatal("Handing off a link connected")
	}
}

func TestTerminalLinkIsNotReplayedAfterUpdateRestart(t *testing.T) {
	for _, tc := range []struct{ in, want []string }{
		{[]string{"--terminal", "--open-url", "ssh://192.0.2.10"}, []string{"--terminal"}},
		{[]string{"--terminal", "--open-url=ssh://192.0.2.10"}, []string{"--terminal"}},
		{[]string{"-terminal", "-open-url", "serial:COM3", "--data-dir", "C:/data"}, []string{"-terminal", "--data-dir", "C:/data"}},
		{[]string{"--data-dir", "--open-url", "--terminal"}, []string{"--data-dir", "--open-url", "--terminal"}},
	} {
		if got := updateRestartArgs(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("updateRestartArgs(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
