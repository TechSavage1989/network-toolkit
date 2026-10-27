//go:build windows

package desktop

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

func TestNativeBootstrapDocumentFilterRegistrationFailsClosed(t *testing.T) {
	for _, scenario := range []string{"success", "missing-modern-filter", "response-failure", "empty-response", "filter-failure", "registration-failure"} {
		t.Run(scenario, func(t *testing.T) {
			type fakeObject struct{ vtable *[124]uintptr }
			coreTable, environmentTable, responseTable := [124]uintptr{}, [124]uintptr{}, [124]uintptr{}
			core := &fakeObject{vtable: &coreTable}
			environment := &fakeObject{vtable: &environmentTable}
			response := &fakeObject{vtable: &responseTable}
			var pinned runtime.Pinner
			for _, value := range []any{&coreTable, &environmentTable, &responseTable, core, environment, response} {
				pinned.Pin(value)
			}
			defer pinned.Unpin()
			var retained uintptr
			filters, registrations, releases := 0, 0, 0
			coreTable[0] = syscall.NewCallback(func(this, iid, result uintptr) uintptr {
				if *(*comGUID)(unsafe.Pointer(iid)) != iidCoreWebView22 {
					t.Error("wrong interface requested for iframe-safe filtering")
					return hresultNoInterface
				}
				if scenario == "missing-modern-filter" {
					return hresultNoInterface
				}
				*(*uintptr)(unsafe.Pointer(result)) = this
				return hresultOK
			})
			coreTable[2] = syscall.NewCallback(func(this uintptr) uintptr { return 0 })
			environmentTable[2] = coreTable[2]
			responseTable[2] = syscall.NewCallback(func(this uintptr) uintptr { releases++; return 0 })
			environmentTable[4] = syscall.NewCallback(func(this, content, status, reason, headers, result uintptr) uintptr {
				if content != 0 || status != 403 {
					t.Error("deny response was not empty HTTP403")
				}
				if scenario == "response-failure" {
					return hresultFail
				}
				if scenario != "empty-response" {
					*(*uintptr)(unsafe.Pointer(result)) = uintptr(unsafe.Pointer(response))
				}
				return hresultOK
			})
			coreTable[123] = syscall.NewCallback(func(this, pattern, context, sources uintptr) uintptr {
				filters++
				if *(*uint16)(unsafe.Pointer(pattern)) != '*' || context != 1 || uint32(sources) != 0xffffffff {
					t.Error("filter does not cover all document sources only")
				}
				if scenario == "filter-failure" {
					return hresultFail
				}
				return hresultOK
			})
			coreTable[55] = syscall.NewCallback(func(this, handler, token uintptr) uintptr {
				registrations++
				if scenario == "registration-failure" {
					return hresultFail
				}
				queried, err := comQuery(handler, iidWebResourceRequested)
				if err != nil {
					t.Error("wrong document request handler IID")
				} else {
					comRelease(queried)
				}
				retained = handler
				comAddRef(retained)
				*(*int64)(unsafe.Pointer(token)) = 1
				return hresultOK
			})
			coreTable[56] = syscall.NewCallback(func(this, token uintptr) uintptr {
				if retained != 0 {
					comRelease(retained)
				}
				return hresultOK
			})
			w := &WebView{core: uintptr(unsafe.Pointer(core)), environment: uintptr(unsafe.Pointer(environment))}
			err := w.installDocumentRequestPolicy()
			if scenario == "success" {
				if err != nil || filters != 1 || registrations != 1 || retained == 0 {
					t.Fatalf("document request policy did not install: %v", err)
				}
			} else if err == nil {
				t.Fatalf("%s silently downgraded document request policy", scenario)
			}
			if scenario == "missing-modern-filter" && (filters != 0 || registrations != 0 || w.blockedDocumentResponse != 0) {
				t.Fatal("legacy filter fallback was attempted")
			}
			if (scenario == "response-failure" || scenario == "empty-response") && (filters != 0 || registrations != 0) {
				t.Fatal("filter installed without a usable deny response")
			}
			hadResponse := w.blockedDocumentResponse != 0
			w.Close()
			if hadResponse && releases != 1 {
				t.Fatal("deny response not released on close")
			}
			if retained != 0 && handlerAt(retained) != nil {
				t.Fatal("document request handler leaked after close")
			}
			runtime.KeepAlive(core)
			runtime.KeepAlive(environment)
			runtime.KeepAlive(response)
		})
	}
}
