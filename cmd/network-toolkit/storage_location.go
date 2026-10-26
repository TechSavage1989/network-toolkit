package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	storageSelectionFilename = "Network_Toolkit.storage.json"
	storageGuardFilename     = "Network_Toolkit.storage.guard"
	storageLeaseFilename     = "Network_Toolkit.storage.lease"
	storageDeleteFilename    = "Network_Toolkit.delete-pending.json"
	storageMetadataLimit     = 16 << 10
)

var ErrAppStorageBusy = errors.New("Another Network Toolkit workspace is using this data folder. Close its main and Terminal windows before changing or deleting app data.")

type appStorageConfig struct {
	RootDir    string
	DefaultDir string
	ConfigPath string
	Explicit   bool
}

type appStorageSelection struct {
	Version int    `json:"version"`
	RootDir string `json:"rootDir"`
}

type appStorageStatus struct {
	RootDir         string `json:"rootDir"`
	WorkspaceDir    string `json:"workspaceDir,omitempty"`
	DefaultDir      string `json:"defaultDir"`
	ConfigPath      string `json:"configPath"`
	Explicit        bool   `json:"explicit"`
	NextRootDir     string `json:"nextRootDir"`
	RestartRequired bool   `json:"restartRequired"`
	DeletePending   bool   `json:"deletePending"`
	CopyPending     bool   `json:"copyPending"`
}

type appStorageManager struct {
	mu            sync.Mutex
	config        appStorageConfig
	root          *storageRoot
	lease         *os.File
	nextRoot      string
	deletePending bool
	migration     *storageMigration
	closed        bool
	deletion      storageDeleteOps
}

// executableDir is deliberately not a fallback. Earlier releases wrote there;
// choosing a new per-user default must never silently copy or erase those files.
func resolveAppStorage(executableDir, override string) (appStorageConfig, error) {
	_ = executableDir
	var config appStorageConfig
	var base string
	var err error
	if runtime.GOOS == "windows" {
		base, err = os.UserCacheDir() // %LocalAppData%, not the roaming profile.
	} else {
		base, err = os.UserConfigDir()
	}
	if err != nil || base == "" {
		return config, errors.New("The operating system's per-user app-data folder is unavailable. No executable-folder fallback was used.")
	}
	config.DefaultDir = filepath.Join(base, "NetworkToolkit")
	if err := validateStorageRoot(config.DefaultDir); err != nil {
		return config, err
	}
	config.ConfigPath = filepath.Join(config.DefaultDir, storageSelectionFilename)
	config.RootDir = config.DefaultDir
	if override != "" {
		config.RootDir, err = filepath.Abs(override)
		if err != nil {
			return config, fmt.Errorf("resolve the explicit app-data folder: %w", err)
		}
		config.Explicit = true
	} else {
		root, openErr := openStorageRoot(config.DefaultDir, false)
		if openErr == nil {
			defer root.Close()
			selected, exists, readErr := readStorageSelection(root)
			if readErr != nil {
				return config, readErr
			}
			if exists {
				config.RootDir = selected
			}
		} else if !errors.Is(openErr, os.ErrNotExist) {
			return config, openErr
		}
	}
	if err := validateStorageRoot(config.RootDir); err != nil {
		return config, err
	}
	return config, nil
}

func validateStorageRoot(path string) error {
	if path == "" || len(path) > 4096 || !filepath.IsAbs(path) || filepath.Clean(path) != path ||
		strings.IndexFunc(path, func(r rune) bool { return r < 32 || r == 127 }) >= 0 {
		return errors.New("Choose an absolute app-data folder without control characters.")
	}
	volumeRoot := filepath.VolumeName(path) + string(filepath.Separator)
	if storagePathKey(path) == storagePathKey(volumeRoot) {
		return errors.New("Choose an app-data folder inside a drive, not the drive root.")
	}
	return validateStoragePlatformPath(path)
}

