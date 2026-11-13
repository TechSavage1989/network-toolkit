package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"networktoolkit/internal/editorbuffer"
)

func textToolsCall(t *testing.T, a *application, token string, body map[string]any) map[string]any {
	t.Helper()
	r := editorHTTP(a, token, "/api/editor-views/text-tools", body)
	if r.Code != 200 {
		t.Fatalf("%d %s", r.Code, r.Body.String())
	}
	var out map[string]any
	json.Unmarshal(r.Body.Bytes(), &out)
	return out
}

// Page-held text: the engine runs the tool over the text it was sent.
func TestEditorTextToolsForPageHeldText(t *testing.T) {
	a, _, _ := editorFixture(t)
	out := textToolsCall(t, a, a.token, map[string]any{"action": "run", "tool": "split", "text": "a=1&b=2&", "options": map[string]any{"separator": "&"}})
	if out["ok"] != true || out["changed"] != true || out["text"] != "a=1\nb=2\n" || out["count"].(float64) != 2 {
		t.Fatal(out)
	}
	out = textToolsCall(t, a, a.token, map[string]any{"action": "run", "tool": "sort-asc", "text": "a\nb"})
	if out["ok"] != true || out["changed"] != false || out["text"] != "a\nb" {
		t.Fatal(out)
	}
	out = textToolsCall(t, a, a.token, map[string]any{"action": "run", "tool": "base64-decode", "text": "QUJD\nQU#D"})
	if out["ok"] != false || !strings.Contains(out["error"].(string), "not a Base64 character") || out["offset"].(float64) != 7 {
		t.Fatal(out)
	}
	out = textToolsCall(t, a, a.token, map[string]any{"action": "run", "tool": "no-such-tool", "text": "x"})
	if out["ok"] != false {
		t.Fatal(out)
	}
	// The detection of v1.21.54 is gone: only "run" is a text tool request.
	if r := editorHTTP(a, a.token, "/api/editor-views/text-tools", map[string]any{"action": "detect", "text": "aGVsbG8="}); r.Code != 400 {
		t.Fatal(r.Code)
	}
	// Unknown fields are refused like every editor request.
	if r := editorHTTP(a, a.token, "/api/editor-views/text-tools", map[string]any{"action": "run", "tool": "upper", "text": "x", "path": "/etc/passwd"}); r.Code != 400 {
		t.Fatal(r.Code)
	}
}

// settingsLine is a fictional one-line settings file: key=value& settings
// with %XX escapes as one line of Base64, the kind the editor guide's recipe
// makes readable (Base64 decode, Split at text &, URL decode). readable is
// what the recipe makes of plain: one setting per line, values decoded.
func settingsLine(settings int) (encoded, plain, readable string) {
	var pairs, lines strings.Builder
	add := func(key, value, shown string) {
		pairs.WriteString(key + "=" + value + "&")
		lines.WriteString(key + "=" + shown + "\n")
	}
	for i := 0; i < settings; i++ {
		add(fmt.Sprintf("name_%d", i), fmt.Sprintf("Office%%20%d", i), fmt.Sprintf("Office %d", i))
		add(fmt.Sprintf("address_%d", i), fmt.Sprintf("192.0.2.%d", i%250), fmt.Sprintf("192.0.2.%d", i%250))
		add(fmt.Sprintf("note_%d", i), "a+b%3Ac%20d", "a+b:c d")
		add(fmt.Sprintf("site_%d", i), "www.example.com%2Fpath", "www.example.com/path")
	}
	return base64.StdEncoding.EncodeToString([]byte(pairs.String())), pairs.String(), lines.String()
}

