//go:build windows

package desktop

import (
	"encoding/json"
	"errors"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

// Microsoft.Web.WebView2 1.0.4258.31 SDK WebView2.h. These base CDP APIs
// predate the existing required ICoreWebView2_22 document filter interface.
var iidDevToolsCompleted = guid("5c4889f0-5ef6-4c5a-952c-d8f1b92d0574")
var iidDevToolsEventReceived = guid("e2fda4be-5456-406c-a261-3d452138362c")
var iidDevToolsEventArgs2 = guid("2dc4959d-1494-4393-95ba-bea4cb9ebd1b")
var iidCoreWebView11 = guid("0be78e56-c193-4051-b943-23b460c08bdb")

// Fetch.RequestStage=Request pauses a document before network dispatch. An
// ordinary child frame starts its initial navigation under its parent's Fetch
// settings, before a cross-site child target commits. No child document is
// permitted to commit. Session-aware parsing also rejects a child request that
// reaches this receiver; it never shares the root's one-use startup permit.
func (w *WebView) installDocumentFetchPolicy() error {
	if w == nil || w.closed || w.core == 0 || w.cfg.Post == nil {
		return errors.New("The native interface is unavailable for document interception")
	}
	var err error
	w.core11, err = comQuery(w.core, iidCoreWebView11)
	if err != nil {
		return errors.New("The installed WebView2 Runtime lacks required native document interception")
	}
	eventName := utf16String("Fetch.requestPaused")
	var receiver uintptr
	hr := comCall(w.core, 42, uintptr(unsafe.Pointer(eventName)), uintptr(unsafe.Pointer(&receiver)))
	runtime.KeepAlive(eventName)
	if err := hresultError("Get native document interception events", hr); err != nil {
		comRelease(receiver)
		return err
	}
	if receiver == 0 {
		return errors.New("The native document interceptor returned no event receiver")
	}
	registration, err := addCOMEvent(receiver, 3, 4, iidDevToolsEventReceived, w.documentFetchPaused)
	if err != nil {
		comRelease(receiver)
		return err
	}
	// addCOMEvent keeps the callback alive, but does not own the receiver's
	// COM reference. Retain that reference until Close removes the event.
	w.fetchReceiver = receiver
	w.fetchRegistration = registration
	return w.callDocumentProtocol("", "Fetch.enable", nativeDocumentFetchParameters, func(err error) {
		if err != nil {
			w.fail(err)
			return
		}
		w.documentFetchReady = true
		if err := w.installNativeBootstrap(); err != nil {
			w.fail(err)
		}
	})
}

// Completion results are borrowed UTF-16 JSON, valid only during Invoke. These
// no-result Fetch methods must return an empty object, never a protocol error.
func nativeDocumentProtocolResult(pointer uintptr) error {
	if pointer == 0 {
		return errors.New("The native document interceptor returned no completion result")
	}
	chars := make([]uint16, 0, 64)
	for i := 0; i < 4096; i++ {
		ch := *(*uint16)(unsafe.Add(unsafe.Pointer(pointer), i*2))
		if ch == 0 {
			var result map[string]json.RawMessage
			if json.Unmarshal([]byte(string(utf16.Decode(chars))), &result) == nil && result != nil && len(result) == 0 {
				return nil
			}
			return errors.New("The native document interceptor returned an invalid completion result")
		}
		chars = append(chars, ch)
	}
	return errors.New("The native document interceptor returned an excessively long completion result")
}

// Never treat the method's synchronous S_OK as completion. CDP calls may be
// processed out of order; dependent startup work runs only after this handler,
// posted to the UI thread so error handling cannot reenter COM teardown.
func (w *WebView) callDocumentProtocol(sessionID, method, parameters string, done func(error)) error {
	if w.closed || w.core == 0 || w.cfg.Post == nil || done == nil || len(sessionID) > 256 {
		return errors.New("The native document interceptor is unavailable")
	}
	methodPointer, err := syscall.UTF16PtrFromString(method)
	if err != nil {
		return err
	}
	parametersPointer, err := syscall.UTF16PtrFromString(parameters)
	if err != nil {
		return err
	}
	sessionPointer, err := syscall.UTF16PtrFromString(sessionID)
	if err != nil {
		return err
	}
	handler := newCOMHandler(iidDevToolsCompleted, func(result, resultPointer uintptr) uintptr {
		if w.closed {
			return hresultOK
		}
		err := hresultError("Complete native document interception "+method, result)
		if err == nil {
			err = nativeDocumentProtocolResult(resultPointer)
		}
		if w.cfg.navigationObserver != nil {
			w.cfg.navigationObserver(nativeNavigationObservation{Kind: "fetch-result", ProtocolMethod: method, ProtocolSession: sessionID, ResponseResult: uint32(result)})
		}
		if !w.cfg.Post(func() {
			if !w.closed {
				done(err)
			}
		}) {
			return hresultFail
		}
		return hresultOK
	})
	var hr uintptr
	if sessionID == "" {
		hr = comCall(w.core, 36, uintptr(unsafe.Pointer(methodPointer)), uintptr(unsafe.Pointer(parametersPointer)), handler.pointer())
	} else {
		hr = comCall(w.core11, 99, uintptr(unsafe.Pointer(sessionPointer)), uintptr(unsafe.Pointer(methodPointer)), uintptr(unsafe.Pointer(parametersPointer)), handler.pointer())
	}
	handler.release()
	runtime.KeepAlive(methodPointer)
	runtime.KeepAlive(parametersPointer)
	runtime.KeepAlive(sessionPointer)
	return hresultError("Start native document interception "+method, hr)
}

func (w *WebView) failDocumentFetch(err error) uintptr {
	if !w.cfg.Post(func() { w.fail(err) }) {
		return hresultFail
	}
	return hresultOK
}

func (w *WebView) documentFetchPaused(_ uintptr, args uintptr) uintptr {
	if w.closed {
		return hresultOK
	}
	raw, err := comGetString(args, 3)
	if err != nil {
		return w.failDocumentFetch(err)
	}
	event, err := decodeNativeDocumentFetchEvent(raw)
	if err != nil {
		return w.failDocumentFetch(err)
	}
	args2, err := comQuery(args, iidDevToolsEventArgs2)
	if err != nil {
		return w.failDocumentFetch(errors.New("The native document interceptor could not identify the request session"))
	}
	sessionID, err := comGetString(args2, 4)
	comRelease(args2)
	if err != nil || len(sessionID) > 256 {
		return w.failDocumentFetch(errors.New("The native document interceptor returned an invalid request session"))
	}
	pending := w.documentFetchPermit.pending
	allowed := w.documentFetchPermit.consume(w.cfg.AppURL, sessionID, event)
	method, parameters := nativeDocumentFetchCommand(event, allowed)
	if w.cfg.navigationObserver != nil {
		w.cfg.navigationObserver(nativeNavigationObservation{Kind: "fetch", URI: event.Request.URL, Blocked: !allowed, InitialDocumentPermit: pending, ProtocolMethod: method, ProtocolSession: sessionID})
	}
	if err := w.callDocumentProtocol(sessionID, method, parameters, func(err error) {
		if err != nil {
			w.fail(err)
		}
	}); err != nil {
		return w.failDocumentFetch(err)
	}
	return hresultOK
}
