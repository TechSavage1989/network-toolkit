package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"networktoolkit/internal/vaultcrypto"
)

const credentialVaultV2Filename = "Network_Toolkit.credentials.v2.vault"
const credentialRememberFilename = "Network_Toolkit.credentials.remember"
const credentialTransactionFilename = "Network_Toolkit.credentials.lock"
const credentialConfigIdentityField = "_credentialVaultConfigID"
const credentialRememberLimit = 64 << 10

var errCredentialVaultLocked = errors.New("Unlock the credential vault in the password manager before using saved credentials.")

type credentialVaultStatus struct {
	Available    bool   `json:"available"`
	State        string `json:"state"`
	Remembered   bool   `json:"remembered"`
	HasRecovery  bool   `json:"hasRecovery"`
	LegacyBackup bool   `json:"legacyBackup"`
	Warning      string `json:"warning,omitempty"`
	Protection   string `json:"protection"`
}

type credentialRememberRecord struct {
	Version    int    `json:"version"`
	ConfigID   string `json:"configID"`
	VaultID    string `json:"vaultID"`
	Generation string `json:"generation"`
	Key        []byte `json:"key"`
}

func newCredentialVault(root string) (*credentialVault, error) {
	return newCredentialVaultForSettings(filepath.Join(root, "Network_Toolkit.settings.json"))
}
func newCredentialVaultForSettings(settingsPath string) (*credentialVault, error) {
	if !credentialProtectionAvailable() {
		return nil, errors.New("Saved credentials are unavailable: this build has no supported operating-system protection provider.")
	}
	return newCredentialVaultWithConfig(settingsPath, protectCredentialBlob, unprotectCredentialBlob)
}

// This injectable provider is Go-only, for isolated synthetic fixtures. No HTTP,
// environment or command-line option selects a weaker production provider.
func newCredentialVaultWithProtection(root string, protect, unprotect func([]byte) ([]byte, error)) (*credentialVault, error) {
	return newCredentialVaultWithConfig(filepath.Join(root, "Network_Toolkit.settings.json"), protect, unprotect)
}
func newCredentialVaultWithConfig(settingsPath string, protect, unprotect func([]byte) ([]byte, error)) (*credentialVault, error) {
	if protect == nil || unprotect == nil {
		return nil, errors.New("Credential protection is unavailable.")
	}
	root := filepath.Dir(settingsPath)
	v := &credentialVault{path: filepath.Join(root, credentialVaultV2Filename), legacyPath: filepath.Join(root, credentialVaultFilename), rememberPath: filepath.Join(root, credentialRememberFilename), configPath: settingsPath, entries: map[string]credentialRecord{}, protect: protect, unprotect: unprotect, state: "setup-required"}
	err := v.loadV2()
	if err != nil {
		v.state = "locked"
		v.warning = "The credential vault could not be opened. Existing encrypted data was kept; no legacy fallback was attempted."
	}
	return v, err
}

// Bounded and identity-checked reads reject links, devices and replaced files.
func readCredentialFile(path string, limit int) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || storageIsLink(before) {
		return nil, errors.New("Credential storage must be a regular file, not a link or device.")
	}
	if before.Size() > int64(limit) {
		return nil, errors.New("Credential storage exceeds its supported size limit.")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	after, err := f.Stat()
	if err != nil || !after.Mode().IsRegular() || storageIsLink(after) || !os.SameFile(before, after) {
		return nil, errors.New("Credential storage changed while opening it.")
	}
	raw, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > limit {
		return nil, errors.New("Credential storage exceeds its supported size limit.")
	}
	return raw, nil
}

func credentialConfigID(path string) string {
	raw, err := readCredentialFile(path, 2<<20)
	if err != nil {
		return ""
	}
	var state map[string]json.RawMessage
	if json.Unmarshal(raw, &state) != nil {
		return ""
	}
	var id string
	if json.Unmarshal(state[credentialConfigIdentityField], &id) != nil || len(id) != 32 {
		return ""
	}
	decoded, err := hex.DecodeString(id)
	if err != nil || len(decoded) != 16 {
		return ""
	}
	return id
}

