package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"
)

var ErrAppStorageCleanupPending = errors.New("The browser is still releasing app data. Deletion is saved and will be retried before the next workspace opens.")

const storageDeleteRetryCount = 30
const storageDeleteRetryDelay = 100 * time.Millisecond

// The storage owner closes every window and writer before cleanup. WebView2's
// browser process can release its profile handles shortly after controller
// Close returns, so sharing/lock violations alone receive a bounded retry.
// https://learn.microsoft.com/microsoft-edge/webview2/concepts/process-related-events
// Injected operations let regression tests model Windows locks using only
// temporary fixture directories, without opening a real browser profile.
type storageDeleteOps struct {
	cacheLstat func(*storageRoot, string) (os.FileInfo, error)
	retryable  func(error) bool
	wait       func(time.Duration)
}

func defaultStorageDeleteOps() storageDeleteOps {
	return storageDeleteOps{
		cacheLstat: func(root *storageRoot, name string) (os.FileInfo, error) { return root.Lstat(name) },
		retryable:  func(err error) bool { return runtime.GOOS == "windows" && storageWindowsSharingViolation(err) },
		wait:       time.Sleep,
	}
}

func storageWindowsSharingViolation(err error) bool {
	// Match native codes through PathError/wrapping, never localized text.
	// Access denied, invalid paths, links and other failures are not retried.
	return errors.Is(err, syscall.Errno(32)) || errors.Is(err, syscall.Errno(33))
}

func (ops storageDeleteOps) inspectLive(root *storageRoot) error {
	budget := storageTreeEntryLimit
	return walkStorageData(root, storageWalkOptions{live: true, cacheLstat: ops.cacheLstat}, &budget)
}

func (ops storageDeleteOps) cleanup(root *storageRoot) error {
	for attempt := 0; ; attempt++ {
		err := deleteStorageDataOnce(root, ops)
		if err == nil || !ops.retryable(err) {
			return err
		}
		if attempt == storageDeleteRetryCount {
			// The durable pending marker belongs to the caller and is removed
			// only after complete success. Startup still refuses to open a new
			// profile until this same strict cleanup has succeeded.
			return fmt.Errorf("%w %w", ErrAppStorageCleanupPending, err)
		}
		ops.wait(storageDeleteRetryDelay)
	}
}
