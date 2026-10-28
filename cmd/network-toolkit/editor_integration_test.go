package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestEditorIntegrationPendingMoveBlocksShutdownAndUpdater(t *testing.T) {
	a, m, _ := editorFixture(t)
	_, _, _ = editorFixtureOffer(t, m)
	response := editorHTTP(a, a.token, "/api/shutdown", map[string]bool{})
	if response.Code != http.StatusConflict {
		t.Fatalf("pending move shutdown: %d", response.Code)
	}
	a.views.mu.Lock()
	closing := a.views.homeClosing
	a.views.mu.Unlock()
	if closing || a.shutdownRequested.Load() {
		t.Fatal("rejected close changed irreversible lifecycle state")
	}
	if !strings.Contains(strings.Join(a.updateBusy(), ","), "editor") {
		t.Fatal("live editor omitted from updater restart guard")
	}
}
func TestEditorIntegrationIdleRegistryDoesNotBlockUpdater(t *testing.T) {
	a, _, _ := editorFixture(t)
	if strings.Contains(strings.Join(a.updateBusy(), ","), "editor") {
		t.Fatal("empty registry blocked updater")
	}
}
