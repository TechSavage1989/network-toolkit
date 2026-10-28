//go:build windows

package desktop

import (
	"context"
	"os"
	"os/exec"
	"strconv"
)

func runNativeEditorCloseScript(ctx context.Context, path string, hwnd uintptr) ([]byte, error) {
	return exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-File", path, "-WindowHandle", strconv.FormatUint(uint64(hwnd), 10), "-OwnerProcessID", strconv.Itoa(os.Getpid())).CombinedOutput()
}
