//go:build !windows

package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var sourcePingUtilities sync.Map

type sourcePingUtilityKey struct {
	path           string
	size, modified int64
}

// iputils retains both -I values (device plus source address). Other ping
// implementations may replace the first selection, so refuse them explicitly.
// https://github.com/iputils/iputils/blob/master/ping/ping.c
func sourcePingArgs(binding *SourceBinding, goos string) ([]string, error) {
	if binding == nil {
		return nil, nil
	}
	if goos != "linux" {
		return nil, errors.New("explicit source-interface ICMP is supported on Windows and Linux only")
	}
	ip, err := binding.sourceIP()
	if err != nil {
		return nil, err
	}
	return []string{"-I", binding.InterfaceName, "-I", ip.String(), "-B"}, nil
}

func ensureSourcePingUtility(ctx context.Context, path string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect source-capable ping utility: %w", err)
	}
	key := sourcePingUtilityKey{path: path, size: stat.Size(), modified: stat.ModTime().UnixNano()}
	if _, ok := sourcePingUtilities.Load(key); ok {
		return nil
	}
	versionCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	command := exec.CommandContext(versionCtx, path, "-V")
	ConfigureCommand(command)
	output, runErr := command.CombinedOutput()
	if err := versionCtx.Err(); err != nil {
		return err
	}
	if err := verifySourcePingVersion(output, runErr); err != nil {
		return err
	}
	sourcePingUtilities.Store(key, struct{}{})
	return nil
}

func verifySourcePingVersion(output []byte, runErr error) error {
	if runErr != nil || !strings.Contains(strings.ToLower(string(output)), "iputils") {
		return errors.New("explicit source-interface ICMP on Linux requires iputils ping; automatic routing was not used")
	}
	return nil
}
