package main

// Loading and saving a page's settings with the shared values of the data
// folder (docs/SHARED_APP_DATA.md). application.state is this process's view:
// its own window preferences plus the shared values as last read from disk.
import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
)

// sharedSettingsPath is the settings file that holds the shared values: the
// main window's own file, or the data folder's main file for a standalone app.
func (a *application) sharedSettingsPath() string {
	a.ensureShared()
	if a.shared.standalone {
		return a.shared.rootSettings
	}
	return a.dataPath
}

// ensureShared gives an application object built without newApplication (in
// unit tests) the main window's view of its own data folder.
func (a *application) ensureShared() {
	if a.shared == nil {
		a.shared = newSharedData(filepath.Dir(a.dataPath), a.dataPath)
		a.stateMark = sha256.Sum256(a.state)
		a.shared.served(sharedValuesOf(a.state))
		if root, err := readRootSettings(a.dataPath); err == nil {
			a.shared.seen = sha256.Sum256(root.raw)
		}
	}
}

func stateMembers(state []byte) map[string]json.RawMessage {
	members := map[string]json.RawMessage{}
	if len(state) > 0 && json.Unmarshal(state, &members) != nil || members == nil {
		return map[string]json.RawMessage{}
	}
	return members
}

func sharedValuesOf(state []byte) map[string]any {
	members := stateMembers(state)
	picked := map[string]any{}
	for _, key := range sharedDataKeys {
		if raw, ok := members[key.Name]; ok {
			var value any
			if json.Unmarshal(raw, &value) == nil && value != nil {
				picked[key.Name] = value
			}
		}
	}
	return sharedSection(picked)
}

// setStateLocked replaces the in-memory settings. Callers hold stateMu.
func (a *application) setStateLocked(state []byte) {
	a.state = state
	a.stateMark = sha256.Sum256(state)
}

// overlaySharedLocked puts shared values into the in-memory settings.
func (a *application) overlaySharedLocked(values map[string]any) {
	members := stateMembers(a.state)
	for _, key := range sharedDataKeys {
		if value, ok := values[key.Name]; ok && value != nil {
			if raw, err := json.Marshal(value); err == nil {
				members[key.Name] = raw
				continue
			}
		}
		delete(members, key.Name)
	}
	if raw, err := json.MarshalIndent(members, "", "  "); err == nil {
		a.setStateLocked(raw)
	}
}

// absorbRootLocked takes the shared values of the main settings file into
// the in-memory settings when another app changed that file. It reports
// whether the shared values changed.
func (a *application) absorbRootLocked(root *rootSettings, force bool) bool {
	digest := sha256.Sum256(root.raw)
	if !force && digest == a.shared.seen {
		return false
	}
	a.shared.seen = digest
	before := sharedRevision(sharedValuesOf(a.state))
	values := root.shared()
	if !force && before == sharedRevision(values) {
		return false
	}
	a.overlaySharedLocked(values)
	return true
}

// syncSharedLocked brings the in-memory settings up to date before they are
// read: settings replaced outside this file count as newly loaded, and shared
// values another app saved are read from disk.
func (a *application) syncSharedLocked() {
	a.ensureShared()
	if sha256.Sum256(a.state) != a.stateMark {
		a.stateMark = sha256.Sum256(a.state)
		a.shared.served(sharedValuesOf(a.state))
	}
	if root, err := readRootSettings(a.sharedSettingsPath()); err == nil {
		a.absorbRootLocked(root, false)
	}
}

// loadSharedState runs once when the application object is built.
func (a *application) loadSharedState() {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	if a.shared.standalone {
		// Shared keys still in this workspace's own file are a copy from before
		// sharing. They are kept as they are (an earlier version of the app
		// still reads them) and are not used.
		a.shared.legacy = map[string]json.RawMessage{}
		if own, err := readRootSettings(a.dataPath); err == nil {
			for _, key := range sharedDataKeys {
				if raw, ok := own.members[key.Name]; ok {
					a.shared.legacy[key.Name] = raw
				}
			}
		}
		members := stateMembers(a.state)
		for _, key := range sharedDataKeys {
			delete(members, key.Name)
		}
		if raw, err := json.MarshalIndent(members, "", "  "); err == nil {
			a.state = raw
		}
	}
	a.stateMark = sha256.Sum256(a.state)
	if root, err := readRootSettings(a.sharedSettingsPath()); err == nil {
		if a.shared.standalone {
			a.absorbRootLocked(root, true)
		} else {
			a.shared.seen = sha256.Sum256(root.raw)
		}
	} else if a.shared.standalone && a.storageError == "" {
		a.storageError = "Saved connections could not be read: " + err.Error()
	}
	a.shared.served(sharedValuesOf(a.state))
}

func (a *application) serveState(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.stateMu.Lock()
		a.syncSharedLocked()
		// What the page is given now is what its next save starts from.
		a.shared.served(sharedValuesOf(a.state))
		raw := append([]byte(nil), a.state...)
		a.stateMu.Unlock()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(raw)
	case http.MethodPut:
		a.putState(w, r)
	default:
		methodError(w)
	}
}

