package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The real searches of the Find window keep large documents responsive:
// browser checks served without the engine time a stand-in, so the speed
// of the engine's own search is bound here, through the real handlers. The
// bounds are generous for a shared CI runner; a search that reads text
// more than once per match or holds every match in memory misses them by
// far.
func TestEditorFindSpeedOnLargeDocuments(t *testing.T) {
	if testing.Short() {
		t.Skip("reads about 60 MB")
	}
	within := func(name string, limit time.Duration, run func()) {
		t.Helper()
		start := time.Now()
		run()
		if took := time.Since(start); took > limit {
			t.Errorf("%s took %v, more than %v", name, took, limit)
		} else {
			t.Logf("%s: %v", name, took)
		}
	}
	// Text a page holds: 16 MiB, the match on its last line.
	a, _, _ := editorFixture(t)
	var page strings.Builder
	for i := 0; page.Len() < 16<<20; i++ {
		fmt.Fprintf(&page, "interface ge-0/0/%d description uplink to 192.0.2.%d\n", i, i%250)
	}
	page.WriteString("SYNTHETIC_END\n")
	text := page.String()
	within("Find Next to the end of 16 MiB of page-held text", 8*time.Second, func() {
		if next := findCall(t, a, a.token, map[string]any{"action": "next", "text": text, "search": map[string]any{"query": "SYNTHETIC_END", "matchCase": true}, "at": 0}); next["found"] != true {
			t.Fatal(next)
		}
	})
	within("Find All of 300,000 hits in 16 MiB of page-held text", 15*time.Second, func() {
		job := findCall(t, a, a.token, map[string]any{"action": "start", "kind": "all", "text": text, "search": map[string]any{"query": "uplink"}})["job"].(string)
		if done := waitFind(t, a, a.token, job); done["count"].(float64) < 280000 {
			t.Fatal(done["count"])
		}
	})
	// A disk-backed document: about 48 MB.
	var disk strings.Builder
	for i := 0; disk.Len() < 48<<20; i++ {
		fmt.Fprintf(&disk, "host-%07d.example.com 192.0.2.%d port %d\n", i, i%250, 1000+i%9000)
	}
	disk.WriteString("SYNTHETIC_END\n")
	path := filepath.Join(t.TempDir(), "large.txt")
	if err := os.WriteFile(path, []byte(disk.String()), 0600); err != nil {
		t.Fatal(err)
	}
	b, m, _ := editorFixture(t)
	lease, status, code := openAnyFile(t, b, path)
	if code != 200 {
		t.Fatal(code, status.Error)
	}
	defer m.releaseDocument("", editorFixtureUID, lease)
	within("Find Next to the end of a 48 MB disk-backed document", 10*time.Second, func() {
		if next := bufferCall(t, b, lease, map[string]any{"action": "find", "search": map[string]any{"query": "SYNTHETIC_END", "matchCase": true}, "at": 0}); next["found"] != true {
			t.Fatal(next)
		}
	})
	within("Count of a regular expression in a 48 MB disk-backed document", 20*time.Second, func() {
		job := bufferCall(t, b, lease, map[string]any{"action": "find-start", "kind": "count", "search": map[string]any{"query": `port 9\d{3}$`, "regex": true}})["job"].(string)
		if done := waitFind(t, b, b.token, job); done["count"].(float64) < 1000 {
			t.Fatal(done["count"])
		}
	})
	within("Find All of a million hits in a 48 MB disk-backed document", 30*time.Second, func() {
		job := bufferCall(t, b, lease, map[string]any{"action": "find-start", "kind": "all", "search": map[string]any{"query": "example.com", "matchCase": true}})["job"].(string)
		done := waitFind(t, b, b.token, job)
		if done["count"].(float64) < 1000000 {
			t.Fatal(done["count"])
		}
		rows := findCall(t, b, b.token, map[string]any{"action": "page", "job": job, "start": 999000, "limit": 200})["items"].([]any)
		if len(rows) != 200 {
			t.Fatal(len(rows))
		}
	})
}
