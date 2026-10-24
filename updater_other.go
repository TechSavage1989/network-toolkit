//go:build !windows

package main

import (
	"context"
	"errors"
	"os/exec"
	"time"
)

func updateInstallSupported() bool               { return false }
func configureUpdateHelperProcess(cmd *exec.Cmd) {}
func waitUpdateParent(pid int, timeout time.Duration, ready string) error {
	return errors.New("Windows is required.")
}
func replaceUpdateExecutable(target, candidate, backup string) error {
	return errors.New("Windows is required.")
}

func openUpdateReleases(ctx context.Context) error {
	return errors.New("Open the public GitHub release page in your browser.")
}
func restoreMissingUpdateTarget(backup, target string) error {
	return errors.New("Windows is required.")
}

func configureUpdateAppProcess(cmd *exec.Cmd) {}
