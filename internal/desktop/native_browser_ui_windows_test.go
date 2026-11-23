//go:build windows

package desktop

import (
	"errors"
	"fmt"
	"unsafe"
)

// Called by the installed-runtime test on its native UI thread. Read the
// settings back from the actual WebView2 instead of checking source strings or
// mirroring the setters in a mock. This also verifies their SDK vtable slots.
func checkNativeBrowserSettings(dataDir string) error {
	var view *WebView
	wndWindows.Range(func(_, value any) bool {
		if host, ok := value.(*hostWindow); ok && host.cfg.DataDir == dataDir && host.ready {
			view = host.view
		}
		return true
	})
	if view == nil || view.core == 0 {
		return errors.New("native browser settings fixture host was not found")
	}
	var settings uintptr
	if err := hresultError("Read installed WebView2 settings", comCall(view.core, 3, uintptr(unsafe.Pointer(&settings)))); err != nil {
		return err
	}
	if settings == 0 {
		return errors.New("installed WebView2 returned no settings")
	}
	defer comRelease(settings)
	check := func(object uintptr, slot int, name string, want int32) error {
		value := int32(-1)
		if err := hresultError("Read "+name, comCall(object, slot, uintptr(unsafe.Pointer(&value)))); err != nil {
			return err
		}
		if value != want {
			return fmt.Errorf("installed WebView2 %s = %d, want %d", name, value, want)
		}
		return nil
	}
	for _, property := range []struct {
		slot int
		name string
		want int32
	}{
		{3, "interface JavaScript", 1},
		{5, "host web messaging", 0},
		{7, "browser script dialogs", 0},
		{9, "browser status bar", 0},
		{11, "developer tools", 0},
		{13, "browser context menus", 0},
		{15, "script host objects", 0},
		{17, "browser zoom controls", 0},
		{19, "built-in error pages", 0},
	} {
		if err := check(settings, property.slot, property.name, property.want); err != nil {
			return err
		}
	}
	settings3, err := comQuery(settings, iidSettings3)
	if err != nil {
		return err
	}
	defer comRelease(settings3)
	if err := check(settings3, 23, "browser accelerator keys", 0); err != nil {
		return err
	}
	settings4, err := comQuery(settings, iidSettings4)
	if err != nil {
		return err
	}
	defer comRelease(settings4)
	if err := check(settings4, 25, "password autosave", 0); err != nil {
		return err
	}
	return check(settings4, 27, "general autofill", 0)
}
