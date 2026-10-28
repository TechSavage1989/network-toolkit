//go:build windows

package desktop

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var (
	wndUser32 = syscall.NewLazyDLL("user32.dll")
	wndKernel = syscall.NewLazyDLL("kernel32.dll")
	wndGDI32  = syscall.NewLazyDLL("gdi32.dll")
	wndOLE32  = syscall.NewLazyDLL("ole32.dll")

	wndRegisterClass    = wndUser32.NewProc("RegisterClassExW")
	wndUnregisterClass  = wndUser32.NewProc("UnregisterClassW")
	wndCreateWindow     = wndUser32.NewProc("CreateWindowExW")
	wndDestroyWindow    = wndUser32.NewProc("DestroyWindow")
	wndDefWindowProc    = wndUser32.NewProc("DefWindowProcW")
	wndGetMessage       = wndUser32.NewProc("GetMessageW")
	wndTranslateMessage = wndUser32.NewProc("TranslateMessage")
	wndDispatchMessage  = wndUser32.NewProc("DispatchMessageW")
	wndPostMessage      = wndUser32.NewProc("PostMessageW")
	wndPostQuitMessage  = wndUser32.NewProc("PostQuitMessage")
	wndShowWindow       = wndUser32.NewProc("ShowWindow")
	wndUpdateWindow     = wndUser32.NewProc("UpdateWindow")
	wndSetForeground    = wndUser32.NewProc("SetForegroundWindow")
	wndFlashWindow      = wndUser32.NewProc("FlashWindowEx")
	wndLoadIcon         = wndUser32.NewProc("LoadIconW")
	wndLoadCursor       = wndUser32.NewProc("LoadCursorW")
	wndGetClientRect    = wndUser32.NewProc("GetClientRect")
	wndGetWindowRect    = wndUser32.NewProc("GetWindowRect")
	wndIsZoomed         = wndUser32.NewProc("IsZoomed")
	wndIsIconic         = wndUser32.NewProc("IsIconic")
	wndIsWindowVisible  = wndUser32.NewProc("IsWindowVisible")
	wndSetWindowPos     = wndUser32.NewProc("SetWindowPos")
	wndMonitorFromRect  = wndUser32.NewProc("MonitorFromRect")
	wndGetMonitorInfo   = wndUser32.NewProc("GetMonitorInfoW")
	wndGetSystemMetrics = wndUser32.NewProc("GetSystemMetrics")
	wndGetDPIForWindow  = wndUser32.NewProc("GetDpiForWindow")
	wndBeginPaint       = wndUser32.NewProc("BeginPaint")
	wndEndPaint         = wndUser32.NewProc("EndPaint")
	wndFillRect         = wndUser32.NewProc("FillRect")
	wndDrawText         = wndUser32.NewProc("DrawTextW")
	wndInvalidateRect   = wndUser32.NewProc("InvalidateRect")
	wndMessageBox       = wndUser32.NewProc("MessageBoxW")
	wndSendMessage      = wndUser32.NewProc("SendMessageW")
	wndSetTimer         = wndUser32.NewProc("SetTimer")
	wndKillTimer        = wndUser32.NewProc("KillTimer")
	wndGetModuleHandle  = wndKernel.NewProc("GetModuleHandleW")
	wndCreateFont       = wndGDI32.NewProc("CreateFontW")
	wndDeleteObject     = wndGDI32.NewProc("DeleteObject")
	wndSelectObject     = wndGDI32.NewProc("SelectObject")
	wndSetTextColor     = wndGDI32.NewProc("SetTextColor")
	wndSetBkMode        = wndGDI32.NewProc("SetBkMode")
	wndGetStockObject   = wndGDI32.NewProc("GetStockObject")
	wndCoInitializeEx   = wndOLE32.NewProc("CoInitializeEx")
	wndCoUninitialize   = wndOLE32.NewProc("CoUninitialize")
	wndProcedurePointer = syscall.NewCallback(windowProcedure)
	wndWindows          sync.Map
	wndPending          sync.Map
	wndNextToken        atomic.Uint64
	wndRunning          atomic.Bool
)

