package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAdminReopenArguments(t *testing.T) {
	never := func(int, time.Duration, string) error { t.Fatal("waited without a restart request"); return nil }
	plain := []string{"app.exe", "--terminal"}
	if got, err := prepareAdminReopen(plain, never); err != nil || strings.Join(got, " ") != "app.exe --terminal" {
		t.Fatalf("an ordinary start changed: %v %v", got, err)
	}
	root := t.TempDir()
	dir := filepath.Join(root, adminReopenPrefix+"x")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	var waited string
	ok := func(pid int, limit time.Duration, folder string) error {
		if pid != 42 || limit != adminReopenWait {
			t.Fatalf("wait(%d, %v)", pid, limit)
		}
		waited = folder
		return os.WriteFile(filepath.Join(folder, "ready"), []byte("ready"), 0o600)
	}
	got, err := prepareAdminReopen([]string{"app.exe", adminReopenFlag, "42", dir, "--terminal"}, ok)
	if err != nil || strings.Join(got, " ") != "app.exe --terminal" || waited != dir {
		t.Fatalf("restart start: %v %v %q", got, err, waited)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("the restart folder was left behind: %v", err)
	}
	for name, args := range map[string][]string{
		"missing":  {"app.exe", adminReopenFlag, "42"},
		"pid":      {"app.exe", adminReopenFlag, "0", dir},
		"text pid": {"app.exe", adminReopenFlag, "x", dir},
		"folder":   {"app.exe", adminReopenFlag, "42", filepath.Join(root, "other")},
		"relative": {"app.exe", adminReopenFlag, "42", adminReopenPrefix + "x"},
	} {
		if _, err := prepareAdminReopen(args, never); err == nil {
			t.Fatalf("%s: accepted %v", name, args)
		}
	}
	// A cancelled restart ends quietly and opens nothing.
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	cancelled := func(int, time.Duration, string) error { return errAdminReopenCancelled }
	if _, err := prepareAdminReopen([]string{"app.exe", adminReopenFlag, "42", dir}, cancelled); !errors.Is(err, errAdminReopenCancelled) {
		t.Fatalf("cancel: %v", err)
	}
	// The flag cannot be repeated to hide a second request.
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareAdminReopen([]string{"app.exe", adminReopenFlag, "42", dir, adminReopenFlag, "1", dir}, ok); err == nil {
		t.Fatal("a repeated restart switch was accepted")
	}
}

func TestAdminRestartArgumentsKeepTheRunsSwitches(t *testing.T) {
	saved := os.Args
	t.Cleanup(func() { os.Args = saved })
	os.Args = []string{"app.exe", "--terminal", "--data-dir", `C:\Data`, "--update-health", `C:\stage`}
	got := strings.Join(adminRestartArgs(7, `C:\x\`+adminReopenPrefix+"1"), " ")
	if got != adminReopenFlag+` 7 C:\x\`+adminReopenPrefix+`1 --terminal --data-dir C:\Data` {
		t.Fatalf("restart switches: %s", got)
	}
}

func TestAdminRestartRoute(t *testing.T) {
	a := &application{windowMode: "main"}
	call := func(method, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		a.handleAdminRestart(w, httptest.NewRequest(method, "/api/admin-restart", strings.NewReader(body)))
		return w
	}
	if w := call(http.MethodGet, ""); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET: %d", w.Code)
	}
	for _, body := range []string{`{"action":"start"}`, `{"action":"start","confirm":true,"extra":1}`, `{"action":"other"}`, `not json`} {
		if w := call(http.MethodPost, body); w.Code == http.StatusOK {
			t.Fatalf("%s was accepted: %s", body, w.Body)
		}
	}
	if w := call(http.MethodPost, `{"action":"cancel"}`); w.Code != http.StatusOK {
		t.Fatalf("cancel: %d %s", w.Code, w.Body)
	}
	a.windowMode = "editor"
	if a.adminRestartAvailable() {
		t.Fatal("the Text Editor app offers the restart")
	}
	if err := a.startAdminRestart(); err == nil {
		t.Fatal("a restart started outside the main window and the Terminal app")
	}
}
