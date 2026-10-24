package desktop

import (
	"errors"
	"net/url"
	"strings"
)

const (
	webviewPermissionClipboardRead int32 = 6
	webviewPermissionAllow         int32 = 1
	webviewPermissionDeny          int32 = 2
)

// The adapter keeps the permission decision and failure paths testable without
// a native WebView. The Windows implementation owns all COM resources.
type webviewPermissionRequest interface {
	uri() (string, error)
	kind() (int32, error)
	userInitiated() (bool, error)
	disablePersistence() error
	setState(int32) error
}

// Clipboard access is granted for one explicit interaction with this app only.
// Establish denial first; metadata, runtime support, or COM failures must never
// leave the default browser permission prompt or an earlier allow decision.
func applyWebViewPermission(appURL string, closed func() bool, request webviewPermissionRequest) error {
	if err := request.setState(webviewPermissionDeny); err != nil {
		return err
	}
	if closed() {
		return nil
	}
	uri, err := request.uri()
	if err != nil {
		return nil
	}
	kind, err := request.kind()
	if err != nil || kind != webviewPermissionClipboardRead || !sameAppOrigin(appURL, uri, false) {
		return nil
	}
	userInitiated, err := request.userInitiated()
	if err != nil {
		return nil
	}
	// Also keep a denied background read transient, so it cannot cache a denial
	// that prevents a later explicit Paste gesture from reaching this policy.
	if err = request.disablePersistence(); err != nil {
		return nil
	}
	if !userInitiated || closed() {
		return nil
	}
	if err = request.setState(webviewPermissionAllow); err != nil {
		// A failed allow setter is not a usable permission decision. Reassert the
		// already-established deny state and report either native failure.
		return errors.Join(err, request.setState(webviewPermissionDeny))
	}
	return nil
}

// Blob URLs retain their creator's origin. The interface uses same-origin blob
// URLs for user-requested report downloads; foreign blobs remain disallowed.
func sameAppOrigin(appURL, candidate string, allowBlob bool) bool {
	app, err := url.Parse(appURL)
	if err != nil || app.Scheme != "http" || app.Hostname() != "127.0.0.1" || app.Port() == "" || app.User != nil {
		return false
	}
	if allowBlob && strings.HasPrefix(candidate, "blob:") {
		candidate = strings.TrimPrefix(candidate, "blob:")
	}
	target, err := url.Parse(candidate)
	if err != nil || target.User != nil {
		return false
	}
	return target.Scheme == app.Scheme && target.Host == app.Host
}
