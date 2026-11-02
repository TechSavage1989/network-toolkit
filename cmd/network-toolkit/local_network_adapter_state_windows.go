//go:build windows

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"networktoolkit/internal/platform"
)

func readLocalIP(ctx context.Context) (localIPSnapshot, error) {
	return localReadIPWithAdmin(ctx, readLocalIPConfiguration, localReadAdapterAdmin)
}

// readLocalIPBasic is the fast native read only. Administrative state is
// normalized to unknown/not editable until the full read replaces it.
func readLocalIPBasic(ctx context.Context) (localIPSnapshot, error) {
	snapshot, err := readLocalIPConfiguration(ctx)
	if err != nil {
		return snapshot, err
	}
	snapshot.Adapters = localMergeAdapterAdmin(snapshot.Adapters, nil)
	return snapshot, ctx.Err()
}
func localReadAdapterAdmin(ctx context.Context) ([]localAdapterAdminRecord, error) {
	if localSystemDirectoryError != nil {
		return nil, localSystemDirectoryError
	}
	commandCtx, cancel := context.WithTimeout(ctx, localAdapterAdminReadTimeout)
	defer cancel()
	plan := localPowerShellPlan(localReadAdapterAdminScript, "")
	cmd := exec.CommandContext(commandCtx, filepath.Join(localSystemDirectory, filepath.FromSlash(plan.Program)), plan.Arguments...)
	platform.ConfigureCommand(cmd)
	cmd.Dir = localSystemDirectory
	cmd.Env = localTrustedPowerShellEnvironment()
	records, err := localCollectAdapterAdmin(func(stdout, stderr io.Writer) error {
		cmd.Stdout, cmd.Stderr = stdout, stderr
		return cmd.Run()
	})
	if err != nil {
		if commandCtx.Err() != nil {
			return nil, commandCtx.Err()
		}
		return nil, fmt.Errorf("Read adapter administrative state: %w", err)
	}
	return records, nil
}
func localTrustedPowerShellEnvironment() []string {
	env := []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(strings.ToLower(entry), "psmodulepath=") {
			env = append(env, entry)
		}
	}
	return append(env, "PSModulePath="+filepath.Join(localSystemDirectory, "WindowsPowerShell", "v1.0", "Modules"))
}
