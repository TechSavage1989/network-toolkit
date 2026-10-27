//go:build windows

package desktop

import (
	"errors"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

// SDK ICoreWebView2AddScriptToExecuteOnDocumentCreatedCompletedHandler.
// Invoke receives a borrowed UTF-16 script ID, not a COM interface pointer.
var iidAddScriptCompleted = guid("b99369f3-9b11-47b5-bc6f-8e7895fcea17")

func nativeBootstrapScriptID(pointer uintptr) (string, error) {
	if pointer == 0 {
		return "", errors.New("The native interface bootstrap returned no script identity")
	}
	chars := make([]uint16, 0, 64)
	for i := 0; i < 256; i++ {
		ch := *(*uint16)(unsafe.Add(unsafe.Pointer(pointer), i*2))
		if ch == 0 {
			if len(chars) == 0 {
				break
			}
			return string(utf16.Decode(chars)), nil
		}
		chars = append(chars, ch)
	}
	return "", errors.New("The native interface bootstrap returned an invalid script identity")
}

// Navigation must wait for successful asynchronous script registration. A
// missing callback, failed registration, closed host or refused dispatch never
// falls back to unauthenticated navigation or a token-bearing HTTP response.
func (w *WebView) installNativeBootstrap() error {
	if w == nil || w.closed || w.core == 0 || w.cfg.Post == nil {
		return errors.New("The native interface is unavailable for private bootstrap")
	}
	script, err := nativeSessionBootstrap(w.cfg.AppURL, w.cfg.SessionToken)
	if err != nil {
		return err
	}
	source, err := syscall.UTF16PtrFromString(script)
	if err != nil {
		return errors.New("The native interface bootstrap could not be prepared")
	}
	handler := newCOMHandler(iidAddScriptCompleted, func(result, idPointer uintptr) uintptr {
		if w.closed {
			return hresultOK
		}
		err := hresultError("Install the private interface bootstrap", result)
		id := ""
		if err == nil {
			id, err = nativeBootstrapScriptID(idPointer)
		}
		// Copy the borrowed ID before returning from the callback. Run navigation
		// outside the callback, so close/error paths cannot reenter COM teardown.
		if !w.cfg.Post(func() {
			if w.closed {
				return
			}
			if err != nil {
				w.fail(err)
				return
			}
			w.bootstrapScriptID = id
			if !w.documentRequestPermit.begin() {
				w.fail(errors.New("The native interface document was already started; close and reopen the window"))
				return
			}
			if err := comPutString(w.core, 5, w.cfg.AppURL); err != nil {
				w.fail(err)
			}
		}) {
			return hresultFail
		}
		return hresultOK
	})
	hr := comCall(w.core, 27, uintptr(unsafe.Pointer(source)), handler.pointer())
	handler.release()
	runtime.KeepAlive(source)
	return hresultError("Register the private interface bootstrap", hr)
}
