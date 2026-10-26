//go:build !windows

package main

import (
	"os"
	"sync"
)

// Name management is Windows-only at runtime. A process lock makes portable
// unit tests deterministic without introducing persistent lock files.
var updateNamingTestLock sync.Mutex

func lockUpdateNamingDirectory(string) (func(), error) {
	updateNamingTestLock.Lock()
	return updateNamingTestLock.Unlock, nil
}

func moveUpdateNamingNoReplace(source, target string) error {
	// link is atomic and fails if target exists, unlike Unix rename.
	if err := os.Link(source, target); err != nil {
		return err
	}
	return os.Remove(source)
}
