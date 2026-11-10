package main

import (
	"encoding/json"
	"testing"
)

func TestSharedMigrationNoticesReachEveryAppUntilAcknowledged(t *testing.T) {
	root := migrationFixture(t)
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	main := sharedTestApp(t, root, "", "main")
	terminal := sharedTestApp(t, root, terminalWorkspaceName, "terminal")
	notices := func(a *application) []sharedNotice {
		var polled struct {
			Notices []sharedNotice `json:"notices"`
		}
		r := requestApp(a, "GET", "/api/shared-data", "", true)
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &polled) != nil {
			t.Fatalf("poll: %d %s", r.Code, r.Body.String())
		}
		return polled.Notices
	}
	for _, app := range []*application{main, terminal} {
		if got := notices(app); len(got) != 1 || got[0].Kind != "host-key-conflict" || got[0].Items[0] != "192.0.2.1:22" {
			t.Fatalf("%s is not told about the host with two different fingerprints: %+v", app.windowMode, got)
		}
	}
	id := notices(main)[0].ID
	for _, body := range []string{`{}`, `{"acknowledge":[]}`, `[]`} {
		if r := requestApp(terminal, "POST", "/api/shared-data/notices", body, true); r.Code != 400 {
			t.Fatalf("an empty acknowledgement was accepted: %d %s", r.Code, body)
		}
	}
	if r := requestApp(terminal, "POST", "/api/shared-data/notices", `{"acknowledge":["`+id+`"]}`, false); r.Code != 401 {
		t.Fatalf("a notice was acknowledged without the session token: %d", r.Code)
	}
	if r := requestApp(terminal, "GET", "/api/shared-data/notices", "", true); r.Code != 405 {
		t.Fatalf("notices: %d", r.Code)
	}
	if r := requestApp(terminal, "POST", "/api/shared-data/notices", `{"acknowledge":["`+id+`"]}`, true); r.Code != 200 {
		t.Fatalf("acknowledge: %d %s", r.Code, r.Body.String())
	}
	// Read in one app, it is not shown again in any.
	for _, app := range []*application{main, terminal} {
		if got := notices(app); len(got) != 0 {
			t.Fatalf("%s still shows an acknowledged notice: %+v", app.windowMode, got)
		}
	}
}
