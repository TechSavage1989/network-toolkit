//go:build !windows

package main

import "os"

// Only isolated synthetic test providers can reach this on unsupported systems.
func createSecureCredentialTempFile(dir string) (*os.File, error) {
	return os.CreateTemp(dir, ".network-toolkit-credential-*")
}

func createSecureCredentialFileAt(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
}
