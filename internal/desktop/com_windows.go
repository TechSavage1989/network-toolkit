//go:build windows

package desktop

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

// GUIDs and interface slots in this package were checked against the official
// Microsoft.Web.WebView2 1.0.4258.31 SDK WebView2.h, not inferred from examples.
type comGUID [16]byte

func guid(text string) comGUID {
	b, err := hex.DecodeString(strings.ReplaceAll(text, "-", ""))
	if err != nil || len(b) != 16 {
		panic("invalid COM interface identifier")
	}
	var out comGUID
	binary.LittleEndian.PutUint32(out[0:4], binary.BigEndian.Uint32(b[0:4]))
	binary.LittleEndian.PutUint16(out[4:6], binary.BigEndian.Uint16(b[4:6]))
	binary.LittleEndian.PutUint16(out[6:8], binary.BigEndian.Uint16(b[6:8]))
	copy(out[8:], b[8:])
	return out
}

var iidIUnknown = guid("00000000-0000-0000-c000-000000000046")
var oleMemory = syscall.NewLazyDLL("ole32.dll").NewProc("CoTaskMemFree")

const (
	hresultOK          uintptr = 0
	hresultNoInterface uintptr = 0x80004002
	hresultPointer     uintptr = 0x80004003
	hresultFail        uintptr = 0x80004005
)

func failed(hr uintptr) bool { return int32(uint32(hr)) < 0 }

func hresultError(operation string, hr uintptr) error {
	if !failed(hr) {
		return nil
	}
	return fmt.Errorf("%s failed (HRESULT 0x%08X)", operation, uint32(hr))
}

// Like syscall.Proc.Call, callers pass pointers converted to uintptr. Keep
// their referenced Go storage escaped and alive throughout this nested call.
//
//go:uintptrescapes
func comCall(object uintptr, slot int, args ...uintptr) uintptr {
	if object == 0 {
		return hresultPointer
	}
	vtbl := *(*uintptr)(unsafe.Pointer(object))
	method := *(*uintptr)(unsafe.Pointer(vtbl + uintptr(slot)*unsafe.Sizeof(uintptr(0))))
	if method == 0 {
		return hresultPointer
	}
	values := make([]uintptr, 1, len(args)+1)
	values[0] = object
	values = append(values, args...)
	hr, _, _ := syscall.SyscallN(method, values...)
	return hr
}

func comQuery(object uintptr, iid comGUID) (uintptr, error) {
	var value uintptr
	hr := comCall(object, 0, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&value)))
	runtime.KeepAlive(iid)
	if err := hresultError("QueryInterface", hr); err != nil {
		return 0, err
	}
	if value == 0 {
		return 0, fmt.Errorf("QueryInterface returned no interface")
	}
	return value, nil
}

func comAddRef(object uintptr) {
	if object != 0 {
		comCall(object, 1)
	}
}
func comRelease(object uintptr) {
	if object != 0 {
		comCall(object, 2)
	}
}

func comGetString(object uintptr, slot int) (string, error) {
	var pointer *uint16
	hr := comCall(object, slot, uintptr(unsafe.Pointer(&pointer)))
	if pointer != nil {
		defer oleMemory.Call(uintptr(unsafe.Pointer(pointer)))
	}
	if err := hresultError("Read WebView2 string", hr); err != nil {
		return "", err
	}
	if pointer == nil {
		return "", nil
	}
	chars := make([]uint16, 0, 128)
	for i := 0; i < 1<<20; i++ {
		ch := *(*uint16)(unsafe.Add(unsafe.Pointer(pointer), i*2))
		if ch == 0 {
			return string(utf16.Decode(chars)), nil
		}
		chars = append(chars, ch)
	}
	return "", fmt.Errorf("WebView2 returned an excessively long string")
}

func comPutString(object uintptr, slot int, value string) error {
	pointer, err := syscall.UTF16PtrFromString(value)
	if err != nil {
		return err
	}
	hr := comCall(object, slot, uintptr(unsafe.Pointer(pointer)))
	runtime.KeepAlive(pointer)
	return hresultError("Set WebView2 string", hr)
}

