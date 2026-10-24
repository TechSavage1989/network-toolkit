//go:build windows

package desktop

// The runtime entry-point adapter and environment-option defaults are adapted
// from Wails go-webview2 v1.0.23, commit
// e08e9397f04ad0b1050d7e5bdc8e27faddc87987, webviewloader/env_create*.go.
//
// ISC License (ISC)
// Copyright (c) 2020 John Chadwick
// Copyright (c) 2022 Wails Project Developers
//
// Permission to use, copy, modify, and/or distribute this software for any
// purpose with or without fee is hereby granted, provided that the above
// copyright notice and this permission notice appear in all copies.
//
// THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH
// REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY
// AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT,
// INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM
// LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR
// OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR
// PERFORMANCE OF THIS SOFTWARE.

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

var (
	iidCoreEnvironment     = guid("b96d755e-0319-4e92-a296-23436f46a1fc")
	iidEnvironmentOptions  = guid("2fde08a8-1e9a-4766-8c05-95a9ceb9d1c5")
	iidEnvironmentOptions2 = guid("ff85c98a-1ba7-4a6b-90c8-2b752c89e9e2")
	runtimeCoTaskMemAlloc  = syscall.NewLazyDLL("ole32.dll").NewProc("CoTaskMemAlloc")
)

// This matches the upstream Go loader's compatibility target. The existing
// controller setup separately requires the actual settings and download APIs
// used by this application; a runtime lacking them fails with an update error.
const runtimeMinimumCompatibleVersion = "86.0.616.0"

// Apply the upstream loader's process-local override policy. Empty environment
// values prevent arbitrary arguments, debugger pipes, or a different profile
// from being substituted for the application's explicit settings. No machine
// environment, registry, browser policy, or installed runtime is changed.
func prepareWebViewRuntimeEnvironment() error {
	for _, setting := range []struct{ name, value string }{
		{"WEBVIEW2_PIPE_FOR_SCRIPT_DEBUGGER", ""},
		{"WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", ""},
		{"WEBVIEW2_RELEASE_CHANNEL_PREFERENCE", "0"},
		{"WEBVIEW2_BROWSER_EXECUTABLE_FOLDER", ""},
		{"WEBVIEW2_USER_DATA_FOLDER", ""},
	} {
		if err := os.Setenv(setting.name, setting.value); err != nil {
			return fmt.Errorf("Set the application WebView2 environment: %w", err)
		}
	}
	return nil
}

// The runtime's internal callback can expose IUnknown rather than the public
// environment interface. Match the upstream adapter by querying the public IID
// before forwarding, then release that temporary owned reference. The receiver
// takes its own reference if it keeps the environment after the callback.
func adaptRuntimeEnvironmentCompleted(forward func(uintptr, uintptr) uintptr) func(uintptr, uintptr) uintptr {
	return func(result, unknown uintptr) uintptr {
		if unknown == 0 {
			return forward(result, 0)
		}
		environment, err := comQuery(unknown, iidCoreEnvironment)
		if err != nil {
			return forward(hresultFail, 0)
		}
		defer comRelease(environment)
		return forward(result, environment)
	}
}

// Environment options are immutable after construction, as in the upstream
// loader. Both interface addresses share one IUnknown identity and refcount.
// The map roots every object until the runtime releases its last reference.
type webViewEnvironmentOptions struct {
	vtable   *[11]uintptr
	options2 webViewEnvironmentOptions2
	refs     atomic.Int32
}

type webViewEnvironmentOptions2 struct {
	vtable *[5]uintptr
}

var runtimeOptionObjects = struct {
	sync.Mutex
	objects map[uintptr]*webViewEnvironmentOptions
}{objects: make(map[uintptr]*webViewEnvironmentOptions)}

var runtimeOptionVtable = [11]uintptr{
	syscall.NewCallback(runtimeOptionsQuery),
	syscall.NewCallback(runtimeOptionsAddRef),
	syscall.NewCallback(runtimeOptionsRelease),
	syscall.NewCallback(runtimeOptionsEmptyString), // AdditionalBrowserArguments
	syscall.NewCallback(runtimeOptionsNoChange),
	syscall.NewCallback(runtimeOptionsEmptyString), // Language: system default
	syscall.NewCallback(runtimeOptionsNoChange),
	syscall.NewCallback(runtimeOptionsCompatibleVersion),
	syscall.NewCallback(runtimeOptionsNoChange),
	syscall.NewCallback(runtimeOptionsFalse), // No OS-account single sign-on
	syscall.NewCallback(runtimeOptionsNoChange),
}

