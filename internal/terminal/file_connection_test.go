package terminal

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"networktoolkit/internal/model"
)

type fileSSHFixture struct {
	listener       net.Listener
	key            ssh.Signer
	tcp, auth      atomic.Int32
	shellAllowed   bool
	mu             sync.Mutex
	connections    map[net.Conn]bool
	listingGate    <-chan struct{}
	listingStarted chan struct{}
	wg             sync.WaitGroup
}

func newFileSSHFixture(t *testing.T, shell bool) *fileSSHFixture {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	key, _ := generateSSHKey(t)
	f := &fileSSHFixture{listener: listener, key: key, shellAllowed: shell, connections: map[net.Conn]bool{}, listingStarted: make(chan struct{}, 8)}
	config := &ssh.ServerConfig{PasswordCallback: func(metadata ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		f.auth.Add(1)
		if metadata.User() == "operator" && string(password) == "fixture-secret" {
			return nil, nil
		}
		return nil, errors.New("authentication rejected")
	}}
	config.AddHostKey(key)
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			f.tcp.Add(1)
			f.mu.Lock()
			f.connections[connection] = true
			f.mu.Unlock()
			f.wg.Add(1)
			go f.serve(connection, config)
		}
	}()
	t.Cleanup(func() {
		listener.Close()
		f.mu.Lock()
		for conn := range f.connections {
			conn.Close()
		}
		f.mu.Unlock()
		done := make(chan struct{})
		go func() { f.wg.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("file SSH fixture did not close")
		}
	})
	return f
}

func (f *fileSSHFixture) serve(connection net.Conn, config *ssh.ServerConfig) {
	defer f.wg.Done()
	defer func() { connection.Close(); f.mu.Lock(); delete(f.connections, connection); f.mu.Unlock() }()
	server, channels, requests, err := ssh.NewServerConn(connection, config)
	if err != nil {
		return
	}
	defer server.Close()
	go func() {
		for request := range requests {
			request.Reply(false, nil)
		}
	}()
	var workers sync.WaitGroup
	for incoming := range channels {
		if incoming.ChannelType() != "session" {
			incoming.Reject(ssh.UnknownChannelType, "session only")
			continue
		}
		channel, requests, err := incoming.Accept()
		if err != nil {
			continue
		}
		workers.Add(1)
		go func() {
			defer workers.Done()
			defer channel.Close()
			for request := range requests {
				switch request.Type {
				case "subsystem":
					var value struct{ Name string }
					if ssh.Unmarshal(request.Payload, &value) != nil || value.Name != "sftp" {
						request.Reply(false, nil)
						continue
					}
					request.Reply(true, nil)
					f.sftp(channel)
					return
				case "pty-req":
					request.Reply(f.shellAllowed, nil)
				case "shell":
					request.Reply(f.shellAllowed, nil)
					if !f.shellAllowed {
						continue
					}
					io.WriteString(channel, "fixture shell ready\r\n")
					io.Copy(io.Discard, channel)
					return
				default:
					request.Reply(false, nil)
				}
			}
		}()
	}
	server.Close()
	workers.Wait()
}

func fileU32(value uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, value)
	return b
}
func fileU64(value uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, value)
	return b
}
func fileString(value string) []byte { return append(fileU32(uint32(len(value))), []byte(value)...) }
func fileTakeString(data *[]byte) string {
	if len(*data) < 4 {
		return ""
	}
	n := int(binary.BigEndian.Uint32((*data)[:4]))
	*data = (*data)[4:]
	if n > len(*data) {
		return ""
	}
	value := string((*data)[:n])
	*data = (*data)[n:]
	return value
}

func (f *fileSSHFixture) sftp(channel ssh.Channel) {
	content := []byte("hostname fixture\ninterface 1/1/1\n")
	read := func() (byte, []byte, error) {
		var header [4]byte
		if _, err := io.ReadFull(channel, header[:]); err != nil {
			return 0, nil, err
		}
		n := binary.BigEndian.Uint32(header[:])
		if n < 1 || n > 1<<20 {
			return 0, nil, errors.New("invalid SFTP frame")
		}
		b := make([]byte, n)
		if _, err := io.ReadFull(channel, b); err != nil {
			return 0, nil, err
		}
		return b[0], b[1:], nil
	}
	send := func(kind byte, data ...[]byte) error {
		body := []byte{kind}
		for _, value := range data {
			body = append(body, value...)
		}
		_, err := channel.Write(append(fileU32(uint32(len(body))), body...))
		return err
	}
	typ, _, err := read()
	if err != nil || typ != 1 {
		return
	}
	if send(2, fileU32(3)) != nil {
		return
	}
	listed := false
	for {
		op, payload, err := read()
		if err != nil || len(payload) < 4 {
			return
		}
		id := append([]byte(nil), payload[:4]...)
		payload = payload[4:]
		status := func(code uint32) { send(101, id, fileU32(code), fileString("fixture status"), fileString("")) }
		attrs := func() []byte {
			return append(append(fileU32(5), fileU64(uint64(len(content)))...), fileU32(0100644)...)
		}
		switch op {
		case 11:
			listed = false
			send(102, id, fileString("directory"))
		case 12:
			f.mu.Lock()
			gate := f.listingGate
			f.mu.Unlock()
			if gate != nil {
				select {
				case f.listingStarted <- struct{}{}:
				default:
				}
				<-gate
			}
			if listed {
				status(1)
				continue
			}
			listed = true
			send(104, id, fileU32(1), fileString("configuration.txt"), fileString("configuration.txt"), attrs())
		case 7:
			send(105, id, attrs())
		case 3:
			send(102, id, fileString("file"))
		case 5:
			fileTakeString(&payload)
			if len(payload) < 12 {
				status(4)
				continue
			}
			offset := binary.BigEndian.Uint64(payload[:8])
			size := uint64(binary.BigEndian.Uint32(payload[8:12]))
			if offset >= uint64(len(content)) {
				status(1)
				continue
			}
			end := offset + size
			if end > uint64(len(content)) {
				end = uint64(len(content))
			}
			send(103, id, fileString(string(content[offset:end])))
		case 4:
			status(0)
		default:
			status(8)
		}
	}
}

