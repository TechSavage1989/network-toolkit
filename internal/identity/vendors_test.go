package identity

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func fixtureStore(t *testing.T) *store {
	t.Helper()
	db, err := buildDatabase(fileData{Format: 1, Source: "IEEE test fixture", SourceDate: "2026-10-20", UpdatedAt: "2026-10-20T00:00:00Z", Entries: []assignment{
		{"001122", "Large assignment", "MA-L"},
		{"0011223", "Medium assignment", "MA-M"},
		{"001122334", "Small assignment", "MA-S"},
		{"0050C2123", "Legacy assignment", "IAB"},
	}}, true)
	if err != nil {
		t.Fatal(err)
	}
	s := &store{dir: t.TempDir()}
	s.current.Store(db)
	return s
}

func csvFixture(registry, prefix, vendor string) []byte {
	return []byte("Registry,Assignment,Organization Name,Organization Address\n" + registry + "," + prefix + "," + vendor + ",fixture\n")
}

func fixtureFetch(_ context.Context, source registrySource) ([]byte, error) {
	prefixes := map[string]string{"MA-L": "001122", "MA-M": "0011223", "MA-S": "001122334", "IAB": "0050C2123"}
	return csvFixture(source.registry, prefixes[source.registry], "Updated "+source.registry), nil
}

func TestLookupLongestPrefixAndAddressKinds(t *testing.T) {
	s := fixtureStore(t)
	for _, test := range []struct{ mac, vendor, kind, prefix string }{
		{"001122334455", "Small assignment", "global", "001122334"},
		{"00-11-22-3A-00-01", "Medium assignment", "global", "0011223"},
		{"0011.2244.0001", "Large assignment", "global", "001122"},
		{"00:50:c2:12:34:56", "Legacy assignment", "global", "0050C2123"},
		{"F8:AC:DE:00:00:01", "", "global", ""},
		{"02:11:22:33:44:55", "", "local", ""},
		{"01:11:22:33:44:55", "", "multicast", ""},
		{"ff:ff:ff:ff:ff:ff", "", "broadcast", ""},
		{"00:00:00:00:00:00", "", "unavailable", ""},
		{"", "", "unavailable", ""},
		{"not-a-mac", "", "invalid", ""},
		{"00:11:22:33:44:55:66:77", "", "invalid", ""},
	} {
		got := s.lookup(test.mac)
		if got.Vendor != test.vendor || got.MACKind != test.kind || got.Prefix != test.prefix {
			t.Errorf("%q: got %+v", test.mac, got)
		}
	}
}

