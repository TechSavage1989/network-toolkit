package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppStorageAPIConfirmationAndDeferredShutdown(t *testing.T) {
	a := testApplication(t)
	a.storage = storageFixture(t, true)
	a.dataPath = filepath.Join(a.storage.config.RootDir, "Network_Toolkit.settings.json")
	settings := writeStorageFixture(t, a.storage.config.RootDir, "Network_Toolkit.settings.json", "saved app state")
	report := writeStorageFixture(t, a.storage.config.RootDir, "exported-report.html", "exported report")
	if result := requestApp(a, http.MethodPost, "/api/storage/delete", `{"confirmation":"DELETE APP DATA"}`, false); result.Code != http.StatusUnauthorized {
		t.Fatal("unauthenticated deletion reached the storage handler")
	}
	for _, body := range []string{`null`, `{}`, `{"confirmation":"delete"}`, `{"confirmation":"DELETE APP DATA"} {}`} {
		if result := requestApp(a, http.MethodPost, "/api/storage/delete", body, true); result.Code != http.StatusBadRequest {
			t.Fatalf("unconfirmed or malformed deletion accepted: %d %s", result.Code, result.Body.String())
		}
	}
	if a.shutdownRequested.Load() || a.storage.status().DeletePending {
		t.Fatal("invalid deletion changed the lifecycle")
	}
	if result := requestApp(a, http.MethodGet, "/api/storage/delete", "", true); result.Code != http.StatusMethodNotAllowed {
		t.Fatal("deletion accepted GET")
	}
	if result := requestApp(a, http.MethodPost, "/api/storage/delete", `{"confirmation":"DELETE APP DATA"}`, true); result.Code != http.StatusOK {
		t.Fatalf("confirmed deletion failed: %d %s", result.Code, result.Body.String())
	}
	if !a.shutdownRequested.Load() || !a.storage.status().DeletePending {
		t.Fatal("deletion was acknowledged before shutdown intent")
	}
	assertStorageContents(t, settings, "saved app state")
	select {
	case <-a.ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("confirmed deletion did not request full shutdown")
	}
	if err := a.storage.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(settings); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("deferred app-data cleanup did not finish")
	}
	assertStorageContents(t, report, "exported report")
}

func TestAppStorageAPILocationAndOtherWorkspaceConflict(t *testing.T) {
	a := testApplication(t)
	a.storage = storageFixture(t, false)
	a.dataPath = filepath.Join(a.storage.config.RootDir, "Network_Toolkit.settings.json")
	if result := requestApp(a, http.MethodPut, "/api/storage/location", `null`, true); result.Code != http.StatusBadRequest {
		t.Fatal("null location request changed the selection")
	}
	other := newAppStorage(a.storage.config)
	if err := other.Open(); err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if result := requestApp(a, http.MethodPost, "/api/storage/delete", `{"confirmation":"DELETE APP DATA"}`, true); result.Code != http.StatusConflict {
		t.Fatalf("active workspace did not block deletion: %d %s", result.Code, result.Body.String())
	}
	if err := other.Close(); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "selected")
	encoded, _ := json.Marshal(map[string]any{"path": destination, "copyExisting": false})
	result := requestApp(a, http.MethodPut, "/api/storage/location", string(encoded), true)
	if result.Code != http.StatusOK {
		t.Fatalf("location change failed: %d %s", result.Code, result.Body.String())
	}
	var response struct {
		OK      bool             `json:"ok"`
		Storage appStorageStatus `json:"storage"`
	}
	if err := json.Unmarshal(result.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.OK || !response.Storage.RestartRequired || response.Storage.NextRootDir != destination || response.Storage.WorkspaceDir != filepath.Dir(a.dataPath) {
		t.Fatalf("wrong storage response: %+v", response)
	}
	if a.ctx.Err() != nil {
		t.Fatal("selecting a next-launch folder closed the current workspace")
	}
}
