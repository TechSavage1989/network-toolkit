package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestConnectionOpenCanonicalDestinations(t *testing.T) {
	for _, test := range []struct {
		protocol, host string
		port           int
		wantHost       string
		wantPort       int
		wantTarget     string
	}{
		{"http", "192.0.2.10", 0, "192.0.2.10", 80, "http://192.0.2.10/"},
		{"http", "switch.example", 8080, "switch.example", 8080, "http://switch.example:8080/"},
		{"https", "ROUTER.Example", 0, "router.example", 443, "https://router.example/"},
		{"https", "router.example.", 443, "router.example.", 443, "https://router.example./"},
		{"https", "xn--bcher-kva.example", 9443, "xn--bcher-kva.example", 9443, "https://xn--bcher-kva.example:9443/"},
		{"http", "2001:db8::1", 0, "2001:db8::1", 80, "http://[2001:db8::1]/"},
		{"https", "[2001:0DB8:0:0::1]", 8443, "2001:db8::1", 8443, "https://[2001:db8::1]:8443/"},
		{"https", "[::ffff:192.0.2.10]", 0, "192.0.2.10", 443, "https://192.0.2.10/"},
		{"rdp", "desktop-12", 0, "desktop-12", 3389, "desktop-12:3389"},
		{"rdp", "192.0.2.20", 3390, "192.0.2.20", 3390, "192.0.2.20:3390"},
		{"rdp", "fd00::25", 3389, "fd00::25", 3389, "[fd00::25]:3389"},
		{"rdp", "[2001:db8::25]", 65535, "2001:db8::25", 65535, "[2001:db8::25]:65535"},
		{"http", "localhost", 1, "localhost", 1, "http://localhost:1/"},
	} {
		t.Run(test.protocol+test.host, func(t *testing.T) {
			target, err := buildConnectionOpenTarget(connectionOpenRequest{Protocol: test.protocol, Host: test.host, Port: test.port})
			if err != nil {
				t.Fatal(err)
			}
			if target.Protocol != test.protocol || target.Host != test.wantHost || target.Port != test.wantPort || target.Target != test.wantTarget {
				t.Fatalf("target %#v, want %s", target, test.wantTarget)
			}
			if target.Protocol != "rdp" {
				parsed, err := url.Parse(target.Target)
				if err != nil || parsed.Scheme != target.Protocol || parsed.Hostname() != target.Host || parsed.User != nil || parsed.Path != "/" || parsed.RawQuery != "" || parsed.Fragment != "" {
					t.Fatalf("browser destination changed authority or added data: %#v, %v", parsed, err)
				}
			}
		})
	}
}

func TestConnectionOpenRejectsUnsafeAndAmbiguousHosts(t *testing.T) {
	for _, host := range []string{
		"", " router.example", "router.example ", "router example", "router\texample", "router\nexample", "router\r\nHost:evil", "router\x00example", "router\u00a0example",
		"http://router", "https://router/path", "file:///C:/Windows/notepad.exe", "user@router", "router/path", "router\\path", "router?query", "router#fragment", "router:443", "[::1]:443",
		"-option", "/admin", "router /admin", "router\" /admin", "router;command", "router&command", "router|command", "router$(command)", "%PATH%", "router%2fpath",
		"[router]", "[192.0.2.1]", "[[::1]]", "[::1", "::1]", "[2001:db8::zz]", "fe80::1%12", "[fe80::1%Ethernet]", "[fe80::1%2512]",
		"0.0.0.0", "::", "[::ffff:0.0.0.0]", "255.255.255.255", "::ffff:255.255.255.255", "224.0.0.1", "::ffff:224.0.0.1", "ff02::1",
		"bad..name", ".name", "-bad.name", "bad-.name", "bad_name", "bücher.example", strings.Repeat("a", 64) + ".example", strings.Repeat("a.", 127) + "a",
		"2130706433", "127.1", "127.0.1", "0x7f000001", "0X7F000001", "0x", "0x7f.1", "0177.0.0.1", "127.0.0.01", "999.1.1.1", "192.0.2.1.", "router.123",
	} {
		for _, protocol := range []string{"http", "https", "rdp"} {
			if target, err := buildConnectionOpenTarget(connectionOpenRequest{Protocol: protocol, Host: host}); err == nil {
				t.Fatalf("accepted %s host %q as %#v", protocol, host, target)
			}
		}
	}
	for _, request := range []connectionOpenRequest{
		{Host: "router.example"}, {Protocol: "HTTP", Host: "router.example"}, {Protocol: "ssh", Host: "router.example"},
		{Protocol: "http", Host: "router.example", Port: -1}, {Protocol: "rdp", Host: "router.example", Port: 65536},
	} {
		if _, err := buildConnectionOpenTarget(request); err == nil {
			t.Fatalf("accepted invalid request %#v", request)
		}
	}
}

