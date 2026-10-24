// Package identity resolves MAC assignments using an embedded IEEE registry.
// Lookups do not send addresses or other device information to the internet.
package identity

import (
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"
)

const DatabaseFilename = "Network_Toolkit.vendors.json.gz"
const maxDatabaseBytes = 24 << 20
const maxCSVBytes = 16 << 20
const maxEntries = 250000

var ErrBusy = errors.New("a vendor database operation is already running")

//go:embed data/vendors.json.gz
var bundledBytes []byte

type MACInfo struct {
	MAC          string `json:"mac,omitempty"`
	Vendor       string `json:"vendor,omitempty"`
	VendorSource string `json:"vendorSource,omitempty"`
	MACKind      string `json:"macKind"`
	Prefix       string `json:"prefix,omitempty"`
}

type Status struct {
	Source          string `json:"source"`
	SourceDate      string `json:"sourceDate"`
	UpdatedAt       string `json:"updatedAt"`
	Entries         int    `json:"entries"`
	Bundled         bool   `json:"bundled"`
	Updating        bool   `json:"updating"`
	UpdateAvailable bool   `json:"updateAvailable"`
	Error           string `json:"error,omitempty"`
}

type assignment struct {
	Prefix   string `json:"p"`
	Vendor   string `json:"v"`
	Registry string `json:"r"`
}

type fileData struct {
	Format     int          `json:"format"`
	Source     string       `json:"source"`
	SourceDate string       `json:"sourceDate"`
	UpdatedAt  string       `json:"updatedAt"`
	Entries    []assignment `json:"entries"`
}

type database struct {
	fileData
	byPrefix map[string]assignment
	counts   map[string]int
	bundled  bool
}

type store struct {
	current  atomic.Pointer[database]
	updating atomic.Bool
	mu       sync.RWMutex
	dir      string
	lastErr  string
}

var once sync.Once
var defaultStore *store

func global() *store {
	once.Do(func() {
		db, err := decodeDatabase(bundledBytes, true)
		if err != nil {
			panic("The embedded vendor database is invalid: " + err.Error())
		}
		defaultStore = &store{}
		defaultStore.current.Store(db)
	})
	return defaultStore
}

func Configure(dataDir string) error { return global().configure(dataDir) }
func DatabaseStatus() Status         { return global().status() }
func LookupMAC(raw string) MACInfo   { return global().lookup(raw) }
func UpdateDatabase(ctx context.Context) (Status, error) {
	err := global().update(ctx, downloadRegistry)
	return global().status(), err
}
func ImportDatabaseCSV(ctx context.Context, data []byte) (Status, error) {
	err := global().importCSV(ctx, data)
	return global().status(), err
}
func ResetDatabase() (Status, error) {
	err := global().reset()
	return global().status(), err
}

func (s *store) configure(dataDir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir, err := filepath.Abs(dataDir)
	if err != nil {
		s.lastErr = err.Error()
		return err
	}
	s.dir = dir
	f, err := os.Open(filepath.Join(dir, DatabaseFilename))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		s.lastErr = "Could not read the saved vendor database; using the bundled copy. " + err.Error()
		return errors.New(s.lastErr)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxDatabaseBytes+1))
	if err == nil && len(data) > maxDatabaseBytes {
		err = errors.New("saved database is too large")
	}
	var db *database
	if err == nil {
		db, err = decodeDatabase(data, false)
	}
	if err != nil {
		s.lastErr = "Could not read the saved vendor database; using the bundled copy. " + err.Error()
		return errors.New(s.lastErr)
	}
	s.current.Store(db)
	s.lastErr = ""
	return nil
}

func (s *store) status() Status {
	db := s.current.Load()
	s.mu.RLock()
	err := s.lastErr
	s.mu.RUnlock()
	return Status{Source: db.Source, SourceDate: db.SourceDate, UpdatedAt: db.UpdatedAt,
		Entries: len(db.Entries), Bundled: db.bundled, Updating: s.updating.Load(), UpdateAvailable: true, Error: err}
}

