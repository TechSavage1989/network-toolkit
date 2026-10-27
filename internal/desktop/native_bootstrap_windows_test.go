//go:build windows

package desktop

import (
	"runtime"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

func TestNativeBootstrapWaitsForSuccessfulRegistration(t *testing.T) {
	for _, scenario := range []string{"success", "callback-failure", "empty-id", "closed-before-callback", "closed-before-post", "registration-failure", "dispatch-refused"} {
		t.Run(scenario, func(t *testing.T) {
			type fakeObject struct{ vtable *[29]uintptr }
			vtbl := [29]uintptr{}
			var retained uintptr
			navigations, removals, failures := 0, 0, 0
			vtbl[2] = syscall.NewCallback(func(this uintptr) uintptr { return 0 })
			vtbl[5] = syscall.NewCallback(func(this, uri uintptr) uintptr { navigations++; return hresultOK })
			vtbl[27] = syscall.NewCallback(func(this, source, callback uintptr) uintptr {
				if scenario == "registration-failure" {
					return hresultFail
				}
				retained = callback
				comAddRef(retained)
				return hresultOK
			})
			vtbl[28] = syscall.NewCallback(func(this, id uintptr) uintptr { removals++; return hresultOK })
			object := &fakeObject{vtable: &vtbl}
			// uintptr does not keep a Go allocation on the heap or stable across
			// stack growth. Native COM objects are not Go memory; pin this mock.
			var pinned runtime.Pinner
			pinned.Pin(&vtbl)
			pinned.Pin(object)
			defer pinned.Unpin()
			var posted []func()
			w := &WebView{core: uintptr(unsafe.Pointer(object)), cfg: WebViewConfig{
				AppURL: "http://127.0.0.1:1234/", SessionToken: strings.Repeat("ab", 32),
				Post: func(fn func()) bool {
					if scenario == "dispatch-refused" {
						return false
					}
					posted = append(posted, fn)
					return true
				},
			}, ready: func(_ *WebView, err error) {
				if err != nil {
					failures++
				}
			}}
			err := w.installNativeBootstrap()
			if scenario == "registration-failure" {
				if err == nil || retained != 0 || navigations != 0 {
					t.Fatal("registration failure did not fail closed")
				}
				w.Close()
				return
			}
			if err != nil || retained == 0 || navigations != 0 || len(posted) != 0 || w.documentRequestPermit.started {
				t.Fatalf("navigation raced registration: %v", err)
			}
			if _, err := comQuery(retained, iidAddScriptCompleted); err != nil {
				t.Fatal("callback did not advertise SDK add-script IID")
			} else {
				comRelease(retained)
			}
			if scenario == "closed-before-callback" {
				w.Close()
			}
			result := hresultOK
			if scenario == "callback-failure" {
				result = hresultFail
			}
			id := "bootstrap-fixture-id"
			if scenario == "empty-id" {
				id = ""
			}
			buffer, _ := syscall.UTF16FromString(id)
			runtime.GC()
			comCall(retained, 3, result, uintptr(unsafe.Pointer(&buffer[0])))
			comRelease(retained)
			buffer[0] = 'X' // The callback must have copied its borrowed string.
			if navigations != 0 {
				t.Fatal("navigation happened inside callback")
			}
			if scenario == "closed-before-post" {
				w.Close()
			}
			for _, fn := range posted {
				fn()
			}
			if scenario == "success" {
				if navigations != 1 || w.bootstrapScriptID != id || failures != 0 || !w.documentRequestPermit.pending || !w.documentRequestPermit.navigationPending {
					t.Fatal("successful callback did not release exact navigation")
				}
			} else if navigations != 0 {
				t.Fatalf("%s navigated despite failed/closed bootstrap", scenario)
			}
			if (scenario == "callback-failure" || scenario == "empty-id") && failures != 1 {
				t.Fatal("asynchronous bootstrap failure not reported")
			}
			w.Close()
			if w.documentRequestPermit.pending || w.documentRequestPermit.navigationPending {
				t.Fatal("closed host retained an initial navigation permit")
			}
			if scenario == "success" && removals != 1 {
				t.Fatal("native bootstrap script was not removed on close")
			}
			if handlerAt(retained) != nil {
				t.Fatal("bootstrap callback leaked after native Release")
			}
			runtime.KeepAlive(buffer)
			runtime.KeepAlive(object)
		})
	}
}

func TestNativeBootstrapSDKIdentifier(t *testing.T) {
	// Microsoft.Web.WebView2 SDK WebView2.h, IID in native Windows byte order.
	want := comGUID{0xf3, 0x69, 0x93, 0xb9, 0x11, 0x9b, 0xb5, 0x47, 0xbc, 0x6f, 0x8e, 0x78, 0x95, 0xfc, 0xea, 0x17}
	if iidAddScriptCompleted != want {
		t.Fatalf("AddScript completion IID mismatch: %x", iidAddScriptCompleted)
	}
}

func TestCredentialConfirmationNativeDefaultIsNo(t *testing.T) {
	if nativeCredentialConfirmationFlags&0x0000000f != 0x4 || nativeCredentialConfirmationFlags&0x00000300 != 0x100 {
		t.Fatal("native security prompt must be Yes/No with No as its default")
	}
}
