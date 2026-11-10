package main

// Moving the app-data folder also moves each primary workspace's Text Editor
// session (the main window and the standalone Text Editor). Kept copies have no
// size ceiling, so they are streamed file by file instead of being read into
// memory like the small portable settings files.
import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var editorSessionWorkspaces = []string{"", standaloneEditorDirectory}

const editorSessionMigrating = editorSessionDirectory + ".migrating"

func openStorageWorkspace(root *storageRoot, workspace string, create bool) (*storageRoot, func(), error) {
	if workspace == "" {
		return root, func() {}, nil
	}
	if create {
		if err := root.Mkdir(workspace, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, nil, err
		}
	}
	child, err := openStorageChild(root, workspace)
	if err != nil {
		return nil, nil, err
	}
	return child, func() { _ = child.Close() }, nil
}

// editorSessionDigest reports the digest of a workspace's saved session index.
func editorSessionDigest(root *storageRoot, workspace string) ([32]byte, bool, error) {
	parent, closeParent, err := openStorageWorkspace(root, workspace, false)
	if errors.Is(err, os.ErrNotExist) {
		return [32]byte{}, false, nil
	}
	if err != nil {
		return [32]byte{}, false, err
	}
	defer closeParent()
	session, err := openStorageChild(parent, editorSessionDirectory)
	if errors.Is(err, os.ErrNotExist) {
		return [32]byte{}, false, nil
	}
	if err != nil {
		return [32]byte{}, false, err
	}
	defer session.Close()
	raw, err := readStorageFile(session, editorSessionIndexName, editorSessionIndexLimit)
	if errors.Is(err, os.ErrNotExist) {
		return [32]byte{}, false, nil
	}
	if err != nil {
		return [32]byte{}, false, err
	}
	return sha256.Sum256(raw), true, nil
}

