package terminal

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
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
)

func generateSSHKey(t *testing.T) (ssh.Signer, ed25519.PrivateKey) {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	return signer, private
}

type sshFixture struct {
	listener     net.Listener
	key          ssh.Signer
	input        chan []byte
	pty          chan [2]int
	resize       chan [2]int
	keepalive    chan struct{}
	algorithms   chan ssh.NegotiatedAlgorithms
	auth         atomic.Int32
	mu           sync.Mutex
	connections  map[net.Conn]bool
	wg           sync.WaitGroup
	ackKeepalive bool
}

func newSSHFixture(t *testing.T, configure func(*ssh.ServerConfig), ackKeepalive bool) *sshFixture {
	t.Helper()
	key, _ := generateSSHKey(t)
	return newSSHFixtureWithKey(t, key, configure, ackKeepalive)
}

func newSSHFixtureWithKey(t *testing.T, key ssh.Signer, configure func(*ssh.ServerConfig), ackKeepalive bool) *sshFixture {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &sshFixture{listener: listener, key: key, input: make(chan []byte, 32), pty: make(chan [2]int, 8), resize: make(chan [2]int, 8), keepalive: make(chan struct{}, 8), algorithms: make(chan ssh.NegotiatedAlgorithms, 32), connections: map[net.Conn]bool{}, ackKeepalive: ackKeepalive}
	config := &ssh.ServerConfig{PasswordCallback: func(c ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		f.auth.Add(1)
		if c.User() == "fixture-user" && string(password) == "fixture-secret" {
			return nil, nil
		}
		return nil, errors.New("fixture authentication refused")
	}}
	if configure != nil {
		configure(config)
	}
	config.AddHostKey(key)
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
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
		for connection := range f.connections {
			connection.Close()
		}
		f.mu.Unlock()
		done := make(chan struct{})
		go func() { f.wg.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("SSH fixture connections did not close")
		}
	})
	return f
}

func (f *sshFixture) serve(connection net.Conn, config *ssh.ServerConfig) {
	defer f.wg.Done()
	defer func() { connection.Close(); f.mu.Lock(); delete(f.connections, connection); f.mu.Unlock() }()
	server, channels, requests, err := ssh.NewServerConn(connection, config)
	if err != nil {
		return
	}
	select {
	case f.algorithms <- server.Conn.(ssh.AlgorithmsConnMetadata).Algorithms():
	default:
	}
	defer server.Close()
	requestsDone := make(chan struct{})
	go func() {
		defer close(requestsDone)
		for request := range requests {
			if request.Type == "keepalive@openssh.com" {
				select {
				case f.keepalive <- struct{}{}:
				default:
				}
				if f.ackKeepalive {
					request.Reply(false, nil)
				}
			} else {
				request.Reply(false, nil)
			}
		}
	}()
	var channelWorkers sync.WaitGroup
	for incoming := range channels {
		if incoming.ChannelType() != "session" {
			incoming.Reject(ssh.UnknownChannelType, "fixture session only")
			continue
		}
		channel, requests, err := incoming.Accept()
		if err != nil {
			continue
		}
		channelWorkers.Add(1)
		go func() {
			defer channelWorkers.Done()
			defer channel.Close()
			shell := false
			for request := range requests {
				switch request.Type {
				case "pty-req":
					var p struct {
						Term                      string
						Cols, Rows, Width, Height uint32
						Modes                     string
					}
					if ssh.Unmarshal(request.Payload, &p) != nil {
						request.Reply(false, nil)
						continue
					}
					f.pty <- [2]int{int(p.Cols), int(p.Rows)}
					request.Reply(true, nil)
				case "window-change":
					var p struct{ Cols, Rows, Width, Height uint32 }
					if ssh.Unmarshal(request.Payload, &p) == nil {
						f.resize <- [2]int{int(p.Cols), int(p.Rows)}
					}
				case "shell":
					request.Reply(true, nil)
					if !shell {
						shell = true
						channelWorkers.Add(1)
						go func() {
							defer channelWorkers.Done()
							defer channel.Close()
							io.WriteString(channel, "\x1b[32mfixture ready\x1b[0m\r\n")
							io.WriteString(channel.Stderr(), "fixture stderr\r\n")
							buffer := make([]byte, 8192)
							for {
								n, err := channel.Read(buffer)
								if n > 0 {
									chunk := append([]byte(nil), buffer[:n]...)
									select {
									case f.input <- chunk:
									default:
									}
									channel.Write(chunk)
								}
								if err != nil {
									return
								}
							}
						}()
					}
				default:
					request.Reply(false, nil)
				}
			}
		}()
	}
	server.Close()
	channelWorkers.Wait()
	<-requestsDone
}

