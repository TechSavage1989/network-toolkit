package main

import (
	"context"
	"errors"
	"net/http"

	"networktoolkit/internal/desktop"
)

// The vault's master password, recovery key and their confirmations are typed
// into the app's own form (owner decision after v1.21.81). Network Toolkit
// used to ask for them in a separate native Windows password box, and
// SentinelOne's Behavioral AI removed the program as a "Credential Prompt"
// for it. The form's fields arrive with the vault action, are checked with the
// same rules as that box (desktop.VaultSecretFromEntry) and are cleared after
// use. They are never saved, logged or returned. Native Yes/No confirmations
// and the recovery key display stay native.

// errVaultSecretNeeded asks the page for the vault form (code
// "vault-password-required"); nothing was changed.
var errVaultSecretNeeded = errors.New("Enter the master password in the Password manager form.")

type vaultSecretInput struct {
	Password         string `json:"password,omitempty"`
	Confirm          string `json:"confirm,omitempty"`
	CurrentPassword  string `json:"currentPassword,omitempty"`
	RecoveryKey      string `json:"recoveryKey,omitempty"`
	GenerateRecovery bool   `json:"generateRecovery,omitempty"`
	Remember         bool   `json:"remember,omitempty"`
}

// clear drops the references; Go strings cannot be overwritten in place, so
// clearing them is best effort, like the byte slices the native box gave.
func (s *vaultSecretInput) clear() {
	if s != nil {
		*s = vaultSecretInput{}
	}
}

type vaultSecretInputKey struct{}

func withVaultSecretInput(ctx context.Context, input *vaultSecretInput) context.Context {
	return context.WithValue(ctx, vaultSecretInputKey{}, input)
}

func vaultSecretInputFrom(ctx context.Context) *vaultSecretInput {
	input, _ := ctx.Value(vaultSecretInputKey{}).(*vaultSecretInput)
	return input
}

// vaultSecret returns the secrets an operation needs from the page's form.
// Without one it asks for the form; a.promptVaultSecret exists for tests only.
func (a *application) vaultSecret(ctx context.Context, input *vaultSecretInput, request desktop.VaultSecretRequest) (desktop.VaultSecretResponse, error) {
	if input == nil {
		if a.promptVaultSecret != nil {
			return a.promptVaultSecret(ctx, request)
		}
		return desktop.VaultSecretResponse{}, errVaultSecretNeeded
	}
	if err := ctx.Err(); err != nil {
		return desktop.VaultSecretResponse{}, err
	}
	entry := desktop.VaultSecretEntry{Password: []byte(input.Password), Confirm: []byte(input.Confirm), CurrentPassword: []byte(input.CurrentPassword), RecoveryKey: []byte(input.RecoveryKey), GenerateRecovery: input.GenerateRecovery, Remember: input.Remember}
	defer clear(entry.Password)
	defer clear(entry.Confirm)
	defer clear(entry.CurrentPassword)
	defer clear(entry.RecoveryKey)
	return desktop.VaultSecretFromEntry(request, entry)
}

func writeVaultSecretError(w http.ResponseWriter, err error) {
	if errors.Is(err, errVaultSecretNeeded) {
		jsonResponse(w, http.StatusPreconditionRequired, map[string]any{"error": err.Error(), "code": "vault-password-required"})
		return
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		jsonError(w, 408, "Vault action was canceled.")
		return
	}
	// The checks' messages are fixed text and never contain what was typed.
	jsonError(w, 400, err.Error()+".")
}
