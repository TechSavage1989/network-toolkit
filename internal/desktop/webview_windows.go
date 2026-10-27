//go:build windows

package desktop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

var (
	iidEnvironmentCompleted = guid("4e8a3389-c9d8-4bd2-b6b5-124fee6cc14d")
	iidControllerCompleted  = guid("6c4819f3-c9b7-4260-8127-c9f5bde7f68c")
	iidNavigationStarting   = guid("9adbe429-f36d-432b-9ddc-f8881fbd76e3")
	iidNavigationCompleted  = guid("d33a35bf-1c49-4f98-93ab-006e0533fe1c")
	iidNewWindowRequested   = guid("d4c185fe-c81c-4989-97af-2d3fa7ab5651")
	iidPermissionRequested  = guid("15e1c6a3-c72a-4df3-91d7-d097fbec6bfd")
	iidProcessFailed        = guid("79e0aea4-990b-42d9-aa1d-0fcc2e5bc7f1")
	iidDownloadStarting     = guid("efedc989-c396-41ca-83f7-07f845a55724")
	iidDownloadStateChanged = guid("81336594-7ede-4ba9-bf71-acf0a95b58dd")
	iidScriptCompleted      = guid("49511172-cc67-4bca-9923-137112f4c4cc")
	iidCoreWebView4         = guid("20d02d59-6df2-42dc-bd06-f98a694b1302")
	iidSettings3            = guid("fdb5ab74-af33-4854-84f0-0a631deb5eba")
	iidSettings4            = guid("cb56846c-4168-4d53-b04f-03b6d6796ff2")
	iidSettings5            = guid("183e7052-1d03-43a0-ab99-98e043b66b39")
	iidSettings6            = guid("11cb3acd-9bc8-43b8-83bf-f40753714f87")
	iidController2          = guid("c979903e-d4ca-4228-92eb-47ee3fa96eab")
)

// WebView must be created, resized, focused, and closed on the caller's STA UI
// thread. The Win32 window owner supplies the message pump and COM apartment.
type WebView struct {
	parent                  uintptr
	cfg                     WebViewConfig
	ready                   func(*WebView, error)
	readySent               bool
	closed                  bool
	webRuntime              *installedWebViewRuntime
	environment             uintptr
	controller              uintptr
	core                    uintptr
	core4                   uintptr
	bounds                  Rect
	registrations           []eventRegistration
	downloads               map[uintptr]eventRegistration
	pendingDownloads        map[*pendingDownload]bool
	bootstrapScriptID       string
	blockedDocumentResponse uintptr
	documentRequestPermit   nativeDocumentRequestPermit
}

type pendingDownload struct {
	args      uintptr
	deferral  uintptr
	operation uintptr
	finished  bool
}

