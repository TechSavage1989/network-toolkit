//go:build windows

package main

import (
	"context"
	"strings"
	"syscall"
	"unsafe"

	"networktoolkit/internal/desktop"
	"networktoolkit/internal/platform"
)

func runLifecycle(ctx context.Context, url, sessionToken string, stop context.CancelFunc, noBrowser bool, dataDir, title string, activate <-chan struct{}, onClosing, onReady func()) {
	if noBrowser {
		<-ctx.Done()
		return
	}
	defer stop()
	err := desktop.Run(ctx, desktop.Config{
		Title:                title,
		MinWidth:             minimumDesktopWidth(title),
		MinHeight:            minimumDesktopHeight(title),
		RequireCloseApproval: mainEditorCloseApproval(title),
		URL:                  url,
		SessionToken:         sessionToken,
		DataDir:              dataDir,
		Stop:                 stop,
		Activate:             activate,
		OnClosing:            onClosing,
		OnReady:              onReady,
	})
	if err != nil {
		platform.NotifyFatal("Unable to open Network Toolkit.\n\n" + err.Error())
	}
}

func replaceSettingsFile(source, target string) error {
	a, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	b, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	r, _, e := syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW").Call(uintptr(unsafe.Pointer(a)), uintptr(unsafe.Pointer(b)), 1|8)
	if r == 0 {
		return e
	}
	return nil
}

// Detached Terminal and Editor windows use the smaller approved minimum.
// Keep the main diagnostics workspace at 960 by 600 logical pixels.
func detachedDesktopTitle(title string) bool {
	return strings.Contains(strings.ToLower(title), "terminal") || strings.Contains(strings.ToLower(title), "editor")
}
func minimumDesktopWidth(title string) int32 {
	if detachedDesktopTitle(title) { return 640 }
	return 960
}
func minimumDesktopHeight(title string) int32 {
	if detachedDesktopTitle(title) { return 400 }
	return 600
}