// waitTool polls a text tool until it ends.
func waitTool(t *testing.T, a *application, lease uint64, job any) map[string]any {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		r := diskCall(a, a.token, lease, map[string]any{"action": "tool-status", "job": job})
		var state map[string]any
		json.Unmarshal(r.Body.Bytes(), &state)
		if r.Code != 200 {
			t.Fatalf("%d %s", r.Code, r.Body.String())
		}
		if state["state"] != "running" {
			return state
		}
		if time.Now().After(deadline) {
			t.Fatal("the tool is still running after 20 seconds")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func windowText(t *testing.T, a *application, uid string, lease uint64, start, limit int) editorbuffer.Window {
	t.Helper()
	r := editorHTTP(a, a.token, "/api/editor-views/documents/"+uid+"/buffer", map[string]any{"action": "window", "lease": lease, "start": start, "limit": limit})
	var w editorbuffer.Window
	json.Unmarshal(r.Body.Bytes(), &w)
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	return w
}

// A disk-backed document: the whole of a large single-line document is
// decoded in the engine as one undo step, with progress and a result that
// names what was done.
func TestEditorTextToolsOnADiskDocument(t *testing.T) {
	a, m, _ := editorFixture(t)
	plain := strings.Repeat("interface port0/1\n description uplink to 192.0.2.1\n", 25000)
	encoded := base64.StdEncoding.EncodeToString([]byte(plain))
	path := filepath.Join(t.TempDir(), "large.b64")
	os.WriteFile(path, []byte(encoded), 0600)
	lease, status, code := openAnyFile(t, a, path)
	if code != 200 {
		t.Fatal(code)
	}
	defer m.releaseDocument("", editorFixtureUID, lease)
	revision := status.Metadata.Revision
	r := diskCall(a, a.token, lease, map[string]any{"action": "tool-start", "revision": revision, "tool": "base64-decode"})
	var started map[string]any
	json.Unmarshal(r.Body.Bytes(), &started)
	if r.Code != 200 || started["job"] == nil {
		t.Fatal(r.Body.String())
	}
	state := waitTool(t, a, lease, started["job"])
	if state["state"] != "done" || state["changed"] != true || state["scope"] != "document" || state["from"].(float64) != 0 || int(state["to"].(float64)) != len(plain) {
		t.Fatal(state)
	}
	if w := windowText(t, a, editorFixtureUID, lease, 0, 64); !strings.HasPrefix(w.Text, "interface port0/1\n") || w.Units != int64(len(plain)) || !w.Dirty {
		t.Fatalf("%q %+v", w.Text[:20], w.Metadata)
	}
	meta := state["metadata"].(map[string]any)
	// One undo step gives the Base64 back.
	if r = diskCall(a, a.token, lease, map[string]any{"action": "undo", "revision": meta["revision"]}); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if w := windowText(t, a, editorFixtureUID, lease, 0, 64); !strings.HasPrefix(w.Text, encoded[:64]) || w.Dirty {
		t.Fatal("undo did not restore the document")
	}
	// A tool that would change nothing says so and changes nothing.
	r = diskCall(a, a.token, lease, map[string]any{"action": "tool-start", "revision": revision + 2, "tool": "trim-trailing"})
	json.Unmarshal(r.Body.Bytes(), &started)
	if state = waitTool(t, a, lease, started["job"]); state["state"] != "done" || state["changed"] != false {
		t.Fatal(state)
	}
	// A tool that fails names the line and column and changes nothing.
	r = diskCall(a, a.token, lease, map[string]any{"action": "tool-start", "revision": revision + 2, "tool": "json-pretty"})
	json.Unmarshal(r.Body.Bytes(), &started)
	if state = waitTool(t, a, lease, started["job"]); state["state"] != "failed" || !strings.Contains(state["error"].(string), "(line 1, column 1)") {
		t.Fatal(state)
	}
	// An outdated revision changes nothing.
	r = diskCall(a, a.token, lease, map[string]any{"action": "tool-start", "revision": revision, "tool": "lower"})
	json.Unmarshal(r.Body.Bytes(), &started)
	if state = waitTool(t, a, lease, started["job"]); state["state"] != "failed" || !strings.Contains(state["error"].(string), "changed while the tool ran") {
		t.Fatal(state)
	}
	// A tool the engine does not have changes nothing.
	if r = diskCall(a, a.token, lease, map[string]any{"action": "tool-start", "revision": revision + 2, "tool": "no-such-tool"}); r.Code != 409 {
		t.Fatal(r.Code)
	}
	if r = diskCall(a, a.token, lease, map[string]any{"action": "tool-status", "job": "0123"}); r.Code != 409 {
		t.Fatal(r.Code)
	}
}

// A cancelled tool stops and changes nothing; a sort larger than its memory
// budget still finishes.
func TestEditorTextToolCancel(t *testing.T) {
	a, m, _ := editorFixture(t)
	var text strings.Builder
	for i := 0; i < 400000; i++ {
		fmt.Fprintf(&text, "host-%d.example.com\n", (i*7919)%400000)
	}
	path := filepath.Join(t.TempDir(), "hosts.txt")
	os.WriteFile(path, []byte(text.String()), 0600)
	lease, status, code := openAnyFile(t, a, path)
	if code != 200 {
		t.Fatal(code)
	}
	defer m.releaseDocument("", editorFixtureUID, lease)
	r := diskCall(a, a.token, lease, map[string]any{"action": "tool-start", "revision": status.Metadata.Revision, "tool": "sort-asc"})
	var started map[string]any
	json.Unmarshal(r.Body.Bytes(), &started)
	if r = diskCall(a, a.token, lease, map[string]any{"action": "tool-start", "revision": status.Metadata.Revision, "tool": "lower"}); r.Code != 409 || !strings.Contains(r.Body.String(), "still running") {
		t.Fatal("a second tool started while the first ran: ", r.Code)
	}
	if r = diskCall(a, a.token, lease, map[string]any{"action": "tool-cancel", "job": started["job"]}); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	state := waitTool(t, a, lease, started["job"])
	if state["state"] == "done" {
		t.Skip("the sort finished before it could be cancelled")
	}
	if !strings.Contains(state["error"].(string), "cancelled") {
		t.Fatal(state)
	}
	if w := windowText(t, a, editorFixtureUID, lease, 0, 32); w.Revision != status.Metadata.Revision || w.Dirty {
		t.Fatal("a cancelled tool changed the document")
	}
	r = diskCall(a, a.token, lease, map[string]any{"action": "tool-start", "revision": status.Metadata.Revision, "tool": "sort-asc"})
	json.Unmarshal(r.Body.Bytes(), &started)
	if state = waitTool(t, a, lease, started["job"]); state["state"] != "done" || state["count"].(float64) != 400000 {
		t.Fatal(state)
	}
	if w := windowText(t, a, editorFixtureUID, lease, 0, 40); !strings.HasPrefix(w.Text, "host-0.example.com\nhost-1.example.com\n") {
		t.Fatalf("%q", w.Text)
	}
}

// runTool starts a text tool on a disk-backed document and waits for it.
func runTool(t *testing.T, a *application, lease uint64, body map[string]any) map[string]any {
	t.Helper()
	body["action"] = "tool-start"
	r := diskCall(a, a.token, lease, body)
	var started map[string]any
	json.Unmarshal(r.Body.Bytes(), &started)
	if r.Code != 200 || started["job"] == nil {
		t.Fatal(r.Code, r.Body.String())
	}
	return waitTool(t, a, lease, started["job"])
}

// Page-held text: a decoder that stops at a character it does not decode
// changes nothing and names the position; when the text before it decodes on
// its own the answer offers it ("partial"), and the page sends that part
// again. Here the page's side is done by hand: the selection [start, end) is
// sent, then [start, start+offset), and the result goes in its place.
func TestEditorDecodeThePartBeforeForPageHeldText(t *testing.T) {
	a, _, _ := editorFixture(t)
	encoded, plain, _ := settingsLine(3)
	cases := []struct {
		tool, content string
		start, end    int
		want          string // the document afterwards; "" when nothing is offered
	}{
		// A one-line file whose Base64 is followed by "&&", nothing selected.
		{"base64-decode", encoded + "&&", 0, len(encoded) + 2, plain + "&&"},
		// A character that is not Base64 in the middle of a selection.
		{"base64-decode", "before\naGVsbG8gd29y#bGQ=\nafter", 7, 24, "before\nhello wor#bGQ=\nafter"},
		{"hex-decode", "x 68656c6c6f;20776f726c64 y", 2, 25, "x hello;20776f726c64 y"},
		// Nothing before it, or an incomplete group: nothing is offered.
		{"base64-decode", "&&QUJD", 0, 6, ""},
		{"base64-decode", "QUJDR&&", 0, 7, ""},
	}
	for _, c := range cases {
		text := c.content[c.start:c.end]
		out := textToolsCall(t, a, a.token, map[string]any{"action": "run", "tool": c.tool, "text": text})
		if out["ok"] != false || !strings.Contains(out["error"].(string), " is not a") {
			t.Fatalf("%q: %v", text, out)
		}
		if c.want == "" {
			if out["partial"] != nil {
				t.Errorf("%q offers the part before: %v", text, out)
			}
			continue
		}
		offset := int(out["offset"].(float64))
		if out["partial"] != true || offset <= 0 {
			t.Fatalf("%q: %v", text, out)
		}
		part := textToolsCall(t, a, a.token, map[string]any{"action": "run", "tool": c.tool, "text": text[:offset]})
		if part["ok"] != true || part["changed"] != true {
			t.Fatalf("the part before %q: %v", text[:offset], part)
		}
		got := c.content[:c.start] + part["text"].(string) + c.content[c.start+offset:]
		if got != c.want {
			t.Errorf("%q became %q, want %q", c.content, got, c.want)
		}
	}
}

// A disk-backed document: the decoder stops, changes nothing and names the
// line and column; the failure names the range before the problem, and the
// same tool over that range decodes it as one undo step. Everything after
// the problem keeps its bytes, line endings included.
func TestEditorDecodeThePartBeforeOnADiskDocument(t *testing.T) {
	encoded, plain, _ := settingsLine(9000)
	cases := []struct {
		name, tool, file string
		from, to         int
		partial          map[string]any
		saved            string
	}{
		{"a one-line file whose Base64 is followed by &&", "base64-decode", encoded + "&&", 0, 0,
			map[string]any{"from": 0, "to": len(encoded), "line": 1, "column": len(encoded) + 1, "scope": "document"}, plain + "&&"},
		{"a selection with a character in the middle", "base64-decode", "header\r\naGVsbG8gd29y#bGQ=\nfooter\r\n", 7, 24,
			map[string]any{"from": 7, "to": 19, "line": 2, "column": 13, "scope": "selection"}, "header\r\nhello wor#bGQ=\nfooter\r\n"},
		{"Hex to text", "hex-decode", "68656c6c6f;\r\nrest\n", 0, 0,
			map[string]any{"from": 0, "to": 10, "line": 1, "column": 11, "scope": "document"}, "hello;\r\nrest\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, m, _ := editorFixture(t)
			path := filepath.Join(t.TempDir(), "settings.txt")
			os.WriteFile(path, []byte(c.file), 0600)
			lease, status, code := openAnyFile(t, a, path)
			if code != 200 {
				t.Fatal(code)
			}
			defer m.releaseDocument("", editorFixtureUID, lease)
			revision := status.Metadata.Revision
			state := runTool(t, a, lease, map[string]any{"revision": revision, "tool": c.tool, "from": c.from, "to": c.to})
			message, _ := state["error"].(string)
			if state["state"] != "failed" || !strings.Contains(message, " is not a") || !strings.Contains(message, fmt.Sprintf("(line %d, column %d) Nothing was changed.", c.partial["line"], c.partial["column"])) {
				t.Fatal(state)
			}
			want, _ := json.Marshal(c.partial)
			if got, _ := json.Marshal(state["partial"]); string(got) != string(want) {
				t.Fatalf("partial %s, want %s", got, want)
			}
			if w := windowText(t, a, editorFixtureUID, lease, 0, 16); w.Revision != revision || w.Dirty {
				t.Fatal("the failed tool changed the document")
			}
			partial := state["partial"].(map[string]any)
			state = runTool(t, a, lease, map[string]any{"revision": revision, "tool": c.tool, "from": partial["from"], "to": partial["to"]})
			if state["state"] != "done" || state["changed"] != true || state["from"] != partial["from"] {
				t.Fatal(state)
			}
			meta := state["metadata"].(map[string]any)
			if r := diskCall(a, a.token, lease, map[string]any{"action": "save", "revision": meta["revision"], "format": status.Format}); r.Code != 200 {
				t.Fatal(r.Body.String())
			}
			if raw, _ := os.ReadFile(path); string(raw) != c.saved {
				t.Fatalf("saved %q..., want %q...", raw[:min(len(raw), 80)], c.saved[:min(len(c.saved), 80)])
			}
			// One undo step gives the text back as it was.
			w := windowText(t, a, editorFixtureUID, lease, 0, 16)
			if r := diskCall(a, a.token, lease, map[string]any{"action": "undo", "revision": w.Revision}); r.Code != 200 {
				t.Fatal(r.Body.String())
			}
			w = windowText(t, a, editorFixtureUID, lease, 0, 16)
			if r := diskCall(a, a.token, lease, map[string]any{"action": "save", "revision": w.Revision, "format": status.Format}); r.Code != 200 {
				t.Fatal(r.Body.String())
			}
			if raw, _ := os.ReadFile(path); string(raw) != c.file {
				t.Fatal("undo did not give the document back")
			}
		})
	}
	// Nothing before the problem, or an incomplete group: no part is offered.
	for _, file := range []string{"&&QUJD", "QUJDR&&"} {
		a, m, _ := editorFixture(t)
		path := filepath.Join(t.TempDir(), "other.txt")
		os.WriteFile(path, []byte(file), 0600)
		lease, status, code := openAnyFile(t, a, path)
		if code != 200 {
			t.Fatal(code)
		}
		if state := runTool(t, a, lease, map[string]any{"revision": status.Metadata.Revision, "tool": "base64-decode"}); state["state"] != "failed" || state["partial"] != nil {
			t.Errorf("%q: %v", file, state)
		}
		m.releaseDocument("", editorFixtureUID, lease)
	}
}

// A single Replace of a selected match expands a regular expression's Replace
// text with that match's groups.
func TestEditorReplaceTextOfASelectedMatch(t *testing.T) {
	f := openMixed(t, "host=a\r\nport=22\n")
	result := f.call(map[string]any{"action": "replace-text", "revision": f.revision, "query": `(\w+)=(\d+)`, "replacement": `$2\t$1`, "regex": true, "matchCase": true, "from": 7, "to": 14})
	if result["found"] != true || result["text"] != "22\tport" {
		t.Fatal(result)
	}
	if result = f.call(map[string]any{"action": "replace-text", "revision": f.revision, "query": `(\w+)=(\d+)`, "replacement": "x", "regex": true, "from": 0, "to": 6}); result["found"] != false {
		t.Fatal("a selection that is not a match was replaced: ", result)
	}
}
