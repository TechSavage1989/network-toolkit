//go:build windows

package desktop

import (
	"encoding/json"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

type documentProtocolTestCall struct {
	session, method, parameters string
	callback                    uintptr
}

type documentFetchTestObject struct{ vtable *[100]uintptr }

type documentFetchFixture struct {
	t                  *testing.T
	w                  *WebView
	pinned             runtime.Pinner
	commands           []documentProtocolTestCall
	posted             []func()
	listener           uintptr
	bootstrap          uintptr
	bootstrapCount     int
	navigations        int
	failures           int
	closeOrder         []string
	failMethod         string
	refusePost         bool
}

func documentProtocolTestString(pointer uintptr) string {
	if pointer == 0 {
		return ""
	}
	var chars []uint16
	for i := 0; i < 4096; i++ {
		ch := *(*uint16)(unsafe.Add(unsafe.Pointer(pointer), i*2))
		if ch == 0 {
			return syscall.UTF16ToString(chars)
		}
		chars = append(chars, ch)
	}
	return "invalid oversized string"
}

func newDocumentFetchFixture(t *testing.T, scenario string) *documentFetchFixture {
	t.Helper()
	f := &documentFetchFixture{t: t}
	coreTable, receiverTable, controllerTable := &[100]uintptr{}, &[100]uintptr{}, &[100]uintptr{}
	core, receiver, controller := &documentFetchTestObject{coreTable}, &documentFetchTestObject{receiverTable}, &documentFetchTestObject{controllerTable}
	for _, value := range []any{coreTable, receiverTable, controllerTable, core, receiver, controller} {
		f.pinned.Pin(value)
	}
	coreTable[0] = syscall.NewCallback(func(this, iid, result uintptr) uintptr {
		if scenario == "missing-interface" || *(*comGUID)(unsafe.Pointer(iid)) != iidCoreWebView11 {
			return hresultNoInterface
		}
		*(*uintptr)(unsafe.Pointer(result)) = this
		return hresultOK
	})
	coreTable[2] = syscall.NewCallback(func(this uintptr) uintptr { return 0 })
	controllerTable[2] = coreTable[2]
	controllerTable[24] = syscall.NewCallback(func(this uintptr) uintptr {
		f.closeOrder = append(f.closeOrder, "controller-close")
		return hresultOK
	})
	receiverTable[2] = syscall.NewCallback(func(this uintptr) uintptr {
		f.closeOrder = append(f.closeOrder, "receiver-release")
		return 0
	})
	receiverTable[3] = syscall.NewCallback(func(this, handler, token uintptr) uintptr {
		if scenario == "event-failure" {
			return hresultFail
		}
		queried, err := comQuery(handler, iidDevToolsEventReceived)
		if err != nil {
			t.Error("wrong SDK event handler IID")
		} else {
			comRelease(queried)
		}
		f.listener = handler
		comAddRef(handler)
		*(*int64)(unsafe.Pointer(token)) = 7
		return hresultOK
	})
	receiverTable[4] = syscall.NewCallback(func(this, token uintptr) uintptr {
		f.closeOrder = append(f.closeOrder, "receiver-remove")
		if f.listener != 0 {
			comRelease(f.listener)
			f.listener = 0
		}
		return hresultOK
	})
	coreTable[42] = syscall.NewCallback(func(this, eventName, result uintptr) uintptr {
		if documentProtocolTestString(eventName) != "Fetch.requestPaused" {
			t.Error("wrong request interception event")
		}
		if scenario == "receiver-failure" {
			return hresultFail
		}
		if scenario != "empty-receiver" {
			*(*uintptr)(unsafe.Pointer(result)) = uintptr(unsafe.Pointer(receiver))
		}
		return hresultOK
	})
	record := func(session, method, parameters, callback uintptr) uintptr {
		name := documentProtocolTestString(method)
		if f.listener == 0 {
			t.Error("interception enabled before its event handler existed")
		}
		if scenario == "enable-call-failure" || name == f.failMethod {
			return hresultFail
		}
		queried, err := comQuery(callback, iidDevToolsCompleted)
		if err != nil {
			t.Error("wrong SDK protocol completion IID")
		} else {
			comRelease(queried)
		}
		comAddRef(callback)
		f.commands = append(f.commands, documentProtocolTestCall{documentProtocolTestString(session), name, documentProtocolTestString(parameters), callback})
		return hresultOK
	}
	coreTable[36] = syscall.NewCallback(func(this, method, parameters, callback uintptr) uintptr {
		return record(0, method, parameters, callback)
	})
	coreTable[99] = syscall.NewCallback(func(this, session, method, parameters, callback uintptr) uintptr {
		return record(session, method, parameters, callback)
	})
	coreTable[27] = syscall.NewCallback(func(this, source, callback uintptr) uintptr {
		f.bootstrapCount++
		f.bootstrap = callback
		comAddRef(callback)
		return hresultOK
	})
	coreTable[28] = syscall.NewCallback(func(this, id uintptr) uintptr { return hresultOK })
	coreTable[5] = syscall.NewCallback(func(this, uri uintptr) uintptr { f.navigations++; return hresultOK })
	f.w = &WebView{core: uintptr(unsafe.Pointer(core)), controller: uintptr(unsafe.Pointer(controller)), cfg: WebViewConfig{
		AppURL: "http://127.0.0.1:1234/", SessionToken: strings.Repeat("ab", 32),
		Post: func(fn func()) bool {
			if f.refusePost {
				return false
			}
			f.posted = append(f.posted, fn)
			return true
		},
	}, ready: func(_ *WebView, err error) {
		if err != nil {
			f.failures++
		}
	}}
	t.Cleanup(func() {
		f.w.Close()
		for _, call := range f.commands {
			if call.callback != 0 {
				comRelease(call.callback)
			}
		}
		if f.bootstrap != 0 {
			comRelease(f.bootstrap)
		}
		f.pinned.Unpin()
	})
	return f
}

func (f *documentFetchFixture) complete(index int, result uintptr, raw string) uintptr {
	f.t.Helper()
	callback := f.commands[index].callback
	var pointer uintptr
	var buffer []uint16
	if raw != "" {
		buffer, _ = syscall.UTF16FromString(raw)
		pointer = uintptr(unsafe.Pointer(&buffer[0]))
	}
	runtime.GC()
	hr := comCall(callback, 3, result, pointer)
	comRelease(callback)
	f.commands[index].callback = 0
	if len(buffer) != 0 {
		buffer[0] = 'X'
	}
	runtime.KeepAlive(buffer)
	return hr
}

func (f *documentFetchFixture) drain() {
	for len(f.posted) > 0 {
		fn := f.posted[0]
		f.posted = f.posted[1:]
		fn()
	}
}

func (f *documentFetchFixture) start() {
	f.t.Helper()
	if err := f.w.installDocumentFetchPolicy(); err != nil {
		f.t.Fatal(err)
	}
	f.complete(0, hresultOK, "{}")
	f.drain()
	id, _ := syscall.UTF16FromString("fetch-bootstrap-id")
	comCall(f.bootstrap, 3, hresultOK, uintptr(unsafe.Pointer(&id[0])))
	comRelease(f.bootstrap)
	f.bootstrap = 0
	f.drain()
	runtime.KeepAlive(id)
	if f.navigations != 1 {
		f.t.Fatal("fixture did not complete native startup")
	}
}

func documentProtocolTestOwnedString(raw string, result uintptr) uintptr {
	encoded, err := syscall.UTF16FromString(raw)
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
}

func (f *documentFetchFixture) dispatch(raw, session string, missingSession bool) uintptr {
	f.t.Helper()
	table := &[100]uintptr{}
	args := &documentFetchTestObject{table}
	f.pinned.Pin(table)
	f.pinned.Pin(args)
	table[0] = syscall.NewCallback(func(this, iid, result uintptr) uintptr {
		if missingSession || *(*comGUID)(unsafe.Pointer(iid)) != iidDevToolsEventArgs2 {
			return hresultNoInterface
		}
		*(*uintptr)(unsafe.Pointer(result)) = this
		return hresultOK
	})
	table[2] = syscall.NewCallback(func(this uintptr) uintptr { return 0 })
	table[3] = syscall.NewCallback(func(this, result uintptr) uintptr { return documentProtocolTestOwnedString(raw, result) })
	table[4] = syscall.NewCallback(func(this, result uintptr) uintptr { return documentProtocolTestOwnedString(session, result) })
	return comCall(f.listener, 3, f.w.core, uintptr(unsafe.Pointer(args)))
}

func TestNativeBootstrapDocumentFetchRegistrationFailsClosed(t *testing.T) {
	for _, scenario := range []string{"missing-interface", "receiver-failure", "empty-receiver", "event-failure", "enable-call-failure"} {
		t.Run(scenario, func(t *testing.T) {
			f := newDocumentFetchFixture(t, scenario)
			if err := f.w.installDocumentFetchPolicy(); err == nil {
				t.Fatal("document interception silently downgraded")
			}
			if f.bootstrapCount != 0 || f.navigations != 0 || f.w.documentFetchReady {
				t.Fatal("failed interception started the native page")
			}
		})
	}
}

func TestNativeBootstrapDocumentFetchWaitsForCompletedEnableAndBootstrap(t *testing.T) {
	for _, scenario := range []string{"success", "completion-failure", "empty-result", "protocol-error", "closed-before-callback", "closed-before-post", "dispatch-refused"} {
		t.Run(scenario, func(t *testing.T) {
			f := newDocumentFetchFixture(t, "")
			if err := f.w.installNativeBootstrap(); err == nil {
				t.Fatal("bootstrap bypassed the Fetch completion gate")
			}
			if err := f.w.installDocumentFetchPolicy(); err != nil {
				t.Fatal(err)
			}
			if len(f.commands) != 1 || f.commands[0].method != "Fetch.enable" || f.commands[0].parameters != nativeDocumentFetchParameters || f.bootstrapCount != 0 || f.navigations != 0 {
				t.Fatal("startup did not wait for the document-only Request-stage interception")
			}
			if scenario == "closed-before-callback" {
				f.w.Close()
			}
			result, raw := hresultOK, "{}"
			if scenario == "completion-failure" {
				result = hresultFail
			} else if scenario == "empty-result" {
				raw = ""
			} else if scenario == "protocol-error" {
				raw = `{"error":{"message":"synthetic failure"}}`
			}
			f.refusePost = scenario == "dispatch-refused"
			hr := f.complete(0, result, raw)
			if f.bootstrapCount != 0 || f.navigations != 0 {
				t.Fatal("startup reentered COM from the enable callback")
			}
			if scenario == "closed-before-post" {
				f.w.Close()
			}
			f.drain()
			if scenario == "success" {
				if !f.w.documentFetchReady || f.bootstrapCount != 1 || f.navigations != 0 {
					t.Fatal("successful Fetch enable did not await private bootstrap registration")
				}
				id, _ := syscall.UTF16FromString("fetch-bootstrap-id")
				comCall(f.bootstrap, 3, hresultOK, uintptr(unsafe.Pointer(&id[0])))
				f.drain()
				if f.navigations != 1 || !f.w.documentFetchPermit.pending || !f.w.documentRequestPermit.pending {
					t.Fatal("completed startup did not arm independent interception permits")
				}
				runtime.KeepAlive(id)
			} else if f.bootstrapCount != 0 || f.navigations != 0 {
				t.Fatal("failed or closed interceptor started the native page")
			}
			if (scenario == "completion-failure" || scenario == "empty-result" || scenario == "protocol-error") && f.failures != 1 {
				t.Fatal("asynchronous interception failure was not reported")
			}
			if scenario == "dispatch-refused" && !failed(hr) {
				t.Fatal("refused UI dispatch was reported as completed")
			}
			f.w.Close()
			if strings.Join(f.closeOrder, ",") != "controller-close,receiver-remove,receiver-release" {
				t.Fatal("interception was detached before its controller closed", f.closeOrder)
			}
			if f.w.documentFetchPermit.pending || f.w.documentFetchReady {
				t.Fatal("closed host retained a document interception permit")
			}
		})
	}
}

func TestNativeBootstrapDocumentFetchResolvesRequestsInTheirOwnSession(t *testing.T) {
	f := newDocumentFetchFixture(t, "")
	f.start()
	for _, name := range []string{"initial", "repeat", "child"} {
		event := documentFetchTestEvent(t, f.w.cfg.AppURL)
		event.RequestID = "request-" + name
		raw, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		session := ""
		if name == "child" {
			session = "child-session"
		}
		before := len(f.commands)
		if hr := f.dispatch(string(raw), session, false); failed(hr) || len(f.commands) != before+1 {
			t.Fatal("request did not receive a protocol decision")
		}
		call := f.commands[before]
		wantMethod := "Fetch.failRequest"
		if name == "initial" {
			wantMethod = "Fetch.continueRequest"
		}
		var parameters map[string]string
		if json.Unmarshal([]byte(call.parameters), &parameters) != nil || call.method != wantMethod || call.session != session || parameters["requestId"] != event.RequestID {
			t.Fatal("request was resolved in the wrong session or with the wrong identity", call)
		}
		if name != "initial" && parameters["errorReason"] != "Aborted" {
			t.Fatal("later document was not aborted")
		}
		f.complete(before, hresultOK, "{}")
		f.drain()
		if f.w.closed || f.failures != 0 {
			t.Fatal("successful request resolution closed the original host")
		}
	}
}

func TestNativeBootstrapDocumentFetchResolutionFailuresCloseWithoutContinuing(t *testing.T) {
	for _, scenario := range []string{"malformed-event", "missing-session", "synchronous-failure", "completion-failure"} {
		t.Run(scenario, func(t *testing.T) {
			f := newDocumentFetchFixture(t, "")
			f.start()
			event := documentFetchTestEvent(t, "http://localhost:1234/forbidden")
			raw, _ := json.Marshal(event)
			if scenario == "malformed-event" {
				raw = []byte("{")
			} else if scenario == "synchronous-failure" {
				f.failMethod = "Fetch.failRequest"
			}
			before := len(f.commands)
			f.dispatch(string(raw), "", scenario == "missing-session")
			if scenario == "completion-failure" {
				if len(f.commands) != before+1 || f.commands[before].method != "Fetch.failRequest" {
					t.Fatal("forbidden request was not denied")
				}
				f.complete(before, hresultFail, "{}")
			} else if len(f.commands) != before {
				t.Fatal("unresolvable request was continued")
			}
			if f.w.closed {
				t.Fatal("request callback reentered controller teardown")
			}
			f.drain()
			if !f.w.closed || f.failures != 1 || f.w.documentFetchPermit.pending {
				t.Fatal("failed request resolution left an active native host")
			}
		})
	}
}
