//go:build windows

package main

import "syscall"

var peerAllowSetForeground = syscall.NewLazyDLL("user32.dll").NewProc("AllowSetForegroundWindow")

// peerAllowForeground lets the other standalone app bring its own window to
// the front for the request this app is about to send it. Windows otherwise
// only flashes that app's taskbar button, and a paste review or the position
// bar would open behind this window. The process identifier comes from the
// protected instance record. This is best effort: Windows may still refuse,
// and the receiving app then flashes its taskbar button as before.
func peerAllowForeground(pid int) {
	if pid > 0 && peerAllowSetForeground.Find() == nil {
		_, _, _ = peerAllowSetForeground.Call(uintptr(uint32(pid)))
	}
}
