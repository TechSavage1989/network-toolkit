//go:build linux

package main

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Add to applications menu (Linux edition, docs/LINUX_PLAN.md). The portable
// program can be placed anywhere, so the menu entry points at the program's
// current location. It writes only the user's own XDG entries:
//
//	$XDG_DATA_HOME/applications/network-toolkit.desktop
//	$XDG_DATA_HOME/icons/hicolor/scalable/apps/network-toolkit.svg
//
// No system file is changed and no administrator approval is needed.

const desktopEntryName = "network-toolkit.desktop"

type desktopEntryPaths struct {
	entry, icon string
}

func linuxDataHome() (string, error) {
	if v := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); filepath.IsAbs(v) {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", errors.New("Your home folder could not be found.")
	}
	return filepath.Join(home, ".local", "share"), nil
}

func desktopEntryLocations() (desktopEntryPaths, error) {
	base, err := linuxDataHome()
	if err != nil {
		return desktopEntryPaths{}, err
	}
	return desktopEntryPaths{
		entry: filepath.Join(base, "applications", desktopEntryName),
		icon:  filepath.Join(base, "icons", "hicolor", "scalable", "apps", "network-toolkit.svg"),
	}, nil
}

// desktopExecQuote quotes a path for the Exec key (Desktop Entry
// Specification: a quoted argument escapes ", `, $ and \ with a backslash;
// a literal % is written as %%).
func desktopExecQuote(path string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range path {
		switch r {
		case '"', '`', '$', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '%':
			b.WriteString("%%")
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func desktopEntryText(executable string) string {
	return "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Version=1.0\n" +
		"Name=Network Toolkit\n" +
		"GenericName=Network diagnostics\n" +
		"Comment=Test devices and websites, connect with SSH, Telnet and serial terminals, and transfer files\n" +
		"Exec=" + desktopExecQuote(executable) + "\n" +
		"Icon=network-toolkit\n" +
		"Terminal=false\n" +
		"Categories=Network;Utility;\n" +
		"Keywords=ping;traceroute;dns;ssh;telnet;serial;sftp;network;\n" +
		"StartupWMClass=NetworkToolkit\n"
}

func currentExecutable() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path, nil
}

type desktopEntryState struct {
	Supported bool   `json:"supported"`
	Installed bool   `json:"installed"`
	Current   bool   `json:"current"` // the entry opens this copy of the program
	Path      string `json:"path,omitempty"`
}

func readDesktopEntryState() desktopEntryState {
	state := desktopEntryState{Supported: true}
	paths, err := desktopEntryLocations()
	exe, exeErr := currentExecutable()
	if err != nil || exeErr != nil {
		return state
	}
	state.Path = paths.entry
	data, err := os.ReadFile(paths.entry)
	if err != nil {
		return state
	}
	state.Installed = true
	state.Current = string(data) == desktopEntryText(exe)
	return state
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".network-toolkit-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func installDesktopEntry() error {
	paths, err := desktopEntryLocations()
	if err != nil {
		return err
	}
	exe, err := currentExecutable()
	if err != nil {
		return errors.New("The program's location could not be read.")
	}
	icon, err := web.ReadFile("assets/Network_Toolkit_Icon.svg")
	if err != nil {
		return errors.New("The app icon is unavailable.")
	}
	if err := writeFileAtomic(paths.icon, icon, 0o644); err != nil {
		return errors.New("The icon could not be saved: " + err.Error())
	}
	if err := writeFileAtomic(paths.entry, []byte(desktopEntryText(exe)), 0o644); err != nil {
		return errors.New("The menu entry could not be saved: " + err.Error())
	}
	return nil
}

func removeDesktopEntry() error {
	paths, err := desktopEntryLocations()
	if err != nil {
		return err
	}
	for _, path := range []string{paths.entry, paths.icon} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return errors.New("The menu entry could not be removed: " + err.Error())
		}
	}
	return nil
}

func (a *application) handleDesktopEntry(w http.ResponseWriter, r *http.Request) {
	var err error
	switch r.Method {
	case http.MethodGet:
	case http.MethodPost:
		err = installDesktopEntry()
	case http.MethodDelete:
		err = removeDesktopEntry()
	default:
		methodError(w)
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, readDesktopEntryState())
}
