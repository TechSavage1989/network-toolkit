//go:build windows

package desktop

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
	"unsafe"
)

// This capture implementation is linked only into the test executable. It
// prints the isolated test HWND, never reads the desktop or clipboard, and is
// used only for blank inputs and clearly marked synthetic recovery fixtures.
func captureNativeVaultDialog(hwnd uintptr, filename string) (int, int, error) {
	wndShowWindow.Call(hwnd, 5)
	wndUpdateWindow.Call(hwnd)
	var rect Rect
	if ok, _, _ := wndGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return 0, 0, errors.New("get native fixture dimensions")
	}
	width, height := int(rect.Right-rect.Left), int(rect.Bottom-rect.Top)
	if width < 100 || height < 100 || width > 4096 || height > 4096 {
		return 0, 0, errors.New("invalid native fixture dimensions")
	}
	dc, _, _ := wndUser32.NewProc("GetWindowDC").Call(hwnd)
	if dc == 0 {
		return 0, 0, errors.New("get native fixture window DC")
	}
	defer wndUser32.NewProc("ReleaseDC").Call(hwnd, dc)
	compatible, _, _ := wndGDI32.NewProc("CreateCompatibleDC").Call(dc)
	if compatible == 0 {
		return 0, 0, errors.New("create native fixture capture DC")
	}
	defer wndGDI32.NewProc("DeleteDC").Call(compatible)
	bitmap, _, _ := wndGDI32.NewProc("CreateCompatibleBitmap").Call(dc, uintptr(width), uintptr(height))
	if bitmap == 0 {
		return 0, 0, errors.New("create native fixture capture bitmap")
	}
	defer wndDeleteObject.Call(bitmap)
	old, _, _ := wndSelectObject.Call(compatible, bitmap)
	if old == 0 || old == ^uintptr(0) {
		return 0, 0, errors.New("select native fixture capture bitmap")
	}
	printed, _, _ := wndUser32.NewProc("PrintWindow").Call(hwnd, compatible, 2) // PW_RENDERFULLCONTENT
	wndSelectObject.Call(compatible, old)                                       // GetDIBits requires the bitmap unselected.
	if printed == 0 {
		return 0, 0, errors.New("PrintWindow could not render the actual native fixture")
	}
	type bitmapHeader struct {
		Size            uint32
		Width           int32
		Height          int32
		Planes          uint16
		BitCount        uint16
		Compression     uint32
		SizeImage       uint32
		XPelsPerMeter   int32
		YPelsPerMeter   int32
		ColorsUsed      uint32
		ColorsImportant uint32
	}
	header := bitmapHeader{Size: 40, Width: int32(width), Height: -int32(height), Planes: 1, BitCount: 32}
	pixels := make([]byte, width*height*4)
	lines, _, _ := wndGDI32.NewProc("GetDIBits").Call(compatible, bitmap, 0, uintptr(height), uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&header)), 0)
	runtime.KeepAlive(pixels)
	runtime.KeepAlive(header)
	if int(lines) != height {
		return 0, 0, errors.New("read native fixture bitmap pixels")
	}
	colors := make(map[uint32]struct{})
	for offset := 0; offset < len(pixels); offset += 4 {
		pixels[offset], pixels[offset+2] = pixels[offset+2], pixels[offset]
		pixels[offset+3] = 255
		if len(colors) < 8 {
			colors[uint32(pixels[offset])<<16|uint32(pixels[offset+1])<<8|uint32(pixels[offset+2])] = struct{}{}
		}
	}
	if len(colors) < 4 {
		return 0, 0, errors.New("native fixture capture is blank or unrendered")
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		return 0, 0, err
	}
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return 0, 0, err
	}
	img := &image.NRGBA{Pix: pixels, Stride: width * 4, Rect: image.Rect(0, 0, width, height)}
	err = png.Encode(file, img)
	closeErr := file.Close()
	if err != nil {
		return 0, 0, err
	}
	if closeErr != nil {
		return 0, 0, closeErr
	}
	return width, height, nil
}

func TestVaultDialogRuntimeScreenshots(t *testing.T) {
	requireVaultDialogRuntime(t)
	output := os.Getenv("TOOLKIT_NATIVE_TEST_OUTPUT")
	if output == "" {
		t.Skip("set TOOLKIT_NATIVE_TEST_OUTPUT to retain real native fixture PNGs")
	}
	for _, mode := range []VaultSecretMode{VaultSecretSetup, VaultSecretUnlock} {
		t.Run(string(mode), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			spec, err := vaultSecretSpec(VaultSecretRequest{Mode: mode})
			if err != nil {
				t.Fatal(err)
			}
			var captureErr error
			var width, height int
			_, err = showNativeVaultDialogObserved(ctx, spec, func(d *nativeVaultDialog) {
				width, height, captureErr = captureNativeVaultDialog(d.hwnd, filepath.Join(output, fmt.Sprintf("vault-%s.png", mode)))
				wndPostMessage.Call(d.hwnd, wmClose, 0, 0)
			})
			if !errors.Is(err, ErrVaultDialogCancelled) || captureErr != nil {
				t.Fatalf("native screenshot capture failed: dialog=%v capture=%v", err, captureErr)
			}
			t.Logf("Captured actual blank native %s dialog: %d x %d", mode, width, height)
			assertVaultDialogRegistryEmpty(t)
		})
	}
	t.Run("recovery", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var captureErr error
		var width, height int
		allowed, err := confirmVaultRecovery(ctx, "SYNTHETIC-OFFLINE-RECOVERY-KEY-NOT-FOR-REAL-USE", func(ctx context.Context, spec vaultDialogSpec) (vaultDialogResult, error) {
			return showNativeVaultDialogObserved(ctx, spec, func(d *nativeVaultDialog) {
				width, height, captureErr = captureNativeVaultDialog(d.hwnd, filepath.Join(output, "vault-recovery.png"))
				wndPostMessage.Call(d.hwnd, wmClose, 0, 0)
			})
		})
		if allowed || err != nil || captureErr != nil {
			t.Fatalf("native recovery screenshot capture failed: dialog=%v capture=%v", err, captureErr)
		}
		t.Logf("Captured actual native synthetic recovery display: %d x %d", width, height)
		assertVaultDialogRegistryEmpty(t)
	})
}
