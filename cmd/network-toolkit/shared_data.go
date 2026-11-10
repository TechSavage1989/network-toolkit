package main

// Data shared by every app of one data folder: the main Toolkit window
// (portable or installed), the standalone Terminal and the standalone Text
// Editor. See docs/SHARED_APP_DATA.md.
//
// The shared settings live in the data folder's main settings file, so the
// main window's file keeps the format earlier versions read. A standalone
// workspace's own settings file holds only that window's preferences.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const (
	appSettingsFilename = "Network_Toolkit.settings.json"
	appSettingsLimit    = 2 << 20
	// One OS lock serializes changes to the shared known hosts, macro library,
	// migration and editor-session adoption. The settings file itself is
	// guarded by the credential transaction lock, as in earlier versions.
	sharedDataLockFilename = "Network_Toolkit.shared-data.lock"
	// Sent by a page with its settings: the revision of the shared values the
	// page holds. Never stored.
	sharedBaseField     = "_sharedBase"
	sharedLockWait      = 5 * time.Second
	sharedSnapshotLimit = 8
)

// terminalPromptNameIdentity names a learned prompt name by its endpoint.
func terminalPromptNameIdentity(item map[string]any) string {
	kind, _ := item["type"].(string)
	if kind == "serial" {
		port, _ := item["serialPort"].(string)
		if port == "" {
			return ""
		}
		return "serial\x00" + port
	}
	host, _ := item["host"].(string)
	user, _ := item["username"].(string)
	port, ok := item["port"].(float64)
	if kind == "" || host == "" || !ok {
		return ""
	}
	return kind + "\x00" + host + "\x00" + strconv.Itoa(int(port)) + "\x00" + user
}

// sharedDataKeys is the one table of settings shared by every app of a data
// folder. Everything else in a settings file belongs to one window: layout,
// sidebar width and tab, active page, theme, terminal appearance and the
// connection form draft.
//
//	terminalProfiles          saved terminal connections (folder is a field of each)
//	terminalRecent            recent terminal connections
//	terminalSavedManualOrder  whether Saved uses the user's own order (the list order)
//	terminalPromptNames       device names learned from prompts
//	terminalNameRevision      display-name revision counter (kept by the engine)
//	ftpProfiles, ftpRecent    saved and recent FTP/FTPS connections (Terminal sidebar, Text Editor)
//	sftpRecent                recent SFTP connections (Text Editor and file browser)
//	sftpVerifiedHostKeys      SSH fingerprints the user verified for the file browser
//	editorPreferences         Text Editor preferences (read when an editor starts)
var sharedDataKeys = []sharedKey{
	{Name: "terminalProfiles", Kind: sharedList, Live: true},
	{Name: "terminalRecent", Kind: sharedList, Live: true, NewestFirst: true, Limit: 50},
	{Name: "terminalSavedManualOrder", Kind: sharedValue, Live: true},
	{Name: "terminalPromptNames", Kind: sharedList, Identity: terminalPromptNameIdentity, Live: true},
	{Name: "terminalNameRevision", Kind: sharedCounter},
	{Name: "ftpProfiles", Kind: sharedList, Live: true},
	{Name: "ftpRecent", Kind: sharedList, Live: true, NewestFirst: true, Limit: 50},
	{Name: "sftpRecent", Kind: sharedList, Live: true, NewestFirst: true, Limit: 50},
	{Name: "sftpVerifiedHostKeys", Kind: sharedMap, Live: true},
	{Name: "editorPreferences", Kind: sharedMap},
}

func sharedKeyNamed(name string) (sharedKey, bool) {
	for _, key := range sharedDataKeys {
		if key.Name == name {
			return key, true
		}
	}
	return sharedKey{}, false
}

// sharedSection returns the canonical shared values of a settings object,
// after the same sanitizers every settings file passes through.
func sharedSection(state map[string]any) map[string]any {
	picked := map[string]any{}
	for _, key := range sharedDataKeys {
		if value, ok := state[key.Name]; ok && value != nil {
			// The sanitizers expect values as decoded from JSON.
			if canonical, _, err := sharedCanonical(value); err == nil && canonical != nil {
				picked[key.Name] = canonical
			}
		}
	}
	sanitizeTerminalSettings(picked)
	sanitizeFTPProfiles(picked)
	sanitizeFTPRecent(picked)
	sanitizeTransferClientSettings(picked)
	out := map[string]any{}
	for name, value := range picked {
		if canonical, _, err := sharedCanonical(value); err == nil && canonical != nil {
			out[name] = canonical
		}
	}
	return out
}

func sharedRevision(values map[string]any) string {
	raw, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:16])
}

type sharedSnapshot struct {
	revision string
	values   map[string]any
}

func newSharedSnapshot(values map[string]any) sharedSnapshot {
	return sharedSnapshot{revision: sharedRevision(values), values: values}
}

