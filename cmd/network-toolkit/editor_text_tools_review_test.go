package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"networktoolkit/internal/editorbuffer"
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

// The readable copy of an export that fails (here a truncated one) while its
// source is being closed: the source snapshot is released before the error
// message reads the source again, so neither the copy nor the close waits
// forever.
func TestEditorReadableCopyFailureReleasesTheSource(t *testing.T) {
	_, m, _ := editorFixture(t)
	encoded := strings.TrimRight(base64.StdEncoding.EncodeToString([]byte("buildNum=7.1&shortProdName=TZ%20370&checksum=1&")), "=")
	truncated := encoded[:len(encoded)-len(encoded)%4] + "Q"
	v, err := editorbuffer.Import(t.Context(), strings.NewReader(truncated), t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	source := &editorDiskDocument{imported: v}
	closed := make(chan struct{})
	ctx := &hookContext{Context: t.Context(), hook: func() {
		go func() { v.Buffer.Close(); close(closed) }()
		time.Sleep(300 * time.Millisecond) // the close now waits for the derive's snapshot
	}}
	finishes(t, "a readable copy that fails while its source closes", func() {
		_, err := m.deriveDiskDocument(ctx, &editorDiskDocument{}, source, editorDeriveRequest{Tool: "sonicwall-readable", Revision: v.Buffer.Metadata().Revision})
		if err == nil {
			t.Error("a truncated export was made readable")
		}
	})
	finishes(t, "closing the source", func() { <-closed })
}

// A reload just after a tool started stops it: the stop is registered
// before the start is answered, not later in the tool's own goroutine.
func TestEditorTextToolStartIsStoppedByAnImmediateReload(t *testing.T) {
	for round := 0; round < 20; round++ {
		v, err := editorbuffer.Import(t.Context(), strings.NewReader(strings.Repeat("some text\n", 1000)), t.TempDir(), nil)
		if err != nil {
			t.Fatal(err)
		}
		d := &editorDiskDocument{ctx: t.Context(), imported: v}
		started, err := d.startTool(v, editorDiskRequest{Tool: "upper", Revision: v.Buffer.Metadata().Revision})
		if err != nil {
			t.Fatal(err)
		}
		d.stopReplaceAll() // what a reload does before it replaces the buffer
		var state any
		finishes(t, "the stopped tool", func() {
			for {
				if state, err = d.toolStatus(editorDiskRequest{Job: started.(map[string]any)["job"].(string)}, false); err != nil || state.(map[string]any)["state"] != "running" {
					return
				}
				time.Sleep(time.Millisecond)
			}
		})
		if got := state.(map[string]any); got["state"] != "failed" || !strings.Contains(got["error"].(string), "cancelled") {
			t.Fatalf("round %d: a tool started just before a reload went on: %v", round, got)
		}
		if meta := v.Buffer.Metadata(); meta.Dirty {
			t.Fatal("the stopped tool changed the document")
		}
		v.Buffer.Close()
	}
}

// Line tools take in an empty last selected line. Selecting lines 1 to 3 of
// "a\n\n\nb" by dragging to the start of "b" is the range [0,4).
func TestEditorLineToolsTakeAnEmptyLastLine(t *testing.T) {
	cases := []struct {
		text, tool string
		options    map[string]any
		from, to   int
		want       string
		count      float64
	}{
		{"a\n\n\nb", "remove-empty", nil, 0, 4, "a\nb", 2},
		{"a\n\nb", "join", map[string]any{"separator": "+"}, 0, 3, "a+\nb", 1},
		{"\nx\n\nb", "dedupe", nil, 0, 4, "\nx\nb", 1},
		{"b\n\nc", "sort-asc", nil, 0, 3, "\nb\nc", 2},
	}
	for _, c := range cases {
		a, m, _ := editorFixture(t)
		path := filepath.Join(t.TempDir(), "lines.txt")
		os.WriteFile(path, []byte(c.text), 0600)
		lease, status, code := openAnyFile(t, a, path)
		if code != 200 {
			t.Fatal(code)
		}
		r := diskCall(a, a.token, lease, map[string]any{"action": "tool-start", "revision": status.Metadata.Revision, "tool": c.tool, "options": c.options, "from": c.from, "to": c.to})
		var started map[string]any
		json.Unmarshal(r.Body.Bytes(), &started)
		state := waitTool(t, a, lease, started["job"])
		if got := windowText(t, a, editorFixtureUID, lease, 0, 64).Text; got != c.want || state["count"] != c.count {
			t.Errorf("%s of %q [%d,%d): %q (%v), want %q (%v)", c.tool, c.text, c.from, c.to, got, state["count"], c.want, c.count)
		}
		m.releaseDocument("", editorFixtureUID, lease)
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