func (s *store) lookup(raw string) MACInfo {
	if strings.TrimSpace(raw) == "" {
		return MACInfo{MACKind: "unavailable"}
	}
	normalized := strings.TrimSpace(raw)
	if len(normalized) == 12 {
		if data, err := hex.DecodeString(normalized); err == nil {
			normalized = net.HardwareAddr(data).String()
		}
	}
	mac, err := net.ParseMAC(normalized)
	if err != nil || len(mac) != 6 {
		return MACInfo{MACKind: "invalid"}
	}
	info := MACInfo{MAC: strings.ToUpper(mac.String()), MACKind: "global"}
	if bytes.Equal(mac, []byte{0, 0, 0, 0, 0, 0}) {
		info.MACKind = "unavailable"
		return info
	}
	if bytes.Equal(mac, []byte{255, 255, 255, 255, 255, 255}) {
		info.MACKind = "broadcast"
		return info
	}
	if mac[0]&1 != 0 {
		info.MACKind = "multicast"
		return info
	}
	if mac[0]&2 != 0 {
		info.MACKind = "local"
		return info
	}
	key := strings.ToUpper(hex.EncodeToString(mac))
	db := s.current.Load()
	for _, length := range []int{9, 7, 6} {
		if item, found := db.byPrefix[key[:length]]; found {
			info.Vendor, info.Prefix = item.Vendor, item.Prefix
			info.VendorSource = "IEEE " + item.Registry
			if strings.HasPrefix(db.Source, "Imported") {
				info.VendorSource = "Local CSV database / " + item.Registry
			}
			return info
		}
	}
	return info
}

func cleanVendor(value string) (string, error) {
	if !utf8.ValidString(value) {
		return "", errors.New("organization name is not valid UTF-8")
	}
	value = strings.Join(strings.Fields(value), " ")
	if value == "" || len(value) > 1024 {
		return "", errors.New("organization name is empty or too long")
	}
	for _, c := range value {
		if unicode.IsControl(c) {
			return "", errors.New("organization name contains a control character")
		}
	}
	return value, nil
}

func validateAssignment(item assignment) (assignment, error) {
	item.Registry = strings.ToUpper(strings.TrimSpace(item.Registry))
	lengths := map[string]int{"MA-L": 6, "MA-M": 7, "MA-S": 9, "IAB": 9}
	length, found := lengths[item.Registry]
	item.Prefix = strings.ToUpper(strings.TrimSpace(item.Prefix))
	if !found || len(item.Prefix) != length {
		return item, errors.New("invalid registry or assignment length")
	}
	for _, c := range item.Prefix {
		if !(c >= '0' && c <= '9' || c >= 'A' && c <= 'F') {
			return item, errors.New("assignment is not hexadecimal")
		}
	}
	var err error
	item.Vendor, err = cleanVendor(item.Vendor)
	return item, err
}

func buildDatabase(data fileData, bundled bool) (*database, error) {
	if data.Format != 1 || len(data.Entries) == 0 || len(data.Entries) > maxEntries {
		return nil, errors.New("unsupported database format or entry count")
	}
	if data.SourceDate == "" {
		if !strings.HasPrefix(data.Source, "Imported") {
			return nil, errors.New("invalid database date")
		}
	} else if _, err := time.Parse("2006-01-02", data.SourceDate); err != nil {
		return nil, errors.New("invalid database date")
	}
	if _, err := time.Parse(time.RFC3339, data.UpdatedAt); err != nil {
		return nil, errors.New("invalid database update time")
	}
	if len(data.Source) > 200 || strings.TrimSpace(data.Source) == "" {
		return nil, errors.New("invalid database source")
	}
	db := &database{fileData: data, byPrefix: make(map[string]assignment, len(data.Entries)), counts: make(map[string]int), bundled: bundled}
	for i, item := range data.Entries {
		item, err := validateAssignment(item)
		if err != nil {
			return nil, fmt.Errorf("database entry %d: %w", i+1, err)
		}
		if previous, found := db.byPrefix[item.Prefix]; found && previous != item {
			return nil, fmt.Errorf("conflicting assignments for prefix %s", item.Prefix)
		}
		db.byPrefix[item.Prefix] = item
	}
	db.Entries = make([]assignment, 0, len(db.byPrefix))
	for _, item := range db.byPrefix {
		db.Entries = append(db.Entries, item)
		db.counts[item.Registry]++
	}
	sort.Slice(db.Entries, func(i, j int) bool { return db.Entries[i].Prefix < db.Entries[j].Prefix })
	return db, nil
}

