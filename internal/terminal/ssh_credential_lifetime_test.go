package terminal

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestExpectedHostFingerprintCannotBeSuppliedOrExposedThroughJSON(t *testing.T) {
	for _, name := range []string{"expectedHostFingerprint", "ExpectedHostFingerprint", "expectedhostfingerprint"} {
		var req ConnectRequest
		if err := json.Unmarshal([]byte(`{"`+name+`":"SHA256:client-controlled"}`), &req); err != nil {
			t.Fatal(err)
		}
		if req.ExpectedHostFingerprint != "" {
			t.Fatal("JSON supplied a backend-only host fingerprint")
		}
	}
	req := ConnectRequest{ExpectedHostFingerprint: "SHA256:backend-only"}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "backend-only") || strings.Contains(strings.ToLower(string(raw)), "expectedhostfingerprint") {
		t.Fatal("backend-only fingerprint escaped through JSON")
	}
}

func TestSavedSSHHostFingerprintMismatchRejectsBeforeAuthentication(t *testing.T) {
	f := newSSHFixture(t, nil, true)
	other, _ := generateSSHKey(t)
	m := sshManager(t, t.TempDir())
	req := f.request()
	req.CredentialID = "credential-pin-fixture"
	req.ExpectedHostFingerprint = ssh.FingerprintSHA256(other.PublicKey())
	info, err := m.Connect(req)
	if err != nil {
		t.Fatal(err)
	}
	failed := awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "error" || s.Prompt != nil })
	if failed.Prompt != nil || !strings.Contains(failed.Error, "host key mismatch") {
		t.Fatalf("saved pin mismatch was not refused without browser trust approval: %+v", failed)
	}
	if f.auth.Load() != 0 {
		t.Fatal("saved password reached the server before its approved host key matched")
	}
	m.mu.Lock()
	retained := m.sessions[info.ID].request
	m.mu.Unlock()
	assertNoRetainedAuthentication(t, retained)
	if retained.ExpectedHostFingerprint != req.ExpectedHostFingerprint {
		t.Fatal("safe reconnect settings lost the authenticated host pin")
	}
}

func TestSavedSSHReconnectUsesFreshBackendHostFingerprint(t *testing.T) {
	f := newSSHFixture(t, nil, true)
	other, _ := generateSSHKey(t)
	m := sshManager(t, t.TempDir())
	req := f.request()
	req.CredentialID = "credential-renewed-pin-fixture"
	req.ExpectedHostFingerprint = ssh.FingerprintSHA256(f.key.PublicKey())
	info, err := m.Connect(req)
	if err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	if f.auth.Load() != 1 {
		t.Fatal("fixture did not authenticate exactly once with its approved pin")
	}
	renewedPin := ssh.FingerprintSHA256(other.PublicKey())
	m.SetCredentialResolver(func(_ context.Context, safe ConnectRequest) (ConnectRequest, error) {
		assertNoRetainedAuthentication(t, safe)
		safe.Password = "fixture-secret"
		safe.ExpectedHostFingerprint = renewedPin
		return safe, nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := m.Reconnect(ctx, info.ID); err != nil {
		t.Fatal(err)
	}
	failed := awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "error" || s.Prompt != nil })
	if failed.Prompt != nil || !strings.Contains(failed.Error, "host key mismatch") {
		t.Fatalf("reconnect ignored the renewed vault host pin: %+v", failed)
	}
	if f.auth.Load() != 1 {
		t.Fatal("reconnect sent the saved password to a server not matching its renewed approved pin")
	}
	m.mu.Lock()
	retained := m.sessions[info.ID].request
	m.mu.Unlock()
	if retained.ExpectedHostFingerprint != renewedPin {
		t.Fatal("reconnected session retained an obsolete backend host pin")
	}
}

func TestFileConnectionCannotOverrideBackendHostFingerprint(t *testing.T) {
	f := newSSHFixture(t, nil, true)
	other, _ := generateSSHKey(t)
	req := f.request()
	req.CredentialID = "credential-file-pin-fixture"
	req.ExpectedHostFingerprint = ssh.FingerprintSHA256(other.PublicKey())
	file, err := OpenFileConnection(context.Background(), req, ssh.FingerprintSHA256(f.key.PublicKey()))
	if file != nil {
		_ = file.Close()
		t.Fatal("file connection ignored its backend-approved host fingerprint")
	}
	if err == nil || !strings.Contains(err.Error(), "saved credential destination") {
		t.Fatalf("conflicting caller pin returned %v", err)
	}
	if f.auth.Load() != 0 {
		t.Fatal("file connection sent credentials before resolving conflicting host pins")
	}
}

func TestReleaseSSHAuthenticationClearsSharedBackingArray(t *testing.T) {
	signer, _ := generateSSHKey(t)
	config := &ssh.ClientConfig{Auth: []ssh.AuthMethod{ssh.Password("fixture-password"), ssh.PublicKeys(signer)}}
	shallowCopy := *config
	retained := config.Auth
	releaseSSHAuthentication(config)
	if config.Auth != nil {
		t.Fatal("caller configuration retained authentication methods")
	}
	for _, alias := range [][]ssh.AuthMethod{retained, shallowCopy.Auth} {
		for _, method := range alias {
			if method != nil {
				t.Fatal("a shallow configuration copy retained password or signer callbacks")
			}
		}
	}
	releaseSSHAuthentication(nil)
	releaseSSHAuthentication(config)
}

func TestSSHRekeyAndLiveTrafficDoNotRequireRetainedAuthentication(t *testing.T) {
	f := newSSHFixture(t, func(config *ssh.ServerConfig) {
		config.RekeyThreshold = 1024
	}, true)
	m := sshManager(t, t.TempDir())
	req := f.request()
	req.TimeoutMs = 5000
	info, err := m.Connect(req)
	if err != nil {
		t.Fatal(err)
	}
	acceptCurrentPrompt(t, m, info.ID, "save")
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	m.mu.Lock()
	retained := m.sessions[info.ID].request
	m.mu.Unlock()
	assertNoRetainedAuthentication(t, retained)
	for round := 0; round < 3; round++ {
		want := bytes.Repeat([]byte{byte('a' + round)}, 32*1024)
		if err := m.Write(info.ID, want); err != nil {
			t.Fatal(err)
		}
		var got []byte
		deadline := time.NewTimer(5 * time.Second)
		for len(got) < len(want) {
			select {
			case chunk := <-f.input:
				got = append(got, chunk...)
			case <-deadline.C:
				t.Fatal("live SSH traffic stopped after authentication callbacks were released")
			}
		}
		deadline.Stop()
		if !bytes.Equal(got, want) {
			t.Fatal("rekey changed terminal input")
		}
	}
	if f.auth.Load() != 1 {
		t.Fatalf("rekey repeated user authentication %d times", f.auth.Load())
	}
	if current, err := m.Get(info.ID); err != nil || current.State != "connected" {
		t.Fatalf("session did not remain connected after rekey: %+v, %v", current, err)
	}
	if _, err := m.Events(context.Background(), 0, 0); err != nil {
		t.Fatal(err)
	}
}