const (
	wmDestroy          = 0x0002
	wmMove             = 0x0003
	wmSize             = 0x0005
	wmSetFocus         = 0x0007
	wmPaint            = 0x000f
	wmClose            = 0x0010
	wmQueryEndSession  = 0x0011
	wmEraseBackground  = 0x0014
	wmEndSession       = 0x0016
	wmGetMinMaxInfo    = 0x0024
	wmWindowPosChanged = 0x0047
	wmNCCreate         = 0x0081
	wmNCDestroy        = 0x0082
	wmCommand          = 0x0111
	wmTimer            = 0x0113
	wmDPIChanged       = 0x02e0
	wmRunUI            = 0x8001
	wmRequestActivate  = 0x8002
	windowCloseButton  = 1001
	windowCloseTimer   = 1002
	windowStartupTimer = 1003
)

type windowClass struct {
	Size        uint32
	Style       uint32
	Procedure   uintptr
	ClassExtra  int32
	WindowExtra int32
	Instance    uintptr
	Icon        uintptr
	Cursor      uintptr
	Background  uintptr
	MenuName    *uint16
	ClassName   *uint16
	SmallIcon   uintptr
}

type windowMessage struct {
	Window  uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Point   winPoint
	Private uint32
}

type winPoint struct{ X, Y int32 }

type createWindowStruct struct {
	CreateParams uintptr
	Instance     uintptr
	Menu         uintptr
	Parent       uintptr
	Height       int32
	Width        int32
	Y            int32
	X            int32
	Style        int32
	Name         *uint16
	Class        *uint16
	ExStyle      uint32
}

type monitorInfo struct {
	Size    uint32
	Monitor Rect
	Work    Rect
	Flags   uint32
}

type minMaxInfo struct {
	Reserved     winPoint
	MaxSize      winPoint
	MaxPosition  winPoint
	MinTrackSize winPoint
	MaxTrackSize winPoint
}

type paintStruct struct {
	DC        uintptr
	Erase     int32
	Paint     Rect
	Restore   int32
	IncUpdate int32
	Reserved  [32]byte
}

type windowFlashInfo struct {
	Size    uint32
	Window  uintptr
	Flags   uint32
	Count   uint32
	Timeout uint32
}

type hostWindow struct {
	ctx              context.Context
	cfg              Config
	hwnd             uintptr // UI thread only; posted work reads the atomic copy.
	postedHandle     atomic.Uintptr
	activationQueued atomic.Bool
	closeButton      uintptr
	font             uintptr
	headingFont      uintptr
	view             *WebView
	state            windowState
	ready            bool
	mainShown        bool
	startup          startupGate
	startupNotice    startupNotification
	startupTimer     uintptr
	splash           *splashWindow
	statusTitle      string
	statusText       string
	initializing     bool
	closeFlow        closeHandshake
	closeTimer       uintptr
	closeAttempt     uint64
	closingNotified  bool // UI thread only; set before invoking OnClosing.
	stopOnce         sync.Once
	queueMu          sync.Mutex
	queue            []func()
	closing          bool // protected by queueMu for cross-thread Post calls.
}

