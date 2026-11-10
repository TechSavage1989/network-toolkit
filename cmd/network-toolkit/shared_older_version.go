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
)

type savedListsCopy struct {
	Version int                        `json:"version"`
	Lists   map[string]json.RawMessage `json:"lists"`
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
	raw, err := readSharedFile(filepath.Join(dir, savedListsFilename), savedListsLimit)
	if err != nil {
		return nil
	}
	var kept savedListsCopy
	if json.Unmarshal(raw, &kept) != nil || kept.Version != 1 || kept.Lists == nil {
		return nil
	}
	return &kept
}

// writeSavedListsCopy stores the saved lists of a settings file that is about
// to be written. The file is left alone when it already holds them.
func writeSavedListsCopy(dir string, members map[string]json.RawMessage) error {
	kept := savedListsCopy{Version: 1, Lists: map[string]json.RawMessage{}}
	for _, key := range savedListKeys() {
		kept.Lists[key.Name] = json.RawMessage("[]")
		if raw, ok := members[key.Name]; ok {
			var list []any
			if json.Unmarshal(raw, &list) == nil && list != nil {
				kept.Lists[key.Name] = raw
			}
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
	if err != nil || !settings.exists {
		return settings, err
	}
	if _, merging := settings.members[sharedWriterField]; merging {
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

// keepMainOnlySavedLists is for a standalone app that writes a settings file
// which went missing while it ran. It knows nothing of the lists only the
// main window uses (saved targets, saved TFTP profiles); they are taken from
// the copy of the saved lists, so neither the new file nor the copy written
// with it loses them.
func keepMainOnlySavedLists(root *rootSettings, dir string) {
	kept := readSavedListsCopy(dir)
	if kept == nil {
		return
	}
	for _, key := range savedListKeys() {
		if _, present := root.members[key.Name]; key.MainOnly && !present {
			var list []any
			if raw := kept.Lists[key.Name]; json.Unmarshal(raw, &list) == nil && len(list) > 0 {
				root.members[key.Name] = raw
			}
		}
	}
}

// flushSharedNotices tells the user about saved entries that were put back.
// It takes the shared-data lock and is therefore called with no lock held.
func (a *application) flushSharedNotices() {
	a.stateMu.Lock()
	a.ensureShared()
	names, root := a.shared.pendingPutBack, a.sharedRoot()
	a.shared.pendingPutBack = nil
	a.stateMu.Unlock()
	recordSharedNotice(root, "older-version", "", names)
}

// writeSharedSettings is the one way this version writes the data folder's
// settings file: the copy of the saved lists first, then the file. The caller
// holds the data folder's settings lock and passes a file with the mark of a
// version that merges (marshalRootSettings adds it).
func writeSharedSettings(path string, raw []byte) error {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil || members == nil {
		return errors.New("Settings were not saved: they are not a JSON object.")
	}
	if _, marked := members[sharedWriterField]; !marked {
		return errors.New("Settings were not saved: the file does not say which version wrote it.")
	}
	if err := writeSavedListsCopy(filepath.Dir(path), members); err != nil {
		return fmt.Errorf("the copy of the saved lists could not be written: %w", err)
	}
	return writeRootSettings(path, raw)
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
func recordSharedNotice(root, kind, source string, items []string) {
	if len(items) == 0 {
		return
	}
	release, err := acquireSharedDataLock(root)
	if err != nil {
		return
	}
	defer release()
	marker, err := readSharedMarker(root)
	if err != nil {
		return
	}
	before := len(marker.Notices)
	marker.notice(kind, source, sharedNoticeNames(items))
	if len(marker.Notices) != before {
		_ = writeSharedMarker(root, marker)
	}
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
	if err := writeSharedSettings(path, raw); err != nil {
		return err
	}
	if len(settings.putBack) > 0 {
		m.marker.notice("older-version", "", sharedNoticeNames(settings.putBack))
		m.changed = true
	}
	return nil
}
