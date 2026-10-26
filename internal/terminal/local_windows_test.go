package terminal

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWindowsLocalShellInteractiveLifecycle(t *testing.T) {
	if !LocalShellAvailable() {
		t.Fatal("ConPTY is unavailable on this Windows runner")
	}
	for _, kind := range []string{"cmd", "powershell"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			req, err := normalizeRequest(ConnectRequest{Type: kind, Cols: 90, Rows: 24})
			if err != nil {
				t.Fatal(err)
			}
			var mu sync.Mutex
			var output strings.Builder
			tr, err := openLocalShell(ctx, req, transportCallbacks{Output: func(b []byte) { mu.Lock(); output.Write(b); mu.Unlock() }})
			if err != nil {
				t.Fatal(err)
			}
			defer tr.Close()
			waitFor := func(marker string) {
				t.Helper()
				deadline := time.Now().Add(30 * time.Second)
				for time.Now().Before(deadline) {
					mu.Lock()
					text := output.String()
					mu.Unlock()
					if strings.Contains(text, marker) {
						return
					}
					time.Sleep(20 * time.Millisecond)
				}
				mu.Lock()
				text := output.String()
				mu.Unlock()
				t.Fatalf("missing %q in local shell output: %q", marker, text)
			}
			if kind == "cmd" {
				_, err = tr.Write([]byte("echo NTK_%PROCESSOR_ARCHITECTURE%_READY\r"))
				waitFor("NTK_AMD64_READY")
			} else {
				_, err = tr.Write([]byte("Write-Output ('NTK_' + 'READY')\r"))
				waitFor("NTK_READY")
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = tr.Resize(67, 19); err != nil {
				t.Fatal(err)
			}
			if kind == "powershell" {
				_, err = tr.Write([]byte("Write-Output ('NTK_WIDTH_' + [Console]::WindowWidth)\r"))
				if err != nil {
					t.Fatal(err)
				}
				waitFor("NTK_WIDTH_67")
			}
			// A long command must accept Ctrl+C and return to the same interactive shell.
			command := "ping -n 60 127.0.0.1 >nul\r"
			if kind == "powershell" {
				command = "Start-Sleep -Seconds 60\r"
			}
			if _, err = tr.Write([]byte(command)); err != nil {
				t.Fatal(err)
			}
			time.Sleep(300 * time.Millisecond)
			if _, err = tr.Write([]byte{3}); err != nil {
				t.Fatal(err)
			}
			time.Sleep(150 * time.Millisecond)
			command = "echo NTK_%PROCESSOR_ARCHITECTURE%_AFTER\r"
			marker := "NTK_AMD64_AFTER"
			if kind == "powershell" {
				command = "Write-Output ('NTK_' + 'AFTER')\r"
				marker = "NTK_AFTER"
			}
			if _, err = tr.Write([]byte(command)); err != nil {
				t.Fatal(err)
			}
			waitFor(marker)
			cancel()
			select {
			case <-tr.Done():
			case <-time.After(10 * time.Second):
				t.Fatal("local shell did not shut down")
			}
			if _, err = tr.Write([]byte("must fail")); err == nil {
				t.Fatal("closed local shell accepted input")
			}
			if err = tr.Resize(80, 24); err == nil {
				t.Fatal("closed local shell accepted resize")
			}
			if err = tr.Close(); err != nil {
				t.Fatal(fmt.Sprint(err))
			}
		})
	}
}