func (v *credentialVault) loadV2() error {
	raw, err := readCredentialFile(v.path, vaultcrypto.MaxEnvelopeBytes)
	if errors.Is(err, os.ErrNotExist) {
		legacy, legacyErr := readCredentialFile(v.legacyPath, 2<<20)
		if legacyErr == nil {
			clear(legacy)
			v.state = "legacy"
			return nil
		}
		if errors.Is(legacyErr, os.ErrNotExist) {
			return nil
		}
		return legacyErr
	}
	if err != nil {
		return err
	}
	v.raw = raw
	v.envelope, err = vaultcrypto.Parse(raw)
	if err != nil {
		return errors.New("The v2 credential vault is invalid; the legacy copy was not used.")
	}
	v.state = "locked"
	v.configID = credentialConfigID(v.configPath)
	remembered, err := readCredentialFile(v.rememberPath, credentialRememberLimit)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		v.warning = "Remembered unlock is unavailable. Enter the master password."
		return nil
	}
	plain, err := v.unprotect(remembered)
	if err != nil {
		v.warning = "Remembered unlock is unavailable. Enter the master password."
		return nil
	}
	defer clear(plain)
	var record credentialRememberRecord
	defer func() { clear(record.Key) }()
	if json.Unmarshal(plain, &record) != nil {
		v.warning = "Remembered unlock is invalid. Enter the master password."
		return nil
	}
	id, generation := v.envelope.Identity()
	if record.Version != 2 || v.configID == "" || record.ConfigID != v.configID || record.VaultID != id || record.Generation != generation || len(record.Key) != vaultcrypto.KeyBytes {
		// This snapshot may already have been replaced by another instance's
		// newly authenticated remember record. Ignore stale ciphertext; only
		// explicit guarded actions remove records.
		v.warning = "Remembered unlock expired after a vault or configuration change. Enter the master password."
		return nil
	}
	payload, err := v.envelope.OpenKey(record.Key)
	if err != nil {
		v.warning = "Remembered unlock could not authenticate this vault. Enter the master password."
		return nil
	}
	defer clear(payload)
	entries, err := decodeCredentialPayload(payload, 2)
	if err != nil {
		return err
	}
	v.entries = entries
	v.key = append([]byte(nil), record.Key...)
	v.state = "unlocked"
	v.remembered = true
	return nil
}

