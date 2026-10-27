package desktop

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// These requests contain only an operation and optional native owner/metadata.
// Passwords and recovery keys are obtained by the native host, never HTML/JS.
type VaultSecretMode string

const (
	VaultSecretSetup   VaultSecretMode = "setup"
	VaultSecretUnlock  VaultSecretMode = "unlock"
	VaultSecretRotate  VaultSecretMode = "rotate"
	VaultSecretRecover VaultSecretMode = "recover"
)

type VaultSecretRequest struct {
	Mode  VaultSecretMode
	Owner uintptr
}

// The backend caller owns these byte slices and must clear them after use.
// Clearing is best effort: Go/Windows may retain copies, paging or crash dumps.
type VaultSecretResponse struct {
	Password         []byte `json:"-"`
	CurrentPassword  []byte `json:"-"`
	RecoveryKey      []byte `json:"-"`
	GenerateRecovery bool
	Remember         bool
}

type VaultAction string

const (
	VaultActionForget        VaultAction = "forget"
	VaultActionLock          VaultAction = "lock"
	VaultActionDelete        VaultAction = "delete"
	VaultActionLegacyCleanup VaultAction = "legacy-cleanup"
)

type VaultActionRequest struct {
	Action VaultAction
	Owner  uintptr
	Target string // Metadata only, never a secret. Quoted before display.
}

var ErrVaultDialogCancelled = errors.New("Native vault dialog was cancelled")

const vaultRememberDisclosure = "Remembering unlock uses this Windows user's DPAPI protection. Other programs running as this user, administrators or a compromised OS may unlock the vault. This is not protection from same-user malware."

const vaultRecoveryWarning = "Anyone with this recovery key can unlock the vault. Keep it offline and private. If both the master password and recovery key are lost, the vault cannot be recovered. Nothing is copied to the clipboard or saved by this dialog."

type vaultDialogField struct {
	id       uint16
	label    string
	masked   bool
	readonly bool
	value    string
}

type vaultDialogSpec struct {
	title       string
	body        string
	owner       uintptr
	fields      []vaultDialogField
	remember    bool
	recovery    bool
	acknowledge bool
	defaultNo   bool
	acceptLabel string
	validate    func(vaultDialogResult) error
}

type vaultDialogResult struct {
	accepted         bool
	values           map[uint16][]byte
	remember         bool
	generateRecovery bool
	acknowledged     bool
}

const (
	vaultFieldPassword uint16 = 100
	vaultFieldConfirm  uint16 = 101
	vaultFieldCurrent  uint16 = 102
	vaultFieldRecovery uint16 = 103
	vaultFieldKeyView  uint16 = 104
)

func (r *vaultDialogResult) clear() {
	for key, value := range r.values {
		clear(value)
		delete(r.values, key)
	}
	r.accepted = false
}

func clearVaultSecretResponse(r *VaultSecretResponse) {
	clear(r.Password)
	clear(r.CurrentPassword)
	clear(r.RecoveryKey)
	*r = VaultSecretResponse{}
}

func validateVaultSecret(value []byte, newPassword bool) error {
	if len(value) == 0 || len(value) > 1024 || !utf8.Valid(value) || bytes.IndexByte(value, 0) >= 0 {
		return errors.New("Enter a valid secret of at most 1024 UTF-8 bytes; NUL characters are not supported")
	}
	if newPassword && utf8.RuneCount(value) < 12 {
		return errors.New("Use at least 12 Unicode characters for the new master password")
	}
	return nil
}

