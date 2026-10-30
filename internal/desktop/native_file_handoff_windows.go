//go:build windows

package desktop

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// This is host-to-page execution of one fixed function with an already-claimed
// opaque job, not a page-to-host messaging or filesystem capability.
func nativeFileHandoffScript(id string, gesture uint64) (string, bool) {
	if len(id) != 32 || strings.Trim(id, "0123456789abcdef") != "" || gesture != 1 {
		return "", false
	}
	return fmt.Sprintf("typeof window.NetworkToolkitNativeDrag?.releaseFileRowCapture==='function'&&window.NetworkToolkitNativeDrag.releaseFileRowCapture('%s',1)===true", id), true
}
func (w *WebView) releaseNativeFileCapture(id string, gesture uint64, done func(bool)) error {
	scriptText, ok := nativeFileHandoffScript(id, gesture)
	if !ok || w == nil || w.closed || w.core == 0 || done == nil {
		return errors.New("The file drag handoff is unavailable")
	}
	script := utf16String(scriptText)
	handler := newCOMHandler(iidScriptCompleted, func(result, jsonResult uintptr) uintptr {
		accepted := !w.closed && !failed(result) && jsonResult != 0
		if accepted {
			for i, want := range [...]uint16{'t', 'r', 'u', 'e', 0} {
				if *(*uint16)(unsafe.Add(unsafe.Pointer(jsonResult), i*2)) != want {
					accepted = false
					break
				}
			}
		}
		// Only send a copied bool to the goroutine; OLE never runs reentrantly in COM.
		done(accepted)
		return hresultOK
	})
	hr := comCall(w.core, 29, uintptr(unsafe.Pointer(script)), handler.pointer())
	handler.release()
	runtime.KeepAlive(script)
	return hresultError("Release file row capture", hr)
}
func awaitNativeFileCapture(ctx context.Context, held func() bool, begin func(func(bool)) bool) bool {
	if ctx.Err() != nil || !held() {
		return false
	}
	result := make(chan bool, 1)
	var once sync.Once
	complete := func(ok bool) { once.Do(func() { result <- ok }) }
	if !begin(complete) {
		return false
	}
	deadline := time.NewTimer(250 * time.Millisecond)
	defer deadline.Stop()
	tick := time.NewTicker(8 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case ok := <-result:
			return ok && ctx.Err() == nil && held()
		case <-ctx.Done():
			return false
		case <-deadline.C:
			return false
		case <-tick.C:
			if !held() {
				return false
			}
		}
	}
}
func (w *hostWindow) releaseNativeFileCapture(ctx context.Context, hwnd uintptr, job nativeFileDragJob, trace *nativeFileTrace) bool {
	trace.emit(NativeFileDragObservation{Kind: "capture-handoff-request"})
	accepted := awaitNativeFileCapture(ctx, func() bool { return !w.isClosing() && nativeFileDragHeld(hwnd) }, func(done func(bool)) bool {
		return w.post(func() {
			if ctx.Err() != nil || w.isClosing() || !nativeFileDragHeld(hwnd) {
				done(false)
				return
			}
			if err := w.view.releaseNativeFileCapture(job.ID, job.Gesture, done); err != nil {
				done(false)
			}
		})
	})
	status := uint32(0)
	if accepted {
		status = 1
	}
	trace.emit(NativeFileDragObservation{Kind: "capture-handoff-result", Status: status, Flags: nativeFileAdmissionFlags(ctx, hwnd, w.isClosing())})
	return accepted
}

// Capture has moved out of WebView after its acknowledgement. Keep observing
// the same physical press until the UI queue actually enters OLE; a lost press
// irreversibly cancels this context, even if another press follows.
func monitorNativeFileAdmission(ctx context.Context, cancel context.CancelFunc, entered <-chan struct{}, state *atomic.Uint32, held func() bool) {
	tick := time.NewTicker(8 * time.Millisecond)
	defer tick.Stop()
	for {
		if ctx.Err() != nil || state.Load() != 0 {
			return
		}
		select {
		case <-entered:
			return
		default:
		}
		if !held() {
			if state.CompareAndSwap(0, 2) {
				cancel()
			}
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-entered:
			return
		case <-tick.C:
		}
	}
}
