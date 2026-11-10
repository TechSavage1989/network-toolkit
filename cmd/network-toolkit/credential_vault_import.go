package main

// Before saved data was shared (docs/SHARED_APP_DATA.md), the standalone
// Terminal and the standalone Text Editor each had their own credential
// vault. Every app of a data folder now uses the folder's one vault. The
// previous vaults stay where they are, untouched, and the password manager
// offers to import their saved credentials:
//
//   - the previous vault is opened with its own protection only: its
//     remembered or automatic unlock for this account, or its master password
//     entered in the native dialog. Nothing is guessed and nothing about
//     either vault's protection is changed;
//   - a credential the shared vault does not have is copied with its id, so
//     the saved connections that name it keep working; one whose id is taken
//     by a different credential gets a new id, and the connections that came
//     from that workspace are pointed to it;
//   - the previous vault file is only read.
import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"

	"networktoolkit/internal/desktop"
)

type previousVault struct {
	Source   string `json:"source"`
	Label    string `json:"label"`
	Imported bool   `json:"imported"`
}

var errCredentialNoChange = errors.New("no credential change")

func previousVaultFile(dir string) bool {
	for _, name := range []string{credentialVaultV2Filename, credentialVaultFilename} {
		if info, err := os.Lstat(filepath.Join(dir, name)); err == nil && info.Mode().IsRegular() && !storageIsLink(info) {
			return true
		}
	}
	return false
}

// previousVaults lists the vaults earlier versions kept for standalone
// windows of this data folder.
func (a *application) previousVaults() []previousVault {
	root := a.sharedRoot()
	out := []previousVault{}
	var marker *sharedMarker
	for _, workspace := range legacyWorkspaces {
		if !previousVaultFile(filepath.Join(root, workspace.Directory)) {
			continue
		}
		if marker == nil {
			if marker, _ = readSharedMarker(root); marker == nil {
				marker = &sharedMarker{}
			}
		}
		_, imported := marker.Imports[workspace.ID]
		out = append(out, previousVault{Source: workspace.ID, Label: workspace.Label, Imported: imported})
	}
	return out
}

// previousVaultHint tells the user where a saved credential that the shared
// vault does not have may still be.
func (a *application) previousVaultHint() string {
	for _, vault := range a.previousVaults() {
		if !vault.Imported {
			return "This saved credential is not in the shared password manager. It may still be in the previous " + vault.Label + " vault: open Password manager and choose Import saved credentials from the previous " + vault.Label + " vault."
		}
	}
	return ""
}

func sameCredentialContent(a, b credentialRecord) bool {
	if a.Name != b.Name || a.Username != b.Username || a.Secret != b.Secret || a.Kind != b.Kind || len(a.Bindings) != len(b.Bindings) {
		return false
	}
	for i := range a.Bindings {
		if a.Bindings[i] != b.Bindings[i] {
			return false
		}
	}
	return true
}

type credentialImportResult struct {
	Imported int `json:"imported"`
	Present  int `json:"alreadyPresent"`
	Skipped  int `json:"skipped"`
	// The names (never the secrets) of the credentials that did not fit.
	SkippedNames []string `json:"skippedNames,omitempty"`
	// Old id to new id, for credentials whose id was taken.
	Renamed map[string]string `json:"-"`
}

// importRecords copies records into the vault in one saved change. A record
// the vault already holds (same id or not) is left alone.
func (v *credentialVault) importRecords(records []credentialRecord) (credentialImportResult, error) {
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	var result credentialImportResult
	v.mu.Lock()
	defer v.mu.Unlock()
	err := v.changeLocked(func() (func(), error) {
		result = credentialImportResult{Renamed: map[string]string{}}
		previous := make(map[string]credentialRecord, len(v.entries))
		for id, record := range v.entries {
			previous[id] = record
		}
		for _, record := range records {
			if existing, taken := v.entries[record.ID]; taken {
				if sameCredentialContent(existing, record) {
					result.Present++
					continue
				}
				// A different credential has this id. Was this one copied before?
				copied := ""
				for id, other := range v.entries {
					if sameCredentialContent(other, record) {
						copied = id
						break
					}
				}
				if copied != "" {
					result.Present++
					result.Renamed[record.ID] = copied
					continue
				}
				if len(v.entries) >= maxCredentialEntries {
					result.Skipped++
					result.SkippedNames = append(result.SkippedNames, record.Name)
					continue
				}
				id := "credential-" + randomHex(12)
				result.Renamed[record.ID] = id
				record.ID = id
			} else if len(v.entries) >= maxCredentialEntries {
				result.Skipped++
				result.SkippedNames = append(result.SkippedNames, record.Name)
				continue
			}
			v.entries[record.ID] = record
			result.Imported++
		}
		if result.Imported == 0 {
			v.entries = previous
			return nil, errCredentialNoChange
		}
		return func() { v.entries = previous }, nil
	})
	if errors.Is(err, errCredentialNoChange) {
		err = nil
	}
	sort.Strings(result.SkippedNames)
	return result, err
}