func (f *fileSSHFixture) request() ConnectRequest {
	host, port, _ := net.SplitHostPort(f.listener.Addr().String())
	number, _ := strconv.Atoi(port)
	return ConnectRequest{Type: "ssh", Host: host, Port: number, Username: "operator", Password: "fixture-secret", AuthMethod: "password", TimeoutMs: 2000, Cols: 80, Rows: 24, Terminal: "xterm-256color"}
}
func (f *fileSSHFixture) open(t *testing.T) *FileConnection {
	t.Helper()
	file, err := OpenFileConnection(context.Background(), f.request(), ssh.FingerprintSHA256(f.key.PublicKey()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() })
	return file
}
func fileOperation(action string) model.Request {
	remote := "/configuration.txt"
	if action == "list" {
		remote = "/"
	}
	return model.Request{Tool: "sftp", Options: map[string]any{"operation": action, "remotePath": remote, "timeoutMs": 2000}}
}
func waitFileState(t *testing.T, m *Manager, id, state string) SessionInfo {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		info, err := m.Get(id)
		if err == nil && info.State == state {
			return info
		}
		if err == nil && info.State == "error" && state != "error" {
			t.Fatalf("unexpected session error: %+v", info)
		}
		time.Sleep(time.Millisecond)
	}
	info, err := m.Get(id)
	t.Fatalf("session did not reach %s: %+v, %v", state, info, err)
	return SessionInfo{}
}

