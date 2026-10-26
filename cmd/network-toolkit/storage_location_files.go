package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	storageTerminalDirectory = "Network_Toolkit.Terminal"
	storageViewsDirectory    = "Network_Toolkit.Views"
	storageBrowserDirectory  = "Network_Toolkit.WebView2"
	storageTreeEntryLimit    = 200000
)

var storagePortableFiles = []struct {
	name  string
	limit int64
}{
	{"Network_Toolkit.settings.json", 2 << 20},
	{terminalMacroLibraryFilename, terminalMacroLibraryLimit},
	{credentialVaultFilename, 2 << 20},
	{"Network_Toolkit.ssh_known_hosts.json", 4 << 20},
	{"Network_Toolkit.vendors.json.gz", 24 << 20},
	{"Network_Toolkit.window.json", 64 << 10},
}

// Open each directory component through an anchored os.Root. No symlink or
// Windows reparse point is accepted, including in the selected root's ancestry.
// SameFile checks bind the inspected entry to the opened directory/file.
func openStorageRoot(path string, create bool) (*storageRoot, error) {
	if err := validateStorageRoot(path); err != nil {
		return nil, err
	}
	volumeRoot := filepath.VolumeName(path) + string(filepath.Separator)
	relative, err := filepath.Rel(volumeRoot, path)
	if err != nil {
		return nil, err
	}
	root, err := openStorageRootBase(volumeRoot)
	if err != nil {
		return nil, err
	}
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if create {
			if _, err := root.Lstat(component); errors.Is(err, os.ErrNotExist) {
				if err := root.Mkdir(component, 0700); err != nil && !errors.Is(err, os.ErrExist) {
					_ = root.Close()
					return nil, err
				}
			}
		}
		next, err := openStorageChild(root, component)
		_ = root.Close()
		if err != nil {
			return nil, fmt.Errorf("open the app-data folder: %w", err)
		}
		root = next
	}
	return root, nil
}

func openStorageChild(parent *storageRoot, name string) (*storageRoot, error) {
	before, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.IsDir() || storageIsLink(before) {
		return nil, errors.New("App-data folders cannot be symbolic links or reparse points.")
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	after, err := child.Stat(".")
	if err != nil || !os.SameFile(before, after) || storageIsLink(after) {
		_ = child.Close()
		if err != nil {
			return nil, err
		}
		return nil, errors.New("An app-data folder changed while it was being opened.")
	}
	return child, nil
}

func openStorageRegular(root *storageRoot, name string, flags int, mode os.FileMode) (*os.File, error) {
	before, err := root.Lstat(name)
	if err == nil {
		if !before.Mode().IsRegular() || storageIsLink(before) {
			return nil, fmt.Errorf("%s must be a regular app-data file, not a link or device", name)
		}
	} else if !errors.Is(err, os.ErrNotExist) || flags&os.O_CREATE == 0 {
		return nil, err
	}
	file, err := root.OpenFile(name, flags, mode)
	if err != nil {
		return nil, err
	}
	after, err := file.Stat()
	if err != nil || !after.Mode().IsRegular() || storageIsLink(after) || (before != nil && !os.SameFile(before, after)) {
		_ = file.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s changed while it was being opened", name)
	}
	return file, nil
}

func readStorageFile(root *storageRoot, name string, limit int64) ([]byte, error) {
	file, err := openStorageRegular(root, name, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("%s exceeds its app-data size limit", name)
	}
	raw, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("%s exceeds its app-data size limit", name)
	}
	return raw, nil
}

func writeStorageAtomic(root *storageRoot, name string, raw []byte) error {
	return writeStorageAtomicLimit(root, name, raw, storageMetadataLimit)
}

