package terminal

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
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
			var child syscall.Handle
			if kind == "powershell" {
				shell, _, err := localShellExecutable(kind)
				if err != nil {
					t.Fatal(err)
				}
				command = "$owned=Start-Process -FilePath '" + strings.ReplaceAll(shell, "'", "''") + "' -ArgumentList '-NoLogo','-NoProfile','-Command','Start-Sleep -Seconds 60' -NoNewWindow -PassThru; Write-Output ('NTK_CHILD_' + $owned.Id)\r"
				if _, err = tr.Write([]byte(command)); err != nil {
					t.Fatal(err)
				}
				marker := regexp.MustCompile(`NTK_CHILD_([0-9]+)`)
				deadline := time.Now().Add(10 * time.Second)
				for time.Now().Before(deadline) {
					mu.Lock()
					match := marker.FindStringSubmatch(output.String())
					mu.Unlock()
					if len(match) > 1 {
						pid, _ := strconv.ParseUint(match[1], 10, 32)
						child, err = syscall.OpenProcess(0x00100000, false, uint32(pid))
						if err != nil {
							t.Fatal(err)
						}
						break
					}
					time.Sleep(20 * time.Millisecond)
				}
				if child == 0 {
					t.Fatal("owned child process did not start")
				}
				defer syscall.CloseHandle(child)
			}
			cancel()
			select {
			case <-tr.Done():
			case <-time.After(10 * time.Second):
				t.Fatal("local shell did not shut down")
			}
			if child != 0 {
				state, err := syscall.WaitForSingleObject(child, 5000)
				if err != nil || state != 0 {
					t.Fatalf("local shell left its owned child running: %d %v", state, err)
				}
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