// The first field is the COM ABI vtable pointer. All callback objects remain
// rooted in callbackObjects until their final native Release, including during
// asynchronous environment/controller creation. COM owns explicit references;
// Go finalizers are deliberately not used for apartment-bound interfaces.
type comHandler struct {
	vtable *[4]uintptr
	refs   atomic.Int32
	iid    comGUID
	invoke func(uintptr, uintptr) uintptr
}

var callbackObjects = struct {
	sync.Mutex
	objects map[uintptr]*comHandler
}{objects: make(map[uintptr]*comHandler)}

var callbackVtable = [4]uintptr{
	syscall.NewCallback(callbackQuery),
	syscall.NewCallback(callbackAddRef),
	syscall.NewCallback(callbackRelease),
	syscall.NewCallback(callbackInvoke),
}

func newCOMHandler(iid comGUID, invoke func(uintptr, uintptr) uintptr) *comHandler {
	h := &comHandler{vtable: &callbackVtable, iid: iid, invoke: invoke}
	h.refs.Store(1)
	callbackObjects.Lock()
	callbackObjects.objects[uintptr(unsafe.Pointer(h))] = h
	callbackObjects.Unlock()
	return h
}

func (h *comHandler) pointer() uintptr { return uintptr(unsafe.Pointer(h)) }
func (h *comHandler) release()         { callbackRelease(h.pointer()) }

func handlerAt(this uintptr) *comHandler {
	callbackObjects.Lock()
	h := callbackObjects.objects[this]
	callbackObjects.Unlock()
	return h
}

func callbackQuery(this, iidPointer, outPointer uintptr) uintptr {
	if iidPointer == 0 || outPointer == 0 {
		return hresultPointer
	}
	*(*uintptr)(unsafe.Pointer(outPointer)) = 0
	h := handlerAt(this)
	if h == nil {
		return hresultPointer
	}
	iid := *(*comGUID)(unsafe.Pointer(iidPointer))
	if iid != iidIUnknown && iid != h.iid {
		return hresultNoInterface
	}
	h.refs.Add(1)
	*(*uintptr)(unsafe.Pointer(outPointer)) = this
	return hresultOK
}

func callbackAddRef(this uintptr) uintptr {
	h := handlerAt(this)
	if h == nil {
		return 0
	}
	return uintptr(h.refs.Add(1))
}

func callbackRelease(this uintptr) uintptr {
	h := handlerAt(this)
	if h == nil {
		return 0
	}
	n := h.refs.Add(-1)
	if n <= 0 {
		callbackObjects.Lock()
		delete(callbackObjects.objects, this)
		callbackObjects.Unlock()
		return 0
	}
	return uintptr(n)
}

func callbackInvoke(this, first, second uintptr) (hr uintptr) {
	defer func() {
		if recover() != nil {
			hr = hresultFail
		}
	}()
	h := handlerAt(this)
	if h == nil || h.invoke == nil {
		return hresultPointer
	}
	return h.invoke(first, second)
}

type eventRegistration struct {
	object     uintptr
	removeSlot int
	token      int64
}

func addCOMEvent(object uintptr, addSlot, removeSlot int, iid comGUID, callback func(uintptr, uintptr) uintptr) (eventRegistration, error) {
	handler := newCOMHandler(iid, callback)
	defer handler.release()
	registration := eventRegistration{object: object, removeSlot: removeSlot}
	hr := comCall(object, addSlot, handler.pointer(), uintptr(unsafe.Pointer(&registration.token)))
	if err := hresultError("Register WebView2 event", hr); err != nil {
		return eventRegistration{}, err
	}
	return registration, nil
}

func (r eventRegistration) remove() {
	if r.object != 0 {
		comCall(r.object, r.removeSlot, uintptr(r.token))
	}
}
