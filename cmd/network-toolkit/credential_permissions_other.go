//go:build !windows

package main

import "os"

// Only isolated synthetic test providers can reach this on unsupported systems.
func createSecureCredentialTempFile(dir string) (*os.File, error) {
	return os.CreateTemp(dir, ".network-toolkit-credential-*")
}
