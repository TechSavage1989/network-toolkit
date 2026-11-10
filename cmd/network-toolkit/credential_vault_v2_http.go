package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"networktoolkit/internal/desktop"
)

func (a *application) handleCredentialVault(w http.ResponseWriter, r *http.Request) {
	if a.credentials == nil {
		jsonError(w, http.StatusServiceUnavailable, "The native credential vault is unavailable in this build.")
		return
	}
	if r.Method == http.MethodGet {
		a.autoUnlockCredentials(r.Context())
		jsonResponse(w, http.StatusOK, a.credentials.status())
		return
	}
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	var request struct {
		Action string `json:"action"`
		// For import-previous: which standalone window's previous vault.
		Source string `json:"source,omitempty"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		jsonError(w, 400, "Supply only a vault action. Passwords and recovery keys must be entered in the native dialog.")
		return
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		jsonError(w, 400, "Supply exactly one vault action.")
		return
	}
	// Human native input may outlast the ordinary HTTP write deadline. No secret
	// crosses this response and cancellation is still honored by native dialogs.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	if !a.credentialChanges.TryLock() {
		jsonError(w, http.StatusConflict, "Another credential change or native vault dialog is already in progress.")
		return
	}
	defer a.credentialChanges.Unlock()
	if err := r.Context().Err(); err != nil {
		jsonError(w, 408, "Vault action was canceled.")
		return
	}
	var err error
	switch request.Action {
	case "setup", "unlock", "change-password", "recover", "set-password", "remove-password":
		status := a.credentials.status()
		if request.Action == "change-password" && status.Mode == "windows" {
			request.Action = "set-password" // No current password exists to change.
		}
		mode := desktop.VaultSecretSetup
		switch request.Action {
		case "unlock":
			mode = desktop.VaultSecretUnlock
		case "change-password":
			mode = desktop.VaultSecretRotate
		case "recover":
			mode = desktop.VaultSecretRecover
		case "set-password":
			mode = desktop.VaultSecretSetPassword
		case "remove-password":
			mode = desktop.VaultSecretRemovePassword
		}
		if request.Action == "set-password" && status.Mode != "windows" {
			jsonError(w, 409, "This vault already uses a master password. Use Change master password.")
			return
		}
		if request.Action == "remove-password" && (status.Mode != "password" || status.State != "unlocked") {
			jsonError(w, 409, "Unlock the master-password vault before removing its master password.")
			return
		}
		if request.Action == "setup" && status.State != "setup-required" && status.State != "legacy" {
			jsonError(w, 409, "The vault is already configured. Unlock or recover it instead.")
			return
		}
		if request.Action != "setup" && (status.State == "setup-required" || status.State == "legacy") {
			jsonError(w, 409, "Set up the master-password vault first.")
			return
		}
		if a.promptVaultSecret == nil {
			jsonError(w, 503, "Native secret entry is unavailable.")
			return
		}
		response, promptErr := a.promptVaultSecret(r.Context(), desktop.VaultSecretRequest{Mode: mode})
		defer clear(response.Password)
		defer clear(response.CurrentPassword)
		defer clear(response.RecoveryKey)
		if promptErr != nil {
			jsonError(w, 400, "Vault action was canceled or native secret entry was unavailable.")
			return
		}
		if err := r.Context().Err(); err != nil {
			jsonError(w, 408, "Vault action was canceled.")
			return
		}
		switch request.Action {
		case "setup":
			err = a.credentials.setup(r.Context(), response.Password, response.GenerateRecovery, a.confirmVaultRecovery)
		case "unlock":
			err = a.credentials.unlock(r.Context(), response.Password)
		case "change-password":
			err = a.credentials.rotate(r.Context(), response.CurrentPassword, response.Password, "", response.GenerateRecovery, a.confirmVaultRecovery)
		case "set-password":
			err = a.credentials.addMasterPassword(r.Context(), response.Password, response.GenerateRecovery, a.confirmVaultRecovery)
		case "remove-password":
			err = a.credentials.removeMasterPassword(r.Context(), response.Password)
		case "recover":
			if len(response.RecoveryKey) == 0 {
				err = errors.New("Enter the offline recovery key in the native dialog.")
			} else {
				err = a.credentials.rotate(r.Context(), nil, response.Password, string(response.RecoveryKey), response.GenerateRecovery, a.confirmVaultRecovery)
			}
		}
		if err == nil {
			// Only a completed native authentication/setup may establish a new app
			// configuration identity or re-enable remembered unlock after reset.
			if request.Action == "recover" && a.credentials.status().Mode == "windows" {
				// A recovered vault now uses the new master password.
				_ = a.credentials.removeAuto()
			}
			configID, configErr := a.ensureCredentialConfigIdentity()
			if configErr == nil {
				// Without a master password, unlock is automatic; no remembered key is needed.
				configErr = a.credentials.remember(configID, response.Remember && request.Action != "remove-password")
			}
			if configErr != nil {
				// Never retain an unbound key after a config/remember failure.
				// Another instance may have just stored a newer remember record;
				// no unguarded cleanup is safe here.
				a.credentials.mu.Lock()
				a.credentials.clearMaterialLocked()
				a.credentials.warning = "The encrypted vault was kept, but unlock configuration could not be completed. This instance is locked; finish other credential operations, check app-data access and unlock again. An existing remembered record may still unlock another launch."
				a.credentials.mu.Unlock()
			}
		}
	case "import-previous":
		// Copy the saved credentials of the vault an earlier version kept for a
		// standalone window into the shared vault. That vault is only read.
		result, importErr := a.importPreviousVault(r.Context(), request.Source)
		if importErr != nil {
			code := http.StatusBadRequest
			if errors.Is(importErr, errCredentialVaultLocked) {
				code = http.StatusLocked
			}
			jsonError(w, code, importErr.Error())
			return
		}
		status := a.credentials.status()
		jsonResponse(w, 200, map[string]any{"ok": true, "available": status.Available, "state": status.State, "remembered": status.Remembered, "hasRecovery": status.HasRecovery, "legacyBackup": status.LegacyBackup, "warning": status.Warning, "protection": status.Protection, "mode": status.Mode, "imported": result.Imported, "alreadyPresent": result.Present, "skipped": result.Skipped, "skippedNames": result.SkippedNames, "previousVaults": a.previousVaults()})
		return
	case "setup-windows":
		status := a.credentials.status()
		if status.State != "setup-required" && status.State != "legacy" {
			jsonError(w, 409, "The vault is already configured.")
			return
		}
		if a.confirmVaultAction == nil {
			jsonError(w, 503, "Native vault confirmation is unavailable.")
			return
		}
		approved, confirmErr := a.confirmVaultAction(r.Context(), desktop.VaultActionRequest{Action: desktop.VaultActionNoPassword})
		if confirmErr != nil || !approved {
			jsonError(w, 400, "Vault setup was canceled; nothing was changed.")
			return
		}
		if err = a.credentials.setupWithoutPassword(r.Context()); err == nil {
			err = a.bindCredentialUnlock()
		}
	case "lock", "forget", "delete-vault", "cleanup-legacy":
		if (request.Action == "lock" || request.Action == "forget") && a.credentials.status().Mode == "windows" {
			jsonError(w, 409, "This vault has no master password and unlocks automatically. Add a master password to be able to lock it.")
			return
		}
		action := desktop.VaultActionLock
		switch request.Action {
		case "forget":
			action = desktop.VaultActionForget
		case "delete-vault":
			action = desktop.VaultActionDelete
		case "cleanup-legacy":
			action = desktop.VaultActionLegacyCleanup
		}
		if request.Action == "delete-vault" {
			state := a.credentials.status().State
			if state == "setup-required" || state == "legacy" {
				jsonError(w, 409, "No master-password vault exists. The retained legacy copy was kept.")
				return
			}
		}
		if request.Action == "cleanup-legacy" {
			if err := a.credentials.requireUnlocked(); err != nil {
				jsonError(w, 423, err.Error())
				return
			}
		}
		if a.confirmVaultAction == nil {
			jsonError(w, 503, "Native vault confirmation is unavailable.")
			return
		}
		approved, confirmErr := a.confirmVaultAction(r.Context(), desktop.VaultActionRequest{Action: action})
		if confirmErr != nil || !approved {
			jsonError(w, 400, "Vault action was canceled; existing data was kept.")
			return
		}
		if err := r.Context().Err(); err != nil {
			jsonError(w, 408, "Vault action was canceled.")
			return
		}
		switch request.Action {
		case "lock", "forget":
			a.stateMu.Lock()
			err = a.credentials.lockAndForget()
			// The vault's configuration identity lives in the main settings file,
			// which a standalone window does not hold in memory.
			a.ensureShared()
			if !a.shared.standalone {
				if raw, readErr := readCredentialFile(a.dataPath, 2<<20); readErr == nil && json.Valid(raw) {
					a.setStateLocked(raw)
					a.shared.seen = sha256.Sum256(raw)
				}
			}
			a.stateMu.Unlock()
		case "delete-vault":
			err = a.credentials.destroy()
		case "cleanup-legacy":
			err = a.credentials.cleanupLegacy()
		}
	default:
		jsonError(w, 400, "Unknown credential-vault action.")
		return
	}
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	status := a.credentials.status()
	jsonResponse(w, 200, map[string]any{"ok": true, "available": status.Available, "state": status.State, "remembered": status.Remembered, "hasRecovery": status.HasRecovery, "legacyBackup": status.LegacyBackup, "warning": status.Warning, "protection": status.Protection, "mode": status.Mode})
}

// bindCredentialUnlock completes an automatic (no-master-password) unlock:
// the pending key is bound to this configuration without a remembered record.
func (a *application) bindCredentialUnlock() error {
	configID, err := a.ensureCredentialConfigIdentity()
	if err == nil {
		err = a.credentials.remember(configID, false)
	}
	if err != nil {
		a.credentials.mu.Lock()
		a.credentials.clearMaterialLocked()
		a.credentials.warning = "The vault was kept, but automatic unlock could not be completed. Reopen the app; if it persists, check app-data access."
		a.credentials.mu.Unlock()
	}
	return err
}

// autoUnlockCredentials opens a no-master-password vault with its
// Windows-account-protected key. It never prompts and never blocks on another
// credential operation; failures leave the vault locked with a warning.
func (a *application) autoUnlockCredentials(ctx context.Context) {
	if a.credentials == nil {
		return
	}
	if status := a.credentials.status(); status.State != "locked" || status.Mode != "windows" {
		return
	}
	if !a.credentialChanges.TryLock() {
		return
	}
	defer a.credentialChanges.Unlock()
	password, err := a.credentials.readAutoPassword()
	if err == nil {
		err = a.credentials.unlock(ctx, password)
		clear(password)
	}
	if err != nil {
		a.credentials.mu.Lock()
		a.credentials.warning = "Automatic unlock failed: " + err.Error()
		a.credentials.mu.Unlock()
		return
	}
	_ = a.bindCredentialUnlock()
}

// The field is backend-owned. Incoming renderer state never chooses or replaces
// it. A missing/deleted/reset settings file cannot revive an old remember record.
func (a *application) ensureCredentialConfigIdentity() (string, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	// Every app of a data folder uses its one vault, bound to the identity in
	// the folder's main settings file.
	path := a.sharedSettingsPath()
	release, err := acquireCredentialGuard(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	defer release()
	id := credentialConfigID(path)
	if id != "" {
		return id, nil
	}
	root, readErr := readSharedSettings(path)
	if readErr != nil {
		return "", errors.New("App settings are invalid; they were not replaced.")
	}
	if !a.shared.standalone {
		// Settings this window holds in memory but has not written yet are
		// saved with the identity, as before.
		for name, raw := range stateMembers(a.state) {
			if _, exists := root.members[name]; !exists {
				root.members[name] = raw
			}
		}
		root.exists = true
	}
	id = randomHex(16)
	root.members[credentialConfigIdentityField], err = json.Marshal(id)
	if err != nil {
		return "", err
	}
	raw, err := root.marshal()
	if err != nil {
		return "", errors.New("App settings exceed their size limit.")
	}
	if err := writeSharedSettings(path, raw); err != nil {
		return "", err
	}
	a.shared.seen = sha256.Sum256(raw)
	if !a.shared.standalone {
		a.setStateLocked(raw)
	}
	return id, nil
}