func TestConnectionOpenPlansOnlyBrowserURLOrOneRDPArgument(t *testing.T) {
	const systemClient = `C:\Windows\System32\mstsc.exe`
	for _, protocol := range []string{"http", "https", "rdp"} {
		target, err := buildConnectionOpenTarget(connectionOpenRequest{Protocol: protocol, Host: "2001:db8::25", Port: 8443})
		if err != nil {
			t.Fatal(err)
		}
		command, err := buildConnectionOpenCommand(target, systemClient)
		if err != nil {
			t.Fatal(err)
		}
		if protocol == "rdp" {
			if command.File != systemClient || !reflect.DeepEqual(command.Arguments, []string{"/v:[2001:db8::25]:8443"}) {
				t.Fatalf("unexpected RDP file or options: %#v", command)
			}
			if _, err := buildConnectionOpenCommand(target, ""); err == nil {
				t.Fatal("missing system RDP client accepted")
			}
		} else if command.File != target.Target || command.Arguments != nil {
			t.Fatalf("browser launch must have its URL and no parameter string: %#v", command)
		}
		target.Target = "file:///C:/Windows/notepad.exe"
		if _, err := buildConnectionOpenCommand(target, systemClient); err == nil {
			t.Fatal("altered launch target accepted")
		}
	}
}

func connectionTestRequest(a *application, method, path, body, contentType string, launcher func(context.Context, connectionOpenTarget) error) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://"+a.host+path, strings.NewReader(body))
	r.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	a.handleConnectionOpenWithLauncher(w, r, launcher)
	return w
}

