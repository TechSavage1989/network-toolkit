//go:build windows

package desktop

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"testing"
)

func nativeEditorCloseScriptCommand(ctx context.Context, path string, hwnd uintptr, action string) (*exec.Cmd, error) {
	if action != "WaitWarning" && action != "CancelEdit" {
		return nil, fmt.Errorf("unexpected native close fixture action %q", action)
	}
	return exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-File", path, "-WindowHandle", strconv.FormatUint(uint64(hwnd), 10), "-OwnerProcessID", strconv.Itoa(os.Getpid()), "-Action", action, "-Diagnostics"), nil
}

func runNativeEditorCloseScript(ctx context.Context, path string, hwnd uintptr, action string) ([]byte, error) {
	command, err := nativeEditorCloseScriptCommand(ctx, path, hwnd, action)
	if err != nil {
		return nil, err
	}
	return command.CombinedOutput()
}

func TestNativeEditorCloseScriptUsesExplicitAction(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, action := range []string{"WaitWarning", "CancelEdit"} {
		command, err := nativeEditorCloseScriptCommand(ctx, "synthetic-close.ps1", 42, action)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"powershell.exe", "-NoProfile", "-NonInteractive", "-File", "synthetic-close.ps1", "-WindowHandle", "42", "-OwnerProcessID", strconv.Itoa(os.Getpid()), "-Action", action, "-Diagnostics"}
		if !reflect.DeepEqual(command.Args, want) || command.Cancel == nil {
			t.Fatalf("exact fixture command/context guard changed: %v", command.Args)
		}
	}
	if _, err := nativeEditorCloseScriptCommand(ctx, "synthetic-close.ps1", 42, ""); err == nil {
		t.Fatal("implicit action was accepted")
	}
}
