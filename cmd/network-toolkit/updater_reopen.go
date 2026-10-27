package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The legacy installer starts the new binary under the old version's name.
// After its transaction commits we may rename that running image. Reopen once
// from the canonical name so Windows process-image identity and elevated pipe
// peer checks remain exact. The child opens the parent handle before shutdown,
// and waits before flags, storage leases, or main-window singleton startup.
func waitUpdateReopenIfRequested() error {
	args, err := prepareUpdateReopen(os.Args, waitUpdateParent)
	if err == nil {
		os.Args = args
	}
	return err
}

func prepareUpdateReopen(args []string, wait func(int, time.Duration, string) error) ([]string, error) {
	if len(args) < 2 || args[1] != "--update-reopen" {
		return args, nil
	}
	if len(args) < 4 {
		return nil, errors.New("Invalid update reopen request")
	}
	pid, err := strconv.Atoi(args[2])
	if err != nil || pid <= 0 {
		return nil, errors.New("Invalid update reopen parent")
	}
	dir := args[3]
	if !strings.HasPrefix(filepath.Base(dir), ".network-toolkit-reopen-") {
		return nil, errors.New("Invalid update reopen folder")
	}
	if err = updateNamingCheckDirectory(dir); err != nil {
		return nil, err
	}
	ready := filepath.Join(dir, "ready")
	if err = wait(pid, 45*time.Second, ready); err != nil {
		return nil, err
	}
	_ = os.Remove(ready)
	_ = os.Remove(dir)
	return append([]string{args[0]}, args[4:]...), nil
}

func (a *application) reopenPromotedUpdate() error {
	if !updateInstallSupported() {
		return nil
	}
	original, err := os.Executable()
	if err != nil {
		return err
	}
	current, err := updateCurrentExecutable()
	if err != nil {
		return err
	}
	if updateNamingSamePath(original, current) {
		return nil
	}
	root, err := updateStorageRoot(filepath.Dir(a.dataPath), filepath.Dir(current))
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp(root, ".network-toolkit-reopen-")
	if err != nil {
		return err
	}
	defer os.Remove(dir)
	args := append([]string{"--update-reopen", strconv.Itoa(os.Getpid()), dir}, updateRestartArgs(os.Args[1:])...)
	cmd := exec.Command(current, args...)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd.Dir = cwd
	configureUpdateAppProcess(cmd)
	if err = cmd.Start(); err != nil {
		return err
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if b, e := updateNamingReadFile(filepath.Join(dir, "ready"), 1024); e == nil && string(b) == "ready" {
			_ = cmd.Process.Release()
			a.stop()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	_ = os.Remove(filepath.Join(dir, "ready"))
	return fmt.Errorf("The canonical reopen did not start. Close and reopen %s before using elevated tools", filepath.Base(current))
}

// Keep the first visible bootstrap window read-only until its final canonical
// reopen. This prevents starting new jobs or editing settings in a window
// that is about to close. The API also enforces the gate, not just the overlay.
func updateFinalizingHTML(html, token string) string {
	encoded, _ := json.Marshal(token)
	script := `<script>document.addEventListener('DOMContentLoaded',()=>{const cover=document.createElement('div');cover.id='updateFinalizingCover';cover.setAttribute('role','status');cover.style.cssText='position:fixed;inset:0;z-index:2147483647;display:grid;place-content:center;background:#111827;color:#fff;font:18px system-ui;text-align:center;padding:32px';cover.textContent='Finishing update… The current version will reopen shortly.';for(const el of document.body.children)el.inert=true;document.body.appendChild(cover);const timer=setInterval(async()=>{try{const r=await fetch('/api/updates/status',{headers:{'X-Session-Token':window.__networkToolkitNativeSession||` + string(encoded) + `}});if(r.ok&&!(await r.json()).finalizing){clearInterval(timer);location.reload()}}catch{}},500)})</script>`
	return strings.Replace(html, "</head>", script+"</head>", 1)
}