// StartWebView uses the compiled-in Go loader to find the installed Evergreen
// Runtime and load its existing client DLL through the normal Windows loader.
// There is no companion DLL, runtime extraction, browser fallback, or installer.
func StartWebView(parent uintptr, cfg WebViewConfig, ready func(*WebView, error)) (*WebView, error) {
	if parent == 0 {
		return nil, errors.New("WebView2 requires a parent window")
	}
	if runtime.GOARCH != "amd64" {
		return nil, errors.New("This native WebView2 build supports Windows x64")
	}
	if !sameAppOrigin(cfg.AppURL, cfg.AppURL, false) {
		return nil, errors.New("The desktop interface URL must use the app's exact local IPv4 HTTP origin")
	}
	if _, err := nativeSessionBootstrap(cfg.AppURL, cfg.SessionToken); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(cfg.UserDataPath) {
		return nil, errors.New("WebView2 requires an absolute application profile path")
	}
	if cfg.Post == nil || cfg.SaveDownload == nil {
		return nil, errors.New("Native export dialogs and UI-thread dispatch must be available")
	}
	if err := os.MkdirAll(cfg.UserDataPath, 0700); err != nil {
		return nil, fmt.Errorf("Create the application WebView2 profile: %w", err)
	}
	profile, err := syscall.UTF16PtrFromString(cfg.UserDataPath)
	if err != nil {
		return nil, err
	}
	if err := prepareWebViewRuntimeEnvironment(); err != nil {
		return nil, err
	}
	webRuntime, err := loadInstalledWebViewRuntime()
	if err != nil {
		return nil, fmt.Errorf("The installed Microsoft Edge WebView2 Runtime could not be loaded: %w", err)
	}
	w := &WebView{parent: parent, cfg: cfg, ready: ready, webRuntime: webRuntime, downloads: make(map[uintptr]eventRegistration), pendingDownloads: make(map[*pendingDownload]bool)}
	handler := newCOMHandler(iidEnvironmentCompleted, adaptRuntimeEnvironmentCompleted(func(result, environment uintptr) uintptr {
		if w.closed {
			return hresultOK
		}
		if failed(result) || environment == 0 {
			w.fail(fmt.Errorf("Microsoft WebView2 Runtime could not start (HRESULT 0x%08X). A working Microsoft Edge WebView2 Runtime is required", uint32(result)))
			return hresultOK
		}
		comAddRef(environment)
		w.environment = environment
		controllerHandler := newCOMHandler(iidControllerCompleted, func(result, controller uintptr) uintptr {
			if w.closed {
				if controller != 0 {
					comCall(controller, 24)
				}
				return hresultOK
			}
			if failed(result) || controller == 0 {
				w.fail(fmt.Errorf("Create the embedded WebView2 controller (HRESULT 0x%08X)", uint32(result)))
				return hresultOK
			}
			comAddRef(controller)
			w.controller = controller
			if err := w.initializeController(); err != nil {
				w.fail(err)
				return hresultOK
			}
			return hresultOK
		})
		hr := comCall(environment, 3, parent, controllerHandler.pointer())
		controllerHandler.release()
		if err := hresultError("Create the embedded WebView2 controller", hr); err != nil {
			w.fail(err)
		}
		return hresultOK
	}))
	// Match the upstream Go loader's runtime ABI: initial flag 1, installed
	// runtime type 0, private profile, COM options, and completion callback.
	// The options preserve default-disabled extensions and OS-account SSO.
	options := newWebViewEnvironmentOptions()
	hr, _, _ := webRuntime.Create.Call(1, 0, uintptr(unsafe.Pointer(profile)), options.pointer(), handler.pointer())
	options.release()
	handler.release()
	runtime.KeepAlive(profile)
	if failed(hr) {
		w.Close()
		return nil, fmt.Errorf("Microsoft WebView2 could not initialize (HRESULT 0x%08X). The Microsoft Edge WebView2 Runtime must already be installed", uint32(hr))
	}
	return w, nil
}