func storagePathKey(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

func newAppStorage(config appStorageConfig) *appStorageManager {
	return &appStorageManager{config: config, nextRoot: config.RootDir, deletion: defaultStorageDeleteOps()}
}

// Open takes a shared lease before any settings, trust, vendor database or
// WebView profile is opened. Multiple workspaces may read/write their own files;
// maintenance cannot upgrade the root lease while another process is present.
func (storage *appStorageManager) Open() error {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	if storage.closed {
		return errors.New("The app-data manager is closed.")
	}
	if storage.lease != nil {
		return nil
	}
	guards, err := acquireStorageGuards(storage.guardPaths(), true)
	if err != nil {
		return err
	}
	defer guards.Close()
	root := guards.root(storage.config.RootDir)
	lease, err := openStorageRegular(root, storageLeaseFilename, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	success := false
	defer func() {
		if !success {
			_ = lease.Close()
		}
	}()
	pending, err := readStorageDeletePending(root)
	if err != nil {
		return err
	}
	if err := lockStorageFile(lease, pending); err != nil {
		return err
	}
	if pending {
		if err := storage.deletion.cleanup(root); err != nil {
			return fmt.Errorf("Finish the requested app-data deletion before reopening: %w", err)
		}
		if err := root.Remove(storageDeleteFilename); err != nil {
			return err
		}
		if err := unlockStorageFile(lease); err != nil {
			return err
		}
		if err := lockStorageFile(lease, false); err != nil {
			return err
		}
	}
	retained, err := root.OpenRoot(".")
	if err != nil {
		return err
	}
	storage.root, storage.lease = retained, lease
	success = true
	return nil
}

func (storage *appStorageManager) guardPaths() []string {
	paths := []string{storage.config.RootDir}
	if !storage.config.Explicit {
		paths = append(paths, storage.config.DefaultDir)
	}
	return paths
}

func (storage *appStorageManager) status() appStorageStatus {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	return storage.statusLocked()
}

func (storage *appStorageManager) statusLocked() appStorageStatus {
	return appStorageStatus{
		RootDir: storage.config.RootDir, DefaultDir: storage.config.DefaultDir, ConfigPath: storage.config.ConfigPath,
		Explicit: storage.config.Explicit, NextRootDir: storage.nextRoot,
		RestartRequired: storagePathKey(storage.nextRoot) != storagePathKey(storage.config.RootDir), DeletePending: storage.deletePending,
		CopyPending: storage.migration != nil,
	}
}

func (storage *appStorageManager) changeLocation(path string, copyExisting bool) (_ appStorageStatus, _ []string, err error) {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	if storage.closed || storage.lease == nil {
		return storage.statusLocked(), nil, errors.New("The app-data folder is not open.")
	}
	if storage.deletePending {
		return storage.statusLocked(), nil, errors.New("App-data deletion is already waiting for this app to close.")
	}
	if storage.config.Explicit {
		return storage.statusLocked(), nil, errors.New("This run uses --data-dir. Relaunch without that override to change the saved app-data location.")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		path = storage.config.DefaultDir
	}
	if err := validateStorageRoot(path); err != nil {
		return storage.statusLocked(), nil, err
	}
	paths := append(storage.guardPaths(), path)
	guards, err := acquireStorageGuards(paths, false)
	if err != nil {
		return storage.statusLocked(), nil, err
	}
	defer guards.Close()
	if err := storage.upgradeLease(); err != nil {
		return storage.statusLocked(), nil, err
	}
	defer func() { err = errors.Join(err, storage.restoreSharedLease()) }()
	destination := guards.root(path)
	same := storagePathKey(path) == storagePathKey(storage.config.RootDir)
	if !same {
		lease, err := openStorageRegular(destination, storageLeaseFilename, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return storage.statusLocked(), nil, err
		}
		defer lease.Close()
		if err := lockStorageFile(lease, true); err != nil {
			return storage.statusLocked(), nil, err
		}
		defer unlockStorageFile(lease)
		if pending, err := readStorageDeletePending(destination); err != nil || pending {
			if err == nil {
				err = errors.New("The destination has an unfinished app-data deletion. Reopen it to finish cleanup before selecting it.")
			}
			return storage.statusLocked(), nil, err
		}
	}
	var copied []string
	var rollback func()
	var migration *storageMigration
	if copyExisting && !same {
		copied, rollback, err = copyStorageData(storage.root, destination)
		if err != nil {
			return storage.statusLocked(), nil, err
		}
		migration, err = recordStorageMigration(destination, path, copied)
		if err != nil {
			rollback()
			return storage.statusLocked(), nil, err
		}
	}
	selection, _ := json.Marshal(appStorageSelection{Version: 1, RootDir: path})
	if err := writeStorageAtomic(guards.root(storage.config.DefaultDir), storageSelectionFilename, append(selection, '\n')); err != nil {
		if rollback != nil {
			rollback()
		}
		return storage.statusLocked(), nil, err
	}
	storage.nextRoot = path
	storage.migration = migration
	return storage.statusLocked(), copied, nil
}

func (storage *appStorageManager) scheduleDelete() (err error) {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	if storage.closed || storage.lease == nil {
		return errors.New("The app-data folder is not open.")
	}
	if storage.deletePending {
		return nil
	}
	guards, err := acquireStorageGuards(storage.guardPaths(), false)
	if err != nil {
		return err
	}
	defer guards.Close()
	if err := storage.upgradeLease(); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, storage.restoreSharedLease()) }()
	// Validate ownership, directories and enumerated cache attributes while
	// the browser is still live. Opening a Cookies database for attributes can
	// itself fail with a sharing violation; strict file-handle inspection is
	// repeated after all windows close, before any deletion takes place.
	if err := storage.deletion.inspectLive(storage.root); err != nil {
		return err
	}
	marker := []byte("{\"version\":1,\"application\":\"NetworkToolkit\"}\n")
	if err := writeStorageAtomic(storage.root, storageDeleteFilename, marker); err != nil {
		return err
	}
	storage.deletePending = true
	return nil
}

