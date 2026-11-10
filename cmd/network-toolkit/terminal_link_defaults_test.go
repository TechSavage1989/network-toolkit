package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTerminalLinkDefaultsOnlyReportAndOpenSettings(t *testing.T) {
	a := testApplication(t)
	opened := 0
	state := terminalLinkRegistration{Supported: true, Registered: true, Current: true}
	readTerminalLinkRegistration = func() terminalLinkRegistration { return state }
	openTerminalLinkDefaultApps = func(context.Context) error { opened++; return nil }
	t.Cleanup(func() {
		readTerminalLinkRegistration = terminalLinkRegistrationState
		openTerminalLinkDefaultApps = openTerminalLinkSettings
	})
	if r := requestApp(a, "GET", "/api/terminal-links", "", false); r.Code != 401 {
		t.Fatalf("Unauthenticated link status: %d", r.Code)
	}
	r := requestApp(a, "GET", "/api/terminal-links", "", true)
	var got terminalLinkRegistration
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &got) != nil || got != state {
		t.Fatalf("Link status: %d %s", r.Code, r.Body.String())
	}
	if r := requestApp(a, "POST", "/api/terminal-links", "{}", true); r.Code != 405 {
		t.Fatalf("Link status accepted POST: %d", r.Code)
	}
	if r := requestApp(a, "GET", "/api/terminal-links/default-apps", "", true); r.Code != 405 || opened != 0 {
		t.Fatalf("Default apps opened without an explicit POST: %d", r.Code)
	}
	if r := requestApp(a, "POST", "/api/terminal-links/default-apps", `{"uri":"calc.exe"}`, true); r.Code != 200 || opened != 1 {
		t.Fatalf("Default apps request: %d %s", r.Code, r.Body.String())
	}
	openTerminalLinkDefaultApps = func(context.Context) error { return errors.New("fixture failure") }
	if r := requestApp(a, "POST", "/api/terminal-links/default-apps", "", true); r.Code != 503 || !strings.Contains(r.Body.String(), "fixture failure") {
		t.Fatalf("Default apps failure: %d %s", r.Code, r.Body.String())
	}
	state = terminalLinkRegistration{}
	if r := requestApp(a, "POST", "/api/terminal-links/default-apps", "", true); r.Code != 501 {
		t.Fatalf("Unsupported system opened Default apps: %d", r.Code)
	}
	if r := requestApp(a, "GET", "/api/terminal-links/other", "", true); r.Code != 404 {
		t.Fatalf("Unknown link route: %d", r.Code)
	}
	if runtime.GOOS != "windows" {
		if got := terminalLinkRegistrationState(); got.Supported || got.Registered {
			t.Fatalf("Non-Windows link registration: %+v", got)
		}
	}
}

// The installer and the app must agree on the names Windows Default apps uses.
func TestTerminalLinkRegistrationMatchesInstaller(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "scripts", "installer", "windows.nsi"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(b)
	for _, want := range []string{
		`!define TERMINAL_NAME "${APP_NAME} Terminal"`,
		`!define TERMINAL_KEY "Software\${APP_NAME}\Terminal"`,
		`!define TERMINAL_CAPABILITIES "${TERMINAL_KEY}\Capabilities"`,
		`!define LINK_PROG_ID "` + terminalLinkProgID + `"`,
		`'"$INSTDIR\${APP_FILENAME}" --terminal --open-url "%1"'`,
		`WriteRegStr HKCU "Software\RegisteredApplications" "${TERMINAL_NAME}" "${TERMINAL_CAPABILITIES}"`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("windows.nsi does not contain %s", want)
		}
	}
	if terminalLinkAppName != "Network Toolkit Terminal" || terminalLinkCapabilities != `Software\Network Toolkit\Terminal\Capabilities` || !strings.HasSuffix(terminalLinkDefaultApps, "registeredAppUser=Network%20Toolkit%20Terminal") {
		t.Fatal("The app's link registration names changed without the installer")
	}
	// Setup must never claim a scheme's default or another app's handler.
	for _, line := range strings.Split(script, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "WriteReg") && !strings.HasPrefix(line, "DeleteReg") {
			continue
		}
		for _, forbidden := range []string{`UserChoice`, `UrlAssociations`, `Software\Classes\ssh`, `Software\Classes\telnet`, `Software\Classes\serial`, `Software\Classes\${SCHEME}\shell`} {
			if strings.Contains(line, forbidden) {
				t.Fatalf("windows.nsi changes %s: %s", forbidden, line)
			}
		}
	}
}
