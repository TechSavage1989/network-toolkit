package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"networktoolkit/internal/sharedfile"
	"networktoolkit/internal/vaultcrypto"
)

const credentialVaultV2Filename = "Network_Toolkit.credentials.v2.vault"
const credentialRememberFilename = "Network_Toolkit.credentials.remember"
const credentialTransactionFilename = "Network_Toolkit.credentials.lock"
const credentialConfigIdentityField = "_credentialVaultConfigID"
const credentialRememberLimit = 64 << 10

// Optional "no master password" mode: the vault keeps its normal Argon2id /
// AES-256-GCM envelope, but its password is a random 256-bit value stored only
// in this CurrentUser-DPAPI-protected file and used to unlock automatically.
const credentialAutoFilename = "Network_Toolkit.credentials.auto"
const credentialAutoLimit = 16 << 10

type credentialAutoRecord struct {
	Version  int    `json:"version"`
	Password []byte `json:"password"`
}

var errCredentialVaultLocked = errors.New("Unlock the credential vault in the password manager before using saved credentials.")

// The vault file exists but could not be read at this moment.
var errCredentialVaultUnreadable = errors.New("The credential vault could not be read just now, so the saved credential was not used. Try again.")

// Another app of this data folder saved the vault between this app's read
// and its write. The change is redone on the newer vault, never forced.
var errCredentialStorageChanged = errors.New("Credential storage changed before saving; it was kept.")

type credentialVaultStatus struct {
	Available    bool   `json:"available"`
	State        string `json:"state"`
	Remembered   bool   `json:"remembered"`
	HasRecovery  bool   `json:"hasRecovery"`
	LegacyBackup bool   `json:"legacyBackup"`
	Warning      string `json:"warning,omitempty"`
	Protection   string `json:"protection"`
	Mode         string `json:"mode"` // "password" or "windows" (no master password)
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
	v := &credentialVault{path: filepath.Join(root, credentialVaultV2Filename), legacyPath: filepath.Join(root, credentialVaultFilename), rememberPath: filepath.Join(root, credentialRememberFilename), autoPath: filepath.Join(root, credentialAutoFilename), configPath: settingsPath, entries: map[string]credentialRecord{}, protect: protect, unprotect: unprotect, state: "setup-required"}
	err := v.loadV2()
	if err != nil {
		v.state = "locked"
		v.warning = "The credential vault could not be opened. Existing encrypted data was kept; no legacy fallback was attempted."
	}
	return v, err
}

// The file that was opened is not the file that was looked at a moment
// before: another app replaced it in between.
var errCredentialFileChanged = errors.New("Credential storage changed while opening it.")

// Bounded and identity-checked reads reject links, devices and replaced files.
// The apps of one data folder replace these files while others read them, so
// a read that meets a replace in progress is made again: every attempt is
// checked in full, and what is returned is one complete file.
func readCredentialFile(path string, limit int) ([]byte, error) {
	var raw []byte
	err := sharedfile.Retry(func() error {
		var err error
		raw, err = readCredentialFileOnce(path, limit)
		return err
	}, func(err error) bool { return errors.Is(err, errCredentialFileChanged) || sharedfile.Transient(err) })
	return raw, err
}

// readCredentialFault lets a test make the read of one file fail the way it
// does when another program holds the file. It is nil outside tests.
var readCredentialFault func(path string) error

