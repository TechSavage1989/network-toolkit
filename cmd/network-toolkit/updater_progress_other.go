//go:build !windows

package main

func startUpdateProgressWindow(string) updateProgressWindow { return silentUpdateProgress{} }