func (w *WebView) initializeController() error {
	if err := hresultError("Get embedded WebView2", comCall(w.controller, 25, uintptr(unsafe.Pointer(&w.core)))); err != nil {
		return err
	}
	if w.core == 0 {
		return errors.New("The WebView2 controller returned no interface")
	}
	if err := hresultError("Hide initializing WebView2", comCall(w.controller, 4, 0)); err != nil {
		return err
	}
	if controller2, err := comQuery(w.controller, iidController2); err == nil {
		// COREWEBVIEW2_COLOR is a four-byte A,R,G,B structure, passed by value.
		comCall(controller2, 27, 0x24140DFF)
		comRelease(controller2)
	}
	var settings uintptr
	if err := hresultError("Get WebView2 settings", comCall(w.core, 3, uintptr(unsafe.Pointer(&settings)))); err != nil {
		return err
	}
	defer comRelease(settings)
	for _, setting := range []struct {
		slot  int
		value uintptr
		name  string
	}{
		{4, 1, "Enable interface JavaScript"}, {6, 0, "Disable host web messaging"}, {8, 0, "Disable browser script dialogs"},
		{10, 0, "Disable browser status bar"}, {12, 0, "Disable developer tools"}, {14, 0, "Disable browser context menus"},
		{16, 0, "Disable script host objects"}, {18, 0, "Disable browser zoom controls"}, {20, 0, "Disable built-in error pages"},
	} {
		if err := hresultError(setting.name, comCall(settings, setting.slot, setting.value)); err != nil {
			return err
		}
	}
	settings3, err := comQuery(settings, iidSettings3)
	if err != nil {
		return errors.New("The installed WebView2 Runtime is too old to disable browser accelerator keys; update the Microsoft Edge WebView2 Runtime")
	}
	err = hresultError("Disable browser accelerator keys", comCall(settings3, 24, 0))
	comRelease(settings3)
	if err != nil {
		return err
	}
	settings4, err := comQuery(settings, iidSettings4)
	if err != nil {
		return errors.New("The installed WebView2 Runtime is too old for the required private input settings; update the Microsoft Edge WebView2 Runtime")
	}
	for _, slot := range []int{26, 28} {
		if err = hresultError("Disable password save and autofill", comCall(settings4, slot, 0)); err != nil {
			comRelease(settings4)
			return err
		}
	}
	comRelease(settings4)
	if settings5, err := comQuery(settings, iidSettings5); err == nil {
		comCall(settings5, 30, 0)
		comRelease(settings5)
	}
	if settings6, err := comQuery(settings, iidSettings6); err == nil {
		comCall(settings6, 32, 0)
		comRelease(settings6)
	}
	w.core4, err = comQuery(w.core, iidCoreWebView4)
	if err != nil {
		return errors.New("The installed WebView2 Runtime does not support native export handling; update the Microsoft Edge WebView2 Runtime")
	}
	add := func(object uintptr, slot, remove int, iid comGUID, handler func(uintptr, uintptr) uintptr) error {
		registration, err := addCOMEvent(object, slot, remove, iid, handler)
		if err == nil {
			w.registrations = append(w.registrations, registration)
		}
		return err
	}
	observeNavigation := func(kind, uri string, args uintptr, blocked bool) {
		observation := nativeNavigationObservation{Kind: kind, URI: uri, Blocked: blocked}
		if blocked {
			observation.CancelResult = uint32(comCall(args, 8, 1))
		}
		if w.cfg.navigationObserver != nil {
			var cancelled int32
			observation.CancelReadResult = uint32(comCall(args, 7, uintptr(unsafe.Pointer(&cancelled))))
			observation.Cancelled = cancelled != 0
			w.cfg.navigationObserver(observation)
		}
	}
	navigation := func(_ uintptr, args uintptr) uintptr {
		uri, err := comGetString(args, 3)
		// Preserve the live application by cancelling every full navigation after
		// native startup, including reloads of this exact URL. Genuine fragment
		// changes do not raise this event. Close/reopen for a fresh bootstrap.
		// Blob reports are accepted only by the separate DownloadStarting path.
		allowed := w.documentRequestPermit.allowNavigation(w.cfg.AppURL, uri)
		observeNavigation("top", uri, args, w.closed || err != nil || !allowed)
		return hresultOK
	}
	if err = add(w.core, 7, 8, iidNavigationStarting, navigation); err != nil {
		return err
	}
	// The application has no document frames. Do not grant a nested document
	// the parent's capability, even on the same origin.
	if err = add(w.core, 17, 18, iidNavigationStarting, func(_ uintptr, args uintptr) uintptr {
		uri, _ := comGetString(args, 3)
		observeNavigation("frame", uri, args, true)
		return hresultOK
	}); err != nil {
		return err
	}
	if err = add(w.core, 44, 45, iidNewWindowRequested, func(_ uintptr, args uintptr) uintptr { comCall(args, 6, 1); return hresultOK }); err != nil {
		return err
	}
	if err = add(w.core, 23, 24, iidPermissionRequested, w.permissionRequested); err != nil {
		return err
	}
	if err = add(w.core4, 75, 76, iidDownloadStarting, w.downloadStarting); err != nil {
		return err
	}
	if err = add(w.core, 25, 26, iidProcessFailed, func(_ uintptr, args uintptr) uintptr {
		var kind int32
		comCall(args, 3, uintptr(unsafe.Pointer(&kind)))
		if kind <= 2 {
			w.notify(fmt.Errorf("The embedded WebView2 process stopped or is unresponsive (kind %d). Close and reopen Network Toolkit", kind))
		}
		return hresultOK
	}); err != nil {
		return err
	}
	if err = add(w.core, 15, 16, iidNavigationCompleted, func(_ uintptr, args uintptr) uintptr {
		if w.closed {
			return hresultOK
		}
		var success int32
		var status int32
		comCall(args, 3, uintptr(unsafe.Pointer(&success)))
		comCall(args, 4, uintptr(unsafe.Pointer(&status)))
		if success == 0 {
			if status != 14 {
				w.fail(fmt.Errorf("The local interface could not load (WebView2 status %d)", status))
			}
			return hresultOK
		}
		if !w.readySent {
			if err := hresultError("Show embedded interface", comCall(w.controller, 4, 1)); err != nil {
				w.fail(err)
				return hresultOK
			}
			w.readySent = true
			if w.ready != nil {
				w.ready(w, nil)
			}
		}
		return hresultOK
	}); err != nil {
		return err
	}
	w.Resize(w.bounds)
	if err := w.installDocumentRequestPolicy(); err != nil {
		return err
	}
	return w.installNativeBootstrap()
}

func (w *WebView) fail(err error) {
	if w.closed {
		return
	}
	if !w.readySent {
		w.readySent = true
		if w.ready != nil {
			w.ready(w, err)
		}
	} else {
		w.notify(err)
	}
	w.Close()
}

func (w *WebView) notify(err error) {
	if !w.closed && w.cfg.OnError != nil {
		w.cfg.OnError(err)
	}
}

