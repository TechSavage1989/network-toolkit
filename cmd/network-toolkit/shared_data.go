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
	// Sent by a page with its settings and never stored: for every shared key
	// the page changed, the value the page started from. The engine merges the
	// change into what is on disk now; shared keys the page does not list were
	// not changed by it and are left as they are on disk.
	sharedBaseField = "_sharedBase"
	// The request may carry the settings and those earlier values.
	appStateRequestLimit = 4 << 20
)

// How long a change waits for another app that is saving at this moment
// before it is reported as busy. A variable so that a test can shorten it.
var sharedLockWait = 5 * time.Second

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

// sharedDataKeys is the one table of everything the apps of a data folder
// share: saved data and every preference the user sets on purpose. A key that
// is not listed here belongs to one window (perWindowStateKeys).
//
// Live keys are refreshed in running pages when another app changes them.
// The other keys are read when a page starts. MainOnly keys are used by the
// main window only (diagnostics, transfer servers); they are shared between
// the portable and the installed main window, and a standalone Terminal or
// Text Editor neither receives nor writes them.
var sharedDataKeys = []sharedKey{
	// Saved connections and what makes them usable.
	{Name: "terminalProfiles", Kind: sharedList, Live: true, MaxSaved: 200},                                       // saved terminal connections (folder is a field of each)
	{Name: "terminalRecent", Kind: sharedList, Live: true, NewestFirst: true, Limit: 50},                          // recent terminal connections
	{Name: "terminalSavedManualOrder", Kind: sharedValue, Live: true},                                             // Saved uses the user's own order (the list order)
	{Name: "terminalPromptNames", Kind: sharedList, Identity: terminalPromptNameIdentity, Live: true, Limit: 200}, // device names learned from prompts, newest first
	{Name: "terminalNameRevision", Kind: sharedCounter},                                                           // display-name revision counter (kept by the engine)
	{Name: "ftpProfiles", Kind: sharedList, Live: true, MaxSaved: 200},                                            // saved FTP/FTPS connections
	{Name: "ftpRecent", Kind: sharedList, Live: true, NewestFirst: true, Limit: 50},                               // recent FTP/FTPS connections
	{Name: "sftpRecent", Kind: sharedList, Live: true, NewestFirst: true, Limit: 50},                              // recent SFTP connections
	{Name: "sftpVerifiedHostKeys", Kind: sharedMap, Live: true},                                                   // SSH fingerprints verified for the file browser

	// Appearance of every app.
	{Name: "theme", Kind: sharedValue, Live: true},
	{Name: "accentColor", Kind: sharedValue, Live: true},
	{Name: "colorPickerPreferences", Kind: sharedMap, Live: true}, // recent and saved colors
	{Name: "diagnosticColors", Kind: sharedMap, Live: true},       // good / warning / bad colors
	{Name: "resultTimePreferences", Kind: sharedMap},              // 12/24-hour clock and time zone

	// Terminal settings (Appearance, Features, Mouse & keyboard, confirmations).
	{Name: "terminalAppearance", Kind: sharedMap, Live: true},
	{Name: "terminalSyntax", Kind: sharedMap, Live: true}, // highlighting rule sets, colors, labels, switches
	{Name: "terminalHighlight", Kind: sharedValue, Live: true},
	{Name: "terminalBehavior", Kind: sharedMap, Live: true},
	{Name: "terminalInteraction", Kind: sharedMap, Live: true}, // mouse, keyboard, menu, paste review and warnings, pager
	{Name: "terminalFontSize", Kind: sharedValue, Live: true},
	{Name: "terminalMacroConfirmation", Kind: sharedValue, Live: true},
	{Name: "terminalActiveActionsPosition", Kind: sharedValue, Live: true}, // active-session bar at the top or bottom
	{Name: "terminalSidebarSide", Kind: sharedValue, Live: true},           // terminal sidebar on the left or right
	{Name: "terminalSessionConfirmDelete", Kind: sharedValue, Live: true},
	{Name: "terminalConfirmClose", Kind: sharedValue, Live: true},
	{Name: "terminalConfirmReconnectAll", Kind: sharedValue, Live: true},
	{Name: "terminalConfirmDisconnectAll", Kind: sharedValue, Live: true},
	{Name: "terminalConfirmCloseAll", Kind: sharedValue, Live: true},

	// Text Editor settings.
	{Name: "editorPreferences", Kind: sharedMap, Live: true}, // restore session, ask before close, wrap, syntax, indent, font, palette, colors, compare
	{Name: "editorFilesPreferences", Kind: sharedMap},        // file sidebar: side, panel, width, dock
	{Name: "transferExplorerSettings", Kind: sharedMap},      // file browser options

	// Used by the main window only.
	{Name: "profiles", Kind: sharedList, MainOnly: true, MaxSaved: 200},                  // saved diagnostic targets
	{Name: "savedResultFilters", Kind: sharedMap, MainOnly: true},                        // saved result filters
	{Name: "tftpProfiles", Kind: sharedList, MainOnly: true, MaxSaved: 100},              // saved TFTP transfers
	{Name: "tftpRecent", Kind: sharedList, MainOnly: true, NewestFirst: true, Limit: 50}, // recent TFTP transfers
	{Name: "transferServerSettings", Kind: sharedMap, MainOnly: true},                    // file server settings
	{Name: "tableColumns", Kind: sharedMap, MainOnly: true},                              // chosen result columns
	{Name: "testTabOrder", Kind: sharedMap, MainOnly: true},                              // the user's order of test tabs
	{Name: "overviewLayout", Kind: sharedMap, MainOnly: true},                            // customized Overview
	{Name: "graphPlotPreferences", Kind: sharedMap, MainOnly: true},                      // which plots a graph shows
	{Name: "packetMapSettings", Kind: sharedMap, MainOnly: true},                         // packet capture map options
}

