//go:build !windows

package identity

import "os"

func replaceDatabaseFile(source, destination string) error { return os.Rename(source, destination) }
