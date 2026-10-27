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
var errNativeVaultCaptureUnpainted = errors.New("native fixture client is blank or unrendered")

func captureNativeVaultDialog(hwnd uintptr, filename string) (int, int, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	wndShowWindow.Call(hwnd, 5)
	wndUpdateWindow.Call(hwnd)
	// Include native child controls, not just the outer dialog/title bar.
	if ok, _, _ := wndUser32.NewProc("RedrawWindow").Call(hwnd, 0, 0, 0x585); ok == 0 { // INVALIDATE|ERASE|ALLCHILDREN|UPDATENOW|FRAME
		return 0, 0, errors.New("repaint native fixture client and children")
	}
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
	for offset := 0; offset < len(pixels); offset += 4 {
		pixels[offset], pixels[offset+2] = pixels[offset+2], pixels[offset]
		pixels[offset+3] = 255
	}
	// Validate the actual client rectangle only. A painted title bar around a
	// completely black client is not rendering evidence.
	var client Rect
	var origin struct{ X, Y int32 }
	if ok, _, _ := wndGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client))); ok == 0 {
		return 0, 0, errors.New("get native fixture client dimensions")
	}
	if ok, _, _ := wndUser32.NewProc("ClientToScreen").Call(hwnd, uintptr(unsafe.Pointer(&origin))); ok == 0 {
		return 0, 0, errors.New("locate native fixture client pixels")
	}
	bounds := image.Rect(int(origin.X-rect.Left), int(origin.Y-rect.Top), int(origin.X-rect.Left+client.Right), int(origin.Y-rect.Top+client.Bottom))
	if !nativeVaultClientPainted(pixels, width, height, bounds) {
		return 0, 0, errNativeVaultCaptureUnpainted
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

// A nontrivial fraction of the client must be painted, with actual control/text
// colors. Uniform client fills and colorful title bars alone are insufficient.
func nativeVaultClientPainted(pixels []byte, width, height int, client image.Rectangle) bool {
	if width <= 0 || height <= 0 || len(pixels) != width*height*4 || client.Empty() || !client.In(image.Rect(0, 0, width, height)) {
		return false
	}
	colors := make(map[uint32]struct{})
	nonblack := 0
	for y := client.Min.Y; y < client.Max.Y; y++ {
		for x := client.Min.X; x < client.Max.X; x++ {
			offset := (y*width + x) * 4
			red, green, blue := pixels[offset], pixels[offset+1], pixels[offset+2]
			if red > 16 || green > 16 || blue > 16 {
				nonblack++
			}
			if len(colors) < 8 {
				colors[uint32(red)<<16|uint32(green)<<8|uint32(blue)] = struct{}{}
			}
		}
	}
	return len(colors) >= 4 && nonblack > client.Dx()*client.Dy()/10
}

type nativeVaultCaptureResult struct {
	width, height int
	err           error
}

func observePaintedNativeVault(ctx context.Context, filename string, result chan<- nativeVaultCaptureResult) func(*nativeVaultDialog) {
	return func(d *nativeVaultDialog) {
		// The observer runs inside WM_INITDIALOG. Return before asking the UI
		// thread to print: capturing synchronously there can precede its first
		// client paint. The result channel synchronizes all captured values.
		go func(hwnd uintptr) {
			captured := nativeVaultCaptureResult{}
			defer func() {
				result <- captured
				if ctx.Err() == nil {
					wndPostMessage.Call(hwnd, wmClose, 0, 0)
				}
			}()
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			var ignored uintptr
			// Cross-thread WM_NULL completes after the initialization callback
			// returns; abort instead of hanging if the synthetic UI cannot pump.
			if ok, _, _ := wndUser32.NewProc("SendMessageTimeoutW").Call(hwnd, 0, 0, 0, 3, 1000, uintptr(unsafe.Pointer(&ignored))); ok == 0 {
				captured.err = errors.New("native fixture initialization did not complete")
				return
			}
			deadline := time.Now().Add(2 * time.Second)
			for {
				if err := ctx.Err(); err != nil {
					captured.err = err
					return
				}
				captured.width, captured.height, captured.err = captureNativeVaultDialog(hwnd, filename)
				if !errors.Is(captured.err, errNativeVaultCaptureUnpainted) || time.Now().After(deadline) {
					return
				}
				timer := time.NewTimer(50 * time.Millisecond)
				select {
				case <-ctx.Done():
					timer.Stop()
					captured.err = ctx.Err()
					return
				case <-timer.C:
				}
			}
		}(d.hwnd)
	}
}

func awaitNativeVaultCapture(t *testing.T, ctx context.Context, done <-chan nativeVaultCaptureResult) nativeVaultCaptureResult {
	t.Helper()
	select {
	case result := <-done:
		return result
	case <-ctx.Done():
		t.Fatal("native fixture capture did not finish before cancellation")
		return nativeVaultCaptureResult{err: ctx.Err()}
	}
}

func TestVaultDialogCaptureRejectsTitleBarOnlyAndUniformClient(t *testing.T) {
	const width, height = 100, 100
	pixels := make([]byte, width*height*4)
	for y := 0; y < 20; y++ {
		for x := 0; x < width; x++ {
			pixels[(y*width+x)*4] = byte(x + 20) // Colorful title, black client.
		}
	}
	client := image.Rect(0, 20, width, height)
	if nativeVaultClientPainted(pixels, width, height, client) {
		t.Fatal("title bar colors counted as client rendering")
	}
	for y := client.Min.Y; y < client.Max.Y; y++ {
		for x := client.Min.X; x < client.Max.X; x++ {
			for c := 0; c < 3; c++ {
				pixels[(y*width+x)*4+c] = 240
			}
		}
	}
	if nativeVaultClientPainted(pixels, width, height, client) {
		t.Fatal("uniform blank client counted as rendered controls")
	}
	for x := 0; x < 8; x++ {
		pixels[(30*width+x)*4] = byte(20 + x*10)
	}
	if !nativeVaultClientPainted(pixels, width, height, client) {
		t.Fatal("painted client content rejected")
	}
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
			done := make(chan nativeVaultCaptureResult, 1)
			_, err = showNativeVaultDialogObserved(ctx, spec, observePaintedNativeVault(ctx, filepath.Join(output, fmt.Sprintf("vault-%s.png", mode)), done))
			if !errors.Is(err, ErrVaultDialogCancelled) {
				t.Fatalf("native screenshot dialog failed: %v", err)
			}
			captured := awaitNativeVaultCapture(t, ctx, done)
			if captured.err != nil {
				t.Fatal(captured.err)
			}
			t.Logf("Captured actual blank native %s dialog: %d x %d", mode, captured.width, captured.height)
			assertVaultDialogRegistryEmpty(t)
		})
	}
	t.Run("recovery", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done := make(chan nativeVaultCaptureResult, 1)
		allowed, err := confirmVaultRecovery(ctx, "SYNTHETIC-OFFLINE-RECOVERY-KEY-NOT-FOR-REAL-USE", func(ctx context.Context, spec vaultDialogSpec) (vaultDialogResult, error) {
			return showNativeVaultDialogObserved(ctx, spec, observePaintedNativeVault(ctx, filepath.Join(output, "vault-recovery.png"), done))
		})
		if allowed || err != nil {
			t.Fatalf("native recovery screenshot dialog failed: %v", err)
		}
		captured := awaitNativeVaultCapture(t, ctx, done)
		if captured.err != nil {
			t.Fatal(captured.err)
		}
		t.Logf("Captured actual native synthetic recovery display: %d x %d", captured.width, captured.height)
		assertVaultDialogRegistryEmpty(t)
	})
}
