//go:build windows

package desktop

// This narrow bridge intentionally does not enable WebView host objects or web
// messaging. A caller-bound staged-copy job is armed by an explicit page drag;
// the native host claims it only while its own foreground window has a real
// left mouse button held. The OS receives copy-only staged local paths.
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var fileDragUser = syscall.NewLazyDLL("user32.dll")
var fileDragKey = fileDragUser.NewProc("GetAsyncKeyState")
var fileDragForeground = fileDragUser.NewProc("GetForegroundWindow")
var fileDragWindowThread = fileDragUser.NewProc("GetWindowThreadProcessId")
var fileDragThreadInfo = fileDragUser.NewProc("GetGUIThreadInfo")
var fileDragCurrentThread = syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentThreadId")
var fileDragOle = syscall.NewLazyDLL("ole32.dll")
var fileDragDo = fileDragOle.NewProc("DoDragDrop")
var fileDragShell = syscall.NewLazyDLL("shell32.dll")
var fileDragPIDL = fileDragShell.NewProc("ILCreateFromPathW")
var fileDragLast = fileDragShell.NewProc("ILFindLastID")
var fileDragFree = fileDragShell.NewProc("ILFree")
var fileDragCreate = fileDragShell.NewProc("SHCreateDataObject")
var fileDragIData = guid("0000010e-0000-0000-c000-000000000046")
var fileDragISource = guid("00000121-0000-0000-c000-000000000046")

type nativeFileDragJob struct {
	ID      string   `json:"id"`
	Gesture uint64   `json:"gesture"`
	Paths   []string `json:"paths"`
	Direct  bool     `json:"direct,omitempty"`
}

