package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Fixes from the review of the text tools (v1.21.53). Each test failed
// before its fix.

// deepJSON is a JSON list of chains arrays nested depth levels: small to
// send, and pretty-printing it would write an indent per level per line.
func deepJSON(depth, chains int) string {
	chain := strings.Repeat("[", depth) + strings.Repeat("]", depth)
	return "[" + strings.TrimSuffix(strings.Repeat(chain+",", chains), ",") + "]"
}

// The result of a tool over page-held text is bounded: a crafted input that
// would blow up is refused instead of filling the engine's memory.
func TestEditorTextToolsPageResultIsBounded(t *testing.T) {
	a, _, _ := editorFixture(t)
	r := editorHTTP(a, a.token, "/api/editor-views/text-tools", map[string]any{"action": "run", "tool": "json-pretty", "text": deepJSON(500, 120), "options": map[string]any{"indent": 4}})
	if r.Code != 200 || r.Body.Len() > 4096 {
		t.Fatalf("%d, a %d byte answer", r.Code, r.Body.Len())
	}
	var out map[string]any
	json.Unmarshal(r.Body.Bytes(), &out)
	if out["ok"] != false || !strings.Contains(out["error"].(string), "too large") {
		t.Fatal(out)
	}
	// Nesting deeper than the formatter takes is refused at once.
	out = textToolsCall(t, a, a.token, map[string]any{"action": "run", "tool": "xml-pretty", "text": strings.Repeat("<a>", 4000) + strings.Repeat("</a>", 4000)})
	if out["ok"] != false || !strings.Contains(out["error"].(string), "deeper than 512 levels") {
		t.Fatal(out)
	}
}

// Closing a disk-backed document while a tool writes far more than it reads
// stops the tool at once; it does not finish writing first.
func TestEditorTextToolStopsWhenItsDocumentCloses(t *testing.T) {
	a, m, _ := editorFixture(t)
	path := filepath.Join(t.TempDir(), "deep.json")
	os.WriteFile(path, []byte(deepJSON(4000, 300)), 0600)
	lease, status, code := openAnyFile(t, a, path)
	if code != 200 {
		t.Fatal(code)
	}
	r := diskCall(a, a.token, lease, map[string]any{"action": "tool-start", "revision": status.Metadata.Revision, "tool": "json-pretty", "options": map[string]any{"indent": 16}})
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	finishes(t, "closing the document while it is pretty-printed", func() { m.releaseDocument("", editorFixtureUID, lease) })
	if took := time.Since(start); took > 3*time.Second {
		t.Fatalf("closing took %v", took)
	}
}
