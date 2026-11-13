package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"networktoolkit/internal/editorbuffer"
)

// Every command that replaces a range of a file whose lines end in different
// ways goes through the real handlers here, is saved, and the bytes of the
// lines it did not change are compared with the bytes that were read.

type mixedFile struct {
	t        *testing.T
	a        *application
	path     string
	lease    uint64
	revision uint64
	format   editorbuffer.Format
}

func openMixed(t *testing.T, raw string) *mixedFile {
	t.Helper()
	a, m, _ := editorFixture(t)
	path := filepath.Join(t.TempDir(), "device.cfg")
	if e := os.WriteFile(path, []byte(raw), 0600); e != nil {
		t.Fatal(e)
	}
	lease, status, code := openAnyFile(t, a, path)
	if code != 200 || status.Format.EOL != "Mixed" {
		t.Fatalf("%d %+v", code, status.Format)
	}
	t.Cleanup(func() { m.releaseDocument("", editorFixtureUID, lease) })
	return &mixedFile{t, a, path, lease, status.Metadata.Revision, status.Format}
}

func (f *mixedFile) call(body map[string]any) map[string]any {
	f.t.Helper()
	r := diskCall(f.a, f.a.token, f.lease, body)
	if r.Code != 200 {
		f.t.Fatalf("%v: %d %s", body["action"], r.Code, r.Body.String())
	}
	var result map[string]any
	json.Unmarshal(r.Body.Bytes(), &result)
	meta, _ := result["metadata"].(map[string]any)
	if meta == nil {
		meta = result
	}
	if revision, ok := meta["revision"].(float64); ok {
		f.revision = uint64(revision)
	}
	return result
}

func (f *mixedFile) saved() string {
	f.t.Helper()
	f.call(map[string]any{"action": "save", "revision": f.revision, "format": f.format})
	raw, _ := os.ReadFile(f.path)
	return string(raw)
}

func (f *mixedFile) text() string {
	f.t.Helper()
	return f.call(map[string]any{"action": "window"})["text"].(string)
}

func TestEditorReplaceAllKeepsMixedLineEndings(t *testing.T) {
	f := openMixed(t, "host A\r\nhost B\nhost C\r\nend\r\n")
	result := f.call(map[string]any{"action": "replace-all", "revision": f.revision, "query": "host C", "replacement": "host Z", "matchCase": true})
	if result["count"].(float64) != 1 {
		t.Fatal(result)
	}
	if got, want := f.saved(), "host A\r\nhost B\nhost Z\r\nend\r\n"; got != want {
		t.Errorf("Replace All of one word saved %q, want %q", got, want)
	}
	// One undo step takes all of it back.
	f.call(map[string]any{"action": "undo", "revision": f.revision})
	if got, want := f.saved(), "host A\r\nhost B\nhost C\r\nend\r\n"; got != want {
		t.Errorf("after undo %q, want %q", got, want)
	}
	if strings.Contains(f.text(), "host Z") {
		t.Error("undo left a replacement")
	}
}

func (f *mixedFile) edit(from, to int, text string) {
	f.t.Helper()
	f.call(map[string]any{"action": "edit", "revision": f.revision, "from": from, "to": to, "text": text})
}

// bulk is how the page sends a replacement too large for one request.
func (f *mixedFile) bulk(from, to int, text string) {
	f.t.Helper()
	id := "bulk-" + strings.Repeat("x", len(text)%7) + text[:min(len(text), 8)]
	id = strings.Map(func(r rune) rune {
		if r == '\n' || r == ' ' {
			return '_'
		}
		return r
	}, id)
	r := diskCall(f.a, f.a.token, f.lease, map[string]any{"action": "edit-begin", "requestId": id, "revision": f.revision, "from": from, "to": to})
	if r.Code != 200 {
		f.t.Fatal(r.Body.String())
	}
	half := len(text) / 2
	for _, part := range [][2]int{{0, half}, {half, len(text)}} {
		if r = diskCall(f.a, f.a.token, f.lease, map[string]any{"action": "edit-chunk", "requestId": id, "offset": part[0], "text": text[part[0]:part[1]]}); r.Code != 200 {
			f.t.Fatal(r.Body.String())
		}
	}
	f.call(map[string]any{"action": "edit-commit", "requestId": id})
}

