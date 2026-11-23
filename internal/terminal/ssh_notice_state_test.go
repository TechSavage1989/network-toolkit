package terminal

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"
)

func TestSSHNoticePreferenceIsSSHOnly(t *testing.T) {
	for _, kind := range []string{"ssh", "telnet", "serial", "cmd", "powershell"} {
		for _, enabled := range []bool{false, true} {
			req := terminalRequest()
			req.Type, req.SSHAutoContinueNotices = kind, enabled
			req.SerialPort = "/dev/ttyUSB7"
			if runtime.GOOS == "windows" {
				req.SerialPort = "COM7"
			}
			got, err := normalizeRequest(req)
			if err != nil {
				t.Fatalf("normalize %s: %v", kind, err)
			}
			if want := kind == "ssh" && enabled; got.SSHAutoContinueNotices != want {
				t.Fatalf("%s notice preference=%v, want %v", kind, got.SSHAutoContinueNotices, want)
			}
		}
	}
	for _, raw := range []string{`{}`, `{"sshAutoContinueNotices":false}`} {
		var req ConnectRequest
		if err := json.Unmarshal([]byte(raw), &req); err != nil || req.SSHAutoContinueNotices {
			t.Fatalf("missing/disabled notice preference enabled automatic continuation: %v", err)
		}
	}
	var req ConnectRequest
	if err := json.Unmarshal([]byte(`{"sshAutoContinueNotices":"true"}`), &req); err == nil {
		t.Fatal("request accepted a non-boolean notice preference")
	}
}

func TestSSHNoticePreferenceSurvivesSessionReconnect(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		requests := make(chan ConnectRequest, 2)
		m := testManager(t, func(_ context.Context, req ConnectRequest, _ transportCallbacks) (transport, error) {
			requests <- req
			return newFakeTerminal(false), nil
		})
		req := terminalRequest()
		req.CredentialID = "fixture-notice-credential"
		req.SSHAutoContinueNotices = enabled
		m.SetCredentialResolver(func(_ context.Context, safe ConnectRequest) (ConnectRequest, error) {
			if safe.SSHAutoContinueNotices != enabled || safe.Password != "" || safe.PrivateKey != "" || safe.Passphrase != "" {
				t.Error("reconnect resolver lost notice policy or received retained secrets")
			}
			// A credential resolver supplies authentication, not a changed policy.
			safe.SSHAutoContinueNotices = !enabled
			safe.Password = "fixture-secret"
			return safe, nil
		})
		info, err := m.Connect(req)
		if err != nil {
			t.Fatal(err)
		}
		if info.SSHAutoContinueNotices != enabled {
			t.Fatal("new session omitted its notice preference")
		}
		awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
		if first := <-requests; first.SSHAutoContinueNotices != enabled {
			t.Fatal("transport did not receive the selected notice preference")
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		next, err := m.Reconnect(ctx, info.ID)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		if next.ID != info.ID || next.SSHAutoContinueNotices != enabled {
			t.Fatal("reconnect lost the session identity or notice preference")
		}
		awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
		if second := <-requests; second.SSHAutoContinueNotices != enabled {
			t.Fatal("reconnect changed the selected notice preference")
		}
		batch, err := m.Events(context.Background(), 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(batch.Sessions) != 1 || batch.Sessions[0].SSHAutoContinueNotices != enabled {
			t.Fatal("restored session snapshot omitted its notice preference")
		}
		m.mu.Lock()
		retained := m.sessions[info.ID].request
		m.mu.Unlock()
		if retained.SSHAutoContinueNotices != enabled || retained.Password != "" || retained.PrivateKey != "" || retained.Passphrase != "" {
			t.Fatal("retained reconnect settings lost the notice policy or included secrets")
		}
		if err := m.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSSHNoticeServerHeadingBoundsAndAccounting(t *testing.T) {
	for _, raw := range []string{strings.Repeat("x", 1025), strings.Repeat("界", 400), strings.Repeat("\xff", 1025)} {
		heading := sshAuthenticationServerName(raw, ConnectRequest{})
		if len(heading) > 1024 || !utf8.ValidString(heading) || !strings.HasSuffix(heading, "[truncated]") {
			t.Fatal("server heading was not safely and visibly truncated")
		}
		if err := validatePrompt(Prompt{ServerName: heading}); err != nil {
			t.Fatalf("bounded display heading rejected: %v", err)
		}
	}
	for _, length := range []int{1024, 1025} {
		prompt := Prompt{ServerName: strings.Repeat("x", length)}
		if err := validatePrompt(prompt); (err == nil) != (length == 1024) {
			t.Fatalf("server heading limit not enforced at %d bytes", length)
		}
	}
	base := Event{Prompt: &Prompt{}, Session: &SessionInfo{Prompt: &Prompt{}}}
	withHeadings := cloneEvent(base)
	withHeadings.Prompt.ServerName = strings.Repeat("x", 1024)
	withHeadings.Session.Prompt.ServerName = strings.Repeat("y", 1024)
	if eventSize(withHeadings)-eventSize(base) != 2048 {
		t.Fatal("event queue accounting omitted standalone or session server headings")
	}
}

func TestSSHNoticeNewServerHeadingRedactsCredentialsAndControls(t *testing.T) {
	req := ConnectRequest{Password: "fixture-login-secret", Passphrase: "fixture-key-passphrase", PrivateKey: "fixture-private-key"}
	raw := "Fixture\r\nname\x1b]52;c;payload\a\u0085\u009b\u202e " + req.Password + " " + req.Passphrase + " " + req.PrivateKey
	heading := sshAuthenticationServerName(raw, req)
	for _, secret := range []string{req.Password, req.Passphrase, req.PrivateKey} {
		if strings.Contains(heading, secret) {
			t.Fatal("new server heading exposed a held credential")
		}
	}
	for _, r := range heading {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			t.Fatal("new server heading preserved a control or formatting character")
		}
	}
	if !strings.Contains(heading, "Fixture") || !strings.Contains(heading, "name") || !utf8.ValidString(heading) {
		t.Fatal("server heading lost its readable text")
	}
}

func TestSSHNoticeOutputRetainsEventBounds(t *testing.T) {
	// Short overlapping secrets must not recursively expand the redaction label.
	req := ConnectRequest{Password: "a", Passphrase: "e", PrivateKey: "d"}
	redacted := sshAuthenticationNoticeOutput("Fixture", strings.Repeat("a", 16*1024), req)
	if len(redacted) > 16*1024*11 {
		t.Fatal("redaction expanded notice output beyond its single-pass bound")
	}
	// Framing every line can expand a valid instruction, but publication still
	// uses the same bounded chunks and event ring as ordinary terminal output.
	notice := sshAuthenticationNoticeOutput(strings.Repeat("x", 1024), "x"+strings.Repeat("\n", 16*1024-1), ConnectRequest{})
	if len(notice) > 16*1024*20+2048 {
		t.Fatal("notice framing exceeded its bound for a valid instruction")
	}
	m := testManager(t, nil)
	session := newLiveSession(context.Background(), "fixture-notice-output", terminalRequest())
	defer session.cancel()
	for range 16 {
		m.output(session, notice)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ringBytes > m.eventBudget || len(m.ring)-m.ringStart > maxEvents {
		t.Fatal("notice publication exceeded the event queue budget")
	}
	for _, stored := range m.ring[m.ringStart:] {
		if len(stored.event.Data) > maxOutputChunk {
			t.Fatal("notice output bypassed bounded event chunks")
		}
	}
}
