package main

import (
	"context"
	"os"
	"strings"
	"time"
)

// A browser window shows the interface where there is no native window: the
// Linux edition always, and Windows when the Microsoft Edge WebView2 Runtime
// is not installed (owner decision after v1.21.72: Windows Server does not get
// the runtime, even with Edge installed). The page is opened through a one-time
// launch link (browserLaunchGate), so only that window receives the token.

// appLaunchGate is set by main when the interface opens in a browser window.
var appLaunchGate *browserLaunchGate

func setAppLaunchGate(g *browserLaunchGate) { appLaunchGate = g }

// appLaunchURL turns the app's address into a fresh one-time launch link.
func appLaunchURL(base string) string {
	if appLaunchGate == nil {
		return base
	}
	return strings.TrimSuffix(base, "/") + appLaunchGate.Ticket()
}

// The page polls the engine about every 2 seconds while open. After the
// window process exits, a page that is still polling lives in another browser
// process; a closed window goes quiet.
const pageQuietAfterExit = 8 * time.Second

func pageStillOpen(ctx context.Context, exited time.Time, wait time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(wait):
	}
	return time.Unix(0, lastPageRequest.Load()).After(exited.Add(wait / 4))
}

// windowsBrowserArgs opens a dedicated Edge or Chrome app window with its own
// profile: no sync, extensions, first-run pages or background mode, so the
// browser process ends when its window closes. Browser protections stay on.
func windowsBrowserArgs(url, profile string) []string {
	return []string{
		"--app=" + url,
		"--user-data-dir=" + profile,
		"--new-window",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-sync",
		"--disable-extensions",
		"--disable-default-apps",
		"--disable-background-mode",
		"--window-size=1280,860",
	}
}

// windowModeChoice reads NETWORK_TOOLKIT_WINDOW: "browser" opens a browser
// window even with WebView2 installed, "webview2" never does; anything else
// leaves the choice to what is installed.
func windowModeChoice() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("NETWORK_TOOLKIT_WINDOW"))) {
	case "browser":
		return "browser"
	case "webview2":
		return "webview2"
	}
	return ""
}

// useBrowserWindow decides, once at start, how a Windows window opens.
func useBrowserWindow(runtimeInstalled func() bool, browserFound func() bool) bool {
	switch windowModeChoice() {
	case "webview2":
		return false
	case "browser":
		return browserFound()
	}
	return !runtimeInstalled() && browserFound()
}
