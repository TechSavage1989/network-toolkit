//go:build !windows

package main

// Registering the Text Editor with Windows exists only on Windows; the Linux
// edition shows no such section.
func editorRegDefaultPlatform() (editorRegPlatform, bool) {
	return editorRegPlatform{}, false
}