func nativeFileDragEndpoint(raw string) (string, bool) {
	u, e := url.Parse(raw)
	if e != nil || !sameAppOrigin(raw, raw, false) || (u.Path != "/" && !(strings.HasPrefix(u.Path, "/editor-view/") && len(strings.TrimPrefix(u.Path, "/editor-view/")) == 32 && strings.Trim(strings.TrimPrefix(u.Path, "/editor-view/"), "0123456789abcdef") == "")) {
		return "", false
	}
	return u.Scheme + "://" + u.Host + "/api/explorer-transfer", true
}
func validNativeFileDrag(j nativeFileDragJob) bool {
	if j.Direct {
		return len(j.ID) == 32 && strings.Trim(j.ID, "0123456789abcdef") == "" && j.Gesture > 0 && len(j.Paths) == 0
	}
	if len(j.ID) != 32 || strings.Trim(j.ID, "0123456789abcdef") != "" || len(j.Paths) < 1 || len(j.Paths) > 100 {
		return false
	}
	parent := ""
	for _, p := range j.Paths {
		if !filepath.IsAbs(p) || len(p) > 32760 || strings.ContainsRune(p, 0) {
			return false
		}
		dir := filepath.Dir(p)
		if parent == "" {
			parent = dir
		} else if !strings.EqualFold(parent, dir) {
			return false
		}
		info, e := os.Lstat(p)
		if e != nil || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 || !info.IsDir() && !info.Mode().IsRegular() {
			return false
		}
	}
	// Only our private per-job export roots can reach the shell. Filesystem paths
	// supplied in page drag data are never accepted by this bridge.
	return strings.HasPrefix(filepath.Base(parent), "ntk-explorer-copy-") && strings.EqualFold(filepath.Dir(parent), filepath.Clean(os.TempDir()))
}
func nativeFileDragHeld(hwnd uintptr) bool {
	pressed, _, _ := fileDragKey.Call(1)
	front, _, _ := fileDragForeground.Call()
	return pressed&0x8000 != 0 && front == hwnd
}
func nativeFileDragContext(parent context.Context, done <-chan struct{}) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	go func() {
		select {
		case <-done:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}
func (w *hostWindow) runNativeFileDrag(done <-chan struct{}) {
	ctx, cancel := nativeFileDragContext(w.ctx, done)
	defer cancel()
	endpoint, ok := nativeFileDragEndpoint(w.cfg.URL)
	if !ok {
		return
	}
	client := &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	interval := 100 * time.Millisecond
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		hwnd := w.postedHandle.Load()
		if hwnd == 0 || !nativeFileDragHeld(hwnd) {
			if interval != 100*time.Millisecond {
				interval = 100 * time.Millisecond
				tick.Reset(interval)
			}
			continue
		}
		// Only a held foreground gesture needs low-latency admission. Idle windows
		// retain the original polling cadence and issue no HTTP requests.
		if interval != 8*time.Millisecond {
			interval = 8 * time.Millisecond
			tick.Reset(interval)
		}
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/native", nil)
		if e != nil {
			return
		}
		req.Header.Set("X-Session-Token", w.cfg.SessionToken)
		res, e := client.Do(req)
		if e != nil {
			continue
		}
		if res.StatusCode != 200 {
			io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
			res.Body.Close()
			continue
		}
		var job nativeFileDragJob
		d := json.NewDecoder(io.LimitReader(res.Body, 64<<10))
		d.DisallowUnknownFields()
		e = d.Decode(&job)
		res.Body.Close()
		if e != nil || !validNativeFileDrag(job) {
			continue
		}
		trace := newNativeFileTrace(hwnd)
		jobCtx := ctx
		if trace != nil {
			jobCtx = context.WithValue(ctx, nativeFileTraceContextKey{}, trace)
			trace.emit(NativeFileDragObservation{Kind: "claim-received", Flags: nativeFileAdmissionFlags(ctx, hwnd, w.isClosing())})
		}
		// A known start acknowledgement is required before OLE; ambiguous HTTP
		// outcomes are reconciled on the same ID, never by starting a second drag.
		if !nativeFileDragTransition(jobCtx, client, endpoint, w.cfg.SessionToken, job.ID, job.Gesture, "start", "") {
			continue
		}
		finished := make(chan struct{})
		dragCtx, cancelDrag := context.WithCancel(jobCtx)
		go monitorNativeFileDragLease(dragCtx, cancelDrag, finished, client, endpoint, w.cfg.SessionToken, job.ID)
		outcome := "cancelled"
		admissionDone := make(chan struct{})
		var admissionOnce sync.Once
		var admissionState atomic.Uint32 // 0 waiting, 1 OLE admitted, 2 ended/cancelled
		endAdmission := func() { admissionState.CompareAndSwap(0, 2); admissionOnce.Do(func() { close(admissionDone) }) }
		if job.Direct {
			go monitorNativeFileAdmission(dragCtx, cancelDrag, admissionDone, &admissionState, func() bool { return !w.isClosing() && nativeFileDragHeld(hwnd) })
		}
		admitted := !job.Direct || w.releaseNativeFileCapture(dragCtx, hwnd, job, trace)
		trace.emit(NativeFileDragObservation{Kind: "dispatch-posted"})
		if !admitted || !w.post(func() {
			defer close(finished)
			defer endAdmission()
			if trace != nil {
				trace.emit(NativeFileDragObservation{Kind: "dispatch-enter", Flags: nativeFileAdmissionFlags(dragCtx, hwnd, w.isClosing())})
			}
			if dragCtx.Err() == nil && !w.isClosing() && nativeFileDragHeld(hwnd) && (!job.Direct || admissionState.CompareAndSwap(0, 1)) {
				endAdmission() // OLE now owns physical release/cancellation observation.
				var effect uint32
				var err error
				if job.Direct {
					effect, err = runNativeDeferredFileCopyDrag(dragCtx, hwnd, client, endpoint, w.cfg.SessionToken, job)
				} else {
					effect, err = runNativeFileCopyDrag(dragCtx, hwnd, job.Paths)
				}
				if err != nil {
					outcome = "failed"
				} else if effect&1 != 0 {
					outcome = "copied"
				}
			} else {
				trace.emit(NativeFileDragObservation{Kind: "dispatch-not-held-or-closing"})
			}
		}) {
			trace.emit(NativeFileDragObservation{Kind: "dispatch-post-failed"})
			endAdmission()
			close(finished)
		}
		// A canceled owner cannot reclaim bytes underneath the active OLE call.
		// Wait for genuine input to let its source report cancellation, then send
		// only the existing idempotent cleanup receipt with unchanged credentials.
		select {
		case <-finished:
		case <-done:
		}
		cancelDrag()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Second)
		if trace != nil {
			cleanupCtx = context.WithValue(cleanupCtx, nativeFileTraceContextKey{}, trace)
		}
		confirmed := nativeFileDragTransition(cleanupCtx, client, endpoint, w.cfg.SessionToken, job.ID, job.Gesture, "finish", outcome)
		cleanupCancel()
		observeNativeFileDrag(hwnd, fmt.Sprintf("finish-receipt confirmed=%t outcome=%s", confirmed, outcome))
		if !confirmed || ctx.Err() != nil {
			return
		}

	}
}

