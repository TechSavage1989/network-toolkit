//go:build !windows

package main

// Setup's Start menu shortcuts exist on Windows only.
func updateCompanionShortcutsNative(dir string) error { return nil }