func writeStorageAtomicLimit(root *storageRoot, name string, raw []byte, limit int) error {
	if len(raw) > limit {
		return errors.New("App-data location metadata exceeds its size limit.")
	}
	if info, err := root.Lstat(name); err == nil {
		if !info.Mode().IsRegular() || storageIsLink(info) {
			return errors.New("App-data location metadata must be a regular file, not a link.")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	temporary := ".network-toolkit-storage-" + hex.EncodeToString(random[:]) + ".tmp"
	file, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	defer file.Close()
	if _, err := file.Write(raw); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return root.Rename(temporary, name)
}

type storageCopy struct {
	path string
	raw  []byte
	info os.FileInfo
}

func copyStorageData(source, destination *storageRoot) ([]string, func(), error) {
	if err := inspectStorageCopyDestination(destination); err != nil {
		return nil, nil, err
	}
	copies, err := collectStorageCopies(source)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		for _, item := range copies {
			clear(item.raw)
		}
	}()
	created := make([]storageCopy, 0, len(copies))
	rollback := func() {
		for i := len(created) - 1; i >= 0; i-- {
			removeStorageCopy(destination, created[i])
		}
	}
	paths := make([]string, 0, len(copies))
	for _, item := range copies {
		parent, closeParent, err := storageCopyParent(destination, item.path, true)
		if err != nil {
			rollback()
			return nil, nil, err
		}
		name := filepath.Base(item.path)
		file, err := openStorageRegular(parent, name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			closeParent()
			rollback()
			return nil, nil, fmt.Errorf("copy app data without overwriting the destination: %w", err)
		}
		item.info, err = file.Stat()
		created = append(created, storageCopy{path: item.path, info: item.info})
		if err == nil {
			_, err = file.Write(item.raw)
		}
		if err == nil {
			err = file.Sync()
		}
		err = errors.Join(err, file.Close())
		closeParent()
		if err != nil {
			rollback()
			return nil, nil, err
		}
		paths = append(paths, item.path)
	}
	return paths, rollback, nil
}

func collectStorageCopies(source *storageRoot) (copies []storageCopy, err error) {
	defer func() {
		if err != nil {
			for _, item := range copies {
				clear(item.raw)
			}
		}
	}()
	for _, workspace := range []string{"", storageTerminalDirectory} {
		root := source
		if workspace != "" {
			var err error
			root, err = openStorageChild(source, workspace)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, err
			}
		}
		for _, allowed := range storagePortableFiles {
			raw, err := readStorageFile(root, allowed.name, allowed.limit)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				if root != source {
					_ = root.Close()
				}
				return nil, err
			}
			copies = append(copies, storageCopy{path: filepath.Join(workspace, allowed.name), raw: raw})
		}
		if root != source {
			_ = root.Close()
		}
	}
	return copies, nil
}

type storageMigration struct {
	destination string
	initial     map[string][32]byte
}

func recordStorageMigration(destination *storageRoot, path string, copied []string) (*storageMigration, error) {
	migration := &storageMigration{destination: path, initial: make(map[string][32]byte)}
	for _, path := range copied {
		raw, err := readStorageCopy(destination, path)
		if err != nil {
			return nil, err
		}
		migration.initial[path] = sha256.Sum256(raw)
		clear(raw)
	}
	return migration, nil
}

func readStorageCopy(root *storageRoot, path string) ([]byte, error) {
	parent, closeParent, err := storageCopyParent(root, path, false)
	if err != nil {
		return nil, err
	}
	defer closeParent()
	for _, allowed := range storagePortableFiles {
		if filepath.Base(path) == allowed.name {
			return readStorageFile(parent, allowed.name, allowed.limit)
		}
	}
	return nil, errors.New("Unknown app-data copy file.")
}

func refreshStorageMigration(source, destination *storageRoot, migration *storageMigration) error {
	if pending, err := readStorageDeletePending(destination); err != nil || pending {
		if err != nil {
			return err
		}
		return errors.New("The copied destination is waiting for deletion; current data was retained in its original folder.")
	}
	copies, err := collectStorageCopies(source)
	if err != nil {
		return err
	}
	defer func() {
		for _, item := range copies {
			clear(item.raw)
		}
	}()
	current := make(map[string]bool, len(copies))
	for path, hash := range migration.initial {
		raw, err := readStorageCopy(destination, path)
		unchanged := err == nil && sha256.Sum256(raw) == hash
		clear(raw)
		if !unchanged {
			return fmt.Errorf("The copied destination %s changed. It was not overwritten; newer app data remains in the original folder.", path)
		}
	}
	for _, item := range copies {
		current[item.path] = true
		if _, existed := migration.initial[item.path]; !existed {
			if _, err := readStorageCopy(destination, item.path); !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("The destination acquired %s after copying. It was not overwritten; current app data remains in the original folder.", item.path)
			}
		}
	}
	for _, item := range copies {
		parent, closeParent, err := storageCopyParent(destination, item.path, true)
		if err != nil {
			return err
		}
		if hash, existed := migration.initial[item.path]; existed {
			err = replaceStorageCopy(parent, filepath.Base(item.path), item.raw, hash)
		} else {
			var file *os.File
			file, err = openStorageRegular(parent, filepath.Base(item.path), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err == nil {
				_, err = file.Write(item.raw)
				if err == nil {
					err = file.Sync()
				}
				err = errors.Join(err, file.Close())
			}
		}
		closeParent()
		if err != nil {
			return err
		}
	}
	for path, hash := range migration.initial {
		if current[path] {
			continue
		}
		parent, closeParent, err := storageCopyParent(destination, path, false)
		if err != nil {
			return err
		}
		raw, err := readStorageCopy(destination, path)
		if err == nil && sha256.Sum256(raw) != hash {
			err = errors.New("The copied destination changed before removal; it was kept.")
		}
		clear(raw)
		if err == nil {
			err = parent.Remove(filepath.Base(path))
		}
		closeParent()
		if err != nil {
			return err
		}
	}
	return nil
}

