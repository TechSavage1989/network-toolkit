package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const standaloneEditorDirectory = "Network_Toolkit.TextEditor"

func prepareStandaloneEditor(root string, request *editorLaunchRequest) (string, func(), bool, error) {
	if request == nil {
		request = &editorLaunchRequest{}
	}
	dir := filepath.Join(root, standaloneEditorDirectory)
	storage, err := openStorageRoot(dir, true)
	if err != nil {
		return "", nil, false, err
	}
	defer storage.Close()
	release, reused, err := prepareEditorWorkspace(dir, standaloneEditorMode, request)
	if reused || err != nil {
		return "", nil, reused, err
	}
	if err := seedStandaloneEditor(root, storage); err != nil {
		release()
		return "", nil, false, err
	}
	return dir, release, false, nil
}

// First use: the Text Editor window opens on the editor. Its settings and the
// saved connections it lists are shared by every app of the data folder and
// are not copied into the workspace (shared_data.go).
func seedStandaloneEditor(root string, editor *storageRoot) error {
	const settings = "Network_Toolkit.settings.json"
	if info, err := editor.Lstat(settings); err == nil {
		if !info.Mode().IsRegular() || storageIsLink(info) {
			return errors.New("The Text Editor settings must be a regular file.")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := json.MarshalIndent(map[string]any{"activePage": "editor"}, "", "  ")
	if err != nil {
		return err
	}
	return writeStorageAtomic(editor, settings, data)
}
