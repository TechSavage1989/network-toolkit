package main

// Older versions on the same data folder (docs/SHARED_APP_DATA.md, "Older
// versions"). A portable copy and an installed copy are updated separately,
// so a version from before shared app data can run on a data folder this
// version uses. That version reads the settings file once at start, keeps at
// most the first 100 saved sessions and FTP connections and only the keys it
// knows, and writes all of that back with every save, without merging.
//
// Two things keep its saves from dropping saved entries for good:
//
//   - Every save of this version puts a mark (sharedWriterField) into the
//     settings file. The older version's page does not know the key and drops
//     it, so a settings file without the mark was last written by a version
//     that does not merge.
//   - Every save of this version first writes the saved lists (the lists with
//     MaxSaved in the key table) to a second file, savedListsFilename, then
//     the settings file.
//
// A reader that finds the settings file without the mark puts back every
// entry of that copy that the file no longer has. It cannot tell an entry the
// older version never saw from one its user removed there, so both come back
// (the user is told which; nothing is ever lost). A settings file with the
// mark is taken as it is, whatever the copy says: this version writes the
// whole file, so after an interrupted save the settings file is the truth.
//
// The copy is also what a settings file that went missing is written again
// from. An older version does not know the copy, so its "Delete app data"
// leaves it behind, and a copy alone cannot say whether its settings file
// was lost or deleted. The copy is therefore paired: the same id is the value
// of the file's mark, is in the copy, and is in a third place that every
// version's "Delete app data" removes (the window state of a detached view
// with a reserved name, savedListsWitnessView). A copy whose id is not in
// that third place belongs to data that was deleted and is never used. See
// docs/SHARED_APP_DATA.md for what this cannot tell apart.
import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	sharedWriterField  = "sharedDataWriter"
	savedListsFilename = "Network_Toolkit.saved-lists.json"
	savedListsLimit    = 4 << 20
	// The most entries a notice names; the rest are counted.
	sharedNoticeItems = 60
	// In a settings file that a standalone app wrote anew without a copy to
	// take the main window's keys from: those keys are absent, not removed.
	// The main window keeps what it holds and removes the note with its save.
	sharedMainOnlyAbsentField = "sharedMainOnlyAbsent"
	// Where the id that pairs the copy with its settings file is kept as
	// well: Network_Toolkit.Views/<this name>/Network_Toolkit.window.json.
	// Every version since v1.19.1 removes that file of every view with
	// "Delete app data"; no detached view has this name (theirs are random).
	savedListsWitnessView = "00000000000000000000000000000000"
	savedListsWitnessFile = "Network_Toolkit.window.json"
	// Named in a notice when only settings of the main window came back.
	sharedRecoveredSettings = "Settings of the main window"
)

type savedListsCopy struct {
	Version int                        `json:"version"`
	Lists   map[string]json.RawMessage `json:"lists"`
	// The id of the settings file this copy belongs to. A copy written by
	// v1.21.40 has none and is used as that version used it.
	Pair string `json:"pair,omitempty"`
	// Every key only the main window uses, as last written. It is read only
	// to write a settings file that is missing: a standalone app does not
	// hold these keys and would otherwise write the file without them.
	MainOnly map[string]json.RawMessage `json:"mainOnly,omitempty"`
}

// savedListKeys returns the keys whose entries the user saves on purpose.
func savedListKeys() []sharedKey {
	keys := []sharedKey{}
	for _, key := range sharedDataKeys {
		if key.Kind == sharedList && key.MaxSaved > 0 {
			keys = append(keys, key)
		}
	}
	return keys
}

// readSavedListsCopy returns the copy of the saved lists, or nil when there
// is none or it cannot be used. A damaged copy is never a reason to refuse a
// read or a save: the next save of this version writes it again.
func readSavedListsCopy(dir string) *savedListsCopy {
	kept, _ := usableSavedListsCopy(dir)
	return kept
}

// usableSavedListsCopy returns the copy when it belongs to the settings file
// of this data folder: its id is the one kept beside it. unreadable says that
// a copy is there but cannot be read. A copy that is paired with nothing
// (an older version deleted the app data) is treated as if it were absent.
func usableSavedListsCopy(dir string) (kept *savedListsCopy, unreadable bool) {
	kept, present := readSavedListsCopyState(dir)
	if kept == nil {
		return nil, present
	}
	if kept.Pair != "" && readSavedListsWitness(dir) != kept.Pair {
		return nil, false
	}
	return kept, false
}