func decodeDatabase(data []byte, bundled bool) (*database, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	plain, err := io.ReadAll(io.LimitReader(r, maxDatabaseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(plain) > maxDatabaseBytes {
		return nil, errors.New("expanded database is too large")
	}
	var stored fileData
	if err := json.Unmarshal(plain, &stored); err != nil {
		return nil, err
	}
	return buildDatabase(stored, bundled)
}

func parseCSV(data []byte, expected string) ([]assignment, error) {
	if len(data) == 0 || len(data) > maxCSVBytes {
		return nil, errors.New("vendor CSV must contain data and be at most 16 MiB")
	}
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})))
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}
	columns := make(map[string]int)
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(h))
		if _, exists := columns[key]; exists {
			return nil, errors.New("vendor CSV contains duplicate columns")
		}
		columns[key] = i
	}
	for _, key := range []string{"registry", "assignment", "organization name"} {
		if _, exists := columns[key]; !exists {
			return nil, errors.New("use an IEEE CSV with Registry, Assignment, and Organization Name columns")
		}
	}
	rows := make([]assignment, 0)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read vendor CSV: %w", err)
		}
		item, err := validateAssignment(assignment{Prefix: record[columns["assignment"]], Vendor: record[columns["organization name"]], Registry: record[columns["registry"]]})
		if err != nil {
			return nil, fmt.Errorf("vendor CSV row %d: %w", len(rows)+2, err)
		}
		if expected != "" && item.Registry != expected {
			return nil, fmt.Errorf("expected the %s registry, received %s", expected, item.Registry)
		}
		rows = append(rows, item)
		if len(rows) > maxEntries {
			return nil, errors.New("vendor CSV contains too many entries")
		}
	}
	if len(rows) == 0 {
		return nil, errors.New("vendor CSV contains no assignments")
	}
	// IEEE retains a few historical prefixes assigned to several organizations.
	// Preserve every listed organization rather than selecting an arbitrary row.
	unique := make(map[string]assignment)
	names := make(map[string]map[string]bool)
	for _, row := range rows {
		if previous, exists := unique[row.Prefix]; exists && previous.Registry != row.Registry {
			return nil, errors.New("vendor CSV assigns one prefix to conflicting registries")
		}
		unique[row.Prefix] = row
		if names[row.Prefix] == nil {
			names[row.Prefix] = make(map[string]bool)
		}
		names[row.Prefix][row.Vendor] = true
	}
	items := make([]assignment, 0, len(unique))
	for prefix, row := range unique {
		organizations := make([]string, 0, len(names[prefix]))
		for name := range names[prefix] {
			organizations = append(organizations, name)
		}
		sort.Strings(organizations)
		row.Vendor = strings.Join(organizations, "; ")
		if len(row.Vendor) > 1024 {
			return nil, errors.New("vendor CSV contains too many organization names for one prefix")
		}
		items = append(items, row)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Prefix < items[j].Prefix })
	return items, nil
}

type registrySource struct{ registry, url string }

var registrySources = []registrySource{
	{"MA-L", "https://standards-oui.ieee.org/oui/oui.csv"},
	{"MA-M", "https://standards-oui.ieee.org/oui28/mam.csv"},
	{"MA-S", "https://standards-oui.ieee.org/oui36/oui36.csv"},
	{"IAB", "https://standards-oui.ieee.org/iab/iab.csv"},
}

type registryFetcher func(context.Context, registrySource) ([]byte, error)

func (s *store) finish(err error) {
	s.mu.Lock()
	if err == nil {
		s.lastErr = ""
	} else {
		s.lastErr = err.Error()
	}
	s.mu.Unlock()
	s.updating.Store(false)
}