// Run owns the native window, its STA message loop, and the WebView controller.
// Returning from Run means the window is closed and Stop has been invoked.
func Run(ctx context.Context, cfg Config) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if !wndRunning.CompareAndSwap(false, true) {
		return errors.New("a desktop window is already running in this process")
	}
	defer wndRunning.Store(false)
	stop := func() {
		if cfg.Stop != nil {
			cfg.Stop()
		}
	}
	defer stop()
	if err := ctx.Err(); err != nil {
		return nil
	}
	var err error
	cfg, err = prepareConfig(cfg)
	if err != nil {
		return err
	}
	profile := filepath.Join(cfg.DataDir, "Network_Toolkit.WebView2")
	if err := os.MkdirAll(profile, 0700); err != nil {
		return fmt.Errorf("create the app's WebView2 profile: %w; move the complete application folder to a location you can write to", err)
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := wndCoInitializeEx.Call(0, 2) // COINIT_APARTMENTTHREADED
	if int32(hr) < 0 {
		return fmt.Errorf("initialize the window's COM apartment: HRESULT 0x%08X", uint32(hr))
	}
	defer wndCoUninitialize.Call()

	instance, _, callErr := wndGetModuleHandle.Call(0)
	if instance == 0 {
		return nativeError("get application module", callErr)
	}
	className := utf16String("NetworkToolkit.WebView2.Window")
	icon, _, _ := wndLoadIcon.Call(instance, 1)  // Linked group icon resource 1.
	cursor, _, _ := wndLoadCursor.Call(0, 32512) // IDC_ARROW
	class := windowClass{
		Size: uint32(unsafe.Sizeof(windowClass{})), Style: 0x0003,
		Procedure: wndProcedurePointer, Instance: instance, Icon: icon,
		SmallIcon: icon, Cursor: cursor, Background: 6, ClassName: className,
	}
	atom, _, callErr := wndRegisterClass.Call(uintptr(unsafe.Pointer(&class)))
	if atom == 0 {
		return nativeError("register the app window", callErr)
	}
	defer func() {
		wndUnregisterClass.Call(uintptr(unsafe.Pointer(className)), instance)
		runtime.KeepAlive(className)
	}()

	saved, _ := readWindowState(filepath.Join(cfg.DataDir, windowStateFilename))
	work := workAreaFor(saved.Bounds)
	saved.Bounds = fitWindowBounds(saved.Bounds, work, 720, 520)
	saved.Version = 1
	w := &hostWindow{
		ctx: ctx, cfg: cfg, state: saved, initializing: true,
		statusTitle: cfg.Title,
		startup:     startupGate{deadline: time.Now().Add(45 * time.Second)},
	}
	token := uintptr(wndNextToken.Add(1))
	wndPending.Store(token, w)
	title := utf16String(cfg.Title)
	b := saved.Bounds
	hwnd, _, callErr := wndCreateWindow.Call(
		0x00040000, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)),
		0x00cf0000|0x02000000, // WS_OVERLAPPEDWINDOW | WS_CLIPCHILDREN
		uintptr(b.Left), uintptr(b.Top), uintptr(b.Right-b.Left), uintptr(b.Bottom-b.Top),
		0, 0, instance, token,
	)
	wndPending.Delete(token)
	if hwnd == 0 {
		return nativeError("create the app window", callErr)
	}
	w.hwnd = hwnd
	w.postedHandle.Store(hwnd)
	defer func() {
		if w.postedHandle.Load() != 0 {
			w.shutdown()
			wndDestroyWindow.Call(hwnd)
		}
		if w.font != 0 {
			wndDeleteObject.Call(w.font)
		}
		if w.headingFont != 0 {
			wndDeleteObject.Call(w.headingFont)
		}
		runtime.KeepAlive(w)
	}()
	w.setFonts()
	buttonClass, buttonText := utf16String("BUTTON"), utf16String("Close")
	w.closeButton, _, _ = wndCreateWindow.Call(0,
		uintptr(unsafe.Pointer(buttonClass)), uintptr(unsafe.Pointer(buttonText)),
		0x40000000|0x00010000|1, 0, 0, 96, 34,
		hwnd, windowCloseButton, instance, 0)
	if w.closeButton != 0 && w.font != 0 {
		wndSendMessage.Call(w.closeButton, 0x0030, w.font, 1) // WM_SETFONT
	}
	w.layout()
	w.splash, err = newSplashWindow(w, instance, work)
	if err != nil {
		return err
	}
	w.startupTimer, _, callErr = wndSetTimer.Call(hwnd, windowStartupTimer, 100, 0)
	if w.startupTimer == 0 {
		return nativeError("start the splash screen timer", callErr)
	}

	done := make(chan struct{})
	defer close(done)
	go func() {
		activate := cfg.Activate
		for {
			select {
			case <-ctx.Done():
				if h := w.postedHandle.Load(); h != 0 {
					wndPostMessage.Call(h, wmClose, 0, 0)
				}
				return
			case <-done:
				return
			case _, open := <-activate:
				if !open {
					activate = nil
					continue
				}
				if ctx.Err() == nil {
					w.postActivation()
				}
			}
		}
	}()

	w.view, err = StartWebView(hwnd, WebViewConfig{
		AppURL: cfg.URL, SessionToken: cfg.SessionToken, UserDataPath: profile,
		navigationObserver: cfg.navigationObserver,
		Post:               w.post,
		SaveDownload: func(suggested string) (string, bool) {
			filename, cancelled, err := saveDownloadPath(hwnd, suggested)
			if err != nil {
				w.showMessage("Unable to save report", err.Error())
			}
			return filename, err == nil && !cancelled
		},
		OnError: func(err error) {
			if err != nil {
				w.post(func() {
					if !w.mainShown || w.view != nil && w.view.closed {
						w.showStartupError(err)
					} else {
						w.showMessage("Network Toolkit", err.Error())
					}
				})
			}
		},
	}, func(view *WebView, readyErr error) {
		// Keep native window changes outside WebView2's COM event callback.
		w.post(func() {
			if w.startup.finished || w.isClosing() {
				return
			}
			if readyErr != nil {
				w.showStartupError(readyErr)
				return
			}
			w.view = view
			w.startup.loaded = true
			if w.splash != nil {
				wndInvalidateRect.Call(w.splash.hwnd, 0, 0)
			}
			w.startupTick()
		})
	})
	if err != nil {
		w.showStartupError(err)
	}
	// The returned handle stores bounds even while the asynchronous environment
	// is still pending, so initial HTML layout has a real client viewport.
	w.layout()

	var msg windowMessage
	for {
		result, _, callErr := wndGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(result) == -1 {
			return nativeError("read a window message", callErr)
		}
		if result == 0 {
			break
		}
		if !w.mainShown && msg.Message == 0x0100 && msg.WParam == 0x1b { // WM_KEYDOWN, VK_ESCAPE
			w.requestClose()
			continue
		}
		wndTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		wndDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
	return nil
}

