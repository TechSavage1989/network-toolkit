package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func approveEditorExplorer(t *testing.T, a *application, id string) string {
	t.Helper()
	token := a.editorViews.views[id].token
	if r := editorHTTP(a, token, "/api/editor-views/current/explorer", map[string]any{}); r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	nonce := a.editorViews.views[id].explorer.RequestID
	if r := editorHTTP(a, a.token, "/api/editor-views/explorer/"+id, map[string]any{"requestId": nonce, "allow": true}); r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	return nonce
}
func TestEditorExplorerApprovalRevocationAndUnrelatedCapabilities(t *testing.T) {
	a, m, _ := editorFixture(t)
	req, _, id := editorFixtureOffer(t, m)
	if _, err := m.accept(id, req.ID); err != nil {
		t.Fatal(err)
	}
	token := m.views[id].token
	dir := t.TempDir()
	target := filepath.Join(dir, "created")
	body := map[string]any{"action": "mkdir", "path": target}
	if r := editorHTTP(a, token, "/api/transfer/local-files", body); r.Code != 403 {
		t.Fatal("unapproved child mutation", r.Code)
	}
	if r := editorHTTP(a, token, "/api/editor-views/explorer/"+id, map[string]any{"allow": true}); r.Code != 403 {
		t.Fatal("child self-approved", r.Code)
	}
	nonce := approveEditorExplorer(t, a, id)
	if r := editorHTTP(a, token, "/api/transfer/local-files", body); r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{"/api/state", "/api/credentials", "/api/terminals", "/api/transfer/server"} {
		if r := editorGET(a, token, url); r.Code != 403 {
			t.Fatal("unrelated capability exposed", url, r.Code)
		}
	}
	if r := editorHTTP(a, token, "/api/file-editor/local", map[string]any{"action": "write-text", "path": filepath.Join(dir, "bypass.txt"), "content": "no"}); r.Code != 403 {
		t.Fatal("generic write bypassed lease", r.Code)
	}
	if r := editorHTTP(a, a.token, "/api/editor-views/explorer/"+id, map[string]any{"requestId": nonce, "allow": false}); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if r := editorHTTP(a, token, "/api/transfer/local-files", map[string]any{"action": "delete", "path": target, "confirm": true}); r.Code != 403 {
		t.Fatal("revoked access usable", r.Code)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatal("revocation changed files", err)
	}
}
func TestEditorExplorerPinsSFTPAndRequiresDocumentLeaseForSaving(t *testing.T) {
	f := newPersistentBrowserFixture(t)
	a := f.app
	m := a.editorViews
	a.launchEditorView = func(context.Context, editorViewLaunch) (<-chan error, error) { return nil, nil }
	connection := f.connect(t, nil)
	req, _, id := editorFixtureOffer(t, m)
	if _, err := m.accept(id, req.ID); err != nil {
		t.Fatal(err)
	}
	token := m.views[id].token
	approveEditorExplorer(t, a, id)
	operation := func(action, connection string) int {
		return editorHTTP(a, token, "/api/remote-browser", map[string]any{"protocol": "ssh", "connectionId": connection, "action": action, "path": "startup.cfg"}).Code
	}
	if code := operation("read-text", connection); code != 200 {
		t.Fatal("approved SFTP read", code)
	}
	for _, action := range []string{"disconnect", "open-terminal", "write-text", "connect"} {
		if code := operation(action, connection); code != 403 {
			t.Fatal("connection or write authority leaked", action, code)
		}
	}
	later := f.connect(t, nil)
	if code := operation("list", later); code != 403 {
		t.Fatal("unapproved new connection", code)
	}
	batch := editorFixtureBatch()
	batch.Documents[0].UID = editorFixtureUID2
	batch.Documents[0].ID = "remote:new"
	batch.Documents[0].Path = "startup.cfg"
	batch.Documents[0].Binding = editorBinding{Kind: "sshConnection", ConnectionID: connection}
	batch.Active = editorFixtureUID2
	if _, err := m.checkpoint(id, batch, nil); err != nil {
		t.Fatal("approved editor binding", err)
	}
	if r := editorHTTP(a, token, "/api/remote-browser", map[string]any{"protocol": "ssh", "connectionId": connection, "action": "remove", "path": "startup.cfg", "confirm": true}); r.Code != 409 {
		t.Fatal("open document deletion", r.Code, r.Body.String())
	}
	a.closeBrowserConnection(connection)
	if code := operation("read-text", connection); code != 403 {
		t.Fatal("stale transport accepted", code)
	}
	if r := editorGET(a, token, "/api/remote-browser"); strings.Contains(r.Body.String(), connection) {
		t.Fatal("stale connection offered")
	}
}
func TestEditorExplorerReservationProtectsOpenFilesAndNewSavePaths(t *testing.T) {
	a, m, _ := editorFixture(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "open.txt")
	if err := os.WriteFile(file, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	batch := editorFixtureBatch()
	batch.Documents[0].Path = file
	batch.Documents[0].Binding = editorBinding{Kind: "local"}
	if _, err := m.checkpoint("", batch, nil); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{file, dir} {
		if r := editorHTTP(a, a.token, "/api/transfer/local-files", map[string]any{"action": "delete", "path": target, "confirm": true}); r.Code != 409 {
			t.Fatal("open file or ancestor mutable", r.Code)
		}
	}
	target := filepath.Join(dir, "incoming.txt")
	release, err := a.reserveExplorerMutation(editorExplorerTarget{Binding: editorBinding{Kind: "local"}, Path: target})
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if r := editorHTTP(a, a.token, "/api/editor-views/documents/"+editorFixtureUID+"/file", map[string]any{"lease": 1, "action": "write-text", "path": target, "content": "no"}); r.Code != 409 {
		t.Fatal("Save As raced reserved explorer destination", r.Code)
	}
	batch.Documents[0].UID = editorFixtureUID2
	batch.Documents[0].ID = "other"
	batch.Documents[0].Path = target
	batch.Active = editorFixtureUID2
	if _, err := m.checkpoint("", batch, nil); err == nil {
		t.Fatal("new grant acquired during conflicting file mutation")
	}
}
func TestEditorLayoutValidationAndTransferRoundTrip(t *testing.T) {
	a, m, _ := editorFixture(t)
	_ = a
	batch := editorFixtureBatch()
	other := batch.Documents[0]
	other.UID = editorFixtureUID2
	other.ID = "right"
	batch.Documents = append(batch.Documents, other)
	batch.Layout = &editorLayout{Split: true, ActiveSide: "right", Left: editorFixtureUID, Right: editorFixtureUID2, Groups: []editorTabGroup{{editorFixtureUID, "left"}, {editorFixtureUID2, "right"}}}
	if err := validateEditorBatch(batch); err != nil {
		t.Fatal(err)
	}
	req := editorTransferRequest{ID: randomHex(16), Destination: "new", Batch: batch}
	out, err := m.offer("", req)
	if err != nil {
		t.Fatal(err)
	}
	received := m.state(out["destination"].(string))
	raw, _ := json.Marshal(received)
	if !strings.Contains(string(raw), `"activeSide":"right"`) {
		t.Fatal("view layout lost")
	}
	for _, mutate := range []func(*editorLayout){func(l *editorLayout) { l.Left = editorFixtureUID2 }, func(l *editorLayout) { l.Groups[1].UID = editorFixtureUID }, func(l *editorLayout) { l.Groups[0].Side = "foreign" }, func(l *editorLayout) { l.Groups = l.Groups[:1] }, func(l *editorLayout) { l.ActiveSide = "foreign" }} {
		bad := editorCloneBatch(batch)
		mutate(bad.Layout)
		if validateEditorBatch(bad) == nil {
			t.Fatal("invalid layout accepted")
		}
	}
	if r := editorGET(a, m.views[out["destination"].(string)].token, "/api/transfer/local"); r.Code != http.StatusForbidden {
		t.Fatal("layout conferred file authority")
	}
}

func TestEditorExplorerCanonicalPathConflicts(t *testing.T) {
	b := editorBinding{Kind: "sshConnection", ConnectionID: "synthetic"}
	for _, test := range []struct {
		target, document string
		want             bool
	}{
		{"./startup.cfg", "startup.cfg", true},
		{"configs/../startup.cfg", "startup.cfg", true},
		{"/configs/./", "/configs/sub/../open.cfg", true},
		{"/", "/configs/open.cfg", true},
		{".", "configs/open.cfg", true},
		{".", "../outside.cfg", false},
		{"/configs", "/configs-other/open.cfg", false},
		{"", "startup.cfg", false},
		{".", "", false},
	} {
		if got := explorerTargetContains(editorExplorerTarget{Binding: b, Path: test.target}, b, test.document); got != test.want {
			t.Errorf("target %q, document %q: got %v, want %v", test.target, test.document, got, test.want)
		}
	}
	other := b
	other.ConnectionID = "other-connection"
	if explorerTargetContains(editorExplorerTarget{Binding: b, Path: "/"}, other, "/open.cfg") {
		t.Fatal("unrelated remote connection was blocked")
	}
}
