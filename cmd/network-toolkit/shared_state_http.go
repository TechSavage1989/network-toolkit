package main

// Loading and saving a page's settings with the shared values of the data
// folder (docs/SHARED_APP_DATA.md). application.state is this process's view:
// its own window preferences plus the shared values as last read from disk.
import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// sharedRoot is the data folder whose shared files this app uses.
func (a *application) sharedRoot() string {
	a.ensureShared()
	if a.shared.standalone {
		return a.shared.root
	}
	return filepath.Dir(a.dataPath)
}

// ensureShared gives an application object built without newApplication (in
// unit tests) the main window's view of its own data folder.
func (a *application) ensureShared() {
	if a.shared == nil {
		a.shared = newSharedData(filepath.Dir(a.dataPath), a.dataPath)
		a.stateMark = sha256.Sum256(a.state)
		a.shared.served(a.sharedValuesLocked())
		if root, err := readSharedSettings(a.dataPath); err == nil {
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

func sharedValuesOf(state []byte, keys []sharedKey) map[string]any {
	members := stateMembers(state)
	picked := map[string]any{}
	for _, key := range keys {
		if raw, ok := members[key.Name]; ok {
			var value any
			if json.Unmarshal(raw, &value) == nil && value != nil {
				picked[key.Name] = value
			}
		}
	}
	return sharedSectionOf(picked, keys)
}

// sharedValuesLocked returns the shared values this app holds in memory.
func (a *application) sharedValuesLocked() map[string]any {
	return sharedValuesOf(a.state, a.shared.keys())
}

// setStateLocked replaces the in-memory settings. Callers hold stateMu.
func (a *application) setStateLocked(state []byte) {
	a.state = state
	a.stateMark = sha256.Sum256(state)
}

// overlaySharedLocked puts shared values into the in-memory settings.
func (a *application) overlaySharedLocked(values map[string]any) {
	members := stateMembers(a.state)
	for _, key := range a.shared.keys() {
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
	a.shared.older = root.older
	if !root.exists && !force {
		// The file is gone. That is not "another app removed everything":
		// this app keeps what it holds, and its next save writes it again.
		return false
	}
	digest := sha256.Sum256(root.raw)
	if !force && digest == a.shared.seen {
		return false
	}
	a.shared.seen = digest
	values := root.shared(a.shared.keys())
	if !force && sharedRevision(a.sharedValuesLocked()) == sharedRevision(values) {
		return false
	}
	a.overlaySharedLocked(values)
	return true
}

// noteReplacedStateLocked treats settings that were replaced outside this
// file (unit tests assign application.state directly) as newly loaded.
func (a *application) noteReplacedStateLocked() {
	if sha256.Sum256(a.state) != a.stateMark {
		a.stateMark = sha256.Sum256(a.state)
		a.shared.served(a.sharedValuesLocked())
	}
}

// syncSharedLocked brings the in-memory settings up to date before they are
// read: shared values another app saved are read from disk.
func (a *application) syncSharedLocked() {
	a.ensureShared()
	a.noteReplacedStateLocked()
	// A file that is missing or cannot be read right now changes nothing.
	if root, err := readSharedSettings(a.sharedSettingsPath()); err == nil {
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
	if root, err := readSharedSettings(a.sharedSettingsPath()); err == nil {
		// The main window holds its own file as it was read; entries an older
		// version dropped from it are put back here too.
		if a.shared.standalone || len(root.putBack) > 0 {
			a.absorbRootLocked(root, true)
		} else {
			a.shared.seen, a.shared.older = sha256.Sum256(root.raw), root.older
		}
	} else if a.shared.standalone && a.storageError == "" {
		a.storageError = "Saved connections and settings could not be read: " + err.Error()
	}
	a.shared.served(a.sharedValuesLocked())
}

func (a *application) serveState(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.stateMu.Lock()
		a.syncSharedLocked()
		// What the page is given now is what its next save starts from.
		a.shared.served(a.sharedValuesLocked())
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
	// The settings themselves keep their 2 MiB limit. A page also sends what
	// it started from for the shared keys it changed, so the request may be
	// larger than the settings it carries.
	counted := &countingBody{ReadCloser: r.Body}
	r.Body = counted
	var state map[string]any
	if err := decodeBody(w, r, appStateRequestLimit, &state); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	if state == nil {
		jsonError(w, 400, "Settings must be a JSON object.")
		return
	}
	// A page says what it started from for each shared key it changed.
	var started map[string]any
	tracked := false
	if value, present := state[sharedBaseField]; present {
		if started, tracked = value.(map[string]any); !tracked {
			jsonError(w, http.StatusBadRequest, errSharedInvalid.Error())
			return
		}
	}
	delete(state, sharedBaseField)
	size := counted.n
	if tracked {
		raw, _ := json.Marshal(state)
		size = int64(len(raw))
	}
	if size > appSettingsLimit {
		jsonError(w, 400, "Invalid JSON request: http: request body too large")
		return
	}
	// Vault config identity is native/backend-owned, never supplied by the renderer.
	delete(state, credentialConfigIdentityField)
	for _, values := range []map[string]any{state, started} {
		if err := validateSharedTypes(values); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
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

	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.ensureShared()
	keys := a.shared.keys()
	if a.shared.standalone {
		// Used by the main window only: a standalone app never writes them.
		for _, key := range sharedDataKeys {
			if key.MainOnly {
				delete(state, key.Name)
			}
		}
	}
	sent := map[string]bool{}
	for _, key := range keys {
		if value, ok := state[key.Name]; ok && value != nil {
			sent[key.Name] = true
		}
	}
	ours := sharedSectionOf(state, keys)
	// One transaction for every app of this data folder: read what is on
	// disk, merge the page's changes into it and write the result.
	release, lockErr := acquireSettingsLock(a.sharedRoot())
	if lockErr != nil {
		jsonError(w, http.StatusConflict, lockErr.Error())
		return
	}
	// Saved entries that an older version had dropped and this save wrote
	// again: the user is told once the settings lock is released.
	var putBack []string
	defer func() {
		release()
		recordSharedNotice(a.sharedRoot(), "older-version", "", putBack)
	}()
	a.noteReplacedStateLocked()
	root, err := readSharedSettings(a.sharedSettingsPath())
	switch {
	case err == nil:
		// A file that is missing is not "everything was removed": this app
		// keeps what it holds (absorbRootLocked) and writes all of it below.
		a.absorbRootLocked(root, false)
	case a.shared.standalone || !errors.Is(err, errRootSettingsInvalid):
		// A standalone app never replaces a main settings file it cannot read,
		// and no app replaces one that merely could not be read right now:
		// what another app saved a moment ago would be lost.
		jsonError(w, http.StatusConflict, "Settings were not saved: "+err.Error())
		return
	default:
		// The main window replaces its own invalid file, as before, with
		// everything it holds.
		root = &rootSettings{members: map[string]json.RawMessage{}, exists: true}
	}
	theirs := a.sharedValuesLocked()
	base := a.shared.last
	if tracked {
		base = sharedSectionOf(started, keys)
	}
	refused := map[string][]map[string]string{}
	for _, key := range keys {
		value := theirs[key.Name]
		_, changed := started[key.Name]
		if key.Kind == sharedCounter || !tracked || changed {
			// The page changed this key (or cannot say): its change is applied on
			// top of what is on disk. Otherwise what is on disk stays.
			if value, err = mergeSharedKey(key, base[key.Name], theirs[key.Name], ours[key.Name], sent[key.Name]); err != nil {
				jsonError(w, http.StatusBadRequest, err.Error())
				return
			}
			// An entry that does not fit in a full list (another app added one
			// at the same time) is not saved, and the page is told.
			var dropped []map[string]string
			if value, dropped = sharedRefuseOverflow(key, theirs[key.Name], value); len(dropped) > 0 {
				refused[key.Name] = dropped
			}
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
	merged := sharedSectionOf(state, keys)
	for _, key := range keys {
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
	const tooLarge = "Saved settings would exceed the 2 MiB limit. Reduce the saved data first."
	if !a.shared.standalone {
		if id := credentialConfigID(a.sharedSettingsPath()); id != "" {
			state[credentialConfigIdentityField] = id
		}
		// The mark of a version that merges saves is never the page's.
		state[sharedWriterField] = float64(1)
		if full, err = json.MarshalIndent(state, "", "  "); err != nil {
			jsonError(w, 400, "Invalid settings.")
			return
		}
		if len(full) > appSettingsLimit {
			jsonError(w, 400, tooLarge)
			return
		}
		if err = writeSharedSettings(a.dataPath, full); err != nil {
			failed(err)
			return
		}
		a.shared.seen, a.shared.older, putBack = sha256.Sum256(full), false, root.putBack
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
			jsonError(w, 400, tooLarge)
			return
		}
		// The main settings file is written when the shared values changed,
		// when an older version wrote it last (so that what it dropped is on
		// disk again), and when it is gone although this app holds values.
		changed := sharedRevision(merged) != sharedRevision(theirs)
		if root.exists && (changed || root.older) || !root.exists && !sharedValuesEmpty(merged) {
			next, err := root.encode(merged, keys)
			if err != nil {
				jsonError(w, 400, err.Error())
				return
			}
			if err = writeSharedSettings(a.sharedSettingsPath(), next); err != nil {
				failed(err)
				return
			}
			a.shared.seen, a.shared.older, putBack = sha256.Sum256(next), false, root.putBack
		}
		if err = writeAtomic(a.dataPath, ownRaw); err != nil {
			failed(err)
			return
		}
	}
	a.setStateLocked(full)
	a.storageWritable = true
	a.storageError = ""
	revision := a.shared.served(merged)
	shared := map[string]any{"revision": revision}
	if len(refused) > 0 {
		shared["refused"] = refused
	}
	for _, key := range keys {
		if key.Live && !sharedEqual(ours[key.Name], merged[key.Name]) {
			// The page does not hold everything that is saved now (another app
			// added or changed something): give it the merged values.
			shared["values"] = sharedLiveValues(merged)
			break
		}
	}
	jsonResponse(w, 200, map[string]any{"ok": true, "terminalNames": terminalDisplayNamesResponse(state), "sharedData": shared})
}

// validateSharedTypes refuses a shared value of the wrong type before the
// sanitizers see it: they would turn it into an empty list, and an empty
// list from a page means "remove everything".
func validateSharedTypes(state map[string]any) error {
	for _, key := range sharedDataKeys {
		value, ok := state[key.Name]
		if !ok || value == nil || key.MainOnly {
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
	release, err := acquireSettingsLock(a.sharedRoot())
	if err != nil {
		return err
	}
	var putBack []string
	defer func() {
		release()
		recordSharedNotice(a.sharedRoot(), "older-version", "", putBack)
	}()
	root, err := readSharedSettings(a.sharedSettingsPath())
	if err != nil {
		return err
	}
	a.noteReplacedStateLocked()
	a.absorbRootLocked(root, false)
	keys := a.shared.keys()
	shared := a.sharedValuesLocked()
	if err := change(shared); err != nil {
		return err
	}
	shared = sharedSectionOf(shared, keys)
	if !a.shared.standalone {
		// Settings this window holds in memory but has not written yet are
		// written with the change, so the file matches this process's view. The
		// vault's configuration identity is always the one on disk.
		for name, raw := range stateMembers(a.state) {
			if _, isShared := sharedKeyNamed(name); !isShared && name != credentialConfigIdentityField {
				root.members[name] = raw
			}
		}
		root.exists = true
	}
	next, err := root.encode(shared, keys)
	if err != nil {
		return err
	}
	if err := writeSharedSettings(a.sharedSettingsPath(), next); err != nil {
		return err
	}
	a.shared.seen, a.shared.older, putBack = sha256.Sum256(next), false, root.putBack
	if a.shared.standalone {
		a.overlaySharedLocked(shared)
	} else {
		a.setStateLocked(next)
	}
	return nil
}

// handleSharedData lets a page notice what another app saved. A page asks
// with the revision it holds and gets the current values of the keys a
// running page refreshes only when something differs.
func (a *application) handleSharedData(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/shared-data/notices" {
		// The user has read a notice of the one-time move to shared data.
		if r.Method != http.MethodPost {
			methodError(w)
			return
		}
		var request struct {
			Acknowledge []string `json:"acknowledge"`
		}
		if err := decodeBody(w, r, 4096, &request); err != nil || len(request.Acknowledge) == 0 || len(request.Acknowledge) > 32 {
			jsonError(w, http.StatusBadRequest, "Name the notices that were read.")
			return
		}
		if err := acknowledgeSharedNotices(a.sharedRoot(), request.Acknowledge); err != nil {
			jsonError(w, http.StatusConflict, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
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
	current := a.sharedValuesLocked()
	revision := sharedRevision(current)
	if known == "" {
		// A page that has just loaded its settings holds what was served.
		known = a.shared.lastRevision
	}
	response := map[string]any{"revision": revision}
	if known != revision {
		a.shared.offered(current)
		response["values"] = sharedLiveValues(current)
	}
	if a.shared.older {
		// An older version, which does not merge, saved the settings file last
		// and may still be running: the page warns the user.
		response["olderVersion"] = true
	}
	a.stateMu.Unlock()
	if a.credentials != nil {
		response["credentials"] = a.credentials.mark()
	}
	if notices := sharedNotices(a.sharedRoot()); len(notices) > 0 {
		response["notices"] = notices
	}
	jsonResponse(w, http.StatusOK, response)
}

type countingBody struct {
	io.ReadCloser
	n int64
}

func (c *countingBody) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	c.n += int64(n)
	return n, err
}