func (s sharedSnapshot) liveValues() map[string]any {
	out := map[string]any{}
	for _, key := range sharedDataKeys {
		if !key.Live {
			continue
		}
		if value, ok := s.values[key.Name]; ok {
			out[key.Name] = value
		}
	}
	return out
}

// sharedData is one process's view of the shared settings. Its fields are
// guarded by application.stateMu.
type sharedData struct {
	root string
	// The data folder's main settings file, for a standalone workspace. The
	// main window's own settings file is the shared one (sharedSettingsPath).
	rootSettings string
	standalone   bool
	// Shared values this process served to, or accepted from, its page; the
	// newest is last. A page names the one it holds when it saves.
	snapshots []sharedSnapshot
	// For keys that pages read only at start: what the page last got or sent.
	deferred map[string]any
	// Digest of the main settings file as last read into application.state.
	seen [32]byte
	// Shared keys found in this standalone workspace's own settings file: a
	// copy from before sharing. It is kept as it is and never used.
	legacy map[string]json.RawMessage
}

func newSharedData(root, dataPath string) *sharedData {
	root = filepath.Clean(root)
	shared := &sharedData{root: root, deferred: map[string]any{}}
	if filepath.Clean(filepath.Dir(dataPath)) != root {
		shared.standalone, shared.rootSettings = true, filepath.Join(root, appSettingsFilename)
	}
	return shared
}

// latest is the newest revision this process served or accepted.
func (s *sharedData) latest() string {
	if n := len(s.snapshots); n > 0 {
		return s.snapshots[n-1].revision
	}
	return ""
}

func (s *sharedData) remember(snapshot sharedSnapshot) {
	for i, old := range s.snapshots {
		if old.revision == snapshot.revision {
			s.snapshots = append(s.snapshots[:i], s.snapshots[i+1:]...)
			break
		}
	}
	s.snapshots = append(s.snapshots, snapshot)
	if len(s.snapshots) > sharedSnapshotLimit {
		s.snapshots = append([]sharedSnapshot(nil), s.snapshots[len(s.snapshots)-sharedSnapshotLimit:]...)
	}
}

// base returns the shared values a saving page started from. A page that
// names no revision is taken to hold what this process last served or
// accepted. An unknown revision gives an empty base: nothing on disk is then
// removed by that save.
func (s *sharedData) base(revision string) map[string]any {
	var live map[string]any
	if revision == "" {
		if n := len(s.snapshots); n > 0 {
			live = s.snapshots[n-1].values
		}
	} else {
		for _, snapshot := range s.snapshots {
			if snapshot.revision == revision {
				live = snapshot.values
			}
		}
	}
	base := map[string]any{}
	for _, key := range sharedDataKeys {
		if key.Live {
			if value, ok := live[key.Name]; ok {
				base[key.Name] = value
			}
		} else if value, ok := s.deferred[key.Name]; ok {
			base[key.Name] = value
		}
	}
	return base
}

// served records what a page was given when it loaded its settings.
func (s *sharedData) served(values map[string]any) sharedSnapshot {
	snapshot := newSharedSnapshot(values)
	s.remember(snapshot)
	s.deferred = map[string]any{}
	for _, key := range sharedDataKeys {
		if value, ok := values[key.Name]; ok && !key.Live {
			s.deferred[key.Name] = value
		}
	}
	return snapshot
}

// rootSettings is the main settings file with every member kept as written,
// so a standalone app that changes shared values leaves the rest untouched.
type rootSettings struct {
	raw     []byte
	exists  bool
	members map[string]json.RawMessage
}

func readRootSettings(path string) (*rootSettings, error) {
	settings := &rootSettings{members: map[string]json.RawMessage{}}
	raw, err := readCredentialFile(path, appSettingsLimit)
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return nil, err
	}
	settings.raw, settings.exists = raw, true
	if err := json.Unmarshal(raw, &settings.members); err != nil || settings.members == nil {
		return nil, errors.New("The main settings file is not valid; it was not changed.")
	}
	return settings, nil
}

// state decodes the members a caller needs as ordinary values.
func (s *rootSettings) state(names ...string) map[string]any {
	out := map[string]any{}
	pick := func(name string) {
		if raw, ok := s.members[name]; ok {
			var value any
			if json.Unmarshal(raw, &value) == nil && value != nil {
				out[name] = value
			}
		}
	}
	if len(names) == 0 {
		for name := range s.members {
			pick(name)
		}
		return out
	}
	for _, name := range names {
		pick(name)
	}
	return out
}

func (s *rootSettings) shared() map[string]any {
	names := make([]string, 0, len(sharedDataKeys))
	for _, key := range sharedDataKeys {
		names = append(names, key.Name)
	}
	return sharedSection(s.state(names...))
}

