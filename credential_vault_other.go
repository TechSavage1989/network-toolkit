//go:build !windows

package main

// The shipped application is Windows-only. Development and unit-test builds on
// other systems use an identity transform so the vault API can be exercised.
func protectCredentialBlob(value []byte) ([]byte, error) { return append([]byte(nil), value...), nil }
func unprotectCredentialBlob(value []byte) ([]byte, error) { return append([]byte(nil), value...), nil }
