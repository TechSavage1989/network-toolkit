//go:build windows

package desktop

import (
	"context"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestNativeHandoff122CanonicalScriptAndExactBoolean(t *testing.T) {
	id := strings.Repeat("a", 32)
	for _, bad := range []string{"", id + "'", strings.Repeat("g", 32), "../" + id} {
		if _, ok := nativeFileHandoffScript(bad, 1); ok {
			t.Fatal("noncanonical job accepted")
		}
	}
	if _, ok := nativeFileHandoffScript(id, 2); ok {
		t.Fatal("foreign gesture accepted")
	}
	expected, _ := nativeFileHandoffScript(id, 1)
	for _, tc := range []struct {
		raw  string
		hr   uintptr
		want bool
	}{{"true", 0, true}, {"false", 0, false}, {`"true"`, 0, false}, {"null", 0, false}, {"", 0, false}, {"true", uintptr(0x80004004), false}} {
		t.Run(tc.raw+string(rune(tc.hr)), func(t *testing.T) {
			type obj struct{ vt *[30]uintptr }
			vt := [30]uintptr{}
			var callback uintptr
			matched := true
			vt[29] = syscall.NewCallback(func(_, script, handler uintptr) uintptr {
				for i, ch := range expected + "\x00" {
					if *(*uint16)(unsafe.Add(unsafe.Pointer(script), i*2)) != uint16(ch) {
						matched = false
						break
					}
				}
				callback = handler
				comAddRef(callback)
				return hresultOK
			})
			object := &obj{vt: &vt}
			w := &WebView{core: uintptr(unsafe.Pointer(object))}
			calls := 0
			got := false
			if e := w.releaseNativeFileCapture(id, 1, func(ok bool) { calls++; got = ok }); e != nil {
				t.Fatal(e)
			}
			if callback == 0 || !matched {
				t.Fatal("wrong fixed script or missing callback")
			}
			runtime.GC()
			var raw *uint16
			if tc.raw != "" {
				raw, _ = syscall.UTF16PtrFromString(tc.raw)
			}
			comCall(callback, 3, tc.hr, uintptr(unsafe.Pointer(raw)))
			comRelease(callback)
			runtime.KeepAlive(raw)
			runtime.KeepAlive(object)
			if calls != 1 || got != tc.want {
				t.Fatalf("calls=%d result=%v", calls, got)
			}
			if handlerAt(callback) != nil {
				t.Fatal("callback retained beyond COM release")
			}
		})
	}
}
func TestNativeHandoff122WaitFailsClosed(t *testing.T) {
	t.Run("not held or rejected post", func(t *testing.T) {
		called := false
		if awaitNativeFileCapture(context.Background(), func() bool { return false }, func(func(bool)) bool { called = true; return true }) || called {
			t.Fatal("unheld admission")
		}
		if awaitNativeFileCapture(context.Background(), func() bool { return true }, func(func(bool)) bool { return false }) {
			t.Fatal("rejected dispatch accepted")
		}
	})
	t.Run("release pending script cannot become a new press", func(t *testing.T) {
		var held atomic.Bool
		held.Store(true)
		var late func(bool)
		result := make(chan bool, 1)
		begun := make(chan struct{})
		go func() {
			result <- awaitNativeFileCapture(context.Background(), held.Load, func(done func(bool)) bool { late = done; close(begun); return true })
		}()
		<-begun
		held.Store(false)
		if <-result {
			t.Fatal("released gesture accepted")
		}
		held.Store(true)
		late(true)
	})
	t.Run("cancel and timeout ignore late callback", func(t *testing.T) {
		for _, cancelNow := range []bool{true, false} {
			ctx, cancel := context.WithCancel(context.Background())
			var late func(bool)
			if awaitNativeFileCapture(ctx, func() bool { return true }, func(done func(bool)) bool {
				late = done
				if cancelNow {
					cancel()
				}
				return true
			}) {
				t.Fatal("missing or cancelled acknowledgement accepted")
			}
			late(true)
			cancel()
		}
	})
	t.Run("exact acceptance", func(t *testing.T) {
		if !awaitNativeFileCapture(context.Background(), func() bool { return true }, func(done func(bool)) bool { done(true); return true }) {
			t.Fatal("valid ack refused")
		}
	})
}
func TestNativeHandoff122GuardPersistsUntilOLEAdmission(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered := make(chan struct{})
	var state atomic.Uint32
	var held atomic.Bool
	held.Store(true)
	stopped := make(chan struct{})
	go func() { monitorNativeFileAdmission(ctx, cancel, entered, &state, held.Load); close(stopped) }()
	// Model successful script result followed by a queued OLE dispatch. A release
	// observed in this gap must remain cancelled after a new press.
	held.Store(false)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("guard did not observe release")
	}
	held.Store(true)
	if ctx.Err() == nil {
		t.Fatal("later press revived old gesture")
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	entered2 := make(chan struct{})
	var state2 atomic.Uint32
	stopped2 := make(chan struct{})
	held.Store(true)
	go func() { monitorNativeFileAdmission(ctx2, cancel2, entered2, &state2, held.Load); close(stopped2) }()
	state2.Store(1)
	close(entered2)
	<-stopped2
	held.Store(false)
	if ctx2.Err() != nil {
		t.Fatal("admitted OLE incorrectly retained foreground monitor")
	}
}

func TestNativeHandoff122GuardCannotCancelAfterAtomicAdmission(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var state atomic.Uint32
	entered := make(chan struct{})
	checking := make(chan struct{})
	resume := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		monitorNativeFileAdmission(ctx, cancel, entered, &state, func() bool { close(checking); <-resume; return false })
		close(stopped)
	}()
	<-checking
	if !state.CompareAndSwap(0, 1) {
		t.Fatal("unexpected cancelled admission")
	}
	close(entered)
	close(resume)
	<-stopped
	if ctx.Err() != nil || state.Load() != 1 {
		t.Fatal("retired monitor cancelled an admitted OLE operation")
	}
}

