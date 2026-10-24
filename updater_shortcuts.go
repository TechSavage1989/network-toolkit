package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

const updateShortcutLimit = 1 << 20
const updateShortcutScanLimit = 4096
const updateShortcutMaxDepth = 8

// Only the current user's Desktop (not its subfolders) and Start Menu are
// eligible. Pinned/taskbar shortcuts, other users, public/system-wide folders,
// and links/reparse points are deliberately not followed or modified.
type updateShortcutRoot struct {
	path  string
	depth int
}

type updateShortcutOps struct {
	// stage must load without Resolve, match an exact supplied old target, and
	// change only the target through IShellLink before saving a copy to stage.
	stage  func(source, stage, newPath string, oldPaths []string) (bool, error)
	commit func(path string, identity os.FileInfo, before, after []byte) error
}

type updateShortcutMigrationError struct {
	updated int
	failed  int
	details []string
}

func (e *updateShortcutMigrationError) add(path string, err error) {
	e.failed++
	if len(e.details) < 8 {
		e.details = append(e.details, fmt.Sprintf("%s: %v", path, err))
	}
}

func (e *updateShortcutMigrationError) Error() string {
	return fmt.Sprintf("Updated %d matching per-user shortcuts; %d shortcut/folder checks or updates could not finish. Original backups are under the settings update folder. Pinned and system-wide shortcuts are not migrated. %s", e.updated, e.failed, strings.Join(e.details, "; "))
}

// Caller supplies only old paths whose application ownership it has verified.
// This additional validation prevents widening that authority to another app,
// directory, user-renamed executable, or an arbitrary replacement target.
func updateShortcutValidatePaths(oldPaths []string, newPath, settingsRoot string) error {
	if !filepath.IsAbs(newPath) || filepath.Clean(newPath) != newPath || filepath.Base(newPath) != "Network_Toolkit.exe" {
		return errors.New("Shortcut replacement must be the stable Network_Toolkit.exe path")
	}
	if err := updateNamingCheckDirectory(filepath.Dir(newPath)); err != nil {
		return err
	}
	if err := updateNamingCheckDirectory(settingsRoot); err != nil {
		return err
	}
	info, err := os.Lstat(newPath)
	if err != nil || !info.Mode().IsRegular() || storageIsLink(info) {
		return errors.New("The stable shortcut target must already exist as a regular file")
	}
	for _, path := range oldPaths {
		_, official := updateOfficialExecutableVersion(filepath.Base(path))
		if !official || !filepath.IsAbs(path) || filepath.Clean(path) != path || !updateNamingSamePath(filepath.Dir(path), filepath.Dir(newPath)) {
			return errors.New("Shortcut migration only accepts verified official old filenames in this installation")
		}
	}
	return nil
}

