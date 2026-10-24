package desktop

import "testing"

func TestNavigationAndDownloadOrigins(t *testing.T) {
	app := "http://127.0.0.1:45678/"
	for _, candidate := range []string{app, app + "?view=reports", app + "#results", app + "api/status"} {
		if !sameAppOrigin(app, candidate, false) {
			t.Errorf("own app URL was rejected: %s", candidate)
		}
	}
	for _, candidate := range []string{
		"http://127.0.0.1:45679/", "https://127.0.0.1:45678/", "http://localhost:45678/",
		"http://127.0.0.1:45678.evil.test/", "http://127.0.0.1:45678@evil.test/", "http://user@127.0.0.1:45678/",
		"http://evil.test/", "file:///C:/data/report.html", "data:text/html,<h1>report</h1>", "about:blank", "javascript:alert(1)",
		"edge://settings/", "chrome://downloads/", "blob:http://127.0.0.1:45678/report",
	} {
		if sameAppOrigin(app, candidate, false) {
			t.Errorf("foreign/non-HTTP navigation allowed: %s", candidate)
		}
	}
	if !sameAppOrigin(app, "blob:http://127.0.0.1:45678/report", true) {
		t.Fatal("own-origin report download rejected")
	}
	for _, candidate := range []string{"blob:http://evil.test/report", "blob:null/report", "blob:http://127.0.0.1:45679/report", "blob:blob:http://127.0.0.1:45678/report"} {
		if sameAppOrigin(app, candidate, true) {
			t.Errorf("foreign/malformed blob download allowed: %s", candidate)
		}
	}
}