// perWindowStateKeys lists every other key a page keeps: things that are
// about one window or one run, not a preference. Each window keeps its own.
// A key in neither table is treated as per-window, and
// TestSharedKeyTableClassifiesEveryPageKey fails until it is classified.
var perWindowStateKeys = []string{
	// What is open or selected in this window.
	"activePage", "utilityTab", "localTab", "fileTransferProtocol", "targetTest", "webTest", "transferModes", "configurationNetworkTab",
	"terminalSidebarTab", "terminalSidebarCollapsed", "terminalComposerCollapsed", "terminalRecentCollapsed", "terminalSavedCollapsed",
	"terminalLayout", "navigationCollapsed", "configurationSections",
	// Sizes and positions of panels in this window.
	"terminalSidebarWidth", "toolLayout", "tableWidths", "discoveryDetails", "packetDetails", "entryDetails", "transferClientLayout",
	"transferServerSidebar", "graphWorkspacePreferences", "packetGraphPreferences",
	// Work in progress: form drafts, inputs, current filters and searches.
	"terminalConnection", "remoteBrowserSettings", "forms", "target", "networkRange", "networkRangeCleared", "networkAdapter",
	"diagnosticSource", "reportLabel", "reportNotes", "subnetInput", "subnetPrefix", "subnetCompare", "bandwidth", "macInput",
	"listPreferences", "resultFilters", "graphTimeRange",
	// Per-app bookkeeping: welcome shown, update reminders, one-time upgrades.
	"firstLaunchWelcomeSeen", "appUpdatePreferences", "discoveryWorkflowVersion", "hostnameColumnsVersion",
	// The identity the credential vault is bound to (kept by the engine).
	credentialConfigIdentityField,
	// The mark of a version that merges saves (kept by the engine, see
	// shared_older_version.go).
	sharedWriterField,
}

func sharedKeyNamed(name string) (sharedKey, bool) {
	for _, key := range sharedDataKeys {
		if key.Name == name {
			return key, true
		}
	}
	return sharedKey{}, false
}

// sharedKeysFor returns the shared keys an app uses: all of them for the
// main window, all but the main-only ones for a standalone app.
func sharedKeysFor(standalone bool) []sharedKey {
	if !standalone {
		return sharedDataKeys
	}
	keys := make([]sharedKey, 0, len(sharedDataKeys))
	for _, key := range sharedDataKeys {
		if !key.MainOnly {
			keys = append(keys, key)
		}
	}
	return keys
}

// sharedSection returns the canonical shared values of a settings object,
// after the same sanitizers every settings file passes through.
func sharedSection(state map[string]any) map[string]any {
	return sharedSectionOf(state, sharedDataKeys)
}