// textTool runs a text tool of the Tools menu in the engine and waits for it
// (editor_text_tools.go). from == to means the whole document.
func (f *mixedFile) textTool(id string, options map[string]any, from, to int) map[string]any {
	f.t.Helper()
	started := f.call(map[string]any{"action": "tool-start", "revision": f.revision, "tool": id, "options": options, "from": from, "to": to})
	deadline := time.Now().Add(10 * time.Second)
	for {
		state := f.call(map[string]any{"action": "tool-status", "job": started["job"]})
		switch state["state"] {
		case "done":
			return state
		case "failed":
			f.t.Fatalf("%s: %v", id, state["error"])
		}
		if time.Now().After(deadline) {
			f.t.Fatalf("%s did not finish", id)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// The commands of the page that rewrite a range (web/assets/file-editor.js
// transformDiskSelection, the disk view's replace, Replace All, insert from
// the Terminal, the text tools of the Tools menu) reach the engine as one of
// four requests: an edit of the range, a bulk edit of the range,
// replace-all, or a text tool (tool-start). Each row is
// one command on a file whose five lines end CRLF, LF, CR, CRLF, CRLF; the
// saved bytes show which endings it kept.
func TestEditorRangeCommandsKeepMixedLineEndings(t *testing.T) {
	const raw = "one\r\ntwo\nthree\rfour\r\nfive\r\n"
	// The editing text is "one\ntwo\nthree\nfour\nfive\n"; lines two to four are 4..18.
	lines := "two\nthree\nfour"
	cases := []struct {
		name string
		run  func(f *mixedFile)
		want string
	}{
		{"UPPERCASE of a selection across lines", func(f *mixedFile) { f.edit(4, 18, strings.ToUpper(lines)) }, "one\r\nTWO\nTHREE\rFOUR\r\nfive\r\n"},
		{"lowercase that changes nothing", func(f *mixedFile) { f.edit(4, 18, lines) }, raw},
		{"Indent lines", func(f *mixedFile) { f.edit(4, 18, "    two\n    three\n    four") }, "one\r\n    two\n    three\r    four\r\nfive\r\n"},
		{"Trim trailing spaces (nothing to trim on the middle line)", func(f *mixedFile) { f.edit(4, 18, "two\nthree\nfour") }, raw},
		{"Toggle comment", func(f *mixedFile) { f.edit(4, 18, "# two\n# three\n# four") }, "one\r\n# two\n# three\r# four\r\nfive\r\n"},
		{"Sort lines: the lines move, each place keeps its ending", func(f *mixedFile) { f.edit(4, 18, "four\nthree\ntwo") }, "one\r\nfour\nthree\rtwo\r\nfive\r\n"},
		{"Duplicate lines: the copy is new text", func(f *mixedFile) { f.edit(4, 18, lines+"\n"+lines) }, "one\r\ntwo\nthree\rfour\r\ntwo\r\nthree\r\nfour\r\nfive\r\n"},
		{"Replace one match", func(f *mixedFile) { f.edit(8, 13, "3") }, "one\r\ntwo\n3\rfour\r\nfive\r\n"},
		{"Replace All, two matches on lines that end differently", func(f *mixedFile) {
			f.call(map[string]any{"action": "replace-all", "revision": f.revision, "query": "t", "replacement": "T", "matchCase": true})
		}, "one\r\nTwo\nThree\rfour\r\nfive\r\n"},
		{"Replace All with a regular expression at every line start", func(f *mixedFile) {
			f.call(map[string]any{"action": "replace-all", "revision": f.revision, "query": "(?m)^", "replacement": "> ", "regex": true})
		}, "> one\r\n> two\n> three\r> four\r\n> five\r\n> "},
		{"Replace All that removes line breaks: only those lines join", func(f *mixedFile) {
			f.call(map[string]any{"action": "replace-all", "revision": f.revision, "query": "two\n", "replacement": "", "matchCase": true})
		}, "one\r\nthree\rfour\r\nfive\r\n"},
		{"Insert text from the Terminal at a line", func(f *mixedFile) { f.edit(8, 8, "show run\n") }, "one\r\ntwo\nshow run\r\nthree\rfour\r\nfive\r\n"},
		{"Convert text: a selection becomes Base64 (one line for two)", func(f *mixedFile) { f.edit(4, 13, "dHdvCnRocmVl") }, "one\r\ndHdvCnRocmVl\rfour\r\nfive\r\n"},
		{"Paste over the whole document, as many lines", func(f *mixedFile) { f.edit(0, 24, "1\n2\n3\n4\n5\n") }, "1\r\n2\n3\r4\r\n5\r\n"},
		{"Paste over the whole document, other lines: all of it is new text", func(f *mixedFile) { f.edit(0, 24, "x\ny\n") }, "x\r\ny\r\n"},
		{"A large paste over lines (bulk edit), as many lines", func(f *mixedFile) { f.bulk(4, 18, "TWO\nTHREE\nFOUR") }, "one\r\nTWO\nTHREE\rFOUR\r\nfive\r\n"},
		{"A large paste (bulk edit) of more lines: the pasted lines are new text", func(f *mixedFile) { f.bulk(4, 18, "a\nb\nc\nd") }, "one\r\na\r\nb\r\nc\r\nd\r\nfive\r\n"},
		// The Tools menu (editor_text_tools.go): a tool keeps what it does
		// not change, so the lines it leaves keep their endings.
		{"Tools: UPPERCASE of a selection across lines", func(f *mixedFile) { f.textTool("upper", nil, 4, 18) }, "one\r\nTWO\nTHREE\rFOUR\r\nfive\r\n"},
		{"Tools: Title Case of the whole document", func(f *mixedFile) { f.textTool("title", nil, 0, 0) }, "One\r\nTwo\nThree\rFour\r\nFive\r\n"},
		{"Tools: Sort A→Z, each place keeps its ending", func(f *mixedFile) { f.textTool("sort-asc", nil, 4, 18) }, "one\r\nfour\nthree\rtwo\r\nfive\r\n"},
		{"Tools: Reverse line order, each place keeps its ending", func(f *mixedFile) { f.textTool("reverse", nil, 6, 16) }, "one\r\nfour\nthree\rtwo\r\nfive\r\n"},
		{"Tools: Trim trailing spaces with nothing to trim", func(f *mixedFile) { f.textTool("trim-trailing", nil, 0, 0) }, raw},
		{"Tools: Remove duplicate lines with none to remove", func(f *mixedFile) { f.textTool("dedupe", nil, 0, 0) }, raw},
		{"Tools: Split at text, the new line break is new text", func(f *mixedFile) {
			f.textTool("split", map[string]any{"separator": "h"}, 4, 18)
		}, "one\r\ntwo\nt\r\nree\rfour\r\nfive\r\n"},
		{"Tools: Join lines, only the joined breaks go", func(f *mixedFile) {
			f.textTool("join", map[string]any{"separator": " "}, 4, 18)
		}, "one\r\ntwo three four\r\nfive\r\n"},
		{"Tools: Base64 encode of a selection (one line for two)", func(f *mixedFile) { f.textTool("base64-encode", nil, 4, 13) }, "one\r\ndHdvCnRocmVl\rfour\r\nfive\r\n"},
		{"Tools: Tabs to spaces with no tabs", func(f *mixedFile) { f.textTool("tabs-to-spaces", map[string]any{"indent": 4}, 0, 0) }, raw},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := openMixed(t, raw)
			c.run(f)
			shown := f.text()
			if got := f.saved(); got != c.want {
				t.Errorf("saved %q, want %q", got, c.want)
			}
			// Whatever was saved reopens as the text the editor showed.
			again := openMixed2(t, f.path)
			if again != shown {
				t.Errorf("the saved file reopens as %q, the editor showed %q", again, shown)
			}
			// One undo step takes the whole command back, endings included.
			if f.revision > 1 {
				f.call(map[string]any{"action": "undo", "revision": f.revision})
				if got := f.saved(); got != raw {
					t.Errorf("after undo %q, want the file as it was read", got)
				}
			}
		})
	}
}

// openMixed2 reads a saved file again in a fresh engine and returns its text.
func openMixed2(t *testing.T, path string) string {
	t.Helper()
	a, m, _ := editorFixture(t)
	lease, _, code := openAnyFile(t, a, path)
	if code != 200 {
		t.Fatal(code)
	}
	defer m.releaseDocument("", editorFixtureUID, lease)
	r := diskCall(a, a.token, lease, map[string]any{"action": "window"})
	var window editorbuffer.Window
	json.Unmarshal(r.Body.Bytes(), &window)
	return window.Text
}

// Enter after a line that ends in a lone CR: Save, Save As, an export and a
// reload all give the two line breaks the editor shows.
func TestEditorSavesALineBreakTypedAfterALoneCR(t *testing.T) {
	f := openMixed(t, "a\rb\nc\nd\n")
	f.edit(2, 2, "\n")
	shown := f.text()
	if shown != "a\n\nb\nc\nd\n" {
		t.Fatal(shown)
	}
	r := diskCall(f.a, f.a.token, f.lease, map[string]any{"action": "export", "revision": f.revision, "format": f.format})
	if r.Code != 200 || r.Body.String() != "a\r\r\nb\nc\nd\n" || r.Header().Get("Content-Length") != "10" {
		t.Fatalf("export: %d %q length %s", r.Code, r.Body.String(), r.Header().Get("Content-Length"))
	}
	copyPath := filepath.Join(filepath.Dir(f.path), "copy.cfg")
	f.call(map[string]any{"action": "save", "revision": f.revision, "path": copyPath, "format": f.format})
	if raw, _ := os.ReadFile(copyPath); string(raw) != "a\r\r\nb\nc\nd\n" {
		t.Fatalf("Save As wrote %q", raw)
	}
	if again := openMixed2(t, copyPath); again != shown {
		t.Fatalf("Save As reopens as %q, the editor showed %q", again, shown)
	}
	f.call(map[string]any{"action": "reload"})
	if again := f.text(); again != shown {
		t.Fatalf("reload shows %q, the editor showed %q", again, shown)
	}
}

// The export of a document that needs a counting pass (Windows-1252, lines
// with their own endings) stops when the request is cancelled.
func TestEditorExportCountingPassIsCancelled(t *testing.T) {
	v, e := editorbuffer.Import(t.Context(), strings.NewReader(strings.Repeat("line\r\nline\n", 200000)), t.TempDir(), nil)
	if e != nil {
		t.Fatal(e)
	}
	defer v.Buffer.Close()
	s, _ := v.Buffer.Snapshot()
	defer s.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, e = s.PrepareEncoding(ctx, v.Format, nil); !errors.Is(e, context.Canceled) {
		t.Fatal("a cancelled export still counted the whole document: ", e)
	}
	if count, _, e := s.PrepareEncoding(t.Context(), v.Format, nil); e != nil || count != 200000*11 {
		t.Fatal(count, e)
	}
}

// A file that is not text, opened from a browser file handle, can be read
// again from that handle and the reading can be cancelled; nothing of it is
// kept for the session.
func TestEditorBinaryFromABrowserHandleReloadsAndIsNeverKept(t *testing.T) {
	a, m, _ := editorFixture(t)
	r := editorHTTP(a, a.token, "/api/editor-views/buffers/open", map[string]any{"document": emptyDiskFixture(), "mode": "upload"})
	if r.Code != 202 {
		t.Fatal(r.Code, r.Body)
	}
	defer m.releaseDocument("", editorFixtureUID, 1)
	call := func(action string, fields map[string]any) (int, string) {
		if fields == nil {
			fields = map[string]any{}
		}
		fields["action"] = action
		r := diskCall(a, a.token, 1, fields)
		return r.Code, r.Body.String()
	}
	program := []byte("MZ\x90\x00\x03\x00\x00\x00binary")
	upload := func(raw []byte) {
		t.Helper()
		if code, body := call("import-chunk", map[string]any{"offset": 0, "data": raw}); code != 200 {
			t.Fatal(code, body)
		}
		if code, body := call("import-finish", map[string]any{"size": len(raw)}); code != 202 {
			t.Fatal(code, body)
		}
	}
	upload(program)
	if r = waitDiskReady(t, a, 1); !strings.Contains(r.Body.String(), `"encoding":"Binary"`) {
		t.Fatal(r.Body)
	}
	// Read it again from the handle, and cancel a reading that has started.
	if code, body := call("reload-start", map[string]any{"revision": 1}); code != 200 {
		t.Fatal("reload-start: ", code, body)
	}
	if code, body := call("import-chunk", map[string]any{"offset": 0, "data": program[:4]}); code != 200 {
		t.Fatal("import-chunk: ", code, body)
	}
	if code, body := call("cancel", nil); code != 200 {
		t.Fatal("cancel: ", code, body)
	}
	if r = waitDiskReady(t, a, 1); !strings.Contains(r.Body.String(), "Browser file reload cancelled.") || !strings.Contains(r.Body.String(), `"encoding":"Binary"`) {
		t.Fatal(r.Body)
	}
	if code, body := call("reload-start", map[string]any{"revision": 1}); code != 200 {
		t.Fatal(code, body)
	}
	upload([]byte("now it is text\n"))
	if r = waitDiskReady(t, a, 1); !strings.Contains(r.Body.String(), `"encoding":"UTF-8"`) {
		t.Fatal("the reloaded text file is still treated as not text: ", r.Body)
	}
	if code, body := call("reload-start", map[string]any{"revision": 1}); code != 200 {
		t.Fatal(code, body)
	}
	upload(program)
	if r = waitDiskReady(t, a, 1); !strings.Contains(r.Body.String(), `"encoding":"Binary"`) {
		t.Fatal(r.Body)
	}
	// The session can never keep a copy of it, whatever the page asks for.
	if _, _, e := m.sessionSnapshot(editorFixtureUID, 1); !errors.Is(e, editorbuffer.ErrBinary) {
		t.Fatal("a snapshot for a kept copy of a file that is not text: ", e)
	}
	keep := requestApp(a, http.MethodPost, editorSessionEndpoint+"/backup", sessionJSON(t, map[string]any{"source": "buffer", "uid": editorFixtureUID, "lease": 1}), true)
	if keep.Code == 200 || bytes.Contains(keep.Body.Bytes(), []byte(`"sha256"`)) {
		t.Fatal("the session kept a copy of a file that is not text: ", keep.Code, keep.Body.String())
	}
}

// hookContext runs a function the first time the search asks whether it was
// cancelled: the moment "in the middle of a Replace All".
type hookContext struct {
	context.Context
	once sync.Once
	hook func()
}

func (c *hookContext) Err() error { c.once.Do(c.hook); return c.Context.Err() }

func finishes(t *testing.T, what string, work func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); work() }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("%s did not finish within 10 seconds", what)
	}
}

