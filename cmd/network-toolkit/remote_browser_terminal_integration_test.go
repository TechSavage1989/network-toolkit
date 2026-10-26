package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"networktoolkit/internal/terminal"
)

// Simulate a device offering an interactive shell and SFTP on separate SSH
// session channels. Assert the browser reuses the one authenticated SSH TCP
// connection, and never closes the shell during file operations.
func TestSSHBrowserReusesTerminalConnection(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "startup.cfg"), []byte("hostname test-switch\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{PasswordCallback: func(c ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if c.User() != "tester" || string(password) != "test-password" {
			return nil, fmt.Errorf("invalid test authentication")
		}
		return nil, nil
	}}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var accepts atomic.Int32
	var workers sync.WaitGroup
	var connections sync.Mutex
	opened := map[net.Conn]bool{}
	workers.Add(1)
	go func() {
		defer workers.Done()
		for {
			tcp, err := listener.Accept()
			if err != nil {
				return
			}
			accepts.Add(1)
			connections.Lock()
			opened[tcp] = true
			connections.Unlock()
			workers.Add(1)
			go func() {
				defer workers.Done()
				defer func() { _ = tcp.Close(); connections.Lock(); delete(opened, tcp); connections.Unlock() }()
				peer, channels, requests, err := ssh.NewServerConn(tcp, config)
				if err != nil {
					return
				}
				defer peer.Close()
				go func() {
					for r := range requests {
						if r.WantReply {
							_ = r.Reply(true, nil)
						}
					}
				}()
				var channelsWG sync.WaitGroup
				for newCh := range channels {
					if newCh.ChannelType() != "session" {
						_ = newCh.Reject(ssh.UnknownChannelType, "sessions only")
						continue
					}
					channel, reqs, e := newCh.Accept()
					if e != nil {
						continue
					}
					channelsWG.Add(1)
					go func() {
						defer channelsWG.Done()
						defer channel.Close()
						for r := range reqs {
							switch r.Type {
							case "pty-req", "window-change":
								if r.WantReply {
									_ = r.Reply(true, nil)
								}
							case "shell":
								if r.WantReply {
									_ = r.Reply(true, nil)
								}
								go func() { _, _ = io.Copy(channel, channel) }()
							case "subsystem":
								var v struct{ Name string }
								if ssh.Unmarshal(r.Payload, &v) != nil || v.Name != "sftp" {
									if r.WantReply {
										_ = r.Reply(false, nil)
									}
									continue
								}
								if r.WantReply {
									_ = r.Reply(true, nil)
								}
								srv := &transferHost{root: root, access: "readwrite", overwrite: true}
								(&sftpSession{channel: channel, server: srv, root: root, access: "readwrite", overwrite: true, handles: make(map[string]*sftpHandleState)}).serve()
								return
							default:
								if r.WantReply {
									_ = r.Reply(false, nil)
								}
							}
						}
					}()
				}
				channelsWG.Wait()
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		connections.Lock()
		for c := range opened {
			_ = c.Close()
		}
		connections.Unlock()
		done := make(chan struct{})
		go func() { workers.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(4 * time.Second):
			t.Error("SSH test server did not shut down")
		}
	})

	manager, err := terminal.NewManager(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	info, err := manager.Connect(terminal.ConnectRequest{Type: "ssh", Host: "127.0.0.1", Port: port, Username: "tester", Password: "test-password", AuthMethod: "password", Cols: 80, Rows: 24, TimeoutMs: 5000})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(8 * time.Second)
	connected := false
	for time.Now().Before(deadline) {
		for _, session := range manager.List() {
			if session.ID != info.ID {
				continue
			}
			if session.Prompt != nil {
				if e := manager.Reply(info.ID, terminal.PromptReply{PromptID: session.Prompt.ID, Action: "save"}); e != nil {
					t.Fatal(e)
				}
			} else if session.State == "connected" {
				connected = true
			}
			if session.State == "error" {
				t.Fatalf("SSH terminal failed: %s", session.Error)
			}
		}
		if connected {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !connected {
		t.Fatalf("SSH connection did not complete: %+v", manager.List())
	}
	app := &application{terminals: manager}
	app.views = &terminalViewRegistry{homePresent: true, owners: make(map[string]terminalViewOwner), views: make(map[string]*terminalView)}
	call := func(op string, fields map[string]any) (int, map[string]any) {
		t.Helper()
		payload := map[string]any{"protocol": "ssh", "sessionId": info.ID, "action": op}
		for k, v := range fields {
			payload[k] = v
		}
		requestBytes, e := json.Marshal(payload)
		if e != nil {
			t.Fatal(e)
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/remote-browser", bytes.NewReader(requestBytes))
		app.handleRemoteBrowser(rec, req)
		var decoded map[string]any
		if e = json.Unmarshal(rec.Body.Bytes(), &decoded); e != nil {
			t.Fatalf("%s: invalid JSON: %s", op, rec.Body.String())
		}
		return rec.Code, decoded
	}
	if status, body := call("probe", nil); status != 200 || body["supported"] != true {
		t.Fatalf("terminal SFTP capability probe: %d %+v", status, body)
	}
	if accepts.Load() != 1 {
		t.Fatalf("probe opened a second SSH TCP connection (%d total)", accepts.Load())
	}
	if status, body := call("list", map[string]any{"path": "."}); status != 200 || len(body["entries"].([]any)) != 1 {
		t.Fatalf("list through terminal channel: %d %+v", status, body)
	}
	if accepts.Load() != 1 {
		t.Fatalf("file browser opened a second SSH TCP connection (%d total)", accepts.Load())
	}
	if status, body := call("preview", map[string]any{"path": "startup.cfg"}); status != 200 || body["content"] != "hostname test-switch\n" {
		t.Fatalf("preview: %d %+v", status, body)
	}
	source := filepath.Join(root, "local-upload.txt")
	if err := os.WriteFile(source, []byte("interface 1/1/1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if status, body := call("upload", map[string]any{"path": "remote-upload.txt", "localPath": source}); status != 200 {
		t.Fatalf("upload: %d %+v", status, body)
	}
	restored := filepath.Join(t.TempDir(), "download.txt")
	if status, body := call("download", map[string]any{"path": "remote-upload.txt", "localPath": restored}); status != 200 {
		t.Fatalf("download: %d %+v", status, body)
	}
	downloaded, err := os.ReadFile(restored)
	if err != nil || string(downloaded) != "interface 1/1/1\n" {
		t.Fatalf("transfer mismatch %q: %v", downloaded, err)
	}
	if status, _ := call("list", map[string]any{"path": ".", "password": "should-not-be-passed"}); status != 400 {
		t.Fatal("terminal SFTP operation accepted separate authentication secrets")
	}
	if status, _ := call("remove", map[string]any{"path": "startup.cfg"}); status != 400 {
		t.Fatal("remote delete without explicit confirmation permitted")
	}
	if status, _ := call("fingerprint", nil); status != 400 {
		t.Fatal("fingerprint inspection should not run against a trusted live session")
	}
	if accepts.Load() != 1 {
		t.Fatalf("browser opened an additional TCP connection: %d", accepts.Load())
	}
	if err := manager.Write(info.ID, []byte("show version\r")); err != nil {
		t.Fatalf("file operations broke terminal input: %v", err)
	}
	if err := manager.Disconnect(info.ID); err != nil {
		t.Fatal(err)
	}
	if status, body := call("list", nil); status != 409 {
		t.Fatalf("disconnected terminal was reused: %d %+v", status, body)
	}
	t.Logf("shared SSH session %s reused server port %s without additional login", info.ID, strconv.Itoa(port))
}