func (f *sshFixture) request() ConnectRequest {
	host, portText, _ := net.SplitHostPort(f.listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	req := terminalRequest()
	req.Host, req.Port = host, port
	req.TimeoutMs = 1000
	req.Terminal = "xterm-256color"
	return req
}

func sshManager(t *testing.T, directory string) *Manager {
	t.Helper()
	m, err := NewManager(context.Background(), directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	return m
}

func acceptCurrentPrompt(t *testing.T, m *Manager, id, action string) Prompt {
	t.Helper()
	pending := awaitSession(t, m, id, func(s SessionInfo) bool { return s.Prompt != nil })
	prompt := *pending.Prompt
	if err := m.Reply(id, PromptReply{PromptID: prompt.ID, Action: action, Fingerprint: prompt.Fingerprint}); err != nil {
		t.Fatal(err)
	}
	return prompt
}

func TestSSHPasswordPTYResizeOutputAndDelayedTrust(t *testing.T) {
	f := newSSHFixture(t, nil, true)
	directory := t.TempDir()
	m := sshManager(t, directory)
	info, err := m.Connect(f.request())
	if err != nil {
		t.Fatal(err)
	}
	pending := awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.Prompt != nil })
	if pending.Prompt.Kind != "host-key" || pending.Prompt.Fingerprint != ssh.FingerprintSHA256(f.key.PublicKey()) || pending.Prompt.Changed {
		t.Fatalf("invalid first-use prompt %+v", pending.Prompt)
	}
	if f.auth.Load() != 0 {
		t.Fatal("password authentication preceded host-key approval")
	}
	// The network timeout is one second; human trust confirmation must not
	// expire with that timer while the independent prompt deadline is active.
	time.Sleep(1200 * time.Millisecond)
	if err := m.Reply(info.ID, PromptReply{PromptID: pending.Prompt.ID, Action: "save"}); err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	select {
	case dimensions := <-f.pty:
		if dimensions != [2]int{80, 24} {
			t.Fatal(dimensions)
		}
	case <-time.After(time.Second):
		t.Fatal("real PTY was not requested")
	}
	if err := m.Resize(info.ID, 132, 43); err != nil {
		t.Fatal(err)
	}
	select {
	case dimensions := <-f.resize:
		if dimensions != [2]int{132, 43} {
			t.Fatal(dimensions)
		}
	case <-time.After(time.Second):
		t.Fatal("SSH resize was not sent")
	}
	input := []byte{'a', 0x03, 'b', '\r'}
	if err := m.Write(info.ID, input); err != nil {
		t.Fatal(err)
	}
	select {
	case data := <-f.input:
		if !bytes.Equal(data, input) {
			t.Fatalf("terminal bytes changed: %v", data)
		}
	case <-time.After(time.Second):
		t.Fatal("terminal input was not delivered")
	}
	var output []byte
	var cursor uint64
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		batch, _ := m.Events(context.Background(), cursor, 50*time.Millisecond)
		cursor = batch.Next
		for _, event := range batch.Events {
			if event.Type == "output" {
				output = append(output, event.Data...)
			}
		}
		if bytes.Contains(output, []byte("fixture ready")) && bytes.Contains(output, []byte("fixture stderr")) {
			break
		}
	}
	if !bytes.Contains(output, []byte("\x1b[32mfixture ready")) || !bytes.Contains(output, []byte("fixture stderr")) {
		t.Fatalf("stdout/stderr/ANSI missing: %q", output)
	}
	known, err := m.KnownHosts()
	if err != nil || len(known) != 1 || known[0].Fingerprint != pending.Prompt.Fingerprint {
		t.Fatalf("saved host key: %+v %v", known, err)
	}
	loaded, err := loadKnownHostStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	list, _ := loaded.list()
	if len(list) != 1 || list[0].Host != f.listener.Addr().String() {
		t.Fatalf("portable trust reload failed: %+v", list)
	}
	batch, _ := m.Events(context.Background(), 0, 0)
	raw, _ := json.Marshal(batch)
	if bytes.Contains(raw, []byte("fixture-secret")) {
		t.Fatal("password appeared in protocol events")
	}
}

