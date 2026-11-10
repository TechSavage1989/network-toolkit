package terminal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/ssh"
)

// Two stores on one trust file stand in for two apps of one data folder (for
// example the main window and the standalone Terminal). The mutex plays the
// part of the OS lock the application supplies.
func sharedKnownHostStores(t *testing.T) (*knownHostStore, *knownHostStore) {
	t.Helper()
	directory := t.TempDir()
	var gate sync.Mutex
	lock := func() (func(), error) { gate.Lock(); return gate.Unlock, nil }
	open := func() *knownHostStore {
		store, err := loadKnownHostStore(directory)
		if err != nil {
			t.Fatal(err)
		}
		store.lock = lock
		return store
	}
	return open(), open()
}

func knownHostAnswer(action string) func(context.Context, Prompt) (PromptReply, error) {
	return func(_ context.Context, prompt Prompt) (PromptReply, error) {
		return PromptReply{Action: action, Fingerprint: prompt.Fingerprint}, nil
	}
}

func knownHostNoPrompt(t *testing.T) func(context.Context, Prompt) (PromptReply, error) {
	return func(context.Context, Prompt) (PromptReply, error) {
		t.Error("a key another app already trusted prompted again")
		return PromptReply{}, errors.New("unexpected prompt")
	}
}

func knownHostNames(t *testing.T, store *knownHostStore) string {
	t.Helper()
	rows, err := store.list()
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, row := range rows {
		names = append(names, row.Host)
	}
	return strings.Join(names, ",")
}

func TestSharedKnownHostsAreTrustedAndListedByEveryApp(t *testing.T) {
	first, second := sharedKnownHostStores(t)
	key, _ := generateSSHKey(t)
	other, _ := generateSSHKey(t)
	ctx := context.Background()
	if err := first.verify(ctx, "core.test:22", key.PublicKey(), knownHostAnswer("save")); err != nil {
		t.Fatal(err)
	}
	// The second app was already running: it trusts the key without a prompt
	// and without a restart, and lists it.
	if err := second.verify(ctx, "core.test:22", key.PublicKey(), knownHostNoPrompt(t)); err != nil {
		t.Fatal(err)
	}
	if got := knownHostNames(t, second); got != "core.test:22" {
		t.Fatalf("second app lists %q", got)
	}
	// It adds its own host; the first app's later removal keeps that host.
	if err := second.verify(ctx, "edge.test:22", other.PublicKey(), knownHostAnswer("save")); err != nil {
		t.Fatal(err)
	}
	manager := newManager(ctx, first, nil)
	defer manager.Close()
	if err := manager.RemoveKnownHost("core.test:22"); err != nil {
		t.Fatal(err)
	}
	for _, store := range []*knownHostStore{first, second} {
		if got := knownHostNames(t, store); got != "edge.test:22" {
			t.Fatalf("after a removal next to another app's addition: %q", got)
		}
	}
	// A removed host is no longer trusted anywhere: it prompts again.
	asked := false
	err := second.verify(ctx, "core.test:22", key.PublicKey(), func(context.Context, Prompt) (PromptReply, error) {
		asked = true
		return PromptReply{Action: "reject"}, nil
	})
	if err == nil || !asked {
		t.Fatalf("a host removed in another app was still trusted: asked=%v err=%v", asked, err)
	}
	// A different key for a host another app trusts is a changed key, never a
	// first-use prompt.
	err = first.verify(ctx, "edge.test:22", key.PublicKey(), func(_ context.Context, prompt Prompt) (PromptReply, error) {
		if !prompt.Changed || len(prompt.PreviousFingerprints) != 1 || prompt.PreviousFingerprints[0] != ssh.FingerprintSHA256(other.PublicKey()) {
			t.Errorf("a changed key was not presented as changed: %+v", prompt)
		}
		return PromptReply{Action: "save"}, nil
	})
	if err == nil {
		t.Fatal("a changed key for another app's host was accepted with the first-use answer")
	}
}

