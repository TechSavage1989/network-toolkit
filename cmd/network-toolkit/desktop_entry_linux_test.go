//go:build linux

package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopEntryInstallsOnlyUserFilesAndRemovesThem(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	a := testApplication(t)
	read := func(method string) desktopEntryState {
		t.Helper()
		w := requestApp(a, method, "/api/linux/desktop-entry", "", true)
		if w.Code != http.StatusOK {
			t.Fatalf("%s desktop entry: %d %s", method, w.Code, w.Body.String())
		}
		var state desktopEntryState
		if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
			t.Fatal(err)
		}
		return state
	}
	if s := read(http.MethodGet); !s.Supported || s.Installed {
		t.Fatalf("fresh state: %#v", s)
	}
	if s := read(http.MethodPost); !s.Installed || !s.Current {
		t.Fatalf("after adding: %#v", s)
	}
	entry, err := os.ReadFile(filepath.Join(data, "applications", "network-toolkit.desktop"))
	if err != nil {
		t.Fatal(err)
	}
	exe, _ := currentExecutable()
	for _, want := range []string{"[Desktop Entry]\n", "Type=Application\n", "Name=Network Toolkit\n", "Icon=network-toolkit\n", "Terminal=false\n", "StartupWMClass=NetworkToolkit\n", "Exec=" + desktopExecQuote(exe) + "\n"} {
		if !strings.Contains(string(entry), want) {
			t.Fatalf("desktop entry lacks %q:\n%s", want, entry)
		}
	}
	icon, err := os.ReadFile(filepath.Join(data, "icons", "hicolor", "scalable", "apps", "network-toolkit.svg"))
	if err != nil || !strings.Contains(string(icon), "<svg") {
		t.Fatalf("icon not installed: %v", err)
	}
	if requestApp(a, http.MethodPost, "/api/linux/desktop-entry", "", false).Code != http.StatusUnauthorized {
		t.Fatal("the menu entry can be written without the session token")
	}
	if s := read(http.MethodDelete); s.Installed {
		t.Fatalf("after removing: %#v", s)
	}
	if _, err := os.Stat(filepath.Join(data, "icons", "hicolor", "scalable", "apps", "network-toolkit.svg")); !os.IsNotExist(err) {
		t.Fatal("icon left behind")
	}
}

func TestDesktopExecQuoteEscapesSpecialCharacters(t *testing.T) {
	got := desktopExecQuote(`/home/a b/Net "Tool" $HOME` + "`x`" + `\100%/network-toolkit`)
	want := `"/home/a b/Net \"Tool\" \$HOME` + "\\`x\\`" + `\\100%%/network-toolkit"`
	if got != want {
		t.Fatalf("Exec quoting:\n got %s\nwant %s", got, want)
	}
}
