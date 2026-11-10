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
	source, err := openStorageRoot(root, false)
	if err != nil {
		return err
	}
	defer source.Close()
	initial := map[string]any{"activePage": "editor"}
	if raw, err := readStorageFile(source, settings, 2<<20); err == nil {
		var prior map[string]any
		if json.Unmarshal(raw, &prior) == nil {
			for _, key := range []string{"theme", "accentColor", "editorPreferences", "editorFilesPreferences", "colorPickerPreferences"} {
				if value, ok := prior[key]; ok {
					initial[key] = value
				}
			}
		}
	}
	sanitizeColorPickerPreferences(initial)
	data, err := json.MarshalIndent(initial, "", "  ")
	if err != nil {
		return err
	}
	return writeStorageAtomic(editor, settings, data)
}