// sharedNameRevisionReset lets a saved connection that the page changed
// after another app removed it come back as a new record.
func sharedNameRevisionReset(state, base, theirs map[string]any) {
	ids := func(value any) map[string]bool {
		out := map[string]bool{}
		list, _ := value.([]any)
		for _, entry := range list {
			if row, ok := entry.(map[string]any); ok {
				if id, _ := row["id"].(string); id != "" {
					out[id] = true
				}
			}
		}
		return out
	}
	for _, name := range []string{"terminalProfiles", "terminalRecent"} {
		known, current := ids(base[name]), ids(theirs[name])
		list, _ := state[name].([]any)
		for _, entry := range list {
			row, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			if id, _ := row["id"].(string); known[id] && !current[id] {
				row["nameRevision"] = float64(0)
			}
		}
	}
}

func (a *application) putState(w http.ResponseWriter, r *http.Request) {
	var state map[string]any
	if err := decodeBody(w, r, appSettingsLimit, &state); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	if state == nil {
		jsonError(w, 400, "Settings must be a JSON object.")
		return
	}
	baseRevision, _ := state[sharedBaseField].(string)
	delete(state, sharedBaseField)
	// Vault config identity is native/backend-owned, never supplied by the renderer.
	delete(state, credentialConfigIdentityField)
	if err := validateSharedTypes(state); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateTerminalDisplayRevisions(state); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	sanitizeTerminalSettings(state)
	sanitizeFTPProfiles(state)
	sanitizeFTPRecent(state)
	sanitizeTransferClientSettings(state)
	sanitizeListPreferences(state)
	sanitizeSavedResultFilters(state)
	sanitizeColorPickerPreferences(state)
	sent := map[string]bool{}
	for _, key := range sharedDataKeys {
		if value, ok := state[key.Name]; ok && value != nil {
			sent[key.Name] = true
		}
	}
	ours := sharedSection(state)

	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.ensureShared()
	// One transaction for every app of this data folder: read what is on
	// disk, merge the page's changes into it and write the result.
	release, lockErr := acquireCredentialGuardWait(a.shared.root, sharedLockWait)
	if lockErr != nil {
		jsonError(w, http.StatusConflict, lockErr.Error())
		return
	}
	defer release()
	if sha256.Sum256(a.state) != a.stateMark {
		a.stateMark = sha256.Sum256(a.state)
		a.shared.served(sharedValuesOf(a.state))
	}
	root, err := readRootSettings(a.sharedSettingsPath())
	if err != nil {
		if a.shared.standalone {
			jsonError(w, http.StatusConflict, "Settings were not saved: "+err.Error())
			return
		}
		// The main window replaces its own unreadable file, as before.
		root = &rootSettings{members: map[string]json.RawMessage{}, exists: true}
	} else {
		a.absorbRootLocked(root, false)
	}
	theirs := sharedValuesOf(a.state)
	base := a.shared.base(baseRevision)
	for _, key := range sharedDataKeys {
		value, err := mergeSharedKey(key, base[key.Name], theirs[key.Name], ours[key.Name], sent[key.Name])
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		if value == nil {
			delete(state, key.Name)
		} else {
			state[key.Name] = value
		}
	}
	sharedNameRevisionReset(state, base, theirs)
	if err := reconcileTerminalDisplayNames(state, theirs); err != nil {
		jsonError(w, http.StatusConflict, err.Error())
		return
	}
	merged := sharedSection(state)
	for _, key := range sharedDataKeys {
		if value, ok := merged[key.Name]; ok {
			state[key.Name] = value
		} else {
			delete(state, key.Name)
		}
	}
	full, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		jsonError(w, 400, "Invalid settings.")
		return
	}
	failed := func(err error) {
		a.storageWritable = false
		a.storageError = err.Error()
		jsonError(w, 500, "Settings were not saved: "+err.Error())
	}
	if !a.shared.standalone {
		if id := credentialConfigID(a.sharedSettingsPath()); id != "" {
			state[credentialConfigIdentityField] = id
			if full, err = json.MarshalIndent(state, "", "  "); err != nil {
				jsonError(w, 400, "Invalid settings.")
				return
			}
		}
		if len(full) > appSettingsLimit {
			jsonError(w, 400, "Saved settings would exceed the 2 MiB limit. Reduce the saved data first.")
			return
		}
		if err = writeAtomic(a.dataPath, full); err != nil {
			failed(err)
			return
		}
		a.shared.seen = sha256.Sum256(full)
	} else {
		// This window's own file: its preferences, the untouched copy from
		// before sharing, and the identity its previous vault was bound to.
		own := map[string]any{}
		for name, value := range state {
			if _, shared := sharedKeyNamed(name); !shared {
				own[name] = value
			}
		}
		for name, raw := range a.shared.legacy {
			own[name] = raw
		}
		if id := credentialConfigID(a.dataPath); id != "" {
			own[credentialConfigIdentityField] = id
		}
		ownRaw, err := json.MarshalIndent(own, "", "  ")
		if err != nil {
			jsonError(w, 400, "Invalid settings.")
			return
		}
		if len(ownRaw) > appSettingsLimit {
			jsonError(w, 400, "Saved settings would exceed the 2 MiB limit. Reduce the saved data first.")
			return
		}
		if sharedRevision(merged) != sharedRevision(theirs) && (root.exists || !sharedValuesEmpty(merged)) {
			next, err := root.encode(merged)
			if err != nil {
				jsonError(w, 400, err.Error())
				return
			}
			if err = writeRootSettings(a.sharedSettingsPath(), next); err != nil {
				failed(err)
				return
			}
			a.shared.seen = sha256.Sum256(next)
		}
		if err = writeAtomic(a.dataPath, ownRaw); err != nil {
			failed(err)
			return
		}
	}
	a.setStateLocked(full)
	a.storageWritable = true
	a.storageError = ""
	snapshot := newSharedSnapshot(merged)
	a.shared.remember(snapshot)
	changed := false
	for _, key := range sharedDataKeys {
		if !key.Live {
			if sent[key.Name] {
				a.shared.deferred[key.Name] = ours[key.Name]
			}
			continue
		}
		if !sharedEqual(ours[key.Name], merged[key.Name]) {
			changed = true
		}
	}
	shared := map[string]any{"revision": snapshot.revision}
	if changed {
		// The page did not have everything that is saved now (another app
		// added or changed something): give it the merged values.
		shared["values"] = snapshot.liveValues()
	}
	jsonResponse(w, 200, map[string]any{"ok": true, "terminalNames": terminalDisplayNamesResponse(state), "sharedData": shared})
}

