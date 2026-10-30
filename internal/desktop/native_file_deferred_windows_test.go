//go:build windows

package desktop

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

func TestNativeDeferred121EarlyProbeDoesNotReadOrAdvertiseAsync(t *testing.T) {
	d := &nativeDeferredCopy{ctx: context.Background(), held: func() bool { return true }}
	f := nativeFormat{Format: 15, Aspect: 1, Index: -1, Medium: 1}
	var medium nativeMedium
	if got := d.query(uintptr(unsafe.Pointer(&f))); got != 0 {
		t.Fatalf("HDROP not advertised: %x", got)
	}
	if got := d.get(uintptr(unsafe.Pointer(&f)), uintptr(unsafe.Pointer(&medium))); got != 0x80040064 || medium.Handle != 0 || d.loading {
		t.Fatal("early hover read data", got)
	}
	f.Format = nativeInLoopFormat
	if got := d.get(uintptr(unsafe.Pointer(&f)), uintptr(unsafe.Pointer(&medium))); got != 0 {
		t.Fatalf("loop flag: %x", got)
	}
	p, _, _ := nativeLock.Call(medium.Handle)
	value := *(*uint32)(unsafe.Pointer(p))
	nativeUnlock.Call(medium.Handle)
	nativeFree.Call(medium.Handle)
	if value != 1 {
		t.Fatal("held pointer not advertised")
	}
	d.held = func() bool { return false }
	f.Format = 15
	if got := d.get(uintptr(unsafe.Pointer(&f)), uintptr(unsafe.Pointer(&medium))); got != 0x80040064 || d.loading {
		t.Fatal("released but unaccepted probe read source")
	}
	f.Format = nativeInLoopFormat
	if got := d.get(uintptr(unsafe.Pointer(&f)), uintptr(unsafe.Pointer(&medium))); got != 0 {
		t.Fatal(got)
	}
	p, _, _ = nativeLock.Call(medium.Handle)
	value = *(*uint32)(unsafe.Pointer(p))
	nativeUnlock.Call(medium.Handle)
	nativeFree.Call(medium.Handle)
	if value != 0 {
		t.Fatal("released pointer still in loop")
	}
	wrapper := &nativeFileCOM{vtable: &fileCOMDataTable, delayed: d}
	wrapper.refs.Store(1)
	ptr := uintptr(unsafe.Pointer(wrapper))
	nativeFileCOMObjects.Store(ptr, wrapper)
	defer fileCOMRelease(ptr)
	iid := guid("3d8b0590-f691-11d2-8ea9-006097df5bd4")
	var out uintptr
	if fileCOMQuery(ptr, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&out))) != hresultNoInterface || out != 0 {
		t.Fatal("async lifetime exposed")
	}
}
func TestNativeDeferred121PublishedPathsAreBoundToJobAndStage(t *testing.T) {
	stage, e := os.MkdirTemp("", "ntk-explorer-copy-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(stage)
	file := filepath.Join(stage, "synthetic.txt")
	os.WriteFile(file, []byte("SYNTHETIC"), 0600)
	for _, mode := range []string{"valid", "foreign-id", "foreign-gesture", "outside-stage", "registration-only", "revoked"} {
		t.Run(mode, func(t *testing.T) {
			id := strings.Repeat("a", 32)
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Session-Token") != "scoped-only" {
					t.Error("wrong token")
				}
				if r.Method == "POST" {
					var body map[string]any
					json.NewDecoder(r.Body).Decode(&body)
					if body["action"] != "materialize" || body["id"] != id {
						t.Error("wrong transition")
					}
					w.WriteHeader(200)
					return
				}
				calls.Add(1)
				if mode == "revoked" {
					w.WriteHeader(409)
					return
				}
				job := nativeFileDragJob{ID: id, Gesture: 7, Paths: []string{file}}
				if mode == "foreign-id" {
					job.ID = strings.Repeat("b", 32)
				}
				if mode == "foreign-gesture" {
					job.Gesture++
				}
				if mode == "registration-only" {
					job.Direct = true
					job.Paths = nil
				}
				if mode == "outside-stage" {
					job.Paths = []string{os.TempDir()}
				}
				json.NewEncoder(w).Encode(job)
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			d := &nativeDeferredCopy{ctx: ctx, client: server.Client(), endpoint: server.URL, token: "scoped-only", job: nativeFileDragJob{ID: id, Gesture: 7, Direct: true}}
			paths, e := d.materialize()
			if (e == nil) != (mode == "valid") {
				t.Fatal(mode, e)
			}
			if mode != "valid" && paths != nil {
				t.Fatal("invalid paths exposed")
			}
			if calls.Load() != 1 {
				t.Fatal("known rejection retried")
			}
		})
	}
}
func TestNativeDeferred121CancellationJoinsMaterialization(t *testing.T) {
	entered := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.WriteHeader(200)
			return
		}
		close(entered)
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := &nativeDeferredCopy{ctx: ctx, client: server.Client(), endpoint: server.URL, token: "scoped", job: nativeFileDragJob{ID: strings.Repeat("a", 32), Gesture: 1, Direct: true}}
	done := make(chan error, 1)
	go func() { _, e := d.materialize(); done <- e }()
	<-entered
	cancel()
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("cancel returned paths")
		}
	case <-time.After(time.Second):
		t.Fatal("materialize ignored cancellation")
	}
}

func TestNativeDeferred121OnlyAcceptedOLEDropAllowsRendering(t *testing.T) {
	for _, escape := range []uintptr{0, 1} {
		ctx, cancel := context.WithCancel(context.Background())
		d := &nativeDeferredCopy{ctx: ctx, cancel: cancel}
		source := &nativeFileCOM{source: true, ctx: ctx, delayed: d}
		p := uintptr(unsafe.Pointer(source))
		nativeFileCOMObjects.Store(p, source)
		result := fileCOMContinue(p, escape, 0)
		nativeFileCOMObjects.Delete(p)
		if escape == 1 {
			if result != 0x40101 || d.dropAllowed.Load() || ctx.Err() == nil {
				t.Fatal("Escape authorized rendering")
			}
		} else if result != 0x40100 || !d.dropAllowed.Load() {
			t.Fatal("accepted drop not latched")
		}
		cancel()
	}
}
