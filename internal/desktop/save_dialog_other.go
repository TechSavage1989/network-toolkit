//go:build !windows

package desktop

import (
	"context"
	"errors"
)

// ErrSaveDialogUnavailable is returned where no native Save As dialog exists.
var (
	ErrSaveDialogUnavailable = errors.New("The Windows Save As dialog is available only in the Windows application")
	ErrSaveDialogBusy        = errors.New("A Save As dialog is already open. Finish or cancel it first.")
)

func PromptSaveFile(ctx context.Context, request SaveFileRequest) (SaveFileResponse, error) {
	return SaveFileResponse{}, ErrSaveDialogUnavailable
}
