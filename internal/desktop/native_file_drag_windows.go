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
}

func nativeFileDragEndpoint(raw string) (string, bool) {
	u, e := url.Parse(raw)
	if e != nil || !sameAppOrigin(raw, raw, false) || (u.Path != "/" && !(strings.HasPrefix(u.Path, "/editor-view/") && len(strings.TrimPrefix(u.Path, "/editor-view/")) == 32 && strings.Trim(strings.TrimPrefix(u.Path, "/editor-view/"), "0123456789abcdef") == "")) {
		return "", false
	}
	return u.Scheme + "://" + u.Host + "/api/explorer-transfer", true
}
func validNativeFileDrag(j nativeFileDragJob) bool {
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
	tick := time.NewTicker(100 * time.Millisecond)
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
			continue
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
		// A known start acknowledgement is required before OLE; ambiguous HTTP
		// outcomes are reconciled on the same ID, never by starting a second drag.
		if !nativeFileDragTransition(ctx, client, endpoint, w.cfg.SessionToken, job.ID, job.Gesture, "start", "") {
			continue
		}
		finished := make(chan struct{})
		outcome := "cancelled"
		if !w.post(func() {
			defer close(finished)
			if !w.isClosing() && nativeFileDragHeld(hwnd) {
				effect, err := runNativeFileCopyDrag(ctx, hwnd, job.Paths)
				if err != nil {
					outcome = "failed"
				} else if effect&1 != 0 {
					outcome = "copied"
				}
			}
		}) {
			close(finished)
		}
		select {
		case <-finished:
		case <-done:
			return
		case <-ctx.Done():
			return
		}
		if !nativeFileDragTransition(ctx, client, endpoint, w.cfg.SessionToken, job.ID, job.Gesture, "finish", outcome) {
			return
		}

	}
}

// Transport failure is not proof that a state transition did not happen.
// Repeating these two ID-bound transitions is explicitly idempotent server-side.
func nativeFileDragTransition(ctx context.Context, client *http.Client, endpoint, token, id string, gesture uint64, action, outcome string) bool {
	body, _ := json.Marshal(map[string]any{"action": action, "id": id, "gesture": gesture, "outcome": outcome})
	for ctx.Err() == nil {
		req, e := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if e != nil {
			return false
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-Token", token)
		res, e := client.Do(req)
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
	vtable *[12]uintptr
	refs   atomic.Int32
	base   uintptr
	source bool
	ctx    context.Context
	active atomic.Bool
}

var nativeFileCOMObjects sync.Map
var nativeFileDragOwners sync.Map

type nativeFileDragOwner struct {
	window *hostWindow
	active *atomic.Bool
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
func cancelNativeFileDrag(hwnd uintptr) {
	if nativeFileDragCancellationTarget(hwnd) {
		wndPostMessage.Call(hwnd, 0x001f, 0, 0)
	}
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
	return comCall(o.base, 3, f, m)
}
func fileCOMQueryData(p, f uintptr) uintptr {
	o := nativeFileObject(p)
	if o == nil || !o.active.Load() {
		return hresultPointer
	}
	return comCall(o.base, 5, f)
}
func fileCOMEnum(p, d, out uintptr) uintptr {
	o := nativeFileObject(p)
	if o == nil || !o.active.Load() {
		return hresultPointer
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
		return 0x40101
	}
	if escape != 0 {
		return 0x40101
	}
	if key&1 == 0 {
		return 0x40100
	}
	if o != nil && o.ctx != nil {
		pressed, _, _ := fileDragKey.Call(1)
		if pressed&0x8000 == 0 {
			return 0x40100
		}
	}
	return 0
}
func fileCOMFeedback(p, effect uintptr) uintptr { return 0x40102 }

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
	wrapper.refs.Store(1)
	wrapper.active.Store(true)
	source := &nativeFileCOM{vtable: &fileCOMSourceTable, source: true, ctx: ctx}
	source.active.Store(true)
	source.refs.Store(1)
	wp, sp := uintptr(unsafe.Pointer(wrapper)), uintptr(unsafe.Pointer(source))
	nativeFileCOMObjects.Store(wp, wrapper)
	nativeFileCOMObjects.Store(sp, source)
	defer fileCOMRelease(wp)
	defer fileCOMRelease(sp)
	defer wrapper.active.Store(false)
	if current, ok := wndWindows.Load(hwnd); ok {
		owner := &nativeFileDragOwner{window: current.(*hostWindow), active: &source.active}
		nativeFileDragOwners.Store(hwnd, owner)
		defer nativeFileDragOwners.CompareAndDelete(hwnd, owner)
	}
	dragEnded := make(chan struct{})
	defer close(dragEnded)
	defer source.active.Store(false)
	// WM_CANCELMODE is delivered only to our own window. Its default handler
	// releases this UI thread's capture; no global key/mouse input is injected.
	go func() {
		select {
		case <-ctx.Done():
			cancelNativeFileDrag(hwnd)
		case <-dragEnded:
		}
	}()
	var effect uint32
	hr, _, _ = fileDragDo.Call(wp, sp, 1, uintptr(unsafe.Pointer(&effect)))
	runtime.KeepAlive(wrapper)
	runtime.KeepAlive(source)
	runtime.KeepAlive(children)
	runtime.KeepAlive(parent)
	return effect, hresultError("Windows Explorer copy", hr)
}