func TestBundledDatabaseAllRegistriesAndHistoricalAssignments(t *testing.T) {
	db, err := decodeDatabase(bundledBytes, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(db.Entries) < 50000 || !db.bundled {
		t.Fatalf("unexpected bundled table: %d", len(db.Entries))
	}
	for _, registry := range []string{"MA-L", "MA-M", "MA-S", "IAB"} {
		if db.counts[registry] < 1000 {
			t.Errorf("missing registry %s", registry)
		}
	}
	if vendor := db.byPrefix["080030"].Vendor; !strings.Contains(vendor, "CERN") || !strings.Contains(vendor, "NETWORK RESEARCH CORPORATION") || !strings.Contains(vendor, "ROYAL MELBOURNE") {
		t.Fatalf("a historical assignment was discarded: %q", vendor)
	}
}

func TestCSVValidationAndHistoricalDuplicates(t *testing.T) {
	for _, data := range [][]byte{
		{}, []byte("<html>server error</html>"), []byte("Registry,Assignment,Organization Name\n"),
		csvFixture("MA-M", "001122", "Wrong length"), csvFixture("MA-L", "00ZZ22", "Invalid prefix"),
		csvFixture("MA-L", "001122", ""), []byte("Registry,Assignment,Organization Name\nMA-L,001122,\"truncated"),
		[]byte("Registry,Assignment,Organization Name\nMA-L,001122,Vendor,extra\n"),
		[]byte("Registry,Assignment,Organization Name\nMA-L,001122,\xff\n"),
		[]byte("Registry,Assignment,Organization Name,Registry\nMA-L,001122,Vendor,MA-L\n"),
		[]byte("Registry,Assignment,Organization Name\nIAB,0050C2123,A\nMA-S,0050C2123,B\n"),
	} {
		if _, err := parseCSV(data, ""); err == nil {
			t.Errorf("invalid CSV accepted: %q", data)
		}
	}
	if _, err := parseCSV(csvFixture("IAB", "0050C2123", "Legacy"), "MA-S"); err == nil {
		t.Fatal("download accepted the wrong registry")
	}
	data := []byte("\xef\xbb\xbfRegistry,Assignment,Organization Name\nMA-L,080030,Second\nMA-L,080030,First\nMA-L,080030,First\n")
	rows, err := parseCSV(data, "MA-L")
	if err != nil || len(rows) != 1 || rows[0].Vendor != "First; Second" {
		t.Fatalf("historical organizations not retained once: %+v %v", rows, err)
	}
}

func TestImportMergesPersistsReloadsAndResets(t *testing.T) {
	s := fixtureStore(t)
	data := csvFixture("MA-L", "001122", "Replacement organization")
	if err := s.importCSV(context.Background(), data); err != nil {
		t.Fatal(err)
	}
	if s.status().Bundled || s.status().Entries != 4 || s.lookup("00:11:22:44:00:01").Vendor != "Replacement organization" {
		t.Fatal("import did not replace the matching prefix while retaining other registries")
	}
	if status := s.status(); status.SourceDate != "" || status.UpdatedAt == "" {
		t.Fatal("CSV import must record its import time without claiming an unknown source date")
	}
	reloaded := fixtureStore(t)
	if err := reloaded.configure(s.dir); err != nil {
		t.Fatal(err)
	}
	if got := reloaded.lookup("00:11:22:44:00:01"); got.Vendor != "Replacement organization" || !strings.Contains(got.VendorSource, "Local CSV") {
		t.Fatalf("saved CSV identity lost: %+v", got)
	}
	if reloaded.lookup("00:11:22:33:44:55").Vendor != "Small assignment" {
		t.Fatal("import removed the more specific registry")
	}
	if err := reloaded.reset(); err != nil || !reloaded.status().Bundled {
		t.Fatalf("reset failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.dir, DatabaseFilename)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reset did not remove saved override: %v", err)
	}
}

func TestFailedUpdateImportAndCancellationRetainPriorBytes(t *testing.T) {
	s := fixtureStore(t)
	if err := s.persist(context.Background(), s.current.Load()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.dir, DatabaseFilename)
	before, _ := os.ReadFile(path)
	old := s.current.Load()
	called := 0
	err := s.update(context.Background(), func(ctx context.Context, source registrySource) ([]byte, error) {
		called++
		if called == 3 {
			return nil, errors.New("fixture network failure")
		}
		return fixtureFetch(ctx, source)
	})
	if err == nil || called != 3 || s.current.Load() != old {
		t.Fatal("partial multi-registry update was published")
	}
	if err := s.importCSV(context.Background(), []byte("corrupt")); err == nil {
		t.Fatal("corrupt import succeeded")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.importCSV(ctx, csvFixture("MA-L", "001122", "Cancelled replacement")); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled import: %v", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) || s.current.Load() != old || s.status().Updating {
		t.Fatal("a failed operation changed the previous database or stayed busy")
	}
	files, err := os.ReadDir(s.dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("temporary files left behind: %v %v", files, err)
	}
}

func TestUpdatePublishesAllRegistriesAndBusyDoesNotBlockLookups(t *testing.T) {
	s := fixtureStore(t)
	started, proceed, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		done <- s.update(context.Background(), func(ctx context.Context, source registrySource) ([]byte, error) {
			if source.registry == "MA-L" {
				close(started)
				<-proceed
			}
			return fixtureFetch(ctx, source)
		})
	}()
	<-started
	if !s.status().Updating || s.lookup("00:11:22:33:44:55").Vendor != "Small assignment" {
		t.Fatal("lookups unavailable during an update")
	}
	if err := s.importCSV(context.Background(), csvFixture("MA-L", "001122", "Other")); !errors.Is(err, ErrBusy) {
		t.Fatalf("overlapping import was not rejected: %v", err)
	}
	if err := s.reset(); !errors.Is(err, ErrBusy) {
		t.Fatalf("overlapping reset was not rejected: %v", err)
	}
	close(proceed)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := s.lookup("00:11:22:33:44:55"); got.Vendor != "Updated MA-S" || s.status().Bundled || s.status().Updating {
		t.Fatalf("update was not published: %+v %+v", got, s.status())
	}
}

