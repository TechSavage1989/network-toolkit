//go:build windows

package desktop

import (
	"context"
	"errors"
	"runtime"
	"syscall"
	"unsafe"
)

const nativeCredentialConfirmationFlags = 0x00050134

func ConfirmCredentialDestination(ctx context.Context, request CredentialDestinationConfirmation) (bool, error) {
	return confirmCredentialDestination(ctx, request, func(text string) (bool, error) {
		body, _ := syscall.UTF16PtrFromString(text)
		title, _ := syscall.UTF16PtrFromString("Network Toolkit — credential destination approval")
		// YESNO | ICONWARNING | DEFBUTTON2 | SETFOREGROUND | TOPMOST. The
		// default is No. This is a native consent dialog, NOT authentication.
		result, _, _ := wndMessageBox.Call(0, uintptr(unsafe.Pointer(body)), uintptr(unsafe.Pointer(title)), nativeCredentialConfirmationFlags)
		runtime.KeepAlive(body)
		runtime.KeepAlive(title)
		if result == 0 {
			return false, errors.New("Native credential confirmation could not be displayed")
		}
		return result == 6, nil // IDYES only; every other result denies.
	})
}