func decodeCredentialPayload(plain []byte, version int) (map[string]credentialRecord, error) {
	var file credentialVaultFile
	if len(plain) > vaultcrypto.MaxPayloadBytes || json.Unmarshal(plain, &file) != nil || file.Version != version || len(file.Entries) > maxCredentialEntries {
		return nil, errors.New("Credential vault payload is invalid or exceeds its limit.")
	}
	entries := make(map[string]credentialRecord, len(file.Entries))
	for _, item := range file.Entries {
		if item.ID == "" || item.Secret == "" {
			return nil, errors.New("Credential vault contains an invalid record.")
		}
		if _, duplicate := entries[item.ID]; duplicate {
			return nil, errors.New("Credential vault contains duplicate record IDs.")
		}
		clean, err := sanitizeCredentialRecord(item, item.ID)
		if err != nil || clean.ID != item.ID {
			return nil, errors.New("Credential vault contains an invalid or noncanonical record.")
		}
		clean.UpdatedAt = item.UpdatedAt
		entries[item.ID] = clean
	}
	return entries, nil
}
func marshalCredentialPayload(entries map[string]credentialRecord) ([]byte, error) {
	items := make([]credentialRecord, 0, len(entries))
	for _, item := range entries {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	plain, err := json.Marshal(credentialVaultFile{Version: 2, Entries: items})
	if err != nil {
		return nil, err
	}
	if len(plain) > vaultcrypto.MaxPayloadBytes {
		clear(plain)
		return nil, errors.New("Credential vault would exceed its supported 2 MiB payload limit. Remove or shorten an entry first.")
	}
	return plain, nil
}

func (v *credentialVault) clearMaterialLocked() {
	clear(v.key)
	v.key = nil
	// Go strings and copies already handed to authenticated sessions cannot be
	// guaranteed erased; dropping records prevents any future credential lookup.
	clear(v.entries)
	v.entries = map[string]credentialRecord{}
	if v.envelope != nil || len(v.raw) > 0 {
		v.state = "locked"
	}
	v.remembered = false
}
func (v *credentialVault) close() { v.mu.Lock(); defer v.mu.Unlock(); v.clearMaterialLocked() }
func (v *credentialVault) requireUnlockedLocked() error {
	if v.state != "unlocked" || len(v.key) != vaultcrypto.KeyBytes {
		return errCredentialVaultLocked
	}
	if v.configID == "" || credentialConfigID(v.configPath) != v.configID {
		v.clearMaterialLocked()
		// Do not delete a different instance's newer remembered record.
		v.warning = "App configuration was deleted or reset. Enter the master password again."
		return errCredentialVaultLocked
	}
	raw, err := readCredentialFile(v.path, vaultcrypto.MaxEnvelopeBytes)
	if err != nil || !bytes.Equal(raw, v.raw) {
		v.clearMaterialLocked()
		v.warning = "The encrypted vault changed outside this instance. Close and reopen the app before unlocking."
		return errCredentialVaultLocked
	}
	return nil
}
func (v *credentialVault) requireUnlocked() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.requireUnlockedLocked()
}
func (v *credentialVault) status() credentialVaultStatus {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.state == "unlocked" {
		_ = v.requireUnlockedLocked()
	}
	_, legacyErr := os.Lstat(v.legacyPath)
	state := v.state
	if state == "pending-authentication" {
		// Authenticated material is not usable until configuration identity is
		// bound under the cross-process guard. Never expose an unlocked gap.
		state = "locked"
	}
	return credentialVaultStatus{Available: true, State: state, Remembered: v.remembered, HasRecovery: v.envelope != nil && v.envelope.HasRecovery(), LegacyBackup: legacyErr == nil && v.envelope != nil, Warning: v.warning, Protection: "Master password (Argon2id / AES-256-GCM); optional Windows current-user remembered unlock"}
}

