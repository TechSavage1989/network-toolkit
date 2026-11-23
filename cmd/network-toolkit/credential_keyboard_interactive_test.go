package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"networktoolkit/internal/terminal"
)

// Exercise the application's saved-credential resolver and public Terminal API,
// not a request with an already-resolved password. The peer is loopback-only
// and offers keyboard-interactive authentication without the password method.
func TestCredentialKeyboardInteractiveSavedPasswordAndServerPin(t *testing.T) {
	for _, test := range []struct {
		name     string
		wrongPin bool
	}{
		{name: "approved-saved-password"},
		{name: "different-server-fingerprint", wrongPin: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			const secret = "SYNTHETIC_SAVED_KEYBOARD_PASSWORD_77"
			a := credentialTestApplication(t)
			_, private, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			signer, err := ssh.NewSignerFromKey(private)
			if err != nil {
				t.Fatal(err)
			}
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			var accepts, challenges, correctAnswers atomic.Int32
			config := &ssh.ServerConfig{KeyboardInteractiveCallback: func(peer ssh.ConnMetadata, challenge ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
				challenges.Add(1)
				if peer.User() != "operator" {
					return nil, errors.New("unexpected synthetic SSH user")
				}
				answers, err := challenge("", "", []string{"Password:"}, []bool{false})
				if err != nil {
					return nil, err
				}
				if len(answers) != 1 || answers[0] != secret {
					return nil, errors.New("incorrect synthetic SSH answer")
				}
				answers[0] = ""
				correctAnswers.Add(1)
				return nil, nil
			}}
			config.AddHostKey(signer)
			serverDone := make(chan struct{})
			go func() {
				defer close(serverDone)
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				accepts.Add(1)
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
				peer, channels, requests, err := ssh.NewServerConn(conn, config)
				if err != nil {
					return
				}
				defer peer.Close()
				go ssh.DiscardRequests(requests)
				for incoming := range channels {
					if incoming.ChannelType() != "session" {
						_ = incoming.Reject(ssh.UnknownChannelType, "synthetic session only")
						continue
					}
					channel, requests, err := incoming.Accept()
					if err != nil {
						continue
					}
					for request := range requests {
						accepted := request.Type == "pty-req" || request.Type == "shell" || request.Type == "window-change"
						_ = request.Reply(accepted, nil)
					}
					_ = channel.Close()
				}
			}()
			defer func() {
				_ = a.terminals.Close()
				_ = listener.Close()
				select {
				case <-serverDone:
				case <-time.After(5 * time.Second):
					t.Error("synthetic SSH peer did not stop after Terminal shutdown")
				}
			}()

			pin := ssh.FingerprintSHA256(signer.PublicKey())
			if test.wrongPin {
				pin = "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
			}
			port := listener.Addr().(*net.TCPAddr).Port
			item, err := a.credentials.put(credentialRecord{Name: "Synthetic keyboard password", Username: "operator", Kind: "password", Secret: secret,
				Bindings: []credentialBinding{{Protocol: "ssh", Host: "127.0.0.1", Port: port, Username: "operator", Fingerprint: pin}}})
			if err != nil {
				t.Fatal(err)
			}
			vaultBefore, err := os.ReadFile(a.credentials.path)
			if err != nil || bytes.Contains(vaultBefore, []byte(secret)) {
				t.Fatal("synthetic saved password was not protected before connection")
			}
			profile := map[string]any{"id": "synthetic-keyboard-profile", "name": "Synthetic SSH", "type": "ssh", "host": "127.0.0.1", "port": port, "username": "operator", "authMethod": "password", "credentialId": item.ID}
			if response := viewTestRequest(a, a.token, http.MethodPut, "/api/state", map[string]any{"terminalProfiles": []any{profile}}, nil); response.Code != http.StatusOK {
				t.Fatalf("saved profile setup status %d", response.Code)
			}
			request := terminal.ConnectRequest{Type: "ssh", Host: "127.0.0.1", Port: port, Username: "operator", AuthMethod: "password", CredentialID: item.ID, ProfileID: "synthetic-keyboard-profile", TimeoutMs: 3000}
			response := viewTestRequest(a, a.token, http.MethodPost, "/api/terminals", request, nil)
			if response.Code != http.StatusCreated || bytes.Contains(response.Body.Bytes(), []byte(secret)) {
				t.Fatalf("saved-credential connection status %d, or secret in response", response.Code)
			}
			var created struct {
				Session terminal.SessionInfo `json:"session"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil || created.Session.ID == "" {
				t.Fatal("saved-credential connection did not return a session")
			}
			var current terminal.SessionInfo
			viewTestWait(t, func() bool {
				current, _ = a.terminalSession(created.Session.ID)
				if current.Prompt != nil {
					t.Fatalf("saved-password or pinned-host path unexpectedly asked for %q", current.Prompt.Kind)
				}
				return current.State == "connected" || current.State == "error"
			})
			if accepts.Load() != 1 {
				t.Fatal("saved-credential test did not reach its one loopback SSH peer")
			}
			if test.wrongPin {
				if current.State != "error" || !strings.Contains(current.Error, "SSH host key mismatch") || challenges.Load() != 0 || correctAnswers.Load() != 0 {
					t.Fatal("saved password reached authentication before its approved server fingerprint matched")
				}
			} else if current.State != "connected" || challenges.Load() != 1 || correctAnswers.Load() != 1 {
				t.Fatal("saved password did not authenticate the keyboard-only peer exactly once")
			}
			if current.ProfileID != "synthetic-keyboard-profile" || current.Host != "127.0.0.1" || current.Port != port || current.Username != "operator" {
				t.Fatal("saved credential changed the profile or connection destination")
			}

			for _, path := range []string{"/api/terminals", "/api/terminals/events?wait=0", "/api/credentials", "/api/credentials/" + item.ID, "/api/state"} {
				result := requestApp(a, http.MethodGet, path, "", true)
				if result.Code != http.StatusOK || bytes.Contains(result.Body.Bytes(), []byte(secret)) {
					t.Fatalf("saved-password metadata/event boundary failed for %s (status %d)", path, result.Code)
				}
				if path == "/api/terminals/events?wait=0" {
					var batch terminal.Batch
					if err := json.Unmarshal(result.Body.Bytes(), &batch); err != nil {
						t.Fatal(err)
					}
					for _, event := range batch.Events {
						if event.Prompt != nil || event.Session != nil && event.Session.Prompt != nil || bytes.Contains(event.Data, []byte(secret)) {
							t.Fatal("automatic saved-password use published a prompt or secret output")
						}
					}
				}
			}
			vaultAfter, err := os.ReadFile(a.credentials.path)
			if err != nil || !bytes.Equal(vaultBefore, vaultAfter) {
				t.Fatal("SSH authentication changed the encrypted saved credential")
			}
			stored, found := a.credentials.get(item.ID)
			if !found || stored.Secret != secret {
				t.Fatal("saved password was lost after authentication")
			}
			settings, err := os.ReadFile(a.dataPath)
			if err != nil || bytes.Contains(settings, []byte(secret)) {
				t.Fatal("SSH authentication put its saved password in profile settings")
			}
		})
	}
}
