package desktop

import (
	"strings"
	"testing"
)

func TestNativeBootstrapInitialDocumentPermit(t *testing.T) {
	for _, document := range []string{"/", "/terminal-view/" + strings.Repeat("ab", 16), "/editor-view/" + strings.Repeat("cd", 16)} {
		app := "http://127.0.0.1:1234" + document
		var missing nativeDocumentRequestPermit
		if missing.consume(app, app) || missing.allowNavigation(app, app) || missing.pending || missing.navigationPending {
			t.Fatal("renderer navigation armed a missing native permit")
		}
		for _, resourceFirst := range []bool{false, true} {
			var permit nativeDocumentRequestPermit
			if !permit.begin() || permit.begin() {
				t.Fatal("initial permit could not be armed exactly once")
			}
			if resourceFirst {
				if !permit.consume(app, app) || !permit.allowNavigation(app, app) {
					t.Fatal("initial exact document was rejected in resource-first order")
				}
			} else if !permit.allowNavigation(app, app) || !permit.consume(app, app) {
				t.Fatal("initial exact document was rejected in navigation-first order")
			}
			if permit.consume(app, app) || permit.allowNavigation(app, app) || permit.begin() {
				t.Fatal("iframe, reload or repeated native start reused the startup permit")
			}
		}
		for _, target := range []string{"", "http://127.0.0.1:1234/forbidden", app + "?x=1", app + "#fragment", "http://localhost:1234" + document, "https://example.invalid/"} {
			var permit nativeDocumentRequestPermit
			permit.begin()
			if permit.consume(app, target) || permit.consume(app, app) {
				t.Fatalf("first wrong document %q failed to consume the resource permit", target)
			}
			if permit.allowNavigation(app, target) || permit.allowNavigation(app, app) || permit.begin() {
				t.Fatalf("wrong or ordinary navigation %q rearmed the initial permit", target)
			}
		}
	}
}

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

func TestEditorNativeDocumentRemainsExact(t *testing.T) {
	good := "http://127.0.0.1:1234/editor-view/" + strings.Repeat("cd", 16)
	if _, err := nativeDocumentURL(good); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{good + "?x=1", good + "/extra", strings.Replace(good, "cd", "CD", 1), strings.Replace(good, "/editor-view/", "/editor-view/%2f", 1), strings.Replace(good, "127.0.0.1", "localhost", 1)} {
		if _, err := nativeDocumentURL(bad); err == nil {
			t.Fatal("noncanonical editor document allowed", bad)
		}
	}
	if sameNativeDocument(good, strings.Replace(good, "editor-view", "terminal-view", 1)) {
		t.Fatal("editor capability applies to terminal document")
	}
}
