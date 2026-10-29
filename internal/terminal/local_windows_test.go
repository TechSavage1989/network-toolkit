package terminal

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// Capture actual ConPTY rendering so browser tests can replay real Windows
// prompts/SGR attributes rather than assuming an unformatted synthetic prompt.
func TestWindowsLocalShellColorTranscript(t *testing.T) {
	if !LocalShellAvailable() {
		t.Fatal("ConPTY is unavailable")
	}
	for _, kind := range []string{"cmd", "powershell"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			req, err := normalizeRequest(ConnectRequest{Type: kind, Cols: 120, Rows: 20})
			if err != nil {
				t.Fatal(err)
			}
			var mu sync.Mutex
			var output strings.Builder
			tr, err := openLocalShell(ctx, req, transportCallbacks{Output: func(b []byte) {
				mu.Lock()
				if output.Len()+len(b) <= 128*1024 {
					output.Write(b)
				}
				mu.Unlock()
			}})
			if err != nil {
				t.Fatal(err)
			}
			defer tr.Close()
			deadline := time.Now().Add(10 * time.Second)
			for time.Now().Before(deadline) {
				mu.Lock()
				ready := strings.Contains(output.String(), ">")
				mu.Unlock()
				if ready {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			command := "echo %TEMP% && dir /a REM_NTK_COLOR_SAMPLE"
			if kind == "powershell" {
				command = "Get-ChildItem -Path \"$env:TEMP\" | Select-Object -First 2 # NTK_COLOR_SAMPLE"
			}
			// Intentionally no Enter: this records typing without executing a command.
			if _, err = tr.Write([]byte(command)); err != nil {
				t.Fatal(err)
			}
			time.Sleep(800 * time.Millisecond)
			mu.Lock()
			raw := output.String()
			mu.Unlock()
			if !strings.Contains(raw, ">") || !strings.Contains(raw, "NTK_COLOR_SAMPLE") {
				t.Fatalf("native command line did not render: %q", raw)
			}
			t.Logf("NTK_SHELL_CAPTURE_%s=%s", strings.ToUpper(kind), base64.StdEncoding.EncodeToString([]byte(raw)))
			cancel()
			select {
			case <-tr.Done():
			case <-time.After(10 * time.Second):
				t.Fatal("probe shell did not close")
			}
		})
	}
}

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
			// ConPTY can split prompt/color sequences across reads. Only new
			// output may acknowledge a command or completed interruption.
			control := regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07]*(?:\x07|\x1b\\))`)
			prompt := regexp.MustCompile(`(?m)^(?:PS )?[^\r\n>]*>[ \t]*\r?$`)
			watermark := func() int {
				mu.Lock()
				defer mu.Unlock()
				return output.Len()
			}
			waitForSince := func(from int, description string, ready func(string) bool) {
				t.Helper()
				deadline := time.Now().Add(30 * time.Second)
				for time.Now().Before(deadline) {
					mu.Lock()
					text := output.String()
					mu.Unlock()
					if ready(control.ReplaceAllString(text[from:], "")) {
						return
					}
					time.Sleep(20 * time.Millisecond)
				}
				mu.Lock()
				text := output.String()
				mu.Unlock()
				t.Fatalf("missing %s in local shell output after byte %d: %q", description, from, text)
			}
			commandReady := func(command, marker string, requirePrompt bool) {
				t.Helper()
				from := watermark()
				if _, err := tr.Write([]byte(command)); err != nil {
					t.Fatal(err)
				}
				waitForSince(from, marker, func(text string) bool {
					index := strings.Index(text, marker)
					return index >= 0 && (!requirePrompt || prompt.MatchString(text[index+len(marker):]))
				})
			}
			waitForSince(0, "initial shell prompt", prompt.MatchString)
			if kind == "cmd" {
				commandReady("echo NTK_%PROCESSOR_ARCHITECTURE%_READY\r", "NTK_AMD64_READY", true)
			} else {
				commandReady("Write-Output ('NTK_' + 'READY')\r", "NTK_READY", true)
			}
			if err = tr.Resize(67, 19); err != nil {
				t.Fatal(err)
			}
			if kind == "powershell" {
				commandReady("Write-Output ('NTK_WIDTH_' + [Console]::WindowWidth)\r", "NTK_WIDTH_67", true)
			}
			// A long command must accept Ctrl+C and return to the same interactive shell.
			// Wait for ping's first real loopback reply. A preceding echo can
			// complete before the child is ready to handle console input.
			// TTL= is absent from the command echo and is emitted by ping itself.
			command := "ping -n 60 127.0.0.1\r"
			marker := "TTL="
			if kind == "powershell" {
				// The evaluated marker cannot be satisfied by the input echo.
				command = "Write-Output ('NTK_' + 'BUSY'); Start-Sleep -Seconds 60\r"
				marker = "NTK_BUSY"
			}
			commandReady(command, marker, false)
			beforeInterrupt := watermark()
			if _, err = tr.Write([]byte{3}); err != nil {
				t.Fatal(err)
			}
			waitForSince(beforeInterrupt, "new shell prompt after Ctrl+C", prompt.MatchString)
			command = "echo NTK_%PROCESSOR_ARCHITECTURE%_AFTER\r"
			marker = "NTK_AMD64_AFTER"
			if kind == "powershell" {
				command = "Write-Output ('NTK_' + 'AFTER')\r"
				marker = "NTK_AFTER"
			}
			commandReady(command, marker, true)
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

func TestWindowsLocalShellExplicitDirectoryAndToken(t *testing.T) {
	for _, kind := range []string{"cmd", "powershell"} {
		t.Run(kind, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "folder & parentheses (117) 日本語")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			canonical, err := filepath.EvalSymlinks(dir)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			var mu sync.Mutex
			var out strings.Builder
			conn, err := OpenLocalShellProcess(ctx, LocalShellOptions{Type: kind, WorkingDirectory: dir, Cols: 500, Rows: 25}, func(data []byte) {
				mu.Lock()
				defer mu.Unlock()
				if out.Len() < 128<<10 {
					out.Write(data)
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			control := regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07]*(?:\x07|\x1b\\))`)
			readOutput := func() string { mu.Lock(); defer mu.Unlock(); return control.ReplaceAllString(out.String(), "") }
			for !strings.Contains(readOutput(), ">") {
				select {
				case <-ctx.Done():
					t.Fatalf("initial prompt absent; expected=%q actualState=%+v output=%q", canonical, conn.LocalShellState(), readOutput())
				case <-time.After(20 * time.Millisecond):
				}
			}
			mu.Lock()
			from := out.Len()
			mu.Unlock()
			command := "echo NTK_LOCAL117_CWD=%CD%\r"
			if kind == "powershell" {
				command = "Write-Output ('NTK_LOCAL117_CWD='+(Get-Location).Path)\r"
			}
			if _, err = conn.Write([]byte(command)); err != nil {
				t.Fatal(err)
			}
			for {
				mu.Lock()
				found := strings.Contains(control.ReplaceAllString(out.String()[from:], ""), "NTK_LOCAL117_CWD="+canonical)
				mu.Unlock()
				if found {
					break
				}
				select {
				case <-ctx.Done():
					t.Fatalf("literal working directory not reported; requested=%q canonical=%q actualState=%+v output=%q", dir, canonical, conn.LocalShellState(), readOutput())
				case <-time.After(20 * time.Millisecond):
				}
			}
			if conn.LocalShellState().WorkingDirectory != canonical {
				t.Fatal(conn.LocalShellState())
			}
			current, _ := syscall.GetCurrentProcess()
			elevated, err := localProcessElevated(current)
			if err != nil || conn.LocalShellState().Elevated != elevated {
				t.Fatal("ordinary child must inherit actual parent token", err)
			}
			t.Logf("shell=%s parentElevated=%t childElevated=%t literalDirectory=true", kind, elevated, conn.LocalShellState().Elevated)
			cancel()
			select {
			case <-conn.Done():
			case <-time.After(8 * time.Second):
				t.Fatal("shell cleanup timed out")
			}
		})
	}
}
