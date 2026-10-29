package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"networktoolkit/internal/model"
)

const editorFixtureUID = "11111111-1111-4111-8111-111111111111"
const editorFixtureUID2 = "22222222-2222-4222-8222-222222222222"

func editorFixture(t *testing.T) (*application, *editorViewRegistry, *atomic.Int32) {
	t.Helper()
	a := testApplication(t)
	calls := new(atomic.Int32)
	a.launchEditorView = func(_ context.Context, launch editorViewLaunch) (<-chan error, error) {
		if err := launch.validate(); err != nil {
			t.Fatalf("invalid actual child bootstrap: %v", err)
		}
		calls.Add(1)
		return nil, nil
	}
	return a, a.editorViews, calls
}
func editorFixtureBatch() editorBatch {
	d := editorDocumentSnapshot{ID: "local-fixture", UID: editorFixtureUID, Path: "Untitled.txt", Source: "Untitled", Binding: editorBinding{Kind: "localDraft"}, Encoding: "UTF-8", Original: "private-document-canary", Content: "private-document-canary\nedit", Saved: "private-document-canary", SavedEOL: "LF", SavedEncoding: "UTF-8", EOL: "LF", Dirty: true, Undo: []string{"before"}, Redo: []string{}, Untitled: true, EditRevision: 2}
	return editorBatch{Documents: []editorDocumentSnapshot{d}, Active: d.UID}
}
func editorFixtureOffer(t *testing.T, m *editorViewRegistry) (editorTransferRequest, map[string]any, string) {
	t.Helper()
	req := editorTransferRequest{ID: randomHex(16), Destination: "new", Batch: editorFixtureBatch()}
	out, err := m.offer("", req)
	if err != nil {
		t.Fatal(err)
	}
	return req, out, out["destination"].(string)
}
func editorHTTP(a *application, token, path string, body any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPost, "http://"+a.host+path, bytes.NewReader(raw))
	r.Header.Set("X-Session-Token", token)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func editorGET(a *application, token, path string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "http://"+a.host+path, nil)
	r.Header.Set("X-Session-Token", token)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}

func TestEditorRegistryOfferAcceptIsSingleOwnerAndIdempotent(t *testing.T) {
	_, m, launches := editorFixture(t)
	req, out, destination := editorFixtureOffer(t, m)
	uid := req.Batch.Active
	if out["state"] != "offered" || m.documents[uid].owner != "" || m.documents[uid].pending != req.ID {
		t.Fatal("offer changed source ownership")
	}
	if _, err := m.checkpoint("", req.Batch, map[string]uint64{uid: 1}); err == nil {
		t.Fatal("source edited during pending transfer")
	}
	if err := m.beginHomeClose(); err == nil {
		t.Fatal("main closed during pending transfer")
	}
	repeat, err := m.offer("", req)
	if err != nil || repeat["id"] != req.ID || launches.Load() != 1 {
		t.Fatal("idempotent offer launched twice", err)
	}
	accepted, err := m.accept(destination, req.ID)
	if err != nil || accepted["state"] != "accepted" {
		t.Fatal(accepted, err)
	}
	if m.documents[uid].owner != destination || m.documents[uid].lease != 2 {
		t.Fatal("lease not transferred atomically")
	}
	if _, err = m.accept(destination, req.ID); err != nil || m.documents[uid].lease != 2 {
		t.Fatal("lost ACK retry changed lease", err)
	}
	cancel, err := m.cancel("", req.ID)
	if err != nil || cancel["state"] != "accepted" {
		t.Fatal("source rolled back accepted destination", err)
	}
	m.mu.Lock()
	m.transfers[req.ID].created = m.now().Add(-time.Hour)
	m.expireLocked()
	m.mu.Unlock()
	if m.documents[uid].owner != destination {
		t.Fatal("timeout reclaimed accepted destination")
	}
	if _, err = m.checkpoint("", req.Batch, map[string]uint64{uid: 1}); err == nil {
		t.Fatal("old source wrote after acceptance")
	}
	if err = m.release(destination); err == nil {
		t.Fatal("renderer released owned documents")
	}
	if err = m.beginHomeClose(); err != nil {
		t.Fatal("detached owner unnecessarily blocks main close", err)
	}
	if err := m.approveHomeClose(); err != nil {
		t.Fatal(err)
	}
	m.setHomePresent(false)
	if !m.hasLiveViews() || m.documents[uid].owner != destination {
		t.Fatal("closing main lost detached document")
	}
}