// The buffer is closed while Replace All searches (the tab is closed, or a
// reload replaced the buffer). The request must return, and the document's
// edit lock must be free again: otherwise no later edit, undo or close of
// that document ever returns.
func TestEditorReplaceAllReturnsWhenItsBufferIsClosed(t *testing.T) {
	v, e := editorbuffer.Import(t.Context(), strings.NewReader("foo bar foo"), t.TempDir(), nil)
	if e != nil {
		t.Fatal(e)
	}
	d := &editorDiskDocument{}
	closed := make(chan struct{})
	ctx := &hookContext{Context: t.Context(), hook: func() {
		go func() { v.Buffer.Close(); close(closed) }()
		time.Sleep(300 * time.Millisecond) // Close is now waiting for the search
	}}
	finishes(t, "Replace All on a buffer that is being closed", func() {
		_, e := d.replaceDiskMatches(ctx, v.Buffer, editorDiskRequest{Revision: v.Buffer.Metadata().Revision, Query: "foo", Replace: "x", MatchCase: true})
		if e != nil && !errors.Is(e, editorbuffer.ErrClosed) {
			t.Errorf("replace-all: %v", e)
		}
	})
	finishes(t, "closing the buffer", func() { <-closed })
	if !d.editMu.TryLock() {
		t.Fatal("the document's edit lock is still held")
	}
	d.editMu.Unlock()
}