func updateShortcutSnapshot(path string) ([]byte, os.FileInfo, error) {
	if err := updateNamingCheckDirectory(filepath.Dir(path)); err != nil {
		return nil, nil, err
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	data, err := updateNamingReadFile(path, updateShortcutLimit)
	if err != nil {
		return nil, nil, err
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) || storageIsLink(after) {
		return nil, nil, errors.New("Shortcut identity changed during inspection")
	}
	return data, before, nil
}

func updateShortcutBackup(settingsRoot, path string, original []byte) error {
	dir := filepath.Join(settingsRoot, "shortcut-backups")
	if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	if err := updateNamingCheckDirectory(dir); err != nil {
		return err
	}
	digest := updateHash(original)
	base := updateHash([]byte(path)) + "-" + digest
	// Never replace an existing backup or claim unrelated bytes as a backup.
	for _, file := range []struct {
		name string
		data []byte
	}{
		{base + ".lnk", original},
		{base + ".json", func() []byte {
			b, _ := json.Marshal(struct {
				Path   string `json:"path"`
				SHA256 string `json:"sha256"`
			}{path, digest})
			return b
		}()},
	} {
		target := filepath.Join(dir, file.name)
		if err := updateWriteExclusive(target, file.data, 0600); err != nil && !os.IsExist(err) {
			return err
		}
		data, err := updateNamingReadFile(target, updateShortcutLimit)
		if err != nil || !bytes.Equal(data, file.data) {
			return errors.New("The original shortcut backup could not be verified; shortcut was not changed")
		}
	}
	return nil
}

// Testable filesystem orchestration. The Windows entrypoint obtains roots from
// SHGetKnownFolderPath; no environment-variable or caller-supplied scan roots
// are accepted by the production API.
func updateMigrateShortcutsInRoots(oldPaths []string, newPath, settingsRoot string, roots []updateShortcutRoot, ops updateShortcutOps) error {
	if len(oldPaths) == 0 {
		return nil
	}
	if err := updateShortcutValidatePaths(oldPaths, newPath, settingsRoot); err != nil {
		return err
	}
	work, err := os.MkdirTemp(settingsRoot, ".shortcut-migration-")
	if err != nil {
		return err
	}
	// This directory is created exclusively by this invocation. Do not perform
	// recursive cleanup: remove only our own staging leaf and empty directory.
	staged := filepath.Join(work, "candidate.lnk")
	defer os.Remove(work)
	defer os.Remove(staged)
	report := &updateShortcutMigrationError{}
	seen := map[string]bool{}
	entries := 0
	var visit func(string, int, bool)
	visit = func(dir string, depth int, recursive bool) {
		if entries >= updateShortcutScanLimit {
			report.add(dir, errors.New("Per-user shortcut scan limit reached; remaining folders were not inspected"))
			return
		}
		if err := updateNamingCheckDirectory(dir); err != nil {
			if !os.IsNotExist(err) {
				report.add(dir, err)
			}
			return
		}
		before, err := os.Lstat(dir)
		if err != nil {
			report.add(dir, err)
			return
		}
		f, err := os.Open(dir)
		if err != nil {
			report.add(dir, err)
			return
		}
		opened, statErr := f.Stat()
		after, pathErr := os.Lstat(dir)
		if statErr != nil || pathErr != nil || storageIsLink(after) || !os.SameFile(before, opened) || !os.SameFile(opened, after) {
			_ = f.Close()
			report.add(dir, errors.New("Shortcut folder identity changed; not inspected"))
			return
		}
		// A bounded directory read avoids allocating an unbounded entry list.
		children, readErr := f.ReadDir(updateShortcutScanLimit - entries + 1)
		_ = f.Close()
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			report.add(dir, readErr)
		}
		if len(children) == 0 {
			return
		}
		for _, child := range children {
			entries++
			if entries > updateShortcutScanLimit {
				report.add(dir, errors.New("Per-user shortcut scan limit reached; remaining entries were not inspected"))
				return
			}
			path := filepath.Join(dir, child.Name())
			info, err := os.Lstat(path)
			if err != nil {
				report.add(path, err)
				continue
			}
			if storageIsLink(info) {
				if info.IsDir() || info.Mode()&os.ModeSymlink != 0 || strings.EqualFold(filepath.Ext(path), ".lnk") {
					report.add(path, errors.New("Link or reparse point was left unchanged"))
				}
				continue
			}
			if info.IsDir() {
				if depth > 0 {
					visit(path, depth-1, recursive)
				} else if recursive {
					report.add(path, errors.New("Start Menu nesting limit reached; deeper shortcuts were not inspected"))
				}
				continue
			}
			key := strings.ToLower(path)
			if !strings.EqualFold(filepath.Ext(path), ".lnk") || seen[key] {
				continue
			}
			seen[key] = true
			changed, err := updateMigrateOneShortcut(path, staged, oldPaths, newPath, settingsRoot, ops)
			if err != nil {
				report.add(path, err)
			} else if changed {
				report.updated++
			}
		}
	}
	for _, root := range roots {
		visit(root.path, root.depth, root.depth > 0)
	}
	if report.failed > 0 {
		return report
	}
	return nil
}

func updateMigrateOneShortcut(path, staged string, oldPaths []string, newPath, settingsRoot string, ops updateShortcutOps) (bool, error) {
	original, identity, err := updateShortcutSnapshot(path)
	if err != nil {
		return false, err
	}
	defer os.Remove(staged)
	matched, err := ops.stage(path, staged, newPath, oldPaths)
	if err != nil || !matched {
		return false, err
	}
	replacement, err := updateNamingReadFile(staged, updateShortcutLimit)
	if err != nil {
		return false, err
	}
	if err := updateShortcutPreservesSettingsForTarget(original, replacement, oldPaths, newPath); err != nil {
		return false, err
	}
	current, currentIdentity, err := updateShortcutSnapshot(path)
	if err != nil || !os.SameFile(identity, currentIdentity) || !bytes.Equal(original, current) {
		return false, errors.New("Shortcut changed while it was being prepared; left unchanged")
	}
	if err := updateShortcutBackup(settingsRoot, path, original); err != nil {
		return false, err
	}
	if err := ops.commit(path, identity, original, replacement); err != nil {
		return false, err
	}
	return true, nil
}

// Read just enough MS-SHLLINK structure to prove that native serialization did
// not lose customization. The native Shell API remains responsible for target
// parsing and encoding. Reject malformed or unsupported representations rather
// than recreating a shortcut with lost arguments, icon, hotkey or custom data.
// https://learn.microsoft.com/openspecs/windows_protocols/ms-shllink/747629b3-b5be-452a-8101-b9a2ec49978c
type updateShortcutSettings struct {
	flags   uint32
	header  []byte
	strings [5][]byte
	extra   []byte
}