func vaultSecretSpec(request VaultSecretRequest) (vaultDialogSpec, error) {
	spec := vaultDialogSpec{owner: request.Owner, remember: true, acceptLabel: "Continue"}
	newPassword := true
	switch request.Mode {
	case VaultSecretSetup:
		spec.title = "Network Toolkit — set master password"
		spec.body = "Create a master password (at least 12 Unicode characters). Spaces are significant; passwords are not trimmed. Existing legacy data, if present, is migrated only after successful setup; its original DPAPI-only copy is retained until explicit cleanup."
	case VaultSecretUnlock:
		spec.title = "Network Toolkit — unlock credential vault"
		spec.body = "Enter the master password to unlock saved credentials. Existing authenticated connections are not changed."
		newPassword = false
	case VaultSecretRotate:
		spec.title = "Network Toolkit — change master password"
		spec.body = "Enter the current master password and choose a new one (at least 12 Unicode characters). Spaces are significant. Previous recovery keys will no longer unlock the new vault; retained old copies keep their old protection."
		spec.fields = append(spec.fields, vaultDialogField{id: vaultFieldCurrent, label: "Current master password", masked: true})
	case VaultSecretRecover:
		spec.title = "Network Toolkit — recover credential vault"
		spec.body = "Enter your offline recovery key and choose a new master password (at least 12 Unicode characters). The key is not your password. Previous recovery keys will no longer unlock the new vault; retained old copies keep their old protection."
		spec.fields = append(spec.fields, vaultDialogField{id: vaultFieldRecovery, label: "Offline recovery key (visible text)"})
	default:
		return vaultDialogSpec{}, errors.New("Unsupported native vault password operation")
	}
	label := "New master password"
	if !newPassword {
		label = "Master password"
	}
	spec.fields = append(spec.fields, vaultDialogField{id: vaultFieldPassword, label: label, masked: true})
	if newPassword {
		spec.fields = append(spec.fields, vaultDialogField{id: vaultFieldConfirm, label: "Confirm new master password", masked: true})
		spec.recovery = true // Optional, unchecked. No key is generated here.
	}
	spec.validate = func(result vaultDialogResult) error {
		if err := validateVaultSecret(result.values[vaultFieldPassword], newPassword); err != nil {
			return err
		}
		if newPassword && !bytes.Equal(result.values[vaultFieldPassword], result.values[vaultFieldConfirm]) {
			return errors.New("The new master passwords do not match")
		}
		if request.Mode == VaultSecretRotate {
			return validateVaultSecret(result.values[vaultFieldCurrent], false)
		}
		if request.Mode == VaultSecretRecover {
			return validateVaultSecret(result.values[vaultFieldRecovery], false)
		}
		return nil
	}
	return spec, nil
}

// Hold the existing security-decision slot until native destruction completes,
// even if the request is cancelled while the dialog is visible. No prompt is
// queued: the user must explicitly retry after the current decision closes.
func runVaultDialog(ctx context.Context, spec vaultDialogSpec, prompt func(context.Context, vaultDialogSpec) (vaultDialogResult, error)) (vaultDialogResult, error) {
	if err := validateCredentialConfirmationContext(ctx); err != nil {
		return vaultDialogResult{}, err
	}
	if prompt == nil {
		return vaultDialogResult{}, errors.New("Native vault dialog is unavailable")
	}
	select {
	case credentialConfirmationSlot <- struct{}{}:
	default:
		return vaultDialogResult{}, errors.New("Another native security dialog is already open")
	}
	defer func() { <-credentialConfirmationSlot }()
	if err := ctx.Err(); err != nil {
		return vaultDialogResult{}, err
	}
	result, err := prompt(ctx, spec)
	if cancelled := ctx.Err(); cancelled != nil {
		result.clear()
		return vaultDialogResult{}, cancelled
	}
	if err != nil || !result.accepted {
		result.clear()
		if err == nil {
			err = ErrVaultDialogCancelled
		}
		return vaultDialogResult{}, err
	}
	if spec.validate != nil {
		if err := spec.validate(result); err != nil {
			result.clear()
			return vaultDialogResult{}, err
		}
	}
	return result, nil
}

func promptVaultSecret(ctx context.Context, request VaultSecretRequest, prompt func(context.Context, vaultDialogSpec) (vaultDialogResult, error)) (VaultSecretResponse, error) {
	spec, err := vaultSecretSpec(request)
	if err != nil {
		return VaultSecretResponse{}, err
	}
	result, err := runVaultDialog(ctx, spec, prompt)
	if err != nil {
		return VaultSecretResponse{}, err
	}
	// Move only the requested secrets to the backend; discard confirmation and
	// unexpected fields, even for an injected test callback.
	response := VaultSecretResponse{Password: result.values[vaultFieldPassword], Remember: result.remember, GenerateRecovery: spec.recovery && result.generateRecovery}
	delete(result.values, vaultFieldPassword)
	if request.Mode == VaultSecretRotate {
		response.CurrentPassword = result.values[vaultFieldCurrent]
		delete(result.values, vaultFieldCurrent)
	}
	if request.Mode == VaultSecretRecover {
		response.RecoveryKey = result.values[vaultFieldRecovery]
		delete(result.values, vaultFieldRecovery)
	}
	result.clear()
	return response, nil
}

