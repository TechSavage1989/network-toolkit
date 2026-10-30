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

// The page acknowledgement clears the DOM pending override; the UI dispatch
// still must verify the native capture transfer. Observe the same press until
// actual OLE admission: a lost press cancels irreversibly despite a later press.
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

// DOM releasePointerCapture clears a pending override, which WebView may not
// apply until another pointer event. Complete that ownership transition with
// documented Win32 capture APIs on our own foreground UI-thread HWND. Never
// release or capture a foreign top-level window, and never synthesize input.
type nativeFileCaptureOps struct {
	currentThread func() uint32
	identity      func(uintptr) (uint32, uint32)
	capture       func(uint32) (uintptr, bool)
	root          func(uintptr) uintptr
	set           func(uintptr)
	release       func() bool
}

func transferNativeFileCapture(hwnd uintptr, held func() bool, ops nativeFileCaptureOps) bool {
	if hwnd == 0 || !held() {
		return false
	}
	thread, pid := ops.identity(hwnd)
	if thread == 0 || pid == 0 || ops.currentThread() != thread {
		return false
	}
	captured, ok := ops.capture(thread)
	if !ok {
		return false
	}
	if captured == 0 {
		return held()
	}
	if captured != hwnd && ops.root(captured) != hwnd {
		return false
	}
	if !held() {
		return false
	}
	if captured != hwnd {
		ops.set(hwnd)
		captured, ok = ops.capture(thread)
		if !ok || captured != hwnd {
			return false
		}
	}
	// Even if the press ends here, relinquish only the capture we just verified
	// as ours; physical/context checks below still prohibit OLE admission.
	if !ops.release() {
		return false
	}
	captured, ok = ops.capture(thread)
	return ok && captured == 0 && held()
}
func (w *hostWindow) transferNativeFileCapture(ctx context.Context, hwnd uintptr, trace *nativeFileTrace) bool {
	set := fileDragUser.NewProc("SetCapture")
	release := fileDragUser.NewProc("ReleaseCapture")
	ancestor := fileDragUser.NewProc("GetAncestor")
	accepted := transferNativeFileCapture(hwnd, func() bool { return ctx.Err() == nil && !w.isClosing() && nativeFileDragHeld(hwnd) }, nativeFileCaptureOps{
		currentThread: func() uint32 { v, _, _ := fileDragCurrentThread.Call(); return uint32(v) },
		identity:      nativeFileWindowIdentity, capture: nativeFileThreadCapture,
		root: func(window uintptr) uintptr { v, _, _ := ancestor.Call(window, 2); return v },
		set:  func(window uintptr) { set.Call(window) }, release: func() bool { v, _, _ := release.Call(); return v != 0 },
	})
	status := uint32(0)
	if accepted {
		status = 1
	}
	trace.emit(NativeFileDragObservation{Kind: "capture-native-transfer", Status: status, Flags: nativeFileAdmissionFlags(ctx, hwnd, w.isClosing())})
	return accepted
}