func copyStorageStream(source, destination *storageRoot, name string, size int64) error {
	in, err := openStorageRegular(source, name, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := openStorageRegular(destination, name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	written, err := io.Copy(out, in)
	if err == nil && written != size {
		err = fmt.Errorf("%s changed while it was copied", name)
	}
	if err == nil {
		err = out.Sync()
	}
	return errors.Join(err, out.Close())
}

// copyEditorSession copies a workspace's session index and the kept copies it
// names into a new folder called target. A missing or unreadable session is
// left in place and not copied; it never blocks moving the other app data.
func copyEditorSession(source, destination *storageRoot, workspace, target string) (bool, error) {
	from, closeFrom, err := openStorageWorkspace(source, workspace, false)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer closeFrom()
	session, err := openStorageChild(from, editorSessionDirectory)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer session.Close()
	raw, err := readStorageFile(session, editorSessionIndexName, editorSessionIndexLimit)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	index, err := readEditorSessionIndex(session)
	if err != nil {
		return false, nil
	}
	to, closeTo, err := openStorageWorkspace(destination, workspace, true)
	if err != nil {
		return false, err
	}
	defer closeTo()
	if err := to.Mkdir(target, 0700); err != nil {
		return false, fmt.Errorf("copy the Text Editor session without overwriting the destination: %w", err)
	}
	copied, err := openStorageChild(to, target)
	if err != nil {
		return false, err
	}
	defer copied.Close()
	for _, tab := range index.Tabs {
		if tab.Backup == nil {
			continue
		}
		if err := copyStorageStream(session, copied, editorSessionBackupFile(tab.Backup.ID), tab.Backup.Size); err != nil {
			return true, err
		}
	}
	file, err := openStorageRegular(copied, editorSessionIndexName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return true, err
	}
	_, err = file.Write(raw)
	if err == nil {
		err = file.Sync()
	}
	return true, errors.Join(err, file.Close())
}

// removeEditorSessionFolder removes a session folder this app created: only
// its index, kept copies and temporary files, then the folder when empty.
func removeEditorSessionFolder(parent *storageRoot, name string) error {
	folder, err := openStorageChild(parent, name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	entries, err := readStorageDirectory(folder)
	if err != nil {
		_ = folder.Close()
		return err
	}
	for _, entry := range entries {
		item := entry.Name()
		if item != editorSessionIndexName && !editorSessionBackupName.MatchString(item) && !editorSessionTempName.MatchString(item) && !editorSessionAsideName.MatchString(item) {
			continue
		}
		info, err := folder.Lstat(item)
		if err != nil || !info.Mode().IsRegular() || storageIsLink(info) {
			continue
		}
		if err := folder.Remove(item); err != nil && !errors.Is(err, os.ErrNotExist) {
			_ = folder.Close()
			return err
		}
	}
	_ = folder.Close()
	return removeStorageEmptyDirectory(parent, name)
}

// copyEditorSessions copies every primary workspace's session to a new
// app-data folder. The returned rollback removes what was copied.
func copyEditorSessions(source, destination *storageRoot) (func(), error) {
	var created []string
	rollback := func() {
		for i := len(created) - 1; i >= 0; i-- {
			workspace := created[i]
			if parent, closeParent, err := openStorageWorkspace(destination, workspace, false); err == nil {
				_ = removeEditorSessionFolder(parent, editorSessionDirectory)
				closeParent()
			}
			if workspace != "" {
				_ = removeStorageEmptyDirectory(destination, workspace)
			}
		}
	}
	for _, workspace := range editorSessionWorkspaces {
		copied, err := copyEditorSession(source, destination, workspace, editorSessionDirectory)
		if copied {
			created = append(created, workspace)
		}
		if err != nil {
			rollback()
			return nil, err
		}
	}
	return rollback, nil
}

func recordEditorSessionMigration(destination *storageRoot, migration *storageMigration) error {
	migration.sessions = make(map[string][32]byte)
	for _, workspace := range editorSessionWorkspaces {
		digest, exists, err := editorSessionDigest(destination, workspace)
		if err != nil {
			return err
		}
		if exists {
			migration.sessions[workspace] = digest
		}
	}
	return nil
}

// checkEditorSessionMigration refuses to replace a destination session that
// changed after it was copied.
func checkEditorSessionMigration(destination *storageRoot, migration *storageMigration) error {
	for _, workspace := range editorSessionWorkspaces {
		digest, exists, err := editorSessionDigest(destination, workspace)
		if err != nil {
			return err
		}
		recorded, copied := migration.sessions[workspace]
		if exists != copied || exists && digest != recorded {
			return fmt.Errorf("The copied destination %s changed. It was not overwritten; newer app data remains in the original folder.", filepath.Join(workspace, editorSessionDirectory))
		}
	}
	return nil
}

// refreshEditorSessions replaces each copied session with the session as it
// is when the app closes. The new copy is complete before the old one goes.
func refreshEditorSessions(source, destination *storageRoot) error {
	for _, workspace := range editorSessionWorkspaces {
		parent, closeParent, err := openStorageWorkspace(destination, workspace, false)
		if err == nil {
			err = removeEditorSessionFolder(parent, editorSessionMigrating)
			closeParent()
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		copied, err := copyEditorSession(source, destination, workspace, editorSessionMigrating)
		parent, closeParent, openErr := openStorageWorkspace(destination, workspace, false)
		if errors.Is(openErr, os.ErrNotExist) {
			if err != nil {
				return err
			}
			continue
		}
		if openErr != nil {
			return errors.Join(err, openErr)
		}
		if err != nil {
			_ = removeEditorSessionFolder(parent, editorSessionMigrating)
			closeParent()
			return err
		}
		err = removeEditorSessionFolder(parent, editorSessionDirectory)
		if err == nil {
			if _, statErr := parent.Lstat(editorSessionDirectory); statErr == nil {
				err = errors.New("The copied Text Editor session folder contains other files; the newer session remains in the original folder.")
			}
		}
		if err == nil && copied {
			err = parent.Rename(editorSessionMigrating, editorSessionDirectory)
		}
		if err != nil {
			_ = removeEditorSessionFolder(parent, editorSessionMigrating)
		}
		closeParent()
		if err != nil {
			return err
		}
	}
	return nil
}
