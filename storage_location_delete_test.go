package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestAppStorageDeleteLockedCookiesClosesThenRetries(t *testing.T) {
	a := testApplication(t)
	manager := storageFixture(t, true)
	a.storage = manager
	a.dataPath = filepath.Join(manager.config.RootDir, "Network_Toolkit.settings.json")
	settings := writeStorageFixture(t, manager.config.RootDir, "Network_Toolkit.settings.json", "saved settings")
	report := writeStorageFixture(t, manager.config.RootDir, "exported-report.html", "keep report")
	var cookies []string
	for _, profile := range []string{
		storageBrowserDirectory,
		filepath.Join(storageTerminalDirectory, storageBrowserDirectory),
		filepath.Join(storageViewsDirectory, strings.Repeat("ab", 16), storageBrowserDirectory),
	} {
		cookies = append(cookies, writeStorageFixture(t, manager.config.RootDir, filepath.Join(profile, "Default", "Network", "Cookies"), "fixture cookies"))
	}
	locked, strictReads, waits := true, 0, 0
	lstat := manager.deletion.cacheLstat
	manager.deletion.cacheLstat = func(root *storageRoot, name string) (os.FileInfo, error) {
		if name == "Cookies" {
			strictReads++
			if locked {
				return nil, &os.PathError{Op: "statat", Path: name, Err: syscall.Errno(32)}
			}
		}
		return lstat(root, name)
	}
	manager.deletion.retryable = storageWindowsSharingViolation
	manager.deletion.wait = func(delay time.Duration) {
		waits++
		if delay != storageDeleteRetryDelay || a.ctx.Err() == nil {
			t.Fatal("cleanup retried before shutdown or with an unexpected delay")
		}
		// A failed strict inspection must not partly delete other app data.
		assertStorageContents(t, settings, "saved settings")
		for _, path := range cookies {
			assertStorageContents(t, path, "fixture cookies")
		}
		if waits == 2 {
			locked = false // Simulate the browser releasing only fixture handles.
		}
	}
	response := requestApp(a, http.MethodPost, "/api/storage/delete", `{"confirmation":"DELETE APP DATA"}`, true)
	var accepted struct {
		OK      bool `json:"ok"`
		Closing bool `json:"closing"`
	}
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &accepted) != nil || !accepted.OK || !accepted.Closing {
		t.Fatalf("live Cookies lock blocked the requested close: %d %s", response.Code, response.Body.String())
	}
	if strictReads != 0 || waits != 0 {
		t.Fatal("live preflight opened a cache file or retried cleanup before closing")
	}
	if pending, err := readStorageDeletePending(manager.root); err != nil || !pending || !manager.status().DeletePending {
		t.Fatalf("close was acknowledged without a durable deletion marker: %t %v", pending, err)
	}
	assertStorageContents(t, settings, "saved settings")
	select {
	case <-a.ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("accepted deletion did not stop the workspace")
	}
	if err := manager.Close(); err != nil {
		t.Fatal("cleanup failed after the fixture lock was released:", err)
	}
	if waits != 2 || strictReads == 0 {
		t.Fatalf("strict cleanup did not retry the sharing violation: waits=%d reads=%d", waits, strictReads)
	}
	for _, path := range append(cookies, settings, filepath.Join(manager.config.RootDir, storageDeleteFilename)) {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("completed cleanup left an owned file or marker: %s %v", path, err)
		}
	}
	assertStorageContents(t, report, "keep report")
}

func TestAppStorageCleanupPendingRetainsMarkerAndBlocksReopen(t *testing.T) {
	for _, code := range []syscall.Errno{32, 33} {
		t.Run(fmt.Sprint(uintptr(code)), func(t *testing.T) {
			manager := storageFixture(t, true)
			root := manager.config.RootDir
			settings := writeStorageFixture(t, root, "Network_Toolkit.settings.json", "keep until cleanup")
			cookies := writeStorageFixture(t, root, filepath.Join(storageBrowserDirectory, "Default", "Cookies"), "fixture cookies")
			report := writeStorageFixture(t, root, "report.json", "keep export")
			locked, attempts, waits := true, 0, 0
			lstat := manager.deletion.cacheLstat
			manager.deletion.cacheLstat = func(root *storageRoot, name string) (os.FileInfo, error) {
				if name == "Cookies" {
					attempts++
					if locked {
						return nil, &os.PathError{Op: "statat", Path: name, Err: code}
					}
				}
				return lstat(root, name)
			}
			manager.deletion.retryable = storageWindowsSharingViolation
			manager.deletion.wait = func(delay time.Duration) {
				if delay != storageDeleteRetryDelay {
					t.Fatalf("unexpected cleanup delay: %s", delay)
				}
				waits++ // No real delay or locked system file in this regression.
			}
			if err := manager.scheduleDelete(); err != nil {
				t.Fatal("live lock prevented scheduling:", err)
			}
			if err := manager.Close(); !errors.Is(err, ErrAppStorageCleanupPending) || !errors.Is(err, code) {
				t.Fatalf("exhausted lock retry did not retain its typed cause: %v", err)
			}
			if attempts != storageDeleteRetryCount+1 || waits != storageDeleteRetryCount {
				t.Fatalf("cleanup was not bounded: attempts=%d waits=%d", attempts, waits)
			}
			if manager.root != nil || manager.lease != nil || !manager.closed {
				t.Fatal("pending cleanup retained the closing workspace's handles")
			}
			assertStorageContents(t, settings, "keep until cleanup")
			assertStorageContents(t, cookies, "fixture cookies")
			marker := filepath.Join(root, storageDeleteFilename)
			if _, err := os.Stat(marker); err != nil {
				t.Fatal("pending cleanup lost its durable marker:", err)
			}
			next := newAppStorage(manager.config)
			next.deletion = manager.deletion
			defer next.Close()
			attempts, waits = 0, 0
			if err := next.Open(); !errors.Is(err, ErrAppStorageCleanupPending) || !errors.Is(err, code) {
				t.Fatalf("startup ignored the still-blocked cleanup: %v", err)
			}
			if next.root != nil || next.lease != nil || attempts != storageDeleteRetryCount+1 || waits != storageDeleteRetryCount {
				t.Fatal("failed startup opened storage or escaped the retry bound")
			}
			assertStorageContents(t, settings, "keep until cleanup")
			assertStorageContents(t, cookies, "fixture cookies")
			if _, err := os.Stat(marker); err != nil {
				t.Fatal("failed startup lost the pending marker:", err)
			}
			locked = false
			if err := next.Open(); err != nil {
				t.Fatal("startup could not recover after the fixture lock released:", err)
			}
			for _, path := range []string{settings, cookies, marker} {
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("reopen did not complete pending cleanup: %s %v", path, err)
				}
			}
			assertStorageContents(t, report, "keep export")
		})
	}
}

