package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditorLocal119MainAbsentAndRemoteBoundaries(t *testing.T) {
	a, m, _ := editorFixture(t)
	req, _, id := editorFixtureOffer(t, m)
	if _, err := m.accept(id, req.ID); err != nil {
		t.Fatal(err)
	}
	if err := m.complete("", req.ID); err != nil {
		t.Fatal(err)
	}
	if err := m.beginHomeClose(); err != nil {
		t.Fatal(err)
	}
	if err := m.approveHomeClose(); err != nil {
		t.Fatal(err)
	}
	m.setHomePresent(false)
	token := m.views[id].token
	dir := t.TempDir()
	file := filepath.Join(dir, "synthetic.txt")
	if err := os.WriteFile(file, []byte("SYNTHETIC LOCAL119"), 0600); err != nil {
		t.Fatal(err)
	}
	if r := editorGET(a, token, "/api/transfer/local?path="+url.QueryEscape(dir)); r.Code != 200 || !strings.Contains(r.Body.String(), "synthetic.txt") {
		t.Fatal("local browse without main", r.Code, r.Body.String())
	}
	if r := editorHTTP(a, token, "/api/file-editor/local", map[string]any{"action": "read-text", "path": file}); r.Code != 200 {
		t.Fatal("explicit local read", r.Code)
	}
	folder := filepath.Join(dir, "new-folder")
	if r := editorHTTP(a, token, "/api/transfer/local-files", map[string]any{"action": "mkdir", "path": folder}); r.Code != 200 {
		t.Fatal("local mkdir", r.Code, r.Body.String())
	}
	batch := editorFixtureBatch()
	batch.Documents[0].UID = editorFixtureUID2
	batch.Documents[0].ID = "local119"
	batch.Documents[0].Path = file
	batch.Documents[0].Binding = editorBinding{Kind: "local"}
	batch.Documents[0].Content = "SYNTHETIC LOCAL119"
	batch.Documents[0].Hash = editorDigest([]byte("SYNTHETIC LOCAL119"))
	batch.Active = editorFixtureUID2
	leases, err := m.checkpoint(id, batch, nil)
	if err != nil {
		t.Fatal(err)
	}
	route := "/api/editor-views/documents/" + editorFixtureUID2 + "/file"
	body := map[string]any{"lease": leases[editorFixtureUID2], "action": "write-text", "path": file, "content": "SYNTHETIC LOCAL119 SAVED", "expectedHash": batch.Documents[0].Hash, "encoding": "UTF-8"}
	if r := editorHTTP(a, token, route, body); r.Code != 200 {
		t.Fatal("lease save without main", r.Code, r.Body.String())
	}
	if data, _ := os.ReadFile(file); string(data) != "SYNTHETIC LOCAL119 SAVED" {
		t.Fatal("save bytes", string(data))
	}
	if r := editorHTTP(a, token, "/api/transfer/local-files", map[string]any{"action": "delete", "path": dir, "confirm": true}); r.Code != 409 {
		t.Fatal("open-document ancestor protection", r.Code)
	}
	for _, p := range []string{"/api/state", "/api/credentials", "/api/terminals", "/api/transfer/server", "/api/explorer-transfer/foreign", "/api/editor-views/saved-connections"} {
		if r := editorGET(a, token, p); r.Code != 403 {
			t.Fatal("unrelated/remote capability", p, r.Code)
		}
	}
	for _, b := range []editorBinding{{Kind: "sshConnection", ConnectionID: "guessed"}, {Kind: "terminal", SessionID: "guessed"}, {Kind: "local", ConnectionID: "guessed"}, {Kind: "local", SessionID: "guessed"}, {Kind: "local", Revision: 1}} {
		r := editorHTTP(a, token, "/api/explorer-transfer", explorerTransferRequest{Action: "prepare", Binding: b, Paths: []string{file}})
		if r.Code != 400 && r.Code != 403 {
			t.Fatal("mixed/remote export admitted", b, r.Code)
		}
	}
	if r := editorHTTP(a, token, "/api/file-editor/local", body); r.Code != 403 {
		t.Fatal("generic write bypass", r.Code)
	}
	if r := editorHTTP(a, token, "/api/remote-browser", map[string]any{"protocol": "ssh", "action": "list", "connectionId": "guessed", "path": "/"}); r.Code != 403 {
		t.Fatal("remote admitted", r.Code)
	}
	if r := editorGET(a, token, "/api/editor-views/current/explorer"); r.Code != 200 {
		t.Fatal(r.Code)
	} else {
		var status map[string]any
		json.Unmarshal(r.Body.Bytes(), &status)
		if status["localAllowed"] != true || status["allowed"] != false {
			t.Fatal(status)
		}
	}
	for _, state := range []string{"returned", "closed"} {
		m.mu.Lock()
		m.views[id].info.State = state
		m.mu.Unlock()
		if r := editorGET(a, token, "/api/transfer/local?path="+url.QueryEscape(dir)); r.Code < 400 {
			t.Fatal("dead child local access", state, r.Code)
		}
	}
}

func TestEditorLocal119CopyGuardIgnoresRemoteRevokeButRejectsClosedOwner(t *testing.T) {
	a, m, _ := editorFixture(t)
	_, _, id := editorFixtureOffer(t, m)
	valid := a.localExplorerGuard(id)
	if !valid() {
		t.Fatal("new live child denied")
	}
	m.mu.Lock()
	m.views[id].explorer = &editorExplorerAccess{Allowed: true}
	m.mu.Unlock()
	if !valid() {
		t.Fatal("remote approval interrupted local copy")
	}
	m.mu.Lock()
	m.views[id].explorer = &editorExplorerAccess{Allowed: false}
	m.mu.Unlock()
	if !valid() {
		t.Fatal("remote revocation interrupted local copy")
	}
	root := t.TempDir()
	raw, _ := json.Marshal(map[string]any{"Action": "begin", "Path": root})
	request := httptest.NewRequest(http.MethodPost, "http://"+a.host+"/api/transfer/local-drop", strings.NewReader(string(raw)))
	request.Header.Set("X-Session-Token", m.views[id].token)
	request.Header.Set("Content-Type", "application/json")
	r := httptest.NewRecorder()
	a.ServeHTTP(r, request)
	if r.Code != http.StatusOK {
		t.Fatal(r.Code, r.Body.String())
	}
	var result map[string]string
	json.Unmarshal(r.Body.Bytes(), &result)
	m.mu.Lock()
	m.views[id].info.State = "closed"
	m.mu.Unlock()
	if valid() {
		t.Fatal("closed child still valid")
	}
	job, err := a.importSession(id, result["id"])
	if err != nil {
		t.Fatal(err)
	}
	defer job.root.Close()
	if err := job.verify("."); err == nil {
		t.Fatal("closed child import retained authority")
	}
}