func prepareConfig(cfg Config) (Config, error) {
	if strings.TrimSpace(cfg.Title) == "" {
		cfg.Title = "Network Toolkit"
	}
	u, err := url.Parse(cfg.URL)
	if err != nil || u.Scheme != "http" || u.User != nil {
		return cfg, errors.New("the desktop app requires its local HTTP server address")
	}
	ip := net.ParseIP(u.Hostname())
	port, portErr := strconv.Atoi(u.Port())
	if ip == nil || !ip.IsLoopback() || portErr != nil || port < 1 || port > 65535 {
		return cfg, errors.New("the desktop app address must use a loopback IP and a valid port")
	}
	if cfg.DataDir == "" {
		exe, err := os.Executable()
		if err != nil {
			return cfg, fmt.Errorf("locate the portable application: %w", err)
		}
		cfg.DataDir = filepath.Dir(exe)
	}
	cfg.DataDir, err = filepath.Abs(cfg.DataDir)
	if err != nil {
		return cfg, fmt.Errorf("resolve the application folder: %w", err)
	}
	return cfg, nil
}

func (w *hostWindow) post(fn func()) bool {
	if fn == nil {
		return false
	}
	w.queueMu.Lock()
	defer w.queueMu.Unlock()
	if w.closing || w.postedHandle.Load() == 0 {
		return false
	}
	w.queue = append(w.queue, fn)
	ok, _, _ := wndPostMessage.Call(w.postedHandle.Load(), wmRunUI, 0, 0)
	if ok == 0 {
		w.queue = w.queue[:len(w.queue)-1]
		return false
	}
	return true
}

// postActivation coalesces requests while the UI thread is busy. The channel
// listener never calls window or WebView activation methods off the STA thread.
func (w *hostWindow) postActivation() {
	hwnd := w.postedHandle.Load()
	if hwnd == 0 || !w.activationQueued.CompareAndSwap(false, true) {
		return
	}
	if ok, _, _ := wndPostMessage.Call(hwnd, wmRequestActivate, 0, 0); ok == 0 {
		w.activationQueued.Store(false)
	}
}

func (w *hostWindow) activate() {
	if w.hwnd == 0 || w.ctx.Err() != nil || w.isClosing() || w.closeFlow.pending {
		return
	}
	if !w.mainShown {
		if w.splash != nil && w.splash.hwnd != 0 {
			wndSetForeground.Call(w.splash.hwnd)
		}
		return
	}
	if iconic, _, _ := wndIsIconic.Call(w.hwnd); iconic != 0 {
		wndShowWindow.Call(w.hwnd, 9) // SW_RESTORE preserves the pre-minimize state.
	} else {
		wndShowWindow.Call(w.hwnd, 5) // SW_SHOW preserves a maximized window.
	}
	flash := windowFlashInfo{Size: uint32(unsafe.Sizeof(windowFlashInfo{})), Window: w.hwnd}
	if foreground, _, _ := wndSetForeground.Call(w.hwnd); foreground != 0 {
		wndFlashWindow.Call(uintptr(unsafe.Pointer(&flash))) // FLASHW_STOP.
		if w.view != nil && w.ready {
			w.view.Focus()
		}
		return
	}
	// Respect Windows foreground restrictions. A bounded taskbar indication
	// makes the requested window discoverable without forcing focus ownership.
	flash.Flags, flash.Count = 2, 3 // FLASHW_TRAY, three flashes.
	wndFlashWindow.Call(uintptr(unsafe.Pointer(&flash)))
}

