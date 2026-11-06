//go:build linux

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"networktoolkit/internal/platform"
)

// The Linux edition shows its interface in an app-mode window of an installed
// Chromium-family browser (owner decision, docs/LINUX_PLAN.md). The window
// uses its own browser profile under the app-data folder, so it is a separate
// browser process: when the user closes it, the process exits and the app
// stops, as closing the Windows window does. If no such browser is installed,
// the interface opens in the default browser and the app keeps running until
// the user chooses Quit Network Toolkit (a tab's close cannot be observed).
//
// The page is opened through a one-time launch link (browserLaunchGate), so the
// session token is never served to other local processes or users.

// linuxLaunchGate is set by main for the Linux window mode.
var linuxLaunchGate *browserLaunchGate

type appBrowser struct {
	command []string // executable and any wrapper arguments (flatpak run ID)
	name    string
	kind    string // "native", "snap" or "flatpak"
	id      string // snap name or flatpak application ID
}

var appBrowserCandidates = []string{"google-chrome-stable", "google-chrome", "chromium", "chromium-browser", "microsoft-edge-stable", "microsoft-edge", "brave-browser", "brave", "vivaldi-stable", "vivaldi"}

var appBrowserFlatpaks = []string{"com.google.Chrome", "org.chromium.Chromium", "com.microsoft.Edge", "com.brave.Browser", "com.vivaldi.Vivaldi"}

// findAppBrowser picks the browser for the app window. NETWORK_TOOLKIT_BROWSER
// may name an executable explicitly.
func findAppBrowser(lookPath func(string) (string, error), resolve func(string) (string, error), flatpakInstalled func(string) bool) (appBrowser, bool) {
	if custom := strings.TrimSpace(os.Getenv("NETWORK_TOOLKIT_BROWSER")); custom != "" {
		if path, err := lookPath(custom); err == nil {
			return classifyAppBrowser(path, resolve), true
		}
	}
	for _, name := range appBrowserCandidates {
		if path, err := lookPath(name); err == nil {
			return classifyAppBrowser(path, resolve), true
		}
	}
	if path, err := lookPath("flatpak"); err == nil {
		for _, id := range appBrowserFlatpaks {
			if flatpakInstalled(id) {
				return appBrowser{command: []string{path, "run", id}, name: id, kind: "flatpak", id: id}, true
			}
		}
	}
	return appBrowser{}, false
}

func classifyAppBrowser(path string, resolve func(string) (string, error)) appBrowser {
	b := appBrowser{command: []string{path}, name: filepath.Base(path), kind: "native"}
	real := path
	if r, err := resolve(path); err == nil {
		real = r
	}
	// Snap browsers live under /snap and can only use profile folders inside
	// ~/snap/<name>/; a hidden folder elsewhere in the home is blocked.
	if strings.HasPrefix(path, "/snap/") || strings.HasPrefix(real, "/snap/") || strings.Contains(real, "/usr/bin/snap") {
		b.kind = "snap"
		b.id = filepath.Base(path)
		if b.id == "chromium-browser" {
			b.id = "chromium"
		}
	}
	return b
}

// appWindowProfile is the browser profile folder for the app window.
func appWindowProfile(b appBrowser, dataDir, home string) string {
	switch b.kind {
	case "snap":
		return filepath.Join(home, "snap", b.id, "common", "network-toolkit-window")
	case "flatpak":
		return filepath.Join(home, ".var", "app", b.id, "data", "network-toolkit-window")
	}
	return filepath.Join(dataDir, "browser-window")
}

func appWindowArgs(b appBrowser, url, profile string) []string {
	args := append([]string{}, b.command...)
	return append(args,
		"--app="+url,
		"--user-data-dir="+profile,
		"--class=NetworkToolkit",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-sync",
		"--window-size=1280,860",
	)
}

func flatpakAppInstalled(id string) bool {
	path, err := exec.LookPath("flatpak")
	if err != nil {
		return false
	}
	return exec.Command(path, "info", id).Run() == nil
}

func linuxLaunchURL(base string) string {
	if linuxLaunchGate == nil {
		return base
	}
	return strings.TrimSuffix(base, "/") + linuxLaunchGate.Ticket()
}

func runLifecycle(ctx context.Context, url, sessionToken string, stop context.CancelFunc, noBrowser bool, dataDir, title string, activate <-chan struct{}, onClosing, onReady func()) {
	if noBrowser {
		<-ctx.Done()
		return
	}
	browser, found := findAppBrowser(exec.LookPath, filepath.EvalSymlinks, flatpakAppInstalled)
	if !found || os.Geteuid() == 0 && strings.TrimSpace(os.Getenv("NETWORK_TOOLKIT_BROWSER")) == "" {
		// Chromium-family browsers refuse to run as root without disabling
		// their sandbox, which the app never does. An explicitly chosen
		// browser (NETWORK_TOOLKIT_BROWSER) is still started, sandbox on.
		runBrowserTab(ctx, url, stop, activate, onReady)
		return
	}
	home, _ := os.UserHomeDir()
	profile := appWindowProfile(browser, dataDir, home)
	if err := os.MkdirAll(profile, 0o700); err != nil {
		runBrowserTab(ctx, url, stop, activate, onReady)
		return
	}
	cmd := exec.Command(browser.command[0], appWindowArgs(browser, linuxLaunchURL(url), profile)[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		runBrowserTab(ctx, url, stop, activate, onReady)
		return
	}
	setAppUIHost("linux-window")
	if onReady != nil {
		onReady()
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	for {
		select {
		case <-ctx.Done():
			// The app was quit from its own window: close that window too.
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				<-done
			}
			return
		case <-activate:
			// The window is already open; a second start must not open another
			// copy of the same workspace.
		case <-done:
			if pageStillOpen(ctx, time.Now(), pageQuietAfterExit) {
				// The browser handed the window to another of its processes
				// (for example a window left open by an earlier run), so this
				// window's close cannot be observed. Keep running like a tab.
				setAppUIHost("browser-tab")
				waitBrowserTab(ctx, url, activate)
				return
			}
			if onClosing != nil {
				onClosing()
			}
			stop()
			return
		}
	}
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

func runBrowserTab(ctx context.Context, url string, stop context.CancelFunc, activate <-chan struct{}, onReady func()) {
	setAppUIHost("browser-tab")
	launch := linuxLaunchURL(url)
	if err := platform.OpenBrowser(launch); err != nil {
		platform.NotifyFatal("No browser could be opened. Open this address in a browser on this PC within two minutes: " + launch)
	}
	fmt.Fprintln(os.Stderr, "Network Toolkit is running. Choose Quit Network Toolkit in its window to stop it.")
	if onReady != nil {
		onReady()
	}
	waitBrowserTab(ctx, url, activate)
}

func waitBrowserTab(ctx context.Context, url string, activate <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-activate:
			_ = platform.OpenBrowser(linuxLaunchURL(url))
		}
	}
}

func replaceSettingsFile(source, target string) error { return os.Rename(source, target) }