// Transport failure is not proof that a state transition did not happen.
// Repeating these two ID-bound transitions is explicitly idempotent server-side.
func nativeFileDragTransition(ctx context.Context, client *http.Client, endpoint, token, id string, gesture uint64, action, outcome string) bool {
	body, _ := json.Marshal(map[string]any{"action": action, "id": id, "gesture": gesture, "outcome": outcome})
	for ctx.Err() == nil {
		trace := nativeTraceFromContext(ctx)
		trace.emit(NativeFileDragObservation{Kind: action + "-request"})
		req, e := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if e != nil {
			return false
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-Token", token)
		res, e := client.Do(req)
		status := uint32(0)
		if res != nil {
			status = uint32(res.StatusCode)
		}
		trace.emit(NativeFileDragObservation{Kind: action + "-response", Status: status})
		if e == nil {
			io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
			res.Body.Close()
			if res.StatusCode == 200 {
				return true
			}
			if res.StatusCode >= 400 && res.StatusCode < 500 {
				return false
			}
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(250 * time.Millisecond):
		}
	}
	return false
}

// The data wrapper exposes IDataObject only, deliberately NOT asynchronous
// shell capability. Explorer must finish reading staged files before DoDragDrop
// returns and the owner removes them. No move effect is ever offered.
type nativeFileCOM struct {
	vtable  *[12]uintptr
	refs    atomic.Int32
	base    uintptr
	source  bool
	ctx     context.Context
	hwnd    uintptr
	active  atomic.Bool
	delayed *nativeDeferredCopy
	trace   *nativeFileTrace
}

type nativeFileTrace struct {
	callback func(NativeFileDragObservation)
	count    atomic.Uint32
	feedback atomic.Uint32
	sequence uint64
	formats  sync.Map
}

type nativeFileTraceContextKey struct{}

var nativeFileTraceSequence atomic.Uint64
var nativeFileTraceTick = syscall.NewLazyDLL("kernel32.dll").NewProc("GetTickCount64")

func newNativeFileTrace(hwnd uintptr) *nativeFileTrace {
	callback := nativeFileTraceDiagnosticSink()
	if value, ok := wndWindows.Load(hwnd); ok && value.(*hostWindow).cfg.NativeFileDragObserver != nil {
		callback = value.(*hostWindow).cfg.NativeFileDragObserver
	}
	if callback == nil {
		return nil
	}
	return &nativeFileTrace{callback: callback, sequence: nativeFileTraceSequence.Add(1)}
}
func nativeTraceFromContext(ctx context.Context) *nativeFileTrace {
	t, _ := ctx.Value(nativeFileTraceContextKey{}).(*nativeFileTrace)
	return t
}
func nativeFileAdmissionFlags(ctx context.Context, hwnd uintptr, closing bool) uint32 {
	var flags uint32
	pressed, _, _ := fileDragKey.Call(1)
	if pressed&0x8000 != 0 {
		flags |= 1
	}
	front, _, _ := fileDragForeground.Call()
	if front == hwnd {
		flags |= 2
	}
	if closing {
		flags |= 4
	}
	if ctx.Err() != nil {
		flags |= 8
	}
	tid, pid := nativeFileWindowIdentity(hwnd)
	if captured, ok := nativeFileThreadCapture(tid); ok && captured != 0 {
		flags |= 16
		capturedThread, capturedProcess := nativeFileWindowIdentity(captured)
		if capturedThread == tid && capturedProcess == pid {
			flags |= 32
		}
		ancestor, _, _ := fileDragUser.NewProc("GetAncestor").Call(captured, 2)
		if ancestor == hwnd {
			flags |= 64
		}
	}
	return flags
}
func (t *nativeFileTrace) emit(event NativeFileDragObservation) {
	if t != nil && t.callback != nil && (event.Kind == "ole-return" || t.count.Add(1) <= 127) {
		tick, _, _ := nativeFileTraceTick.Call()
		event.TickMS = uint64(tick)
		event.Sequence = t.sequence
		t.callback(event)
	}
}
func (t *nativeFileTrace) format(kind string, raw, result uintptr) {
	if t == nil || t.callback == nil || t.count.Load() >= 127 {
		return
	}
	event := NativeFileDragObservation{Kind: kind, HRESULT: uint32(result)}
	if raw != 0 {
		f := (*nativeFormat)(unsafe.Pointer(raw))
		event.Format = uint32(f.Format)
		event.Aspect = f.Aspect
		event.Medium = f.Medium
	}
	// Repeated target probes must not exhaust the bounded trace before DROP.
	key := struct {
		kind                           string
		format, aspect, medium, result uint32
	}{kind, event.Format, event.Aspect, event.Medium, event.HRESULT}
	if _, seen := t.formats.LoadOrStore(key, struct{}{}); seen {
		return
	}
	t.emit(event)
}

var nativeFileCOMObjects sync.Map
var nativeFileDragOwners sync.Map

// Populated only by the in-package synthetic native fixture. No web/API hook,
// production logging, file paths or connection data are exposed.
var nativeFileDragObservers sync.Map

func observeNativeFileDrag(hwnd uintptr, event string) {
	if value, ok := nativeFileDragObservers.Load(hwnd); ok {
		value.(func(string))(event)
	}
}

type nativeFileDragOwner struct {
	window       *hostWindow
	active       *atomic.Bool
	threadID     uint32
	closePending atomic.Bool
}

type nativeFileGUIThreadInfo struct {
	Size, Flags                                        uint32
	Active, Focus, Capture, MenuOwner, MoveSize, Caret uintptr
	CaretRect                                          Rect
}

func nativeFileWindowIdentity(hwnd uintptr) (uint32, uint32) {
	var pid uint32
	tid, _, _ := fileDragWindowThread.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return uint32(tid), pid
}
func nativeFileThreadCapture(tid uint32) (uintptr, bool) {
	if tid == 0 {
		return 0, false
	}
	info := nativeFileGUIThreadInfo{Size: uint32(unsafe.Sizeof(nativeFileGUIThreadInfo{}))}
	ok, _, _ := fileDragThreadInfo.Call(uintptr(tid), uintptr(unsafe.Pointer(&info)))
	return info.Capture, ok != 0
}

func nativeFileDragCancellationTarget(hwnd uintptr) bool {
	raw, ok := nativeFileDragOwners.Load(hwnd)
	if !ok {
		return false
	}
	owner := raw.(*nativeFileDragOwner)
	current, ok := wndWindows.Load(hwnd)
	return ok && current == owner.window && owner.active.Load()
}

// OLE may capture through its own helper HWND, not the visible host. Resolve
// only the recorded active owner's exact UI thread; never the foreground thread.
func nativeFileDragCaptureTarget(hwnd uintptr, identity func(uintptr) (uint32, uint32), capture func(uint32) (uintptr, bool)) uintptr {
	raw, ok := nativeFileDragOwners.Load(hwnd)
	if !ok || !nativeFileDragCancellationTarget(hwnd) {
		return 0
	}
	owner := raw.(*nativeFileDragOwner)
	tid, pid := identity(hwnd)
	if tid == 0 || tid != owner.threadID || pid != uint32(os.Getpid()) {
		return 0
	}
	target, ok := capture(tid)
	if !ok || target == 0 {
		return 0
	}
	targetThread, targetProcess := identity(target)
	if targetThread != tid || targetProcess != pid {
		return 0
	}
	current, ok := nativeFileDragOwners.Load(hwnd)
	if !ok || current != raw || !nativeFileDragCancellationTarget(hwnd) {
		return 0
	}
	return target
}

// Window teardown waits for the active OLE callback to return. OLE asks its
// source whether to cancel on the next genuine input-state change.
func deferNativeFileDragClose(hwnd uintptr) bool {
	raw, ok := nativeFileDragOwners.Load(hwnd)
	if !ok || !nativeFileDragCancellationTarget(hwnd) {
		return false
	}
	owner := raw.(*nativeFileDragOwner)
	if !owner.requestClose() || !nativeFileDragCancellationTarget(hwnd) {
		return false
	}
	observeNativeFileDrag(hwnd, "close-deferred-until-ole-return")
	return true
}

// If OLE completed before the pending flag became visible, the caller must
// post the ordinary close itself. Atomic ordering prevents a lost close.
func (owner *nativeFileDragOwner) requestClose() bool {
	owner.closePending.Store(true)
	return owner.active.Load()
}

func nativeFileObject(p uintptr) *nativeFileCOM {
	v, _ := nativeFileCOMObjects.Load(p)
	o, _ := v.(*nativeFileCOM)
	return o
}
func fileCOMQuery(p, iid, out uintptr) uintptr {
	o := nativeFileObject(p)
	if o == nil || iid == 0 || out == 0 {
		return hresultPointer
	}
	*(*uintptr)(unsafe.Pointer(out)) = 0
	g := *(*comGUID)(unsafe.Pointer(iid))
	if g != iidIUnknown && (!o.source && g != fileDragIData || o.source && g != fileDragISource) {
		return hresultNoInterface
	}
	o.refs.Add(1)
	*(*uintptr)(unsafe.Pointer(out)) = p
	return 0
}
func fileCOMAdd(p uintptr) uintptr {
	o := nativeFileObject(p)
	if o == nil {
		return 0
	}
	return uintptr(o.refs.Add(1))
}
func fileCOMRelease(p uintptr) uintptr {
	o := nativeFileObject(p)
	if o == nil {
		return 0
	}
	n := o.refs.Add(-1)
	if n == 0 {
		nativeFileCOMObjects.Delete(p)
		if o.base != 0 {
			comRelease(o.base)
			o.base = 0
		}
	}
	return uintptr(n)
}
func fileCOMGet(p, f, m uintptr) uintptr {
	o := nativeFileObject(p)
	if o == nil || !o.active.Load() {
		return hresultPointer
	}
	if o.delayed != nil {
		result := o.delayed.get(f, m)
		o.trace.format("get-data", f, result)
		return result
	}
	return comCall(o.base, 3, f, m)
}
func fileCOMQueryData(p, f uintptr) uintptr {
	o := nativeFileObject(p)
	if o == nil || !o.active.Load() {
		return hresultPointer
	}
	if o.delayed != nil {
		result := o.delayed.query(f)
		o.trace.format("query-data", f, result)
		return result
	}
	return comCall(o.base, 5, f)
}
func fileCOMEnum(p, d, out uintptr) uintptr {
	o := nativeFileObject(p)
	if o == nil || !o.active.Load() {
		return hresultPointer
	}
	if o.delayed != nil {
		result := o.delayed.enumerate(d, out)
		o.trace.emit(NativeFileDragObservation{Kind: "enum-format", Medium: uint32(d), HRESULT: uint32(result)})
		return result
	}
	return comCall(o.base, 8, d, out)
}
func fileCOMNotImpl(p, a, b uintptr) uintptr      { return 0x80004001 }
func fileCOMSet(p, a, b, c uintptr) uintptr       { return 0x80004001 }
func fileCOMAdvise(p, a, b, c, d uintptr) uintptr { return 0x80040003 }
func fileCOMUnadvise(p, a uintptr) uintptr        { return 0x80040003 }
func fileCOMContinue(p, escape, key uintptr) uintptr {
	o := nativeFileObject(p)
	if o != nil && o.ctx != nil && o.ctx.Err() != nil {
		observeNativeFileDrag(o.hwnd, "query-continue-cancel")
		o.trace.emit(NativeFileDragObservation{Kind: "lease-cancel"})
		return 0x40101
	}
	if escape != 0 {
		if o != nil {
			o.trace.emit(NativeFileDragObservation{Kind: "escape-cancel"})
		}
		if o != nil && o.delayed != nil && o.delayed.cancel != nil {
			o.delayed.cancel()
		}
		return 0x40101
	}
	if key&1 == 0 {
		if o != nil {
			o.trace.emit(NativeFileDragObservation{Kind: "drop-key-release"})
		}
		if o != nil && o.delayed != nil {
			o.delayed.dropAllowed.Store(true)
		}
		return 0x40100
	}
	if o != nil && o.ctx != nil {
		pressed, _, _ := fileDragKey.Call(1)
		if pressed&0x8000 == 0 {
			o.trace.emit(NativeFileDragObservation{Kind: "drop-physical-release"})
			if o.delayed != nil {
				o.delayed.dropAllowed.Store(true)
			}
			return 0x40100
		}
	}
	return 0
}
func fileCOMFeedback(p, effect uintptr) uintptr {
	if o := nativeFileObject(p); o != nil && o.trace != nil && o.trace.feedback.Swap(uint32(effect)+1) != uint32(effect)+1 {
		o.trace.emit(NativeFileDragObservation{Kind: "feedback", Effect: uint32(effect)})
	}
	return 0x40102
}

var fileCOMDataTable = [12]uintptr{syscall.NewCallback(fileCOMQuery), syscall.NewCallback(fileCOMAdd), syscall.NewCallback(fileCOMRelease), syscall.NewCallback(fileCOMGet), syscall.NewCallback(fileCOMNotImpl), syscall.NewCallback(fileCOMQueryData), syscall.NewCallback(fileCOMNotImpl), syscall.NewCallback(fileCOMSet), syscall.NewCallback(fileCOMEnum), syscall.NewCallback(fileCOMAdvise), syscall.NewCallback(fileCOMUnadvise), syscall.NewCallback(fileCOMNotImpl)}
var fileCOMSourceTable = [12]uintptr{syscall.NewCallback(fileCOMQuery), syscall.NewCallback(fileCOMAdd), syscall.NewCallback(fileCOMRelease), syscall.NewCallback(fileCOMContinue), syscall.NewCallback(fileCOMFeedback)}

func runNativeFileCopyDrag(ctx context.Context, hwnd uintptr, paths []string) (uint32, error) {
	hrInit, _, _ := fileDragOle.NewProc("OleInitialize").Call(0)
	if err := hresultError("Initialize Windows file drag", hrInit); err != nil {
		return 0, err
	}
	defer fileDragOle.NewProc("OleUninitialize").Call()
	parent, e := syscall.UTF16PtrFromString(filepath.Dir(paths[0]))
	if e != nil {
		return 0, e
	}
	pidl, _, _ := fileDragPIDL.Call(uintptr(unsafe.Pointer(parent)))
	if pidl == 0 {
		return 0, syscall.EINVAL
	}
	defer fileDragFree.Call(pidl)
	children := make([]uintptr, 0, len(paths))
	absolute := make([]uintptr, 0, len(paths))
	defer func() {
		for _, p := range absolute {
			fileDragFree.Call(p)
		}
	}()
	for _, p := range paths {
		str, e := syscall.UTF16PtrFromString(p)
		if e != nil {
			return 0, e
		}
		id, _, _ := fileDragPIDL.Call(uintptr(unsafe.Pointer(str)))
		runtime.KeepAlive(str)
		if id == 0 {
			return 0, syscall.EINVAL
		}
		absolute = append(absolute, id)
		last, _, _ := fileDragLast.Call(id)
		children = append(children, last)
	}
	var data uintptr
	hr, _, _ := fileDragCreate.Call(pidl, uintptr(len(children)), uintptr(unsafe.Pointer(&children[0])), 0, uintptr(unsafe.Pointer(&fileDragIData)), uintptr(unsafe.Pointer(&data)))
	if e := hresultError("Prepare Explorer copy", hr); e != nil {
		return 0, e
	}

	wrapper := &nativeFileCOM{vtable: &fileCOMDataTable, base: data}
	effect, err := runNativeFileDataDrag(ctx, hwnd, wrapper)
	runtime.KeepAlive(children)
	runtime.KeepAlive(parent)
	return effect, err
}
func runNativeFileDataDrag(ctx context.Context, hwnd uintptr, wrapper *nativeFileCOM) (uint32, error) {
	wrapper.trace = nativeTraceFromContext(ctx)
	if wrapper.trace == nil {
		wrapper.trace = newNativeFileTrace(hwnd)
	}
	wrapper.refs.Store(1)
	wrapper.active.Store(true)
	source := &nativeFileCOM{vtable: &fileCOMSourceTable, source: true, ctx: ctx, hwnd: hwnd, delayed: wrapper.delayed, trace: wrapper.trace}
	source.active.Store(true)
	source.refs.Store(1)
	wp, sp := uintptr(unsafe.Pointer(wrapper)), uintptr(unsafe.Pointer(source))
	nativeFileCOMObjects.Store(wp, wrapper)
	nativeFileCOMObjects.Store(sp, source)
	defer fileCOMRelease(wp)
	defer fileCOMRelease(sp)
	defer wrapper.active.Store(false)
	if current, ok := wndWindows.Load(hwnd); ok {
		threadID, _, _ := fileDragCurrentThread.Call()
		owner := &nativeFileDragOwner{window: current.(*hostWindow), active: &source.active, threadID: uint32(threadID)}
		nativeFileDragOwners.Store(hwnd, owner)
		defer func() {
			source.active.Store(false)
			nativeFileDragOwners.CompareAndDelete(hwnd, owner)
			if owner.closePending.Load() {
				if current, ok := wndWindows.Load(hwnd); ok && current == owner.window {
					wndPostMessage.Call(hwnd, wmClose, 0, 0)
				}
			}
		}()
	}
	defer source.active.Store(false)
	var effect uint32
	observeNativeFileDrag(hwnd, "ole-enter")
	if wrapper.trace != nil {
		wrapper.trace.emit(NativeFileDragObservation{Kind: "ole-enter", Flags: nativeFileAdmissionFlags(ctx, hwnd, false)})
	}
	hr, _, _ := fileDragDo.Call(wp, sp, 1, uintptr(unsafe.Pointer(&effect)))
	observeNativeFileDrag(hwnd, fmt.Sprintf("ole-return hr=%x effect=%d", hr, effect))
	if wrapper.trace != nil {
		wrapper.trace.emit(NativeFileDragObservation{Kind: "ole-return", HRESULT: uint32(hr), Effect: effect, Flags: nativeFileAdmissionFlags(ctx, hwnd, false)})
	}
	runtime.KeepAlive(wrapper)
	runtime.KeepAlive(source)
	return effect, hresultError("Windows Explorer copy", hr)
}

// The lease status read is coalesced and limited to this already-started job.
// Denial, unknown status or owner transport loss cancels rather than granting
// another operation. No network device is contacted by this check.
func monitorNativeFileDragLease(ctx context.Context, cancel context.CancelFunc, finished <-chan struct{}, client *http.Client, endpoint, token, id string) {
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-finished:
			return
		case <-tick.C:
		}
		requestCtx, stop := context.WithTimeout(ctx, time.Second)
		req, err := http.NewRequestWithContext(requestCtx, "GET", endpoint+"?id="+url.QueryEscape(id), nil)
		if err != nil {
			stop()
			cancel()
			return
		}
		req.Header.Set("X-Session-Token", token)
		res, err := client.Do(req)
		valid := false
		if err == nil {
			var state struct {
				State           string `json:"state"`
				CancelRequested bool   `json:"cancelRequested"`
			}
			decodeErr := json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&state)
			res.Body.Close()
			valid = res.StatusCode == 200 && decodeErr == nil && state.State == "dragging" && !state.CancelRequested
		}
		stop()
		if !valid {
			status := uint32(0)
			if res != nil {
				status = uint32(res.StatusCode)
			}
			nativeTraceFromContext(ctx).emit(NativeFileDragObservation{Kind: "lease-check-rejected", Status: status})
			cancel()
			return
		}
	}
}
