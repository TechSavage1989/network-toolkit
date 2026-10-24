//go:build !windows

package main

import (
	"context"
	"os"

	"networktoolkit/internal/platform"
)

func runLifecycle(ctx context.Context, url string, stop context.CancelFunc, noBrowser bool, dataDir, title string, activate <-chan struct{}, onClosing func()) {
	if !noBrowser {
		if err := platform.OpenBrowser(url); err != nil {
			platform.NotifyFatal("Unable to open the development interface: " + err.Error())
			stop()
		}
	}
	<-ctx.Done()
}
func replaceSettingsFile(source, target string) error { return os.Rename(source, target) }
