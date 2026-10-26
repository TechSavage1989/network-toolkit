package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Verify that the browser API, not just the underlying FTP/SFTP protocols,
// supports real file operations and omits credentials from its responses.
func TestRemoteBrowserHTTPFileOperations(t *testing.T) {
	for _, protocol := range []string{"ftp", "ssh"} {
		t.Run(protocol, func(t *testing.T) {
			base := t.TempDir()
			m := newTransferHostManager(context.Background(), base)
			defer m.Close()
			l, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			port := l.Addr().(*net.TCPAddr).Port
			_ = l.Close()
			root := filepath.Join(base, "share")
			config := transferServerConfig{Root: root, Bind: "127.0.0.1", Port: port, Access: "readwrite", Username: "client", Password: "test-secret", Overwrite: true}
			if protocol == "ftp" {
				config.Protocol = "ftp"
				config.Security = "explicit"
			} else {
				config.Protocol = "sftp"
			}
			status, err := m.start(config)
			if err != nil {
				t.Fatal(err)
			}
			fingerprint, _ := status["fingerprint"].(string)
			if fingerprint == "" {
				t.Fatal("missing server fingerprint")
			}
			a := &application{}
			call := func(action string, fields map[string]any) (int, map[string]any) {
				t.Helper()
				payload := map[string]any{"protocol": protocol, "action": action, "host": "127.0.0.1", "port": port, "username": "client", "password": "test-secret", "fingerprint": fingerprint, "security": "explicit", "authMode": "password"}
				for key, value := range fields {
					payload[key] = value
				}
				encoded, err := json.Marshal(payload)
				if err != nil {
					t.Fatal(err)
				}
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/api/remote-browser", bytes.NewReader(encoded))
				a.handleRemoteBrowser(rec, req)
				var data map[string]any
				if err = json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
					t.Fatalf("%s (%d): bad JSON: %v: %s", action, rec.Code, err, rec.Body.String())
				}
				if strings.Contains(rec.Body.String(), "test-secret") {
					t.Fatal("exposed authentication secret in API output")
				}
				return rec.Code, data
			}
			if code, body := call("mkdir", map[string]any{"path": "backups"}); code != 200 {
				t.Fatalf("mkdir: %d %v", code, body)
			}
			source := filepath.Join(base, "local-config.txt")
			if err = os.WriteFile(source, []byte("switch configuration\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if code, body := call("upload", map[string]any{"path": "backups/config.txt", "localPath": source}); code != 200 {
				t.Fatalf("upload: %d %v", code, body)
			}
			code, listing := call("list", map[string]any{"path": "backups"})
			if code != 200 {
				t.Fatalf("list: %d %v", code, listing)
			}
			entries, _ := listing["entries"].([]any)
			if len(entries) != 1 {
				t.Fatalf("expected one remote file, got %v", listing)
			}
			if code, body := call("preview", map[string]any{"path": "backups/config.txt"}); code != 200 || body["content"] != "switch configuration\n" {
				t.Fatalf("preview: %d %v", code, body)
			}
			code, text := call("read-text", map[string]any{"path": "backups/config.txt"})
			if code != 200 || text["content"] != "switch configuration\n" || text["hash"] != editorDigest([]byte("switch configuration\n")) {
				t.Fatalf("editor read: %d %v", code, text)
			}
			if code, body := call("write-text", map[string]any{"path": "backups/config.txt", "content": "saved configuration\r\n", "expectedHash": text["hash"]}); code != 200 {
				t.Fatalf("editor save: %d %v", code, body)
			}
			if code, _ := call("write-text", map[string]any{"path": "backups/config.txt", "content": "stale", "expectedHash": text["hash"]}); code != 409 {
				t.Fatalf("stale remote editor save was accepted: %d", code)
			}
			// Keep the existing transfer checks using their original fixture.
			if err := os.WriteFile(filepath.Join(root, "backups/config.txt"), []byte("switch configuration\n"), 0600); err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(base, "restored.txt")
			if code, body := call("download", map[string]any{"path": "backups/config.txt", "localPath": destination}); code != 200 {
				t.Fatalf("download: %d %v", code, body)
			}
			if b, e := os.ReadFile(destination); e != nil || string(b) != "switch configuration\n" {
				t.Fatalf("unexpected download: %q, %v", b, e)
			}
			if code, _ := call("remove", map[string]any{"path": "backups/config.txt"}); code != 400 {
				t.Fatal("remote delete without confirmation was permitted")
			}
			if code, body := call("rename", map[string]any{"path": "backups/config.txt", "newPath": "backups/renamed.txt"}); code != 200 {
				t.Fatalf("rename: %d %v", code, body)
			}
			if code, body := call("remove", map[string]any{"path": "backups/renamed.txt", "confirm": true}); code != 200 {
				t.Fatalf("remove: %d %v", code, body)
			}
			if code, body := call("rmdir", map[string]any{"path": "backups", "confirm": true}); code != 200 {
				t.Fatalf("rmdir: %d %v", code, body)
			}
			if protocol == "ssh" {
				if code, _ := call("list", map[string]any{"path": "/", "fingerprint": "SHA256:WRONG"}); code != 400 {
					t.Fatal("accepted incorrect SSH server key")
				}
			}
		})
	}
}

func TestRemoteBrowserSessionRequiresAuthenticatedTerminal(t *testing.T) {
	a := &application{}
	req := httptest.NewRequest(http.MethodGet, "/api/remote-browser", nil)
	rec := httptest.NewRecorder()
	a.handleRemoteBrowser(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"sessions":[]`) {
		t.Fatalf("unexpected session list %d: %s", rec.Code, rec.Body.String())
	}
	for _, tc := range []struct {
		name   string
		fields map[string]any
		code   int
	}{
		{"unavailable", map[string]any{"protocol": "ssh", "action": "list", "sessionId": "missing", "path": "/"}, 503},
		{"credential-injection", map[string]any{"protocol": "ssh", "action": "list", "sessionId": "missing", "username": "admin"}, 400},
		{"ftp-mode", map[string]any{"protocol": "ftp", "action": "list", "sessionId": "missing"}, 400},
		{"probe-requires-session", map[string]any{"protocol": "ssh", "action": "probe", "host": "127.0.0.1"}, 400},
		{"probe-credential-injection", map[string]any{"protocol": "ssh", "action": "probe", "sessionId": "missing", "privateKey": "injected-secret"}, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, e := json.Marshal(tc.fields)
			if e != nil {
				t.Fatal(e)
			}
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/api/remote-browser", bytes.NewReader(b))
			a.handleRemoteBrowser(w, r)
			if w.Code != tc.code {
				t.Fatalf("wanted status %d got %d: %s", tc.code, w.Code, w.Body.String())
			}
		})
	}
}
