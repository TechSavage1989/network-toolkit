package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const credentialVaultFilename = "Network_Toolkit.credentials.vault"
const credentialVaultVersion = 1
const maxCredentialEntries = 200
const maxCredentialSecretBytes = 256 << 10

type credentialRecord struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username,omitempty"`
	Secret    string `json:"secret"`
	Kind      string `json:"kind"`
	UpdatedAt string `json:"updatedAt"`
}

type credentialMetadata struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username,omitempty"`
	Kind      string `json:"kind"`
	UpdatedAt string `json:"updatedAt"`
}

type credentialVaultFile struct {
	Version int                `json:"version"`
	Entries []credentialRecord `json:"entries"`
}

type credentialVault struct {
	mu      sync.Mutex
	path    string
	entries map[string]credentialRecord
}

func newCredentialVault(root string) (*credentialVault, error) {
	vault := &credentialVault{path: filepath.Join(root, credentialVaultFilename), entries: map[string]credentialRecord{}}
	if err := vault.load(); err != nil {
		return nil, err
	}
	return vault, nil
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
	if record.Name == "" {
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
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return record, nil
}

func (v *credentialVault) load() error {
	raw, err := os.ReadFile(v.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read credential vault: %w", err)
	}
	if len(raw) > 2<<20 {
		return errors.New("Credential vault is larger than the supported 2 MiB limit.")
	}
	plain, err := unprotectCredentialBlob(raw)
	if err != nil {
		return fmt.Errorf("unlock credential vault: %w", err)
	}
	var file credentialVaultFile
	if err := json.Unmarshal(plain, &file); err != nil || file.Version != credentialVaultVersion {
		return errors.New("Credential vault is unreadable or uses an unsupported format.")
	}
	for _, item := range file.Entries {
		if len(v.entries) >= maxCredentialEntries || item.ID == "" || item.Secret == "" || !validCredentialKind(item.Kind) {
			continue
		}
		v.entries[item.ID] = item
	}
	return nil
}

func (v *credentialVault) saveLocked() error {
	entries := make([]credentialRecord, 0, len(v.entries))
	for _, item := range v.entries {
		entries = append(entries, item)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	plain, err := json.Marshal(credentialVaultFile{Version: credentialVaultVersion, Entries: entries})
	if err != nil {
		return err
	}
	encrypted, err := protectCredentialBlob(plain)
	if err != nil {
		return fmt.Errorf("protect credential vault: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(v.path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(v.path), ".network-toolkit-credential-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(encrypted)
	}
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return replaceSettingsFile(temp, v.path)
}

func (v *credentialVault) list() []credentialMetadata {
	v.mu.Lock()
	defer v.mu.Unlock()
	result := make([]credentialMetadata, 0, len(v.entries))
	for _, item := range v.entries {
		result = append(result, credentialMetadata{ID: item.ID, Name: item.Name, Username: item.Username, Kind: item.Kind, UpdatedAt: item.UpdatedAt})
	}
	sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name) })
	return result
}

func (v *credentialVault) get(id string) (credentialRecord, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	item, ok := v.entries[id]
	return item, ok
}

func (v *credentialVault) put(record credentialRecord) (credentialMetadata, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if record.ID == "" && len(v.entries) >= maxCredentialEntries {
		return credentialMetadata{}, fmt.Errorf("The password manager is limited to %d saved credentials.", maxCredentialEntries)
	}
	currentID := record.ID
	clean, err := sanitizeCredentialRecord(record, currentID)
	if err != nil {
		return credentialMetadata{}, err
	}
	previous, hadPrevious := v.entries[clean.ID]
	v.entries[clean.ID] = clean
	if err := v.saveLocked(); err != nil {
		if hadPrevious {
			v.entries[clean.ID] = previous
		} else {
			delete(v.entries, clean.ID)
		}
		return credentialMetadata{}, err
	}
	return credentialMetadata{ID: clean.ID, Name: clean.Name, Username: clean.Username, Kind: clean.Kind, UpdatedAt: clean.UpdatedAt}, nil
}

func (v *credentialVault) delete(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	previous, ok := v.entries[id]
	if !ok {
		return os.ErrNotExist
	}
	delete(v.entries, id)
	if err := v.saveLocked(); err != nil {
		v.entries[id] = previous
		return err
	}
	return nil
}

func (v *credentialVault) clear() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if len(v.entries) == 0 {
		return nil
	}
	previous := v.entries
	v.entries = map[string]credentialRecord{}
	if err := v.saveLocked(); err != nil {
		v.entries = previous
		return err
	}
	return nil
}
