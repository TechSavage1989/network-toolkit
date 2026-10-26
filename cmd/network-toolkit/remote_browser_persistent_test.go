package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"mime/multipart"
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

	"golang.org/x/crypto/ssh"
)

type persistentBrowserFixture struct {
	app         *application
	server      *transferHostManager
	root        string
	credentials map[string]any
	connections *atomic.Int32
	logins      *atomic.Int32
}

func newPersistentBrowserFixture(t *testing.T) *persistentBrowserFixture {
	t.Helper()
	app := testApplication(t)
	server := newTransferHostManager(context.Background(), t.TempDir())
	t.Cleanup(server.Close)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "startup.cfg"), []byte("hostname test-switch\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	status, err := server.start(transferServerConfig{Protocol: "sftp", Bind: "127.0.0.1", Port: port, Root: root, Access: "readwrite", Overwrite: true, Username: "tester", Password: "persistent-test-password"})
	if err != nil {
		t.Fatal(err)
	}
	return &persistentBrowserFixture{app: app, server: server, root: root, credentials: map[string]any{
		"protocol": "ssh", "host": "127.0.0.1", "port": port, "username": "tester", "password": "persistent-test-password", "authMode": "password", "fingerprint": status["fingerprint"],
	}}
}

func (f *persistentBrowserFixture) call(t *testing.T, fields map[string]any) (int, map[string]any) {
	t.Helper()
	payload, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	response := requestApp(f.app, http.MethodPost, "/api/remote-browser", string(payload), true)
	var result map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("invalid browser response (%d): %s", response.Code, response.Body.String())
	}
	for _, forbidden := range []string{"persistent-test-password", "private-key-canary", "passphrase-canary"} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("authentication secret exposed in browser response: %s", response.Body.String())
		}
	}
	return response.Code, result
}

func (f *persistentBrowserFixture) connect(t *testing.T, overrides map[string]any) string {
	t.Helper()
	fields := map[string]any{"action": "connect", "path": "."}
	for key, value := range f.credentials {
		fields[key] = value
	}
	for key, value := range overrides {
		fields[key] = value
	}
	code, body := f.call(t, fields)
	if code != http.StatusOK {
		t.Fatalf("persistent connection failed: %d %#v", code, body)
	}
	id, ok := body["connectionId"].(string)
	if !ok || len(id) < 24 || len(id) > 128 {
		t.Fatalf("missing opaque browser connection ID: %#v", body)
	}
	if entries, ok := body["entries"].([]any); !ok || len(entries) != 1 {
		t.Fatalf("connect did not return the initial directory: %#v", body)
	}
	return id
}

func (f *persistentBrowserFixture) operations(t *testing.T, id, action string, fields map[string]any) (int, map[string]any) {
	t.Helper()
	request := map[string]any{"protocol": "ssh", "connectionId": id, "action": action}
	for key, value := range fields {
		request[key] = value
	}
	return f.call(t, request)
}

func (f *persistentBrowserFixture) logCount(operation string) int {
	if f.server == nil {
		if operation == "CONNECT" {
			return int(f.connections.Load())
		}
		if operation == "LOGIN" {
			return int(f.logins.Load())
		}
		return 0
	}
	records, _ := f.server.status("sftp")["transfers"].([]transferLog)
	count := 0
	for _, record := range records {
		if record.Operation == operation {
			count++
		}
	}
	return count
}