// A reload replaces the buffer. It stops a Replace All that is still
// searching the old one instead of waiting for it, and nothing of that
// Replace All is applied.
func TestEditorReloadStopsAReplaceAllInFlight(t *testing.T) {
	v, e := editorbuffer.Import(t.Context(), strings.NewReader(strings.Repeat("foo bar\n", 4000)), t.TempDir(), nil)
	if e != nil {
		t.Fatal(e)
	}
	defer v.Buffer.Close()
	d := &editorDiskDocument{}
	ctx := &hookContext{Context: t.Context(), hook: func() { d.stopReplaceAll() }}
	finishes(t, "a Replace All stopped by a reload", func() {
		_, e := d.replaceDiskMatches(ctx, v.Buffer, editorDiskRequest{Revision: 1, Query: "foo", Replace: "x", MatchCase: true})
		if !errors.Is(e, context.Canceled) {
			t.Errorf("replace-all after the reload stopped it: %v", e)
		}
	})
	if meta := v.Buffer.Metadata(); meta.Revision != 1 || meta.Dirty {
		t.Fatalf("a stopped Replace All changed the document: %+v", meta)
	}
	d.mu.Lock()
	left := len(d.replacing)
	d.mu.Unlock()
	if left != 0 {
		t.Fatal("a finished Replace All is still registered")
	}
	// The next one runs normally.
	result, e := d.replaceDiskMatches(t.Context(), v.Buffer, editorDiskRequest{Revision: 1, Query: "foo", Replace: "x", MatchCase: true})
	if e != nil || result.(map[string]any)["count"].(int64) != 4000 {
		t.Fatal(result, e)
	}
}