// previousVaultRecords opens a previous vault with its own protection and
// returns copies of its records. The vault file is only read.
func (a *application) previousVaultRecords(ctx context.Context, workspace legacyWorkspace, dir string) ([]credentialRecord, error) {
	old, err := newCredentialVaultWithConfig(filepath.Join(dir, appSettingsFilename), a.credentials.protect, a.credentials.unprotect)
	if err != nil {
		return nil, errors.New("The previous " + workspace.Label + " vault could not be opened. It was not changed.")
	}
	defer old.close()
	old.mu.Lock()
	state := old.state
	old.mu.Unlock()
	switch state {
	case "setup-required":
		return nil, nil
	case "legacy":
		// The first vault format: protected by this Windows account only.
		raw, err := readCredentialFile(old.legacyPath, 2<<20)
		if err != nil {
			return nil, err
		}
		plain, err := old.unprotect(raw)
		if err != nil {
			return nil, errors.New("Windows could not open the previous " + workspace.Label + " vault for this account. It was not changed.")
		}
		defer clear(plain)
		entries, err := decodeCredentialPayload(plain, credentialVaultVersion)
		if err != nil {
			return nil, err
		}
		return credentialRecordList(entries), nil
	case "unlocked":
		// Its remembered unlock for this account is still valid.
	default:
		if old.autoModeLocked() {
			// It has no master password: its key is protected by this account.
			password, err := old.readAutoPassword()
			if err != nil {
				return nil, err
			}
			err = old.unlock(ctx, password)
			clear(password)
			if err != nil {
				return nil, err
			}
		} else {
			if a.promptVaultSecret == nil {
				return nil, errors.New("Native secret entry is unavailable.")
			}
			response, err := a.promptVaultSecret(ctx, desktop.VaultSecretRequest{Mode: desktop.VaultSecretImport, Label: workspace.Label})
			defer clear(response.Password)
			defer clear(response.CurrentPassword)
			defer clear(response.RecoveryKey)
			if err != nil {
				return nil, errors.New("The import was canceled; nothing was changed.")
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if err := old.unlock(ctx, response.Password); err != nil {
				return nil, errors.New("That master password did not open the previous " + workspace.Label + " vault. Nothing was imported and neither vault was changed.")
			}
		}
	}
	old.mu.Lock()
	defer old.mu.Unlock()
	return credentialRecordList(old.entries), nil
}

func credentialRecordList(entries map[string]credentialRecord) []credentialRecord {
	out := make([]credentialRecord, 0, len(entries))
	for _, record := range entries {
		record.Bindings = append([]credentialBinding{}, record.Bindings...)
		out = append(out, record)
	}
	return out
}

// importPreviousVault copies the saved credentials of a previous standalone
// vault into the shared vault. Called with credentialChanges held.
func (a *application) importPreviousVault(ctx context.Context, source string) (credentialImportResult, error) {
	var workspace legacyWorkspace
	for _, candidate := range legacyWorkspaces {
		if candidate.ID == source {
			workspace = candidate
		}
	}
	root := a.sharedRoot()
	dir := filepath.Join(root, workspace.Directory)
	if workspace.ID == "" || !previousVaultFile(dir) {
		return credentialImportResult{}, errors.New("There is no previous vault to import from.")
	}
	if err := a.credentials.requireUnlocked(); err != nil {
		return credentialImportResult{}, err
	}
	records, err := a.previousVaultRecords(ctx, workspace, dir)
	if err != nil {
		return credentialImportResult{}, err
	}
	result, err := a.credentials.importRecords(records)
	if err != nil {
		return result, err
	}
	// Point the saved connections that came from that workspace to
	// credentials that had to be given a new id, then record the import.
	if len(result.Renamed) > 0 {
		marker, err := readSharedMarker(root)
		if err != nil {
			return result, err
		}
		references := map[string]string{} // "key\x00id" to the new credential id
		for i := range marker.Credentials {
			reference := &marker.Credentials[i]
			if id, ok := result.Renamed[reference.CredentialID]; ok && reference.Source == workspace.ID {
				references[reference.Key+"\x00"+reference.ID] = id
			}
		}
		if len(references) > 0 {
			a.stateMu.Lock()
			err = a.updateSharedLocked(func(shared map[string]any) error {
				for name, value := range shared {
					list, _ := value.([]any)
					for _, entry := range list {
						row, ok := entry.(map[string]any)
						if !ok {
							continue
						}
						id, _ := row["id"].(string)
						current, _ := row["credentialId"].(string)
						if next, ok := references[name+"\x00"+id]; ok {
							if _, renamed := result.Renamed[current]; renamed {
								row["credentialId"] = next
							}
						}
					}
				}
				return nil
			})
			a.stateMu.Unlock()
			if err != nil {
				return result, err
			}
		}
	}
	release, err := acquireSharedDataLock(root)
	if err != nil {
		return result, err
	}
	defer release()
	marker, err := readSharedMarker(root)
	if err != nil {
		return result, err
	}
	for i := range marker.Credentials {
		reference := &marker.Credentials[i]
		if id, ok := result.Renamed[reference.CredentialID]; ok && reference.Source == workspace.ID {
			reference.CredentialID = id
		}
	}
	if result.Skipped == 0 {
		// Everything is in the shared vault. While credentials are left out
		// (the vault is full) the import stays on offer.
		if marker.Imports == nil {
			marker.Imports = map[string]sharedVaultImport{}
		}
		marker.Imports[workspace.ID] = sharedVaultImport{At: time.Now().UTC().Format(time.RFC3339), Imported: result.Imported}
	}
	return result, writeSharedMarker(root, marker)
}
