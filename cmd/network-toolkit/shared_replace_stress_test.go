package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Three apps poll the shared files every few seconds while any of them may
// replace one. A reader that meets a replace in progress must neither fail,
// nor see half a file, nor take the moment for "the file was reset"; a writer
// must not fail because a reader has the file open (on Windows a replace
// fails with a sharing violation while another handle is open without
// delete sharing).

const sharedStressReplaces = 400

// stressReaders runs read in several goroutines until stop is closed and
// returns how often it was called.
func stressReaders(count int, stop *atomic.Bool, read func()) (*sync.WaitGroup, *atomic.Int64) {
	var wg sync.WaitGroup
	var calls atomic.Int64
	for n := 0; n < count; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				read()
				calls.Add(1)
			}
		}()
	}
	return &wg, &calls
}

func TestSharedFileReadersNeverSeeAReplaceInProgress(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, appSettingsFilename)
	identity := randomHex(16)
	document := func(n int) []byte {
		raw, _ := json.Marshal(map[string]any{credentialConfigIdentityField: identity, sharedWriterField: 1, "theme": "dark", "generation": n, "padding": fmt.Sprintf("%0*d", 4096+n%512, n)})
		return raw
	}
	if err := writeAtomic(path, document(0)); err != nil {
		t.Fatal(err)
	}
	var stop atomic.Bool
	var failures sync.Map
	fail := func(kind string) {
		count, _ := failures.LoadOrStore(kind, new(atomic.Int64))
		count.(*atomic.Int64).Add(1)
	}
	wg, reads := stressReaders(4, &stop, func() {
		// The vault's check before every use of a credential.
		if id, err := credentialConfigIdentity(path); err != nil {
			fail("identity could not be read: " + err.Error())
		} else if id != identity {
			fail("identity read as " + id)
		}
		// Every poll and every save.
		settings, err := readSharedSettings(path)
		switch {
		case err != nil:
			fail("settings could not be read: " + err.Error())
		case !settings.exists:
			fail("settings read as missing")
		case len(settings.members["padding"]) < 4096:
			fail("settings read short")
		}
	})
	writeFailures := 0
	var lastWriteError error
	for n := 1; n <= sharedStressReplaces; n++ {
		if _, err := writeSharedSettings(path, document(n)); err != nil {
			writeFailures++
			lastWriteError = err
		}
	}
	stop.Store(true)
	wg.Wait()
	if writeFailures > 0 {
		t.Errorf("%d of %d replaces failed while readers had the file open: %v", writeFailures, sharedStressReplaces, lastWriteError)
	}
	failures.Range(func(kind, count any) bool {
		t.Errorf("%d of %d reads: %s", count.(*atomic.Int64).Load(), reads.Load(), kind)
		return true
	})
	if reads.Load() < sharedStressReplaces {
		t.Logf("only %d reads ran next to %d replaces", reads.Load(), sharedStressReplaces)
	}
}

// The reviewer's case, through the real objects: one app saves its settings
// over and over while the other app's vault is asked for its state, as every
// poll does. The vault must stay unlocked.
func TestSharedVaultStaysUnlockedWhileAnotherAppSavesSettings(t *testing.T) {
	_, main, terminal := sharedVaultApps(t)
	page := openSharedPage(t, main)
	if _, err := terminal.credentials.put(credentialRecord{Name: "Kept", Username: "operator", Kind: "password", Secret: "fixture-secret"}); err != nil {
		t.Fatal(err)
	}
	var stop atomic.Bool
	var locked atomic.Value
	wg, checks := stressReaders(3, &stop, func() {
		_ = terminal.credentials.mark()
		status := terminal.credentials.status()
		if status.State != "unlocked" {
			locked.CompareAndSwap(nil, status.State+": "+status.Warning)
		}
		if _, ok := terminal.credentials.get("missing"); ok {
			locked.CompareAndSwap(nil, "a credential that does not exist was returned")
		}
	})
	saves := 0
	for n := 0; n < sharedStressReplaces; n++ {
		page.state["terminalSidebarWidth"] = float64(300 + n%100)
		if code, body := page.trySave(); code != 200 {
			stop.Store(true)
			wg.Wait()
			t.Fatalf("save %d failed while the other app read: %d %s", n, code, body)
		}
		saves++
	}
	stop.Store(true)
	wg.Wait()
	if state := locked.Load(); state != nil {
		t.Fatalf("the other app's vault did not stay unlocked through %d saves and %d checks: %v", saves, checks.Load(), state)
	}
	if got := credentialNames(terminal.credentials); got != "Kept" {
		t.Fatalf("credentials after the run: %q", got)
	}
}

