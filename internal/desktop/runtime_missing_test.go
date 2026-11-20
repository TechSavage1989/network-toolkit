package desktop

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

// Windows Server 2022 has no WebView2 Runtime (owner report on v1.21.72): the
// window explains it plainly and offers Microsoft's page, never a download.
func TestWebViewRuntimeMissingIsExplained(t *testing.T) {
	err := error(runtimeMissingError{})
	if !errors.Is(err, ErrWebViewRuntimeMissing) || !errors.Is(fmt.Errorf("start: %w", err), ErrWebViewRuntimeMissing) {
		t.Fatal("a missing runtime must be recognized through wrapping")
	}
	text := err.Error()
	for _, want := range []string{"WebView2 Runtime", "Microsoft Edge or Google Chrome", "Neither was found", "Windows Server", webViewRuntimeButton, "x64", "open Network Toolkit again", "settings and saved data stay"} {
		if !strings.Contains(text, want) {
			t.Errorf("message lacks %q: %s", want, text)
		}
	}
	if strings.Contains(text, "Details") {
		t.Errorf("no details without problems: %s", text)
	}
	detailed := runtimeMissingError{problems: []string{"machine runtime: missing DLL"}}.Error()
	if !strings.HasSuffix(detailed, "Details: machine runtime: missing DLL") {
		t.Errorf("details missing: %s", detailed)
	}
	page, parseErr := url.Parse(WebViewRuntimeDownloadPage)
	if parseErr != nil || page.Scheme != "https" || page.Host != "developer.microsoft.com" || !strings.Contains(page.Path, "webview2") {
		t.Fatalf("the page must be Microsoft's WebView2 page over HTTPS: %q", WebViewRuntimeDownloadPage)
	}
	if errors.Is(errors.New("other"), ErrWebViewRuntimeMissing) {
		t.Fatal("other errors are not a missing runtime")
	}
}

func TestBrowserHostedDialogsShowAboveTheBrowser(t *testing.T) {
	t.Cleanup(func() { SetBrowserHosted(false) })
	if dialogOwnerStyle() != 0x80 {
		t.Fatalf("the native window's dialogs keep their owner style: %#x", dialogOwnerStyle())
	}
	SetBrowserHosted(true)
	if dialogOwnerStyle() != 0x88 {
		t.Fatalf("in a browser window the dialog owner is topmost: %#x", dialogOwnerStyle())
	}
}
