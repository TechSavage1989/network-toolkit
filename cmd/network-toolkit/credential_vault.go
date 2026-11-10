package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"networktoolkit/internal/vaultcrypto"
)

const credentialVaultFilename = "Network_Toolkit.credentials.vault"
const credentialVaultVersion = 1
const maxCredentialEntries = 200
const maxCredentialSecretBytes = 256 << 10

type credentialRecord struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	Username  string              `json:"username,omitempty"`
	Secret    string              `json:"secret"`
	Kind      string              `json:"kind"`
	UpdatedAt string              `json:"updatedAt"`
	Bindings  []credentialBinding `json:"bindings"`
}

type credentialMetadata struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	Username  string              `json:"username,omitempty"`
	Kind      string              `json:"kind"`
	UpdatedAt string              `json:"updatedAt"`
	Bindings  []credentialBinding `json:"bindings"`
}

type credentialVaultFile struct {
	Version int                `json:"version"`
	Entries []credentialRecord `json:"entries"`
}

type credentialVault struct {
	// The settings file could not be read when the vault was loaded.
	configUnread bool
	mu           sync.Mutex
	path         string
	legacyPath   string
	rememberPath string
	autoPath     string
	configPath   string
	configID     string
	entries      map[string]credentialRecord
	protect      func([]byte) ([]byte, error)
	unprotect    func([]byte) ([]byte, error)
	envelope     *vaultcrypto.Envelope
	raw          []byte
	key          []byte
	state        string
	remembered   bool
	warning      string
}

func validCredentialKind(value string) bool {
	switch value {
	case "password", "ssh-key", "key-passphrase", "generic":
		return true
	default:
		return false
	}
}

func sanitizeCredentialRecord(record credentialRecord, existingID string) (credentialRecord, error) {
	record.ID = strings.TrimSpace(record.ID)
	if record.ID == "" {
		record.ID = existingID
	}
	if record.ID == "" {
		record.ID = "credential-" + randomHex(12)
	}
	if len(record.ID) > 120 || strings.ContainsAny(record.ID, "/\\\x00") {
		return credentialRecord{}, errors.New("Credential ID is invalid.")
	}
	record.Name = strings.TrimSpace(record.Name)
	if record.Name == "" || strings.IndexFunc(record.Name, unicode.IsControl) >= 0 {
		return credentialRecord{}, errors.New("Enter a credential name.")
	}
	if len(record.Name) > 120 {
		return credentialRecord{}, errors.New("Credential names must be 120 characters or fewer.")
	}
	record.Username = strings.TrimSpace(record.Username)
	if len(record.Username) > 256 {
		return credentialRecord{}, errors.New("Usernames must be 256 characters or fewer.")
	}
	if record.Kind == "" {
		record.Kind = "password"
	}
	if !validCredentialKind(record.Kind) {
		return credentialRecord{}, errors.New("Credential type is not supported.")
	}
	if len([]byte(record.Secret)) > maxCredentialSecretBytes {
		return credentialRecord{}, fmt.Errorf("Credentials must be %d KiB or smaller.", maxCredentialSecretBytes>>10)
	}
	if record.Secret == "" {
		return credentialRecord{}, errors.New("Enter a password or passphrase.")
	}
	bindings, err := normalizeCredentialBindings(record.Bindings)
	if err != nil {
		return credentialRecord{}, err
	}
	record.Bindings = bindings
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return record, nil
}

func (v *credentialVault) list() []credentialMetadata {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.requireUnlockedLocked() != nil {
		return []credentialMetadata{}
	}
	result := make([]credentialMetadata, 0, len(v.entries))
	for _, item := range v.entries {
		result = append(result, credentialMetadataFor(item))
	}
	sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name) })
	return result
}

func (v *credentialVault) get(id string) (credentialRecord, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.requireUnlockedLocked() != nil {
		return credentialRecord{}, false
	}
	item, ok := v.entries[id]
	item.Bindings = append([]credentialBinding{}, item.Bindings...)
	return item, ok
}

func (v *credentialVault) put(record credentialRecord) (credentialMetadata, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	var saved credentialMetadata
	err := v.changeLocked(func() (func(), error) {
		_, exists := v.entries[record.ID]
		if !exists && len(v.entries) >= maxCredentialEntries {
			return nil, fmt.Errorf("The password manager is limited to %d saved credentials.", maxCredentialEntries)
		}
		clean, err := sanitizeCredentialRecord(record, record.ID)
		if err != nil {
			return nil, err
		}
		previous, hadPrevious := v.entries[clean.ID]
		v.entries[clean.ID] = clean
		saved = credentialMetadataFor(clean)
		return func() {
			if hadPrevious {
				v.entries[clean.ID] = previous
			} else {
				delete(v.entries, clean.ID)
			}
		}, nil
	})
	if err != nil {
		return credentialMetadata{}, err
	}
	return saved, nil
}

func (v *credentialVault) delete(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.changeLocked(func() (func(), error) {
		previous, ok := v.entries[id]
		if !ok {
			return nil, os.ErrNotExist
		}
		delete(v.entries, id)
		return func() { v.entries[id] = previous }, nil
	})
}

func (v *credentialVault) clear() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if err := v.requireUnlockedLocked(); err != nil {
		return err
	}
	if len(v.entries) == 0 {
		return nil
	}
	return v.changeLocked(func() (func(), error) {
		previous := v.entries
		v.entries = map[string]credentialRecord{}
		return func() { v.entries = previous }, nil
	})
}
