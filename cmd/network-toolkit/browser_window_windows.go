//go:build windows

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"networktoolkit/internal/desktop"
)

// findWindowsBrowser returns Edge, else Chrome, from their standard folders
// or the App Paths entries their installers write. NETWORK_TOOLKIT_BROWSER may
// name an executable explicitly (hosted checks use a stand-in).
func findWindowsBrowser() (string, bool) {
	if custom := strings.TrimSpace(os.Getenv("NETWORK_TOOLKIT_BROWSER")); custom != "" {
		if info, err := os.Stat(custom); err == nil && !info.IsDir() && filepath.IsAbs(custom) {
			return custom, true
		}
	}
	for _, candidate := range windowsBrowserCandidates() {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

func windowsBrowserCandidates() []string {
	var standard []string
	for _, relative := range []string{filepath.Join("Microsoft", "Edge", "Application", "msedge.exe"), filepath.Join("Google", "Chrome", "Application", "chrome.exe")} {
		for _, root := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles"), os.Getenv("LOCALAPPDATA")} {
			if root != "" && filepath.IsAbs(root) {
				standard = append(standard, filepath.Join(root, relative))
			}
		}
	}
	var registered []string
	for _, exe := range []string{"msedge.exe", "chrome.exe"} {
		for _, root := range []syscall.Handle{syscall.HKEY_LOCAL_MACHINE, syscall.HKEY_CURRENT_USER} {
			if path := appPathEntry(root, exe); path != "" {
				registered = append(registered, path)
			}
		}
	}
	result := append([]string{}, standard...)
	for _, path := range registered {
		known := false
		for _, existing := range result {
			if strings.EqualFold(filepath.Clean(existing), filepath.Clean(path)) {
				known = true
				break
			}
		}
		if !known {
			result = append(result, path)
		}
	}
	return result
}

// appPathEntry reads the default value of App Paths\<exe>; only an absolute
// path to a file of that very name is accepted.
func appPathEntry(root syscall.Handle, exe string) string {
	name, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\App Paths\` + exe)
	var key syscall.Handle
	if syscall.RegOpenKeyEx(root, name, 0, syscall.KEY_QUERY_VALUE, &key) != nil {
		return ""
	}
	defer syscall.RegCloseKey(key)
	buffer := make([]uint16, 1024)
	size, kind := uint32(len(buffer)*2), uint32(0)
	if syscall.RegQueryValueEx(key, nil, nil, &kind, (*byte)(unsafe.Pointer(&buffer[0])), &size) != nil || kind != syscall.REG_SZ {
		return ""
	}
	path := filepath.Clean(strings.Trim(strings.TrimSpace(syscall.UTF16ToString(buffer)), `"`))
	if !filepath.IsAbs(path) || !strings.EqualFold(filepath.Base(path), exe) {
		return ""
	}
	return path
}

func decideBrowserWindow() bool {
	return useBrowserWindow(desktop.WebViewRuntimeAvailable, func() bool { _, ok := findWindowsBrowser(); return ok })
}

// runBrowserWindow shows the interface in an Edge or Chrome app window and
// returns when it closes (then the app stops, as after closing the native
// window) or when the app stops (then it closes the window). It reports false
// when no window could be started, so the caller shows the native explanation.
func runBrowserWindow(ctx context.Context, url string, stop context.CancelFunc, dataDir string, activate <-chan struct{}, onClosing, onReady func()) bool {
	browser, found := findWindowsBrowser()
	if !found {
		return false
	}
	// Inside the WebView2 profile folder, which moving and deleting app data
	// already cover (storageBrowserDirectory).
	profile := filepath.Join(dataDir, storageBrowserDirectory, "BrowserWindow")
	if err := os.MkdirAll(profile, 0o700); err != nil {
		return false
	}
	cmd := exec.Command(browser, windowsBrowserArgs(appLaunchURL(url), profile)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		return false
	}
	setAppUIHost("browser-window")
	if onReady != nil {
		onReady()
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	for {
		select {
		case <-ctx.Done():
			// The app was quit from its own window: close that window too.
			_ = cmd.Process.Kill()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
			}
			return true
		case <-activate:
			// The window is already open; a second start does not open another.
		case <-done:
			if pageStillOpen(ctx, time.Now(), pageQuietAfterExit) {
				// The browser handed the window to another of its processes (one
				// left open by an earlier run), so its close cannot be observed.
				// Keep running; the page offers Quit Network Toolkit.
				setAppUIHost("browser-tab")
				<-ctx.Done()
				return true
			}
			if onClosing != nil {
				onClosing()
			}
			stop()
			return true
		}
	}
}