func newShellSFTPBrowserFixture(t *testing.T) *persistentBrowserFixture {
	t.Helper()
	f := &persistentBrowserFixture{app: testApplication(t), root: t.TempDir(), connections: &atomic.Int32{}, logins: &atomic.Int32{}}
	if err := os.WriteFile(filepath.Join(f.root, "startup.cfg"), []byte("hostname test-switch\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{PasswordCallback: func(metadata ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if metadata.User() != "tester" || string(password) != "persistent-test-password" {
			return nil, io.EOF
		}
		f.logins.Add(1)
		return nil, nil
	}}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	opened := map[net.Conn]bool{}
	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		for {
			tcp, err := listener.Accept()
			if err != nil {
				return
			}
			f.connections.Add(1)
			mu.Lock()
			opened[tcp] = true
			mu.Unlock()
			workers.Add(1)
			go func() {
				defer workers.Done()
				defer func() { _ = tcp.Close(); mu.Lock(); delete(opened, tcp); mu.Unlock() }()
				server, channels, requests, err := ssh.NewServerConn(tcp, config)
				if err != nil {
					return
				}
				defer server.Close()
				go func() {
					for request := range requests {
						if request.WantReply {
							_ = request.Reply(true, nil)
						}
					}
				}()
				for request := range channels {
					if request.ChannelType() != "session" {
						_ = request.Reject(ssh.UnknownChannelType, "session only")
						continue
					}
					channel, reqs, err := request.Accept()
					if err != nil {
						continue
					}
					workers.Add(1)
					go func() {
						defer workers.Done()
						defer channel.Close()
						for req := range reqs {
							switch req.Type {
							case "pty-req", "window-change":
								if req.WantReply {
									_ = req.Reply(true, nil)
								}
							case "shell":
								if req.WantReply {
									_ = req.Reply(true, nil)
								}
								_, _ = io.Copy(channel, channel)
								return
							case "subsystem":
								var subsystem struct{ Name string }
								if ssh.Unmarshal(req.Payload, &subsystem) != nil || subsystem.Name != "sftp" {
									_ = req.Reply(false, nil)
									continue
								}
								_ = req.Reply(true, nil)
								host := &transferHost{root: f.root, access: "readwrite", overwrite: true}
								(&sftpSession{channel: channel, server: host, root: f.root, access: "readwrite", overwrite: true, handles: make(map[string]*sftpHandleState)}).serve()
								return
							default:
								if req.WantReply {
									_ = req.Reply(false, nil)
								}
							}
						}
					}()
				}
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		mu.Lock()
		for connection := range opened {
			_ = connection.Close()
		}
		mu.Unlock()
		done := make(chan struct{})
		go func() { workers.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("promotion fixture failed to close")
		}
	})
	f.credentials = map[string]any{"protocol": "ssh", "host": "127.0.0.1", "port": listener.Addr().(*net.TCPAddr).Port, "username": "tester", "password": "persistent-test-password", "authMode": "password", "fingerprint": ssh.FingerprintSHA256(signer.PublicKey())}
	return f
}

