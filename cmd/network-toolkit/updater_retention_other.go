//go:build !windows

package main

import "sync"

var updateRetentionTestLock sync.Mutex

func lockUpdateRetentionDirectory(string) (func(), error) {
	updateRetentionTestLock.Lock()
	return updateRetentionTestLock.Unlock, nil
}
