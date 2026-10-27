//go:build !windows

package main

import "errors"

// Production and development executables fail closed outside Windows. Unit tests
// inject a synthetic encrypted provider into their own temporary vault only.
func credentialProtectionAvailable() bool { return false }
func protectCredentialBlob([]byte) ([]byte, error) {
	return nil, errors.New("Windows credential protection is unavailable")
}
func unprotectCredentialBlob([]byte) ([]byte, error) {
	return nil, errors.New("Windows credential protection is unavailable")
}
