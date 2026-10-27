package main

import "runtime"

// Native Windows windows receive their capability through an in-memory host
// bootstrap, never from a publicly readable localhost document. Non-Windows
// development HTML retains its isolated test/preview bootstrap; the credential
// vault is unavailable on that platform.
func httpBootstrapToken(token string) string {
	return publicBootstrapToken(token, runtime.GOOS == "windows")
}

func publicBootstrapToken(token string, native bool) string {
	if native {
		return "__SESSION_TOKEN__"
	}
	return token
}
