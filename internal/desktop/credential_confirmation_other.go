//go:build !windows

package desktop

import (
	"context"
	"errors"
)

func ConfirmCredentialDestination(ctx context.Context, request CredentialDestinationConfirmation) (bool, error) {
	if err := validateCredentialConfirmationContext(ctx); err != nil {
		return false, err
	}
	return false, errors.New("Credential destination approval requires the native Windows application")
}