func TestEditorRegistryTimeoutAndLaunchFailureRetainSource(t *testing.T) {
	for _, kind := range []string{"timeout", "launch-failure"} {
		t.Run(kind, func(t *testing.T) {
			a, m, _ := editorFixture(t)
			if kind == "launch-failure" {
				a.launchEditorView = func(context.Context, editorViewLaunch) (<-chan error, error) {
					return nil, errors.New("private-launch-canary")
				}
			}
			req, _, destination := editorFixtureOffer(t, m)
			if kind == "timeout" {
				m.mu.Lock()
				m.transfers[req.ID].created = m.now().Add(-21 * time.Second)
				m.expireLocked()
				m.mu.Unlock()
			}
			d := m.documents[req.Batch.Active]
			if d.owner != "" || d.lease != 1 || d.pending != "" {
				t.Fatal("failed move lost original owner")
			}
			if _, err := m.accept(destination, req.ID); err == nil {
				t.Fatal("accepted expired transfer")
			}
			state, _ := json.Marshal(m.state(""))
			if strings.Contains(string(state), "private-launch-canary") || strings.Contains(string(state), "private-document-canary") {
				t.Fatal("status leaked source content/error")
			}
		})
	}
}

func TestEditorRegistryCapabilitiesExposeOnlyIntendedDocuments(t *testing.T) {
	a, m, _ := editorFixture(t)
	req, _, dest := editorFixtureOffer(t, m)
	token, ok := m.token(dest)
	if !ok || token == a.token {
		t.Fatal("no isolated token")
	}
	other := randomHex(16)
	m.views[other] = &editorWindow{info: editorViewInfo{ID: other, State: "ready"}, token: randomHex(32)}
	source, _ := json.Marshal(m.state(""))
	unrelated, _ := json.Marshal(m.state(other))
	target, _ := json.Marshal(m.state(dest))
	if strings.Contains(string(source), "private-document-canary") || strings.Contains(string(unrelated), "private-document-canary") || !strings.Contains(string(target), "private-document-canary") {
		t.Fatal("snapshot audience mismatch")
	}
	if strings.Contains(string(target), token) || strings.Contains(string(target), a.token) {
		t.Fatal("status included capability")
	}
	for _, path := range []string{"/api/state", "/api/credentials", "/api/terminals", "/api/jobs", "/api/packet-capture/interfaces", "/api/transfer-server", "/api/system"} {
		if r := editorGET(a, token, path); r.Code != http.StatusForbidden {
			t.Fatalf("scope widened %s: %d", path, r.Code)
		}
	}
	if r := editorHTTP(a, token, "/api/shutdown", nil); r.Code != http.StatusForbidden {
		t.Fatal("child shutdown")
	}
	if r := editorGET(a, token, "/api/status"); r.Code != 200 || strings.Contains(r.Body.String(), "system") || strings.Contains(r.Body.String(), "vault") {
		t.Fatal(r.Code, r.Body.String())
	}
	if r := editorGET(a, token, "/api/editor-views/current"); r.Code != 200 {
		t.Fatal(r.Code)
	}
	if r := editorHTTP(a, token, "/api/editor-views/transfers/"+req.ID+"/accept", nil); r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	if r := editorHTTP(a, token, "/api/editor-views/documents/"+req.Batch.Active+"/release", map[string]any{"lease": 2}); r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	if r := editorHTTP(a, token, "/api/editor-views/current/release", nil); r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	if !m.hasLiveViews() {
		t.Fatal("owner stopped before child could receive returned pulse")
	}
	if _, ok = m.token(dest); ok {
		t.Fatal("returned page bootstrapped again")
	}
	if r := editorGET(a, token, "/api/editor-views/current"); r.Code != 403 {
		t.Fatal("retired token scope", r.Code)
	}
	if r := editorHTTP(a, token, "/api/editor-views/current/pulse", nil); r.Code != 410 {
		t.Fatal("retired token cannot finish native child", r.Code)
	}
	m.processExited(dest)
	if _, ok = m.authenticate(token); ok {
		t.Fatal("dead child token retained")
	}
}