func TestFileConnectionReusesAuthenticationForBrowsingAndPromotion(t *testing.T) {
	f := newFileSSHFixture(t, true)
	file := f.open(t)
	for i := 0; i < 4; i++ {
		if _, err := file.Browse(context.Background(), fileOperation("list")); err != nil {
			t.Fatal(err)
		}
		result, err := file.Browse(context.Background(), fileOperation("read-text"))
		if err != nil {
			t.Fatal(err)
		}
		if got := string(result.Data.(map[string]any)["contentBytes"].([]byte)); got != "hostname fixture\ninterface 1/1/1\n" {
			t.Fatalf("incorrect file: %q", got)
		}
	}
	if f.tcp.Load() != 1 || f.auth.Load() != 1 {
		t.Fatalf("repeated login during browsing: tcp=%d auth=%d", f.tcp.Load(), f.auth.Load())
	}
	m, err := NewManager(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	info, err := m.AdoptFileConnection(file, f.request())
	if err != nil {
		t.Fatal(err)
	}
	waitFileState(t, m, info.ID, "connected")
	client, _, err := m.SFTPClient(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if client != file.transport.client {
		t.Fatal("promoted Terminal does not use the file connection's SSH transport")
	}
	if _, err = m.BrowseSFTP(context.Background(), info.ID, fileOperation("list")); err != nil {
		t.Fatal(err)
	}
	if f.tcp.Load() != 1 || f.auth.Load() != 1 {
		t.Fatalf("promotion reauthenticated: tcp=%d auth=%d", f.tcp.Load(), f.auth.Load())
	}
	if err = m.Disconnect(info.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-file.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("terminal disconnect left adopted file transport running")
	}
}

func TestFileConnectionRejectedShellKeepsSFTPAlive(t *testing.T) {
	f := newFileSSHFixture(t, false)
	file := f.open(t)
	m, err := NewManager(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	for attempt := 0; attempt < 2; attempt++ {
		info, err := m.AdoptFileConnection(file, f.request())
		if err != nil {
			t.Fatal(err)
		}
		failed := waitFileState(t, m, info.ID, "error")
		if !strings.Contains(failed.Error, "request SSH terminal") {
			t.Fatalf("unexpected failed promotion: %+v", failed)
		}
		if err = m.Remove(info.ID); err != nil {
			t.Fatal(err)
		}
		if _, err = file.Browse(context.Background(), fileOperation("read-text")); err != nil {
			t.Fatalf("failed promotion closed SFTP: %v", err)
		}
	}
	if f.tcp.Load() != 1 || f.auth.Load() != 1 {
		t.Fatal("rejected promotion caused another connection")
	}
}

func TestFileConnectionCancelledBrowseDoesNotDisconnect(t *testing.T) {
	f := newFileSSHFixture(t, false)
	file := f.open(t)
	gate := make(chan struct{})
	f.mu.Lock()
	f.listingGate = gate
	f.mu.Unlock()
	defer func() {
		select {
		case <-gate:
		default:
			close(gate)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := file.Browse(ctx, fileOperation("list")); done <- err }()
	select {
	case <-f.listingStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("browse did not reach the SFTP server")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled browse succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("browse cancellation did not unblock")
	}
	f.mu.Lock()
	f.listingGate = nil
	f.mu.Unlock()
	close(gate)
	if _, err := file.Browse(context.Background(), fileOperation("read-text")); err != nil {
		t.Fatal(err)
	}
	if f.tcp.Load() != 1 || f.auth.Load() != 1 {
		t.Fatal("cancelled browse reauthenticated")
	}
}

func TestFileConnectionRejectsChangedPinBeforeAuthentication(t *testing.T) {
	f := newFileSSHFixture(t, true)
	other, _ := generateSSHKey(t)
	file, err := OpenFileConnection(context.Background(), f.request(), ssh.FingerprintSHA256(other.PublicKey()))
	if err == nil {
		file.Close()
		t.Fatal("wrong server key accepted")
	}
	if !strings.Contains(err.Error(), "host key mismatch") {
		t.Fatalf("unexpected host verification error: %v", err)
	}
	if f.auth.Load() != 0 {
		t.Fatalf("credentials were attempted before checking the server key: %d", f.auth.Load())
	}
}

func TestFileConnectionUncommittedShellCancellationPreservesSFTP(t *testing.T) {
	f := newFileSSHFixture(t, true)
	file := f.open(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request, err := normalizeRequest(f.request())
	if err != nil {
		t.Fatal(err)
	}
	shell, err := file.transport.openShell(ctx, request, transportCallbacks{Output: func([]byte) {}})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-shell.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled provisional shell did not exit")
	}
	if _, err = file.Browse(context.Background(), fileOperation("read-text")); err != nil {
		t.Fatalf("provisional shell cancellation disconnected files: %v", err)
	}
	if f.tcp.Load() != 1 || f.auth.Load() != 1 {
		t.Fatal(fmt.Sprintf("unexpected reconnect: tcp=%d auth=%d", f.tcp.Load(), f.auth.Load()))
	}
}

func TestFileConnectionUsesSharedInteractiveAuthentication(t *testing.T) {
	t.Run("keyboard challenge", func(t *testing.T) {
		f := newSSHFixture(t, func(config *ssh.ServerConfig) {
			config.PasswordCallback = nil
			config.KeyboardInteractiveCallback = func(meta ssh.ConnMetadata, challenge ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
				answers, err := challenge("fixture-user", "Authentication code", []string{"One-time code:"}, []bool{false})
				if err != nil {
					return nil, err
				}
				if len(answers) != 1 || answers[0] != "123456" {
					return nil, errors.New("wrong code")
				}
				return nil, nil
			}
		}, true)
		request := f.request()
		request.AuthMethod = "keyboard"
		request.Password = ""
		calls := 0
		file, err := OpenFileConnectionWithPrompts(context.Background(), request, ssh.FingerprintSHA256(f.key.PublicKey()), func(ctx context.Context, prompt Prompt) (PromptReply, error) {
			calls++
			if prompt.Kind != "keyboard-interactive" || len(prompt.Fields) != 1 {
				t.Errorf("wrong prompt: %+v", prompt)
			}
			return PromptReply{Action: "submit", Answers: []string{"123456"}}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if calls != 1 {
			t.Fatalf("challenge count: %d", calls)
		}
	})
	t.Run("encrypted key passphrase", func(t *testing.T) {
		signer, private := generateSSHKey(t)
		block, err := ssh.MarshalPrivateKeyWithPassphrase(private, "fixture", []byte("fixture-passphrase"))
		if err != nil {
			t.Fatal(err)
		}
		f := newSSHFixture(t, func(config *ssh.ServerConfig) {
			config.PasswordCallback = nil
			config.PublicKeyCallback = func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
				if bytes.Equal(key.Marshal(), signer.PublicKey().Marshal()) {
					return nil, nil
				}
				return nil, errors.New("wrong key")
			}
		}, true)
		request := f.request()
		request.AuthMethod = "key"
		request.Password = ""
		request.PrivateKey = string(pem.EncodeToMemory(block))
		request.Passphrase = ""
		calls := 0
		file, err := OpenFileConnectionWithPrompts(context.Background(), request, ssh.FingerprintSHA256(f.key.PublicKey()), func(ctx context.Context, prompt Prompt) (PromptReply, error) {
			calls++
			if prompt.Kind != "key-passphrase" {
				t.Errorf("wrong prompt: %+v", prompt)
			}
			return PromptReply{Action: "submit", Answers: []string{"fixture-passphrase"}}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if calls != 1 {
			t.Fatalf("passphrase prompt count: %d", calls)
		}
	})
}
