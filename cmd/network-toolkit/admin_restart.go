package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Restart as administrator (owner decision after v1.21.80). Run as
// administrator in the Terminal opens a shell as an ordinary tab of a process
// that already runs as administrator. When this process does not, the page asks
// whether to restart Network Toolkit as administrator:
//
//  1. POST /api/admin-restart {"action":"start","confirm":true} starts this
//     program again through Windows' own approval ("runas", a visible normal
//     start, never a hidden helper) with --admin-reopen <pid> <folder> before
//     its usual switches (updateRestartArgs). The new copy opens this process
//     for SYNCHRONIZE, writes "ready" and waits, before any flag, settings,
//     storage lock or window, until this process has exited.
//  2. The page then closes the app normally (unsaved work is kept or asked
//     about, as on any close). If the user keeps the app open instead, the
//     page sends {"action":"cancel"}: the waiting copy sees "cancel" and exits
//     without opening anything. It also gives up after adminReopenWait.
//
// Only the main window and the standalone Terminal offer it, never a detached
// window, and only while no popped-out window of this engine is open (closing
// the main window would leave this engine running for them).
const (
	adminReopenFlag   = "--admin-reopen"
	adminReopenPrefix = ".network-toolkit-admin-reopen-"
	adminReopenWait   = 3 * time.Minute
	adminReadyWait    = 15 * time.Second
)

var (
	errAdminReopenCancelled = errors.New("the administrator restart was cancelled")
	errAdminRestartDeclined = errors.New("Windows administrator approval was cancelled. Network Toolkit keeps running as it is.")
	errAdminRestartAlready  = errors.New("Network Toolkit already runs as administrator.")
	errAdminRestartWindows  = errors.New("Close the popped-out Terminal and Text Editor windows first, then choose Restart as administrator again.")
	errAdminRestartWindow   = errors.New("Restart as administrator is available in the main window and the Terminal app.")
)

func waitAdminReopenIfRequested() error {
	args, err := prepareAdminReopen(os.Args, waitAdminParent)
	if err == nil {
		os.Args = args
	}
	return err
}

func prepareAdminReopen(args []string, wait func(int, time.Duration, string) error) ([]string, error) {
	if len(args) < 2 || args[1] != adminReopenFlag {
		return args, nil
	}
	if len(args) < 4 {
		return nil, errors.New("invalid restart request")
	}
	pid, err := strconv.Atoi(args[2])
	if err != nil || pid <= 0 {
		return nil, errors.New("invalid restart request")
	}
	dir := args[3]
	if !filepath.IsAbs(dir) || !strings.HasPrefix(filepath.Base(dir), adminReopenPrefix) {
		return nil, errors.New("invalid restart folder")
	}
	if err = updateNamingCheckDirectory(dir); err != nil {
		return nil, err
	}
	err = wait(pid, adminReopenWait, dir)
	_ = os.Remove(filepath.Join(dir, "ready"))
	_ = os.Remove(filepath.Join(dir, "cancel"))
	_ = os.Remove(dir)
	if err != nil {
		return nil, err
	}
	for _, arg := range args[4:] {
		if arg == adminReopenFlag {
			return nil, errors.New("invalid restart request")
		}
	}
	return append([]string{args[0]}, args[4:]...), nil
}

// adminRestart is the one pending restart of this process.
type adminRestart struct {
	mu      sync.Mutex
	dir     string
	running bool
}

var adminRestartState adminRestart

// adminRestartAvailable reports whether this window may offer the restart.
func (a *application) adminRestartAvailable() bool {
	return adminRestartSupported() && !processElevated() && (a.windowMode == "main" || a.windowMode == "terminal")
}

func (a *application) handleAdminRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	var body struct {
		Action  string `json:"action"`
		Confirm bool   `json:"confirm"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "Invalid restart request.")
		return
	}
	switch body.Action {
	case "cancel":
		adminRestartState.cancel()
		jsonResponse(w, http.StatusOK, map[string]any{"cancelled": true})
	case "start":
		if !body.Confirm {
			jsonError(w, http.StatusBadRequest, "Confirm the restart first.")
			return
		}
		if err := a.startAdminRestart(); err != nil {
			status := http.StatusConflict
			if errors.Is(err, errAdminRestartDeclined) {
				status = http.StatusOK
				jsonResponse(w, status, map[string]any{"started": false, "declined": true, "message": err.Error()})
				return
			}
			jsonError(w, status, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"started": true})
	default:
		jsonError(w, http.StatusBadRequest, "Invalid restart request.")
	}
}

func (a *application) startAdminRestart() error {
	switch {
	case !adminRestartSupported():
		return errors.New("Restart as administrator is available on Windows only.")
	case processElevated():
		return errAdminRestartAlready
	case a.windowMode != "main" && a.windowMode != "terminal":
		return errAdminRestartWindow
	case a.views != nil && a.views.count() > 0, a.editorViews != nil && a.editorViews.hasLiveViews():
		return errAdminRestartWindows
	}
	s := &adminRestartState
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New("A restart is already being prepared.")
	}
	s.running = true
	previous := s.dir
	s.dir = ""
	s.mu.Unlock()
	if previous != "" {
		adminRestartCancelDir(previous)
	}
	dir, err := a.launchAdminRestart()
	s.mu.Lock()
	s.running = false
	s.dir = dir
	s.mu.Unlock()
	return err
}

func (s *adminRestart) cancel() {
	s.mu.Lock()
	dir := s.dir
	s.dir = ""
	s.mu.Unlock()
	if dir != "" {
		adminRestartCancelDir(dir)
	}
}

// adminRestartCancelDir tells a waiting copy not to open. That copy removes
// the folder; one that never started leaves it to the next restart's cleanup.
func adminRestartCancelDir(dir string) {
	_ = updateWriteExclusive(filepath.Join(dir, "cancel"), []byte("cancel"), 0600)
}

// adminRestartArgs are the switches the new copy starts with: this run's own,
// without the one-shot ones an update restart leaves out too.
func adminRestartArgs(pid int, dir string) []string {
	return append([]string{adminReopenFlag, strconv.Itoa(pid), dir}, updateRestartArgs(os.Args[1:])...)
}