func TestEditorRegistryStrictSnapshotsAndAdditiveCheckpoint(t *testing.T) {
	a, m, _ := editorFixture(t)
	batch := editorFixtureBatch()
	leases, err := m.checkpoint("", batch, nil)
	if err != nil {
		t.Fatal(err)
	}
	second := editorFixtureBatch()
	second.Active = editorFixtureUID2
	second.Documents[0].UID = editorFixtureUID2
	second.Documents[0].ID = "second"
	leases, err = m.checkpoint("", second, nil)
	if err != nil || len(leases) != 2 {
		t.Fatal("single checkpoint removed other document", leases, err)
	}
	batch.Documents[0].Binding.Kind = "local"
	batch.Documents[0].Path = filepath.Join(t.TempDir(), "saved.txt")
	if _, err = m.checkpoint("", batch, map[string]uint64{batch.Active: 1}); err != nil {
		t.Fatal("local draft Save As transition", err)
	}
	changed := editorCloneBatch(batch)
	changed.Documents[0].Binding = editorBinding{Kind: "sshConnection", ConnectionID: "guessed"}
	if _, err = m.checkpoint("", changed, map[string]uint64{batch.Active: 1}); err == nil {
		t.Fatal("local binding changed to remote")
	}
	for _, mutate := range []func(*editorBatch){
		func(b *editorBatch) { b.Documents[0].UID = "bad" }, func(b *editorBatch) { b.Documents = append(b.Documents, b.Documents[0]) }, func(b *editorBatch) { b.Active = "missing" }, func(b *editorBatch) { b.Documents[0].Binding.Kind = "ftp" }, func(b *editorBatch) { b.Documents[0].Binding.ConnectionID = "unrelated" }, func(b *editorBatch) { b.Documents[0].Hash = "fixture-hash" }, func(b *editorBatch) { b.Documents[0].Undo = make([]string, 81) }, func(b *editorBatch) { b.Documents[0].Content = string([]byte{0}) }, func(b *editorBatch) { b.Documents[0].View = &editorSelection{Direction: "forward", Start: 2, End: 1} }, func(b *editorBatch) { b.Compare = &editorCompare{Left: b.Documents[0].ID, Right: "missing", Scale: 1} }, func(b *editorBatch) { b.Preferences = &editorPreferences{FontSize: 99, Palette: "auto"} },
	} {
		bad := editorCloneBatch(batch)
		mutate(&bad)
		if err = validateEditorBatch(bad); err == nil {
			t.Fatalf("accepted malformed snapshot %+v", bad.Documents[0].Binding)
		}
	}
	encoded, _ := json.Marshal(map[string]any{"id": randomHex(16), "destination": "new", "batch": batch, "password": "secret-canary"})
	r := httptest.NewRequest("POST", "http://"+a.host+"/api/editor-views/transfers", bytes.NewReader(encoded))
	r.Header.Set("X-Session-Token", a.token)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 400 || strings.Contains(w.Body.String(), "secret-canary") {
		t.Fatal("unknown credential fields accepted/leaked", w.Code, w.Body.String())
	}
}

