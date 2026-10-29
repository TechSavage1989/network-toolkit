package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEditorMemoryAdmissionSerializesAndReleases(t *testing.T) {
	var active, peak atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, release, ok := admitEditorMemory(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil))
			if !ok {
				t.Error("unexpected admission rejection")
				return
			}
			n := active.Add(1)
			for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
			}
			_, nested, ok := admitEditorMemory(httptest.NewRecorder(), r)
			if !ok {
				t.Error("nested proxy rejected")
			}
			nested()
			time.Sleep(time.Millisecond)
			active.Add(-1)
			release()
			release()
		}()
	}
	wg.Wait()
	if peak.Load() != 1 {
		t.Fatalf("concurrent heavy allocation=%d", peak.Load())
	}
}
func TestEditorMemoryAdmissionCancellationAndErrorRelease(t *testing.T) {
	_, release, ok := admitEditorMemory(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil))
	if !ok {
		t.Fatal("initial admission")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() {
		w := httptest.NewRecorder()
		_, _, ok := admitEditorMemory(w, httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx))
		if ok {
			done <- 200
		} else {
			done <- w.Code
		}
	}()
	cancel()
	if code := <-done; code != http.StatusRequestTimeout {
		t.Fatalf("cancel status %d", code)
	}
	release()
	w := httptest.NewRecorder()
	(&application{}).handleLocalTextEditor(w, httptest.NewRequest(http.MethodPost, "/api/file-editor/local", strings.NewReader("{")))
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, release, ok = admitEditorMemory(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx))
	if !ok {
		t.Fatal("error path leaked admission")
	}
	release()
}

type editorAdmissionBody struct{ reads atomic.Int32 }

func (b *editorAdmissionBody) Read(p []byte) (int, error) { b.reads.Add(1); return 0, io.EOF }
func TestEditorMemoryAdmissionPrecedesDecode(t *testing.T) {
	_, release, ok := admitEditorMemory(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil))
	if !ok {
		t.Fatal("initial admission")
	}
	defer release()
	body := &editorAdmissionBody{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	w := httptest.NewRecorder()
	go func() {
		defer close(done)
		(&application{}).handleLocalTextEditor(w, httptest.NewRequest(http.MethodPost, "/api/file-editor/local", body).WithContext(ctx))
	}()
	cancel()
	<-done
	if body.reads.Load() != 0 || w.Code != http.StatusRequestTimeout {
		t.Fatalf("read before admission: reads=%d status=%d", body.reads.Load(), w.Code)
	}
}

func TestEditorMemorySmallDisconnectRemainsResponsive(t *testing.T) {
	_, release, ok := admitEditorMemory(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil))
	if !ok {
		t.Fatal("initial admission")
	}
	defer release()
	// Unknown connection still reaches ordinary ownership validation immediately;
	// no connection is created and no large-operation slot is needed.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	w := httptest.NewRecorder()
	(&application{}).handleRemoteBrowser(w, httptest.NewRequest(http.MethodPost, "/api/remote-browser", strings.NewReader(`{"protocol":"ssh","action":"disconnect","connectionId":"missing"}`)).WithContext(ctx))
	if w.Code == http.StatusRequestTimeout {
		t.Fatal("disconnect queued behind held file slot")
	}
	if w.Code != http.StatusConflict {
		t.Fatalf("ordinary validation not retained: %d %s", w.Code, w.Body.String())
	}
}
