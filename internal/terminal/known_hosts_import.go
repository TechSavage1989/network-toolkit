package terminal

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
)

// KnownHostConflict is a host for which two trust files remember different
// keys. Fingerprints are public.
type KnownHostConflict struct {
	Host    string `json:"host"`
	Current string `json:"current"`
	Other   string `json:"other"`
}

// KnownHostImport reports what ImportKnownHosts did.
type KnownHostImport struct {
	// Hosts added to the trust file.
	Added []string
	// Hosts whose key differs from the one already trusted. Nothing was
	// changed for them: the trusted key stays.
	Conflicts []KnownHostConflict
	// Every entry of the other file, as host and fingerprint, including
	// those skipped or already present.
	Entries map[string]string
}

// ImportKnownHosts adds the hosts of another trust file (one an earlier
// version kept for a standalone window) to the trust file of dataDir. Both
// files pass the same strict validation as at start. A host is added only if
// it is not trusted yet; a host with a different key is never replaced and is
// reported as a conflict. skip(host, fingerprint) lets the caller leave out
// entries it has handled before. The other file is only read.
func ImportKnownHosts(dataDir, otherFile string, lock func() (func(), error), skip func(host, fingerprint string) bool) (KnownHostImport, error) {
	result := KnownHostImport{Entries: map[string]string{}}
	other := &knownHostStore{path: otherFile}
	entries, _, err := other.read()
	if err != nil {
		return result, fmt.Errorf("the previous SSH host trust file %s could not be read and was not used: %w", otherFile, err)
	}
	if len(entries) == 0 {
		return result, nil
	}
	store, err := loadKnownHostStore(dataDir)
	if err != nil {
		return result, err
	}
	store.lock = lock
	hosts := make([]string, 0, len(entries))
	for host := range entries {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	store.mu.Lock()
	defer store.mu.Unlock()
	err = store.transactionLocked(func() error {
		next := make(map[string]knownHostEntry, len(store.entries)+len(entries))
		for host, entry := range store.entries {
			next[host] = entry
		}
		for _, host := range hosts {
			entry := entries[host]
			result.Entries[host] = entry.Fingerprint
			if skip != nil && skip(host, entry.Fingerprint) {
				continue
			}
			current, trusted := next[host]
			switch {
			case trusted && bytes.Equal(current.Key, entry.Key):
			case trusted:
				result.Conflicts = append(result.Conflicts, KnownHostConflict{Host: host, Current: current.Fingerprint, Other: entry.Fingerprint})
			case len(next) >= maxKnownHosts:
				return errors.New("known-host store reached its 2048-host limit")
			default:
				next[host] = entry
				result.Added = append(result.Added, host)
			}
		}
		if len(result.Added) == 0 {
			return nil
		}
		if err := store.persist(next); err != nil {
			return err
		}
		store.entries = next
		return nil
	})
	if err != nil {
		result.Added = nil
	}
	return result, err
}
