//go:build windows && amd64

package desktop

import (
	"testing"
	"unsafe"
)

func TestWindowNativeLayouts(t *testing.T) {
	checks := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"RECT size", unsafe.Sizeof(Rect{}), 16},
		{"WNDCLASSEXW size", unsafe.Sizeof(windowClass{}), 80},
		{"WNDCLASSEXW procedure", unsafe.Offsetof(windowClass{}.Procedure), 8},
		{"MSG size", unsafe.Sizeof(windowMessage{}), 48},
		{"MSG wParam", unsafe.Offsetof(windowMessage{}.WParam), 16},
		{"CREATESTRUCTW size", unsafe.Sizeof(createWindowStruct{}), 80},
		{"CREATESTRUCTW name", unsafe.Offsetof(createWindowStruct{}.Name), 56},
		{"MONITORINFO size", unsafe.Sizeof(monitorInfo{}), 40},
		{"MINMAXINFO size", unsafe.Sizeof(minMaxInfo{}), 40},
		{"PAINTSTRUCT size", unsafe.Sizeof(paintStruct{}), 72},
		{"OPENFILENAMEW size", unsafe.Sizeof(openFilenameW{}), 152},
		{"OPENFILENAMEW file", unsafe.Offsetof(openFilenameW{}.File), 48},
		{"OPENFILENAMEW flags", unsafe.Offsetof(openFilenameW{}.Flags), 96},
		{"OPENFILENAMEW default extension", unsafe.Offsetof(openFilenameW{}.DefaultExt), 104},
		{"OPENFILENAMEW reserved", unsafe.Offsetof(openFilenameW{}.Reserved), 136},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s = %d, want %d", check.name, check.got, check.want)
		}
	}
}

func TestPrepareConfigRequiresLocalServer(t *testing.T) {
	for _, input := range []string{"https://example.com", "http://127.0.0.1", "http://localhost:1234", "http://192.168.1.1:1234", "file:///C:/report.html", "http://user@127.0.0.1:1234", "http://127.0.0.1:70000"} {
		if _, err := prepareConfig(Config{URL: input, DataDir: t.TempDir()}); err == nil {
			t.Errorf("nonlocal or invalid origin accepted: %q", input)
		}
	}
	for _, input := range []string{"http://127.0.0.1:1234/", "http://[::1]:1234/"} {
		if _, err := prepareConfig(Config{URL: input, DataDir: t.TempDir()}); err != nil {
			t.Errorf("local origin rejected: %q: %v", input, err)
		}
	}
}
