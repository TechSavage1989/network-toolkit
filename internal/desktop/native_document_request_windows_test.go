//go:build windows

package desktop

import (
	"fmt"
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
				if *(*uint16)(unsafe.Pointer(pattern)) != '*' || context != 0 || uint32(sources) != 0xffffffff {
					t.Error("diagnostic filter does not cover all resource contexts and sources")
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

// The diagnostic filter sees assets and API fetches too. They must not consume
// the one-use document permit or be blocked simply because startup completed.
func TestNativeBootstrapDocumentContextDispatch(t *testing.T) {
	const appURL = "http://127.0.0.1:4000/"
	type contextCase struct {
		name        string
		context     int32
		contextHR   uintptr
		uri         string
		spent       bool
		closed      bool
		wantBlocked bool
		wantPending bool
	}
	cases := []contextCase{
		{name: "initial-document", context: 1, uri: appURL},
		{name: "repeated-document", context: 1, uri: appURL, spent: true, wantBlocked: true},
		{name: "other-document", context: 1, uri: appURL + "other", wantBlocked: true},
		{name: "api-after-startup", context: 7, uri: appURL + "api", spent: true},
		{name: "unreadable-context", context: -1, contextHR: hresultFail, uri: appURL, wantBlocked: true, wantPending: true},
		{name: "missing-context-value", context: -1, uri: appURL, wantBlocked: true, wantPending: true},
		{name: "closed-host", context: 8, uri: appURL + "api", closed: true, wantBlocked: true, wantPending: true},
	}
	// Include ALL, Other and an unrecognized positive future value. This probe
	// deliberately preserves readable non-document contexts until CI identifies
	// whether unexpected navigation was classified outside Document.
	for _, resourceContext := range []int32{0, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 99} {
		cases = append(cases, contextCase{name: fmt.Sprintf("non-document-%d", resourceContext), context: resourceContext, uri: appURL + "resource", wantPending: true})
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			type fakeObject struct{ vtable *[8]uintptr }
			argsTable, requestTable := [8]uintptr{}, [8]uintptr{}
			args, request := &fakeObject{vtable: &argsTable}, &fakeObject{vtable: &requestTable}
			var pinned runtime.Pinner
			for _, value := range []any{&argsTable, &requestTable, args, request} {
				pinned.Pin(value)
			}
			defer pinned.Unpin()
			responses, releases, observations := 0, 0, 0
			var observed nativeNavigationObservation
			requestTable[2] = syscall.NewCallback(func(this uintptr) uintptr { releases++; return 0 })
			requestTable[3] = syscall.NewCallback(func(this, result uintptr) uintptr {
				encoded, err := syscall.UTF16FromString(test.uri)
				if err != nil {
					return hresultFail
				}
				buffer, _, _ := runtimeCoTaskMemAlloc.Call(uintptr(len(encoded) * 2))
				if buffer == 0 {
					return hresultFail
				}
				copy(unsafe.Slice((*uint16)(unsafe.Pointer(buffer)), len(encoded)), encoded)
				*(*uintptr)(unsafe.Pointer(result)) = buffer
				return hresultOK
			})
			argsTable[3] = syscall.NewCallback(func(this, result uintptr) uintptr {
				*(*uintptr)(unsafe.Pointer(result)) = uintptr(unsafe.Pointer(request))
				return hresultOK
			})
			argsTable[5] = syscall.NewCallback(func(this, response uintptr) uintptr {
				responses++
				if response != 1 {
					t.Error("request did not receive the prepared deny response")
				}
				return hresultOK
			})
			argsTable[7] = syscall.NewCallback(func(this, result uintptr) uintptr {
				*(*int32)(unsafe.Pointer(result)) = test.context
				return test.contextHR
			})
			w := &WebView{cfg: WebViewConfig{AppURL: appURL, navigationObserver: func(value nativeNavigationObservation) {
				observations++
				observed = value
			}}, closed: test.closed, blockedDocumentResponse: 1}
			w.documentRequestPermit.begin()
			if test.spent {
				w.documentRequestPermit.consume(appURL, appURL)
			}
			if hr := w.documentResourceRequested(0, uintptr(unsafe.Pointer(args))); failed(hr) {
				t.Fatalf("request handler failed: %#x", hr)
			}
			if (responses != 0) != test.wantBlocked || responses > 1 || observed.Blocked != test.wantBlocked {
				t.Fatalf("blocked=%t response count=%d, want blocked=%t", observed.Blocked, responses, test.wantBlocked)
			}
			if w.documentRequestPermit.pending != test.wantPending {
				t.Fatalf("startup permit pending=%t, want %t", w.documentRequestPermit.pending, test.wantPending)
			}
			if observations != 1 || observed.ResourceContext != test.context || observed.ResourceContextResult != uint32(test.contextHR) || observed.URI != test.uri {
				t.Fatalf("request context was not observed accurately: %+v", observed)
			}
			if releases != 1 {
				t.Fatalf("request reference releases=%d, want 1", releases)
			}
			runtime.KeepAlive(args)
			runtime.KeepAlive(request)
		})
	}
}
