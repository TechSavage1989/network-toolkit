package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type updatePlan struct {
	Target          string   `json:"target"`
	TargetDir       string   `json:"targetDir,omitempty"`
	LaunchTarget    string   `json:"launchTarget,omitempty"`
	WorkingDir      string   `json:"workingDir"`
	OriginalSHA256  string   `json:"originalSHA256"`
	CandidateSHA256 string   `json:"candidateSHA256"`
	Version         string   `json:"version"`
	ParentPID       int      `json:"parentPID"`
	Nonce           string   `json:"nonce"`
	Args            []string `json:"args"`
}

func updateReadPlan(dir string) (updatePlan, error) {
	var p updatePlan
	if !filepath.IsAbs(dir) || !strings.HasPrefix(filepath.Base(dir), ".network-toolkit-update-") {
		return p, errors.New("Invalid update staging folder.")
	}
	real, e := filepath.EvalSymlinks(dir)
	if e != nil || !strings.EqualFold(real, filepath.Clean(dir)) {
		return p, errors.New("Update staging folder must not be a link.")
	}
	data, e := updateNamingReadFile(filepath.Join(dir, "plan.json"), 64<<10)
	if e != nil {
		return p, e
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if e = d.Decode(&p); e != nil {
		return p, e
	}
	if e = d.Decode(new(any)); e != io.EOF {
		return p, errors.New("Update plan has trailing data")
	}
	if !filepath.IsAbs(p.WorkingDir) || p.Target != filepath.Base(p.Target) || strings.ContainsAny(p.Target, "/\\:") || !strings.HasSuffix(strings.ToLower(p.Target), ".exe") || len(p.Nonce) != 64 || p.ParentPID <= 0 || !updateVersionPattern.MatchString(p.Version) || len(p.OriginalSHA256) != 64 || len(p.CandidateSHA256) != 64 {
		return p, errors.New("Invalid update plan.")
	}
	for _, digest := range []string{p.Nonce, p.OriginalSHA256, p.CandidateSHA256} {
		b, err := hex.DecodeString(digest)
		if err != nil || len(b) != 32 {
			return p, errors.New("Invalid update plan checksum or nonce")
		}
	}
	if p.TargetDir != "" {
		if err := updateNamingCheckDirectory(p.TargetDir); err != nil {
			return p, err
		}
	}
	if p.LaunchTarget != "" && (p.TargetDir == "" || (p.LaunchTarget != updateOfficialExecutableName(p.Version) && p.LaunchTarget != updateStableExecutableName)) {
		return p, errors.New("Invalid versioned launch target")
	}
	return p, nil
}

func updatePlanTarget(dir string, p updatePlan) string {
	root := p.TargetDir
	if root == "" {
		root = filepath.Dir(dir)
	} // 1.17.58-60 bootstrap plans.
	return filepath.Join(root, p.Target)
}
func updatePlanLaunchTarget(dir string, p updatePlan) string {
	if p.LaunchTarget != "" {
		return filepath.Join(p.TargetDir, p.LaunchTarget)
	}
	return updatePlanTarget(dir, p)
}
func updateRestartArgs(args []string) []string {
	out := []string{}
	for i := 0; i < len(args); i++ {
		if args[i] == "--update-health" || args[i] == "-update-health" {
			i++
			continue
		}
		if strings.HasPrefix(args[i], "--update-health=") || strings.HasPrefix(args[i], "-update-health=") {
			continue
		}
		out = append(out, args[i])
	}
	return out
}
func startUpdateInstaller(s *stagedUpdate) error {
	if !updateInstallSupported() {
		return errors.New("Installing updates is supported on Windows only.")
	}
	exe, e := updateCurrentExecutable()
	if e != nil {
		return e
	}
	exe, e = filepath.EvalSymlinks(exe)
	if e != nil {
		return e
	}
	if e = updateNamingCheckDirectory(s.dir); e != nil {
		return e
	}
	if e = updateNamingCheckDirectory(filepath.Dir(exe)); e != nil {
		return e
	}
	b, e := updateNamingReadFile(filepath.Join(s.dir, "candidate.exe"), updateMaxEXE)
	if e != nil {
		return e
	}
	if updateHash(b) != s.SHA256 {
		return errors.New("Staged update changed. Import or download it again.")
	}
	if e = updateValidatePE(b, s.Version); e != nil {
		return e
	}
	old, e := updateNamingReadFile(exe, updateMaxEXE)
	if e != nil {
		return e
	}
	helper := filepath.Join(s.dir, "installer.exe")
	// A previous failed start is never silently reused.
	if e = updateWriteExclusive(helper, old, 0700); e != nil {
		return e
	}
	workingDir, e := os.Getwd()
	if e != nil {
		os.Remove(helper)
		return fmt.Errorf("Cannot preserve working directory: %w", e)
	}
	p := updatePlan{TargetDir: filepath.Dir(exe), WorkingDir: workingDir, Target: filepath.Base(exe), OriginalSHA256: updateHash(old), CandidateSHA256: s.SHA256, Version: s.Version, ParentPID: os.Getpid(), Nonce: randomHex(32), Args: updateRestartArgs(os.Args[1:])}
	if _, official := updateOfficialExecutableVersion(p.Target); official {
		p.LaunchTarget = updateStableExecutableName
		if _, err := os.Lstat(filepath.Join(p.TargetDir, p.LaunchTarget)); !os.IsNotExist(err) {
			_ = os.Remove(helper)
			return errors.New("The new version filename already exists; it was not overwritten")
		}
	}
	encoded, _ := json.Marshal(p)
	if e = updateWriteExclusive(filepath.Join(s.dir, "plan.json"), encoded, 0600); e != nil {
		os.Remove(helper)
		return e
	}
	cmd := exec.Command(helper, "--apply-update", s.dir)
	configureUpdateHelperProcess(cmd)
	if e = cmd.Start(); e != nil {
		os.Remove(helper)
		os.Remove(filepath.Join(s.dir, "plan.json"))
		return e
	}
	// Do not stop the original until the helper has opened its process handle.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(s.dir, "helper-ready")); err == nil {
			_ = cmd.Process.Release()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	os.Remove(filepath.Join(s.dir, "helper-ready"))
	os.Remove(helper)
	os.Remove(filepath.Join(s.dir, "plan.json"))
	return errors.New("The update helper did not start. The current application remains running.")
}
func runUpdateHelperIfRequested() bool {
	if len(os.Args) < 2 || os.Args[1] != "--apply-update" {
		return false
	}
	if len(os.Args) != 3 || !updateInstallSupported() {
		platform.NotifyFatal("Invalid update helper request.")
		return true
	}
	if e := applyUpdatePlan(os.Args[2]); e != nil {
		platform.NotifyFatal("Network Toolkit update failed.\n\n" + e.Error() + "\n\nYour saved settings have not been changed. Inspect the update folder in the app settings location for previous.exe and update-result.txt.")
	}
	return true
}
func signalUpdateHealth(dir string) {
	if dir == "" {
		return
	}
	p, e := updateReadPlan(dir)
	if e != nil {
		return
	}
	exe, e := os.Executable()
	if e != nil {
		return
	}
	exe, e = filepath.EvalSymlinks(exe)
	if e != nil {
		return
	}
	if !strings.EqualFold(exe, updatePlanLaunchTarget(dir, p)) || p.Version != model.Version {
		return
	}
	_ = updateWriteExclusive(filepath.Join(dir, "ready"), []byte(p.Nonce), 0600)
}

type updateInstallOperations struct {
	executable     func() (string, error)
	waitParent     func(int, time.Duration, string) error
	replace        func(string, string, string) error
	restoreMissing func(string, string) error
	startupTimeout time.Duration
}

func applyUpdatePlan(dir string) error {
	return applyUpdatePlanWithOperations(dir, updateInstallOperations{os.Executable, waitUpdateParent, replaceUpdateExecutable, restoreMissingUpdateTarget, 90 * time.Second})
}

func applyUpdatePlanWithOperations(dir string, ops updateInstallOperations) (result error) {
	p, e := updateReadPlan(dir)
	if e != nil {
		return e
	}
	defer func() {
		message := "Installed " + p.Version + " and confirmed the application interface started. Previous executable retained as previous.exe."
		if result != nil {
			message = result.Error()
		}
		updateLog(dir, "%s", message)
		_ = os.WriteFile(filepath.Join(dir, "update-result.txt"), []byte(message+"\n"), 0600)
	}()
	self, e := ops.executable()
	if e != nil {
		return e
	}
	self, e = filepath.EvalSymlinks(self)
	if e != nil {
		return e
	}
	if !strings.EqualFold(self, filepath.Join(dir, "installer.exe")) {
		return errors.New("Update helper is outside its staging folder.")
	}
	selfHash, e := updateFileHash(self)
	if e != nil || selfHash != p.OriginalSHA256 {
		return errors.New("Update helper integrity check failed.")
	}
	progress := startUpdateProgressWindow(p.Version)
	defer progress.Close()
	updateLog(dir, "Helper started; waiting for original application to exit.")
	target := updatePlanTarget(dir, p)
	candidate := filepath.Join(dir, "candidate.exe")
	backup := filepath.Join(dir, "previous.exe")
	if _, e = os.Lstat(backup); !os.IsNotExist(e) {
		return errors.New("An update backup already exists. Refusing to overwrite it.")
	}
	if e = ops.waitParent(p.ParentPID, 45*time.Second, filepath.Join(dir, "helper-ready")); e != nil {
		return e
	}
	progress.Set("Verifying the application and update files...")
	updateLog(dir, "Original application exited; verifying files.")
	oldHash, e := updateFileHash(target)
	if e != nil || oldHash != p.OriginalSHA256 {
		return errors.New("Installed executable changed; update was not applied.")
	}
	newHash, e := updateFileHash(candidate)
	if e != nil || newHash != p.CandidateSHA256 {
		return errors.New("Staged executable changed; update was not applied.")
	}
	// ReplaceFileW requires all participants on the target volume. Settings
	// may be on a different drive; only these transaction files live beside
	// the application, and their nonce+hash ownership is recorded by the plan.
	stagedCandidate := candidate
	stagedBackup := backup
	if p.TargetDir != "" {
		candidate = filepath.Join(p.TargetDir, ".network-toolkit-swap-"+p.Nonce+"-candidate.exe")
		backup = filepath.Join(p.TargetDir, ".network-toolkit-swap-"+p.Nonce+"-previous.exe")
		if _, err := os.Lstat(backup); !os.IsNotExist(err) {
			return errors.New("Recovery transaction file already exists; refusing to overwrite it")
		}
		if e = updateCopyVerified(stagedCandidate, candidate, p.CandidateSHA256); e != nil {
			return e
		}
		defer updateRemoveVerified(candidate, p.CandidateSHA256)
	}
	progress.Set("Installing the verified update...")
	if e = ops.replace(target, candidate, backup); e != nil {
		// ReplaceFileW can leave the original under its backup name on error1177.
		if _, statErr := os.Stat(target); os.IsNotExist(statErr) {
			if hash, err := updateFileHash(backup); err == nil && hash == p.OriginalSHA256 {
				if err = ops.restoreMissing(backup, target); err != nil {
					return fmt.Errorf("Replacement failed (%v), and recovery failed (%v). Restore previous.exe manually", e, err)
				}
			}
		}
		return fmt.Errorf("Could not replace the application (another window, antivirus, or folder permissions may be blocking it): %w", e)
	}
	originalTarget := target
	var startErr error
	launchTarget := updatePlanLaunchTarget(dir, p)
	if !updateNamingSamePath(target, launchTarget) {
		startErr = moveUpdateNamingNoReplace(target, launchTarget)
		if startErr == nil {
			target = launchTarget
		}
	}
	updateLog(dir, "Executable replaced; starting version %s at %s.", p.Version, target)
	progress.Set("Starting the updated application...")
	cmd := updateRestartCommand(target, p, dir)
	configureUpdateAppProcess(cmd)
	if startErr == nil {
		startErr = cmd.Start()
	}
	if startErr == nil {
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		progress.Set("Waiting for the updated interface to be ready...")
		updateLog(dir, "New process started (PID %d); waiting for visible interface readiness.", cmd.Process.Pid)
		timeout := time.NewTimer(ops.startupTimeout)
		defer timeout.Stop()
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		waiting := true
		for waiting {
			select {
			case <-ticker.C:
				b, _ := os.ReadFile(filepath.Join(dir, "ready"))
				if string(b) == p.Nonce {
					updateLog(dir, "Application startup confirmed.")
					if p.TargetDir != "" {
						if e = updateCopyVerified(backup, stagedBackup, p.OriginalSHA256); e != nil {
							// The healthy app stays running. Preserve the only rollback
							// copy beside it. Retention resumes from the verified plan.
							updateLog(dir, "Previous-version transfer pending: %v. Recovery retained at %s", e, backup)
							return nil
						}
						if e = updateRemoveVerified(backup, p.OriginalSHA256); e != nil {
							updateLog(dir, "Temporary recovery copy cleanup pending: %v", e)
						}
					}
					return nil
				}
			case e := <-done:
				startErr = fmt.Errorf("New app exited before confirming startup: %v", e)
				waiting = false
			case <-timeout.C:
				_ = cmd.Process.Kill()
				<-done
				startErr = errors.New("New app did not confirm startup within 90 seconds.")
				waiting = false
			}
		}
	}
	progress.Set("Restoring the previous application after a restart failure...")
	updateLog(dir, "Restart failed: %v. Restoring the previous executable.", startErr)
	// The failed new process is stopped before rollback; keep its binary for diagnostics.
	failed := filepath.Join(dir, "failed.exe")
	if p.TargetDir != "" {
		failed = filepath.Join(p.TargetDir, ".network-toolkit-swap-"+p.Nonce+"-failed.exe")
	}
	if e = ops.replace(target, backup, failed); e != nil {
		return fmt.Errorf("%v Rollback also failed: %w. Restore previous.exe manually while all app windows are closed", startErr, e)
	}
	if !updateNamingSamePath(target, originalTarget) {
		if err := moveUpdateNamingNoReplace(target, originalTarget); err != nil {
			return fmt.Errorf("Previous executable restored at %s, but its original name could not be restored: %w", target, err)
		}
		target = originalTarget
	}
	if p.TargetDir != "" {
		if err := updateCopyVerified(failed, filepath.Join(dir, "failed.exe"), p.CandidateSHA256); err == nil {
			_ = updateRemoveVerified(failed, p.CandidateSHA256)
		}
	}
	restore := updateRestartCommand(target, p, "")
	configureUpdateAppProcess(restore)
	if e = restore.Start(); e != nil {
		return fmt.Errorf("%v The previous executable was restored, but restart failed: %w", startErr, e)
	}
	updateLog(dir, "Previous executable restored; recovery process started (PID %d).", restore.Process.Pid)
	_ = restore.Process.Release()
	return fmt.Errorf("%v The previous executable was restored and restarted", startErr)
}

// Keep the launch directory as well as arguments: relative --data-dir values
// must resolve to the same settings folder after installation and rollback.
func updateRestartCommand(target string, p updatePlan, healthDir string) *exec.Cmd {
	args := append([]string{}, p.Args...)
	if healthDir != "" {
		args = append(args, "--update-health", healthDir)
	}
	cmd := exec.Command(target, args...)
	cmd.Dir = p.WorkingDir
	return cmd
}

// Logs are local to the already validated staging folder and contain no settings.
func updateLog(dir, format string, args ...any) {
	f, err := os.OpenFile(filepath.Join(dir, "update-progress.txt"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().UTC().Format(time.RFC3339), fmt.Sprintf(format, args...))
}

// Copies are exclusive and durable before source removal. These helpers never
// follow symlinks and never remove bytes not matching their transaction hash.
func updateCopyVerified(source, dest, digest string) error {
	b, err := updateNamingReadFile(source, updateMaxEXE)
	if err != nil {
		return err
	}
	if updateHash(b) != digest {
		return errors.New("Update transaction checksum changed")
	}
	if err = updateNamingWriteNewFile(dest, b, 0700); err != nil {
		return err
	}
	got, err := updateNamingReadFile(dest, updateMaxEXE)
	if err != nil {
		return err
	}
	if updateHash(got) != digest {
		return errors.New("Update transaction copy verification failed")
	}
	return nil
}

func updateRemoveVerified(path, digest string) error {
	b, err := updateNamingReadFile(path, updateMaxEXE)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if updateHash(b) != digest {
		return errors.New("Update transaction cleanup refused: checksum changed")
	}
	return os.Remove(path)
}
