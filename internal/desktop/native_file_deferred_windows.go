//go:build windows

package desktop

// Deferred CF_HDROP follows the Shell's InShellDragLoop contract. No async
// capability is exposed: the private copy remains alive until DoDragDrop returns.
// https://learn.microsoft.com/en-us/windows/win32/shell/clipboard#cfstr_indragloop
import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

type nativeFormat struct {
	Format uint16
	_      [6]byte
	Target uintptr
	Aspect uint32
	Index  int32
	Medium uint32
	_      uint32
}
type nativeMedium struct {
	Medium  uint32
	_       uint32
	Handle  uintptr
	Release uintptr
}
type nativeDeferredCopy struct {
	ctx             context.Context
	client          *http.Client
	endpoint, token string
	job             nativeFileDragJob
	paths           []string
	err             error
	loading         bool
	held            func() bool
	cancel          context.CancelFunc
	dropAllowed     atomic.Bool
}

var nativeGlobal = syscall.NewLazyDLL("kernel32.dll")
var nativeAlloc = nativeGlobal.NewProc("GlobalAlloc")
var nativeLock = nativeGlobal.NewProc("GlobalLock")
var nativeUnlock = nativeGlobal.NewProc("GlobalUnlock")
var nativeFree = nativeGlobal.NewProc("GlobalFree")
var nativePeek = fileDragUser.NewProc("PeekMessageW")
var nativeRegisterClipboard = fileDragUser.NewProc("RegisterClipboardFormatW")
var nativeEnumFormats = fileDragShell.NewProc("SHCreateStdEnumFmtEtc")
var nativeInLoopFormat = nativeClipboardFormat("InShellDragLoop")
var nativePreferredFormat = nativeClipboardFormat("Preferred DropEffect")