func TestAppStorageCleanupDoesNotRetryOtherFailures(t *testing.T) {
	for _, code := range []syscall.Errno{5, 145} {
		t.Run(fmt.Sprint(uintptr(code)), func(t *testing.T) {
			manager := storageFixture(t, true)
			settings := writeStorageFixture(t, manager.config.RootDir, "Network_Toolkit.settings.json", "keep settings")
			writeStorageFixture(t, manager.config.RootDir, filepath.Join(storageBrowserDirectory, "Cookies"), "fixture cookies")
			attempts, waits := 0, 0
			manager.deletion.cacheLstat = func(_ *storageRoot, name string) (os.FileInfo, error) {
				attempts++
				return nil, &os.PathError{Op: "statat", Path: name, Err: code}
			}
			manager.deletion.retryable = storageWindowsSharingViolation
			manager.deletion.wait = func(time.Duration) { waits++ }
			if err := manager.scheduleDelete(); err != nil {
				t.Fatal(err)
			}
			if err := manager.Close(); !errors.Is(err, code) || errors.Is(err, ErrAppStorageCleanupPending) {
				t.Fatalf("non-lock error was hidden as pending browser cleanup: %v", err)
			}
			if attempts != 1 || waits != 0 {
				t.Fatalf("non-lock error retried: attempts=%d waits=%d", attempts, waits)
			}
			assertStorageContents(t, settings, "keep settings")
			if _, err := os.Stat(filepath.Join(manager.config.RootDir, storageDeleteFilename)); err != nil {
				t.Fatal("failed cleanup lost the marker:", err)
			}
		})
	}
}

func TestAppStorageCleanupRevalidatesAfterLivePreflight(t *testing.T) {
	manager := storageFixture(t, true)
	settings := writeStorageFixture(t, manager.config.RootDir, "Network_Toolkit.settings.json", "keep settings")
	writeStorageFixture(t, manager.config.RootDir, filepath.Join(storageBrowserDirectory, "Cookies"), "fixture cookies")
	outside := t.TempDir()
	protected := writeStorageFixture(t, outside, "protected.txt", "outside content")
	if err := manager.scheduleDelete(); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(manager.config.RootDir, storageBrowserDirectory, "late-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	waits := 0
	manager.deletion.retryable = storageWindowsSharingViolation
	manager.deletion.wait = func(time.Duration) { waits++ }
	if err := manager.Close(); err == nil || errors.Is(err, ErrAppStorageCleanupPending) {
		t.Fatalf("late cache link was accepted or treated as a temporary lock: %v", err)
	}
	if waits != 0 {
		t.Fatal("unsafe path was retried")
	}
	assertStorageContents(t, settings, "keep settings")
	assertStorageContents(t, protected, "outside content")
	if _, err := os.Stat(filepath.Join(manager.config.RootDir, storageDeleteFilename)); err != nil {
		t.Fatal("unsafe cleanup lost the pending marker:", err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	next := newAppStorage(manager.config)
	if err := next.Open(); err != nil {
		t.Fatal("safe recovery after removing only the fixture link failed:", err)
	}
	defer next.Close()
	assertStorageContents(t, protected, "outside content")
	if _, err := os.Stat(settings); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("safe recovery did not finish pending deletion:", err)
	}
}

func TestAppStorageSharingErrorsUseNativeCodes(t *testing.T) {
	for _, code := range []syscall.Errno{32, 33} {
		if got := defaultStorageDeleteOps().retryable(code); got != (runtime.GOOS == "windows") {
			t.Fatalf("platform cleanup interpreted errno %d as a Windows lock on %s: %t", uintptr(code), runtime.GOOS, got)
		}
	}
	for _, err := range []error{
		syscall.Errno(32), syscall.Errno(33),
		&os.PathError{Op: "statat", Path: "Cookies", Err: syscall.Errno(32)},
		fmt.Errorf("cleanup: %w", &os.PathError{Op: "removeat", Path: "Cookies", Err: syscall.Errno(33)}),
		errors.Join(errors.New("fixture context"), syscall.Errno(32)),
	} {
		if !storageWindowsSharingViolation(err) {
			t.Fatalf("wrapped native sharing/lock error was not recognized: %v", err)
		}
	}
	for _, err := range []error{nil, syscall.Errno(5), syscall.Errno(145), os.ErrPermission, errors.New("The process cannot access the file because it is being used by another process.")} {
		if storageWindowsSharingViolation(err) {
			t.Fatalf("non-lock error was classified by text or broad permission matching: %v", err)
		}
	}
}
