//go:build windows

package main

func credentialProtectionLabel() string { return "Windows DPAPI - current user" }