func readCredentialFileOnce(path string, limit int) ([]byte, error) {
	if readCredentialFault != nil {
		if err := readCredentialFault(path); err != nil {
			return nil, err
		}
	}
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
	f, err := sharedfile.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	after, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !after.Mode().IsRegular() || storageIsLink(after) {
		return nil, errors.New("Credential storage must be a regular file, not a link or device.")
	}
	if !os.SameFile(before, after) {
		return nil, errCredentialFileChanged
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

// credentialConfigID returns the identity in the settings file at path, or ""
// when there is none or the file cannot be read. Code that locks the vault
// when the identity is gone uses credentialConfigIdentity, which tells the
// two apart.
func credentialConfigID(path string) string {
	id, _ := credentialConfigIdentity(path)
	return id
}

// The settings file exists but its identity could not be determined now.
var errCredentialConfigUnreadable = errors.New("App settings could not be read just now, so the saved credential was not used. Try again.")

// credentialConfigIdentity reads the identity the vault is bound to. It
// returns "" with no error only when the settings file is really gone or
// holds no valid identity: the configuration was deleted or reset. A file
// that could not be read, or that is not a settings object at this moment,
// is an error. The caller then refuses the operation and locks nothing: the
// app next door replaces this file with every save, and a reader that met
// such a moment used to take it for a reset and locked the vault.
func credentialConfigIdentity(path string) (string, error) {
	raw, err := readCredentialFile(path, 2<<20)
	if errors.Is(err, os.ErrNotExist) {
		// Gone for good only if it stays gone: a file being replaced must
		// never read as deleted.
		for attempt := 0; attempt < 2 && errors.Is(err, os.ErrNotExist); attempt++ {
			time.Sleep(5 * time.Millisecond)
			raw, err = readCredentialFile(path, 2<<20)
		}
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
	}
	if err != nil {
		return "", fmt.Errorf("%w (%v)", errCredentialConfigUnreadable, err)
	}
	var state map[string]json.RawMessage
	if json.Unmarshal(raw, &state) != nil || state == nil {
		return "", errCredentialConfigUnreadable
	}
	return credentialConfigIDOf(state), nil
}

// credentialConfigIDOf returns the valid identity among the members of a
// settings file that has been read, or "".
func credentialConfigIDOf(members map[string]json.RawMessage) string {
	var id string
	if json.Unmarshal(members[credentialConfigIdentityField], &id) != nil || len(id) != 32 {
		return ""
	}
	decoded, err := hex.DecodeString(id)
	if err != nil || len(decoded) != 16 {
		return ""
	}
	return id
}

// configCurrentLocked checks that the settings file still carries the
// identity this vault was unlocked for. Only a settings file that was read
// and shows another identity (or none, or is gone) locks the vault.
func (v *credentialVault) configCurrentLocked() error {
	if v.configID != "" {
		id, err := credentialConfigIdentity(v.configPath)
		if err != nil {
			return err
		}
		if id == v.configID {
			return nil
		}
	}
	v.clearMaterialLocked()
	// Do not delete a different instance's newer remembered record.
	v.warning = "App configuration was deleted or reset. Enter the master password again."
	return errCredentialVaultLocked
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
	v.configUnread = false
	if v.configID, err = credentialConfigIdentity(v.configPath); err != nil {
		// The settings file could not be read at this moment. A remembered
		// unlock is neither used nor called expired; status() looks again.
		v.configID, v.configUnread = "", true
		v.warning = "App settings could not be read just now. Enter the master password, or try again in a moment."
		return nil
	}
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
	if err := v.configCurrentLocked(); err != nil {
		return err
	}
	raw, err := readCredentialFile(v.path, vaultcrypto.MaxEnvelopeBytes)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		// The vault file could not be read at this moment. That says nothing
		// about the vault: refuse this use and keep everything as it is.
		return fmt.Errorf("%w (%v)", errCredentialVaultUnreadable, err)
	}
	if err != nil || !bytes.Equal(raw, v.raw) {
		// The main window, the Terminal and the Text Editor of one data folder
		// use this one vault. A vault another of them saved is taken over when
		// the key this app unlocked with still opens it (same vault, same
		// master password); otherwise the ordinary unlock rules apply again.
		if err == nil && v.adoptExternalLocked(raw) {
			return nil
		}
		v.reloadExternalLocked()
		if v.state == "unlocked" {
			return nil
		}
		return errCredentialVaultLocked
	}
	return nil
}

// adoptExternalLocked accepts a vault file written by another app only if it
// is the same vault generation and authenticates with the key already held.
// Authenticated decryption with that key is what an unlock proves, so this
// grants nothing a fresh unlock with the same password would not.
func (v *credentialVault) adoptExternalLocked(raw []byte) bool {
	if v.envelope == nil || len(v.key) != vaultcrypto.KeyBytes {
		return false
	}
	envelope, err := vaultcrypto.Parse(raw)
	if err != nil {
		return false
	}
	id, generation := envelope.Identity()
	heldID, heldGeneration := v.envelope.Identity()
	if id != heldID || generation != heldGeneration {
		return false
	}
	payload, err := envelope.OpenKey(v.key)
	if err != nil {
		return false
	}
	defer clear(payload)
	entries, err := decodeCredentialPayload(payload, 2)
	if err != nil {
		return false
	}
	clear(v.entries)
	v.envelope, v.raw, v.entries = envelope, raw, entries
	return true
}

// reloadExternalLocked starts over from the files on disk, exactly as a newly
// started app would: a vault whose master password another app changed, or
// that was deleted or recreated, is locked here unless a valid remembered
// unlock for the new vault exists.
func (v *credentialVault) reloadExternalLocked() {
	v.clearMaterialLocked()
	v.envelope, v.raw, v.configID = nil, nil, ""
	v.state, v.warning = "setup-required", ""
	if err := v.loadV2(); err != nil {
		v.state = "locked"
		v.warning = "The credential vault could not be opened. Existing encrypted data was kept; no legacy fallback was attempted."
		return
	}
	if v.state == "locked" && v.warning == "" {
		v.warning = "The credential vault was changed in another Network Toolkit window (for example its master password). Unlock it again."
	}
}

// requireCurrentLocked is the check before a write: the vault must be
// unlocked and the file must still be the one this app holds. A newer file
// is reported, not adopted, so the pending change is not silently dropped.
func (v *credentialVault) requireCurrentLocked() error {
	if v.state != "unlocked" || len(v.key) != vaultcrypto.KeyBytes {
		return errCredentialVaultLocked
	}
	if err := v.configCurrentLocked(); err != nil {
		return err
	}
	raw, err := readCredentialFile(v.path, vaultcrypto.MaxEnvelopeBytes)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w (%v)", errCredentialVaultUnreadable, err)
	}
	if err != nil {
		v.reloadExternalLocked()
		return errCredentialVaultLocked
	}
	if !bytes.Equal(raw, v.raw) {
		return errCredentialStorageChanged
	}
	return nil
}

