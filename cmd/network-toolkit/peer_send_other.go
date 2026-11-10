//go:build !windows

package main

// Only Windows restricts which process may bring a window to the front.
func peerAllowForeground(int) {}