func (w *WebView) Resize(bounds Rect) {
	if w == nil || w.closed {
		return
	}
	w.bounds = bounds
	if w.controller == 0 {
		return
	}
	// A 16-byte RECT parameter is passed indirectly under the Windows x64 ABI.
	// Use a 16-byte-aligned temporary, as required for by-value aggregate args.
	var storage [32]byte
	base := unsafe.Pointer(&storage[0])
	padding := (16 - uintptr(base)%16) % 16
	pointer := unsafe.Add(base, padding)
	*(*Rect)(pointer) = bounds
	hr := comCall(w.controller, 6, uintptr(pointer))
	runtime.KeepAlive(storage)
	if err := hresultError("Resize embedded interface", hr); err != nil {
		w.notify(err)
	}
}

func (w *WebView) Focus() {
	if w != nil && !w.closed && w.controller != 0 {
		comCall(w.controller, 12, 0)
	}
}
func (w *WebView) PositionChanged() {
	if w != nil && !w.closed && w.controller != 0 {
		comCall(w.controller, 23)
	}
}

const requestCloseScript = "typeof window.__networkToolkitClose==='function'?(window.__networkToolkitClose(),true):false"

const startupStateScript = "window.__networkToolkitStartupState===1?1:window.__networkToolkitStartupState===-1?-1:0"

// ProbeStartup reads only the app's completion flag. It exposes no host object
// or web-message bridge and copies the borrowed result before posting work.
func (w *WebView) ProbeStartup(done func(int)) error {
	if w == nil || w.closed || w.core == 0 || done == nil || w.cfg.Post == nil {
		return errors.New("The embedded interface is unavailable during startup")
	}
	script := utf16String(startupStateScript)
	handler := newCOMHandler(iidScriptCompleted, func(result, jsonResult uintptr) uintptr {
		if w.closed {
			return hresultOK
		}
		state := 0
		if !failed(result) && jsonResult != 0 {
			at := func(i int) uint16 { return *(*uint16)(unsafe.Add(unsafe.Pointer(jsonResult), i*2)) }
			if at(0) == '1' && at(1) == 0 {
				state = 1
			} else if at(0) == '-' && at(1) == '1' && at(2) == 0 {
				state = -1
			}
		}
		w.cfg.Post(func() {
			if !w.closed {
				done(state)
			}
		})
		return hresultOK
	})
	hr := comCall(w.core, 29, uintptr(unsafe.Pointer(script)), handler.pointer())
	handler.release()
	runtime.KeepAlive(script)
	return hresultError("Read interface startup state", hr)
}

// RequestClose asks the loaded interface to save its final state before it
// stops the HTTP server. The callback reports whether that hook was invoked;
// the host still waits for server cancellation, with its own timeout. Dispatch
// completion outside the COM callback so fallback Close never runs reentrantly.
func (w *WebView) RequestClose(done func(handled bool)) error {
	if w == nil || w.closed || w.core == 0 {
		return errors.New("The embedded interface is unavailable for a final settings save")
	}
	if done == nil || w.cfg.Post == nil {
		return errors.New("Final settings save requires a UI-thread completion callback")
	}
	script, err := syscall.UTF16PtrFromString(requestCloseScript)
	if err != nil {
		return err
	}
	handler := newCOMHandler(iidScriptCompleted, func(result, jsonResult uintptr) uintptr {
		if w.closed {
			return hresultOK
		}
		// ExecuteScript supplies a borrowed UTF-16 JSON string valid only during
		// this callback. Inspect the exact JSON boolean before posting any work;
		// quoted strings, null, failed execution, and missing results are false.
		handled := !failed(result) && jsonResult != 0
		if handled {
			for i, expected := range [...]uint16{'t', 'r', 'u', 'e', 0} {
				if *(*uint16)(unsafe.Add(unsafe.Pointer(jsonResult), i*2)) != expected {
					handled = false
					break
				}
			}
		}
		w.cfg.Post(func() { done(handled) })
		return hresultOK
	})
	// Slot 29 and the completion IID are from the base SDK ICoreWebView2
	// interface, so this handshake adds no newer-runtime requirement.
	hr := comCall(w.core, 29, uintptr(unsafe.Pointer(script)), handler.pointer())
	handler.release()
	runtime.KeepAlive(script)
	return hresultError("Request final settings save", hr)
}