func TestSharedKnownHostsConcurrentSavesKeepEveryHost(t *testing.T) {
	first, second := sharedKnownHostStores(t)
	const each = 20
	var wg sync.WaitGroup
	failures := make(chan error, 2*each)
	for index, store := range []*knownHostStore{first, second} {
		wg.Add(1)
		go func(index int, store *knownHostStore) {
			defer wg.Done()
			for n := 0; n < each; n++ {
				key, _ := generateSSHKey(t)
				if err := store.verify(context.Background(), fmt.Sprintf("app%d-host%d.test:22", index, n), key.PublicKey(), knownHostAnswer("save")); err != nil {
					failures <- err
					return
				}
			}
		}(index, store)
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	reloaded, err := loadKnownHostStore(filepath.Dir(first.path))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := reloaded.list()
	if err != nil || len(rows) != 2*each {
		t.Fatalf("interleaved saves from two apps kept %d of %d hosts: %v", len(rows), 2*each, err)
	}
}

func TestSharedKnownHostDecisionMadeMeanwhileInAnotherAppIsNotReplaced(t *testing.T) {
	first, second := sharedKnownHostStores(t)
	mine, _ := generateSSHKey(t)
	theirs, _ := generateSSHKey(t)
	ctx := context.Background()
	// While the first app's first-use prompt is open, the second app saves a
	// different key for the same host. The first answer must not replace it.
	err := first.verify(ctx, "router.test:22", mine.PublicKey(), func(context.Context, Prompt) (PromptReply, error) {
		if err := second.verify(ctx, "router.test:22", theirs.PublicKey(), knownHostAnswer("save")); err != nil {
			t.Error(err)
		}
		return PromptReply{Action: "save"}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "changed while confirmation was pending") {
		t.Fatalf("a decision made in another app was silently replaced: %v", err)
	}
	if err := first.verify(ctx, "router.test:22", theirs.PublicKey(), knownHostNoPrompt(t)); err != nil {
		t.Fatal(err)
	}
	// The same key saved by both apps is simply trusted.
	err = first.verify(ctx, "switch.test:22", mine.PublicKey(), func(context.Context, Prompt) (PromptReply, error) {
		if err := second.verify(ctx, "switch.test:22", mine.PublicKey(), knownHostAnswer("save")); err != nil {
			t.Error(err)
		}
		return PromptReply{Action: "save"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSharedKnownHostsUnreadableFileAndBusyLockNeverResetTrust(t *testing.T) {
	first, second := sharedKnownHostStores(t)
	key, _ := generateSSHKey(t)
	ctx := context.Background()
	if err := first.verify(ctx, "core.test:22", key.PublicKey(), knownHostAnswer("save")); err != nil {
		t.Fatal(err)
	}
	// Another app is in the middle of a change and the lock cannot be taken:
	// nothing is written and the failure is reported.
	second.lock = func() (func(), error) { return nil, errors.New("busy") }
	other, _ := generateSSHKey(t)
	if err := second.verify(ctx, "edge.test:22", other.PublicKey(), knownHostAnswer("save")); err == nil {
		t.Fatal("a host key was saved without the cross-app lock")
	}
	if got := knownHostNames(t, first); got != "core.test:22" {
		t.Fatalf("trust after a refused save: %q", got)
	}
	// A trust file damaged underneath a running app disables SSH there, as at
	// start; it is never replaced by what the app holds in memory.
	if err := os.WriteFile(first.path, []byte(`{"version":1,"hosts":[`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := first.verify(ctx, "core.test:22", key.PublicKey(), knownHostAnswer("save")); err == nil {
		t.Fatal("a damaged trust file was ignored")
	}
	if _, err := first.list(); err == nil {
		t.Fatal("a damaged trust file still listed hosts")
	}
	if raw, _ := os.ReadFile(first.path); string(raw) != `{"version":1,"hosts":[` {
		t.Fatal("the damaged trust file was overwritten")
	}
}