// changeLocked applies one change to the entries and saves it. If another
// app saved the vault meanwhile, the change is undone, the newer vault is
// loaded and the change is applied to it again: neither app's change is lost.
func (v *credentialVault) changeLocked(apply func() (undo func(), err error)) error {
	for attempt := 0; ; attempt++ {
		if err := v.requireUnlockedLocked(); err != nil {
			return err
		}
		undo, err := apply()
		if err != nil {
			return err
		}
		err = v.saveLocked()
		if err == nil {
			return nil
		}
		if v.state == "unlocked" && undo != nil {
			undo()
		}
		if !errors.Is(err, errCredentialStorageChanged) || attempt >= 3 {
			return err
		}
	}
}

// mark changes whenever the vault file or its lock state changes, so a page
// knows when to read the list of saved credentials again. It reveals nothing
// about the contents.
func (v *credentialVault) mark() string {
	state := v.status().State
	v.mu.Lock()
	defer v.mu.Unlock()
	digest := sha256.Sum256(append([]byte(state+"\x00"), v.raw...))
	return hex.EncodeToString(digest[:8])
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
	} else if v.state != "pending-authentication" {
		// Another app of this data folder may have set the vault up, deleted
		// it or changed it since this app last looked: start over from the
		// files on disk, as a newly started app would.
		raw, err := readCredentialFile(v.path, vaultcrypto.MaxEnvelopeBytes)
		if missing := errors.Is(err, os.ErrNotExist); missing != (v.raw == nil) || err == nil && !bytes.Equal(raw, v.raw) || v.configUnread {
			v.reloadExternalLocked()
		}
	}
	_, legacyErr := os.Lstat(v.legacyPath)
	state := v.state
	if state == "pending-authentication" {
		// Authenticated material is not usable until configuration identity is
		// bound under the cross-process guard. Never expose an unlocked gap.
		state = "locked"
	}
	mode, protection := "password", "Master password (Argon2id / AES-256-GCM); optional Windows current-user remembered unlock"
	if v.envelope != nil && v.autoModeLocked() {
		mode, protection = "windows", "No master password: random vault key (Argon2id / AES-256-GCM) protected by this Windows account (DPAPI) and unlocked automatically"
	}
	return credentialVaultStatus{Available: true, State: state, Remembered: v.remembered, HasRecovery: v.envelope != nil && v.envelope.HasRecovery(), LegacyBackup: legacyErr == nil && v.envelope != nil, Warning: v.warning, Protection: protection, Mode: mode}
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
		return errCredentialStorageChanged
	}
	// Another app of the data folder may be reading the vault right now.
	return sharedfile.Replace(temp, path)
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
	if err := v.requireCurrentLocked(); err != nil {
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
	// Another app may be saving its settings at this moment (the same lock);
	// wait briefly for it instead of failing the user's change.
	release, err := acquireCredentialGuardWait(filepath.Dir(v.path), 1500*time.Millisecond)
	if err != nil {
		return err
	}
	defer release()
	if err := writeCredentialAtomicGuarded(v.path, raw, v.raw, vaultcrypto.MaxEnvelopeBytes); err != nil {
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
	return v.setupWith(ctx, password, recovery, confirm, nil)
}

// setupWithoutPassword creates the vault under a random password kept only in
// the DPAPI-protected automatic-unlock file. That file is written under the
// same guard immediately before the vault, and restored if the vault write fails.
func (v *credentialVault) setupWithoutPassword(ctx context.Context) error {
	password, err := newCredentialAutoPassword()
	if err != nil {
		return err
	}
	defer clear(password)
	var restore func()
	err = v.setupWith(ctx, password, false, nil, func() error {
		r, e := v.writeAutoGuarded(password)
		restore = r
		return e
	})
	if err != nil && restore != nil {
		restore()
	}
	return err
}

func (v *credentialVault) setupWith(ctx context.Context, password []byte, recovery bool, confirm func(context.Context, string) (bool, error), extra func() error) error {
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
			if extra != nil {
				return extra()
			}
			return nil
		}
		if err != nil || !bytes.Equal(current, legacyExpected) {
			return errors.New("Legacy credential storage changed during setup; all existing data was kept.")
		}
		if extra != nil {
			return extra()
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
	return v.rotateWith(ctx, current, newPassword, recoveryKey, recovery, confirm, nil)
}

// removeMasterPassword re-encrypts the vault under a new random password kept
// only in the DPAPI-protected automatic-unlock file (written immediately
// before the vault under the guard, restored if the vault write fails).
func (v *credentialVault) removeMasterPassword(ctx context.Context, current []byte) error {
	password, err := newCredentialAutoPassword()
	if err != nil {
		return err
	}
	defer clear(password)
	var restore func()
	err = v.rotateWith(ctx, current, password, "", false, nil, func() error {
		r, e := v.writeAutoGuarded(password)
		restore = r
		return e
	})
	if err != nil && restore != nil {
		restore()
	}
	return err
}

// addMasterPassword re-encrypts a no-master-password vault under the user's
// new master password, then removes the automatic-unlock file.
func (v *credentialVault) addMasterPassword(ctx context.Context, newPassword []byte, recovery bool, confirm func(context.Context, string) (bool, error)) error {
	current, err := v.readAutoPassword()
	if err != nil {
		return err
	}
	defer clear(current)
	if err := v.rotateWith(ctx, current, newPassword, "", recovery, confirm, nil); err != nil {
		return err
	}
	return v.removeAuto()
}

func (v *credentialVault) rotateWith(ctx context.Context, current, newPassword []byte, recoveryKey string, recovery bool, confirm func(context.Context, string) (bool, error), beforeCommit func() error) error {
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
	return v.activate(ctx, newPassword, payload, recovery, confirm, raw, beforeCommit)
}

func newCredentialAutoPassword() ([]byte, error) {
	random := make([]byte, 32)
	defer clear(random)
	if _, err := rand.Read(random); err != nil {
		return nil, errors.New("A random vault key could not be generated.")
	}
	return base64.RawURLEncoding.AppendEncode(nil, random), nil
}

func (v *credentialVault) autoModeLocked() bool {
	info, err := os.Lstat(v.autoPath)
	return err == nil && info.Mode().IsRegular() && !storageIsLink(info)
}

// readAutoPassword returns the caller-owned random vault password.
func (v *credentialVault) readAutoPassword() ([]byte, error) {
	raw, err := readCredentialFile(v.autoPath, credentialAutoLimit)
	if err != nil {
		return nil, errors.New("The automatic-unlock key is missing or unreadable. Use the offline recovery key if you created one.")
	}
	plain, err := v.unprotect(raw)
	if err != nil {
		return nil, errors.New("Windows could not open the automatic-unlock key for this account.")
	}
	defer clear(plain)
	var record credentialAutoRecord
	if json.Unmarshal(plain, &record) != nil || record.Version != 1 || len(record.Password) < 32 || len(record.Password) > 1024 {
		clear(record.Password)
		return nil, errors.New("The automatic-unlock key is invalid.")
	}
	return record.Password, nil
}

// Caller holds the credential guard. The returned restore puts back whatever
// was there before (or removes the new file) if the following vault write fails.
func (v *credentialVault) writeAutoGuarded(password []byte) (func(), error) {
	plain, err := json.Marshal(credentialAutoRecord{Version: 1, Password: password})
	if err != nil {
		return nil, err
	}
	defer clear(plain)
	encrypted, err := v.protect(plain)
	if err != nil {
		return nil, errors.New("Windows could not protect the automatic-unlock key.")
	}
	previous, readErr := readCredentialFile(v.autoPath, credentialAutoLimit)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return nil, readErr
	}
	if readErr != nil {
		previous = nil
	}
	if err := writeCredentialAtomicGuarded(v.autoPath, encrypted, previous, credentialAutoLimit); err != nil {
		return nil, err
	}
	return func() {
		if previous == nil {
			_ = removeCredentialFile(v.autoPath)
		} else {
			_ = writeCredentialAtomic(v.autoPath, previous, encrypted, credentialAutoLimit)
		}
	}, nil
}

