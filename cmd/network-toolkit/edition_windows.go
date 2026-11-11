//go:build windows

package main

// programWritableByOthers names who, besides this user and the system, can
// change a program or its folder (detachedViewProgram).
func programWritableByOthers(path string) (string, error) { return editorRegWritableByOthers(path) }