var runtimeOption2Vtable = [5]uintptr{
	syscall.NewCallback(runtimeOptionsQuery),
	syscall.NewCallback(runtimeOptionsAddRef),
	syscall.NewCallback(runtimeOptionsRelease),
	syscall.NewCallback(runtimeOptionsFalse), // Normal private-profile access
	syscall.NewCallback(runtimeOptionsNoChange),
}

func newWebViewEnvironmentOptions() *webViewEnvironmentOptions {
	options := &webViewEnvironmentOptions{vtable: &runtimeOptionVtable, options2: webViewEnvironmentOptions2{vtable: &runtimeOption2Vtable}}
	options.refs.Store(1)
	runtimeOptionObjects.Lock()
	runtimeOptionObjects.objects[options.pointer()] = options
	runtimeOptionObjects.objects[options.pointer2()] = options
	runtimeOptionObjects.Unlock()
	return options
}

func (o *webViewEnvironmentOptions) pointer() uintptr { return uintptr(unsafe.Pointer(o)) }
func (o *webViewEnvironmentOptions) pointer2() uintptr {
	return uintptr(unsafe.Pointer(&o.options2))
}
func (o *webViewEnvironmentOptions) release() { runtimeOptionsRelease(o.pointer()) }

func runtimeOptionsAt(this uintptr) *webViewEnvironmentOptions {
	runtimeOptionObjects.Lock()
	options := runtimeOptionObjects.objects[this]
	runtimeOptionObjects.Unlock()
	return options
}

func runtimeOptionsQuery(this, iidPointer, outPointer uintptr) uintptr {
	if outPointer == 0 {
		return hresultPointer
	}
	*(*uintptr)(unsafe.Pointer(outPointer)) = 0
	if iidPointer == 0 {
		return hresultPointer
	}
	options := runtimeOptionsAt(this)
	if options == nil {
		return hresultPointer
	}
	var result uintptr
	switch *(*comGUID)(unsafe.Pointer(iidPointer)) {
	case iidIUnknown, iidEnvironmentOptions:
		result = options.pointer()
	case iidEnvironmentOptions2:
		result = options.pointer2()
	default:
		return hresultNoInterface
	}
	options.refs.Add(1)
	*(*uintptr)(unsafe.Pointer(outPointer)) = result
	return hresultOK
}

func runtimeOptionsAddRef(this uintptr) uintptr {
	if options := runtimeOptionsAt(this); options != nil {
		return uintptr(options.refs.Add(1))
	}
	return 0
}

func runtimeOptionsRelease(this uintptr) uintptr {
	options := runtimeOptionsAt(this)
	if options == nil {
		return 0
	}
	refs := options.refs.Add(-1)
	if refs > 0 {
		return uintptr(refs)
	}
	runtimeOptionObjects.Lock()
	delete(runtimeOptionObjects.objects, options.pointer())
	delete(runtimeOptionObjects.objects, options.pointer2())
	runtimeOptionObjects.Unlock()
	return 0
}

func runtimeOptionsNoChange(this, value uintptr) uintptr {
	if runtimeOptionsAt(this) == nil {
		return hresultPointer
	}
	return 1 // S_FALSE: upstream setters leave the supplied defaults unchanged.
}

func runtimeOptionsEmptyString(this, value uintptr) uintptr {
	return runtimeOptionsString(this, value, "")
}

func runtimeOptionsCompatibleVersion(this, value uintptr) uintptr {
	return runtimeOptionsString(this, value, runtimeMinimumCompatibleVersion)
}

func runtimeOptionsString(this, value uintptr, text string) uintptr {
	if value == 0 || runtimeOptionsAt(this) == nil {
		return hresultPointer
	}
	*(*uintptr)(unsafe.Pointer(value)) = 0
	encoded, err := syscall.UTF16FromString(text)
	if err != nil {
		return hresultFail
	}
	buffer, _, _ := runtimeCoTaskMemAlloc.Call(uintptr(len(encoded) * 2))
	if buffer == 0 {
		return 0x8007000E // E_OUTOFMEMORY
	}
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(buffer)), len(encoded)), encoded)
	*(*uintptr)(unsafe.Pointer(value)) = buffer
	return hresultOK
}

func runtimeOptionsFalse(this, value uintptr) uintptr {
	if value == 0 || runtimeOptionsAt(this) == nil {
		return hresultPointer
	}
	*(*int32)(unsafe.Pointer(value)) = 0
	return hresultOK
}
