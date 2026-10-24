//go:build windows && amd64

package desktop

import "unsafe"

// These Win64 ABI invariants are compiled into every Windows build. They are
// not dependent on executing the Windows tests on the build host.
var (
	_ [80 - unsafe.Sizeof(windowClass{})]byte
	_ [unsafe.Sizeof(windowClass{}) - 80]byte
	_ [48 - unsafe.Sizeof(windowMessage{})]byte
	_ [unsafe.Sizeof(windowMessage{}) - 48]byte
	_ [80 - unsafe.Sizeof(createWindowStruct{})]byte
	_ [unsafe.Sizeof(createWindowStruct{}) - 80]byte
	_ [72 - unsafe.Sizeof(paintStruct{})]byte
	_ [unsafe.Sizeof(paintStruct{}) - 72]byte
	_ [152 - unsafe.Sizeof(openFilenameW{})]byte
	_ [unsafe.Sizeof(openFilenameW{}) - 152]byte
	_ [48 - unsafe.Offsetof(openFilenameW{}.File)]byte
	_ [unsafe.Offsetof(openFilenameW{}.File) - 48]byte
	_ [104 - unsafe.Offsetof(openFilenameW{}.DefaultExt)]byte
	_ [unsafe.Offsetof(openFilenameW{}.DefaultExt) - 104]byte
)