func updateShortcutParseSettings(data []byte) (updateShortcutSettings, error) {
	var result updateShortcutSettings
	invalid := errors.New("Shortcut structure is unsupported or damaged; left unchanged")
	classID := []byte{1, 0x14, 2, 0, 0, 0, 0, 0, 0xc0, 0, 0, 0, 0, 0, 0, 0x46}
	if len(data) < 80 || binary.LittleEndian.Uint32(data) != 76 || !bytes.Equal(data[4:20], classID) {
		return result, invalid
	}
	flags := binary.LittleEndian.Uint32(data[20:24])
	// Only target structure-presence flags may change. Other execution and
	// policy flags, Unicode encoding, icon and show-state settings must survive.
	result.flags = flags &^ uint32(1|2|8|0x200)
	result.header = data[56:76]
	pos := 76
	if flags&1 != 0 {
		if pos+2 > len(data) {
			return result, invalid
		}
		pos += 2 + int(binary.LittleEndian.Uint16(data[pos:]))
	}
	if flags&2 != 0 {
		if pos+4 > len(data) {
			return result, invalid
		}
		n := uint64(binary.LittleEndian.Uint32(data[pos:]))
		if n < 0x1c || n > uint64(len(data)-pos) {
			return result, invalid
		}
		pos += int(n)
	}
	for i := range result.strings {
		if flags&(4<<i) == 0 {
			continue
		}
		if pos+2 > len(data) {
			return result, invalid
		}
		n := int(binary.LittleEndian.Uint16(data[pos:]))
		if flags&0x80 != 0 {
			n *= 2
		}
		pos += 2
		if n > len(data)-pos {
			return result, invalid
		}
		result.strings[i] = data[pos : pos+n]
		pos += n
	}
	for pos+4 <= len(data) {
		n := uint64(binary.LittleEndian.Uint32(data[pos:]))
		if n < 4 {
			if pos+4 != len(data) {
				return result, invalid
			}
			return result, nil
		}
		if n < 8 || n > uint64(len(data)-pos) {
			return result, invalid
		}
		signature := binary.LittleEndian.Uint32(data[pos+4:])
		switch signature {
		case 0xa0000001, 0xa0000003, 0xa0000005, 0xa000000b, 0xa000000c:
			// Target environment, tracking, special/known-folder and target ID
			// blocks describe the replaced target and may legitimately change.
		default:
			// Include unknown blocks as well as console, icon environment,
			// shim and property store blocks, without decoding or normalizing.
			result.extra = append(result.extra, data[pos:pos+int(n)]...)
		}
		pos += int(n)
	}
	return result, invalid
}

func updateShortcutPreservesSettings(before, after []byte) error {
	return updateShortcutPreservesSettingsForTarget(before, after, nil, "")
}

func updateShortcutPreservesSettingsForTarget(before, after []byte, oldPaths []string, newPath string) error {
	a, err := updateShortcutParseSettings(before)
	if err != nil {
		return err
	}
	b, err := updateShortcutParseSettings(after)
	if err != nil {
		return err
	}
	if a.flags != b.flags || !bytes.Equal(a.header, b.header) || !bytes.Equal(a.extra, b.extra) {
		return errors.New("Windows changed shortcut options or custom data; original was left unchanged")
	}
	for i := range a.strings {
		if i != 1 && !bytes.Equal(a.strings[i], b.strings[i]) { // relative target is not a user option
			if i == 4 && newPath != "" {
				oldIcon := updateShortcutString(a.strings[i], a.flags&0x80 != 0)
				newIcon := updateShortcutString(b.strings[i], b.flags&0x80 != 0)
				if strings.EqualFold(newIcon, newPath) {
					allowed := false
					for _, oldPath := range oldPaths {
						allowed = allowed || strings.EqualFold(oldIcon, oldPath)
					}
					if allowed {
						continue
					}
				}
			}
			return errors.New("Windows changed shortcut description, arguments, working directory or icon; original was left unchanged")
		}
	}
	return nil
}

func updateShortcutString(data []byte, unicode bool) string {
	if !unicode {
		// ANSI paths with non-ASCII characters need code-page interpretation.
		// Do not guess one when deciding whether an icon may be changed.
		for _, b := range data {
			if b >= 128 {
				return ""
			}
		}
		return string(data)
	}
	chars := make([]uint16, len(data)/2)
	for i := range chars {
		chars[i] = binary.LittleEndian.Uint16(data[i*2:])
	}
	return string(utf16.Decode(chars))
}
