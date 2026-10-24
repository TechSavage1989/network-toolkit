//go:build windows

package desktop

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

func TestRuntimeOptionsIdentityDefaultsAndLifetime(t *testing.T) {
	options := newWebViewEnvironmentOptions()
	base := options.pointer()
	second, err := comQuery(base, iidEnvironmentOptions2)
	if err != nil || second == base {
		t.Fatalf("Options2 must expose its independent SDK vtable: %v", err)
	}
	identity, err := comQuery(second, iidIUnknown)
	if err != nil || identity != base {
		t.Fatalf("Options2 did not preserve the base IUnknown identity: %v", err)
	}
	comRelease(identity)
	options.release() // Model release of the caller's construction reference.
	runtime.GC()
	if runtimeOptionsAt(base) == nil || runtimeOptionsAt(second) == nil {
		t.Fatal("retained environment options were not rooted")
	}
	for _, entry := range []struct {
		slot int
		want string
	}{{3, ""}, {5, ""}, {7, runtimeMinimumCompatibleVersion}} {
		value, err := comGetString(base, entry.slot)
		if err != nil || value != entry.want {
			t.Fatalf("option slot %d: %q %v", entry.slot, value, err)
		}
	}
	for _, entry := range []struct {
		object uintptr
		slot   int
	}{{base, 9}, {second, 3}} {
		value := int32(1)
		if failed(comCall(entry.object, entry.slot, uintptr(unsafe.Pointer(&value)))) || value != 0 {
			t.Fatalf("boolean option slot %d was enabled", entry.slot)
		}
	}
	newArgs, _ := syscall.UTF16PtrFromString("--unexpected-arguments")
	if hr := comCall(base, 4, uintptr(unsafe.Pointer(newArgs))); hr != 1 {
		t.Fatalf("immutable setter returned %x instead of S_FALSE", hr)
	}
	runtime.KeepAlive(newArgs)
	if value, err := comGetString(base, 3); err != nil || value != "" {
		t.Fatal("a setter changed the application's empty browser arguments")
	}
	if _, err := comQuery(base, iidSettings4); err == nil {
		t.Fatal("environment options claimed an unrelated interface")
	}
	if comCall(base, 3, 0) != hresultPointer || comCall(second, 3, 0) != hresultPointer {
		t.Fatal("option getters accepted a null result pointer")
	}
	comRelease(second)
	if runtimeOptionsAt(base) != nil || runtimeOptionsAt(second) != nil {
		t.Fatal("final runtime Release did not remove both interface roots")
	}
}

func TestRuntimeEnvironmentAdapterQueriesAndReleases(t *testing.T) {
	environment := newCOMHandler(iidCoreEnvironment, nil)
	defer environment.release()
	pointer := environment.pointer()
	var kept uintptr
	callback := adaptRuntimeEnvironmentCompleted(func(result, value uintptr) uintptr {
		if failed(result) || value != pointer || environment.refs.Load() != 2 {
			t.Fatal("environment was forwarded without an owned public-interface query")
		}
		comAddRef(value)
		kept = value
		return hresultOK
	})
	if failed(callback(hresultOK, pointer)) || kept != pointer || environment.refs.Load() != 2 {
		t.Fatal("adapter did not release its temporary reference after forwarding")
	}
	comRelease(kept)
	if environment.refs.Load() != 1 {
		t.Fatal("receiver's environment reference was not independent")
	}

	wrong := newCOMHandler(iidSettings3, nil)
	defer wrong.release()
	called := false
	callback = adaptRuntimeEnvironmentCompleted(func(result, value uintptr) uintptr {
		called = true
		if !failed(result) || value != 0 {
			t.Fatal("unsupported runtime environment identity was forwarded as success")
		}
		return hresultOK
	})
	callback(hresultOK, wrong.pointer())
	if !called || wrong.refs.Load() != 1 {
		t.Fatal("failed identity query leaked a reference or omitted completion")
	}
}
