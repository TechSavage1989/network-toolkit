package sharedfile

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Readers open and read a file while a writer replaces it a few hundred
// times. No replace fails because a reader has the file, and no reader sees
// anything but one complete version. On Windows this is the test of delete
// sharing and of the wait for a reader that has the file in the way.
func TestReplaceWhileReadersHoldTheFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "shared.json")
	version := func(n int) []byte { return bytes.Repeat([]byte(fmt.Sprintf("%06d\n", n)), 2048) }
	if err := os.WriteFile(target, version(0), 0600); err != nil {
		t.Fatal(err)
	}
	var stop atomic.Bool
	var reads atomic.Int64
	var problem atomic.Value
	var wg sync.WaitGroup
	reader := func(open func(string) (*os.File, error), hold time.Duration) {
		defer wg.Done()
		for !stop.Load() {
			file, err := open(target)
			if err != nil {
				problem.CompareAndSwap(nil, "open: "+err.Error())
				return
			}
			raw, err := io.ReadAll(file)
			if hold > 0 {
				time.Sleep(hold) // a program that keeps the file open for a while
			}
			file.Close()
			if hold > 0 {
				time.Sleep(5 * hold) // and comes back a little later
			}
			if err != nil {
				problem.CompareAndSwap(nil, "read: "+err.Error())
				return
			}
			if len(raw) != 7*2048 || !bytes.Equal(raw, bytes.Repeat(raw[:7], 2048)) {
				problem.CompareAndSwap(nil, fmt.Sprintf("a reader saw %d bytes that are not one version", len(raw)))
				return
			}
			reads.Add(1)
		}
	}
	for n := 0; n < 4; n++ {
		wg.Add(1)
		go reader(Open, 0)
	}
	// One reader that does not allow deletion while it has the file (a virus
	// scanner, a backup program, an older version of this app).
	wg.Add(1)
	go reader(os.Open, 2*time.Millisecond)
	for n := 1; n <= 300; n++ {
		temporary := filepath.Join(dir, fmt.Sprintf(".next-%d", n))
		if err := os.WriteFile(temporary, version(n), 0600); err != nil {
			t.Fatal(err)
		}
		if err := Replace(temporary, target); err != nil {
			stop.Store(true)
			wg.Wait()
			t.Fatalf("replace %d failed while readers had the file open: %v", n, err)
		}
	}
	stop.Store(true)
	wg.Wait()
	if p := problem.Load(); p != nil {
		t.Fatal(p)
	}
	if raw, err := os.ReadFile(target); err != nil || !bytes.Equal(raw, version(300)) {
		t.Fatalf("the last version is not in place: %v", err)
	}
	if reads.Load() == 0 {
		t.Fatal("no read completed")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("temporary files were left behind: %d entries", len(entries))
	}
}

func TestRetryStopsOnSuccessOnOtherErrorsAndWhenPatienceRunsOut(t *testing.T) {
	busy, other := errors.New("busy"), errors.New("other")
	again := func(err error) bool { return errors.Is(err, busy) }
	calls := 0
	if err := Retry(func() error {
		if calls++; calls < 4 {
			return busy
		}
		return nil
	}, again); err != nil || calls != 4 {
		t.Fatalf("a call that succeeds on the fourth try: %v after %d calls", err, calls)
	}
	calls = 0
	if err := Retry(func() error { calls++; return other }, again); !errors.Is(err, other) || calls != 1 {
		t.Fatalf("an error that is not worth repeating: %v after %d calls", err, calls)
	}
	started := time.Now()
	if err := Retry(func() error { return busy }, again); !errors.Is(err, busy) || time.Since(started) < patience || time.Since(started) > patience+time.Second {
		t.Fatalf("patience: %v after %v", err, time.Since(started))
	}
	if _, err := Open(filepath.Join(t.TempDir(), "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a missing file: %v", err)
	}
}
