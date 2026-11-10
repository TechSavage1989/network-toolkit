package main

// One-time move to shared app data (docs/SHARED_APP_DATA.md). Before sharing,
// the standalone Terminal and the standalone Text Editor each kept a copy of
// saved connections, settings, SSH host trust and macros in their own
// workspace folder. migrateSharedData adds what those copies hold to the data
// folder's shared files:
//
//   - nothing in the data folder is overwritten or removed: a saved connection
//     or macro with the same id but different content is added as a second
//     entry, a setting or a host key that differs keeps the data folder's
//     value;
//   - the workspace files are only read and stay where they are;
//   - each data-folder file is copied once, untouched, before its first change;
//   - a marker records what has been handled, so nothing is added twice and a
//     connection the user later removes does not come back. A workspace file
//     that an older version of the app changes afterwards is looked at again,
//     and only its new entries are added.
//
// The run holds the data folder's shared-data lock. Every step can be
// repeated: after a crash the next start completes it.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"networktoolkit/internal/macros"
	"networktoolkit/internal/terminal"
)

const (
	sharedMarkerFilename = "Network_Toolkit.shared-data.json"
	sharedMarkerLimit    = 1 << 20
	// Untouched copies of the data folder's files as they were before the
	// first change made for sharing.
	sharedBackupSettings   = "Network_Toolkit.before-sharing.settings.json"
	sharedBackupMacros     = "Network_Toolkit.before-sharing.macros.json"
	sharedBackupKnownHosts = "Network_Toolkit.before-sharing.ssh_known_hosts.json"
	knownHostsFilename     = "Network_Toolkit.ssh_known_hosts.json"
	knownHostsLimit        = 4 << 20
)

// A standalone workspace that kept its own copy of what is shared now.
type legacyWorkspace struct{ ID, Directory, Label string }

var legacyWorkspaces = []legacyWorkspace{
	{"terminal", terminalWorkspaceName, "Terminal"},
	{"editor", standaloneEditorDirectory, "Text Editor"},
}

type sharedMigrationSource struct {
	// Digest of the workspace data as last examined.
	Digest string `json:"digest"`
	// Entries already handled: "key\x00identity" to a digest of the entry.
	Seen map[string]string `json:"seen,omitempty"`
}

// A saved connection that came from a workspace and names a saved credential
// of that workspace's previous vault.
type sharedMigratedCredential struct {
	Source       string `json:"source"`
	Key          string `json:"key"`
	ID           string `json:"id"`
	CredentialID string `json:"credentialId"`
}

type sharedVaultImport struct {
	At       string `json:"at"`
	Imported int    `json:"imported"`
}

// Something the user should know about. It is shown until acknowledged.
type sharedNotice struct {
	ID           string   `json:"id"`
	Kind         string   `json:"kind"`
	Source       string   `json:"source"`
	Items        []string `json:"items"`
	At           string   `json:"at"`
	Acknowledged bool     `json:"acknowledged,omitempty"`
}

type sharedMarker struct {
	Version int                               `json:"version"`
	Sources map[string]*sharedMigrationSource `json:"sources"`
	// Ids given to entries that were added as a second entry: source, then
	// "key\x00old id" to the new id.
	Renamed     map[string]map[string]string `json:"renamed,omitempty"`
	Credentials []sharedMigratedCredential   `json:"credentials,omitempty"`
	Imports     map[string]sharedVaultImport `json:"imports,omitempty"`
	Notices     []sharedNotice               `json:"notices,omitempty"`
}

func readSharedMarker(root string) (*sharedMarker, error) {
	marker := &sharedMarker{Version: 1, Sources: map[string]*sharedMigrationSource{}}
	storage, err := openStorageRoot(root, false)
	if errors.Is(err, os.ErrNotExist) {
		return marker, nil
	}
	if err != nil {
		return nil, err
	}
	defer storage.Close()
	raw, err := readStorageFile(storage, sharedMarkerFilename, sharedMarkerLimit)
	if errors.Is(err, os.ErrNotExist) {
		return marker, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, marker); err != nil || marker.Version != 1 {
		return nil, errors.New("The shared-data record is not valid; nothing was changed.")
	}
	if marker.Sources == nil {
		marker.Sources = map[string]*sharedMigrationSource{}
	}
	return marker, nil
}

