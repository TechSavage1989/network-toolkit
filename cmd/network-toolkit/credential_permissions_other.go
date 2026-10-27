//go:build !windows

package main

import "os"

// Only isolated synthetic test providers can reach this on unsupported systems.
func secureCredentialTempFile(file *os.File) error { return file.Chmod(0600) }
