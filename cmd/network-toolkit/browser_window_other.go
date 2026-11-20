//go:build !windows

package main

// Only Windows chooses between its native window and a browser window.
func decideBrowserWindow() bool { return false }
