package terminal

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKnownHostTrustOnceSaveRemoveAndReload(t *testing.T) {
	directory := t.TempDir()
	store, err := loadKnownHostStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	key, _ := generateSSHKey(t)
	prompts := 0
	once := func(context.Context, Prompt) (PromptReply, error) { prompts++; return PromptReply{Action: "once"}, nil }
	for i := 0; i < 2; i++ {
		if err := store.verify(context.Background(), "router.test:22", key.PublicKey(), once); err != nil {
			t.Fatal(err)
		}
	}
	if prompts != 2 {
		t.Fatal("trust once was saved unexpectedly")
	}
	if _, err := os.Stat(store.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("trust once wrote a file")
	}
	if err := store.verify(context.Background(), "router.test:22", key.PublicKey(), func(context.Context, Prompt) (PromptReply, error) { return PromptReply{Action: "save"}, nil }); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadKnownHostStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := loaded.verify(context.Background(), "router.test:22", key.PublicKey(), func(context.Context, Prompt) (PromptReply, error) {
		t.Error("remembered key prompted again")
		return PromptReply{}, errors.New("unexpected prompt")
	}); err != nil {
		t.Fatal(err)
	}
	m := newManager(context.Background(), loaded, nil)
	defer m.Close()
	rows, err := m.KnownHosts()
	if err != nil || len(rows) != 1 || rows[0].Host != "router.test:22" {
		t.Fatalf("known-host list %+v %v", rows, err)
	}
	if err := m.RemoveKnownHost("router.test"); !errors.Is(err, ErrNotFound) {
		t.Fatal("removal accepted a different endpoint")
	}
	if err := m.RemoveKnownHost("router.test:22"); err != nil {
		t.Fatal(err)
	}
	reloaded, err := loadKnownHostStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	rows, _ = reloaded.list()
	if len(rows) != 0 {
		t.Fatal("removed host key survived restart")
	}
}

func TestKnownHostChangedKeyCannotUseTrustOnce(t *testing.T) {
	store, err := loadKnownHostStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, _ := generateSSHKey(t)
	second, _ := generateSSHKey(t)
	if err := store.verify(context.Background(), "router.test:2222", first.PublicKey(), func(context.Context, Prompt) (PromptReply, error) { return PromptReply{Action: "save"}, nil }); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(store.path)
	for _, action := range []string{"once", "save", "replace", "reject"} {
		err := store.verify(context.Background(), "router.test:2222", second.PublicKey(), func(_ context.Context, p Prompt) (PromptReply, error) {
			if !p.Changed || len(p.PreviousFingerprints) != 1 {
				t.Error("changed-key context missing")
			}
			return PromptReply{Action: action, Fingerprint: "incorrect"}, nil
		})
		if err == nil {
			t.Errorf("changed key accepted action %q without verified fingerprint", action)
		}
	}
	after, _ := os.ReadFile(store.path)
	if !bytes.Equal(before, after) {
		t.Fatal("failed changed-key confirmation altered stored trust")
	}
}

func TestKnownHostConcurrentDecisionIsNotSilentlyReplaced(t *testing.T) {
	store, err := loadKnownHostStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, _ := generateSSHKey(t)
	second, _ := generateSSHKey(t)
	err = store.verify(context.Background(), "router.test:22", first.PublicKey(), func(context.Context, Prompt) (PromptReply, error) {
		if err := store.verify(context.Background(), "router.test:22", second.PublicKey(), func(context.Context, Prompt) (PromptReply, error) { return PromptReply{Action: "save"}, nil }); err != nil {
			t.Fatal(err)
		}
		return PromptReply{Action: "save"}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "while confirmation was pending") {
		t.Fatalf("concurrent trust mutation was overwritten: %v", err)
	}
}

func TestKnownHostCorruptionDoesNotDisableOtherTransportsOrOverwrite(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "Network_Toolkit.ssh_known_hosts.json")
	original := []byte("{broken known-host file\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(context.Background(), directory)
	if err == nil || m == nil {
		t.Fatalf("malformed trust handling manager=%v err=%v", m, err)
	}
	defer m.Close()
	if !strings.Contains(err.Error(), "restart") {
		t.Fatal("repair guidance omitted required restart")
	}
	if _, err := m.Connect(terminalRequest()); err == nil {
		t.Fatal("SSH ignored unreadable remembered trust")
	}
	if _, err := m.KnownHosts(); err == nil {
		t.Fatal("unreadable known-host list appeared empty")
	}
	if err := m.RemoveKnownHost("router.test:22"); err == nil {
		t.Fatal("unreadable trust was reset by removal")
	}
	m.openers["telnet"] = func(context.Context, ConnectRequest, transportCallbacks) (transport, error) {
		return newFakeTerminal(false), nil
	}
	req := terminalRequest()
	req.Type = "telnet"
	info, err := m.Connect(req)
	if err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	after, _ := os.ReadFile(path)
	if !bytes.Equal(original, after) {
		t.Fatal("malformed trust was silently overwritten")
	}
}

func TestKnownHostFailedSaveDoesNotChangeMemory(t *testing.T) {
	store, err := loadKnownHostStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(store.path, 0700); err != nil {
		t.Fatal(err)
	}
	key, _ := generateSSHKey(t)
	err = store.verify(context.Background(), "router.test:22", key.PublicKey(), func(context.Context, Prompt) (PromptReply, error) { return PromptReply{Action: "save"}, nil })
	if err == nil {
		t.Fatal("saved a key over a non-file path")
	}
	rows, _ := store.list()
	if len(rows) != 0 {
		t.Fatal("failed save changed in-memory remembered trust")
	}
}
