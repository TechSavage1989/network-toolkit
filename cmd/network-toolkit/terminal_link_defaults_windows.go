//go:build windows

package main

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
)

func terminalLinkRegistrationState() terminalLinkRegistration {
	state := terminalLinkRegistration{Supported: true}
	if terminalLinkRegistryString(`Software\RegisteredApplications`, terminalLinkAppName) != terminalLinkCapabilities {
		return state
	}
	command := terminalLinkRegistryString(`Software\Classes\`+terminalLinkProgID+`\shell\open\command`, "")
	state.Registered = command != ""
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		state.Current = terminalLinkCommandIsCurrent(command, exe)
	}
	return state
}

// Read one per-user string value; any absence or error reads as "".
func terminalLinkRegistryString(path, name string) string {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	var key syscall.Handle
	if syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, p, 0, syscall.KEY_QUERY_VALUE, &key) != nil {
		return ""
	}
	defer syscall.RegCloseKey(key)
	values, err := localRegistryStrings(key, name)
	if err != nil || len(values) != 1 {
		return ""
	}
	return values[0]
}

func openTerminalLinkSettings(ctx context.Context) error {
	// Fixed Settings URIs only. Windows versions that do not know the
	// registeredAppUser page open the general Default apps page instead.
	if err := shellExecuteWindowsOpen(ctx, terminalLinkDefaultApps, nil); err == nil {
		return nil
	}
	return shellExecuteWindowsOpen(ctx, "ms-settings:defaultapps", nil)
}
