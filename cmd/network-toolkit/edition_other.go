//go:build !windows

package main

// programWritableByOthers: only Windows starts a separate program for a
// detached window (detachedViewProgram); elsewhere nothing is checked.
func programWritableByOthers(string) (string, error) { return "", nil }
