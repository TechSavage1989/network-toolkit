package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func sharedFTPProfile(id, name, host string) map[string]any {
	return map[string]any{"id": id, "name": name, "host": host, "port": float64(21), "username": "operator", "security": "explicit", "fingerprint": "", "credentialId": ""}
}

func sharedManyProfiles(prefix string, count int) []any {
	out := make([]any, 0, count)
	for n := 0; n < count; n++ {
		out = append(out, sharedProfile(fmt.Sprintf("%s%03d", prefix, n), fmt.Sprintf("%s %d", prefix, n), fmt.Sprintf("192.0.2.%d", n%250+1)))
	}
	return out
}

func sharedManyFTP(prefix string, count int) []any {
	out := make([]any, 0, count)
	for n := 0; n < count; n++ {
		out = append(out, sharedFTPProfile(fmt.Sprintf("%s%03d", prefix, n), fmt.Sprintf("%s %d", prefix, n), fmt.Sprintf("files%d.example.com", n)))
	}
	return out
}

// Saved lists are merged by entry, so a list that comes back shorter means
// "the user removed these". Nothing between the settings file and a page may
// therefore shorten one: not loading, not saving, not a sanitizer.
func TestSharedSavedListsAreNeverShortenedByLoadingOrSaving(t *testing.T) {
	for _, count := range []int{150, 230} {
		root := t.TempDir()
		raw, _ := json.Marshal(map[string]any{"terminalProfiles": sharedManyProfiles("ssh", count), "ftpProfiles": sharedManyFTP("ftp", count), "profiles": []any{map[string]any{"id": "t", "name": "Gateway", "target": "192.0.2.1"}}})
		writeFixture(t, filepath.Join(root, appSettingsFilename), string(raw))
		apps := []*application{sharedTestApp(t, root, "", "main"), sharedTestApp(t, root, terminalWorkspaceName, "terminal"), sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode)}
		for index, app := range apps {
			page := openSharedPage(t, app)
			if got := len(page.ids("terminalProfiles")); got != count {
				t.Fatalf("%s was given %d of %d saved sessions", app.windowMode, got, count)
			}
			if got := len(page.ids("ftpProfiles")); got != count {
				t.Fatalf("%s was given %d of %d saved FTP connections", app.windowMode, got, count)
			}
			// The user changes one saved session and one FTP connection far
			// down the list; the page saves all of its settings.
			page.row("terminalProfiles", fmt.Sprintf("ssh%03d", count-5-index))["username"] = "admin"
			page.row("ftpProfiles", fmt.Sprintf("ftp%03d", count-5-index))["username"] = "admin"
			page.save()
			if got := len(page.ids("terminalProfiles")); got != count {
				t.Fatalf("%s holds %d of %d saved sessions after its save", app.windowMode, got, count)
			}
			for _, name := range []string{"terminalProfiles", "ftpProfiles"} {
				if got := sharedDiskIDs(t, root, name); len(got) != count {
					t.Fatalf("after a save in %s the data folder has %d of %d %s", app.windowMode, len(got), count, name)
				}
			}
		}
		disk := sharedDisk(t, root)
		for _, name := range []string{"terminalProfiles", "ftpProfiles"} {
			changed := 0
			for _, entry := range disk[name].([]any) {
				if entry.(map[string]any)["username"] == "admin" {
					changed++
				}
			}
			if changed != len(apps) {
				t.Fatalf("%s: %d of %d edits were saved", name, changed, len(apps))
			}
		}
	}
}

