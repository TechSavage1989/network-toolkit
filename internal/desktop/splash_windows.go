//go:build windows

package desktop

import (
	"errors"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"networktoolkit/internal/model"
)

var (
	splashProcedurePointer = syscall.NewCallback(splashProcedure)
	splashWindows          sync.Map
	splashPending          sync.Map
	splashLoadImage        = wndUser32.NewProc("LoadImageW")
	splashDrawIcon         = wndUser32.NewProc("DrawIconEx")
	splashCreateBrush      = wndGDI32.NewProc("CreateSolidBrush")
)

type splashWindow struct {
	owner     *hostWindow
	hwnd      uintptr
	instance  uintptr
	className *uint16
	icon      uintptr // LR_SHARED: owned by the application resource module.
	phase     int32
}

func newSplashWindow(owner *hostWindow, instance uintptr, work Rect) (*splashWindow, error) {
	s := &splashWindow{owner: owner, instance: instance, className: utf16String("NetworkToolkit.Startup.Splash")}
	icon, _, _ := wndLoadIcon.Call(instance, 1)
	cursor, _, _ := wndLoadCursor.Call(0, 32512)
	class := windowClass{
		Size: uint32(unsafe.Sizeof(windowClass{})), Style: 0x00020003, // CS_DROPSHADOW
		Procedure: splashProcedurePointer, Instance: instance, Icon: icon,
		SmallIcon: icon, Cursor: cursor, ClassName: s.className,
	}
	if atom, _, err := wndRegisterClass.Call(uintptr(unsafe.Pointer(&class))); atom == 0 {
		return nil, nativeError("register the splash screen", err)
	}
	token := uintptr(wndNextToken.Add(1))
	splashPending.Store(token, s)
	r := splashBounds(work, owner.scale(440), owner.scale(224))
	title := utf16String(owner.cfg.Title + " - Loading")
	hwnd, _, err := wndCreateWindow.Call(0x00000080, // WS_EX_TOOLWINDOW
		uintptr(unsafe.Pointer(s.className)), uintptr(unsafe.Pointer(title)),
		0x80000000|0x00080000|0x02000000, // WS_POPUP | WS_SYSMENU | WS_CLIPCHILDREN
		uintptr(r.Left), uintptr(r.Top), uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top),
		owner.hwnd, 0, instance, token)
	splashPending.Delete(token)
	if hwnd == 0 {
		wndUnregisterClass.Call(uintptr(unsafe.Pointer(s.className)), instance)
		return nil, nativeError("create the splash screen", err)
	}
	s.hwnd = hwnd
	s.icon, _, _ = splashLoadImage.Call(instance, 1, 1, uintptr(owner.scale(56)), uintptr(owner.scale(56)), 0x8000)
	buttonClass, text := utf16String("BUTTON"), utf16String("Cancel")
	button, _, _ := wndCreateWindow.Call(0, uintptr(unsafe.Pointer(buttonClass)), uintptr(unsafe.Pointer(text)),
		0x40000000|0x10000000|0x00010000, uintptr(owner.scale(326)), uintptr(owner.scale(176)),
		uintptr(owner.scale(86)), uintptr(owner.scale(29)), hwnd, windowCloseButton, instance, 0)
	if button != 0 {
		wndSendMessage.Call(button, 0x0030, owner.font, 1)
	}
	wndShowWindow.Call(hwnd, 1)
	wndUpdateWindow.Call(hwnd)
	return s, nil
}

func (s *splashWindow) close() {
	if s.hwnd != 0 {
		wndDestroyWindow.Call(s.hwnd)
		s.hwnd = 0
		wndUnregisterClass.Call(uintptr(unsafe.Pointer(s.className)), s.instance)
		runtime.KeepAlive(s.className)
	}
}

func (s *splashWindow) pulse() {
	s.phase = (s.phase + 1) % 60
	r := Rect{s.owner.scale(28), s.owner.scale(145), s.owner.scale(412), s.owner.scale(151)}
	wndInvalidateRect.Call(s.hwnd, uintptr(unsafe.Pointer(&r)), 0)
}

