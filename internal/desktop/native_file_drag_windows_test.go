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
	"testing"
	"time"
	"unsafe"
)

func TestNativeFileDragEndpointAndStagingBoundary(t *testing.T) {
	for _, good := range []string{"http://127.0.0.1:1234/", "http://127.0.0.1:1234/editor-view/" + strings.Repeat("a", 32)} {
		if _, ok := nativeFileDragEndpoint(good); !ok {
			t.Fatal(good)
		}
	}
	for _, bad := range []string{"https://remote.test/", "http://127.0.0.1:1234/terminal-view/" + strings.Repeat("a", 32), "http://127.0.0.1:1234/editor-view/notvalid", "http://127.0.0.1:1234/editor-view/" + strings.Repeat("a", 32) + "/other"} {
		if _, ok := nativeFileDragEndpoint(bad); ok {
			t.Fatal("broad native surface", bad)
		}
	}
	dir, err := os.MkdirTemp("", "ntk-explorer-copy-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "test.txt")
	os.WriteFile(file, []byte("SYNTHETIC"), 0600)
	job := nativeFileDragJob{ID: strings.Repeat("a", 32), Paths: []string{file}}
	if !validNativeFileDrag(job) {
		t.Fatal("valid staged file rejected")
	}
	job.Paths = []string{os.TempDir()}
	if validNativeFileDrag(job) {
		t.Fatal("arbitrary folder allowed")
	}
	job.Paths = []string{file, filepath.Join(t.TempDir(), "other")}
	if validNativeFileDrag(job) {
		t.Fatal("mixed staging roots")
	}
}
func TestNativeFileDragCOMHasNoAsyncCapability(t *testing.T) {
	o := &nativeFileCOM{vtable: &fileCOMDataTable}
	o.refs.Store(1)
	p := uintptr(unsafe.Pointer(o))
	nativeFileCOMObjects.Store(p, o)
	defer fileCOMRelease(p)
	iid := guid("3d8b0590-f691-11d2-8ea9-006097df5bd4")
	var out uintptr
	if result := fileCOMQuery(p, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&out))); result != hresultNoInterface || out != 0 {
		t.Fatal("async capability exposed")
	}
	if fileCOMContinue(p, 1, 1) != 0x40101 || fileCOMContinue(p, 0, 0) != 0x40100 || fileCOMContinue(p, 0, 1) != 0 {
		t.Fatal("OLE gesture cancellation/drop semantics")
	}
}

func TestNativeFileDragReconcilesLostTransitionResponses(t *testing.T) {
	for _, action := range []string{"start", "finish"} {
		t.Run(action, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var b map[string]any
				json.NewDecoder(r.Body).Decode(&b)
				if b["action"] != action || b["id"] != "same-job" || b["gesture"] != float64(3) || r.Header.Get("X-Session-Token") != "synthetic" {
					t.Error("transition identity changed", b)
				}
				if calls == 1 {
					conn, _, _ := w.(http.Hijacker).Hijack()
					conn.Close()
					return
				}
				w.WriteHeader(200)
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if !nativeFileDragTransition(ctx, server.Client(), server.URL, "synthetic", "same-job", 3, action, "cancelled") || calls != 2 {
				t.Fatal("lost acknowledgement not reconciled", calls)
			}
		})
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(409) }))
	defer server.Close()
	if nativeFileDragTransition(context.Background(), server.Client(), server.URL, "synthetic", "gone", 1, "start", "") || calls != 1 {
		t.Fatal("known rejection retried")
	}
}

func TestNativeFileDragStopsReconciliationWithOwningWindow(t *testing.T) {
	done := make(chan struct{})
	ctx, cancel := nativeFileDragContext(context.Background(), done)
	defer cancel()
	close(done)
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("owning HWND close did not cancel native receipts")
	}
}