// Writes are fully prepared in a new private file. Existing targets are checked
// again before the atomic rename; no failure downgrades to the legacy format.
func writeCredentialAtomic(path string, raw, expected []byte, limit int) error {
	release, err := acquireCredentialGuard(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer release()
	return writeCredentialAtomicGuarded(path, raw, expected, limit)
}

// Caller holds the persistent OS transaction lock for this directory.
func writeCredentialAtomicGuarded(path string, raw, expected []byte, limit int) error {
	if len(raw) > limit {
		return errors.New("Credential storage would exceed its supported size limit.")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := createSecureCredentialTempFile(filepath.Dir(path))
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	readback, err := readCredentialFile(temp, limit)
	if err != nil || !bytes.Equal(readback, raw) {
		return errors.New("Credential candidate read-back verification failed; existing data was kept.")
	}
	current, readErr := readCredentialFile(path, limit)
	if expected == nil {
		if !errors.Is(readErr, os.ErrNotExist) {
			return errors.New("Credential storage already exists or cannot be safely inspected; it was kept.")
		}
	} else if readErr != nil || !bytes.Equal(current, expected) {
		return errors.New("Credential storage changed before saving; it was kept.")
	}
	return replaceSettingsFile(temp, path)
}
func removeCredentialFile(path string) error {
	before, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !before.Mode().IsRegular() || storageIsLink(before) {
		return errors.New("Refusing to remove linked or nonregular credential storage.")
	}
	return os.Remove(path)
}
func (v *credentialVault) saveLocked() error {
	if err := v.requireUnlockedLocked(); err != nil {
		return err
	}
	plain, err := marshalCredentialPayload(v.entries)
	if err != nil {
		return err
	}
	defer clear(plain)
	next, err := v.envelope.Reseal(plain, v.key)
	if err != nil {
		return errors.New("Credential vault encryption failed.")
	}
	raw, err := next.Marshal()
	if err != nil {
		return err
	}
	parsed, err := vaultcrypto.Parse(raw)
	if err != nil {
		return err
	}
	verified, err := parsed.OpenKey(v.key)
	if err != nil {
		return err
	}
	defer clear(verified)
	if !bytes.Equal(verified, plain) {
		return errors.New("Credential vault verification failed.")
	}
	if err := writeCredentialAtomic(v.path, raw, v.raw, vaultcrypto.MaxEnvelopeBytes); err != nil {
		return err
	}
	v.envelope = next
	v.raw = raw
	return nil
}

func (v *credentialVault) activate(ctx context.Context, password, payload []byte, recovery bool, confirmRecovery func(context.Context, string) (bool, error), expected []byte, beforeCommit func() error) error {
	entries, err := decodeCredentialPayload(payload, 2)
	if err != nil {
		return err
	}
	next, key, recoveryKey, err := vaultcrypto.Create(password, payload, recovery)
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		if !keep {
			clear(key)
		}
	}()
	verified, testKey, err := next.UnlockPassword(password)
	clear(testKey)
	if err != nil {
		return err
	}
	defer clear(verified)
	if !bytes.Equal(verified, payload) {
		return errors.New("New vault verification failed; existing data was kept.")
	}
	if recovery {
		if confirmRecovery == nil {
			return errors.New("Native recovery-key verification is unavailable.")
		}
		checked, recoveryDEK, err := next.UnlockRecovery(recoveryKey)
		clear(recoveryDEK)
		if err != nil {
			return err
		}
		same := bytes.Equal(checked, payload)
		clear(checked)
		if !same {
			return errors.New("Recovery verification failed.")
		}
		approved, err := confirmRecovery(ctx, recoveryKey)
		if err != nil {
			return err
		}
		if !approved {
			return errors.New("Recovery setup was canceled; existing data was kept.")
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	raw, err := next.Marshal()
	if err != nil {
		return err
	}
	parsed, parseErr := vaultcrypto.Parse(raw)
	if parseErr != nil {
		return parseErr
	}
	readback, parseErr := parsed.OpenKey(key)
	if parseErr != nil {
		return parseErr
	}
	matches := bytes.Equal(readback, payload)
	clear(readback)
	if !matches {
		return errors.New("Serialized vault verification failed; existing data was kept.")
	}
	release, err := acquireCredentialGuard(filepath.Dir(v.path))
	if err != nil {
		return err
	}
	defer release()
	if beforeCommit != nil {
		if err := beforeCommit(); err != nil {
			return err
		}
	}
	if err := writeCredentialAtomicGuarded(v.path, raw, expected, vaultcrypto.MaxEnvelopeBytes); err != nil {
		return err
	}
	v.clearMaterialLocked()
	v.envelope = next
	v.raw = raw
	v.key = key
	v.state = "pending-authentication"
	v.configID = ""
	v.warning = ""
	keep = true
	v.entries = entries
	// Old records cannot unlock this new identity/generation even if deletion fails.
	if err := removeCredentialFile(v.rememberPath); err != nil {
		v.warning = "Old remembered-unlock record could not be removed. It cannot unlock the new vault."
	}
	return nil
}

func (v *credentialVault) setup(ctx context.Context, password []byte, recovery bool, confirm func(context.Context, string) (bool, error)) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.state != "setup-required" && v.state != "legacy" {
		return errors.New("The vault is already configured. Unlock or recover it instead.")
	}
	entries := map[string]credentialRecord{}
	var legacyExpected []byte
	if v.state == "legacy" {
		raw, err := readCredentialFile(v.legacyPath, 2<<20)
		if err != nil {
			return err
		}
		legacyExpected = raw
		plain, err := v.unprotect(raw)
		if err != nil {
			return errors.New("The legacy Windows-protected vault could not be read; it was not changed.")
		}
		defer clear(plain)
		entries, err = decodeCredentialPayload(plain, credentialVaultVersion)
		if err != nil {
			return err
		}
	}
	plain, err := marshalCredentialPayload(entries)
	if err != nil {
		return err
	}
	defer clear(plain)
	return v.activate(ctx, password, plain, recovery, confirm, nil, func() error {
		current, err := readCredentialFile(v.legacyPath, 2<<20)
		if legacyExpected == nil && errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil || !bytes.Equal(current, legacyExpected) {
			return errors.New("Legacy credential storage changed during setup; all existing data was kept.")
		}
		return nil
	})
}
func (v *credentialVault) unlock(ctx context.Context, password []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	raw, err := readCredentialFile(v.path, vaultcrypto.MaxEnvelopeBytes)
	if err != nil {
		return errors.New("The v2 vault could not be read. No legacy fallback was attempted.")
	}
	envelope, err := vaultcrypto.Parse(raw)
	if err != nil {
		return errors.New("The encrypted vault is invalid. Existing data was kept.")
	}
	payload, key, err := envelope.UnlockPassword(password)
	if err != nil {
		return errors.New("The master password did not unlock this vault, or its encrypted data is damaged.")
	}
	defer clear(payload)
	entries, err := decodeCredentialPayload(payload, 2)
	if err != nil {
		clear(key)
		return err
	}
	if err := ctx.Err(); err != nil {
		clear(key)
		return err
	}
	v.clearMaterialLocked()
	v.envelope = envelope
	v.raw = raw
	v.key = key
	v.entries = entries
	v.state = "pending-authentication"
	v.warning = ""
	v.configID = ""
	// The subsequent guarded remember operation owns record replacement.
	// Deleting here could race a newer native unlock in another instance.
	return nil
}
func (v *credentialVault) rotate(ctx context.Context, current, newPassword []byte, recoveryKey string, recovery bool, confirm func(context.Context, string) (bool, error)) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	raw, err := readCredentialFile(v.path, vaultcrypto.MaxEnvelopeBytes)
	if err != nil {
		return err
	}
	envelope, err := vaultcrypto.Parse(raw)
	if err != nil {
		return errors.New("The encrypted vault is invalid. Existing data was kept.")
	}
	var payload, key []byte
	if recoveryKey != "" {
		payload, key, err = envelope.UnlockRecovery(recoveryKey)
	} else {
		payload, key, err = envelope.UnlockPassword(current)
	}
	clear(key)
	if err != nil {
		return errors.New("The supplied unlock secret did not authenticate this vault; existing data was kept.")
	}
	defer clear(payload)
	if _, err := decodeCredentialPayload(payload, 2); err != nil {
		return err
	}
	return v.activate(ctx, newPassword, payload, recovery, confirm, raw, nil)
}
func (v *credentialVault) remember(configID string, enabled bool) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	release, err := acquireCredentialGuard(filepath.Dir(v.path))
	if err != nil {
		return err
	}
	defer release()
	// Binding is to the authenticated envelope identity/generation, not mutable
	// payload bytes. Ordinary saves need only one-file atomic transactions.
	if (v.state != "pending-authentication" && v.state != "unlocked") || len(v.key) != vaultcrypto.KeyBytes {
		return errCredentialVaultLocked
	}
	if configID == "" || credentialConfigID(v.configPath) != configID {
		v.clearMaterialLocked()
		return errors.New("Unlock configuration changed before authentication completed. Unlock again.")
	}
	v.configID = configID
	v.state = "unlocked"
	if err := v.requireUnlockedLocked(); err != nil {
		return err
	}
	v.remembered = false
	if err := removeCredentialFile(v.rememberPath); err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	if configID == "" {
		return errors.New("Remembered unlock requires saved app configuration.")
	}
	id, generation := v.envelope.Identity()
	plain, err := json.Marshal(credentialRememberRecord{Version: 2, ConfigID: configID, VaultID: id, Generation: generation, Key: v.key})
	if err != nil {
		return err
	}
	defer clear(plain)
	encrypted, err := v.protect(plain)
	if err != nil {
		return errors.New("Windows could not protect the remembered unlock key.")
	}
	if err := writeCredentialAtomicGuarded(v.rememberPath, encrypted, nil, credentialRememberLimit); err != nil {
		return err
	}
	v.remembered = true
	return nil
}
func (v *credentialVault) lockAndForget() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.clearMaterialLocked()
	release, err := acquireCredentialGuard(filepath.Dir(v.path))
	if err != nil {
		v.warning = "Locked in this process, but other launches could not be invalidated. Retry Lock and forget after the other credential operation finishes."
		return err
	}
	defer release()
	configErr := invalidateCredentialConfigIdentity(v.configPath)
	removeErr := removeCredentialFile(v.rememberPath)
	if configErr != nil || removeErr != nil {
		v.warning = "Locked in this process, but configuration invalidation or remembered-key removal failed. Other open or future launches may remain unlocked; retry Lock and forget after fixing app-data file access."
		return errors.Join(configErr, removeErr)
	}
	v.warning = ""
	return nil
}
func (v *credentialVault) destroy() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.clearMaterialLocked()
	release, err := acquireCredentialGuard(filepath.Dir(v.path))
	if err != nil {
		return err
	}
	defer release()
	if err := removeCredentialFile(v.rememberPath); err != nil {
		return err
	}
	if err := removeCredentialFile(v.path); err != nil {
		return err
	}
	v.envelope = nil
	v.raw = nil
	v.configID = ""
	v.state = "setup-required"
	v.warning = ""
	if _, err := os.Lstat(v.legacyPath); err == nil {
		v.state = "legacy"
		v.warning = "The current vault was deleted. A legacy DPAPI-only copy remains and was not erased."
	}
	return nil
}
func (v *credentialVault) cleanupLegacy() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	release, err := acquireCredentialGuard(filepath.Dir(v.path))
	if err != nil {
		return err
	}
	defer release()
	if err := v.requireUnlockedLocked(); err != nil {
		return err
	}
	if err := removeCredentialFile(v.legacyPath); err != nil {
		return fmt.Errorf("Legacy copy was not removed: %w", err)
	}
	return nil
}

