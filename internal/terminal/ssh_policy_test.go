package terminal

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestSSHCompatibilityPolicyIsFinitePerConnection(t *testing.T) {
	baseline := ssh.SupportedAlgorithms()
	modern := sshAlgorithmPolicy(SSHCompatibilityModern)
	wantHosts := []string{}
	for _, algorithm := range baseline.HostKeys {
		if !strings.Contains(algorithm, "-cert-") {
			wantHosts = append(wantHosts, algorithm)
		}
	}
	if !reflect.DeepEqual(modern.KeyExchanges, baseline.KeyExchanges) || !reflect.DeepEqual(modern.Ciphers, baseline.Ciphers) ||
		!reflect.DeepEqual(modern.MACs, baseline.MACs) || !reflect.DeepEqual(modern.HostKeys, wantHosts) {
		t.Fatal("modern policy changed")
	}
	legacy := sshAlgorithmPolicy(SSHCompatibilityLegacy)
	wantKex := append(slices.Clone(modern.KeyExchanges), ssh.InsecureKeyExchangeDH14SHA1, ssh.InsecureKeyExchangeDHGEXSHA1, ssh.InsecureKeyExchangeDH1SHA1)
	if !reflect.DeepEqual(legacy.KeyExchanges, wantKex) ||
		!reflect.DeepEqual(legacy.Ciphers, append(slices.Clone(modern.Ciphers), ssh.InsecureCipherAES128CBC)) ||
		!reflect.DeepEqual(legacy.HostKeys, append(slices.Clone(modern.HostKeys), ssh.KeyAlgoRSA)) ||
		!reflect.DeepEqual(legacy.MACs, modern.MACs) {
		t.Fatal("legacy profile did not append exactly its finite allowlist")
	}
	for _, denied := range []string{ssh.InsecureKeyAlgoDSA, ssh.InsecureCertAlgoDSAv01, ssh.CertAlgoRSAv01, ssh.InsecureCipherTripleDESCBC, ssh.InsecureCipherRC4, ssh.InsecureCipherRC4128, ssh.InsecureCipherRC4256, ssh.InsecureHMACSHA196} {
		for _, list := range [][]string{legacy.KeyExchanges, legacy.HostKeys, legacy.Ciphers, legacy.MACs} {
			if slices.Contains(list, denied) {
				t.Fatalf("legacy policy included forbidden algorithm %q", denied)
			}
		}
	}
	legacy.KeyExchanges[0], legacy.Ciphers[0], legacy.HostKeys[0], legacy.MACs[0] = "changed", "changed", "changed", "changed"
	if !reflect.DeepEqual(sshAlgorithmPolicy(SSHCompatibilityModern), modern) || !reflect.DeepEqual(ssh.SupportedAlgorithms(), baseline) {
		t.Fatal("a per-connection policy changed library or subsequent connection defaults")
	}
}

func TestSSHCompatibilityNormalizationAndSessionPolicy(t *testing.T) {
	for _, profile := range []string{"", SSHCompatibilityModern, SSHCompatibilityLegacy} {
		req := terminalRequest()
		req.SSHCompatibility = profile
		got, err := normalizeRequest(req)
		if err != nil {
			t.Fatal(err)
		}
		want := profile
		if want == "" {
			want = SSHCompatibilityModern
		}
		if got.SSHCompatibility != want {
			t.Fatalf("normalized profile %q, want %q", got.SSHCompatibility, want)
		}
		req.Type = "telnet"
		got, err = normalizeRequest(req)
		if err != nil || got.SSHCompatibility != SSHCompatibilityModern {
			t.Fatalf("non-SSH retained active legacy policy: %+v, %v", got, err)
		}
	}
	for _, invalid := range []string{"auto", "LEGACY", " legacy", "all", "insecure"} {
		req := terminalRequest()
		req.SSHCompatibility = invalid
		if _, err := normalizeRequest(req); err == nil {
			t.Fatalf("accepted unknown compatibility policy %q", invalid)
		}
	}
	m := testManager(t, func(ctx context.Context, req ConnectRequest, _ transportCallbacks) (transport, error) {
		if req.SSHCompatibility != SSHCompatibilityLegacy {
			return nil, errors.New("profile not delivered to transport")
		}
		<-ctx.Done()
		return nil, ctx.Err()
	})
	req := terminalRequest()
	req.SSHCompatibility = SSHCompatibilityLegacy
	info, err := m.Connect(req)
	if err != nil || info.SSHCompatibility != SSHCompatibilityLegacy {
		t.Fatalf("session did not publish selected policy: %+v %v", info, err)
	}
	if got := m.List(); len(got) != 1 || got[0].SSHCompatibility != SSHCompatibilityLegacy {
		t.Fatalf("restored snapshot lost policy: %+v", got)
	}
}