func (w *hostWindow) runPosted() {
	w.queueMu.Lock()
	queued := w.queue
	w.queue = nil
	w.queueMu.Unlock()
	for _, fn := range queued {
		if w.isClosing() {
			break
		}
		fn()
	}
}

func (w *hostWindow) isClosing() bool {
	w.queueMu.Lock()
	defer w.queueMu.Unlock()
	return w.closing
}

func (w *hostWindow) notifyClosing() {
	if w.closingNotified {
		return
	}
	w.closingNotified = true
	if w.cfg.OnClosing != nil {
		w.cfg.OnClosing()
	}
}

func (w *hostWindow) shutdown() {
	w.queueMu.Lock()
	if w.closing {
		w.queueMu.Unlock()
		return
	}
	w.closing = true
	w.queue = nil
	w.queueMu.Unlock()
	w.closeFlow.finished = true
	w.finishStartup()
	w.notifyClosing()
	if w.closeTimer != 0 {
		wndKillTimer.Call(w.hwnd, w.closeTimer)
		w.closeTimer = 0
	}
	w.captureBounds()
	_ = writeWindowState(filepath.Join(w.cfg.DataDir, windowStateFilename), w.state)
	w.stopOnce.Do(func() {
		if w.cfg.Stop != nil {
			w.cfg.Stop()
		}
	})
	if w.view != nil {
		w.view.Close()
		w.view = nil
	}
}

func (w *hostWindow) requestClose() {
	ready := w.ready && w.view != nil && !w.view.closed
	w.closeFlow.approvalRequired = w.cfg.RequireCloseApproval && w.ready
	decision := w.closeFlow.request(ready, w.ctx.Err() != nil)
	if decision == closeWindowNow {
		w.closeNow()
		return
	}
	if decision == closeNotApproved {
		w.closeApprovalBlocked()
		return
	}
	if decision != requestRendererClose {
		return
	}
	if !w.closeFlow.approvalRequired {
		w.notifyClosing()
	}
	w.closeAttempt++
	attempt := w.closeAttempt
	// WM_TIMER bounds a stalled renderer or final save. It is handled by the
	// same UI message loop as the close request and is removed during shutdown.
	timer, _, _ := wndSetTimer.Call(w.hwnd, windowCloseTimer, 5000, 0)
	if timer == 0 {
		w.finishCloseRequest(false)
		return
	}
	w.closeTimer = timer
	if err := w.view.RequestClose(func(handled bool) {
		if attempt == w.closeAttempt {
			w.finishCloseRequest(handled)
		}
	}); err != nil {
		w.finishCloseRequest(false)
	}
}

func (w *hostWindow) finishCloseRequest(handled bool) {
	// RequestClose posts this callback to the UI queue after its COM callback
	// returns, so destroying the controller here cannot reenter that callback.
	switch w.closeFlow.completed(handled) {
	case closeWindowNow:
		w.closeNow()
	case closeNotApproved:
		w.closeApprovalBlocked()
	}
}

func (w *hostWindow) closeApprovalBlocked() {
	w.closeAttempt++ // A late callback cannot complete a newer close attempt.
	if w.closeTimer != 0 {
		wndKillTimer.Call(w.hwnd, w.closeTimer)
		w.closeTimer = 0
	}
	w.showMessage("Close not approved", "This window remains open because closing was not approved. Finish or cancel any open save or document-transfer dialog, then try again. The timeout does not approve discarding documents.")
}

func (w *hostWindow) closeNow() {
	w.shutdown()
	wndDestroyWindow.Call(w.hwnd)
}

func (w *hostWindow) captureBounds() {
	if w.initializing || w.hwnd == 0 {
		return
	}
	iconic, _, _ := wndIsIconic.Call(w.hwnd)
	if iconic != 0 {
		return // Preserve both normal bounds and the last non-minimized state.
	}
	zoomed, _, _ := wndIsZoomed.Call(w.hwnd)
	w.state.Maximized = zoomed != 0
	if zoomed != 0 {
		return
	}
	var bounds Rect
	ok, _, _ := wndGetWindowRect.Call(w.hwnd, uintptr(unsafe.Pointer(&bounds)))
	if ok != 0 && validRect(bounds) {
		w.state.Bounds = bounds
	}
}

