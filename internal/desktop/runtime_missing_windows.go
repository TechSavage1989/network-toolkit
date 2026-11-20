//go:build windows

package desktop

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

var runtimePageShell = syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW")

// openRuntimeDownloadPage opens Microsoft's WebView2 page in the default
// browser, on the user's click only. The address is the fixed constant.
func openRuntimeDownloadPage() error {
	verb, _ := syscall.UTF16PtrFromString("open")
	file, err := syscall.UTF16PtrFromString(WebViewRuntimeDownloadPage)
	if err != nil {
		return err
	}
	result, _, _ := runtimePageShell.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(file)), 0, 0, 1)
	runtime.KeepAlive(verb)
	runtime.KeepAlive(file)
	if result <= 32 {
		return fmt.Errorf("open the WebView2 page: Windows shell code %d", result)
	}
	return nil
}