func TestSSHLegacyDiagnosisRequiresTypedRelevantOverlap(t *testing.T) {
	for _, test := range []struct {
		what string
		peer []string
		want bool
	}{
		{"key exchange", []string{ssh.InsecureKeyExchangeDHGEXSHA1, ssh.InsecureKeyExchangeDH1SHA1}, true},
		{"key exchange", []string{ssh.InsecureKeyExchangeDH14SHA1}, true},
		{"host key", []string{ssh.KeyAlgoRSA}, true},
		{"client to server cipher", []string{ssh.InsecureCipherAES128CBC}, true},
		{"server to client cipher", []string{ssh.InsecureCipherAES128CBC}, true},
		{"host key", []string{ssh.InsecureKeyAlgoDSA, ssh.CertAlgoRSAv01}, false},
		{"key exchange", []string{"vendor-unknown-kex"}, false},
		{"client to server cipher", []string{ssh.InsecureCipherTripleDESCBC, ssh.InsecureCipherRC4}, false},
		{"client to server MAC", []string{ssh.InsecureHMACSHA196}, false},
		{"host key", []string{ssh.InsecureKeyExchangeDH1SHA1}, false},
		{"key exchange", []string{ssh.KeyAlgoRSA}, false},
	} {
		original := fmt.Errorf("SSH handshake: %w", &ssh.AlgorithmNegotiationError{What: test.what, RequestedAlgorithms: test.peer, SupportedAlgorithms: []string{"modern-fixture"}})
		var typed *sshLegacyRequiredError
		got := diagnoseSSHHandshake(original, SSHCompatibilityModern)
		if errors.As(got, &typed) != test.want || got.Error() != original.Error() || !errors.Is(got, original) {
			t.Fatalf("incorrect diagnosis for %s %v: %T %v", test.what, test.peer, got, got)
		}
		if errors.As(diagnoseSSHHandshake(original, SSHCompatibilityLegacy), &typed) {
			t.Fatal("legacy mode suggested enabling itself")
		}
	}
	for _, original := range []error{errors.New("ssh: no common algorithm for key exchange; peer offered diffie-hellman-group1-sha1"), errors.New("authentication refused"), context.Canceled} {
		var typed *sshLegacyRequiredError
		if errors.As(diagnoseSSHHandshake(original, SSHCompatibilityModern), &typed) {
			t.Fatal("classified an untyped error by its message")
		}
	}
}

func rsaFixtureKey(t *testing.T) (ssh.Signer, string) {
	t.Helper()
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	key := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(private)})
	return signer, string(key)
}

func restrictedFixtureSigner(t *testing.T, signer ssh.Signer, algorithms ...string) ssh.Signer {
	t.Helper()
	algorithmSigner, ok := signer.(ssh.AlgorithmSigner)
	if !ok {
		t.Fatal("fixture key has no selectable signature algorithm")
	}
	restricted, err := ssh.NewSignerWithAlgorithms(algorithmSigner, algorithms)
	if err != nil {
		t.Fatal(err)
	}
	return restricted
}

