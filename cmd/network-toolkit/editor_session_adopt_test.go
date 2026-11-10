package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Two application objects on one data folder stand in for the main window's
// editor and the standalone Text Editor.
func editorSessionApps(t *testing.T) (root string, main, editor *application) {
	t.Helper()
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("requires the platform file lock")
	}
	root = t.TempDir()
	return root, sharedTestApp(t, root, "", "main"), sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode)
}

// closeEditorApp ends an app the way a closed process does: its lock on its
// editor session is released.
func closeEditorApp(t *testing.T, a *application) {
	t.Helper()
	a.stop()
	deadline := time.Now().Add(5 * time.Second)
	for {
		a.editorSession.leaseMu.Lock()
		released := a.editorSession.lease == nil
		a.editorSession.leaseMu.Unlock()
		if released {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("a closed app kept its editor session lock")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func sessionLocalPath(name string) string {
	if runtime.GOOS == "windows" {
		return `C:\fixtures\` + name
	}
	return "/srv/fixtures/" + name
}

func untitledTab(name string, backup editorSessionBackup) map[string]any {
	return map[string]any{"kind": "untitled", "name": name, "dirty": true, "format": sessionFormat(), "backup": backup}
}

func dirtyLocalTab(path string, backup editorSessionBackup) map[string]any {
	return map[string]any{"kind": "local", "path": path, "dirty": true, "hash": "", "format": sessionFormat(), "savedFormat": sessionFormat(), "backup": backup}
}

func cleanLocalTab(path string) map[string]any {
	return map[string]any{"kind": "local", "path": path, "dirty": false}
}

func mustPutSession(t *testing.T, a *application, active int, tabs ...map[string]any) {
	t.Helper()
	if r := putSession(t, a, map[string]any{"version": 1, "active": active, "tabs": tabs}); r.Code != http.StatusOK {
		t.Fatalf("save session: %d %s", r.Code, r.Body.String())
	}
}

func sessionFiles(t *testing.T, a *application) []string {
	t.Helper()
	entries, err := os.ReadDir(sessionDir(a))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func diskSession(t *testing.T, a *application) editorSessionIndex {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(sessionDir(a), editorSessionIndexName))
	if os.IsNotExist(err) {
		return emptyEditorSession()
	}
	var index editorSessionIndex
	if err != nil || json.Unmarshal(raw, &index) != nil {
		t.Fatalf("session index: %v", err)
	}
	return index
}

func keptText(t *testing.T, a *application, id string) string {
	t.Helper()
	r := requestApp(a, http.MethodGet, editorSessionEndpoint+"/backup?id="+id, "", true)
	if r.Code != http.StatusOK {
		t.Fatalf("kept copy %s: %d %s", id, r.Code, r.Body.String())
	}
	var out struct {
		Text string `json:"text"`
	}
	_ = json.Unmarshal(r.Body.Bytes(), &out)
	return out.Text
}

func tabSummary(index editorSessionIndex) string {
	parts := []string{}
	for _, tab := range index.Tabs {
		label := tab.Path
		if label == "" {
			label = tab.Name
		}
		part := tab.Kind + ":" + filepath.Base(strings.ReplaceAll(label, `\`, "/"))
		if tab.Backup != nil {
			part += "*"
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, " ")
}

func TestEditorSessionAdoptsTheOtherEditorsTabsExactlyOnce(t *testing.T) {
	_, main, editor := editorSessionApps(t)
	shared := sessionLocalPath("shared.txt")
	// The standalone Text Editor kept three tabs: a draft, a saved file and a
	// file with unsaved changes.
	draft, changed := keepText(t, editor, "draft kept by the Text Editor"), keepText(t, editor, "unsaved text for shared.txt from the Text Editor")
	mustPutSession(t, editor, 1, untitledTab("Notes.txt", draft), cleanLocalTab(sessionLocalPath("saved.txt")), dirtyLocalTab(shared, changed))
	// The main window's editor has the same file open with its own unsaved text.
	own := keepText(t, main, "unsaved text for shared.txt from the main window")
	mustPutSession(t, main, 0, dirtyLocalTab(shared, own))

	// Both apps are running: each keeps its own session.
	index, warning := readSession(t, main)
	if tabSummary(index) != "local:shared.txt*" || warning != "" {
		t.Fatalf("tabs were taken from a running Text Editor: %s (%q)", tabSummary(index), warning)
	}
	if got := tabSummary(diskSession(t, editor)); got != "untitled:Notes.txt* local:saved.txt local:shared.txt*" {
		t.Fatalf("the running Text Editor's session changed: %s", got)
	}

	// The Text Editor is closed. The main window's editor starts (a new
	// process) and takes over what the Text Editor kept.
	closeEditorApp(t, editor)
	closeEditorApp(t, main)
	main = sharedTestApp(t, filepath.Dir(main.dataPath), "", "main")
	index, warning = readSession(t, main)
	if got := tabSummary(index); got != "local:shared.txt* untitled:Notes.txt* local:saved.txt copy:shared.txt*" {
		t.Fatalf("tabs after taking over the Text Editor's session: %s", got)
	}
	if !strings.Contains(warning, "Also reopened 3 files that were open in the Text Editor window.") {
		t.Fatalf("the user is not told where the extra tabs came from: %q", warning)
	}
	if index.Active != 0 {
		t.Fatalf("the main window's own active tab changed: %d", index.Active)
	}
	// Every kept copy arrived with its text; none was dropped or replaced.
	if keptText(t, main, index.Tabs[0].Backup.ID) != "unsaved text for shared.txt from the main window" || keptText(t, main, index.Tabs[1].Backup.ID) != "draft kept by the Text Editor" || keptText(t, main, index.Tabs[3].Backup.ID) != "unsaved text for shared.txt from the Text Editor" {
		t.Fatal("a kept copy does not hold its text after the move")
	}
	if copyTab := index.Tabs[3]; copyTab.Origin != shared || copyTab.Path != "" || copyTab.Backup.SHA256 != changed.SHA256 {
		t.Fatalf("the second version of the same file: %+v", copyTab)
	}
	// Moved, not copied: the Text Editor's session is empty now.
	if remaining := diskSession(t, editor); len(remaining.Tabs) != 0 {
		t.Fatalf("tabs stayed in the Text Editor's session: %s", tabSummary(remaining))
	}
	for _, name := range sessionFiles(t, editor) {
		if editorSessionBackupName.MatchString(name) {
			t.Fatalf("a kept copy stayed behind in the Text Editor's session: %s", name)
		}
	}
	// Exactly once: reading again adds nothing.
	again, warning := readSession(t, main)
	if tabSummary(again) != tabSummary(index) || warning != "" {
		t.Fatalf("a second read changed the session: %s (%q)", tabSummary(again), warning)
	}
	// The Text Editor starts again while the main window runs: it finds its
	// session empty and takes nothing from the running main window.
	editor = sharedTestApp(t, filepath.Dir(main.dataPath), standaloneEditorDirectory, standaloneEditorMode)
	if restarted, _ := readSession(t, editor); len(restarted.Tabs) != 0 {
		t.Fatalf("the restarted Text Editor has tabs: %s", tabSummary(restarted))
	}
	if got := tabSummary(diskSession(t, main)); got != tabSummary(index) {
		t.Fatalf("the running main window lost tabs: %s", got)
	}
	// The other direction: the main window closes, the Text Editor restarts
	// and takes over everything, including the tabs that came from it.
	closeEditorApp(t, main)
	closeEditorApp(t, editor)
	editor = sharedTestApp(t, filepath.Dir(main.dataPath), standaloneEditorDirectory, standaloneEditorMode)
	back, warning := readSession(t, editor)
	if got := tabSummary(back); got != "local:shared.txt* untitled:Notes.txt* local:saved.txt copy:shared.txt*" || !strings.Contains(warning, "that were open in the main Network Toolkit window") {
		t.Fatalf("the Text Editor after taking over the main window's session: %s (%q)", got, warning)
	}
	if back.Active != 0 {
		t.Fatalf("the active tab did not come along into an empty session: %d", back.Active)
	}
	if keptText(t, editor, back.Tabs[3].Backup.ID) != "unsaved text for shared.txt from the Text Editor" {
		t.Fatal("a kept copy lost its text on the way back")
	}
}

func TestEditorSessionAdoptionRecoversFromAnInterruptedMove(t *testing.T) {
	for _, next := range []string{"main", "editor"} {
		for _, stage := range []string{"copies made", "index written"} {
			t.Run(stage+", then "+next+" starts", func(t *testing.T) {
				root, main, editor := editorSessionApps(t)
				draft := keepText(t, editor, "draft kept by the Text Editor")
				mustPutSession(t, editor, 0, untitledTab("Notes.txt", draft), cleanLocalTab(sessionLocalPath("saved.txt")))
				own := keepText(t, main, "the main window's own draft")
				mustPutSession(t, main, 0, untitledTab("Own.txt", own))
				// The main window was taking over the Text Editor's session and
				// stopped part-way: the kept copy exists in both folders, and at
				// the later stage its own index already lists the tabs. The
				// Text Editor's index and copies are still as they were.
				name := editorSessionBackupFile(draft.ID)
				raw, err := os.ReadFile(filepath.Join(sessionDir(editor), name))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(sessionDir(main), name), raw, 0600); err != nil {
					t.Fatal(err)
				}
				if stage == "index written" {
					mustPutSession(t, main, 0, untitledTab("Own.txt", own), untitledTab("Notes.txt", draft), cleanLocalTab(sessionLocalPath("saved.txt")))
					// A save removes copies its index does not name; this one is named.
				}
				closeEditorApp(t, main)
				closeEditorApp(t, editor)
				var started, other *application
				if next == "main" {
					started = sharedTestApp(t, root, "", "main")
					other = editor
				} else {
					started = sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode)
					other = main
				}
				index, _ := readSession(t, started)
				seen := map[string]int{}
				for _, tab := range index.Tabs {
					label := tab.Name + tab.Path
					seen[label]++
					if tab.Backup != nil && keptText(t, started, tab.Backup.ID) == "" {
						t.Fatalf("the kept copy of %s is gone", label)
					}
				}
				if len(index.Tabs) != 3 || seen["Notes.txt"] != 1 || seen["Own.txt"] != 1 || seen[sessionLocalPath("saved.txt")] != 1 {
					t.Fatalf("tabs after completing an interrupted move: %s", tabSummary(index))
				}
				if remaining := diskSession(t, other); len(remaining.Tabs) != 0 {
					t.Fatalf("the other session still lists tabs: %s", tabSummary(remaining))
				}
				// A copy the interrupted run made but never listed may still lie in
				// the other folder. No index names it; that app removes it with
				// its next save, like any copy it no longer needs.
				reopened := sharedTestApp(t, root, strings.TrimPrefix(strings.TrimPrefix(filepath.Dir(other.dataPath), root), string(filepath.Separator)), other.windowMode)
				if again, _ := readSession(t, reopened); len(again.Tabs) != 0 {
					t.Fatalf("the other app took tabs back from a running app: %s", tabSummary(again))
				}
				mustPutSession(t, reopened, -1)
				for _, file := range sessionFiles(t, reopened) {
					if editorSessionBackupName.MatchString(file) {
						t.Fatalf("a kept copy stayed in the other session: %s", file)
					}
				}
				if final := diskSession(t, started); len(final.Tabs) != 3 {
					t.Fatalf("the session that took the tabs over changed: %s", tabSummary(final))
				}
			})
		}
	}
}

func TestEditorSessionAdoptionLeavesWhatItCannotMoveSafely(t *testing.T) {
	root, main, editor := editorSessionApps(t)
	// The data folder is on a volume without hard links: copies are made and
	// checked against the recorded size and digest.
	previous := editorSessionLink
	editorSessionLink = func(string, string) error { return errors.New("links are not supported here") }
	defer func() { editorSessionLink = previous }()
	good, damaged := keepText(t, editor, "an intact kept copy"), keepText(t, editor, "this copy will be damaged")
	mustPutSession(t, editor, -1, untitledTab("Good.txt", good), untitledTab("Damaged.txt", damaged))
	path := filepath.Join(sessionDir(editor), editorSessionBackupFile(damaged.ID))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 0xff // same size, different content
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	closeEditorApp(t, editor)
	index, warning := readSession(t, main)
	if tabSummary(index) != "untitled:Good.txt*" || !strings.Contains(warning, "Also reopened 1 file that was open in the Text Editor window.") || keptText(t, main, index.Tabs[0].Backup.ID) != "an intact kept copy" {
		t.Fatalf("after a move with one damaged copy: %s (%q)", tabSummary(index), warning)
	}
	// The damaged copy and its record stay with the Text Editor, untouched.
	remaining := diskSession(t, editor)
	if tabSummary(remaining) != "untitled:Damaged.txt*" {
		t.Fatalf("the record of a copy that could not be moved: %s", tabSummary(remaining))
	}
	if now, err := os.ReadFile(path); err != nil || string(now) != string(raw) {
		t.Fatalf("the copy that could not be moved was changed or removed: %v", err)
	}
	for _, name := range sessionFiles(t, main) {
		if strings.Contains(name, damaged.ID) || strings.HasSuffix(name, ".tmp") {
			t.Fatalf("a damaged or temporary copy was left in the main window's session: %s", name)
		}
	}

	// A full session takes only what fits; the rest stays where it is.
	second := t.TempDir()
	main2 := sharedTestApp(t, second, "", "main")
	editor2 := sharedTestApp(t, second, standaloneEditorDirectory, standaloneEditorMode)
	var many []map[string]any
	for n := 0; n < editorSessionTabLimit-1; n++ {
		many = append(many, cleanLocalTab(sessionLocalPath(fmt.Sprintf("main-%02d.txt", n))))
	}
	mustPutSession(t, main2, 0, many...)
	extra := keepText(t, editor2, "kept although the main window is full")
	mustPutSession(t, editor2, 0, cleanLocalTab(sessionLocalPath("fits.txt")), untitledTab("Later.txt", extra), cleanLocalTab(sessionLocalPath("later.txt")))
	closeEditorApp(t, editor2)
	closeEditorApp(t, main2)
	main2 = sharedTestApp(t, second, "", "main")
	full, _ := readSession(t, main2)
	if len(full.Tabs) != editorSessionTabLimit || full.Tabs[editorSessionTabLimit-1].Path != sessionLocalPath("fits.txt") {
		t.Fatalf("a full session after taking over: %d tabs", len(full.Tabs))
	}
	if got := tabSummary(diskSession(t, editor2)); got != "untitled:Later.txt* local:later.txt" {
		t.Fatalf("tabs that did not fit: %s", got)
	}
	if _, err := os.Stat(filepath.Join(sessionDir(editor2), editorSessionBackupFile(extra.ID))); err != nil {
		t.Fatalf("the kept copy of a tab that did not fit was removed: %v", err)
	}

	// An unreadable session of the other app is left for that app.
	third := t.TempDir()
	main3 := sharedTestApp(t, third, "", "main")
	broken := filepath.Join(third, standaloneEditorDirectory, editorSessionDirectory, editorSessionIndexName)
	writeFixture(t, broken, `{"version":1,"tabs":[`)
	writeFixture(t, filepath.Join(filepath.Dir(broken), editorSessionBackupFile(strings.Repeat("a", 32))), "orphan")
	if index, warning := readSession(t, main3); len(index.Tabs) != 0 || warning != "" {
		t.Fatalf("an unreadable session was taken over: %s (%q)", tabSummary(index), warning)
	}
	if raw, _ := os.ReadFile(broken); string(raw) != `{"version":1,"tabs":[` {
		t.Fatal("the other app's unreadable session was changed")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(broken), editorSessionBackupFile(strings.Repeat("a", 32)))); err != nil {
		t.Fatal("a kept copy next to an unreadable session was removed")
	}
	// The Terminal keeps no editor session and takes none over.
	terminal := sharedTestApp(t, root, terminalWorkspaceName, "terminal")
	if dir, _ := terminal.otherEditorWorkspace(); dir != "" {
		t.Fatalf("the Terminal would take over an editor session from %s", dir)
	}
}

func TestEditorSessionWaitsWhileItsTabsAreBeingTakenOver(t *testing.T) {
	_, main, _ := editorSessionApps(t)
	previous := editorSessionLeaseWait
	editorSessionLeaseWait = 150 * time.Millisecond
	defer func() { editorSessionLeaseWait = previous }()
	// Another app holds this workspace's session exclusively: it is moving
	// tabs out of it.
	workspace, err := openStorageRoot(filepath.Dir(main.dataPath), true)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := openStorageRegular(workspace, editorSessionLeaseName, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if err := lockStorageFile(lease, true); err != nil {
		t.Fatal(err)
	}
	if r := requestApp(main, http.MethodGet, editorSessionEndpoint, "", true); r.Code != http.StatusServiceUnavailable {
		t.Fatalf("a session was read while its tabs were being moved: %d %s", r.Code, r.Body.String())
	}
	if r := putSession(t, main, map[string]any{"version": 1, "active": -1, "tabs": []any{}}); r.Code != http.StatusServiceUnavailable {
		t.Fatalf("a session was saved while its tabs were being moved: %d", r.Code)
	}
	if err := unlockStorageFile(lease); err != nil {
		t.Fatal(err)
	}
	if index, _ := readSession(t, main); len(index.Tabs) != 0 {
		t.Fatalf("session after the move finished: %s", tabSummary(index))
	}
}