// Through the real handlers: a reload and a Replace All of the same large
// document at the same time both answer, whichever comes first.
func TestEditorReloadAndReplaceAllTogetherBothAnswer(t *testing.T) {
	f := openMixed(t, strings.Repeat("interface a\r\n shutdown\n", 150000))
	codes := make(chan int, 2)
	finishes(t, "a reload during a Replace All", func() {
		var wait sync.WaitGroup
		for _, body := range []map[string]any{
			{"action": "replace-all", "revision": f.revision, "query": "shutdown", "replacement": "no shutdown", "matchCase": true},
			{"action": "reload"},
		} {
			wait.Add(1)
			go func() {
				defer wait.Done()
				codes <- diskCall(f.a, f.a.token, f.lease, body).Code
			}()
			time.Sleep(20 * time.Millisecond)
		}
		wait.Wait()
	})
	for i := 0; i < 2; i++ {
		if code := <-codes; code != 200 && code != 409 {
			t.Fatal("unexpected answer: ", code)
		}
	}
	// The document still answers and can be edited.
	status := f.call(map[string]any{"action": "status"})
	f.revision = uint64(status["metadata"].(map[string]any)["revision"].(float64))
	f.edit(0, 0, "x")
	if !strings.HasPrefix(f.text(), "x") {
		t.Fatal("the document cannot be edited after a reload met a Replace All")
	}
}
