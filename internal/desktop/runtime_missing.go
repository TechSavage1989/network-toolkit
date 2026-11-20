package desktop

import (
	"errors"
	"strings"
	"sync/atomic"
)

// WebViewRuntimeDownloadPage is Microsoft's page with the WebView2 Runtime
// installers. The window only opens it in the browser when the user chooses
// to: the app never downloads or installs the runtime itself.
const WebViewRuntimeDownloadPage = "https://developer.microsoft.com/microsoft-edge/webview2/"

// ErrWebViewRuntimeMissing reports that no usable registered WebView2 Runtime
// was found (Windows Server and some Windows 10 installations do not include
// one). The app reaches the native window only when no Edge or Chrome was
// found either (browser_window.go in cmd/network-toolkit).
var ErrWebViewRuntimeMissing = errors.New("the Microsoft Edge WebView2 Runtime is not installed")

// webViewRuntimeMissingTitle heads the window that explains it.
const webViewRuntimeMissingTitle = "The Microsoft Edge WebView2 Runtime is needed"

// webViewRuntimeButton is the label of the button that opens the page.
const webViewRuntimeButton = "Get the WebView2 Runtime"

type runtimeMissingError struct{ problems []string }

func (e runtimeMissingError) Error() string { return runtimeMissingMessage(e.problems) }

func (e runtimeMissingError) Is(target error) bool { return target == ErrWebViewRuntimeMissing }

// runtimeMissingMessage says in plain words what is missing and how to get it,
// followed by what was found wrong with a registration, if anything.
func runtimeMissingMessage(problems []string) string {
	message := "Network Toolkit opens in its own window through the Microsoft Edge WebView2 Runtime, or in a Microsoft Edge or Google Chrome window when that runtime is missing. " +
		"Neither was found on this computer. Windows 11 includes the runtime; Windows Server and some Windows 10 installations do not.\n\n" +
		"Choose " + webViewRuntimeButton + " to open Microsoft's download page in your browser. " +
		"Download the Evergreen Standalone Installer for x64, run it (it may ask for administrator approval), then open Network Toolkit again. " +
		"Installing Microsoft Edge or Google Chrome works too. Your settings and saved data stay as they are."
	if len(problems) != 0 {
		message += "\n\nDetails: " + strings.Join(problems, "; ")
	}
	return message
}

// browserHosted is set when the interface runs in an Edge or Chrome window
// instead of the native one. The engine's own dialogs (vault password, Save
// As) then have no window of this process to come in front of, and Windows
// usually refuses this process the foreground while the browser has it, so
// their hidden owner is made topmost: the dialog shows above the browser.
var browserHosted atomic.Bool

// SetBrowserHosted records that the interface runs in a browser window.
func SetBrowserHosted(hosted bool) { browserHosted.Store(hosted) }

// dialogOwnerStyle is the extended style of a hidden dialog owner window:
// WS_EX_TOOLWINDOW, plus WS_EX_TOPMOST in a browser-hosted interface.
func dialogOwnerStyle() uintptr {
	if browserHosted.Load() {
		return 0x00000080 | 0x00000008
	}
	return 0x00000080
}
