package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
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
		jsonResponse(w, http.StatusOK, a.credentials.status())
		return
	}
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	var request struct {
		Action string `json:"action"`
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
	case "setup", "unlock", "change-password", "recover":
		mode := desktop.VaultSecretSetup
		switch request.Action {
		case "unlock":
			mode = desktop.VaultSecretUnlock
		case "change-password":
			mode = desktop.VaultSecretRotate
		case "recover":
			mode = desktop.VaultSecretRecover
		}
		status := a.credentials.status()
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
			configID, configErr := a.ensureCredentialConfigIdentity()
			if configErr == nil {
				configErr = a.credentials.remember(configID, response.Remember)
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
	case "lock", "forget", "delete-vault", "cleanup-legacy":
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
			if raw, readErr := readCredentialFile(a.dataPath, 2<<20); readErr == nil && json.Valid(raw) {
				a.state = raw
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
	jsonResponse(w, 200, map[string]any{"ok": true, "available": status.Available, "state": status.State, "remembered": status.Remembered, "hasRecovery": status.HasRecovery, "legacyBackup": status.LegacyBackup, "warning": status.Warning, "protection": status.Protection})
}

// The field is backend-owned. Incoming renderer state never chooses or replaces
// it. A missing/deleted/reset settings file cannot revive an old remember record.
func (a *application) ensureCredentialConfigIdentity() (string, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	release, err := acquireCredentialGuard(filepath.Dir(a.dataPath))
	if err != nil {
		return "", err
	}
	defer release()
	id := credentialConfigID(a.dataPath)
	if id != "" {
		return id, nil
	}
	state := map[string]any{}
	current, readErr := readCredentialFile(a.dataPath, 2<<20)
	if readErr == nil {
		if json.Unmarshal(current, &state) != nil || state == nil {
			return "", errors.New("App settings are invalid; they were not replaced.")
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return "", readErr
	}

	id = randomHex(16)
	state[credentialConfigIdentityField] = id
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return "", err
	}
	if len(raw) > 2<<20 {
		return "", errors.New("App settings exceed their size limit.")
	}
	if err := writeAtomic(a.dataPath, raw); err != nil {
		return "", err
	}
	a.state = raw
	return id, nil
}
