package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Server lifecycle and local file management must remain available only to
// authenticated requests from the app, and file management must not require
// leaving an unauthenticated TFTP/SFTP listener running.
func TestTransferServerAPIAccessAndOfflineFileManager(t *testing.T) {
	a := testApplication(t)
	for _, endpoint := range []string{"/api/transfer/server?protocol=sftp", "/api/transfer/files?protocol=sftp", "/api/transfer/local"} {
		if r := requestApp(a, http.MethodGet, endpoint, "", false); r.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated GET %s was accepted: %d", endpoint, r.Code)
		}
	}
	if r := requestApp(a, http.MethodPost, "/api/transfer/server", `{"action":"setroot","protocol":"sftp","root":"/tmp"}`, false); r.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated root modification accepted: %d", r.Code)
	}
	for _, endpoint := range []string{"/api/transfer/server", "/api/transfer/files"} {
		req := httptest.NewRequest(http.MethodPost, "http://"+a.host+endpoint, strings.NewReader(`{}`))
		req.Header.Set("X-Session-Token", a.token)
		req.Header.Set("Origin", "https://example.invalid")
		recorder := httptest.NewRecorder()
		a.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("cross-origin action %s accepted: %d", endpoint, recorder.Code)
		}
	}
	root := filepath.Join(t.TempDir(), "Config Backups")
	body, err := json.Marshal(map[string]any{"action": "setroot", "protocol": "sftp", "root": root})
	if err != nil {
		t.Fatal(err)
	}
	result := requestApp(a, http.MethodPost, "/api/transfer/server", string(body), true)
	if result.Code != http.StatusOK {
		t.Fatalf("select file-manager root: %d %s", result.Code, result.Body.String())
	}
	var status struct {
		Running bool   `json:"running"`
		Root    string `json:"root"`
	}
	if err := json.Unmarshal(result.Body.Bytes(), &status); err != nil || status.Running || status.Root != root {
		t.Fatalf("file-manager root must not start listener: %+v %v", status, err)
	}
	if r := requestApp(a, http.MethodGet, "/api/transfer/files?protocol=sftp", "", true); r.Code != http.StatusOK {
		t.Fatalf("offline file browser: %d %s", r.Code, r.Body.String())
	}
	if r := requestApp(a, http.MethodPost, "/api/transfer/files", `{"protocol":"sftp","action":"mkdir","path":"Backups"}`, true); r.Code != http.StatusOK {
		t.Fatalf("offline create folder: %d %s", r.Code, r.Body.String())
	}
	if info, err := os.Stat(filepath.Join(root, "Backups")); err != nil || !info.IsDir() {
		t.Fatalf("folder not created: %v", err)
	}
	if r := requestApp(a, http.MethodPost, "/api/transfer/files", `{"protocol":"sftp","action":"delete","path":"Backups"}`, true); r.Code != http.StatusBadRequest {
		t.Fatalf("deletion without confirmation allowed: %d", r.Code)
	}
	if r := requestApp(a, http.MethodPost, "/api/transfer/files", `{"protocol":"sftp","action":"delete","path":"Backups","confirm":true}`, true); r.Code != http.StatusOK {
		t.Fatalf("confirmed delete failed: %d %s", r.Code, r.Body.String())
	}
	if a.transfers.status("sftp")["running"] != false {
		t.Fatal("file manager unexpectedly started SFTP server")
	}
}
