//go:build windows

package desktop

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// Use a child process so no earlier test can consume STARTUPINFO's initial
// ShowWindow override. This exercises the launch mode used by old updaters.
func TestStartupWindowRecoversHiddenLaunch(t *testing.T) {
	if os.Getenv("NETWORK_TOOLKIT_HIDDEN_STARTUP_TEST") != "1" {
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, exe, "-test.run=^TestStartupWindowRecoversHiddenLaunch$")
		cmd.Env = append(os.Environ(), "NETWORK_TOOLKIT_HIDDEN_STARTUP_TEST=1")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("hidden launch did not reveal the native window: %v\n%s", err, output)
		}
		return
	}
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	var startup syscall.StartupInfo
	if err := syscall.GetStartupInfo(&startup); err != nil {
		t.Fatal(err)
	}
	if startup.Flags&syscall.STARTF_USESHOWWINDOW == 0 || startup.ShowWindow != syscall.SW_HIDE {
		t.Fatal("fixture was not launched with the old updater's SW_HIDE override")
	}
	w := startupFixtureWindow(t)
	w.ready = true
	calls := 0
	w.cfg.OnReady = func() {
		calls++
		visible, _, _ := wndIsWindowVisible.Call(w.hwnd)
		if visible == 0 || !w.mainShown {
			t.Error("readiness was reported before the native window was visible")
		}
	}
	w.showMainWindow()
	w.showMainWindow()
	if calls != 1 || !w.mainShown {
		t.Fatalf("visible startup acknowledgement count = %d, shown = %v", calls, w.mainShown)
	}
}

func TestStartupErrorAndCancelledWindowNeverAcknowledge(t *testing.T) {
	for _, state := range []string{"error", "cancelled", "closing"} {
		t.Run(state, func(t *testing.T) {
			runtime.LockOSThread()
			t.Cleanup(runtime.UnlockOSThread)
			w := startupFixtureWindow(t)
			calls := 0
			w.cfg.OnReady = func() { calls++ }
			w.ready = true
			switch state {
			case "error":
				w.showStartupError(errors.New("fixture interface failure"))
			case "cancelled":
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				w.ctx = ctx
				w.showMainWindow()
			case "closing":
				w.closing = true
				w.showMainWindow()
			}
			if calls != 0 || w.startupNotice.sent {
				t.Fatal("failed or interrupted startup was acknowledged")
			}
		})
	}
}

// A native STATIC window avoids a dependency on an installed WebView2 Runtime.
// The real startup gate sets ready only after the UI initialization probe passes.
func startupFixtureWindow(t *testing.T) *hostWindow {
	t.Helper()
	class, title := utf16String("STATIC"), utf16String("Network Toolkit startup test")
	hwnd, _, err := wndCreateWindow.Call(0,
		uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)),
		0x00cf0000, // WS_OVERLAPPEDWINDOW, deliberately without WS_VISIBLE.
		40, 40, 320, 200, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatal(nativeError("create startup test window", err))
	}
	t.Cleanup(func() { wndDestroyWindow.Call(hwnd) })
	return &hostWindow{ctx: context.Background(), hwnd: hwnd, initializing: true}
}
