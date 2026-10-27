//go:build windows

package desktop

import (
	"errors"
	"runtime"
	"unsafe"
)

var iidWebResourceRequested = guid("ab00b74c-15f1-4646-80e8-e76341d25d71")
var iidCoreWebView22 = guid("db75dfc7-a857-4632-a398-6969dde26c0a")

// NavigationStarting can cancel document execution after the browser has already
// issued a request. Intercept document resources themselves, before network
// dispatch. Script/style/image/API requests are not included in this filter.
// Slots and the handler IID are from Microsoft.Web.WebView2 SDK WebView2.h.
func (w *WebView) installDocumentRequestPolicy() error {
	if w.cfg.navigationObserver != nil {
		version, err := comGetString(w.environment, 5)
		if err == nil {
			w.cfg.navigationObserver(nativeNavigationObservation{Kind: "runtime", RuntimeVersion: version})
		}
	}
	core22, err := comQuery(w.core, iidCoreWebView22)
	if err != nil {
		return errors.New("The installed Microsoft Edge WebView2 Runtime lacks required private document filtering (ICoreWebView2_22). Install or update the Evergreen x64 Runtime from https://developer.microsoft.com/microsoft-edge/webview2/#download, then reopen Network Toolkit. This capability requires Runtime 122.0.2365.46 or later; use the latest security-updated Evergreen version. Network Toolkit will not install it automatically or use a weaker browser fallback.")
	}
	defer comRelease(core22)
	reason := utf16String("Blocked")
	headers := utf16String("Content-Type: text/plain\r\nCache-Control: no-store\r\nContent-Length: 0\r\n")
	hr := comCall(w.environment, 4, 0, 403, uintptr(unsafe.Pointer(reason)), uintptr(unsafe.Pointer(headers)), uintptr(unsafe.Pointer(&w.blockedDocumentResponse)))
	runtime.KeepAlive(reason)
	runtime.KeepAlive(headers)
	if err := hresultError("Prepare blocked document response", hr); err != nil {
		return err
	}
	if w.blockedDocumentResponse == 0 {
		return errors.New("The native document request policy has no deny response")
	}
	pattern := utf16String("*")
	// The deprecated two-argument filter does not reliably cover iframes.
	// DOCUMENT resource context; all request sources, including nested frames.
	hr = comCall(core22, 123, uintptr(unsafe.Pointer(pattern)), 1, 0xffffffff)
	runtime.KeepAlive(pattern)
	if err := hresultError("Install native document resource filter", hr); err != nil {
		return err
	}
	registration, err := addCOMEvent(w.core, 55, 56, iidWebResourceRequested, w.documentResourceRequested)
	if err != nil {
		return err
	}
	w.registrations = append(w.registrations, registration)
	return nil
}

func (w *WebView) documentResourceRequested(_ uintptr, args uintptr) uintptr {
	var request uintptr
	uri := ""
	if !failed(comCall(args, 3, uintptr(unsafe.Pointer(&request)))) && request != 0 {
		defer comRelease(request)
		uri, _ = comGetString(request, 3)
	}
	// Fetch-Metadata headers are not yet available at this interception stage.
	// A single-use native startup permit distinguishes the initial trusted page
	// without relying on renderer-controlled headers or same-URL frame guesses.
	pending := w.documentRequestPermit.pending
	allowed := w.documentRequestPermit.consume(w.cfg.AppURL, uri)
	blocked := w.closed || !allowed
	hr := hresultOK
	if blocked {
		hr = comCall(args, 5, w.blockedDocumentResponse)
	}
	if w.cfg.navigationObserver != nil {
		w.cfg.navigationObserver(nativeNavigationObservation{Kind: "resource", URI: uri, Blocked: blocked, InitialDocumentPermit: pending, ResponseResult: uint32(hr)})
	}
	if failed(hr) {
		// Never continue with the original request if response replacement failed.
		w.fail(errors.New("The native document request could not be blocked"))
	}
	return hr
}