func TestPersistentSFTPTransfersOwnershipToTerminalWithoutReauthentication(t *testing.T) {
	f := newShellSFTPBrowserFixture(t)
	id := f.connect(t, nil)
	code, body := f.operations(t, id, "open-terminal", nil)
	if code != 200 {
		t.Fatalf("terminal promotion failed: %d %#v", code, body)
	}
	session, _ := body["session"].(map[string]any)
	sessionID, _ := session["id"].(string)
	if sessionID == "" || body["viewState"] == nil {
		t.Fatalf("terminal promotion omitted ownership/session state: %#v", body)
	}
	deadline := time.Now().Add(3 * time.Second)
	connected := false
	for time.Now().Before(deadline) {
		for _, current := range f.app.terminals.List() {
			if current.ID == sessionID && current.State == "connected" {
				connected = true
			}
		}
		if connected {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !connected {
		t.Fatalf("promoted terminal did not become connected: %#v", f.app.terminals.List())
	}
	if code, result := f.call(t, map[string]any{"protocol": "ssh", "sessionId": sessionID, "action": "read-text", "path": "startup.cfg"}); code != 200 || result["content"] != "hostname test-switch\r\n" {
		t.Fatalf("terminal-owned SFTP no longer works: %d %#v", code, result)
	}
	if code, result := f.operations(t, id, "list", nil); code != 409 {
		t.Fatalf("old browser owner retained access after promotion: %d %#v", code, result)
	}
	if f.logCount("CONNECT") != 1 || f.logCount("LOGIN") != 1 {
		t.Fatal("terminal promotion created a new SSH TCP connection or login")
	}
	if err := f.app.terminals.Write(sessionID, []byte("show version\r")); err != nil {
		t.Fatalf("promoted shell is unusable: %v", err)
	}
}

func TestPersistentSFTPReusesAuthenticatedTransportAcrossEveryFileEndpoint(t *testing.T) {
	f := newPersistentBrowserFixture(t)
	id := f.connect(t, nil)
	for i := 0; i < 3; i++ {
		if code, body := f.operations(t, id, "list", map[string]any{"path": "."}); code != 200 {
			t.Fatalf("list: %d %#v", code, body)
		}
		if code, body := f.operations(t, id, "preview", map[string]any{"path": "startup.cfg"}); code != 200 || body["content"] != "hostname test-switch\r\n" {
			t.Fatalf("preview: %d %#v", code, body)
		}
	}
	code, original := f.operations(t, id, "read-text", map[string]any{"path": "startup.cfg"})
	if code != 200 || original["content"] != "hostname test-switch\r\n" {
		t.Fatalf("editor read: %d %#v", code, original)
	}
	if code, body := f.operations(t, id, "write-text", map[string]any{"path": "startup.cfg", "content": "hostname edited\r\n", "expectedHash": original["hash"]}); code != 200 {
		t.Fatalf("editor save: %d %#v", code, body)
	}
	if code, body := f.operations(t, id, "copy", map[string]any{"path": "startup.cfg", "newPath": "copy.cfg"}); code != 200 {
		t.Fatalf("remote copy: %d %#v", code, body)
	}
	contents, err := os.ReadFile(filepath.Join(f.root, "copy.cfg"))
	if err != nil || string(contents) != "hostname edited\r\n" {
		t.Fatalf("remote copy changed data: %q %v", contents, err)
	}
	// Browser drop-upload forwards its connection ID through the central API.
	var multipartBody bytes.Buffer
	writer := multipart.NewWriter(&multipartBody)
	connection, _ := json.Marshal(map[string]any{"protocol": "ssh", "connectionId": id, "path": "."})
	if err := writer.WriteField("connection", string(connection)); err != nil {
		t.Fatal(err)
	}
	file, err := writer.CreateFormFile("file", "dropped.cfg")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(file, "interface 1/1/1\n")
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	upload := httptest.NewRequest(http.MethodPost, "http://"+f.app.host+"/api/remote-browser/drop-upload", &multipartBody)
	upload.Header.Set("Content-Type", writer.FormDataContentType())
	upload.Header.Set("X-Session-Token", f.app.token)
	uploadResult := httptest.NewRecorder()
	f.app.ServeHTTP(uploadResult, upload)
	if uploadResult.Code != 200 {
		t.Fatalf("drop upload: %d %s", uploadResult.Code, uploadResult.Body.String())
	}
	// Native drag performs stat and download on the same authenticated client.
	ticketBody, _ := json.Marshal(map[string]any{"protocol": "ssh", "connectionId": id, "action": "stat", "path": "copy.cfg"})
	ticketResponse := requestApp(f.app, http.MethodPost, "/api/remote-browser/drag-ticket", string(ticketBody), true)
	var ticket map[string]any
	if ticketResponse.Code != 200 || json.Unmarshal(ticketResponse.Body.Bytes(), &ticket) != nil {
		t.Fatalf("drag ticket: %d %s", ticketResponse.Code, ticketResponse.Body.String())
	}
	name, _ := ticket["ticket"].(string)
	download := requestApp(f.app, http.MethodGet, "/drag-download/"+name, "", false)
	if download.Code != 200 || download.Body.String() != "hostname edited\r\n" {
		t.Fatalf("drag download: %d %s", download.Code, download.Body.String())
	}
	if got := f.logCount("CONNECT"); got != 1 {
		t.Fatalf("file operations opened %d SSH TCP connections; expected exactly one", got)
	}
	if got := f.logCount("LOGIN"); got != 1 {
		t.Fatalf("file operations authenticated %d times; expected exactly one", got)
	}
	if code, body := f.operations(t, id, "disconnect", nil); code != 200 {
		t.Fatalf("disconnect: %d %#v", code, body)
	}
	if code, body := f.operations(t, id, "list", map[string]any{"path": "."}); code != 409 {
		t.Fatalf("disconnected ID was reusable: %d %#v", code, body)
	}
}

func TestPersistentSFTPRejectsMixedCredentialsAndUnknownIDs(t *testing.T) {
	f := newPersistentBrowserFixture(t)
	id := f.connect(t, nil)
	for _, extra := range []map[string]any{
		{"password": "persistent-test-password"}, {"privateKey": "private-key-canary"}, {"passphrase": "passphrase-canary"}, {"host": "127.0.0.1"}, {"sessionId": "forged-terminal-session"}, {"protocol": "ftp"},
	} {
		if code, body := f.operations(t, id, "list", extra); code != 400 {
			t.Fatalf("mixed credentials/identity accepted: %#v: %d %#v", extra, code, body)
		}
	}
	if code, body := f.operations(t, strings.Repeat("a", 48), "list", nil); code != 409 {
		t.Fatalf("unknown connection ID did not fail closed: %d %#v", code, body)
	}
	if code, body := f.operations(t, id, "list", map[string]any{"path": "."}); code != 200 {
		t.Fatalf("invalid requests damaged the live connection: %d %#v", code, body)
	}
	if f.logCount("CONNECT") != 1 || f.logCount("LOGIN") != 1 {
		t.Fatal("invalid requests triggered new SSH connections or authentication")
	}
}

func TestPersistentSFTPFailedShellPromotionKeepsBrowserConnected(t *testing.T) {
	f := newPersistentBrowserFixture(t)
	id := f.connect(t, nil)
	if code, body := f.operations(t, id, "open-terminal", nil); code < 400 {
		t.Fatalf("SFTP-only server unexpectedly opened a terminal: %d %#v", code, body)
	}
	if code, body := f.operations(t, id, "read-text", map[string]any{"path": "startup.cfg"}); code != 200 || body["content"] != "hostname test-switch\r\n" {
		t.Fatalf("failed terminal request broke SFTP: %d %#v", code, body)
	}
	if f.logCount("CONNECT") != 1 || f.logCount("LOGIN") != 1 {
		t.Fatal("opening a terminal reauthenticated instead of sharing SSH")
	}
}

func TestPersistentSFTPCanceledApplicationClosesTransport(t *testing.T) {
	f := newPersistentBrowserFixture(t)
	_ = f.connect(t, nil)
	f.app.stop()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if f.logCount("DISCONNECT") == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("browser SSH transport survived application shutdown")
}

func TestPersistentSFTPRejectsChangedHostKeyBeforeAuthentication(t *testing.T) {
	f := newPersistentBrowserFixture(t)
	fields := map[string]any{"action": "connect", "path": "."}
	for key, value := range f.credentials {
		fields[key] = value
	}
	fields["fingerprint"] = "SHA256:" + strings.Repeat("A", 43)
	if code, body := f.call(t, fields); code != 400 && code != 409 {
		t.Fatalf("wrong fingerprint accepted: %d %#v", code, body)
	}
	if f.logCount("LOGIN") != 0 {
		t.Fatal("an unverified server received login credentials")
	}
}

func TestPersistentSFTPConnectionLimitAndExplicitSlotRelease(t *testing.T) {
	f := newPersistentBrowserFixture(t)
	ids := make([]string, 8)
	for i := range ids {
		ids[i] = f.connect(t, nil)
	}
	fields := map[string]any{"action": "connect", "path": "."}
	for key, value := range f.credentials {
		fields[key] = value
	}
	if code, body := f.call(t, fields); code < 400 {
		t.Fatalf("unbounded standalone connections accepted: %d %#v", code, body)
	}
	if f.logCount("CONNECT") != 8 {
		t.Fatal("capacity rejection opened another network connection")
	}
	if code, body := f.operations(t, ids[0], "disconnect", nil); code != 200 {
		t.Fatalf("slot release failed: %d %#v", code, body)
	}
	newID := f.connect(t, nil)
	if newID == ids[0] {
		t.Fatal("reconnected file session reused a retired capability ID")
	}
	if f.logCount("CONNECT") != 9 || f.logCount("LOGIN") != 9 {
		t.Fatal("reconnecting did not open exactly one new authenticated transport")
	}
}

func TestPersistentSFTPPrimaryCloseRetiresHomeConnectionsWithoutStoppingApplication(t *testing.T) {
	f := newPersistentBrowserFixture(t)
	id := f.connect(t, nil)
	_ = f.app.finishPrimaryClose()
	if f.app.ctx.Err() != nil {
		t.Fatal("primary-close test canceled the app instead of closing home-owned connections")
	}
	deadline := time.Now().Add(3 * time.Second)
	for f.logCount("DISCONNECT") == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if f.logCount("DISCONNECT") != 1 {
		t.Fatal("primary close left a home SFTP transport running")
	}
	if code, body := f.operations(t, id, "list", nil); code != 403 {
		t.Fatalf("closed primary window retained browser capability: %d %#v", code, body)
	}
	fields := map[string]any{"action": "connect", "path": "."}
	for key, value := range f.credentials {
		fields[key] = value
	}
	if code, body := f.call(t, fields); code < 400 {
		t.Fatalf("closed home created another SFTP connection: %d %#v", code, body)
	}
	if f.logCount("CONNECT") != 1 {
		t.Fatal("closed home created another SSH network connection")
	}
}

func TestPersistentSFTPDisconnectRevokesPreparedNativeDragCapability(t *testing.T) {
	f := newPersistentBrowserFixture(t)
	id := f.connect(t, nil)
	encoded, _ := json.Marshal(map[string]any{"protocol": "ssh", "connectionId": id, "action": "stat", "path": "startup.cfg"})
	response := requestApp(f.app, http.MethodPost, "/api/remote-browser/drag-ticket", string(encoded), true)
	var ticket map[string]any
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &ticket) != nil {
		t.Fatalf("prepare drag: %d %s", response.Code, response.Body.String())
	}
	if code, body := f.operations(t, id, "disconnect", nil); code != 200 {
		t.Fatalf("disconnect: %d %#v", code, body)
	}
	value, _ := ticket["ticket"].(string)
	download := requestApp(f.app, http.MethodGet, "/drag-download/"+value, "", false)
	if download.Code < 400 || strings.Contains(download.Body.String(), "hostname test-switch") {
		t.Fatalf("retired connection's drag ticket still returned remote content: %d %s", download.Code, download.Body.String())
	}
	if f.logCount("CONNECT") != 1 || f.logCount("LOGIN") != 1 {
		t.Fatal("a stale drag ticket silently reauthenticated")
	}
}
