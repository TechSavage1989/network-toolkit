//go:build !windows

package main

// Windows Shell shortcuts are deliberately not inspected on other platforms.
func updateMigrateShortcuts(oldPaths []string, newPath, settingsRoot string) error { return nil }
