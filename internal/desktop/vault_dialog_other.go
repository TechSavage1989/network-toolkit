//go:build !windows

package desktop

import (
	"context"
	"errors"
)

func nativeVaultDialogUnavailable(ctx context.Context) error {
	if err := validateCredentialConfirmationContext(ctx); err != nil {
		return err
	}
	return errors.New("The credential vault comes to the Linux edition in a later beta")
}

func PromptVaultSecret(ctx context.Context, request VaultSecretRequest) (VaultSecretResponse, error) {
	return VaultSecretResponse{}, nativeVaultDialogUnavailable(ctx)
}

func ConfirmVaultAction(ctx context.Context, request VaultActionRequest) (bool, error) {
	return false, nativeVaultDialogUnavailable(ctx)
}

func ConfirmVaultRecovery(ctx context.Context, key string) (bool, error) {
	return false, nativeVaultDialogUnavailable(ctx)
}
