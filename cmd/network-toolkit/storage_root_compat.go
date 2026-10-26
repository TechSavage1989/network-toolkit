package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// storageRoot is a compatibility implementation of the anchored file API used
// by newer Go releases. Every operation is constrained to an inspected root,
// rejects symlinks/reparse points before use, and verifies directory identity
// after opening. Higher-level storage helpers also verify file identity before
// reading or writing app data.
type storageRoot struct {
	path string
}

func openStorageRootBase(path string) (*storageRoot, error) {
	clean, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(clean)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || storageIsLink(info) {
		return nil, errors.New("App-data root must be a real directory, not a link or reparse point.")
	}
	return &storageRoot{path: clean}, nil
}

func (r *storageRoot) Close() error { return nil }

func validStorageRelative(name string) (string, error) {
	if name == "" {
		return "", errors.New("empty app-data path")
	}
	if filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
		return "", errors.New("absolute paths are not allowed inside app data")
	}
	clean := filepath.Clean(name)
	if clean == "." {
		return clean, nil
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("app-data path escapes its root")
	}
	return clean, nil
}

func (r *storageRoot) checkedPath(name string, requireParents bool) (string, error) {
	if r == nil || r.path == "" {
		return "", errors.New("app-data root is closed")
	}
	clean, err := validStorageRelative(name)
	if err != nil {
		return "", err
	}
	if clean == "." {
		return r.path, nil
	}
	full := filepath.Join(r.path, clean)
	rel, err := filepath.Rel(r.path, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", errors.New("app-data path escapes its root")
	}
	if requireParents {
		parent := filepath.Dir(clean)
		if parent != "." {
			current := r.path
			for _, component := range strings.Split(parent, string(filepath.Separator)) {
				if component == "" || component == "." {
					continue
				}
				current = filepath.Join(current, component)
				info, statErr := os.Lstat(current)
				if statErr != nil {
					return "", statErr
				}
				if !info.IsDir() || storageIsLink(info) {
					return "", fmt.Errorf("app-data parent %s is linked or not a directory", component)
				}
			}
		}
	}
	return full, nil
}

func (r *storageRoot) Lstat(name string) (os.FileInfo, error) {
	full, err := r.checkedPath(name, true)
	if err != nil {
		return nil, err
	}
	return os.Lstat(full)
}

func (r *storageRoot) Stat(name string) (os.FileInfo, error) {
	info, err := r.Lstat(name)
	if err != nil {
		return nil, err
	}
	if storageIsLink(info) {
		return nil, errors.New("Refusing to follow a linked app-data path.")
	}
	full, err := r.checkedPath(name, true)
	if err != nil {
		return nil, err
	}
	return os.Stat(full)
}

func (r *storageRoot) Open(name string) (*os.File, error) {
	full, err := r.checkedPath(name, true)
	if err != nil {
		return nil, err
	}
	before, err := os.Lstat(full)
	if err != nil {
		return nil, err
	}
	if storageIsLink(before) {
		return nil, errors.New("Refusing to open linked app data.")
	}
	file, err := os.Open(full)
	if err != nil {
		return nil, err
	}
	after, statErr := file.Stat()
	if statErr != nil || !os.SameFile(before, after) {
		_ = file.Close()
		if statErr != nil {
			return nil, statErr
		}
		return nil, errors.New("App-data entry changed while it was being opened.")
	}
	return file, nil
}

func (r *storageRoot) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	full, err := r.checkedPath(name, true)
	if err != nil {
		return nil, err
	}
	before, beforeErr := os.Lstat(full)
	if beforeErr == nil && storageIsLink(before) {
		return nil, errors.New("Refusing to open linked app data.")
	}
	if beforeErr != nil && !errors.Is(beforeErr, os.ErrNotExist) {
		return nil, beforeErr
	}
	file, err := os.OpenFile(full, flag, perm)
	if err != nil {
		return nil, err
	}
	after, statErr := file.Stat()
	if statErr != nil || storageIsLink(after) || (beforeErr == nil && !os.SameFile(before, after)) {
		_ = file.Close()
		if statErr != nil {
			return nil, statErr
		}
		return nil, errors.New("App-data entry changed while it was being opened.")
	}
	return file, nil
}

func (r *storageRoot) OpenRoot(name string) (*storageRoot, error) {
	full, err := r.checkedPath(name, true)
	if err != nil {
		return nil, err
	}
	before, err := os.Lstat(full)
	if err != nil {
		return nil, err
	}
	if !before.IsDir() || storageIsLink(before) {
		return nil, errors.New("App-data folder is linked or not a directory.")
	}
	after, err := os.Stat(full)
	if err != nil || !os.SameFile(before, after) || storageIsLink(after) {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("App-data folder changed while it was being opened.")
	}
	return &storageRoot{path: full}, nil
}

func (r *storageRoot) Mkdir(name string, perm os.FileMode) error {
	full, err := r.checkedPath(name, true)
	if err != nil {
		return err
	}
	return os.Mkdir(full, perm)
}

func (r *storageRoot) Remove(name string) error {
	full, err := r.checkedPath(name, true)
	if err != nil {
		return err
	}
	info, err := os.Lstat(full)
	if err != nil {
		return err
	}
	if storageIsLink(info) {
		return errors.New("Refusing to remove linked app data.")
	}
	return os.Remove(full)
}

func (r *storageRoot) Rename(oldName, newName string) error {
	oldPath, err := r.checkedPath(oldName, true)
	if err != nil {
		return err
	}
	newPath, err := r.checkedPath(newName, true)
	if err != nil {
		return err
	}
	source, err := os.Lstat(oldPath)
	if err != nil {
		return err
	}
	if storageIsLink(source) {
		return errors.New("Refusing to rename linked app data.")
	}
	if dest, destErr := os.Lstat(newPath); destErr == nil && storageIsLink(dest) {
		return errors.New("Refusing to replace linked app data.")
	} else if destErr != nil && !errors.Is(destErr, os.ErrNotExist) {
		return destErr
	}
	return os.Rename(oldPath, newPath)
}
