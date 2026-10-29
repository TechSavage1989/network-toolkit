package terminal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LocalShellOptions is deliberately not a remote connection or arbitrary command.
type LocalShellOptions struct {
	Type             string `json:"type"`
	WorkingDirectory string `json:"workingDirectory,omitempty"`
	Cols             int    `json:"cols"`
	Rows             int    `json:"rows"`
}
type LocalShellState struct {
	WorkingDirectory string `json:"workingDirectory"`
	Elevated         bool   `json:"elevated"`
}
type LocalShellTransport interface {
	Write([]byte) (int, error)
	Resize(int, int) error
	Close() error
	Done() <-chan error
	LocalShellState() LocalShellState
}
type ElevatedLocalShellOpener func(context.Context, LocalShellOptions, func([]byte)) (LocalShellTransport, error)

func (m *Manager) SetElevatedLocalShellOpener(opener ElevatedLocalShellOpener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.elevatedLocalShell = opener
}
func (m *Manager) openManagedLocalShell(ctx context.Context, req ConnectRequest, cb transportCallbacks) (transport, error) {
	if !req.RunAsAdministrator {
		return openLocalShell(ctx, req, cb)
	}
	m.mu.Lock()
	opener := m.elevatedLocalShell
	m.mu.Unlock()
	if opener == nil {
		return nil, errors.New("administrator local shells are unavailable")
	}
	return opener(ctx, LocalShellOptions{req.Type, req.WorkingDirectory, req.Cols, req.Rows}, cb.Output)
}

// ValidateLocalWorkingDirectory never expands shell syntax or changes process cwd.
// Symlink/reparse targets are resolved explicitly; native startup revalidates them.
func ValidateLocalWorkingDirectory(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if len(value) > 32760 || strings.IndexFunc(value, func(r rune) bool { return r < 32 || r == 127 }) >= 0 {
		return "", errors.New("starting directory contains invalid characters or is too long")
	}
	if !filepath.IsAbs(value) {
		return "", errors.New("starting directory must be an absolute folder path")
	}
	path, err := filepath.EvalSymlinks(value)
	if err != nil {
		return "", fmt.Errorf("open starting directory: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("open starting directory: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("starting directory must be a folder")
	}
	return filepath.Clean(path), nil
}

// OpenLocalShellProcess is used only inside the fixed-role native helper. It
// cannot launch a custom executable, flags, credentials or another helper.
func OpenLocalShellProcess(ctx context.Context, options LocalShellOptions, output func([]byte)) (LocalShellTransport, error) {
	if options.Type != "cmd" && options.Type != "powershell" {
		return nil, errors.New("unsupported local shell")
	}
	req, err := normalizeRequest(ConnectRequest{Type: options.Type, WorkingDirectory: options.WorkingDirectory, Cols: options.Cols, Rows: options.Rows})
	if err != nil {
		return nil, err
	}
	conn, err := openLocalShell(ctx, req, transportCallbacks{Output: output})
	if err != nil {
		return nil, err
	}
	local, ok := conn.(LocalShellTransport)
	if !ok {
		conn.Close()
		return nil, errors.New("local shell metadata is unavailable")
	}
	return local, nil
}
