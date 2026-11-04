package desktop

import (
	"bytes"
	"context"
	"errors"
	"math"
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
	// Adds a master password to a vault set up without one.
	VaultSecretSetPassword VaultSecretMode = "set-password"
	// Asks for the current master password before switching to Windows-account protection.
	VaultSecretRemovePassword VaultSecretMode = "remove-password"
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
	VaultActionNoPassword    VaultAction = "no-password"
)

type VaultActionRequest struct {
	Action VaultAction
	Owner  uintptr
	Target string // Metadata only, never a secret. Quoted before display.
}

var ErrVaultDialogCancelled = errors.New("Native vault dialog was cancelled")

const vaultRememberDisclosure = "Remembering unlock uses this Windows user's DPAPI protection. Other programs running as this user, administrators or a compromised OS may unlock the vault. This is not protection from same-user malware."

const vaultNoPasswordDisclosure = "Without a master password, saved credentials are encrypted with a random key that is protected by this Windows account (DPAPI) and unlocked automatically when you open Network Toolkit. Other programs running as this Windows user, administrators or a compromised OS may read them. You can add a master password later in Password manager."

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
	strength    bool // Live strength gauge under the new password field.
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
	return nil
}

// Any non-empty master password is accepted (owner decision). The dialog shows
// this estimate as a live gauge so the choice is informed, never enforced.
type vaultPasswordStrength struct {
	Score int // 0 very weak … 4 very strong
	Label string
	Hint  string
}

var vaultCommonPasswords = []string{"password", "passw0rd", "123456", "12345678", "123456789", "1234567890", "qwerty", "qwertyuiop", "letmein", "welcome", "admin", "administrator", "root", "toor", "cisco", "changeme", "default", "secret", "iloveyou", "monkey", "dragon", "master", "network", "toolkit", "abc123", "111111", "000000", "trustno1", "login", "guest"}

// estimateVaultPasswordStrength is a deliberately simple, offline estimate:
// character-pool entropy, minus runs, repeats, keyboard/number sequences and a
// small list of very common passwords. It reads only the slice it is given.
func estimateVaultPasswordStrength(password []byte) vaultPasswordStrength {
	runes := []rune(string(password))
	if len(runes) == 0 {
		return vaultPasswordStrength{Score: 0, Label: "Enter a password", Hint: "Any password is allowed. Longer passphrases are much harder to guess."}
	}
	var lower, upper, digit, symbol, space, other bool
	for _, r := range runes {
		switch {
		case r >= 'a' && r <= 'z':
			lower = true
		case r >= 'A' && r <= 'Z':
			upper = true
		case r >= '0' && r <= '9':
			digit = true
		case r == ' ':
			space = true
		case r < 128:
			symbol = true
		default:
			other = true
		}
	}
	pool := 0
	for _, part := range []struct {
		used bool
		size int
	}{{lower, 26}, {upper, 26}, {digit, 10}, {symbol, 33}, {space, 1}, {other, 100}} {
		if part.used {
			pool += part.size
		}
	}
	perChar := math.Log2(float64(max(pool, 2)))
	// Characters that continue a repeat or a +1/-1 sequence add little.
	effective := 1.0
	for i := 1; i < len(runes); i++ {
		delta := runes[i] - runes[i-1]
		if delta == 0 || delta == 1 || delta == -1 {
			effective += 0.25
		} else {
			effective++
		}
	}
	bits := effective * perChar
	folded := strings.ToLower(string(password))
	trimmed := strings.TrimRight(strings.TrimLeft(folded, "0123456789!@#$%^&*. "), "0123456789!@#$%^&*. ")
	for _, common := range vaultCommonPasswords {
		if folded == common || trimmed == common {
			bits = math.Min(bits, 10)
			break
		}
		if strings.Contains(folded, common) {
			bits -= float64(utf8.RuneCountInString(common)) * perChar * 0.75
		}
	}
	strength := vaultPasswordStrength{}
	switch {
	case bits < 28:
		strength = vaultPasswordStrength{0, "Very weak", "Easy to guess. Use a longer passphrase of several unrelated words."}
	case bits < 40:
		strength = vaultPasswordStrength{1, "Weak", "Add more length; mixing character types helps less than length."}
	case bits < 60:
		strength = vaultPasswordStrength{2, "Fair", "Reasonable. A few more characters or words make it much stronger."}
	case bits < 80:
		strength = vaultPasswordStrength{3, "Strong", "Hard to guess offline."}
	default:
		strength = vaultPasswordStrength{4, "Very strong", "Very hard to guess offline."}
	}
	return strength
}