func vaultActionSpec(request VaultActionRequest) (vaultDialogSpec, error) {
	if len(request.Target) > 120 || strings.IndexFunc(request.Target, unicode.IsControl) >= 0 || !utf8.ValidString(request.Target) {
		return vaultDialogSpec{}, errors.New("Invalid native vault confirmation metadata")
	}
	spec := vaultDialogSpec{title: "Network Toolkit — confirm vault action", owner: request.Owner, defaultNo: true, acceptLabel: "Yes"}
	switch request.Action {
	case VaultActionForget:
		spec.body = "Forget the remembered unlock and lock the credential vault? The master password or offline recovery key will be required to unlock it again. New saved-credential connections and reconnects stay blocked until an explicit unlock."
	case VaultActionLock:
		spec.body = "Lock the credential vault and forget the remembered unlock? The master password or offline recovery key will be required to unlock it again. New saved-credential connections and reconnects stay blocked until an explicit unlock."
	case VaultActionDelete:
		spec.body = "Permanently delete the current master-password vault and ALL its saved credentials, including its remembered unlock? The legacy DPAPI-only copy, if present, remains until separate legacy cleanup. This does not revoke remote passwords/keys or erase other backups and old disk blocks."
	case VaultActionLegacyCleanup:
		spec.body = "Permanently delete the retained legacy DPAPI-only credential copy? This cannot be undone by Network Toolkit and does not erase other backups or old disk blocks. Confirm only after checking that the new vault works and retaining any recovery key you chose."
	default:
		return vaultDialogSpec{}, errors.New("Unsupported native vault confirmation action")
	}
	if request.Target != "" {
		spec.body += "\n\nTarget: " + strconv.QuoteToASCII(request.Target)
	}
	spec.body += "\n\nExisting authenticated connections stay connected. Approve only if you initiated this exact action in Network Toolkit."
	return spec, nil
}

func confirmVaultAction(ctx context.Context, request VaultActionRequest, prompt func(context.Context, vaultDialogSpec) (vaultDialogResult, error)) (bool, error) {
	spec, err := vaultActionSpec(request)
	if err != nil {
		return false, err
	}
	result, err := runVaultDialog(ctx, spec, prompt)
	defer result.clear()
	if errors.Is(err, ErrVaultDialogCancelled) {
		return false, nil
	}
	return err == nil && result.accepted, err
}

func confirmVaultRecovery(ctx context.Context, key string, prompt func(context.Context, vaultDialogSpec) (vaultDialogResult, error)) (bool, error) {
	if len(key) == 0 || len(key) > 1024 || !utf8.ValidString(key) || strings.IndexFunc(key, unicode.IsControl) >= 0 {
		return false, errors.New("Invalid native recovery key")
	}
	spec := vaultDialogSpec{
		title: "Network Toolkit — retain offline recovery key", body: vaultRecoveryWarning,
		acknowledge: true, defaultNo: true, acceptLabel: "Activate",
		fields: []vaultDialogField{
			{id: vaultFieldKeyView, label: "Recovery key — keep offline (read only)", readonly: true, value: key},
			{id: vaultFieldRecovery, label: "Re-enter the complete key from your retained copy"},
		},
	}
	spec.validate = func(result vaultDialogResult) error {
		if !bytes.Equal(result.values[vaultFieldRecovery], []byte(key)) || !result.acknowledged {
			return errors.New("Re-enter the exact recovery key and acknowledge that you retained it offline")
		}
		return nil
	}
	result, err := runVaultDialog(ctx, spec, prompt)
	defer result.clear()
	if errors.Is(err, ErrVaultDialogCancelled) {
		return false, nil
	}
	return err == nil && result.accepted, err
}