func TestConnectionOpenHTTPAcceptanceAndNativeFailures(t *testing.T) {
	a := testApplication(t)
	for _, protocol := range []string{"http", "https", "rdp"} {
		calls := 0
		var launched connectionOpenTarget
		w := connectionTestRequest(a, "POST", "/api/connections/open", `{"protocol":"`+protocol+`","host":"[2001:db8::25]"}`, "application/json; charset=utf-8", func(ctx context.Context, target connectionOpenTarget) error {
			calls++
			if ctx != a.ctx {
				t.Error("launcher did not receive the app lifetime context")
			}
			launched = target
			return nil
		})
		if w.Code != http.StatusAccepted || calls != 1 {
			t.Fatalf("launch status %d, calls %d: %s", w.Code, calls, w.Body.String())
		}
		var result struct {
			OK       bool   `json:"ok"`
			Protocol string `json:"protocol"`
			Target   string `json:"target"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || !result.OK || result.Protocol != protocol || result.Target != launched.Target {
			t.Fatalf("accepted response mismatch: %s", w.Body.String())
		}
	}
	for _, test := range []struct {
		err  error
		code int
	}{
		{errors.New("fixture: no default handler"), http.StatusInternalServerError},
		{errConnectionLaunchUnavailable, http.StatusNotImplemented},
		{context.Canceled, http.StatusConflict},
	} {
		w := connectionTestRequest(a, "POST", "/api/connections/open", `{"protocol":"http","host":"router.example"}`, "application/json", func(context.Context, connectionOpenTarget) error { return test.err })
		if w.Code != test.code || strings.Contains(w.Body.String(), `"ok":true`) {
			t.Fatalf("native error %v returned %d: %s", test.err, w.Code, w.Body.String())
		}
	}
}

func TestConnectionOpenHTTPRejectsUnsafeEnvelopeBeforeLaunch(t *testing.T) {
	a := testApplication(t)
	launch := func(context.Context, connectionOpenTarget) error {
		t.Fatal("invalid request reached the OS launcher")
		return nil
	}
	for _, body := range []string{
		`null`, `[]`, `{}`, `{"protocol":"http"}`, `{"host":"router"}`,
		`{"protocol":"http","host":"router","password":"secret"}`,
		`{"protocol":"rdp","host":"router","arguments":["/admin"]}`,
		`{"protocol":"http","Host":"router"}`,
		`{"protocol":"http","host":"router","host":"other"}`,
		`{"protocol":"http","host":"router","port":null}`,
		`{"protocol":"http","host":null}`, `{"protocol":null,"host":"router"}`,
		`{"protocol":"http","host":123}`, `{"protocol":"http","host":"router","port":"80"}`,
		`{"protocol":"http","host":"router","port":80.1}`, `{"protocol":"http","host":"router","port":-1}`,
		`{"protocol":"http","host":"router","port":65536}`, `{"protocol":"http","host":"router","port":999999999999999999999999999999}`,
		`{"protocol":"http","host":"router"} {}`, `{"protocol":"http","host":"router"} null`, `{"protocol":"http","host":"router"`,
		`{"protocol":"http","host":"router"}` + strings.Repeat(" ", 4096),
	} {
		w := connectionTestRequest(a, "POST", "/api/connections/open", body, "application/json", launch)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("invalid envelope status %d for %.160s: %s", w.Code, body, w.Body.String())
		}
	}
	for _, contentType := range []string{"", "text/plain", "application/x-www-form-urlencoded"} {
		if w := connectionTestRequest(a, "POST", "/api/connections/open", `{"protocol":"http","host":"router"}`, contentType, launch); w.Code != 400 {
			t.Fatalf("accepted content type %q", contentType)
		}
	}
	if w := connectionTestRequest(a, "GET", "/api/connections/open", "", "application/json", launch); w.Code != 405 {
		t.Fatal("GET could open a connection")
	}
	if w := connectionTestRequest(a, "POST", "/api/connections/open/extra", `{}`, "application/json", launch); w.Code != 404 {
		t.Fatal("unexpected launch route accepted")
	}
}

type connectionClosingReader struct {
	reader io.Reader
	close  func()
}

func (reader *connectionClosingReader) Read(buffer []byte) (int, error) {
	if reader.close != nil {
		reader.close()
		reader.close = nil
	}
	return reader.reader.Read(buffer)
}

func TestConnectionOpenRechecksClosingAfterBodyRead(t *testing.T) {
	a := testApplication(t)
	r := httptest.NewRequest("POST", "http://"+a.host+"/api/connections/open", nil)
	r.Header.Set("Content-Type", "application/json")
	r.Body = io.NopCloser(&connectionClosingReader{
		reader: strings.NewReader(`{"protocol":"rdp","host":"desktop.example"}`),
		close: func() {
			a.views.mu.Lock()
			a.views.homeClosing = true
			a.views.mu.Unlock()
		},
	})
	w := httptest.NewRecorder()
	a.handleConnectionOpenWithLauncher(w, r, func(context.Context, connectionOpenTarget) error {
		t.Fatal("a home closing during body decode launched a client")
		return nil
	})
	if w.Code != http.StatusConflict || a.ctx.Err() != nil {
		t.Fatalf("closing home with live engine status %d: %s", w.Code, w.Body.String())
	}
}

func TestConnectionOpenAuthenticatedHomeScope(t *testing.T) {
	a := testApplication(t)
	valid := `{"protocol":"http","host":"router.example"}`
	if w := requestApp(a, "POST", "/api/connections/open", valid, false); w.Code != 401 {
		t.Fatal("unauthenticated connection launch reached handler")
	}
	id, token := strings.Repeat("a", 32), strings.Repeat("b", 64)
	a.views.mu.Lock()
	a.views.views[id] = &terminalView{info: terminalViewInfo{ID: id, SessionID: "fixture-session", State: "detached"}, token: token, lastPulse: time.Now()}
	a.views.mu.Unlock()
	r := httptest.NewRequest("POST", "http://"+a.host+"/api/connections/open", strings.NewReader(valid))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Session-Token", token)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("detached capability could launch a new client: %d %s", w.Code, w.Body.String())
	}
	// This route exercises production dispatch but never launches an OS client.
	if runtime.GOOS != "windows" {
		if w := requestApp(a, "POST", "/api/connections/open", valid, true); w.Code != http.StatusNotImplemented {
			t.Fatalf("non-Windows production launch status %d: %s", w.Code, w.Body.String())
		}
	}
	a.shutdownRequested.Store(true)
	if w := requestApp(a, "POST", "/api/connections/open", valid, true); w.Code != http.StatusConflict {
		t.Fatal("closing workspace accepted a new client")
	}
}