func (w *WebView) downloadStarting(_ uintptr, args uintptr) uintptr {
	comCall(args, 9, 1) // Suppress WebView2's download flyout and save UI.
	if w.closed {
		comCall(args, 5, 1)
		return hresultOK
	}
	var operation uintptr
	if failed(comCall(args, 3, uintptr(unsafe.Pointer(&operation)))) || operation == 0 {
		comCall(args, 5, 1)
		return hresultOK
	}
	uri, err := comGetString(operation, 9)
	if err != nil || !sameAppOrigin(w.cfg.AppURL, uri, true) {
		comCall(args, 5, 1)
		comRelease(operation)
		return hresultOK
	}
	suggested, err := comGetString(args, 6)
	if err != nil {
		comCall(args, 5, 1)
		comRelease(operation)
		w.notify(err)
		return hresultOK
	}
	suggested = filepath.Base(suggested)
	if suggested == "" || suggested == "." || suggested == string(filepath.Separator) {
		suggested = "Network_Toolkit_Report.json"
	}
	var deferral uintptr
	if failed(comCall(args, 10, uintptr(unsafe.Pointer(&deferral)))) || deferral == 0 {
		comCall(args, 5, 1)
		comRelease(operation)
		w.notify(errors.New("The native export dialog could not be prepared"))
		return hresultOK
	}
	comAddRef(args)
	pending := &pendingDownload{args: args, deferral: deferral, operation: operation}
	w.pendingDownloads[pending] = true
	finish := func(cancel bool) { w.finishDownload(pending, cancel) }
	if !w.cfg.Post(func() {
		if pending.finished {
			return
		}
		if w.closed {
			finish(true)
			return
		}
		path, ok := w.cfg.SaveDownload(suggested)
		if !ok || w.closed {
			finish(true)
			return
		}
		if !filepath.IsAbs(path) || strings.ContainsRune(path, 0) {
			finish(true)
			w.notify(errors.New("The export path must be an absolute file path"))
			return
		}
		if err := comPutString(args, 7, path); err != nil {
			finish(true)
			w.notify(fmt.Errorf("Set export destination: %w", err))
			return
		}
		registration, err := addCOMEvent(operation, 7, 8, iidDownloadStateChanged, func(sender, _ uintptr) uintptr { return w.downloadStateChanged(sender) })
		if err != nil {
			finish(true)
			w.notify(err)
			return
		}
		comAddRef(operation)
		w.downloads[operation] = registration
		finish(false)
	}) {
		finish(true)
	}
	return hresultOK
}

func (w *WebView) finishDownload(pending *pendingDownload, cancel bool) {
	if pending.finished {
		return
	}
	pending.finished = true
	if cancel {
		comCall(pending.args, 5, 1)
	}
	comCall(pending.deferral, 3)
	comRelease(pending.deferral)
	comRelease(pending.args)
	comRelease(pending.operation)
	delete(w.pendingDownloads, pending)
}

func (w *WebView) downloadStateChanged(operation uintptr) uintptr {
	var state, reason int32
	if failed(comCall(operation, 16, uintptr(unsafe.Pointer(&state)))) || state == 0 {
		return hresultOK
	}
	if state == 1 {
		comCall(operation, 17, uintptr(unsafe.Pointer(&reason)))
		if reason != 26 && reason != 27 {
			w.notify(fmt.Errorf("The export was not saved completely (WebView2 download reason %d)", reason))
		}
	}
	if registration, ok := w.downloads[operation]; ok {
		delete(w.downloads, operation)
		registration.remove()
		comRelease(operation)
	}
	return hresultOK
}

// Close is idempotent, including while asynchronous initialization is pending.
// Only normal COM Release and controller.Close are used. The installed runtime
// module stays loaded until process exit, so late completion callbacks never
// return into an unloaded module.
func (w *WebView) Close() {
	if w == nil || w.closed {
		return
	}
	w.closed = true
	w.documentRequestPermit.pending = false
	w.documentRequestPermit.navigationPending = false
	if w.bootstrapScriptID != "" && w.core != 0 {
		_ = comPutString(w.core, 28, w.bootstrapScriptID)
		w.bootstrapScriptID = ""
	}
	w.cfg.SessionToken = ""
	for pending := range w.pendingDownloads {
		w.finishDownload(pending, true)
	}
	for operation, registration := range w.downloads {
		registration.remove()
		comCall(operation, 18)
		comRelease(operation)
		delete(w.downloads, operation)
	}
	for i := len(w.registrations) - 1; i >= 0; i-- {
		w.registrations[i].remove()
	}
	w.registrations = nil
	if w.controller != 0 {
		comCall(w.controller, 24)
	}
	comRelease(w.blockedDocumentResponse)
	w.blockedDocumentResponse = 0
	comRelease(w.core4)
	comRelease(w.core)
	comRelease(w.controller)
	comRelease(w.environment)
	w.core4 = 0
	w.core = 0
	w.controller = 0
	w.environment = 0
}