func TestEditorRegistryScopedLocalFileLeaseAndHashChecks(t *testing.T) {
	a, m, _ := editorFixture(t)
	file := filepath.Join(t.TempDir(), "synthetic.txt")
	original := []byte("before")
	if err := os.WriteFile(file, original, 0600); err != nil {
		t.Fatal(err)
	}
	req := editorTransferRequest{ID: randomHex(16), Destination: "new", Batch: editorFixtureBatch()}
	req.Batch.Documents[0].Path = file
	req.Batch.Documents[0].Binding.Kind = "local"
	out, err := m.offer("", req)
	if err != nil {
		t.Fatal(err)
	}
	dest := out["destination"].(string)
	token, _ := m.token(dest)
	route := "/api/editor-views/documents/" + req.Batch.Active + "/file"
	payload := map[string]any{"lease": 2, "action": "write-text", "path": file, "content": "after", "encoding": "UTF-8", "expectedHash": editorDigest(original)}
	if r := editorHTTP(a, token, route, payload); r.Code != 409 {
		t.Fatal("staged destination wrote", r.Code)
	}
	if _, err = m.accept(dest, req.ID); err != nil {
		t.Fatal(err)
	}
	payload["lease"] = 1
	if r := editorHTTP(a, token, route, payload); r.Code != 409 {
		t.Fatal("stale lease wrote", r.Code)
	}
	payload["lease"] = 2
	payload["expectedHash"] = strings.Repeat("0", 64)
	if r := editorHTTP(a, token, route, payload); r.Code != 409 {
		t.Fatal("bad hash overwrote", r.Code)
	}
	payload["expectedHash"] = editorDigest(original)
	if r := editorHTTP(a, token, route, payload); r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	if contents, _ := os.ReadFile(file); string(contents) != "after" {
		t.Fatal("scoped save failed")
	}
	newPath := filepath.Join(filepath.Dir(file), "saved-copy.txt")
	payload["path"], payload["expectedHash"] = newPath, ""
	if r := editorHTTP(a, token, route, payload); r.Code != 200 {
		t.Fatal("Save As failed", r.Code, r.Body.String())
	}
	if r := editorHTTP(a, token, route, map[string]any{"lease": 2, "action": "read-text", "path": newPath}); r.Code != 200 {
		t.Fatal("immediate Save As reload failed before checkpoint", r.Code, r.Body.String())
	}
	if r := editorHTTP(a, token, route, map[string]any{"lease": 2, "action": "read-text", "path": file}); r.Code != 409 {
		t.Fatal("old path retained read authority after Save As", r.Code)
	}
	if r := editorHTTP(a, token, "/api/file-editor/local", map[string]any{"action": "write-text", "path": file, "content": "bypass"}); r.Code != 403 {
		t.Fatal("generic child write bypass", r.Code)
	}
	if r := editorHTTP(a, token, "/api/file-editor/local", map[string]any{"action": "read-text", "path": file}); r.Code != 200 {
		t.Fatal("explicit local open denied", r.Code)
	}
	payload["password"] = "secret"
	if r := editorHTTP(a, token, route, payload); r.Code != 400 {
		t.Fatal("proxy accepted auth field", r.Code)
	}
}

func TestEditorRegistryConfirmedExitRecoversButMissedPulseDoesNot(t *testing.T) {
	_, m, _ := editorFixture(t)
	req, _, dest := editorFixtureOffer(t, m)
	if _, err := m.accept(dest, req.ID); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.views[dest].lastPulse = m.now().Add(-time.Hour)
	m.expireLocked()
	m.mu.Unlock()
	if m.documents[req.Batch.Active].owner != dest {
		t.Fatal("missed heartbeat stole live editor")
	}
	m.processExited(dest)
	d := m.documents[req.Batch.Active]
	if d.owner != "" || d.lease != 3 || !m.recovery[req.Batch.Active] {
		t.Fatal("confirmed crash lost last checkpoint")
	}
	if err := m.beginHomeClose(); err == nil {
		t.Fatal("discarded unacknowledged recovery")
	}
	if _, err := m.checkpoint("", req.Batch, map[string]uint64{req.Batch.Active: 3}); err != nil {
		t.Fatal(err)
	}
	if len(m.recovery) != 0 {
		t.Fatal("restored recovery not acknowledged")
	}
}

