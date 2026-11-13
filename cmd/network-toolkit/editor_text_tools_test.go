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
	export, _ := sonicwallFixture(2)
	if out = textToolsCall(t, a, a.token, map[string]any{"action": "detect", "text": export[:200]}); out["sonicwall"] != true {
		t.Fatal(out)
	}
	if out = textToolsCall(t, a, a.token, map[string]any{"action": "detect", "text": "hello"}); out["sonicwall"] != false {
		t.Fatal(out)
	}
	// Unknown fields are refused like every editor request.
	if r := editorHTTP(a, a.token, "/api/editor-views/text-tools", map[string]any{"action": "run", "tool": "upper", "text": "x", "path": "/etc/passwd"}); r.Code != 400 {
		t.Fatal(r.Code)
	}
}

// A fictional SonicWall export: Base64 of key=value& settings.
func sonicwallFixture(settings int) (string, string) {
	var plain, readable strings.Builder
	add := func(key, value, shown string) {
		plain.WriteString(key + "=" + value + "&")
		readable.WriteString(key + "=" + shown + "\n")
	}
	add("checksumVersion", "1", "1")
	add("buildNum", "7.1.3-0000-R0000", "7.1.3-0000-R0000")
	add("shortProdName", "TZ%20370", "TZ 370")
	for i := 0; i < settings; i++ {
		add(fmt.Sprintf("addrObjId_%d", i), fmt.Sprintf("LAN%%20Subnets%%20%d", i), fmt.Sprintf("LAN Subnets %d", i))
		add(fmt.Sprintf("addrObjIp1_%d", i), fmt.Sprintf("192.0.2.%d", i%250), fmt.Sprintf("192.0.2.%d", i%250))
		add(fmt.Sprintf("schedObjType_%d", i), "a+b%3ac", "a+b:c")
		add(fmt.Sprintf("uuid_InternalInstanceId_%d", i), "x%0Ay", `x\ny`)
	}
	add("checksum", "0000abcd", "0000abcd")
	return base64.StdEncoding.EncodeToString([]byte(plain.String())), readable.String()
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
	// The readable copy of an export never replaces the export.
	if r = diskCall(a, a.token, lease, map[string]any{"action": "tool-start", "revision": revision + 2, "tool": "sonicwall-readable"}); r.Code != 409 {
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

// The readable copy of a SonicWall export is a new untitled document filled
// by the engine from the export's buffer; the export does not change.
func TestEditorSonicWallReadableCopy(t *testing.T) {
	a, m, _ := editorFixture(t)
	export, readable := sonicwallFixture(9000)
	path := filepath.Join(t.TempDir(), "firewall.exp")
	os.WriteFile(path, []byte(export+"&&"), 0600) // a real export ends with "&&"
	lease, status, code := openAnyFile(t, a, path)
	if code != 200 {
		t.Fatal(code)
	}
	defer m.releaseDocument("", editorFixtureUID, lease)
	if r := diskCall(a, a.token, lease, map[string]any{"action": "tool-detect"}); r.Code != 200 || !strings.Contains(r.Body.String(), `"sonicwall":true`) {
		t.Fatal(r.Body.String())
	}
	copySnapshot := emptyDiskFixture()
	copySnapshot.UID, copySnapshot.ID, copySnapshot.Path = editorFixtureUID2, "untitled:readable", "firewall.exp (readable).txt"
	derive := map[string]any{"uid": editorFixtureUID, "lease": lease, "tool": "sonicwall-readable", "revision": status.Metadata.Revision}
	// Another window's lease, or a missing source, opens nothing.
	if r := editorHTTP(a, a.token, "/api/editor-views/buffers/open", map[string]any{"document": copySnapshot, "mode": "derive", "derive": map[string]any{"uid": editorFixtureUID, "lease": lease + 5, "tool": "sonicwall-readable", "revision": 1}}); r.Code != 409 {
		t.Fatal(r.Code, r.Body.String())
	}
	r := editorHTTP(a, a.token, "/api/editor-views/buffers/open", map[string]any{"document": copySnapshot, "mode": "derive", "derive": derive})
	if r.Code != 202 {
		t.Fatal(r.Code, r.Body.String())
	}
	var opened struct {
		Lease uint64 `json:"lease"`
	}
	json.Unmarshal(r.Body.Bytes(), &opened)
	defer m.releaseDocument("", editorFixtureUID2, opened.Lease)
	deadline := time.Now().Add(20 * time.Second)
	var result map[string]any
	for {
		r = editorHTTP(a, a.token, "/api/editor-views/documents/"+editorFixtureUID2+"/buffer", map[string]any{"action": "status", "lease": opened.Lease})
		if r.Code == 200 {
			json.Unmarshal(r.Body.Bytes(), &result)
			break
		}
		if r.Code != 202 || time.Now().After(deadline) {
			t.Fatal(r.Code, r.Body.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
	meta := result["metadata"].(map[string]any)
	if result["derived"].(float64) != 3+4*9000+1 || meta["dirty"] != true || meta["units"].(float64) != float64(len(readable)) {
		t.Fatal(result)
	}
	var got strings.Builder
	for start := 0; start < len(readable); {
		w := windowText(t, a, editorFixtureUID2, opened.Lease, start, editorbuffer.MaxWindowUnits)
		got.WriteString(w.Text)
		start += len(w.Text)
	}
	if got.String() != readable {
		t.Fatal("the readable copy differs")
	}
	if w := windowText(t, a, editorFixtureUID, lease, 0, 64); w.Revision != status.Metadata.Revision || w.Dirty || w.Text != export[:64] {
		t.Fatal("the export changed")
	}
	if raw, _ := os.ReadFile(path); string(raw) != export+"&&" {
		t.Fatal("the export changed on disk")
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
