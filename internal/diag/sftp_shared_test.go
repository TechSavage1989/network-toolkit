package diag

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"networktoolkit/internal/model"
)

// Exercise an interactive shell and SFTP over the same SSH TCP connection.
// SFTP transfers should never close or reauthenticate the live shell.
func TestSFTPOnExistingSSHConnection(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	serverConfig := &ssh.ServerConfig{PasswordCallback: func(meta ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if meta.User() == "operator" && string(password) == "secret" {
			return nil, nil
		}
		return nil, errors.New("not authenticated")
	}}
	serverConfig.AddHostKey(key)
	files := map[string][]byte{"/readme.txt": []byte("test file content")}
	var filesMu sync.Mutex
	connections := make(chan struct{}, 1)
	var subsystems atomic.Int32
	go func() {
		conn, e := listener.Accept()
		if e != nil {
			return
		}
		defer conn.Close()
		server, channels, requests, e := ssh.NewServerConn(conn, serverConfig)
		if e != nil {
			return
		}
		defer server.Close()
		connections <- struct{}{}
		go ssh.DiscardRequests(requests)
		for inbound := range channels {
			if inbound.ChannelType() != "session" {
				_ = inbound.Reject(ssh.UnknownChannelType, "session only")
				continue
			}
			channel, reqs, e := inbound.Accept()
			if e != nil {
				continue
			}
			go func() {
				defer channel.Close()
				for r := range reqs {
					if r.Type == "shell" {
						_ = r.Reply(true, nil)
						_, _ = io.Copy(io.Discard, channel)
						return
					}
					if r.Type == "subsystem" && len(r.Payload) >= 4 && string(r.Payload[4:]) == "sftp" {
						subsystems.Add(1)
						_ = r.Reply(true, nil)
						fakeSFTP(channel, files, &filesMu)
						return
					}
					_ = r.Reply(false, nil)
				}
			}()
		}
	}()
	client, err := ssh.Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{
		User: "operator", Auth: []ssh.AuthMethod{ssh.Password("secret")},
		HostKeyCallback: ssh.FixedHostKey(key.PublicKey()), Timeout: 3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	select {
	case <-connections:
	case <-time.After(2 * time.Second):
		t.Fatal("no authenticated SSH transport")
	}
	shell, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer shell.Close()
	stdin, err := shell.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = shell.Shell(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	run := func(action, remote, local string) (model.Result, error) {
		return RunSFTPOnClient(ctx, client, model.Request{Tool: "sftp", Options: map[string]any{
			"operation": action, "remotePath": remote, "localPath": local, "timeoutMs": 3000,
		}}, nil)
	}
	result, err := run("list", "/", "")
	if err != nil {
		t.Fatalf("list failed over active shell: %v", err)
	}
	if result.Data.(map[string]any)["entryCount"] != 1 {
		t.Fatalf("list incorrect: %v", result.Data)
	}
	before := subsystems.Load()
	text, err := run("read-text", "/readme.txt", "")
	if err != nil {
		t.Fatalf("editor read failed: %v", err)
	}
	if got := string(text.Data.(map[string]any)["contentBytes"].([]byte)); got != "test file content" {
		t.Fatalf("editor content changed: %q", got)
	}
	if subsystems.Load()-before != 1 {
		t.Fatal("opening one editor file used more than one SFTP subsystem")
	}
	filesMu.Lock()
	files["/too-large.txt"] = make([]byte, (8<<20)+1)
	filesMu.Unlock()
	if _, err = run("read-text", "/too-large.txt", ""); err == nil {
		t.Fatal("editor accepted an oversized remote file")
	}
	dst := filepath.Join(t.TempDir(), "readme.txt")
	if _, err = run("download", "/readme.txt", dst); err != nil {
		t.Fatalf("download failed: %v", err)
	}
	body, err := os.ReadFile(dst)
	if err != nil || string(body) != "test file content" {
		t.Fatalf("bad download %q %v", body, err)
	}
	// The shell channel stays alive even after separate SFTP channel closures.
	if _, err = stdin.Write([]byte("still connected\n")); err != nil {
		t.Fatalf("SFTP disconnected SSH shell: %v", err)
	}
	_, err = run("list", "/", "")
	if err != nil {
		t.Fatalf("SFTP second channel failed: %v", err)
	}
}
