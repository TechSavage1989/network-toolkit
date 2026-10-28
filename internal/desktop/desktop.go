// Package desktop hosts the local application in a native Windows WebView2 window.
package desktop

import "context"

// Config describes one app window. DataDir is the portable application's folder.
// The embedded UI uses the Microsoft WebView2 Runtime installed on Windows.
type Config struct {
	Title   string
	URL     string
	DataDir string
	Stop    context.CancelFunc
	// SessionToken is an in-memory capability, delivered only to this native
	// WebView. Never put it in the URL, process arguments, profile or HTTP HTML.
	SessionToken string
	// RequireCloseApproval never treats a ready renderer invocation or timeout
	// as approval to discard editor work. Only context cancellation closes it.
	// The default terminal close behavior remains unchanged.
	RequireCloseApproval bool
	// Activate requests that an existing window be restored and brought forward.
	// Sends may be coalesced; closing the channel disables further requests.
	Activate <-chan struct{}
	// OnClosing is called once on the UI thread when the window starts its
	// close handshake or immediate shutdown. Approval-required hosts defer it
	// until approved shutdown. It must return promptly and cannot shorten
	// the final save wait.
	OnClosing func()
	// OnReady is called once on the UI thread after interface startup succeeds
	// and the native main window is verified visible. An error screen does not
	// qualify. The callback must return promptly.
	OnReady func()
	// Package-private native acceptance instrumentation. It observes decisions
	// but cannot alter the document policy or supply a capability.
	navigationObserver func(nativeNavigationObservation)
}

type nativeNavigationObservation struct {
	Kind                  string
	URI                   string
	Blocked               bool
	CancelResult          uint32
	CancelReadResult      uint32
	Cancelled             bool
	InitialDocumentPermit bool
	ResponseResult        uint32
	RuntimeVersion        string
}

// Rect has the layout of a Win32 RECT. Coordinates are physical screen pixels
// for window state and client pixels for the WebView controller.
type Rect struct {
	Left   int32 `json:"left"`
	Top    int32 `json:"top"`
	Right  int32 `json:"right"`
	Bottom int32 `json:"bottom"`
}

// WebViewConfig connects the COM browser with the native window. All callbacks
// except Post execute on the window's STA thread. Post may be called from any
// goroutine and returns false once the window starts shutting down.
type WebViewConfig struct {
	AppURL             string
	SessionToken       string
	UserDataPath       string
	SaveDownload       func(suggestedName string) (path string, ok bool)
	Post               func(func()) bool
	OnError            func(error)
	navigationObserver func(nativeNavigationObservation)
}
