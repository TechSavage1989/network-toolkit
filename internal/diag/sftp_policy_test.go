package diag

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"networktoolkit/internal/model"
)

// A device may publish more than one SSH server host key. The old SFTP
// library-default ordering selects ECDSA, but the terminal's explicit modern
// policy selects RSA SHA-2. SFTP must match Terminal, or a previously trusted
// terminal key incorrectly appears to be a changed host key.
func TestSFTPHostKeySelectionMatchesInteractiveTerminalPolicy(t *testing.T) {
	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ecdsaSigner, err := ssh.NewSignerFromKey(ecdsaKey)
	if err != nil {
		t.Fatal(err)
	}
	rsaSigner, err := ssh.NewSignerFromKey(rsaKey)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{PasswordCallback: func(meta ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if meta.User() == "operator" && string(password) == "secret" {
			return nil, nil
		}
		return nil, io.EOF
	}}
	cfg.AddHostKey(ecdsaSigner)
	cfg.AddHostKey(rsaSigner)
	files := map[string][]byte{"/readme.txt": []byte("Host key policy integration test")}
	var mu sync.Mutex
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
				client, channels, requests, err := ssh.NewServerConn(conn, cfg)
				if err == nil {
					defer client.Close()
					go ssh.DiscardRequests(requests)
					for ch := range channels {
						if ch.ChannelType() != "session" {
							_ = ch.Reject(ssh.UnknownChannelType, "session only")
							continue
						}
						channel, requests, e := ch.Accept()
						if e != nil {
							continue
						}
						go func() {
							for request := range requests {
								if request.Type != "subsystem" || len(request.Payload) < 4 || string(request.Payload[4:]) != "sftp" {
									_ = request.Reply(false, nil)
									continue
								}
								_ = request.Reply(true, nil)
								fakeSFTP(channel, files, &mu)
								return
							}
						}()
					}
				}
			}()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req := model.Request{Tool: "sftp", Target: "127.0.0.1", Options: map[string]any{
		"port":      listener.Addr().(*net.TCPAddr).Port,
		"operation": "fingerprint", "sshCompatibility": "modern", "timeoutMs": 2500,
	}}
	result, err := sftpTest(ctx, req, nil)
	if err != nil {
		t.Fatalf("SFTP fingerprint inspection: %v", err)
	}
	got := result.Data.(map[string]any)["hostKeyFingerprint"]
	want := ssh.FingerprintSHA256(rsaSigner.PublicKey())
	if got != want {
		t.Fatalf("SFTP host key %v differs from Terminal policy's RSA SHA-2 key %s", got, want)
	}
	// Verify the fixture really does reproduce the prior algorithm difference.
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	prior := ""
	_, _, _, err = ssh.NewClientConn(conn, conn.RemoteAddr().String(), &ssh.ClientConfig{
		User: "operator", Timeout: 2 * time.Second, HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			prior = ssh.FingerprintSHA256(key)
			return errors.New("inspection done")
		},
	})
	if err == nil || prior == "" {
		t.Fatalf("default client inspection did not reach a host key: %v", err)
	}
	if prior == want {
		t.Fatal("fixture no longer distinguishes the default library ordering from the terminal algorithm policy")
	}
	listReq := model.Request{Tool: "sftp", Target: "127.0.0.1", Options: map[string]any{
		"port":      listener.Addr().(*net.TCPAddr).Port,
		"operation": "list", "remotePath": "/", "username": "operator", "password": "secret",
		"fingerprint": want, "sshCompatibility": "modern", "authMode": "password", "timeoutMs": 2500,
	}}
	listing, err := sftpTest(ctx, listReq, nil)
	if err != nil {
		t.Fatalf("standalone SFTP could not use the trusted terminal RSA host key: %v", err)
	}
	if listing.Data.(map[string]any)["entryCount"].(int) != 1 {
		t.Fatalf("unexpected SFTP listing: %#v", listing.Data)
	}
	listReq.Options["fingerprint"] = ssh.FingerprintSHA256(ecdsaSigner.PublicKey())
	if _, err = sftpTest(ctx, listReq, nil); err == nil {
		t.Fatal("SFTP accepted the wrong host key algorithm fingerprint")
	}
	listener.Close()
	<-done
}
