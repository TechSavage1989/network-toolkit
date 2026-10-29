package main

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func TestEditorLoadedDocumentsAreSecretFreeAndCountedOnceThroughTransfer(t *testing.T) {
	a, m, _ := editorFixture(t)
	if err := m.updateLoaded("", []editorLoadedDocument{{UID: editorFixtureUID, Loaded: true}}); err != nil {
		t.Fatal(err)
	}
	if n := a.activitySnapshot().Tools["editor"].Documents; n != 1 {
		t.Fatal(n)
	}
	req, _, child := editorFixtureOffer(t, m)
	if n := m.loadedCount(); n != 1 {
		t.Fatalf("offered duplicate: %d", n)
	}
	if err := m.updateLoaded(child, []editorLoadedDocument{{UID: editorFixtureUID, Loaded: true}}); err == nil {
		t.Fatal("staged receiver claimed activity ownership")
	}
	if _, err := m.accept(child, req.ID); err != nil {
		t.Fatal(err)
	}
	lease := m.documents[editorFixtureUID].lease
	if err := m.updateLoaded("", []editorLoadedDocument{{UID: editorFixtureUID, Lease: lease - 1, Loaded: true}}); err == nil {
		t.Fatal("stale source activity accepted")
	}
	if err := m.updateLoaded(child, []editorLoadedDocument{{UID: editorFixtureUID, Lease: lease, Loaded: true}}); err != nil {
		t.Fatal(err)
	}
	if n := m.loadedCount(); n != 1 {
		t.Fatal(n)
	}
	encoded, _ := json.Marshal(a.activitySnapshot())
	if strings.Contains(string(encoded), "private-document") || strings.Contains(string(encoded), editorFixtureUID) || strings.Contains(string(encoded), "Untitled") {
		t.Fatalf("metadata leak: %s", encoded)
	}
	if err := m.releaseDocument(child, editorFixtureUID, lease); err != nil {
		t.Fatal(err)
	}
	if n := m.loadedCount(); n != 0 {
		t.Fatalf("released source metadata resurrected: %d", n)
	}
}
func TestEditorLoadedEmptyAndUnregisteredLifetime(t *testing.T) {
	_, m, _ := editorFixture(t)
	d := editorFixtureBatch().Documents[0]
	d.Content = ""
	d.Dirty = false
	d.Undo = nil
	d.Redo = nil
	d.EditRevision = 1
	if editorSnapshotLoaded(d) {
		t.Fatal("untouched draft counted")
	}
	d.Untitled = false
	if !editorSnapshotLoaded(d) {
		t.Fatal("saved empty file omitted")
	}
	d.Untitled = true
	d.EditRevision = 3
	if !editorSnapshotLoaded(d) {
		t.Fatal("typed then undone draft omitted")
	}
	_, _, child := editorFixtureOffer(t, m)
	if err := m.updateLoaded(child, []editorLoadedDocument{{UID: editorFixtureUID2, Loaded: true}}); err != nil {
		t.Fatal(err)
	}
	if m.loadedCount() != 2 {
		t.Fatal("desktop document omitted")
	}
	m.processExited(child)
	if m.loadedCount() != 1 {
		t.Fatal("exited renderer metadata retained")
	}
	if err := m.updateLoaded("", []editorLoadedDocument{{UID: editorFixtureUID2, Loaded: false}}); err != nil {
		t.Fatal(err)
	}
	if m.loadedCount() != 1 {
		t.Fatal("blank counted")
	}
	if err := m.updateLoaded("", []editorLoadedDocument{}); err != nil {
		t.Fatal(err)
	}
}
func TestEditorLoadedSchemaLeaseAndBounds(t *testing.T) {
	a, m, _ := editorFixture(t)
	for _, body := range []any{map[string]any{"documents": []any{map[string]any{"uid": editorFixtureUID, "loaded": true, "content": "secret"}}}, map[string]any{"documents": []any{map[string]any{"uid": editorFixtureUID, "loaded": "yes"}}}} {
		r := editorHTTP(a, a.token, "/api/editor-views/current/activity", body)
		if r.Code != 400 {
			t.Fatalf("schema %d %s", r.Code, r.Body.String())
		}
	}
	for _, rows := range [][]editorLoadedDocument{{{UID: "bad"}}, {{UID: editorFixtureUID, Lease: 9}}, {{UID: editorFixtureUID}, {UID: editorFixtureUID}}, make([]editorLoadedDocument, 65)} {
		if m.updateLoaded("", rows) == nil {
			t.Fatal("invalid metadata accepted")
		}
	}
	if err := m.updateLoaded("unknown", nil); err == nil {
		t.Fatal("unknown view accepted")
	}
	if len(m.documents) != 0 {
		t.Fatal("activity created a grant")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = m.updateLoaded("", []editorLoadedDocument{{UID: editorFixtureUID, Loaded: j%2 == 0}})
				_ = a.activitySnapshot()
			}
		}()
	}
	wg.Wait()
}
func TestEditorClosePreferenceDefaultFalseAndReadOnlyChildProjection(t *testing.T) {
	a, m, _ := editorFixture(t)
	for _, tc := range []struct {
		state string
		want  bool
	}{{`{}`, true}, {`{"editorPreferences":{"askSaveBeforeClose":false}}`, false}, {`{"editorPreferences":{"askSaveBeforeClose":true}}`, true}, {`{"editorPreferences":{"askSaveBeforeClose":"false"}}`, true}} {
		a.stateMu.Lock()
		a.state = []byte(tc.state)
		a.stateMu.Unlock()
		if a.editorAskSaveBeforeClose() != tc.want {
			t.Fatal(tc.state)
		}
		if m.state("")["askSaveBeforeClose"] != tc.want {
			t.Fatal("state projection")
		}
	}
	_, _, child := editorFixtureOffer(t, m)
	token, _ := m.token(child)
	response := editorHTTP(a, token, "/api/state", map[string]any{"editorPreferences": map[string]bool{"askSaveBeforeClose": false}})
	if response.Code != 403 {
		t.Fatalf("child preference authority %d", response.Code)
	}
}
