//go:build windows

package main

import (
	"context"
	"syscall"
	"unsafe"

	"networktoolkit/internal/desktop"
	"networktoolkit/internal/platform"
)

func runLifecycle(ctx context.Context, url string, stop context.CancelFunc, noBrowser bool, dataDir, title string, activate <-chan struct{}, onClosing, onReady func()) {
	if noBrowser {
		<-ctx.Done()
		return
	}
	defer stop()
	err := desktop.Run(ctx, desktop.Config{
		Title:     title,
		URL:       url,
		DataDir:   dataDir,
		Stop:      stop,
		Activate:  activate,
		OnClosing: onClosing,
		OnReady:   onReady,
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
