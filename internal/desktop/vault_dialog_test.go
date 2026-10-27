package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestVaultDialogPasswordValidationPreservesUnicodeAndSpaces(t *testing.T) {
	for _, value := range []string{"twelve chars", "            ", "  significant spaces  ", strings.Repeat("🔐", 12), strings.Repeat("a", 1024)} {
		if err := validateVaultSecret([]byte(value), true); err != nil {
			t.Fatalf("valid synthetic password rejected: %v", err)
		}
	}
	for _, value := range [][]byte{nil, []byte("elevenchars"), []byte(strings.Repeat("🔐", 11)), []byte(strings.Repeat("a", 1025)), []byte(strings.Repeat("界", 342)), []byte("validlength\x00password"), {0xff, 0xfe}} {
		if err := validateVaultSecret(value, true); err == nil {
			t.Fatal("invalid synthetic password accepted")
		}
	}
	if err := validateVaultSecret([]byte("short legacy"), false); err != nil {
		t.Fatal("unlock incorrectly applies new-password length requirements")
	}
}

func TestVaultDialogSecretModesAndBackendOnlyResponse(t *testing.T) {
	for _, mode := range []VaultSecretMode{VaultSecretSetup, VaultSecretUnlock, VaultSecretRotate, VaultSecretRecover} {
		t.Run(string(mode), func(t *testing.T) {
			password := []byte("  synthetic🔐secret  ")
			confirmation := append([]byte(nil), password...)
			current := []byte("current synthetic secret")
			recovery := []byte("SYNTHETIC-OFFLINE-KEY")
			unexpected := []byte("must be discarded")
			response, err := promptVaultSecret(context.Background(), VaultSecretRequest{Mode: mode}, func(_ context.Context, spec vaultDialogSpec) (vaultDialogResult, error) {
				if !spec.remember || spec.recovery != (mode != VaultSecretUnlock) {
					t.Error("remember/recovery availability did not match the mode")
				}
				for _, field := range spec.fields {
					if field.id == vaultFieldRecovery && field.masked {
						t.Error("recovery-key entry was mislabeled as password entry")
					}
					if field.id != vaultFieldRecovery && (!field.masked || field.value != "") {
						t.Error("password fields were not empty native masked inputs")
					}
				}
				return vaultDialogResult{accepted: true, remember: false, generateRecovery: true, values: map[uint16][]byte{vaultFieldPassword: password, vaultFieldConfirm: confirmation, vaultFieldCurrent: current, vaultFieldRecovery: recovery, 999: unexpected}}, nil
			})
			if err != nil || !bytes.Equal(response.Password, []byte("  synthetic🔐secret  ")) || response.Remember || response.GenerateRecovery != (mode != VaultSecretUnlock) {
				t.Fatalf("native secret response contract failed: %v", err)
			}
			if (len(response.CurrentPassword) != 0) != (mode == VaultSecretRotate) || (len(response.RecoveryKey) != 0) != (mode == VaultSecretRecover) {
				t.Fatal("response leaked secrets unrelated to the requested mode")
			}
			if bytes.Count(confirmation, []byte{0}) != len(confirmation) || bytes.Count(unexpected, []byte{0}) != len(unexpected) {
				t.Fatal("unused secret buffers were not cleared")
			}
			encoded, err := json.Marshal(response)
			if err != nil || strings.Contains(string(encoded), "Password") || strings.Contains(string(encoded), "RecoveryKey") {
				t.Fatal("native secrets were JSON serializable")
			}
			clearVaultSecretResponse(&response)
			if len(response.Password) != 0 || bytes.Count(password, []byte{0}) != len(password) {
				t.Fatal("backend cleanup helper did not clear the owned password")
			}
		})
	}
}

