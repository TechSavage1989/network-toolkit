package desktop

import (
	"strings"
	"testing"
)

func TestNativeBootstrapAcceptsOnlyCanonicalDocumentsAndCapabilities(t *testing.T) {
	token := strings.Repeat("ab", 32)
	for _, address := range []string{"http://127.0.0.1:1234/", "http://127.0.0.1:1234/terminal-view/" + strings.Repeat("ab", 16)} {
		script, err := nativeSessionBootstrap(address, token)
		if err != nil || !strings.Contains(script, token) || strings.Contains(script, "__TOKEN__") || !strings.Contains(script, "window.top!==window") {
			t.Fatalf("native bootstrap was not generated: %v", err)
		}
		if !sameNativeDocument(address, address) || !sameNativeDocument(address, address+"#terminal") {
			t.Fatal("same-document navigation was rejected")
		}
		for _, target := range []string{address + "?x=1", "http://127.0.0.1:1234/api/state", "http://127.0.0.1:1235/", "http://localhost:1234/", "http://user@127.0.0.1:1234/", "about:blank", "blob:http://127.0.0.1:1234/id", "file:///C:/test.html"} {
			if sameNativeDocument(address, target) {
				t.Fatalf("different capability document was accepted: %q", target)
			}
		}
	}
	for _, address := range []string{"http://127.0.0.1:1234", "http://127.0.0.1:01234/", "http://127.0.0.1:1234/?", "http://127.0.0.1:1234/?x=1", "http://127.0.0.1:1234/#x", "http://127.0.0.1:1234/api/state", "http://127.0.0.1:1234/%2f", "http://127.0.0.1:1234/terminal-view/invalid", "https://127.0.0.1:1234/", "http://[::1]:1234/", "http://127.0.0.1:70000/"} {
		if _, err := nativeSessionBootstrap(address, token); err == nil {
			t.Fatalf("noncanonical document accepted: %q", address)
		}
	}
	for _, invalid := range []string{"", "short", strings.Repeat("AB", 32), strings.Repeat("g", 64), strings.Repeat("a", 65), "';alert(1)//"} {
		if _, err := nativeSessionBootstrap("http://127.0.0.1:1234/", invalid); err == nil {
			t.Fatal("invalid capability accepted")
		}
	}
}
