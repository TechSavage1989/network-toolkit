package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Owner decision: Edit in the SFTP browser of a detached Terminal workspace
// opens the remote file in the main window's Text Editor, on the same SSH
// connection, with the main window's own read and save checks. The workspace's
// browser stays read-only for everything else, and the main window gains
// nothing of the session but that one file.
func TestWindowSendRemoteFileFromADetachedWorkspace(t *testing.T) {
	f := newTerminalSFTP123Fixture(t, true)
	a := f.app
	session := f.session.ID
	view := f.launch.ViewToken
	main := func(action, path string, extra map[string]any) *jsonReply {
		body := map[string]any{"protocol": "ssh", "sessionId": session, "action": action, "path": path}
		for k, v := range extra {
			body[k] = v
		}
		return decodeReply(t, viewTestRequest(a, a.token, "POST", "/api/remote-browser", body, nil))
	}
	// Before the workspace asks, the main window cannot use the session.
	if r := main("read-text", "startup.cfg", nil); r.code != http.StatusConflict {
		t.Fatalf("the main window read a detached session's file: %d %s", r.code, r.raw)
	}
	// Both windows' pages run.
	for _, token := range []string{a.token, view} {
		windowSendCall(t, a, token, "GET", "/api/window-send/inbox", nil)
		windowSendCall(t, a, token, "POST", "/api/window-send/report", map[string]any{"targets": []any{}, "documents": []any{}})
	}
	for _, tc := range []struct {
		name, token string
		body        map[string]any
		status      int
	}{
		{"from the main window", a.token, map[string]any{"kind": "remote", "window": "main", "session": session, "path": "startup.cfg"}, http.StatusBadRequest},
		{"to another window", view, map[string]any{"kind": "remote", "window": windowSendEditor + strings.Repeat("0", 32), "session": session, "path": "startup.cfg"}, http.StatusBadRequest},
		{"with text", view, map[string]any{"kind": "remote", "window": "main", "session": session, "path": "startup.cfg", "text": "x"}, http.StatusBadRequest},
		{"without a path", view, map[string]any{"kind": "remote", "window": "main", "session": session, "path": ""}, http.StatusBadRequest},
		{"a session it does not own", view, map[string]any{"kind": "remote", "window": "main", "session": "other-session", "path": "startup.cfg"}, http.StatusConflict},
		{"a path on a text send", view, map[string]any{"kind": "editor", "window": "main", "target": "new", "text": "x", "path": "startup.cfg"}, http.StatusBadRequest},
	} {
		if code, out := windowSendCall(t, a, tc.token, "POST", "/api/window-send/send", tc.body); code != tc.status {
			t.Fatalf("%s: %d %v", tc.name, code, out)
		}
	}
	code, out := windowSendCall(t, a, view, "POST", "/api/window-send/send", map[string]any{"kind": "remote", "window": "main", "session": session, "path": "startup.cfg"})
	if code != 200 {
		t.Fatalf("Edit from the workspace: %d %v", code, out)
	}
	id := out["requestId"].(string)
	_, inbox := windowSendCall(t, a, a.token, "GET", "/api/window-send/inbox", nil)
	request, _ := inbox["request"].(map[string]any)
	if inbox["kind"] != "remote" || request["session"] != session || request["path"] != "startup.cfg" || request["text"] != "" {
		t.Fatalf("the main window's request: %v", inbox)
	}
	// The main window's own SFTP editor code now reads and saves that file.
	read := main("read-text", "startup.cfg", nil)
	var file struct {
		Content string `json:"content"`
		Hash    string `json:"hash"`
	}
	if read.code != 200 || json.Unmarshal([]byte(read.raw), &file) != nil || !strings.Contains(file.Content, "hostname test-switch") {
		t.Fatalf("read the granted file: %d %s", read.code, read.raw)
	}
	if r := main("write-text", "startup.cfg", map[string]any{"content": "hostname edited\n", "expectedHash": strings.Repeat("0", 64), "encoding": "UTF-8"}); r.code == 200 {
		t.Fatal("a save with a stale hash was accepted")
	}
	if r := main("write-text", "startup.cfg", map[string]any{"content": "hostname edited\n", "expectedHash": file.Hash, "encoding": "UTF-8"}); r.code != 200 {
		t.Fatalf("save the granted file: %d %s", r.code, r.raw)
	}
	if raw, err := os.ReadFile(filepath.Join(f.root, "startup.cfg")); err != nil || string(raw) != "hostname edited\n" {
		t.Fatalf("the remote file: %q %v", raw, err)
	}
	// Only that file, and only reading and saving it.
	if r := main("read-text", "other.cfg", nil); r.code != http.StatusConflict {
		t.Fatalf("another file of the session: %d %s", r.code, r.raw)
	}
	for _, action := range []string{"remove", "list", "rename"} {
		if r := main(action, "startup.cfg", map[string]any{"confirm": true, "newPath": "x.cfg"}); r.code != http.StatusConflict {
			t.Fatalf("%s through the grant: %d %s", action, r.code, r.raw)
		}
	}
	// The workspace's own browser stays read-only.
	if w := viewTestRequest(a, view, "POST", "/api/remote-browser", map[string]any{"protocol": "ssh", "sessionId": session, "action": "write-text", "path": "startup.cfg"}, &f.revision); w.Code != http.StatusForbidden {
		t.Fatalf("the workspace wrote: %d", w.Code)
	}
	// Second review, HIGH: Save As elsewhere is refused on both save paths.
	if r := main("write-text", "/etc/cron.d/synthetic", map[string]any{"content": "x\n", "encoding": "UTF-8"}); r.code != http.StatusConflict || !strings.Contains(r.raw, "Save a copy locally") {
		t.Fatalf("Save As through the page-held path: %d %s", r.code, r.raw)
	}
	snapshot := editorDocumentSnapshot{UID: editorFixtureUID2, Path: "startup.cfg", Binding: editorBinding{Kind: "terminal", SessionID: session}}
	g, err := a.editorViews.makeGrant("", snapshot)
	if err != nil {
		t.Fatalf("the disk-backed grant for the opened file: %v", err)
	}
	local := filepath.Join(t.TempDir(), "copy")
	if _, err := g.run(t.Context(), remoteBrowserRequest{Protocol: "sftp", Action: "download", Path: "startup.cfg", LocalPath: local}); err != nil {
		t.Fatalf("download of the opened file: %v", err)
	}
	for _, req := range []remoteBrowserRequest{
		{Protocol: "sftp", Action: "upload", Path: "/etc/cron.d/synthetic", LocalPath: local, Overwrite: true, StagedUpload: true},
		{Protocol: "sftp", Action: "download", Path: "other.cfg", LocalPath: local},
		{Protocol: "sftp", Action: "remove", Path: "startup.cfg"},
		{Protocol: "sftp", Action: "rename", Path: "startup.cfg", NewPath: "moved.cfg"},
		{Protocol: "sftp", Action: "list", Path: "."},
	} {
		if _, err := g.run(t.Context(), req); err == nil {
			t.Fatalf("the disk-backed grant ran %s %s", req.Action, req.Path)
		}
	}
	if _, err := remoteSaveDestination(g, "startup.cfg", "/etc/cron.d/synthetic"); err == nil || !strings.Contains(err.Error(), "Save a copy locally") {
		t.Fatalf("Save As through the disk-backed path: %v", err)
	}
	if destination, err := remoteSaveDestination(g, "startup.cfg", ""); err != nil || destination != "startup.cfg" {
		t.Fatalf("saving the opened file: %q %v", destination, err)
	}
	// MEDIUM: the document never leaves the main window.
	batch := editorFixtureBatch()
	batch.Documents[0].UID, batch.Documents[0].ID, batch.Documents[0].Path, batch.Documents[0].Binding = editorFixtureUID2, "terminal:"+session+":startup.cfg", "startup.cfg", editorBinding{Kind: "terminal", SessionID: session}
	batch.Documents[0].Untitled, batch.Active = false, editorFixtureUID2
	if _, err := a.editorViews.offer("", editorTransferRequest{ID: randomHex(16), Destination: "new", Batch: batch}); err == nil || !strings.Contains(err.Error(), "stays in the main window") {
		t.Fatalf("a remote file opened from a workspace moved to another window: %v", err)
	}
	// The main page confirms with the tab it opened, and reports it.
	if code, out := windowSendCall(t, a, a.token, "POST", "/api/window-send/ack", map[string]any{"requestId": id, "ok": true, "outcome": "opened", "document": editorFixtureUID2}); code != 200 || out["found"] != true {
		t.Fatalf("ack: %d %v", code, out)
	}
	if _, out := windowSendCall(t, a, view, "GET", "/api/window-send/result?requestId="+id, nil); out["state"] != peerDone || out["outcome"] != "opened" {
		t.Fatalf("result: %v", out)
	}
	windowSendCall(t, a, a.token, "POST", "/api/window-send/report", map[string]any{"targets": []any{}, "documents": []map[string]any{{"id": editorFixtureUID2, "name": "startup.cfg"}}})
	if r := main("read-text", "startup.cfg", nil); r.code != 200 {
		t.Fatalf("the open tab lost its file: %d %s", r.code, r.raw)
	}
	// LOW 2: closing the tab in the main window ends the grant, also for the disk path.
	windowSendCall(t, a, a.token, "POST", "/api/window-send/report", map[string]any{"targets": []any{}, "documents": []any{}})
	if r := main("read-text", "startup.cfg", nil); r.code != http.StatusConflict {
		t.Fatalf("a closed tab kept its grant: %d %s", r.code, r.raw)
	}
	if g.valid() {
		t.Fatal("the disk-backed grant outlived the closed tab")
	}
	// LOW 4: a save after that says why, plainly.
	if _, err := remoteSaveDestination(g, "startup.cfg", ""); err == nil || !strings.Contains(err.Error(), "no longer holds the connection") {
		t.Fatalf("a save after the grant ended: %v", err)
	}
	// A request the main page refused leaves no grant behind.
	_, out = windowSendCall(t, a, view, "POST", "/api/window-send/send", map[string]any{"kind": "remote", "window": "main", "session": session, "path": "startup.cfg"})
	refused := out["requestId"].(string)
	windowSendCall(t, a, a.token, "GET", "/api/window-send/inbox", nil)
	windowSendCall(t, a, a.token, "POST", "/api/window-send/ack", map[string]any{"requestId": refused, "ok": false, "error": "A dialog is open."})
	if r := main("read-text", "startup.cfg", nil); r.code != http.StatusConflict {
		t.Fatalf("a refused request kept its grant: %d %s", r.code, r.raw)
	}
	// Once the session comes back to the main window the grant is not needed;
	// when the transport ends it no longer admits anything.
	if err := a.terminals.Disconnect(session); err != nil {
		t.Fatal(err)
	}
	viewTestWait(t, func() bool { _, _, err := a.terminals.SFTPClient(session); return err != nil })
	if r := main("read-text", "startup.cfg", nil); r.code == 200 {
		t.Fatal("a grant outlived its SSH transport")
	}
}

type jsonReply struct {
	code int
	raw  string
}

func decodeReply(t *testing.T, w interface {
	Result() *http.Response
}) *jsonReply {
	t.Helper()
	response := w.Result()
	defer response.Body.Close()
	var b strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := response.Body.Read(buf)
		b.Write(buf[:n])
		if err != nil {
			break
		}
	}
	return &jsonReply{code: response.StatusCode, raw: b.String()}
}