func TestVaultDialogRejectsInvalidResponsesAndNeverEchoesSecrets(t *testing.T) {
	for _, mode := range []VaultSecretMode{VaultSecretSetup, VaultSecretRotate, VaultSecretRecover} {
		for _, issue := range []string{"mismatch", "short", "missing-factor"} {
			if issue == "missing-factor" && mode == VaultSecretSetup {
				continue
			}
			secret := []byte("synthetic master secret")
			confirmation := append([]byte(nil), secret...)
			if issue == "mismatch" {
				confirmation = []byte("different synthetic secret")
			}
			if issue == "short" {
				secret, confirmation = []byte("short"), []byte("short")
			}
			values := map[uint16][]byte{vaultFieldPassword: secret, vaultFieldConfirm: confirmation}
			if issue != "missing-factor" {
				values[vaultFieldCurrent] = []byte("current test secret")
				values[vaultFieldRecovery] = []byte("SYNTHETIC-KEY")
			}
			response, err := promptVaultSecret(context.Background(), VaultSecretRequest{Mode: mode}, func(context.Context, vaultDialogSpec) (vaultDialogResult, error) {
				return vaultDialogResult{accepted: true, values: values}, nil
			})
			if err == nil || len(response.Password) != 0 || strings.Contains(err.Error(), "synthetic") || bytes.Count(secret, []byte{0}) != len(secret) {
				t.Fatal("invalid native response was accepted, retained or echoed")
			}
		}
	}
	if _, err := promptVaultSecret(context.Background(), VaultSecretRequest{Mode: "unknown"}, func(context.Context, vaultDialogSpec) (vaultDialogResult, error) {
		t.Error("invalid mode reached native UI")
		return vaultDialogResult{}, nil
	}); err == nil {
		t.Fatal("unknown native mode was accepted")
	}
	if _, err := promptVaultSecret(nil, VaultSecretRequest{Mode: VaultSecretUnlock}, nil); err == nil {
		t.Fatal("nil context was accepted")
	}
}

func TestVaultDialogCancellationSerializesAndDiscardsLateAcceptance(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	secret := []byte("synthetic unlock secret")
	go func() {
		_, err := promptVaultSecret(ctx, VaultSecretRequest{Mode: VaultSecretUnlock}, func(context.Context, vaultDialogSpec) (vaultDialogResult, error) {
			close(entered)
			<-release
			return vaultDialogResult{accepted: true, values: map[uint16][]byte{vaultFieldPassword: secret}}, nil
		})
		finished <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("synthetic native callback did not start")
	}
	cancel()
	if _, err := promptVaultSecret(context.Background(), VaultSecretRequest{Mode: VaultSecretUnlock}, func(context.Context, vaultDialogSpec) (vaultDialogResult, error) {
		t.Error("concurrent dialog opened")
		return vaultDialogResult{}, nil
	}); err == nil {
		t.Error("concurrent security decision was not rejected")
	}
	request := CredentialDestinationConfirmation{Action: "allow", CredentialName: "Synthetic", Protocol: "ssh", Host: "example.invalid", Port: 22, Fingerprint: "SHA256:synthetic"}
	if allowed, err := confirmCredentialDestination(context.Background(), request, func(string) (bool, error) { t.Error("destination approval overlapped vault prompt"); return true, nil }); allowed || err == nil {
		t.Error("vault and destination dialogs did not share the security slot")
	}
	close(release)
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("late native acceptance survived cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled synthetic callback did not finish")
	}
	if len(credentialConfirmationSlot) != 0 || bytes.Count(secret, []byte{0}) != len(secret) {
		t.Fatal("cancelled secret/serialization state was retained")
	}
}

