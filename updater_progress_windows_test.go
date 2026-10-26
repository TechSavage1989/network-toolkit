package main

import (
	"testing"
	"time"
)

func TestUpdateProgressWindowLifecycle(t *testing.T) {
	progress := startUpdateProgressWindow("1.17.99")
	if _, ok := progress.(*nativeUpdateProgress); !ok {
		t.Fatal("native installation progress window could not be created")
	}
	progress.Set("Verifying update files...")
	time.Sleep(100 * time.Millisecond)
	done := make(chan struct{})
	go func() { progress.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("installation progress window did not close")
	}
	progress.Set("Late status must be ignored")
	progress.Close()
}