// Close must run after every local window/view process, transport and writer has
// stopped. A failed cleanup keeps its marker; the next launch retries before it
// creates a browser profile or opens application data. Locks are never unlinked.
func (storage *appStorageManager) Close() (err error) {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	if storage.closed {
		return nil
	}
	storage.closed = true
	if storage.lease == nil {
		return nil
	}
	defer func() {
		_ = unlockStorageFile(storage.lease)
		_ = storage.lease.Close()
		_ = storage.root.Close()
		storage.lease, storage.root = nil, nil
	}()
	if !storage.deletePending && storage.migration == nil {
		return nil
	}
	paths := storage.guardPaths()
	if storage.migration != nil && !storage.deletePending {
		paths = append(paths, storage.migration.destination)
	}
	guards, err := acquireStorageGuards(paths, true)
	if err != nil {
		return err
	}
	defer guards.Close()
	if err := storage.upgradeLease(); err != nil {
		return err
	}
	if storage.deletePending {
		if err := storage.deletion.cleanup(storage.root); err != nil {
			return err
		}
		return storage.root.Remove(storageDeleteFilename)
	}
	destination := guards.root(storage.migration.destination)
	lease, err := openStorageRegular(destination, storageLeaseFilename, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lease.Close()
	if err := lockStorageFile(lease, true); err != nil {
		return err
	}
	defer unlockStorageFile(lease)
	selected, exists, err := readStorageSelection(guards.root(storage.config.DefaultDir))
	if err != nil {
		return err
	}
	if !exists || storagePathKey(selected) != storagePathKey(storage.migration.destination) {
		return nil
	}
	return refreshStorageMigration(storage.root, destination, storage.migration)
}

func (storage *appStorageManager) upgradeLease() error {
	if err := unlockStorageFile(storage.lease); err != nil {
		return err
	}
	if err := lockStorageFile(storage.lease, true); err != nil {
		return errors.Join(err, lockStorageFile(storage.lease, false))
	}
	return nil
}

func (storage *appStorageManager) restoreSharedLease() error {
	// The maintenance guard is still held, so no app process can race an
	// exclusive maintenance lock into this shared-lock transition.
	if err := unlockStorageFile(storage.lease); err != nil {
		return err
	}
	return lockStorageFile(storage.lease, false)
}

type storageGuards struct {
	roots map[string]*storageRoot
	files []*os.File
}

func (guards *storageGuards) root(path string) *storageRoot {
	return guards.roots[storagePathKey(path)]
}
func (guards *storageGuards) Close() {
	for i := len(guards.files) - 1; i >= 0; i-- {
		_ = unlockStorageFile(guards.files[i])
		_ = guards.files[i].Close()
	}
	for _, root := range guards.roots {
		_ = root.Close()
	}
}

func acquireStorageGuards(paths []string, wait bool) (*storageGuards, error) {
	deadline := time.Now().Add(3 * time.Second)
	for {
		guards, err := tryStorageGuards(paths)
		if err == nil || !wait || !errors.Is(err, ErrAppStorageBusy) || time.Now().After(deadline) {
			return guards, err
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func tryStorageGuards(paths []string) (_ *storageGuards, err error) {
	guards := &storageGuards{roots: make(map[string]*storageRoot)}
	defer func() {
		if err != nil {
			guards.Close()
		}
	}()
	unique := make(map[string]string)
	for _, path := range paths {
		unique[storagePathKey(path)] = path
	}
	keys := make([]string, 0, len(unique))
	for key := range unique {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		root, openErr := openStorageRoot(unique[key], true)
		if openErr != nil {
			return nil, openErr
		}
		guards.roots[key] = root
		file, openErr := openStorageRegular(root, storageGuardFilename, os.O_CREATE|os.O_RDWR, 0600)
		if openErr != nil {
			return nil, openErr
		}
		if lockErr := lockStorageFile(file, true); lockErr != nil {
			_ = file.Close()
			return nil, lockErr
		}
		guards.files = append(guards.files, file)
	}
	return guards, nil
}

func readStorageSelection(root *storageRoot) (string, bool, error) {
	raw, err := readStorageFile(root, storageSelectionFilename, storageMetadataLimit)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	var selection appStorageSelection
	if err := decodeStorageJSON(raw, &selection); err != nil || selection.Version != 1 {
		return "", false, errors.New("The saved app-data location is invalid. No alternate data folder was selected.")
	}
	if err := validateStorageRoot(selection.RootDir); err != nil {
		return "", false, err
	}
	return selection.RootDir, true, nil
}

func readStorageDeletePending(root *storageRoot) (bool, error) {
	raw, err := readStorageFile(root, storageDeleteFilename, storageMetadataLimit)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var marker struct {
		Version     int    `json:"version"`
		Application string `json:"application"`
	}
	if err := decodeStorageJSON(raw, &marker); err != nil || marker.Version != 1 || marker.Application != "NetworkToolkit" {
		return false, errors.New("The app-data deletion marker is invalid; no data was deleted.")
	}
	return true, nil
}

func decodeStorageJSON(raw []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("extra JSON data")
	}
	return nil
}