// The other shared files: SSH host trust, the macro library, the record of
// the one-time move and the editor session index are replaced while they are
// read.
func TestSharedStoresSurviveReplacesWhileTheyAreRead(t *testing.T) {
	root := t.TempDir()
	key := knownHostFixtureKey(t)
	writeKnownHostFixture(t, root, knownHostFixtureEntry("keep.test:22", key))
	writer := sharedTestApp(t, root, "", "main")
	reader := sharedTestApp(t, root, terminalWorkspaceName, "terminal")
	const macroPath = "/api/terminal-automation/library"
	var stop atomic.Bool
	var problem atomic.Value
	report := func(format string, args ...any) { problem.CompareAndSwap(nil, fmt.Sprintf(format, args...)) }
	wg, _ := stressReaders(3, &stop, func() {
		if r := requestApp(reader, "GET", "/api/terminals/known-hosts", "", true); r.Code != 200 {
			report("trusted hosts could not be listed: %d %s", r.Code, r.Body.String())
		}
		if r := requestApp(reader, "GET", macroPath, "", true); r.Code != 200 {
			report("the macro library could not be read: %d %s", r.Code, r.Body.String())
		}
		if _, err := readSharedMarker(root); err != nil {
			report("the record of the one-time move could not be read: %v", err)
		}
	})
	library := macroHTTPLibrary(t, requestApp(writer, "GET", macroPath, "", true), 200)
	for n := 0; n < 150; n++ {
		host := fmt.Sprintf("drop%d.test:22", n)
		// Trust a host and remove it again: two replaces of the trust file.
		entries := []knownHostFixture{knownHostFixtureEntry("keep.test:22", key), knownHostFixtureEntry(host, key)}
		raw, _ := json.MarshalIndent(map[string]any{"version": 1, "hosts": entries}, "", "  ")
		if err := writeAtomic(filepath.Join(root, knownHostsFilename), raw); err != nil {
			report("the trust file could not be replaced: %v", err)
		}
		if r := requestApp(writer, "DELETE", "/api/terminals/known-hosts", `{"host":"`+host+`"}`, true); r.Code != 200 {
			report("a trusted host could not be removed while another app read: %d %s", r.Code, r.Body.String())
		}
		library.Macros = append(library.Macros[:0], macroHTTPDefinition(fmt.Sprintf("macro%d", n)))
		saved := viewTestRequest(writer, writer.token, "PUT", macroPath, library, nil)
		if saved.Code != 200 {
			report("the macro library could not be saved while another app read: %d %s", saved.Code, saved.Body.String())
			break
		}
		library = macroHTTPLibrary(t, saved, 200)
		marker := &sharedMarker{Version: 1, Sources: map[string]*sharedMigrationSource{"terminal/settings": {Digest: fmt.Sprint(n)}}}
		if err := writeSharedMarker(root, marker); err != nil {
			report("the record of the one-time move could not be replaced: %v", err)
		}
	}
	stop.Store(true)
	wg.Wait()
	if p := problem.Load(); p != nil {
		t.Fatal(p)
	}
	// Reading never latched an error: SSH still works with the trust file.
	if got := knownHostsOf(t, reader); !sameIDs(got, "keep.test:22") {
		t.Fatalf("trusted hosts after the run: %v", got)
	}
}

// A read that fails once (the file is being replaced, or a scanner holds it
// for a moment) must not disable SSH for the rest of the run.
func TestKnownHostsRecoverFromAReadThatFailedOnce(t *testing.T) {
	root := t.TempDir()
	key := knownHostFixtureKey(t)
	writeKnownHostFixture(t, root, knownHostFixtureEntry("keep.test:22", key))
	app := sharedTestApp(t, root, "", "main")
	if got := knownHostsOf(t, app); !sameIDs(got, "keep.test:22") {
		t.Fatalf("fixture: %v", got)
	}
	path := filepath.Join(root, knownHostsFilename)
	good, _ := os.ReadFile(path)
	// For a moment the file cannot be read as host trust.
	if err := os.WriteFile(path, []byte(`{"version":1,"hosts":[`), 0600); err != nil {
		t.Fatal(err)
	}
	if r := requestApp(app, "GET", "/api/terminals/known-hosts", "", true); r.Code == 200 {
		// Either answer is fine while the file is unreadable: the last good
		// list, or an error. What matters is the next read.
		t.Log("the last good list was served while the file was unreadable")
	}
	if err := os.WriteFile(path, good, 0600); err != nil {
		t.Fatal(err)
	}
	// Give the file a different time stamp where the store looks at it.
	_ = os.Chtimes(path, time.Now().Add(2*time.Second), time.Now().Add(2*time.Second))
	if got := knownHostsOf(t, app); !sameIDs(got, "keep.test:22") {
		t.Fatalf("host trust stayed disabled after the file was readable again: %v", got)
	}
	if r := requestApp(app, "DELETE", "/api/terminals/known-hosts", `{"host":"keep.test:22"}`, true); r.Code != 200 {
		t.Fatalf("host trust could not be changed after the file was readable again: %d %s", r.Code, r.Body.String())
	}
}
