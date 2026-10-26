package main

import (
	"context"
	"errors"
	"networktoolkit/internal/model"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Startup completion belongs to the native shown-window callback, never an
// early HTTP request. Old installers accept the same nonce marker, so this
// fixes their premature-success check without changing their plan format.
var updateStartupOnce sync.Once

func (a *application) updateStartupReady() {
	updateStartupOnce.Do(func() {
		signalUpdateHealth(a.updateHealthDir)
		if a.updateHealthDir != "" {
			a.activatePrimary()
		}
		go func() {
			defer a.updateFinalizing.Store(false)
			// Readiness is not yet a committed install: the helper could still observe
			// process exit or timeout and roll back. Never point old launchers at the
			// new binary until the helper itself records successful installation.
			if a.updateHealthDir != "" {
				if err := awaitUpdateCommit(a.ctx, a.updateHealthDir, model.Version, 95*time.Second); err != nil {
					updateNamingState.Lock()
					updateNamingState.status.Warning = "Versioned filename setup was not applied: " + err.Error()
					updateNamingState.Unlock()
					return
				}
			}
			if err := a.updateRetentionAfterHealthyStartup(); err != nil {
				updateNamingState.Lock()
				updateNamingState.status.Warning = "Update cleanup pending: " + err.Error()
				updateNamingState.Unlock()
			}
			if err := a.reopenPromotedUpdate(); err != nil {
				updateNamingState.Lock()
				updateNamingState.status.Warning = err.Error()
				updateNamingState.Unlock()
			}
		}()
	})
}

func updateCommittedResult(result, version string) bool {
	result = strings.TrimSpace(result)
	return result == "Installed "+version+" successfully. Previous executable retained as previous.exe." ||
		result == "Installed "+version+" and confirmed the application interface started. Previous executable retained as previous.exe."
}

func awaitUpdateCommit(ctx context.Context, dir, version string, timeout time.Duration) error {
	p, err := updateReadPlan(dir)
	if err != nil || p.Version != version {
		return errors.New("The update confirmation plan is invalid")
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		b, err := updateNamingReadFile(filepath.Join(dir, "update-result.txt"), 64<<10)
		if err == nil && len(strings.TrimSpace(string(b))) > 0 {
			if updateCommittedResult(string(b), version) {
				return nil
			}
			return errors.New("The update helper did not confirm a successful installation; existing launchers were preserved")
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return errors.New("The update helper has not confirmed completion; existing launchers were preserved")
		case <-ticker.C:
		}
	}
}
