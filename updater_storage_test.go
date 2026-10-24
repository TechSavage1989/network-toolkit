package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestUpdateFinalizingMutationGate(t *testing.T) {
	a := testApplication(t)
	a.updateFinalizing.Store(true)
	for _, route := range []struct{ method, path string }{{"POST", "/api/jobs"}, {"PUT", "/api/state"}, {"POST", "/api/local-tools/apply"}, {"POST", "/api/updates/install"}} {
		if r := requestApp(a, route.method, route.path, `{}`, true); r.Code != 503 {
			t.Fatalf("%s %s not blocked: %d", route.method, route.path, r.Code)
		}
	}
	if len(a.jobs) != 0 {
		t.Fatal("job created while finalizing")
	}
	if _, err := os.Stat(a.dataPath); !os.IsNotExist(err) {
		t.Fatal("settings written while finalizing")
	}
	r := requestApp(a, "GET", "/", "", false)
	if r.Code != 200 || !strings.Contains(r.Body.String(), "updateFinalizingCover") {
		t.Fatal("bootstrap interface overlay absent")
	}
	if r := requestApp(a, "GET", "/api/updates/status", "", true); r.Code != 200 || !strings.Contains(r.Body.String(), `"finalizing":true`) {
		t.Fatal("finalizing status unavailable")
	}
	a.updateFinalizing.Store(false)
	if r := requestApp(a, "PUT", "/api/state", `{"theme":"dark"}`, true); r.Code != 200 {
		t.Fatal("gate did not clear", r.Code)
	}
}

func TestUpdateStageUsesResolvedSettingsAndSelectiveCleanup(t *testing.T) {
	updateNamingState.Lock()
	prior := updateNamingState.status
	updateNamingState.status = updateExecutableNamingStatus{}
	updateNamingState.Unlock()
	defer func() { updateNamingState.Lock(); updateNamingState.status = prior; updateNamingState.Unlock() }()
	a := testApplication(t)
	binary := []byte("candidate owned by session")
	if err := a.updateStageBinary("1.17.99", binary); err != nil {
		t.Fatal(err)
	}
	dir := a.updater.staged.dir
	rel, err := filepath.Rel(filepath.Dir(a.dataPath), dir)
	if err != nil || strings.HasPrefix(rel, "..") || !strings.HasPrefix(rel, "updates"+string(filepath.Separator)) {
		t.Fatalf("not settings based: %s", dir)
	}
	foreign := filepath.Join(dir, "my-file.txt")
	os.WriteFile(foreign, []byte("preserve"), 0600)
	a.cleanupStagedUpdate()
	if b, err := os.ReadFile(foreign); err != nil || string(b) != "preserve" {
		t.Fatal("unknown file removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "candidate.exe")); !os.IsNotExist(err) {
		t.Fatal("owned candidate remains")
	}
}

func TestUpdateCopyVerifiedRefusesChangedAndExistingFiles(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "source"), filepath.Join(dir, "dest")
	os.WriteFile(src, []byte("original"), 0600)
	hash := updateHash([]byte("original"))
	if err := updateCopyVerified(src, dst, hash); err != nil {
		t.Fatal(err)
	}
	if err := updateCopyVerified(src, dst, hash); err == nil {
		t.Fatal("overwrote existing target")
	}
	os.WriteFile(dst, []byte("user changed"), 0600)
	if err := updateRemoveVerified(dst, hash); err == nil {
		t.Fatal("deleted changed bytes")
	}
}

func TestUpdateReopenWaitsBeforeNormalArguments(t *testing.T) {
	dir, err := os.MkdirTemp(t.TempDir(), ".network-toolkit-reopen-")
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"canonical.exe", "--update-reopen", "123", dir, "--data-dir", "relative settings", "--no-browser"}
	waited := false
	got, err := prepareUpdateReopen(args, func(pid int, timeout time.Duration, ready string) error {
		waited = true
		if pid != 123 || timeout != 45*time.Second || filepath.Dir(ready) != dir {
			t.Fatal("parent wait contract")
		}
		return os.WriteFile(ready, []byte("ready"), 0600)
	})
	if err != nil || !waited || !reflect.DeepEqual(got, []string{"canonical.exe", "--data-dir", "relative settings", "--no-browser"}) {
		t.Fatalf("%v %v", got, err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("reopen marker left behind")
	}
	for _, bad := range [][]string{{"app", "--update-reopen"}, {"app", "--update-reopen", "0", dir}, {"app", "--update-reopen", "123", t.TempDir()}} {
		if _, err := prepareUpdateReopen(bad, func(int, time.Duration, string) error { t.Fatal("invalid request waited"); return nil }); err == nil {
			t.Fatal("accepted invalid reopen")
		}
	}
}