// vaultStrengthGaugeText renders a five-segment text gauge for the native dialog.
func vaultStrengthGaugeText(password []byte) string {
	s := estimateVaultPasswordStrength(password)
	if len(password) == 0 {
		return "Strength: " + strings.Repeat("░", 10) + "  " + s.Label + ". " + s.Hint
	}
	filled := (s.Score + 1) * 2
	return "Strength: " + strings.Repeat("█", filled) + strings.Repeat("░", 10-filled) + "  " + s.Label + ". " + s.Hint
}

func vaultSecretSpec(request VaultSecretRequest) (vaultDialogSpec, error) {
	spec := vaultDialogSpec{owner: request.Owner, remember: true, acceptLabel: "Continue"}
	newPassword := true
	switch request.Mode {
	case VaultSecretSetup:
		spec.title = "Network Toolkit — set master password"
		spec.body = "Create a master password. Any password is allowed; the gauge shows how hard it is to guess. Spaces are significant; passwords are not trimmed. Existing legacy data, if present, is migrated only after successful setup; its original DPAPI-only copy is retained until explicit cleanup."
	case VaultSecretUnlock:
		spec.title = "Network Toolkit — unlock credential vault"
		spec.body = "Enter the master password to unlock saved credentials. Existing authenticated connections are not changed."
		newPassword = false
	case VaultSecretRotate:
		spec.title = "Network Toolkit — change master password"
		spec.body = "Enter the current master password and choose a new one. Any password is allowed; the gauge shows how hard it is to guess. Spaces are significant. Previous recovery keys will no longer unlock the new vault; retained old copies keep their old protection."
		spec.fields = append(spec.fields, vaultDialogField{id: vaultFieldCurrent, label: "Current master password", masked: true})
	case VaultSecretSetPassword:
		spec.title = "Network Toolkit — add a master password"
		spec.body = "Choose a master password for the credential vault. Any password is allowed; the gauge shows how hard it is to guess. After this, the vault no longer unlocks automatically and the stored Windows-account key is removed."
	case VaultSecretRemovePassword:
		spec.title = "Network Toolkit — remove master password"
		spec.body = "Enter the current master password to switch to automatic unlock. " + vaultNoPasswordDisclosure
		spec.remember = false
		spec.acceptLabel = "Remove password"
		newPassword = false
	case VaultSecretRecover:
		spec.title = "Network Toolkit — recover credential vault"
		spec.body = "Enter your offline recovery key and choose a new master password (any password; the gauge shows its strength). The key is not your password. Previous recovery keys will no longer unlock the new vault; retained old copies keep their old protection."
		spec.fields = append(spec.fields, vaultDialogField{id: vaultFieldRecovery, label: "Offline recovery key (visible text)"})
	default:
		return vaultDialogSpec{}, errors.New("Unsupported native vault password operation")
	}
	label := "New master password"
	if !newPassword {
		label = "Master password"
		if request.Mode == VaultSecretRemovePassword {
			label = "Current master password"
		}
	}
	spec.fields = append(spec.fields, vaultDialogField{id: vaultFieldPassword, label: label, masked: true})
	if newPassword {
		spec.fields = append(spec.fields, vaultDialogField{id: vaultFieldConfirm, label: "Confirm new master password", masked: true})
		spec.recovery = true // Optional, unchecked. No key is generated here.
		spec.strength = true
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
	case VaultActionNoPassword:
		spec.body = "Set up the credential vault without a master password?\n\n" + vaultNoPasswordDisclosure
		spec.acceptLabel = "Set up"
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