// encode returns the file with its shared members replaced by values.
func (s *rootSettings) encode(values map[string]any) ([]byte, error) {
	members := make(map[string]json.RawMessage, len(s.members)+len(values))
	for name, raw := range s.members {
		members[name] = raw
	}
	for _, key := range sharedDataKeys {
		value, ok := values[key.Name]
		if !ok || value == nil {
			delete(members, key.Name)
			continue
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		members[key.Name] = raw
	}
	if !s.exists {
		// The main window has not saved anything yet. Without this, its first
		// start would take a file written by another app for its own and skip
		// the first-launch welcome.
		if _, ok := members["firstLaunchWelcomeSeen"]; !ok {
			members["firstLaunchWelcomeSeen"] = json.RawMessage("false")
		}
	}
	raw, err := json.MarshalIndent(members, "", "  ")
	if err != nil {
		return nil, err
	}
	if len(raw) > appSettingsLimit {
		return nil, errors.New("Saved settings would exceed the 2 MiB limit. Reduce the saved data first.")
	}
	return raw, nil
}

// writeRootSettings replaces the main settings file. The caller holds the
// data folder's credential transaction lock.
func writeRootSettings(path string, raw []byte) error {
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || storageIsLink(info) {
			return errors.New("The main settings file must be a regular file, not a link.")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return writeAtomic(path, raw)
}

var errCredentialGuardBusy = errors.New("Another Network Toolkit instance is changing credentials. Finish that action and retry.")

// acquireCredentialGuardWait waits a short time for another app that is
// saving at this moment, instead of failing at once.
func acquireCredentialGuardWait(dir string, wait time.Duration) (func(), error) {
	deadline := time.Now().Add(wait)
	for {
		release, err := acquireCredentialGuard(dir)
		if err == nil || !errors.Is(err, errCredentialGuardBusy) || time.Now().After(deadline) {
			return release, err
		}
		time.Sleep(15 * time.Millisecond)
	}
}

var sharedLockLocal sync.Map // one in-process mutex per lock file

// acquireSharedDataLock takes the data folder's shared-data lock, waiting a
// short time for another app. The lock file stays in place: ownership is an
// OS lock on it, never the presence of the file.
func acquireSharedDataLock(root string) (func(), error) {
	return acquireSharedFileLock(root, sharedDataLockFilename, sharedLockWait)
}

func acquireSharedFileLock(dir, name string, wait time.Duration) (func(), error) {
	storage, err := openStorageRoot(dir, true)
	if err != nil {
		return nil, err
	}
	defer storage.Close()
	path, err := storage.checkedPath(name, true)
	if err != nil {
		return nil, err
	}
	// Threads of one process are ordered here first, so they never spin on
	// the OS lock against each other.
	local, _ := sharedLockLocal.LoadOrStore(storagePathKey(path), &sync.Mutex{})
	mutex := local.(*sync.Mutex)
	mutex.Lock()
	file, err := openStorageRegular(storage, name, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		mutex.Unlock()
		return nil, err
	}
	deadline := time.Now().Add(wait)
	for {
		err = lockStorageFile(file, true)
		if err == nil {
			return func() { _ = unlockStorageFile(file); _ = file.Close(); mutex.Unlock() }, nil
		}
		if !errors.Is(err, ErrAppStorageBusy) || time.Now().After(deadline) {
			_ = file.Close()
			mutex.Unlock()
			if errors.Is(err, ErrAppStorageBusy) {
				return nil, errors.New("Another Network Toolkit app is changing shared data. Try again in a moment.")
			}
			return nil, fmt.Errorf("lock shared data: %w", err)
		}
		time.Sleep(15 * time.Millisecond)
	}
}

// sanitizeFTPRecent keeps only the connection fields of recent FTP/FTPS
// connections. The list is shared by every app of the data folder, so no
// caller may put anything else (least of all a secret) into it.
func sanitizeFTPRecent(state map[string]any) {
	raw, present := state["ftpRecent"]
	if !present {
		return
	}
	clean := []map[string]any{}
	seen := map[string]bool{}
	list, _ := raw.([]any)
	for _, entry := range list {
		if len(clean) >= 50 {
			break
		}
		item, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		id, okID := transferText(item["id"], 80)
		name, okName := item["name"].(string)
		host, okHost := item["host"].(string)
		username, okUser := item["username"].(string)
		port, okPort := transferPort(item["port"])
		if !okID || !okName || !okHost || !okUser || !okPort || seen[id] || len(name) > 100 || host == "" || len(host) > 253 || len(username) > 256 {
			continue
		}
		seen[id] = true
		security, _ := item["security"].(string)
		if security != "explicit" && security != "implicit" && security != "plain" {
			security = "explicit"
		}
		fingerprint, _ := item["fingerprint"].(string)
		if len(fingerprint) > 256 {
			fingerprint = ""
		}
		credentialID, _ := transferCredentialID(item["credentialId"])
		clean = append(clean, map[string]any{"id": id, "name": name, "host": host, "port": port, "username": username, "security": security, "fingerprint": fingerprint, "credentialId": credentialID, "lastUsed": transferTimestamp(item["lastUsed"])})
	}
	state["ftpRecent"] = clean
}
