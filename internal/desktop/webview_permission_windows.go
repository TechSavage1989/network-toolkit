//go:build windows

package desktop

import "unsafe"

// Checked against Microsoft.Web.WebView2 1.0.4258.31 WebView2.h. The shipped
// Args3 interface inherits Args2 and Args; the early design proposal's UUID and
// standalone-IUnknown layout are different and must not be used here.
var iidPermissionRequestedArgs3 = guid("e61670bc-3dce-4177-86d2-c629ae3cb6ac")

type nativePermissionRequest struct{ args uintptr }

func (r nativePermissionRequest) uri() (string, error) {
	// comGetString checks the HRESULT and frees the returned CoTaskMem string.
	return comGetString(r.args, 3)
}

func (r nativePermissionRequest) kind() (int32, error) {
	var value int32
	err := hresultError("Read requested permission kind", comCall(r.args, 4, uintptr(unsafe.Pointer(&value))))
	return value, err
}

func (r nativePermissionRequest) userInitiated() (bool, error) {
	var value int32
	err := hresultError("Read permission user gesture", comCall(r.args, 5, uintptr(unsafe.Pointer(&value))))
	return value != 0, err
}

func (r nativePermissionRequest) disablePersistence() error {
	extended, err := comQuery(r.args, iidPermissionRequestedArgs3)
	if err != nil {
		return err
	}
	defer comRelease(extended)
	// IUnknown 0..2; Args 3..8; Args2 9..10; Args3 get/put 11..12.
	return hresultError("Disable permission persistence", comCall(extended, 12, 0))
}

func (r nativePermissionRequest) setState(value int32) error {
	return hresultError("Set requested permission state", comCall(r.args, 7, uintptr(value)))
}

func (w *WebView) permissionRequested(_ uintptr, args uintptr) uintptr {
	if err := applyWebViewPermission(w.cfg.AppURL, func() bool { return w.closed }, nativePermissionRequest{args}); err != nil {
		return hresultFail
	}
	return hresultOK
}
