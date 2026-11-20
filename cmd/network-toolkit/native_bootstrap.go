package main

import "runtime"

// Native Windows windows receive their capability through an in-memory host
// bootstrap, never from a publicly readable localhost document. Non-Windows
// development HTML retains its isolated test/preview bootstrap; the credential
// vault is unavailable on that platform.
// nativeHostBootstrap is false when a Windows interface opens in a browser
// window: that page is gated by the one-time launch link instead.
var nativeHostBootstrap = runtime.GOOS == "windows"

func httpBootstrapToken(token string) string {
	return publicBootstrapToken(token, nativeHostBootstrap)
}

func publicBootstrapToken(token string, native bool) string {
	if native {
		return "__SESSION_TOKEN__"
	}
	return token
}
