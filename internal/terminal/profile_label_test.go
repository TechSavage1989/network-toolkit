package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"runtime"
	"testing"
)

func TestSavedProfileDisplayNameAndFallback(t *testing.T) {
	for _, kind := range []string{"ssh", "telnet", "serial", "cmd", "powershell"} {
		t.Run(kind, func(t *testing.T) {
			serial := "/dev/ttyS0"
			if runtime.GOOS == "windows" {
				serial = "COM4"
			}
			req, err := normalizeConnectionRequest(ConnectRequest{Type: kind, DisplayName: "Core <lab>", ProfileID: "saved", Host: "192.0.2.8", Username: "operator", SerialPort: serial}, true)
			if err != nil {
				t.Fatal(err)
			}
			s := newLiveSession(context.Background(), "fixture", req)
			defer s.cancel()
			if s.info.Label != "Core <lab>" {
				t.Fatalf("saved name lost: %q", s.info.Label)
			}
			raw, err := json.Marshal(req)
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string]any
			if err = json.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
			if _, ok := decoded["DisplayName"]; ok {
				t.Fatal("internal display name leaked into request JSON")
			}
			req.DisplayName = ""
			fallback := newLiveSession(context.Background(), "fallback", req)
			defer fallback.cancel()
			if fallback.info.Label == "" || fallback.info.Label == s.info.Label {
				t.Fatal("missing sensible fallback")
			}
		})
	}
}

func TestExplicitDisplayNameProvenance128(t *testing.T) {
	req := ConnectRequest{Type: "ssh", Host: "192.0.2.23", Port: 22, DisplayName: "192.0.2.23", DisplayNameExplicit: true, ProfileID: "saved", Password: "not-persisted"}
	s := newLiveSession(context.Background(), "manual128", req)
	defer s.cancel()
	if s.info.ManualName != req.DisplayName || !s.info.NameExplicit || s.info.LabelSource != "profile" {
		t.Fatalf("missing explicit display provenance: %#v", s.info)
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ConnectRequest
	if err := json.Unmarshal([]byte(`{"type":"ssh","DisplayName":"injected","DisplayNameExplicit":true,"manualName":"injected","nameExplicit":true,"labelSource":"manual"}`), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.DisplayName != "" || decoded.DisplayNameExplicit {
		t.Fatal("client supplied backend-only provenance")
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	if _, ok := obj["DisplayNameExplicit"]; ok {
		t.Fatal("backend provenance leaked into connection request")
	}
	retained := reconnectSettings(req)
	if retained.Password != "" || retained.DisplayName != req.DisplayName || !retained.DisplayNameExplicit {
		t.Fatal("safe reconnect provenance lost or password retained")
	}
	m := &Manager{sessions: map[string]*liveSession{s.info.ID: s}, eventBudget: 1024 * 1024, changed: make(chan struct{})}
	if err := m.Rename(s.info.ID, "Manual renamed <east>"); err != nil {
		t.Fatal(err)
	}
	if s.info.Label != "Manual renamed <east>" || s.info.ManualName != s.info.Label || !s.info.NameExplicit || s.info.LabelSource != "manual" {
		t.Fatal("rename provenance missing")
	}
	if s.info.Host != req.Host || s.info.Port != req.Port || s.info.ProfileID != "saved" {
		t.Fatal("rename changed connection identity")
	}
}

func TestRenameDisplay128PersistenceTransactionAndExplicitClear(t *testing.T) {
	req := ConnectRequest{Type: "ssh", Host: "192.0.2.73", Port: 22, DisplayName: "Original", DisplayNameExplicit: true, ProfileID: "saved"}
	s := newLiveSession(context.Background(), "transaction128", req)
	defer s.cancel()
	m := &Manager{sessions: map[string]*liveSession{s.info.ID: s}, eventBudget: 1024 * 1024, changed: make(chan struct{})}
	before := s.info
	if err := m.RenameDisplay(s.info.ID, "New", false, func(SessionInfo) (uint64, error) { return 0, errors.New("disk full") }); err == nil {
		t.Fatal("persistence failure ignored")
	}
	if !reflect.DeepEqual(before, s.info) || m.seq != 0 {
		t.Fatal("failed persistence changed live metadata or emitted status")
	}
	if err := m.RenameDisplay(s.info.ID, "", false, nil); err == nil {
		t.Fatal("blank ordinary rename accepted")
	}
	if err := m.RenameDisplay(s.info.ID, "", true, func(next SessionInfo) (uint64, error) {
		if next.ManualName != "" || next.NameExplicit || next.Host != req.Host || next.ProfileID != req.ProfileID {
			t.Fatal("clear altered identity")
		}
		return 7, nil
	}); err != nil {
		t.Fatal(err)
	}
	if s.info.NameRevision != 7 || s.info.ManualName != "" || s.info.NameExplicit || s.info.LabelSource != "default" {
		t.Fatalf("explicit clear not retained %+v", s.info)
	}
}