func TestSSHChangedHostKeyRequiresExactVerifiedReplacement(t *testing.T) {
	f := newSSHFixture(t, nil, true)
	m := sshManager(t, t.TempDir())
	old, _ := generateSSHKey(t)
	if err := m.known.verify(context.Background(), f.listener.Addr().String(), old.PublicKey(), func(context.Context, Prompt) (PromptReply, error) { return PromptReply{Action: "save"}, nil }); err != nil {
		t.Fatal(err)
	}
	info, err := m.Connect(f.request())
	if err != nil {
		t.Fatal(err)
	}
	pending := awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.Prompt != nil })
	if !pending.Prompt.Changed || len(pending.Prompt.PreviousFingerprints) != 1 || pending.Prompt.PreviousFingerprints[0] != ssh.FingerprintSHA256(old.PublicKey()) {
		t.Fatalf("changed-key evidence missing: %+v", pending.Prompt)
	}
	for _, reply := range []PromptReply{{PromptID: pending.Prompt.ID, Action: "once"}, {PromptID: pending.Prompt.ID, Action: "replace", Fingerprint: "wrong"}} {
		if err := m.Reply(info.ID, reply); err == nil {
			t.Fatal("changed key bypassed explicit verification")
		}
	}
	if f.auth.Load() != 0 {
		t.Fatal("credentials were attempted before changed-key approval")
	}
	if err := m.Reply(info.ID, PromptReply{PromptID: pending.Prompt.ID, Action: "replace", Fingerprint: pending.Prompt.Fingerprint}); err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	known, _ := m.KnownHosts()
	if len(known) != 1 || known[0].Fingerprint != ssh.FingerprintSHA256(f.key.PublicKey()) {
		t.Fatalf("verified replacement was not saved: %+v", known)
	}
}

func TestSSHKeyboardInteractiveChallenges(t *testing.T) {
	f := newSSHFixture(t, func(config *ssh.ServerConfig) {
		config.PasswordCallback = nil
		config.KeyboardInteractiveCallback = func(_ ssh.ConnMetadata, challenge ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			answers, err := challenge("fixture", "Two fixture questions", []string{"One-time code", "Echoed choice"}, []bool{false, true})
			if err != nil {
				return nil, err
			}
			if len(answers) != 2 || answers[0] != "654321" || answers[1] != "yes" {
				return nil, errors.New("fixture challenge refused")
			}
			return nil, nil
		}
	}, true)
	m := sshManager(t, t.TempDir())
	req := f.request()
	req.AuthMethod = "keyboard"
	req.Password = ""
	info, err := m.Connect(req)
	if err != nil {
		t.Fatal(err)
	}
	acceptCurrentPrompt(t, m, info.ID, "once")
	pending := awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.Prompt != nil && s.Prompt.Kind == "keyboard-interactive" })
	if len(pending.Prompt.Fields) != 2 || pending.Prompt.Fields[0].Echo || !pending.Prompt.Fields[1].Echo {
		t.Fatalf("keyboard-interactive echo policy changed: %+v", pending.Prompt)
	}
	if err := m.Reply(info.ID, PromptReply{PromptID: pending.Prompt.ID, Action: "submit", Answers: []string{"654321", "yes"}}); err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	batch, _ := m.Events(context.Background(), 0, 0)
	raw, _ := json.Marshal(batch)
	if bytes.Contains(raw, []byte("654321")) {
		t.Fatal("challenge answer appeared in output protocol")
	}
	known, _ := m.KnownHosts()
	if len(known) != 0 {
		t.Fatal("trust-once unexpectedly persisted a host key")
	}
}