func fixtureAlgorithms(t *testing.T, f *sshFixture) ssh.NegotiatedAlgorithms {
	t.Helper()
	select {
	case algorithms := <-f.algorithms:
		return algorithms
	case <-time.After(time.Second):
		t.Fatal("fixture did not report negotiated algorithms")
		return ssh.NegotiatedAlgorithms{}
	}
}

func TestSSHLegacyKEXRequiresExplicitOptIn(t *testing.T) {
	for _, kex := range []string{ssh.InsecureKeyExchangeDHGEXSHA1, ssh.InsecureKeyExchangeDH1SHA1, ssh.InsecureKeyExchangeDH14SHA1} {
		t.Run(kex, func(t *testing.T) {
			f := newSSHFixture(t, func(config *ssh.ServerConfig) { config.KeyExchanges = []string{kex} }, true)
			m := sshManager(t, t.TempDir())
			info, err := m.Connect(f.request())
			if err != nil {
				t.Fatal(err)
			}
			failed := awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "error" })
			if failed.ErrorCode != "ssh_legacy_required" || failed.SSHCompatibility != SSHCompatibilityModern || !strings.Contains(failed.Error, kex) || !strings.Contains(failed.Error, "peer offered") {
				t.Fatalf("missing typed negotiation diagnosis: %+v", failed)
			}
			if f.auth.Load() != 0 {
				t.Fatal("default connection bypassed negotiation policy")
			}
			req := f.request()
			req.SSHCompatibility = SSHCompatibilityLegacy
			info, err = m.Connect(req)
			if err != nil {
				t.Fatal(err)
			}
			pending := awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.Prompt != nil })
			if pending.Prompt.Kind != "host-key" || f.auth.Load() != 0 {
				t.Fatal("legacy mode bypassed host-key verification")
			}
			acceptCurrentPrompt(t, m, info.ID, "once")
			connected := awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
			if connected.SSHCompatibility != SSHCompatibilityLegacy || connected.ErrorCode != "" || fixtureAlgorithms(t, f).KeyExchange != kex {
				t.Fatalf("wrong legacy session policy or selected KEX: %+v", connected)
			}
		})
	}
}

func TestSSHModernKEXCanonicalVendorCompatibility(t *testing.T) {
	for _, kex := range []string{ssh.KeyExchangeMLKEM768X25519, ssh.KeyExchangeCurve25519} {
		t.Run(kex, func(t *testing.T) {
			f := newSSHFixture(t, func(config *ssh.ServerConfig) { config.KeyExchanges = []string{kex} }, true)
			m := sshManager(t, t.TempDir())
			info, err := m.Connect(f.request())
			if err != nil {
				t.Fatal(err)
			}
			pending := awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.Prompt != nil })
			if pending.Prompt.Kind != "host-key" || f.auth.Load() != 0 {
				t.Fatal("modern key exchange bypassed host verification")
			}
			acceptCurrentPrompt(t, m, info.ID, "once")
			connected := awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
			if connected.SSHCompatibility != SSHCompatibilityModern || fixtureAlgorithms(t, f).KeyExchange != kex {
				t.Fatalf("modern connection failed to negotiate the supported key exchange %q", kex)
			}
		})
	}
}