func (s *store) update(ctx context.Context, fetch registryFetcher) (err error) {
	if !s.updating.CompareAndSwap(false, true) {
		return ErrBusy
	}
	defer func() { s.finish(err) }()
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	old := s.current.Load()
	rows := make([]assignment, 0, len(old.Entries))
	for _, source := range registrySources {
		data, err := fetch(ctx, source)
		if err != nil {
			return fmt.Errorf("%s download failed. The previous database was kept. %w", source.registry, err)
		}
		items, err := parseCSV(data, source.registry)
		if err != nil {
			return fmt.Errorf("%s validation failed. The previous database was kept. %w", source.registry, err)
		}
		if len(items) < old.counts[source.registry]*3/4 {
			return fmt.Errorf("the %s download is unexpectedly small; the previous database was kept", source.registry)
		}
		rows = append(rows, items...)
	}
	now := time.Now().UTC()
	db, err := buildDatabase(fileData{Format: 1, Source: "IEEE public registries (MA-L, MA-M, MA-S, IAB)", SourceDate: now.Format("2006-01-02"), UpdatedAt: now.Format(time.RFC3339), Entries: rows}, false)
	if err != nil {
		return err
	}
	return s.persist(ctx, db)
}

func (s *store) importCSV(ctx context.Context, data []byte) (err error) {
	if !s.updating.CompareAndSwap(false, true) {
		return ErrBusy
	}
	defer func() { s.finish(err) }()
	rows, err := parseCSV(data, "")
	if err != nil {
		return err
	}
	byPrefix := make(map[string]assignment, len(s.current.Load().Entries)+len(rows))
	for _, row := range s.current.Load().Entries {
		byPrefix[row.Prefix] = row
	}
	seen := make(map[string]assignment)
	for _, row := range rows {
		if previous, exists := seen[row.Prefix]; exists && previous != row {
			return errors.New("import contains conflicting assignments for the same prefix")
		}
		seen[row.Prefix], byPrefix[row.Prefix] = row, row
	}
	merged := make([]assignment, 0, len(byPrefix))
	for _, row := range byPrefix {
		merged = append(merged, row)
	}
	now := time.Now().UTC()
	// A CSV carries no verifiable snapshot date. Record the actual import time,
	// but do not label old or mixed registry contents as a current-day snapshot.
	db, err := buildDatabase(fileData{Format: 1, Source: "Imported IEEE CSV merged with the local database", UpdatedAt: now.Format(time.RFC3339), Entries: merged}, false)
	if err != nil {
		return err
	}
	return s.persist(ctx, db)
}

func (s *store) persist(ctx context.Context, db *database) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// The saved representation must satisfy the same limit enforced on restart.
	// CSV size alone cannot guarantee this: imports merge with existing rows,
	// and JSON escaping can make organization names considerably larger.
	plain, err := json.Marshal(db.fileData)
	if err != nil {
		return err
	}
	if len(plain)+1 > maxDatabaseBytes {
		return errors.New("the merged vendor database exceeds the 24 MiB saved-data limit; the previous database was kept")
	}
	s.mu.RLock()
	dir := s.dir
	s.mu.RUnlock()
	if dir == "" {
		return errors.New("the vendor database storage folder is not configured")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("could not save the vendor database; previous data kept: %w", err)
	}
	f, err := os.CreateTemp(dir, ".network-toolkit-vendors-*.tmp")
	if err != nil {
		return fmt.Errorf("could not save the vendor database; previous data kept: %w", err)
	}
	name := f.Name()
	defer os.Remove(name)
	defer f.Close()
	z := gzip.NewWriter(f)
	if _, err := z.Write(append(plain, '\n')); err != nil {
		_ = z.Close()
		return err
	}
	if err := z.Close(); err != nil {
		return err
	}
	if info, err := f.Stat(); err != nil {
		return err
	} else if info.Size() > maxDatabaseBytes {
		return errors.New("the compressed vendor database exceeds the 24 MiB saved-file limit; the previous database was kept")
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := replaceDatabaseFile(name, filepath.Join(dir, DatabaseFilename)); err != nil {
		return fmt.Errorf("could not replace the vendor database; previous data kept: %w", err)
	}
	s.current.Store(db)
	return nil
}

func (s *store) reset() (err error) {
	if !s.updating.CompareAndSwap(false, true) {
		return ErrBusy
	}
	defer func() { s.finish(err) }()
	db, err := decodeDatabase(bundledBytes, true)
	if err != nil {
		return err
	}
	s.mu.RLock()
	dir := s.dir
	s.mu.RUnlock()
	if dir != "" {
		if err := os.Remove(filepath.Join(dir, DatabaseFilename)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("could not remove the saved vendor database: %w", err)
		}
	}
	s.current.Store(db)
	return nil
}
