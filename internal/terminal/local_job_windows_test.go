//go:build windows

package terminal

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
	"unsafe"
)

// Owner report on v1.21.82: SentinelOne's Behavioral AI reported "Preload
// Injection" when the app started a shell suspended and resumed it. The shell
// now starts inside its cleanup job (PROC_THREAD_ATTRIBUTE_JOB_LIST) and is
// never created suspended.
func TestLocalShellStartsInItsJobWithoutASuspendedStart(t *testing.T) {
	if localCreationFlags&0x00000004 != 0 {
		t.Fatal("local shells are created suspended")
	}
	source, err := os.ReadFile("local_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, word := range []string{"ResumeThread", "AssignProcessToJobObject", "0x00000004"} {
		if strings.Contains(string(source), word) {
			t.Fatalf("local_windows.go uses %s", word)
		}
	}
	if !LocalShellAvailable() {
		t.Skip("ConPTY is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := OpenLocalShellProcess(ctx, LocalShellOptions{Type: "cmd", Cols: 80, Rows: 24}, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	shell, ok := conn.(*localShellTransport)
	if !ok {
		t.Fatalf("unexpected transport %T", conn)
	}
	var inJob int32
	r, _, callErr := localKernel.NewProc("IsProcessInJob").Call(uintptr(shell.process), uintptr(shell.job), uintptr(unsafe.Pointer(&inJob)))
	if r == 0 || inJob == 0 {
		t.Fatalf("the shell is not in its cleanup job: %v", callErr)
	}
}
