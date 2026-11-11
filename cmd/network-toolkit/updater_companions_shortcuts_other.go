//go:build !windows

package main

// Setup's Start menu shortcuts exist on Windows only.
func updateCompanionShortcutsNative(dir string) error { return nil }

// companionLongPath is the path unchanged: short names exist on Windows only.
func companionLongPath(path string) string { return path }
