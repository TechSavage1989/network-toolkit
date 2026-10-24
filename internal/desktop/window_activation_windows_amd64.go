//go:build windows && amd64

package desktop

import "unsafe"

// FLASHWINFO contains an aligned pointer after its UINT size field on Win64.
var (
	_ [32 - unsafe.Sizeof(windowFlashInfo{})]byte
	_ [unsafe.Sizeof(windowFlashInfo{}) - 32]byte
	_ [8 - unsafe.Offsetof(windowFlashInfo{}.Window)]byte
	_ [unsafe.Offsetof(windowFlashInfo{}.Window) - 8]byte
	_ [16 - unsafe.Offsetof(windowFlashInfo{}.Flags)]byte
	_ [unsafe.Offsetof(windowFlashInfo{}.Flags) - 16]byte
)