// validateSharedTypes refuses a shared value of the wrong type before the
// sanitizers see it: they would turn it into an empty list, and an empty
// list from a page means "remove everything".
func validateSharedTypes(state map[string]any) error {
	for _, key := range sharedDataKeys {
		value, ok := state[key.Name]
		if !ok || value == nil {
			continue
		}
		switch key.Kind {
		case sharedList:
			if _, ok := value.([]any); !ok {
				return fmt.Errorf("%w %s must be a list.", errSharedInvalid, key.Name)
			}
		case sharedMap:
			if _, ok := value.(map[string]any); !ok {
				return fmt.Errorf("%w %s must be an object.", errSharedInvalid, key.Name)
			}
		}
	}
	return nil
}

func sharedValuesEmpty(values map[string]any) bool {
	for _, value := range values {
		switch v := value.(type) {
		case []any:
			if len(v) > 0 {
				return false
			}
		case map[string]any:
			if len(v) > 0 {
				return false
			}
		case bool:
			if v {
				return false
			}
		case float64:
			if v != 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// updateSharedLocked changes shared values from the engine itself (not from a
// page): it reads the current values from disk, applies change and saves.
// Callers hold stateMu.
func (a *application) updateSharedLocked(change func(shared map[string]any) error) error {
	a.ensureShared()
	release, err := acquireCredentialGuardWait(a.shared.root, sharedLockWait)
	if err != nil {
		return err
	}
	defer release()
	root, err := readRootSettings(a.sharedSettingsPath())
	if err != nil {
		return err
	}
	if sha256.Sum256(a.state) != a.stateMark {
		a.stateMark = sha256.Sum256(a.state)
		a.shared.served(sharedValuesOf(a.state))
	}
	a.absorbRootLocked(root, false)
	shared := sharedValuesOf(a.state)
	if err := change(shared); err != nil {
		return err
	}
	shared = sharedSection(shared)
	// Members of the file that are not on disk yet (settings held in memory
	// only) are written with it, so the file matches this process's view.
	if !a.shared.standalone {
		for name, raw := range stateMembers(a.state) {
			if _, isShared := sharedKeyNamed(name); !isShared {
				root.members[name] = raw
			}
		}
		root.exists = true
	}
	next, err := root.encode(shared)
	if err != nil {
		return err
	}
	if err := writeRootSettings(a.sharedSettingsPath(), next); err != nil {
		return err
	}
	a.shared.seen = sha256.Sum256(next)
	if a.shared.standalone {
		a.overlaySharedLocked(shared)
	} else {
		a.setStateLocked(next)
	}
	return nil
}

// handleSharedData lets a page notice what another app saved. A page asks
// with the revision it holds and gets the current values only when they
// differ.
func (a *application) handleSharedData(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/shared-data" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		methodError(w)
		return
	}
	known := r.URL.Query().Get("revision")
	if len(known) > 64 {
		jsonError(w, http.StatusBadRequest, "Invalid shared-data revision.")
		return
	}
	a.stateMu.Lock()
	a.syncSharedLocked()
	current := newSharedSnapshot(sharedValuesOf(a.state))
	if known == "" {
		// A page that has just loaded its settings holds what was served.
		known = a.shared.latest()
	}
	response := map[string]any{"revision": current.revision}
	if known != current.revision {
		a.shared.remember(current)
		response["values"] = current.liveValues()
	}
	a.stateMu.Unlock()
	jsonResponse(w, http.StatusOK, response)
}
