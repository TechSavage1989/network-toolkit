//go:build windows

package desktop

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

func TestCOMCallbackIdentityAndLifetime(t *testing.T) {
	iid := iidEnvironmentCompleted
	called := false
	h := newCOMHandler(iid, func(first, second uintptr) uintptr { called = first == 0x1234 && second == 0x5678; return hresultOK })
	pointer := h.pointer()
	queried, err := comQuery(pointer, iid)
	if err != nil || queried != pointer {
		t.Fatalf("handler interface unavailable: %v", err)
	}
	comRelease(queried)
	unknown, err := comQuery(pointer, iidIUnknown)
	if err != nil || unknown != pointer {
		t.Fatalf("IUnknown identity mismatch: %v", err)
	}
	comRelease(unknown)
	if _, err = comQuery(pointer, iidSettings4); err == nil {
		t.Fatal("handler claimed an unsupported interface")
	}
	runtime.GC()
	if failed(comCall(pointer, 3, 0x1234, 0x5678)) || !called {
		t.Fatal("retained callback did not invoke through its native vtable")
	}
	h.release()
	if handlerAt(pointer) != nil {
		t.Fatal("final COM Release did not remove the callback root")
	}
}

func TestCOMOutParameterRemainsLiveAcrossCallback(t *testing.T) {
	type fakeObject struct{ vtable *[4]uintptr }
	vtbl := [4]uintptr{}
	vtbl[3] = syscall.NewCallback(func(this, out uintptr) uintptr {
		// Force GC during the syscall callback before writing to the caller's
		// Go output location, exercising the custom uintptrescapes wrapper.
		runtime.GC()
		*(*uintptr)(unsafe.Pointer(out)) = 0x7abc1234
		return hresultOK
	})
	object := &fakeObject{vtable: &vtbl}
	var output uintptr
	hr := comCall(uintptr(unsafe.Pointer(object)), 3, uintptr(unsafe.Pointer(&output)))
	runtime.KeepAlive(object)
	if failed(hr) || output != 0x7abc1234 {
		t.Fatalf("out parameter was not preserved: hr=%x output=%x", hr, output)
	}
}

func TestRECTUsesWindowsX64AggregateConvention(t *testing.T) {
	if unsafe.Sizeof(Rect{}) != 16 {
		t.Fatal("RECT must contain four signed 32-bit coordinates")
	}
	type fakeObject struct{ vtable *[26]uintptr }
	vtbl := [26]uintptr{}
	var received Rect
	var aligned bool
	vtbl[6] = syscall.NewCallback(func(this, rectPointer uintptr) uintptr {
		aligned = rectPointer%16 == 0
		received = *(*Rect)(unsafe.Pointer(rectPointer))
		return hresultOK
	})
	object := &fakeObject{vtable: &vtbl}
	w := &WebView{controller: uintptr(unsafe.Pointer(object))}
	w.Resize(Rect{Left: 3, Top: 5, Right: 1920, Bottom: 1080})
	runtime.KeepAlive(object)
	if !aligned || received != (Rect{Left: 3, Top: 5, Right: 1920, Bottom: 1080}) {
		t.Fatalf("RECT aggregate ABI mismatch: aligned=%v bounds=%+v", aligned, received)
	}
}

func TestWebViewCloseBeforeInitialization(t *testing.T) {
	w := &WebView{downloads: make(map[uintptr]eventRegistration), pendingDownloads: make(map[*pendingDownload]bool)}
	w.Resize(Rect{Right: 800, Bottom: 600})
	w.Focus()
	w.PositionChanged()
	w.Close()
	w.Close()
	w.Focus()
	w.Resize(Rect{Right: 10, Bottom: 10})
	if !w.closed {
		t.Fatal("uninitialized WebView was not closed")
	}
	if w.RequestClose(func(bool) {}) == nil {
		t.Fatal("closed WebView accepted a final-save request")
	}
}