func TestVaultDialogActionDefaultsAndDenial(t *testing.T) {
	for _, action := range []VaultAction{VaultActionForget, VaultActionLock, VaultActionDelete, VaultActionLegacyCleanup} {
		spec, err := vaultActionSpec(VaultActionRequest{Action: action, Target: "fixture\u202e"})
		if err != nil || !spec.defaultNo || len(spec.fields) != 0 || !strings.Contains(spec.body, "Existing authenticated connections stay connected") || strings.ContainsRune(spec.body, '\u202e') {
			t.Fatal("destructive native action omitted safe default, quoting or session policy")
		}
		if action == VaultActionDelete && (!strings.Contains(spec.body, "ALL") || !strings.Contains(spec.body, "legacy DPAPI-only copy") || !strings.Contains(spec.body, "old disk blocks")) {
			t.Fatal("delete confirmation omitted scope and retained-copy limits")
		}
		for _, fixture := range []struct {
			accepted bool
			err      error
		}{{false, nil}, {false, ErrVaultDialogCancelled}, {true, errors.New("synthetic native failure")}} {
			allowed, err := confirmVaultAction(context.Background(), VaultActionRequest{Action: action}, func(context.Context, vaultDialogSpec) (vaultDialogResult, error) {
				return vaultDialogResult{accepted: fixture.accepted}, fixture.err
			})
			if allowed || (fixture.err != nil && !errors.Is(fixture.err, ErrVaultDialogCancelled) && err == nil) {
				t.Fatal("native denial/failure approved a destructive action")
			}
		}
	}
	for _, request := range []VaultActionRequest{{Action: "unknown"}, {Action: VaultActionDelete, Target: "injected\nlabel"}, {Action: VaultActionDelete, Target: strings.Repeat("a", 121)}} {
		if _, err := vaultActionSpec(request); err == nil {
			t.Fatal("invalid action metadata was accepted")
		}
	}
}

func TestVaultDialogRecoveryRequiresExactReentryAndAcknowledgement(t *testing.T) {
	const key = "SYNTHETIC-OFFLINE-RECOVERY-KEY"
	for _, fixture := range []struct {
		entered   string
		ack, want bool
	}{{key, true, true}, {key, false, false}, {strings.ToLower(key), true, false}, {key + " ", true, false}, {"", true, false}} {
		entered := []byte(fixture.entered)
		allowed, err := confirmVaultRecovery(context.Background(), key, func(_ context.Context, spec vaultDialogSpec) (vaultDialogResult, error) {
			if !spec.defaultNo || !spec.acknowledge || !spec.fields[0].readonly || spec.fields[0].masked || spec.fields[1].masked || spec.fields[0].value != key || !strings.Contains(spec.body, "Anyone with this recovery key can unlock") || !strings.Contains(spec.body, "cannot be recovered") {
				t.Error("offline recovery disclosure/control contract was incomplete")
			}
			return vaultDialogResult{accepted: true, acknowledged: fixture.ack, values: map[uint16][]byte{vaultFieldRecovery: entered}}, nil
		})
		if allowed != fixture.want || (err == nil) != fixture.want || bytes.Count(entered, []byte{0}) != len(entered) {
			t.Fatal("recovery re-entry or cleanup contract failed")
		}
	}
	for _, invalid := range []string{"", "injected\nkey", strings.Repeat("k", 1025)} {
		if allowed, err := confirmVaultRecovery(context.Background(), invalid, nil); allowed || err == nil {
			t.Fatal("invalid offline key reached native display")
		}
	}
}

func TestVaultDialogUnsupportedPlatformFailsClosed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := PromptVaultSecret(ctx, VaultSecretRequest{Mode: VaultSecretUnlock}); err == nil || len(result.Password) != 0 {
		t.Fatal("cancelled native prompt did not fail closed")
	}
	if runtime.GOOS == "windows" {
		return // Never open an interactive Windows prompt in an ungated unit test.
	}
	if result, err := PromptVaultSecret(context.Background(), VaultSecretRequest{Mode: VaultSecretSetup}); err == nil || len(result.Password) != 0 {
		t.Fatal("non-Windows secret entry did not fail closed")
	}
	if allowed, err := ConfirmVaultAction(context.Background(), VaultActionRequest{Action: VaultActionDelete}); allowed || err == nil {
		t.Fatal("non-Windows destructive confirmation did not fail closed")
	}
	if allowed, err := ConfirmVaultRecovery(context.Background(), "SYNTHETIC-KEY"); allowed || err == nil {
		t.Fatal("non-Windows recovery display did not fail closed")
	}
}