func replaceStorageCopy(parent *storageRoot, name string, raw []byte, expected [32]byte) error {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	temporary := ".network-toolkit-copy-" + hex.EncodeToString(random[:]) + ".tmp"
	file, err := parent.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer parent.Remove(temporary)
	defer file.Close()
	if _, err := file.Write(raw); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	previous, err := readStorageCopy(parent, name)
	unchanged := err == nil && sha256.Sum256(previous) == expected
	clear(previous)
	if !unchanged {
		return errors.New("A copied destination file changed before replacement; it was kept.")
	}
	return parent.Rename(temporary, name)
}

func storageCopyParent(root *storageRoot, path string, create bool) (*storageRoot, func(), error) {
	parent := filepath.Dir(path)
	if parent == "." {
		return root, func() {}, nil
	}
	if parent != storageTerminalDirectory {
		return nil, nil, errors.New("Unknown app-data copy path.")
	}
	if create {
		if err := root.Mkdir(parent, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, nil, err
		}
	}
	directory, err := openStorageChild(root, parent)
	if err != nil {
		return nil, nil, err
	}
	return directory, func() { _ = directory.Close() }, nil
}

func removeStorageCopy(root *storageRoot, item storageCopy) {
	parent, closeParent, err := storageCopyParent(root, item.path, false)
	if err != nil {
		return
	}
	defer closeParent()
	name := filepath.Base(item.path)
	info, err := parent.Lstat(name)
	if err == nil && item.info != nil && !storageIsLink(info) && os.SameFile(info, item.info) {
		_ = parent.Remove(name)
	}
}

func inspectStorageCopyDestination(destination *storageRoot) error {
	for _, workspace := range []string{"", storageTerminalDirectory} {
		root := destination
		if workspace != "" {
			var err error
			root, err = openStorageChild(destination, workspace)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
		}
		names := []string{storageBrowserDirectory, storageViewsDirectory, "Network_Toolkit.instance.json"}
		for _, allowed := range storagePortableFiles {
			names = append(names, allowed.name)
		}
		for _, name := range names {
			_, err := root.Lstat(name)
			if err == nil {
				if root != destination {
					_ = root.Close()
				}
				return fmt.Errorf("The destination already contains %s. Choose an empty app-data folder or turn off copying; existing data was kept.", filepath.Join(workspace, name))
			}
			if !errors.Is(err, os.ErrNotExist) {
				if root != destination {
					_ = root.Close()
				}
				return err
			}
		}
		if root != destination {
			_ = root.Close()
		}
	}
	return nil
}

type storageWalkOptions struct {
	remove     bool
	live       bool
	cacheLstat func(*storageRoot, string) (os.FileInfo, error)
}

func deleteStorageDataOnce(root *storageRoot, ops storageDeleteOps) error {
	// Inspection and removal both use fresh anchored Lstat after shutdown.
	// Live enumeration metadata can authorize closing, never a deletion.
	budget := storageTreeEntryLimit
	options := storageWalkOptions{cacheLstat: ops.cacheLstat}
	if err := walkStorageData(root, options, &budget); err != nil {
		return err
	}
	budget = storageTreeEntryLimit
	options.remove = true
	return walkStorageData(root, options, &budget)
}

