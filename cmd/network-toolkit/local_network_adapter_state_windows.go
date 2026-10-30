//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"networktoolkit/internal/platform"
)

func readLocalIP(ctx context.Context) (localIPSnapshot, error) {
	return localReadIPWithAdmin(ctx, readLocalIPConfiguration, localReadAdapterAdmin)
}
func localReadAdapterAdmin(ctx context.Context) ([]localAdapterAdminRecord, error) {
	if localSystemDirectoryError != nil {
		return nil, localSystemDirectoryError
	}
	commandCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	plan := localPowerShellPlan(localReadAdapterAdminScript, "")
	cmd := exec.CommandContext(commandCtx, filepath.Join(localSystemDirectory, filepath.FromSlash(plan.Program)), plan.Arguments...)
	platform.ConfigureCommand(cmd)
	cmd.Dir = localSystemDirectory
	cmd.Env = localTrustedPowerShellEnvironment()
	var output localOutputBuffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		if commandCtx.Err() != nil {
			return nil, commandCtx.Err()
		}
		return nil, fmt.Errorf("Read adapter administrative state: %w", err)
	}
	return localDecodeAdapterAdmin(output.data)
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