func TestSSHEncryptedOpenSSHKeyAndPEM(t *testing.T) {
	signer, private := generateSSHKey(t)
	block, err := ssh.MarshalPrivateKeyWithPassphrase(private, "fixture", []byte("fixture-key-passphrase"))
	if err != nil {
		t.Fatal(err)
	}
	encoded := pem.EncodeToMemory(block)
	f := newSSHFixture(t, func(config *ssh.ServerConfig) {
		config.PasswordCallback = nil
		config.PublicKeyCallback = func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if bytes.Equal(key.Marshal(), signer.PublicKey().Marshal()) {
				return nil, nil
			}
			return nil, errors.New("fixture public key refused")
		}
	}, true)
	m := sshManager(t, t.TempDir())
	req := f.request()
	req.AuthMethod = "key"
	req.Password = ""
	req.PrivateKey = string(encoded)
	info, err := m.Connect(req)
	if err != nil {
		t.Fatal(err)
	}
	pending := awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.Prompt != nil })
	if pending.Prompt.Kind != "key-passphrase" {
		t.Fatalf("encrypted key did not request a passphrase: %+v", pending.Prompt)
	}
	if err := m.Reply(info.ID, PromptReply{PromptID: pending.Prompt.ID, Action: "submit", Answers: []string{"fixture-key-passphrase"}}); err != nil {
		t.Fatal(err)
	}
	pending = awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.Prompt != nil && s.Prompt.Kind == "host-key" })
	if err := m.Reply(info.ID, PromptReply{PromptID: pending.Prompt.ID, Action: "once"}); err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	batch, _ := m.Events(context.Background(), 0, 0)
	raw, _ := json.Marshal(batch)
	if bytes.Contains(raw, []byte("fixture-key-passphrase")) || bytes.Contains(raw, []byte("BEGIN OPENSSH PRIVATE KEY")) {
		t.Fatal("key material appeared in protocol")
	}
	// Existing encrypted traditional PEM keys are accepted through the same
	// bundled parser; RSA authentication is restricted to SHA256/SHA512 signing.
	rsaPrivate, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemBlock, err := x509.EncryptPEMBlock(rand.Reader, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(rsaPrivate), []byte("legacy-fixture-pass"), x509.PEMCipherAES256)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseSSHSigner(context.Background(), ConnectRequest{PrivateKey: string(pem.EncodeToMemory(pemBlock)), Passphrase: "legacy-fixture-pass"}, nil)
	if err != nil || parsed.PublicKey().Type() != ssh.KeyAlgoRSA {
		t.Fatalf("encrypted PEM key failed: %v", err)
	}
}

func TestSSHDisconnectPendingTrustAndKeepaliveTimeout(t *testing.T) {
	t.Run("pending trust", func(t *testing.T) {
		f := newSSHFixture(t, nil, true)
		m := sshManager(t, t.TempDir())
		info, err := m.Connect(f.request())
		if err != nil {
			t.Fatal(err)
		}
		awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.Prompt != nil })
		if err := m.Disconnect(info.ID); err != nil {
			t.Fatal(err)
		}
		done := make(chan struct{})
		go func() { m.Close(); close(done) }()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("SSH host-key prompt prevented shutdown")
		}
		if f.auth.Load() != 0 {
			t.Fatal("authentication continued after cancellation")
		}
	})
	t.Run("keepalive timeout", func(t *testing.T) {
		f := newSSHFixture(t, nil, false)
		known, err := loadKnownHostStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		m := newManager(context.Background(), known, map[string]transportOpener{"ssh": func(ctx context.Context, req ConnectRequest, cb transportCallbacks) (transport, error) {
			return openSSHWithTiming(ctx, req, cb, known, sshTiming{50 * time.Millisecond, 100 * time.Millisecond})
		}})
		t.Cleanup(func() { m.Close() })
		info, err := m.Connect(f.request())
		if err != nil {
			t.Fatal(err)
		}
		acceptCurrentPrompt(t, m, info.ID, "once")
		closed := awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "error" })
		if !strings.Contains(closed.Error, "keepalive timed out") {
			t.Fatalf("missing keepalive failure: %+v", closed)
		}
	})
}

func TestSSHHandshakeCancellationClosesSocket(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			accepted <- connection
		}
	}()
	m := sshManager(t, t.TempDir())
	req := terminalRequest()
	host, portText, _ := net.SplitHostPort(listener.Addr().String())
	req.Host = host
	req.Port, _ = strconv.Atoi(portText)
	info, err := m.Connect(req)
	if err != nil {
		t.Fatal(err)
	}
	var connection net.Conn
	select {
	case connection = <-accepted:
	case <-time.After(time.Second):
		t.Fatal("SSH handshake did not connect")
	}
	defer connection.Close()
	if err := m.Disconnect(info.ID); err != nil {
		t.Fatal(err)
	}
	connection.SetReadDeadline(time.Now().Add(time.Second))
	buffer := make([]byte, 4096)
	for {
		_, err = connection.Read(buffer)
		if err != nil {
			break
		}
	}
	if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatal("cancelled SSH handshake left TCP open")
	}
	if err := m.Close(); err != nil {
		t.Fatal(fmt.Errorf("close manager: %w", err))
	}
}
