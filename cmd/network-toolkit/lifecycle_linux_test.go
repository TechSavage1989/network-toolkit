//go:build linux

package main

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func fakeLookPath(found map[string]string) func(string) (string, error) {
	return func(name string) (string, error) {
		if path, ok := found[name]; ok {
			return path, nil
		}
		return "", errors.New("not found")
	}
}

func sameResolve(path string) (string, error) { return path, nil }

func TestFindAppBrowserPrefersChromiumFamilyInOrder(t *testing.T) {
	t.Setenv("NETWORK_TOOLKIT_BROWSER", "")
	b, ok := findAppBrowser(fakeLookPath(map[string]string{"brave-browser": "/usr/bin/brave-browser", "chromium": "/usr/bin/chromium"}), sameResolve, func(string) bool { return false })
	if !ok || b.command[0] != "/usr/bin/chromium" || b.kind != "native" {
		t.Fatalf("expected Chromium before Brave, got %#v", b)
	}
	if _, ok := findAppBrowser(fakeLookPath(map[string]string{"firefox": "/usr/bin/firefox"}), sameResolve, func(string) bool { return false }); ok {
		t.Fatal("a browser without app mode was chosen")
	}
	t.Setenv("NETWORK_TOOLKIT_BROWSER", "my-chrome")
	b, ok = findAppBrowser(fakeLookPath(map[string]string{"my-chrome": "/opt/my/chrome", "chromium": "/usr/bin/chromium"}), sameResolve, func(string) bool { return false })
	if !ok || b.command[0] != "/opt/my/chrome" {
		t.Fatalf("NETWORK_TOOLKIT_BROWSER was not honoured: %#v", b)
	}
}

func TestAppBrowserProfilesFollowSnapAndFlatpakConfinement(t *testing.T) {
	t.Setenv("NETWORK_TOOLKIT_BROWSER", "")
	snap, _ := findAppBrowser(fakeLookPath(map[string]string{"chromium-browser": "/usr/bin/chromium-browser"}), func(string) (string, error) { return "/usr/bin/snap", nil }, func(string) bool { return false })
	if snap.kind != "snap" || snap.id != "chromium" {
		t.Fatalf("Ubuntu's snap Chromium not recognised: %#v", snap)
	}
	if got := appWindowProfile(snap, "/home/u/.config/NetworkToolkit", "/home/u"); got != filepath.Join("/home/u", "snap", "chromium", "common", "network-toolkit-window") {
		t.Fatalf("snap profile outside the snap's writable folder: %s", got)
	}
	flat, ok := findAppBrowser(fakeLookPath(map[string]string{"flatpak": "/usr/bin/flatpak"}), sameResolve, func(id string) bool { return id == "com.brave.Browser" })
	if !ok || flat.kind != "flatpak" || !slices.Equal(flat.command, []string{"/usr/bin/flatpak", "run", "com.brave.Browser"}) {
		t.Fatalf("Flatpak browser not used: %#v", flat)
	}
	if got := appWindowProfile(flat, "/d", "/home/u"); !strings.HasPrefix(got, "/home/u/.var/app/com.brave.Browser/") {
		t.Fatalf("flatpak profile outside its sandbox folder: %s", got)
	}
	if got := appWindowProfile(appBrowser{kind: "native"}, "/home/u/.config/NetworkToolkit", "/home/u"); got != "/home/u/.config/NetworkToolkit/browser-window" {
		t.Fatalf("native profile: %s", got)
	}
}

func TestAppWindowArgsOpenADedicatedAppWindow(t *testing.T) {
	args := appWindowArgs(appBrowser{command: []string{"/usr/bin/flatpak", "run", "org.chromium.Chromium"}}, "http://127.0.0.1:4000/launch?ticket=x", "/p")
	for _, want := range []string{"/usr/bin/flatpak", "run", "org.chromium.Chromium", "--app=http://127.0.0.1:4000/launch?ticket=x", "--user-data-dir=/p", "--class=NetworkToolkit", "--no-first-run"} {
		if !slices.Contains(args, want) {
			t.Fatalf("missing %q in %v", want, args)
		}
	}
	for _, arg := range args {
		if strings.Contains(arg, "no-sandbox") || strings.Contains(arg, "disable-web-security") {
			t.Fatalf("the app window must keep the browser's protections: %v", args)
		}
	}
}

func TestLinuxLaunchURLUsesOneTimeTickets(t *testing.T) {
	old := linuxLaunchGate
	t.Cleanup(func() { linuxLaunchGate = old })
	linuxLaunchGate = newBrowserLaunchGate()
	a, b := linuxLaunchURL("http://127.0.0.1:4000/"), linuxLaunchURL("http://127.0.0.1:4000/")
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

func TestDefaultBrowserPreference(t *testing.T) {
	t.Setenv("NETWORK_TOOLKIT_BROWSER", "Default")
	if !preferDefaultBrowser() {
		t.Fatal("NETWORK_TOOLKIT_BROWSER=default was ignored")
	}
	t.Setenv("NETWORK_TOOLKIT_BROWSER", "/usr/bin/chromium")
	if preferDefaultBrowser() {
		t.Fatal("an explicit browser path was treated as the default browser")
	}
}