func (v *credentialVault) removeAuto() error {
	release, err := acquireCredentialGuard(filepath.Dir(v.autoPath))
	if err != nil {
		return err
	}
	defer release()
	if err := removeCredentialFile(v.autoPath); err != nil {
		v.mu.Lock()
		v.warning = "The master password was set, but the old automatic-unlock key could not be removed. It no longer unlocks this vault."
		v.mu.Unlock()
		return nil
	}
	return nil
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
	current, err := credentialConfigIdentity(v.configPath)
	if err != nil {
		return err
	}
	if configID == "" || current != configID {
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
	if err := removeCredentialFile(v.autoPath); err != nil {
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
				if sharedfile.HeldByAnother(err) {
					// The app that created the lock file a moment ago holds it
					// without sharing (Windows) until its change is done: that
					// is the same as finding the file locked.
					return nil, errCredentialGuardBusy
				}
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
			return nil, errCredentialGuardBusy
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
	// The settings file of the data folder: read and written like every
	// other change to it, so nothing else in it is lost.
	settings, err := readSharedSettings(path)
	if err != nil || !settings.exists {
		return errors.New("App settings cannot be safely invalidated.")
	}
	settings.members[credentialConfigIdentityField], err = json.Marshal(randomHex(16))
	if err != nil {
		return err
	}
	raw, err := settings.marshal()
	if err != nil {
		return errors.New("App settings exceed their size limit.")
	}
	_, err = writeSharedSettings(path, raw)
	return err
}
