//go:build !windows

package desktop

// WebViewRuntimeAvailable is a Windows question; elsewhere there is none.
func WebViewRuntimeAvailable() bool { return false }