func nativeClipboardFormat(name string) uint16 {
	p, _ := syscall.UTF16PtrFromString(name)
	v, _, _ := nativeRegisterClipboard.Call(uintptr(unsafe.Pointer(p)))
	return uint16(v)
}
func nativeFormats() []nativeFormat {
	return []nativeFormat{{Format: 15, Aspect: 1, Index: -1, Medium: 1}, {Format: nativeInLoopFormat, Aspect: 1, Index: -1, Medium: 1}, {Format: nativePreferredFormat, Aspect: 1, Index: -1, Medium: 1}}
}
func (d *nativeDeferredCopy) query(raw uintptr) uintptr {
	if raw == 0 {
		return hresultPointer
	}
	f := (*nativeFormat)(unsafe.Pointer(raw))
	if f.Aspect != 1 || f.Medium&1 == 0 {
		return 0x80040064
	}
	if f.Format == 15 || f.Format == nativeInLoopFormat || f.Format == nativePreferredFormat {
		return 0
	}
	return 0x80040064
}
func (d *nativeDeferredCopy) enumerate(direction, out uintptr) uintptr {
	if out == 0 {
		return hresultPointer
	}
	*(*uintptr)(unsafe.Pointer(out)) = 0
	if direction != 1 {
		return 0x80004001
	}
	formats := nativeFormats()
	hr, _, _ := nativeEnumFormats.Call(uintptr(len(formats)), uintptr(unsafe.Pointer(&formats[0])), out)
	runtime.KeepAlive(formats)
	return hr
}
func nativeGlobalMedium(data []byte, out uintptr) uintptr {
	if out == 0 {
		return hresultPointer
	}
	mem, _, _ := nativeAlloc.Call(0x42, uintptr(len(data)))
	if mem == 0 {
		return 0x8007000e
	}
	ptr, _, _ := nativeLock.Call(mem)
	if ptr == 0 {
		nativeFree.Call(mem)
		return 0x8007000e
	}
	copy(unsafe.Slice((*byte)(unsafe.Pointer(ptr)), len(data)), data)
	nativeUnlock.Call(mem)
	*(*nativeMedium)(unsafe.Pointer(out)) = nativeMedium{Medium: 1, Handle: mem}
	return 0
}
func (d *nativeDeferredCopy) get(raw, out uintptr) uintptr {
	if result := d.query(raw); result != 0 {
		return result
	}
	f := (*nativeFormat)(unsafe.Pointer(raw))
	pressed, _, _ := fileDragKey.Call(1)
	if d.held != nil {
		if d.held() {
			pressed = 0x8000
		} else {
			pressed = 0
		}
	}
	if f.Format == nativeInLoopFormat {
		value := uint32(0)
		if pressed&0x8000 != 0 {
			value = 1
		}
		data := make([]byte, 4)
		binary.LittleEndian.PutUint32(data, value)
		return nativeGlobalMedium(data, out)
	}
	if f.Format == nativePreferredFormat {
		return nativeGlobalMedium([]byte{1, 0, 0, 0}, out)
	}
	// A target probing while the pointer is held must not trigger a network read.
	if pressed&0x8000 != 0 || !d.dropAllowed.Load() {
		return 0x80040064
	}
	if d.loading {
		return 0x8000000a
	}
	if d.paths == nil && d.err == nil {
		d.loading = true
		done := make(chan struct{})
		go func() { d.paths, d.err = d.materialize(); close(done) }()
		for {
			select {
			case <-done:
				d.loading = false
				goto completed
			default:
			}
			// Keep this owning UI apartment responsive while the bounded backend stages
			// bytes. Reentrant GetData is refused; no application/registry lock is held.
			var msg windowMessage
			for n := 0; n < 32; n++ {
				ok, _, _ := nativePeek.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 1)
				if ok == 0 {
					break
				}
				if msg.Message == 0x12 {
					if d.cancel != nil {
						d.cancel()
					}
					defer fileDragUser.NewProc("PostQuitMessage").Call(msg.WParam)
				}
				wndTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
				wndDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
completed:
	if d.err != nil || d.ctx.Err() != nil {
		return 0x80004004
	}
	data := make([]byte, 20)
	binary.LittleEndian.PutUint32(data, 20)
	binary.LittleEndian.PutUint32(data[16:], 1)
	for _, path := range d.paths {
		u, e := syscall.UTF16FromString(path)
		if e != nil {
			return 0x80070057
		}
		for _, v := range u {
			data = append(data, byte(v), byte(v>>8))
		}
	}
	data = append(data, 0, 0)
	return nativeGlobalMedium(data, out)
}
func (d *nativeDeferredCopy) materialize() ([]string, error) {
	body, _ := json.Marshal(map[string]any{"action": "materialize", "id": d.job.ID})
	req, e := http.NewRequestWithContext(d.ctx, http.MethodPost, d.endpoint, bytes.NewReader(body))
	if e != nil {
		return nil, e
	}
	req.Header.Set("X-Session-Token", d.token)
	req.Header.Set("Content-Type", "application/json")
	res, e := d.client.Do(req)
	if e != nil {
		return nil, e
	}
	io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.New("Windows copy was not accepted")
	}
	for {
		if e := d.ctx.Err(); e != nil {
			return nil, e
		}
		esc, _, _ := fileDragKey.Call(0x1b)
		if esc&0x8000 != 0 {
			return nil, context.Canceled
		}
		req, e = http.NewRequestWithContext(d.ctx, http.MethodGet, d.endpoint+"/native?id="+d.job.ID, nil)
		if e != nil {
			return nil, e
		}
		req.Header.Set("X-Session-Token", d.token)
		res, e = d.client.Do(req)
		if e != nil {
			return nil, e
		}
		if res.StatusCode == 200 {
			var j nativeFileDragJob
			decoder := json.NewDecoder(io.LimitReader(res.Body, 64<<10))
			decoder.DisallowUnknownFields()
			e = decoder.Decode(&j)
			res.Body.Close()
			if e != nil || j.ID != d.job.ID || j.Gesture != d.job.Gesture || j.Direct || !validNativeFileDrag(j) {
				return nil, errors.New("Invalid staged copy")
			}
			return j.Paths, nil
		}
		io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
		res.Body.Close()
		if res.StatusCode != 202 {
			return nil, errors.New("Copy cancelled or failed")
		}
		select {
		case <-d.ctx.Done():
			return nil, d.ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}
func runNativeDeferredFileCopyDrag(ctx context.Context, hwnd uintptr, client *http.Client, endpoint, token string, job nativeFileDragJob) (uint32, error) {
	hr, _, _ := fileDragOle.NewProc("OleInitialize").Call(0)
	if e := hresultError("Initialize Windows file drag", hr); e != nil {
		return 0, e
	}
	defer fileDragOle.NewProc("OleUninitialize").Call()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	d := &nativeDeferredCopy{ctx: ctx, cancel: cancel, client: client, endpoint: endpoint, token: token, job: job}
	wrapper := &nativeFileCOM{vtable: &fileCOMDataTable, delayed: d}
	return runNativeFileDataDrag(ctx, hwnd, wrapper)
}
