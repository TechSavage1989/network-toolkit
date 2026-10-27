package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"networktoolkit/internal/desktop"
)

func fixtureVaultConfig(t *testing.T, v *credentialVault, id string) {
	t.Helper()
	raw, _ := json.Marshal(map[string]string{credentialConfigIdentityField: id})
	if err := os.WriteFile(v.configPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

// Complete the same explicit configuration binding that the HTTP native flow
// performs. A successful password derivation alone must not expose credentials.
func bindFixtureVault(t *testing.T, v *credentialVault) {
	t.Helper()
	id := credentialConfigID(v.configPath)
	if id == "" {
		id = strings.Repeat("b", 32)
		fixtureVaultConfig(t, v, id)
	}
	if err := v.remember(id, false); err != nil {
		t.Fatal(err)
	}
}

func reloadFixtureVault(t *testing.T, v *credentialVault) *credentialVault {
	t.Helper()
	next, err := newCredentialVaultWithConfig(v.configPath, v.protect, v.unprotect)
	if err != nil {
		t.Fatal(err)
	}
	return next
}
func fixtureUnconfiguredVaultApp(t *testing.T) *application {
	t.Helper()
	a := testApplication(t)
	a.credentials = syntheticUnconfiguredCredentialVault(t, filepath.Dir(a.dataPath))
	a.credentials.configPath = a.dataPath
	a.confirmVaultAction = func(context.Context, desktop.VaultActionRequest) (bool, error) { return true, nil }
	a.confirmVaultRecovery = func(context.Context, string) (bool, error) { return true, nil }
	return a
}
func fixturePrompt(password string, remember bool) func(context.Context, desktop.VaultSecretRequest) (desktop.VaultSecretResponse, error) {
	return func(context.Context, desktop.VaultSecretRequest) (desktop.VaultSecretResponse, error) {
		return desktop.VaultSecretResponse{Password: []byte(password), Remember: remember}, nil
	}
}

func TestVaultV2RequiresExplicitSetupAndDoesNotPromptAtStartup(t *testing.T) {
	v := syntheticUnconfiguredCredentialVault(t, t.TempDir())
	if got := v.status(); got.State != "setup-required" || got.Remembered || len(v.list()) != 0 {
		t.Fatalf("initial status: %+v", got)
	}
	if _, err := v.put(credentialRecord{Name: "No implicit setup", Kind: "password", Secret: "fixture"}); err == nil {
		t.Fatal("write implicitly created vault")
	}
	if _, err := os.Stat(v.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("startup created vault")
	}
}

func TestVaultV2RememberedRestartConfigResetAndManualLock(t *testing.T) {
	v := syntheticCredentialVault(t, t.TempDir())
	item, err := v.put(credentialRecord{Name: "Fixture", Kind: "password", Secret: "saved-secret-canary"})
	if err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("1", 32)
	fixtureVaultConfig(t, v, id)
	if err := v.remember(id, true); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(v.rememberPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range [][]byte{[]byte(syntheticMasterPassword), []byte("saved-secret-canary"), v.key} {
		if bytes.Contains(raw, secret) {
			t.Fatal("remembered record exposed plaintext material")
		}
	}
	restarted := reloadFixtureVault(t, v)
	if got := restarted.status(); got.State != "unlocked" || !got.Remembered {
		t.Fatalf("restart did not remember: %+v", got)
	}
	if record, ok := restarted.get(item.ID); !ok || record.Secret != "saved-secret-canary" {
		t.Fatal("remembered key did not authenticate payload")
	}
	vaultBefore, _ := os.ReadFile(v.path)
	if err := os.Remove(v.configPath); err != nil {
		t.Fatal(err)
	}
	if got := restarted.status(); got.State != "locked" || got.Remembered {
		t.Fatal("configuration deletion did not lock")
	}
	if _, ok := restarted.get(item.ID); ok {
		t.Fatal("configuration deletion retained credentials")
	}
	vaultAfter, _ := os.ReadFile(v.path)
	if !bytes.Equal(vaultBefore, vaultAfter) {
		t.Fatal("configuration deletion changed vault")
	}
	// Replaying an old remember record without its app identity cannot unlock.
	if err := os.WriteFile(v.rememberPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	reset := reloadFixtureVault(t, v)
	if reset.status().State != "locked" {
		t.Fatal("stale remember replay survived reset")
	}
	if err := reset.unlock(context.Background(), []byte(syntheticMasterPassword)); err != nil {
		t.Fatal(err)
	}
	newID := strings.Repeat("2", 32)
	fixtureVaultConfig(t, reset, newID)
	if err := reset.remember(newID, true); err != nil {
		t.Fatal(err)
	}
	if err := reset.lockAndForget(); err != nil {
		t.Fatal(err)
	}
	if len(reset.key) != 0 || len(reset.entries) != 0 || reset.status().State != "locked" {
		t.Fatal("manual lock retained in-memory access")
	}
	if _, err := os.Stat(reset.rememberPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("manual lock retained remembered key")
	}
	if reloadFixtureVault(t, reset).status().State != "locked" {
		t.Fatal("manual lock did not survive restart")
	}
}

func TestVaultV2LegacyMigrationCancellationAndNoCorruptionFallback(t *testing.T) {
	v := syntheticUnconfiguredCredentialVault(t, t.TempDir())
	plain, _ := json.Marshal(credentialVaultFile{Version: 1, Entries: []credentialRecord{{ID: "legacy-fixture", Name: "Legacy", Kind: "password", Secret: "legacy-secret"}}})
	legacy, err := v.protect(plain)
	clear(plain)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(v.legacyPath, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	v = reloadFixtureVault(t, v)
	if v.status().State != "legacy" || len(v.entries) != 0 {
		t.Fatal("legacy data automatically opened")
	}
	canceled := func(context.Context, string) (bool, error) { return false, nil }
	if err := v.setup(context.Background(), []byte(syntheticMasterPassword), true, canceled); err == nil {
		t.Fatal("canceled recovery activated vault")
	}
	if _, err := os.Stat(v.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("canceled setup persisted v2")
	}
	unchanged, _ := os.ReadFile(v.legacyPath)
	if !bytes.Equal(legacy, unchanged) {
		t.Fatal("canceled setup modified legacy")
	}
	if err := v.setup(context.Background(), []byte(syntheticMasterPassword), false, nil); err != nil {
		t.Fatal(err)
	}
	bindFixtureVault(t, v)
	if item, ok := v.get("legacy-fixture"); !ok || item.Secret != "legacy-secret" {
		t.Fatal("verified migration lost record")
	}
	if !v.status().LegacyBackup {
		t.Fatal("retained legacy warning missing")
	}
	unchanged, _ = os.ReadFile(v.legacyPath)
	if !bytes.Equal(legacy, unchanged) {
		t.Fatal("successful migration modified legacy")
	}
	if err := os.WriteFile(v.path, []byte(`{"version":2,"damaged":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	damaged, err := newCredentialVaultWithConfig(v.configPath, v.protect, v.unprotect)
	if err == nil || damaged == nil || damaged.status().State != "locked" || len(damaged.entries) != 0 {
		t.Fatal("corrupt v2 silently fell back to legacy")
	}
}

func TestVaultV2RotationInvalidatesOldMasterRecoveryAndRemember(t *testing.T) {
	v := syntheticUnconfiguredCredentialVault(t, t.TempDir())
	recoveryKey := ""
	if err := v.setup(context.Background(), []byte(syntheticMasterPassword), true, func(_ context.Context, key string) (bool, error) { recoveryKey = key; return true, nil }); err != nil {
		t.Fatal(err)
	}
	bindFixtureVault(t, v)
	item, err := v.put(credentialRecord{Name: "Rotate fixture", Kind: "password", Secret: "rotation-secret"})
	if err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("3", 32)
	fixtureVaultConfig(t, v, id)
	if err := v.remember(id, true); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(v.path)
	oldRemember, _ := os.ReadFile(v.rememberPath)
	oldKey := append([]byte(nil), v.key...)
	defer clear(oldKey)
	if err := v.rotate(context.Background(), []byte("wrong"), []byte("new synthetic master password"), "", false, nil); err == nil {
		t.Fatal("wrong master rotated vault")
	}
	if err := v.rotate(context.Background(), []byte(syntheticMasterPassword), []byte("new synthetic master password"), "", true, func(context.Context, string) (bool, error) { return false, nil }); err == nil {
		t.Fatal("canceled recovery rotation changed vault")
	}
	after, _ := os.ReadFile(v.path)
	if !bytes.Equal(before, after) {
		t.Fatal("failed rotation modified vault")
	}
	if err := v.rotate(context.Background(), nil, []byte("new synthetic master password"), recoveryKey, false, nil); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(oldKey, v.key) || v.envelope.HasRecovery() {
		t.Fatal("recovery did not rotate key/disable old recovery")
	}
	if _, _, err := v.envelope.UnlockRecovery(recoveryKey); err == nil {
		t.Fatal("old recovery remained valid")
	}
	if err := os.WriteFile(v.rememberPath, oldRemember, 0600); err != nil {
		t.Fatal(err)
	}
	replay := reloadFixtureVault(t, v)
	if replay.status().State != "locked" {
		t.Fatal("old remembered key survived rekey")
	}
	if err := replay.unlock(context.Background(), []byte(syntheticMasterPassword)); err == nil {
		t.Fatal("old master survived rekey")
	}
	if err := replay.unlock(context.Background(), []byte("new synthetic master password")); err != nil {
		t.Fatal(err)
	}
	bindFixtureVault(t, replay)
	if record, ok := replay.get(item.ID); !ok || record.Secret != "rotation-secret" {
		t.Fatal("rotation lost credentials")
	}
}

func TestVaultV2HTTPUsesNativeSecretsOnlyAndRemembersAfterSuccess(t *testing.T) {
	a := fixtureUnconfiguredVaultApp(t)
	prompts := 0
	a.promptVaultSecret = func(ctx context.Context, req desktop.VaultSecretRequest) (desktop.VaultSecretResponse, error) {
		prompts++
		return fixturePrompt(syntheticMasterPassword, true)(ctx, req)
	}
	for _, body := range []string{`{"action":"setup","password":"renderer-secret"}`, `{"action":"setup","recoveryKey":"renderer-secret"}`, `{"action":"setup","remember":true}`, `{"action":"setup"} {}`} {
		r := requestApp(a, http.MethodPost, "/api/credential-vault", body, true)
		if r.Code != 400 || prompts != 0 || strings.Contains(r.Body.String(), "renderer-secret") {
			t.Fatal("HTTP accepted/echoed a secret or opened native prompt")
		}
	}
	r := requestApp(a, http.MethodPost, "/api/credential-vault", `{"action":"setup"}`, true)
	if r.Code != 200 || prompts != 1 {
		t.Fatalf("native setup: %d %s", r.Code, r.Body.String())
	}
	if a.credentials.status().State != "unlocked" || !a.credentials.status().Remembered {
		t.Fatal("successful native setup did not remember")
	}
	originalID := credentialConfigID(a.dataPath)
	if originalID == "" {
		t.Fatal("config identity not persisted")
	}
	body := `{"theme":"dark","_credentialVaultConfigID":"` + strings.Repeat("f", 32) + `"}`
	if r := requestApp(a, http.MethodPut, "/api/state", body, true); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if credentialConfigID(a.dataPath) != originalID {
		t.Fatal("renderer replaced backend config identity")
	}
	if err := os.Remove(a.dataPath); err != nil {
		t.Fatal(err)
	}
	if r := requestApp(a, http.MethodPut, "/api/state", body, true); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if credentialConfigID(a.dataPath) != "" {
		t.Fatal("renderer resurrected reset configuration identity")
	}
	if a.credentials.status().State != "locked" {
		t.Fatal("reset did not require native unlock")
	}
	r = requestApp(a, http.MethodPost, "/api/credentials", `{"name":"Locked","kind":"password","secret":"fixture"}`, true)
	if r.Code != http.StatusLocked {
		t.Fatal("locked credential mutation not rejected")
	}
	r = requestApp(a, http.MethodGet, "/api/credentials", "", true)
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"state":"locked"`) || !strings.Contains(r.Body.String(), `"entries":[]`) {
		t.Fatal("locked metadata status unusable")
	}
}

func TestVaultV2CancellationAndRememberWriteFailureKeepSafeState(t *testing.T) {
	a := fixtureUnconfiguredVaultApp(t)
	a.promptVaultSecret = func(context.Context, desktop.VaultSecretRequest) (desktop.VaultSecretResponse, error) {
		return desktop.VaultSecretResponse{}, desktop.ErrVaultDialogCancelled
	}
	if r := requestApp(a, http.MethodPost, "/api/credential-vault", `{"action":"setup"}`, true); r.Code == 200 {
		t.Fatal("canceled native setup succeeded")
	}
	if _, err := os.Stat(a.credentials.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("cancel persisted vault")
	}
	if credentialConfigID(a.dataPath) != "" {
		t.Fatal("cancel changed config")
	}
	a.promptVaultSecret = fixturePrompt(syntheticMasterPassword, true)
	a.credentials.protect = func([]byte) ([]byte, error) { return nil, errors.New("synthetic unavailable provider") }
	r := requestApp(a, http.MethodPost, "/api/credential-vault", `{"action":"setup"}`, true)
	if r.Code != 200 || a.credentials.status().Remembered || a.credentials.status().Warning == "" {
		t.Fatalf("remember failure not reported safely: %d %s", r.Code, r.Body.String())
	}
	if err := a.credentials.lockAndForget(); err != nil {
		t.Fatal(err)
	}
	if err := a.credentials.unlock(context.Background(), []byte(syntheticMasterPassword)); err != nil {
		t.Fatal("remember failure destroyed valid master vault", err)
	}
}

func TestVaultV2ConcurrentMutationsRemainEncryptedAndComplete(t *testing.T) {
	v := syntheticCredentialVault(t, t.TempDir())
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := v.put(credentialRecord{Name: "Concurrent fixture", Kind: "password", Secret: "concurrent-secret"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if len(v.list()) != 8 {
		t.Fatal("concurrent save lost a record")
	}
	loaded := reloadFixtureVault(t, v)
	if err := loaded.unlock(context.Background(), []byte(syntheticMasterPassword)); err != nil {
		t.Fatal(err)
	}
	bindFixtureVault(t, loaded)
	if len(loaded.list()) != 8 {
		t.Fatal("atomic persisted payload is incomplete")
	}
}

func TestVaultV2DeleteRecordAndConfigurationNeverDeleteVault(t *testing.T) {
	v := syntheticCredentialVault(t, t.TempDir())
	id := strings.Repeat("4", 32)
	fixtureVaultConfig(t, v, id)
	if err := v.remember(id, true); err != nil {
		t.Fatal(err)
	}
	item, err := v.put(credentialRecord{Name: "Last credential", Kind: "password", Secret: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	oldID, oldGeneration := v.envelope.Identity()
	if err := v.delete(item.ID); err != nil {
		t.Fatal(err)
	}
	newID, newGeneration := v.envelope.Identity()
	if newID != oldID || newGeneration != oldGeneration {
		t.Fatal("ordinary record deletion unexpectedly rekeyed vault")
	}
	if _, err := os.Stat(v.path); err != nil {
		t.Fatal("last record deletion removed vault")
	}
	if !reloadFixtureVault(t, v).status().Remembered {
		t.Fatal("ordinary payload mutation invalidated remember")
	}
}

func TestVaultV2StorageDeletePreservesVaultsRemovesRemember(t *testing.T) {
	manager := storageFixture(t, false)
	root := manager.config.RootDir
	for _, workspace := range []string{"", storageTerminalDirectory} {
		for _, name := range []string{credentialVaultFilename, credentialVaultV2Filename} {
			writeStorageFixture(t, root, filepath.Join(workspace, name), "encrypted-fixture")
		}
		writeStorageFixture(t, root, filepath.Join(workspace, credentialRememberFilename), "remember-fixture")
		writeStorageFixture(t, root, filepath.Join(workspace, "Network_Toolkit.settings.json"), "config-fixture")
	}
	if err := manager.scheduleDelete(); err != nil {
		t.Fatal(err)
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	for _, workspace := range []string{"", storageTerminalDirectory} {
		for _, name := range []string{credentialVaultFilename, credentialVaultV2Filename} {
			assertStorageContents(t, filepath.Join(root, workspace, name), "encrypted-fixture")
		}
		for _, name := range []string{credentialRememberFilename, "Network_Toolkit.settings.json"} {
			if _, err := os.Stat(filepath.Join(root, workspace, name)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("configuration/remember survived deletion: %s", name)
			}
		}
	}
}

func TestVaultV2CleanupRequiresUnlockedCurrentVault(t *testing.T) {
	a := fixtureUnconfiguredVaultApp(t)
	calls := 0
	a.confirmVaultAction = func(context.Context, desktop.VaultActionRequest) (bool, error) { calls++; return true, nil }
	r := requestApp(a, http.MethodPost, "/api/credential-vault", `{"action":"cleanup-legacy"}`, true)
	if r.Code != http.StatusLocked || calls != 0 {
		t.Fatal("cleanup of sole legacy copy was offered without unlocked v2")
	}
}

func TestVaultV2CrossInstanceWritesCannotOverwriteEachOther(t *testing.T) {
	first := syntheticCredentialVault(t, t.TempDir())
	second := reloadFixtureVault(t, first)
	if err := second.unlock(context.Background(), []byte(syntheticMasterPassword)); err != nil {
		t.Fatal(err)
	}
	bindFixtureVault(t, second)
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, v := range []*credentialVault{first, second} {
		go func(v *credentialVault) {
			<-start
			_, err := v.put(credentialRecord{Name: "Cross-instance fixture", Kind: "password", Secret: "fixture"})
			results <- err
		}(v)
	}
	close(start)
	successes := 0
	for i := 0; i < 2; i++ {
		if <-results == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected one safe commit and one stale/busy rejection, got %d commits", successes)
	}
	loaded := reloadFixtureVault(t, first)
	if err := loaded.unlock(context.Background(), []byte(syntheticMasterPassword)); err != nil {
		t.Fatal(err)
	}
	bindFixtureVault(t, loaded)
	if len(loaded.list()) != 1 {
		t.Fatal("cross-instance transaction lost or corrupted committed records")
	}
}

func TestVaultV2LockInvalidatesOtherOpenInstancesAndStaleRemember(t *testing.T) {
	first := syntheticCredentialVault(t, t.TempDir())
	id := strings.Repeat("5", 32)
	fixtureVaultConfig(t, first, id)
	if err := first.remember(id, true); err != nil {
		t.Fatal(err)
	}
	second := reloadFixtureVault(t, first)
	if second.status().State != "unlocked" {
		t.Fatal("fixture restart did not unlock")
	}
	oldRemember, err := os.ReadFile(first.rememberPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.lockAndForget(); err != nil {
		t.Fatal(err)
	}
	if credentialConfigID(first.configPath) == id {
		t.Fatal("lock did not invalidate shared configuration identity")
	}
	if second.status().State != "locked" || len(second.key) != 0 || len(second.entries) != 0 {
		t.Fatal("another open instance retained credential access after lock")
	}
	if err := os.WriteFile(first.rememberPath, oldRemember, 0600); err != nil {
		t.Fatal(err)
	}
	if reloadFixtureVault(t, first).status().State != "locked" {
		t.Fatal("manual lock accepted old remember replay")
	}
}

func TestVaultV2TransactionGuardAndStaleCandidatePreserveDisk(t *testing.T) {
	v := syntheticCredentialVault(t, t.TempDir())
	before, err := os.ReadFile(v.path)
	if err != nil {
		t.Fatal(err)
	}
	release, err := acquireCredentialGuard(filepath.Dir(v.path))
	if err != nil {
		t.Fatal(err)
	}
	_, err = v.put(credentialRecord{Name: "Guarded fixture", Kind: "password", Secret: "fixture"})
	release()
	if err == nil {
		t.Fatal("second transaction ignored process-shared lock")
	}
	if err := writeCredentialAtomic(v.path, []byte("candidate"), []byte("stale expected value"), 4<<20); err == nil {
		t.Fatal("stale expected snapshot overwrote vault")
	}
	after, err := os.ReadFile(v.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("rejected transaction changed disk")
	}
	if len(v.list()) != 0 {
		t.Fatal("rejected transaction changed memory")
	}
}

func TestVaultV2RejectsNoncanonicalLegacyIDsBeforeActivation(t *testing.T) {
	v := syntheticUnconfiguredCredentialVault(t, t.TempDir())
	plain, _ := json.Marshal(credentialVaultFile{Version: 1, Entries: []credentialRecord{
		{ID: "fixture", Name: "First", Kind: "password", Secret: "one"},
		{ID: " fixture", Name: "Second", Kind: "password", Secret: "two"},
	}})
	encrypted, err := v.protect(plain)
	clear(plain)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(v.legacyPath, encrypted, 0600); err != nil {
		t.Fatal(err)
	}
	v = reloadFixtureVault(t, v)
	if err := v.setup(context.Background(), []byte(syntheticMasterPassword), false, nil); err == nil {
		t.Fatal("ambiguous canonical IDs migrated")
	}
	if _, err := os.Stat(v.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("invalid legacy payload activated v2")
	}
	after, _ := os.ReadFile(v.legacyPath)
	if !bytes.Equal(after, encrypted) {
		t.Fatal("invalid legacy input was modified")
	}
}

func TestVaultV2NativeSecretBuffersAreClearedAndDialogsDoNotQueue(t *testing.T) {
	a := fixtureUnconfiguredVaultApp(t)
	secret := []byte(syntheticMasterPassword)
	a.promptVaultSecret = func(context.Context, desktop.VaultSecretRequest) (desktop.VaultSecretResponse, error) {
		return desktop.VaultSecretResponse{Password: secret}, nil
	}
	a.credentialChanges.Lock()
	busy := requestApp(a, http.MethodPost, "/api/credential-vault", `{"action":"setup"}`, true)
	a.credentialChanges.Unlock()
	if busy.Code != http.StatusConflict {
		t.Fatal("queued another native dialog")
	}
	result := requestApp(a, http.MethodPost, "/api/credential-vault", `{"action":"setup"}`, true)
	if result.Code != 200 {
		t.Fatalf("setup: %d %s", result.Code, result.Body.String())
	}
	for _, b := range secret {
		if b != 0 {
			t.Fatal("native password byte buffer retained after request")
		}
	}
	if strings.Contains(result.Body.String(), syntheticMasterPassword) {
		t.Fatal("native password crossed HTTP response")
	}
}

func TestVaultV2PendingAuthenticationCannotExposeCredentials(t *testing.T) {
	v := syntheticCredentialVault(t, t.TempDir())
	item, err := v.put(credentialRecord{Name: "Pending fixture", Kind: "password", Secret: "pending-secret-canary"})
	if err != nil {
		t.Fatal(err)
	}
	v.close()
	if err := v.unlock(context.Background(), []byte(syntheticMasterPassword)); err != nil {
		t.Fatal(err)
	}
	if v.state != "pending-authentication" || v.status().State != "locked" {
		t.Fatal("password derivation exposed an unbound unlocked state")
	}
	var readers sync.WaitGroup
	for i := 0; i < 8; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			if v.requireUnlocked() == nil || v.status().State != "locked" || len(v.list()) != 0 {
				t.Error("concurrent reader observed unbound authenticated material")
			}
			if _, ok := v.get(item.ID); ok {
				t.Error("pending lookup exposed a credential")
			}
		}()
	}
	readers.Wait()
	bindFixtureVault(t, v)
	if record, ok := v.get(item.ID); !ok || record.Secret != "pending-secret-canary" {
		t.Fatal("completed guarded binding lost authenticated credentials")
	}
}

func TestVaultV2StaleInstanceCannotDeleteNewRememberedUnlock(t *testing.T) {
	first := syntheticCredentialVault(t, t.TempDir())
	oldID := credentialConfigID(first.configPath)
	if err := first.remember(oldID, true); err != nil {
		t.Fatal(err)
	}
	oldRemember, err := os.ReadFile(first.rememberPath)
	if err != nil {
		t.Fatal(err)
	}
	stale := reloadFixtureVault(t, first)
	if err := first.lockAndForget(); err != nil {
		t.Fatal(err)
	}
	if err := first.unlock(context.Background(), []byte(syntheticMasterPassword)); err != nil {
		t.Fatal(err)
	}
	if err := first.remember(credentialConfigID(first.configPath), true); err != nil {
		t.Fatal(err)
	}
	freshRemember, err := os.ReadFile(first.rememberPath)
	if err != nil {
		t.Fatal(err)
	}
	if stale.status().State != "locked" {
		t.Fatal("old instance retained access after lock")
	}
	if raw, err := os.ReadFile(first.rememberPath); err != nil || !bytes.Equal(raw, freshRemember) {
		t.Fatal("stale instance removed the newly authenticated remember record")
	}
	// Constructor snapshots stale bytes, then another instance replaces them
	// before decryption/identity validation finishes. Stale cleanup must not
	// delete the replacement file.
	if err := os.WriteFile(first.rememberPath, oldRemember, 0600); err != nil {
		t.Fatal(err)
	}
	unprotect := func(raw []byte) ([]byte, error) {
		if err := os.WriteFile(first.rememberPath, freshRemember, 0600); err != nil {
			return nil, err
		}
		return first.unprotect(raw)
	}
	loaded, err := newCredentialVaultWithConfig(first.configPath, first.protect, unprotect)
	if err != nil || loaded.status().State != "locked" {
		t.Fatal("stale startup record unlocked")
	}
	if raw, err := os.ReadFile(first.rememberPath); err != nil || !bytes.Equal(raw, freshRemember) {
		t.Fatal("startup stale snapshot removed a newer remembered record")
	}
	if reloadFixtureVault(t, first).status().State != "unlocked" {
		t.Fatal("fresh remembered record no longer unlocks")
	}
}

func TestVaultV2ConfigBindingFailureClearsPendingKey(t *testing.T) {
	a := fixtureUnconfiguredVaultApp(t)
	a.promptVaultSecret = fixturePrompt(syntheticMasterPassword, true)
	if r := requestApp(a, http.MethodPost, "/api/credential-vault", `{"action":"setup"}`, true); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	other := reloadFixtureVault(t, a.credentials)
	before, err := os.ReadFile(a.credentials.rememberPath)
	if err != nil {
		t.Fatal(err)
	}
	a.credentials.close()
	release, err := acquireCredentialGuard(filepath.Dir(a.dataPath))
	if err != nil {
		t.Fatal(err)
	}
	response := requestApp(a, http.MethodPost, "/api/credential-vault", `{"action":"unlock"}`, true)
	release()
	if response.Code != 200 || a.credentials.status().State != "locked" || len(a.credentials.key) != 0 || len(a.credentials.entries) != 0 || a.credentials.status().Warning == "" {
		t.Fatalf("failed config binding retained usable key: %d %s", response.Code, response.Body.String())
	}
	if raw, err := os.ReadFile(a.credentials.rememberPath); err != nil || !bytes.Equal(raw, before) {
		t.Fatal("binding failure modified another instance's remembered record")
	}
	if err := other.lockAndForget(); err != nil {
		t.Fatal(err)
	}
	if a.credentials.requireUnlocked() == nil || a.credentials.status().State != "locked" {
		t.Fatal("failed binding escaped subsequent cross-instance lock")
	}
}