func TestSSHLegacyExactDeviceOfferRSAAndCBC(t *testing.T) {
	key, _ := rsaFixtureKey(t)
	key = restrictedFixtureSigner(t, key, ssh.KeyAlgoRSA)
	f := newSSHFixtureWithKey(t, key, func(config *ssh.ServerConfig) {
		config.KeyExchanges = []string{ssh.InsecureKeyExchangeDHGEXSHA1, ssh.InsecureKeyExchangeDH1SHA1}
		config.Ciphers = []string{ssh.InsecureCipherAES128CBC}
		config.MACs = []string{ssh.HMACSHA1}
	}, true)
	m := sshManager(t, t.TempDir())
	req := f.request()
	info, err := m.Connect(req)
	if err != nil {
		t.Fatal(err)
	}
	failed := awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "error" })
	if failed.ErrorCode != "ssh_legacy_required" || !strings.Contains(failed.Error, ssh.InsecureKeyExchangeDHGEXSHA1) || !strings.Contains(failed.Error, ssh.InsecureKeyExchangeDH1SHA1) {
		t.Fatalf("exact peer offer was not preserved: %+v", failed)
	}
	req.SSHCompatibility = SSHCompatibilityLegacy
	info, err = m.Connect(req)
	if err != nil {
		t.Fatal(err)
	}
	prompt := acceptCurrentPrompt(t, m, info.ID, "save")
	if prompt.Algorithm != ssh.KeyAlgoRSA || prompt.Fingerprint != ssh.FingerprintSHA256(key.PublicKey()) {
		t.Fatalf("RSA legacy trust did not retain SHA256 fingerprint: %+v", prompt)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	selected := fixtureAlgorithms(t, f)
	if selected.KeyExchange != ssh.InsecureKeyExchangeDHGEXSHA1 || selected.HostKey != ssh.KeyAlgoRSA || selected.Read.Cipher != ssh.InsecureCipherAES128CBC || selected.Write.Cipher != ssh.InsecureCipherAES128CBC || selected.Read.MAC != ssh.HMACSHA1 || selected.Write.MAC != ssh.HMACSHA1 {
		t.Fatalf("wrong finite compatibility algorithms: %+v", selected)
	}
	known, err := m.KnownHosts()
	if err != nil || len(known) != 1 || known[0].Fingerprint != prompt.Fingerprint {
		t.Fatalf("legacy host trust did not persist normally: %+v %v", known, err)
	}
}

func TestSSHLegacyStillPrefersModernAlgorithms(t *testing.T) {
	key, _ := rsaFixtureKey(t)
	f := newSSHFixtureWithKey(t, key, func(config *ssh.ServerConfig) {
		config.KeyExchanges = []string{ssh.InsecureKeyExchangeDH1SHA1, ssh.InsecureKeyExchangeDHGEXSHA1, ssh.KeyExchangeCurve25519}
		config.Ciphers = []string{ssh.InsecureCipherAES128CBC, ssh.CipherAES128CTR}
	}, true)
	m := sshManager(t, t.TempDir())
	req := f.request()
	req.SSHCompatibility = SSHCompatibilityLegacy
	info, err := m.Connect(req)
	if err != nil {
		t.Fatal(err)
	}
	acceptCurrentPrompt(t, m, info.ID, "once")
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	selected := fixtureAlgorithms(t, f)
	if selected.KeyExchange != ssh.KeyExchangeCurve25519 || selected.HostKey != ssh.KeyAlgoRSASHA256 || selected.Read.Cipher != ssh.CipherAES128CTR || selected.Write.Cipher != ssh.CipherAES128CTR {
		t.Fatalf("legacy mode displaced a common modern algorithm: %+v", selected)
	}
}