func writeSharedMarker(root string, marker *sharedMarker) error {
	raw, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return err
	}
	storage, err := openStorageRoot(root, true)
	if err != nil {
		return err
	}
	defer storage.Close()
	return writeStorageAtomicLimit(storage, sharedMarkerFilename, append(raw, '\n'), sharedMarkerLimit)
}

func (m *sharedMarker) source(name string) *sharedMigrationSource {
	if m.Sources[name] == nil {
		m.Sources[name] = &sharedMigrationSource{}
	}
	if m.Sources[name].Seen == nil {
		m.Sources[name].Seen = map[string]string{}
	}
	return m.Sources[name]
}

func (m *sharedMarker) notice(kind, source string, items []string) {
	if len(items) == 0 {
		return
	}
	sort.Strings(items)
	digest := sha256.Sum256([]byte(kind + "\x00" + source + "\x00" + strings.Join(items, "\x00")))
	id := hex.EncodeToString(digest[:8])
	for _, existing := range m.Notices {
		if existing.ID == id {
			return
		}
	}
	m.Notices = append(m.Notices, sharedNotice{ID: id, Kind: kind, Source: source, Items: items, At: time.Now().UTC().Format(time.RFC3339)})
}

func sharedDigest(value any) string {
	_, raw, err := sharedCanonical(value)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

// backupSharedFile keeps one untouched copy of a data-folder file from before
// the first change made for sharing. An existing copy is never replaced.
func backupSharedFile(root, name, backup string, limit int64) error {
	storage, err := openStorageRoot(root, false)
	if err != nil {
		return err
	}
	defer storage.Close()
	if _, err := storage.Lstat(backup); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	raw, err := readStorageFile(storage, name, limit)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return writeStorageAtomicLimit(storage, backup, raw, int(limit))
}

type sharedMigration struct {
	root    string
	marker  *sharedMarker
	changed bool
}

// migrateSharedData brings what standalone workspaces kept for themselves
// into the data folder's shared files. It is safe to call at every start.
func migrateSharedData(root string) error {
	found := false
	for _, workspace := range legacyWorkspaces {
		if info, err := os.Lstat(filepath.Join(root, workspace.Directory)); err == nil && info.IsDir() && !storageIsLink(info) {
			found = true
		}
	}
	if !found {
		return nil // A new data folder: there is nothing to bring over.
	}
	release, err := acquireSharedDataLock(root)
	if err != nil {
		return err
	}
	defer release()
	marker, err := readSharedMarker(root)
	if err != nil {
		return err
	}
	m := &sharedMigration{root: root, marker: marker}
	var failures []error
	for _, workspace := range legacyWorkspaces {
		dir := filepath.Join(root, workspace.Directory)
		if info, err := os.Lstat(dir); err != nil || !info.IsDir() || storageIsLink(info) {
			continue
		}
		// Each part stands alone: one that fails is reported and tried again
		// at the next start, and does not hold back the others.
		for _, step := range []func(legacyWorkspace, string) error{m.settings, m.knownHosts, m.macros} {
			if err := step(workspace, dir); err != nil {
				failures = append(failures, fmt.Errorf("%s: %w", workspace.Label, err))
			}
		}
	}
	if m.changed {
		if err := writeSharedMarker(root, marker); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func sharedMigrationCopyID(id, source string, item any) string {
	digest := sharedDigest(item)
	if len(id) > 60 {
		id = id[:60]
	}
	return id + "-" + source + "-" + digest[:8]
}

func sharedMigrationName(name, label string) string {
	suffix := " (" + label + ")"
	runes := []rune(name)
	if len(runes)+len([]rune(suffix)) > 100 {
		runes = runes[:100-len([]rune(suffix))]
	}
	return string(runes) + suffix
}

func (m *sharedMigration) settings(workspace legacyWorkspace, dir string) error {
	own, err := readRootSettings(filepath.Join(dir, appSettingsFilename))
	if err != nil || !own.exists {
		return err
	}
	keys := sharedKeysFor(true)
	legacy := own.shared(keys)
	if len(legacy) == 0 {
		return nil
	}
	source := m.marker.source(workspace.ID + "/settings")
	digest := sharedDigest(legacy)
	if source.Digest == digest {
		return nil
	}
	settingsPath := filepath.Join(m.root, appSettingsFilename)
	release, err := acquireCredentialGuardWait(m.root, sharedLockWait)
	if err != nil {
		return err
	}
	defer release()
	rootFile, err := readRootSettings(settingsPath)
	if err != nil {
		return err
	}
	current := rootFile.shared(sharedDataKeys)
	before := sharedDigest(current)
	renamed := m.marker.Renamed[workspace.ID]
	if renamed == nil {
		renamed = map[string]string{}
	}
	var conflicts, full []string
	for _, key := range keys {
		value, present := legacy[key.Name]
		if !present {
			continue
		}
		switch key.Kind {
		case sharedList:
			theirs, okTheirs, _ := sharedListOf(value, key.Identity)
			ours, okOurs, _ := sharedListOf(current[key.Name], key.Identity)
			if !okTheirs || !okOurs {
				continue
			}
			// A saved list keeps both versions of an entry that the two sides
			// changed, up to the limit of the key table.
			limit, duplicate := key.MaxSaved, key.MaxSaved > 0
			list, _ := current[key.Name].([]any)
			list = append([]any{}, list...)
			have := map[string][]byte{}
			for id, raw := range ours.raw {
				have[id] = raw
			}
			add := func(id string, item map[string]any) bool {
				if limit > 0 && len(list) >= limit {
					return false
				}
				list = append(list, item)
				have[id], _ = json.Marshal(item)
				if credential, _ := item["credentialId"].(string); credential != "" {
					m.marker.Credentials = append(m.marker.Credentials, sharedMigratedCredential{Source: workspace.ID, Key: key.Name, ID: id, CredentialID: credential})
				}
				return true
			}
			for _, id := range theirs.order {
				seenKey, itemDigest := key.Name+"\x00"+id, sharedDigest(theirs.items[id])
				if source.Seen[seenKey] == itemDigest {
					continue
				}
				item, _ := theirs.items[id].(map[string]any)
				existing, exists := have[id]
				switch {
				case exists && string(existing) == string(theirs.raw[id]):
				case exists && !duplicate:
					// Recent entries and learned names: the data folder's stays.
				case exists:
					// Both sides changed the same saved connection: keep both.
					copyID := sharedMigrationCopyID(id, workspace.ID, item)
					if _, done := have[copyID]; !done {
						copied := map[string]any{}
						for name, field := range item {
							copied[name] = field
						}
						name, _ := copied["name"].(string)
						copied["id"], copied["name"] = copyID, sharedMigrationName(name, workspace.Label)
						if key.Name == "terminalProfiles" {
							copied["nameExplicit"] = true
						}
						if !add(copyID, copied) {
							full = append(full, key.Name)
							continue
						}
					}
					renamed[key.Name+"\x00"+id] = copyID
				default:
					if !add(id, item) {
						full = append(full, key.Name)
						continue
					}
				}
				source.Seen[seenKey] = itemDigest
			}
			if key.NewestFirst {
				sort.SliceStable(list, func(i, j int) bool { return sharedLastUsed(list[i]) > sharedLastUsed(list[j]) })
			}
			if key.Limit > 0 && len(list) > key.Limit {
				list = list[:key.Limit]
			}
			if len(list) > 0 {
				current[key.Name] = list
			}
		case sharedMap:
			theirs, _ := value.(map[string]any)
			ours, _ := current[key.Name].(map[string]any)
			next := map[string]any{}
			for name, member := range ours {
				next[name] = member
			}
			for name, member := range theirs {
				seenKey, memberDigest := key.Name+"\x00"+name, sharedDigest(member)
				if source.Seen[seenKey] == memberDigest {
					continue
				}
				source.Seen[seenKey] = memberDigest
				existing, exists := next[name]
				switch {
				case !exists:
					next[name] = member
				case key.Name == "terminalSyntax" && name == "custom":
					// Rule sets the user wrote in the standalone window are kept
					// next to the data folder's own.
					next[name] = sharedMigrationRuleSets(existing, member)
				case key.Name == "sftpVerifiedHostKeys" && !sharedEqual(existing, member):
					conflicts = append(conflicts, name)
				}
			}
			if len(next) > 0 {
				current[key.Name] = next
			}
		case sharedCounter:
			if theirs, ok := sharedNumber(value); ok {
				if ours, have := sharedNumber(current[key.Name]); !have || theirs > ours {
					current[key.Name] = theirs
				}
			}
		default:
			// A setting: the data folder's value stays; a setting it does not
			// have yet is taken from the workspace.
			if _, exists := current[key.Name]; !exists {
				current[key.Name] = value
			}
		}
	}
	// The display-name counter never falls behind a record's own revision.
	counter, _ := sharedNumber(current["terminalNameRevision"])
	for _, name := range []string{"terminalProfiles", "terminalRecent"} {
		list, _ := current[name].([]any)
		for _, entry := range list {
			if row, ok := entry.(map[string]any); ok {
				if revision, ok := sharedNumber(row["nameRevision"]); ok && revision > counter {
					counter = revision
				}
			}
		}
	}
	if counter > 0 {
		current["terminalNameRevision"] = counter
	}
	current = sharedSection(current)
	if sharedDigest(current) != before {
		if err := backupSharedFile(m.root, appSettingsFilename, sharedBackupSettings, appSettingsLimit); err != nil {
			return err
		}
		next, err := rootFile.encode(current, sharedDataKeys)
		if err != nil {
			return err
		}
		if err := writeRootSettings(settingsPath, next); err != nil {
			return err
		}
	}
	if len(renamed) > 0 {
		if m.marker.Renamed == nil {
			m.marker.Renamed = map[string]map[string]string{}
		}
		m.marker.Renamed[workspace.ID] = renamed
	}
	m.marker.notice("host-key-conflict", workspace.ID, conflicts)
	m.marker.notice("limit", workspace.ID, full)
	if len(full) == 0 {
		source.Digest = digest
	}
	m.changed = true
	return nil
}

// sharedMigrationRuleSets adds the workspace's own highlighting rule sets to
// the data folder's, keeping the data folder's where both have the same id.
func sharedMigrationRuleSets(ours, theirs any) any {
	list, _ := ours.([]any)
	list = append([]any{}, list...)
	have := map[string]bool{}
	for _, entry := range list {
		if set, ok := entry.(map[string]any); ok {
			id, _ := set["id"].(string)
			have[id] = true
		}
	}
	extra, _ := theirs.([]any)
	for _, entry := range extra {
		set, ok := entry.(map[string]any)
		id, _ := set["id"].(string)
		if !ok || id == "" || have[id] || len(list) >= terminalSyntaxMaxCustom {
			continue
		}
		have[id] = true
		list = append(list, set)
	}
	return list
}

func (m *sharedMigration) knownHosts(workspace legacyWorkspace, dir string) error {
	path := filepath.Join(dir, knownHostsFilename)
	raw, err := readBoundedRegularFile(path, knownHostsLimit)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	source := m.marker.source(workspace.ID + "/known-hosts")
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	if source.Digest == digest {
		return nil
	}
	if err := backupSharedFile(m.root, knownHostsFilename, sharedBackupKnownHosts, knownHostsLimit); err != nil {
		return err
	}
	// The shared-data lock is already held for the whole run.
	imported, err := terminal.ImportKnownHosts(m.root, path, nil, func(host, fingerprint string) bool {
		return source.Seen[host] == fingerprint
	})
	if err != nil {
		return err
	}
	conflicts := []string{}
	for _, conflict := range imported.Conflicts {
		// The data folder's key stays trusted. The user is told which hosts
		// differ; the other key is never trusted silently.
		conflicts = append(conflicts, conflict.Host)
	}
	for host, fingerprint := range imported.Entries {
		source.Seen[host] = fingerprint
	}
	m.marker.notice("host-key-conflict", workspace.ID, conflicts)
	source.Digest = digest
	m.changed = true
	return nil
}

func (m *sharedMigration) macros(workspace legacyWorkspace, dir string) error {
	legacyRoot, err := openStorageRoot(dir, false)
	if err != nil {
		return err
	}
	defer legacyRoot.Close()
	raw, err := readStorageFile(legacyRoot, terminalMacroLibraryFilename, terminalMacroLibraryLimit)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	source := m.marker.source(workspace.ID + "/macros")
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	if source.Digest == digest {
		return nil
	}
	legacy, err := readTerminalMacroLibrary(legacyRoot)
	if err != nil {
		return err
	}
	storage, err := openStorageRoot(m.root, true)
	if err != nil {
		return err
	}
	defer storage.Close()
	library, err := readTerminalMacroLibrary(storage)
	if err != nil {
		return err
	}
	library.Revision = ""
	have, shortcuts := map[string]string{}, map[string]bool{}
	for _, definition := range library.Macros {
		have[definition.ID] = sharedDigest(definition)
		if definition.Shortcut != "" {
			shortcuts[definition.Shortcut] = true
		}
	}
	renamedProfiles := m.marker.Renamed[workspace.ID]
	renamedMacros := map[string]string{}
	changed := false
	add := func(definition macros.Definition) bool {
		if len(library.Macros) >= 500 {
			return false
		}
		if shortcuts[definition.Shortcut] {
			definition.Shortcut = "" // The data folder's macro keeps the shortcut.
		}
		if definition.Shortcut != "" {
			shortcuts[definition.Shortcut] = true
		}
		library.Macros = append(library.Macros, definition)
		have[definition.ID] = sharedDigest(definition)
		changed = true
		return true
	}
	full := false
	for _, definition := range legacy.Macros {
		seenKey, itemDigest := "macro\x00"+definition.ID, sharedDigest(definition)
		if source.Seen[seenKey] == itemDigest {
			if copied, ok := source.Seen["copy\x00"+definition.ID]; ok {
				renamedMacros[definition.ID] = copied
			}
			continue
		}
		existing, exists := have[definition.ID]
		switch {
		case exists && existing == itemDigest:
		case exists:
			copied := definition
			copied.ID = sharedMigrationCopyID(definition.ID, workspace.ID, definition)
			copied.Name = sharedMigrationName(definition.Name, workspace.Label)
			if _, done := have[copied.ID]; !done && !add(copied) {
				full = true
				continue
			}
			renamedMacros[definition.ID] = copied.ID
			source.Seen["copy\x00"+definition.ID] = copied.ID
		default:
			if !add(definition) {
				full = true
				continue
			}
		}
		source.Seen[seenKey] = itemDigest
	}
	bound := map[string]bool{}
	for _, binding := range library.Bindings {
		bound[binding.ProfileID+"\x00"+binding.MacroID] = true
	}
	for _, binding := range legacy.Bindings {
		seenKey := "binding\x00" + binding.ProfileID + "\x00" + binding.MacroID
		if source.Seen[seenKey] != "" {
			continue
		}
		// A startup macro follows its saved session and its macro to the ids
		// they have in the data folder.
		if id, ok := renamedProfiles["terminalProfiles\x00"+binding.ProfileID]; ok {
			binding.ProfileID = id
		}
		if id, ok := renamedMacros[binding.MacroID]; ok {
			binding.MacroID = id
		}
		if _, exists := have[binding.MacroID]; !exists {
			continue
		}
		if !bound[binding.ProfileID+"\x00"+binding.MacroID] {
			if len(library.Bindings) >= 1000 {
				full = true
				continue
			}
			library.Bindings = append(library.Bindings, binding)
			bound[binding.ProfileID+"\x00"+binding.MacroID] = true
			changed = true
		}
		source.Seen[seenKey] = "1"
	}
	if changed {
		validated, err := validateTerminalMacroLibrary(library)
		if err != nil {
			return err
		}
		if err := backupSharedFile(m.root, terminalMacroLibraryFilename, sharedBackupMacros, terminalMacroLibraryLimit); err != nil {
			return err
		}
		encoded, err := json.MarshalIndent(validated, "", "  ")
		if err != nil {
			return err
		}
		if err := writeStorageAtomicLimit(storage, terminalMacroLibraryFilename, encoded, terminalMacroLibraryLimit); err != nil {
			return err
		}
	}
	if full {
		m.marker.notice("limit", workspace.ID, []string{"macros"})
	} else {
		source.Digest = digest
	}
	m.changed = true
	return nil
}

// sharedNotices returns what the user has not acknowledged yet.
func sharedNotices(root string) []sharedNotice {
	marker, err := readSharedMarker(root)
	if err != nil {
		return nil
	}
	pending := []sharedNotice{}
	for _, notice := range marker.Notices {
		if !notice.Acknowledged {
			pending = append(pending, notice)
		}
	}
	return pending
}

func acknowledgeSharedNotices(root string, ids []string) error {
	release, err := acquireSharedDataLock(root)
	if err != nil {
		return err
	}
	defer release()
	marker, err := readSharedMarker(root)
	if err != nil {
		return err
	}
	wanted := map[string]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	changed := false
	for i := range marker.Notices {
		if wanted[marker.Notices[i].ID] && !marker.Notices[i].Acknowledged {
			marker.Notices[i].Acknowledged, changed = true, true
		}
	}
	if !changed {
		return nil
	}
	return writeSharedMarker(root, marker)
}