func (w *hostWindow) layout() {
	if w.hwnd == 0 {
		return
	}
	var client Rect
	ok, _, _ := wndGetClientRect.Call(w.hwnd, uintptr(unsafe.Pointer(&client)))
	if ok == 0 {
		return
	}
	if w.view != nil && client.Right > 0 && client.Bottom > 0 {
		w.view.Resize(client)
	}
	if w.closeButton != 0 {
		width, height := w.scale(100), w.scale(36)
		x, y := (client.Right-width)/2, client.Bottom-height-w.scale(32)
		wndSetWindowPos.Call(w.closeButton, 0, uintptr(x), uintptr(max(y, 0)), uintptr(width), uintptr(height), 0x0004|0x0010)
	}
}

func (w *hostWindow) scale(value int32) int32 {
	dpi := uintptr(96)
	if w.hwnd != 0 && wndGetDPIForWindow.Find() == nil {
		if got, _, _ := wndGetDPIForWindow.Call(w.hwnd); got != 0 {
			dpi = got
		}
	}
	return int32((int64(value)*int64(dpi) + 48) / 96)
}

func (w *hostWindow) setFonts() {
	face := utf16String("Segoe UI")
	create := func(size, weight int32) uintptr {
		font, _, _ := wndCreateFont.Call(uintptr(-w.scale(size)), 0, 0, 0, uintptr(weight),
			0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)))
		return font
	}
	old, oldHeading := w.font, w.headingFont
	w.font, w.headingFont = create(17, 400), create(26, 600)
	if w.closeButton != 0 && w.font != 0 {
		wndSendMessage.Call(w.closeButton, 0x0030, w.font, 1)
	}
	if old != 0 {
		wndDeleteObject.Call(old)
	}
	if oldHeading != 0 {
		wndDeleteObject.Call(oldHeading)
	}
}

func (w *hostWindow) showStartupError(err error) {
	if w.isClosing() {
		return
	}
	w.ready = false
	w.statusTitle = "The application window is unavailable"
	w.statusText = err.Error() + "\n\nClose this window after reviewing the error."
	if w.view != nil {
		w.view.Close()
		w.view = nil
	}
	w.showMainWindow()
	if w.closeButton != 0 {
		wndShowWindow.Call(w.closeButton, 1)
	}
	wndInvalidateRect.Call(w.hwnd, 0, 1)
}

func (w *hostWindow) showMessage(title, message string) {
	if w.isClosing() {
		return
	}
	t, m := utf16String(title), utf16String(message)
	wndMessageBox.Call(w.hwnd, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x00000030)
}