func TestRequestCloseUsesPostedCompletion(t *testing.T) {
	for _, tc := range []struct {
		name string
		hr   uintptr
		json string
		want bool
	}{
		{"true", hresultOK, "true", true},
		{"false", hresultOK, "false", false},
		{"string", hresultOK, `"true"`, false},
		{"missing", hresultOK, "", false},
		{"failed", hresultFail, "true", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			type fakeObject struct{ vtable *[30]uintptr }
			vtbl := [30]uintptr{}
			var retained uintptr
			sourceOK := true
			vtbl[29] = syscall.NewCallback(func(this, source, callback uintptr) uintptr {
				for i, ch := range requestCloseScript + "\x00" {
					if *(*uint16)(unsafe.Add(unsafe.Pointer(source), i*2)) != uint16(ch) {
						sourceOK = false
						break
					}
				}
				retained = callback
				comAddRef(retained) // Model WebView2's asynchronous ownership.
				return hresultOK
			})
			object := &fakeObject{vtable: &vtbl}
			var posted []func()
			w := &WebView{core: uintptr(unsafe.Pointer(object)), cfg: WebViewConfig{Post: func(fn func()) bool {
				posted = append(posted, fn)
				return true
			}}}
			called, got := false, false
			if err := w.RequestClose(func(handled bool) { called, got = true, handled }); err != nil {
				t.Fatal(err)
			}
			if !sourceOK || retained == 0 || handlerAt(retained) == nil {
				t.Fatal("ExecuteScript did not receive the fixed script and a retained callback")
			}
			runtime.GC()
			var value *uint16
			if tc.json != "" {
				value, _ = syscall.UTF16PtrFromString(tc.json)
			}
			comCall(retained, 3, tc.hr, uintptr(unsafe.Pointer(value)))
			comRelease(retained)
			runtime.KeepAlive(value)
			runtime.KeepAlive(object)
			if called || len(posted) != 1 {
				t.Fatal("close completion ran inside the COM callback instead of being posted")
			}
			posted[0]()
			if !called || got != tc.want {
				t.Fatalf("posted completion: called=%v handled=%v, want %v", called, got, tc.want)
			}
			if handlerAt(retained) != nil {
				t.Fatal("script callback remained rooted after native Release")
			}
		})
	}
}

func TestStartupProbeCopiesAndPostsOnlyExactNumericStates(t *testing.T) {
	for _, tc := range []struct {
		json string
		want int
	}{
		{"1", 1}, {"0", 0}, {"-1", -1}, {`"1"`, 0}, {"true", 0}, {"null", 0}, {"11", 0}, {"", 0},
	} {
		t.Run(tc.json, func(t *testing.T) {
			type fakeObject struct{ vtable *[30]uintptr }
			vtbl := [30]uintptr{}
			var callback uintptr
			sourceOK := true
			vtbl[29] = syscall.NewCallback(func(this, source, handler uintptr) uintptr {
				for i, ch := range startupStateScript + "\x00" {
					if *(*uint16)(unsafe.Add(unsafe.Pointer(source), i*2)) != uint16(ch) {
						sourceOK = false
					}
				}
				callback = handler
				comAddRef(callback)
				return hresultOK
			})
			object := &fakeObject{vtable: &vtbl}
			var posted []func()
			w := &WebView{core: uintptr(unsafe.Pointer(object)), cfg: WebViewConfig{Post: func(fn func()) bool { posted = append(posted, fn); return true }}}
			got := 99
			if err := w.ProbeStartup(func(state int) { got = state }); err != nil || !sourceOK {
				t.Fatalf("fixed startup script was not accepted: %v", err)
			}
			value, _ := syscall.UTF16FromString(tc.json)
			comCall(callback, 3, hresultOK, uintptr(unsafe.Pointer(&value[0])))
			comRelease(callback)
			value[0] = 'X' // The borrowed result is no longer available after Invoke.
			runtime.KeepAlive(value)
			runtime.KeepAlive(object)
			if got != 99 || len(posted) != 1 {
				t.Fatal("startup completed inside a COM callback")
			}
			posted[0]()
			if got != tc.want || handlerAt(callback) != nil {
				t.Fatalf("startup state=%d, want %d, or callback was leaked", got, tc.want)
			}
		})
	}
}

func TestHRESULTFailureClassification(t *testing.T) {
	if failed(0) || failed(1) || !failed(0x80004005) || !failed(0xffffffff80004005) {
		t.Fatal("HRESULT sign handling is incorrect")
	}
	if hresultError("operation", 0) != nil || hresultError("operation", 0x80004005) == nil {
		t.Fatal("HRESULT error conversion failed")
	}
}

func TestSDKInterfaceIdentifierEncoding(t *testing.T) {
	// ICoreWebView2Settings4 UUID from the official Microsoft SDK, in Windows
	// GUID in-memory byte order (first 3 fields little endian).
	want := comGUID{0x6c, 0x84, 0x56, 0xcb, 0x68, 0x41, 0x53, 0x4d, 0xb0, 0x4f, 0x03, 0xb6, 0xd6, 0x79, 0x6f, 0xf2}
	if iidSettings4 != want {
		t.Fatalf("GUID byte order mismatch: %x", iidSettings4)
	}
}