func TestEditorRegistryConcurrentAcceptCancelAndCheckpoint(t *testing.T) {
	_, m, _ := editorFixture(t)
	req, _, dest := editorFixtureOffer(t, m)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(3)
		go func() { defer wg.Done(); _, _ = m.accept(dest, req.ID) }()
		go func() { defer wg.Done(); _, _ = m.cancel("", req.ID) }()
		go func() { defer wg.Done(); _ = m.state(dest) }()
	}
	wg.Wait()
	d := m.documents[req.Batch.Active]
	if d.owner == dest {
		if d.lease != 2 || m.transfers[req.ID].state != "accepted" {
			t.Fatal("non-atomic acceptance")
		}
	} else if d.owner != "" || d.lease != 1 || m.transfers[req.ID].state != "cancelled" {
		t.Fatal("non-atomic cancellation")
	}
}

func TestEditorRegistryPinnedOperationFailsClosedMidWriteAndBlocksHandoff(t *testing.T) {
	a, m, _ := editorFixture(t)
	req, _, dest := editorFixtureOffer(t, m)
	_, _ = m.accept(dest, req.ID)
	token, _ := m.token(dest)
	d := m.documents[req.Batch.Active]
	var valid atomic.Bool
	valid.Store(true)
	d.snapshot.Binding = editorBinding{Kind: "sshConnection", ConnectionID: "synthetic-only"}
	d.valid = valid.Load
	calls := []string{}
	d.run = func(_ context.Context, r remoteBrowserRequest) (model.Result, error) {
		calls = append(calls, r.Action)
		if r.Action == "stat" {
			valid.Store(false)
			return model.Result{Data: map[string]any{"sizeBytes": int64(5)}}, nil
		}
		t.Fatal("new/replaced transport was used after binding changed")
		return model.Result{}, nil
	}
	out := editorHTTP(a, token, "/api/editor-views/documents/"+req.Batch.Active+"/file", map[string]any{"lease": 2, "action": "write-text", "path": d.snapshot.Path, "content": "after", "encoding": "UTF-8", "expectedHash": strings.Repeat("a", 64)})
	if out.Code == 200 || len(calls) != 1 || calls[0] != "stat" {
		t.Fatal("write continued after reconnect", out.Code, calls)
	}
	valid.Store(true)
	started, finish := make(chan struct{}), make(chan struct{})
	d.run = func(context.Context, remoteBrowserRequest) (model.Result, error) {
		close(started)
		<-finish
		return model.Result{Data: map[string]any{"contentBytes": []byte("last read")}}, nil
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- editorHTTP(a, token, "/api/editor-views/documents/"+req.Batch.Active+"/file", map[string]any{"lease": 2, "action": "read-text", "path": d.snapshot.Path})
	}()
	<-started
	if got := a.activitySnapshot().Tools["sftp"].Operations; got != 1 {
		t.Fatalf("pinned remote operation count=%d", got)
	}
	move := editorTransferRequest{ID: randomHex(16), Destination: "main", Batch: req.Batch, Leases: map[string]uint64{req.Batch.Active: 2}}
	move.Batch.Documents[0].Binding = d.snapshot.Binding
	if _, err := m.offer(dest, move); err == nil {
		t.Fatal("moved document during save/read")
	}
	m.processExited(dest)
	m.mu.Lock()
	if !d.orphan || d.owner != dest {
		t.Fatal("reclaimed document during in-flight write")
	}
	m.mu.Unlock()
	close(finish)
	<-done
	if got := a.activitySnapshot().Tools["sftp"].Operations; got != 0 {
		t.Fatalf("completed pinned operation count=%d", got)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if d.owner != "" || !m.recovery[req.Batch.Active] {
		t.Fatal("quiescent orphan not recovered")
	}
}

func TestEditorRegistryExistingSSHBindingRejectsReplacement(t *testing.T) {
	a, m, _ := editorFixture(t)
	base := t.TempDir()
	servers := newTransferHostManager(a.ctx, base)
	defer servers.Close()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	status, err := servers.start(transferServerConfig{Protocol: "sftp", Bind: "127.0.0.1", Port: port, Root: filepath.Join(base, "synthetic"), Access: "readwrite", Username: "fixture", Password: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := a.openBrowserConnection(httptest.NewRequest("POST", "/api/remote-browser", nil), remoteBrowserRequest{Protocol: "ssh", Host: "127.0.0.1", Port: port, Username: "fixture", Password: "fixture", AuthMode: "password", Fingerprint: status["fingerprint"].(string)})
	if err != nil {
		t.Fatal(err)
	}
	defer a.closeBrowserConnections()
	batch := editorFixtureBatch()
	batch.Documents[0].Binding = editorBinding{Kind: "sshConnection", ConnectionID: id}
	grant, err := m.makeGrant("", batch.Documents[0])
	if err != nil || !grant.valid() {
		t.Fatal(err)
	}
	if _, err = m.makeGrant("child", batch.Documents[0]); err == nil {
		t.Fatal("child delegated arbitrary remote ID")
	}
	a.browserMu.Lock()
	entry := a.browserConnections[id]
	a.browserConnections[id] = &browserFileConnection{client: entry.client, cancel: entry.cancel}
	a.browserMu.Unlock()
	if grant.valid() {
		t.Fatal("replaced SSH registry identity remained valid")
	}
	a.browserMu.Lock()
	a.browserConnections[id] = entry
	a.browserMu.Unlock()
	entry.client.Close()
	if grant.valid() {
		t.Fatal("closed SSH transport remained valid")
	}
}

func TestEditorRegistryPreparedMainCloseIsReversibleAndBlocksNewMoves(t *testing.T) {
	a, m, _ := editorFixture(t)
	req, _, destination := editorFixtureOffer(t, m)
	_, _ = m.accept(destination, req.ID)
	token, _ := m.token(destination)
	path := "/api/editor-views/current/prepare-close"
	if r := editorHTTP(a, token, path, nil); r.Code != 403 {
		t.Fatal("child prepared main close", r.Code)
	}
	if r := editorHTTP(a, a.token, path, nil); r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	if !m.homeClosing || a.shutdownRequested.Load() {
		t.Fatal("prepare close had irreversible shutdown effects")
	}
	returned := editorTransferRequest{ID: randomHex(16), Destination: "main", Batch: req.Batch, Leases: map[string]uint64{req.Batch.Active: 2}}
	if _, err := m.offer(destination, returned); err == nil {
		t.Fatal("destination accepted new transfer during dirty consent")
	}
	if _, err := m.checkpoint("", editorFixtureBatch(), nil); err == nil {
		t.Fatal("main began editing while close prepared")
	}
	if r := editorHTTP(a, a.token, "/api/editor-views/current/cancel-close", nil); r.Code != 200 || m.homeClosing {
		t.Fatal("Cancel did not restore main", r.Code)
	}
	if m.documents[req.Batch.Active].owner != destination {
		t.Fatal("Cancel changed document owner")
	}
	if _, err := m.offer(destination, returned); err != nil {
		t.Fatal("Cancel failed to restore transfers", err)
	}
	if r := editorHTTP(a, a.token, path, nil); r.Code != 409 {
		t.Fatal("main prepared close during incoming transfer", r.Code)
	}
	if _, err := m.accept("", returned.ID); err != nil {
		t.Fatal("main could not accept returned documents", err)
	}
	if r := editorHTTP(a, a.token, path, nil); r.Code != 200 {
		t.Fatal("main could not prepare final close", r.Code)
	}
	if err := m.beginHomeClose(); err != nil {
		t.Fatal("shutdown's repeated prepare was not idempotent", err)
	}
}

func TestEditorRegistryBoundedRecoveryAndPreferences(t *testing.T) {
	_, m, _ := editorFixture(t)
	batch := editorFixtureBatch()
	batch.Preferences = &editorPreferences{WordWrap: true, Syntax: true, Indent: 2, FontSize: 13, Palette: "custom", Colors: map[string]string{"bg": "#123456", "fg": "#abcdef"}}
	if err := validateEditorBatch(batch); err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"light", "dark"} {
		batch.Preferences.AppTheme = theme
		if err := validateEditorBatch(batch); err != nil {
			t.Fatal("supported application theme rejected", err)
		}
	}
	batch.Preferences.AppTheme = "system"
	if err := validateEditorBatch(batch); err == nil {
		t.Fatal("unsupported application theme accepted")
	}
	batch.Preferences.AppTheme = ""
	batch.Preferences.Colors["password"] = "#123456"
	if err := validateEditorBatch(batch); err == nil {
		t.Fatal("unknown preference key accepted")
	}
	delete(batch.Preferences.Colors, "password")
	batch.Documents[0].Content = strings.Repeat("x", editorDocumentLimit+1)
	if err := validateEditorBatch(batch); err == nil {
		t.Fatal("oversize document accepted")
	}
	batch = editorFixtureBatch()
	// Three independently serialized fields use six JSON bytes per control character.
	// Exceed the aggregate without also exceeding any individual document bound.
	escapedCount := editorTransferLimit/(3*6) + 1
	if escapedCount > editorDocumentLimit {
		t.Fatal("aggregate fixture exceeds individual document bound")
	}
	batch.Documents[0].Content = strings.Repeat("\x01", escapedCount)
	batch.Documents[0].Saved = batch.Documents[0].Content
	batch.Documents[0].Original = batch.Documents[0].Content
	if err := validateEditorBatch(batch); err == nil {
		t.Fatal("escaped aggregate exceeds budget but accepted")
	}
	if len(m.documents) != 0 {
		t.Fatal("validation mutated registry")
	}
}

func TestEditorRegistryUnapprovedPrimaryExitKeepsAcknowledgedRecovery(t *testing.T) {
	for _, approved := range []bool{false, true} {
		t.Run(map[bool]string{false: "unapproved", true: "approved"}[approved], func(t *testing.T) {
			_, m, _ := editorFixture(t)
			batch := editorFixtureBatch()
			if _, err := m.checkpoint("", batch, nil); err != nil {
				t.Fatal(err)
			}
			if !m.hasLiveViews() {
				t.Fatal("unapproved main checkpoint did not retain backend")
			}
			if err := m.beginHomeClose(); err != nil {
				t.Fatal(err)
			}
			if approved {
				if err := m.approveHomeClose(); err != nil {
					t.Fatal(err)
				}
			}
			m.setHomePresent(false)
			if approved {
				if len(m.documents) != 0 || m.hasRecovery() || m.hasLiveViews() {
					t.Fatal("approved discard retained ordinary main document")
				}
				return
			}
			if !m.hasRecovery() || !m.hasLiveViews() || m.documents[batch.Active].lease != 2 {
				t.Fatal("prepared-but-unapproved close discarded checkpoint")
			}
			m.setHomePresent(true)
			if m.homeCloseApproved {
				t.Fatal("reopened main retained old discard consent")
			}
			if _, err := m.checkpoint("", batch, map[string]uint64{batch.Active: 2}); err != nil {
				t.Fatal(err)
			}
			if m.hasRecovery() {
				t.Fatal("restoration did not acknowledge recovery")
			}
		})
	}
}

func TestEditorRegistryApprovedShutdownCannotBeCancelled(t *testing.T) {
	a, m, _ := editorFixture(t)
	if err := m.approveHomeClose(); err == nil {
		t.Fatal("unprepared shutdown accepted")
	}
	if err := m.beginHomeClose(); err != nil {
		t.Fatal(err)
	}
	if err := m.approveHomeClose(); err != nil {
		t.Fatal(err)
	}
	if r := editorHTTP(a, a.token, "/api/editor-views/current/cancel-close", nil); r.Code != 409 {
		t.Fatal("accepted shutdown cancellation", r.Code)
	}
	if !m.homeClosing || !m.homeCloseApproved {
		t.Fatal("lost shutdown response reversed accepted consent")
	}
	if _, err := m.offer("", editorTransferRequest{ID: randomHex(16), Destination: "new", Batch: editorFixtureBatch()}); err == nil {
		t.Fatal("approved shutdown admitted a new transfer")
	}
}

func TestEditorRegistryUnreceivedACKSurvivesFullLedger(t *testing.T) {
	_, m, _ := editorFixture(t)
	first, _, one := editorFixtureOffer(t, m)
	if _, err := m.accept(one, first.ID); err != nil {
		t.Fatal(err)
	}
	batch := editorFixtureBatch()
	batch.Active = editorFixtureUID2
	batch.Documents[0].UID = editorFixtureUID2
	batch.Documents[0].ID = "second-document"
	req := editorTransferRequest{ID: randomHex(16), Destination: "new", Batch: batch}
	out, err := m.offer("", req)
	if err != nil {
		t.Fatal(err)
	}
	two := out["destination"].(string)
	if _, err = m.accept(two, req.ID); err != nil {
		t.Fatal(err)
	}
	owner := two
	for len(m.order) < 64 {
		destination := one
		if owner == one {
			destination = two
		}
		req = editorTransferRequest{ID: randomHex(16), Destination: destination, Batch: batch, Leases: map[string]uint64{batch.Active: m.documents[batch.Active].lease}}
		if _, err = m.offer(owner, req); err != nil {
			t.Fatal(err)
		}
		if _, err = m.accept(destination, req.ID); err != nil {
			t.Fatal(err)
		}
		owner = destination
	}
	destination := one
	if owner == one {
		destination = two
	}
	next := editorTransferRequest{ID: randomHex(16), Destination: destination, Batch: batch, Leases: map[string]uint64{batch.Active: m.documents[batch.Active].lease}}
	if _, err = m.offer(owner, next); err == nil {
		t.Fatal("full unresolved ledger silently erased ownership evidence")
	}
	replay, err := m.offer("", first)
	if err != nil || replay["state"] != "accepted" {
		t.Fatal("lost original ACK was not recoverable", err)
	}
	if err = m.complete(one, first.ID); err == nil {
		t.Fatal("destination forged source receipt")
	}
	if err = m.complete("", first.ID); err != nil {
		t.Fatal(err)
	}
	if err = m.complete("", first.ID); err != nil {
		t.Fatal("receipt not idempotent", err)
	}
	if _, err = m.offer(owner, next); err != nil {
		t.Fatal("acknowledged ledger did not free bounded slot", err)
	}
	if m.transfers[first.ID] != nil || len(m.order) != 64 {
		t.Fatal("only acknowledged record should be pruned")
	}
	if err = m.complete(owner, next.ID); err == nil {
		t.Fatal("pending offer receipt accepted")
	}
}

func TestEditorComparisonPreferencesOptionalBooleans(t *testing.T) {
	for _, value := range []bool{false, true} {
		batch := editorFixtureBatch()
		batch.Preferences = &editorPreferences{FontSize: 13, Indent: 4, Palette: "auto", Syntax: true, CompareHorizontalSync: value, CompareUnderline: &value, CompareIntralineHighlight: &value}
		if err := validateEditorBatch(batch); err != nil {
			t.Fatal(err)
		}
		clone := editorCloneBatch(batch)
		if clone.Preferences.CompareHorizontalSync != value || clone.Preferences.CompareUnderline == nil || *clone.Preferences.CompareUnderline != value || clone.Preferences.CompareIntralineHighlight == nil || *clone.Preferences.CompareIntralineHighlight != value {
			t.Fatal("comparison preference lost")
		}
	}
	var old editorPreferences
	if err := json.Unmarshal([]byte(`{"fontSize":13,"palette":"auto"}`), &old); err != nil || old.CompareUnderline != nil || old.CompareIntralineHighlight != nil || old.CompareHorizontalSync {
		t.Fatal("legacy omitted defaults changed", err)
	}
	for _, field := range []string{"compareHorizontalSync", "compareUnderline", "compareIntralineHighlight"} {
		var bad editorPreferences
		if err := json.Unmarshal([]byte(`{"`+field+`":"false"}`), &bad); err == nil {
			t.Fatal("non-boolean accepted", field)
		}
	}
}