func (w *hostWindow) paint() {
	var ps paintStruct
	hdc, _, _ := wndBeginPaint.Call(w.hwnd, uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer wndEndPaint.Call(w.hwnd, uintptr(unsafe.Pointer(&ps)))
	if w.ready {
		return
	}
	var client Rect
	wndGetClientRect.Call(w.hwnd, uintptr(unsafe.Pointer(&client)))
	brush, _, _ := wndGetStockObject.Call(0) // WHITE_BRUSH
	wndFillRect.Call(hdc, uintptr(unsafe.Pointer(&client)), brush)
	wndSetBkMode.Call(hdc, 1)
	wndSetTextColor.Call(hdc, 0x003e2b19)
	heading := Rect{w.scale(40), max(w.scale(48), client.Bottom/4-w.scale(30)), client.Right - w.scale(40), 0}
	heading.Bottom = heading.Top + w.scale(90)
	body := Rect{w.scale(48), heading.Bottom + w.scale(12), client.Right - w.scale(48), client.Bottom - w.scale(96)}
	draw := func(text string, rect *Rect, font uintptr) {
		if font == 0 {
			font, _, _ = wndGetStockObject.Call(17) // DEFAULT_GUI_FONT
		}
		old, _, _ := wndSelectObject.Call(hdc, font)
		p := utf16String(text)
		wndDrawText.Call(hdc, uintptr(unsafe.Pointer(p)), ^uintptr(0), uintptr(unsafe.Pointer(rect)), 0x0001|0x0010|0x0800)
		wndSelectObject.Call(hdc, old)
	}
	draw(w.statusTitle, &heading, w.headingFont)
	draw(w.statusText, &body, w.font)
}

func windowProcedure(hwnd uintptr, message uint32, wparam, lparam uintptr) uintptr {
	if message == wmNCCreate {
		cs := (*createWindowStruct)(unsafe.Pointer(lparam))
		if pending, ok := wndPending.Load(cs.CreateParams); ok {
			w := pending.(*hostWindow)
			w.hwnd = hwnd
			wndWindows.Store(hwnd, w)
			return 1
		}
	}
	value, found := wndWindows.Load(hwnd)
	if !found {
		result, _, _ := wndDefWindowProc.Call(hwnd, uintptr(message), wparam, lparam)
		return result
	}
	w := value.(*hostWindow)
	switch message {
	case wmClose:
		w.requestClose()
		return 0
	case wmDestroy:
		w.shutdown()
		wndPostQuitMessage.Call(0)
		return 0
	case wmNCDestroy:
		w.postedHandle.Store(0)
		wndWindows.Delete(hwnd)
	case wmQueryEndSession:
		w.captureBounds()
		_ = writeWindowState(filepath.Join(w.cfg.DataDir, windowStateFilename), w.state)
		return 1
	case wmEndSession:
		if wparam != 0 {
			w.closeNow()
		}
		return 0
	case wmTimer:
		if w.startupTimer != 0 && wparam == w.startupTimer {
			w.startupTick()
			return 0
		}
		if w.closeTimer != 0 && wparam == w.closeTimer {
			switch w.closeFlow.expired() {
			case closeWindowNow:
				w.closeNow()
			case closeNotApproved:
				w.closeApprovalBlocked()
			}
			return 0
		}
	case wmRunUI:
		w.runPosted()
		return 0
	case wmRequestActivate:
		w.activationQueued.Store(false)
		w.activate()
		return 0
	case wmSize:
		if wparam != 1 { // SIZE_MINIMIZED
			w.layout()
			w.captureBounds()
		}
		return 0
	case wmMove:
		if w.view != nil {
			w.view.PositionChanged()
		}
		return 0
	case wmWindowPosChanged:
		w.captureBounds()
	case wmSetFocus:
		if w.view != nil {
			w.view.Focus()
		}
		return 0
	case wmGetMinMaxInfo:
		mmi := (*minMaxInfo)(unsafe.Pointer(lparam))
		bounds := w.state.Bounds
		if w.hwnd != 0 {
			var actual Rect
			if ok, _, _ := wndGetWindowRect.Call(w.hwnd, uintptr(unsafe.Pointer(&actual))); ok != 0 && validRect(actual) {
				bounds = actual
			}
		}
		work := workAreaFor(bounds)
		mmi.MinTrackSize = winPoint{min(w.scale(720), work.Right-work.Left), min(w.scale(520), work.Bottom-work.Top)}
		return 0
	case wmDPIChanged:
		suggested := *(*Rect)(unsafe.Pointer(lparam))
		if validRect(suggested) {
			r := fitWindowBounds(suggested, workAreaFor(suggested), w.scale(720), w.scale(520))
			wndSetWindowPos.Call(hwnd, 0, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top), 0x0004|0x0010)
		}
		w.setFonts()
		w.layout()
		wndInvalidateRect.Call(hwnd, 0, 1)
		return 0
	case wmCommand:
		if uint16(wparam) == windowCloseButton {
			wndPostMessage.Call(hwnd, wmClose, 0, 0)
			return 0
		}
	case wmEraseBackground:
		if w.ready {
			return 1
		}
	case wmPaint:
		w.paint()
		return 0
	}
	result, _, _ := wndDefWindowProc.Call(hwnd, uintptr(message), wparam, lparam)
	return result
}

func workAreaFor(bounds Rect) Rect {
	monitor, _, _ := wndMonitorFromRect.Call(uintptr(unsafe.Pointer(&bounds)), 2) // MONITOR_DEFAULTTONEAREST
	info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
	if monitor != 0 {
		ok, _, _ := wndGetMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info)))
		if ok != 0 && validRect(info.Work) {
			return info.Work
		}
	}
	width, _, _ := wndGetSystemMetrics.Call(0)
	height, _, _ := wndGetSystemMetrics.Call(1)
	if width == 0 || height == 0 {
		return Rect{Right: 1280, Bottom: 800}
	}
	return Rect{Right: int32(width), Bottom: int32(height)}
}

func utf16String(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(strings.ReplaceAll(s, "\x00", ""))
	return p
}

func nativeError(action string, err error) error {
	if err == nil || err == syscall.Errno(0) {
		return fmt.Errorf("%s failed", action)
	}
	return fmt.Errorf("%s: %w", action, err)
}
