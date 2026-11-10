package main

import (
	"encoding/json"
	"path/filepath"
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

// The one-time move unites SSH host trust. A host that only a standalone
// window trusted (the user may have removed it in the main window on purpose)
// is trusted by every app afterwards, so the user is told which hosts those
// are; the page says where to remove one.
func TestSharedMigrationNamesTheHostsItAddedToTrust(t *testing.T) {
	root := t.TempDir()
	both, onlyTerminal, onlyEditor := knownHostFixtureKey(t), knownHostFixtureKey(t), knownHostFixtureKey(t)
	writeFixture(t, filepath.Join(root, appSettingsFilename), `{"theme":"dark"}`)
	writeKnownHostFixture(t, root, knownHostFixtureEntry("both.test:22", both))
	writeKnownHostFixture(t, filepath.Join(root, terminalWorkspaceName), knownHostFixtureEntry("both.test:22", both), knownHostFixtureEntry("removed-in-main.test:22", onlyTerminal), knownHostFixtureEntry("lab.test:2222", onlyTerminal))
	writeKnownHostFixture(t, filepath.Join(root, standaloneEditorDirectory), knownHostFixtureEntry("files.test:22", onlyEditor))
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	added := map[string][]string{}
	for _, notice := range sharedNotices(root) {
		if notice.Kind == "hosts-added" {
			added[notice.Source] = append(added[notice.Source], notice.Items...)
		}
	}
	if !sameIDs(added["terminal"], "removed-in-main.test:22", "lab.test:2222") || !sameIDs(added["editor"], "files.test:22") {
		t.Fatalf("the hosts that are trusted by every app now are not named: %v", added)
	}
	app := sharedTestApp(t, root, "", "main")
	if got := knownHostsOf(t, app); !sameIDs(got, "both.test:22", "removed-in-main.test:22", "lab.test:2222", "files.test:22") {
		t.Fatalf("trusted hosts after the move: %v", got)
	}
	// A repeated start says nothing new, and a host the user then removes
	// stays removed.
	if r := requestApp(app, "DELETE", "/api/terminals/known-hosts", `{"host":"removed-in-main.test:22"}`, true); r.Code != 200 {
		t.Fatalf("remove: %d %s", r.Code, r.Body.String())
	}
	before := len(sharedNotices(root))
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	if got := knownHostsOf(t, app); len(sharedNotices(root)) != before || !sameIDs(got, "both.test:22", "lab.test:2222", "files.test:22") {
		t.Fatalf("after a second start: %d notices, trusted %v", len(sharedNotices(root)), got)
	}
}
