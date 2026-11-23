package main

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestAppLaunchURLUsesOneTimeTickets(t *testing.T) {
	old := appLaunchGate
	t.Cleanup(func() { appLaunchGate = old })
	appLaunchGate = newBrowserLaunchGate()
	a, b := appLaunchURL("http://127.0.0.1:4000/"), appLaunchURL("http://127.0.0.1:4000/")
	if !strings.HasPrefix(a, "http://127.0.0.1:4000/launch?ticket=") || a == b {
		t.Fatalf("launch links are not fresh one-time tickets: %q %q", a, b)
	}
}

func TestWindowCloseIsToldApartFromAHandedOffWindow(t *testing.T) {
	exited := time.Now()
	lastPageRequest.Store(exited.Add(-time.Second).UnixNano())
	if pageStillOpen(context.Background(), exited, 40*time.Millisecond) {
		t.Fatal("a page that went quiet when the window closed was treated as still open")
	}
	exited = time.Now()
	go func() { time.Sleep(25 * time.Millisecond); lastPageRequest.Store(time.Now().UnixNano()) }()
	if !pageStillOpen(context.Background(), exited, 60*time.Millisecond) {
		t.Fatal("a page still polling after the browser process exited was treated as closed")
	}
}

// Windows Server has no WebView2 Runtime even with Edge installed (owner
// report on v1.21.72): the window then opens in Edge or Chrome.
func TestWindowsUsesABrowserWindowOnlyWithoutWebView2(t *testing.T) {
	yes, no := func() bool { return true }, func() bool { return false }
	for _, c := range []struct {
		env              string
		runtime, browser func() bool
		want             bool
	}{
		{"", yes, yes, false}, // WebView2 installed: the native window as before
		{"", no, yes, true},   // no runtime, Edge or Chrome: a browser window
		{"", no, no, false},   // neither: the native window explains what to install
		{"browser", yes, yes, true},
		{"Browser", yes, no, false}, // asked for, but no browser to open
		{"webview2", no, yes, false},
	} {
		t.Setenv("NETWORK_TOOLKIT_WINDOW", c.env)
		if got := useBrowserWindow(c.runtime, c.browser); got != c.want {
			t.Errorf("NETWORK_TOOLKIT_WINDOW=%q runtime=%v browser=%v: got %v", c.env, c.runtime(), c.browser(), got)
		}
	}
}

func TestWindowsBrowserWindowIsADedicatedAppWindow(t *testing.T) {
	args := windowsBrowserArgs("http://127.0.0.1:4000/launch?ticket=x", `C:\data\browser-window`)
	joined := strings.Join(args, " ")
	for _, want := range []string{"--app=http://127.0.0.1:4000/launch?ticket=x", `--user-data-dir=C:\data\browser-window`, "--new-window", "--disable-background-mode", "--no-first-run", "--disable-sync", "--disable-extensions", "--disable-component-extensions-with-background-pages", "--disable-notifications", "--disable-features=Translate"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %s in %v", want, args)
		}
	}
	for _, arg := range args {
		if strings.Contains(arg, "no-sandbox") || strings.Contains(arg, "disable-web-security") || strings.Contains(arg, "remote-debugging") {
			t.Fatalf("the window must keep the browser's protections: %v", args)
		}
	}
}

func TestAppBrowserPolicyKeepsClipboardDialogsAndSecurity(t *testing.T) {
	args := appBrowserArgs("http://127.0.0.1:4000/launch?ticket=x", "/private/app profile")
	for _, want := range []string{
		"--app=http://127.0.0.1:4000/launch?ticket=x", "--user-data-dir=/private/app profile",
		"--no-first-run", "--no-default-browser-check", "--disable-sync",
		"--disable-extensions", "--disable-component-extensions-with-background-pages",
		"--disable-default-apps", "--disable-background-mode", "--disable-notifications",
		"--disable-features=Translate",
	} {
		if !slices.Contains(args, want) {
			t.Errorf("application browser policy lacks %q: %v", want, args)
		}
	}
	for _, forbidden := range []string{
		"--no-sandbox", "--disable-web-security", "--ignore-certificate-errors",
		"--disable-background-networking", "--disable-popup-blocking",
		"--deny-permission-prompts", "--disable-external-intent-requests",
		"--disable-javascript", "--enable-automation", "--test-type", "--noerrdialogs",
		"--remote-debugging-port", "--remote-debugging-pipe", "--block-new-web-contents",
	} {
		for _, arg := range args {
			if arg == forbidden || strings.HasPrefix(arg, forbidden+"=") {
				t.Errorf("application browser must preserve supported user actions and protections, found %q", arg)
			}
		}
	}
}

func TestBrowserWindowServesTheTokenOverHTTP(t *testing.T) {
	old := nativeHostBootstrap
	t.Cleanup(func() { nativeHostBootstrap = old })
	nativeHostBootstrap = false
	if got := httpBootstrapToken("abc"); got != "abc" {
		t.Fatalf("a browser window needs the token in its (gated) page: %q", got)
	}
	nativeHostBootstrap = true
	if got := httpBootstrapToken("abc"); got != "__SESSION_TOKEN__" {
		t.Fatalf("the native window keeps the in-memory bootstrap: %q", got)
	}
}
