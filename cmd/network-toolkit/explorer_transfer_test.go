package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"networktoolkit/internal/model"
)

func exportRequest(a *application, owner, method, route string, body any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, route, bytes.NewReader(raw))
	if owner != "" {
		r = withEditorRequestView(r, owner)
	}
	w := httptest.NewRecorder()
	a.handleExplorerTransfer(w, r)
	return w
}
func waitExport(t *testing.T, a *application, id string) *explorerTransferJob {
	t.Helper()
	end := time.Now().Add(3 * time.Second)
	for time.Now().Before(end) {
		a.explorerTransfers.mu.Lock()
		j := a.explorerTransfers.jobs[id]
		state := j.state
		a.explorerTransfers.mu.Unlock()
		if state != "preparing" {
			return j
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("export did not finish")
	return nil
}
func TestExplorerCopyLocalTreeTicketAndCleanup(t *testing.T) {
	a := &application{}
	root := t.TempDir()
	src := filepath.Join(root, "folder")
	os.MkdirAll(filepath.Join(src, "empty"), 0700)
	os.WriteFile(filepath.Join(src, "one.txt"), []byte("SYNTHETIC\r\n"), 0600)
	w := exportRequest(a, "", "POST", "/api/explorer-transfer", explorerTransferRequest{Action: "prepare", Binding: editorBinding{Kind: "local"}, Paths: []string{src}})
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	id := out["id"].(string)
	j := waitExport(t, a, id)
	if j.state != "ready" || j.bytes != 11 || j.entries != 3 {
		t.Fatal(j.state, j.note, j.bytes, j.entries)
	}
	stage := j.stage
	t.Cleanup(func() { os.RemoveAll(stage) })
	for _, value := range []string{src, stage, "password", "connectionId"} {
		if strings.Contains(w.Body.String(), value) {
			t.Fatal("metadata leaked", value)
		}
	}
	if got := exportRequest(a, "", "GET", "/api/explorer-transfer/native", nil); got.Code != 204 {
		t.Fatal("unarmed native request consumed job")
	}
	if got := exportRequest(a, "", "POST", "/api/explorer-transfer", map[string]any{"action": "arm", "id": id, "gesture": 1}); got.Code != 200 {
		t.Fatal(got.Body.String())
	}
	if got := exportRequest(a, "", "GET", "/api/explorer-transfer/native", nil); got.Code != 200 {
		t.Fatal(got.Body.String())
	}
	if got := exportRequest(a, "", "GET", "/api/explorer-transfer/native", nil); got.Code != 200 {
		t.Fatal("lost claim response cannot reconcile")
	}
	if got := exportRequest(a, "", "POST", "/api/explorer-transfer", map[string]any{"action": "start", "id": id, "gesture": 1}); got.Code != 200 {
		t.Fatal(got.Body.String())
	}
	if got := exportRequest(a, "", "POST", "/api/explorer-transfer", map[string]any{"action": "start", "id": id, "gesture": 1}); got.Code != 200 {
		t.Fatal("lost start response cannot reconcile")
	}
	if got := exportRequest(a, "", "GET", "/api/explorer-transfer/native", nil); got.Code != 204 {
		t.Fatal("active drag claimed twice")
	}
	if got := exportRequest(a, "", "POST", "/api/explorer-transfer", map[string]string{"action": "cancel", "id": id}); got.Code != 409 {
		t.Fatal("cancel deleted active drag")
	}
	got := exportRequest(a, "", "POST", "/api/explorer-transfer", map[string]string{"action": "finish", "id": id, "outcome": "copied"})
	if got.Code != 200 {
		t.Fatal(got.Body.String())
	}
	if got := exportRequest(a, "", "POST", "/api/explorer-transfer", map[string]string{"action": "finish", "id": id, "outcome": "copied"}); got.Code != 200 {
		t.Fatal("lost finish response cannot reconcile")
	}
	if _, e := os.Stat(stage); !os.IsNotExist(e) {
		t.Fatal("staging not removed")
	}
	if b, e := os.ReadFile(filepath.Join(src, "one.txt")); e != nil || string(b) != "SYNTHETIC\r\n" {
		t.Fatal("original changed")
	}
}
func TestExplorerCopySafetyAndCancellation(t *testing.T) {
	for _, name := range []string{"../bad", "C:\\bad", "a.lnk", "NUL.txt"} {
		t.Run(name, func(t *testing.T) {
			if validateExplorerDropName(name) == nil {
				t.Fatal("accepted unsafe name")
			}
		})
	}
	a := &application{}
	src := filepath.Join(t.TempDir(), "source")
	os.Mkdir(src, 0700)
	os.WriteFile(filepath.Join(src, "okay"), []byte("x"), 0600)
	os.Symlink(t.TempDir(), filepath.Join(src, "link"))
	ctx, cancel := context.WithCancel(context.Background())
	j := &explorerTransferJob{valid: func() bool { return true }}
	dest := filepath.Join(t.TempDir(), "dest")
	err := a.stageExplorerLocal(ctx, j, src, dest, 0)
	if err == nil {
		t.Fatal("followed source link")
	}
	cancel()
	if err = a.stageExplorerLocal(ctx, j, src, filepath.Join(t.TempDir(), "cancel"), 0); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err = a.exportAddBytes(&explorerTransferJob{planned: explorerExportMaxBytes}, 1); err == nil {
		t.Fatal("unbounded bytes")
	}
	if err = a.exportCheck(context.Background(), &explorerTransferJob{entries: explorerExportMaxEntries, valid: func() bool { return true }}, 0); err == nil {
		t.Fatal("unbounded entries")
	}
	if err = a.exportCheck(context.Background(), &explorerTransferJob{valid: func() bool { return true }}, 33); err == nil {
		t.Fatal("unbounded depth")
	}
}
func TestExplorerCopyRejectsForeignGrantAndAuthPayload(t *testing.T) {
	a, m, _ := editorFixture(t)
	_, _, child := editorFixtureOffer(t, m)
	for _, req := range []any{map[string]any{"action": "prepare", "binding": map[string]string{"kind": "local"}, "paths": []string{t.TempDir()}, "password": "SYNTHETIC_SECRET"}, explorerTransferRequest{Action: "prepare", Binding: editorBinding{Kind: "sshConnection", ConnectionID: "guessed"}, Paths: []string{"/file"}}} {
		w := exportRequest(a, "", "POST", "/api/explorer-transfer", req)
		if w.Code == 202 {
			t.Fatal("accepted credentials/guessed session")
		}
	}
	if w := exportRequest(a, child, "POST", "/api/explorer-transfer", explorerTransferRequest{Action: "prepare", Binding: editorBinding{Kind: "local"}, Paths: []string{t.TempDir()}}); w.Code != 403 {
		t.Fatal("unapproved child gained filesystem access")
	}
	if w := exportRequest(a, "", "DELETE", "/api/explorer-transfer", nil); w.Code != 405 {
		t.Fatal("method guard")
	}
}
func TestExplorerCopyRemotePinnedTraversal(t *testing.T) {
	a := &application{}
	j := &explorerTransferJob{valid: func() bool { return true }}
	calls := 0
	g := &editorDocumentGrant{valid: func() bool { return true }, run: func(_ context.Context, r remoteBrowserRequest) (model.Result, error) {
		calls++
		if !r.SafeExport {
			t.Fatal("unbounded download")
		}
		data := map[string]any{}
		switch r.Action {
		case "stat":
			if r.Path == "/folder" {
				data["kind"] = "directory"
			} else {
				data["kind"] = "file"
				data["sizeBytes"] = int64(4)
			}
		case "list":
			data["entries"] = []map[string]any{{"name": "item", "kind": "File"}}
		case "download":
			if r.ExportSize != 4 {
				t.Fatal("Expected size not pinned", r.ExportSize)
			}
			if err := os.WriteFile(r.LocalPath, []byte("DATA"), 0600); err != nil {
				return model.Result{}, err
			}
		default:
			t.Fatal(r.Action)
		}
		return model.Result{Data: data}, nil
	}}
	dest := filepath.Join(t.TempDir(), "copy")
	if err := a.stageExplorerRemote(context.Background(), j, g, "/folder", dest, 0); err != nil {
		t.Fatal(err)
	}
	if calls != 4 || j.bytes != 4 {
		t.Fatal(calls, j.bytes)
	}
	g.run = func(context.Context, remoteBrowserRequest) (model.Result, error) {
		return model.Result{Data: map[string]any{"kind": "link"}}, nil
	}
	if err := a.stageExplorerRemote(context.Background(), j, g, "/link", filepath.Join(t.TempDir(), "out"), 0); err == nil {
		t.Fatal("remote link followed")
	}
}
func TestExplorerLocalBrowserByteUpload(t *testing.T) {
	a := &application{}
	root := t.TempDir()
	send := func(name string, overwrite bool) *httptest.ResponseRecorder {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		meta, _ := json.Marshal(map[string]any{"path": root, "name": name, "overwrite": overwrite})
		mw.WriteField("destination", string(meta))
		f, _ := mw.CreateFormFile("file", "source.txt")
		io.WriteString(f, "SYNTHETIC_BYTES")
		mw.Close()
		r := httptest.NewRequest(http.MethodPost, "/api/transfer/local-drop", &body)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		w := httptest.NewRecorder()
		a.handleLocalExplorerDrop(w, r)
		return w
	}
	if w := send("target.txt", false); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := send("target.txt", false); w.Code == 200 {
		t.Fatal("silent overwrite")
	}
	if w := send("target.txt", true); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := send("../escape", true); w.Code == 200 {
		t.Fatal("escaped destination")
	}
}

func TestExplorerCopyRealRetainedSFTP(t *testing.T) {
	a, m, _ := editorFixture(t)
	base := t.TempDir()
	servers := newTransferHostManager(a.ctx, base)
	defer servers.Close()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	root := filepath.Join(base, "synthetic")
	os.MkdirAll(filepath.Join(root, "folder", "empty"), 0700)
	os.WriteFile(filepath.Join(root, "folder", "actual.txt"), []byte("SYNTHETIC_LOOPBACK114"), 0600)
	status, err := servers.start(transferServerConfig{Protocol: "sftp", Bind: "127.0.0.1", Port: port, Root: root, Access: "readwrite", Username: "fixture", Password: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := a.openBrowserConnection(httptest.NewRequest("POST", "/api/remote-browser", nil), remoteBrowserRequest{Protocol: "ssh", Host: "127.0.0.1", Port: port, Username: "fixture", Password: "fixture", AuthMode: "password", Fingerprint: status["fingerprint"].(string)})
	if err != nil {
		t.Fatal(err)
	}
	defer a.closeBrowserConnections()
	g, err := m.makeGrant("", editorDocumentSnapshot{Binding: editorBinding{Kind: "sshConnection", ConnectionID: id}})
	if err != nil {
		t.Fatal(err)
	}
	j := &explorerTransferJob{valid: g.valid}
	out := filepath.Join(t.TempDir(), "export")
	if err = a.stageExplorerRemote(a.ctx, j, g, "/folder", out, 0); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "actual.txt"))
	if err != nil || string(b) != "SYNTHETIC_LOOPBACK114" {
		t.Fatal("wire export bytes", string(b), err)
	}
	if info, err := os.Stat(filepath.Join(out, "empty")); err != nil || !info.IsDir() {
		t.Fatal("empty remote folder lost")
	}

	originalRun := g.run
	g.run = func(ctx context.Context, r remoteBrowserRequest) (model.Result, error) {
		if r.Action == "download" {
			os.WriteFile(filepath.Join(root, "folder", "actual.txt"), []byte("SYNTHETIC_LOOPBACK114_GREW_AFTER_STAT"), 0600)
		}
		return originalRun(ctx, r)
	}
	changed := filepath.Join(t.TempDir(), "changed")
	if err := a.stageExplorerRemote(a.ctx, &explorerTransferJob{valid: g.valid}, g, "/folder/actual.txt", changed, 0); err == nil {
		t.Fatal("remote growth escaped aggregate size reservation")
	}
	if _, err := os.Stat(changed); !os.IsNotExist(err) {
		t.Fatal("changed remote file published")
	}
	g.run = originalRun
	a.browserMu.Lock()
	entry := a.browserConnections[id]
	a.browserConnections[id] = &browserFileConnection{client: entry.client, cancel: entry.cancel}
	a.browserMu.Unlock()
	if err = a.stageExplorerRemote(a.ctx, j, g, "/folder", filepath.Join(t.TempDir(), "stale"), 0); err == nil {
		t.Fatal("stale SSH binding reused")
	}
	a.browserMu.Lock()
	a.browserConnections[id] = entry
	a.browserMu.Unlock()
}

func TestExplorerCopyGestureReleaseAndLostAcknowledgements(t *testing.T) {
	a := &application{}
	j := &explorerTransferJob{id: strings.Repeat("a", 32), state: "ready", expires: time.Now().Add(time.Minute), valid: func() bool { return true }, cancel: func() {}}
	a.explorerTransfers.jobs = map[string]*explorerTransferJob{j.id: j}
	call := func(action string, gesture int) *httptest.ResponseRecorder {
		return exportRequest(a, "", "POST", "/api/explorer-transfer", map[string]any{"action": action, "id": j.id, "gesture": gesture})
	}
	if call("disarm", 1).Code != 200 || call("arm", 1).Code != 409 {
		t.Fatal("delayed arm resurrected ended gesture")
	}
	if call("arm", 2).Code != 200 {
		t.Fatal("new gesture refused")
	}
	exportRequest(a, "", "GET", "/api/explorer-transfer/native", nil)
	if call("start", 2).Code != 200 {
		t.Fatal("start failed")
	}
	call("disarm", 2)
	if call("start", 2).Code != 200 {
		t.Fatal("lost start response stranded active job after release")
	}
	if call("finish", 2).Code != 200 || call("finish", 2).Code != 200 {
		t.Fatal("finish cannot reconcile")
	}
	j.state = "ready"
	if call("arm", 3).Code != 200 {
		t.Fatal("arm")
	}
	exportRequest(a, "", "GET", "/api/explorer-transfer/native", nil)
	j.armed = time.Now().Add(-time.Second)
	if got := exportRequest(a, "", "GET", "/api/explorer-transfer/native", nil); got.Code != 204 || j.state != "ready" {
		t.Fatal("lost claim did not expire safely")
	}
}
func importAction(a *application, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	r := httptest.NewRequest("POST", "/api/transfer/local-drop", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	a.handleLocalExplorerDrop(w, r)
	return w
}
func TestExplorerLocalRecursiveRootAndAncestorSwap(t *testing.T) {
	a := &application{}
	root, outside := t.TempDir(), t.TempDir()
	begin := importAction(a, map[string]any{"action": "begin", "path": root})
	if begin.Code != 200 {
		t.Fatal(begin.Body.String())
	}
	var result map[string]string
	json.Unmarshal(begin.Body.Bytes(), &result)
	id := result["id"]
	defer importAction(a, map[string]any{"action": "end", "id": id})
	mkdir := func(p string) *httptest.ResponseRecorder {
		return importAction(a, map[string]any{"action": "mkdir", "id": id, "path": p})
	}
	if mkdir("copy").Code != 200 || mkdir("copy/nested").Code != 200 {
		t.Fatal("create")
	}
	if mkdir("copy").Code == 200 {
		t.Fatal("merged existing folder")
	}
	os.Rename(filepath.Join(root, "copy"), filepath.Join(root, "original"))
	if err := os.Symlink(outside, filepath.Join(root, "copy")); err != nil {
		t.Skip("symlink unavailable", err)
	}
	if mkdir("copy/escape").Code == 200 {
		t.Fatal("ancestor swap traversed")
	}
	if _, err := os.Stat(filepath.Join(outside, "escape")); !os.IsNotExist(err) {
		t.Fatal("outside root changed")
	}
	j, _ := a.importSession("", id)
	j.mu.Lock()
	if j.verify("copy/nested") == nil {
		t.Fatal("replaced nested destination accepted")
	}
	j.mu.Unlock()
	// Replacing the selected root itself also cannot redirect its retained handle.
	moved := root + "-moved"
	os.Remove(filepath.Join(root, "copy"))
	os.Rename(root, moved)
	defer os.RemoveAll(moved)
	os.Symlink(outside, root)
	defer os.Remove(root)
	if mkdir("new").Code == 200 {
		t.Fatal("root replacement accepted")
	}
	if _, err := os.Stat(filepath.Join(outside, "new")); !os.IsNotExist(err) {
		t.Fatal("outside root mutated")
	}
}

type explorerRevokingReader struct {
	io.Reader
	revoke func()
}

func (r *explorerRevokingReader) Read(b []byte) (int, error) {
	if r.revoke != nil {
		r.revoke()
		r.revoke = nil
	}
	return r.Reader.Read(b)
}
func TestExplorerLocalDropRevocationBeforePublication(t *testing.T) {
	a, m, _ := editorFixture(t)
	_, _, child := editorFixtureOffer(t, m)
	m.mu.Lock()
	m.views[child].explorer = &editorExplorerAccess{Allowed: true}
	m.mu.Unlock()
	root := t.TempDir()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	metadata, _ := json.Marshal(map[string]any{"path": root, "name": "copy.txt"})
	mw.WriteField("destination", string(metadata))
	file, _ := mw.CreateFormFile("file", "source.txt")
	io.WriteString(file, "SYNTHETIC")
	mw.Close()
	reader := &explorerRevokingReader{Reader: &body, revoke: func() { m.mu.Lock(); m.views[child].explorer = &editorExplorerAccess{Allowed: true}; m.mu.Unlock() }}
	r := withEditorRequestView(httptest.NewRequest("POST", "/api/transfer/local-drop", reader), child)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	a.handleLocalExplorerDrop(w, r)
	if w.Code == 200 {
		t.Fatal("old approval published under replacement approval")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("revoked copy left published/staged file", entries)
	}
}