// The lock file is persistent and never unlinked: every cooperating process
// must lock the same filesystem object. It contains no secrets and is not copied
// as part of data migration. Lock contention fails visibly, never queues a prompt.
func acquireCredentialGuard(dir string) (func(), error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, credentialTransactionFilename)
	var file *os.File
	for attempt := 0; attempt < 2; attempt++ {
		before, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			file, err = createSecureCredentialFileAt(path)
			if errors.Is(err, os.ErrExist) {
				continue
			}
			if err != nil {
				return nil, err
			}
		} else {
			if err != nil {
				return nil, err
			}
			if !before.Mode().IsRegular() || storageIsLink(before) {
				return nil, errors.New("Credential transaction lock must be a regular private file.")
			}
			file, err = os.OpenFile(path, os.O_RDWR, 0)
			if err != nil {
				return nil, err
			}
			after, err := file.Stat()
			if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
				file.Close()
				return nil, errors.New("Credential transaction lock changed while opening it.")
			}
		}
		if err := lockStorageFile(file, true); err != nil {
			file.Close()
			return nil, errors.New("Another Network Toolkit instance is changing credentials. Finish that action and retry.")
		}
		return func() { _ = unlockStorageFile(file); _ = file.Close() }, nil
	}
	return nil, errors.New("Credential transaction lock changed while opening it.")
}

// Caller holds the transaction guard. Rotating this backend-owned, nonsecret
// identity invalidates every open instance and every stale remembered record.
func invalidateCredentialConfigIdentity(path string) error {
	if credentialConfigID(path) == "" {
		return nil
	}
	raw, err := readCredentialFile(path, 2<<20)
	if err != nil {
		return err
	}
	var state map[string]json.RawMessage
	if json.Unmarshal(raw, &state) != nil || state == nil {
		return errors.New("App settings cannot be safely invalidated.")
	}
	state[credentialConfigIdentityField], err = json.Marshal(randomHex(16))
	if err != nil {
		return err
	}
	raw, err = json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if len(raw) > 2<<20 {
		return errors.New("App settings exceed their size limit.")
	}
	return writeAtomic(path, raw)
}
