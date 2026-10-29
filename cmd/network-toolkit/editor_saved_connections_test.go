package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func editorSavedFixtureState(a *application, host string) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.state, _ = json.Marshal(map[string]any{"privateSetting": "never-expose-setting", "terminalProfiles": []any{
		map[string]any{"id": "saved-ssh", "type": "ssh", "name": "Lab SSH", "host": host, "port": 22, "username": "never-expose-user", "credentialId": "never-expose-credential", "password": "never-expose-password", "privateKey": "never-expose-key"},
		map[string]any{"id": "telnet", "type": "telnet", "name": "Console", "host": host, "port": 23},
	}, "ftpProfiles": []any{map[string]any{"id": "saved-ftps", "name": "Lab FTPS", "host": "files.test", "port": 990, "username": "never-expose-user", "security": "implicit", "credentialId": "never-expose-credential"}}})
}
func editorSavedFixture(t *testing.T) (*application, *editorViewRegistry, string, string) {
	t.Helper()
	a, m, _ := editorFixture(t)
	req, _, id := editorFixtureOffer(t, m)
	if _, err := m.accept(id, req.ID); err != nil {
		t.Fatal(err)
	}
	editorSavedFixtureState(a, "ssh.test")
	return a, m, id, m.views[id].token
}
func editorSavedRequest(t *testing.T, a *application, token string) string {
	t.Helper()
	r := editorHTTP(a, token, "/api/editor-views/saved-connections/request", map[string]any{"id": "ssh:saved-ssh"})
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(r.Body.Bytes(), &out)
	return out["requestId"].(string)
}
func TestEditorSavedConnectionsMetadataAndCapabilities(t *testing.T) {
	a, m, id, token := editorSavedFixture(t)
	if r := editorGET(a, token, "/api/editor-views/saved-connections"); r.Code != 403 {
		t.Fatal(r.Code)
	}
	approveEditorExplorer(t, a, id)
	for _, test := range []struct {
		token string
		want  int
	}{{a.token, 2}, {token, 1}} {
		r := editorGET(a, test.token, "/api/editor-views/saved-connections")
		if r.Code != 200 {
			t.Fatal(r.Code, r.Body.String())
		}
		if strings.Contains(r.Body.String(), "never-expose") || strings.Contains(r.Body.String(), "credential") || strings.Contains(r.Body.String(), "telnet") {
			t.Fatal("metadata leaked", r.Body.String())
		}
		var out struct {
			Profiles []map[string]any `json:"profiles"`
		}
		_ = json.Unmarshal(r.Body.Bytes(), &out)
		if len(out.Profiles) != test.want {
			t.Fatal(out)
		}
		for _, p := range out.Profiles {
			if len(p) != 5 {
				t.Fatal("unexpected metadata", p)
			}
		}
	}
	for _, p := range []string{"/api/state", "/api/credentials", "/api/terminals/known-hosts"} {
		if r := editorGET(a, token, p); r.Code != 403 {
			t.Fatal(p, r.Code)
		}
	}
	if r := editorHTTP(a, token, "/api/remote-browser", map[string]any{"protocol": "ssh", "action": "connect", "host": "unapproved.test", "password": "secret"}); r.Code != 403 {
		t.Fatal(r.Code, r.Body.String())
	}
	if r := editorHTTP(a, token, "/api/editor-views/saved-connections/request", map[string]any{"id": "ftp:saved-ftps"}); r.Code != 404 {
		t.Fatal(r.Code)
	}
	before := m.documents[editorFixtureUID].snapshot.Content
	request := editorSavedRequest(t, a, token)
	if repeat := editorSavedRequest(t, a, token); repeat != request {
		t.Fatal("not idempotent")
	}
	state := m.state("")
	encoded, _ := json.Marshal(state["savedConnectionRequests"])
	if strings.Contains(string(encoded), "never-expose") {
		t.Fatal(string(encoded))
	}
	if len(m.state(id)["savedConnectionRequests"].([]map[string]any)) != 0 {
		t.Fatal("child received main queue")
	}
	r := editorHTTP(a, a.token, "/api/editor-views/saved-connections/review", map[string]any{"viewId": id, "requestId": request, "connect": true})
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	if m.documents[editorFixtureUID].snapshot.Content != before || len(m.views[id].explorer.Remotes) != 0 || len(a.browserConnections) != 0 {
		t.Fatal("review changed document or granted/connected transport")
	}
}
func TestEditorSavedConnectionsRevocationExpiryAndStaleProfile(t *testing.T) {
	for _, mode := range []string{"revoke", "refresh", "close", "expire", "profile", "prepare-close"} {
		t.Run(mode, func(t *testing.T) {
			a, m, id, token := editorSavedFixture(t)
			nonce := approveEditorExplorer(t, a, id)
			request := editorSavedRequest(t, a, token)
			switch mode {
			case "revoke":
				editorHTTP(a, a.token, "/api/editor-views/explorer/"+id, map[string]any{"requestId": nonce, "allow": false})
			case "refresh":
				editorHTTP(a, token, "/api/editor-views/current/explorer", map[string]any{})
			case "close":
				m.processExited(id)
			case "expire":
				m.mu.Lock()
				m.views[id].savedConnection.expires = m.now().Add(-time.Second)
				m.mu.Unlock()
			case "profile":
				editorSavedFixtureState(a, "different.test")
			case "prepare-close":
				if err := m.beginHomeClose(); err != nil {
					t.Fatal(err)
				}
			}
			r := editorHTTP(a, a.token, "/api/editor-views/saved-connections/review", map[string]any{"viewId": id, "requestId": request, "connect": true})
			if r.Code != 409 && r.Code != 403 {
				t.Fatal(mode, r.Code, r.Body.String())
			}
		})
	}
}
func TestEditorSavedConnectionsCancelReviewRaceAndStrictInput(t *testing.T) {
	a, m, id, token := editorSavedFixture(t)
	approveEditorExplorer(t, a, id)
	for _, body := range []map[string]any{{"id": "ssh:saved-ssh", "password": "secret"}, {"id": "ssh:saved-ssh", "viewId": id}, {"id": "ssh:saved-ssh", "connect": true}} {
		if r := editorHTTP(a, token, "/api/editor-views/saved-connections/request", body); r.Code == 200 {
			t.Fatal("accepted extra authority", body)
		}
	}
	request := editorSavedRequest(t, a, token)
	if r := editorHTTP(a, token, "/api/editor-views/saved-connections/review", map[string]any{"viewId": id, "requestId": request, "connect": true}); r.Code != 403 {
		t.Fatal(r.Code)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	codes := make(chan int, 2)
	go func() {
		defer wg.Done()
		codes <- editorHTTP(a, token, "/api/editor-views/saved-connections/cancel", map[string]any{"requestId": request}).Code
	}()
	go func() {
		defer wg.Done()
		codes <- editorHTTP(a, a.token, "/api/editor-views/saved-connections/review", map[string]any{"viewId": id, "requestId": request, "connect": true}).Code
	}()
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != http.StatusOK && code != http.StatusConflict {
			t.Fatal(code)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.views[id].savedConnection != nil {
		t.Fatal("request retained")
	}
}