func savedListsWitnessPath(dir string) string {
	return filepath.Join(dir, storageViewsDirectory, savedListsWitnessView, savedListsWitnessFile)
}

// readSavedListsWitness returns the id kept beside the copy, or "".
func readSavedListsWitness(dir string) string {
	raw, err := readSharedFile(savedListsWitnessPath(dir), 4096)
	var witness struct {
		SavedLists string `json:"savedLists"`
	}
	if err != nil || json.Unmarshal(raw, &witness) != nil {
		return ""
	}
	return witness.SavedLists
}

func writeSavedListsWitness(dir, id string) error {
	if readSavedListsWitness(dir) == id {
		return nil
	}
	path := savedListsWitnessPath(dir)
	folder := filepath.Dir(path)
	for _, directory := range []string{filepath.Dir(folder), folder} {
		info, err := os.Lstat(directory)
		if errors.Is(err, os.ErrNotExist) {
			if err := os.Mkdir(directory, 0700); err != nil && !errors.Is(err, os.ErrExist) {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() || storageIsLink(info) {
			return errors.New("The folder of window states must be a folder, not a link.")
		}
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || storageIsLink(info) {
			return errors.New("The id of the saved lists must be a regular file, not a link.")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	raw, err := json.Marshal(map[string]string{"savedLists": id})
	if err != nil {
		return err
	}
	return writeAtomic(path, raw)
}

func removeSavedListsCopy(dir string) error {
	path := filepath.Join(dir, savedListsFilename)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || storageIsLink(info) {
		return errors.New("The copy of the saved lists must be a regular file, not a link.")
	}
	return os.Remove(path)
}

// sharedLineageOf returns the id in a settings file's mark, or "".
func sharedLineageOf(members map[string]json.RawMessage) string {
	var id string
	if json.Unmarshal(members[sharedWriterField], &id) != nil {
		return ""
	}
	return id
}

// savedListNames returns the names of all entries in the saved lists.
func savedListNames(members map[string]json.RawMessage) []string {
	var names []string
	for _, key := range savedListKeys() {
		var list []map[string]any
		if json.Unmarshal(members[key.Name], &list) == nil {
			for _, item := range list {
				name, _ := item["name"].(string)
				names = append(names, name)
			}
		}
	}
	return names
}

// readSavedListsCopyState also says whether a copy is there at all.
func readSavedListsCopyState(dir string) (kept *savedListsCopy, present bool) {
	raw, err := readSharedFile(filepath.Join(dir, savedListsFilename), savedListsLimit)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false
	}
	if err != nil {
		return nil, true
	}
	var copied savedListsCopy
	if json.Unmarshal(raw, &copied) != nil || copied.Version != 1 || copied.Lists == nil {
		return nil, true
	}
	return &copied, true
}

// writeSavedListsCopy stores the saved lists of a settings file that is about
// to be written. The file is left alone when it already holds them.
func writeSavedListsCopy(dir string, members map[string]json.RawMessage, pair string) error {
	kept := savedListsCopy{Version: 1, Lists: map[string]json.RawMessage{}, Pair: pair}
	for _, key := range savedListKeys() {
		kept.Lists[key.Name] = json.RawMessage("[]")
		if raw, ok := members[key.Name]; ok {
			var list []any
			if json.Unmarshal(raw, &list) == nil && list != nil {
				kept.Lists[key.Name] = raw
			}
		}
	}
	for _, key := range sharedDataKeys {
		if raw, ok := members[key.Name]; ok && key.MainOnly {
			if kept.MainOnly == nil {
				kept.MainOnly = map[string]json.RawMessage{}
			}
			kept.MainOnly[key.Name] = raw
		}
	}
	next, err := json.Marshal(kept)
	if err != nil {
		return err
	}
	if len(next) > savedListsLimit {
		return errors.New("The saved lists exceed their size limit.")
	}
	path := filepath.Join(dir, savedListsFilename)
	if current, err := readSharedFile(path, savedListsLimit); err == nil && bytes.Equal(current, next) {
		return nil
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || storageIsLink(info) {
			return errors.New("The copy of the saved lists must be a regular file, not a link.")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return writeAtomic(path, next)
}

// restoreSavedList returns current with every entry of kept that it no
// longer has, and the names of those entries. An entry is the same when it
// has the same id, or (for an entry without a usable id) the same content.
// What the settings file has always wins; entries that are put back follow.
func restoreSavedList(current, kept json.RawMessage) (json.RawMessage, []string) {
	var have, copied []any
	if len(current) > 0 {
		_ = json.Unmarshal(current, &have)
	}
	if json.Unmarshal(kept, &copied) != nil {
		return current, nil
	}
	ids, contents := map[string]bool{}, map[string]bool{}
	note := func(entry any) {
		if item, ok := entry.(map[string]any); ok {
			if id := sharedItemID(item); id != "" {
				ids[id] = true
			}
		}
		if _, raw, err := sharedCanonical(entry); err == nil {
			contents[string(raw)] = true
		}
	}
	for _, entry := range have {
		note(entry)
	}
	out := append([]any{}, have...)
	var names []string
	for _, entry := range copied {
		item, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		_, raw, err := sharedCanonical(entry)
		if id := sharedItemID(item); err != nil || id != "" && ids[id] || contents[string(raw)] {
			continue
		}
		note(entry)
		out = append(out, entry)
		name, _ := item["name"].(string)
		names = append(names, name)
	}
	if len(names) == 0 {
		return current, nil
	}
	next, err := json.Marshal(out)
	if err != nil {
		return current, nil
	}
	return next, names
}

// readSharedSettings reads the data folder's settings file the way every
// reader of shared values must: when a version that does not merge wrote it
// last, the saved entries it dropped are put back (in memory; the next save
// of this version writes them).
func readSharedSettings(path string) (*rootSettings, error) {
	settings, err := readRootSettings(path)
	if err != nil {
		return settings, err
	}
	if !settings.exists {
		// The file is missing. If its copy exists, the file was lost (deleting
		// app data removes both, and what an older version's deletion leaves
		// is not this file's copy any more): the saved lists and the
		// main-window keys of the copy are what a writer starts from, so that
		// no app writes the file, and then the copy, without them.
		kept, unreadable := usableSavedListsCopy(filepath.Dir(path))
		settings.copyUnreadable = unreadable
		if kept != nil {
			settings.recovered, settings.copy = true, kept
			for name, raw := range kept.MainOnly {
				if key, ok := sharedKeyNamed(name); ok && key.MainOnly {
					settings.members[name] = raw
				}
			}
			for _, key := range savedListKeys() {
				var list []any
				if raw := kept.Lists[key.Name]; json.Unmarshal(raw, &list) == nil && len(list) > 0 {
					settings.members[key.Name] = raw
				}
			}
		}
		return settings, nil
	}
	if _, merging := settings.members[sharedWriterField]; merging {
		settings.lineage = sharedLineageOf(settings.members)
		return settings, nil
	}
	kept := readSavedListsCopy(filepath.Dir(path))
	if kept == nil {
		// No version that merges has saved here yet: nothing to put back.
		return settings, nil
	}
	settings.older = true
	for _, key := range savedListKeys() {
		if raw, ok := kept.Lists[key.Name]; ok {
			if next, names := restoreSavedList(settings.members[key.Name], raw); len(names) > 0 {
				settings.members[key.Name] = next
				settings.putBack = append(settings.putBack, names...)
			}
		}
	}
	return settings, nil
}

// digest identifies what a reader of the settings file gets. For a file with
// the mark that is the file. For a file an older version wrote it is the
// file together with the saved lists after entries were put back: the older
// version can write the very same bytes again while the copy of the saved
// lists has changed in between, and the result is then a different one.
func (s *rootSettings) digest() [32]byte {
	if !s.older {
		return sha256.Sum256(s.raw)
	}
	hash := sha256.New()
	hash.Write(s.raw)
	for _, key := range savedListKeys() {
		hash.Write([]byte{0})
		hash.Write(s.members[key.Name])
	}
	var out [32]byte
	copy(out[:], hash.Sum(nil))
	return out
}

// sharedNamesWritten returns those of names that a settings file which was
// just written really holds in its saved lists, so a notice never says that
// an entry was put back when it was not.
func sharedNamesWritten(raw []byte, names []string) []string {
	if len(names) == 0 {
		return nil
	}
	var members map[string]json.RawMessage
	if json.Unmarshal(raw, &members) != nil {
		return nil
	}
	held := map[string]int{}
	for _, key := range savedListKeys() {
		var list []map[string]any
		if json.Unmarshal(members[key.Name], &list) == nil {
			for _, item := range list {
				name, _ := item["name"].(string)
				held[name]++
			}
		}
	}
	var out []string
	for _, name := range names {
		if held[name] > 0 {
			held[name]--
			out = append(out, name)
		}
	}
	return out
}

// recreateRootLocked is what every writer calls before it writes a settings
// file that does not exist (root.exists is false). readSharedSettings has
// filled root from the copy, and absorbRootLocked has taken the copy into
// what this app holds; this adds everything the app holds. A standalone app
// holds nothing of what only the main window uses. With a copy, those keys
// come from there. Without one it still writes what it holds (refusing would
// leave that nowhere but in its memory) and notes in the file that the
// main-window keys are absent, not removed: a main window that is running
// keeps its own. The vault's identity is never carried over: a file that was
// gone is a reset. Callers hold stateMu and the settings lock.
func (a *application) recreateRootLocked(root *rootSettings) {
	if root.exists {
		return
	}
	for name, raw := range stateMembers(a.state) {
		key, shared := sharedKeyNamed(name)
		if name == credentialConfigIdentityField || name == sharedMainOnlyAbsentField || a.shared.standalone && (!shared || key.MainOnly) {
			continue
		}
		root.members[name] = raw
	}
	if a.shared.standalone && root.copy == nil {
		root.members[sharedMainOnlyAbsentField] = json.RawMessage("1")
	}
}

// wrote records that this app has just written the settings file: what it
// holds is what the file holds. recreated says that the file did not exist.
func (a *application) wroteSharedLocked(written []byte, root *rootSettings, recreated bool) {
	shared := a.shared
	shared.seen, shared.older, shared.fromOlder = sha256.Sum256(written), false, false
	shared.had, shared.lost, shared.dirty = true, false, false
	var members map[string]json.RawMessage
	if json.Unmarshal(written, &members) == nil {
		shared.lineage = sharedLineageOf(members)
	}
	shared.pendingPutBack = append(shared.pendingPutBack, sharedNamesWritten(written, root.putBack)...)
	if recreated {
		names := sharedNamesWritten(written, shared.recovering)
		if len(names) == 0 && root.copy != nil && len(root.copy.MainOnly) > 0 && !shared.standalone {
			names = []string{sharedRecoveredSettings}
		}
		shared.pendingRecovered = append(shared.pendingRecovered, names...)
	}
	shared.recovering = nil
}

// flushSharedNotices tells the user about saved entries that were put back
// after an older version dropped them, and about what came back from the
// copy after the settings file was lost. It takes the shared-data lock and
// is therefore called with no lock held.
func (a *application) flushSharedNotices() {
	a.stateMu.Lock()
	a.ensureShared()
	names, recovered, root := a.shared.pendingPutBack, a.shared.pendingRecovered, a.sharedRoot()
	a.shared.pendingPutBack, a.shared.pendingRecovered = nil, nil
	a.stateMu.Unlock()
	if !recordSharedNotice(root, "older-version", "", names) {
		// Not recorded (the lock was busy, or the record could not be
		// written): the names are kept for the next attempt.
		a.stateMu.Lock()
		a.shared.pendingPutBack = append(names, a.shared.pendingPutBack...)
		a.stateMu.Unlock()
	}
	if !recordSharedNotice(root, "recovered", "", recovered) {
		a.stateMu.Lock()
		a.shared.pendingRecovered = append(recovered, a.shared.pendingRecovered...)
		a.stateMu.Unlock()
	}
}

// writeSharedSettings is the one way this version writes the data folder's
// settings file: the id that pairs the copy with the file, the copy of the
// saved lists, then the file. The caller holds the data folder's settings
// lock and passes a file with the mark of a version that merges
// (marshalRootSettings adds it). The id becomes the value of the mark, so
// what is returned, the file as written, differs from raw in that member.
//
// The id stays the same for as long as the copy and the id beside it are
// both there (v1.21.40 writes the copy without the id; the one beside it is
// then used again). Otherwise (a new data folder, a copy that is gone, app
// data that an older version deleted) the file gets a new one, and every
// running app sees that this is not the file it knew.
func writeSharedSettings(path string, raw []byte) ([]byte, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil || members == nil {
		return nil, errors.New("Settings were not saved: they are not a JSON object.")
	}
	if _, marked := members[sharedWriterField]; !marked {
		return nil, errors.New("Settings were not saved: the file does not say which version wrote it.")
	}
	dir := filepath.Dir(path)
	pair := ""
	if kept, _ := readSavedListsCopyState(dir); kept != nil {
		if beside := readSavedListsWitness(dir); kept.Pair == "" || kept.Pair == beside {
			pair = beside
		}
	}
	if pair == "" {
		pair = randomHex(16)
	}
	// First the id beside the copy: a copy with an id that is nowhere else
	// would not be used.
	if err := writeSavedListsWitness(dir, pair); err != nil {
		// It cannot be kept here. The copy is then written without an id
		// and used as v1.21.40 used it, rather than not at all.
		pair = ""
	}
	members[sharedWriterField] = json.RawMessage("1")
	if pair != "" {
		members[sharedWriterField], _ = json.Marshal(pair)
	}
	written, err := json.MarshalIndent(members, "", "  ")
	if err != nil {
		return nil, err
	}
	if len(written) > appSettingsLimit {
		return nil, errors.New("Saved settings would exceed the 2 MiB limit. Reduce the saved data first.")
	}
	if err := writeSavedListsCopy(dir, members, pair); err != nil {
		return nil, fmt.Errorf("the copy of the saved lists could not be written: %w", err)
	}
	if err := writeRootSettings(path, written); err != nil {
		return nil, err
	}
	return written, nil
}

// sharedNoticeNames shortens a long list of names for a notice.
func sharedNoticeNames(names []string) []string {
	if len(names) <= sharedNoticeItems {
		return names
	}
	return append(append([]string{}, names[:sharedNoticeItems]...), fmt.Sprintf("… and %d more", len(names)-sharedNoticeItems))
}

// recordSharedNotice adds a notice for the user. It takes the shared-data
// lock, so the caller must not hold the settings lock.
func recordSharedNotice(root, kind, source string, items []string) bool {
	if len(items) == 0 {
		return true
	}
	release, err := acquireSharedDataLock(root)
	if err != nil {
		return false
	}
	defer release()
	marker, err := readSharedMarker(root)
	if err != nil {
		return false
	}
	before := len(marker.Notices)
	marker.notice(kind, source, sharedNoticeNames(append([]string{}, items...)))
	if len(marker.Notices) != before {
		return writeSharedMarker(root, marker) == nil
	}
	return true
}

// sharedSettingsNeedRepair reports, without a lock, whether a version that
// does not merge wrote the settings file since this version last saved.
func sharedSettingsNeedRepair(root string) bool {
	settings, err := readSharedSettings(filepath.Join(root, appSettingsFilename))
	return err == nil && settings.older
}

// olderVersion runs at every start, before any page is served: a settings
// file that an older version wrote last is written again by this version,
// with the saved entries it dropped, and the user is told which those were.
func (m *sharedMigration) olderVersion() error {
	path := filepath.Join(m.root, appSettingsFilename)
	release, err := acquireCredentialGuardWait(m.root, sharedLockWait)
	if err != nil {
		return err
	}
	defer release()
	settings, err := readSharedSettings(path)
	if err != nil || !settings.older {
		return err
	}
	raw, err := settings.marshal()
	if err != nil {
		return err
	}
	if _, err := writeSharedSettings(path, raw); err != nil {
		return err
	}
	if len(settings.putBack) > 0 {
		m.marker.notice("older-version", "", sharedNoticeNames(settings.putBack))
		m.changed = true
	}
	return nil
}

// savedListsCopyNeedsSettling reports, without a lock, whether the copy of
// the saved lists is not paired with the settings file as it should be.
func savedListsCopyNeedsSettling(root string) bool {
	kept, _ := readSavedListsCopyState(root)
	return kept != nil && kept.Pair != "" && readSavedListsWitness(root) != kept.Pair
}

// settleSavedListsCopy runs at every start, before any page is served, for a
// copy whose id is not kept beside it.
//
//   - The settings file carries the same id: the data folder was moved or
//     restored without the folder of window states. The id is written again.
//   - Otherwise an older version deleted the app data (it removes the
//     settings file and the id, and does not know the copy). The copy holds
//     deleted data: it is removed, which completes the deletion.
func (m *sharedMigration) settleSavedListsCopy() error {
	release, err := acquireCredentialGuardWait(m.root, sharedLockWait)
	if err != nil {
		return err
	}
	defer release()
	kept, _ := readSavedListsCopyState(m.root)
	if kept == nil || kept.Pair == "" || readSavedListsWitness(m.root) == kept.Pair {
		return nil
	}
	settings, err := readRootSettings(filepath.Join(m.root, appSettingsFilename))
	if err != nil {
		return err
	}
	if settings.exists && sharedLineageOf(settings.members) == kept.Pair {
		return writeSavedListsWitness(m.root, kept.Pair)
	}
	return removeSavedListsCopy(m.root)
}