func sharedSectionOf(state map[string]any, keys []sharedKey) map[string]any {
	picked := map[string]any{}
	for _, key := range keys {
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
	sanitizeSavedResultFilters(picked)
	sanitizeColorPickerPreferences(picked)
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

func sharedLiveValues(values map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range sharedDataKeys {
		if !key.Live {
			continue
		}
		if value, ok := values[key.Name]; ok {
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
	// The shared values this process last served to, or accepted from, its
	// page. A page that does not say what it started from (pages do, see
	// sharedBaseField) is taken to have started from these.
	last         map[string]any
	lastRevision string
	// Digest of the main settings file as last read into application.state.
	seen [32]byte
	// The settings file as last read was written by a version that does not
	// merge saves (shared_older_version.go): pages warn the user.
	older bool
	// Shared keys found in this standalone workspace's own settings file: a
	// copy from before sharing. It is kept as it is and never used.
	legacy map[string]json.RawMessage
}

func newSharedData(root, dataPath string) *sharedData {
	root = filepath.Clean(root)
	shared := &sharedData{root: root, last: map[string]any{}}
	if filepath.Clean(filepath.Dir(dataPath)) != root {
		shared.standalone, shared.rootSettings = true, filepath.Join(root, appSettingsFilename)
	}
	return shared
}

func (s *sharedData) keys() []sharedKey { return sharedKeysFor(s.standalone) }

// served records what a page was given or what was accepted from it.
func (s *sharedData) served(values map[string]any) string {
	s.last, s.lastRevision = values, sharedRevision(values)
	return s.lastRevision
}

// offered records the live values a running page was given by a poll. The
// other values are not sent to a running page, so their base stays.
func (s *sharedData) offered(values map[string]any) {
	next := map[string]any{}
	for name, value := range s.last {
		next[name] = value
	}
	for _, key := range sharedDataKeys {
		if !key.Live {
			continue
		}
		if value, ok := values[key.Name]; ok {
			next[key.Name] = value
		} else {
			delete(next, key.Name)
		}
	}
	s.last, s.lastRevision = next, sharedRevision(values)
}

// rootSettings is the main settings file with every member kept as written,
// so a standalone app that changes shared values leaves the rest untouched.
type rootSettings struct {
	raw     []byte
	exists  bool
	members map[string]json.RawMessage
	// Set by readSharedSettings: a version that does not merge wrote the file
	// last, and the names of the saved entries that were put back.
	older   bool
	putBack []string
}

// The file could be read but does not hold a settings object. Any other
// error means the file could not be read at this moment.
var errRootSettingsInvalid = errors.New("The main settings file is not valid; it was not changed.")

// readRootSettings reads a settings file as it is. Shared values of the data
// folder are read with readSharedSettings.
func readRootSettings(path string) (*rootSettings, error) {
	settings := &rootSettings{members: map[string]json.RawMessage{}}
	raw, err := readSharedFile(path, appSettingsLimit)
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return nil, err
	}
	settings.raw, settings.exists = raw, true
	if err := json.Unmarshal(raw, &settings.members); err != nil || settings.members == nil {
		return nil, errRootSettingsInvalid
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

// shared returns the canonical shared values of the file for keys.
func (s *rootSettings) shared(keys []sharedKey) map[string]any {
	names := make([]string, 0, len(keys))
	for _, key := range keys {
		names = append(names, key.Name)
	}
	return sharedSectionOf(s.state(names...), keys)
}

// encode returns the file with the shared members of keys replaced by values.
func (s *rootSettings) encode(values map[string]any, keys []sharedKey) ([]byte, error) {
	members := make(map[string]json.RawMessage, len(s.members)+len(values))
	for name, raw := range s.members {
		members[name] = raw
	}
	for _, key := range keys {
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
	return marshalRootSettings(members, s.exists)
}

// marshal returns the file with its members as they are.
func (s *rootSettings) marshal() ([]byte, error) { return marshalRootSettings(s.members, s.exists) }

func marshalRootSettings(members map[string]json.RawMessage, exists bool) ([]byte, error) {
	// Written by a version that merges saves (shared_older_version.go).
	members[sharedWriterField] = json.RawMessage("1")
	if !exists {
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
// data folder's credential transaction lock. Only writeSharedSettings calls
// it, so the copy of the saved lists is always written first.
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

// errSharedSettingsBusy is reported when another app of the data folder was
// still saving after the wait. Nothing was written; the page keeps the change
// and saves it again.
var errSharedSettingsBusy = errors.New("Settings were not saved: another Network Toolkit app is saving at this moment. Try again in a moment.")

// acquireSettingsLock takes the lock that guards the data folder's settings
// file, waiting a short time for another app that is saving.
func acquireSettingsLock(root string) (func(), error) {
	release, err := acquireCredentialGuardWait(root, sharedLockWait)
	if errors.Is(err, errCredentialGuardBusy) {
		return nil, errSharedSettingsBusy
	}
	return release, err
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