func walkStorageData(root *storageRoot, options storageWalkOptions, budget *int) error {
	if err := walkStorageWorkspace(root, options, budget); err != nil {
		return err
	}
	terminal, err := openStorageChild(root, storageTerminalDirectory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	err = walkStorageWorkspace(terminal, options, budget)
	_ = terminal.Close()
	if err != nil {
		return err
	}
	if options.remove {
		return removeStorageEmptyDirectory(root, storageTerminalDirectory)
	}
	return nil
}

func walkStorageWorkspace(root *storageRoot, options storageWalkOptions, budget *int) error {
	names := []string{"Network_Toolkit.instance.json"}
	for _, allowed := range storagePortableFiles {
		names = append(names, allowed.name)
	}
	for _, name := range names {
		info, err := root.Lstat(name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || storageIsLink(info) {
			return fmt.Errorf("Refusing to delete linked or nonregular app data: %s", name)
		}
		if options.remove {
			if err := root.Remove(name); err != nil {
				return err
			}
		}
	}
	if err := walkStorageTree(root, storageBrowserDirectory, options, budget, 0); err != nil {
		return err
	}
	views, err := openStorageChild(root, storageViewsDirectory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	entries, err := readStorageDirectory(views)
	if err != nil {
		_ = views.Close()
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if len(name) != 32 || name != strings.ToLower(name) {
			continue
		}
		if _, err := hex.DecodeString(name); err != nil {
			continue
		}
		view, err := openStorageChild(views, name)
		if err != nil {
			_ = views.Close()
			return err
		}
		// Detached folders own only geometry and a browser profile. Unrecognized
		// files are deliberately retained, even within a correctly named view.
		info, statErr := view.Lstat("Network_Toolkit.window.json")
		if statErr == nil {
			if !info.Mode().IsRegular() || storageIsLink(info) {
				err = errors.New("Detached window state is linked or nonregular.")
			} else if options.remove {
				err = view.Remove("Network_Toolkit.window.json")
			}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			err = statErr
		}
		if err == nil {
			err = walkStorageTree(view, storageBrowserDirectory, options, budget, 0)
		}
		_ = view.Close()
		if err == nil && options.remove {
			err = removeStorageEmptyDirectory(views, name)
		}
		if err != nil {
			_ = views.Close()
			return err
		}
	}
	_ = views.Close()
	if options.remove {
		return removeStorageEmptyDirectory(root, storageViewsDirectory)
	}
	return nil
}

func walkStorageTree(parent *storageRoot, name string, options storageWalkOptions, budget *int, depth int) error {
	if depth > 64 {
		return errors.New("The app cache exceeds the safe directory-depth limit; no linked path was followed.")
	}
	directory, err := openStorageChild(parent, name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	entries, err := readStorageDirectory(directory)
	if err != nil {
		_ = directory.Close()
		return err
	}
	for _, entry := range entries {
		*budget--
		if *budget < 0 {
			_ = directory.Close()
			return errors.New("The app cache exceeds the safe entry limit.")
		}
		var info os.FileInfo
		var err error
		if options.live && !options.remove {
			// Go's Windows DirEntry.Info returns the cached enumeration record,
			// including FILE_ATTRIBUTE_REPARSE_POINT, without opening the file.
			// This permits safe preflight while Chromium locks Cookies. Every
			// directory is still opened through openStorageChild, which rejects
			// links/reparse points and verifies the opened directory's identity.
			info, err = entry.Info()
			if errors.Is(err, os.ErrNotExist) {
				continue // A live cache can remove an entry after enumeration.
			}
		} else {
			info, err = options.cacheLstat(directory, entry.Name())
		}
		if err != nil {
			_ = directory.Close()
			return err
		}
		if storageIsLink(info) {
			_ = directory.Close()
			return errors.New("Refusing to follow a linked or reparse-point entry in the app cache.")
		}
		if info.IsDir() {
			err = walkStorageTree(directory, entry.Name(), options, budget, depth+1)
		} else if !info.Mode().IsRegular() {
			err = errors.New("Refusing to delete a nonregular entry in the app cache.")
		} else if options.remove {
			err = directory.Remove(entry.Name())
		}
		if err != nil {
			_ = directory.Close()
			return err
		}
	}
	_ = directory.Close()
	if options.remove {
		return parent.Remove(name)
	}
	return nil
}

func readStorageDirectory(root *storageRoot) ([]os.DirEntry, error) {
	file, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	entries, err := file.ReadDir(storageTreeEntryLimit + 1)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if len(entries) > storageTreeEntryLimit {
		return nil, errors.New("The app-data directory exceeds the safe entry limit.")
	}
	return entries, nil
}

func removeStorageEmptyDirectory(parent *storageRoot, name string) error {
	directory, err := openStorageChild(parent, name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	entries, err := readStorageDirectory(directory)
	_ = directory.Close()
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return nil
	}
	return parent.Remove(name)
}
