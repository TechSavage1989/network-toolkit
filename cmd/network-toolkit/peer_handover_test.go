package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// Tests of the document hand-over: the main Toolkit window's page asks its own
// engine (/api/peer-handover/) to give tabs to the running standalone Text
// Editor of the same data root (/api/peer/documents/). The source keeps every
// tab until the receiving page confirmed it, per tab.

// peerHandoverFile writes a local file and returns its path and digest.
func peerHandoverFile(t *testing.T, dir, name, text string) (string, string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return path, editorDigest([]byte(text))
}

func peerHandoverJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

var peerHandoverLF = &peerHandoverFormat{Encoding: "UTF-8", EOL: "LF"}

func peerHandoverAbsolute(name string) string {
	if runtime.GOOS == "windows" {
		return `C:\Users\Example\` + name
	}
	return "/home/example/" + name
}

func TestPeerHandoverValidation(t *testing.T) {
	path := peerHandoverAbsolute("notes.txt")
	hash := strings.Repeat("a", 64)
	good := []peerHandoverItem{
		{ID: "doc-1", Kind: "file", Path: path, Hash: hash},
		{ID: "doc-2", Kind: "file", Path: path + ".bak", Hash: hash, Dirty: true, Text: "edited\n", Format: &peerHandoverFormat{Encoding: "UTF-16LE", BOM: true, EOL: "CRLF"}, SavedFormat: peerHandoverLF, Language: "cisco"},
		{ID: "doc-3", Kind: "untitled", Name: "Untitled.txt", Dirty: true, Text: "draft", Format: peerHandoverLF},
		{ID: "doc-4", Kind: "untitled", Name: "blank.txt", Format: &peerHandoverFormat{Encoding: "UTF-8", EOL: "CR"}},
	}
	if err := validatePeerHandover(good); err != nil {
		t.Fatalf("valid items refused: %v", err)
	}
	unc, device := `\\server\share\notes.txt`, `\\?\C:\notes.txt`
	if runtime.GOOS != "windows" {
		unc, device = "//server/share/notes.txt", "//?/C:/notes.txt"
	}
	cp1252 := &peerHandoverFormat{Encoding: "Windows-1252", EOL: "LF"}
	mixed := &peerHandoverFormat{Encoding: "UTF-8", EOL: "Mixed"}
	binary := &peerHandoverFormat{Encoding: "Binary", EOL: "LF"}
	cases := map[string][]peerHandoverItem{
		"no items":            {},
		"relative path":       {{ID: "d", Kind: "file", Path: "notes.txt", Hash: hash}},
		"UNC path":            {{ID: "d", Kind: "file", Path: unc, Hash: hash}},
		"device path":         {{ID: "d", Kind: "file", Path: device, Hash: hash}},
		"parent element":      {{ID: "d", Kind: "file", Path: peerHandoverAbsolute("../x.txt"), Hash: hash}},
		"NUL in path":         {{ID: "d", Kind: "file", Path: path + "\x00", Hash: hash}},
		"control in path":     {{ID: "d", Kind: "file", Path: path + "\n", Hash: hash}},
		"file without hash":   {{ID: "d", Kind: "file", Path: path}},
		"bad hash":            {{ID: "d", Kind: "file", Path: path, Hash: "zz"}},
		"clean with text":     {{ID: "d", Kind: "file", Path: path, Hash: hash, Text: "x"}},
		"clean with format":   {{ID: "d", Kind: "file", Path: path, Hash: hash, Format: peerHandoverLF}},
		"file with name":      {{ID: "d", Kind: "file", Path: path, Hash: hash, Name: "x"}},
		"dirty no format":     {{ID: "d", Kind: "file", Path: path, Hash: hash, Dirty: true, Text: "x"}},
		"dirty no saved fmt":  {{ID: "d", Kind: "file", Path: path, Hash: hash, Dirty: true, Text: "x", Format: peerHandoverLF}},
		"Windows-1252":        {{ID: "d", Kind: "file", Path: path, Hash: hash, Dirty: true, Text: "x", Format: cp1252, SavedFormat: peerHandoverLF}},
		"saved Windows-1252":  {{ID: "d", Kind: "file", Path: path, Hash: hash, Dirty: true, Text: "x", Format: peerHandoverLF, SavedFormat: cp1252}},
		"Mixed endings":       {{ID: "d", Kind: "file", Path: path, Hash: hash, Dirty: true, Text: "x", Format: mixed, SavedFormat: peerHandoverLF}},
		"binary":              {{ID: "d", Kind: "file", Path: path, Hash: hash, Dirty: true, Text: "x", Format: binary, SavedFormat: binary}},
		"untitled path":       {{ID: "d", Kind: "untitled", Name: "x", Path: path, Format: peerHandoverLF}},
		"untitled no name":    {{ID: "d", Kind: "untitled", Format: peerHandoverLF}},
		"untitled folder":     {{ID: "d", Kind: "untitled", Name: "a/b.txt", Format: peerHandoverLF}},
		"untitled hash":       {{ID: "d", Kind: "untitled", Name: "x", Hash: hash, Format: peerHandoverLF}},
		"untitled no format":  {{ID: "d", Kind: "untitled", Name: "x"}},
		"untitled Mixed":      {{ID: "d", Kind: "untitled", Name: "x", Format: mixed}},
		"unknown kind":        {{ID: "d", Kind: "remote", Path: path, Hash: hash}},
		"bad identifier":      {{ID: "bad id", Kind: "file", Path: path, Hash: hash}},
		"duplicate":           {{ID: "d", Kind: "file", Path: path, Hash: hash}, {ID: "d", Kind: "file", Path: path + "2", Hash: hash}},
		"invalid UTF-8":       {{ID: "d", Kind: "untitled", Name: "x", Dirty: true, Text: "\xff", Format: peerHandoverLF}},
		"bad language":        {{ID: "d", Kind: "file", Path: path, Hash: hash, Language: "a b"}},
		"over 8 MiB of text":  {{ID: "d", Kind: "untitled", Name: "x", Dirty: true, Text: strings.Repeat("x", peerHandoverTextLimit+1), Format: peerHandoverLF}},
		"over 8 MiB together": {{ID: "d1", Kind: "untitled", Name: "x", Dirty: true, Text: strings.Repeat("x", peerHandoverTextLimit/2+1), Format: peerHandoverLF}, {ID: "d2", Kind: "untitled", Name: "y", Dirty: true, Text: strings.Repeat("x", peerHandoverTextLimit/2+1), Format: peerHandoverLF}},
	}
	many := make([]peerHandoverItem, peerHandoverItemLimit+1)
	for i := range many {
		many[i] = peerHandoverItem{ID: "d" + strings.Repeat("x", i+1), Kind: "untitled", Name: "x", Format: peerHandoverLF}
	}
	cases["too many tabs"] = many
	for name, items := range cases {
		if err := validatePeerHandover(items); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// Exactly 8 MiB of text in total is allowed.
	if err := validatePeerHandover([]peerHandoverItem{{ID: "d", Kind: "untitled", Name: "x", Dirty: true, Text: strings.Repeat("x", peerHandoverTextLimit), Format: peerHandoverLF}}); err != nil {
		t.Fatalf("8 MiB refused: %v", err)
	}
}

// peerHandoverPair starts a main window and a standalone Text Editor of one
// data root, both registered. The editor's page runs (it reports and polls).
func peerHandoverPair(t *testing.T) (root string, main, editor *application, collected <-chan map[string]any) {
	t.Helper()
	root = t.TempDir()
	main, _ = peerTestApp(t, root, "main")
	editor, _ = peerTestApp(t, root, standaloneEditorMode)
	peerRegister(t, main, root)
	peerRegister(t, editor, root)
	collected = peerLivePage(t, editor, peerOwnDocuments)
	return root, main, editor, collected
}

func peerHandoverAwaitReceiver(t *testing.T, main *application) map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		code, reply, raw := peerPage(t, main, http.MethodGet, "/api/peer-handover/receiver", "")
		if code == http.StatusOK && reply["available"] == true {
			return reply
		}
		if time.Now().After(deadline) {
			t.Fatalf("the standalone Text Editor never became available: %d %s", code, raw)
		}
		time.Sleep(40 * time.Millisecond)
	}
}

func peerHandoverAwait(t *testing.T, main *application, id string, final func(string) bool) map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		code, reply, raw := peerPage(t, main, http.MethodGet, "/api/peer-handover/result?requestId="+id, "")
		state, _ := reply["state"].(string)
		if code == http.StatusOK && final(state) {
			return reply
		}
		if time.Now().After(deadline) {
			t.Fatalf("hand-over result never settled: %d %s", code, raw)
		}
		time.Sleep(40 * time.Millisecond)
	}
}

func peerHandoverAck(t *testing.T, editor *application, body string) (int, string) {
	t.Helper()
	code, _, raw := peerPage(t, editor, http.MethodPost, "/api/peer-send/ack", body)
	return code, raw
}

func TestPeerHandoverConfirmThenRelease(t *testing.T) {
	root, main, editor, collected := peerHandoverPair(t)
	reply := peerHandoverAwaitReceiver(t, main)
	if reply["name"] != standaloneEditorTitle {
		t.Fatalf("receiver name: %v", reply)
	}
	files := t.TempDir()
	path, hash := peerHandoverFile(t, files, "running-config.txt", "hostname lab-switch-01\n")
	items := []peerHandoverItem{
		{ID: "doc-file", Kind: "file", Path: path, Hash: hash, Dirty: true, Text: "hostname lab-switch-02\n", Format: peerHandoverLF, SavedFormat: peerHandoverLF},
		{ID: "doc-clean", Kind: "file", Path: path, Hash: hash},
		{ID: "doc-draft", Kind: "untitled", Name: "Untitled.txt", Dirty: true, Text: "interface Gi0/1\n", Format: peerHandoverLF},
	}
	code, started, raw := peerPage(t, main, http.MethodPost, "/api/peer-handover/start", peerHandoverJSON(t, map[string]any{"items": items}))
	if code != http.StatusOK || started["state"] != peerQueued {
		t.Fatalf("start: %d %s", code, raw)
	}
	id, _ := started["requestId"].(string)
	if !peerRequestID.MatchString(id) || len(started["sent"].([]any)) != 3 || len(started["ineligible"].([]any)) != 0 {
		t.Fatalf("start reply: %s", raw)
	}
	for _, secret := range []string{main.token, main.windowActivationToken, editor.token, editor.windowActivationToken, root} {
		if strings.Contains(raw, secret) {
			t.Fatal("the start reply exposed a token or the data folder")
		}
	}
	peerRaised(t, editor, "the standalone Text Editor")
	// The receiving page gets the documents, file paths included, from its own engine.
	request := peerAwaitRequest(t, collected, "hand-over")
	if request["kind"] != peerHandoverKind || request["from"] != appName || request["requestId"] != id {
		t.Fatalf("receiving page got %v", request)
	}
	documents, _ := request["documents"].([]any)
	if len(documents) != 3 || documents[0].(map[string]any)["path"] != path || documents[0].(map[string]any)["text"] != "hostname lab-switch-02\n" {
		t.Fatalf("documents on the receiving page: %v", request)
	}
	// Before the page confirms, the source keeps its tabs: the result is "taken".
	if state := peerHandoverAwait(t, main, id, func(s string) bool { return s == peerTaken })["state"]; state != peerTaken {
		t.Fatalf("state before the confirmation: %v", state)
	}
	// A second hand-over is refused while one is waiting.
	if code, _, raw := peerPage(t, main, http.MethodPost, "/api/peer-handover/start", peerHandoverJSON(t, map[string]any{"items": items[2:]})); code != http.StatusConflict {
		t.Fatalf("second hand-over while one waits: %d %s", code, raw)
	}
	// The confirmation must name every tab that was sent, once.
	for name, body := range map[string]string{
		"missing tab":   `{"requestId":"` + id + `","ok":true,"outcome":"opened","items":[{"id":"doc-file","ok":true}]}`,
		"unknown tab":   `{"requestId":"` + id + `","ok":true,"outcome":"opened","items":[{"id":"doc-file","ok":true},{"id":"doc-clean","ok":true},{"id":"doc-other","ok":true}]}`,
		"duplicate tab": `{"requestId":"` + id + `","ok":true,"outcome":"opened","items":[{"id":"doc-file","ok":true},{"id":"doc-file","ok":true},{"id":"doc-draft","ok":true}]}`,
		"no items":      `{"requestId":"` + id + `","ok":true,"outcome":"opened"}`,
		"wrong outcome": `{"requestId":"` + id + `","ok":true,"outcome":"inserted","items":[{"id":"doc-file","ok":true},{"id":"doc-clean","ok":true},{"id":"doc-draft","ok":true}]}`,
	} {
		if code, raw := peerHandoverAck(t, editor, body); code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", name, code, raw)
		}
	}
	if code, raw := peerHandoverAck(t, editor, `{"requestId":"`+id+`","ok":true,"outcome":"opened","items":[{"id":"doc-file","ok":true},{"id":"doc-clean","ok":true},{"id":"doc-draft","ok":false,"error":"A dialog is open in Network Toolkit Text Editor."}]}`); code != http.StatusOK {
		t.Fatalf("confirmation: %d %s", code, raw)
	}
	result := peerHandoverAwait(t, main, id, func(s string) bool { return s == peerDone })
	got := map[string]bool{}
	for _, item := range result["items"].([]any) {
		entry := item.(map[string]any)
		got[entry["id"].(string)] = entry["ok"] == true
	}
	if len(got) != 3 || !got["doc-file"] || !got["doc-clean"] || got["doc-draft"] {
		t.Fatalf("per-tab result: %v", result)
	}
	// The hand-over is over: a new one may start.
	code, _, raw = peerPage(t, main, http.MethodPost, "/api/peer-handover/start", peerHandoverJSON(t, map[string]any{"items": items[2:]}))
	if code != http.StatusOK {
		t.Fatalf("next hand-over: %d %s", code, raw)
	}
}

func TestPeerHandoverKeepsTabsOnRefusalTimeoutAndCrash(t *testing.T) {
	draft := []peerHandoverItem{{ID: "doc-draft", Kind: "untitled", Name: "Untitled.txt", Dirty: true, Text: "draft", Format: peerHandoverLF}}
	start := func(t *testing.T, main *application) string {
		t.Helper()
		code, reply, raw := peerPage(t, main, http.MethodPost, "/api/peer-handover/start", peerHandoverJSON(t, map[string]any{"items": draft}))
		if code != http.StatusOK {
			t.Fatalf("start: %d %s", code, raw)
		}
		return reply["requestId"].(string)
	}
	t.Run("refused", func(t *testing.T) {
		_, main, editor, collected := peerHandoverPair(t)
		peerHandoverAwaitReceiver(t, main)
		id := start(t, main)
		peerAwaitRequest(t, collected, "refusal")
		if code, raw := peerHandoverAck(t, editor, `{"requestId":"`+id+`","ok":false,"error":"A dialog is open in Network Toolkit Text Editor. Finish or close it there, then send again."}`); code != http.StatusOK {
			t.Fatalf("refusal: %d %s", code, raw)
		}
		result := peerHandoverAwait(t, main, id, func(s string) bool { return s == peerFailed })
		if !strings.Contains(result["message"].(string), "A dialog is open") || len(result["items"].([]any)) != 0 {
			t.Fatalf("refusal result: %v", result)
		}
	})
	t.Run("timeout", func(t *testing.T) {
		_, main, _, collected := peerHandoverPair(t)
		peerHandoverAwaitReceiver(t, main)
		advance := peerTestClock(t)
		id := start(t, main)
		peerAwaitRequest(t, collected, "timeout")
		advance(peerRequestLifetime + time.Second)
		result := peerHandoverAwait(t, main, id, func(s string) bool { return s == peerExpired })
		if len(result["items"].([]any)) != 0 || result["message"] == "" {
			t.Fatalf("expired result: %v", result)
		}
		// The main window's own record of the hand-over ends with it.
		main.peer.mu.Lock()
		pending := main.peer.handover
		main.peer.mu.Unlock()
		if pending != nil {
			t.Fatal("an expired hand-over is still pending")
		}
	})
	t.Run("receiver closed", func(t *testing.T) {
		root := t.TempDir()
		main, _ := peerTestApp(t, root, "main")
		editor, server := peerTestApp(t, root, standaloneEditorMode)
		peerRegister(t, main, root)
		peerRegister(t, editor, root)
		peerLivePage(t, editor, peerOwnDocuments)
		peerHandoverAwaitReceiver(t, main)
		advance := peerTestClock(t)
		id := start(t, main)
		server.CloseClientConnections()
		server.Close()
		// One failed status check does not end the hand-over: the editor may
		// still answer. It ends as lost only at the overall deadline.
		if _, reply, raw := peerPage(t, main, http.MethodGet, "/api/peer-handover/result?requestId="+id, ""); reply["state"] != peerTaken {
			t.Fatalf("a failed status check ended the hand-over: %s", raw)
		}
		advance(peerHandoverStatusDeadline + time.Second)
		result := peerHandoverAwait(t, main, id, func(s string) bool { return s == "lost" })
		if len(result["items"].([]any)) != 0 || !strings.Contains(result["message"].(string), "closed") {
			t.Fatalf("lost result: %v", result)
		}
	})
	t.Run("a status check fails once", func(t *testing.T) {
		root := t.TempDir()
		main, _ := peerTestApp(t, root, "main")
		var mu sync.Mutex
		failed := 0
		editor, _ := peerTestAppWith(t, root, standaloneEditorMode, func(a *application) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				fail := strings.HasPrefix(r.URL.Path, peerInboundPrefix+peerHandoverKind+"/result") && failed == 0
				if fail {
					failed++
				}
				mu.Unlock()
				if fail {
					http.Error(w, "busy", http.StatusServiceUnavailable)
					return
				}
				a.ServeHTTP(w, r)
			})
		})
		peerRegister(t, main, root)
		peerRegister(t, editor, root)
		collected := peerLivePage(t, editor, peerOwnDocuments)
		peerHandoverAwaitReceiver(t, main)
		id := start(t, main)
		peerAwaitRequest(t, collected, "flaky status")
		if _, reply, raw := peerPage(t, main, http.MethodGet, "/api/peer-handover/result?requestId="+id, ""); reply["state"] != peerTaken {
			t.Fatalf("the failed check: %s", raw)
		}
		if code, raw := peerHandoverAck(t, editor, `{"requestId":"`+id+`","ok":true,"outcome":"opened","items":[{"id":"doc-draft","ok":true}]}`); code != http.StatusOK {
			t.Fatalf("ack: %d %s", code, raw)
		}
		result := peerHandoverAwait(t, main, id, func(s string) bool { return s == peerDone })
		if items := result["items"].([]any); len(items) != 1 {
			t.Fatalf("done result: %v", result)
		}
	})
	t.Run("the start reply is lost", func(t *testing.T) {
		root := t.TempDir()
		main, _ := peerTestApp(t, root, "main")
		editor, _ := peerTestAppWith(t, root, standaloneEditorMode, func(a *application) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, peerInboundPrefix+peerHandoverKind+"/handover") {
					// The editor took the request, but its answer never arrives.
					a.ServeHTTP(httptest.NewRecorder(), r)
					if hijacker, ok := w.(http.Hijacker); ok {
						if conn, _, err := hijacker.Hijack(); err == nil {
							conn.Close()
						}
					}
					return
				}
				a.ServeHTTP(w, r)
			})
		})
		peerRegister(t, main, root)
		peerRegister(t, editor, root)
		peerLivePage(t, editor, peerOwnDocuments)
		peerHandoverAwaitReceiver(t, main)
		code, _, raw := peerPage(t, main, http.MethodPost, "/api/peer-handover/start", peerHandoverJSON(t, map[string]any{"items": draft}))
		if code == http.StatusOK || !strings.Contains(raw, "The tabs stay here") || !strings.Contains(raw, "in case they also opened there") {
			t.Fatalf("a lost start reply: %d %s", code, raw)
		}
	})
}

func TestPeerHandoverRefusesAFileChangedOnDisk(t *testing.T) {
	_, main, editor, collected := peerHandoverPair(t)
	peerHandoverAwaitReceiver(t, main)
	files := t.TempDir()
	path, hash := peerHandoverFile(t, files, "notes.txt", "one\n")
	changed, oldHash := peerHandoverFile(t, files, "changed.txt", "before\n")
	if err := os.WriteFile(changed, []byte("after\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(files, "gone.txt")
	items := []peerHandoverItem{
		{ID: "doc-ok", Kind: "file", Path: path, Hash: hash, Dirty: true, Text: "two\n", Format: peerHandoverLF, SavedFormat: peerHandoverLF},
		{ID: "doc-changed", Kind: "file", Path: changed, Hash: oldHash, Dirty: true, Text: "mine\n", Format: peerHandoverLF, SavedFormat: peerHandoverLF},
		{ID: "doc-gone", Kind: "file", Path: gone, Hash: oldHash, Dirty: true, Text: "mine\n", Format: peerHandoverLF, SavedFormat: peerHandoverLF},
	}
	code, reply, raw := peerPage(t, main, http.MethodPost, "/api/peer-handover/start", peerHandoverJSON(t, map[string]any{"items": items}))
	if code != http.StatusOK {
		t.Fatalf("start: %d %s", code, raw)
	}
	sent, _ := reply["sent"].([]any)
	ineligible, _ := reply["ineligible"].([]any)
	if len(sent) != 1 || sent[0] != "doc-ok" || len(ineligible) != 2 {
		t.Fatalf("changed files must not be handed over: %s", raw)
	}
	for _, entry := range ineligible {
		reason, _ := entry.(map[string]any)["reason"].(string)
		if !strings.Contains(reason, "on disk") {
			t.Fatalf("reason: %s", raw)
		}
	}
	request := peerAwaitRequest(t, collected, "changed file")
	if documents := request["documents"].([]any); len(documents) != 1 || documents[0].(map[string]any)["id"] != "doc-ok" {
		t.Fatalf("receiving page got %v", request)
	}
	_ = editor
	// Nothing at all to send: no request reaches the receiver.
	main.peer.mu.Lock()
	main.peer.handover = nil
	main.peer.mu.Unlock()
	code, reply, raw = peerPage(t, main, http.MethodPost, "/api/peer-handover/start", peerHandoverJSON(t, map[string]any{"items": items[1:]}))
	if code != http.StatusOK || reply["requestId"] != "" || len(reply["sent"].([]any)) != 0 {
		t.Fatalf("only changed files: %d %s", code, raw)
	}
	// The receiver checks again: a file that changed between the two checks
	// refuses the whole request there, and the source keeps its tabs.
	if r := peerInbound(editor, http.MethodPost, "/api/peer/documents/handover", peerHandoverJSON(t, peerHandoverRequest{RequestID: strings.Repeat("c", 32), Documents: items[1:2]}), editor.windowActivationToken, nil); r.Code != http.StatusConflict || !strings.Contains(r.Body.String(), "changed.txt") {
		t.Fatalf("receiver accepted a changed file: %d %s", r.Code, r.Body)
	}
}

func TestPeerHandoverWithoutAReceiver(t *testing.T) {
	root := t.TempDir()
	main, _ := peerTestApp(t, root, "main")
	peerRegister(t, main, root)
	draft := peerHandoverJSON(t, map[string]any{"items": []peerHandoverItem{{ID: "d", Kind: "untitled", Name: "x", Format: peerHandoverLF}}})
	// Nothing runs: not available, and a start sends nothing (the caller keeps the tabs).
	if code, reply, raw := peerPage(t, main, http.MethodGet, "/api/peer-handover/receiver", ""); code != http.StatusOK || reply["available"] != false {
		t.Fatalf("receiver without an editor: %d %s", code, raw)
	}
	if code, _, raw := peerPage(t, main, http.MethodPost, "/api/peer-handover/start", draft); code != http.StatusServiceUnavailable {
		t.Fatalf("start without an editor: %d %s", code, raw)
	}
	// A standalone editor whose page is not running is not available either.
	editor, _ := peerTestApp(t, root, standaloneEditorMode)
	unregister := peerRegister(t, editor, root)
	if code, reply, raw := peerPage(t, main, http.MethodGet, "/api/peer-handover/receiver", ""); code != http.StatusOK || reply["available"] != false {
		t.Fatalf("receiver without a page: %d %s", code, raw)
	}
	unregister()
	// An older standalone editor (no document routes) is never a receiver.
	requests := peerOlderEngine(t, peerWorkspaceDir(root, standaloneEditorMode))
	if code, reply, raw := peerPage(t, main, http.MethodGet, "/api/peer-handover/receiver", ""); code != http.StatusOK || reply["available"] != false {
		t.Fatalf("older receiver: %d %s", code, raw)
	}
	if code, _, raw := peerPage(t, main, http.MethodPost, "/api/peer-handover/start", draft); code < 400 {
		t.Fatalf("start to an older editor: %d %s", code, raw)
	}
	for _, seen := range requests() {
		if strings.HasPrefix(seen, http.MethodPost) {
			t.Fatalf("text reached an older editor: %s", seen)
		}
	}
}

func TestPeerHandoverRoutesAndGuards(t *testing.T) {
	root := t.TempDir()
	// Only the main window has the page routes.
	for _, mode := range []string{standaloneEditorMode, "terminal"} {
		a, _ := peerTestApp(t, root, mode)
		if r := requestApp(a, http.MethodGet, "/api/peer-handover/receiver", "", true); r.Code != http.StatusNotFound {
			t.Fatalf("%s served the hand-over page route: %d", mode, r.Code)
		}
	}
	bare := testApplication(t)
	if r := requestApp(bare, http.MethodGet, "/api/peer-handover/receiver", "", true); r.Code != http.StatusNotFound {
		t.Fatalf("a main window without a data root: %d", r.Code)
	}
	main, _ := peerTestApp(t, root, "main")
	if r := requestApp(main, http.MethodGet, "/api/peer-handover/receiver", "", false); r.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated: %d", r.Code)
	}
	if r := peerInbound(main, http.MethodGet, "/api/peer-handover/receiver", "", main.windowActivationToken, nil); r.Code != http.StatusUnauthorized {
		t.Fatalf("activation token on the page route: %d", r.Code)
	}
	for path, method := range map[string]string{"receiver": http.MethodPost, "start": http.MethodGet, "result": http.MethodPost, "other": http.MethodGet} {
		if r := requestApp(main, method, "/api/peer-handover/"+path, "", true); r.Code != http.StatusMethodNotAllowed && r.Code != http.StatusNotFound {
			t.Fatalf("%s %s: %d", method, path, r.Code)
		}
	}
	for name, body := range map[string]string{
		"unknown field": `{"items":[],"extra":1}`,
		"two objects":   `{"items":[]}{"items":[]}`,
		"not JSON":      `items`,
		"invalid item":  `{"items":[{"id":"d","kind":"file","path":"notes.txt","hash":"` + strings.Repeat("a", 64) + `"}]}`,
	} {
		if r := requestApp(main, http.MethodPost, "/api/peer-handover/start", body, true); r.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", name, r.Code, r.Body)
		}
	}
	if r := requestApp(main, http.MethodGet, "/api/peer-handover/result?requestId=nope", "", true); r.Code != http.StatusBadRequest {
		t.Fatalf("bad request id: %d", r.Code)
	}
	if r := requestApp(main, http.MethodGet, "/api/peer-handover/result?requestId="+peerTestRequestID, "", true); r.Code != http.StatusNotFound {
		t.Fatalf("unknown request id: %d", r.Code)
	}
	// Receiving routes: the standalone Text Editor only, activation token
	// only, loopback only, never from a page.
	editor, _ := peerTestApp(t, root, standaloneEditorMode)
	token := editor.windowActivationToken
	status := "/api/peer/documents/status"
	if r := peerInbound(editor, http.MethodGet, status, "", token, nil); r.Code != http.StatusOK {
		t.Fatalf("status: %d %s", r.Code, r.Body)
	}
	for name, r := range map[string]*httptest.ResponseRecorder{
		"no token":      peerInbound(editor, http.MethodGet, status, "", "", nil),
		"session token": peerInbound(editor, http.MethodGet, status, "", "", func(r *http.Request) { r.Header.Set("X-Session-Token", editor.token) }),
	} {
		if r.Code != http.StatusUnauthorized {
			t.Fatalf("%s: %d", name, r.Code)
		}
	}
	for name, change := range map[string]func(*http.Request){
		"other computer": func(r *http.Request) { r.RemoteAddr = "192.0.2.7:50123" },
		"page origin":    func(r *http.Request) { r.Header.Set("Origin", "http://"+editor.host) },
		"cookie":         func(r *http.Request) { r.Header.Set("Cookie", "a=b") },
	} {
		if r := peerInbound(editor, http.MethodGet, status, "", token, change); r.Code != http.StatusForbidden {
			t.Fatalf("%s: %d", name, r.Code)
		}
	}
	for _, a := range []*application{main, func() *application { a, _ := peerTestApp(t, t.TempDir(), "terminal"); return a }()} {
		for _, path := range []string{status, "/api/peer/documents/handover", "/api/peer/documents/result"} {
			if r := peerInbound(a, http.MethodGet, path, "", a.windowActivationToken, nil); r.Code != http.StatusNotFound {
				t.Fatalf("%s served %s: %d", a.windowMode, path, r.Code)
			}
		}
	}
	handover := "/api/peer/documents/handover"
	good := peerHandoverRequest{RequestID: strings.Repeat("d", 32), Documents: []peerHandoverItem{{ID: "d", Kind: "untitled", Name: "x", Format: peerHandoverLF}}}
	if r := peerInbound(editor, http.MethodGet, handover, "", token, nil); r.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET hand-over: %d", r.Code)
	}
	// The page has not reported: not ready yet.
	if r := peerInbound(editor, http.MethodPost, handover, peerHandoverJSON(t, good), token, nil); r.Code != http.StatusServiceUnavailable {
		t.Fatalf("hand-over before the page runs: %d %s", r.Code, r.Body)
	}
	for name, body := range map[string]string{
		"bad id":        `{"requestId":"x","documents":[{"id":"d","kind":"untitled","name":"x","format":{"encoding":"UTF-8","eol":"LF"}}]}`,
		"unknown field": `{"requestId":"` + strings.Repeat("d", 32) + `","documents":[],"text":"x"}`,
		"too large":     `{"requestId":"` + strings.Repeat("d", 32) + `","documents":[{"id":"d","kind":"untitled","name":"x","format":{"encoding":"UTF-8","eol":"LF"},"dirty":true,"text":"` + strings.Repeat("x", int(peerHandoverBodyLimit)) + `"}]}`,
	} {
		if r := peerInbound(editor, http.MethodPost, handover, body, token, nil); r.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", name, r.Code, r.Body)
		}
	}
	// An editor insert cannot carry documents, and documents cannot be
	// acknowledged as an insert.
	if r := peerInbound(editor, http.MethodPost, "/api/peer/editor/insert", `{"requestId":"`+peerTestRequestID+`","target":"new","text":"x","documents":[{"id":"d"}]}`, token, nil); r.Code != http.StatusBadRequest {
		t.Fatalf("an insert carried documents: %d %s", r.Code, r.Body)
	}
	peerReport(t, editor, `[]`)
	if r := peerInbound(editor, http.MethodPost, "/api/peer/editor/insert", `{"requestId":"`+peerTestRequestID+`","target":"new","text":"x"}`, token, nil); r.Code != http.StatusOK {
		t.Fatalf("insert: %d %s", r.Code, r.Body)
	}
	if request := peerReport(t, editor, `[]`); request == nil {
		t.Fatal("insert not collected")
	}
	if code, raw := peerHandoverAck(t, editor, `{"requestId":"`+peerTestRequestID+`","ok":true,"outcome":"opened","items":[{"id":"d","ok":true}]}`); code != http.StatusBadRequest {
		t.Fatalf("an insert acknowledged as opened documents: %d %s", code, raw)
	}
	if code, raw := peerHandoverAck(t, editor, `{"requestId":"`+peerTestRequestID+`","ok":true,"outcome":"inserted","items":[{"id":"d","ok":true}]}`); code != http.StatusBadRequest {
		t.Fatalf("an insert acknowledged with items: %d %s", code, raw)
	}
	// The terminal never acknowledges documents.
	term, _ := peerTestApp(t, t.TempDir(), "terminal")
	if r := requestApp(term, http.MethodPost, "/api/peer-send/ack", `{"requestId":"`+peerTestRequestID+`","ok":true,"outcome":"opened","items":[]}`, true); r.Code != http.StatusBadRequest {
		t.Fatalf("terminal acknowledged documents: %d %s", r.Code, r.Body)
	}
}

// What goes to the receiver is the request identifier and the documents the
// page chose; never a token of the main window or its data folder.
func TestPeerHandoverPutsOnlyTheDocumentsOnTheWire(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Data Folder")
	main, _ := peerTestApp(t, root, "main")
	peerRegister(t, main, root)
	recorded := peerRecordingReceiver(t, peerWorkspaceDir(root, standaloneEditorMode), standaloneEditorMode)
	if code, reply, raw := peerPage(t, main, http.MethodGet, "/api/peer-handover/receiver", ""); code != http.StatusOK || reply["available"] != true {
		t.Fatalf("receiver: %d %s", code, raw)
	}
	items := []peerHandoverItem{{ID: "doc-draft", Kind: "untitled", Name: "Untitled.txt", Dirty: true, Text: "draft\n", Format: peerHandoverLF}}
	if code, _, raw := peerPage(t, main, http.MethodPost, "/api/peer-handover/start", peerHandoverJSON(t, map[string]any{"items": items})); code != http.StatusOK {
		t.Fatalf("start: %d %s", code, raw)
	}
	posts := 0
	for _, request := range recorded() {
		headers, _ := json.Marshal(request.Header)
		body, _ := io.ReadAll(request.Body)
		for _, private := range []string{main.token, main.windowActivationToken, root, "Data Folder"} {
			if strings.Contains(string(body), private) || strings.Contains(string(headers), private) || strings.Contains(request.URL.String(), private) {
				t.Fatalf("a token or the data folder crossed: %s %s", request.URL, body)
			}
		}
		if request.Method == http.MethodPost {
			posts++
			var got peerHandoverRequest
			if err := json.Unmarshal(body, &got); err != nil || !peerRequestID.MatchString(got.RequestID) || len(got.Documents) != 1 {
				t.Fatalf("wire: %s", body)
			}
			if sent := got.Documents[0]; sent.ID != items[0].ID || sent.Kind != "untitled" || sent.Name != items[0].Name || sent.Text != items[0].Text || !sent.Dirty || *sent.Format != *items[0].Format || sent.Path != "" {
				t.Fatalf("wire: %s", body)
			}
			if request.URL.Path != "/api/peer/documents/handover" {
				t.Fatalf("posted to %s", request.URL.Path)
			}
		}
	}
	if posts != 1 {
		t.Fatalf("%d POSTs", posts)
	}
}
