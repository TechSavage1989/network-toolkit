package main

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

var (
	updateProgressUser       = syscall.NewLazyDLL("user32.dll")
	updateProgressCreate     = updateProgressUser.NewProc("CreateWindowExW")
	updateProgressDestroy    = updateProgressUser.NewProc("DestroyWindow")
	updateProgressDefault    = updateProgressUser.NewProc("DefWindowProcW")
	updateProgressPost       = updateProgressUser.NewProc("PostMessageW")
	updateProgressText       = updateProgressUser.NewProc("SetWindowTextW")
	updateProgressSend       = updateProgressUser.NewProc("SendMessageW")
	updateProgressRegister   = updateProgressUser.NewProc("RegisterClassExW")
	updateProgressUnregister = updateProgressUser.NewProc("UnregisterClassW")
	updateProgressGetMessage = updateProgressUser.NewProc("GetMessageW")
	updateProgressTranslate  = updateProgressUser.NewProc("TranslateMessage")
	updateProgressDispatch   = updateProgressUser.NewProc("DispatchMessageW")
	updateProgressMetrics    = updateProgressUser.NewProc("GetSystemMetrics")
	updateProgressQuit       = updateProgressUser.NewProc("PostQuitMessage")
	updateProgressProcedure  = syscall.NewCallback(func(hwnd, msg, wparam, lparam uintptr) uintptr {
		if msg == 0x10 {
			return 0
		} // File replacement is not cancellable by closing a dialog.
		if msg == 2 {
			updateProgressQuit.Call(0)
			return 0
		}
		result, _, _ := updateProgressDefault.Call(hwnd, msg, wparam, lparam)
		return result
	})
)

type updateProgressClass struct {
	size, style                        uint32
	procedure                          uintptr
	classExtra, windowExtra            int32
	instance, icon, cursor, background uintptr
	menu, name                         *uint16
	smallIcon                          uintptr
}
type updateProgressMessage struct {
	hwnd           uintptr
	message        uint32
	wparam, lparam uintptr
	time           uint32
	point          struct{ x, y int32 }
	private        uint32
}
type nativeUpdateProgress struct {
	mu     sync.Mutex
	hwnd   uintptr
	text   string
	closed bool
	done   chan struct{}
}

func (p *nativeUpdateProgress) Set(text string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.closed {
		p.text = text
		updateProgressPost.Call(p.hwnd, 0x8001, 0, 0)
	}
}
func (p *nativeUpdateProgress) Close() {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		updateProgressPost.Call(p.hwnd, 0x8002, 0, 0)
	}
	p.mu.Unlock()
	<-p.done
}
func startUpdateProgressWindow(version string) updateProgressWindow {
	ready := make(chan updateProgressWindow, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		wide := func(text string) *uint16 { value, _ := syscall.UTF16PtrFromString(text); return value }
		instance, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
		className := wide(fmt.Sprintf("NetworkToolkitUpdateProgress-%d", syscall.Getpid()))
		class := updateProgressClass{procedure: updateProgressProcedure, instance: instance, background: 16, name: className}
		class.size = uint32(unsafe.Sizeof(class))
		atom, _, _ := updateProgressRegister.Call(uintptr(unsafe.Pointer(&class)))
		if atom == 0 {
			ready <- silentUpdateProgress{}
			return
		}
		defer updateProgressUnregister.Call(uintptr(unsafe.Pointer(className)), instance)
		controls := struct{ size, classes uint32 }{8, 0x20} // ICC_PROGRESS_CLASS
		initialized, _, _ := syscall.NewLazyDLL("comctl32.dll").NewProc("InitCommonControlsEx").Call(uintptr(unsafe.Pointer(&controls)))
		if initialized == 0 {
			ready <- silentUpdateProgress{}
			return
		}
		screenWidth, _, _ := updateProgressMetrics.Call(0)
		screenHeight, _, _ := updateProgressMetrics.Call(1)
		width, height := uintptr(480), uintptr(168)
		x, y := uintptr(0), uintptr(0)
		if screenWidth > width {
			x = (screenWidth - width) / 2
		}
		if screenHeight > height {
			y = (screenHeight - height) / 2
		}
		hwnd, _, _ := updateProgressCreate.Call(1, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(wide("Updating Network Toolkit "+version))), 0x80c00000, x, y, width, height, 0, 0, instance, 0)
		if hwnd == 0 {
			ready <- silentUpdateProgress{}
			return
		}
		createChild := func(class, text string, style, x, y, w, h uintptr) uintptr {
			child, _, _ := updateProgressCreate.Call(0, uintptr(unsafe.Pointer(wide(class))), uintptr(unsafe.Pointer(wide(text))), 0x50000000|style, x, y, w, h, hwnd, 0, instance, 0)
			return child
		}
		label := createChild("STATIC", "Waiting for Network Toolkit to close...", 0, 20, 20, 430, 36)
		bar := createChild("msctls_progress32", "Installation progress", 8, 20, 62, 430, 12)
		hint := createChild("STATIC", "Your saved settings are kept. The app will reopen automatically.", 0, 20, 88, 430, 32)
		if label == 0 || bar == 0 || hint == 0 {
			updateProgressDestroy.Call(hwnd)
			ready <- silentUpdateProgress{}
			return
		}
		font, _, _ := syscall.NewLazyDLL("gdi32.dll").NewProc("GetStockObject").Call(17)
		updateProgressSend.Call(label, 0x30, font, 1)
		updateProgressSend.Call(hint, 0x30, font, 1)
		updateProgressSend.Call(bar, 0x040a, 1, 30) // PBM_SETMARQUEE
		// Consume the helper's inherited SW_HIDE startup setting, then show
		// this explicit progress window without exposing a console window.
		show := updateProgressUser.NewProc("ShowWindow")
		show.Call(hwnd, 10) // SW_SHOWDEFAULT
		show.Call(hwnd, 5)  // SW_SHOW
		updateProgressUser.NewProc("UpdateWindow").Call(hwnd)
		progress := &nativeUpdateProgress{hwnd: hwnd, done: make(chan struct{})}
		ready <- progress
		defer close(progress.done)
		var msg updateProgressMessage
		for {
			result, _, _ := updateProgressGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if int32(result) <= 0 {
				break
			}
			if msg.hwnd == hwnd && msg.message == 0x8002 {
				updateProgressDestroy.Call(hwnd)
				break
			}
			if msg.hwnd == hwnd && msg.message == 0x8001 {
				progress.mu.Lock()
				text := progress.text
				progress.mu.Unlock()
				updateProgressText.Call(label, uintptr(unsafe.Pointer(wide(text))))
				continue
			}
			updateProgressTranslate.Call(uintptr(unsafe.Pointer(&msg)))
			updateProgressDispatch.Call(uintptr(unsafe.Pointer(&msg)))
		}
		progress.mu.Lock()
		progress.closed = true
		progress.mu.Unlock()
	}()
	return <-ready
}
