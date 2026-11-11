package main

import (
	"context"
	"net/http"
	"networktoolkit/internal/model"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Security review 11, item 1: the Setup that starts must be the verified
// bytes. The download lives in a private folder of its own (never the data
// folder), each check is repeated before it starts, and a folder that was
// swapped for a link, or a root that others can change, refuses to start.
// full_install_windows_test.go proves the held handle and the launch on
// Windows.

func TestReviewSetupDownloadLivesInAPrivateFolderOutsideTheData(t *testing.T) {
	f := newFullInstallFixture(t)
	f.publish(model.Version, synthSetup("p"), "")
	a := f.app()
	f.call(a, "/full-install/find", map[string]any{})
	if code, out := f.call(a, "/full-install/download", map[string]any{"version": model.Version, "confirm": true}); code != http.StatusOK {
		t.Fatalf("download: %d %v", code, out)
	}
	d := a.fullInstall.download
	if filepath.Dir(d.dir) != f.root || filepath.Dir(d.path) != d.dir {
		t.Fatalf("the download is at %s, not in its own folder under %s", d.path, f.root)
	}
	if strings.HasPrefix(d.path, filepath.Dir(a.dataPath)) {
		t.Fatal("the download is in the data folder")
	}
	if err := fullInstallCheckPrivateDir(d.dir); err != nil {
		t.Fatalf("the download folder is not private: %v", err)
	}
}

func TestReviewSetupStartRefusesASwappedFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need a privilege on Windows; full_install_windows_test.go uses a junction")
	}
	f := newFullInstallFixture(t)
	f.publish(model.Version, synthSetup("w"), "")
	a := f.app()
	f.call(a, "/full-install/find", map[string]any{})
	if code, _ := f.call(a, "/full-install/download", map[string]any{"version": model.Version, "confirm": true}); code != http.StatusOK {
		t.Fatal("download failed")
	}
	d := a.fullInstall.download
	// Another user's folder with a file of the same name and the same bytes
	// takes the place of the download folder.
	elsewhere := filepath.Join(f.dir, "elsewhere")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(d.path, filepath.Join(elsewhere, d.Name)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(d.dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, d.dir); err != nil {
		t.Fatal(err)
	}
	code, out := f.call(a, "/full-install/start", map[string]any{"version": model.Version, "confirm": true})
	if code == http.StatusOK || len(f.launched) != 0 {
		t.Fatalf("Setup started from a swapped folder: %d %v launched=%v", code, out, f.launched)
	}
}

func TestReviewSetupDownloadRefusesARootOthersCanChange(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows access lists: full_install_windows_test.go")
	}
	f := newFullInstallFixture(t)
	f.publish(model.Version, synthSetup("r"), "")
	if err := os.Chmod(f.root, 0o777); err != nil {
		t.Fatal(err)
	}
	a := f.app()
	f.call(a, "/full-install/find", map[string]any{})
	code, out := f.call(a, "/full-install/download", map[string]any{"version": model.Version, "confirm": true})
	if code == http.StatusOK || !strings.Contains(out["error"].(string), "only you can change") {
		t.Fatalf("download into a folder others can change: %d %v", code, out)
	}
	if files := f.setupFiles(); len(files) != 0 {
		t.Fatalf("a file was written: %v", files)
	}
}

// Item 6: status reads stay answered while Setup is being verified and started.
func TestReviewStartDoesNotHoldTheStatusLock(t *testing.T) {
	f := newFullInstallFixture(t)
	f.publish(model.Version, synthSetup("q"), "")
	a := f.app()
	f.call(a, "/full-install/find", map[string]any{})
	if code, _ := f.call(a, "/full-install/download", map[string]any{"version": model.Version, "confirm": true}); code != http.StatusOK {
		t.Fatal("download failed")
	}
	entered, release := make(chan struct{}), make(chan struct{})
	old := fullInstallLaunch
	t.Cleanup(func() { fullInstallLaunch = old })
	fullInstallLaunch = func(ctx context.Context, path string) error {
		close(entered)
		<-release
		return nil
	}
	done := make(chan int)
	go func() {
		code, _ := f.call(a, "/full-install/start", map[string]any{"version": model.Version, "confirm": true})
		done <- code
	}()
	<-entered
	statusRead := make(chan struct{})
	go func() {
		a.fullInstallStatus(editorRegistrationStatus{})
		close(statusRead)
	}()
	select {
	case <-statusRead:
	case <-time.After(2 * time.Second):
		t.Fatal("a status read waited for Setup to start")
	}
	// A second start while the first is under way is refused.
	if code, _ := f.call(a, "/full-install/start", map[string]any{"version": model.Version, "confirm": true}); code != http.StatusConflict {
		t.Fatalf("a second concurrent start: %d", code)
	}
	close(release)
	if code := <-done; code != http.StatusOK {
		t.Fatalf("start: %d", code)
	}
}