func TestNativeHandoff122CaptureTransferIsOwnerScoped(t *testing.T) {
	for _, name := range []string{"descendant", "owner", "none", "foreign-root", "foreign-thread", "unheld", "unknown-capture", "set-failed", "release-failed", "capture-returned", "released-during-transfer"} {
		t.Run(name, func(t *testing.T) {
			const owner uintptr = 100
			captured := uintptr(200)
			held := true
			setCalls, releaseCalls := 0, 0
			ops := nativeFileCaptureOps{
				currentThread: func() uint32 {
					if name == "foreign-thread" {
						return 9
					}
					return 7
				}, identity: func(uintptr) (uint32, uint32) { return 7, 1 },
				capture: func(uint32) (uintptr, bool) { return captured, name != "unknown-capture" }, root: func(uintptr) uintptr {
					if name == "foreign-root" {
						return 999
					}
					return owner
				},
				set: func(window uintptr) {
					setCalls++
					if window != owner {
						t.Fatal("attempted foreign capture")
					}
					if name != "set-failed" {
						captured = window
					}
					if name == "released-during-transfer" {
						held = false
					}
				},
				release: func() bool {
					releaseCalls++
					if captured != owner {
						t.Fatal("released unowned capture")
					}
					if name == "release-failed" {
						return false
					}
					captured = 0
					if name == "capture-returned" {
						captured = 200
					}
					return true
				},
			}
			if name == "owner" {
				captured = owner
			}
			if name == "none" {
				captured = 0
			}
			if name == "unheld" {
				held = false
			}
			got := transferNativeFileCapture(owner, func() bool { return held }, ops)
			want := name == "descendant" || name == "owner" || name == "none"
			if got != want {
				t.Fatalf("accepted=%v want%v", got, want)
			}
			if name == "foreign-root" || name == "foreign-thread" || name == "unheld" || name == "unknown-capture" || name == "none" {
				if setCalls != 0 || releaseCalls != 0 {
					t.Fatal("mutated capture outside the admitted owner")
				}
			}
			if name == "descendant" && (setCalls != 1 || releaseCalls != 1) {
				t.Fatal("did not complete own-thread capture transfer")
			}
			if name == "released-during-transfer" && releaseCalls != 1 {
				t.Fatal("left owned capture after cancellation")
			}
		})
	}
}
