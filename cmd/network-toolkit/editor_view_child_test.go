package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func validEditorLaunch(t *testing.T) editorViewLaunch {
	t.Helper()
	id := strings.Repeat("ab", 16)
	return editorViewLaunch{Version: 1, OwnerHost: "127.0.0.1:12345", OwnerInstanceID: strings.Repeat("cd", 16), ViewID: id, ViewToken: strings.Repeat("ef", 32), DataDir: filepath.Join(t.TempDir(), editorViewDirectoryName, id), Title: "Network Toolkit Editor"}
}
func TestEditorLaunchPrivateBoundedBootstrap(t *testing.T) {
	launch := validEditorLaunch(t)
	raw, err := encodeEditorViewLaunch(launch)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeEditorViewLaunch(bytes.NewReader(raw))
	if err != nil || got.ViewID != launch.ViewID {
		t.Fatal("roundtrip failed", err)
	}
	if strings.Contains(strings.Join(editorViewChildArguments(false), " "), launch.ViewToken) || strings.Join(editorViewChildArguments(false), " ") != "--editor-view" {
		t.Fatal("capability leaked in arguments")
	}
	for _, mutate := range []func(*editorViewLaunch){func(x *editorViewLaunch) { x.OwnerHost = "localhost:12345" }, func(x *editorViewLaunch) { x.OwnerHost = "127.0.0.1:012345" }, func(x *editorViewLaunch) { x.ViewToken = "bad" }, func(x *editorViewLaunch) { x.DataDir = filepath.Join(t.TempDir(), "other", x.ViewID) }, func(x *editorViewLaunch) { x.Title = "bad\nheader" }, func(x *editorViewLaunch) { x.Version = 2 }} {
		bad := launch
		mutate(&bad)
		if _, err := encodeEditorViewLaunch(bad); err == nil {
			t.Fatal("invalid bootstrap accepted")
		}
	}
	for _, raw := range []string{`{"unknown":"document body"}`, strings.Repeat("x", editorViewBootstrapLimit+1), string(raw) + "{}"} {
		if _, err := decodeEditorViewLaunch(strings.NewReader(raw)); err == nil {
			t.Fatal("invalid or oversized bootstrap accepted")
		}
	}
}
func TestEditorControlIsExactCapabilityBoundedLoopback(t *testing.T) {
	launch := validEditorLaunch(t)
	seen := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen++
		if r.URL.RawQuery != "" || r.Method != "POST" || r.URL.Path != "/api/editor-views/current/pulse" || r.Header.Get("X-Session-Token") != launch.ViewToken {
			t.Error("wrong private route")
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "{}" {
			t.Error("control carries document data")
		}
		_ = json.NewEncoder(w).Encode(editorViewPulse{OwnerInstanceID: launch.OwnerInstanceID, ViewID: launch.ViewID, State: "opening"})
	}))
	defer server.Close()
	launch.OwnerHost = strings.TrimPrefix(server.URL, "http://")
	control := newEditorViewControl(launch)
	defer control.client.CloseIdleConnections()
	pulse, err := control.pulse(context.Background())
	if err != nil || pulse.State != "opening" || seen != 1 {
		t.Fatal(pulse, err, seen)
	}
	if _, _, err := control.request(context.Background(), "../file"); err == nil || seen != 1 {
		t.Fatal("untrusted control route accepted")
	}
}