func (s *splashWindow) paint() {
	var ps paintStruct
	dc, _, _ := wndBeginPaint.Call(s.hwnd, uintptr(unsafe.Pointer(&ps)))
	if dc == 0 {
		return
	}
	defer wndEndPaint.Call(s.hwnd, uintptr(unsafe.Pointer(&ps)))
	scale := s.owner.scale
	fill := func(r Rect, color uintptr) {
		brush, _, _ := splashCreateBrush.Call(color)
		if brush != 0 {
			wndFillRect.Call(dc, uintptr(unsafe.Pointer(&r)), brush)
			wndDeleteObject.Call(brush)
		}
	}
	var client Rect
	wndGetClientRect.Call(s.hwnd, uintptr(unsafe.Pointer(&client)))
	fill(client, 0x00443629)
	fill(Rect{1, 1, client.Right - 1, client.Bottom - 1}, 0x0024140d)
	wndSetBkMode.Call(dc, 1)
	draw := func(text string, rect Rect, font, color uintptr) {
		old, _, _ := wndSelectObject.Call(dc, font)
		wndSetTextColor.Call(dc, color)
		p := utf16String(text)
		wndDrawText.Call(dc, uintptr(unsafe.Pointer(p)), ^uintptr(0), uintptr(unsafe.Pointer(&rect)), 0x0020|0x0800) // SINGLELINE | NOPREFIX
		wndSelectObject.Call(dc, old)
	}
	if s.icon != 0 {
		splashDrawIcon.Call(dc, uintptr(scale(28)), uintptr(scale(30)), s.icon, uintptr(scale(56)), uintptr(scale(56)), 0, 0, 3)
	}
	draw("Network Toolkit", Rect{scale(102), scale(31), scale(418), scale(69)}, s.owner.headingFont, 0x00faf6f2)
	draw("Version "+model.Version, Rect{scale(104), scale(72), scale(412), scale(99)}, s.owner.font, 0x00baa89a)
	label := "Loading interface..."
	if s.owner.startup.loaded {
		label = "Preparing workspace..."
	}
	draw(label, Rect{scale(28), scale(112), scale(412), scale(139)}, s.owner.font, 0x00d4c5b4)
	track := Rect{scale(28), scale(145), scale(412), scale(151)}
	fill(track, 0x00443629)
	width := track.Right - track.Left
	start := track.Left + (width+scale(100))*s.phase/60 - scale(100)
	fill(Rect{max(track.Left, start), track.Top, min(track.Right, start+scale(100)), track.Bottom}, 0x00f3b64c)
	draw("Esc to cancel", Rect{scale(28), scale(181), scale(300), scale(207)}, s.owner.font, 0x00baa89a)
}

func splashProcedure(hwnd uintptr, message uint32, wparam, lparam uintptr) uintptr {
	if message == wmNCCreate {
		cs := (*createWindowStruct)(unsafe.Pointer(lparam))
		if value, ok := splashPending.Load(cs.CreateParams); ok {
			s := value.(*splashWindow)
			s.hwnd = hwnd
			splashWindows.Store(hwnd, s)
			return 1
		}
	}
	if value, ok := splashWindows.Load(hwnd); ok {
		s := value.(*splashWindow)
		switch message {
		case wmClose:
			wndPostMessage.Call(s.owner.hwnd, wmClose, 0, 0)
			return 0
		case wmCommand:
			if uint16(wparam) == windowCloseButton {
				wndPostMessage.Call(s.owner.hwnd, wmClose, 0, 0)
			}
			return 0
		case wmPaint:
			s.paint()
			return 0
		case wmEraseBackground:
			return 1
		case wmNCDestroy:
			splashWindows.Delete(hwnd)
		}
	}
	result, _, _ := wndDefWindowProc.Call(hwnd, uintptr(message), wparam, lparam)
	return result
}

func (w *hostWindow) startupTick() {
	if w.isClosing() || w.startup.finished {
		return
	}
	if w.ctx.Err() != nil {
		w.requestClose()
		return
	}
	if w.splash != nil {
		w.splash.pulse()
	}
	switch w.startup.tick(time.Now()) {
	case startupFail:
		w.showStartupError(errors.New("The interface did not finish loading within 45 seconds. Close this window and reopen Network Toolkit"))
	case startupProbe:
		if err := w.view.ProbeStartup(func(state int) {
			if w.isClosing() || w.ctx.Err() != nil {
				return
			}
			switch w.startup.completed(state) {
			case startupShow:
				w.ready = true
				w.showMainWindow()
			case startupFail:
				w.showStartupError(errors.New("The application interface could not finish starting. Close this window and reopen Network Toolkit"))
			}
		}); err != nil {
			w.showStartupError(err)
		}
	}
}

func (w *hostWindow) finishStartup() {
	w.startup.finished = true
	if w.startupTimer != 0 {
		wndKillTimer.Call(w.hwnd, w.startupTimer)
		w.startupTimer = 0
	}
	if w.splash != nil {
		w.splash.close()
		w.splash = nil
	}
}

func (w *hostWindow) showMainWindow() {
	w.finishStartup()
	if !w.mainShown {
		show := uintptr(1)
		if w.state.Maximized {
			show = 3
		}
		wndShowWindow.Call(w.hwnd, show)
		w.mainShown = true
		w.initializing = false
		w.captureBounds()
	}
	w.layout()
	wndUpdateWindow.Call(w.hwnd)
	if w.ready && w.view != nil {
		w.view.Focus()
	}
}