// Two apps each add an entry to a list with room for one more. The second
// add does not fit: it is not saved, the page is told which entry that was,
// and nothing else is removed, now or by that page's later saves.
func TestSharedSaveRefusesEntriesPastTheLimitAndTellsThePage(t *testing.T) {
	limit := 0
	for _, key := range sharedDataKeys {
		if key.Name == "terminalProfiles" {
			limit = key.MaxSaved
		}
	}
	if limit < 100 {
		t.Fatalf("saved sessions have no limit in the key table: %d", limit)
	}
	root := t.TempDir()
	raw, _ := json.Marshal(map[string]any{"terminalProfiles": sharedManyProfiles("p", limit-1)})
	writeFixture(t, filepath.Join(root, appSettingsFilename), string(raw))
	main := openSharedPage(t, sharedTestApp(t, root, "", "main"))
	terminal := openSharedPage(t, sharedTestApp(t, root, terminalWorkspaceName, "terminal"))
	main.add("terminalProfiles", sharedProfile("from-main", "From main", "192.0.2.3"))
	main.save()
	terminal.add("terminalProfiles", sharedProfile("from-terminal", "From terminal", "192.0.2.4"))
	refused := terminal.saveRefused()
	if len(refused["terminalProfiles"]) != 1 || refused["terminalProfiles"][0] != "from-terminal" {
		t.Fatalf("the page was not told which entry did not fit: %v", refused)
	}
	ids := sharedDiskIDs(t, root, "terminalProfiles")
	if len(ids) != limit || !strings.Contains(strings.Join(ids, " "), "from-main") || strings.Contains(strings.Join(ids, " "), "from-terminal") {
		t.Fatalf("data folder after the second add: %d entries", len(ids))
	}
	// The page was given the saved list: it holds what is on disk.
	if got := terminal.ids("terminalProfiles"); len(got) != limit || terminal.row("terminalProfiles", "from-terminal") != nil || terminal.row("terminalProfiles", "from-main") == nil {
		t.Fatalf("the page still shows the entry that was not saved: %d entries", len(got))
	}
	// Its later saves remove nothing.
	terminal.state["theme"] = "light"
	terminal.save()
	if got := sharedDiskIDs(t, root, "terminalProfiles"); len(got) != limit {
		t.Fatalf("a later save of that page left %d of %d entries", len(got), limit)
	}
	// A list that is over its limit already (entries put back after an older
	// version ran) can be edited and shortened; only growing it is refused.
	over := t.TempDir()
	raw, _ = json.Marshal(map[string]any{"terminalProfiles": sharedManyProfiles("p", limit+20)})
	writeFixture(t, filepath.Join(over, appSettingsFilename), string(raw))
	page := openSharedPage(t, sharedTestApp(t, over, "", "main"))
	page.row("terminalProfiles", "p003")["username"] = "admin"
	page.remove("terminalProfiles", "p004")
	page.save()
	if got := sharedDiskIDs(t, over, "terminalProfiles"); len(got) != limit+19 {
		t.Fatalf("editing a list that is over its limit left %d of %d entries", len(got), limit+19)
	}
	page.add("terminalProfiles", sharedProfile("one-more", "One more", "192.0.2.9"))
	if refused := page.saveRefused(); len(refused["terminalProfiles"]) != 1 {
		t.Fatalf("an entry was added to a list that is over its limit: %v", refused)
	}
	if got := sharedDiskIDs(t, over, "terminalProfiles"); len(got) != limit+19 {
		t.Fatalf("after the refused add: %d", len(got))
	}
}

// The limits of the one-time move are those of the key table.
func TestSharedMigrationStopsAtTheLimitOfTheKeyTable(t *testing.T) {
	for _, key := range sharedDataKeys {
		if key.Kind != sharedList {
			continue
		}
		if (key.MaxSaved > 0) == (key.Limit > 0) {
			t.Errorf("%s must be either a saved list (MaxSaved) or a rolling list (Limit)", key.Name)
		}
	}
	root := t.TempDir()
	rootRaw, _ := json.Marshal(map[string]any{"terminalProfiles": sharedManyProfiles("main", 80), "ftpProfiles": sharedManyFTP("mainftp", 80)})
	workspaceRaw, _ := json.Marshal(map[string]any{"terminalProfiles": sharedManyProfiles("term", 100), "ftpProfiles": sharedManyFTP("termftp", 100)})
	writeFixture(t, filepath.Join(root, appSettingsFilename), string(rootRaw))
	writeFixture(t, filepath.Join(root, terminalWorkspaceName, appSettingsFilename), string(workspaceRaw))
	if err := migrateSharedData(root); err != nil {
		t.Fatal(err)
	}
	// 80 + 100 of each fit: every saved session and FTP connection of both
	// apps is there.
	for _, name := range []string{"terminalProfiles", "ftpProfiles"} {
		if got := sharedDiskIDs(t, root, name); len(got) != 180 {
			t.Fatalf("%s after the move: %d of 180", name, len(got))
		}
	}
	if notices := sharedNotices(root); len(notices) != 0 {
		t.Fatalf("notices although everything fit: %+v", notices)
	}
}
