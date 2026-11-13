package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The Find window's requests (editor_find.go) through the real handlers.

func findCall(t *testing.T, a *application, token string, body map[string]any) map[string]any {
	t.Helper()
	r := editorHTTP(a, token, "/api/editor-views/find", body)
	if r.Code != 200 {
		t.Fatalf("%v: %d %s", body["action"], r.Code, r.Body.String())
	}
	var out map[string]any
	json.Unmarshal(r.Body.Bytes(), &out)
	return out
}

func bufferCall(t *testing.T, a *application, lease uint64, body map[string]any) map[string]any {
	t.Helper()
	r := diskCall(a, a.token, lease, body)
	if r.Code != 200 {
		t.Fatalf("%v: %d %s", body["action"], r.Code, r.Body.String())
	}
	var out map[string]any
	json.Unmarshal(r.Body.Bytes(), &out)
	return out
}

// waitFind polls a job until it is done.
func waitFind(t *testing.T, a *application, token, job string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		state := findCall(t, a, token, map[string]any{"action": "status", "job": job})
		switch state["state"] {
		case "done":
			return state
		case "failed":
			t.Fatalf("job failed: %v", state["error"])
		}
		if time.Now().After(deadline) {
			t.Fatal("the search did not finish")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func openFindFile(t *testing.T, text string) (*application, uint64, string) {
	t.Helper()
	a, m, _ := editorFixture(t)
	path := filepath.Join(t.TempDir(), "config.txt")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	lease, status, code := openAnyFile(t, a, path)
	if code != 200 {
		t.Fatalf("%d %s", code, status.Error)
	}
	t.Cleanup(func() { m.releaseDocument("", editorFixtureUID, lease) })
	return a, lease, path
}

// A disk-backed document: Find Next with a range, wrap-around and
// direction; the marks of a page; Count, Find All read a page at a time,
// Mark All with its lines, Copy Marked Text and Replace All "In selection"
// as one undo step.
func TestEditorFindWindowOnADiskDocument(t *testing.T) {
	var text strings.Builder
	for i := 0; i < 3000; i++ {
		fmt.Fprintf(&text, "interface ge-0/0/%d\n description link to 192.0.2.%d\n", i, i%250)
	}
	a, lease, path := openFindFile(t, text.String())
	search := map[string]any{"query": "192.0.2.7", "matchCase": true, "wholeWord": true}
	// Find Next: the first match, then from its end the next; without wrap
	// nothing after the last; backwards from the first wraps to the last.
	first := bufferCall(t, a, lease, map[string]any{"action": "find", "search": search, "at": 0, "wrap": true})
	if first["found"] != true || first["line"].(float64) != 15 {
		t.Fatal(first)
	}
	last := bufferCall(t, a, lease, map[string]any{"action": "find", "search": search, "at": 0, "backwards": true, "wrap": true})
	if last["found"] != true || last["wrapped"] != true || last["line"].(float64) != 5515 {
		t.Fatal(last)
	}
	if none := bufferCall(t, a, lease, map[string]any{"action": "find", "search": search, "at": last["to"], "wrap": false}); none["found"] != false {
		t.Fatal(none)
	}
	// In selection: lines 0 to 100.
	within := []int64{0, int64(first["to"].(float64)) + 2000}
	inside := bufferCall(t, a, lease, map[string]any{"action": "find", "search": search, "at": first["to"], "within": within, "wrap": true})
	if inside["found"] != true || inside["wrapped"] != true || inside["from"] != first["from"] {
		t.Fatal(inside)
	}
	// The marks of a shown page, bounded.
	page := bufferCall(t, a, lease, map[string]any{"action": "find-matches", "search": map[string]any{"query": "interface", "matchCase": true}, "from": 0, "to": 200})
	if hits := page["hits"].([]any); len(hits) != 4 || page["more"] != false {
		t.Fatal(page)
	}
	start := func(kind string, extra map[string]any) map[string]any {
		body := map[string]any{"action": "find-start", "kind": kind, "search": search}
		for k, v := range extra {
			body[k] = v
		}
		started := bufferCall(t, a, lease, body)
		return waitFind(t, a, a.token, started["job"].(string))
	}
	if count := start("count", nil); count["count"].(float64) != 12 {
		t.Fatal(count)
	}
	if count := start("count", map[string]any{"within": within}); count["count"].(float64) != 1 {
		t.Fatal(count)
	}
	all := start("all", map[string]any{"search": map[string]any{"query": `ge-0/0/\d+$`, "regex": true}})
	if all["count"].(float64) != 3000 || all["rows"].(float64) != 3000 {
		t.Fatal(all)
	}
	job := bufferCall(t, a, lease, map[string]any{"action": "find-start", "kind": "all", "search": map[string]any{"query": "link"}})["job"].(string)
	waitFind(t, a, a.token, job)
	rows := findCall(t, a, a.token, map[string]any{"action": "page", "job": job, "start": 2990, "limit": 100})
	items := rows["items"].([]any)
	if len(items) != 10 || rows["hits"].(float64) != 3000 {
		t.Fatal(len(items), rows["hits"])
	}
	row := items[9].(map[string]any)
	if row["line"].(float64) != 5999 || row["text"] != " description link to 192.0.2.249" {
		t.Fatal(row)
	}
	// Another window cannot read this window's results.
	other := randomHex(16)
	token := randomHex(32)
	a.editorViews.mu.Lock()
	a.editorViews.views[other] = &editorWindow{info: editorViewInfo{ID: other, State: "ready"}, token: token}
	a.editorViews.mu.Unlock()
	if r := editorHTTP(a, token, "/api/editor-views/find", map[string]any{"action": "page", "job": job, "start": 0, "limit": 10}); r.Code != 409 {
		t.Fatal(r.Code, r.Body.String())
	}
	findCall(t, a, a.token, map[string]any{"action": "release", "job": job})
	if r := editorHTTP(a, a.token, "/api/editor-views/find", map[string]any{"action": "page", "job": job, "start": 0, "limit": 10}); r.Code != 409 {
		t.Fatal("released results were still read")
	}
	mark := start("mark", map[string]any{"search": map[string]any{"query": "192.0.2.249"}})
	if mark["count"].(float64) != 12 || len(mark["lines"].([]any)) != 12 || mark["lines"].([]any)[0].(float64) != 499 {
		t.Fatal(mark)
	}
	copied := start("copy", map[string]any{"search": map[string]any{"query": `ge-0/0/1\d\b`, "regex": true}, "within": []int64{0, 2000}, "separator": "\r\n"})
	if copied["text"] != "ge-0/0/10\r\nge-0/0/11\r\nge-0/0/12\r\nge-0/0/13\r\nge-0/0/14\r\nge-0/0/15\r\nge-0/0/16\r\nge-0/0/17\r\nge-0/0/18\r\nge-0/0/19" {
		t.Fatal(copied["text"])
	}
	// Replace All in the first 100 lines: one undo step.
	status := bufferCall(t, a, lease, map[string]any{"action": "status"})
	revision := status["metadata"].(map[string]any)["revision"]
	replaced := start("replace", map[string]any{"revision": revision, "search": map[string]any{"query": `link to (192\.0\.2)\.(\d+)`, "regex": true}, "replacement": "uplink $1.$2/24", "within": []int64{0, 2000}})
	// 38 settings in the range, each two characters longer.
	if replaced["count"].(float64) != 38 || replaced["to"].(float64) != 2000+38*2 {
		t.Fatal(replaced)
	}
	window := bufferCall(t, a, lease, map[string]any{"action": "window", "start": 0, "limit": 60})
	if !strings.HasPrefix(window["text"].(string), "interface ge-0/0/0\n description uplink 192.0.2.0/24\n") {
		t.Fatal(window["text"])
	}
	meta := replaced["metadata"].(map[string]any)
	bufferCall(t, a, lease, map[string]any{"action": "undo", "revision": meta["revision"]})
	window = bufferCall(t, a, lease, map[string]any{"action": "window", "start": 0, "limit": 60})
	if !strings.HasPrefix(window["text"].(string), "interface ge-0/0/0\n description link to 192.0.2.0\n") {
		t.Fatal(window["text"])
	}
	// A Replace All of an older revision changes nothing.
	if r := diskCall(a, a.token, lease, map[string]any{"action": "find-start", "kind": "replace", "revision": revision, "search": search, "replacement": "x"}); r.Code != 409 {
		t.Fatal(r.Code, r.Body.String())
	}
	if raw, _ := os.ReadFile(path); string(raw) != text.String() {
		t.Fatal("the file changed on disk")
	}
}

// A long search is cancelled, and closing the document stops the jobs that
// read it.
func TestEditorFindWindowCancelAndClose(t *testing.T) {
	text := strings.Repeat("abcdefghij\n", 400000)
	a, lease, _ := openFindFile(t, text)
	started := bufferCall(t, a, lease, map[string]any{"action": "find-start", "kind": "all", "search": map[string]any{"query": "e"}})
	job := started["job"].(string)
	findCall(t, a, a.token, map[string]any{"action": "cancel", "job": job})
	deadline := time.Now().Add(10 * time.Second)
	for {
		state := findCall(t, a, a.token, map[string]any{"action": "status", "job": job})
		if state["state"] == "failed" {
			if state["error"] != "The search was cancelled." {
				t.Fatal(state)
			}
			break
		}
		if state["state"] == "done" {
			// It finished before the cancel arrived: still a whole result.
			if state["count"].(float64) != 400000 {
				t.Fatal(state)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the cancelled search went on")
		}
		time.Sleep(time.Millisecond)
	}
	started = bufferCall(t, a, lease, map[string]any{"action": "find-start", "kind": "count", "search": map[string]any{"query": "zz"}})
	a.editorViews.releaseDocument("", editorFixtureUID, lease)
	state := waitFindEnd(t, a, started["job"].(string))
	if state["state"] == "done" && state["count"].(float64) != 0 {
		t.Fatal(state)
	}
}

func waitFindEnd(t *testing.T, a *application, job string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		state := findCall(t, a, a.token, map[string]any{"action": "status", "job": job})
		if state["state"] != "running" {
			return state
		}
		if time.Now().After(deadline) {
			t.Fatal("the search of a closed document went on")
		}
		time.Sleep(time.Millisecond)
	}
}

// Text held by a page: the same searches, sent with the text.
func TestEditorFindWindowOnPageText(t *testing.T) {
	a, _, _ := editorFixture(t)
	text := "vlan 10\n name users\nvlan 20\n name voice\nend\n"
	next := findCall(t, a, a.token, map[string]any{"action": "next", "text": text, "search": map[string]any{"query": `^vlan`, "regex": true}, "at": 1, "wrap": true})
	if next["found"] != true || next["from"].(float64) != 20 || next["line"].(float64) != 2 {
		t.Fatal(next)
	}
	dot := findCall(t, a, a.token, map[string]any{"action": "next", "text": text, "search": map[string]any{"query": `users.vlan`, "regex": true, "dotAll": true}})
	if dot["found"] != true || dot["from"].(float64) != 14 {
		t.Fatal(dot)
	}
	matches := findCall(t, a, a.token, map[string]any{"action": "matches", "text": text, "search": map[string]any{"query": "name"}, "from": 0, "to": 50})
	if len(matches["hits"].([]any)) != 2 {
		t.Fatal(matches)
	}
	replaced := findCall(t, a, a.token, map[string]any{"action": "replace", "text": text, "search": map[string]any{"query": `vlan (\d+)`, "regex": true}, "replacement": "vlan ${1}0", "within": []int64{0, 20}})
	if replaced["text"] != "vlan 100\n name users\nvlan 20\n name voice\nend\n" || replaced["count"].(float64) != 1 {
		t.Fatal(replaced)
	}
	one := findCall(t, a, a.token, map[string]any{"action": "replace-text", "text": text, "search": map[string]any{"query": `name (\w+)`, "regex": true}, "replacement": "description $1", "from": 9, "to": 19})
	if one["found"] != true || one["text"] != "description users" {
		t.Fatal(one)
	}
	job := findCall(t, a, a.token, map[string]any{"action": "start", "kind": "all", "text": text, "search": map[string]any{"query": "e"}})["job"].(string)
	done := waitFind(t, a, a.token, job)
	if done["count"].(float64) != 5 || done["rows"].(float64) != 3 {
		t.Fatal(done)
	}
	rows := findCall(t, a, a.token, map[string]any{"action": "page", "job": job, "start": 0, "limit": 10})["items"].([]any)
	if first := rows[0].(map[string]any); first["text"] != " name users" || len(first["hits"].([]any)) != 2 {
		t.Fatal(rows)
	}
	if r := editorHTTP(a, a.token, "/api/editor-views/find", map[string]any{"action": "next", "text": text, "search": map[string]any{"query": "(", "regex": true}}); r.Code != 409 || !strings.Contains(r.Body.String(), "Invalid regular expression") {
		t.Fatal(r.Code, r.Body.String())
	}
	if r := editorHTTP(a, a.token, "/api/editor-views/find", map[string]any{"action": "start", "kind": "replace", "text": text, "search": map[string]any{"query": "e"}}); r.Code != 409 {
		t.Fatal(r.Code)
	}
	if r := editorHTTP(a, a.token, "/api/editor-views/find", map[string]any{"action": "next", "text": text, "path": "/etc/passwd"}); r.Code != 400 {
		t.Fatal(r.Code)
	}
}

// The search history lives in the engine's memory only: the 20 latest
// entries of each list, shared by its editor windows, never in the saved
// settings.
func TestEditorFindHistoryStaysInMemory(t *testing.T) {
	a, m, _ := editorFixture(t)
	other := randomHex(16)
	token := randomHex(32)
	m.mu.Lock()
	m.views[other] = &editorWindow{info: editorViewInfo{ID: other, State: "ready"}, token: token}
	m.mu.Unlock()
	for i := 0; i < 25; i++ {
		findCall(t, a, a.token, map[string]any{"action": "remember", "list": "find", "value": fmt.Sprintf("secret-%d", i)})
	}
	findCall(t, a, token, map[string]any{"action": "remember", "list": "find", "value": "secret-10"})
	findCall(t, a, token, map[string]any{"action": "remember", "list": "replace", "value": "masked"})
	history := findCall(t, a, a.token, map[string]any{"action": "history"})
	list := history["find"].([]any)
	if len(list) != 20 || list[0] != "secret-10" || list[1] != "secret-24" || history["replace"].([]any)[0] != "masked" {
		t.Fatal(history)
	}
	for _, bad := range []map[string]any{{"list": "other", "value": "x"}, {"list": "find", "value": ""}, {"list": "find", "value": strings.Repeat("x", 4097)}} {
		bad["action"] = "remember"
		if r := editorHTTP(a, a.token, "/api/editor-views/find", bad); r.Code != 409 {
			t.Fatal(bad, r.Code)
		}
	}
	if raw, _ := os.ReadFile(a.dataPath); strings.Contains(string(raw), "secret-") {
		t.Fatal("search history reached the settings file")
	}
}

// A file that is not text: Count, Find All, Mark All and Copy Marked Text
// read it; Replace All is refused by the engine.
func TestEditorFindWindowReadsAFileThatIsNotText(t *testing.T) {
	a, m, _ := editorFixture(t)
	path := filepath.Join(t.TempDir(), "image.bin")
	raw := append([]byte("\x89PNG\r\n\x1a\n\x00name="), []byte("value\x00name=other\x00")...)
	os.WriteFile(path, raw, 0600)
	lease, status, code := openAnyFile(t, a, path)
	if code != 200 || status.Format.Encoding != "Binary" {
		t.Fatal(code, status.Format)
	}
	defer m.releaseDocument("", editorFixtureUID, lease)
	search := map[string]any{"query": "name=", "matchCase": true}
	for _, kind := range []string{"count", "all", "mark", "copy"} {
		started := bufferCall(t, a, lease, map[string]any{"action": "find-start", "kind": kind, "search": search})
		if done := waitFind(t, a, a.token, started["job"].(string)); done["count"].(float64) != 2 {
			t.Fatal(kind, done)
		}
	}
	if found := bufferCall(t, a, lease, map[string]any{"action": "find", "search": search, "wrap": true}); found["found"] != true {
		t.Fatal(found)
	}
	if hits := bufferCall(t, a, lease, map[string]any{"action": "find-matches", "search": search, "from": 0, "to": 40}); len(hits["hits"].([]any)) != 2 {
		t.Fatal(hits)
	}
	revision := status.Metadata.Revision
	for _, refused := range []map[string]any{
		{"action": "find-start", "kind": "replace", "revision": revision, "search": search, "replacement": "x"},
		{"action": "replace-all", "revision": revision, "search": search, "replacement": "x", "within": []int64{0, 10}},
	} {
		if r := diskCall(a, a.token, lease, refused); r.Code == 200 || !strings.Contains(r.Body.String(), "not a text file") {
			t.Fatal(refused, r.Code, r.Body.String())
		}
	}
	if got, _ := os.ReadFile(path); string(got) != string(raw) {
		t.Fatal("the file changed on disk")
	}
}