func TestInvalidSavedDatabaseFallsBackAndWriteFailureRetainsCurrent(t *testing.T) {
	s := fixtureStore(t)
	if err := os.WriteFile(filepath.Join(s.dir, DatabaseFilename), []byte("truncated gzip"), 0600); err != nil {
		t.Fatal(err)
	}
	old := s.current.Load()
	if err := s.configure(s.dir); err == nil || s.current.Load() != old || !s.status().Bundled || s.status().Error == "" {
		t.Fatal("bad saved database replaced the bundled fallback")
	}
	blocked := filepath.Join(t.TempDir(), "file-instead-of-folder")
	if err := os.WriteFile(blocked, nil, 0600); err != nil {
		t.Fatal(err)
	}
	s.dir = blocked
	if err := s.importCSV(context.Background(), csvFixture("MA-L", "001122", "Unsaved")); err == nil || s.current.Load() != old {
		t.Fatal("failed persistence published new in-memory data")
	}
}

func TestConcurrentReadersSeeWholeDatabases(t *testing.T) {
	s := fixtureStore(t)
	old := s.current.Load()
	newData := old.fileData
	newData.Entries = []assignment{{"001122", "Replacement", "MA-L"}}
	replacement, err := buildDatabase(newData, false)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 2000; n++ {
				got := s.lookup("00:11:22:33:44:55").Vendor
				if got != "Small assignment" && got != "Replacement" {
					t.Errorf("partial database observed: %q", got)
					return
				}
			}
		}()
	}
	for i := 0; i < 2000; i++ {
		s.current.Store(replacement)
		s.current.Store(old)
	}
	wg.Wait()
}

func TestImportRejectsOversizedSavedJSONBeforeReplacingDatabase(t *testing.T) {
	s := fixtureStore(t)
	if err := s.persist(context.Background(), s.current.Load()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.dir, DatabaseFilename)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	old := s.current.Load()
	// This valid CSV is below 16 MiB, but JSON escapes each '<' to six bytes.
	// Such an import must not create a file that startup would later reject.
	var csv strings.Builder
	csv.WriteString("Registry,Assignment,Organization Name\n")
	for i := 0; i < 14000; i++ {
		csv.WriteString("MA-L,")
		csv.WriteString(strconv.FormatInt(int64(0x100000+i), 16))
		csv.WriteByte(',')
		csv.WriteString(strings.Repeat("<", 350))
		csv.WriteByte('\n')
	}
	if csv.Len() >= maxCSVBytes {
		t.Fatal("regression fixture must fit the CSV input limit")
	}
	if err := s.importCSV(context.Background(), []byte(csv.String())); err == nil || !strings.Contains(err.Error(), "24 MiB") {
		t.Fatalf("oversized saved JSON was not rejected: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) || s.current.Load() != old || s.status().Updating {
		t.Fatal("oversized import changed the previous database")
	}
	if _, err := decodeDatabase(after, false); err != nil {
		t.Fatalf("retained database cannot reload: %v", err)
	}
}