func TestSSHLegacyRSAClientSignatureOptInAndPreference(t *testing.T) {
	clientKey, privateKey := rsaFixtureKey(t)
	for _, test := range []struct {
		name    string
		profile string
		server  []string
		want    string
	}{
		{"modern rejects SHA1", SSHCompatibilityModern, []string{ssh.KeyAlgoRSA}, ""},
		{"legacy allows SHA1", SSHCompatibilityLegacy, []string{ssh.KeyAlgoRSA}, ssh.KeyAlgoRSA},
		{"legacy prefers SHA512", SSHCompatibilityLegacy, []string{ssh.KeyAlgoRSA, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSASHA512}, ssh.KeyAlgoRSASHA512},
	} {
		t.Run(test.name, func(t *testing.T) {
			verified := make(chan string, 1)
			f := newSSHFixture(t, func(config *ssh.ServerConfig) {
				config.PasswordCallback = nil
				config.PublicKeyAuthAlgorithms = test.server
				config.PublicKeyCallback = func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
					if !bytes.Equal(key.Marshal(), clientKey.PublicKey().Marshal()) {
						return nil, errors.New("wrong fixture key")
					}
					return nil, nil
				}
				config.VerifiedPublicKeyCallback = func(_ ssh.ConnMetadata, _ ssh.PublicKey, permissions *ssh.Permissions, algorithm string) (*ssh.Permissions, error) {
					verified <- algorithm
					return permissions, nil
				}
			}, true)
			m := sshManager(t, t.TempDir())
			req := f.request()
			req.AuthMethod, req.PrivateKey, req.Password, req.SSHCompatibility = "key", privateKey, "", test.profile
			info, err := m.Connect(req)
			if err != nil {
				t.Fatal(err)
			}
			acceptCurrentPrompt(t, m, info.ID, "once")
			if test.want == "" {
				failed := awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "error" })
				if !strings.Contains(failed.Error, "no common public key signature algorithm") || failed.ErrorCode != "" {
					t.Fatalf("modern client signature policy was not enforced: %+v", failed)
				}
				select {
				case algorithm := <-verified:
					t.Fatalf("modern mode signed with %s", algorithm)
				default:
				}
				return
			}
			awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
			select {
			case algorithm := <-verified:
				if algorithm != test.want {
					t.Fatalf("client signed with %s, want %s", algorithm, test.want)
				}
			case <-time.After(time.Second):
				t.Fatal("RSA client signature was not verified")
			}
			batch, err := m.Events(context.Background(), 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(batch)
			if bytes.Contains(raw, []byte("BEGIN RSA PRIVATE KEY")) {
				t.Fatal("private key was retained in protocol metadata")
			}
		})
	}
}

func TestSSHLegacyTrustCancellationAndChangedKey(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprint("changed=", changed), func(t *testing.T) {
			f := newSSHFixture(t, func(config *ssh.ServerConfig) { config.KeyExchanges = []string{ssh.InsecureKeyExchangeDH1SHA1} }, true)
			m := sshManager(t, t.TempDir())
			if changed {
				old, _ := generateSSHKey(t)
				if err := m.known.verify(context.Background(), f.listener.Addr().String(), old.PublicKey(), func(context.Context, Prompt) (PromptReply, error) { return PromptReply{Action: "save"}, nil }); err != nil {
					t.Fatal(err)
				}
			}
			req := f.request()
			req.SSHCompatibility = SSHCompatibilityLegacy
			info, err := m.Connect(req)
			if err != nil {
				t.Fatal(err)
			}
			pending := awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.Prompt != nil })
			if pending.Prompt.Changed != changed || f.auth.Load() != 0 {
				t.Fatal("legacy connection bypassed trust state")
			}
			if changed {
				for _, reply := range []PromptReply{{PromptID: pending.Prompt.ID, Action: "once"}, {PromptID: pending.Prompt.ID, Action: "replace", Fingerprint: "wrong"}} {
					if err := m.Reply(info.ID, reply); err == nil {
						t.Fatal("legacy connection bypassed exact changed-key verification")
					}
				}
				if err := m.Reply(info.ID, PromptReply{PromptID: pending.Prompt.ID, Action: "replace", Fingerprint: pending.Prompt.Fingerprint}); err != nil {
					t.Fatal(err)
				}
				awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
			}
			if err := m.Disconnect(info.ID); err != nil {
				t.Fatal(err)
			}
			closed := awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "closed" })
			if closed.Prompt != nil || closed.ErrorCode != "" {
				t.Fatalf("cancelled connection retained prompt or downgrade diagnosis: %+v", closed)
			}
			done := make(chan struct{})
			go func() { m.Close(); close(done) }()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("legacy SSH cancellation leaked connection workers")
			}
			if !changed && f.auth.Load() != 0 {
				t.Fatal("legacy authentication continued after trust cancellation")
			}
		})
	}
}
